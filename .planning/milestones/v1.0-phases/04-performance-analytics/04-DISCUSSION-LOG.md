# Phase 4: Performance Analytics - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-03
**Phase:** 04-performance-analytics
**Areas discussed:** Dashboard structure, PnL decomposition detail, Charting approach

---

## Dashboard Structure

| Option | Description | Selected |
|--------|-------------|----------|
| New "Analytics" page | Dedicated page in sidebar nav for charts, strategy comparison, exchange metrics. Enhance History with expandable PnL breakdown. Overview keeps stat cards. | |
| Expand Overview | Add charts and metrics directly to Overview. History gets per-position detail. No new page. | |
| New page + enhance both | New Analytics page for time-series charts and comparisons. Overview gets summary metrics. History gets per-position drill-down. | ✓ |

**User's choice:** New page + enhance both
**Notes:** Analytics page for charts/comparisons, Overview keeps summary metrics, History gets per-position drill-down.

---

## PnL Decomposition Detail

| Option | Description | Selected |
|--------|-------------|----------|
| Practical breakdown | Entry fees, funding earned, exit fees (estimate), net PnL. Skip basis/slippage. Borrow cost for spot-futures only. | |
| Full decomposition | Entry fees, exit fees, funding earned, basis gain/loss, borrow cost, slippage estimate, net PnL. Requires new tracking fields. | ✓ |
| Start practical, add detail later | Ship 4-5 component breakdown first, add basis/slippage later if needed. | |

**User's choice:** Full decomposition
**Notes:** All components tracked and displayed. New fields needed on position models.

### Follow-up: Slippage Calculation

| Option | Description | Selected |
|--------|-------------|----------|
| Estimated at execution | Compare fill price against best bid/ask at order placement. Requires capturing BBO snapshot. | ✓ |
| Approximated from fee tiers | Use exchange fee schedule to estimate. Less accurate, no new data capture. | |

**User's choice:** Estimated at execution — capture BBO snapshot at order time
**Notes:** None

---

## Charting Approach

### Library Selection

| Option | Description | Selected |
|--------|-------------|----------|
| Recharts | Most popular React charting lib. Declarative, composable. ~450KB. All chart types. Good React 19 + Tailwind support. | ✓ |
| Lightweight Charts | TradingView's lib. Excellent for financial time-series. ~45KB. Limited chart types (no bar/pie). | |
| Chart.js + react-chartjs-2 | Canvas-based, fast. ~200KB. All chart types. More imperative API. | |

**User's choice:** Recharts
**Notes:** Covers all needed chart types, React-native components, Tailwind friendly.

### Interactivity Level

| Option | Description | Selected |
|--------|-------------|----------|
| Basic | Tooltips, time range selector (7d/30d/90d/all), responsive sizing. No zoom/pan. | |
| Interactive | Tooltips, time range selector, click-to-zoom, brush selector for sub-range. | ✓ |
| You decide | Claude picks what fits. | |

**User's choice:** Interactive — tooltips, time range presets, zoom, brush selector
**Notes:** None

---

## Not Discussed

- **Storage architecture** (area 1) — User chose not to discuss. ROADMAP specifies SQLite for time-series; deferred to Claude's discretion within ROADMAP constraints.

## Claude's Discretion

- SQLite schema design and migration approach
- Analytics page layout and section ordering
- Exchange metrics aggregation approach
- APR calculation formula
- Win rate segmentation UI
- Strategy comparison chart type
- Position model field naming for new tracking fields

## Deferred Ideas

None — discussion stayed within phase scope
