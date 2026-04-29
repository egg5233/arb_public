# AGENTS.md

Agent instructions for working with code in this repository. Applies to Codex, codex-companion, and other non-Claude-Code agents that read `AGENTS.md` as their project instruction file.

**If you are a Paperclip agent (you have `PAPERCLIP_AGENT_ID` or `PAPERCLIP_RUN_ID` set), IGNORE this entire file. Follow your Paperclip agent instructions instead — they take priority over everything below.**

## Role

You are assisting with **Arb Bot**, a live funding-rate arbitrage system trading on 6 centralized exchanges. This is production trading code — real money, real positions. The CRITICAL rules below are not suggestions; follow them unconditionally. Most of this file describes project context that applies to both implementation and review tasks. A dedicated "Code review mode" section near the end gives extra checklist guidance when the task is specifically a code review.

## CRITICAL: npm security lockdown
**DO NOT run `npm install`, `npm update`, `npm upgrade`, `npx`, or `pnpm install` in the `web/` directory or anywhere in this repo.** The npm axios package has been compromised with a malicious dependency. All frontend dependencies are locked in `web/package-lock.json` — use ONLY `npm ci` (clean install from lockfile) if a fresh install is absolutely needed. Never add, remove, or update npm packages without explicit user approval.

## CRITICAL: Do not modify config.json
**DO NOT read, write, modify, or delete `config.json` in the repo root.** This is the live runtime configuration containing API keys, exchange credentials, and tuned trading parameters. Any accidental overwrite or value change can break live trading. If a task requires config changes, tell the user — never touch the file directly.

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
Before making structural changes or adding new modules, read `ARCHITECTURE.md` in the repo root for system design, module boundaries, and strategy details.

## graphify

Two pre-built knowledge graphs are available — use them before grepping the repo:

- `graphify-publish/` — Full codebase graph (46584 nodes, 108015 edges, rebuilt 2026-04-08). Covers code + docs across 7894 files. Start here for architecture, code flow, and cross-module questions.
- `graphify-out/` — Doc-only graph (122 nodes, high confidence). Use for exchange API comparisons, margin models, and risk design questions.

Routing — pick the right router for the task:
- **Architecture / codebase questions** → read `graphify-publish/AI_ROUTER.md` first (routes by intent to module + doc)
- **Code review / bug finding / regression detection** → read `graphify-publish/AI_REVIEW_ROUTER.md` first (routes by change area with focus checklists and high-risk patterns)
- **Exchange API specifics** → prefer `graphify-out/` (higher confidence edges)

Rules:
- Always start with the appropriate router above, not repo-wide grep
- `graphify-publish/wiki/` has 135 community pages — navigate those instead of reading raw files when available
- `GRAPH_REPORT.md` in graphify-publish may surface noisy frontend/tooling symbols — treat global god nodes as hints, not authority; stay anchored to module-scoped routing
- After modifying code files, let the agent decide whether a graphify refresh is warranted based on the scope and risk of the change. Prefer refreshing after broad structural, routing, or cross-module changes; skip it for narrow fixes, tests, docs, or when tests already cover the touched behavior. If a refresh is needed and the graphify Python module is available, the agent may run `python3 -c "from graphify.watch import _rebuild_code; from pathlib import Path; _rebuild_code(Path('.'))"`.

## Project

**Arb Bot — Unified Arbitrage System**

A funding rate arbitrage system that monitors funding rate differentials across 6 exchanges (Binance, Bybit, Gate.io, Bitget, OKX, BingX) and executes delta-neutral positions. Two strategy engines — perp-perp (live since mid-March 2026) and spot-futures (first trade 2026-04-01, partially working). The system is evolving from two independent engines into a unified arbitrage platform with intelligent capital allocation.

**Core Value:** "I deposit USDT, select my risk preference, and the system automatically finds opportunities across both strategies, opens positions, collects funding, exits when profitable, and I can see exactly how much each position earned."

### Constraints

