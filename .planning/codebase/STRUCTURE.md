# Codebase Structure

**Analysis Date:** 2026-04-01

## Directory Layout

```
arb/
├── cmd/                        # CLI entry points (main binary + utilities)
│   ├── main.go                 # Primary entry point -- wires all components
│   ├── balance/main.go         # Balance check CLI
│   ├── closepos/main.go        # Manual position close CLI
│   ├── fundmove/main.go        # Spot <-> futures transfer CLI
│   ├── gatetest/main.go        # Gate.io-specific test CLI
│   ├── livetest/main.go        # Live exchange API test suite (22 tests)
│   ├── simtrade/main.go        # Trade simulation CLI (dry run + live)
│   ├── spotarb/main.go         # Standalone CoinGlass scraper CLI
│   ├── testfunding/main.go     # Funding rate test CLI
│   ├── transfer/main.go        # Fund transfer CLI
│   ├── transfertest/main.go    # Transfer test CLI
│   ├── validate-docs/main.go   # API documentation validator
│   └── wstest/main.go          # WebSocket connection tester
├── internal/                   # Application-private packages
│   ├── api/                    # Dashboard HTTP + WebSocket server
│   ├── config/                 # Configuration loading (JSON + env)
│   ├── database/               # Redis persistence layer
│   ├── discovery/              # Funding rate polling, verification, ranking
│   ├── engine/                 # Core perp-perp arbitrage engine
│   ├── models/                 # Shared data structures and interfaces
│   ├── notify/                 # Telegram notification service
│   ├── risk/                   # Risk management (pre-trade, monitor, health)
│   ├── scraper/                # CoinGlass headless Chrome scraper
│   └── spotengine/             # Spot-futures arbitrage engine
├── pkg/                        # Reusable packages (importable externally)
│   ├── exchange/               # Unified exchange interface + 6 adapters
│   │   ├── exchange.go         # Exchange interface (35 methods)
│   │   ├── types.go            # Shared types (Order, Position, Balance, etc.)
│   │   ├── factory.go          # Placeholder (factory logic in cmd/main.go)
│   │   ├── binance/            # Binance adapter
│   │   ├── bitget/             # Bitget adapter
│   │   ├── bybit/              # Bybit adapter
│   │   ├── gateio/             # Gate.io adapter
│   │   ├── okx/                # OKX adapter
│   │   └── bingx/              # BingX adapter
│   └── utils/                  # Math helpers + structured logging
│       ├── math.go             # RoundToStep, EstimateSlippage, FormatPrice, etc.
│       └── logging.go          # Logger with module context, file rotation, subscriber broadcast
├── web/                        # React dashboard frontend
│   ├── embed.go                # go:embed directive for dist/
│   ├── src/                    # React source (Vite + Tailwind)
│   ├── dist/                   # Build output (embedded into Go binary)
│   ├── package.json            # Frontend dependencies
│   └── vite.config.ts          # Vite build config
├── doc/                        # Exchange API documentation
│   ├── .annotations/           # Curated API doc annotations
│   ├── binance/                # Binance API docs
│   ├── bybit/                  # Bybit API docs
│   ├── gateio/                 # Gate.io API docs
│   ├── bitget/                 # Bitget API docs
│   ├── okx/                    # OKX API docs
│   ├── bingx/                  # BingX API docs
│   └── exchange/               # Shared exchange reference
├── docs/                       # Project documentation
├── scripts/                    # Operational scripts
│   ├── pull-release.sh         # Release pull script
│   ├── query-bingx-pnl/       # BingX PnL query tool
│   └── query-bitget-pnl/      # Bitget PnL query tool
├── logs/                       # Runtime log files (daily rotation)
├── ARCHITECTURE.md             # Detailed system architecture documentation
├── CHANGELOG.md                # Version changelog (every commit updates this)
├── CLAUDE.md                   # Claude Code instructions
├── VERSION                     # Semver version file (e.g. "0.22.49")
├── Makefile                    # Build targets (build-frontend, build, dev, clean)
├── config.json                 # Runtime configuration (JSON)
├── go.mod                      # Go module definition
├── go.sum                      # Go dependency checksums
└── arb                         # Compiled binary
```

## Directory Purposes

**`cmd/`:**
- Purpose: All executable entry points
- Contains: One `main.go` per subdirectory, each a standalone CLI tool
- Key file: `cmd/main.go` -- the primary bot binary; wires all components, contains exchange factory function
- Pattern: Each subfolder is a self-contained `package main` with its own `main()` function

