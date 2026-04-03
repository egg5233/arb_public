# Phase 4: Performance Analytics - Research

**Researched:** 2026-04-03
**Domain:** Go backend (SQLite time-series, PnL decomposition), React frontend (Recharts charting, Analytics page)
**Confidence:** HIGH

## Summary

Phase 4 adds performance analytics to the arbitrage system: per-position PnL decomposition, strategy comparison, exchange-level metrics, cumulative PnL charting backed by SQLite, and APR/win-rate calculations. The phase spans three domains: (1) backend data model enrichment for perp-perp positions (adding exit fees, basis, slippage fields that spot-futures already has), (2) a new `internal/analytics` Go package with SQLite for time-series storage, and (3) a new Analytics dashboard page using Recharts plus an enhanced History page with drill-down.

The spot-futures position model (`SpotFuturesPosition`) already tracks `EntryFees`, `ExitFees`, `BorrowCostAccrued`, and funding collected -- this is the reference for perp-perp enrichment. The perp-perp `ArbitragePosition` model currently has `EntryFees` and `FundingCollected` but lacks `ExitFees`, `BasisGainLoss`, and `Slippage`. The `ClosePnL` struct returned by exchange APIs provides `Fees`, `Funding`, `PricePnL`, and `NetPnL` -- the reconciliation path (`reconcilePnL`) already decomposes these but discards the component breakdown when writing back. The fix is to persist decomposed components during reconciliation.

**Primary recommendation:** Use `modernc.org/sqlite` (pure Go, no CGO) for time-series storage alongside existing Redis. Add Recharts 3.8.1 to the frontend with `react-is` as an explicit peer dependency. Enrich the perp-perp position model to match spot-futures granularity, persist decomposition during PnL reconciliation, then build unified analytics API endpoints that query both Redis history and SQLite time-series.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** New dedicated "Analytics" page in sidebar nav for time-series charts, strategy comparison, and exchange metrics.
- **D-02:** Enhance existing History page with expandable per-position PnL breakdown (drill-down per row).
- **D-03:** Overview page keeps its existing stat cards as summary metrics -- no charts added to Overview.
- **D-04:** Full decomposition per closed position: entry fees, exit fees, funding earned, basis gain/loss, borrow cost (spot-futures only), slippage, net PnL. All components displayed in History drill-down.
- **D-05:** New tracking fields required on position models: exit fees, basis gain/loss, borrow cost (accumulated interest for spot-futures), slippage (BBO delta).
- **D-06:** Slippage estimated at execution time -- capture best bid/ask (BBO) snapshot at order placement, compare against actual fill price, store delta on position.
- **D-07:** Charting library: Recharts. Declarative React components, all chart types needed (line, area, bar), good React 19 + Tailwind compatibility.
- **D-08:** Interactive charts: tooltips on hover, time range presets (7d / 30d / 90d / all), click-to-zoom, brush selector for sub-range selection.
- **D-09:** npm lockfile update required to add Recharts -- follows existing npm security constraint (only `npm ci` after lockfile is updated).
- **D-10:** SQLite for time-series storage (from ROADMAP, user deferred explicit discussion).

