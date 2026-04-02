# Architecture

**Analysis Date:** 2026-04-01

## Pattern Overview

**Overall:** Event-driven orchestration engine with scan-triggered execution pipeline

**Key Characteristics:**
- Two independent arbitrage engines (perp-perp and spot-futures) sharing exchange adapters
- Scan-driven execution model: fixed-minute scheduler fires discovery scans that dispatch to typed handlers (rebalance, exit, rotate, entry)
- Delta-neutral strategy: always paired long/short positions across exchanges
- Interface-driven dependency injection: `Discoverer`, `RiskChecker`, `StateStore` abstractions in `internal/models/interfaces.go`
- Single Go binary embeds React frontend via `go:embed`
- Redis DB 2 as sole persistence layer (state, locks, history, stats)

## Layers

**Exchange Adapters (`pkg/exchange/`):**
- Purpose: Unified API abstraction across 6 CEXes (Binance, Bybit, Gate.io, Bitget, OKX, BingX)
- Location: `pkg/exchange/`
- Contains: `Exchange` interface (35 methods) in `pkg/exchange/exchange.go`, shared types in `pkg/exchange/types.go`, per-exchange adapter packages
- Depends on: Nothing internal (standalone, importable)
- Used by: All internal packages (`engine`, `discovery`, `risk`, `spotengine`)
- Each adapter has: `adapter.go` (interface impl), `client.go` (REST), `ws.go` (public WS), `ws_private.go` (private WS), optional `margin.go`
- Optional `SpotMarginExchange` interface for borrow/repay (all except BingX)

**Discovery (`internal/discovery/`):**
- Purpose: Poll funding rate data, merge sources, verify rates, rank opportunities
- Location: `internal/discovery/`
- Contains: `scanner.go` (polling loop, schedule), `verifier.go` (4-check rate verification), `ranker.go` (profitability filter, scoring), `backtest.go` (historical validation), `delist.go` (Binance delist monitor)
- Depends on: `pkg/exchange`, `internal/database`, `internal/models`, `internal/config`
- Used by: `internal/engine` (consumes `ScanResult` from channel)
- Produces: `ScanResult{Opps []Opportunity, Type ScanType}` on a buffered channel

**Engine (`internal/engine/`):**
- Purpose: Main orchestration loop -- dispatches scan results to rebalance/exit/rotate/entry handlers
- Location: `internal/engine/`
- Contains: `engine.go` (main loop, trade execution, funding tracker, SL detection, 2883 lines), `exit.go` (exit strategy, depth-fill, rotation, PnL reconciliation, 2518 lines), `consolidate.go` (position reconciliation, 524 lines), `capital.go` (allocator helpers, 72 lines)
- Depends on: All internal packages (`discovery`, `risk`, `database`, `api`, `models`, `config`), `pkg/exchange`
- Used by: `cmd/main.go` (instantiated and started)
- Central state holder: tracks exit goroutines, entry guards, SL index, own-order tracking

**Risk (`internal/risk/`):**
- Purpose: Pre-trade approval (11-point check), active monitoring, margin health, capital allocation
- Location: `internal/risk/`
- Contains: `manager.go` (pre-trade approval, 598 lines), `monitor.go` (4-dimension active monitoring, 429 lines), `health.go` (L0-L5 margin health tiers, 469 lines), `limits.go` (hard constraints), `allocator.go` (cross-strategy capital), `spread_stability.go` (Redis-backed CV gate), `exchange_scorer.go` (health scoring), `liq_trend.go` (liquidation trend tracking)
- Depends on: `pkg/exchange`, `internal/database`, `internal/models`, `internal/config`
- Used by: `internal/engine`
- Emits: `HealthAction` on a channel consumed by the engine

