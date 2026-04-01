# Technology Stack

**Project:** Funding Rate Arbitrage - Unified Capital Allocation, Backtesting, Metrics, Paper Trading
**Researched:** 2026-04-01
**Mode:** Ecosystem research for milestone stack additions
**Overall confidence:** HIGH (existing stack is proven; additions are well-understood Go/React libraries)

## Current Stack (No Changes)

The foundation is solid and unchanged. These are documented for reference only.

| Technology | Version | Purpose |
|------------|---------|---------|
| Go | 1.26.0 | Backend, engine, API server |
| Redis | 7.4.1 (server) | Real-time state, locks, positions, config |
| go-redis/v9 | 9.18.0 | Redis client |
| gorilla/websocket | 1.5.3 | Exchange streams + dashboard WS |
| React | 19.2.0 | Dashboard SPA |
| Vite | 7.3.1 | Frontend build |
| Tailwind CSS | 4.2.1 | Styling |
| TypeScript | ~5.9.3 | Type safety |
| Node.js | v22+ | Frontend build only (nvm) |

## Recommended Additions

### 1. Analytics Storage: SQLite via ncruces/go-sqlite3

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `github.com/ncruces/go-sqlite3` | v0.33+ | Persistent analytics, trade history, backtesting data | CGo-free (like modernc), but 314% faster reads in benchmarks. Pure Go via Wasm/wazero -- no C compiler needed, cross-compiles cleanly. database/sql compatible. |
| `github.com/ncruces/go-sqlite3/driver` | (same module) | database/sql driver registration | Standard Go database/sql interface -- no ORM, no magic |

**Confidence:** HIGH. Verified via pkg.go.dev (v0.33.2 published 2026-03-29). Benchmarked against modernc.org/sqlite and mattn/go-sqlite3.

**Why ncruces over alternatives:**

| Driver | CGo? | Performance | Cross-compile | Notes |
|--------|------|-------------|---------------|-------|
| `ncruces/go-sqlite3` | No (Wasm) | Fast (best CGo-free) | Easy | **Recommended.** Active development, modern approach |
| `modernc.org/sqlite` | No (transpiled) | Slower (39% slower reads) | Easy | Viable fallback. Larger dependency tree |
| `mattn/go-sqlite3` | Yes | Fastest | Requires C toolchain | Problematic for CI/CD, Alpine, cross-compile |

**What SQLite stores (not Redis):**
- `trade_history` -- closed positions with full PnL breakdown (replaces 1000-entry Redis cap)
- `daily_performance` -- aggregated daily PnL by strategy/exchange
- `funding_snapshots` -- historical funding rate observations for backtesting
- `backtest_results` -- saved simulation runs with parameter configs
- `paper_trades` -- virtual position history for paper trading mode

**What stays in Redis:**
- Active positions, locks, real-time state, config, balances -- all existing keys unchanged

### 2. Dashboard Charting: Recharts + Lightweight Charts

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `recharts` | ^3.8.1 | General charts (PnL curves, bar charts, allocation pie) | React-native SVG components. Declarative API. 3.8.1 is current (published 2026-03-26). Largest React charting community. |
| `lightweight-charts` | ^5.1.0 | Financial time-series charts (funding rate history, spread visualization) | TradingView's open-source library. Canvas-based (handles thousands of data points). Purpose-built for financial data. 5.1.0 is current. |

**Confidence:** HIGH. Verified versions via npm registry. Both libraries are actively maintained, widely used, and have no dependency on compromised packages.

**Why two charting libraries:**
- **Recharts** excels at standard dashboard charts: bar charts for PnL comparison, area charts for cumulative returns, pie charts for allocation breakdown. React-idiomatic, easy to style with Tailwind.
- **Lightweight Charts** excels at financial time-series: funding rate history with crosshair, spread visualization over time, backtesting equity curves. Canvas rendering handles dense tick data that SVG-based Recharts would choke on.

**Do NOT use these alternatives:**

| Library | Why Not |
|---------|---------|
| D3.js | Low-level imperative API -- massive overhead for standard chart types |
| Chart.js | Not React-native (wrapper needed), less maintainable than Recharts |
| ApexCharts | Heavy bundle size (~500KB), most features unneeded |
| ECharts | Chinese-first documentation, heavy, WebGL overkill for this scale |
| Grafana | External service, separate deployment, wrong tool for embedded SPA |

