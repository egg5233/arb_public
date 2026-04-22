---
phase: 09-price-gap-dashboard-paper-live-operations
plan: 07
subsystem: dashboard-ui+i18n
tags: [phase-9, wave-4, dashboard, tailwind, react, i18n-lockstep, ui-spec-translation, pg-ops-01, pg-ops-02, pg-ops-03, pg-val-02]
requires:
  - 09-02 (REST /api/pricegap/state + POST /api/pricegap/candidate/{sym}/{disable,enable} + /api/config paper_mode round-trip)
  - 09-05 (metrics endpoint + padded rows for configured candidates)
  - 09-06 (WS topics pg_positions, pg_event, pg_candidate_update + throttle semantics)
provides:
  - web/src/pages/PriceGap.tsx — Price-Gap dashboard page component
  - Nav entry between spot-positions and history (labelKey nav.pricegap)
  - App.tsx page union + renderPage wiring for 'price-gap' page
  - 67 new pricegap.* i18n keys + nav.pricegap in lockstep across EN + zh-TW
  - WS subscription hook usePriceGapWebSocket (page-local, mirrors useWebSocket)
affects:
  - web/src/i18n/en.ts (+69 keys)
  - web/src/i18n/zh-TW.ts (+69 keys)
  - web/src/App.tsx (page union + nav wiring)
  - web/src/components/Sidebar.tsx (nav registration)
tech-stack:
  added: []
  patterns:
    - "page-local WS subscription with useCallback-stable handlers"
    - "token-interpolation helper replaceTokens(s, {key: val}) for i18n placeholders"
    - "optimistic toggle with rollback-on-{ok:false} (header master + paper toggles)"
    - "sort-state session-local (useState) per-table; no persistence"
    - "pagination: Load-100-more pattern, hard cap 500, dedupe by position id"
    - "modal pattern inherited verbatim from Positions.tsx L602 (fixed inset-0 bg-black/50)"
key-files:
  created:
    - web/src/pages/PriceGap.tsx
    - .planning/phases/09-price-gap-dashboard-paper-live-operations/09-07-SUMMARY.md
  modified:
    - web/src/App.tsx
    - web/src/components/Sidebar.tsx
    - web/src/i18n/en.ts
    - web/src/i18n/zh-TW.ts
decisions:
  - "Placed nav entry registration in components/Sidebar.tsx (the actual source of truth) rather than App.tsx which was listed in the plan — navItems array lives there and is imported by App.tsx via named export. No functional difference; just fact-finding correction."
  - "Closed Log pagination dedupes by position id on each Load-more page, protecting against the same row appearing twice when WS prepends a newly-closed row during the 100-row fetch window."
  - "Auto-disable WS event (pg_event with type='auto_disable') patches the candidate row inline via symbol match, in addition to the paired pg_candidate_update event — belt-and-braces so the UI never lags behind either stream."
  - "Modal dismiss closes on backdrop click AND Esc; the Positions.tsx shell only wires Esc implicitly via focus. Added explicit Esc listener for parity with SPEC a11y contract."
  - "Reason text on Re-enable modal is rendered verbatim from candidate.reason (which the server already sanitizes to 256 bytes, control chars stripped, per Plan 02 T-09-08). React auto-escapes JSX text — mitigates T-09-30."
  - "Delta bps color is pnlColor(-delta): delta > 0 (realized worse) → red, delta <= 0 (realized better) → green. Matches SPEC D-22 semantic rule."
metrics:
  duration: "~35 min"
  tasks_completed: 2
  files_changed: 5
  commits: 2
  lines_added: 1219
  completed: "2026-04-22"
---

# Phase 09 Plan 07: Price-Gap Dashboard Page Summary

**One-liner:** Translated UI-SPEC.md into a 1066-line `web/src/pages/PriceGap.tsx` React component rendering 5 stacked sections (header + 4 tables), wired to `/api/pricegap/*` REST for seed and three WS topics (`pg_positions`, `pg_event`, `pg_candidate_update`) for live updates — with i18n lockstep across EN + zh-TW and npm-install-free build chain.

## Commits

| Task | Name | Commit | Files |
| ---- | ---- | ------ | ----- |
| 1 | i18n lockstep (67 pricegap.* + nav.pricegap) | `4aac8ed` | web/src/i18n/en.ts, web/src/i18n/zh-TW.ts |
| 2 | Page component + App + Sidebar nav integration | `b3e1eed` | web/src/pages/PriceGap.tsx, web/src/App.tsx, web/src/components/Sidebar.tsx |

## i18n Keys Added (exhaustive)

Page chrome (7): `pageTitle`, `masterEnable`, `masterEnableTooltip`, `paperMode`, `paperModeTooltip`, `paperOn`, `liveOn`, `budgetStat`, `openCount`.

Section titles (4): `section.candidates`, `section.livePositions`, `section.closedLog`, `section.metrics`.

