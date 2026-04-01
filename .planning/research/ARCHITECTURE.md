# Architecture Patterns

**Domain:** Multi-Strategy Crypto Arbitrage with Unified Capital Allocation
**Researched:** 2026-04-01
**Focus:** How should a multi-strategy system be structured? Component boundaries, data flow, build order.

## Current Architecture Assessment

The existing system is well-structured for its original scope (two independent engines, shared exchange adapters). The key architectural facts that constrain evolution:

1. **Two engines share exchange adapters but nothing else.** The perp-perp engine (`internal/engine/`) and spot-futures engine (`internal/spotengine/`) have completely separate discovery, state, monitoring, and execution. The only shared pieces are `pkg/exchange/` adapters, `internal/risk/allocator.go`, and `internal/database/`.

2. **A capital allocator already exists but is optional.** `risk.CapitalAllocator` uses Redis-backed Reserve/Commit/Release lifecycle with optimistic locking. It tracks exposure by strategy and by exchange. Both engines already integrate with it via thin wrapper methods (`capital.go` in each engine package). It is gated behind `EnableCapitalAllocator` config flag (default: OFF).

3. **The Exchange interface is the key abstraction boundary.** The 35-method `Exchange` interface in `pkg/exchange/exchange.go` plus the optional 7-method `SpotMarginExchange` interface is what every engine depends on. This is the natural interception point for paper trading, backtesting, and cross-exchange coordination.

4. **Redis is the only persistence layer.** All state (positions, history, funding snapshots, stats, locks, allocator state) lives in Redis DB 2. There is no time-series storage, no SQL database, no dedicated analytics store.

5. **The main binary wires everything.** `cmd/main.go` is the single composition root. It instantiates adapters, scanner, allocator, risk manager, health monitor, API server, engine, and spot engine in order. This is where mode switching (live vs paper) would be decided.

## Recommended Architecture: Layered Strategy Orchestration

The system needs to evolve from "two independent engines with optional shared caps" to "a strategy orchestrator that allocates capital across pluggable strategies." This section defines the target architecture and the incremental path to get there.

### Component Hierarchy (Target State)

```
cmd/main.go (composition root, mode selection)
  |
  +-- internal/config/          Config + Risk Profiles
  |
  +-- pkg/exchange/             Exchange adapters (real or simulated)
  |     +-- SimulatedExchange   (wraps real adapter for paper mode)
  |
  +-- internal/capital/         Unified Capital Allocation (promoted from risk/)
  |     +-- allocator.go        Reserve/Commit/Release lifecycle
  |     +-- budget.go           Per-strategy budget computation
  |     +-- rebalancer.go       Dynamic reallocation when strategies idle
  |
  +-- internal/risk/            Risk Management
  |     +-- manager.go          Pre-trade approval (11-point check)
  |     +-- health.go           Margin health tiers (L0-L5)
  |     +-- profiles.go         Named risk preference bundles
  |     +-- circuit_breaker.go  Per-exchange circuit state
  |
  +-- internal/engine/          Perp-Perp Strategy Engine
  +-- internal/spotengine/      Spot-Futures Strategy Engine (single-exchange)
  +-- internal/crossengine/     Cross-Exchange Spot-Futures Engine (future)
  |
  +-- internal/analytics/       Performance Tracking (SQLite)
  +-- internal/backtest/        Historical Replay Engine
  +-- internal/simulator/       Paper Trading Execution Layer
  |
  +-- internal/discovery/       Rate Scanning + Verification
  +-- internal/database/        Redis State Layer
  +-- internal/api/             Dashboard HTTP + WebSocket
  +-- internal/notify/          Telegram Alerts
```

### Component Boundaries and Responsibilities

