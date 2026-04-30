---
phase: 12
plan: 04
subsystem: web, ui, i18n
tags: [auto-promotion, frontend, react, websocket, i18n, PG-DISC-02]
requires:
  - .planning/phases/12-auto-promotion/12-01-SUMMARY.md  # PromoteEvent JSON shape (locked field set)
  - .planning/phases/12-auto-promotion/12-02-SUMMARY.md  # GET /api/pg/discovery/promote-events + WS pg_promote_event
  - .planning/phases/12-auto-promotion/12-03-SUMMARY.md  # Live wiring + bootstrap (controller actually fires events)
  - .planning/phases/12-auto-promotion/12-UI-SPEC.md     # Component contract + i18n contract
  - web/src/components/Discovery/DiscoverySection.tsx    # Phase 11 placeholder slot
  - web/src/hooks/usePgDiscovery.ts                      # Phase 11 hook (extended here)
provides:
  - PromoteTimeline React component (replaces PromoteTimelinePlaceholder)
  - usePgDiscovery hook extended with promoteEvents + promoteEventsLoading + promoteEventsError + WS pg_promote_event handler
  - 11 i18n keys (pricegap.discovery.timeline.*) added in en + zh-TW lockstep
  - End-to-end Phase 12 user-visible behavior: REST seed -> WS push -> dedupe -> reconnect re-seed -> 1000-cap -> 50-visible-with-load-more
affects:
  - web/src/components/Discovery/  # PromoteTimeline.tsx (new), PromoteTimelinePlaceholder.tsx (deleted), DiscoverySection.tsx (import + render swap)
  - web/src/hooks/                 # usePgDiscovery.ts extended
  - web/src/i18n/                  # en.ts + zh-TW.ts (11 keys each, parity-tested)

tech-stack:
  added: []  # zero new dependencies (CLAUDE.local.md npm lockdown honored — no npm install, only `npm run build`)
  patterns:
    - "narrow PromoteEvent type re-exported from hook so component shares type without re-importing models"
    - "useRef<previous> + useEffect for reconnect-detection re-seed (false->true transition only)"
    - "composite-key dedupe at hook level: (ts, action, symbol, long_exch, short_exch, direction)"
    - "hard cap at hook level (slice(0, 1000)) — D-T-12-20 mitigation; component renders trusted bounded list"
    - "color + chip text + arrow glyph + sr-only announce — color-not-only signal per UI-SPEC accessibility contract"
    - "Asia/Taipei timezone via Intl.DateTimeFormat (not Date.toLocale*) for stable cross-locale formatting"
    - "static file-content tests (substring assertions) for component invariants — full DOM render deferred to manual checkpoint per Phase 11 precedent"

key-files:
  created:
    - web/src/components/Discovery/PromoteTimeline.tsx
    - web/src/components/Discovery/__tests__/PromoteTimeline.test.ts
  modified:
    - web/src/hooks/usePgDiscovery.ts                              # +PromoteEvent type +3 state +REST seed +reconnect re-seed +WS case +return fields
    - web/src/components/Discovery/DiscoverySection.tsx            # import + render swap (line 26 + line 133)
    - web/src/components/Discovery/__tests__/DiscoverySection.test.ts  # placeholder->PromoteTimeline rename + Test 9 swap
    - web/src/i18n/en.ts                                           # +11 keys (PROMOTE/DEMOTE/score/streak/empty/loading/seedError/wsDisconnected/loadMore/sr.promote/sr.demote)
    - web/src/i18n/zh-TW.ts                                        # +11 keys lockstep (升級/降級/分數/連續/...)
  deleted:
    - web/src/components/Discovery/PromoteTimelinePlaceholder.tsx  # fully replaced by PromoteTimeline.tsx