### Claude's Discretion
- SQLite schema design and migration approach
- Specific position model field types and names for new tracking fields (exit fees, basis, borrow cost, slippage)
- Analytics page layout (chart placement, section ordering)
- Exchange metrics aggregation approach (AN-03)
- APR calculation formula details (AN-05)
- Win rate segmentation UI (AN-06)
- Strategy comparison chart type (AN-02) -- line, bar, or combined

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PP-04 | Trade history dashboard shows per-position detailed breakdown (entry/exit prices, fees, funding collected, hold time, PnL components) | Position model enrichment (exit fees, basis, slippage) + History drill-down UI |
| AN-01 | Per-position PnL decomposition: entry fees, funding earned, exit fees, basis gain/loss, net PnL | ClosePnL struct already provides components; persist during reconcilePnL |
| AN-02 | Strategy-level performance comparison: perp-perp vs spot-futures returns over configurable time window | SQLite time-series + unified analytics API aggregating both strategy histories |
| AN-03 | Exchange-level metrics: profit, slippage, fill rate, error rate per exchange | Aggregate from Redis history + new slippage/BBO tracking fields |
| AN-04 | Cumulative PnL chart over time with SQLite time-series storage | modernc.org/sqlite + periodic snapshot goroutine + Recharts LineChart |
| AN-05 | APR calculation for perp-perp positions (funding collected vs capital deployed vs hold time) | Formula: (funding / notional) * (8760 / holdHours) * 100 |
| AN-06 | Win rate and average win/loss segmented by strategy and exchange | Computed from Redis history data, grouped by exchange and strategy type |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `modernc.org/sqlite` | latest (SQLite 3.51.3) | Pure Go SQLite driver for time-series PnL data | No CGO required, cross-compilable, production-ready, works with `database/sql` |
| `recharts` | 3.8.1 | React charting (line, area, bar, brush) | Declarative React API, React 19 compatible, all required chart types, active maintenance |
| `react-is` | ^19.0.0 | Peer dependency for Recharts 3.x | Required peer dep; must match React version (19.x) |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `react` | ^19.2.0 | Already installed | Existing |
| `react-dom` | ^19.2.0 | Already installed | Existing |
| `github.com/redis/go-redis/v9` | v9.18.0 | Already installed -- history queries | Existing |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `modernc.org/sqlite` | `github.com/mattn/go-sqlite3` | CGO-based, faster but requires C toolchain, breaks cross-compilation |
| `modernc.org/sqlite` | `github.com/ncruces/go-sqlite3` | WASM-based pure Go, slightly faster in benchmarks but less mature |
| `recharts` | `visx` | Lower-level D3 wrapper -- more flexibility but significantly more code |
| `recharts` | `chart.js` via `react-chartjs-2` | Imperative canvas API, worse React integration |

**Go dependency installation:**
```bash
cd /var/solana/data/arb
go get modernc.org/sqlite
```

**Frontend dependency installation (requires user approval per npm lockdown):**
```bash
cd /var/solana/data/arb/web
# Step 1: Update package.json to add recharts and react-is
# Step 2: Regenerate lockfile (ONE-TIME controlled update, user must approve)
npm install --package-lock-only recharts@^3.8.1 react-is@^19.0.0
# Step 3: Clean install from updated lockfile
npm ci
```

**CRITICAL:** The npm security lockdown forbids `npm install` in normal workflow. Adding Recharts requires a controlled, user-approved lockfile update. The approach is: (1) add dependencies to package.json, (2) run `npm install --package-lock-only` to regenerate ONLY the lockfile without installing, (3) review lockfile diff for suspicious packages, (4) commit lockfile, (5) run `npm ci` for clean install. This must be a separate, auditable step.

## Architecture Patterns

### Recommended Project Structure
```
internal/
  analytics/
    store.go          # SQLite time-series store (init, write, query)
    store_test.go     # Tests using temp SQLite DB
    aggregator.go     # Compute exchange metrics, APR, win rate from history
    aggregator_test.go
  api/
    analytics_handlers.go  # New analytics API endpoints
  models/
    position.go       # Add ExitFees, BasisGainLoss, Slippage, BBOSnapshot fields
    spot_position.go  # Already has needed fields (reference model)
  engine/
    engine.go         # Hook BBO capture before PlaceOrder
    exit.go           # Persist decomposed PnL in reconcilePnL
web/src/
  pages/
    Analytics.tsx      # New page: cumulative PnL chart, strategy comparison, exchange metrics
    History.tsx        # Enhanced: expandable drill-down per row
  components/
    PnLChart.tsx       # Recharts cumulative PnL line chart with brush
    StrategyComparison.tsx  # Strategy comparison chart (bar + line)
    ExchangeMetrics.tsx     # Per-exchange metrics table + charts
    TimeRangeSelector.tsx   # 7d/30d/90d/all preset buttons
```

### Pattern 1: SQLite Time-Series Store
**What:** A dedicated `internal/analytics` package wrapping `database/sql` with SQLite for append-only time-series data.
**When to use:** For cumulative PnL snapshots that grow indefinitely and need range queries -- Redis lists are not suited for this.

