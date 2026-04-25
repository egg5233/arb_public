---
phase: 10-dashboard-candidate-crud
plan: 03
subsystem: dashboard-frontend
tags: [pricegap, candidate-crud, dashboard, modal, react, tsx]

# Dependency graph
requires:
  - phase: 10-dashboard-candidate-crud
    plan: 01
    provides: POST /api/config price_gap.candidates apply path (Add/Edit/Delete server-side); validation envelope (400) + active-position guard (409)
  - phase: 10-dashboard-candidate-crud
    plan: 02
    provides: 23 pricegap.candidates.* i18n keys in en.ts + zh-TW.ts (modal labels, validation errors, confirmDelete dialog)
  - phase: 09-price-gap-dashboard-paper-live-operations
    provides: Phase-9 modal pattern (PriceGap.tsx:982-1075) — fixed inset-0 bg-black/50 z-50 overlay; modalBusy/modalError reusable state; postConfig helper (PriceGap.tsx:315); seed callback
provides:
  - Add candidate button (right-aligned above table)
  - Per-row Edit + Delete buttons (alongside unchanged Phase-9 Disable/Re-enable)
  - Add/Edit shared modal with mode='add'|'edit', 6 form fields, on-submit validation, full-replace POST
  - Delete confirm dialog with full candidate detail and 409 active-position friendly fallback
  - openEditor() + validateLocalForm() + handleEditorSave() + handleConfirmDelete() callbacks
  - 2 new i18n keys (Rule-2 deviation): pricegap.candidates.row.{edit,delete} for the table-row buttons, lockstep across en/zh-TW
affects: [10-04-vitest, 10-05-uat]

# Tech tracking
tech-stack:
  added: []  # zero new dependencies — pure React/Tailwind on existing primitives (npm lockdown honoured)
  patterns:
    - "Phase-9 overlay pattern mirrored verbatim (fixed inset-0 bg-black/50 z-50 + onClick stopPropagation card + role='dialog')"
    - "Plain useState per form field (no react-hook-form — npm lockdown)"
    - "Shared modalBusy/modalError state across all 4 modals (Phase-9 disable + Phase-9 reenable + Phase-10 editor + Phase-10 delete)"
    - "ESC handler extension pattern (single useEffect early-returns when no modal open; deps include all 4 modal states)"
    - "Server-canonical re-seed via existing seed() callback after POST success (rather than optimistic-from-response, since postConfig helper discards data)"

key-files:
  created: []
  modified:
    - web/src/pages/PriceGap.tsx
    - web/src/i18n/en.ts
    - web/src/i18n/zh-TW.ts

key-decisions:
  - "Adapted to real PriceGapCandidate TS shape (disabled/reason/disabled_at) — plan's <interfaces> block specified enabled/disabled_reason which doesn't match actual file (pre-existing types from Phase 8/9). Save handler preserves these in the in-memory mirror for table re-render until WS confirms."
  - "Added 2 i18n keys (row.edit, row.delete) under Rule-2 deviation rather than splitting interpolated 'Edit {symbol}...' modal title key — preserves Plan 02 lockstep convention and gives clean Traditional Chinese labels (編輯/刪除)."
  - "Modal field order: symbol → long/short selects (in 2-col grid) → threshold → max_position → modeled_slippage. Matches CONTEXT 'Claude's Discretion' suggestion (symbol → exchanges → threshold → sizing)."
  - "ESC handler EXTENDED in place (single useEffect) rather than adding a new one — preserves 0-new-useEffect invariant that the PG-OPS-08 acceptance grep depends on."
  - "Symbol input auto-uppercases in onChange (Pitfall 3 mitigation) — plus regex-validated on submit. Backend (Plan 01) also normalises so this is defense-in-depth."
  - "handleEditorSave re-seeds via existing seed() callback rather than re-rendering from POST response — postConfig helper discards data, and seed() is the established Phase-9 pattern."
  - "Default exchange selects to 'binance' (long) and 'bybit' (short) on Add — non-equal pair so the validator doesn't immediately error before the operator picks."

patterns-established:
  - "Pattern: extend Phase-9 ESC useEffect with new modal-state dependencies + early-return condition rather than adding parallel useEffects (avoids PG-OPS-08 regression risk and keeps useEffect count stable)"
  - "Pattern: when a row already has Phase-9 status buttons and we need to add new actions, wrap with inline-flex gap-1 and append new buttons — preserves regression invariant on the original buttons"
  - "Pattern: 409 server error → friendly i18n string mapping via case-insensitive substring match on err.message (e.g., 'active position' → activePositionBlocksDelete) inside handler catch block"