| Component | Responsibility | Depends On | Used By |
|-----------|---------------|------------|---------|
| `internal/capital/` | Global capital budget, per-strategy allocation, reserve/commit/release, rebalancing | config, database, models | engine, spotengine, crossengine, risk |
| `internal/risk/profiles.go` | Named parameter bundles (conservative/balanced/aggressive), config overlay | config | api (profile selector), capital (sizing) |
| `internal/simulator/` | Wraps Exchange interface, simulates fills using live BBO + slippage model, tracks simulated balances | pkg/exchange | cmd/main.go (wiring only) |
| `internal/backtest/` | Replays historical funding rates through a time-stepped simulation, produces performance metrics | analytics, models, discovery (data format) | cmd/backtest (CLI), api (on-demand) |
| `internal/analytics/` | SQLite-backed trade recording, PnL decomposition, time-series aggregation | models | api (dashboard endpoints), backtest (result storage) |
| `internal/crossengine/` | Cross-exchange spot-futures: borrow on exchange A, hedge on exchange B, coordinate two margin pools | pkg/exchange, capital, database, risk | cmd/main.go |

### Data Flow Diagrams

**Capital Allocation Flow (unified):**

```
User sets risk profile via Dashboard
  |
  v
Config applies profile overrides
  |
  v
Capital Allocator computes per-strategy budgets:
  - Total USDT exposure cap
  - Perp-perp allocation % (e.g. 60%)
  - Spot-futures allocation % (e.g. 40%)
  - Per-exchange cap %
  |
  v
Engine requests capital: allocator.Reserve(strategy, exposures)
  |
  v
Allocator checks: total cap, strategy cap, exchange cap, existing reservations
  |
  +-- APPROVED: reservation created (5-min TTL)
  |     |
  |     v
  |   Engine executes trade
  |     |
  |     v
  |   allocator.Commit(reservation, posID, actualExposures)
  |     |
  |     v
  |   Position closed -> allocator.ReleasePosition(posID)
  |
  +-- DENIED: capital cap exceeded
        |
        v
      Engine skips opportunity, logs reason
```

**Paper Trading Flow:**

```
cmd/main.go detects PaperMode=true in config
  |
  v
For each exchange adapter:
  realAdapter := newExchange(name, cfg)
  exchanges[name] = simulator.Wrap(realAdapter, simulatorCfg)
  |
  v
All downstream components receive SimulatedExchange
  (they see the Exchange interface, don't know it's simulated)
  |
  v
SimulatedExchange behavior:
  - Market data methods: delegate to real adapter (live prices, BBO, funding rates)
  - Order methods: simulate fill at BBO + slippage, update simulated position state
  - Balance methods: return simulated balance (initial capital - margin used)
  - Position methods: return simulated positions from in-memory state
  |
  v
State isolation:
  - Paper positions use Redis key prefix "paper:" (separate namespace)
  - Paper history uses "paper:history:" prefix
  - Paper stats use "paper:stats:" prefix
  - Dashboard shows "[PAPER]" badge, can toggle live/paper view
```

**Backtesting Flow:**

```
Historical data source:
  Loris API /funding/historical (already used in backtest.go)
  + Local cache in SQLite (analytics store, avoid re-fetching)
  |
  v
Backtest runner (CLI or dashboard-triggered):
  1. Load historical funding rates for date range
  2. For each settlement timestamp (chronological):
     a. Construct synthetic ScanResult with opportunities at that timestamp
     b. Feed through discovery ranker (same logic as live)
     c. Feed through risk approval (same logic, simulated balances)
     d. Simulate entry at historical BBO (or mid-price + slippage estimate)
     e. Track simulated positions, apply funding at each settlement
     f. Check exit conditions using historical spread data
  3. Produce results: total PnL, per-position PnL, Sharpe ratio, max drawdown
  |
  v
Results stored in analytics SQLite for comparison:
  - Config snapshot (what parameters were used)
  - Trade-by-trade log
  - Summary metrics
  |
  v
Dashboard: backtest comparison page (run A vs run B)
```

**Cross-Exchange Spot-Futures Flow:**