**Spot-Futures Engine (`internal/spotengine/`):**
- Purpose: Independent engine for spot margin + futures hedge arbitrage
- Location: `internal/spotengine/`
- Contains: `engine.go` (lifecycle, discovery loop), `discovery.go` (CoinGlass reader, scoring), `execution.go` (entry/exit trades, 1678 lines), `exit_manager.go` (exit triggers, 622 lines), `monitor.go` (stuck exit retry, repay retry), `risk_gate.go` (pre-entry checks), `autoentry.go` (automated entry), `capital.go`, `rate_velocity.go`
- Depends on: `pkg/exchange`, `internal/database`, `internal/api`, `internal/models`, `internal/config`, `internal/risk`, `internal/notify`
- Used by: `cmd/main.go` (conditionally started if `SpotFuturesEnabled`)
- Fully independent from perp-perp engine: separate goroutines, separate Redis keys, separate API routes

**Database (`internal/database/`):**
- Purpose: Redis persistence layer for all state
- Location: `internal/database/`
- Contains: `redis.go` (connection), `state.go` (positions, history, funding, stats, transfers), `locks.go` (distributed locks with SET NX + TTL + Lua scripts), `spot_state.go` (spot-futures position CRUD)
- Depends on: `github.com/redis/go-redis/v9`, `internal/models`
- Used by: All internal packages
- Implements `models.StateStore` interface (compile-time verified)

**API/Dashboard (`internal/api/`):**
- Purpose: HTTP REST + WebSocket server for dashboard and manual controls
- Location: `internal/api/`
- Contains: `server.go` (HTTP mux, lifecycle), `handlers.go` (REST endpoints), `ws.go` (WebSocket hub), `auth.go` (session tokens), `frontend.go` (SPA embedding), `spot_handlers.go` (spot-futures endpoints), `drift_monitor.go` (binary staleness detection)
- Depends on: `internal/database`, `internal/config`, `internal/models`, `internal/risk`, `pkg/exchange`
- Used by: `cmd/main.go`, both engines broadcast via `api.Server`

**Models (`internal/models/`):**
- Purpose: Shared data structures and interface definitions
- Location: `internal/models/`
- Contains: `interfaces.go` (Discoverer, RiskChecker, StateStore), `opportunity.go`, `position.go`, `funding.go`, `spot_position.go`, `rejection.go`
- Depends on: Nothing internal
- Used by: All internal packages

**Config (`internal/config/`):**
- Purpose: Unified configuration from JSON + environment variables
- Location: `internal/config/config.go`
- Depends on: Standard library only
- Used by: All internal packages
- Load priority: defaults -> `config.json` -> env vars -> `EnsureScanMinutes()` auto-includes special minutes

**Notifications (`internal/notify/`):**
- Purpose: Telegram Bot API alerts for trade lifecycle events
- Location: `internal/notify/telegram.go`
- Used by: `internal/spotengine` only

**Scraper (`internal/scraper/`):**
- Purpose: Headless Chrome scraper for CoinGlass spot-futures data
- Location: `internal/scraper/spotarb.go`
- Depends on: `github.com/chromedp/chromedp`, `internal/database`
- Used by: `cmd/main.go` (conditionally started)

**Utilities (`pkg/utils/`):**
- Purpose: Math helpers, structured logging with daily rotation
- Location: `pkg/utils/`
- Contains: `math.go` (RoundToStep, EstimateSlippage, RateToBpsPerHour, GenerateID), `logging.go` (Logger with module context, file output, subscriber broadcast)
- Depends on: Standard library only
- Used by: All packages

## Data Flow

**Discovery to Execution Pipeline:**

1. `Scanner.Start()` fires at configured minutes each hour (`internal/discovery/scanner.go:153`)
2. Scanner polls Loris API + reads CoinGlass Redis key, merges, deduplicates
3. Verifier checks each opportunity against exchange-native APIs (direction, interval, magnitude, spread, cap)
4. Ranker applies profitability filter, composite scoring, persistence filter
5. Scanner sends `ScanResult{Opps, Type}` on `oppChan` channel
6. `Engine.run()` receives from channel, dispatches by `ScanType` (`internal/engine/engine.go:675`)
   - `NormalScan` -> broadcast to dashboard
   - `RebalanceScan` -> `rebalanceFunds()`
   - `ExitScan` -> `checkIntervalChanges()` + `checkExitsV2()`
   - `RotateScan` -> `checkRotations()`
   - `EntryScan` -> `executeArbitrage(opps)`

