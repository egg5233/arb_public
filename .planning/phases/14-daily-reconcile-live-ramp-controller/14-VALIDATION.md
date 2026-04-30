---
phase: 14
slug: daily-reconcile-live-ramp-controller
status: draft
nyquist_compliant: true
wave_0_complete: planned
created: 2026-04-30
---

# Phase 14 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | `go.mod` |
| **Quick run command** | `go test ./internal/pricegaptrader/... -count=1 -timeout=60s` |
| **Full suite command** | `go test ./... -count=1 -timeout=300s` |
| **Estimated runtime** | ~30 seconds (pricegaptrader) / ~180 seconds (full) |

---

## Sampling Rate

- **After every task commit:** Run quick command (pricegaptrader package only)
- **After every plan wave:** Run full suite command
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds (pricegaptrader package)

---

## Per-Task Verification Map

> Populated by gsd-planner. Every plan task with concrete code change must have a row here with an `<automated>` command, OR a `Wave 0` dependency that creates the test stub. The planner will fill this from RESEARCH.md §"Validation Architecture" (26 named tests across 7 files).

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 14-01-T1 | 14-01 | 1 | PG-LIVE-01/03 | T-14-01,06 | Wave-0 test stubs (red→green pattern) | scaffold | `go test ./internal/pricegaptrader/... -run 'TestReconcile_\|TestRampController_\|TestRiskGate_Gate6_Ramp\|TestSizer_\|TestNotifier_\|TestDaemon_'` | ✅ created in 14-01 | ⬜ pending |
| 14-01-T2 | 14-01 | 1 | PG-LIVE-01 | T-14-01,04 | Validator rejects live+paper combo, ceiling>1000, stage order violation | unit | `go test ./internal/config/... -run 'TestValidatePriceGapLive\|TestPriceGapLiveDTORoundTrip' -count=1` | ✅ created in 14-01 | ⬜ pending |
| 14-01-T3 | 14-01 | 1 | PG-LIVE-03 | T-14-06,09 | Position model + Store interface + monitor SADD hook | unit | `go test ./internal/database/... -run 'TestPriceGapClosedSet\|TestPriceGapReconcileDaily\|TestPriceGapRampState\|TestPriceGapRampEvents' -count=1` | ✅ created in 14-01 | ⬜ pending |
| 14-02-T1 | 14-02 | 2 | PG-LIVE-03 | T-14-06 | DailyReconcileRecord typed struct (deterministic JSON) | unit | `go build ./internal/pricegaptrader/...` | ✅ created in 14-02 | ⬜ pending |
| 14-02-T2 | 14-02 | 2 | PG-LIVE-03 | T-14-03,06,10,11 | Reconciler tests (RED phase — Reconciler undefined) | unit (RED) | `go test ./internal/pricegaptrader/... -run 'TestReconcile_' -count=1` (expected FAIL) | ✅ created in 14-02 | ⬜ pending |
| 14-02-T3 | 14-02 | 2 | PG-LIVE-03 | T-14-03,06,10,11 | Reconciler implementation (GREEN — all 4 tests pass) | unit | `go test ./internal/pricegaptrader/... -run 'TestReconcile_' -count=1` | ✅ created in 14-02 | ⬜ pending |
| 14-03-T1 | 14-03 | 2 | PG-LIVE-01 | T-14-01,07 | Sizer.Cap defense-in-depth typo Stage3=9999 still caps at 1000 | unit | `go test ./internal/pricegaptrader/... -run 'TestSizer_' -count=1` | ✅ created in 14-03 | ⬜ pending |
| 14-03-T2 | 14-03 | 2 | PG-LIVE-01 | T-14-02,05,08,12 | RampController tests (RED) — invariant + persistence + force-ops | unit (RED) | `go test ./internal/pricegaptrader/... -run 'TestRampController_' -count=1` (expected FAIL) | ✅ created in 14-03 | ⬜ pending |
| 14-03-T3 | 14-03 | 2 | PG-LIVE-01 | T-14-02,05,08,12 | RampController + RedisWSRampSink (GREEN — 7 tests pass) | unit | `go test ./internal/pricegaptrader/... -run 'TestRampController_' -count=1 -count=100` (idempotency check) | ✅ created in 14-03 | ⬜ pending |
| 14-03-T4 | 14-03 | 2 | PG-LIVE-01 | T-14-01 | Gate 6 ramp inserted; ErrPriceGapRampExceeded; "ramp" in Telegram allowlist | unit | `go test ./internal/pricegaptrader/... -run 'TestRiskGate_' -count=1` | ✅ created in 14-03 | ⬜ pending |
| 14-04-T1 | 14-04 | 3 | PG-LIVE-01/03 | T-14-13 | TelegramNotifier 4 methods + allowlist invariant test | unit | `go test ./internal/pricegaptrader/... -run 'TestNotifier_' -count=1` | ✅ created in 14-04 | ⬜ pending |
| 14-04-T2 | 14-04 | 3 | PG-LIVE-01/03 | T-14-04,14 | reconcileLoop daemon + boot guard + boot-catchup; nextUTCFireTime | unit | `go test ./internal/pricegaptrader/... -run 'TestDaemon_' -count=1` | ✅ created in 14-04 | ⬜ pending |
| 14-04-T3 | 14-04 | 3 | PG-LIVE-01/03 | T-14-13,15,16 | pg-admin 6 subcommands + VERSION+CHANGELOG bump | integration | `go test ./cmd/pg-admin/... -count=1` | ✅ created in 14-04 | ⬜ pending |
| 14-05-T1 | 14-05 | 4 | PG-LIVE-01/03 | T-14-15 | API handlers /api/pg/ramp + /api/pg/reconcile/{date} (regex date validation) | integration | `go test ./internal/api/... -run 'TestHandlePgRamp\|TestHandlePgReconcile' -count=1` | ✅ created in 14-05 | ⬜ pending |
| 14-05-T2 | 14-05 | 4 | PG-LIVE-01/03 | T-14-17,18 | Frontend RampReconcileSection + i18n EN+zh-TW lockstep | static | `npm run build && grep -c 'pricegap\.ramp\.\|pricegap\.reconcile\.' web/src/i18n/{en,zh-TW}.ts` | ✅ created in 14-05 | ⬜ pending |
| 14-05-T3 | 14-05 | 4 | PG-LIVE-01 | T-14-18 | Human-verify dashboard widget (paper/live badge, no mutation, EN/zh-TW) | manual | n/a (checkpoint) | n/a | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

