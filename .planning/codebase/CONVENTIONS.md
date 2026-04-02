# Coding Conventions

**Analysis Date:** 2026-04-01

## Naming Patterns

**Go Packages:**
- Lowercase single-word names: `engine`, `discovery`, `risk`, `models`, `api`, `database`, `notify`
- Exchange adapters under `pkg/exchange/{name}/`: `binance`, `bybit`, `gateio`, `bitget`, `okx`, `bingx`
- Utility package: `pkg/utils/`

**Go Files:**
- Snake_case for Go source files: `spot_state.go`, `risk_gate.go`, `rate_velocity.go`
- Test files co-located with source: `{name}_test.go`
- Each exchange adapter has a consistent file set: `adapter.go`, `client.go`, `ws.go`, `ws_private.go`, optional `margin.go`

**Go Functions:**
- PascalCase for exported: `PlaceOrder()`, `GetFuturesBalance()`, `LoadAllContracts()`
- camelCase for unexported: `mapSide()`, `buildQuery()`, `isMarginError()`
- Constructors use `New` prefix: `NewClient()`, `NewAdapter()`, `NewServer()`, `NewLogger()`

**Go Variables:**
- camelCase throughout: `priceStore`, `depthSyms`, `exitCancels`
- Sync primitives suffixed with `Mu` or `Lock`: `exitMu`, `logMu`, `priceMu`, `slIndexMu`
- Constants use PascalCase: `StatusActive`, `SideBuy`, `WSEventConnect`

**Go Struct Fields:**
- PascalCase exported with JSON snake_case tags: `LongExchange string \`json:"long_exchange"\``
- Consistent `json:"snake_case"` tag convention across all models

**Frontend Files:**
- PascalCase for React components: `Overview.tsx`, `StatusBadge.tsx`, `Sidebar.tsx`
- camelCase for hooks: `useApi.ts`, `useWebSocket.ts`
- camelCase for utilities: `tradingUrl.tsx`
- Lowercase for i18n locale files: `en.ts`, `zh-TW.ts`

**Frontend Variables/Functions:**
- camelCase for functions and variables: `handleLogin`, `silentCheckUpdate`, `dismissUpdate`
- PascalCase for React component names: `Overview`, `StatusBadge`
- Type interfaces use PascalCase: `Opportunity`, `Position`, `ExchangeInfo`

## Code Style

**Formatting:**
- Go: standard `gofmt` formatting (no custom config detected)
- Frontend: ESLint with TypeScript-ESLint, react-hooks, react-refresh plugins
- Config: `web/eslint.config.js`
- No Prettier config detected -- rely on ESLint rules

**Go Error Handling:**
- Return `(result, error)` tuples consistently
- Wrap errors with context using `fmt.Errorf("MethodName: %w", err)`
- Check errors immediately after call, never defer error checks
- Pattern in adapters:
```go
body, err := b.client.Post("/fapi/v1/order", params)
if err != nil {
    return "", fmt.Errorf("PlaceOrder: %w", err)
}
```

**Exchange API Error Types:**
- Each exchange client defines its own `APIError` struct:
```go
type APIError struct {
    Code int    `json:"code"`
    Msg  string `json:"msg"`
}
func (e *APIError) Error() string {
    return fmt.Sprintf("binance API error code=%d msg=%s", e.Code, e.Msg)
}
```

**Error Classification:**
- `isMarginError()` in `internal/engine/engine.go` detects margin errors across exchanges using case-insensitive string matching
- Custom sentinel errors: `ErrRepayBlackout` in `pkg/exchange/types.go`

**Nil Receiver Safety:**
- Methods on optional types (e.g., `*TelegramNotifier`) are safe to call on nil receivers -- see `internal/notify/telegram.go`

**API Response Envelope:**
- Standard JSON wrapper for all dashboard API responses:
```go
type Response struct {
    OK    bool        `json:"ok"`
    Data  interface{} `json:"data,omitempty"`
    Error string      `json:"error,omitempty"`
}
```
- Helper: `writeJSON(w, status, Response{...})` in `internal/api/handlers.go`

## Logging

**Framework:** Custom `utils.Logger` in `pkg/utils/logging.go`

**Log Format:**
```
YYYY-MM-DD HH:MM:SS.mmm [LEVEL] [module] message
```
Example: `2026-04-01 14:30:15.123 [INFO] [engine] Starting funding rate arbitrage bot...`

**Levels:** `INFO`, `WARN`, `ERROR`, `DEBUG` (DEBUG requires `DEBUG` env var)

**Creating Loggers:**
```go
log := utils.NewLogger("engine")
log.Info("Configuration loaded")
log.Error("Failed to connect to Redis: %v", err)
log.Warn("Exchange %s returned empty balance", name)
```

**Convention:** Use `log` as the variable name for the logger. Every module creates its own logger with a descriptive module name.

**Log Output:** Dual-write to stdout and buffered file (`logs/arb.log`), with daily rotation at 00:10. Real-time log entries broadcast to WebSocket subscribers.

