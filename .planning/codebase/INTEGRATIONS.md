# External Integrations

**Analysis Date:** 2026-04-01

## Exchange APIs (6 Exchanges)

All exchange adapters live under `pkg/exchange/` and implement the `Exchange` interface defined in `pkg/exchange/exchange.go`. Each adapter has: `adapter.go` (interface impl), `client.go` (HTTP client), `ws.go` (public WebSocket), `ws_private.go` (private WebSocket). Five of six also have `margin.go` (spot margin operations implementing `SpotMarginExchange`).

### Binance

- **REST API:** `https://fapi.binance.com` (USDT-M Futures)
- **Spot REST API:** `https://api.binance.com` (margin borrow/repay via `/sapi/v1/margin/borrow-repay`)
- **Public WS:** `wss://fstream.binance.com/stream?streams=...` (bookTicker for BBO)
- **Private WS:** `wss://fstream.binance.com/ws/{listenKey}` (order updates)
- **Auth:** HMAC-SHA256 signed requests
- **Adapter files:** `pkg/exchange/binance/adapter.go`, `client.go`, `ws.go`, `ws_private.go`, `margin.go`
- **Spot Margin:** Yes (`SpotMarginExchange` interface)
- **Env vars:** `BINANCE_API_KEY`, `BINANCE_SECRET_KEY`

### Bybit

- **REST API:** `https://api.bybit.com` (v5 unified)
- **Public WS:** `wss://stream.bybit.com/v5/public/linear` (bookTicker)
- **Private WS:** `wss://stream.bybit.com/v5/private` (order updates)
- **Auth:** HMAC-SHA256 signed requests
- **Adapter files:** `pkg/exchange/bybit/adapter.go`, `client.go`, `ws.go`, `ws_private.go`, `margin.go`
- **Spot Margin:** Yes
- **Env vars:** `BYBIT_API_KEY`, `BYBIT_SECRET_KEY`

### Gate.io

- **REST API:** `https://api.gateio.ws/api/v4`
- **Public WS:** `wss://fx-ws.gateio.ws/v4/ws/usdt` (bookTicker)
- **Private WS:** Same endpoint, HMAC-SHA512 authenticated channels
- **Auth:** HMAC-SHA512 signed requests
- **Adapter files:** `pkg/exchange/gateio/adapter.go`, `client.go`, `ws.go`, `ws_private.go`, `margin.go`
- **Spot Margin:** Yes (unified account mode with auto-detection)
- **Special:** Unified vs classic account mode auto-detection (`DetectUnifiedMode()` in `cmd/main.go`)
- **Env vars:** `GATEIO_API_KEY`, `GATEIO_SECRET_KEY`

### Bitget

- **REST API:** `https://api.bitget.com` (v2)
- **Public WS:** `wss://ws.bitget.com/v2/ws/public`
- **Private WS:** `wss://ws.bitget.com/v2/ws/private`
- **Auth:** HMAC-SHA256 signed requests + passphrase
- **Adapter files:** `pkg/exchange/bitget/adapter.go`, `client.go`, `ws.go`, `ws_private.go`, `margin.go`
- **Spot Margin:** Yes
- **Env vars:** `BITGET_API_KEY`, `BITGET_SECRET_KEY`, `BITGET_PASSPHRASE`

### OKX

- **REST API:** `https://www.okx.com` (v5)
- **Public WS:** `wss://ws.okx.com:8443/ws/v5/public`
- **Private WS:** `wss://ws.okx.com:8443/ws/v5/private`
- **Auth:** HMAC-SHA256 signed requests + passphrase
- **Adapter files:** `pkg/exchange/okx/adapter.go`, `client.go`, `ws.go`, `ws_private.go`, `margin.go`
- **Spot Margin:** Yes (unified account, manual borrow/repay via `/api/v5/account/spot-manual-borrow-repay`)
- **Env vars:** `OKX_API_KEY`, `OKX_SECRET_KEY`, `OKX_PASSPHRASE`

### BingX

- **REST API:** `https://open-api.bingx.com`
- **Public WS:** `wss://open-api-swap.bingx.com/swap-market`
- **Private WS:** `wss://open-api-swap.bingx.com/swap-market` (same endpoint)
- **Auth:** HMAC-SHA256 signed requests
- **Adapter files:** `pkg/exchange/bingx/adapter.go`, `client.go`, `ws.go`, `ws_private.go`
- **Spot Margin:** No (BingX does not implement `SpotMarginExchange`)
- **Env vars:** `BINGX_API_KEY`, `BINGX_SECRET_KEY`

### Exchange Interface Summary

