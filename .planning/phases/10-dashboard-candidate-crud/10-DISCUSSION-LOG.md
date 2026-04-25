# Phase 10: Dashboard Candidate CRUD - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-25
**Phase:** 10-dashboard-candidate-crud
**Mode:** auto (single-pass; recommended defaults selected for all gray areas)
**Areas analyzed:** Modal Behavior, Field Validation, Identity & Edit Semantics, Delete Safety, Backend API Shape, Tracker Hot-Reload, Real-Time Update Surface, i18n Lockstep

---

## Modal Behavior & Layout

| Option | Description | Selected |
|--------|-------------|----------|
| Inline form below table | Less code, no overlay machinery | |
| Single shared Add/Edit modal with `mode` prop | One component, two flows; lightweight delete-confirm | ✓ |
| Separate Add modal + Edit modal | Cleaner per-flow code, more duplication | |

**Auto-selected:** Single shared modal (D-01..D-04).
**Reason:** PG-OPS-07 acceptance criterion explicitly says "modal"; mirrors existing tab patterns; minimizes code surface.

---

## Field Validation Rules

| Decision | Selected Default | Alternatives Considered |
|---|---|---|
| `symbol` format | `^[A-Z0-9]+USDT$`, auto-uppercase | Allow per-exchange formats (rejected — adapters handle mapping) |
| Exchange selectors | `<select>` of 6 hardcoded values | Free-text (rejected — typo risk); checkbox list (rejected — wrong semantics) |
| `long_exch ≠ short_exch` | Hard validation error | Soft warning (rejected — strategy is delta-neutral by definition) |
| `threshold_bps` range | 50–1000, default 200 | 1–10000 (rejected — too permissive, masks typos) |
| `max_position_usdt` range | 100–50000, default 5000 | No bounds (rejected — single typo could create $50k+ candidate) |
| `modeled_slippage_bps` range | 0–100, default 5 | No bounds (rejected — same risk) |
| Validation site | Frontend + backend (defense-in-depth) | Frontend only; backend only |

**Auto-selected:** D-05..D-10 — defense-in-depth with reasonable bounds.
**Reason:** Numeric fields without bounds are the most common cause of operator-typo incidents; backend re-validation matches existing config-handler patterns.

---

## Identity & Edit Semantics

| Option | Description | Selected |
|--------|-------------|----------|
| Array-index based edit | Edit "row 3"; brittle if list reorders | |
| Tuple-based natural key `(symbol, long_exch, short_exch)` | Stable, matches Phase 8 D-08 direction-pinned model | ✓ |
| Add `candidate_id` UUID field | Cleanest but requires schema change | |

**Auto-selected:** Tuple-based natural key (D-11..D-13).
**Reason:** Phase 8 already treats this tuple as the addressable entity; introducing a new ID field is unnecessary churn. Edit preserves the per-candidate `enabled` flag from Phase 9.

---

## Delete Safety

| Option | Description | Selected |
|--------|-------------|----------|
| No safety check, simple confirm | Fastest; risks orphaning active position state | |
| Block delete if active position; single-step confirm otherwise | Safer default; clear error message | ✓ |
| Two-step "type symbol to confirm" | Overkill for small candidate set | |
| Soft-delete with retention window | Useful but out of scope for v2.1 | |

**Auto-selected:** Block-on-active + single-step confirm (D-14, D-15).
**Reason:** Avoids orphaned `pg:positions:active` entries; small candidate set doesn't warrant heavy confirmation friction.

---

## Backend API Shape

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse `POST /api/config` with full candidate array | Matches PG-OPS-07 explicit ask; no new route | ✓ |
| New `/api/pricegap/candidates` REST CRUD | More REST-ful but adds surface area | |
| WebSocket-driven mutations | Overkill for single-operator system | |

**Auto-selected:** Reuse `POST /api/config` (D-16..D-18).
**Reason:** PG-OPS-07 explicitly names this endpoint; existing `.bak` backup + in-memory update flow handles persistence; no new test surface.

---

## Tracker Hot-Reload

| Option | Description | Selected |
|--------|-------------|----------|
| Restart-required after edit | Worst UX; defeats purpose of dashboard CRUD | |
| Tracker re-reads `cfg.PriceGapCandidates` per scan tick | Existing Phase 8 pattern; no new code | ✓ |
| Explicit "reload" button on dashboard | Unnecessary if per-tick read works | |

**Auto-selected:** Per-tick re-read (D-19).
**Reason:** Phase 8 D-22/D-23 already specifies config reload through `/api/config`. Plan-phase must verify no startup-cache exists; if it does, add a refresh hook.

---

## Real-Time Update Surface

| Option | Description | Selected |
|--------|-------------|----------|
| Add `BroadcastConfigUpdated` WS event | Multi-operator sync; new surface for v2.1 | |
| Frontend re-renders from POST response | Minimal; sufficient for single-operator | ✓ |
| Polling `GET /api/config` every N seconds | Wasteful | |

**Auto-selected:** Re-render from POST response (D-20).
**Reason:** Arb bot is single-operator today; multi-operator concurrent-edit reconciliation is v3.0 territory (matches Phase 9 D-09 minimal-scope rationale).

---

## i18n Lockstep

| Option | Description | Selected |
|--------|-------------|----------|
| Add keys to `en.ts` only, hardcode zh-TW later | Violates project convention | |
| Add to both `en.ts` and `zh-TW.ts` in same commit, namespace `priceGap.candidates.*` | Project standard | ✓ |
| Switch to `i18next` with auto-extraction | npm lockdown forbids new deps | |

**Auto-selected:** Locale parity in same commit (D-21).
**Reason:** `CLAUDE.local.md` i18n rule is non-negotiable; npm lockdown forbids new tooling.

---

## Claude's Discretion

The following were left to the planner / executor:

- Exact CSS / Tailwind class names (mirror `Positions.tsx` aesthetic)
- Modal field order (suggested: symbol → exchanges → threshold → sizing)
- Whether tuple alone or a synthetic id is shown in the table column header
- Modal accessibility (focus trap, ARIA roles) — defer to standard React patterns already in use
- Optional component-level tests for the modal — leave to planner per project test-coverage norms

## Auto-Resolved (auto mode)

All 8 areas were auto-resolved with the recommended defaults documented above. No areas required user input. Single discussion pass; CONTEXT.md is the canonical record.

## Deferred Ideas

Captured in CONTEXT.md `<deferred>` section:
- pg-admin CLI parity (v2.2 if requested)
- WS broadcast on candidate change (v3.0)
- Bulk CSV import (post-Phase 11)
- Audit log of edits (v3.0)
- Optimistic UI (defer; current scale doesn't need it)
- Per-candidate "test fire" button (debugging aid; not in PG-OPS-07)
