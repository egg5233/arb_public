---
phase: 14
slug: daily-reconcile-live-ramp-controller
status: draft
nyquist_compliant: false
wave_0_complete: false
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
| TBD | TBD | TBD | PG-LIVE-01 / PG-LIVE-03 | TBD | TBD | unit/integration | TBD | ❌ W0 | ⬜ pending |

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

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s for pricegaptrader package
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