All adapters implement `Exchange` interface (`pkg/exchange/exchange.go`) providing:
- Order management: `PlaceOrder`, `CancelOrder`, `GetPendingOrders`, `GetOrderFilledQty`
- Position queries: `GetPosition`, `GetAllPositions`
- Balance: `GetFuturesBalance`, `GetSpotBalance`
- Funding rates: `GetFundingRate`, `GetFundingInterval`
- Contract info: `LoadAllContracts`
- WebSocket: `StartPriceStream`, `SubscribeSymbol`, `GetBBO`, `SubscribeDepth`, `StartPrivateStream`, `SetOrderCallback`
- Stop-loss: `PlaceStopLoss`, `CancelStopLoss`
- Account setup: `SetLeverage`, `SetMarginMode`, `EnsureOneWayMode`
- Transfers: `Withdraw`, `TransferToSpot`, `TransferToFutures`
- Trade history: `GetUserTrades`, `GetFundingFees`, `GetClosePnL`
- Lifecycle: `Close` (graceful WS shutdown)

Optional interfaces (use type assertion):
- `PermissionChecker` - API key permission introspection
- `SpotMarginExchange` - Spot margin borrow/repay/order (5 of 6 exchanges; not BingX)
- `WSMetricsCallbackSetter` - WebSocket health event reporting
- `OrderMetricsCallbackSetter` - Order fill-rate tracking

## Data Sources

### Loris API (Primary Funding Rate Source)

- **Current rates:** `https://api.loris.tools/funding` (`internal/discovery/scanner.go:25`)
- **Historical rates:** `https://loris.tools/api/funding/historical` (`internal/discovery/backtest.go:16`)
- **Usage:** Scanner polls Loris for cross-exchange funding rates, then verifies against exchange-native APIs
- **Rate format:** 8h-equivalent rates -- always divide by 8 for bps/h normalization
- **Rate limiting:** Built-in backoff (60s cooldown on 429 responses, `internal/discovery/backtest.go`)
- **Consumer:** `internal/discovery/scanner.go` (Scanner.Scan)

### CoinGlass (Spot-Futures Arbitrage Data)

- **URL:** `https://www.coinglass.com/ArbitrageList` (`internal/scraper/spotarb.go:19`)
- **Method:** Headless Chrome scraping via chromedp (parses rendered HTML table)
- **Schedule:** Configurable cron-like minutes (default: 15, 35 each hour)
- **Data stored in Redis:** Key `coinGlassSpotArb` as JSON payload
- **Consumer:** `internal/spotengine/discovery.go` reads scraped data for spot-futures opportunity ranking

### Binance Delist Monitor

- **URL:** `https://www.binance.com/bapi/composite/v1/public/cms/article/list/query?type=1&catalogId=161&pageNo=1&pageSize=20` (`internal/discovery/delist.go:13`)
- **Purpose:** Monitors Binance announcement API for delisting notices, blacklists affected symbols
- **Poll interval:** Every 6 hours
- **Blacklist stored in Redis:** Keys `arb:delist:{symbol}` with 7-day buffer
- **Toggle:** `DelistFilterEnabled` config flag

## Data Storage

### Redis (Primary State Store)

- **Client:** `github.com/redis/go-redis/v9` via wrapper `internal/database/redis.go`
- **Database:** DB 2 (default, configurable via `REDIS_DB`)
- **Connection:** `REDIS_ADDR` (default `localhost:6379`), `REDIS_PASS`
- **Timeouts:** Dial 5s, Read 3s, Write 3s

**Key namespace (`internal/database/state.go`):**
- `arb:config` - HASH: runtime configuration (dashboard-editable)
- `arb:positions` - HASH: all positions by ID (JSON serialized)
- `arb:positions:active` - SET: IDs of active positions
- `arb:history` - LIST: closed position history (max 1000)
- `arb:funding:latest` - Latest funding rate snapshot
- `arb:funding:snapshots:{symbol}` - LIST: funding rate snapshots (max 100)
- `arb:stats` - Aggregate statistics
- `arb:transfers` - Active transfers
- `arb:transfers:history` - LIST: transfer history (max 200)
- `arb:spread:history:{symbol}|{longExch}|{shortExch}` - LIST: spread observations (max 256)
- `arb:lossCooldown:{symbol}` - STRING: loss cooldown TTL
- `arb:reEnterCooldown:{symbol}` - STRING: re-entry cooldown TTL
- `arb:exchange:{name}:balance` - STRING: futures balance per exchange
- `arb:exchange:{name}:spotBalance` - STRING: spot balance per exchange
- `arb:locks:{resource}` - STRING: distributed locks with Lua scripts
- `arb:delist:{symbol}` - STRING: delisted symbol blacklist

**Spot-futures keys (`internal/database/spot_state.go`):**
- `arb:spot_positions` - HASH: spot-futures positions by ID
- `arb:spot_positions:active` - SET: active spot position IDs
- `arb:spot_history` - LIST: spot-futures closed history (max 500)
- `arb:spot_stats` - Spot-futures aggregate stats
- `arb:spot_persistence:{symbol}` - STRING: persistence tracking (TTL 20min)

