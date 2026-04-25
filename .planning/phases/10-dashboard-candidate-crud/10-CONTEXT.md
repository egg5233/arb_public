# Phase 10: Dashboard Candidate CRUD - Context

**Gathered:** 2026-04-25 (auto mode)
**Status:** Ready for planning

<domain>
## Phase Boundary

Operator can Add, Edit, and Delete `PriceGapCandidate` entries from the Price-Gap dashboard tab without hand-editing `config.json`. Changes round-trip through the existing `POST /api/config` path, persist to `config.json` via `SaveJSON` (with `.bak` backup), and surface in the running tracker on the next scan tick. Existing per-candidate Disable/Re-enable buttons (shipped in Phase 9) are unchanged.

**Locked milestone constraints (carried from v2.1):**
- Paper-only — `PriceGapPaperMode=true` enforced; this phase introduces no new `PlaceOrder` paths.
- 6 existing exchanges only (Binance, Bybit, Gate.io, Bitget, OKX, BingX) for the dropdown options.
- Score-threshold auto-promotion (no operator review gate) — Phase 12 territory; not in scope here.

**Out of scope for Phase 10 (deferred):**
- Auto-discovery scanner output (Phase 11) and auto-promotion (Phase 12).
- Live-mode flip and any real `PlaceOrder` calls (v2.2).
- pg-admin CLI parity for add/edit/delete (Phase 9 CLI surface stays as-is: status/enable/disable/positions list).
- Multi-operator concurrent-edit reconciliation (v3.0 territory).

</domain>

<decisions>
## Implementation Decisions

### Modal Behavior & Layout
- **D-01:** Add and Edit share a single modal component with a `mode: "add" | "edit"` prop. Delete uses a lightweight confirmation dialog (no full modal). Matches existing per-tab patterns in `web/src/pages/`.
- **D-02:** Modal mounts on top of `PriceGap.tsx` candidate table; uses portal-style overlay (no nav obstruction). Backdrop click + ESC + Cancel button all dismiss without saving.
- **D-03:** Validation is on-submit with inline field-level error messages. Save button is enabled by default and surfaces errors after first submit attempt — avoids the UX trap of "Save is greyed out and I don't know why".
- **D-04:** "Add candidate" button rendered above the candidate table, right-aligned, matching the existing tab header pattern in `PriceGap.tsx`.

### Field Validation Rules
- **D-05:** `symbol` — uppercase plain format `^[A-Z0-9]+USDT$` (matches internal symbol convention; per-exchange mapping happens inside adapters per `CLAUDE.local.md` exchange-adapter rules). Auto-uppercase on input. Required.
- **D-06:** `long_exch` and `short_exch` — `<select>` dropdowns of exactly 6 values: `binance`, `bybit`, `gateio`, `bitget`, `okx`, `bingx`. Both required. **Must differ from each other** (validation error if equal — strategy is delta-neutral cross-exchange by definition).
- **D-07:** `threshold_bps` — positive integer, range 50–1000, default 200 (matches Phase 8 D-08 `SOON` example). Required.
- **D-08:** `max_position_usdt` — positive number, range 100–50000, default 5000 (matches `PriceGapBudget` MVP default). Step 100. Required.
- **D-09:** `modeled_slippage_bps` — non-negative number, range 0–100, default 5. Step 1. Required.
- **D-10:** Server-side re-validation in `internal/api/config_handlers*.go` with the same rules (defense in depth — never trust the dashboard alone). Returns standard `{ok: false, error: "..."}` envelope on failure.

### Identity & Edit Semantics
- **D-11:** Natural key for a candidate is the tuple `(symbol, long_exch, short_exch)`. Two candidates with the same tuple are forbidden. Add with a colliding tuple returns a validation error.
- **D-12:** Edit identifies the target row by its existing tuple (not array index). Tuple-changing edits are allowed but must not collide with another existing candidate.
- **D-13:** Edit preserves the per-candidate `enabled` flag introduced in Phase 9 — Add/Edit modal does not expose this field. Disable/Re-enable continues through the existing buttons (PG-OPS-07 explicit constraint).

### Delete Safety
- **D-14:** Block delete when the candidate has an active position open (tuple match against `pg:positions:active` Redis set). Server returns `{ok: false, error: "candidate has active position; close it first"}`. Frontend surfaces this with a clear message and links to the position row.
- **D-15:** When safe to delete, frontend shows a single-step confirm dialog displaying the full candidate detail (symbol, exchanges, threshold) before posting. No two-step "type the symbol to confirm" pattern — the candidate set is small and operator-editable.