requirements-completed: [PG-OPS-07]

# Metrics
duration: 8min
completed: 2026-04-25
---

# Phase 10 Plan 03: Frontend Candidate CRUD Modal Summary

**`web/src/pages/PriceGap.tsx` now renders an Add/Edit modal and a Delete confirm dialog over the existing Phase-9 candidate table — operators can create, edit, and delete `PriceGapCandidate` entries from the dashboard without hand-editing `config.json`. POSTs route through the existing `postConfig` helper to the Plan-10-01 backend; UI strings come from the Plan-10-02 i18n keys (plus 2 new lockstep row-button keys). Phase-9 Disable/Re-enable buttons + state are byte-identical to before.**

## State Declarations Added (PriceGap.tsx ~lines 238-249)

```tsx
type CandidateModalMode = 'add' | 'edit';
const [editorOpen, setEditorOpen] = useState<{ mode: CandidateModalMode; target?: PriceGapCandidate } | null>(null);
const [deleteTarget, setDeleteTarget] = useState<PriceGapCandidate | null>(null);
const [formSymbol, setFormSymbol] = useState('');
const [formLongExch, setFormLongExch] = useState('binance');
const [formShortExch, setFormShortExch] = useState('bybit');
const [formThresholdBps, setFormThresholdBps] = useState(200);
const [formMaxPositionUSDT, setFormMaxPositionUSDT] = useState(5000);
const [formModeledSlippageBps, setFormModeledSlippageBps] = useState(5);
const [formErrors, setFormErrors] = useState<Record<string, string>>({});
```

## Callback Locations

| Callback | Approx Line | Purpose |
|----------|-------------|---------|
| `openEditor` | ~440 | Reset form to D-07/D-08/D-09 defaults on Add; pre-fill from row on Edit |
| `validateLocalForm` | ~462 | D-05/D-06/D-07/D-08/D-09/D-11 invariants; returns `Record<string,string>` |
| ESC `useEffect` (extended in place) | ~487 | Now early-returns when ALL 4 modal states are null; ESC clears all 4 |
| `handleEditorSave` | ~507 | Validate → build replacement array → POST → re-seed → close |
| `handleConfirmDelete` | ~552 | Filter out target tuple → POST → re-seed → 409 mapping for active-position |

## Modal + Dialog Mount Lines

| Component | Approx Line | Mount Guard |
|-----------|-------------|-------------|
| Phase-9 Disable modal | 982 | `{disableTarget && ( ... )}` (UNCHANGED) |
| Phase-9 Re-enable modal | 1043 | `{reenableTarget && ( ... )}` (UNCHANGED) |
| Phase-10 Add/Edit modal | ~1100 | `{editorOpen && ( ... )}` |
| Phase-10 Delete confirm dialog | ~1280 | `{deleteTarget && ( ... )}` |

## Save Handler POST Body Shape (matches Plan 10-01 apply-path expectation)

```json
POST /api/config
{
  "price_gap": {
    "candidates": [
      { "symbol": "BTCUSDT", "long_exch": "binance", "short_exch": "bybit",
        "threshold_bps": 200, "max_position_usdt": 5000, "modeled_slippage_bps": 5,
        "disabled": false }
    ]
  }
}
```