> Test stubs that must be created in Wave 0 before any production code lands.

- [ ] `internal/pricegaptrader/reconciler_test.go` — idempotency byte-equality, 3-retry, anomaly flagging
- [ ] `internal/pricegaptrader/ramp_controller_test.go` — asymmetric ratchet invariant, kill -9 persistence, hard ceiling
- [ ] `internal/pricegaptrader/risk_gate_test.go` — Gate 6 returns `ErrPriceGapRampExceeded` for over-budget
- [ ] `internal/pricegaptrader/sizer_test.go` — `min(stage_size, hard_ceiling)` enforced at sizing call site
- [ ] `internal/pricegaptrader/notify_test.go` — `NotifyPriceGapDailyDigest` dispatch via `pricegap_digest` allowlist
- [ ] `internal/pricegaptrader/daemon_test.go` — UTC 00:30 fire, boot-time catchup, graceful shutdown
- [ ] `cmd/pg-admin/reconcile_test.go` and `cmd/pg-admin/ramp_test.go` — subcommand semantics (force-promote, force-demote, reset)

*Existing test scaffolding in `internal/pricegaptrader/` covers Phase 11+12 patterns; reuse fixture helpers where present.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Telegram digest message readable on phone | PG-LIVE-03 | UX/format judgement, not API contract | After first day with closed Strategy 4 positions, verify digest contains realized PnL, count, win/loss, ramp stage, anomaly list — readable in Telegram mobile UI |
| Dashboard widget visual layout | PG-LIVE-01 | UI rendering / i18n EN+zh-TW lockstep | Open dashboard → Pricegap-tracker tab → confirm Ramp + Reconcile panel renders, "Live capital: ON/OFF" badge color-coded, both locales display correct labels |
| `kill -9` mid-stage restart | PG-LIVE-01 (#3) | Requires SIGKILL (not testable via go test) | Set ramp to stage 2 with counter=3, `kill -9 $(pgrep arb)`, restart, run `pg-admin ramp show` → assert stage=2, counter=3 |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (planning)
- [x] Sampling continuity verified (each task has automated verify)
- [x] Wave 0 (Plan 14-01 Task 1) creates 6 stub files with 22 named tests; downstream plans replace t.Skip with real bodies
- [x] No watch-mode flags (all -count=1)
- [x] Feedback latency < 60s (pricegaptrader unit tests)
- [x] `nyquist_compliant: true` will be set after Plan 14-01 commits stubs

**Approval:** approved (planner) — execution pending
