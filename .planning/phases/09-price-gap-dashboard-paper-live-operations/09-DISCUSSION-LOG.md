# Phase 9: Price-Gap Dashboard & Paper→Live Operations - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-22
**Phase:** 09-price-gap-dashboard-paper-live-operations
**Areas discussed:** Tab layout & density, Paper mode UX & scope, Disable/re-enable workflow, Telegram content + rolling metrics

---

## Tab Layout & Density

### Q1 — Dashboard nav organization

| Option | Description | Selected |
|--------|-------------|----------|
| Single tab, stacked sections (Recommended) | One page: Status → Candidates → Live → Closed → Metrics | ✓ |
| Two tabs: Price-Gap + PG History | Split active vs history, matches Positions/History split | |
| Three tabs (PG / PG Positions / PG History) | Sibling pages like SpotPositions | |

**User's choice:** Single tab, stacked sections.

### Q2 — Candidate + live-position display

| Option | Description | Selected |
|--------|-------------|----------|
| Dense tables (Recommended) | Rows-and-columns, scales to many candidates | ✓ |
| Cards with inline sparklines | Dashboard-y but heavier | |
| Mixed: table + detail drawer | Deep-dive UX but more work | |

**User's choice:** Dense tables.

### Q3 — Real-time update mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Extend WS hub with pg_* topics (Recommended) | Matches existing dashboard pattern | ✓ |
| REST polling only (5s) | Simpler but inconsistent | |
| SSE / EventSource | New transport | |

**User's choice:** Extend existing WS hub.

### Q4 — Live-positions columns

| Option | Description | Selected |
|--------|-------------|----------|
| Core (Recommended) | Symbol, exchs, size, entry/current spread, hold, PnL, paper/live | ✓ |
| Distance-to-exit signals | T/2, max-hold remaining, ready-to-close | |
| Executional detail | Slippage, fill time, order IDs | |
| Risk gate reference | Gates evaluated at entry | |

**User's choice:** Core columns only.

---

## Paper Mode UX & Scope

### Q1 — Where the paper toggle lives

| Option | Description | Selected |
|--------|-------------|----------|
| Price-Gap page header (Recommended) | Prominent, makes mode obvious during 3-day validation | ✓ |
| Config tab > Price-Gap section | Consistent with other config, less discoverable | |
| Both | Status in Config, authoritative on page header | |

**User's choice:** Page header.

### Q2 — Visual distinction paper vs live

| Option | Description | Selected |
|--------|-------------|----------|
| 'PAPER' badge on each row (Recommended) | Single table, single codepath, readable across transition | ✓ |
| Separate paper/live table sections | Clean split but duplicates UI code | |
| Global banner + single table | Simpler but brittle if flipped mid-day | |

**User's choice:** Badge + row tint.

### Q3 — Paper persistence

| Option | Description | Selected |
|--------|-------------|----------|
| Shared keys + mode field (Recommended) | One codepath, uniform closed log | ✓ |
| Namespace split pg:paper:* vs pg:live:* | Cleaner separation, double the code | |
| Paper in-memory only | Fails on restart | |

**User's choice:** Shared keys with `mode` field.

### Q4 — Paper→Live transition

| Option | Description | Selected |
|--------|-------------|----------|
| Manual toggle only (Recommended) | Defensible, no surprise flips | ✓ |
| Manual + force-close-paper button | Safer against mixed-mode window | |
| Auto after N clean days | Couples two decisions | |

**User's choice:** Manual toggle only.

---

## Disable / Re-enable Workflow

### Q1 — Disabled row presentation

| Option | Description | Selected |
|--------|-------------|----------|
| Greyed row + reason + timestamp + Re-enable button (Recommended) | Operator sees the exec-quality problem inline | ✓ |
| Hide disabled in collapsible section | Cleaner top view but hides safety issues | |
| Toggle switch inline | Minimal, loses "why" | |

**User's choice:** Greyed row + inline reason + Re-enable button.

### Q2 — Re-enable confirmation

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — modal with reason + disabled-at (Recommended) | Safety-gate override requires conscious step | ✓ |
| No — direct action | Fewer clicks, matches CLI | |
| Yes only for exec-quality | Two codepaths | |

**User's choice:** Modal confirmation for all re-enables.

### Q3 — Dashboard manual disable

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, with reason text field (Recommended) | Symmetric with pg-admin CLI | ✓ |
| No — pg-admin only | Read-only view, requires CLI | |

**User's choice:** Yes, with reason.

### Q4 — Disable history in UI

| Option | Description | Selected |
|--------|-------------|----------|
| No — only current state (Recommended) | Minimal MVP, history in logs | ✓ |
| Yes — last 5 toggles | Needs new Redis schema | |

**User's choice:** Current state only.

---

## Telegram Content + Rolling Metrics

### Q1 — Telegram events

| Option | Description | Selected |
|--------|-------------|----------|
| Entry + Exit (Recommended) | Entry/exit no cooldown | ✓ (initial) |
| Risk-gate block (cooldowned) (Recommended) | Per-gate-per-symbol cooldown | ✓ (after scope-check) |
| Exec-quality auto-disable (Recommended) | High-signal, no cooldown | |
| Paper↔live toggle flip | Paper-trail of capital deployment | |

**User's initial choice:** Entry + Exit only.
**Scope check:** PG-OPS-05 explicitly names entry/exit/risk-gate blocks → user confirmed keep the requirement as written.
**User's final choice:** Entry + Exit + Risk-gate block (cooldowned).
**Notes:** Exec-quality auto-disable alert NOT added in Phase 9 (deferred); paper↔live toggle-flip alert NOT added (deferred).

### Q2 — Alerts in Paper mode

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, tagged '📝 PAPER' (Recommended) | Validates pipeline before live money | ✓ |
| No — live only | Silent paper runs | |
| Yes, only entries/exits | Skip paper risk blocks | |

**User's choice:** Yes, tagged '📝 PAPER'.

### Q3 — PG-VAL-02 metrics computation

| Option | Description | Selected |
|--------|-------------|----------|
| On-demand from closed positions (Recommended) | Simplest, no new write path | ✓ |
| Pre-aggregated sorted set on close | Faster query, rebuild-tool burden | |
| Redis Streams + rollup goroutine | Overkill | |

**User's choice:** On-demand.

### Q4 — Metrics display

| Option | Description | Selected |
|--------|-------------|----------|
| Per-candidate table 24h/7d/30d (Recommended) | Matches dense-tables choice | ✓ |
| Table + 30d sparkline | Nice but needs chart primitive | |
| Table + promote/demote hint | Bakes thresholds in, defers | |

**User's choice:** Table only, 24h/7d/30d columns.

---

## Claude's Discretion

- i18n key names and exact English + zh-TW copy (must be in lockstep).
- Row tint / badge / button colors.
- Empty-state copy per section.
- Default pagination / row cap on closed log.
- Default sort order per table (except closed log, pinned to close-time desc).
- Loading skeleton approach.
- Modal component choice.
- REST seed endpoint shape (aggregate vs per-section).
- Whether candidate enabled/disabled toggle persists to `config.json` or stays in Redis — planner chooses to match Phase 8's data model without schema drift.

## Deferred Ideas

- Per-candidate sparkline charts.
- Promote/demote "Suggested action" column.
- Disable history UI.
- Auto paper→live transition.
- Force-close-all-paper button.
- Detail drawer per candidate/position.
- Separate pages for PG Positions and PG History.
- SSE/EventSource transport.
- Redis Streams + background rollup.
- Exec-quality auto-disable Telegram alert.
- Paper↔live toggle-flip Telegram alert.
