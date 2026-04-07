# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository outside of Paperclip.

## CRITICAL: npm security lockdown
**DO NOT run `npm install`, `npm update`, `npm upgrade`, `npx`, or `pnpm install` in the `web/` directory or anywhere in this repo.** The npm axios package has been compromised with a malicious dependency. All frontend dependencies are locked in `web/package-lock.json` — use ONLY `npm ci` (clean install from lockfile) if a fresh install is absolutely needed. Never add, remove, or update npm packages without explicit board/user approval.

## CRITICAL: Do not modify config.json
**DO NOT read, write, modify, or delete `config.json` in the repo root.** This is the live runtime configuration containing API keys, exchange credentials, and tuned trading parameters. Any accidental overwrite or value change can break live trading. If a task requires config changes, tell the user — never touch the file directly.

## CRITICAL: Delegation mode
- You are the coordinator/team lead. For any task involving 2+ files or non-trivial logic, break it into subtasks and delegate to teammates. Wait for teammates to complete before proceeding.
- For trivial changes (single-line fixes, config edits, typos), you may implement directly.
- Use only Sonnet4.6 or Opus4.6 when creating teams.

## Build & Run

**CRITICAL BUILD ORDER**: Frontend uses `go:embed` — MUST build frontend BEFORE Go binary.
Always: `npm run build` (web/) → then `go build`. If reversed, the binary serves stale JS.

```bash
# Full build (frontend + Go binary)
make build

# Dev mode (assumes frontend already built)
make dev

# Frontend only
make build-frontend
```

## Project Structure
Before making structural changes or adding new modules, read ARCHITECTURE.md in the repo root for system design, module boundaries, and strategy details.

## Skill instruction for agent team

### Go code navigation
    Prefer the LSP tool (gopls) over Grep/Glob for Go code navigation — use `goToDefinition`, `findReferences`, `incomingCalls`, `outgoingCalls`, and `hover` when tracing call paths, finding callers, or checking types. Fall back to Grep/Glob only when LSP is unavailable or for cross-language searches.

### Debugging
    load `/sdebug` skill when debugging, bug fixing, or troubleshooting issues. Follow the four-phase framework: Root Cause Investigation → Pattern Analysis → Hypothesis Testing → Implementation.

### When code change involves any exchange api
    load `/local-api-docs` skill


## Codex Integration
If user asks to run codex , read instructions in ~/.claude/plugins/marketplaces/openai-codex/README.md and decide which /codex: command to run

<!-- GSD:project-start source:PROJECT.md -->
## Project

**Arb Bot — Unified Arbitrage System**

A funding rate arbitrage system that monitors funding rate differentials across 6 exchanges (Binance, Bybit, Gate.io, Bitget, OKX, BingX) and executes delta-neutral positions. Two strategy engines — perp-perp (live since mid-March 2026) and spot-futures (first trade 2026-04-01, partially working). The system needs to evolve from two independent engines into a unified arbitrage platform with intelligent capital allocation.

**Core Value:** "I deposit USDT, select my risk preference, and the system automatically finds opportunities across both strategies, opens positions, collects funding, exits when profitable, and I can see exactly how much each position earned."

### Constraints

- **Live system**: changes must not break existing perp-perp trading
- **Tech stack**: Go 1.22+, Redis DB 2, React/Vite/Tailwind, systemd deployment
- **npm lockdown**: axios compromise — only `npm ci`, never `npm install`
- **Build order**: frontend must build before Go binary (go:embed)
- **Exchange APIs**: rate limits, blackout windows (Bybit :04-:05:30), per-exchange quirks
- **Single server**: runs on one machine under systemd
<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->
## Technology Stack