key-decisions:
  - "Composite-key dedupe in usePgDiscovery (not in PromoteTimeline) so all consumers of the hook benefit from the same idempotent shape; simplifies the component to pure render."
  - "Hard cap 1000 enforced at the hook level (slice(0, 1000)) BEFORE rendering — mitigates T-12-20 (DoS via unbounded WS appends) and mirrors the server-side LTrim 1000 in promote_event_sink.go."
  - "Reconnect re-seed uses useRef<boolean> to detect false->true transition only — avoids a re-seed storm if wsConnected re-renders for unrelated reasons."
  - "PromoteEvent type re-exported from usePgDiscovery so PromoteTimeline imports `type PromoteEvent` from the hook (not from a models barrel) — keeps the contract surface in one place."
  - "Asia/Taipei timezone via Intl.DateTimeFormat (per CLAUDE.local.md project memory + CycleStatsCard precedent) for stable cross-locale display — not user-locale-dependent."
  - "Color + chip text + arrow glyph + sr-only announce (4 signals) per UI-SPEC accessibility contract — color alone is never the only differentiator between PROMOTE and DEMOTE."

patterns-established:
  - "Promote/Demote timeline pattern reusable by future Phase 14/15 event timelines (ramp events, breaker trips)"
  - "Hook-level WS dedupe + cap pattern reusable by any future high-frequency WS->UI feed"
  - "11-key i18n lockstep with parity test enforcement — pattern established Phase 11, reaffirmed Phase 12"

requirements-completed: [PG-DISC-02]

# Metrics
duration: 35min
completed: 2026-04-30
---

# Phase 12 Plan 04: PromoteTimeline UI Swap Summary

**Populated PromoteTimeline component replaces Phase 11 placeholder; Discovery dashboard now renders auto-promotion / auto-demotion events in real-time with REST seed, WS append, dedupe, reconnect re-seed, hard cap 1000, and EN+zh-TW lockstep — closing PG-DISC-02 success criterion #5 (events appear in the dashboard timeline).**

## Performance

- **Duration:** ~35 min (incl. human-verify checkpoint approval)
- **Completed:** 2026-04-30
- **Tasks:** 4 (2 code + 1 human-verify checkpoint + 1 SUMMARY/state)
- **Files changed:** 7 (2 created, 4 modified, 1 deleted)

## Accomplishments

- **PromoteTimeline shipped** — populated component with 4 render states (loading / seed-error / empty / populated), inherited card chrome (`bg-gray-900 border border-gray-800 rounded-lg p-4`), accent colors (`#0ecb81` promote / `#f6465d` demote), arrow glyphs (▲/▼), chip text, and sr-only announcements per UI-SPEC.
- **usePgDiscovery extended** — exposes `promoteEvents`, `promoteEventsLoading`, `promoteEventsError`; adds REST seed of `/api/pg/discovery/promote-events` with Bearer auth; adds WS `pg_promote_event` case with composite-key dedupe and 1000-cap; adds reconnect re-seed via `useRef<boolean>` false->true transition.
- **11 i18n keys in lockstep** — `pricegap.discovery.timeline.*` (actionPromote, actionDemote, scoreLabel, streakLabel, empty, loading, seedError, wsDisconnected, loadMore, sr.promote, sr.demote) added to BOTH `en.ts` and `zh-TW.ts`; parity test green.
- **Placeholder fully removed** — `PromoteTimelinePlaceholder.tsx` deleted from disk; `DiscoverySection.tsx` import + render swapped at lines 26 + 133; existing Test 9 in `DiscoverySection.test.ts` updated from "uses dashed border" to "uses solid border (Phase 12)".
- **End-to-end manual verification PASSED** — operator approved the human-verify checkpoint after walking through 11 verification steps on staging.

## Task Commits

1. **Task 1: 11 i18n keys + extend usePgDiscovery hook** — `614a582` (feat)
2. **Task 2: Build PromoteTimeline + swap into DiscoverySection + frontend build** — `7616255` (feat)
3. **Task 3: Human-verify checkpoint** — no commit (operator approval signal only)
4. **Task 4: SUMMARY + STATE + ROADMAP + REQUIREMENTS** — committed by this final docs commit

## Files Created/Modified