**npm lockdown note:** Both recharts and lightweight-charts must be added to `web/package-lock.json` via a controlled update. Neither depends on axios. Verify with `npm ls axios` after lockfile update. Use `npm ci` exclusively after update.

### 3. Statistics / Math: gonum/stat

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `gonum.org/v1/gonum` | v0.17.0 | Statistical computations for backtesting and risk metrics | Standard Go numerics library. Provides Sharpe ratio calculation, standard deviation, correlation, linear regression (for trend analysis). Published 2025-12-29. |

**Confidence:** HIGH. Verified on pkg.go.dev. Gonum is the de facto Go numerics library with no real competitor.

**Specific subpackages needed:**
- `gonum.org/v1/gonum/stat` -- Mean, StdDev, Correlation, descriptive statistics
- `gonum.org/v1/gonum/stat/distuv` -- Distribution functions (for risk modeling)
- `gonum.org/v1/gonum/floats` -- Float slice operations (sum, scale, etc.)

**Do NOT use:**
- Custom math implementations -- gonum is battle-tested, avoid reinventing stat functions
- Python/NumPy via subprocess -- unnecessary complexity, Go-native is sufficient for these calculations

### 4. Metrics Instrumentation: Custom In-Process (NOT Prometheus)

**Recommendation: Do NOT add Prometheus.**

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| Custom Go structs + SQLite | N/A | Trading performance metrics | Single-server, single-user system. Prometheus adds an external scraper, separate storage, and a Grafana instance for visualization -- three new services for metrics the dashboard already displays. |

**Confidence:** HIGH. This is an architectural decision based on the deployment model.

**Rationale:**
- The system runs on ONE server with ONE user viewing ONE dashboard
- Prometheus is designed for multi-service, multi-instance monitoring -- architecturally wrong here
- The dashboard already has WebSocket push for real-time state
- SQLite + dashboard charts cover all metrics use cases:
  - PnL over time (SQLite query -> Recharts line chart)
  - Win rate by strategy (SQLite aggregation -> Recharts bar chart)  
  - Funding earned per position (SQLite detail -> table component)
  - Capital utilization (Redis live state -> gauge component)

**If operational monitoring is later needed** (uptime, memory, goroutine count), expose a `/metrics` endpoint with `expvar` (Go stdlib) rather than Prometheus.

### 5. No New Dependencies for Paper Trading

Paper trading is an in-process simulation layer, not a separate system.

| Approach | Implementation | Why |
|----------|---------------|-----|
| Simulated order fills | Go interface wrapping existing `Exchange` interface | Same code path as live, with mock fills based on real BBO prices |
| Virtual positions | Redis keys with `paper:` prefix | Reuses existing position state management, isolated namespace |
| Virtual balance tracking | In-memory float64 ledger | No exchange API calls for balance, reset on demand |
| Dashboard integration | Existing WebSocket broadcast with `paper_` message types | No new frontend framework needed |

**Confidence:** HIGH. The existing `Exchange` interface and adapter pattern make this straightforward. The codebase already has `cmd/simtrade/` as a proof of concept.

### 6. No New Dependencies for Capital Allocation Engine

The capital allocator already exists (`internal/risk/allocator.go`) with Redis-backed reservations, strategy caps, exchange caps, and reconciliation. Enhancements are pure business logic.

| Enhancement | Implementation | Dependencies |
|-------------|---------------|-------------|
| Risk preference presets | Config struct additions + preset loader | None (Go stdlib) |
| Dynamic rebalancing | Periodic goroutine evaluating allocation vs. targets | None (Go stdlib + existing Redis) |
| Allocation optimizer | gonum/stat for correlation analysis between strategies | gonum (already recommended above) |
| Dashboard allocation view | New API endpoint + Recharts pie/bar | recharts (already recommended above) |

**Confidence:** HIGH. Reviewed existing allocator.go -- it already handles Reserve/Commit/Release/Reconcile lifecycle with Redis WATCH transactions.

### 7. Backtesting Engine: Custom + Loris Historical API

No external backtesting library exists for Go that fits this domain. Build custom.