```
Discovery finds: ETHUSDT borrow rate 2% on Bybit, funding rate -5% on Binance
  -> Opportunity: borrow ETH on Bybit, short ETH perpetual on Binance
  |
  v
Cross-engine validation:
  1. Check margin available on BOTH exchanges
  2. Check borrow capacity on source exchange
  3. Check futures margin on destination exchange
  4. Compute cross-exchange transfer cost (if needed) or confirm both funded
  |
  v
Execution (TWO-PHASE):
  Phase 1 (borrow side): MarginBorrow + MarginSell on exchange A
  Phase 2 (hedge side): Short perpetual on exchange B
  |
  v
Monitoring (dual-exchange):
  - Borrow APR on exchange A (rising APR erodes profit)
  - Margin health on exchange A (borrow margin ratio)
  - Margin health on exchange B (futures margin ratio)
  - Combined spread: borrow cost + funding income
  |
  v
Exit (coordinated):
  Phase 1: Close futures position on exchange B
  Phase 2: Buy back + repay loan on exchange A
  Order matters: close hedge FIRST to avoid unhedged exposure
```

## Key Architectural Decisions

### Decision 1: Promote Capital Allocator to Its Own Package

**Current state:** `internal/risk/allocator.go` (600 lines) lives inside the risk package.

**Recommendation:** Move to `internal/capital/` as a standalone package. The allocator is not a risk check -- it is a resource manager. Both engines depend on it, and future features (rebalancing, dynamic budget adjustment, cross-engine coordination) will grow it significantly.

**Why:** The risk package already has 8 files and clear responsibilities (pre-trade approval, health monitoring, spread stability). Capital allocation is a different concern. Keeping it in risk/ creates an increasingly misleading boundary.

**Migration path:** Rename `risk.CapitalAllocator` to `capital.Allocator`. Update imports in engine, spotengine, risk/manager.go, cmd/main.go. The Strategy type and Reserve/Commit/Release API stay identical.

**Build order implication:** This is a refactor-only change. Do it in Phase 1 before adding new capital allocation features so the package boundary is clean.

### Decision 2: Paper Trading via Exchange Interface Wrapping

**Recommendation:** Implement `simulator.SimulatedExchange` that wraps a real `exchange.Exchange`. Market data comes from the real adapter. Order execution is simulated.

**Why not engine-level branching:** The engine has 2883 lines of trade execution logic. Adding `if paper {}` branches throughout would be fragile, hard to test, and easy to get wrong. The interface wrapping approach means the engine code is identical in live and paper mode.

**State isolation strategy:** Paper mode uses Redis key prefixes (`paper:positions:`, `paper:history:`, etc.). The database package gains a `KeyPrefix` field set at initialization time. When running in paper mode, all state operations use the prefixed keys. Live state is untouched.

**What the simulator must model:**
- Fill simulation: use live BBO + configurable slippage
- Balance tracking: deduct margin on entry, return on exit
- Position tracking: maintain in-memory position state, sync to Redis with paper prefix
- Funding accrual: apply actual funding rates (from live feeds) to simulated positions
- Latency simulation: optional delay to model real execution time

**What the simulator should NOT model:**
- Exchange-specific quirks (partial fills, order book depth impact) -- too complex, low value for funding arb
- Network failures -- paper mode is for strategy validation, not infrastructure testing

### Decision 3: Backtesting as Replay Engine, Not Separate Strategy Code

**Recommendation:** The backtest engine replays historical data through the SAME discovery ranker and risk approval logic. It does NOT re-implement strategy logic.

**Why:** The existing `backtest.go` in discovery already fetches historical Loris data. The backtester should extend this pattern: construct synthetic `ScanResult` objects from historical data, feed them through the same pipeline the live engine uses (ranker, risk check), simulate execution, and accumulate results.

**Architecture:**
```go
// internal/backtest/runner.go
type Runner struct {
    ranker    *discovery.Ranker    // same ranking logic as live
    risk      models.RiskChecker   // same risk approval as live
    config    *config.Config       // config snapshot for this run
    analytics *analytics.Store     // where results go
}

func (r *Runner) Run(params BacktestParams) (*BacktestResult, error) {
    // Load historical data from Loris API / cached SQLite
    // Iterate timestamps chronologically
    // At each funding settlement:
    //   1. Rank opportunities using r.ranker
    //   2. Approve using r.risk (with simulated balances)
    //   3. Simulate fills
    //   4. Apply funding to held positions
    //   5. Check exit conditions
    // Compute aggregate metrics
}
```

