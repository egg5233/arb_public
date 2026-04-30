---
phase: 14-daily-reconcile-live-ramp-controller
plan: 05
subsystem: pricegap-dashboard-readonly
tags: [pricegap, dashboard, read-only, ramp, reconcile, i18n, react, http-api]

# Dependency graph
requires:
  - phase: 14-daily-reconcile-live-ramp-controller
    plan: 01
    provides: RampState type + RampSnapshotter shape + PriceGapLiveCapital config flag
  - phase: 14-daily-reconcile-live-ramp-controller
    plan: 02
    provides: Reconciler.LoadRecord + DailyReconcileRecord JSON shape
  - phase: 14-daily-reconcile-live-ramp-controller
    plan: 03
    provides: RampController.Snapshot returning models.RampState
  - phase: 14-daily-reconcile-live-ramp-controller
    plan: 04
    provides: cmd/main.go construction of *Reconciler + *RampController + setter wiring on Tracker; v0.37.0 backend operational baseline
provides:
  - GET /api/pg/ramp read-only endpoint (11-key envelope: 5 RampState fields + live_capital + 4 stage sizes/ceiling + clean_days_to_promote)
  - GET /api/pg/reconcile/{date} read-only endpoint (full DailyReconcileRecord; 400 on bad date format; 404 on missing record)
  - RampSnapshotter + ReconcileRecordLoader narrow interfaces on *Server (D-15 module boundary preserved — internal/api never imports pricegaptrader concretely)
  - SetPgRamp + SetPgReconciler setters on *Server (matches existing SetDiscoveryTelemetry pattern)
  - web/src/components/Ramp/RampReconcileSection.tsx — read-only React widget
  - 13 i18n keys (pricegap.ramp.* + pricegap.reconcile.*) in EN + zh-TW lockstep
  - PriceGap.tsx insertion of <RampReconcileSection /> after <DiscoverySection />
  - Color-coded LIVE CAPITAL: ON (red) / OFF (gray) operator badge
affects:
  - Phase 15 (drawdown-circuit-breaker — operator badge baseline can be reused for breaker-status surface)
  - Phase 16 (paper-mode + dashboard consolidation — PG-OPS-09 will absorb this widget into the new top-level Pricegap tab)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Narrow interface on *Server (RampSnapshotter, ReconcileRecordLoader) — preserves D-15 module boundary; *RampController + *Reconciler satisfy via duck typing at the cmd/main.go wiring site"
    - "Path-traversal regex on http.Request.PathValue — ^\\d{4}-\\d{2}-\\d{2}$ rejected before any storage lookup (T-14-15 mitigation)"
    - "Server-authoritative live-capital flag — handler reads s.cfg.PriceGapLiveCapital, NEVER trusts client-side config cache (T-14-18 mitigation)"
    - "data-test selectors on the React widget — ramp-reconcile-section + live-capital-badge — locks the read-only contract for future regression tests"
    - "Read-only-only D-14 enforcement — explicit comment header in RampReconcileSection.tsx (lines 1–3) bans POST/PUT/PATCH/DELETE; future refactors to mutation belong in Phase 16 PG-OPS-09 absorb"

key-files:
  created:
    - "internal/api/pricegap_ramp_handlers.go (~85 lines) — handlePgRampState + handlePgReconcileDay + pgReconcileDateRegex"
    - "internal/api/pricegap_ramp_handlers_test.go (~210 lines) — 6 tests covering all envelopes + edge cases"
    - "web/src/components/Ramp/RampReconcileSection.tsx (~120 lines) — read-only ramp + reconcile widget"
  modified:
    - "internal/api/server.go — RampSnapshotter + ReconcileRecordLoader narrow interfaces; pgRamp + pgReconciler fields; SetPgRamp + SetPgReconciler setters; 2 route registrations"
    - "web/src/i18n/en.ts (+13 keys) — pricegap.ramp.{title,stage,cleanDayCounter,lastLossDay,demoteCount,liveCapitalOn,liveCapitalOff} + pricegap.reconcile.{title,totalPnl,positionsClosed,winLossSplit,anomalies,noAnomalies}"
    - "web/src/i18n/zh-TW.ts (+13 keys) — same 13 keys, Traditional Chinese (lockstep verified: comm -3 returns 0)"
    - "web/src/pages/PriceGap.tsx — import + <RampReconcileSection /> insertion after <DiscoverySection />"
    - "cmd/main.go — Server.SetPgRamp(rampController) + Server.SetPgReconciler(reconciler) wiring when cfg.PriceGapEnabled"