- **Live system**: changes must not break existing perp-perp trading
- **Tech stack**: Go 1.26+, Redis DB 2, React/Vite/Tailwind, systemd deployment
- **npm lockdown**: axios compromise — only `npm ci`, never `npm install`
- **Build order**: frontend must build before Go binary (go:embed)
- **Exchange APIs**: rate limits, blackout windows (Bybit :04-:05:30), per-exchange quirks
- **Single server**: runs on one machine under systemd

## Technology Stack

Language/framework/dependency versions live in `go.mod` and `web/package.json` — don't hardcode them here. CLI tool inventory: `ls cmd/`. File-level structure: query graphify.

### Known drift
- Go 1.26+ required by `go.mod`. **`install.sh` still provisions 1.22.12** — fresh installs via the script will fail `go build` and need a Go upgrade first.

### Deployment
- Single Go binary (`./arb`) — self-contained, embeds frontend via `go:embed`
- Deployed via systemd (`install.sh` configures the service unit); detects `INVOCATION_ID` env var
- Binary drift monitor auto-restarts under systemd when the binary file is updated (`internal/api/drift_monitor.go`); exits code 1 to trigger `Restart=on-failure`
- Graceful shutdown on SIGINT/SIGTERM closes WS connections and stops engine components in reverse startup order
- Release updates via `scripts/pull-release.sh`

### Versioning
- `VERSION` file at repo root is authoritative
- Every commit must update both `CHANGELOG.md` and `VERSION`

## Conventions

Go style is standard `gofmt` + stdlib idioms — assume normal Go conventions unless noted below. For existing patterns, query graphify rather than listing examples here. The rules below are the project-specific ones that aren't derivable from reading the code.

### New feature rollout pattern
Any new risk/strategy feature requires **all three**:
1. Config boolean switch: `Enable{FeatureName}` field, `enable_{feature_name}` in JSON, **default OFF**
2. Dashboard UI toggle in the relevant tab
3. Persisted to `config.json` (Redis config persistence was removed 2026-04-05 — `config.json` is the sole source of truth)

Dashboard POST `/api/config` updates in-memory `*config.Config` and writes `config.json` with a `.bak` backup. Config reload on restart reads only `config.json` and environment variables.

### Internationalization (i18n)
- Two locales: `en` (English) and `zh-TW` (Traditional Chinese, default)
- All user-facing strings must use translation keys — no hardcoded text
- `web/src/i18n/en.ts` and `web/src/i18n/zh-TW.ts` **must stay in sync** when adding keys
- `TranslationKey` type is derived from the English locale file for compile-time safety
- `useLocale()` hook provides `{ locale, setLocale, t }` throughout the app

### Module boundaries (design rules, not description)
- `pkg/exchange/` and `pkg/utils/` are standalone — must not import `internal/`
- Engine depends on `models.Discoverer`, **not** `*discovery.Scanner`
- Engine depends on `models.StateStore`, **not** `*database.Client`
- Interface-based DI is deliberate: it allows test stubs to be injected. Don't leak concrete types across the boundary

### Exchange adapter rules (non-obvious)
- Internal symbol format is `BTCUSDT` (uppercase, no separators). Exchange-specific mapping happens *inside* each adapter (Gate.io `BTC_USDT`, OKX `BTC-USDT-SWAP`, BingX `BTC-USDT`)
- Callers always work in base-asset units (e.g., 480 BTC units). Contract-count conversion (OKX `ctVal`, Gate.io `quanto_multiplier`) happens inside the adapter
- `SpotMarginExchange` is an optional interface on 5 of 6 adapters (not BingX). Type-assert with `exch.(exchange.SpotMarginExchange)` to check support
- Each adapter has its own `APIError` struct. `isMarginError()` in `internal/engine/engine.go` detects margin errors via case-insensitive string matching across exchanges
- Custom sentinel: `ErrRepayBlackout` in `pkg/exchange/types.go`
- Methods on optional types (e.g., `*TelegramNotifier`) are safe to call on nil receivers

