---
phase: 16
slug: paper-mode-cleanup-dashboard-consolidation
status: approved
nyquist_compliant: true
wave_0_complete: false
created: 2026-05-02
revised: 2026-05-02
---

# Phase 16 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
>
> **Revision addendum (2026-05-02):** Path corrected from `internal/pricegap/` to `internal/pricegaptrader/` (the actual package). Test IDs aligned to plan reality (TestRealizedSlip_PaperNonZero, TestConfig_PaperMode_RequiresOperatorAction_RejectFlat/Nested, TestConfigGet_DoesNotMutate_PaperMode, ConfigCard, lockstep). Status flipped to approved + nyquist_compliant: true after checker warning #3 closure (added GET-no-mutate test to Plan 02 + path/test-ID alignment).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (backend) + vitest (frontend) |
| **Config file** | `go.mod` (root) / `web/vitest.config.ts` |
| **Quick run command** | `go test ./internal/pricegaptrader/... ./internal/api/... -run <Test>` |
| **Full suite command** | `go test ./... && (cd web && npm run test:run)` |
| **Estimated runtime** | ~90 seconds |

---

## Sampling Rate

- **After every task commit:** Run scoped quick test for the package touched
- **After every plan wave:** Run full Go suite + frontend vitest
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 16-01-01 | 01 | 1 | PG-FIX-01 | T-16-01-02, T-16-01-04 | N/A (correctness) | unit | `go test ./internal/pricegaptrader/ -run TestRealizedSlip_PaperNonZero` | ❌ W0 | ⬜ pending |
| 16-01-02 | 01 | 1 | PG-FIX-01 | T-16-01-02 | N/A (correctness) | unit | `go test ./internal/pricegaptrader/ -run TestRealizedSlip_PaperBBOStaleFallback` | ❌ W0 | ⬜ pending |
| 16-01-03 | 01 | 1 | PG-FIX-01 | T-16-01-05 | N/A (regression guard) | unit | `go test ./internal/pricegaptrader/ -run TestRealizedSlip_LiveStillWorks` | ❌ W0 | ⬜ pending |
| 16-02-01 | 02 | 1 | PG-FIX-02 | T-16-02-01, T-16-02-07 | Mode immutable after entry; only operator-marked POSTs may flip paper_mode (flat write path) | unit | `go test ./internal/api/ -run TestConfig_PaperMode_RequiresOperatorAction_RejectFlat` | ❌ W0 | ⬜ pending |
| 16-02-02 | 02 | 1 | PG-FIX-02 | T-16-02-01, T-16-02-08 | Same guard reachable from nested write path | unit | `go test ./internal/api/ -run TestConfig_PaperMode_RequiresOperatorAction_RejectNested` | ❌ W0 | ⬜ pending |
| 16-02-03 | 02 | 1 | PG-FIX-02 | T-16-02-01 | Marker-bearing POST is accepted | unit | `go test ./internal/api/ -run TestConfig_PaperMode_AcceptWithOperatorAction` | ❌ W0 | ⬜ pending |
| 16-02-04 | 02 | 1 | PG-FIX-02 | — | Guard scoped to paper_mode keys only | unit | `go test ./internal/api/ -run TestConfig_PaperMode_NonPaperWritesUnaffected` | ❌ W0 | ⬜ pending |
| 16-02-05 | 02 | 1 | PG-FIX-02 | T-16-02-10 | Hydration GET never mutates paper_mode (Pitfall-1-class) | integration | `go test ./internal/api/ -run TestConfigGet_DoesNotMutate_PaperMode` | ❌ W0 | ⬜ pending |
| 16-03-01 | 03 | 1 | DEV-01 | T-16-03-01 | TestOrder dry-run safety verified before retrofit | manual+make | `make probe-bingx` | ✅ | ⬜ pending |
| 16-04-01 | 04 | 2 | PG-OPS-09 | T-16-04-03 | togglePaper sends operator_action: true (transferred from Plan 02 per checker blocker #1) | unit+manual | `cd web && npm run test:run -- ConfigCard` | ❌ W0 | ⬜ pending |
| 16-04-02 | 04 | 2 | PG-OPS-09 | T-16-04-01, T-16-04-02 | Typed-phrase confirm gates breaker + live-capital toggle (D-21: NO operator_action marker) | unit | `cd web && npm run test:run -- ConfigCard` | ❌ W0 | ⬜ pending |
| 16-04-03 | 04 | 2 | PG-OPS-09 | T-16-04-04 | i18n en/zh-TW key parity | unit | `cd web && npm run test:run -- lockstep` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/pricegaptrader/realized_slip_test.go` — Plan 01 Task 1 stubs (TestRealizedSlip_PaperNonZero, TestRealizedSlip_PaperBBOStaleFallback, TestRealizedSlip_LiveStillWorks)
- [ ] `internal/api/paper_mode_immutability_test.go` — Plan 02 Task 2 stubs (TestConfig_PaperMode_RequiresOperatorAction_RejectFlat/Nested, TestConfig_PaperMode_AcceptWithOperatorAction, TestConfig_PaperMode_NonPaperWritesUnaffected, TestConfigGet_DoesNotMutate_PaperMode)
- [ ] `web/src/components/PriceGap/ConfigCard.test.tsx` — Plan 04 stubs (PG-OPS-09 — consolidated card renders all controls; ENABLE-LIVE-CAPITAL POST has NO operator_action per D-21)
- [ ] `web/src/i18n/lockstep.test.ts` — Plan 04 script asserting `keyof en === keyof zhTW`
- [ ] No new framework installs — `go test` and `vitest` already configured

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| DevTools Network capture during dashboard page load shows no offending POST | PG-FIX-02 | Browser-only behavior; HAR capture | Open dashboard with paper_mode=true, record Network tab, confirm no POST /api/config with paper_mode=false |
| DevTools Network capture confirms togglePaper POST body carries `operator_action: true` (server returns 200, not 409) | PG-OPS-09 (Plan 04 transferred from Plan 02 per checker blocker #1) | Browser-only behavior | Click PaperMode toggle on PriceGap tab; inspect POST /api/config body; confirm `operator_action: true` present and HTTP 200 |
| DevTools Network capture confirms ENABLE-LIVE-CAPITAL POST body does NOT carry `operator_action` (D-21) | PG-OPS-09 | Browser-only behavior | Click ENABLE-LIVE-CAPITAL toggle, complete typed-phrase modal, inspect POST body; confirm only `price_gap_live_capital: true` present, no operator_action field |
| `make probe-bingx` prints successful BingX probe response end-to-end | DEV-01 | Requires live BingX endpoint | Run `make probe-bingx`, expect non-error stdout response |
| Legacy controls in other tabs migrate or proxy to new Price-Gap tab | PG-OPS-09 | Cross-tab UX verification | Visually inspect Config tab and Strategy 4 tabs; confirm controls either moved or render as proxies |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter
- [x] Path corrected: `internal/pricegap/` → `internal/pricegaptrader/` (checker warning #3)
- [x] Test IDs aligned to plan reality (checker warning #3)
- [x] `TestConfigGet_DoesNotMutate_PaperMode` added to Plan 02 (Pitfall-1-class regression closure)

**Approval:** approved
</content>
</invoke>