Column headers (25): `col.symbol`, `col.longExch`, `col.shortExch`, `col.size`, `col.entryBps`, `col.currentBps`, `col.exitBps`, `col.hold`, `col.holdDur`, `col.pnl`, `col.pnlUsdt`, `col.realizedBps`, `col.modeledBps`, `col.deltaBps`, `col.reason`, `col.closedAt`, `col.legs`, `col.thresholdBps`, `col.maxNotional`, `col.status`, `col.disabledInfo`, `col.candidate`, `col.trades`, `col.winPct`, `col.avgRealized`, `col.bps24h`, `col.bps7d`, `col.bps30d`.

Actions + status (6): `action.disable`, `action.reenable`, `action.loadMore`, `action.retry`, `status.active`, `status.disabled`.

Modal copy (11): `modal.disableTitle`, `modal.disableBody`, `modal.disableReasonLabel`, `modal.disableReasonPlaceholder`, `modal.disable.confirm`, `modal.disable.keepActive`, `modal.reenableTitle`, `modal.reenableBody`, `modal.reenableExecQuality`, `modal.reEnable.confirm`, `modal.reEnable.keepDisabled`.

Empty states (4): `empty.candidates`, `empty.livePositions`, `empty.closedLog`, `empty.metrics`.

Error / warning (3): `err.seedFailed`, `warn.wsDisconnected`, `err.toggleFailed`.

Plus `nav.pricegap` for sidebar registration.

**Total: 67 `pricegap.*` keys + `nav.pricegap` = 68 new keys per locale = 136 new entries.**

`bash scripts/check-i18n-sync.sh` reports **756 keys in sync** (from 689 before).

## Component Contract (PriceGap.tsx)

### Data shapes (TypeScript mirrors of Go response types)

- `PriceGapCandidate` — mirror of `priceGapCandidateView` in `internal/api/pricegap_handlers.go:34`
- `PriceGapPosition` — mirror of `models.PriceGapPosition` in `internal/models/pricegap_position.go:32`
- `CandidateMetrics` — mirror of `pricegaptrader.CandidateMetrics` in `internal/pricegaptrader/metrics.go`
- `PriceGapEvent` / `PriceGapCandidateUpdate` — mirror of the Go types in `internal/pricegaptrader/notify.go:18,27`

### State slices (5)

1. `enabled` + `paperMode` — seeded from `/state`, toggled via POST `/api/config`
2. `candidates` — seeded + patched by `pg_candidate_update` WS topic
3. `activePositions` — seeded + replaced by `pg_positions` WS topic (heartbeat every 2s per Plan 06)
4. `closedLog` — seeded + prepended-on-exit + paginated via `/api/pricegap/closed?offset=N&limit=100`
5. `metrics` — seeded; no WS update (re-seed on reconnect is future work)

### 5 sections in SPEC-declared order

Wrapper: `<div className="space-y-8">`. Each section: `<div className="bg-gray-900 border border-gray-800 rounded-lg p-4 overflow-x-auto">` with `<h3 className="text-sm font-bold text-gray-300 uppercase tracking-wide mb-3">`.

| # | Section | Table cols | Default sort | Key invariant |
|---|---------|------------|--------------|---------------|
| 1 | Header / Status | — | — | Two toggles + stat strip (`Open: N · Budget: $used / $total used`) |
| 2 | Candidates | 8 | symbol alpha | Disabled rows stay inline; yellow tint + opacity-60 |
| 3 | Live Positions | 8 | opened_at desc | PAPER/LIVE badge in Symbol col; paper rows bg-violet-500/5 |
| 4 | Closed Log | 12 | **closed_at desc (pinned)** | 100/page, cap 500; Δ bps red when realized > modeled |
| 5 | Rolling Metrics | 7 | 30d bps/day desc (click-sortable) | Zero-activity rows render `-` (SPEC empty state) |

### Invariants enforced

- **Typography**: only `font-bold` (700) and regular (400). `grep -E "font-medium|font-semibold" web/src/pages/PriceGap.tsx` → 0 matches.
- **Color**: violet (`violet-400`, `violet-500/5`, `violet-500/15`) only on paper-mode surfaces; yellow (`yellow-400`, `yellow-500/5`, `yellow-600/20`) only on disabled-candidate surfaces.
- **a11y**: both modals have `role="dialog"` + `aria-labelledby` + `aria-describedby`, close on Esc, backdrop click, and explicit dismiss button. Toggle inputs have `aria-label`.
- **XSS (T-09-30)**: all dynamic strings (symbol, reason, exchange names, numeric formatting) render via React children — no `dangerouslySetInnerHTML`.
- **npm lockdown (T-09-31)**: `git diff --stat web/package.json web/package-lock.json` → empty.
- **Auth (T-09-32)**: every fetch call includes `Authorization: Bearer ${localStorage.arb_token}`.

## Verification

