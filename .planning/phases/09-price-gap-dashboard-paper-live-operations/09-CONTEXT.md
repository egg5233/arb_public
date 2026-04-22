# Phase 9: Price-Gap Dashboard & Paper→Live Operations - Context

**Gathered:** 2026-04-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Operator-facing surface for the Phase 8 `internal/pricegaptrader/` subsystem:

1. New dashboard page that shows the candidate list with enable/disable toggles, live positions, closed-positions log with realized-vs-modeled edge, and rolling per-candidate metrics (PG-OPS-01, PG-OPS-02, PG-OPS-03, PG-VAL-02).
2. Paper mode toggle that runs full event/entry/exit logic without placing real orders, used for the first ~3 live days (PG-OPS-04).
3. Telegram notifications for entry, exit, and risk-gate blocks, reusing the existing `internal/notify/` cooldown infrastructure (PG-OPS-05).
4. Per-trade realized-vs-modeled slippage logging, with rolling 7d/30d bps/day per candidate surfaced to the dashboard (PG-VAL-01, PG-VAL-02).

Out of scope: new risk gates, scanner/discovery layer, full Gated-Discovery (D) architecture, auto-promotion from paper to live, price-gap analytics beyond rolling bps/day. The tracker core from Phase 8 is fixed — this phase wraps it.

</domain>

<decisions>
## Implementation Decisions

### Dashboard Layout
- **D-01:** One new top-level dashboard page `Price-Gap` registered alongside existing pages (`overview`, `positions`, `spot-positions`, `history`, `analytics`, `config`, ...). Page is a single scroll with stacked sections: **Status bar → Candidates (with toggles) → Live Positions → Closed Log → Rolling Metrics**. No sibling `PG Positions` / `PG History` pages.
- **D-02:** Dense tables throughout — no cards, no inline sparklines, no detail drawer in this phase. Matches existing `Positions`/`SpotPositions`/`History` ergonomics.
- **D-03:** Live-positions table columns (PG-OPS-02 core): Symbol, Long-exch, Short-exch, Size, Entry spread (bps), Current spread (bps), Hold time, Current PnL (USDT + bps), Paper/Live badge. No distance-to-exit / executional detail / risk-gate reference columns in v1 — those are deferred.
- **D-04:** Closed-log table columns (PG-OPS-03): Symbol, Long/Short, Size, Entry spread, Exit spread, Hold duration, Reason, Realized bps, **Modeled bps**, **Delta (realized − modeled) bps**, PnL USDT, Mode. Default sort: close-time descending.

### Real-Time Updates
- **D-05:** Extend the existing WebSocket hub in `internal/api/server.go` with new topics: `pg_positions` (full active list), `pg_event` (entry/exit/auto-disable), `pg_candidate_update` (toggle / disable / re-enable). REST endpoint seeds initial state; WS pushes incremental updates. No SSE, no polling fallback, no new transport.
- **D-06:** Frontend uses the existing WS pattern from `Positions`/`Opportunities` pages — REST seed on mount, WS handlers replace/patch state.

### Paper Mode (PG-OPS-04)
- **D-07:** Paper toggle lives prominently in the Price-Gap page **header**, next to the master enable switch. Not in the Config tab. This keeps the mode obvious during the 3-day paper validation period.
- **D-08:** Paper positions are visually distinguished by a **`PAPER` badge** in the Symbol column plus a soft row tint. Single table, single codepath; no split paper/live sections, no global banner.
- **D-09:** Paper trades persist in the same Redis keys as live (`pg:positions`, `pg:positions:active`, closed set, `pg:slippage:*`). Each position carries a `mode: "paper" | "live"` field stamped at entry. This lets rehydrate, monitor, and the closed log stay single-codepath.
- **D-10:** Paper mode implementation must run the **full event detection, risk-gate evaluation, and entry/exit decision logic** — only the actual `PlaceOrder` call is skipped. Fills are synthesized at mid (or the gap-derived quote) and realized slippage for paper is computed identically to live so the slippage-logging pipeline is exercised end-to-end.
- **D-11:** `paper → live` transition is **manual-only**: operator flips the toggle explicitly. No auto-promote after N days, no "force-close all paper" button in this phase. Existing paper positions keep their `mode: "paper"` tag through close.
- **D-12:** Config surface: `PriceGapPaperMode bool` (default `true`, safe until validated). Dashboard POST `/api/config` round-trip persists the flag to `config.json` (per the project new-feature rollout pattern). Never written by anything else.

