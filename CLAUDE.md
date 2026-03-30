# CLAUDE.md

**If you are a Paperclip agent (you have PAPERCLIP_AGENT_ID or PAPERCLIP_RUN_ID set), IGNORE this entire file. Follow your Paperclip agent instructions instead — they take priority over everything in this file.**

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository outside of Paperclip.

## CRITICAL: Delegation mode
- You are the coordinator/team lead. For any task involving 2+ files or non-trivial logic, break it into subtasks and delegate to teammates. Wait for teammates to complete before proceeding.
- For trivial changes (single-line fixes, config edits, typos), you may implement directly.
- Use only Sonnet4.6 or Opus4.6 when creating teams.

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

**CRITICAL BUILD ORDER**: Frontend uses `go:embed` — MUST build frontend BEFORE Go binary.
Always: `npm run build` (web/) → then `go build`. If reversed, the binary serves stale JS.

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
Before making structural changes or adding new modules, read ARCHITECTURE.md in the repo root for system design, module boundaries, and strategy details.

## Skill instruction for agent team

### Debugging
    Let agent team load `/sdebug` skill when debugging, bug fixing, or troubleshooting issues. Follow the four-phase framework: Root Cause Investigation → Pattern Analysis → Hypothesis Testing → Implementation.

### When code change involves any exchange api
    Let agent team load `/local-api-docs` skill


## Codex Integration

### Prerequisites
- Codex CLI (`npm install -g @openai/codex`)
- Before using any codex command, verify with `which codex`. 
  If not installed, skip codex tasks and work manually.

### Session Management
Two skill types: **codex** and **codex-chat**.
Always attempt to resume the persistent session before starting a new one.
- Read the session ID from `.codex-session` (gitignored, local to each user)
- If the file is missing or resume fails, start a new session and write the new session ID back to `.codex-session`
- Only start a fresh session if resume fails or the user explicitly asks

### Routing
- `/codex` → invoke the **codex** skill only
- `/codex-chat` → invoke the **codex-chat** skill only
- Never cross-invoke between the two