**Trade Execution Flow:**

1. `executeArbitrage()` acquires capacity lock (`internal/engine/engine.go:1328`)
2. Phase 1 (sequential): iterate opportunities, acquire per-symbol Redis lock, run 11-point risk approval via `risk.ApproveWithReserved()`
3. Reserve pending position slot in Redis before releasing capacity mutex
4. Phase 2 (parallel): launch approved candidates as goroutines via `sync.WaitGroup`
5. Each `executeTradeV2()` goroutine: set leverage/margin, subscribe WS depth, run depth-fill loop (100ms tick, IOC orders), activate position

**Position Lifecycle:**
```
pending -> partial -> active -> exiting -> closed
```

**State Management:**
- All position state persisted to Redis (DB 2) via `internal/database/state.go`
- Distributed locks via Redis SET NX + TTL + Lua scripts (`internal/database/locks.go`)
- In-memory tracking for exit goroutines (`exitActive`, `exitCancels`, `exitDone` maps in Engine)
- `entryActive` map prevents consolidator from interfering with in-progress depth fills
- `ownOrders` sync.Map tracks engine-placed orders to avoid false SL triggers

**Spot-Futures Data Flow:**

1. `internal/scraper/spotarb.go` scrapes CoinGlass via headless Chrome, writes to Redis key `coinGlassSpotArb`
2. `SpotEngine.discoveryLoop()` reads from Redis, queries live borrow rates, scores opportunities
3. `attemptAutoEntries()` runs risk gate checks, enters positions if `auto_enabled`
4. `monitorLoop()` checks exit triggers (spread, yield, price spike, margin health) on interval
5. Separate Redis keys (`spot:positions`, `spot:history`, `spot:stats`)

## Key Abstractions

**Exchange Interface:**
- Purpose: Unified 35-method interface for all CEX operations
- Definition: `pkg/exchange/exchange.go`
- Implementations: `pkg/exchange/{binance,bybit,gateio,bitget,okx,bingx}/adapter.go`
- Pattern: Each adapter normalizes exchange-specific symbol formats, response structures, and WebSocket protocols

**SpotMarginExchange Interface:**
- Purpose: Optional spot margin operations (borrow, repay, margin orders)
- Definition: `pkg/exchange/exchange.go` (below `Exchange` interface)
- Implementations: All except BingX
- Pattern: Type assertion `exch.(exchange.SpotMarginExchange)` to check support

**Discoverer/RiskChecker/StateStore:**
- Purpose: Decouple engine from concrete implementations
- Definition: `internal/models/interfaces.go`
- Pattern: Compile-time interface satisfaction checks (`var _ models.Discoverer = (*Scanner)(nil)`)

**ScanResult:**
- Purpose: Typed scan result carrying opportunities + scan classification
- Definition: `internal/discovery/scanner.go:44`
- Pattern: Channel-based communication from Scanner to Engine

## Entry Points

**Main Binary (`cmd/main.go`):**
- Location: `cmd/main.go`
- Triggers: Direct execution or systemd service
- Responsibilities: Wire all components, start in order (Scanner -> RiskMon -> HealthMon -> API -> Engine -> SpotEngine), handle graceful shutdown on SIGINT/SIGTERM
- Exchange factory: `newExchange()` switch at `cmd/main.go:343` (avoids import cycles)