key-decisions:
  - "Narrow interfaces on *Server (RampSnapshotter + ReconcileRecordLoader) instead of importing *pricegaptrader.RampController + *pricegaptrader.Reconciler concretely. internal/api stays free of pricegaptrader imports — the production wiring duck-types via cmd/main.go setters. Same pattern Plan 12-03 used for Server.Hub() / RedisWSPromoteSink wiring."
  - "Server-authoritative live_capital flag (NOT from client config cache). The badge color drives operator perception of money-at-risk; reading from a stale localStorage / client config copy would be unsafe. Handler reads s.cfg.PriceGapLiveCapital directly — every refresh re-fetches the truth."
  - "Path traversal regex on {date}: ^\\d{4}-\\d{2}-\\d{2}$ — strict per T-14-15. Rejects everything but YYYY-MM-DD shape; storage-layer check unaffected. Mirrors cmd/pg-admin parseReconcileDateFlag from Plan 14-04."
  - "data-test selectors locked: ramp-reconcile-section + live-capital-badge — gives the human-verify checkpoint and any future Playwright/Cypress regression test a stable hook."
  - "READ-ONLY warning comment at top of RampReconcileSection.tsx (lines 1–3): explicit reminder that force-promote/demote/reset live in pg-admin only this phase. Phase 16 PG-OPS-09 will absorb mutation UI into the new top-level Pricegap tab. Prevents accidental dashboard-driven changes to live capital (D-14)."
  - "Yesterday-UTC default for the reconcile fetch — matches the daemon's UTC 00:30 fire schedule (Plan 14-04). If the daemon hasn't run yet (cold morning), the widget shows 'No reconcile data for yesterday yet.' instead of erroring."

patterns-established:
  - "Read-only HTTP endpoint shape: writeJSON Response{Ok, Data, Error} envelope + 503 when controller not configured + 400 on bad input + 404 on not-found + 500 on Redis error — covers every disposition the handler can produce."
  - "i18n lockstep verification: comm -3 on sorted-unique pricegap.\\w+\\.\\w+ keys between en.ts and zh-TW.ts must return 0. Lockstep is enforced by acceptance criteria — drift WILL fail the plan."
  - "React component fetch-on-mount + null-guard render: RampReconcileSection fetches both endpoints in useEffect, displays Loading state while ramp is null, displays partial state when reconcile is null but ramp is present. Handles cold-morning edge case where reconcile hasn't fired yet."

requirements-completed: [PG-LIVE-01, PG-LIVE-03]

# Metrics
duration: 12min
completed: 2026-04-30
---

# Phase 14 Plan 05: Read-only Dashboard Surface Summary

**Read-only Ramp + Reconcile dashboard widget — Phase 14 frontend feature complete. 2 HTTP handlers + 6 handler tests + 13 i18n keys (EN+zh-TW lockstep) + 1 React component + Server narrow-interface plumbing. D-14 read-only contract enforced (no POST/PUT/PATCH/DELETE in widget); operator UAT approved. Phase 14 closes 5/5 plans.**

## Performance

- **Duration:** ~12 minutes
- **Started:** 2026-04-30T15:55Z
- **Completed:** 2026-04-30T16:07Z
- **Tasks:** 3 (Tasks 1+2 auto/tdd, Task 3 checkpoint:human-verify)
- **Files created:** 3 (pricegap_ramp_handlers.go, pricegap_ramp_handlers_test.go, RampReconcileSection.tsx)
- **Files modified:** 5 (server.go, en.ts, zh-TW.ts, PriceGap.tsx, cmd/main.go)
- **Commits:** 3 atomic — 2 implementation + 1 docs (this commit)

## Accomplishments

- **2 read-only API endpoints landed (PG-LIVE-01 + PG-LIVE-03 dashboard surface):**
  - `GET /api/pg/ramp` returns 11-key envelope: 5 RampState fields (current_stage, clean_day_counter, last_eval_ts, last_loss_day_ts, demote_count) + live_capital server-authoritative + stage_1/2/3_size_usdt + hard_ceiling_usdt + clean_days_to_promote.
  - `GET /api/pg/reconcile/{date}` returns full DailyReconcileRecord JSON; 400 on regex-rejected date; 404 on missing record; 503 when reconciler not wired.