### Backend API Shape
- **D-16:** Reuse the existing `POST /api/config` endpoint with a `pricegap.candidates` payload (full array replace, last-write-wins). No new route. Matches PG-OPS-07 explicit ask and the established config-update pattern (see `internal/api/config_handlers_test.go` for shape reference).
- **D-17:** Server applies the candidate-list change to `*config.Config` in-memory **and** writes `config.json` with `.bak` backup, identically to the existing `SpotFutures*` and `Risk*` config field paths. No new persistence code; reuse `SaveJSON`.
- **D-18:** `GET /api/config` returns the current `PriceGapCandidates` list (already does, via Phase 8 D-22/D-23). Frontend seeds from this on tab mount; no separate fetch endpoint.

### Tracker Hot-Reload
- **D-19:** `internal/pricegaptrader/tracker` reads `cfg.PriceGapCandidates` at the start of each scan tick (existing pattern from Phase 8). No process restart required after Add/Edit/Delete. Plan must verify this read path is not cached at startup; if it is, the planner adds a refresh hook.

### Real-Time Update Surface
- **D-20:** No new WebSocket broadcast for v2.1. The POST response contains the updated candidate list and the frontend re-renders from that response. Multi-operator sync is explicitly v3.0 territory (matches Phase 9 D-09 minimal-scope rationale).

### i18n Lockstep
- **D-21:** Every new translation key added to both `web/src/i18n/en.ts` and `web/src/i18n/zh-TW.ts` in the same commit. Key namespace `priceGap.candidates.*` for modal labels, validation errors, and confirmation copy. CI / human review verifies parity (matches existing convention from `CLAUDE.local.md` i18n rules).

### Claude's Discretion
- Exact CSS / Tailwind class names for the modal layout (match `Positions.tsx` table aesthetic).
- Form field order within the modal (suggest grouping: symbol → exchanges → threshold → sizing).
- Optimistic-UI pattern for the candidate table (re-render from POST response is fine; no need for optimistic insert before server ack).
- Whether to show a "candidate id" or just the tuple in the table — leave to planner; tuple display is acceptable.
- Whether to add Storybook stories or component-level tests for the modal — leave to planner per project test-coverage norms.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### v2.1 milestone scope
- `.planning/REQUIREMENTS.md` §"Dashboard CRUD (Phase 10)" — PG-OPS-07 acceptance criteria
- `.planning/ROADMAP.md` §"v2.1 Candidate Operations" — locked milestone constraints (paper-only, 6 exchanges, score-threshold)

### Phase 8 / 9 continuity (already shipped — patterns to mirror)
- `.planning/phases/08-price-gap-tracker-core/08-CONTEXT.md` §"Config Surface" — D-22 PriceGapCandidate struct definition; D-23 /api/config reload contract
- `.planning/phases/09-price-gap-dashboard-paper-live-operations/09-CONTEXT.md` §"Candidate Disable / Re-enable Workflow" — existing per-candidate enable flag and button pattern; D-12 paper-mode config round-trip
- `.planning/phases/09-price-gap-dashboard-paper-live-operations/09-CONTEXT.md` §"Dashboard Layout" — Price-Gap tab structure that this modal extends

### Project-level constraints
- `CLAUDE.local.md` §"CRITICAL: Do not modify config.json" — config.json is live runtime; only `/api/config` POST + `SaveJSON` may write it
- `CLAUDE.local.md` §"npm security lockdown" — no new npm dependencies; modal must use existing React/Tailwind primitives
- `CLAUDE.local.md` §"Build & Run" — frontend build before Go binary (go:embed)
- `CLAUDE.local.md` §"Internationalization (i18n)" — EN + zh-TW lockstep enforcement
- `CLAUDE.local.md` §"New feature rollout pattern" — config switch + dashboard toggle + persisted to config.json