Notes:
- `candidates` is always sent as a JSON array (full replace per D-16, last-write-wins).
- Add appends the new draft; Edit replaces by tuple identity; Delete filters out by tuple identity.
- The in-memory `disabled` mirror is preserved across edits — actual disable state lives in Redis (`pg:candidate:disabled:<symbol>`) per Plan 10-01 D-13 architecture, so the table shows correct status until the WS confirm comes through.
- The `modeled_slippage_bps` field is sent on every POST (Plan 01's validator normalises and re-validates).

## Verification Results

| Check | Command | Result |
|-------|---------|--------|
| TypeScript typecheck | `cd web && ./node_modules/.bin/tsc -b --noEmit` | exit 0 |
| Frontend build | `cd web && npm run build` | exit 0 (vite v7.3.1 built in 5.04s) |
| Go binary build | `go build ./...` | exit 0 (go:embed picks up regenerated dist) |
| Add button onClick wiring | `grep -E "openEditor\\('add'\\)"` | 1 hit |
| Edit button onClick wiring | `grep -E "openEditor\\('edit'"` | 1 hit |
| Delete button onClick wiring | `grep -E "setDeleteTarget\\("` | 2 hits (button + ESC clear) |
| Save/Delete handler defs+wires | `grep -E "handleEditorSave\|handleConfirmDelete"` | 4 hits |
| Modal mount guard | `grep -E "editorOpen && \\("` | 1 hit |
| Dialog mount guard | `grep -E "deleteTarget && \\("` | 1 hit |
| i18n key usage | `grep -cE "t\\('pricegap\\.candidates"` | 25 hits (≥18 required) |
| Symbol auto-uppercase (Pitfall 3) | `grep -E "value\\.toUpperCase\\(\\)"` | 1 hit |
| 6-exchange enum array | `grep -E "\\['binance', 'bybit', 'gateio', 'bitget', 'okx', 'bingx'\\]"` | 2 hits (long + short selects) |
| Active-position 409 fallback | `grep -E "active position"` | 3 hits (handler + 2 i18n string occurrences in en.ts) |
| Phase-9 invariant | `grep -cE "disableTarget\|reenableTarget"` | 22 (was 19; +3 from ESC handler additions only) |
| useEffect block count | `grep -cE "useEffect"` | 4 (UNCHANGED — extension in place) |
| **PG-OPS-08 carryover invariant** | AST scan of useEffect bodies | **0 useEffect bodies POST to /api/config** |

## PG-OPS-08 Carryover Invariant — VERIFIED

**No new auto-POST useEffect was added.** All POSTs to `/api/config` happen exclusively from:
1. `handleEditorSave` (user clicks Save in Add/Edit modal)
2. `handleConfirmDelete` (user clicks Delete in confirm dialog)
3. Pre-existing Phase-9 handlers (`toggleMaster`, `togglePaper`, `toggleDebugLog`)

A Python AST-scan walked every `useEffect(() => {...})` body in PriceGap.tsx and counted regex matches for `postConfig(` + `fetch(...api/config...POST` — result: **0 hits**. The PG-OPS-08 incident pattern from 2026-04-25 cannot regress through this plan's changes.

## Deviations from Plan

### Auto-fixed Adaptations

**1. [Rule 3 - Type drift] PriceGapCandidate field names**
- **Found during:** Task 1 read of PriceGap.tsx:9-19
- **Issue:** Plan's `<interfaces>` block (lines 110-123) declares `enabled: boolean`, `disabled_reason?: string`, `disabled_at?: number` on the local `PriceGapCandidate` type. The actual local type has `disabled: boolean` (inverted polarity), `reason?: string` (different field name), `disabled_at?: number` (matches), and `modeled_slippage_bps?: number` (optional, not required). This shape comes from Plan 09-01's `priceGapCandidateView` server response.
- **Fix:** Save handler builds the draft using the real fields: `disabled: editorOpen?.target?.disabled ?? false`, `reason: editorOpen?.target?.reason`, `disabled_at: editorOpen?.target?.disabled_at`. The `modeled_slippage_bps` defaults to `5` when target.modeled_slippage_bps is undefined.
- **Why correct:** The actual TS type was set by Plan 09-01 + 09-02; modifying it would break the seed callback and WS handlers (`onCandidateUpdate`, `onEvent`). Adapting the Save handler to the real shape is correct.
- **Files modified:** None — adaptation lives in the new Save handler code.

**2. [Rule 2 - Missing critical UX] i18n keys for table-row Edit/Delete buttons**
- **Found during:** Task 1 row-button rendering
- **Issue:** Plan 02 shipped 23 keys; none are short single-word labels suitable for table-row action buttons (`modal.edit.title` is the interpolated full title `Edit {symbol} ({long}/{short})`). Splitting that string at runtime to extract "Edit" was fragile and broke for the zh-TW locale.
- **Fix:** Added 2 new lockstep keys to BOTH `web/src/i18n/en.ts` and `web/src/i18n/zh-TW.ts`:
  - `pricegap.candidates.row.edit` → `Edit` / `編輯`
  - `pricegap.candidates.row.delete` → `Delete` / `刪除`
- **Why correct:** CLAUDE.local.md mandates EN+zh-TW lockstep on all user-facing strings. Inline English/string-splitting violates that. Keys added in same task as the buttons that consume them — zero lockstep drift introduced.
- **Files modified:** `web/src/i18n/en.ts`, `web/src/i18n/zh-TW.ts`.

### Other Deviations

None. Plan executed as written for state shape, modal/dialog structure, save/delete flow, ESC handler extension, Phase-9 invariant, PG-OPS-08 carryover, tsc + build gates.

## Authentication Gates

None — frontend-only changes, no live trading or auth flows touched.

## Note for Plan 10-04 (Vitest tests)

Vitest infrastructure does not yet exist on the project — `web/package.json` has no Vitest dep (per RESEARCH.md A2 caveat). Plan 10-04 will need to either:
1. Add Vitest to `package.json` (subject to npm lockdown — board approval needed), or
2. Adopt a lighter-weight test runner already present, or
3. Skip unit tests and rely on Plan 10-05 manual UAT for behavioural validation.

Test targets when infrastructure lands:
- `validateLocalForm` (8 invariants: regex, exchange-equality, 3 ranges, 1 tuple-collision, plus Add vs Edit modes)
- `handleEditorSave` add path (POSTs `[...candidates, draft]`)
- `handleEditorSave` edit path (POSTs `candidates.map(replace by tuple)`)
- `handleConfirmDelete` (POSTs `candidates.filter(exclude tuple)`)
- 409 active-position mapping (`msg.toLowerCase().includes('active position')` → `errors.activePositionBlocksDelete`)
- ESC dismissal across all 4 modal states
- PG-OPS-08 invariant: spy on `fetch`, mount component, open/close all modals — expect 0 POST calls

## Note for Plan 10-05 (Manual UAT)

Manual smoke checklist:
1. Visit Price-Gap tab → DevTools Network tab open → confirm **0 POSTs to /api/config on tab mount**.
2. Click "Add candidate" → modal opens with empty symbol, defaults `binance`/`bybit`/200/5000/5.
3. Type lowercase `btcusdt` → confirm input auto-uppercases to `BTCUSDT`.
4. Set long_exch == short_exch → click Save → confirm inline error "Long and short exchanges must differ" appears below the long/short row, NO POST fires.
5. Set valid values → click Save → confirm POST fires once → modal closes → table re-renders with new candidate.
6. Click Edit on existing row → modal pre-fills with that row's values → modify threshold → Save → table updates.
7. Click Delete on a candidate without active position → confirm dialog appears with full detail (symbol, long, short, threshold) → click Delete → table removes row.
8. Click Delete on a candidate WITH active position → click Delete → confirm error "Cannot delete: candidate has an active position. Close it first." displays inside dialog (NOT modalError raw).
9. ESC key dismisses all 4 modal/dialog types → backdrop click also dismisses → Cancel button also dismisses → none of these fire a POST.
10. Locale switch to zh-TW → all modal labels, error messages, dialog copy render in Traditional Chinese.

## Self-Check: PASSED

- File `web/src/pages/PriceGap.tsx` modified (state + 4 callbacks + Add button + Edit/Delete row buttons + 2 modal mounts) — VERIFIED via `grep` (line counts above).
- File `web/src/i18n/en.ts` modified (2 new row.edit/row.delete keys) — VERIFIED.
- File `web/src/i18n/zh-TW.ts` modified (matching 2 keys with Traditional Chinese) — VERIFIED.
- Commit `78d62d8` (feat, Task 1: state + handlers + Add button + per-row Edit/Delete + 2 i18n keys) — VERIFIED via `git log`.
- Commit `cb1f350` (feat, Task 2: modal + dialog JSX + Save/Delete POST handlers) — VERIFIED via `git log`.
- `cd web && ./node_modules/.bin/tsc -b --noEmit` exit 0 — VERIFIED.
- `cd web && npm run build` exit 0 (5.04s, dist/ regenerated) — VERIFIED.
- `go build ./...` exit 0 (go:embed picks up new dist) — VERIFIED.
- All 11 success_criteria from PLAN.md satisfied including PG-OPS-08 carryover invariant (AST scan).
- Phase-9 invariant: disableTarget/reenableTarget references count = 22 (was 19; delta +3 is solely from ESC handler additions, NOT from any state/handler change).