## Import Organization

**Go Import Order (3 groups, separated by blank lines):**
1. Standard library (`encoding/json`, `fmt`, `time`, etc.)
2. Internal project packages (`arb/internal/...`, `arb/pkg/...`)
3. Third-party packages (`github.com/gorilla/websocket`, `github.com/redis/go-redis/v9`)

**Example from `internal/engine/engine.go`:**
```go
import (
    "context"
    "fmt"
    "math"
    "strconv"
    "strings"
    "sync"
    "time"

    "arb/internal/api"
    "arb/internal/config"
    "arb/internal/database"
    "arb/internal/discovery"
    "arb/internal/models"
    "arb/internal/risk"
    "arb/pkg/exchange"
    "arb/pkg/utils"
)
```

**Frontend Import Order (3 groups):**
1. React/library imports (`import { useState } from 'react'`)
2. Local types/hooks (`import type { Position } from '../types.ts'`)
3. Local components/pages (`import Overview from './pages/Overview.tsx'`)

**Path Style:** Frontend uses relative paths with `.ts`/`.tsx` extensions: `'../types.ts'`, `'./hooks/useApi.ts'`. No path aliases configured.

## Comment/Documentation Style

**Go Interface Documentation:**
- Every interface method has a single-line `//` comment explaining its purpose
- Interface-level `//` comment describes the abstraction's role
- Example from `pkg/exchange/exchange.go`:
```go
// Exchange is the unified interface that all exchange adapters must implement.
type Exchange interface {
    // Identity
    Name() string

    // Orders
    PlaceOrder(req PlaceOrderParams) (orderID string, err error)
    ...
}
```

**Compile-Time Interface Checks:**
- Use `var _ InterfaceName = (*ConcreteType)(nil)` at top of adapter files
- Example in `pkg/exchange/binance/adapter.go`:
```go
var _ exchange.Exchange = (*Adapter)(nil)
```
- Also in `internal/discovery/scanner.go`:
```go
var _ models.Discoverer = (*Scanner)(nil)
```

**Struct Field Comments:**
- Inline `//` comments on struct fields describing purpose and units:
```go
Spread  float64 `json:"spread"`  // ShortRate - LongRate in bps/h (positive = profitable)
```

**Section Dividers:**
- Use `// ---------------------------------------------------------------------------` comment blocks to separate logical sections within large files (see adapter.go files)

**Test Comments:**
- Test functions include descriptive comments with regression ticket IDs:
```go
// TestCheckRiskGate_DryRunAtCapacity verifies that when both dry-run and
// at-capacity are true, the gate reports at_capacity (not dry_run).
// Regression test for ARB-74.
```

## Configuration Patterns

**Priority Order:** Environment variables > `config.json` > default values

**Config Loading:** `internal/config/config.go` `Load()` function:
1. Initialize struct with all defaults
2. Parse `config.json` (path from `CONFIG_FILE` env var, default `config.json`)
3. Apply JSON values with zero-value guards (pointers for optional fields: `*bool`, `*int`, `*float64`)
4. Override with environment variables (e.g., `BINANCE_API_KEY`, `REDIS_ADDR`)

**JSON Config Structure:** Nested sections mirror the config struct:
```json
{
  "dry_run": false,
  "exchanges": { "binance": { "api_key": "...", "secret_key": "...", "enabled": true } },
  "redis": { "addr": "localhost:6379", "db": 2 },
  "strategy": { "discovery": {...}, "entry": {...}, "exit": {...} },
  "risk": { "margin_l3_threshold": 0.50, ... },
  "spot_futures": { "enabled": true, "max_positions": 2, ... }
}
```

**Zero-Value Guards:** JSON config uses pointer types (`*bool`, `*int`, `*float64`) so that zero values in JSON don't overwrite defaults. Only non-nil pointer values are applied.

**Runtime Config Changes:**
- Dashboard POST `/api/config` updates in-memory `*config.Config`, persists to `config.json` (with `.bak` backup), and stores critical toggles in Redis
- Config reload on restart reads Redis overrides after JSON loading

**New Risk/Feature Toggles:**
- Must have a config boolean switch (default OFF)
- Must have a dashboard UI toggle
- Pattern: `Enable{FeatureName}` field in Config struct, `enable_{feature_name}` in JSON, persisted to Redis

**Environment Variables (selected):**
- Exchange keys: `BINANCE_API_KEY`, `BINANCE_SECRET_KEY`, `BYBIT_API_KEY`, etc.
- Redis: `REDIS_ADDR`, `REDIS_PASS`, `REDIS_DB`
- Dashboard: `DASHBOARD_ADDR`, `DASHBOARD_PASSWORD`
- Logging: `LOG_FILE` (default `logs/arb.log`), `DEBUG` (enables debug logs)

## Exchange Adapter Interface Pattern

**Core Interface:** `pkg/exchange/exchange.go` defines `Exchange` interface (~30 methods)