**Data storage:** Historical rate data cached in SQLite (analytics store) to avoid repeated Loris API calls. Results stored in SQLite for comparison across runs.

**Build order implication:** Backtesting depends on analytics (SQLite store) and discovery (ranker). Build analytics first, then backtesting.

### Decision 4: Risk Profiles as Config Overlays

**Recommendation:** Risk profiles are named sets of config overrides stored as Go constants (for built-in profiles) with user-custom profiles in Redis.

**Why not separate config files:** Hot-switching profiles via dashboard should not require file I/O or binary restart. Profiles overlay the running config at the field level.

**Structure:**
```go
type RiskProfile struct {
    Name              string
    Description       string
    Overrides         map[string]interface{} // field name -> value
}

// Built-in profiles
var Conservative = RiskProfile{
    Name: "conservative",
    Overrides: map[string]interface{}{
        "MaxPositions":      3,
        "Leverage":          2,
        "MinEntrySpreadBps": 3.0,
        "MaxPerpPerpPct":    0.50,
        "MaxSpotFuturesPct": 0.30,
        "PositionSizeUSDT":  200,
    },
}
```

**Integration flow:**
1. User selects profile in dashboard
2. API handler calls `config.ApplyProfile(profileName)`
3. Config applies overrides to running config
4. Next scan cycle uses updated values (no restart needed)
5. Dashboard shows active profile badge

### Decision 5: Cross-Exchange Spot-Futures as Separate Engine

**Recommendation:** Cross-exchange spot-futures is a new package `internal/crossengine/`, NOT an extension of `internal/spotengine/`.

**Why separate:** Cross-exchange spot-futures is fundamentally different from single-exchange:
- **Two margin pools** instead of one (borrow margin on A, futures margin on B)
- **Coordinated execution** (must execute both legs, rollback if one fails)
- **Coordinated monitoring** (health checks on two exchanges simultaneously)
- **Coordinated exit** (must close hedge before closing borrow to avoid unhedged exposure)
- **Capital flows between exchanges** if rebalancing is needed

Adding this complexity to `spotengine/` (which is already 1678 lines in execution.go alone) would create a monolith. A separate engine keeps the single-exchange case clean.

**Shared code:** Both `spotengine` and `crossengine` will use the same `SpotMarginExchange` interface for borrow/repay operations. Common types and helpers can live in `internal/models/` or a new `internal/spotcommon/` package.

**Build order implication:** Cross-exchange depends on single-exchange being stable on all 5 exchanges first. This is a later phase.

## Component Interaction Map

```
                    +-----------+
                    |  Config   |
                    | (profiles)|
                    +-----+-----+
                          |
          +---------------+---------------+
          |               |               |
    +-----v-----+  +-----v-----+  +------v------+
    |  Capital   |  |   Risk    |  |  Simulator  |
    | Allocator  |  |  Manager  |  | (paper mode)|
    +-----+------+  +-----+-----+  +------+------+
          |               |               |
          |         +-----+-----+         |
          |         |           |         |
    +-----v---+ +---v---+ +----v----+    |
    |  Engine  | | Spot  | | Cross   |   |
    | (perp-   | |Engine | | Engine  |   |
    |  perp)   | |       | | (future)|   |
    +-----+----+ +---+---+ +----+----+   |
          |           |          |        |
          +-----+-----+----+----+        |
                |           |             |
          +-----v-----+ +--v---+    +----v----+
          | Exchange   | |Redis |    |Exchange |
          | Adapters   | |DB 2  |    |Adapters |
          | (real/sim) | +------+    |(wrapped)|
          +------------+             +---------+
                |
          +-----v-------+
          |  Analytics   |
          | (SQLite)     |
          +-----+--------+
                |
          +-----v-----+
          | Backtest   |
          | Runner     |
          +-----------+
```

**Arrow direction = "depends on" (data flows upward through the stack):**
- Engines depend on Capital Allocator for budget checks
- Engines depend on Risk Manager for trade approval
- Engines depend on Exchange Adapters for market data + execution
- Engines write to Redis (state) and Analytics (history)
- Backtest Runner depends on Analytics (data) and Discovery (ranking logic)
- Simulator wraps Exchange Adapters (same interface, different behavior)
- Capital Allocator depends on Config for limits and profiles

