# Project Research Summary

**Project:** Funding Rate Arbitrage -- Unified Capital Allocation, Backtesting, Metrics, Paper Trading
**Domain:** Multi-Strategy Crypto Arbitrage (Perp-Perp + Spot-Futures)
**Researched:** 2026-04-01
**Confidence:** HIGH

## Executive Summary

This milestone extends a live, production-proven funding rate arbitrage system (6 exchanges, two strategy engines) with four capabilities: unified capital allocation, performance analytics, backtesting, and paper trading. The existing codebase is well-structured -- two engines share exchange adapters via a clean 35-method interface, a capital allocator already exists with Redis-backed Reserve/Commit/Release lifecycle, and historical data fetching scaffolding is in place. The recommended approach builds incrementally on this foundation rather than introducing new frameworks or architectural paradigms.

The most critical prerequisite is completing the spot-futures engine across all 5 margin exchanges (currently only Bybit works end-to-end). The v0.22.44-v0.22.49 bug sequence proved that each exchange has unique margin API quirks that only surface during real execution. Until both engines are stable, analytics data is unreliable and capital allocation decisions are uninformed. After engine stabilization, the build order follows a clear dependency chain: analytics storage (SQLite) enables performance metrics, which inform capital allocation weights, which enable paper trading validation, which enables backtesting. Attempting to skip this chain -- particularly activating capital allocation without performance data -- is the most likely path to capital misallocation.

The stack additions are minimal and high-confidence: SQLite via ncruces/go-sqlite3 (CGo-free, fast) for persistent analytics, Recharts + Lightweight Charts for dashboard visualization, and gonum for statistical computations. No external services (Prometheus, PostgreSQL, InfluxDB) are needed -- the system is single-server, single-user. Paper trading is implemented via Exchange interface wrapping (the most important architectural pattern), not engine-level branching. Backtesting reuses live strategy logic via historical data replay, avoiding the common pitfall of divergent backtest/live codebases.

## Key Findings

### Recommended Stack

The existing stack (Go 1.26, Redis 7.4, React 19, Vite 7) is unchanged. Four additions are recommended, totaling 2 new Go dependencies and 2 new npm packages. The key decision is SQLite over Redis TimeSeries for analytics: Redis 7.4 does not include the TimeSeries module (requires 8+), and SQLite provides richer SQL aggregation for the queries this system needs ("PnL by strategy by exchange by date range"). Prometheus is explicitly rejected -- three external services for a single-user system is architectural mismatch.

**Core additions:**
- **ncruces/go-sqlite3 (v0.33+):** Persistent analytics, trade history, backtest data -- CGo-free via Wasm, 314% faster reads than modernc/sqlite, cross-compiles cleanly
- **gonum (v0.17.0):** Statistical computations (Sharpe ratio, standard deviation, correlation) -- de facto Go numerics library, no real alternative
- **Recharts (3.8.1):** General dashboard charts (PnL curves, allocation pie, bar charts) -- React-native, SVG-based, idiomatic with existing React 19 setup
- **Lightweight Charts (5.1.0):** Financial time-series (funding rate history, backtest equity curves) -- TradingView's Canvas-based library, handles dense tick data that SVG chokes on

**Critical note:** npm packages must be added via controlled lockfile update per project security policy. Neither depends on the compromised axios package.

### Expected Features

**Must have (table stakes):**
- Spot-futures full lifecycle on all 5 margin exchanges (currently Bybit only -- Priority 1 blocker)
- Auto-borrow/auto-repay working on all exchanges (exchange-native bots handle this transparently)
- Per-position PnL breakdown (funding earned, borrow cost, entry/exit basis, fees, net)
- APR calculation for perp-perp positions (spot-futures already has it)
- Circuit breaker for exchange-wide failures (identified as missing in CONCERNS.md)
- Telegram notifications for perp engine critical events (spot engine already has this)

**Should have (differentiators):**
- Unified capital allocation driving both strategies (infrastructure exists, needs activation with performance data)
- Risk preference profiles (conservative/balanced/aggressive as config overlays)
- Strategy-level performance comparison (perp-perp vs spot-futures)
- Cumulative PnL charts over time
- Paper trading mode via Exchange interface wrapping
- Historical funding rate replay (backtesting)

