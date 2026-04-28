---
phase: 11
plan: 06
subsystem: dashboard-frontend
tags: [pg-disc-03, frontend, react, recharts, i18n, discovery-ui]
requires:
  - 11-05  # /api/pg/discovery/state + /api/pg/discovery/scores/{symbol} REST endpoints
  - 11-04  # Scanner core writing pg:scan:* keys
  - 11-02  # Telemetry write path (pg_scan_* WS events)
provides:
  - DiscoverySection container component (mounted in PriceGap.tsx)
  - usePgDiscovery hook (REST seed + WS subscriptions for pg_scan_cycle/metrics/score)
  - 5 read-only Discovery cards (Banner, CycleStats, ScoreHistory, WhyRejected, Phase-12 placeholder)
  - 39 pricegap.discovery.* i18n keys (en + zh-TW lockstep)
affects:
  - web/src/pages/PriceGap.tsx (additive: <DiscoverySection /> insertion between master-toggle and candidates sections)
tech-stack:
  added: []  # zero new npm dependencies — Recharts already locked via PnLChart.tsx
  patterns:
    - "React 19 functional components with FC<Props>"
    - "Tailwind v4 utility classes (Binance dark theme tokens)"
    - "Recharts LineChart + ReferenceArea for threshold band"
    - "node:test + --experimental-strip-types for tests (CLAUDE.local.md npm lockdown)"
key-files:
  created:
    - web/src/hooks/usePgDiscovery.ts
    - web/src/components/Discovery/DiscoverySection.tsx
    - web/src/components/Discovery/DiscoveryBanner.tsx
    - web/src/components/Discovery/CycleStatsCard.tsx
    - web/src/components/Discovery/ScoreHistoryCard.tsx
    - web/src/components/Discovery/WhyRejectedCard.tsx
    - web/src/components/Discovery/PromoteTimelinePlaceholder.tsx
    - web/src/components/Discovery/__tests__/DiscoverySection.test.ts
    - web/src/i18n/__tests__/discovery_keys.test.ts
  modified:
    - web/src/i18n/en.ts (added 39 pricegap.discovery.* keys)
    - web/src/i18n/zh-TW.ts (added 39 pricegap.discovery.* keys mirroring en)
    - web/src/pages/PriceGap.tsx (added DiscoverySection import + JSX insertion)
decisions:
  - "Test framework: node:test + --experimental-strip-types instead of vitest. CLAUDE.local.md forbids npm install; existing parity.test.ts already proved this pattern works."
  - "Test file is .ts not .tsx because Node strip-types cannot evaluate JSX. The .ts test asserts module contracts (file existence, exports, regex-matched i18n keys, Recharts subcomponent imports, integration order) instead of mounting React. The visual checkpoint (Task 3) covers behavioral correctness."
  - "Tooltip.labelFormatter signature: Recharts 3.x emits ReactNode for label, not number. Wrapped with typeof guard to keep TypeScript happy without breaking runtime behavior."
  - "Auto-pick first symbol when score_snapshot has entries: avoids blank chart when operator opens dashboard mid-cycle. Mirrors useEffect-based auto-selection patterns in Analytics.tsx."
  - "errorContext.exchanges placeholder: Plan 04/05 telemetry doesn't (yet) emit a per-exchange failure list, so we pass a generic ['exchanges'] label that satisfies the {exchanges} token. Phase 12 telemetry can wire a real list without changing the i18n contract."
metrics:
  duration: "~30 minutes (frontend-only plan, no integration tests)"
  completed: 2026-04-28
  tasks_complete: 2
  tasks_blocked_on_human: 2
  test_count: 23  # 6 i18n discovery + 5 i18n parity + 12 component contract = 23
  i18n_keys_added: 39
  components_created: 6
  hooks_created: 1
---

# Phase 11 Plan 06: Discovery Dashboard UI Summary

Discovery sub-section UI for the Price-Gap dashboard tab — read-only scanner status + cycle stats + score history (with Recharts threshold-band overlay) + why-rejected breakdown — built on Plan 05 REST endpoints and WS push, in lockstep en + zh-TW i18n, with zero new npm dependencies.

## What Shipped

### Components (6 files under `web/src/components/Discovery/`)

| File | Role |
|------|------|
| `DiscoverySection.tsx` | Container — header + status pill (3 colors) + conditional banner + 4-card layout |
| `DiscoveryBanner.tsx` | Conditional banner (`disabled` / `errored` / `null` variants) with `{n}`/`{exchanges}` token interpolation |
| `CycleStatsCard.tsx` | 7-tile responsive grid (last_run, next_run_in, candidates_seen, accepted, rejected, errors, duration) using `font-mono tabular-nums` |
| `ScoreHistoryCard.tsx` | Symbol selector + Recharts `<LineChart>` with `<ReferenceArea>` threshold-band overlay (Binance-yellow `#f0b90b`); empty/loading/no-universe states |
| `WhyRejectedCard.tsx` | Reason→count table (sorted desc), i18n reason labels with raw-string fallback for unknown codes |
| `PromoteTimelinePlaceholder.tsx` | Phase-12 `border-dashed` placeholder card |