### Candidate Disable / Re-enable Workflow
- **D-13:** Disabled candidate rows are visually greyed; the row shows `Disabled: <reason>` (e.g., `exec-quality breach`, `manual`) plus the disabled-at timestamp, plus an inline **Re-enable** button. Disabled rows remain in the main candidate list — not hidden in a collapsible section.
- **D-14:** Re-enabling is gated by a **confirmation modal** that displays the reason and disabled-at ("Re-enable BTCUSDT? Disabled 2h ago for exec-quality breach (realized 34 bps vs modeled 18 bps). This clears the safety gate."). Applies to both auto and manual disables.
- **D-15:** Operators can **manually disable** a candidate from the dashboard with an optional reason text field (defaults to `"manual"`). Backend writes `pg:candidate:disabled:<symbol>` identically to the `pg-admin disable` CLI path — same Redis key, same value shape.
- **D-16:** No disable history UI in this phase. Only current state (disabled flag + reason + disabled-at) is rendered. Historical toggles remain visible in logs.
- **D-17:** The dashboard re-enable path is fully equivalent to `pg-admin enable <symbol>` (clears `pg:candidate:disabled:<symbol>`). `pg-admin` CLI remains supported — dashboard does not replace or deprecate it, it just covers the common case.

### Telegram Notifications (PG-OPS-05)
- **D-18:** Three notification methods added to `internal/notify/telegram.go`, each a nil-safe method on `*TelegramNotifier`:
  - `NotifyPriceGapEntry(pos *PriceGapPosition)` — no cooldown
  - `NotifyPriceGapExit(pos *PriceGapPosition, reason string, pnl float64, duration time.Duration)` — no cooldown
  - `NotifyPriceGapRiskBlock(symbol string, gate string, detail string)` — cooldown keyed by `pg_risk:<gate>:<symbol>`, reusing the existing 1 h default `checkCooldown` helper
- **D-19:** Entry message content: symbol, long/short exchanges, size, entry spread bps, modeled edge bps, paper/live tag.
- **D-20:** Exit message content: symbol, exit reason (`max_hold` / `spread_reversion` / `manual` / other), PnL (USDT + bps), realized slippage bps, hold duration, paper/live tag.
- **D-21:** Risk-gate block message content: symbol, which gate (`concentration`, `max_concurrent`, `kline_stale`, `delist`, `budget`, `exec_quality`), short detail string.
- **D-22:** Alerts fire in **Paper mode** too, with the message prefixed by `📝 PAPER` (emoji + label). This validates the alert pipeline before live money trades and gives the operator parity between paper and live.

### Slippage & Rolling Metrics (PG-VAL-01, PG-VAL-02)
- **D-23:** Confirm Phase 8's `pg:slippage:*` write path writes per-trade realized slippage at both entry and exit (round-trip). If Phase 8 only persists the aggregated-ratio value used by the auto-disable gate, extend the schema to persist per-trade records sufficient for the realized-vs-modeled comparison in the closed log. Shape: `{position_id, candidate, entry_ts, exit_ts, modeled_bps, realized_entry_bps, realized_exit_bps, realized_total_bps, pnl_usdt, mode}`.
- **D-24:** PG-VAL-02 rolling metrics are computed **on-demand** — at request time from the closed-position set + `pg:slippage:*`. No pre-aggregated sorted set, no background rollup goroutine. Acceptable because the data volume (hundreds of trades per candidate) is well within a single Redis read.
- **D-25:** Metrics table columns: Candidate, Trades (window), Win %, Avg realized bps, **24h bps/day**, **7d bps/day**, **30d bps/day**. Sortable. One row per candidate. No per-candidate sparklines, no promote/demote recommendation column (deferred).