**`internal/engine/`:**
- Purpose: Core perp-perp arbitrage engine orchestrating the full trade lifecycle
- Contains: Main loop, trade execution (depth-fill V2), exit strategies, position consolidation, funding tracking, SL detection
- Key files:
  - `engine.go` (2883 lines) -- Engine struct, Start/Stop, run loop, executeArbitrage, executeTradeV2, rebalanceFunds, SL callbacks
  - `exit.go` (2518 lines) -- checkExitsV2, executeDepthExit, checkSpreadReversal, reducePosition, closePosition, checkRotations, rotateLeg, reconcilePnL
  - `consolidate.go` (524 lines) -- StartConsolidator, consolidatePositions
  - `capital.go` (72 lines) -- Capital allocator integration helpers

**`internal/discovery/`:**
- Purpose: Funding rate data acquisition, verification, and opportunity ranking
- Contains: Scan scheduling, Loris API polling, CoinGlass Redis reader, exchange-native verification, persistence filter, profitability filter
- Key files:
  - `scanner.go` (904 lines) -- Scanner struct, Start/Stop, scan loop, Poll (Loris), persistence history
  - `verifier.go` (286 lines) -- 4-check rate verification against exchange APIs
  - `ranker.go` (276 lines) -- Profitability filter, composite scoring, fee schedules
  - `backtest.go` -- Historical funding data validation
  - `delist.go` -- Binance delist announcement monitor

**`internal/risk/`:**
- Purpose: All risk management: pre-trade approval, active monitoring, margin health tiers, capital allocation
- Contains: 11-point risk check, 5-tier margin health system, spread stability gate, exchange health scoring, liquidation trend tracking
- Key files:
  - `manager.go` (598 lines) -- Approve(), 11-point risk check, position sizing, slippage binary search
  - `monitor.go` (429 lines) -- 4-dimension active position monitoring
  - `health.go` (469 lines) -- L0-L5 margin health tiers, HealthAction emission
  - `allocator.go` -- Cross-strategy capital allocator (Reserve -> Commit -> Release lifecycle)
  - `spread_stability.go` -- Redis-backed spread coefficient of variation gate
  - `exchange_scorer.go` -- Exchange health scoring (latency, uptime, fill rate)
  - `liq_trend.go` -- Liquidation distance trend tracking
  - `limits.go` -- Hard constraints (5x leverage, 60% exposure, 10 USDT floor)

**`internal/spotengine/`:**
- Purpose: Independent spot-futures arbitrage engine (runs alongside perp-perp)
- Contains: Discovery from CoinGlass, entry/exit with spot margin + futures hedge, borrow rate monitoring
- Key files:
  - `engine.go` (306 lines) -- SpotEngine struct, Start/Stop, discoveryLoop, monitorLoop
  - `discovery.go` -- CoinGlass reader, borrow rate queries, opportunity scoring
  - `execution.go` (1678 lines) -- Trade entry/exit with retryLeg, idempotency guards
  - `exit_manager.go` (622 lines) -- Exit triggers (spread, yield, price spike, margin health)
  - `monitor.go` -- Stuck exit retry, pending repay retry
  - `risk_gate.go` -- Pre-entry checks (capacity, cooldown, duplicate, persistence)
  - `autoentry.go` -- Automated entry from discovery loop
  - `rate_velocity.go` -- Borrow APR spike detection

**`internal/api/`:**
- Purpose: Dashboard HTTP/WebSocket server
- Contains: REST endpoints, WebSocket hub, session auth, frontend embedding, binary drift monitor
- Key files:
  - `server.go` -- HTTP mux setup, Start/Stop, CORS middleware
  - `handlers.go` -- REST endpoints (positions, history, config, exchanges, transfers, diagnose)
  - `spot_handlers.go` -- Spot-futures specific REST + WS endpoints
  - `ws.go` -- WebSocket Hub with broadcast, register/unregister
  - `auth.go` -- Session-based authentication (24h TTL tokens)
  - `frontend.go` -- SPA embedding via go:embed with fallback routing
  - `drift_monitor.go` -- Detects stale binary after deploy, triggers systemd restart

**`internal/database/`:**
- Purpose: Redis persistence layer (DB 2)
- Contains: Connection management, position CRUD, history, stats, distributed locks, spot-futures state
- Key files:
  - `redis.go` (60 lines) -- Connection setup, Ping, Close
  - `state.go` -- Position CRUD, history, funding snapshots, stats, transfers, cooldowns, spread history
  - `locks.go` -- Distributed locks with Lua scripts (OwnedLock with auto-renewal)
  - `spot_state.go` -- Spot-futures position/history/stats/persistence CRUD

