---
phase: 8
slug: price-gap-tracker-core
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-21
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

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| {populated by planner} | | | | | | | | | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/pricegaptrader/tracker_test.go` — event-detection state machine stubs (PG-01)
- [ ] `internal/pricegaptrader/risk_test.go` — pre-entry gate unit tests (PG-RISK-01..05)
- [ ] `internal/pricegaptrader/entry_test.go` — simultaneous-IOC + unwind-to-match partial-fill (PG-02)
- [ ] `internal/pricegaptrader/exit_test.go` — spread reversion + max-hold trigger (PG-03)
- [ ] `internal/database/pricegap_state_test.go` — Redis round-trip with miniredis (PG-04)
- [ ] `internal/pricegaptrader/fakes_test.go` — shared stubExchange + fake clock fixtures

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| SOON Binance↔Gate live round-trip | PG-02, PG-03 | Requires live market conditions + small real notional | Enable `PriceGapEnabled=true` with only SOON in candidate list, `PriceGapBudget=100`, watch logs for entry+exit; confirm delta-neutral and closed fully |
| pg-admin disable round-trip | PG-RISK-03 | Validates Redis flag integration with running tracker | `pg-admin disable SOON` → verify next tick log shows candidate blocked with `exec_quality` reason |
| Graceful shutdown with open position | PG-OPS-06 | Requires live process + SIGTERM | Open one position, SIGTERM, restart, verify rehydration from Redis and monitor loop resumes |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