**Optional Interfaces (use type assertion to check):**
- `SpotMarginExchange` -- spot margin borrow/trade (5 of 6 exchanges, not BingX)
- `SpotMarginOrderQuerier` -- query spot margin order status
- `PermissionChecker` -- API key permission introspection
- `WSMetricsCallbackSetter` -- WebSocket health metrics
- `OrderMetricsCallbackSetter` -- order fill tracking

**Adapter File Structure (per exchange):**
| File | Purpose |
|------|---------|
| `adapter.go` | Implements `Exchange` interface, order/position/funding methods |
| `client.go` | Low-level HTTP client with auth signing |
| `ws.go` | Public WebSocket (BBO prices, depth) |
| `ws_private.go` | Private WebSocket (order updates) |
| `margin.go` | Implements `SpotMarginExchange` (optional) |

**Constructor Pattern:**
```go
func NewAdapter(cfg exchange.ExchangeConfig) *Adapter {
    return &Adapter{
        client:    NewClient(cfg.ApiKey, cfg.SecretKey),
        apiKey:    cfg.ApiKey,
        secretKey: cfg.SecretKey,
        priceSyms: make(map[string]bool),
        depthSyms: make(map[string]bool),
    }
}
```

**Factory in main.go:** Exchange creation logic lives in `cmd/main.go` (not in the `exchange` package) to avoid import cycles. The `exchange` package provides types/interfaces only.

**Symbol Normalization:**
- Internal format: `BTCUSDT` (uppercase, no separators)
- Exchange-specific mapping happens inside each adapter (e.g., Gate.io uses `BTC_USDT`, OKX uses `BTC-USDT-SWAP`)

**Contract Multiplier Handling:**
- OKX has `ctVal` (contract value per contract), Gate.io has `quanto_multiplier`
- Adapters convert between base-asset units and raw contract counts internally
- Callers always work in base-asset units (e.g., 480 BTC units, not 48 contracts)

## Frontend Conventions (React/Tailwind/i18n)

**Component Pattern:**
- Functional components using `FC<Props>` type annotation
- Props interfaces defined at top of file
- `useLocale()` hook for translations in every page component
- State managed with `useState`, effects with `useEffect`, refs with `useRef`

**Example Component Structure:**
```tsx
import { useState, type FC } from 'react';
import type { Position } from '../types.ts';
import { useLocale } from '../i18n/index.ts';

interface OverviewProps {
  positions: Position[];
  stats: Stats | null;
}

const Overview: FC<OverviewProps> = ({ positions, stats }) => {
  const { t } = useLocale();
  // ...
};

export default Overview;
```

**Tailwind Usage:**
- Dark theme: `bg-gray-950`, `bg-gray-900`, `text-gray-100`
- Responsive: `md:` prefix for desktop breakpoints (mobile-first)
- Status colors via mapped classes: `bg-green-500/20 text-green-400` for active
- Utility-first inline classes, no custom CSS (except `index.css` for Tailwind import)
- Plugin: `@tailwindcss/vite` (v4 style, integrated via Vite plugin)

**i18n System:**
- Two locales: `en` (English) and `zh-TW` (Traditional Chinese, default)
- Translation keys use dot-notation: `'nav.overview'`, `'overview.totalPnl'`
- `LocaleContext` provides `{ locale, setLocale, t }` throughout the app
- Translation function: `t(key: TranslationKey) => string`
- All user-facing strings must use translation keys, not hardcoded text
- Both locale files must stay in sync when adding new keys: `web/src/i18n/en.ts` and `web/src/i18n/zh-TW.ts`
- `TranslationKey` type is derived from the English locale file keys for type safety

**API Communication:**
- `useApi()` hook in `web/src/hooks/useApi.ts` for REST calls
- `useWebSocket()` hook in `web/src/hooks/useWebSocket.ts` for real-time updates
- Standard envelope: `{ ok: boolean, data?: T, error?: string }`
- Bearer token auth stored in `localStorage` key `arb_token`
- REST seeds initial state, WebSocket pushes incremental updates

**Frontend Build:**
- Vite + React (no SSR)
- Output to `web/dist/`, embedded in Go binary via `go:embed`
- Build: `npm run build` in `web/` directory (Node.js >= 22 required)
- Dev proxy: `/api` and `/ws` proxied to `http://localhost:8080`

## Module Boundaries

**Dependency Direction:** Always flows inward:
- `cmd/` depends on everything
- `internal/engine/` depends on `internal/discovery/`, `internal/risk/`, `internal/models/`, `internal/database/`, `pkg/exchange/`
- `internal/models/` defines interfaces (`Discoverer`, `RiskChecker`, `StateStore`) that concrete implementations satisfy
- `pkg/exchange/` is standalone (no `internal/` imports)
- `pkg/utils/` is standalone (no `internal/` imports)

**Interface-Driven Coupling:**
- Engine depends on `models.Discoverer` not `*discovery.Scanner`
- Engine depends on `models.StateStore` not `*database.Client`
- This allows test stubs to be injected

---

*Convention analysis: 2026-04-01*