**`internal/models/`:**
- Purpose: Shared data structures and interface definitions (dependency-free)
- Contains: Core types used across all packages
- Key files:
  - `interfaces.go` -- Discoverer, RiskChecker, StateStore abstractions
  - `opportunity.go` -- Opportunity struct (symbol, exchanges, rates, spread, score)
  - `position.go` -- ArbitragePosition struct, status constants (pending/partial/active/exiting/closed)
  - `funding.go` -- FundingRate, FundingSnapshot, LorisResponse, CoinGlassResponse
  - `spot_position.go` -- SpotFuturesPosition data model
  - `rejection.go` -- RejectionStore for dashboard display

**`internal/config/`:**
- Purpose: Unified configuration loading
- Contains: Config struct with all parameters, JSON + env override logic
- Key file: `config.go` -- Config struct, Load(), defaults, env overrides, EnsureScanMinutes
- Load order: defaults -> `config.json` -> environment variables

**`internal/notify/`:**
- Purpose: Telegram Bot API notifications for spot-futures trade events
- Key file: `telegram.go` -- TelegramNotifier for auto-entry, exit, emergency alerts

**`internal/scraper/`:**
- Purpose: Headless Chrome CoinGlass scraper for spot-futures opportunity data
- Key file: `spotarb.go` -- StartSpotArbScraper, chromedp-based scraping, writes to Redis

**`pkg/exchange/`:**
- Purpose: Reusable exchange adapter package (importable by external projects)
- Contains: Unified interface + 6 exchange implementations
- Key files:
  - `exchange.go` -- Exchange interface (35 methods), SpotMarginExchange interface (7 methods)
  - `types.go` -- Shared types (PlaceOrderParams, Order, Position, Balance, Orderbook, BBO, ContractInfo, etc.)
  - `factory.go` -- Placeholder (factory logic in `cmd/main.go` to avoid import cycles)

**`pkg/exchange/{exchange}/` (per adapter):**
- Standard files in each adapter:
  - `adapter.go` -- Exchange interface implementation (largest file, ~800-1000 lines)
  - `client.go` -- REST HTTP client with HMAC signing (~200-300 lines)
  - `ws.go` -- Public WebSocket (BBO, depth streams, ~200-300 lines)
  - `ws_private.go` -- Private WebSocket (order updates, SL fills, ~150-200 lines)
  - `margin.go` -- SpotMarginExchange implementation (optional, not on BingX)

**`pkg/utils/`:**
- Purpose: Shared utility functions
- Key files:
  - `math.go` -- RoundToStep, EstimateSlippage, RateToBpsPerHour, GenerateID, FormatPrice
  - `logging.go` -- Logger struct with module context, file output with daily rotation, subscriber broadcast for dashboard

**`web/src/`:**
- Purpose: React dashboard frontend (Vite + Tailwind CSS)
- Contains: Pages, hooks, components, i18n, types
- Key files:
  - `App.tsx` -- Main app component, page routing, update check
  - `main.tsx` -- React entry point
  - `types.ts` -- TypeScript type definitions
  - `hooks/useApi.ts` -- REST API client hook
  - `hooks/useWebSocket.ts` -- WebSocket connection hook
  - `components/Sidebar.tsx` -- Navigation sidebar
  - `pages/Overview.tsx` -- Dashboard overview
  - `pages/Positions.tsx` -- Active positions view
  - `pages/Opportunities.tsx` -- Discovery results
  - `pages/History.tsx` -- Closed position history
  - `pages/Config.tsx` -- Runtime config editor
  - `pages/Transfers.tsx` -- Fund transfer history
  - `pages/Logs.tsx` -- Live log viewer
  - `pages/Rejections.tsx` -- Trade rejection log
  - `pages/Permissions.tsx` -- API key permission display
  - `pages/Login.tsx` -- Authentication page
  - `i18n/` -- Internationalization (English + Traditional Chinese)
  - `utils/tradingUrl.tsx` -- Exchange trading URL generator

## Key File Locations

**Entry Points:**
- `cmd/main.go`: Primary bot binary entry point
- `web/src/main.tsx`: React frontend entry point

**Configuration:**
- `config.json`: Runtime JSON configuration (read at startup)
- `internal/config/config.go`: Config struct definition, Load() function
- `VERSION`: Semver version string
- `.env*` files: Environment variable overrides (exist but never read by tools)