### Codebase references (patterns to mirror)
- `internal/config/config.go` — `PriceGapCandidate` struct + `PriceGapCandidates []PriceGapCandidate` slice (Phase 8 D-22 location)
- `internal/api/config_handlers.go` and `config_handlers_test.go` — `/api/config` GET/POST envelope, `.bak` backup flow, validation error pattern
- `internal/api/spot_handlers.go:370` — `handleSpotAutoConfig` reference pattern for shared GET/POST handler with persistence
- `internal/pricegaptrader/` — tracker reads `cfg.PriceGapCandidates` per tick (verify in plan-phase that no startup-cache exists)
- `cmd/pg-admin/main.go` — existing CLI; out of scope for Phase 10 but referenced for symmetry checks
- `web/src/pages/PriceGap.tsx` — existing tab to extend; existing Disable/Re-enable buttons live here
- `web/src/pages/Positions.tsx`, `web/src/pages/SpotPositions.tsx` — table + modal patterns to mirror
- `web/src/i18n/en.ts`, `web/src/i18n/zh-TW.ts` — i18n lockstep; `priceGap.*` namespace already in use
- `web/src/App.tsx` — Page union and nav already include `'price-gap'`; no nav changes needed

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `PriceGapCandidate` struct (Phase 8) — already defines all 6 fields the modal manipulates. No new model needed.
- `/api/config` POST handler — already does in-memory update + `config.json` write with `.bak` backup. Add `pricegap.candidates` to the JSON schema it accepts.
- `writeJSON` helper in `internal/api/handlers.go` — standard `{ok, data?, error?}` envelope.
- `useLocale()` hook + `t()` translator — wired throughout `web/src/`; `priceGap.*` keys already in both locales from Phase 9.
- `pg:positions:active` Redis set — already maintained by the tracker; reuse for delete-safety check (D-14).

### Established Patterns
- Config switches default OFF, written only via `/api/config` POST, with `.bak` backup (per `CLAUDE.local.md` rollout pattern).
- Frontend tabs follow REST-seed + WS-deltas; this phase reuses REST seed via `GET /api/config` and adds no WS surface.
- Modal-style overlays in other tabs use simple Tailwind absolute positioning; no UI library dependency (npm lockdown).
- i18n keys added to BOTH `en.ts` and `zh-TW.ts` in the same commit; `TranslationKey` type is derived from `en.ts` so missing zh-TW keys still typecheck (gap caught only at runtime — planner should consider a parity test).

### Integration Points
- `internal/api/config_handlers*.go` — extend the JSON schema accepted by `POST /api/config` to include `pricegap.candidates: PriceGapCandidate[]`. Validate per D-05..D-10. Wire delete-safety check (D-14) by reading `pg:positions:active`.
- `internal/config/config.go` — no struct changes needed (PriceGapCandidate already defined). Verify the `pricegap` JSON tag mapping covers the candidate slice for round-trip.
- `web/src/pages/PriceGap.tsx` — add "Add candidate" button + modal mount + delete confirmation. Re-render candidate table from POST response.
- `web/src/i18n/{en,zh-TW}.ts` — add `priceGap.candidates.modal.*`, `priceGap.candidates.errors.*`, `priceGap.candidates.confirmDelete.*` namespaces.

</code_context>

<specifics>
## Specific Ideas

- The PG-OPS-07 acceptance criterion explicitly names six fields; treat that list as authoritative — don't add `enabled`, `priority`, or any new field in this phase even if "it would be easy". Phase 9's separate Disable button keeps the modal scope clean.
- The `(symbol, long_exch, short_exch)` tuple as natural key is the same identity Phase 8 D-08 already uses for direction-pinned events ("operators who want both directions configure two candidate entries") — no new identity model.
- Delete-with-active-position blocking is the safer default and avoids orphaned position state in `pg:positions:active`. If operators object during UAT, downgrade to a warning in v2.2.

</specifics>

<deferred>
## Deferred Ideas

- **pg-admin CLI parity** for add/edit/delete — current CLI is read-only on candidate config (status/enable/disable/positions list). Worth doing if operators ask for it; otherwise dashboard-only is the simpler surface. Backlog for v2.2 if requested.
- **WS broadcast on candidate change** — multi-operator live sync. Defer to v3.0 (matches Phase 9 D-09 minimal-scope rationale; arb bot is single-operator today).
- **Bulk import / CSV upload** of candidates — operationally useful once auto-discovery (Phase 11) is producing dozens of scored entries; out of scope for the manual-CRUD phase.
- **Audit log of candidate edits** — who/when/what — useful for multi-operator world; v3.0.
- **Optimistic UI** for the candidate table — defer; re-render from POST response is sufficient at current scale (≤10 candidates expected pre-auto-discovery).
- **Per-candidate "test fire"** button (run risk gates without entry) — debugging aid; not requested in PG-OPS-07.

</deferred>

---

*Phase: 10-dashboard-candidate-crud*
*Context gathered: 2026-04-25 (auto mode — recommended defaults selected for all gray areas; see DISCUSSION-LOG.md for full audit trail)*
