# Phase 4: Performance Analytics - Context

**Gathered:** 2026-04-03
**Status:** Ready for planning

<domain>
## Phase Boundary

The user can see exactly how much each position earned (full PnL decomposition), compare strategy performance (perp-perp vs spot-futures), evaluate exchange performance, and track cumulative PnL over time with interactive charts. This phase adds a new Analytics dashboard page, enhances the existing History page with per-position drill-down, and keeps Overview stat cards as summary metrics.

</domain>

<decisions>
## Implementation Decisions

### Dashboard Structure
- **D-01:** New dedicated "Analytics" page in sidebar nav for time-series charts, strategy comparison, and exchange metrics.
- **D-02:** Enhance existing History page with expandable per-position PnL breakdown (drill-down per row).
- **D-03:** Overview page keeps its existing stat cards as summary metrics — no charts added to Overview.

### PnL Decomposition (AN-01, PP-04)
- **D-04:** Full decomposition per closed position: entry fees, exit fees, funding earned, basis gain/loss, borrow cost (spot-futures only), slippage, net PnL. All components displayed in History drill-down.
- **D-05:** New tracking fields required on position models: exit fees, basis gain/loss, borrow cost (accumulated interest for spot-futures), slippage (BBO delta).
- **D-06:** Slippage estimated at execution time — capture best bid/ask (BBO) snapshot at order placement, compare against actual fill price, store delta on position.

### Charting (AN-04)
- **D-07:** Charting library: Recharts. Declarative React components, all chart types needed (line, area, bar), good React 19 + Tailwind compatibility.
- **D-08:** Interactive charts: tooltips on hover, time range presets (7d / 30d / 90d / all), click-to-zoom, brush selector for sub-range selection.
- **D-09:** npm lockfile update required to add Recharts — follows existing npm security constraint (only `npm ci` after lockfile is updated).

### Storage Architecture
- **D-10:** Not discussed — user deferred storage architecture decisions. ROADMAP specifies SQLite for time-series. Claude and downstream agents should follow the ROADMAP specification (SQLite-backed time-series).

### Claude's Discretion
- SQLite schema design and migration approach
- Specific position model field types and names for new tracking fields (exit fees, basis, borrow cost, slippage)
- Analytics page layout (chart placement, section ordering)
- Exchange metrics aggregation approach (AN-03)
- APR calculation formula details (AN-05)
- Win rate segmentation UI (AN-06)
- Strategy comparison chart type (AN-02) — line, bar, or combined

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Position Models & PnL
- `internal/models/position.go` — Current position fields: `RealizedPnL`, `FundingCollected`, `EntryFees`, `RotationPnL` (lines 22, 38-40). New fields needed here.
- `internal/models/spot_position.go` — Spot-futures position model, needs borrow cost field
- `internal/engine/exit.go:794-955` — `reconcilePnL()` — current PnL reconciliation logic, handles shared positions
- `internal/engine/engine.go:1660-1807` — `trackFunding()` / `updateFundingCollected()` — funding collection infrastructure

### Existing Dashboard
- `web/src/pages/History.tsx:74-114` — Current history table (entry/exit prices, spread, funding, PnL, hold time, exit reason). Needs expandable drill-down.
- `web/src/pages/Overview.tsx:81-96` — Stat cards (Total PnL, Win Rate, Active Positions, Balance). Stays as-is.
- `internal/api/handlers.go:128-154` — `handleGetHistory()` — history API endpoint
- `internal/api/handlers.go:157-268` — `handleGetPositionFunding()` — per-position funding history

### Redis State
- `internal/database/state.go:157-189` — Active positions in `arb:positions`
- `internal/database/state.go:274-295` — Closed positions in `arb:history:0`
- `internal/database/state.go:399-412` — Stats hash `arb:stats` (total_pnl, trade_count, win_count, loss_count)

### Spot-Futures Engine (borrow cost tracking)
- `internal/spotengine/execution.go` — `executeBorrowSellLong()`, `executeBuySpotShort()` — where borrow cost tracking would start
- `internal/spotengine/exit_manager.go` — Exit flow where borrow cost accumulation would finalize
- `pkg/exchange/types.go` — `SpotMarginExchange` interface, `GetMarginInterestRate()` method

### Config & Dashboard Patterns
- `internal/config/config.go` — 6-touch-point config field convention
- `web/src/i18n/en.ts` / `web/src/i18n/zh-TW.ts` — Both locales must stay in sync for new analytics strings
- `web/src/hooks/useApi.ts` — REST hook pattern
- `web/src/hooks/useWebSocket.ts` — Real-time update pattern

### Prior Phase Contexts
- `.planning/phases/03-operational-safety/03-CONTEXT.md` — Safety tab color convention, config toggle patterns

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **PnL reconciliation** (`exit.go:reconcilePnL()`): Already queries exchange ClosePnL, handles multi-leg positions. Extend to capture exit fees and basis separately.
- **Funding tracker** (`engine.go:trackFunding()`): Runs continuously, updates per-position funding. Infrastructure for time-series snapshots.
- **History API** (`handlers.go:handleGetHistory()`): Paginated closed position endpoint. Extend response with decomposition fields.
- **Position funding endpoint** (`handlers.go:handleGetPositionFunding()`): Per-payment funding history with timestamps.
- **Stats hash** (`arb:stats`): Aggregate counters already maintained. Extend for per-strategy/per-exchange segmentation.
- **Borrow rate fetching** (`spotengine/discovery.go:getCachedBorrowRate()`): 5-min cached rates per exchange. Can inform borrow cost accumulation.

### Established Patterns
- Config fields: `Enable{Feature}` bool (default OFF) + JSON tag + dashboard toggle (6 touch-points)
- Dashboard pages: functional components with `useLocale()`, `useApi()`, `useWebSocket()` hooks
- API responses: `{ ok: bool, data?: T, error?: string }` envelope via `writeJSON()`
- i18n: dot-notation keys, both `en.ts` and `zh-TW.ts` must stay in sync
- Sidebar nav: existing page routing in `App.tsx`

### Integration Points
- `internal/engine/engine.go` — Where BBO snapshot capture would hook in (before `PlaceOrder` calls)
- `internal/spotengine/execution.go` — Where borrow cost tracking starts (on entry) and accumulates
- `web/src/App.tsx` — Add Analytics route + sidebar nav entry
- `cmd/main.go` — SQLite initialization and injection into API server
- `internal/api/server.go` — New analytics API endpoints

</code_context>

<specifics>
## Specific Ideas

- BBO snapshot: capture `bestBid`/`bestAsk` from the price store immediately before placing each order leg, store on the position for post-trade slippage calculation
- Borrow cost for spot-futures: accumulate interest payments over position lifetime using periodic `GetMarginInterestRate()` queries, finalize on exit
- Recharts brush component enables drag-to-zoom on the cumulative PnL chart — matches the "interactive" requirement naturally

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 04-performance-analytics*
*Context gathered: 2026-04-03*