### Dashboard API envelope
- Standard JSON response: `{ ok: boolean, data?: T, error?: string }` via `writeJSON(w, status, Response{...})` in `internal/api/handlers.go`
- Bearer token auth in `localStorage` key `arb_token`
- REST seeds initial state, WebSocket (`/ws`) pushes incremental updates

## Architecture

For file-level structure, query graphify (start with `graphify-publish/AI_ROUTER.md`). The sections below are the **conceptual model + operational contracts** that aren't obvious from reading individual files.

### Pattern Overview
- **Two independent arbitrage engines** (perp-perp and spot-futures) sharing exchange adapters. The spot-futures engine has its own goroutines, Redis keys, and API routes — do not cross-couple
- **Scan-driven execution**: a fixed-minute scheduler fires discovery scans that dispatch to typed handlers (rebalance, exit, rotate, entry). See Scan Schedule below
- **Delta-neutral strategy**: always paired long/short positions across exchanges
- **Interface-driven DI**: concrete types hidden behind `Discoverer`, `RiskChecker`, `StateStore` in `internal/models/interfaces.go`
- Single Go binary embeds the React frontend via `go:embed`
- **Redis DB 2** is the sole *state* persistence layer. **`config.json`** is the sole *config* persistence

### Layers (purpose only; file-level detail in graphify)
- `pkg/exchange/` — unified 35-method `Exchange` interface + per-CEX adapters (Binance, Bybit, Gate.io, Bitget, OKX, BingX). Standalone package, no `internal/` imports
- `internal/discovery/` — polls funding rates, merges sources, verifies with 4 checks, ranks by profitability. Produces `ScanResult` on a channel consumed by the engine
- `internal/engine/` — main orchestration loop. Dispatches scan results to rebalance/exit/rotate/entry handlers. Central state holder: exit goroutines, entry guards, SL index, own-order tracking
- `internal/risk/` — pre-trade approval (11-point check), active monitoring, L0–L5 margin health tiers, cross-strategy capital allocation. Emits `HealthAction` on a channel
- `internal/spotengine/` — fully independent spot margin + futures hedge engine with its own discovery, risk gate, exit manager, autoentry. Conditional on `SpotFuturesEnabled`
- `internal/database/` — Redis persistence + distributed locks (SET NX + TTL + Lua scripts). Implements `models.StateStore`
- `internal/api/` — HTTP REST + WebSocket + embedded SPA + binary drift monitor
- `internal/models/` — shared types + interface definitions
- `internal/config/` — JSON + env var loading; `config.json` is sole source of truth
- `internal/notify/` — Telegram alerts (spot-futures only)
- `internal/scraper/` — headless Chrome scraper for CoinGlass data
- `pkg/utils/` — math helpers (`RoundToStep`, `EstimateSlippage`, `RateToBpsPerHour`) + structured logging. Standalone

### Startup order
`Scanner → RiskMon → HealthMon → API → Engine → SpotEngine (conditional)`. Graceful shutdown on SIGINT/SIGTERM reverses this order.

### Concurrency invariants (behavioral rules, not mutex list)
- Engine main loop is **single-goroutine**, consuming from `oppChan`
- **Consolidator skips** positions with `exitActive[posID]` or `entryActive["exchange:symbol"]` set — prevents interference with in-progress depth fills
- `capacityMu` serializes concurrent manual-open and automated-entry
- **Per-symbol Redis locks** prevent duplicate execution across restarts/instances
- **Exit goroutines are preemptable**: L4/L5 health actions cancel running exit via stored `CancelFunc` (500ms grace period)
- `UpdatePositionFields` uses atomic read-modify-write with a predicate function
- **BingX legs require 3 consecutive misses** before the consolidator acts — guards against transient empty-position API responses
- `ownOrders` sync.Map tracks engine-placed order IDs to avoid false SL triggers on own fills

