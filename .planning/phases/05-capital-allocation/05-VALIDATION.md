---
phase: 05
slug: capital-allocation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-04
---

# Phase 05 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none — uses test helpers |
| **Quick run command** | `go test ./internal/risk/... ./internal/config/... -count=1 -short` |
| **Full suite command** | `go test ./... -count=1 -short` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/risk/... ./internal/config/... -count=1 -short`
- **After every plan wave:** Run `go test ./... -count=1 -short`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 05-01-01 | 01 | 1 | CA-01, CA-02 | unit | `go test ./internal/risk/... -run TestProfile -count=1` | ❌ W0 | ⬜ pending |
| 05-01-02 | 01 | 1 | CA-03 | unit | `go test ./internal/risk/... -run TestAllocation -count=1` | ❌ W0 | ⬜ pending |
| 05-02-01 | 02 | 2 | CA-01 | integration | `go test ./internal/risk/... ./internal/engine/... -count=1 -short` | ❌ W0 | ⬜ pending |
| 05-03-01 | 03 | 3 | CA-04 | unit | `go test ./internal/risk/... -run TestDynamic -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/risk/allocator_test.go` — extend with profile and dynamic allocation tests
- [ ] `internal/config/config_test.go` — profile config round-trip tests

*Existing infrastructure covers test framework — no new framework install needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Dashboard profile selector | CA-02 | Requires browser interaction | Select each profile, verify fields update, change a field and verify "custom" |
| Dynamic capital shift | CA-04 | Requires live trading conditions | Run with one strategy having no opportunities, verify capital shifts |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