| Component | Implementation | Dependencies |
|-----------|---------------|-------------|
| Historical data source | Loris Historical API (already integrated in `internal/discovery/backtest.go`) | None new -- HTTP client exists |
| Data cache | SQLite `funding_snapshots` table | ncruces/go-sqlite3 (above) |
| Simulation engine | Event-loop replaying funding intervals with configurable parameters | None -- pure Go business logic |
| Result storage | SQLite `backtest_results` table | ncruces/go-sqlite3 (above) |
| Equity curve visualization | Lightweight Charts on dashboard | lightweight-charts (above) |
| Statistical analysis | gonum for Sharpe ratio, max drawdown, win rate | gonum (above) |

**Confidence:** HIGH. The existing backtest.go already fetches and caches Loris historical data. The backtesting engine extends this with a simulation loop.

**Why NOT use an off-the-shelf backtesting framework:**
- No Go backtesting libraries exist at production quality (ecosystem is Python-dominated: Backtrader, Zipline, etc.)
- The domain is highly specific: funding rate arbitrage across exchange pairs, not traditional price-action trading
- The simulation only needs to replay funding intervals (every 1-8 hours), not tick-by-tick order books
- Existing code already has 70% of the foundation (Loris API client, opportunity model, risk checks)

## Full Dependency Addition Summary

### Go (go.mod additions)

```bash
# Analytics storage -- CGo-free SQLite
go get github.com/ncruces/go-sqlite3@latest

# Statistics for backtesting and risk analysis
go get gonum.org/v1/gonum@v0.17.0
```

**Total new Go dependencies: 2** (plus their transitive deps). The existing go.mod has only 3 direct dependencies -- this keeps the tree lean.

### Frontend (web/package.json additions)

```bash
# After controlled lockfile update (requires explicit approval per CLAUDE.md):
# General dashboard charts
# recharts@^3.8.1

# Financial time-series charts  
# lightweight-charts@^5.1.0
```

**Total new npm dependencies: 2.** Both are production dependencies (not devDependencies). Neither transitively depends on axios. Verify before committing lockfile.

**CRITICAL:** Per project rules, do NOT run `npm install`. Update `package.json` manually, then regenerate `package-lock.json` via a controlled, approved process.

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| SQLite driver | ncruces/go-sqlite3 | modernc.org/sqlite | 39% slower on reads. ncruces uses Wasm (wazero) which is actively developed and faster |
| SQLite driver | ncruces/go-sqlite3 | mattn/go-sqlite3 | Requires CGo/C compiler. Breaks cross-compilation. Current build chain is pure Go |
| Time-series DB | SQLite | Redis TimeSeries module | Redis 7.4.1 does not include TimeSeries (requires Redis 8+ or manual module install). go-redis v9 has TS commands built in, but useless without the server module. SQLite avoids this dependency |
| Time-series DB | SQLite | PostgreSQL | External service. Overkill for single-server, single-user. Adds operational burden |
| Time-series DB | SQLite | InfluxDB | Heavy external service. Domain needs SQL aggregations, not high-cardinality metrics |
| General charts | Recharts 3.x | Chart.js | Not React-native. Requires wrapper library. Recharts is more idiomatic for React 19 |
| Financial charts | Lightweight Charts 5.x | Recharts alone | Recharts uses SVG -- chokes on 1000+ data points. Lightweight Charts uses Canvas, purpose-built for financial time-series |
| Metrics system | Custom + SQLite | Prometheus + Grafana | Three external services for one user. Architecturally wrong. Dashboard already handles real-time display |
| Statistics | gonum | Custom math | gonum is the standard. Reimplementing Sharpe ratio / StdDev is error-prone and wasteful |
| Decimal precision | float64 (existing) | shopspring/decimal | Entire codebase uses float64 (15+ occurrences per model file). Migration would touch every file. Float64 precision is adequate for USDT amounts at the scale traded |
| Backtesting | Custom Go | Python (Backtrader/Zipline) | Different language. Subprocess overhead. The domain (funding rate replay) is simpler than price-action backtesting |
| Paper trading | In-process mock | Exchange testnet | Testnets have limited pairs, unreliable uptime, no real funding rates. Mock layer with real market data is superior |

## Redis TimeSeries: Why Not (Detailed)

