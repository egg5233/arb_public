---
phase: 16
slug: paper-mode-cleanup-dashboard-consolidation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-02
---

# Phase 16 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (backend) + vitest (frontend) |
| **Config file** | `go.mod` (root) / `web/vitest.config.ts` |
| **Quick run command** | `go test ./internal/pricegap/... ./internal/api/... -run <Test>` |
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
| 16-01-01 | 01 | 1 | PG-FIX-01 | — | N/A | unit | `go test ./internal/pricegap/ -run TestSynthFillRealizedSlippage` | ❌ W0 | ⬜ pending |
| 16-01-02 | 01 | 1 | PG-FIX-01 | — | N/A | unit | `go test ./internal/pricegap/ -run TestPaperPositionMidAtExit` | ❌ W0 | ⬜ pending |
| 16-02-01 | 02 | 1 | PG-FIX-02 | — | Mode immutable after entry; only operator-marked POSTs may flip paper_mode | unit | `go test ./internal/api/ -run TestPaperModeRequiresOperatorAction` | ❌ W0 | ⬜ pending |
| 16-02-02 | 02 | 1 | PG-FIX-02 | — | Hydration GET never POSTs paper_mode | integration | `go test ./internal/api/ -run TestConfigGetDoesNotMutate` | ❌ W0 | ⬜ pending |
| 16-03-01 | 03 | 2 | DEV-01 | — | N/A | manual+make | `make probe-bingx` | ✅ | ⬜ pending |
| 16-04-01 | 04 | 2 | PG-OPS-09 | — | N/A | unit | `cd web && npm run test:run -- PriceGapTab` | ❌ W0 | ⬜ pending |
| 16-04-02 | 04 | 2 | PG-OPS-09 | — | Typed-phrase confirm gates breaker + paper toggle | unit | `cd web && npm run test:run -- TypedPhraseConfirm` | ❌ W0 | ⬜ pending |
| 16-04-03 | 04 | 2 | PG-OPS-09 | — | i18n en/zh-TW key parity | unit | `cd web && npm run test:run -- i18nLockstep` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/pricegap/synthfill_test.go` — stubs for PG-FIX-01 (synth fill realized slippage non-zero across modeled-vs-realized delta)
- [ ] `internal/api/paper_mode_immutability_test.go` — stubs for PG-FIX-02 (operator_action marker required; hydration GET does not mutate)
- [ ] `web/src/components/PriceGapTab.test.tsx` — stubs for PG-OPS-09 (consolidated tab renders all controls)
- [ ] `web/src/i18n/lockstep.test.ts` — script asserting `keyof en === keyof zhTW`
- [ ] No new framework installs — `go test` and `vitest` already configured

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| DevTools Network capture during dashboard page load shows no offending POST | PG-FIX-02 | Browser-only behavior; HAR capture | Open dashboard with paper_mode=true, record Network tab, confirm no POST /api/config with paper_mode=false |
| `make probe-bingx` prints successful BingX probe response end-to-end | DEV-01 | Requires live BingX endpoint | Run `make probe-bingx`, expect non-error stdout response |
| Legacy controls in other tabs migrate or proxy to new Price-Gap tab | PG-OPS-09 | Cross-tab UX verification | Visually inspect Config tab and Strategy 4 tabs; confirm controls either moved or render as proxies |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
