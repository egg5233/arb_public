# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Funding rate arbitrage bot — monitors funding rate differentials across **Binance, Bybit, Gate.io, Bitget, OKX, BingX** and executes delta-neutral positions (long low-rate exchange, short high-rate exchange).

## Tech Stack

- **Language**: Go 1.22+
- **Database**: Redis DB 2 (state, locks, position tracking)
- **Data source**: Loris API (`https://api.loris.tools/funding`) with exchange-native verification
- **Dashboard**: Go HTTP/WebSocket backend + React (Vite + Tailwind) frontend, embedded via go:embed
- **Config**: Environment variables
- **Node**: v22.13.0 via nvm (for frontend builds)

## Build & Run

```bash
# Full build (frontend + Go binary)
make build

# Dev mode (assumes frontend already built)
make dev

# Frontend only
make build-frontend

# Live exchange tests
go run ./cmd/livetest/ --exchange binance
go run ./cmd/livetest/ --exchange bitget --skip-orders
```

## Project Structure

Read ARCHITECTURE.md for system design, module structure, and strategy details.

## Codex Integration

When using the `/skill-codex:codex` skill, always resume the persistent Codex session by ID instead of starting a new one. After each run , extract the session ID from the most recent file in `~/.codex/sessions/` and store it below.

**Current session ID:** `019d22e3-2ad1-7a92-9ff7-f119dfab5334`

Only start a new session if resume fails or if the user explicitly asks for a fresh session. After starting a new session, update the session ID above.

## Debugging

Load `/sdebug` skill when debugging, bug fixing, or troubleshooting issues. Follow the four-phase framework: Root Cause Investigation → Pattern Analysis → Hypothesis Testing → Implementation.

## When you need to research about exchange api document

use playwright skills (headless=true) and check the link:
  Binance Futures:https://developers.binance.com/docs/derivatives/usds-margined-futures
  Binance Spot/Wallet:https://developers.binance.com/docs/wallet
  Bitget Contract:https://www.bitget.com/api-doc/contract/intro
  Bitget Spot/Wallet:https://www.bitget.com/api-doc/spot/intro
  Okx:https://www.okx.com/docs-v5/en/
  Gate.io:https://www.gate.com/docs/developers/apiv4/en/
  Bybit:https://bybit-exchange.github.io/docs/v5/guide
  BingX:https://bingx-api.github.io/docs-v3/#/en/info

## When you need to check history before 0.13.2

Check CHANGELOG.md