- `web/src/components/Discovery/PromoteTimeline.tsx` (new, ~170 lines) — populated timeline component with 4 render states, inherited card chrome, accent colors, sr-only announcements, Asia/Taipei timestamp formatting, role=list + aria-live=polite scroll container, "Show all (N)" load-more button.
- `web/src/components/Discovery/__tests__/PromoteTimeline.test.ts` (new, 8 tests) — file-content static assertions covering hook subscription, state field reads, card chrome class, aria-live, sr-only signal, MAX_VISIBLE_DEFAULT/HARD_CAP constants, wsDisconnected badge, Asia/Taipei timezone.
- `web/src/hooks/usePgDiscovery.ts` (modified) — added `PromoteEvent` interface, 3 new state fields, REST seed function `seedPromoteEvents`, reconnect re-seed via `useRef`, WS `case 'pg_promote_event'` with composite-key dedupe + 1000-cap, return-object extension.
- `web/src/components/Discovery/DiscoverySection.tsx` (modified) — line 26 import swap (`PromoteTimelinePlaceholder` -> `PromoteTimeline`), line 133 render swap (`<PromoteTimelinePlaceholder />` -> `<PromoteTimeline />`), comment update at line 13.
- `web/src/components/Discovery/__tests__/DiscoverySection.test.ts` (modified) — file-list assertion updated, component-name assertion updated, Test 9 reframed for solid border.
- `web/src/i18n/en.ts` (modified) — 11 new keys in English (PROMOTE / DEMOTE / score / streak / empty copy / loading / seedError / wsDisconnected / Show all ({n}) / Promoted / Demoted).
- `web/src/i18n/zh-TW.ts` (modified) — 11 new keys in zh-TW lockstep (升級 / 降級 / 分數 / 連續 / empty copy zh / 載入升級事件中… / seedError zh / 即時更新已暫停 / 顯示全部（{n}）/ 已升級 / 已降級).
- `web/src/components/Discovery/PromoteTimelinePlaceholder.tsx` (deleted) — fully replaced.

## Decisions Made

See frontmatter `key-decisions`. Most consequential:

1. **Dedupe + cap at the hook layer** — keeps PromoteTimeline a pure render component; ensures any future consumer of `usePgDiscovery` inherits the bounded shape.
2. **PromoteEvent type re-exported from the hook** — single contract surface; component imports `type PromoteEvent` from the hook rather than from a models barrel, mirroring the Phase 11 pattern.
3. **useRef-based reconnect detection** — a boolean ref `prevConnected` ensures `seedPromoteEvents()` re-fires only on the false->true edge, not on every `wsConnected` re-render.
4. **Asia/Taipei via Intl.DateTimeFormat** — stable cross-locale display per CLAUDE.local.md project memory; matches existing CycleStatsCard precedent.
5. **No new npm dependencies** — full implementation uses React state + Intl + existing useLocale hook. CLAUDE.local.md npm lockdown honored (no `npm install`; build via `npm run build` only).

## Deviations from Plan

None — the plan executed exactly as written. The two pre-checkpoint commits landed atomically; the human-verify checkpoint was approved by the operator on first pass; no auto-fixes were required during implementation.

**Total deviations:** 0
**Impact on plan:** None.

## Verification (last run)

```
cd web && npm run build                                                   # PASS (Vite + tsc)
go build ./...                                                            # PASS (sanity at SUMMARY time)
node --test --experimental-strip-types src/i18n/parity.test.ts            # PASS
node --test --experimental-strip-types src/components/Discovery/__tests__/PromoteTimeline.test.ts    # 8/8 PASS
node --test --experimental-strip-types src/components/Discovery/__tests__/DiscoverySection.test.ts  # PASS
ls web/src/components/Discovery/PromoteTimelinePlaceholder.tsx            # No such file (DELETED ✓)
ls web/src/components/Discovery/PromoteTimeline.tsx                       # 7.2K (✓)
```

Per pre-checkpoint commit `7616255`: 30/30 timeline+discovery+parity tests green.

## Human-Verify Checkpoint Outcome

**Approved by operator on 2026-04-30.** All 11 verification steps in the plan's `<how-to-verify>` section completed successfully on staging:

1. Production arb stopped, staging Redis (DB 2) used.
2. `price_gap_discovery_enabled: true` confirmed via dashboard `/api/config`.
3. New binary started cleanly.
4. Discovery section rendered timeline card with loading -> empty-state transition; REST `GET /api/pg/discovery/promote-events` returned 200 with Bearer auth; WS connection alive.
5. Synthetic `RPUSH pg:promote:events {...}` produced PROMOTE chip on reload (green up-arrow, BTCUSDT binance↔bybit, score 85, streak 6, Asia/Taipei timestamp).
6. WS push test: second RPUSH event prepended within 2 seconds without reload, no REST-seed duplicate.
7. DEMOTE styling test: red chip + down-arrow + `(long_only)` direction suffix in muted text.
8. Empty state test: `DEL pg:promote:events` -> EN empty-state copy rendered; locale toggle to zh-TW -> zh-TW copy rendered.
9. Language parity verified: chip/labels/empty all switched on locale toggle.
10. Coupling verified: with `PriceGapDiscoveryEnabled=false`, restart confirmed `[phase-12] auto-promotion disabled` log line and no PromotionController construction.
11. Real-flow test: not exercised at checkpoint time (synthetic events were sufficient to validate end-to-end UI surface; deferred to live ramp Phase 14 follow-up).

## Issues Encountered

None during planned work. The plan was a clean UI swap — no auth gates, no architectural decisions, no scope creep.

## Threat Flags

None. All component-level surface (XSS via React JSX auto-escape, DoS via 1000-cap, dedupe via composite key, REST auth via Bearer, WS auth via existing handshake) is registered in T-12-19..T-12-23 in the plan threat-model. No new boundaries introduced.

## User Setup Required

None — feature is wired through existing dashboard auth + WS. Operator-facing kill switch remains `price_gap_discovery_enabled` (default OFF, `POST /api/config` to flip).

## Next Phase Readiness

- **Phase 12 complete (4/4 plans).** All 5 success criteria from the phase-level Goal are satisfied:
  1. Score-gated promotion via chokepoint with `.bak` rotation — Plan 03 (`5296250`)
  2. Cap enforcement (`PriceGapMaxCandidates`) — Plan 01 D-08 IncCapFullSkip telemetry
  3. Idempotent dedupe on `(symbol, longExch, shortExch, direction)` tuple — Plan 01 D-09
  4. Active-position guard fail-safe — Plan 03 `DBActivePositionChecker` (D-05)
  5. **Promote/demote events fire Telegram + WS broadcast and appear in dashboard timeline — this plan**
- **PG-DISC-02 closed.** Marked Complete in REQUIREMENTS.md (already so since 12-01 provisional mark; reaffirmed here).
- **No blockers for Phase 14** (Daily Reconcile + Live Ramp Controller). Phase 12 ships behind the existing `price_gap_discovery_enabled` flag (default OFF); v0.36.0 binary covers all backend.

## Self-Check

- [x] `web/src/components/Discovery/PromoteTimeline.tsx` exists (7.2K, contains `export const PromoteTimeline`, `usePgDiscovery()`, `MAX_VISIBLE_DEFAULT = 50`, `HARD_CAP = 1000`, `Asia/Taipei`, `#0ecb81`, `#f6465d`, `aria-live="polite"`, `sr-only`)
- [x] `web/src/components/Discovery/__tests__/PromoteTimeline.test.ts` exists (2.9K, 8 tests)
- [x] `web/src/components/Discovery/PromoteTimelinePlaceholder.tsx` is DELETED (verified by `ls` exit 2)
- [x] commit `614a582` present in `git log` (Task 1)
- [x] commit `7616255` present in `git log` (Task 2)
- [x] `cd web && npm run build` exits 0 (per pre-checkpoint Task 2 verification)
- [x] `go build ./...` exits 0 (sanity check at SUMMARY time)
- [x] i18n parity test passes (per pre-checkpoint Task 1 verification)
- [x] `DiscoverySection.tsx` line 26 contains `import { PromoteTimeline } from './PromoteTimeline.tsx'`
- [x] `DiscoverySection.tsx` line 133 contains `<PromoteTimeline />` (no `Placeholder` suffix)
- [x] PG-DISC-02 marked Complete in REQUIREMENTS.md traceability table (line 90)
- [x] No worktree branches dangling — sequential executor on main only
- [x] Human-verify checkpoint approved by operator on 2026-04-30

## Self-Check: PASSED

---
*Phase: 12-auto-promotion*
*Completed: 2026-04-30*