## Patterns to Follow

### Pattern 1: Interface-Based Mode Selection

**What:** Paper trading and live trading are selected at the composition root (cmd/main.go) by choosing which Exchange implementation to inject.

**When:** Always. This is the foundation of the multi-mode architecture.

**Example:**
```go
// cmd/main.go
for _, name := range cfg.EnabledExchanges() {
    exc, _ := newExchange(name, excCfg)
    if cfg.PaperMode {
        exchanges[name] = simulator.Wrap(exc, simulator.Config{
            InitialBalance: cfg.PaperInitialBalance,
            Slippage:       cfg.PaperSlippageBps,
        })
    } else {
        exchanges[name] = exc
    }
}
// Everything downstream is identical
```

### Pattern 2: Event-Sourced Analytics

**What:** Trade events (open, close, funding received, exit) are recorded as immutable events in SQLite. Aggregations are computed from events, not from mutable state.

**When:** All trade lifecycle transitions.

**Why:** Enables retrospective analysis, backtest comparison, and debugging. Redis state is mutable (positions update). SQLite events are append-only.

**Example:**
```go
type TradeEvent struct {
    Timestamp  time.Time
    EventType  string  // "open", "close", "funding", "partial_fill", "emergency_exit"
    Strategy   string  // "perp_perp", "spot_futures"
    PositionID string
    Symbol     string
    Exchanges  []string
    Details    json.RawMessage  // event-specific payload
}
```

### Pattern 3: Strategy Registration

**What:** Each engine registers itself with the capital allocator and the API server using a standard interface, rather than being wired with engine-specific knowledge.

**When:** When adding the third engine (crossengine), this pattern prevents the capital allocator from becoming a switch statement.

**Example:**
```go
type StrategyEngine interface {
    Name() string
    ActiveExposure() map[string]float64  // exchange -> USDT exposure
    CanAcceptCapital() bool
    Start()
    Stop()
}
```

This is aspirational for a future refactor. For the near term, the two-engine explicit wiring is fine. Add this interface when the third engine arrives.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Shared Mutable State Between Engines

**What:** Having engines read/write each other's position state or share in-memory maps.
**Why bad:** Race conditions, unclear ownership, debugging nightmare. The engines currently have clean separation -- preserve this.
**Instead:** Engines communicate only through the Capital Allocator (for budget) and Redis (for state queries via database package). Never import one engine into another.

### Anti-Pattern 2: Monolithic Backtester

**What:** Building a standalone backtesting binary that re-implements discovery, risk, and execution logic.
**Why bad:** Strategy logic drift -- backtester and live engine diverge over time, making backtest results unreliable.
**Instead:** Backtester reuses the same ranker, risk checker, and config as the live system. It only provides the data replay loop and simulated execution.

### Anti-Pattern 3: Cross-Exchange Spot-Futures as SpotEngine Extension

**What:** Adding cross-exchange logic to `internal/spotengine/execution.go` with "if crossExchange { ... }" branches.
**Why bad:** Single-exchange spot-futures is already complex (1678 lines). Cross-exchange doubles the state space (two margin pools, coordinated exit). Mixing them creates untestable code.
**Instead:** Separate `internal/crossengine/` package that shares types but has its own execution and monitoring logic.

### Anti-Pattern 4: Capital Allocator as Rate Limiter

**What:** Using the capital allocator to throttle trade frequency or enforce cooldowns.
**Why bad:** Capital allocation is about "how much money goes where." Rate limiting is about "how often can we trade." Mixing these concerns makes both harder to reason about.
**Instead:** Rate limiting stays in risk checks (existing cooldown logic). Capital allocator only answers "is there budget for this trade?"

## Suggested Build Order

The build order is driven by dependencies between components.