```go
// internal/analytics/store.go
package analytics

import (
    "database/sql"
    _ "modernc.org/sqlite"
)

type Store struct {
    db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
    db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL")
    if err != nil {
        return nil, fmt.Errorf("open analytics db: %w", err)
    }
    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("ping analytics db: %w", err)
    }
    if err := migrate(db); err != nil {
        return nil, fmt.Errorf("migrate analytics db: %w", err)
    }
    return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
    _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS pnl_snapshots (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            timestamp INTEGER NOT NULL,          -- Unix seconds
            strategy TEXT NOT NULL,               -- "perp" or "spot"
            exchange TEXT NOT NULL DEFAULT '',     -- exchange name or "" for aggregate
            cumulative_pnl REAL NOT NULL,
            position_count INTEGER NOT NULL DEFAULT 0,
            win_count INTEGER NOT NULL DEFAULT 0,
            loss_count INTEGER NOT NULL DEFAULT 0,
            funding_total REAL NOT NULL DEFAULT 0,
            fees_total REAL NOT NULL DEFAULT 0
        );
        CREATE INDEX IF NOT EXISTS idx_pnl_snapshots_ts ON pnl_snapshots(timestamp);
        CREATE INDEX IF NOT EXISTS idx_pnl_snapshots_strategy_ts ON pnl_snapshots(strategy, timestamp);
    `)
    return err
}
```

### Pattern 2: PnL Decomposition on Close
**What:** Persist all PnL components (not just net) when a position closes.
**When to use:** During `reconcilePnL()` and spot-futures `finalizeExit()`.

The existing `reconcilePnL` in `exit.go:821-955` already extracts `longAgg.Fees`, `longAgg.Funding`, `longAgg.NetPnL`, `shortAgg.Fees`, `shortAgg.Funding`, `shortAgg.NetPnL` -- it just discards the decomposition. The fix:

```go
// In reconcilePnL, after computing components, persist them:
if err := e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
    if needsPnLUpdate {
        fresh.RealizedPnL = reconciledPnL
        fresh.ExitFees = totalFees  // NEW: persist decomposed fees
        fresh.BasisGainLoss = longAgg.PricePnL + shortAgg.PricePnL  // NEW: price-based P/L
    }
    // ... existing exit price updates
    return true
}); err != nil {
    // handle
}
```

### Pattern 3: BBO Snapshot at Order Placement
**What:** Capture best bid/ask from the price store immediately before each PlaceOrder call.
**When to use:** At trade execution time in both engines.

```go
// Before placing order:
bbo := e.priceStore.GetBBO(symbol)  // existing price store
pos.LongBBOBid = bbo.Bid
pos.LongBBOAsk = bbo.Ask
// After fill:
pos.LongSlippage = actualFillPrice - bbo.Ask  // for buy orders
```

### Pattern 4: Unified Analytics API
**What:** Single set of API endpoints serving both perp-perp and spot-futures data.
**When to use:** Dashboard analytics queries.

```
GET /api/analytics/pnl-history?from=UNIX&to=UNIX&strategy=perp|spot|all
GET /api/analytics/exchange-metrics?from=UNIX&to=UNIX
GET /api/analytics/summary?from=UNIX&to=UNIX
```

### Anti-Patterns to Avoid
- **Don't store time-series in Redis lists:** Redis LPush/LRange is O(n) for range queries and has no efficient time-range filtering. SQLite with indexed timestamps is the correct choice for analytics queries.
- **Don't compute analytics on the frontend:** All aggregation (APR, win rate, exchange metrics) should happen server-side. The frontend only displays pre-computed data.
- **Don't modify existing PnL calculation logic:** The existing `reconcilePnL` math is correct and live-tested. Only ADD field persistence alongside the existing writes -- never change the calculation formulas.
- **Don't block the engine goroutine with SQLite writes:** Use a buffered channel or goroutine to decouple time-series writes from the hot trading path.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Time-series storage | Custom Redis sorted set scheme | SQLite `pnl_snapshots` table | Range queries, aggregation, persistence across restarts, no memory pressure |
| Charting | Canvas/SVG drawing code | Recharts `<LineChart>`, `<BarChart>`, `<Brush>` | Declarative, responsive, tooltips, brush selection built-in |
| APR calculation | Complex annualization formula | Simple: `(pnl / notional) * (8760 / holdHours) * 100` | Standard funding rate APR formula, matches industry convention |
| PnL decomposition | Re-query exchanges for closed positions | Persist components at close time from existing `ClosePnL` struct | Exchange APIs have limited history windows (7-90 days); data must be captured at close time |

**Key insight:** The exchange `ClosePnL` API returns decomposed PnL (fees, funding, price PnL) but this data has retention limits. It MUST be captured and persisted at position close time -- retroactive decomposition will fail for old positions.

## Common Pitfalls

### Pitfall 1: npm Lockfile Contamination
**What goes wrong:** Running `npm install` to add Recharts pulls in the compromised axios transitive dependency.
**Why it happens:** The npm lockdown exists because a dependency in the existing tree was compromised.
**How to avoid:** Use `npm install --package-lock-only recharts@^3.8.1 react-is@^19.0.0` to update ONLY the lockfile, then audit the diff for any axios references before committing. Never run bare `npm install`.
**Warning signs:** Any lockfile change that includes `axios` or unknown packages.

### Pitfall 2: SQLite Concurrent Access from Multiple Goroutines
**What goes wrong:** SQLite "database is locked" errors when multiple goroutines write simultaneously.
**Why it happens:** SQLite defaults to exclusive locking; the bot has 10+ concurrent goroutines.
**How to avoid:** Use WAL mode (`_journal_mode=WAL`), set `_busy_timeout=5000`, use a single `*sql.DB` connection pool with `db.SetMaxOpenConns(1)` for writes. Reads can be concurrent with WAL mode.
**Warning signs:** "SQLITE_BUSY" errors in logs.

### Pitfall 3: Historical Data Gap for Existing Closed Positions
**What goes wrong:** Positions closed before Phase 4 deployment lack decomposed PnL fields -- they show zeros in the drill-down.
**Why it happens:** New fields (ExitFees, BasisGainLoss, Slippage) don't exist on historical data in Redis.
**How to avoid:** Handle gracefully in the UI -- show "N/A" or "-" for missing decomposition fields. Don't attempt retroactive re-computation (exchange APIs have limited history). Include a note like "Detailed breakdown available for positions closed after v0.27.0".
**Warning signs:** Zero values for all decomposition fields on old positions.

### Pitfall 4: Recharts react-is Peer Dependency
**What goes wrong:** Build fails or runtime errors due to react-is version mismatch with React 19.
**Why it happens:** Recharts 3.x declares `react-is` as a peer dependency matching `^19.0.0` but doesn't bundle it.
**How to avoid:** Explicitly install `react-is@^19.0.0` alongside `recharts@^3.8.1` in the lockfile update step.
**Warning signs:** "Module not found: react-is" or version conflict errors during `npm ci`.

### Pitfall 5: Blocking Engine Hot Path with Analytics Writes
**What goes wrong:** SQLite writes during position close add latency to the trading critical path.
**Why it happens:** SQLite fsync + WAL checkpoint can take 5-50ms.
**How to avoid:** Use a buffered channel for analytics writes, processed by a dedicated goroutine. Position model field persistence (ExitFees etc.) goes to Redis as before (fast); SQLite time-series snapshot is decoupled.
**Warning signs:** Increased exit latency, slower trade execution.

### Pitfall 6: i18n Key Sync
**What goes wrong:** New analytics strings appear as raw keys in the Chinese locale.
**Why it happens:** Developer adds keys to `en.ts` but forgets `zh-TW.ts`.
**How to avoid:** TypeScript type-safety on `TranslationKey` will catch missing keys at build time. Always add to both files simultaneously.
**Warning signs:** Type errors in `npm run build` when keys are missing from either locale file.

## Code Examples

### Example 1: Analytics API Handler Pattern
```go
// internal/api/analytics_handlers.go
func (s *Server) handleGetAnalyticsPnLHistory(w http.ResponseWriter, r *http.Request) {
    from, _ := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
    to, _ := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
    strategy := r.URL.Query().Get("strategy") // "perp", "spot", or "all"

    if to == 0 {
        to = time.Now().Unix()
    }
    if from == 0 {
        from = to - 30*86400 // default 30 days
    }

    snapshots, err := s.analyticsStore.GetPnLHistory(from, to, strategy)
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, Response{Error: "analytics query failed"})
        return
    }
    writeJSON(w, http.StatusOK, Response{OK: true, Data: snapshots})
}
```

### Example 2: Recharts Cumulative PnL Chart
```tsx
// web/src/components/PnLChart.tsx
import { LineChart, Line, XAxis, YAxis, Tooltip, Brush, ResponsiveContainer, CartesianGrid } from 'recharts';

