---
phase: 6
slug: spot-futures-risk-hardening
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-05
---

# Phase 6 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `github.com/alicebob/miniredis/v2` |
| **Config file** | None -- uses Go conventions (`*_test.go` co-located) |
| **Quick run command** | `go test ./internal/spotengine/ -run TestMaintenance -count=1 -v` |
| **Full suite command** | `go test ./internal/spotengine/ ./internal/risk/ ./pkg/exchange/... -count=1 -v` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/spotengine/ ./internal/risk/ -count=1 -v`
- **After every plan wave:** Run `go test ./internal/spotengine/ ./internal/risk/ ./pkg/exchange/... -count=1 -v`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 06-01-01 | 01 | 1 | SF-RISK-01 | T-06-01 | Bounds-check maintenance rate (0 < rate < 1.0) | unit | `go test ./internal/spotengine/ -run TestMaintenanceRateGate -count=1 -v` | ❌ W0 | ⬜ pending |
| 06-01-02 | 01 | 1 | SF-RISK-01 | T-06-01 | Pre-entry allows when survivable drop >= threshold | unit | `go test ./internal/spotengine/ -run TestMaintenanceRateGate -count=1 -v` | ❌ W0 | ⬜ pending |
| 06-01-03 | 01 | 1 | SF-RISK-01 | — | Manual open bypasses gate with warning | unit | `go test ./internal/spotengine/ -run TestMaintenanceManualBypass -count=1 -v` | ❌ W0 | ⬜ pending |
| 06-02-01 | 02 | 1 | SF-RISK-02 | T-06-02 | Liq distance emergency triggers parallel close | unit | `go test ./internal/spotengine/ -run TestLiqDistanceEmergency -count=1 -v` | ❌ W0 | ⬜ pending |
| 06-02-02 | 02 | 1 | SF-RISK-02 | T-06-02 | Liq distance exit triggers normal close | unit | `go test ./internal/spotengine/ -run TestLiqDistanceExit -count=1 -v` | ❌ W0 | ⬜ pending |
| 06-02-03 | 02 | 1 | SF-RISK-02 | — | Liq distance warn logs but does not exit | unit | `go test ./internal/spotengine/ -run TestLiqDistanceWarn -count=1 -v` | ❌ W0 | ⬜ pending |
| 06-03-01 | 03 | 2 | SF-RISK-03 | — | Health monitor includes spot positions in checkAll | unit | `go test ./internal/risk/ -run TestHealthMonitorSpotPositions -count=1 -v` | ❌ W0 | ⬜ pending |
| 06-04-01 | 04 | 1 | SF-RISK-04 | — | Discovery penalizes high maintenance_rate coins | unit | `go test ./internal/spotengine/ -run TestDiscoveryMaintenanceRate -count=1 -v` | ❌ W0 | ⬜ pending |
| 06-XX-01 | All | 1 | All | T-06-03 | Adapter maintenance_rate normalization per exchange | unit | `go test ./pkg/exchange/... -run TestMaintenanceRate -count=1 -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/spotengine/maintenance_rate_test.go` — stubs for SF-RISK-01 (risk gate) and SF-RISK-02 (exit trigger) tests
- [ ] `internal/risk/health_spot_test.go` — stubs for SF-RISK-03 (health monitor spot integration)
- [ ] Per-adapter maintenance rate parsing tests (add to existing adapter test files or new files)

*Existing test infrastructure covers framework needs — no new framework install required.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Dashboard shows liquidation distance column | SF-RISK-02 | Frontend display | Open dashboard, verify liq distance in spot positions table |
| Config toggle enables/disables maintenance gate | SF-RISK-01 | Config integration | Toggle in dashboard, verify gate behavior changes |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
