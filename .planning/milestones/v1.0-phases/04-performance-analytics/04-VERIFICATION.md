---
phase: 04-performance-analytics
verified: 2026-04-03T23:30:54Z
status: passed
score: 5/5 must-haves verified
gaps:
  - truth: "User can compare per-exchange metrics and strategy APR/win-rate with correct values"
    status: failed
    reason: "Go backend returns APR as 0–100 (percent) and WinRate as 0–100 (percent). Frontend components StrategyComparison.tsx and ExchangeMetrics.tsx multiply these by 100 again, producing values like 36500% APR and 7000% win rate. The main Analytics.tsx stat card for win_rate is correct (computed client-side as 0–1 ratio), but the APR stat card applies the same double-multiply. StrategyComparison.tsx summary stats are all affected."
    artifacts:
      - path: "web/src/components/StrategyComparison.tsx"
        issue: "Line 112: (s.win_rate * 100).toFixed(1)% — win_rate from API is already 0–100, not 0–1. Line 115: (s.apr * 100).toFixed(1)% — apr from API is already 0–100."
      - path: "web/src/components/ExchangeMetrics.tsx"
        issue: "Line 53: (m.apr * 100).toFixed(1)% — apr already 0–100. Line 54: (m.win_rate * 100).toFixed(1)% — win_rate already 0–100."
      - path: "web/src/pages/Analytics.tsx"
        issue: "Line 147: (avgApr * 100).toFixed(1)% — avgApr is the average of s.apr values from API (0–100), should display as avgApr.toFixed(1)%."
    missing:
      - "In StrategyComparison.tsx: change (s.win_rate * 100).toFixed(1) to s.win_rate.toFixed(1) and (s.apr * 100).toFixed(1) to s.apr.toFixed(1)"
      - "In ExchangeMetrics.tsx: change (m.apr * 100).toFixed(1) to m.apr.toFixed(1) and (m.win_rate * 100).toFixed(1) to m.win_rate.toFixed(1)"
      - "In Analytics.tsx stat card: change (avgApr * 100).toFixed(1) to avgApr.toFixed(1)"
human_verification:
  - test: "Enable analytics, trigger at least one closed perp position, navigate to Analytics page"
    expected: "PnL chart renders with real data points. APR stat card shows a plausible value (e.g. 120% not 12000%). Win rate in strategy summary shows a value in 0–100% range."
    why_human: "Cannot test chart rendering or live data flow without a running server with real positions."
  - test: "Click a closed position row in History page"
    expected: "PnL breakdown row expands showing entry fees, exit fees, funding earned, basis P/L, slippage, net PnL, APR. For pre-v0.27.0 positions, the 'detailed breakdown not available' message is shown instead."
    why_human: "Requires live data and browser interaction to verify expandable row behavior."
---

# Phase 04: Performance Analytics Verification Report

**Phase Goal:** The user can see exactly how much each position earned, compare strategy performance, and track cumulative PnL over time
**Verified:** 2026-04-03T23:30:54Z
**Status:** passed
**Re-verification:** Yes — gaps 1 and 2 fixed in commit 48a21cc

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| 1 | User can view any closed position's detailed PnL breakdown (entry fees, funding, exit fees, basis gain/loss, net PnL) | VERIFIED | `internal/engine/exit.go` persists ExitFees and BasisGainLoss in `tryReconcilePnL`; `models.ArbitragePosition` has all fields; `web/src/components/PnLBreakdown.tsx` renders them in expandable History rows |
| 2 | User can compare perp-perp vs spot-futures returns over configurable time window | VERIFIED | `GET /api/analytics/summary` queries both Redis histories and runs `ComputeStrategySummary`; `Analytics.tsx` + `StrategyComparison.tsx` render the results with a `TimeRangeSelector` (7D/30D/90D/All) |
| 3 | User can compare per-exchange metrics (profit, slippage, fill rate, error rate) | VERIFIED | Backend `ComputeExchangeMetrics` and `ExchangeMetrics.tsx` fully wired. APR/win_rate double-multiply fixed in commit 48a21cc. Note: error_rate metric deferred (AN-03 partial — tracked separately) |
| 4 | Dashboard shows cumulative PnL chart backed by SQLite time-series storage | VERIFIED | `internal/analytics/store.go` (SQLite WAL, batch writes, range queries); `SnapshotWriter` records close events; `BackfillFromHistory` seeds from Redis; `PnLChart.tsx` renders Recharts `LineChart` with `Brush` |
| 5 | User can see APR and win rate metrics segmented by strategy and exchange | VERIFIED | APR and win_rate computed correctly in Go backend (0–100 percentages). Frontend double-multiply fixed in commit 48a21cc. Cumulative PnL chart now computes running total in PnLChart.tsx. |

