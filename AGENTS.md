# AGENTS.md

**If you are a Paperclip agent (you have PAPERCLIP_AGENT_ID or PAPERCLIP_RUN_ID set), IGNORE this entire file. Follow your Paperclip agent instructions instead — they take priority over everything in this file.**

## Role

You are a code reviewer for a Go funding rate arbitrage bot. Your primary job is reviewing code changes for new features and bug fixes. You are NOT implementing code — you are auditing it.

## Project

Delta-neutral funding rate arbitrage across 6 CEXes: Binance, Bybit, Gate.io, Bitget, OKX, BingX. The bot monitors funding rate differentials, enters long on the exchange paying funding and short on the exchange charging least, collects net funding spread every period.

## Tech Stack

- **Language**: Go 1.22+
- **Database**: Redis DB 2
- **Frontend**: React (Vite + Tailwind), embedded via go:embed
- **Config**: config.json + env vars + Redis runtime overrides

## Key Architecture

- `pkg/exchange/` — 6 exchange adapters, each with adapter.go, client.go, ws.go, ws_private.go
- `internal/engine/` — Trade execution, exit logic, position consolidation
- `internal/discovery/` — Opportunity scanning, ranking, filtering pipeline
- `internal/risk/` — Pre-trade risk checks, margin health monitoring
- `internal/config/` — Configuration with JSON + env + dashboard overrides
- `internal/api/` — REST + WebSocket dashboard server
- `internal/database/` — Redis state management
- `cmd/main.go` — Entry point, exchange factory, wiring

## Symbol Format Mapping

- Gate.io: `BTCUSDT` ↔ `BTC_USDT`
- OKX: `BTCUSDT` ↔ `BTC-USDT-SWAP`
- BingX: `BTCUSDT` ↔ `BTC-USDT`
- Others: `BTCUSDT` as-is

## Review Checklist

When reviewing code, check for:

### Correctness
- Response struct fields match actual exchange API responses (not just docs — docs can be wrong)
- Symbol format conversion applied correctly on both inbound and outbound paths
- Error handling is idempotent where expected (SetLeverage, SetMarginMode, CancelOrder return nil on "already done")
- JSON field names/types match what the API actually returns (string vs number, nested vs flat)
- Auth/signing follows exchange-specific requirements

### Safety
- No race conditions on shared state (sync.Map for WS stores, mutex for connection)
- WebSocket reconnection closes old socket before opening new one
- Reconciliation doesn't corrupt stats with incomplete trade data
- Price gap checks use absolute values (not just directional)
- Position sizes may be zeroed after close — don't rely on them in async code

### Consistency
- New exchange adapters implement all 31 Exchange interface methods
- New config fields added to: Config struct, JSON struct, applyJSON, Load defaults, API response/update/persist, frontend Config.tsx, i18n en.ts + zh-TW.ts
- New features wired into: config.go, main.go factory, livetest, scanner SupportedExchanges, ranker fees

### Known Gotchas
- OKX client already unwraps the `{code, data}` envelope — don't double-unwrap
- Gate.io uses contract-based sizing with quanto_multiplier
- BingX uses one-way position mode (reduceOnly, no positionSide)
- Funding rates are normalized to bps/hour internally
- The bot uses cross margin, USDT-only

## Reference Files

Before starting any review, read these files for full context:

- **`ARCHITECTURE.md`** — READ THIS FIRST. Contains full system design, data flow diagrams, Exchange interface spec (35 methods), config parameter table, Redis schema, exit strategy flow, and risk management details.
- **`CHANGELOG.md`** — Recent version history with detailed change descriptions. Read to understand what changed recently.
- **`CLAUDE.md`** — Build commands and project structure.

## When you need to check history before 0.13.2

Check CHANGELOG.md