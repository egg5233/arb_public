---
phase: 05
slug: capital-allocation
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-04
---

# Phase 05 -- Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none -- uses test helpers |
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

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | Wave 0 | Status |
|---------|------|------|-------------|-----------|-------------------|--------|--------|
| 05-01-01 | 01 | 1 | CA-01, CA-02 | unit (TDD) | `go test ./internal/risk/... -run TestProfile -count=1` | TDD task creates tests in RED phase | pending |
| 05-01-02 | 01 | 1 | CA-03, CA-04 | unit (TDD) | `go test ./internal/risk/... -run TestAllocation -count=1` | TDD task creates tests in RED phase | pending |
| 05-02-01 | 02 | 2 | CA-01 | integration | `go test ./internal/engine/... ./internal/risk/... -count=1 -short` | Extends existing test infra | pending |
| 05-02-02 | 02 | 2 | CA-03, CA-04 | integration | `go build ./... && go test ./internal/engine/... ./internal/spotengine/... -count=1 -short` | Compilation + existing tests | pending |
| 05-02-03 | 02 | 2 | CA-02 | integration | `go build ./... && go test ./internal/api/... -count=1 -short` | Extends existing test infra | pending |
| 05-03-01 | 03 | 3 | CA-01, CA-02 | build | `make build-frontend` | N/A (frontend build) | pending |
| 05-03-02 | 03 | 3 | CA-01-CA-04 | build | `make build-frontend && cat VERSION` | N/A (frontend build) | pending |

*Status: pending / green / red / flaky*

---

## Wave 0 Strategy

Plan 01 tasks are `tdd="true"`. The TDD RED phase creates test files as the first step of execution -- this IS the Wave 0 activity. Tests are written before implementation code, satisfying the Nyquist requirement that automated verification exists before production code.

Plan 02 and 03 tasks extend existing test infrastructure (risk, engine, api, spotengine packages all have existing test files). No new test scaffolds needed.

**Wave 0 is satisfied by TDD execution flow (Plan 01) and existing test infrastructure (Plans 02-03).**

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Dashboard profile selector | CA-02 | Requires browser interaction | Select each profile, verify fields update, change a field and verify "custom" |
| Dynamic capital shift | CA-04 | Requires live trading conditions | Run with one strategy having no opportunities, verify capital shifts |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covered by TDD execution (Plan 01) and existing test infra (Plans 02-03)
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