### Claude's Discretion
- i18n key names and the exact English + zh-TW copy (must stay in sync — both locale files updated in lockstep).
- Row tint / badge / button exact colors and Tailwind classes.
- Empty-state copy for each section.
- Default pagination / row cap on the closed log.
- Default sort direction for each table (except the closed log, which is pinned to close-time desc).
- Loading-skeleton / spinner approach during REST seed.
- Exact modal library / component (reuse whatever `/dashboard` already uses).
- The precise shape of the REST seed endpoints (e.g., one aggregate `/api/pricegap/state` vs per-section endpoints).
- Whether the candidate toggle persists to `config.json` (like other config switches) or only to Redis (like the PG-RISK-03 disable flag) — planner to pick the approach that matches the Phase 8 data model without surprise.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 8 continuity (tracker core — already shipped)
- `.planning/phases/08-price-gap-tracker-core/08-CONTEXT.md` — locked decisions for the tracker core, especially D-01..D-23 (module boundary, Redis key prefix, risk-gate names, slippage schema).
- `.planning/phases/08-price-gap-tracker-core/08-PLAN.md` (and all plan siblings) — what was actually built.
- `/tmp/phase0-pricegap/STRATEGY_DESIGN.md` — Phase 0/1/round-2 design doc; MVP scope and (D) deferral rationale.

### Project-level constraints
- `/var/solana/data/arb/CLAUDE.local.md` — New-feature rollout pattern (config switch + dashboard toggle + `config.json` persistence, default OFF), "never modify config.json" rule (Phase 9 dashboard POST `/api/config` is the authorized writer), i18n lockstep rule, module boundary rules, delegation mode, Dashboard API envelope (`{ ok, data?, error? }`), bearer token auth.
- `/var/solana/data/arb/.planning/PROJECT.md` — v2.0 milestone goal, active requirements list, 4–8 week live-validation gate.
- `/var/solana/data/arb/.planning/REQUIREMENTS.md` §"Live Validation Infrastructure" — PG-OPS-01..05, PG-VAL-01, PG-VAL-02 exact wording and acceptance.
- `/var/solana/data/arb/.planning/ROADMAP.md` — Phase 9 goal and success criteria.

### Codebase references (patterns to mirror)
- `internal/pricegaptrader/` — tracker, detector, execution, risk_gate, monitor, rehydrate, slippage. New backend code in this phase (paper-mode branch, manual disable handler, metrics aggregator) lives here or in a thin sibling package; no imports of `internal/engine/` or `internal/spotengine/`.
- `cmd/pg-admin/main.go` — existing CLI for `status / enable / disable / positions list`. Dashboard manual-disable and re-enable must use **identical Redis semantics** (same key, same value shape) so operator can mix CLI and UI without drift.
- `internal/api/server.go` — WebSocket hub (`BroadcastPositionUpdate`, `BroadcastOpportunities`, `BroadcastStats`, `BroadcastAlert`, `hub.Broadcast`). New `BroadcastPriceGap*` methods follow the same signature + registration pattern.
- `internal/api/config_handlers*.go` — `/api/config` GET/POST round-trip, `.bak` backup logic, jsonPriceGap nested struct. PaperMode and per-candidate `enabled` flag extend this.
- `internal/notify/telegram.go` — `NewTelegram`, `send`, `Send`, `checkCooldown{At}`, `NotifyAutoEntry`, `NotifyAutoExit`, `NotifyEmergencyClose`, `NotifySLTriggered`, `formatDuration`, `formatExitReason`. New `NotifyPriceGap*` methods mirror these patterns; nil-safe receiver.
- `web/src/App.tsx` — `Page` union type (add `'price-gap'`), nav item registration, `renderPage()` switch, `labelKey` for i18n.
- `web/src/pages/Positions.tsx`, `web/src/pages/SpotPositions.tsx`, `web/src/pages/History.tsx`, `web/src/pages/Analytics.tsx` — table patterns, WS subscription patterns, empty states, i18n usage.
- `web/src/i18n/en.ts`, `web/src/i18n/zh-TW.ts` — i18n lockstep. Every new key added to both.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **WS hub** (`internal/api/server.go`): `hub.Broadcast(topic, payload)` is the one-line addition path for new pg_* topics.
- **Telegram notifier** (`internal/notify/telegram.go`): `checkCooldown(eventKey)` already supports per-key cooldowns; new `pg_risk:<gate>:<symbol>` keys fit the existing pattern.
- **Config handler** (`internal/api/config_handlers*.go`): `jsonPriceGap` nested struct pattern already exists from Phase 8; adding `paper_mode` and per-candidate `enabled` is a matter of extending the struct.
- **`pg-admin` CLI**: confirms the Redis key/value shapes for disable/enable — dashboard handlers wrap the same Redis ops, no schema divergence allowed.
- **Frontend pages**: `Positions.tsx` is the closest template for a WS-fed, table-dense view and can be copied as the scaffold for the Price-Gap page sections.
- **Locale files**: `t()` hook + `TranslationKey` type-safety already in place.

