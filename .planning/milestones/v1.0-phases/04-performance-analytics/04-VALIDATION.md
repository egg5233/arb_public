---
phase: 4
slug: performance-analytics
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-03
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` package |
| **Config file** | None (stdlib, no config needed) |
| **Quick run command** | `go test ./internal/analytics/... -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/analytics/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 04-01-xx | 01 | 1 | PP-04 | unit | `go test ./internal/models/... -run TestPositionSerialization -count=1` | Wave 0 | ⬜ pending |
| 04-01-xx | 01 | 1 | AN-01 | unit | `go test ./internal/analytics/... -run TestPnLDecomposition -count=1` | Wave 0 | ⬜ pending |
| 04-02-xx | 02 | 1 | AN-04 | unit | `go test ./internal/analytics/... -run TestTimeSeriesStore -count=1` | Wave 0 | ⬜ pending |
| 04-03-xx | 03 | 2 | AN-02 | unit | `go test ./internal/analytics/... -run TestStrategyComparison -count=1` | Wave 0 | ⬜ pending |
| 04-03-xx | 03 | 2 | AN-03 | unit | `go test ./internal/analytics/... -run TestExchangeMetrics -count=1` | Wave 0 | ⬜ pending |
| 04-03-xx | 03 | 2 | AN-05 | unit | `go test ./internal/analytics/... -run TestAPRCalculation -count=1` | Wave 0 | ⬜ pending |
| 04-03-xx | 03 | 2 | AN-06 | unit | `go test ./internal/analytics/... -run TestWinRateSegmentation -count=1` | Wave 0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/analytics/store_test.go` — stubs for AN-04 (SQLite time-series CRUD)
- [ ] `internal/analytics/aggregator_test.go` — stubs for AN-01, AN-02, AN-03, AN-05, AN-06
- [ ] `internal/models/position_test.go` — stubs for PP-04 (new field serialization)

*No frontend test framework exists — frontend testing is manual/visual only (existing project constraint).*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Analytics page renders PnL chart | AN-04 | React UI, no frontend test framework | Build frontend, navigate to /analytics, verify chart renders with data |
| History drill-down shows PnL breakdown | AN-01 | React UI, interactive expansion | Navigate to History, click a closed position, verify decomposition fields |
| Strategy comparison view | AN-02 | React UI, visual layout | Navigate to Analytics, verify perp-perp vs spot-futures comparison |
| Exchange metrics table | AN-03 | React UI, tabular data | Navigate to Analytics, verify per-exchange metrics display |
| Time window selector works | AN-02 | React UI, interactive | Toggle 7d/30d/90d/all presets, verify chart updates |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
