---
phase: 1
slug: spot-futures-exchange-expansion
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-01
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `miniredis` for integration |
| **Config file** | None (standard `go test`) |
| **Quick run command** | `go test ./internal/spotengine/ -run TestClose -count=1 -v` |
| **Full suite command** | `go test ./internal/spotengine/ -count=1 -v && go build ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/spotengine/ -count=1 -v` + `go build ./...`
- **After every plan wave:** Full livetest on changed exchange + `go test ./... -count=1`
- **Before `/gsd:verify-work`:** All 5 exchanges pass livetest + successful manual Dir A and Dir B trades
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 1 | SF-01 | integration (live) | `go run ./cmd/livetest/ --exchange bybit --test-margin` | ✅ (tests 23-26) | ⬜ pending |
| 01-02-01 | 02 | 2 | SF-01, SF-02 | integration (live) | `go run ./cmd/livetest/ --exchange binance --test-margin` | ✅ (tests 23-26) | ⬜ pending |
| 01-02-02 | 02 | 2 | SF-03 | integration (live) | Manual Dir A/B trades on Binance | ❌ manual | ⬜ pending |
| 01-03-01 | 03 | 3 | SF-01, SF-02 | integration (live) | `go run ./cmd/livetest/ --exchange bitget --test-margin` | ✅ (tests 23-26) | ⬜ pending |
| 01-04-01 | 04 | 4 | SF-01, SF-02 | integration (live) | `go run ./cmd/livetest/ --exchange gateio --test-margin` | ✅ (tests 23-26) | ⬜ pending |
| 01-05-01 | 05 | 5 | SF-01, SF-02, SF-03 | integration (live) | `go run ./cmd/livetest/ --exchange okx --test-margin` | ✅ (tests 23-26) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Add `PlaceSpotMarginOrder` livetest (Dir A sell with auto-borrow, Dir B buy) — currently no test for this critical path
- [ ] Add `GetSpotMarginOrder` livetest (fill reconciliation) — not tested
- [ ] No automated Dir A/Dir B lifecycle test — manual trades are the verification per user decision D-04

*Existing livetest infrastructure (tests 23-26) covers: GetMarginBalance, MarginBorrow, MarginRepay, TransferToMargin/TransferFromMargin.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Dir A full lifecycle (borrow-sell-long) | SF-01 | Requires live exchange funds and real positions | Manual trigger via dashboard, verify position opened, auto-borrow occurred, clean exit with auto-repay |
| Dir B full lifecycle (buy-spot-short) | SF-02 | Requires live exchange funds and real positions | Manual trigger via dashboard, verify spot bought, futures short opened, clean exit |
| Auto-borrow/repay correctness | SF-03 | Exchange-side verification needed | Check exchange margin account after open (borrow present), after close (borrow repaid, no residual) |
| Emergency close under margin stress | SF-03 | Cannot safely automate margin stress | Manually reduce margin, trigger close, verify repay completes |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