### Established Patterns
- REST seeds + WS deltas for every live-data page.
- Bearer-token auth via `localStorage[arb_token]` for every request.
- `{ ok, data?, error? }` response envelope (`writeJSON` helper).
- Config switches default OFF and are persisted to `config.json` via POST `/api/config` with `.bak` backup.
- Nil-safe methods on `*TelegramNotifier` (we may have no configured notifier in dev).
- Tables in Asia/Taipei timezone via existing display helpers.

### Integration Points
- `main.go`: pricegaptrader startup block (already exists post-Phase 8); Phase 9 reads the paper-mode flag here before handing the tracker its exchange clients.
- `internal/pricegaptrader/execution.go`: the single chokepoint where live vs paper branches — `PlaceOrder` call is replaced with a synthesized fill when `mode == paper`.
- `internal/pricegaptrader/slippage.go`: the write path for per-trade realized slippage. Planner verifies/extends the persisted record to carry modeled and realized together.
- `internal/api/server.go`: WS hub — new methods + REST handlers.
- `internal/api/router.go` (or equivalent): new routes under `/api/pricegap/*`.
- `web/src/App.tsx`: add `'price-gap'` to `Page` type; import and register new `PriceGap.tsx` page.
- `web/src/i18n/*`: new keys under `pricegap.*`.

</code_context>

<specifics>
## Specific Ideas

- The Price-Gap page should feel like a lightweight extension of `Positions.tsx` — not a new design language. Operator already lives in that UI.
- The `📝 PAPER` emoji+tag in Telegram messages is the signal that matters; plain-text is fine, no rich formatting required.
- Manual-disable with a free-text reason mirrors `pg-admin disable <symbol> [why]` — operators should be able to type "exchange maint window" and have it stick.
- Re-enable confirmation text: surface the exact disabled-reason string from Redis verbatim (so operator sees what the system saw).
- The 24h/7d/30d metrics columns are informational; no gating, no auto-action. Humans decide promote/demote in this milestone.

</specifics>

<deferred>
## Deferred Ideas

- **Per-candidate sparkline charts** on the metrics table — needs a tiny chart primitive and daily binning; defer to v2.1 or the full (D) architecture phase.
- **Promote/Demote "Suggested action" column** based on bps/day thresholds — couples too many decisions (what thresholds? what confidence window?) into a dashboard; belongs in a dedicated strategy-tuning phase.
- **Disable history UI** (last N toggles per candidate) — needs a new `pg:candidate:history:<symbol>` sorted set and write-on-toggle logic; not required for the 4–8 week validation window.
- **Auto paper→live transition after N clean days** — couples "when to trust" with "when to deploy capital"; operator decides explicitly in v2.0.
- **Force-close-all-paper button** — nice defensive UX; defer unless paper→live transition reveals stuck positions.
- **Detail drawer per candidate / position** (chart + slippage history + recent events) — useful deep-dive; out of MVP.
- **Separate pages for PG Positions and PG History** — revisit if the single page becomes unwieldy.
- **SSE / EventSource transport** — not needed; WS hub is sufficient.
- **Redis Streams + background rollup for metrics** — overkill at current data volume.

</deferred>

---

*Phase: 09-price-gap-dashboard-paper-live-operations*
*Context gathered: 2026-04-22*