**CLI Tools:**
- `cmd/livetest/main.go` - Live exchange API test suite (22 tests per exchange)
- `cmd/simtrade/main.go` - Trade simulation CLI (dry run depth analysis + live execution)
- `cmd/balance/main.go` - Balance check CLI
- `cmd/transfer/main.go` - Fund transfer CLI
- `cmd/fundmove/main.go` - Spot-to-futures transfer CLI
- `cmd/spotarb/main.go` - Standalone CoinGlass scraper (`--cron`, `--json`, `--no-redis`)
- `cmd/validate-docs/main.go` - API documentation validator
- `cmd/wstest/main.go` - WebSocket connection tester
- `cmd/closepos/main.go` - Manual position close CLI
- `cmd/gatetest/main.go` - Gate.io-specific test CLI

**Dashboard:**
- HTTP: `internal/api/server.go` serves REST API and embedded React SPA
- WebSocket: `/ws` endpoint for real-time updates
- Frontend: `web/src/App.tsx` (React SPA with page-based routing)

## Concurrency Model

**Goroutine Architecture:**
- Main engine loop: `Engine.run()` -- single goroutine consuming from `oppChan`
- Scanner: single goroutine with timer-based scan schedule
- Per-trade execution: `sync.WaitGroup` launches parallel goroutines for approved candidates
- Exit goroutines: spawned per-position with `context.Context` for cancellation by L4/L5 handlers
- Consolidator: single goroutine on 5-minute ticker
- Health monitor: single goroutine on 30-second ticker
- Risk monitor: single goroutine on configurable interval (default 300s)
- Funding tracker: single goroutine, runs hourly at HH:10:00 UTC
- SL fill consumer: single goroutine reading from buffered channel
- Balance refresh: single goroutine on 60-second ticker
- Spot-futures engine: 2 goroutines (discoveryLoop + monitorLoop)
- API server: goroutine for HTTP listener + WebSocket hub
- Binary drift monitor: background goroutine in API server

**Synchronization Primitives:**
- `sync.Mutex`: `exitMu` (exit goroutine tracking), `entryMu` (entry tracking), `capacityMu` (global capacity), `pnlReconcileMu` (PnL reconciliation serialization), `preSettleMu` (timer dedup)
- `sync.RWMutex`: `slIndexMu` (SL order index), scanner's `mu` (opportunity cache), `intervalsMu` (interval data)
- `sync.Map`: `ownOrders` (engine-placed order IDs), `posMu` in SpotEngine (per-position locks)
- `sync.WaitGroup`: parallel trade execution, SpotEngine lifecycle
- `context.Context`: exit goroutine cancellation
- Redis distributed locks: per-symbol `SET NX + TTL` with Lua-based release (`internal/database/locks.go`)
- Channels: `oppChan` (Scanner -> Engine), `slFillCh` (WS callbacks -> SL consumer), `actionCh` (HealthMonitor -> Engine), `alertCh` (RiskMonitor -> Engine)

**Race Protection Patterns:**
- Consolidator skips positions with `exitActive[posID]` or `entryActive["exchange:symbol"]` set
- `capacityMu` serializes concurrent manual open and automated entry
- Per-symbol Redis locks prevent duplicate execution
- Exit goroutine preemption: L4/L5 health actions cancel running exit via stored `CancelFunc`
- `UpdatePositionFields` atomic read-modify-write with predicate function

## Error Handling

**Strategy:** Log-and-continue for non-critical errors; escalate to emergency close for critical failures

**Patterns:**
- **Exchange API errors**: Logged with exchange name and endpoint context. Stop-loss failures never block entry/exit/rotation
- **Depth fill failures**: Circuit breaker after 5 consecutive `PlaceOrder` failures on either exchange. Partial fills are trimmed to matched portion
- **Exit fallback**: Depth-fill timeout -> market IOC for remainder (must close fully)
- **Emergency escalation**: L5 margin health triggers `handleEmergencyClose()` which preempts any running exit goroutine (500ms grace period)
- **SL fill detection**: Dual-method detection (slIndex lookup + ReduceOnly fill verification) to catch exchange-side stop triggers even when order IDs change
- **Retry patterns**: SpotEngine uses `retryLeg()` with configurable attempts and backoff. PnL reconciliation retries 3 times at 5s/15s/30s
- **Margin errors**: `isMarginError()` helper detects "insufficient", "not enough", "margin available" across exchange error message formats (`internal/engine/engine.go:24`)
- **Consolidator**: BingX legs require 3 consecutive misses before acting (guards against transient empty-position API responses)