### Error handling (design decisions)
- Stop-loss failures **never block** entry/exit/rotation
- Depth fill: circuit breaker after **5 consecutive `PlaceOrder` failures** on either leg. Partial fills are trimmed to matched portion
- Exit fallback: depth-fill timeout → **market IOC for remainder** (positions must close fully)
- L5 emergency close preempts any running exit goroutine (500ms grace)
- **SL detection is dual-method**: `slIndex` lookup + `ReduceOnly` fill verification — catches exchange-side stop triggers even when order IDs change
- PnL reconciliation retries 3 times at 5s/15s/30s
- SpotEngine uses `retryLeg()` with configurable attempts + backoff

### Scan Schedule
| Minute | Scan Type | Handler |
|--------|-----------|---------|
| :10 | NormalScan | Dashboard broadcast only |
| :20 | RebalanceScan | `rebalanceFunds()` |
| :30 | ExitScan | `checkIntervalChanges()` + `checkExitsV2()` |
| :35 | RotateScan | `checkRotations()` |
| :40 | EntryScan | `executeArbitrage(opps)` |
| :45, :50 | NormalScan | Dashboard broadcast only |

**Bybit has a :04–:05:30 blackout window** — avoid scheduling API calls there.

### Risk Tiers (L0–L5 margin health)
| Level | Threshold | Action |
|-------|-----------|--------|
| L0-None | No positions | — |
| L1-Safe | PnL ≥ 0 | — |
| L2-Low | PnL < 0, ratio < 0.50 | — |
| L3-Medium | ratio ≥ 0.50 | Transfer funds from healthiest exchange |
| L4-High | ratio ≥ 0.80 | Reduce positions by 50% |
| L5-Critical | ratio ≥ 0.95 | Emergency close all |

## Code review mode

When the task is to review code changes (rather than implement), apply this checklist in addition to everything above. Start by reading `graphify-publish/AI_REVIEW_ROUTER.md` for change-area-specific focus checklists.

### Correctness
- Response struct fields match actual exchange API responses (not just docs — docs can be wrong)
- Symbol format conversion applied correctly on both inbound and outbound paths
- Error handling is idempotent where expected (`SetLeverage`, `SetMarginMode`, `CancelOrder` return nil on "already done")
- JSON field names/types match what the API actually returns (string vs number, nested vs flat)
- Auth/signing follows exchange-specific requirements

### Safety
- No race conditions on shared state (`sync.Map` for WS stores, mutex for connection)
- WebSocket reconnection closes old socket before opening new one
- Reconciliation doesn't corrupt stats with incomplete trade data
- Price gap checks use absolute values (not just directional)
- Position sizes may be zeroed after close — don't rely on them in async code
- Respect the concurrency invariants in Architecture → "Concurrency invariants"

### Consistency
- New exchange adapters must implement all 35 `Exchange` interface methods
- New config fields must be added to: `Config` struct, JSON struct, `applyJSON`, `Load` defaults, API response/update/persist handlers, frontend `Config.tsx`, i18n `en.ts` + `zh-TW.ts`
- New features wired into: `internal/config/config.go`, `cmd/main.go` factory, `cmd/livetest/`, scanner `SupportedExchanges`, ranker fees

### Known gotchas
- OKX client already unwraps the `{code, data}` envelope — don't double-unwrap inside adapter code
- Gate.io uses contract-based sizing via `quanto_multiplier`
- BingX uses one-way position mode (`reduceOnly`, no `positionSide`)
- Funding rates are normalized to bps/hour internally — Loris rates are 8h-equivalent and must be divided by 8
- The bot uses **cross margin, USDT-only**

## Reference files
Before starting any non-trivial task, read:
- **`ARCHITECTURE.md`** (repo root) — full system design, data flow, Exchange interface spec (35 methods), config parameter table, Redis schema, exit strategy flow, risk management details
- **`CHANGELOG.md`** — recent version history with detailed change descriptions
