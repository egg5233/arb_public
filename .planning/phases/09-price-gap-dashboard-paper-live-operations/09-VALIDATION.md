---
phase: 9
slug: price-gap-dashboard-paper-live-operations
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-22
updated: 2026-04-22
---

# Phase 9 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: 09-RESEARCH.md `## Validation Architecture` section.

**Reconciliation note (2026-04-22):** The original draft pre-declared three test-stub files as Wave 0 artifacts (`phase9_bridge_test.go`, `phase9_handlers_test.go`, `phase9_alerts_test.go`). The actual plans are TDD (each code-producing task has `tdd="true"` and writes tests inline alongside implementation), so no separate Wave 0 stub-file step is needed or desirable — a stub step would create files that each task immediately overwrites. This file has been reconciled to the TDD reality: every test row now names the real test file the plan creates inline, and the task owner that creates it. The only true Wave 0 artifact is `scripts/check-i18n-sync.sh`, created by Plan 01 Task 2.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (stdlib) + shell-based i18n sync gate |
| **Config file** | none — existing Go module |
| **Quick run command** | `go test ./internal/pricegap/... ./internal/pricegaptrader/... ./internal/api/... -count=1` |
| **Full suite command** | `go test ./... -count=1 && bash scripts/check-i18n-sync.sh` |
| **Estimated runtime** | ~30–45s (Go build cached); ~90s cold |

Frontend unit tests are OUT OF SCOPE — no Vitest/Jest configured in `web/`.
Frontend verification is via `npm run build` (type-check) + manual UAT against UI-SPEC.

---

## Sampling Rate

- **After every task commit:** Run the quick package test for the package that task touched (e.g. `go test ./internal/pricegaptrader/... -count=1 -run <pattern>`)
- **After every plan wave:** Run `go test ./... -count=1 && bash scripts/check-i18n-sync.sh && (cd web && npm run build)`
- **Before `/gsd-verify-work`:** Full suite must be green; UAT checklist signed off
- **Max feedback latency:** 45 seconds (quick) / 120 seconds (full)

---

## Per-Task Verification Map