The go-redis v9 library already includes full TimeSeries command support (`TSAdd`, `TSRange`, `TSMRange`, `TSCreateRule`, etc. -- verified in `timeseries_commands.go` on GitHub). However:

1. **Server module not installed:** Redis 7.4.1 on the production server does not include the TimeSeries module. It requires either upgrading to Redis 8+ (major version jump, risk to live trading) or manually compiling and loading the module (`loadmodule redistimeseries.so`).
2. **SQLite is simpler:** A single `.db` file alongside the binary. No module installation, no server restart, no compatibility concerns.
3. **SQLite queries are richer:** `SELECT SUM(pnl) FROM trades WHERE strategy='perp_perp' AND exchange='binance' GROUP BY DATE(closed_at)` -- this requires no application-side aggregation. Redis TimeSeries can aggregate by time window but not by arbitrary dimensions.
4. **Future option:** If Redis is upgraded to 8+ in the future, TimeSeries can complement SQLite for real-time streaming metrics while SQLite handles historical queries.

## Version Verification Summary

| Package | Claimed Version | Verification Source | Confidence |
|---------|----------------|---------------------|------------|
| ncruces/go-sqlite3 | v0.33.2 | pkg.go.dev (2026-03-29) | HIGH |
| gonum | v0.17.0 | pkg.go.dev (2025-12-29) | HIGH |
| recharts | 3.8.1 | npm registry (2026-03-26) | HIGH |
| lightweight-charts | 5.1.0 | npm registry (2026-01) | HIGH |
| Go | 1.26.0 | `go version` on server | HIGH (verified) |
| Redis | 7.4.1 | `redis-server --version` on server | HIGH (verified) |
| go-redis | 9.18.0 | go.mod in repo | HIGH (verified) |

## Architecture Fit

```
                    +-----------------+
                    |   Dashboard     |
                    |  (React + TS)   |
                    |  recharts +     |
                    |  lightweight    |
                    |  charts         |
                    +--------+--------+
                             |
                    HTTP/WS (go:embed)
                             |
              +--------------+--------------+
              |         Go Backend          |
              |  +--------+  +----------+   |
              |  | Engine |  | Backtest |   |
              |  | (live) |  | Engine   |   |
              |  +---+----+  +----+-----+   |
              |      |            |          |
              |  +---+----+  +---+------+   |
              |  | Paper  |  | gonum    |   |
              |  | Sim    |  | /stat    |   |
              |  +---+----+  +----------+   |
              |      |                      |
              |  +---+---+    +----------+  |
              |  |Capital|    | SQLite   |  |
              |  |Alloc  |    | (ncruces)|  |
              |  +---+---+    +----+-----+  |
              |      |             |         |
              +------+------+------+---------+
                     |             |
                +----+----+  +----+-------+
                |  Redis  |  | analytics  |
                |  7.4.1  |  | .db file   |
                |  (live  |  | (SQLite)   |
                |  state) |  |            |
                +---------+  +------------+
```

## Sources

- [ncruces/go-sqlite3 on pkg.go.dev](https://pkg.go.dev/github.com/ncruces/go-sqlite3) -- CGo-free SQLite driver (v0.33.2, 2026-03-29)
- [go-sqlite-bench benchmarks](https://github.com/cvilsmeier/go-sqlite-bench) -- Performance comparison of Go SQLite drivers
- [gonum on pkg.go.dev](https://pkg.go.dev/gonum.org/v1/gonum) -- Numeric/statistics library (v0.17.0, 2025-12-29)
- [Recharts releases on GitHub](https://github.com/recharts/recharts/releases) -- React charting library (v3.8.1)
- [Lightweight Charts on npm](https://www.npmjs.com/package/lightweight-charts) -- TradingView financial charts (v5.1.0)
- [go-redis TimeSeries commands](https://github.com/redis/go-redis/blob/master/timeseries_commands.go) -- Built-in TS support in go-redis v9
- [Redis TimeSeries docs](https://redis.io/docs/latest/develop/data-types/timeseries/) -- Module availability (Redis 8+ built-in)
- [Prometheus client_golang](https://github.com/prometheus/client_golang) -- v1.23.2, deliberately NOT recommended
- Server verification: `go version` (1.26.0), `redis-server --version` (7.4.1) -- verified on production host

---

*Stack research: 2026-04-01*