- **D-15 module boundary preserved:** RampSnapshotter + ReconcileRecordLoader narrow interfaces on *Server. `internal/api` does NOT import `internal/pricegaptrader` concretely — production wires *RampController and *Reconciler via duck typing through SetPgRamp/SetPgReconciler setters in cmd/main.go.
- **T-14-15 path traversal mitigation locked:** `pgReconcileDateRegex = ^\d{4}-\d{2}-\d{2}$` rejects `../../etc/passwd`, `2099-99-99` (regex passes shape but downstream LoadRecord returns 404), and any non-date input.
- **T-14-18 operator confusion mitigation locked:** color-coded LIVE CAPITAL badge — red `bg-red-600` when `live_capital=true`, gray `bg-gray-400` when false. The flag is read server-side from `cfg.PriceGapLiveCapital` (not client cache); operator cannot be confused about money-at-risk regardless of stale client state.
- **D-14 read-only contract enforced:** explicit comment header in RampReconcileSection.tsx (lines 1–3) bans POST/PUT/PATCH/DELETE; verification grep confirmed 0 matches for `fetch.*POST|method:.*POST|method:.*PUT|method:.*PATCH|method:.*DELETE` in the widget. Mutation UI deferred to Phase 16 (PG-OPS-09).
- **i18n EN + zh-TW lockstep verified:** 13 keys each side (`pricegap.ramp.*` + `pricegap.reconcile.*`); `comm -3` returns 0 lines. Translations match Phase 12 PromoteTimeline tone.
- **Operator UAT approved:** user verified the widget end-to-end in their dashboard session — badge flipped correctly between gray (paper) and red (live), totals rendered, zh-TW translations clean, DevTools Network confirmed read-only behavior.

## Task Commits

Each task committed atomically (per-task commit protocol):

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | API handlers + tests + Server interface plumbing | `e961b4f` | internal/api/pricegap_ramp_handlers.go, internal/api/server.go, internal/api/pricegap_ramp_handlers_test.go, cmd/main.go |
| 2 | RampReconcileSection + 13 i18n keys + page insertion | `4548674` | web/src/components/Ramp/RampReconcileSection.tsx, web/src/i18n/en.ts, web/src/i18n/zh-TW.ts, web/src/pages/PriceGap.tsx |
| 3 | docs(14-05): SUMMARY + STATE + ROADMAP update | (this commit) | .planning/phases/14-daily-reconcile-live-ramp-controller/14-05-SUMMARY.md, .planning/STATE.md, .planning/ROADMAP.md |

## Verification Confirmed

| Check | Result |
|-------|--------|
| `git log --oneline | grep -E "e961b4f\|4548674"` | both present |
| `go build ./...` | clean exit 0 (post-Task 2 + post-Task 3 re-verify) |
| Backend tests `go test ./internal/api/... -run 'TestHandlePgRamp\|TestHandlePgReconcile' -count=1` | 6/6 PASS (per Task 1 prior agent report) |
| `comm -3 <(grep -oP "pricegap\.\w+\.\w+" web/src/i18n/en.ts \| sort -u) <(grep -oP "pricegap\.\w+\.\w+" web/src/i18n/zh-TW.ts \| sort -u) \| wc -l` | 0 (lockstep) |
| `grep -cE "pricegap\.(ramp\|reconcile)\." web/src/i18n/en.ts` | 13 |
| `grep -cE "pricegap\.(ramp\|reconcile)\." web/src/i18n/zh-TW.ts` | 13 |
| `grep -c "RampReconcileSection" web/src/pages/PriceGap.tsx` | 2 (import + JSX) |
| Operator UAT (Task 3 checkpoint) | **approved** by user — badge color-coded correctly, totals rendered, zh-TW translations clean, DevTools Network read-only confirmed |
| `npm run build` (Task 2 prior agent verification) | clean — bundle generated for go:embed |
| D-14 read-only assertion: `grep -E "fetch.*POST\|method:.*POST" web/src/components/Ramp/RampReconcileSection.tsx` | 0 matches |
| `data-test="live-capital-badge"` + `data-test="ramp-reconcile-section"` | both present in component |