interface PnLDataPoint {
  timestamp: number;
  cumulative_pnl: number;
  strategy: string;
}

const PnLChart: FC<{ data: PnLDataPoint[] }> = ({ data }) => (
  <ResponsiveContainer width="100%" height={400}>
    <LineChart data={data}>
      <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
      <XAxis
        dataKey="timestamp"
        tickFormatter={(ts) => new Date(ts * 1000).toLocaleDateString()}
        stroke="#9CA3AF"
      />
      <YAxis stroke="#9CA3AF" tickFormatter={(v) => `$${v.toFixed(0)}`} />
      <Tooltip
        contentStyle={{ backgroundColor: '#1F2937', border: '1px solid #374151' }}
        labelFormatter={(ts) => new Date(ts * 1000).toLocaleString()}
        formatter={(value: number) => [`$${value.toFixed(2)}`, 'PnL']}
      />
      <Line type="monotone" dataKey="cumulative_pnl" stroke="#34D399" dot={false} />
      <Brush dataKey="timestamp" height={30} stroke="#4B5563" />
    </LineChart>
  </ResponsiveContainer>
);
```

### Example 3: APR Calculation
```go
// internal/analytics/aggregator.go
func CalculateAPR(pnl, notional float64, holdDuration time.Duration) float64 {
    if notional <= 0 || holdDuration <= 0 {
        return 0
    }
    hoursHeld := holdDuration.Hours()
    if hoursHeld < 1 {
        hoursHeld = 1 // minimum 1 hour to avoid division issues
    }
    // Annualize: (pnl/notional) * (hours_per_year / hours_held) * 100
    return (pnl / notional) * (8760.0 / hoursHeld) * 100
}
```

### Example 4: History Drill-Down Row
```tsx
// In History.tsx, expand a row to show decomposition:
{expandedRow === tr.id && (
  <tr className="bg-gray-800/50">
    <td colSpan={15} className="px-4 py-3">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
        <div>
          <span className="text-gray-500">{t('hist.entryFees')}</span>
          <span className="ml-2 text-red-400">-${(tr.entry_fees ?? 0).toFixed(4)}</span>
        </div>
        <div>
          <span className="text-gray-500">{t('hist.exitFees')}</span>
          <span className="ml-2 text-red-400">-${(tr.exit_fees ?? 0).toFixed(4)}</span>
        </div>
        <div>
          <span className="text-gray-500">{t('hist.funding')}</span>
          <span className={`ml-2 ${tr.funding_collected >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            ${tr.funding_collected.toFixed(4)}
          </span>
        </div>
        <div>
          <span className="text-gray-500">{t('hist.basisGainLoss')}</span>
          <span className={`ml-2 ${(tr.basis_gain_loss ?? 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            ${(tr.basis_gain_loss ?? 0).toFixed(4)}
          </span>
        </div>
      </div>
    </td>
  </tr>
)}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| recharts 2.x | recharts 3.x | 2025 | Complete rewrite of internal state, removed react-smooth, better React 19 support |
| mattn/go-sqlite3 (CGO) | modernc.org/sqlite (pure Go) | 2023+ | No CGO dependency, cross-compilation, simpler build |
| Redis for all storage | Redis (hot state) + SQLite (analytics) | This phase | Appropriate storage for different access patterns |

**Deprecated/outdated:**
- `recharts 2.x`: Still maintained but 3.x is the active development branch. 2.x had `react-smooth` and `recharts-scale` as dependencies which are now internal in 3.x.

## Open Questions

1. **Backfill strategy for SQLite time-series**
   - What we know: When SQLite is first deployed, it will be empty. Redis `arb:history` and `arb:spot_history` contain historical closed positions.
   - What's unclear: Should we backfill SQLite from Redis history on first startup, or start fresh?
   - Recommendation: Backfill on first startup -- iterate Redis history, create daily PnL snapshots from position close timestamps. This gives immediate chart data. Mark backfilled entries distinctly.

2. **SQLite file location**
   - What we know: The bot runs as a systemd service from a working directory. Config is in `config.json`, logs in `logs/`.
   - What's unclear: Where should `analytics.db` live?
   - Recommendation: Same directory as binary, configurable via `ANALYTICS_DB_PATH` env var or config field. Default: `data/analytics.db`.

3. **Snapshot frequency for time-series**
   - What we know: Funding snapshots run hourly (:10). Positions can be held for hours to days.
   - What's unclear: How frequently to snapshot cumulative PnL.
   - Recommendation: Snapshot on every position close event AND hourly (piggyback on existing funding tracker timing). This gives high resolution during active trading and low overhead during quiet periods.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | Backend builds | Yes | 1.26.0 | -- |
| Node.js 22+ via nvm | Frontend builds | Yes | v22.13.0 (nvm) | Switch nvm: `nvm use 22` |
| Redis | State store | Yes | Running (DB 2) | -- |
| SQLite | Analytics store | N/A (pure Go library) | Will be embedded | -- |
| npm | Lockfile update | Yes | via nvm Node 22 | -- |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:**
- Default `node --version` shows v18.0.0, but v22.13.0 is available via nvm. Build scripts already handle this (`nvm use 22`).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` package |
| Config file | None (stdlib, no config needed) |
| Quick run command | `go test ./internal/analytics/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| AN-01 | PnL decomposition persisted on position close | unit | `go test ./internal/analytics/... -run TestPnLDecomposition -count=1` | Wave 0 |
| AN-02 | Strategy comparison aggregation correct | unit | `go test ./internal/analytics/... -run TestStrategyComparison -count=1` | Wave 0 |
| AN-03 | Exchange metrics aggregation correct | unit | `go test ./internal/analytics/... -run TestExchangeMetrics -count=1` | Wave 0 |
| AN-04 | SQLite time-series write and range query | unit | `go test ./internal/analytics/... -run TestTimeSeriesStore -count=1` | Wave 0 |
| AN-05 | APR calculation formula | unit | `go test ./internal/analytics/... -run TestAPRCalculation -count=1` | Wave 0 |
| AN-06 | Win rate segmentation by strategy/exchange | unit | `go test ./internal/analytics/... -run TestWinRateSegmentation -count=1` | Wave 0 |
| PP-04 | New position model fields serialize/deserialize correctly | unit | `go test ./internal/models/... -run TestPositionSerialization -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/analytics/... -count=1`
- **Per wave merge:** `go test ./... -count=1` (currently 283 tests, all passing)
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/analytics/store_test.go` -- covers AN-04 (SQLite time-series CRUD)
- [ ] `internal/analytics/aggregator_test.go` -- covers AN-01, AN-02, AN-03, AN-05, AN-06
- [ ] No frontend test framework exists -- frontend testing is manual/visual only (existing project constraint)

## Project Constraints (from CLAUDE.md)

1. **npm security lockdown:** DO NOT run `npm install`, `npm update`, `npm upgrade`, `npx`, or `pnpm install`. Only `npm ci`. Adding Recharts requires a controlled one-time lockfile update with user approval.
2. **Delegation mode:** Coordinator breaks tasks into subtasks and delegates to teammates (Sonnet4.6 or Opus4.6).
3. **Build order:** Frontend must build BEFORE Go binary (`npm run build` then `go build`).
4. **Every commit must update CHANGELOG.md and VERSION.**
5. **Live system:** Changes must not break existing perp-perp trading.
6. **i18n:** Both `en.ts` and `zh-TW.ts` must stay in sync for all new strings.
7. **Go code navigation:** Prefer LSP (gopls) over Grep/Glob.
8. **Config pattern:** New features need `Enable{Feature}` bool (default OFF) + dashboard toggle + 6 touch points.
9. **API response envelope:** `{ ok: boolean, data?: T, error?: string }` via `writeJSON()`.
10. **Exchange API docs:** Load `/local-api-docs` skill when code changes involve exchange APIs (relevant for BBO capture).

## Existing Data Assets Inventory

### What Already Exists (no work needed)
| Data | Location | Status |
|------|----------|--------|
| Perp-perp entry fees | `ArbitragePosition.EntryFees` | Tracked via `queryEntryFees()` |
| Perp-perp funding collected | `ArbitragePosition.FundingCollected` | Tracked via `trackFunding()` hourly |
| Perp-perp net PnL | `ArbitragePosition.RealizedPnL` | Reconciled via `reconcilePnL()` |
| Perp-perp rotation PnL | `ArbitragePosition.RotationPnL` | Captured during rotation |
| Spot-futures entry fees | `SpotFuturesPosition.EntryFees` | Calculated at execution |
| Spot-futures exit fees | `SpotFuturesPosition.ExitFees` | Calculated at exit |
| Spot-futures borrow cost | `SpotFuturesPosition.BorrowCostAccrued` | Updated by `monitorLoop` |
| Spot-futures funding | `SpotFuturesPosition.FundingCollected` | Tracked continuously |
| Stats (perp) | Redis `arb:stats` hash | total_pnl, trade_count, win_count, loss_count |
| Stats (spot) | Redis `arb:spot_stats` hash | total_pnl, trade_count, win_count, loss_count |
| History (perp) | Redis `arb:history` list | Last 500 closed positions |
| History (spot) | Redis `arb:spot_history` list | Last 500 closed positions |

### What Needs Adding (new fields)
| Data | Model | Source | Notes |
|------|-------|--------|-------|
| Perp exit fees | `ArbitragePosition.ExitFees` | From `reconcilePnL()` `totalFees` - already computed but not persisted separately | Easy -- just persist the value |
| Perp basis gain/loss | `ArbitragePosition.BasisGainLoss` | From `reconcilePnL()` `longAgg.PricePnL + shortAgg.PricePnL` | Price-based P/L excluding funding and fees |
| Perp slippage (BBO) | `ArbitragePosition.Slippage` | New: BBO snapshot at order time vs actual fill | Requires hooking into `PlaceOrder` call sites |
| Perp BBO snapshot | `ArbitragePosition.LongBBO` / `ShortBBO` | Price store `GetBBO()` before order | Store bid/ask at order placement |
| SQLite time-series | New `analytics.db` | Computed from position close events | Append-only, periodic snapshots |

### Key Insight: reconcilePnL Already Has the Data
The `reconcilePnL` function at `exit.go:821-955` already computes:
- `totalFees` = `longAgg.Fees + shortAgg.Fees` (line 871)
- `reconciledFunding` = `longAgg.Funding + shortAgg.Funding` (line 870)
- `longAgg.PricePnL` + `shortAgg.PricePnL` (available from ClosePnL struct)
- `reconciledPnL` = `longAgg.NetPnL + shortAgg.NetPnL + pos.RotationPnL` (line 869)

The only change needed: persist `totalFees` as `ExitFees` and compute `BasisGainLoss = reconciledPnL - reconciledFunding - pos.RotationPnL + totalFees` (i.e., the price-based component). This is a few lines of code in an existing function.

## Sources

### Primary (HIGH confidence)
- Codebase inspection: `internal/models/position.go`, `internal/models/spot_position.go` -- verified current field sets
- Codebase inspection: `internal/engine/exit.go:794-955` -- verified `reconcilePnL` already decomposes PnL components
- Codebase inspection: `pkg/exchange/types.go:200-210` -- verified `ClosePnL` struct fields
- Codebase inspection: `internal/database/state.go` -- verified Redis key patterns and history storage
- Codebase inspection: `web/src/pages/History.tsx` -- verified current History page structure
- Codebase inspection: `web/src/App.tsx` -- verified routing and page registration pattern
- Codebase inspection: `web/src/components/Sidebar.tsx` -- verified nav item registration
- [Recharts 3.8.1 package.json](https://github.com/recharts/recharts/blob/main/package.json) -- verified peer deps: react, react-dom, react-is all ^16.8.0 || ^17 || ^18 || ^19
- [Recharts 3.0 migration guide](https://github.com/recharts/recharts/wiki/3.0-migration-guide) -- verified breaking changes and removed deps

### Secondary (MEDIUM confidence)
- [modernc.org/sqlite docs](https://pkg.go.dev/modernc.org/sqlite) -- pure Go driver, SQLite 3.51.3, production-ready
- [Go SQLite best practices](https://jacob.gold/posts/go-sqlite-best-practices/) -- WAL mode, busy_timeout, connection pooling
- [SQLite time series patterns](https://dev.to/zanzythebar/building-high-performance-time-series-on-sqlite-with-go-uuidv7-sqlc-and-libsql-3ejb) -- INTEGER timestamps, batch inserts, index strategy

### Tertiary (LOW confidence)
- None -- all findings verified against codebase or official sources

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- modernc.org/sqlite is the standard pure-Go SQLite driver; Recharts is the user's locked decision with verified React 19 compatibility
- Architecture: HIGH -- patterns derived directly from existing codebase (extending reconcilePnL, following existing page/API/i18n patterns)
- Pitfalls: HIGH -- derived from project-specific constraints (npm lockdown, CGO avoidance, concurrent goroutines) and verified Recharts peer dependency requirements
- Data model: HIGH -- verified by reading actual Go source and existing field usage in both engines

**Research date:** 2026-04-03
**Valid until:** 2026-05-03 (stable domain, no fast-moving dependencies)