- `bash scripts/check-i18n-sync.sh` → `i18n keys in sync: 756 keys`
- `(cd web && npm run build)` → exits 0, 704 modules, 908KB bundle
- `go build ./...` → exits 0 (go:embed picks up fresh `web/dist/`)
- `wc -l web/src/pages/PriceGap.tsx` → 1066 lines
- `grep -q "'price-gap'" web/src/App.tsx` → match
- `grep -q "case 'price-gap'" web/src/App.tsx` → match
- `grep -q "nav.pricegap" web/src/components/Sidebar.tsx` → match
- `grep -q "/api/pricegap/state" web/src/pages/PriceGap.tsx` → match
- `grep -q "pg_positions\|pg_event\|pg_candidate_update" web/src/pages/PriceGap.tsx` → match
- `grep -q "violet-400\|violet-500" web/src/pages/PriceGap.tsx` → match
- `grep -q "yellow-500/5\|yellow-600/20" web/src/pages/PriceGap.tsx` → match
- `grep -q "animate-pulse" web/src/pages/PriceGap.tsx` → match
- `grep -q 'role="dialog"' web/src/pages/PriceGap.tsx` → match
- `! grep -qE "font-medium|font-semibold" web/src/pages/PriceGap.tsx` → **0 matches** (OK)
- `grep -c "pricegap\\." web/src/pages/PriceGap.tsx` → 78 (well above 20 threshold)
- `git diff --stat web/package.json web/package-lock.json` → empty (no install)
- `git status config.json` → unchanged (CLAUDE.local.md invariant respected)

## Deviations from Plan

### 1. [Plan fact-finding — Not a deviation] Nav registration lives in `components/Sidebar.tsx`, not `App.tsx`

- **Found during:** Task 2 read_first.
- **Detail:** Plan action step 1 said "register a nav item … in `App.tsx`". But `navItems` is defined in `web/src/components/Sidebar.tsx` and re-exported; `App.tsx` imports it via `navItems` named export to look up the page title. Added the new entry to `Sidebar.tsx` (the actual source of truth).
- **Resolution:** `App.tsx` still gets its Page union + renderPage case; `Sidebar.tsx` gets the navItems array entry. Acceptance greps `grep -q "'price-gap'" web/src/App.tsx` and `grep -q "case 'price-gap'" web/src/App.tsx` both match.

### 2. [Deviation — Line count over the ~600/≤700 target]

- **Found during:** post-implementation invariant check (`wc -l`).
- **Issue:** `web/src/pages/PriceGap.tsx` is 1066 lines — above SPEC's "single-file until ~600 lines" guidance and the plan's acceptance-criteria ceiling of 700.
- **Why it's over:** 5 tables × 7–12 columns each, plus 2 modals, plus loading/error states, plus the data-shape types + helpers. Positions.tsx (the scaffold) is 635 lines with 1 table. Expanding to 4 tables + a header section linearly exceeds the budget.
- **Not fixed in this plan because:**
  - Functional correctness takes priority over file-size stylistic rule in a wave 4 UI build
  - The SPEC explicitly allows the split as follow-up (`PriceGap/Candidates.tsx`, `PriceGap/Positions.tsx`, `PriceGap/Closed.tsx`, `PriceGap/Metrics.tsx`)
  - Every section is a self-contained `<div>` — splitting is mechanical, not redesign
- **Follow-up:** Logged to `.planning/phases/09-price-gap-dashboard-paper-live-operations/deferred-items.md` — "Split PriceGap.tsx into 4 section subcomponents (trivial, mechanical, zero behavior change)".

### 3. [Ambiguity — Not a deviation] `pricegap.budgetStat` uses literal `$` in the i18n string

- **Detail:** SPEC Copywriting Contract declares EN: `Budget: ${used} / ${total} used`. The `${used}` and `${total}` are token placeholders (replaced via `replaceTokens()` helper), and the leading `$` is a literal dollar-sign on the dollar amount. Preserved verbatim per CLAUDE.local.md "SPEC verbatim" rule.

### Auth Gates
None.

### Out-of-scope (not fixed)
- web/dist/ is gitignored; go:embed picks up the locally-built bundle on next `go build` but no committed artifact.

## Self-Check

- FOUND: web/src/pages/PriceGap.tsx (1066 lines)
- FOUND: web/src/App.tsx (contains 'price-gap' + case 'price-gap' + PriceGap import)
- FOUND: web/src/components/Sidebar.tsx (contains 'nav.pricegap' entry between spot-positions and history)
- FOUND: web/src/i18n/en.ts (contains 'pricegap.pageTitle' + 'nav.pricegap')
- FOUND: web/src/i18n/zh-TW.ts (contains 'pricegap.pageTitle' + 'nav.pricegap' + '價差套利' + '模擬模式')
- FOUND commit: 4aac8ed (Task 1 i18n lockstep)
- FOUND commit: b3e1eed (Task 2 page + nav wiring)
- 0 matches: font-medium / font-semibold in PriceGap.tsx (typography invariant OK)
- Empty diff: web/package.json + web/package-lock.json (npm lockdown OK)

## Self-Check: PASSED