```
Phase 1: Foundation (no new engine behavior)
  1a. Analytics SQLite store (internal/analytics/)
  1b. Risk profiles (internal/config/profiles.go)
  1c. Circuit breaker (internal/risk/circuit_breaker.go)
  These three are independent of each other -- can be built in parallel.

Phase 2: Capital Promotion (refactor, no behavior change)
  2a. Move allocator to internal/capital/ (refactor)
  2b. Activate capital allocator for production (config change + testing)
  2c. Add rebalancing logic (shift idle capital between strategies)
  Depends on: Phase 1b (profiles feed capital limits)

Phase 3: Paper Trading (new capability)
  3a. SimulatedExchange wrapper (internal/simulator/)
  3b. Redis key prefixing for paper state isolation
  3c. Dashboard paper mode badge and toggle
  Depends on: Phase 1a (paper trades recorded in analytics)

Phase 4: Backtesting (new capability)
  4a. Historical data caching in SQLite
  4b. Backtest runner (internal/backtest/)
  4c. CLI for backtest runs (cmd/backtest/)
  4d. Dashboard backtest comparison page
  Depends on: Phase 1a (analytics store), Phase 3a (simulated execution reuse)

Phase 5: Cross-Exchange Spot-Futures (new engine)
  5a. Cross-engine types and shared helpers
  5b. Cross-engine execution (internal/crossengine/)
  5c. Coordinated monitoring and exit
  5d. Dashboard integration
  Depends on: Phase 2 (unified capital), spotengine stable on all 5 exchanges
```

**Critical path:** Analytics (1a) -> Paper Trading (3) -> Backtesting (4). This chain must be sequential because each builds on the previous.

**Parallel opportunities:**
- Risk profiles (1b) and circuit breaker (1c) can be built in parallel with analytics (1a)
- Capital refactor (2) can begin as soon as profiles (1b) are done
- Cross-exchange (5) is independent of paper/backtest but depends on capital being unified

## Scalability Considerations

| Concern | Current (5-10 positions) | At 20-30 positions | At 50+ positions |
|---------|--------------------------|---------------------|-------------------|
| Capital allocator Redis ops | 2-3 per trade entry/exit | 10-15 per cycle | Consider batching in pipeline |
| Analytics SQLite writes | Negligible | Index on strategy + date | WAL mode, async batch writes |
| Paper + Live state separation | Minimal overhead | Two Redis key namespaces | No issue (Redis handles millions of keys) |
| Backtest data volume | ~100 symbols x 30 days | ~200 symbols x 90 days | SQLite handles this easily |
| Cross-exchange coordination | N/A | 2-4 cross positions | Stagger entry to avoid rate limits |
| WebSocket connections | 12 (6 exchanges x pub+priv) | Same | Same (connections are per-exchange, not per-position) |

## Sources

- Codebase analysis: `internal/risk/allocator.go` (600 lines of Reserve/Commit/Release lifecycle) -- HIGH confidence
- Codebase analysis: `internal/engine/capital.go`, `internal/spotengine/capital.go` (allocator integration) -- HIGH confidence
- Codebase analysis: `internal/discovery/backtest.go` (Loris historical data fetching) -- HIGH confidence
- Codebase analysis: `cmd/main.go` (composition root wiring pattern) -- HIGH confidence
- [Backtesting framework for funding rate arbitrage](https://deepwiki.com/50shadesofgwei/funding-rate-arbitrage/5.1-backtesting-framework) -- MEDIUM confidence (different system, similar domain)
- [Algorithmic trading platform architecture](https://medium.com/@kaur.exe/designing-a-production-style-algorithmic-trading-platform-5dc326faacc8) -- MEDIUM confidence (5-layer architecture pattern)
- [Kelly Criterion for position sizing](https://blog.traderspost.io/article/kelly-criterion-position-sizing-automated-trading) -- MEDIUM confidence (well-established theory, needs adaptation for funding arb)
- [Paper trading simulation in Go](https://medium.com/@krishnan.priyanshu/simulating-a-paper-trading-platform-with-golang-c89a8f369803) -- LOW confidence (basic tutorial, not production architecture)
- [NautilusTrader deterministic event-driven architecture](https://github.com/nautechsystems/nautilus_trader) -- MEDIUM confidence (production-grade reference, different language)
- Exchange interface wrapping pattern for simulation -- HIGH confidence (standard Go idiom, already proven in the codebase's interface-driven design)

---

*Architecture research: 2026-04-01*