> Each task lands one row. Every `<automated>` command in the plan appears here. `File Exists` tracks the test file's existence; it flips to ✅ as the owning plan's task is executed.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | Test File | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-----------|-------------|--------|
| 09-01-T1 | 01 (bridge) | 1 | PG-OPS-02, PG-OPS-04 | T-09-01, T-09-02 | Position.Mode + PriceGapPaperMode default + GetPriceGapHistory reader | unit | `go test ./internal/models/... ./internal/config/... ./internal/database/... -count=1 -race -run 'TestPriceGapPosition_Mode\|TestConfig_PaperMode\|TestGetPriceGapHistory'` | internal/models/pricegap_position_test.go, internal/config/config_test.go, internal/database/pricegap_state_test.go | ⬜ (plan owns inline) | ⬜ pending |
| 09-01-T2 | 01 (bridge) | 1 | PG-OPS-01, PG-OPS-03 | T-09-03, T-09-04 | Broadcaster DI seam + NoopBroadcaster + i18n sync gate | unit+shell | `go test ./internal/pricegaptrader/... -count=1 -race -run 'TestBroadcaster\|TestNoopBroadcaster' && bash scripts/check-i18n-sync.sh` | internal/pricegaptrader/notify_test.go, scripts/check-i18n-sync.sh | ⬜ (plan owns inline) | ⬜ pending |
| 09-01-T3 | 01 (bridge) | 1 | PG-VAL-01, PG-VAL-02 | T-09-06 | D-23 slippage write-path: ModeledSlipBps + RealizedSlipBps persisted on PriceGapPosition at close | integration | `go test ./internal/pricegap/... ./internal/pricegaptrader/... -count=1 -race -run 'TestSlippageWritePath'` | internal/pricegap/slippage_write_path_test.go | ⬜ (plan owns inline) | ⬜ pending |
| 09-02-T1 | 02 (api) | 2 | PG-OPS-01 | — | /api/config round-trip for per-candidate enable | integration | `go test ./internal/api/... -count=1 -race -run 'TestConfigPriceGapCandidates'` | internal/api/pricegap_handlers_test.go | ⬜ (plan owns inline) | ⬜ pending |
| 09-02-T2 | 02 (api) | 2 | PG-OPS-03 | — | WS broadcasts pg_event on entry/exit | integration | `go test ./internal/api/... -count=1 -race -run 'TestWSPriceGapBroadcast'` | internal/api/pricegap_broadcast_test.go | ⬜ (plan owns inline) | ⬜ pending |
| 09-02-T3 | 02 (api) | 2 | PG-OPS-05 | — | /api/pricegap/history + /metrics endpoints | integration | `go test ./internal/api/... -count=1 -race -run 'TestPriceGapEndpoints'` | internal/api/pricegap_handlers_test.go | ⬜ (plan owns inline) | ⬜ pending |
| 09-03-T1 | 03 (paper) | 2 | PG-OPS-02 | — | Paper mode suppresses PlaceOrder at placeLeg | unit | `go test ./internal/pricegaptrader/... -count=1 -race -run 'TestPaperModePlaceLeg'` | internal/pricegaptrader/paper_mode_test.go | ⬜ (plan owns inline) | ⬜ pending |
| 09-03-T2 | 03 (paper) | 2 | PG-OPS-02 | — | Paper mode suppresses close IOC | unit | `go test ./internal/pricegaptrader/... -count=1 -race -run 'TestPaperModeClose'` | internal/pricegaptrader/paper_mode_test.go | ⬜ (plan owns inline) | ⬜ pending |
| 09-03-T3 | 03 (paper) | 2 | PG-VAL-01 | — | Modeled fill synthesis produces non-zero realized edge (Pitfall 7) | unit | `go test ./internal/pricegaptrader/... -count=1 -race -run 'TestModeledFill'` | internal/pricegaptrader/paper_mode_test.go | ⬜ (plan owns inline) | ⬜ pending |
| 09-04-T1 | 04 (notify) | 2 | PG-OPS-04 | — | Telegram fires on entry/exit/blocked, respects cooldown | unit | `go test ./internal/notify/... -count=1 -race -run 'TestPriceGapAlerts'` | internal/notify/pricegap_alerts_test.go | ⬜ (plan owns inline) | ⬜ pending |
| 09-04-T2 | 04 (notify) | 2 | PG-OPS-04 | — | Perp-perp + spot-futures notify paths unchanged (golden-string regression) | regression | `go test ./internal/notify/... -count=1 -race -run 'TestRegressionExistingAlerts'` | internal/notify/regression_test.go | ⬜ (plan owns inline) | ⬜ pending |
| 09-05-T1 | 05 (metrics) | 2 | PG-OPS-05 | — | 7d/30d rolling bps/day per candidate, pure function | unit | `go test ./internal/pricegaptrader/... -count=1 -race -run 'TestComputeCandidateMetrics'` | internal/pricegaptrader/metrics_test.go | ⬜ (plan owns inline) | ⬜ pending |
| 09-05-T2 | 05 (metrics) | 2 | PG-OPS-05 | — | /api/pricegap/metrics handler + state integration | integration | `go test ./internal/api/... -count=1 -race -run 'TestHandlePriceGapMetrics\|TestHandlePriceGapState_IncludesMetrics'` | internal/api/pricegap_handlers_test.go | ⬜ (plan owns inline) | ⬜ pending |
| 09-06-T1 | 06 (hooks) | 3 | PG-OPS-02, PG-OPS-03 | T-09-25 | Hook invocations at 5 sites + rehydrate silence (Go spy test, not bash) | integration | `go test ./internal/pricegaptrader/... -count=1 -race -run 'TestHook_\|TestRehydratePathSilent'` | internal/pricegaptrader/tracker_broadcast_test.go | ⬜ (plan owns inline) | ⬜ pending |
| 09-06-T2 | 06 (hooks) | 3 | PG-OPS-03 | T-09-26, T-09-27 | Heartbeat throttling + cmd/main.go DI compile assertions | integration | `go test ./internal/pricegaptrader/... ./internal/api/... ./internal/notify/... -count=1 -race -run 'TestHeartbeat\|TestDI_CompileAssertion' && go build ./...` | internal/pricegaptrader/tracker_broadcast_test.go | ⬜ (plan owns inline) | ⬜ pending |
| 09-07-T1 | 07 (ui-i18n) | 4 | PG-OPS-01 | T-09-34 | en.ts + zh-TW.ts keys in sync (adds pricegap.*) | gate | `bash scripts/check-i18n-sync.sh && (cd web && npm run build)` | web/src/i18n/en.ts, web/src/i18n/zh-TW.ts | ⬜ (plan owns inline) | ⬜ pending |
| 09-07-T2 | 07 (ui) | 4 | PG-OPS-01, PG-OPS-02, PG-OPS-03, PG-VAL-02 | T-09-30, T-09-31, T-09-32 | UI page renders, nav wired, typography + color invariants | UAT+build | `bash scripts/check-i18n-sync.sh && (cd web && npm run build) && go build ./...` + manual UI-SPEC checklist | web/src/pages/PriceGap.tsx, web/src/App.tsx | ⬜ (plan owns inline) | ⬜ pending |
| 09-08-T1 | 08 (validation) | 5 | PG-VAL-02 | T-09-35, T-09-36 | Paper→live cutover + regression sweep | integration | `go test ./... -count=1 -race -short && bash scripts/check-i18n-sync.sh && (cd web && npm run build)` | internal/pricegaptrader/paper_to_live_test.go, internal/pricegaptrader/e2e_regression_test.go | ⬜ (plan owns inline) | ⬜ pending |
| 09-08-T2 | 08 (validation) | 5 | all | — | UAT checklist + VERSION + CHANGELOG bump | doc | file-existence + version-grep (see plan) | .planning/phases/09-price-gap-dashboard-paper-live-operations/09-UAT.md, CHANGELOG.md, VERSION | ⬜ (plan owns inline) | ⬜ pending |
| 09-08-T3 | 08 (validation) | 5 | all | T-09-37 | Human UAT walkthrough | manual checkpoint | UAT checklist sign-off | .planning/phases/09-price-gap-dashboard-paper-live-operations/09-UAT.md | ⬜ (plan owns) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**`File Exists` convention:** ⬜ (plan owns inline) means the test file does not exist yet but is created by the owning plan's task. The column flips to ✅ the moment `/gsd-execute-phase` commits the task, and to ❌ if the test is missing after commit (validator failure).