## Languages
- Go 1.26 - All backend logic, exchange adapters, engine, API server (`go.mod`)
- TypeScript ~5.9.3 - Dashboard frontend (`web/package.json`)
- Bash - Install script (`install.sh`), release script (`scripts/pull-release.sh`)
- Lua - Redis lock scripts embedded in Go (`internal/database/locks.go`)
## Runtime
- Go 1.26+ (runtime; `go.mod` specifies `go 1.26`)
- Node.js v22+ via nvm (frontend build only, not runtime)
- Redis (state store, DB 2 by default)
- Linux (Ubuntu 22.04+ / Debian 12+ / WSL2 supported per `install.sh`)
- Go modules (`go.mod`, `go.sum` - 45 lines, minimal dependency tree)
- npm with lockfile (`web/package-lock.json`) - **CRITICAL: npm install is forbidden due to compromised axios dependency; use only `npm ci`**
## Frameworks
- Go `net/http` (stdlib) - Dashboard HTTP server (`internal/api/server.go`)
- `gorilla/websocket` v1.5.3 - All WebSocket connections (exchange streams + dashboard push)
- `redis/go-redis/v9` v9.18.0 - State persistence, locks, config
- React 19.2.0 - SPA dashboard (`web/src/`)
- Vite 7.3.1 - Build tool + dev server (`web/vite.config.ts`)
- Tailwind CSS 4.2.1 (via `@tailwindcss/vite` plugin) - Styling
- TypeScript ~5.9.3 - Type safety
- Go stdlib `testing` package - Backend tests
- No frontend test framework detected
- Make - Build orchestration (`Makefile`)
- Vite - Frontend bundling (`web/vite.config.ts`)
- `go:embed` - Embeds `web/dist/` into Go binary (`web/embed.go`)
## Key Dependencies
- `github.com/gorilla/websocket` v1.5.3 - WebSocket client for all 6 exchange streams (public price + private order), plus dashboard WS server
- `github.com/redis/go-redis/v9` v9.18.0 - State persistence (positions, history, config, locks, balances, spread history)
- `github.com/chromedp/chromedp` v0.15.1 - Headless Chrome for CoinGlass web scraping (`internal/scraper/spotarb.go`)
- `github.com/alicebob/miniredis/v2` v2.37.0 - In-memory Redis for tests
- `go.uber.org/atomic` v1.11.0 - Atomic operations
- `github.com/gobwas/ws` v1.4.0 - Low-level WebSocket (used by chromedp)
- `react` ^19.2.0, `react-dom` ^19.2.0 - UI framework
- `@tailwindcss/vite` ^4.2.1 - Styling
- `@vitejs/plugin-react` ^5.1.1 - React fast refresh
- `eslint` ^9.39.1 + `typescript-eslint` ^8.48.0 - Linting
- No runtime npm dependencies beyond React/ReactDOM
## Build System
# Full build (correct order)
# Frontend only
# Dev mode (assumes frontend already built)
- `build-frontend` - Requires Node 22+ via nvm, runs `npm ci && npm run build` in `web/`
- `build` - Depends on `build-frontend`, then `go build -o arb ./cmd/main.go`
- `dev` - `go run ./cmd/main.go` (no frontend rebuild)
- `run` - Execute the compiled `./arb` binary
- `clean` - Remove `arb` binary and `web/dist/`
- `npm run build` = `tsc -b && vite build` (type-check then bundle)
- Output: `web/dist/` (embedded via `web/embed.go`)
## Configuration
- All config loaded from environment variables and/or a JSON config file (`internal/config/config.go`, 1508 lines)
- JSON config stored in Redis at key `arb:config` (dashboard can update at runtime)
- Config struct supports ~100+ fields covering: strategy params, exchange API keys, Redis connection, dashboard settings, risk thresholds, spot-futures engine params, AI diagnostics, Telegram notifications
- Exchange API keys: per-exchange `{EXCHANGE}_API_KEY`, `{EXCHANGE}_SECRET_KEY`, optional `{EXCHANGE}_PASSPHRASE` (Bitget, OKX)
- Exchange credentials (6 exchanges x 2-3 keys each)
- Redis connection (`REDIS_ADDR`, `REDIS_PASS`, `REDIS_DB`)
- Dashboard (`DASHBOARD_ADDR`, `DASHBOARD_PASSWORD`)
- Strategy parameters (50+ tuning knobs)
- AI diagnostics (`AI_ENDPOINT`, `AI_API_KEY`, `AI_MODEL`)
- Telegram (`TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`)
- Spot-futures engine (20+ parameters)
- `Makefile` - Top-level build orchestration
- `web/vite.config.ts` - Frontend build config (React + Tailwind plugins, proxy to :8080 in dev)
- `web/tsconfig.json`, `web/tsconfig.app.json`, `web/tsconfig.node.json` - TypeScript config
- `web/eslint.config.js` - ESLint config
## Platform Requirements
- Go 1.22+ (install script provisions 1.22.12)
- Node.js 22+ via nvm (for frontend builds)
- Redis server (DB 2)
- Linux recommended (Ubuntu 22.04+ / Debian 12+); WSL2 supported
- Single Go binary (`./arb`) - self-contained, embeds frontend
- Redis server accessible (default `localhost:6379`, DB 2)
- Deployed via systemd (`install.sh` configures service unit)
- Binary drift monitor: auto-restart under systemd when binary file is updated (`internal/api/drift_monitor.go`)
- Release updates via `scripts/pull-release.sh` (downloads from GitHub releases)
- Single-binary deployment (Go + embedded React SPA)
- systemd service management (detects `INVOCATION_ID` env var)
- Graceful shutdown on SIGINT/SIGTERM (closes WS connections, stops engine components in reverse order)
- Exit code 1 on drift-triggered restart so systemd `Restart=on-failure` picks it up
## CLI Tools
- `cmd/main.go` - Main bot entry point
- `cmd/livetest/` - Live exchange API testing (`go run ./cmd/livetest/ --exchange binance`)
- `cmd/balance/` - Balance checker
- `cmd/closepos/` - Position closer
- `cmd/fundmove/` - Fund movement
- `cmd/simtrade/` - Simulated trading
- `cmd/spotarb/` - Spot arbitrage CLI
- `cmd/testfunding/` - Funding rate testing
- `cmd/transfer/` - Transfer management
- `cmd/transfertest/` - Transfer testing
- `cmd/gatetest/` - Gate.io specific testing
- `cmd/wstest/` - WebSocket testing
- `cmd/validate-docs/` - API documentation validator
## Versioning
- Current version: 0.24.6 (`VERSION` file)
- Changelog maintained in `CHANGELOG.md`
- Every commit must update both `CHANGELOG.md` and `VERSION`
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