**Defer (v2+):**
- Cross-exchange spot-futures (borrow on exchange A, hedge on B) -- fundamentally different architecture, needs its own engine
- Dynamic capital rebalancing -- needs performance data that does not exist yet
- A/B config comparison -- useful but depends on paper trading infrastructure
- AI/ML rate prediction -- academic research shows funding rates are regime-shifting; simple thresholds outperform overfit models

### Architecture Approach

The system evolves from "two independent engines with optional shared caps" to "a layered strategy orchestration with unified capital management." The key architectural pattern is interface-based mode selection: paper trading is implemented by wrapping real Exchange adapters with a SimulatedExchange at the composition root (cmd/main.go), so all downstream engine code is identical in live and paper mode. The capital allocator should be promoted from `internal/risk/` to its own `internal/capital/` package, reflecting that it is a resource manager, not a risk check. Backtesting reuses live discovery ranker and risk approval logic via a replay loop, avoiding the anti-pattern of a monolithic backtester with divergent strategy code.

**Major components (new or promoted):**
1. **internal/analytics/** -- SQLite-backed trade event recording, PnL decomposition, time-series aggregation
2. **internal/capital/** -- Promoted from risk/allocator.go; adds per-strategy budget computation, risk profile integration, and future rebalancing
3. **internal/simulator/** -- Exchange interface wrapper for paper mode; uses live market data, simulates fills with configurable slippage
4. **internal/backtest/** -- Historical data replay through live ranker/risk logic; results stored in analytics SQLite
5. **internal/risk/profiles.go** -- Named risk preference bundles as config overlays, hot-switchable via dashboard
6. **internal/risk/circuit_breaker.go** -- Per-exchange circuit state (closed/half-open/open) based on consecutive API failures

### Critical Pitfalls

1. **Building analytics before both engines are stable** -- Analytics on unreliable spot-futures data leads to bad capital allocation decisions. Complete spot-futures on all 5 exchanges first, then build cross-strategy metrics.
2. **Activating capital allocator without performance data** -- Static 50/50 splits without feedback lock capital in underperforming strategies. Collect 2-4 weeks of per-strategy data before enabling dynamic allocation.
3. **Exchange API quirks in spot-futures expansion** -- v0.22.44-49 proved each exchange has unique margin behaviors. Per-exchange integration testing with small capital is mandatory; use 6-agent parallel audit pattern.
4. **Time-series storage bolt-on** -- Hacking PnL history into Redis lists breaks when data grows. SQLite decision must be made upfront, not retrofitted.
5. **Overfitting backtest parameters** -- Limited historical funding data + many parameters = classic overfitting. Use backtesting for sanity checks, not optimization. Walk-forward validation, half-Kelly max.

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Engine Stabilization and Operational Safety
**Rationale:** Both engines must be stable before any analytics or allocation work has value. The spot-futures engine only works on Bybit. The perp engine lacks Telegram alerts and circuit breakers. This is the foundation everything else depends on.
**Delivers:** Spot-futures full lifecycle on all 5 margin exchanges; circuit breaker for exchange failures; Telegram notifications for perp engine; healthcheck endpoint.
**Addresses:** Table stakes features (spot-futures lifecycle, auto-borrow/repay, circuit breaker, notifications)
**Avoids:** Pitfall 1 (analytics on unstable engine), Pitfall 3 (exchange API quirks -- mitigated by per-exchange testing)

### Phase 2: Analytics Foundation
**Rationale:** Performance data must exist before capital allocation can be informed. SQLite store, PnL decomposition, and dashboard charts are prerequisites for everything that follows.
**Delivers:** SQLite analytics store (trade events, daily performance); per-position PnL breakdown in dashboard; APR for perp-perp; strategy-level comparison; cumulative PnL charts.
**Uses:** ncruces/go-sqlite3, Recharts, Lightweight Charts
**Implements:** internal/analytics/ (event-sourced trade recording), dashboard chart components
**Avoids:** Pitfall 4 (time-series bolt-on -- storage designed upfront), basis drag masking (PnL decomposition explicitly breaks down all cost components)

### Phase 3: Capital Allocation Activation
**Rationale:** With 2-4 weeks of analytics data from Phase 2, the allocator can be activated with informed weights. Risk profiles bundle parameters into user-friendly presets. Daily loss limits add a safety net.
**Delivers:** Capital allocator promoted to internal/capital/ and activated for production; risk preference profiles (conservative/balanced/aggressive); daily/weekly loss limits; allocator dashboard with allocation visualization.
**Uses:** gonum (for correlation analysis between strategies), Recharts (allocation pie chart)
**Implements:** internal/capital/ (promoted), internal/risk/profiles.go, loss limit checks
**Avoids:** Pitfall 2 (allocator without performance data -- Phase 2 provides the data)

### Phase 4: Paper Trading
**Rationale:** Paper trading validates strategy changes before risking real capital. It depends on analytics (to record paper results) and benefits from the allocator (to simulate capital constraints). The Exchange interface wrapping pattern is the architectural centerpiece.
**Delivers:** SimulatedExchange wrapper with live market data and simulated fills; Redis key prefixing for state isolation; dashboard paper mode toggle with [PAPER] badge; paper trade history in analytics.
**Uses:** Existing Exchange interface, Redis prefix isolation
**Implements:** internal/simulator/ (Exchange wrapper), cmd/main.go mode selection
**Avoids:** Unrealistic paper simulation (uses live BBO + configurable slippage, deducts fees, applies real funding rates)

### Phase 5: Backtesting
**Rationale:** Backtesting requires analytics storage (Phase 2), benefits from paper trading's simulation patterns (Phase 4), and reuses live ranker/risk logic. It is the capstone that ties everything together.
**Delivers:** Historical funding rate replay engine; backtest CLI (cmd/backtest/); backtest comparison page in dashboard; statistical analysis (Sharpe ratio, max drawdown, win rate).
**Uses:** gonum/stat, Lightweight Charts (equity curves), SQLite (data cache + results)
**Implements:** internal/backtest/ (replay runner)
**Avoids:** Overfitting (walk-forward validation, sanity-check approach, not optimization)

### Phase 6: Cross-Exchange Spot-Futures (Future Milestone)
**Rationale:** Fundamentally different architecture (two margin pools, coordinated execution/exit). Requires stable single-exchange spot-futures and unified capital allocation. Separate engine, not an extension of spotengine.
**Delivers:** internal/crossengine/ with cross-exchange borrow-hedge execution.
**Implements:** New engine package with coordinated two-phase execution and dual-exchange monitoring.
**Avoids:** Monolithic spotengine with cross-exchange branches

### Phase Ordering Rationale

- **Dependency chain:** Engine stability (Phase 1) enables reliable analytics data (Phase 2), which informs capital allocation (Phase 3), which can be validated via paper trading (Phase 4), which shares simulation patterns with backtesting (Phase 5).
- **Grouping logic:** Phases 1-2 are about making the existing system production-complete. Phases 3-5 add new capabilities that build on each other. Phase 6 is a separate track that requires Phases 1-3 to be complete.
- **Pitfall avoidance:** The ordering directly addresses the two critical pitfalls -- analytics before engine stability and allocation before performance data. By forcing the dependency chain, the roadmap structurally prevents these mistakes.
- **Parallel opportunities within phases:** Phase 1 can parallelize circuit breaker, Telegram, and per-exchange spot-futures work. Phase 2 can parallelize SQLite store and dashboard charts. These do not need to be sequential.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 1 (Spot-Futures Expansion):** Each of the 4 remaining exchanges (Binance, Gate.io, Bitget, OKX) has unique margin API behaviors. Per-exchange API research is mandatory. Use `/local-api-docs` skill and 6-agent audit pattern.
- **Phase 4 (Paper Trading):** The SimulatedExchange fill model needs careful design -- how to model slippage, partial fills, and funding accrual timing. Review existing `cmd/simtrade/` proof of concept.
- **Phase 6 (Cross-Exchange Spot-Futures):** Novel architecture with no existing codebase pattern. Needs dedicated architecture research for coordinated two-phase execution and rollback.

Phases with standard patterns (skip deep research):
- **Phase 2 (Analytics):** SQLite integration, event sourcing, and dashboard charting are well-documented patterns. Stack decisions are already made.
- **Phase 3 (Capital Allocation):** The allocator infrastructure exists. This is activation + config overlay work, not greenfield architecture.
- **Phase 5 (Backtesting):** Architecture is defined (replay through live logic). Implementation is straightforward once Phases 2 and 4 are complete.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All versions verified against pkg.go.dev and npm registry within 3 days. Alternatives benchmarked with rationale. No speculative dependencies. |
| Features | MEDIUM-HIGH | Table stakes validated against 5 exchange-native bots (Binance, Bitget, Pionex, OKX, Gate.io). Differentiators assessed against TradesViz, CoinGlass, 3Commas. Some features (risk profiles, backtesting) lack direct comparable in funding arb domain. |
| Architecture | HIGH | Recommendations derived from direct codebase analysis (allocator.go, exchange interface, cmd/main.go composition root). Patterns (interface wrapping, event sourcing) are standard Go idioms already proven in the codebase. |
| Pitfalls | HIGH | Critical pitfalls validated by project history (v0.22.44-49 bug sequence, Binance PM revert). Moderate pitfalls sourced from academic research (ScienceDirect regime-shifting study) and industry guides. |

**Overall confidence:** HIGH

### Gaps to Address

- **Per-exchange margin API surface for spot-futures:** Only Bybit is proven. Binance, Gate.io, Bitget, and OKX each need dedicated API investigation during Phase 1 planning. The v0.22.44-49 history suggests 3-5 adapter bugs per exchange.
- **Paper trading fill model fidelity:** Research identifies the need for realistic slippage but does not specify exact parameters. Calibrate slippage model against real trade data from live engine during Phase 4.
- **Backtest data coverage:** Loris API historical data availability and granularity not verified beyond what backtest.go currently uses. Validate date range and resolution during Phase 5 planning.
- **Risk profile parameter interactions:** Parameters are interdependent (low leverage + high threshold = zero entries). Each profile needs validation against historical data before exposure to users.
- **npm lockfile update process:** Both charting libraries need controlled addition to package-lock.json. The exact process for safe lockfile update under the axios security lockdown needs to be defined before Phase 2 frontend work begins.

## Sources

### Primary (HIGH confidence)
- Codebase analysis: `internal/risk/allocator.go`, `internal/engine/`, `internal/spotengine/`, `cmd/main.go`, `internal/discovery/backtest.go` -- architecture and existing capability assessment
- Project history: v0.22.44-v0.22.49 bug sequence, CONCERNS.md, PROJECT.md, ARCHITECTURE.md -- pitfalls and operational experience
- [ncruces/go-sqlite3 on pkg.go.dev](https://pkg.go.dev/github.com/ncruces/go-sqlite3) (v0.33.2, 2026-03-29) -- SQLite driver selection
- [gonum on pkg.go.dev](https://pkg.go.dev/gonum.org/v1/gonum) (v0.17.0, 2025-12-29) -- statistics library
- [Binance Funding Rate Arbitrage Bot FAQ](https://www.binance.com/en/support/faq/f330e17d6fc04679b9b21d6f9350e787) -- feature expectations, spread control
- [Bitget Reverse Arbitrage Mode](https://www.bitget.com/news/detail/12560605130294) -- reverse carry trade expectations
- [ScienceDirect: Risk and Return Profiles of Funding Rate Arbitrage](https://www.sciencedirect.com/science/article/pii/S2096720925000818) -- regime-shifting rates, overfitting risk

### Secondary (MEDIUM confidence)
- [PixelPlex Crypto Arbitrage Bot Development 2026](https://pixelplex.io/blog/crypto-arbitrage-bot-development/) -- circuit breaker, notification, sizing best practices
- [3Commas Risk Management Guide](https://3commas.io/blog/ai-trading-bot-risk-management-guide) -- daily loss limits, paper trading recommendations
- [Amberdata Funding Rate Arbitrage Guide](https://blog.amberdata.io/the-ultimate-guide-to-funding-rate-arbitrage-amberdata) -- PnL decomposition methodology
- [Recharts GitHub releases](https://github.com/recharts/recharts/releases) (v3.8.1), [Lightweight Charts npm](https://www.npmjs.com/package/lightweight-charts) (v5.1.0) -- charting library selection
- [Pionex Spot-Futures Arbitrage Bot](https://www.pionex.com/blog/pionex-arbitrage-bot/) -- feature baseline for exchange-native bots

### Tertiary (LOW confidence)
- [Paper trading simulation in Go](https://medium.com/@krishnan.priyanshu/simulating-a-paper-trading-platform-with-golang-c89a8f369803) -- basic reference for fill simulation patterns
- [Kelly Criterion for position sizing](https://blog.traderspost.io/article/kelly-criterion-position-sizing-automated-trading) -- theoretical basis for half-Kelly recommendation

---
*Research completed: 2026-04-01*
*Ready for roadmap: yes*