---

## Wave 0 Requirements

The TDD plan shape means most tests are authored inline with their implementation — no pre-planned stub-file sweep is required. The only Wave 0 artifact is the i18n gate script, which every Wave 2+ plan's verify command invokes.

- [ ] `scripts/check-i18n-sync.sh` — shell script that diffs `web/src/i18n/en.ts` vs `web/src/i18n/zh-TW.ts` key sets. **Owner:** Plan 01 Task 2.
- [ ] No new framework installs — stdlib `go test` + bash.

The 3 stub-file rows previously listed (`phase9_bridge_test.go`, `phase9_handlers_test.go`, `phase9_alerts_test.go`) are removed: they were a planning artifact from a non-TDD draft and are not needed now that each task creates its test file inline.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Price-Gap tab renders in dashboard per UI-SPEC | PG-OPS-01 | No frontend unit test runner | Load `/`, navigate to Price-Gap tab, compare to 09-UI-SPEC.md mockups section-by-section |
| WebSocket `pg_event` messages arrive in browser | PG-OPS-03 | Requires live WS connection + browser devtools | Open devtools Network → WS frame log, trigger paper entry, observe `pg_event` |
| Telegram messages are human-readable and match copy spec | PG-OPS-04 | Requires live Telegram channel + copy review | Enable Telegram, trigger paper entry/exit/block, screenshot delivered messages |
| zh-TW locale strings render correctly with no English fallbacks | PG-OPS-01 | Requires visual inspection | Switch locale to zh-TW, walk every Price-Gap surface, confirm no English bleed |
| Paper→live cutover preserves user intent | PG-VAL-02 | Requires end-to-end paper run + manual flip | Run paper mode for 1h, flip to live, verify no orphan positions and Closed Log continuity |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or declared manual-only
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (only `scripts/check-i18n-sync.sh`)
- [x] No watch-mode flags
- [x] Feedback latency < 120s (full), < 45s (quick)
- [x] `nyquist_compliant: true` set in frontmatter — TDD plan shape means every plan task that produces code also produces its own tests in the same commit

**Approval:** reconciled to TDD reality — ready for execution.