### Hook

`web/src/hooks/usePgDiscovery.ts`:
- REST seed on mount via `GET /api/pg/discovery/state`
- Lazy per-symbol fetch via `GET /api/pg/discovery/scores/{symbol}` on selector change
- WS subscription mirroring `usePriceGapWebSocket` (newline-separated JSON, switch on `msg.type`):
  - `pg_scan_cycle` → replace summary + why_rejected + score_snapshot
  - `pg_scan_metrics` → merge metrics fields only
  - `pg_scan_score` → append point to `scores[symbol]` (debounced server-side)
- Returns `{ state, scores, loadScoresFor, loadingSymbol, errored, enabled, lastRunAt, wsConnected, seedError }`

### i18n (39 keys in lockstep)

| Namespace | Count |
|-----------|-------|
| `pricegap.discovery.title|subtitle|scanner*` (header) | 5 |
| `pricegap.discovery.banner.disabled.*` (disabled banner) | 2 |
| `pricegap.discovery.banner.error.*` (error banner with `{n}`/`{exchanges}` tokens) | 2 |
| `pricegap.discovery.cycle.*` (7 stat tiles + title + "Never") | 9 |
| `pricegap.discovery.scoreHistory.*` (symbol selector + chart + 3 empty states + loading) | 7 |
| `pricegap.discovery.whyRejected.*` (title + empty + 2 col headers + 8 reason codes) | 12 |
| `pricegap.discovery.timeline.*` (Phase-12 placeholder) | 2 |
| **Total** | **39** in EN + 39 in zh-TW |

### Three Scanner Visual States (UI-SPEC §"Three scanner visual states")

| State | Trigger | Pill | Banner |
|-------|---------|------|--------|
| ON | `enabled=true`, `errors=0`, `!cycle_failed` | green `#0ecb81` | none |
| OFF | `enabled=false` | gray `gray-700` | gray disabled-banner |
| ERRORED | `enabled=true && (errors>0 \|\| cycle_failed)` | red `#f6465d` | red error-banner with token interpolation |

### Recharts Subcomponents Used

`LineChart`, `Line`, `XAxis`, `YAxis`, `Tooltip`, `ResponsiveContainer`, `CartesianGrid`, `ReferenceArea` — all already locked via `PnLChart.tsx`. Zero new imports beyond the existing surface.

## Test Counts

| Suite | Sub-tests |
|-------|-----------|
| `src/i18n/parity.test.ts` (existing) | 5 |
| `src/i18n/__tests__/discovery_keys.test.ts` (new — Task 1) | 5 |
| `src/components/Discovery/__tests__/DiscoverySection.test.ts` (new — Task 2) | 13 |
| **Total** | **23 sub-tests, 23 passing** |

Run: `cd web && node --test --experimental-strip-types src/i18n/parity.test.ts src/i18n/__tests__/discovery_keys.test.ts src/components/Discovery/__tests__/DiscoverySection.test.ts`

## Build Verification

| Check | Result |
|-------|--------|
| `cd web && npx tsc --noEmit` | exit 0 |
| `cd web && npm run build` (Vite production) | exit 0; 711 modules, 949 KB JS gzipped 257 KB |
| `go build ./cmd/main.go` | exit 0 (frontend embedded via `go:embed`) |
| `git diff --stat web/package.json web/package-lock.json` | empty (npm lockdown enforced) |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Plan assumed vitest + @testing-library/react were available**
- **Found during:** Task 1 setup
- **Issue:** `web/package.json` has zero test framework (no vitest, no jest, no @testing-library/react). Plan's acceptance criteria reference `npx vitest run`.
- **Fix:** Mirrored the existing `web/src/i18n/parity.test.ts` pattern — `node --test --experimental-strip-types` with `node:test` + `node:assert/strict`. CLAUDE.local.md npm lockdown forbids `npm install`, so this was the correct path.
- **Files modified:** test files only (no production code change)
- **Commit:** 61e09b1 (Task 1)

**2. [Rule 3 - Blocking] Node `--experimental-strip-types` cannot parse JSX (.tsx)**
- **Found during:** Task 2 test execution
- **Issue:** Initial `DiscoverySection.test.tsx` failed with `ERR_UNKNOWN_FILE_EXTENSION ".tsx"` — Node's strip-types intentionally does not handle JSX.
- **Fix:** Renamed test to `.ts` and rewrote as a static-contract suite that asserts file existence, exports (via regex on source), Recharts subcomponent usage, hook API surface, integration insertion order, and npm-lockdown enforcement. Behavior testing moves to the visual checkpoint (Task 3).
- **Files modified:** `web/src/components/Discovery/__tests__/DiscoverySection.test.ts` (new path, replaces the .tsx file)
- **Commit:** 3875a66 (Task 2)

