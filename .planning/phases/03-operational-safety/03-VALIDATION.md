---
phase: 03
slug: operational-safety
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-02
---

# Phase 03 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` package |
| **Config file** | None (standard Go test conventions) |
| **Quick run command** | `go test ./internal/notify/ ./internal/risk/ -count=1 -v` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/notify/ ./internal/risk/ -count=1 -v`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 03-01-01 | 01 | 1 | PP-01 | unit | `go test ./internal/notify/ -run TestNilSafeNotify -v` | ✅ | ⬜ pending |
| 03-01-02 | 01 | 1 | PP-01 | unit | `go test ./internal/notify/ -run TestCooldown -v` | ❌ W0 | ⬜ pending |
| 03-01-03 | 01 | 1 | PP-01 | unit | `go test ./internal/notify/ -run TestCooldownIndependent -v` | ❌ W0 | ⬜ pending |
| 03-01-04 | 01 | 1 | PP-01 | unit | `go test ./internal/engine/ -run TestAPIErrorCounter -v` | ❌ W0 | ⬜ pending |
| 03-01-05 | 01 | 1 | PP-01 | unit | `go test ./internal/engine/ -run TestAPIErrorCounterReset -v` | ❌ W0 | ⬜ pending |
| 03-02-01 | 02 | 1 | PP-03 | unit | `go test ./internal/risk/ -run TestLossLimit24h -v` | ❌ W0 | ⬜ pending |
| 03-02-02 | 02 | 1 | PP-03 | unit | `go test ./internal/risk/ -run TestLossLimit7d -v` | ❌ W0 | ⬜ pending |
| 03-02-03 | 02 | 1 | PP-03 | unit | `go test ./internal/risk/ -run TestLossLimitBlocks -v` | ❌ W0 | ⬜ pending |
| 03-02-04 | 02 | 1 | PP-03 | unit | `go test ./internal/risk/ -run TestLossLimitAllows -v` | ❌ W0 | ⬜ pending |
| 03-02-05 | 02 | 1 | PP-03 | unit | `go test ./internal/risk/ -run TestLossLimitPrune -v` | ❌ W0 | ⬜ pending |
| 03-03-01 | 03 | 2 | PP-03 | manual-only | Visual inspection of config.go | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/notify/telegram_test.go` — add cooldown tests (TestCooldown, TestCooldownIndependent)
- [ ] `internal/risk/loss_limit_test.go` — new file for loss limit unit tests (uses miniredis)
- [ ] `internal/engine/engine_test.go` — add API error counter tests (TestAPIErrorCounter, TestAPIErrorCounterReset)

*Note: Loss limit tests should use `github.com/alicebob/miniredis/v2` (already in go.mod) for in-memory Redis sorted set testing.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Config fields follow 6-touch-point pattern | PP-03 | Convention compliance requires human review of struct tags, JSON keys, env vars, defaults, Redis persistence, and dashboard UI | Verify each new config field appears in: 1) Config struct, 2) JSON tag, 3) env var, 4) default value, 5) Redis persistence, 6) dashboard toggle |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