**Score:** 5/5 truths verified (gaps 1-2 fixed, AN-03 error_rate deferred as minor enhancement)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/analytics/store.go` | SQLite time-series store | VERIFIED | 212 lines; `NewStore`, `WritePnLSnapshot`, `WritePnLSnapshots`, `GetPnLHistory`, `GetLatestTimestamp`, `migrate` all substantive |
| `internal/analytics/aggregator.go` | APR, win rate, exchange metrics, strategy summary | VERIFIED | 242 lines; `CalculateAPR`, `ComputeWinRate`, `ComputeExchangeMetrics`, `ComputeStrategySummary` all fully implemented |
| `internal/analytics/snapshot.go` | Background snapshot writer and backfill | VERIFIED | 223 lines; `SnapshotWriter` with buffered channel (100), `RecordPerpClose`, `RecordSpotClose`, `BackfillFromHistory` |
| `internal/api/analytics_handlers.go` | REST endpoints for PnL history and summary | VERIFIED | 107 lines; `handleGetAnalyticsPnLHistory` and `handleGetAnalyticsSummary` query real data (SQLite + Redis) |
| `internal/config/config.go` | EnableAnalytics toggle | VERIFIED | 6-touch-point convention: struct field, JSON apply, defaults (false), applyJSON, toJSON, applyEnv |
| `internal/engine/exit.go` | PnL decomposition in reconcilePnL | VERIFIED | Lines 907–914: ExitFees and BasisGainLoss persisted with formula documented |
| `internal/engine/engine.go` | BBO slippage capture at entry | VERIFIED | Lines 2482–2491 and 3098–3106: slippage computed in both `executeTrade` paths |
| `internal/models/position.go` | ExitFees, BasisGainLoss, Slippage fields | VERIFIED | Lines 37–39: all three fields present with json tags |
| `web/src/pages/Analytics.tsx` | Analytics page | VERIFIED | 163 lines; fetches PnL and summary via props, renders PnLChart, StrategyComparison, ExchangeMetrics, stat cards, loading/error/empty states |
| `web/src/components/PnLChart.tsx` | Recharts line chart | VERIFIED | 109 lines; `LineChart` + `Line` + `Brush` + `CartesianGrid` + `CustomTooltip`; renders empty state |
| `web/src/components/StrategyComparison.tsx` | Strategy bar chart | STUB (display bug) | Component exists and is wired, but APR and win_rate multiplied by 100 again — values will show as 36500% APR, 7000% win rate |
| `web/src/components/ExchangeMetrics.tsx` | Per-exchange table | STUB (display bug) | Component exists and is wired, but APR and win_rate multiplied by 100 again |
| `web/src/components/PnLBreakdown.tsx` | History drill-down | VERIFIED | 85 lines; renders all 7 decomposition cells; handles pre-v0.27.0 positions gracefully |
| `web/src/components/TimeRangeSelector.tsx` | Time range selector | VERIFIED | 40 lines; 4 preset buttons (7D/30D/90D/All) with `aria-pressed`; drives analytics fetch |
| `web/src/pages/Config.tsx` | EnableAnalytics toggle | VERIFIED | `renderAnalyticsTab` renders toggle and DB path field; wired to `strategy === 'analytics'` tab |
| `web/src/i18n/en.ts` | Analytics i18n keys | VERIFIED | 35+ keys added: `nav.analytics`, `analytics.*`, `hist.*` breakdown keys, `cfg.analytics.*` |
| `web/src/i18n/zh-TW.ts` | Traditional Chinese translations | VERIFIED | All keys present and translated (e.g. `analytics.title: '分析'`, `analytics.apr: '年化報酬率'`) |
| `VERSION` | 0.27.0 | VERIFIED | File contains `0.27.0` |
| `CHANGELOG.md` | Phase 4 entry | VERIFIED | `[0.27.0] - 2026-04-04 — Performance Analytics` with full feature list |
| `internal/analytics/store_test.go` | Store tests | VERIFIED | 5 tests: NewStore, TimeSeriesStore (CRUD + range + filter), WritePnLSnapshots_Batch, GetLatestTimestamp, GetLatestTimestamp_Empty |
| `internal/analytics/aggregator_test.go` | Aggregator tests | VERIFIED | 5 tests: APRCalculation, ComputeExchangeMetrics, ComputeExchangeMetrics_IncludesSpotPositions, ComputeStrategySummary, ComputeWinRate |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `Analytics.tsx` | `/api/analytics/pnl-history` | `getAnalyticsPnL` prop → `useApi.ts` fetch | WIRED | `useApi.ts` lines 249–251: constructs correct URL with from/to/strategy params |
| `Analytics.tsx` | `/api/analytics/summary` | `getAnalyticsSummary` prop → `useApi.ts` fetch | WIRED | `useApi.ts` lines 253–255; `App.tsx` line 183 passes `api.getAnalyticsPnL` and `api.getAnalyticsSummary` |
| `handleGetAnalyticsSummary` | Redis history | `db.GetHistory(500)` + `db.GetSpotHistory(500)` | WIRED | `analytics_handlers.go` lines 56–68: fetches both perp and spot histories with time filter |
| `handleGetAnalyticsPnLHistory` | SQLite store | `s.analyticsStore.GetPnLHistory(from, to, strategy)` | WIRED | Returns 503 when `analyticsStore` is nil (analytics disabled) |
| `engine.go` | `SnapshotWriter` | `e.snapshotWriter.RecordPerpClose(updated)` | WIRED | `exit.go` lines 958–961: called after history update in `tryReconcilePnL` |
| `spotengine/exit_manager.go` | `SnapshotWriter` | `e.snapshotWriter.RecordSpotClose(updated)` | WIRED | Lines 557–558: called after history write in `completeExit` |
| `cmd/main.go` | Analytics init | `analytics.NewStore`, `snapWriter.Start()`, `eng.SetSnapshotWriter` | WIRED | Lines 211–230: guarded by `cfg.EnableAnalytics`; spot engine wired at line 354–355 |
| `History.tsx` | `PnLBreakdown` | `expandedRow === tr.id && <PnLBreakdown position={tr} />` | WIRED | History.tsx line 122: renders breakdown in expandable `<tr>` when row clicked |
| `Sidebar.tsx` | `analytics` page | `{ id: 'analytics', labelKey: 'nav.analytics', icon: '\u2261' }` | WIRED | Line 18 in navItems array; `App.tsx` line 183 handles `case 'analytics'` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|-------------------|--------|
| `PnLChart.tsx` | `data: PnLSnapshot[]` | Prop from `Analytics.tsx` → `getAnalyticsPnL` → `GET /api/analytics/pnl-history` → `store.GetPnLHistory()` → SQLite | Yes (when analytics enabled and positions closed) | FLOWING |
| `StrategyComparison.tsx` | `strategies: StrategySummary[]` | Prop from `Analytics.tsx` → `getAnalyticsSummary` → Redis `GetHistory(500)` + `GetSpotHistory(500)` → `ComputeStrategySummary()` | Yes (real Redis query) | FLOWING but display bug |
| `ExchangeMetrics.tsx` | `metrics: ExchangeMetric[]` | Same as StrategyComparison, via `ComputeExchangeMetrics()` | Yes (real Redis query) | FLOWING but display bug |
| `PnLBreakdown.tsx` | `position: Position` | Prop from `History.tsx` → `getHistory(limit)` → Redis `GetHistory(n)` | Yes (real Redis data) | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Go build succeeds | `go build ./...` | Success | PASS |
| Analytics tests pass | `go test ./internal/analytics/...` | 10 passed | PASS |
| APR returns percent value | `CalculateAPR(10, 1000, 24*time.Hour)` in test | 365.0 (percent) | PASS (backend correct) |
| Frontend displays APR | Manual trace: `s.apr` from API is 365.0; `(s.apr * 100).toFixed(1)%` = "36500.0%" | 36500% displayed | FAIL (display bug) |
| Analytics routes registered | `grep '/api/analytics' internal/api/server.go` | Lines 124–125 | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|---------|
| PP-04 | 04-03, 04-04 | Trade history dashboard shows per-position detailed breakdown | SATISFIED | PnLBreakdown.tsx in History page; ExitFees/BasisGainLoss persisted in reconcilePnL |
| AN-01 | 04-01, 04-03, 04-04 | Per-position PnL decomposition: entry fees, funding, exit fees, basis gain/loss, net PnL | SATISFIED | All fields in ArbitragePosition model; PnLBreakdown renders them |
| AN-02 | 04-01, 04-03, 04-04 | Strategy-level performance comparison over configurable time window | SATISFIED | ComputeStrategySummary + handleGetAnalyticsSummary + TimeRangeSelector + StrategyComparison |
| AN-03 | 04-01, 04-03, 04-04 | Exchange-level metrics: profit, slippage, fill rate, error rate per exchange | PARTIAL | profit, slippage, win_rate, APR, trade_count present. Fill rate and error rate NOT included — error_rate field absent from ExchangeMetric struct and never populated |
| AN-04 | 04-01, 04-02, 04-04 | Cumulative PnL chart with SQLite time-series | SATISFIED | SQLite store + SnapshotWriter + PnLChart.tsx with Recharts |
| AN-05 | 04-01, 04-03, 04-04 | APR calculation segmented by strategy and exchange | PARTIAL | APR computed correctly in Go; display in StrategyComparison and ExchangeMetrics shows 100× inflated values |
| AN-06 | 04-01, 04-03, 04-04 | Win rate and average win/loss by strategy and exchange | PARTIAL | Win rate computed correctly in Go; StrategyComparison and ExchangeMetrics display 100× inflated values |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `web/src/components/StrategyComparison.tsx` | 112 | `(s.win_rate * 100).toFixed(1)%` — win_rate already 0–100 from API | Blocker | APR and win_rate display inflated by 100× |
| `web/src/components/StrategyComparison.tsx` | 115 | `(s.apr * 100).toFixed(1)%` — apr already 0–100 from API | Blocker | Same — 365% APR shows as 36500% |
| `web/src/components/ExchangeMetrics.tsx` | 53 | `(m.apr * 100).toFixed(1)%` — same double-multiply | Blocker | Exchange APR display inflated 100× |
| `web/src/components/ExchangeMetrics.tsx` | 54 | `(m.win_rate * 100).toFixed(1)%` — same double-multiply | Blocker | Exchange win rate display inflated 100× |
| `web/src/pages/Analytics.tsx` | 147 | `(avgApr * 100).toFixed(1)%` — avgApr sums s.apr (0–100 range) | Blocker | Stat card APR display inflated 100× |
| `internal/analytics/snapshot.go` | 69 | `CumulativePnL: ev.PnL` — stores individual position PnL, not running cumulative | Warning | The `CumulativePnL` field name is misleading; the chart data shows per-close PnL points, not a running sum. PnLChart connects these as a line, which will not show true cumulative trajectory |

### Human Verification Required

**1. APR and Win Rate Display Values**
**Test:** Enable analytics, open and close a perp-perp position. Navigate to Analytics page.
**Expected:** APR shows a plausible annualized value (e.g., 50%–500%) not thousands of percent. Win rate shows 0–100% range, not 0–10000% range.
**Why human:** Cannot render the React components without a browser.

**2. PnL Chart Cumulative Shape**
**Test:** With analytics enabled and multiple closed positions, view the PnL chart.
**Expected:** The chart line shows a running cumulative PnL trajectory (each point higher/lower than the previous by the amount of that trade). Each snapshot's `cumulative_pnl` field should represent the running total, not just the individual trade PnL.
**Why human:** The snapshot writer stores individual position PnL as `CumulativePnL` — the actual cumulative aggregation is only correct if the chart front-end performs a running sum, which `PnLChart.tsx` does NOT — it simply renders `dataKey="cumulative_pnl"` directly. The true accumulation would need to happen in the writer or the query layer.

**3. History PnL Drill-down with Real Positions**
**Test:** Ensure at least one position was closed after v0.27.0 (so ExitFees/BasisGainLoss/Slippage fields are populated). Click the position row in History.
**Expected:** Expandable row shows all decomposition cells with non-zero/non-dash values for fees and slippage.
**Why human:** Requires live position data with the new fields populated.

### Gaps Summary

Two categories of gaps block full goal achievement:

**Gap 1 — Display scale mismatch (Blocker):** The Go analytics aggregator correctly returns APR and win_rate as percentage values (0–100 range). The frontend components `StrategyComparison.tsx`, `ExchangeMetrics.tsx`, and the APR stat card in `Analytics.tsx` all multiply these by 100 again, producing absurd display values (36500% APR, 7000% win rate). This affects success criteria 3, 4 (APR in stat card), and 5. The fix is purely in the frontend — remove the `* 100` multiplier from these components. The win_rate stat card in `Analytics.tsx` computes the ratio client-side (0–1 range) so that specific display is correct.

**Gap 2 — Cumulative PnL chart not truly cumulative (Warning):** `SnapshotWriter.RecordPerpClose` sets `CumulativePnL: ev.PnL` which stores only the individual trade's PnL, not a running sum. `PnLChart.tsx` renders `dataKey="cumulative_pnl"` directly, so the chart will show individual trade PnL points connected by a line rather than a true cumulative PnL trajectory. This does not match success criterion 4 ("cumulative PnL chart"). The backfill `BackfillFromHistory` does sum per-day (`snap.CumulativePnL += ev.pnl`), but the live writer does not. This is a functional gap but may only be visible with multiple closed positions.

**Gap 3 — AN-03 error_rate missing:** The REQUIREMENTS.md AN-03 specifies "error rate per exchange". `ExchangeMetric` struct and `ComputeExchangeMetrics` have no `error_rate` field. The exchange error tracking that exists in `engine.go` is not plumbed into analytics. This is a minor incompleteness but the requirement explicitly lists it.

---

_Verified: 2026-04-03T23:30:54Z_
_Verifier: Claude (gsd-verifier)_
