---
phase: 9
slug: price-gap-dashboard-paper-live-operations
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-22
---

# Phase 9 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: 09-RESEARCH.md `## Validation Architecture` section.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (stdlib) + shell-based i18n sync gate |
| **Config file** | none — existing Go module |
| **Quick run command** | `go test ./internal/pricegap/... ./internal/api/... -count=1 -run TestPhase9` |
| **Full suite command** | `go test ./... -count=1 && bash scripts/check-i18n-sync.sh` |
| **Estimated runtime** | ~30–45s (Go build cached); ~90s cold |

Frontend unit tests are OUT OF SCOPE — no Vitest/Jest configured in `web/`.
Frontend verification is via `npm run build` (type-check) + manual UAT against UI-SPEC.

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/pricegap/... ./internal/api/... -count=1 -run TestPhase9`
- **After every plan wave:** Run `go test ./... -count=1 && bash scripts/check-i18n-sync.sh && (cd web && npm run build)`
- **Before `/gsd-verify-work`:** Full suite must be green; UAT checklist signed off
- **Max feedback latency:** 45 seconds (quick) / 120 seconds (full)

---

## Per-Task Verification Map

> Populated during planning. Each task lands one row. Status transitions during execution.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD-bridge-01 | 01 (bridge) | 1 | PG-OPS-02 | — | PaperMode config field default false | unit | `go test ./internal/config/... -run TestPaperModeDefault` | ❌ W0 | ⬜ pending |
| TBD-bridge-02 | 01 (bridge) | 1 | PG-OPS-02 | — | Position.Mode stamped live/paper | unit | `go test ./internal/pricegap/... -run TestPositionMode` | ❌ W0 | ⬜ pending |
| TBD-bridge-03 | 01 (bridge) | 1 | PG-OPS-05 | — | GetPriceGapHistory reads pg:history | unit | `go test ./internal/pricegap/... -run TestGetHistory` | ❌ W0 | ⬜ pending |
| TBD-api-01 | 02 (api) | 2 | PG-OPS-01 | — | /api/config round-trip for per-candidate enable | integration | `go test ./internal/api/... -run TestConfigPriceGapCandidates` | ❌ W0 | ⬜ pending |
| TBD-api-02 | 02 (api) | 2 | PG-OPS-03 | — | WS broadcasts pg_event on entry/exit | integration | `go test ./internal/api/... -run TestWSPriceGapBroadcast` | ❌ W0 | ⬜ pending |
| TBD-api-03 | 02 (api) | 2 | PG-OPS-05 | — | /api/pricegap/history + /metrics endpoints | integration | `go test ./internal/api/... -run TestPriceGapEndpoints` | ❌ W0 | ⬜ pending |
| TBD-paper-01 | 03 (paper) | 2 | PG-OPS-02 | — | Paper mode suppresses PlaceOrder at placeLeg | unit | `go test ./internal/pricegap/... -run TestPaperModePlaceLeg` | ❌ W0 | ⬜ pending |
| TBD-paper-02 | 03 (paper) | 2 | PG-OPS-02 | — | Paper mode suppresses close IOC | unit | `go test ./internal/pricegap/... -run TestPaperModeClose` | ❌ W0 | ⬜ pending |
| TBD-paper-03 | 03 (paper) | 2 | PG-VAL-01 | — | Modeled fill synthesis produces non-zero realized edge | unit | `go test ./internal/pricegap/... -run TestModeledFill` | ❌ W0 | ⬜ pending |
| TBD-notify-01 | 04 (notify) | 2 | PG-OPS-04 | — | Telegram fires on entry/exit/blocked, respects cooldown | unit | `go test ./internal/notify/... -run TestPriceGapAlerts` | ❌ W0 | ⬜ pending |
| TBD-notify-02 | 04 (notify) | 2 | PG-OPS-04 | — | Perp-perp + spot-futures notify paths unchanged | regression | `go test ./internal/notify/... -run TestRegressionExistingAlerts` | ✅ | ⬜ pending |
| TBD-metrics-01 | 05 (metrics) | 2 | PG-OPS-05 | — | 7d/30d rolling bps/day per candidate | unit | `go test ./internal/pricegap/... -run TestRollingMetrics` | ❌ W0 | ⬜ pending |
| TBD-ui-01 | 06 (ui) | 3 | PG-OPS-01 | — | Tab renders, toggles round-trip | UAT+build | `(cd web && npm run build)` + manual UI-SPEC checklist | ✅ | ⬜ pending |
| TBD-ui-02 | 06 (ui) | 3 | PG-OPS-03 | — | Live positions panel subscribes to WS | UAT+build | `(cd web && npm run build)` + manual | ✅ | ⬜ pending |
| TBD-ui-03 | 06 (ui) | 3 | PG-OPS-05 | — | Closed log + per-candidate metrics render | UAT+build | `(cd web && npm run build)` + manual | ✅ | ⬜ pending |
| TBD-i18n-01 | all | — | PG-OPS-01 | — | en.ts + zh-TW.ts keys in sync | gate | `bash scripts/check-i18n-sync.sh` | ❌ W0 | ⬜ pending |
| TBD-val-01 | 07 (val) | 3 | PG-VAL-02 | — | Paper→live cutover has same position shape, no data loss | integration | `go test ./internal/pricegap/... -run TestPaperToLiveCutover` | ❌ W0 | ⬜ pending |

> Task IDs will be rewritten once the planner emits plan filenames. Row shape is frozen.

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/pricegap/phase9_bridge_test.go` — stubs for PaperMode, Position.Mode, GetHistory
- [ ] `internal/api/phase9_handlers_test.go` — stubs for price-gap endpoints + WS broadcast
- [ ] `internal/notify/phase9_alerts_test.go` — stubs for entry/exit/blocked alerts and regression fixtures
- [ ] `scripts/check-i18n-sync.sh` — shell script that diffs `web/src/i18n/en.ts` vs `web/src/i18n/zh-TW.ts` key sets
- [ ] No new framework installs — stdlib `go test` + bash

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

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s (full), < 45s (quick)
- [ ] `nyquist_compliant: true` set in frontmatter once planner finalizes task IDs

**Approval:** pending