## Naming Patterns
- Lowercase single-word names: `engine`, `discovery`, `risk`, `models`, `api`, `database`, `notify`
- Exchange adapters under `pkg/exchange/{name}/`: `binance`, `bybit`, `gateio`, `bitget`, `okx`, `bingx`
- Utility package: `pkg/utils/`
- Snake_case for Go source files: `spot_state.go`, `risk_gate.go`, `rate_velocity.go`
- Test files co-located with source: `{name}_test.go`
- Each exchange adapter has a consistent file set: `adapter.go`, `client.go`, `ws.go`, `ws_private.go`, optional `margin.go`
- PascalCase for exported: `PlaceOrder()`, `GetFuturesBalance()`, `LoadAllContracts()`
- camelCase for unexported: `mapSide()`, `buildQuery()`, `isMarginError()`
- Constructors use `New` prefix: `NewClient()`, `NewAdapter()`, `NewServer()`, `NewLogger()`
- camelCase throughout: `priceStore`, `depthSyms`, `exitCancels`
- Sync primitives suffixed with `Mu` or `Lock`: `exitMu`, `logMu`, `priceMu`, `slIndexMu`
- Constants use PascalCase: `StatusActive`, `SideBuy`, `WSEventConnect`
- PascalCase exported with JSON snake_case tags: `LongExchange string \`json:"long_exchange"\``
- Consistent `json:"snake_case"` tag convention across all models
- PascalCase for React components: `Overview.tsx`, `StatusBadge.tsx`, `Sidebar.tsx`
- camelCase for hooks: `useApi.ts`, `useWebSocket.ts`
- camelCase for utilities: `tradingUrl.tsx`
- Lowercase for i18n locale files: `en.ts`, `zh-TW.ts`
- camelCase for functions and variables: `handleLogin`, `silentCheckUpdate`, `dismissUpdate`
- PascalCase for React component names: `Overview`, `StatusBadge`
- Type interfaces use PascalCase: `Opportunity`, `Position`, `ExchangeInfo`
## Code Style
- Go: standard `gofmt` formatting (no custom config detected)
- Frontend: ESLint with TypeScript-ESLint, react-hooks, react-refresh plugins
- Config: `web/eslint.config.js`
- No Prettier config detected -- rely on ESLint rules
- Return `(result, error)` tuples consistently
- Wrap errors with context using `fmt.Errorf("MethodName: %w", err)`
- Check errors immediately after call, never defer error checks
- Pattern in adapters:
- Each exchange client defines its own `APIError` struct:
- `isMarginError()` in `internal/engine/engine.go` detects margin errors across exchanges using case-insensitive string matching
- Custom sentinel errors: `ErrRepayBlackout` in `pkg/exchange/types.go`
- Methods on optional types (e.g., `*TelegramNotifier`) are safe to call on nil receivers -- see `internal/notify/telegram.go`
- Standard JSON wrapper for all dashboard API responses:
- Helper: `writeJSON(w, status, Response{...})` in `internal/api/handlers.go`
## Comment/Documentation Style
- Every interface method has a single-line `//` comment explaining its purpose
- Interface-level `//` comment describes the abstraction's role
- Example from `pkg/exchange/exchange.go`:
- Use `var _ InterfaceName = (*ConcreteType)(nil)` at top of adapter files
- Example in `pkg/exchange/binance/adapter.go`:
- Also in `internal/discovery/scanner.go`:
- Inline `//` comments on struct fields describing purpose and units:
- Use `// ---------------------------------------------------------------------------` comment blocks to separate logical sections within large files (see adapter.go files)
- Test functions include descriptive comments with regression ticket IDs:
## Configuration Patterns
- Dashboard POST `/api/config` updates in-memory `*config.Config`, persists to `config.json` (with `.bak` backup), and stores critical toggles in Redis
- Config reload on restart reads Redis overrides after JSON loading
- Must have a config boolean switch (default OFF)
- Must have a dashboard UI toggle
- Pattern: `Enable{FeatureName}` field in Config struct, `enable_{feature_name}` in JSON, persisted to Redis
- Exchange keys: `BINANCE_API_KEY`, `BINANCE_SECRET_KEY`, `BYBIT_API_KEY`, etc.
- Redis: `REDIS_ADDR`, `REDIS_PASS`, `REDIS_DB`
- Dashboard: `DASHBOARD_ADDR`, `DASHBOARD_PASSWORD`
- Logging: `LOG_FILE` (default `logs/arb.log`), `DEBUG` (enables debug logs)
## Exchange Adapter Interface Pattern
- `SpotMarginExchange` -- spot margin borrow/trade (5 of 6 exchanges, not BingX)
- `SpotMarginOrderQuerier` -- query spot margin order status
- `PermissionChecker` -- API key permission introspection
- `WSMetricsCallbackSetter` -- WebSocket health metrics
- `OrderMetricsCallbackSetter` -- order fill tracking
| File | Purpose |
|------|---------|
| `adapter.go` | Implements `Exchange` interface, order/position/funding methods |
| `client.go` | Low-level HTTP client with auth signing |
| `ws.go` | Public WebSocket (BBO prices, depth) |
| `ws_private.go` | Private WebSocket (order updates) |
| `margin.go` | Implements `SpotMarginExchange` (optional) |
- Internal format: `BTCUSDT` (uppercase, no separators)
- Exchange-specific mapping happens inside each adapter (e.g., Gate.io uses `BTC_USDT`, OKX uses `BTC-USDT-SWAP`)
- OKX has `ctVal` (contract value per contract), Gate.io has `quanto_multiplier`
- Adapters convert between base-asset units and raw contract counts internally
- Callers always work in base-asset units (e.g., 480 BTC units, not 48 contracts)
## Frontend Conventions (React/Tailwind/i18n)
- Functional components using `FC<Props>` type annotation
- Props interfaces defined at top of file
- `useLocale()` hook for translations in every page component
- State managed with `useState`, effects with `useEffect`, refs with `useRef`
- Dark theme: `bg-gray-950`, `bg-gray-900`, `text-gray-100`
- Responsive: `md:` prefix for desktop breakpoints (mobile-first)
- Status colors via mapped classes: `bg-green-500/20 text-green-400` for active
- Utility-first inline classes, no custom CSS (except `index.css` for Tailwind import)
- Plugin: `@tailwindcss/vite` (v4 style, integrated via Vite plugin)
- Two locales: `en` (English) and `zh-TW` (Traditional Chinese, default)
- Translation keys use dot-notation: `'nav.overview'`, `'overview.totalPnl'`
- `LocaleContext` provides `{ locale, setLocale, t }` throughout the app
- Translation function: `t(key: TranslationKey) => string`
- All user-facing strings must use translation keys, not hardcoded text
- Both locale files must stay in sync when adding new keys: `web/src/i18n/en.ts` and `web/src/i18n/zh-TW.ts`
- `TranslationKey` type is derived from the English locale file keys for type safety
- `useApi()` hook in `web/src/hooks/useApi.ts` for REST calls
- `useWebSocket()` hook in `web/src/hooks/useWebSocket.ts` for real-time updates
- Standard envelope: `{ ok: boolean, data?: T, error?: string }`
- Bearer token auth stored in `localStorage` key `arb_token`
- REST seeds initial state, WebSocket pushes incremental updates
- Vite + React (no SSR)
- Output to `web/dist/`, embedded in Go binary via `go:embed`
- Build: `npm run build` in `web/` directory (Node.js >= 22 required)
- Dev proxy: `/api` and `/ws` proxied to `http://localhost:8080`
## Module Boundaries
- `cmd/` depends on everything
- `internal/engine/` depends on `internal/discovery/`, `internal/risk/`, `internal/models/`, `internal/database/`, `pkg/exchange/`
- `internal/models/` defines interfaces (`Discoverer`, `RiskChecker`, `StateStore`) that concrete implementations satisfy
- `pkg/exchange/` is standalone (no `internal/` imports)
- `pkg/utils/` is standalone (no `internal/` imports)
- Engine depends on `models.Discoverer` not `*discovery.Scanner`
- Engine depends on `models.StateStore` not `*database.Client`
- This allows test stubs to be injected
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