**Core Logic:**
- `internal/engine/engine.go`: Main engine orchestration (run loop, execution, rebalancing)
- `internal/engine/exit.go`: Exit strategies, rotation, PnL reconciliation
- `internal/discovery/scanner.go`: Discovery scan scheduling and execution
- `internal/risk/manager.go`: Pre-trade risk approval
- `internal/spotengine/engine.go`: Spot-futures engine lifecycle

**Testing:**
- `internal/engine/engine_test.go`: Engine unit tests (effectiveAdvanceMin, classifyRotation)
- `internal/discovery/ranker_test.go`: Ranking tests
- `internal/risk/health_test.go`: Health level tests
- `internal/risk/spread_stability_test.go`: Spread stability gate tests
- `internal/risk/allocator_test.go`: Capital allocator tests
- `internal/risk/exchange_scorer_test.go`: Exchange health scoring tests
- `internal/risk/liq_trend_test.go`: Liquidation trend tests
- `internal/api/handlers_test.go`: API endpoint tests (semver comparison)
- `internal/api/config_handlers_test.go`: Config endpoint tests
- `internal/api/spot_handlers_test.go`: Spot handler tests
- `internal/api/spot_stats_test.go`: Spot stats cold-start regression tests
- `internal/spotengine/discovery_test.go`: Spot discovery tests
- `internal/spotengine/execution_test.go`: Spot execution tests
- `internal/spotengine/exit_triggers_test.go`: Spot exit trigger tests
- `internal/spotengine/risk_gate_test.go`: Spot risk gate tests
- `internal/spotengine/rate_velocity_test.go`: Rate velocity detector tests
- `internal/database/locks_test.go`: Distributed lock tests
- `internal/config/config_test.go`: Config loading tests
- `internal/models/opportunity_test.go`: Opportunity model tests
- `internal/notify/telegram_test.go`: Telegram notifier tests
- `pkg/exchange/gateio/adapter_test.go`: Gate.io adapter tests
- `pkg/exchange/okx/adapter_test.go`: OKX adapter tests
- `pkg/exchange/bitget/client_sign_test.go`: Bitget HMAC signing tests
- `pkg/exchange/bybit/margin_test.go`: Bybit margin tests

**Build:**
- `Makefile`: Build targets (`build-frontend`, `build`, `dev`, `clean`)
- `web/vite.config.ts`: Vite build configuration
- `web/embed.go`: `go:embed all:dist` directive

## Naming Conventions

**Files:**
- Go files: `snake_case.go` (e.g., `spot_state.go`, `drift_monitor.go`, `ws_private.go`)
- Test files: `{name}_test.go` co-located with source
- React pages: `PascalCase.tsx` (e.g., `Positions.tsx`, `Overview.tsx`)
- React hooks: `camelCase.ts` (e.g., `useApi.ts`, `useWebSocket.ts`)
- React components: `PascalCase.tsx` (e.g., `Sidebar.tsx`, `StatusBadge.tsx`)

**Directories:**
- Go packages: `lowercase` single word (e.g., `engine`, `discovery`, `spotengine`, `gateio`)
- CLI tools: `lowercase` descriptive name (e.g., `livetest`, `simtrade`, `fundmove`)

**Go Packages:**
- Internal packages: `package {dirname}` matching directory name
- Exchange adapters: `package {exchangename}` (e.g., `package binance`, `package gateio`)

## Module Dependency Graph

```
cmd/main.go
  ├── internal/api
  ├── internal/config
  ├── internal/database
  ├── internal/discovery
  ├── internal/engine
  ├── internal/models
  ├── internal/risk
  ├── internal/scraper
  ├── internal/spotengine
  ├── pkg/exchange (+ all 6 sub-packages)
  └── pkg/utils

internal/engine
  ├── internal/api
  ├── internal/config
  ├── internal/database
  ├── internal/discovery
  ├── internal/models
  ├── internal/risk
  ├── pkg/exchange
  └── pkg/utils

internal/spotengine
  ├── internal/api
  ├── internal/config
  ├── internal/database
  ├── internal/models
  ├── internal/notify
  ├── internal/risk
  ├── pkg/exchange
  └── pkg/utils

internal/discovery
  ├── internal/config
  ├── internal/database
  ├── internal/models
  ├── pkg/exchange
  └── pkg/utils

internal/risk
  ├── internal/config
  ├── internal/database
  ├── internal/models
  ├── pkg/exchange
  └── pkg/utils

internal/api
  ├── internal/config
  ├── internal/database
  ├── internal/models
  ├── internal/risk
  ├── pkg/exchange
  ├── pkg/utils
  └── web (go:embed)

internal/models
  └── (no internal deps -- leaf node)

internal/config
  └── (standard library only -- leaf node)

pkg/exchange
  └── (standard library + sync -- leaf node)

pkg/utils
  └── (standard library only -- leaf node)

pkg/exchange/{adapter}
  └── pkg/exchange (types + interface only)
```