## Startup and Shutdown

**Startup Sequence (`cmd/main.go`):**
1. Load config (JSON + env overrides + `EnsureScanMinutes`)
2. Connect to Redis (DB 2)
3. Initialize exchange adapters (require min 2, factory at `cmd/main.go:343`)
4. Detect Gate.io unified account mode
5. Check API key permissions on all exchanges
6. Load contract specs for all exchanges
7. Start WebSocket streams (public + private) on all exchanges
8. Initialize Scanner, CapitalAllocator, ExchangeScorer, RiskManager, RiskMonitor, HealthMonitor, API Server, Engine
9. Wire rejection store, close/open handlers, event channels, balance refresh goroutine
10. Start components: Scanner -> DelistMonitor -> RiskMon -> HealthMon -> API -> Engine -> SpotArbScraper -> SpotEngine
11. Block on SIGINT/SIGTERM

**Shutdown Sequence:**
1. SpotEngine.Stop() (waits for in-flight exits)
2. Engine.Stop()
3. API Server.Stop()
4. HealthMonitor.Stop()
5. RiskMonitor.Stop()
6. Scanner.Stop()
7. Close all exchange WebSocket connections
8. Close Redis connection
9. If drift-triggered: exit non-zero for systemd restart

## Scan Schedule

Default scan minutes: `[5, 15, 25, 35, 45, 55]` (auto-includes special minutes via `EnsureScanMinutes`)

| Minute | Scan Type | Handler |
|--------|-----------|---------|
| :05 | NormalScan | Dashboard broadcast only |
| :15 | NormalScan | Dashboard broadcast only |
| :20 | RebalanceScan | `rebalanceFunds()` |
| :25 | ExitScan | `checkIntervalChanges()` + `checkExitsV2()` |
| :35 | EntryScan | `executeArbitrage(opps)` |
| :45 | RotateScan | `checkRotations()` |
| :55 | NormalScan | Dashboard broadcast only |

All minutes are configurable via `ScanMinutes`, `RebalanceScanMinute`, `EntryScanMinute`, `ExitScanMinute`, `RotateScanMinute` in config.

## Risk Tiers

**Pre-Trade (11-Point Risk Approval, `internal/risk/manager.go:57`):**
1. Position count vs `MaxPositions`
2. Both exchanges exist in adapter map
3. Capital check + auto spot-to-futures transfer
4. Leverage clamp to hard cap (5x)
5. Orderbook non-empty on long exchange
6. Position size calculation
7. Margin per leg >= 10 USDT floor
8. Slippage check (binary search for max tradeable size if exceeds limit)
9. Cross-exchange price gap check (absolute hard cap + directional recovery limit)
10. Per-exchange capital exposure cap (60%) -- bypassed when allocator active
11. Cross-strategy capital allocator reserve (if enabled)

**Active Monitoring (5-Tier Margin Health, `internal/risk/health.go`):**

| Level | Threshold | Action |
|-------|-----------|--------|
| L0-None | No positions | -- |
| L1-Safe | PnL >= 0 | -- |
| L2-Low | PnL < 0, ratio < 0.50 | -- |
| L3-Medium | ratio >= 0.50 | Transfer funds from healthiest exchange |
| L4-High | ratio >= 0.80 | Reduce positions by 50% |
| L5-Critical | ratio >= 0.95 | Emergency close all |

---

*Architecture analysis: 2026-04-01*