**Special features:**
- JSON.GET command support (RedisJSON module, `internal/database/redis.go:50`)
- Lua scripting for atomic lock operations (check, refresh, release in `internal/database/locks.go`)
- Pipeline batching for spot position updates

**File Storage:** Local filesystem only (log files in `logs/` and per-package `logs/` subdirectories)

**Caching:** Redis serves as both persistent store and cache (no separate cache layer)

## Authentication & Identity

### Dashboard Auth

- **Provider:** Custom token-based authentication (`internal/api/auth.go`)
- **Implementation:** Password login returns a random hex token; token stored in `authStore` (in-memory map with TTL)
- **Session TTL:** 24 hours (auto-cleanup every 10 minutes)
- **Password:** `DASHBOARD_PASSWORD` env var
- **Client-side:** Token stored in `localStorage` as `arb_token`, sent via `Authorization: Bearer {token}` header (`web/src/hooks/useApi.ts`)
- **WebSocket auth:** Token passed as query parameter `?token=...` (`web/src/hooks/useWebSocket.ts`)

### Exchange API Auth

- All exchanges use HMAC-signed requests (SHA256 for most, SHA512 for Gate.io)
- Bitget and OKX additionally require a passphrase
- Binance private WS uses listenKey (created via REST, auto-refreshed)
- Other exchanges authenticate WS via signed login messages

## Notifications

### Telegram Bot

- **API:** `https://api.telegram.org/bot{token}/sendMessage`
- **Implementation:** `internal/notify/telegram.go`
- **Purpose:** Spot-futures trade alerts (auto-entry, auto-exit, emergency close)
- **Config:** `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`
- **Behavior:** Best-effort (logs errors, never blocks caller); nil-safe (no-op if not configured)

## AI Diagnostics

### Configurable AI Endpoint

- **Purpose:** Dashboard "diagnose" feature sends logs + positions to an AI for analysis
- **Implementation:** `internal/api/handlers.go:1669` (handleDiagnose)
- **Protocol:** OpenAI-compatible chat completions API (also supports Anthropic content-block format)
- **Config:** `AI_ENDPOINT` (URL), `AI_API_KEY`, `AI_MODEL`, `AI_MAX_TOKENS`
- **Auth header:** `X-API-Key` (not `Authorization: Bearer`)
- **Timeout:** 120 seconds
- **Prompt language:** Traditional Chinese (zh-TW)

## Dashboard HTTP/WS Server

### HTTP Server

- **Implementation:** `internal/api/server.go` using Go `net/http`
- **Default address:** `:8080` (configurable via `DASHBOARD_ADDR`)
- **Timeouts:** Read 15s, Write 180s (extended for AI diagnose), Idle 60s

**API Routes:**
- `POST /api/login` - Authentication
- `GET /api/positions` - Active positions
- `GET /api/history` - Closed position history
- `GET /api/opportunities` - Current arbitrage opportunities
- `GET /api/stats` - Trading statistics
- `GET/PATCH /api/config` - Runtime configuration (read/update)
- `GET /api/exchanges` - Exchange info and balances
- `GET /api/exchanges/health` - Exchange health scores
- `POST /api/positions/close` - Manual position close
- `POST /api/positions/open` - Manual position open
- `GET /api/positions/{id}/funding` - Position funding history
- `POST /api/transfer` - Initiate cross-exchange transfer
- `GET /api/transfers` - Transfer history
- `GET /api/addresses` - Deposit addresses
- `GET /api/logs` - Recent log entries
- `GET /api/rejections` - Rejected opportunities
- `POST /api/diagnose` - AI diagnostic analysis
- `GET /api/permissions` - Exchange API key permissions
- `GET /api/spot/positions` - Spot-futures positions
- `GET /api/spot/history` - Spot-futures history
- `GET /api/spot/stats` - Spot-futures statistics
- `GET /api/spot/opportunities` - Spot-futures opportunities
- `POST /api/spot/open` - Manual spot-futures open
- `POST /api/spot/close` - Manual spot-futures close
- `GET /api/spot/positions/{id}/health` - Spot position health
- `GET/PATCH /api/spot/config/auto` - Spot auto-entry config
- `GET /api/check-update` - Check for binary updates
- `POST /api/update` - Trigger binary update
- `/` - Embedded React SPA (fallback to `index.html` for client-side routing)

### WebSocket Server

- **Endpoint:** `/ws?token={token}`
- **Implementation:** `internal/api/ws.go` (Hub pattern with gorilla/websocket)
- **Protocol:** JSON messages with `{type, data}` envelope
- **Broadcast types:** `positions`, `opportunities`, `stats`, `alerts`, `log`, `rejections`, `spot_positions`, `spot_opportunities`
- **Settings:** Write wait 10s, Pong wait 60s, Ping period 54s, Max message 512 bytes, Send buffer 256 messages

