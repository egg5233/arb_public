---
phase: 15
slug: drawdown-circuit-breaker
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-01
---

# Phase 15 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib + testify) |
| **Config file** | none — Go modules already configured |
| **Quick run command** | `go test ./internal/pricegaptrader/... -count=1 -short` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~30s quick / ~3min full |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/pricegaptrader/... -count=1 -short`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

> Filled by planner. Every plan task that affects breaker behavior MUST appear here with an automated verify command, or be flagged Manual-Only with a Wave 0 stub.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 15-XX-XX | XX | X | PG-LIVE-02 | — | {expected behavior} | unit/integration | `go test ./internal/pricegaptrader -run {Test} -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

> Test files that must exist before Wave 1 starts. Planner will populate based on RESEARCH.md §Validation Architecture.

- [ ] `internal/pricegaptrader/breaker_test.go` — two-strike state machine, blackout detection, rolling-24h aggregator
- [ ] `internal/pricegaptrader/breaker_static_test.go` — regex check that `cfg.PriceGapPaperMode` is NOT read directly outside the Tracker accessor (regression guard)
- [ ] `internal/pricegaptrader/breaker_integration_test.go` — Redis trip log persistence + paper-mode flip survives reload
- [ ] `internal/pricegaptrader/synthetic_fire_test.go` — exercises full breaker → paper-flip → operator recovery cycle (success criterion 5)

*Wave 0 may be reduced if planner can prove existing fixtures cover a stub.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Telegram critical alert delivery | PG-LIVE-02 SC#3 | Real Telegram API call; mock at boundary in unit tests | Set bot token + chat ID, fire synthetic test, verify message arrives in Telegram |
| Dashboard recovery button UX | PG-LIVE-02 SC#4 | UI confirmation flow + i18n rendering | Trip breaker via synthetic fire, open dashboard, click "Reset Breaker", confirm sticky flag clears and engine resumes live mode |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