**3. [Rule 1 - Bug] Recharts 3.x Tooltip.labelFormatter signature incompatibility**
- **Found during:** Task 2 Vite build
- **Issue:** TypeScript error: `(ts: number) => string` not assignable to `(label: ReactNode, payload: …) => ReactNode`.
- **Fix:** Wrapped `formatTime` with `typeof label === 'number'` guard so the formatter accepts any ReactNode and returns a string for numeric timestamps.
- **Files modified:** `web/src/components/Discovery/ScoreHistoryCard.tsx`
- **Commit:** 3875a66 (folded into Task 2)

### No-op deviations (out of scope per Rule 3 SCOPE BOUNDARY)

- Pre-existing `go.sum` / `go mod tidy` drift in fresh worktree — resolved with `go mod download` + `go mod tidy`, no changes committed (state was clean after sync). Not a Plan 06 regression.

## Authentication Gates

None. This plan is pure-frontend additive to an existing dashboard tab; no new auth surface introduced (existing Bearer token middleware at `/api/pg/discovery/*` flows through unchanged from Plan 05).

## Known Stubs / Deferred

- `DiscoverySection.errorContext.exchanges` is currently a placeholder `['exchanges']` because Plan 04/05 telemetry does not emit a per-exchange failure list. The `{exchanges}` token still substitutes, but the rendered text reads "across exchanges" instead of "across binance, bybit". When Plan 12 telemetry adds a per-exchange failure breakdown, wire it through without changing the i18n contract.

## Visual Checkpoint Status

### Task 3 (visual UI verification): **DEFERRED — requires human operator**

The Discovery sub-section is built and ships in the Vite bundle. Task 3's verification protocol requires:
- Starting `./arb` against the live runtime (operator only — Claude must not run live trading)
- Manipulating Redis DB 2 keys (`pg:scan:cycles`, `pg:scan:metrics`, `pg:scan:enabled`) to produce ON / ERRORED states
- Visiting `https://localhost:8443` in a browser
- Toggling locale en ↔ zh-TW
- Verifying pill colors, banner copy, threshold-band overlay rendering

This is a checkpoint:human-verify task by design (per UI-SPEC). The orchestrator should escalate to the operator. Issues found become Plan 07 fixes per the plan's own contract.

### Task 4 (live BBO join verification — Phase Success Criterion #5): **DEFERRED — requires human operator**

Requires:
- Operator setting `PriceGapDiscoveryEnabled=true`, `PriceGapDiscoveryUniverse=["BTCUSDT"]`, `PriceGapDiscoveryIntervalSec=60` in **`config.json`** (Claude must NOT modify per CLAUDE.local.md)
- Restarting `arb` under systemd
- Waiting ≥75s for first cycle
- Verifying `redis-cli -n 2 LRANGE pg:scan:cycles -1 -1` shows records covering all 30 cross-pairs of {binance, bybit, gate, bitget, okx, bingx} for BTCUSDT with `why_rejected != "symbol_not_listed_*"`
- Visiting dashboard, confirming green "Scanner ON" pill + populated CycleStats + BTCUSDT in symbol selector + Recharts chart

This is the live runtime gate for PG-DISC-01 Success Criterion #5. The orchestrator should escalate to the operator with the full step-by-step protocol from the plan's `<how-to-verify>` block.

## Phase 16 Migration Note

`<DiscoverySection />` is self-contained inside `web/src/components/Discovery/` and the hook lives in `web/src/hooks/usePgDiscovery.ts`. Phase 16 (PG-OPS-09) can lift the section to a new top-level Price-Gap tab by:
1. Moving the import in `PriceGap.tsx` to a new top-level page
2. (Optional) Splitting the page tree

No restructuring of the section internals is required.

## Self-Check: PASSED

- File `web/src/hooks/usePgDiscovery.ts`: FOUND
- File `web/src/components/Discovery/DiscoverySection.tsx`: FOUND
- File `web/src/components/Discovery/DiscoveryBanner.tsx`: FOUND
- File `web/src/components/Discovery/CycleStatsCard.tsx`: FOUND
- File `web/src/components/Discovery/ScoreHistoryCard.tsx`: FOUND
- File `web/src/components/Discovery/WhyRejectedCard.tsx`: FOUND
- File `web/src/components/Discovery/PromoteTimelinePlaceholder.tsx`: FOUND
- File `web/src/components/Discovery/__tests__/DiscoverySection.test.ts`: FOUND
- File `web/src/i18n/__tests__/discovery_keys.test.ts`: FOUND
- Modified `web/src/i18n/en.ts`: 39 pricegap.discovery.* keys
- Modified `web/src/i18n/zh-TW.ts`: 39 pricegap.discovery.* keys
- Modified `web/src/pages/PriceGap.tsx`: `<DiscoverySection />` inserted
- Commit `61e09b1` (Task 1 — i18n + lockstep test): FOUND
- Commit `3875a66` (Task 2 — hook + 6 components + integration): FOUND
- `npx tsc --noEmit`: exit 0
- `npm run build`: exit 0
- `go build ./cmd/main.go`: exit 0
- `git diff web/package.json web/package-lock.json`: empty
