# Technology Stack

**Analysis Date:** 2026-04-01

## Languages

**Primary:**
- Go 1.26 - All backend logic, exchange adapters, engine, API server (`go.mod`)
- TypeScript ~5.9.3 - Dashboard frontend (`web/package.json`)

**Secondary:**
- Bash - Install script (`install.sh`), release script (`scripts/pull-release.sh`)
- Lua - Redis lock scripts embedded in Go (`internal/database/locks.go`)

## Runtime

**Environment:**
- Go 1.26+ (runtime; `go.mod` specifies `go 1.26`)
- Node.js v22+ via nvm (frontend build only, not runtime)
- Redis (state store, DB 2 by default)
- Linux (Ubuntu 22.04+ / Debian 12+ / WSL2 supported per `install.sh`)

**Package Manager:**
- Go modules (`go.mod`, `go.sum` - 45 lines, minimal dependency tree)
- npm with lockfile (`web/package-lock.json`) - **CRITICAL: npm install is forbidden due to compromised axios dependency; use only `npm ci`**

## Frameworks

**Core:**
- Go `net/http` (stdlib) - Dashboard HTTP server (`internal/api/server.go`)
- `gorilla/websocket` v1.5.3 - All WebSocket connections (exchange streams + dashboard push)
- `redis/go-redis/v9` v9.18.0 - State persistence, locks, config

**Frontend:**
- React 19.2.0 - SPA dashboard (`web/src/`)
- Vite 7.3.1 - Build tool + dev server (`web/vite.config.ts`)
- Tailwind CSS 4.2.1 (via `@tailwindcss/vite` plugin) - Styling
- TypeScript ~5.9.3 - Type safety

**Testing:**
- Go stdlib `testing` package - Backend tests
- No frontend test framework detected

**Build/Dev:**
- Make - Build orchestration (`Makefile`)
- Vite - Frontend bundling (`web/vite.config.ts`)
- `go:embed` - Embeds `web/dist/` into Go binary (`web/embed.go`)

## Key Dependencies

**Critical (direct, in `go.mod`):**
- `github.com/gorilla/websocket` v1.5.3 - WebSocket client for all 6 exchange streams (public price + private order), plus dashboard WS server
- `github.com/redis/go-redis/v9` v9.18.0 - State persistence (positions, history, config, locks, balances, spread history)
- `github.com/chromedp/chromedp` v0.15.1 - Headless Chrome for CoinGlass web scraping (`internal/scraper/spotarb.go`)

**Indirect (notable):**
- `github.com/alicebob/miniredis/v2` v2.37.0 - In-memory Redis for tests
- `go.uber.org/atomic` v1.11.0 - Atomic operations
- `github.com/gobwas/ws` v1.4.0 - Low-level WebSocket (used by chromedp)

**Frontend (in `web/package.json`):**
- `react` ^19.2.0, `react-dom` ^19.2.0 - UI framework
- `@tailwindcss/vite` ^4.2.1 - Styling
- `@vitejs/plugin-react` ^5.1.1 - React fast refresh
- `eslint` ^9.39.1 + `typescript-eslint` ^8.48.0 - Linting
- No runtime npm dependencies beyond React/ReactDOM

## Build System

**Build Order (CRITICAL):**
Frontend must be built BEFORE Go binary because `go:embed` bakes `web/dist/` into the binary at compile time.

```bash
# Full build (correct order)
make build          # runs build-frontend, then go build -o arb ./cmd/main.go

# Frontend only
make build-frontend # nvm use 22 && npm install --silent && npm run build

# Dev mode (assumes frontend already built)
make dev            # go run ./cmd/main.go
```

**Makefile targets** (`Makefile`):
- `build-frontend` - Requires Node 22+ via nvm, runs `npm install && npm run build` in `web/`
- `build` - Depends on `build-frontend`, then `go build -o arb ./cmd/main.go`
- `dev` - `go run ./cmd/main.go` (no frontend rebuild)
- `run` - Execute the compiled `./arb` binary
- `clean` - Remove `arb` binary and `web/dist/`

**Frontend build pipeline** (`web/package.json`):
- `npm run build` = `tsc -b && vite build` (type-check then bundle)
- Output: `web/dist/` (embedded via `web/embed.go`)

**Go binary entry point:** `cmd/main.go`

## Configuration

**Environment:**
- All config loaded from environment variables and/or a JSON config file (`internal/config/config.go`, 1508 lines)
- JSON config stored in Redis at key `arb:config` (dashboard can update at runtime)
- Config struct supports ~100+ fields covering: strategy params, exchange API keys, Redis connection, dashboard settings, risk thresholds, spot-futures engine params, AI diagnostics, Telegram notifications
- Exchange API keys: per-exchange `{EXCHANGE}_API_KEY`, `{EXCHANGE}_SECRET_KEY`, optional `{EXCHANGE}_PASSPHRASE` (Bitget, OKX)

**Key env var categories:**
- Exchange credentials (6 exchanges x 2-3 keys each)
- Redis connection (`REDIS_ADDR`, `REDIS_PASS`, `REDIS_DB`)
- Dashboard (`DASHBOARD_ADDR`, `DASHBOARD_PASSWORD`)
- Strategy parameters (50+ tuning knobs)
- AI diagnostics (`AI_ENDPOINT`, `AI_API_KEY`, `AI_MODEL`)
- Telegram (`TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`)
- Spot-futures engine (20+ parameters)

**Build:**
- `Makefile` - Top-level build orchestration
- `web/vite.config.ts` - Frontend build config (React + Tailwind plugins, proxy to :8080 in dev)
- `web/tsconfig.json`, `web/tsconfig.app.json`, `web/tsconfig.node.json` - TypeScript config
- `web/eslint.config.js` - ESLint config

## Platform Requirements

**Development:**
- Go 1.22+ (install script provisions 1.22.12)
- Node.js 22+ via nvm (for frontend builds)
- Redis server (DB 2)
- Linux recommended (Ubuntu 22.04+ / Debian 12+); WSL2 supported

**Production:**
- Single Go binary (`./arb`) - self-contained, embeds frontend
- Redis server accessible (default `localhost:6379`, DB 2)
- Deployed via systemd (`install.sh` configures service unit)
- Binary drift monitor: auto-restart under systemd when binary file is updated (`internal/api/drift_monitor.go`)
- Release updates via `scripts/pull-release.sh` (downloads from GitHub releases)

**Deployment Model:**
- Single-binary deployment (Go + embedded React SPA)
- systemd service management (detects `INVOCATION_ID` env var)
- Graceful shutdown on SIGINT/SIGTERM (closes WS connections, stops engine components in reverse order)
- Exit code 1 on drift-triggered restart so systemd `Restart=on-failure` picks it up

## CLI Tools

**Additional entry points in `cmd/`:**
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

- Current version: 0.22.49 (`VERSION` file)
- Changelog maintained in `CHANGELOG.md`
- Every commit must update both `CHANGELOG.md` and `VERSION`

---

*Stack analysis: 2026-04-01*