## Pattern Overview
- Two independent arbitrage engines (perp-perp and spot-futures) sharing exchange adapters
- Scan-driven execution model: fixed-minute scheduler fires discovery scans that dispatch to typed handlers (rebalance, exit, rotate, entry)
- Delta-neutral strategy: always paired long/short positions across exchanges
- Interface-driven dependency injection: `Discoverer`, `RiskChecker`, `StateStore` abstractions in `internal/models/interfaces.go`
- Single Go binary embeds React frontend via `go:embed`
- Redis DB 2 as sole persistence layer (state, locks, history, stats)
## Layers
- Purpose: Unified API abstraction across 6 CEXes (Binance, Bybit, Gate.io, Bitget, OKX, BingX)
- Location: `pkg/exchange/`
- Contains: `Exchange` interface (35 methods) in `pkg/exchange/exchange.go`, shared types in `pkg/exchange/types.go`, per-exchange adapter packages
- Depends on: Nothing internal (standalone, importable)
- Used by: All internal packages (`engine`, `discovery`, `risk`, `spotengine`)
- Each adapter has: `adapter.go` (interface impl), `client.go` (REST), `ws.go` (public WS), `ws_private.go` (private WS), optional `margin.go`
- Optional `SpotMarginExchange` interface for borrow/repay (all except BingX)
- Purpose: Poll funding rate data, merge sources, verify rates, rank opportunities
- Location: `internal/discovery/`
- Contains: `scanner.go` (polling loop, schedule), `verifier.go` (4-check rate verification), `ranker.go` (profitability filter, scoring), `backtest.go` (historical validation), `delist.go` (Binance delist monitor)
- Depends on: `pkg/exchange`, `internal/database`, `internal/models`, `internal/config`
- Used by: `internal/engine` (consumes `ScanResult` from channel)
- Produces: `ScanResult{Opps []Opportunity, Type ScanType}` on a buffered channel
- Purpose: Main orchestration loop -- dispatches scan results to rebalance/exit/rotate/entry handlers
- Location: `internal/engine/`
- Contains: `engine.go` (main loop, trade execution, funding tracker, SL detection, 2883 lines), `exit.go` (exit strategy, depth-fill, rotation, PnL reconciliation, 2518 lines), `consolidate.go` (position reconciliation, 524 lines), `capital.go` (allocator helpers, 72 lines)
- Depends on: All internal packages (`discovery`, `risk`, `database`, `api`, `models`, `config`), `pkg/exchange`
- Used by: `cmd/main.go` (instantiated and started)
- Central state holder: tracks exit goroutines, entry guards, SL index, own-order tracking
- Purpose: Pre-trade approval (11-point check), active monitoring, margin health, capital allocation
- Location: `internal/risk/`
- Contains: `manager.go` (pre-trade approval, 598 lines), `monitor.go` (4-dimension active monitoring, 429 lines), `health.go` (L0-L5 margin health tiers, 469 lines), `limits.go` (hard constraints), `allocator.go` (cross-strategy capital), `spread_stability.go` (Redis-backed CV gate), `exchange_scorer.go` (health scoring), `liq_trend.go` (liquidation trend tracking)
- Depends on: `pkg/exchange`, `internal/database`, `internal/models`, `internal/config`
- Used by: `internal/engine`
- Emits: `HealthAction` on a channel consumed by the engine
- Purpose: Independent engine for spot margin + futures hedge arbitrage
- Location: `internal/spotengine/`
- Contains: `engine.go` (lifecycle, discovery loop), `discovery.go` (native Loris scanner + CoinGlass fallback, scoring), `execution.go` (entry/exit trades), `exit_manager.go` (exit triggers + guards: min-hold, settlement, spread), `monitor.go` (stuck exit retry, repay retry), `risk_gate.go` (pre-entry checks + basis gate), `autoentry.go` (automated entry), `capital.go`, `rate_velocity.go`
- Depends on: `pkg/exchange`, `internal/database`, `internal/api`, `internal/models`, `internal/config`, `internal/risk`, `internal/notify`
- Used by: `cmd/main.go` (conditionally started if `SpotFuturesEnabled`)
- Fully independent from perp-perp engine: separate goroutines, separate Redis keys, separate API routes
- Purpose: Redis persistence layer for all state
- Location: `internal/database/`
- Contains: `redis.go` (connection), `state.go` (positions, history, funding, stats, transfers), `locks.go` (distributed locks with SET NX + TTL + Lua scripts), `spot_state.go` (spot-futures position CRUD)
- Depends on: `github.com/redis/go-redis/v9`, `internal/models`
- Used by: All internal packages
- Implements `models.StateStore` interface (compile-time verified)
- Purpose: HTTP REST + WebSocket server for dashboard and manual controls
- Location: `internal/api/`
- Contains: `server.go` (HTTP mux, lifecycle), `handlers.go` (REST endpoints), `ws.go` (WebSocket hub), `auth.go` (session tokens), `frontend.go` (SPA embedding), `spot_handlers.go` (spot-futures endpoints), `drift_monitor.go` (binary staleness detection)
- Depends on: `internal/database`, `internal/config`, `internal/models`, `internal/risk`, `pkg/exchange`
- Used by: `cmd/main.go`, both engines broadcast via `api.Server`
- Purpose: Shared data structures and interface definitions
- Location: `internal/models/`
- Contains: `interfaces.go` (Discoverer, RiskChecker, StateStore), `opportunity.go`, `position.go`, `funding.go`, `spot_position.go`, `rejection.go`
- Depends on: Nothing internal
- Used by: All internal packages
- Purpose: Unified configuration from JSON + environment variables
- Location: `internal/config/config.go`
- Depends on: Standard library only
- Used by: All internal packages
- Load priority: defaults -> `config.json` -> env vars -> `EnsureScanMinutes()` auto-includes special minutes
- Purpose: Telegram Bot API alerts for trade lifecycle events
- Location: `internal/notify/telegram.go`
- Used by: `internal/spotengine` only
- Purpose: Headless Chrome scraper for CoinGlass spot-futures data
- Location: `internal/scraper/spotarb.go`
- Depends on: `github.com/chromedp/chromedp`, `internal/database`
- Used by: `cmd/main.go` (conditionally started)
- Purpose: Math helpers, structured logging with daily rotation
- Location: `pkg/utils/`
- Contains: `math.go` (RoundToStep, EstimateSlippage, RateToBpsPerHour, GenerateID), `logging.go` (Logger with module context, file output, subscriber broadcast)
- Depends on: Standard library only
- Used by: All packages
## Data Flow
- All position state persisted to Redis (DB 2) via `internal/database/state.go`
- Distributed locks via Redis SET NX + TTL + Lua scripts (`internal/database/locks.go`)
- In-memory tracking for exit goroutines (`exitActive`, `exitCancels`, `exitDone` maps in Engine)
- `entryActive` map prevents consolidator from interfering with in-progress depth fills
- `ownOrders` sync.Map tracks engine-placed orders to avoid false SL triggers
## Key Abstractions
- Purpose: Unified 35-method interface for all CEX operations
- Definition: `pkg/exchange/exchange.go`
- Implementations: `pkg/exchange/{binance,bybit,gateio,bitget,okx,bingx}/adapter.go`
- Pattern: Each adapter normalizes exchange-specific symbol formats, response structures, and WebSocket protocols
- Purpose: Optional spot margin operations (borrow, repay, margin orders)
- Definition: `pkg/exchange/exchange.go` (below `Exchange` interface)
- Implementations: All except BingX
- Pattern: Type assertion `exch.(exchange.SpotMarginExchange)` to check support
- Purpose: Decouple engine from concrete implementations
- Definition: `internal/models/interfaces.go`
- Pattern: Compile-time interface satisfaction checks (`var _ models.Discoverer = (*Scanner)(nil)`)
- Purpose: Typed scan result carrying opportunities + scan classification
- Definition: `internal/discovery/scanner.go:44`
- Pattern: Channel-based communication from Scanner to Engine
## Entry Points
- Location: `cmd/main.go`
- Triggers: Direct execution or systemd service
- Responsibilities: Wire all components, start in order (Scanner -> RiskMon -> HealthMon -> API -> Engine -> SpotEngine), handle graceful shutdown on SIGINT/SIGTERM
- Exchange factory: `newExchange()` switch at `cmd/main.go:343` (avoids import cycles)
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
- HTTP: `internal/api/server.go` serves REST API and embedded React SPA
- WebSocket: `/ws` endpoint for real-time updates
- Frontend: `web/src/App.tsx` (React SPA with page-based routing)
## Concurrency Model
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
- `sync.Mutex`: `exitMu` (exit goroutine tracking), `entryMu` (entry tracking), `capacityMu` (global capacity), `pnlReconcileMu` (PnL reconciliation serialization), `preSettleMu` (timer dedup)
- `sync.RWMutex`: `slIndexMu` (SL order index), scanner's `mu` (opportunity cache), `intervalsMu` (interval data)
- `sync.Map`: `ownOrders` (engine-placed order IDs), `posMu` in SpotEngine (per-position locks)
- `sync.WaitGroup`: parallel trade execution, SpotEngine lifecycle
- `context.Context`: exit goroutine cancellation
- Redis distributed locks: per-symbol `SET NX + TTL` with Lua-based release (`internal/database/locks.go`)
- Channels: `oppChan` (Scanner -> Engine), `slFillCh` (WS callbacks -> SL consumer), `actionCh` (HealthMonitor -> Engine), `alertCh` (RiskMonitor -> Engine)
- Consolidator skips positions with `exitActive[posID]` or `entryActive["exchange:symbol"]` set
- `capacityMu` serializes concurrent manual open and automated entry
- Per-symbol Redis locks prevent duplicate execution
- Exit goroutine preemption: L4/L5 health actions cancel running exit via stored `CancelFunc`
- `UpdatePositionFields` atomic read-modify-write with predicate function
## Error Handling
- **Exchange API errors**: Logged with exchange name and endpoint context. Stop-loss failures never block entry/exit/rotation
- **Depth fill failures**: Circuit breaker after 5 consecutive `PlaceOrder` failures on either exchange. Partial fills are trimmed to matched portion
- **Exit fallback**: Depth-fill timeout -> market IOC for remainder (must close fully)
- **Emergency escalation**: L5 margin health triggers `handleEmergencyClose()` which preempts any running exit goroutine (500ms grace period)
- **SL fill detection**: Dual-method detection (slIndex lookup + ReduceOnly fill verification) to catch exchange-side stop triggers even when order IDs change
- **Retry patterns**: SpotEngine uses `retryLeg()` with configurable attempts and backoff. PnL reconciliation retries 3 times at 5s/15s/30s
- **Margin errors**: `isMarginError()` helper detects "insufficient", "not enough", "margin available" across exchange error message formats (`internal/engine/engine.go:24`)
- **Consolidator**: BingX legs require 3 consecutive misses before acting (guards against transient empty-position API responses)
## Startup and Shutdown
## Scan Schedule
| Minute | Scan Type | Handler |
|--------|-----------|---------|
| :05 | NormalScan | Dashboard broadcast only |
| :15 | NormalScan | Dashboard broadcast only |
| :20 | RebalanceScan | `rebalanceFunds()` |
| :25 | ExitScan | `checkIntervalChanges()` + `checkExitsV2()` |
| :35 | EntryScan | `executeArbitrage(opps)` |
| :45 | RotateScan | `checkRotations()` |
| :55 | NormalScan | Dashboard broadcast only |
## Risk Tiers
| Level | Threshold | Action |
|-------|-----------|--------|
| L0-None | No positions | -- |
| L1-Safe | PnL >= 0 | -- |
| L2-Low | PnL < 0, ratio < 0.50 | -- |
| L3-Medium | ratio >= 0.50 | Transfer funds from healthiest exchange |
| L4-High | ratio >= 0.80 | Reduce positions by 50% |
| L5-Critical | ratio >= 0.95 | Emergency close all |
<!-- GSD:architecture-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->

## graphify

This project has a graphify knowledge graph at graphify-out/.

Rules:
- Before answering architecture or codebase questions, read graphify-out/GRAPH_REPORT.md for god nodes and community structure
- If graphify-out/wiki/index.md exists, navigate it instead of reading raw files
- After modifying code files in this session, run `python3 -c "from graphify.watch import _rebuild_code; from pathlib import Path; _rebuild_code(Path('.'))"` to keep the graph current
