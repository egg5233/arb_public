---
phase: 2
slug: spot-futures-automation
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-02
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — uses alicebob/miniredis for Redis |
| **Quick run command** | `go test ./internal/spotengine/...` |
| **Full suite command** | `go test ./internal/...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/spotengine/...`
- **After every plan wave:** Run `go test ./internal/...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | Co-located | Status |
|---------|------|------|-------------|-----------|-------------------|------------|--------|
| P01-T1 | 02-01 | 1 | SF-04 | unit | `go test ./internal/spotengine/... -run "TestNativeScanner\|TestCoinGlassFallback\|TestNetYieldRanking"` | Yes | pending |
| P02-T1 | 02-02 | 2 | SF-06 | unit | `go test ./internal/spotengine/... -run "TestMinHoldGate\|TestSettlementGuard\|TestEmergencyBypass\|TestExitSpreadGate"` | Yes | pending |
| P02-T2 | 02-02 | 2 | SF-07 | unit | `go test ./internal/spotengine/... -run "TestBasisGate"` | Yes | pending |
| P03-T1 | 02-03 | 3 | SF-05 | unit | `go test ./internal/spotengine/... -run "TestAutoEntry"` | Yes | pending |
| P03-T2 | 02-03 | 3 | SF-04,SF-06,SF-07 | frontend | `cd web && npx tsc -b --noEmit` | N/A | pending |
| P03-T3 | 02-03 | 3 | SF-04,SF-05,SF-06,SF-07 | manual | Dashboard + dry-run log inspection | N/A | pending |

*Status: pending / green / red / flaky*

---

## Wave 0 Requirements

Tests are co-located with their implementation (created in the same task). No separate Wave 0 plan is needed.

- [x] Test infrastructure exists (miniredis, exchange stubs in exit_triggers_test.go, risk_gate_test.go)
- [x] Co-located test creation strategy chosen over Wave 0 separation

*Existing infrastructure (miniredis, test patterns) covers framework needs.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Dashboard shows opportunities | SF-04 | Requires browser + running server | Open dashboard, check Opportunities tab shows ranked list |
| Auto-entry triggers on live exchange | SF-05 | Requires live exchange with positions | Enable dry_run, observe logs for auto-entry decisions |
| Blackout window skip | SF-06 | Timing-dependent (Bybit :04-:05:30) | Run during blackout, verify exit skipped in logs |
| Spread rejection visible in logs | SF-07 | Requires live market data | Trigger entry with wide spread, check log shows rejection reason |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify commands
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Co-located tests cover all requirements (no MISSING references)
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** ready