**Import Cycle Prevention:** The exchange factory (`newExchange()` switch) lives in `cmd/main.go` instead of `pkg/exchange/factory.go` because sub-packages import `pkg/exchange` for types/interfaces, so `pkg/exchange` cannot import them back.

## Where to Add New Code

**New Exchange Adapter:**
- Create: `pkg/exchange/{name}/adapter.go`, `client.go`, `ws.go`, `ws_private.go`
- Implement: `exchange.Exchange` interface (35 methods)
- Optional: `margin.go` implementing `exchange.SpotMarginExchange` (7 methods)
- Register: Add case in `newExchange()` switch at `cmd/main.go:343`
- Register: Add case in `exchangeConfig()` at `cmd/main.go:362`
- Config: Add API key fields to `internal/config/config.go` Config struct
- Discovery: Add exchange name to `SupportedExchanges` in `internal/discovery/scanner.go:29`

**New Risk Filter:**
- Add to: `internal/risk/manager.go` in the `approveInternal()` method
- Config: Add config field with on/off switch (default OFF) + env var override in `internal/config/config.go`
- Dashboard: Add toggle in `web/src/pages/Config.tsx`
- Rule: New risk filters must have configurable on/off switch (default OFF)

**New Dashboard Page:**
- Create: `web/src/pages/{PageName}.tsx`
- Register: Add to `Page` type union and route in `web/src/App.tsx`
- Navigation: Add to `navItems` in `web/src/components/Sidebar.tsx`
- API: Add backend endpoint in `internal/api/handlers.go` or `spot_handlers.go`

**New API Endpoint:**
- Handler: Add function in `internal/api/handlers.go` (perp-perp) or `internal/api/spot_handlers.go` (spot)
- Route: Register in `internal/api/server.go` Start() method
- Auth: Wrap with `s.authMiddleware()` for protected endpoints

**New Engine Feature (Perp-Perp):**
- Primary code: `internal/engine/engine.go` or `internal/engine/exit.go`
- If new scan-triggered action: Add `ScanType` const in `internal/discovery/scanner.go`, handle in `Engine.run()` switch

**New Spot-Futures Feature:**
- Primary code: `internal/spotengine/` (appropriate file based on concern)
- Exit trigger: `internal/spotengine/exit_manager.go`
- Execution: `internal/spotengine/execution.go`

**New CLI Tool:**
- Create: `cmd/{toolname}/main.go`
- Pattern: Self-contained `package main`, import from `pkg/exchange` and/or `internal/` as needed

**Utilities:**
- Shared math/formatting helpers: `pkg/utils/math.go`
- Shared logging: `pkg/utils/logging.go`

## Special Directories

**`web/dist/`:**
- Purpose: Compiled React frontend (JS, CSS, HTML)
- Generated: Yes (by `npm run build` in `web/`)
- Committed: Yes (embedded into Go binary via `go:embed`)
- CRITICAL: Must build frontend BEFORE Go binary (`make build` handles order)

**`logs/`:**
- Purpose: Runtime log files with daily rotation
- Generated: Yes (at runtime by `pkg/utils/logging.go`)
- Committed: No (gitignored)

**`.planning/`:**
- Purpose: GSD planning documents and codebase analysis
- Generated: Yes (by GSD tools)
- Committed: Varies

**`doc/`:**
- Purpose: Exchange API documentation (curated + scraped)
- Contains: Per-exchange API docs with `.annotations/` for curated notes
- Generated: Partially (scraped docs via Playwright; annotations are manual)
- Committed: Yes

**`.gocache/`:**
- Purpose: Go build cache
- Generated: Yes
- Committed: No (gitignored)

**`node_modules/` (under `web/`):**
- Purpose: Frontend npm dependencies
- Generated: Yes (by `npm ci`)
- Committed: No (gitignored)
- CRITICAL: Never run `npm install`/`npm update` -- use only `npm ci` from lockfile

---

*Structure analysis: 2026-04-01*