### Frontend SPA

- **Embedding:** `web/embed.go` uses `//go:embed all:dist` to embed built frontend
- **SPA routing:** `internal/api/frontend.go` implements fallback to `index.html` for non-file paths
- **Dev proxy:** Vite dev server proxies `/api` to `http://localhost:8080` and `/ws` with WebSocket upgrade (`web/vite.config.ts`)
- **Pages:** Overview, Positions, History, Opportunities, Config, Logs, Rejections, Transfers, Permissions, Login (`web/src/pages/`)
- **Components:** Sidebar, StatusBadge (`web/src/components/`)
- **State:** WebSocket hook for real-time updates, REST hook for on-demand fetches (`web/src/hooks/`)
- **i18n:** English and Traditional Chinese (`web/src/i18n/en.ts`, `web/src/i18n/zh-TW.ts`)

## WebSocket Connections (Exchange Streams)

Each exchange adapter maintains two persistent WebSocket connections:

**Public Stream (per exchange):**
- Subscribes to bookTicker / BBO updates for monitored symbols
- Dynamic subscription via `SubscribeSymbol()`
- Optional depth subscription (top-5 orderbook) via `SubscribeDepth()`
- Auto-reconnect on disconnect (5s delay)

**Private Stream (per exchange):**
- Receives real-time order fill updates
- Used for stop-loss detection and trade confirmation
- Binance: listenKey-based (auto-refreshed)
- Others: HMAC-signed login message
- Auto-reconnect on disconnect (5s delay)

**Total active connections:** Up to 12 WebSocket connections (2 per exchange x 6 exchanges)

## Monitoring & Observability

**Error Tracking:** No external service; errors logged to file and broadcast to dashboard via WS

**Logs:**
- Custom logger (`pkg/utils/logging.go`) writes to both file and stdout
- Format: `YYYY-MM-DD HH:MM:SS.mmm [LEVEL] [module] message`
- Real-time log streaming to dashboard via WebSocket
- Per-module loggers (e.g., `main`, `engine`, `risk`, `binance-ws-priv`)
- Log files in `logs/` directory and per-package `logs/` subdirectories

**Health Monitoring:**
- Exchange health scorer (`internal/risk/exchange_scorer.go`) - tracks REST latency, WS connectivity, order fill rates
- Health monitor (`internal/risk/health.go`) - periodic margin ratio checks
- Risk monitor (`internal/risk/monitor.go`) - position risk assessment
- Liquidation trend tracker (`internal/risk/liq_trend.go`) - margin ratio regression analysis
- Binary drift monitor (`internal/api/drift_monitor.go`) - detects stale binary, triggers restart

## CI/CD & Deployment

**Hosting:** Self-hosted Linux server (bare metal or VM)

**CI Pipeline:** No CI pipeline detected (no `.github/workflows/`, no `.gitlab-ci.yml`)

**Release Distribution:**
- GitHub releases (repo: `egg5233/arb_public`)
- `scripts/pull-release.sh` downloads latest release binary via `gh` CLI
- Dashboard has built-in update check and trigger (`/api/check-update`, `/api/update`)
- systemd restarts on binary drift detection

**Install Script:** `install.sh` provisions: Go, Node.js (nvm), Redis, systemd service, builds from source

## Environment Configuration

**Required env vars (minimum):**
- At least 2 exchange API key pairs (from: `BINANCE_API_KEY`/`BINANCE_SECRET_KEY`, `BYBIT_API_KEY`/`BYBIT_SECRET_KEY`, `GATEIO_API_KEY`/`GATEIO_SECRET_KEY`, `BITGET_API_KEY`/`BITGET_SECRET_KEY`/`BITGET_PASSPHRASE`, `OKX_API_KEY`/`OKX_SECRET_KEY`/`OKX_PASSPHRASE`, `BINGX_API_KEY`/`BINGX_SECRET_KEY`)
- `REDIS_ADDR` (default: `localhost:6379`)
- `REDIS_PASS`
- `DASHBOARD_PASSWORD`

**Optional env vars:**
- `REDIS_DB` (default: 2)
- `DASHBOARD_ADDR` (default: `:8080`)
- `AI_ENDPOINT`, `AI_API_KEY`, `AI_MODEL`, `AI_MAX_TOKENS` (for diagnostics)
- `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID` (for notifications)
- 50+ strategy tuning parameters (all have sensible defaults)

**Secrets location:** Environment variables only (no secrets files committed). `.env` files may exist locally but are gitignored.

## Webhooks & Callbacks

**Incoming:** None (no webhook endpoints)

**Outgoing:** None (all integrations are pull-based or stream-based)

---

*Integration audit: 2026-04-01*
