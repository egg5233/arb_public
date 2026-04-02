---
phase: 2
slug: spot-futures-automation
status: draft
nyquist_compliant: false
wave_0_complete: false
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

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | SF-04 | unit | `go test ./internal/spotengine/... -run TestNativeDiscovery` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SF-05 | unit | `go test ./internal/spotengine/... -run TestAutoEntry` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SF-06 | unit | `go test ./internal/spotengine/... -run TestAutoExit` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SF-07 | unit | `go test ./internal/spotengine/... -run TestBasisSpread` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Test stubs for SF-04 native discovery scanner
- [ ] Test stubs for SF-05 auto-entry pipeline
- [ ] Test stubs for SF-06 auto-exit edge cases
- [ ] Test stubs for SF-07 basis/spread gating

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

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