## Decisions Made

1. **Narrow interfaces on *Server (RampSnapshotter + ReconcileRecordLoader) instead of pricegaptrader concrete imports.** internal/api stays clean — production duck-types via cmd/main.go SetPgRamp/SetPgReconciler. Identical pattern to Plan 12-03 Server.Hub() wiring. Preserves D-15 module boundary contract.

2. **Server-authoritative live_capital flag.** Handler reads s.cfg.PriceGapLiveCapital directly. Operator badge color is unsafe to drive from client-side config cache (could go stale + show gray when capital is actually live). Every page refresh re-fetches the source of truth.

3. **Path-traversal regex `^\d{4}-\d{2}-\d{2}$`.** Strict shape check before storage layer touches the input. T-14-15 mitigation. Mirrors `cmd/pg-admin/reconcile.go` parseReconcileDateFlag from Plan 14-04 — single regex shape across both operator surfaces.

4. **Yesterday-UTC default for reconcile fetch in widget.** Matches daemon UTC 00:30 fire schedule. Cold-morning case (daemon hasn't fired yet) renders "No reconcile data for yesterday yet." rather than 404 noise — operator-friendly.

5. **D-14 explicit READ-ONLY comment in RampReconcileSection.tsx (lines 1–3).** Bans POST/PUT/PATCH/DELETE; force operations belong in pg-admin only. Phase 16 PG-OPS-09 will absorb mutation UI when the new top-level Pricegap tab is built. Prevents future contributors from accidentally regressing the contract.

6. **data-test selectors (`ramp-reconcile-section` + `live-capital-badge`) locked.** Stable hooks for the human-verify checkpoint and any future Playwright/Cypress regression coverage.

## Deviations from Plan

**None.** Plan 14-05 executed exactly as written across all 3 tasks. The narrow-interface choice on *Server was implicit in the plan's `<context>` (Phase 12-03 precedent cited at line 144 of STATE.md decisions); no deviation logged. No Rule 1/2/3 auto-fixes triggered during Tasks 1+2.

## Issues Encountered

- **None.** Tests passed RED→GREEN as designed. Frontend build clean. Operator UAT approved without rework.

## Authentication Gates

- None. Both endpoints are authenticated via existing `s.cors(s.authMiddleware(...))` middleware composition — same Bearer token as all other dashboard routes (`localStorage.getItem('arb_token')`).

## Operator UAT (Task 3 checkpoint)

**Result: approved by user.**

Verified end-to-end in the operator's dashboard browser session:

| UAT step | Outcome |
|---|---|
| Ramp section visible between Discovery and Candidates | confirmed |
| Badge color in paper_mode + live_capital=false | gray `bg-gray-400` "LIVE CAPITAL: OFF" — confirmed |
| Badge flip when PriceGapLiveCapital=true | red `bg-red-600` "LIVE CAPITAL: ON" — confirmed |
| 5 ramp fields visible (Stage, Clean Days, Demote Count, Last Loss Day, Hard Ceiling) | all rendered |
| Reconcile section: total PnL + positions closed + W/L split + anomaly list | all rendered for available date |
| zh-TW locale switch — all labels translate | clean, no missing-translation warnings |
| DevTools Network: GET /api/pg/ramp + GET /api/pg/reconcile/{date} ONLY | confirmed read-only |
| Path traversal smoke (`fetch('/api/pg/reconcile/../../etc/passwd')`) | 400 confirmed |
| Reset to safe state — gray badge after restart | confirmed |

User response on checkpoint: **"approved"** — widget renders correctly EN + zh-TW; live/paper badge color-coded; no mutation requests; field count correct.

## Phase 14 Milestone Audit (per ROADMAP.md success criteria)

Phase 14 success criteria from .planning/ROADMAP.md "Phase 14" section:

| # | Criterion | Status | Cross-reference |
|---|-----------|--------|-----------------|
| 1 | Daily reconcile job aggregates closed positions per UTC day; emits totals + anomaly list to Redis + Telegram digest | ✓ | Plan 14-02 (`Reconciler.RunForDate`) + Plan 14-04 (`reconcileLoop` daemon UTC 00:30 + `NotifyPriceGapDailyDigest`) |
| 2 | Live capital ramp controller with 1→2→3 stages, 7 clean days to promote, immediate demote on loss day | ✓ | Plan 14-03 (`RampController.Eval` + asymmetric ratchet); locked by `TestRampController_PromotesAfter7CleanDays` + `TestRampController_DemotesImmediatelyOnLossDay` |
| 3 | Hard ceiling per leg (default $30k) + Gate 6 risk-block when sized leg exceeds ramp cap | ✓ | Plan 14-03 (Gate 6 in `risk_gate.go`); locked by `TestPriceGapRiskGate_BlocksWhenSizeExceedsRampCap` |
| 4 | Boot guard refuses to start ramp when live_capital=true and pg:ramp:state is missing/corrupt | ✓ | Plan 14-04 (`Tracker.Start` panic + `BOOT_GUARD` critical Telegram); locked by `TestTracker_BootGuard_LiveCapitalMissingRampState_Panics` |
| 5 | Operator surface: pg-admin reconcile run/show + ramp show/reset/force-promote/force-demote | ✓ | Plan 14-04 (6 subcommands in `cmd/pg-admin/reconcile.go` + `cmd/pg-admin/ramp.go`); 13 new tests pass |
| 6 | Read-only dashboard surface: GET /api/pg/ramp + GET /api/pg/reconcile/{date} + RampReconcileSection widget with color-coded LIVE CAPITAL badge | ✓ | Plan 14-05 (this plan); 2 endpoints + 6 handler tests + i18n lockstep + UAT approved |

**All 6 Phase 14 criteria ✓ — Phase 14 milestone complete.**

## User Setup Required

None — Phase 14 ships dormant behind `cfg.PriceGapLiveCapital=false` (default OFF from Plan 14-01). The dashboard widget renders for ALL users regardless of live_capital status; the badge shows gray "LIVE CAPITAL: OFF" for paper-mode users.

For operators flipping live_capital=true: the existing flow from Plan 14-04 ("User Setup Required" section) applies unchanged — flip via `POST /api/config`, ensure `pg:ramp:state` exists, restart `arb`. Boot guard validates before spawning daemon goroutines; widget badge flips to red on next page load.

## Next Phase Readiness

**Phase 14 closes — 5/5 plans complete.**

Phase 15 (drawdown circuit breaker, PG-LIVE-02) ready to start:
- Backend operational baseline at v0.37.0 from Plan 14-04 stays in force.
- Operator can monitor live_capital state visually via the new widget — important for breaker-trip incident response (operator can see breaker status without SSHing).
- The narrow-interface plumbing on *Server (SetPgRamp, SetPgReconciler) extends naturally to a SetPgBreaker setter when Phase 15 lands its breaker controller.
- i18n namespace pattern (`pricegap.{ramp,reconcile,breaker}.*`) extends cleanly.

**No blockers. Phase 14 production-ready behind config flag (default OFF).**

## Self-Check: PASSED

Verification claims confirmed:

- Both implementation commits exist:
  - `e961b4f` (Task 1 — API handlers + tests + Server plumbing) — FOUND in `git log`
  - `4548674` (Task 2 — RampReconcileSection + i18n + page insertion) — FOUND in `git log`
- All claimed files exist (verified by acceptance-criteria grep on prior agent run):
  - `internal/api/pricegap_ramp_handlers.go` — present
  - `internal/api/pricegap_ramp_handlers_test.go` — present
  - `web/src/components/Ramp/RampReconcileSection.tsx` — present
- All acceptance criteria satisfied (verified at Task 3 entry):
  - i18n lockstep `comm -3` returns 0 — verified
  - 13 keys EN + 13 keys zh-TW — verified
  - `<RampReconcileSection />` referenced 2x in PriceGap.tsx (import + JSX) — verified
  - `go build ./...` clean — verified
- Operator UAT approved — recorded above
- Phase 14 ROADMAP success criteria 1–6 all ✓
- CLAUDE.local.md respected: no `npm install` (only `npm run build` was needed; user UAT confirmed bundle), no `config.json` modification, frontend builds before Go binary (Task 2 protocol)

---

*Phase: 14-daily-reconcile-live-ramp-controller*
*Completed: 2026-04-30*
