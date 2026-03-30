# Changelog

All notable changes to this project will be documented in this file.

## [0.22.21] - 2026-03-31

### Fixed
- **[spot-futures] NegativeYieldSince now starts for zero/negative funding** — removed `fundingAPR > 0` guard from `negativeYield` computation in monitor loop; borrow-cost grace-period timer ([ARB-8](/ARB/issues/ARB-8)) now correctly ages when live funding is zero or negative, matching the [ARB-7](/ARB/issues/ARB-7) monitor spec (`monitor.go`)

### Changed
- **[spot-futures] Direction B price-spike semantics documented and tested** — added explicit canonical rule to ARCHITECTURE.md and corrected ambiguous wording in DESIGN_SPOT_FUTURES_RISK.md: adverse move for Direction B is price UP (short futures liquidation risk), not down; 9 focused regression tests added covering both directions' normal, emergency, no-trigger, and down-move scenarios (`exit_triggers_test.go`)

## [0.22.20] - 2026-03-30

### Fixed
- **yield_below_minimum exit fires when funding drops to zero** — replaced `currentFundingAPR > 0` guard with `hasFundingData` bool so the yield check runs for zero/negative funding when live data is available from `lookupCurrentOpp` (`exit_manager.go`)
- **Discovery logs warning on GetActiveSpotPositions error** — previously silent failure degraded to old broken filter behavior (`discovery.go`)
- **Deduplicated GetActiveSpotPositions call in discovery** — reuses the activeKeys map built for filter bypass instead of calling Redis twice per scan (`discovery.go`)

## [0.22.19] - 2026-03-30

### Fixed
- **Yield-below-minimum exit now fires correctly** — three combined bugs suppressed the exit trigger: (1) discovery filters dropped active positions whose rates degraded, starving monitor of fresh data; (2) `lookupCurrentFundingAPR` ignored zero/negative funding rates; (3) exit threshold omitted fee costs. Fixed by adding active-position bypass to 3 discovery filter gates, replacing `lookupCurrentFundingAPR` with `lookupCurrentOpp` returning the full opportunity, and including `feeAPR` in the yield threshold calculation (`discovery.go`, `monitor.go`, `exit_manager.go`, `spot_position.go`, `execution.go`)

## [0.22.18] - 2026-03-30

### Fixed
- **Spot-futures config persists to config.json on restart** — `SaveJSON()` now writes all 21 spot-futures fields to `config.json`, preventing silent revert of API-set values on service restart (`config.go`)
- **`spot_futures_exchanges` added to Redis persistence** — the `Exchanges` slice was missing from the Redis hash write; now persisted as comma-separated string (`handlers.go`)

## [0.22.17] - 2026-03-30

### Fixed
- **`/api/config` exposes spot-futures settings** — GET response includes full `spot_futures` block (omitted when engine disabled), POST accepts partial updates with validation, all fields persisted to Redis (`handlers.go`)

## [0.22.15] - 2026-03-30

### Fixed
- **Persistence counter deduplication per scan** — `updatePersistenceCounts` now checks the `seen` map before incrementing, so symbols appearing on N exchanges only increment once per scan instead of N times (`engine.go`)

## [0.22.14] - 2026-03-30

### Fixed
- **Bybit MarginRepay checks resultStatus before declaring success** — `MarginRepay` now parses the `resultStatus` field from Bybit's `/v5/account/no-convert-repay` response; returns error for `P` (processing) and `FA` (failed) so the engine correctly sets `PendingRepay=true` and retries (`pkg/exchange/bybit/margin.go`)

## [0.22.13] - 2026-03-30

### Fixed
- **Stuck exit emergency escalation fires on retry 5 not retry 6** — `isEmergency` check now uses post-increment value (`ExitRetryCount+1 >= 5`) instead of stale pre-increment struct (`monitor.go`)

## [0.22.12] - 2026-03-30

### Fixed
- **Direction A margin utilization formula** — changed from `borrowed/(borrowed+available)` to `borrowed/available`, fixing 5.67x–19x delay on 85%/95% safety triggers; added zero-available emergency exit (`exit_manager.go`)

## [0.22.11] - 2026-03-30

### Fixed
- **Funding-drop and negative-yield exits now see all active positions** — discovery scan top-N truncation now preserves entries that match active positions, so `lookupCurrentFundingAPR` never falls back to stale entry-time funding data for open trades (`discovery.go`)

## [0.22.10] - 2026-03-30

### Fixed
- **Manual open honors spot-futures dry-run toggle** — `ManualOpen` now checks `SpotFuturesDryRun` instead of the global `DryRun` flag, consistent with auto-entry risk gate (`execution.go`)

## [0.22.9] - 2026-03-30

### Fixed
- **Spot fill fallback returns expected quantity instead of zero** — `confirmSpotFill` now accepts `expectedQty` parameter and returns it when REST lookup fails (futures-only endpoint), instead of returning `0, 0` which caused false rollbacks on entry and stranded exits after accepted market IOC spot orders (`execution.go`, `monitor.go`)

## [0.22.8] - 2026-03-30

### Fixed
- **Monitor borrow rate bypasses 5-minute discovery cache** — `updateBorrowCost` now calls `getFreshBorrowRate` which always hits the exchange API instead of `getCachedBorrowRate` with its 5-min TTL; borrow-cost exits and negative-yield detection now react within one monitor tick (60s) instead of lagging up to 5 minutes (`monitor.go`, `discovery.go`)

## [0.22.7] - 2026-03-30

### Fixed
- **Direction A repay stall after partial buyback or accrued interest** — `retryPendingRepay` now queries `GetMarginBalance` for actual liability (principal + interest) and available balance; if the account is short, it buys the deficit via market IOC before retrying repay, preventing infinite retry loops with accruing interest (`monitor.go`)

## [0.22.6] - 2026-03-30

### Fixed
- **Spot-futures persistence filter keyed per symbol instead of per exchange** — changed Redis key from `arb:spot_persistence:{symbol}:{exchange}` to `arb:spot_persistence:{symbol}` so persistence counter no longer resets when the qualifying exchange rotates between scans; old composite keys auto-expire via 20-min TTL (`spot_state.go`, `engine.go`, `risk_gate.go`)

## [0.22.5] - 2026-03-30

### Fixed
- **Exit retry/fallback for spot-futures ClosePosition** — three-layer retry architecture prevents positions from being stranded in `exiting` status: (1) `retryLeg()` wrapper retries each close leg 3× with 2s delay, escalating to emergency market close on futures exhaustion; (2) monitor detects stuck exits after 2min and re-triggers `initiateExit`, promoting to emergency after 5 retries; (3) already-closed legs are skipped on retry to avoid double-closing (`execution.go`, `monitor.go`, `exit_manager.go`, `spot_position.go`)

## [0.22.4] - 2026-03-30

### Fixed
- **Spot-futures persistence filter now Redis-backed instead of process-local** — `updatePersistenceCounts` and `getPersistenceCount` use Redis keys (`arb:spot_persistence:{symbol}:{exchange}`) with 20-minute TTL instead of in-memory `map[string]int`; persistence history survives process restarts and is shared across runtimes as specified in ARB-9 (`engine.go`, `spot_state.go`)

## [0.22.3] - 2026-03-30

### Fixed
- **Spot-futures duplicate guard blocks per-symbol across all exchanges** — `checkRiskGate` and `ManualOpen` duplicate-position checks now compare symbol only (not symbol+exchange), preventing concurrent positions on the same asset across different exchanges as documented in ARCHITECTURE.md (`risk_gate.go`, `execution.go`)

## [0.22.2] - 2026-03-30

### Fixed
- **Direction B positions missing margin-health exit** — `checkExitTriggers` now checks futures-side margin ratio via `GetFuturesBalance()` for Direction B (`buy_spot_short`) positions; previously only Direction A had margin-health monitoring, leaving Direction B reliant on price-spike triggers alone (`exit_manager.go`)

## [0.22.1] - 2026-03-30

### Fixed
- **Bybit repay blackout leaves closed positions with outstanding borrow** — when `MarginRepay()` fails (e.g. Bybit 04:00–05:30 UTC blackout), position now stays in `exiting` state with `PendingRepay=true` instead of being marked `closed`; monitor loop retries repay on each tick and completes the exit once repay succeeds; fixes both normal and emergency close paths (`execution.go`, `exit_manager.go`, `monitor.go`, `spot_position.go`)

## [0.22.0] - 2026-03-30

### Added
- **Telegram Alerts** — auto-entry, auto-exit, and emergency close notifications sent via Telegram Bot API; configure with `telegram.bot_token` and `telegram.chat_id` in config.json or `TELEGRAM_BOT_TOKEN`/`TELEGRAM_CHAT_ID` env vars (`internal/notify/telegram.go`)
- **Separate-Account Exchange Support** — Binance and Bitget now supported in spot-futures with per-exchange capital limits; `spot_futures.separate_acct_max_usdt` (default: $200) for separate-account exchanges, `spot_futures.unified_acct_max_usdt` (default: $500) for unified exchanges
- **Dashboard: Spot-Futures Auto-Mode Toggle** — auto-entry on/off and dry-run toggle visible in main Overview dashboard with real-time status
- **Dashboard: Position Health Cards** — active spot-futures positions display margin utilization, borrow APR, net yield, borrow cost accrued, peak price move, and exit trigger proximity
- **Phase 3b Test Suite** — unit tests for capital allocation logic, PnL calculation (both directions), exit reason formatting, and Telegram notifier nil-safety

### Fixed
- **v0.21.9–v0.21.11 fixes included** — see below for individual entries

## [0.21.11] - 2026-03-30

### Fixed
- **Gate.io EnsureOneWayMode missing dual_mode param** — Gate.io expects `dual_mode` as a query parameter, not JSON body; changed from `POST /futures/usdt/dual_mode` with body `{"dual_mode":false}` to query param `?dual_mode=false` (`gateio/adapter.go`)

## [0.21.10] - 2026-03-30

### Fixed
- **Bitget rebalancer zero-amount transfer** — skip spot→futures transfer when amount < $1 instead of calling API with 0.0000 USDT; eliminates hourly `code=40020` errors (`engine.go`)## [0.21.10] - 2026-03-30

### Fixed
- **Bitget API signature failure for non-ASCII symbols** — Chinese character symbols (e.g. 龙虾USDT) caused HMAC signature mismatch due to `http.NewRequest` re-encoding percent-escaped UTF-8 bytes; fixed by setting `req.URL.RawQuery` directly to preserve exact query string used for signing (`client.go`)
## [0.21.9] - 2026-03-30

### Fixed
- **P0: Direction B price spike check inverted** — exit trigger was checking for DOWN moves instead of UP; short futures positions now correctly trigger on adverse price increases (`exit_manager.go`)
- **P0: Incomplete rollback on futures entry failure** — when futures long fails in `executeBorrowSellLong`, borrow is now repaid after spot rollback; prevents orphaned borrows accruing interest (`execution.go`)
- **P0: Zero exit price propagated to PnL** — `confirmFuturesFill` and `confirmSpotFill` could return avg price 0; close flows now fall back to orderbook mid-price, preventing wildly incorrect PnL calculations (`execution.go`)
- **P1: TOCTOU race in UpdateSpotPositionFields** — added per-position mutex via `lockedUpdatePosition` to prevent concurrent monitor tick + exit goroutine from losing each other's writes (`engine.go`, `monitor.go`, `exit_manager.go`)
- **P1: Entry/exit fees never populated** — PnL formula subtracted fees that were always 0; now calculated from fill data using per-exchange taker rates (`execution.go`)
- **P1: Data race on spotOpps** — `spotOpps` slice read/written from different goroutines without sync; replaced with `atomic.Value` (`server.go`, `spot_handlers.go`)

## [0.21.8] - 2026-03-30

### Added
- **Spot-Futures Phase 3b.3 — Risk Gate + Automated Entry with Persistence Filtering**
  - **Risk gate** (`internal/spotengine/risk_gate.go`): 5 pre-entry checks — dry-run, capacity, duplicate, cooldown, persistence — all must pass before auto-entry proceeds
  - **Automated entry** (`internal/spotengine/autoentry.go`): sequential entry orchestration from discovery loop, one entry per scan cycle, distributed lock prevents concurrent entries
  - **Persistence filter**: symbols must appear in N consecutive discovery scans (default: 2) before auto-entry; prevents single-scan spike entries
  - **Dashboard toggle**: `POST /api/spot/config/auto` with `enabled`, `dry_run`, `persistence_scans` fields; `GET` returns current state
  - **Config fields**: `SpotFuturesAutoEnabled` (default: false), `SpotFuturesDryRun` (default: true), `SpotFuturesPersistenceScans` (default: 2)

### Changed
- Discovery loop now updates persistence counts and attempts auto-entries after each scan
- Files: `internal/spotengine/risk_gate.go` (new), `internal/spotengine/autoentry.go` (new), `internal/spotengine/engine.go`, `internal/config/config.go`, `internal/api/spot_handlers.go`, `internal/api/server.go`

### Safety
- Double safety default: auto-entry disabled AND dry-run enabled — must explicitly enable both
- Auto-entry config resets to disabled on process restart (intentional for high-risk feature)
- Cooldown check enforced: `HasSpotCooldown` now blocks auto re-entry after losses (was set but never checked at entry)

## [0.21.7] - 2026-03-30

### Removed
- **Pre-settlement T-10s API check**: Removed `schedulePreSettlementCheck` and `ReversalPreSettlement` config toggle — rates are too volatile near settlement boundaries for reliable decisions

### Fixed
- **Pre-settlement reversal respects tolerance**: Pre-settlement check now compares `ReversalCount` against `SpreadReversalTolerance` (e.g. tolerance=3 means skip exit when count < 3). Previously exited on any count >= 1
- **Zero-spread post-settlement confirmation**: When zero-spread count reaches tolerance, instead of exiting immediately, schedules a check 2 minutes after next funding settlement. If spread recovered, resets count; if still zero, then exits. Prevents premature exits when rates often diverge after settlement
- **Zero-spread epsilon consistency**: Post-settlement zero check now uses symmetric `|spread| < epsilon` matching `checkZeroSpread`, so negative non-zero spreads are no longer misclassified as zero

### Added
- **Spot-Futures Phase 3b.2 — Automated Exit System**: 5 exit triggers evaluated on each monitor tick + emergency close with parallel leg closure
  - **Borrow Cost Drift** (Dir A): exits when borrow APR exceeds max or negative yield persists beyond grace period (`SpotFuturesBorrowGraceMin`, default 30m)
  - **Funding Rate Drop** (both dirs): exits when net yield drops below `SpotFuturesMinNetYieldAPR`; falls back to entry-time funding APR when symbol drops from scan
  - **Price Spike** (both dirs): normal exit at `SpotFuturesPriceExitPct` (20%), emergency at `SpotFuturesPriceEmergencyPct` (30%); tracks `PeakPriceMovePct`
  - **Margin Health** (Dir A): exits at `SpotFuturesMarginExitPct` (85%), emergency at `SpotFuturesMarginEmergencyPct` (95%); tracks `MarginUtilizationPct`
  - **Manual Close**: `POST /api/spot/close` endpoint for dashboard-triggered position close
- **Emergency Close**: parallel leg closure with 5-second hard timeout, market IOC only, accepts any slippage
- **Post-Exit Processing**: direction-aware PnL calculation, per-symbol loss cooldown (`SpotFuturesLossCooldownHours`, default 4h), history recording, stats update
- **Exit tracking fields**: `ExitTriggeredAt`, `ExitCompletedAt`, `PeakPriceMovePct`, `MarginUtilizationPct` on SpotFuturesPosition
- **Redis cooldown keys**: `arb:spot_cooldown:{symbol}` with TTL for re-entry prevention after losses

### Changed
- Monitor loop now checks exit triggers for both Direction A and B positions (previously only updated borrow costs for Dir A)
- Borrow cost tracking extracted to `updateBorrowCost()` helper for cleaner separation
- Files: `internal/spotengine/exit_manager.go` (new), `internal/spotengine/monitor.go`, `internal/spotengine/engine.go`, `internal/spotengine/execution.go`, `internal/models/spot_position.go`, `internal/database/spot_state.go`, `internal/config/config.go`, `internal/api/spot_handlers.go`, `internal/api/server.go`, `cmd/main.go`

## [0.21.6] - 2026-03-30

### Added
- **Spot-Futures Phase 3b.1 — Monitor Loop + Borrow Cost Tracking**: Position monitor goroutine runs every `SpotFuturesMonitorIntervalSec` (default now 60s), refreshes borrow rates from exchange via `GetMarginInterestRate`, calculates accrued borrow cost in USDT, tracks negative yield periods (`NegativeYieldSince`), and alerts when borrow APR exceeds `SpotFuturesMaxBorrowAPR`
- **Position health endpoint** (`GET /api/spot/positions/{id}/health`): Returns current borrow cost, funding income, net yield APR, hours open, and negative yield duration
- **WebSocket health broadcasts** (`spot_position_health` message type): Real-time position health updates pushed to all connected dashboard clients on each monitor tick
- **Monitor fields on SpotFuturesPosition**: `LastBorrowRateCheck`, `CurrentBorrowAPR`, `BorrowCostAccrued`, `NegativeYieldSince`, `FundingAPR`
- **Entry-time rate capture**: `ManualOpen` now records `FundingAPR`, `BorrowRateHourly`, and `CurrentBorrowAPR` from the discovery opportunity at position creation

### Changed
- `SpotFuturesMonitorIntervalSec` default reduced from 300s to 60s (used by new monitor loop)
- Files: `internal/spotengine/monitor.go` (new), `internal/spotengine/engine.go`, `internal/spotengine/execution.go`, `internal/models/spot_position.go`, `internal/api/spot_handlers.go`, `internal/api/server.go`, `internal/config/config.go`

## [0.21.5] - 2026-03-30

### Added
- **`ReversalPreSettlement` config param** (bool, default true): Controls whether `schedulePreSettlementCheck` is called during the spread reversal tolerance window. Configurable via dashboard toggle and JSON config
- **`ReversalResetOnRecover` config param** (bool, default true): Controls whether the reversal count resets to zero when the spread recovers to positive. Configurable via dashboard toggle and JSON config
- **CoinGlass per-leg rate derivation**: Discovery scanner now derives approximate long/short rates from CoinGlass `FundingRate` field instead of leaving them as zero

### Changed
- Files: `internal/config/config.go`, `internal/engine/exit.go`, `internal/api/handlers.go`, `internal/discovery/scanner.go`, `web/src/pages/Config.tsx`, `web/src/i18n/en.ts`, `web/src/i18n/zh-TW.ts`

## [0.21.4] - 2026-03-29

### Added
- **EnableSpreadReversal toggle**: Spread reversal exit check now has a dashboard-controllable on/off switch (default: on). Pre-settlement timer also respects the toggle

## [0.21.3] - 2026-03-29

### Added
- **Spot-Futures Phase 3a — Manual entry execution** (`POST /api/spot/open`): Supports both directions (borrow-sell+long and buy-spot+short), market IOC orders, fill confirmation, rollback on failure. New `internal/spotengine/execution.go` with `ManualOpen` method. Engine caches latest discovery results for lookups; `SetSpotOpenHandler` callback registered in `cmd/main.go`
- **Zero-spread exit**: Triggers exit when both legs have equal funding rate (spread ≈ 0) for N consecutive checks (`ZeroSpreadTolerance`, default 2, dashboard configurable)
- **Pre-settlement check**: Schedules a timer 10s before funding settlement; if `ReversalCount ≥ 1` and spread still negative, exits immediately to avoid paying another negative funding fee
- **Dashboard UI**: Added zero-spread tolerance config field with i18n (en + zh-TW)

### Fixed
- **Spread reversal exit blocked by min-hold gate**: Moved spread reversal check before the min-hold gate in `checkExitsV2` so it is never blocked by continuously-advancing `NextFunding`
- **Pre-settlement timer restored**: Re-added with inline spread calc (bypasses ±10min guard), dedup map, only fires when ReversalCount ≥ 1
- **Login mobile UX**: Removed `autoFocus` to match byreal login style, use placeholder instead
- **Spot-futures discovery: OKX unborrowable coins passing filters**: OKX interest rate API returns rates for coins that can't actually be borrowed (e.g. SENT). Added borrowability verification via `GetMarginBalance(MaxBorrowable > 0)` for Direction A. For Direction B (buy spot + short), removed over-aggressive margin check since no borrowing is needed
- **Bybit borrow rate endpoint returning 0%**: Switched from `/v5/account/collateral-info` (returned 0% within free quota) to `/v5/crypto-loan-common/loanable-data` (returns actual `flexibleAnnualizedInterestRate`). Discovery log renamed "Borrow" to "Interest"

## [0.21.0] - 2026-03-29

### Added — Spot-Futures Arbitrage Engine (Phase 1-2)
- **New `internal/spotengine/` package**: Runs alongside existing perp-perp engine — separate discovery loop, separate position tracking, independent lifecycle
- **Data model** (`internal/models/spot_position.go`): `SpotFuturesPosition` struct supporting both directions — Direction A (borrow-sell + long perp, negative funding) and Direction B (buy-spot + short perp, positive funding). Tracks spot leg, futures leg, borrow costs, funding collected, and P&L
- **Redis CRUD** (`internal/database/spot_state.go`): Separate key namespace (`arb:spot_positions`, `arb:spot_positions:active`, `arb:spot_history`, `arb:spot_stats`). Full CRUD: `SaveSpotPosition`, `GetSpotPosition`, `GetActiveSpotPositions`, `AddToSpotHistory`, `UpdateSpotStats`, `GetSpotHistory`, `GetSpotStats`, `UpdateSpotPositionFields` (atomic read-mutate-write)
- **Discovery scanner** (`internal/spotengine/discovery.go`): Reads CoinGlass spot arb data from Redis, queries live borrow rates via `SpotMarginExchange` interface, calculates net yield (`fundingAPR − borrowAPR − feeAPR`), ranks by net APR, filters by `min_net_yield_apr` and `max_borrow_apr`. Borrow rate cache with 5min TTL
- **Dashboard API** (`internal/api/spot_handlers.go`): Four new authenticated endpoints — `GET /api/spot/positions`, `/api/spot/history`, `/api/spot/stats`, `/api/spot/opportunities`. WebSocket broadcast for spot position updates
- **Config** (`internal/config/config.go`): New `spot_futures` JSON section with `enabled`, `max_positions`, `capital_per_position`, `leverage`, `monitor_interval_sec`, `min_net_yield_apr`, `max_borrow_apr`, `exchanges`, `scan_interval_min`. Env var overrides: `SPOT_FUTURES_ENABLED`, `SPOT_FUTURES_MAX_POSITIONS`, `SPOT_FUTURES_CAPITAL_PER_POSITION`, `SPOT_FUTURES_LEVERAGE`, `SPOT_FUTURES_MONITOR_INTERVAL`
- **Engine integration** (`cmd/main.go`): SpotEngine created and started when `SpotFuturesEnabled=true`, graceful shutdown in reverse order
- **Codex review fixes**: Stale data guard rejects CoinGlass data with unparseable or bad timestamps; discovery results pushed to API endpoint for `/api/spot/opportunities`
- **Gate.io PM support**: `isPM` flag, `SetMarginMode`/`SetLeverage` no-op for portfolio margin mode
- **Risk design document**: `doc/DESIGN_SPOT_FUTURES_RISK.md` — covers extreme condition scenarios (10x squeeze, liquidation cascades) for both directions, mitigation strategies

## [0.20.2] - 2026-03-29

### Added — CoinGlass Spot-Futures Arbitrage Scraper
- **Embedded scraper** (`internal/scraper/spotarb.go`): Scrapes CoinGlass ArbitrageList page using headless Chrome (chromedp) for spot-sell + futures-long arbitrage opportunities
- **Configurable via `config.json`** under `"spot_arb"` section: `enabled`, `schedule` (comma-separated minutes), `chrome_path`
- **Env var overrides**: `SPOT_ARB_ENABLED`, `SPOT_ARB_SCHEDULE`, `SPOT_ARB_CHROME_PATH`
- **Redis output**: Writes scraped data to Redis key `coinGlassSpotArb` as JSON with timestamp and opportunity count
- **Standalone CLI** preserved at `cmd/spotarb/main.go` — imports shared `internal/scraper` package, supports `--cron`, `--no-redis`, `--json` flags
- **Chrome auto-detection**: Scans common Puppeteer/Playwright cache locations and system paths for Chrome binary
- Runs as embedded goroutine on configurable schedule (default: minutes 15, 35 each hour), with immediate run on startup

## [0.20.1] - 2026-03-29

### Reverted — Binance Portfolio Margin (PM)
- **Removed PM auto-detection, papi path remapping, PM balance parsing, PM conditional order handling, `CONDITIONAL_ORDER_TRADE_UPDATE` WS handler** — Binance adapter now uses classic fapi endpoints only. PM mode was too complex with limited benefit (no TP/SL in standard PM, different API surface)
- Removed `doc/EXCHANGEAPI_BINANCE_PM.md` and `cmd/testpm/`
- **Gate.io PM support retained** — `SetMarginMode`/`SetLeverage` no-op for portfolio mode still works

## [0.20.0] - 2026-03-29

### Added — Binance Portfolio Margin (PM) Support
- **Auto-detection**: `NewAdapter` probes `/sapi/v1/account/apiRestrictions` for `enablePortfolioMarginTrading`; classic users completely unaffected — all changes gated behind `isPM`
- **Dual-client architecture**: `client` points to `papi.binance.com` for signed endpoints; `fapiClient` stays on `fapi.binance.com` for public endpoints (depth, exchangeInfo, premiumIndex, fundingInfo)
- **Path remapping**: `remapPath()` converts `/fapi/v1/*` → `/papi/v1/um/*` with special cases for balance (`/papi/v1/balance`), conditional orders (`/papi/v1/um/conditional/order`), and listenKey (`/papi/v1/listenKey`)
- **PM balance parsing**: `parsePMBalance` uses `/papi/v1/balance` for per-asset totals + `/papi/v1/account` for `virtualMaxWithdrawAmount` (available) and margin ratio
- **PM stop-loss**: Uses `strategyType`/`stopPrice`/`reduceOnly` instead of `algoType`/`triggerPrice`/`closePosition`; returns `strategyId`
- **PM cancel stop-loss**: Uses `strategyId` + `symbol` instead of `algoId`
- **PM margin mode**: `SetMarginMode` is a no-op for PM (unified margin)
- **PM WebSocket**: Uses `wss://fstream.binance.com/pm/ws/{listenKey}` and `/papi/v1/listenKey`
- **`CONDITIONAL_ORDER_TRADE_UPDATE` handler**: New WS event type for PM conditional order (SL/TP) triggers, fires `OrderUpdate` callback with strategyId
- **PM API docs**: `doc/EXCHANGEAPI_BINANCE_PM.md`
- **PM test utility**: `cmd/testpm/main.go`

## [0.19.5] - 2026-03-29

### Added — SL Detection Overhaul
- **`ReduceOnly` and `Symbol` fields on `OrderUpdate`**: All 6 exchange WS private handlers now parse reduce-only/close flags and symbol from order fill events (Binance `ro`+`s`, Bybit `reduceOnly`+`symbol`, OKX `reduceOnly`+`instId`, Gate.io `is_reduce_only`/`is_close`+`contract`, Bitget `reduceOnly`+`instId`, BingX `ro`/`STOP_MARKET`+`s`)
- **Dual SL detection in `handleSLFill`**: Method 1 — original slIndex order ID lookup. Method 2 — ReduceOnly fill detection: when a reduce-only fill arrives for a symbol we hold and we didn't initiate it, verifies the leg is actually flat on the exchange before triggering emergency close
- **`ownOrders` sync.Map**: Tracks bot-initiated order IDs (exits, trims, rotation closes, consolidator) to prevent false SL triggers from our own reduce-only fills
- **`entryActive` guard**: Skips ReduceOnly detection during entry fills to avoid false triggers
- **`triggerEmergencyClose` helper**: Extracted from `handleSLFill` for shared use by both detection methods

### Fixed
- **Consolidator PnL on close**: `markPositionClosed` now queries `GetClosePnL` from both exchanges to populate `LongExit`, `ShortExit`, `RealizedPnL`, and calls `UpdateStats` so consolidator-closed positions count in win/loss tracking. Previously these had zero exit prices and zero PnL in history
- **Dashboard funding sort order**: Reversed to oldest-first (was newest-first); latest date group auto-opens instead of oldest
- **Dashboard funding timezone**: Date grouping now uses Asia/Taipei (UTC+8) instead of UTC

## [0.19.4] - 2026-03-29

### Improved
- **Dashboard dual timezone**: Funding history and open time now show both UTC and UTC+8 (e.g. `12:00 / 20:00 +8`)

## [0.19.3] - 2026-03-28

### Fixed — Exit Optimization (Phase 1)
- **Depth subscription hardened**: 8s wait with unsub/resub retry (was 2s no retry); ctx-aware for L4/L5 preemption; no-depth falls back to market immediately
- **Progress-aware timeout**: 30s no-fill early break instead of fixed 300s timeout
- **Market fallback**: `closeFullyWithRetryPriced` (retry up to 10x with actual fill prices) replaces one-shot `executeMarketClose`
- **SL cancel timing**: cancelled after depth loop, before market fallback (was after everything)
- **50bps hard cap**: gap gate clips at 50bps; breaks to market after 60% of timeout while capped
- **Tick interval**: 100ms (was 200ms)
- **formatSize**: exchange-aware in all exit/close paths (was hardcoded 6 decimal)
- **Partial close safety**: reverts to `active` with remaining sizes + SL reattach (was unconditionally marking closed)
- **L4/L5 preemption**: done-channel handshake replaces `cancel()+sleep(500ms)`; 6 ctx check points throughout exit flow

### Fixed — Trading Logic Audit (16 issues from Codex gpt-5.4 audit)
- **(C1)** `closePositionWithMode` verifies both legs flat on exchange before marking closed; treats verification errors as not-flat; reattaches SLs on rollback
- **(C2)** Entry unwind: all `closeLeg` calls replaced with `closeFullyWithRetry` (verified retry)
- **(C3)** Exit depth loop: no longer decrements `closedLong` when short under-fills (was falsifying bookkeeping)
- **(C4)** Rotation: old leg close uses verified retry; aborts on verification failure; writes remaining size back
- **(C5)** `updateRotationStopLoss`: unregisters old SL from slIndex before cancel; registers new SL after placement
- **(H1)** Consolidator `markPositionClosed`: verified close + treats API errors as not-flat
- **(H2)** Consolidator `enforceBalance`: uses actual filled amount instead of assuming full trim
- **(H3)** Entry: formatSize per-leg (long uses long exchange rules, short uses short)
- **(H4)** Risk: sizing enforces stricter of both exchanges' StepSize/MinSize
- **(H5)** Emergency/manual close: SLs cancelled first; both legs fire concurrently via WaitGroup
- **(H6)** L3 cross-exchange transfer: returns early (was executing meaningless intra-exchange transfers)
- **(M1)** Reconcile: `AdjustPnL` adjusts total_pnl only, no longer increments trade/win/loss counts
- **(M2)** `ReversalCount` resets to 0 when spread recovers
- **(M3)** `checkRotations` scans all same-symbol opportunities, picks best spread improvement
- **(M4)** Duplicate symbol check blocks any non-closed status (was only blocking active)
- **(M5)** Exposure cap documented as intentionally per-pair conservative
- **Consolidator orphan close**: all `closeLeg` replaced with `closeFullyWithRetry`

## [0.19.2] - 2026-03-28

### Fixed
- **Bitget `GetFundingFees` symbol filter**: Bitget's bill API (`/api/v2/mix/account/bill`) returns bills from all symbols despite the `symbol` query param. Added `symbol` field parsing from bill response and filter to only include bills matching the requested symbol. This caused dashboard funding history totals to not match `funding_collected` (e.g. showing 4.01 USDT instead of correct 0.99 USDT for Bitget side)
- **Funding history time-bounded leg windows**: `handleGetPositionFunding` now scopes each leg's funding query to the time window the position actually held that leg (start/end from rotation timestamps), preventing account-wide funding payments from other positions leaking into the history

## [0.19.1] - 2026-03-28

### Fixed
- **BingX `GetSpotBalance`**: Now queries Fund account (`/openApi/fund/v1/account/balance`) instead of Spot account — deposits land in Fund after BingX's May 2025 account split. Uses `json.Number` for free/locked fields
- **BingX `TransferToFutures`/`TransferToSpot`**: Now use `/openApi/api/v3/post/asset/transfer` with `type=FUND_PFUTURES`/`PFUTURES_FUND` — old endpoint (`innerTransfer/apply`) was for inter-user transfers, not intra-account
- **Bybit `TransferToFutures`**: Now transfers FUND → UNIFIED via `/v5/asset/transfer/inter-transfer` — was incorrectly a no-op, but deposits land in FUND account, not UNIFIED
- **Binance `GetFuturesBalance`**: Now uses `marginBalance` (includes unrealized PnL) instead of `walletBalance` (deposits only) for Total — dashboard was showing equity without uPnL
- **Gate.io dashboard balance**: Removed incorrect override that replaced futures balance with spot balance when `IsUnified()` returned false

## [0.19.0] - 2026-03-28

### Added
- **SpotMarginExchange interface** (`pkg/exchange/types.go`): New optional interface for spot margin borrowing (borrow-sell-buyback-repay) with types `MarginBorrowParams`, `MarginRepayParams`, `SpotMarginOrderParams`, `MarginInterestRate`, `MarginBalance`
- **Spot margin adapters** (`margin.go`): Implemented for 5 exchanges — Binance, Bybit, OKX, Gate.io (unified `/unified/*` endpoints for cross margin), Bitget. Each implements `MarginBorrow`, `MarginRepay`, `PlaceSpotMarginOrder`, `GetMarginInterestRate`, `GetMarginBalance`, `TransferToMargin`, `TransferFromMargin`. BingX excluded (no spot margin support)
- **Margin API documentation**: `EXCHANGEAPI_{EXCHANGE}_MARGIN.md` for all 5 exchanges, `EXCHANGEAPI_GATEIO_UNIFIED.md` for Gate.io unified account endpoints, `SPEC_BINANCE_SPOT_MARGIN.md` for Binance spot margin spec

### Fixed
- **Consolidator BingX false positives**: BingX legs now require 3 consecutive "missing" detections (`bingxMissThreshold`) before force-closing, preventing false positives from transient API glitches returning empty position arrays
- **`markPositionClosed` closes both legs**: Now attempts to close both legs including the one reported as "missing" (re-queries exchange, falls back to local size), guarding against transient API errors leaving orphan positions

## [0.18.9] - 2026-03-28

### Fixed
- **Funding history ordering**: Truncate timestamps to hour before comparing so different exchanges' sub-second differences don't break tiebreaker sort

## [0.18.8] - 2026-03-28

### Fixed
- **Funding history ordering**: Added stable sort tiebreaker (side) so entries within the same timestamp always display in consistent order (long before short)
- **pull-release.sh**: Removed `--version` probe that accidentally launched a full bot instance, causing port conflict and false update failure

## [0.18.7] - 2026-03-28

### Changed
- **Update mechanism**: `handleUpdate` now uses `pull-release.sh` to download pre-built binaries from GitHub Releases instead of `make build` on server

## [0.18.6] - 2026-03-27

### Added
- **`scripts/pull-release.sh`** — downloads the latest release binary from GitHub. Usage: `./scripts/pull-release.sh [--restart]`
- **Funding history header shows position-level `FundingCollected`** (authoritative) instead of account-wide sum. Removed misleading running total

## [0.18.5] - 2026-03-27

### Fixed
- **`checkIntervalChanges()` respects `AllowMixedIntervals` config**: When `false` (default), exits strictly on any interval mismatch (original fc99679d behavior). Spread-aware logic (keep position if spread positive) only applies when `AllowMixedIntervals=true`

## [0.18.4] - 2026-03-27

### Improved
- **Funding history section redesigned**: Date-grouped collapsible layout with daily subtotals, running totals, side badges (green/red pills), scrollable container, and grand total header. Replaces flat raw table that was hard to read

## [0.18.3] - 2026-03-27

### Added
- **Positions dashboard — expandable detail row**: Click any position to reveal per-leg unrealized PnL, funding payment history, and rotation timeline
- **`GET /api/positions/{id}/funding` endpoint**: Returns per-payment funding history from all exchanges, including rotated-away legs
- **Per-leg unrealized PnL**: `long_unrealized_pnl` and `short_unrealized_pnl` updated every funding cycle, broadcast via WebSocket
- **Rotation history array (`RotationRecord`)**: Tracks full rotation timeline with From, To, LegSide, PnL, Timestamp
- **Absolute open time**: Displayed in position detail row

### Fixed
- **Rotation PnL backfill race**: `RotationRecord.PnL` changed from `float64` to `*float64` (nil = not reconciled, 0 = real zero). Backfill now matches by From exchange + LegSide instead of last array index
- **Frontend funding fetch race**: Added `useRef` to track active position ID, stale responses are discarded
- **Unrealized PnL preserved on API failure**: Defaults to previous value instead of zero when `GetPosition()` fails
- **Unrealized PnL aggregation**: Accumulates across multiple position rows instead of overwriting
- **Funding history endpoint includes rotated-away exchanges**: Now pulls from `RotationHistory`, not just current pair

## [0.18.2] - 2026-03-27

### Added
- **Spread-aware interval monitor**: `checkIntervalChanges()` now checks live spread before exiting on interval mismatch. If spread is still positive (collecting side benefits from shorter interval), the position is kept open instead of exiting. Updates `NextFunding` to the collecting side's settlement time
- **`AllowMixedIntervals` config toggle** (default `false`): When enabled, ranker allows cross-interval pairs (e.g., 1h long + 4h short). The smarter interval monitor protects live positions by only exiting if spread goes negative
- **Exit reason in History dashboard**: New "Exit Reason" / "平倉原因" column shows why each position was closed (spread reversal, interval mismatch, manual close, etc.). `ExitReason` field added to position model, stored when `spawnExitGoroutine()` is called

### Improved
- `checkIntervalChanges()` now uses `GetFundingRate()` instead of `GetFundingInterval()` — gets rate + interval in one call, reducing API traffic
- Added settlement-window guard to interval check (skip within ±10min of funding, same as spread reversal check)
- Removed blanket 8h interval skip — real 8h intervals are now monitored correctly

## [0.18.1] - 2026-03-27

### Fixed
- **Bitget `PlaceStopLoss` `reduceOnly` casing**: Was lowercase `"yes"` — changed to `"YES"` to match Bitget API spec (second occurrence missed in earlier fix at line 1090)
- **Build error from two `main` functions in `scripts/`**: Moved standalone debug scripts to separate subdirectories (`scripts/query-bingx-pnl/main.go`, `scripts/query-bitget-pnl/main.go`) so each has its own package

## [0.18.0] - 2026-03-28

### Added
- **Cross-leg interval matching in ranker**: Pairs are now grouped by funding interval before selecting best long/short. Mixed-interval pairs (e.g. 1h vs 4h) are no longer ranked as opportunities. Falls back to best same-interval pair instead of discarding the symbol entirely
- **Interval change monitor**: `checkIntervalChanges()` runs before every ExitScan, queries both legs' funding intervals via exchange APIs. Triggers position exit if intervals diverge (>30min difference). Skips 8h default values to avoid false exits from silent adapter fallbacks

## [0.17.8] - 2026-03-27

### Fixed
- **BingX `GetClosePnL` double-counted funding fees**: `netProfit` already includes `realisedProfit + positionCommission + totalFunding`, but adapter added `totalFunding` again (`NetPnL: netPnL + funding`). This caused reconciliation to overwrite correct PnL with inflated losses. Example: BARDUSDT showed -$2.53 instead of correct -$0.08

### Added
- **Gap-gated exit**: `executeDepthExit` now checks cross-exchange gap before placing orders, mirroring entry's gap-aware pattern. Single bounded ramp from `ExitMaxGapBPS` (default 10 bps) to 3× over timeout. Spread-capped depth aggregation. Unfillable remainder check (remaining < step/min size)
- **Min-hold gate**: `checkExitsV2` suppresses exits before first funding settlement (`NextFunding`). Manual close and margin emergencies bypass this gate
- **`ExitMaxGapBPS` config** (default 10.0, JSON: `strategy.exit.max_gap_bps`): Maximum cross-exchange gap in basis points allowed for exit depth-fill orders
- **Debug scripts**: `scripts/query_bingx_pnl.go`, `scripts/query_bitget_pnl.go` for exchange PnL investigation

### Documentation
- BingX annotation: `netProfit` includes all components, do not add funding again

## [0.17.7] - 2026-03-26

### Fixed
- **Margin safety buffer**: `Approve()` now requires `available >= requiredMargin × safetyMultiplier` (configurable via `margin_safety_multiplier`, default 2.0). Prevents positions from opening with borderline margin that immediately triggers L5 emergency close after fees are deducted
- **OKX TransferToFutures was a no-op**: OKX has separate funding account (type 6) and trading account (type 18) even below 10000 USDT unified threshold. `TransferToFutures` now calls `/api/v5/asset/transfer` to move funds from funding → trading
- **OKX dashboard balance display**: Removed OKX from `unifiedExchanges` map so dashboard shows both trading and funding account balances separately

### Added
- **`margin_safety_multiplier` config**: Configurable multiplier for margin entry check (Dashboard → Risk), replaces hardcoded L3-derived buffer

## [0.17.6] - 2026-03-26

### Changed
- **Default config aligned to egg's risk profile**: Updated 25 default values in config.go — relaxed persistence filters (lookback 90/180/360m, min_count=1), disabled OI rank and volatility filters, increased slippage limit (50 bps), longer entry/exit timeouts (300s), higher rotation threshold (100 bps/180m cooldown), spread reversal tolerance (1), re-enter cooldown (1h), top opportunities (25)

### Added
- **Adaptive position sizing based on orderbook liquidity**: When slippage exceeds the limit at full size, binary-searches for the largest tradeable size within slippage constraints instead of rejecting outright. Range: [contract min / $10 floor, original size]. Fast path for liquid pairs (zero overhead). Logs `[adaptive]` with old→new size and notional when reduction occurs.
  - New `findMaxSizeForSlippage()` function — pure binary search over `[minSize, maxSize]`, converges in ~10 iterations
  - Uses max(longStep, shortStep) and max(longMin, shortMin) from both exchanges to ensure adapted size is tradeable on both legs
  - Min size rounded UP (ceil) to respect $10 capital floor
  - Falls back to original rejection if contract info unavailable (no unsafe stepSize=1 fallback)
  - `requiredMarginPerLeg` recalculated after size reduction
  - Single file change: `internal/risk/manager.go`
- **Adaptive sizing plan**: `docs/plan-adaptive-sizing.md` — binary search on orderbook depth to find max position size within slippage limit instead of rejecting outright

## [0.17.5] - 2026-03-26

### Fixed
- **OKX orderbook ctVal conversion**: REST `GetOrderbook` and WS `books5` now convert quantities from contracts to base units via ctVal. Previously raw contract counts were used — invisible for ctVal=1 symbols, 10x off for ctVal=10, caused math.MaxFloat64 slippage rejection for ctVal=100 symbols like SOPHUSDT.
- **Risk manager slippage message**: Insufficient orderbook depth now shows "insufficient orderbook depth" instead of printing math.MaxFloat64 as a giant number.

### Added
- **`cmd/validate-docs/`**: Permanent reusable API doc validation tool. Calls all 12 read-only methods on all 6 exchanges, validates field names/types against real API responses. Usage: `go run ./cmd/validate-docs/ [--exchange NAME] [--verbose]`

## [0.17.4] - 2026-03-26

### Fixed
- **OKX GetClosePnL field parsing**: Now parses `fee`, `fundingFee`, `realizedPnl`, `direction` fields from positions-history response — previously reported all PnL as price PnL with zero fees/funding, overstating realized profits
- **Bitget GetFuturesBalance field name**: `frozen` → `locked` to match actual API response (both occurrences)
- **BingX GetPendingOrders field name**: `quantity` → `origQty` to match actual API response — order sizes were always empty
- **BingX GetUserTrades endpoint**: Switched from undocumented `/allFillOrders` to `/fillHistory` with correct params (`startTs`/`endTs`, `fill_history_orders` wrapper, datetime `filledTime` format)
- **Depth fill loop exit**: Now exits when remaining size < step size or min size (prevents hanging on unfillable remainders)
- **Depth fill zero-fills**: Now count as consecutive failures (prevents order spam on instantly-cancelled IOC orders)
- **Depth subscribe retry**: Unsub/resub on first failure (handles stale WS state after reconnect)
- **Depth ready wait**: Increased to 8s with retry (handles slow BingX WS reconnects)
- **queryEntryFees retry loop**: 5s/10s/20s delays for exchanges slow to index fills
- **queryEntryFees per-leg fee logging**: Added diagnostics for fee retrieval

### Added
- **Exchange.Close() interface method**: All 6 adapters now close WS connections on SIGTERM/SIGINT for graceful shutdown
- **Live API validation**: Called all read-only endpoints on all 6 exchanges, compared real responses against local docs, wrote corrections to annotations

### Documentation
- Patched missing endpoints into all 6 EXCHANGEAPI docs (Appendix sections)
- Added/updated annotations in `doc/.annotations/` for all exchanges based on live API validation
- BingX fillHistory response format correction (`fill_history_orders` wrapper, datetime `filledTime`)

## [0.17.3] - 2026-03-26

### Fixed
- **Loris rate normalization**: Loris API returns rates as bps normalized to 8h equivalent (`bps_per_period × 8/interval`), NOT bps/hour as previously assumed. The ranker now divides by 8 to get correct bps/h. Impact: 4h interval symbols were 2x inflated, 1h interval symbols were 8x inflated (root cause of RIVER 1080 bps/h bug), 8h interval symbols unaffected

### Dashboard
- **Clickable exchange links in Rejected Opportunities**: Exchange names in the rejections table now link to the exchange's futures trading page (same `ExchangeLink` component used in History/Positions/Opportunities)

## [0.17.2] - 2026-03-26

### Added
- **Funding rate verification overhaul**: 4 new checks in `verifyOpportunity()` to catch stale/inflated rates:
  1. **Per-leg interval mismatch**: Reject if Loris interval vs exchange-reported interval differ by >0.5h (root cause of RIVERUSDT 1080 bps/h false entry — Loris reported stale 4h after exchange switched to 1h, inflating rate 4x)
  2. **Per-leg rate magnitude**: Reject if Loris rate vs exchange rate differ by >50% (using `rateMagnitudeOK` helper)
  3. **Spread-level magnitude**: Reject if Loris net spread vs exchange-verified net spread differ by >50%
  4. **Funding rate cap validation**: Reject rates exceeding exchange-reported caps (MaxRate/MinRate); falls back to ±150 bps/h default when exchange caps unavailable
- **`MaxRate`/`MinRate` fields** (`*float64`) on `exchange.FundingRate` struct — all 6 adapters now parse funding rate caps:
  - Binance: `adjustedFundingRateCap`/`Floor` from `/fapi/v1/fundingInfo`
  - Bybit: `upperFundingRate`/`lowerFundingRate` from instruments-info
  - OKX: `maxFundingRate`/`minFundingRate` from funding-rate endpoint
  - Gate.io: `funding_rate_limit` (symmetric ±) from contracts endpoint
  - Bitget: `maxFundingRate`/`minFundingRate` from current-fund-rate
  - BingX: `maxFundingRate`/`minFundingRate` from premiumIndex
- All new checks are skipped for CoinGlass-sourced opportunities (zero-rate placeholders)

### Fixed
- **RIVERUSDT false entry at 1080 bps/h**: Loris reported stale 4h interval after exchange switched to 1h, inflating normalized rate by 4x. New interval mismatch check prevents this class of error.

## [0.17.1] - 2026-03-26

### Fixed
- **PnL reconciliation synchronous**: Changed from async (5s delay) to synchronous first attempt (2s), ensuring exchange-reported PnL is used before broadcasting to frontend
- **Reconcile broadcast**: Added `BroadcastPositionUpdate` after successful PnL reconciliation — frontend now receives corrected PnL immediately
- **Rotation PnL race condition**: Added `pnlReconcileMu` mutex to serialize `reconcilePnL` and `reconcileRotationPnL`, eliminating concurrent read/write races
- **Rotation PnL accumulation**: Changed `RotationPnL` from `=` to `+=` to correctly accumulate PnL across multiple rotations
- **Rotation PnL double-count**: When position is already closed, rotation reconcile now re-triggers full `tryReconcilePnL` instead of manually adjusting `RealizedPnL += rotPnL`

## [0.17.0] - 2026-03-25

### Fixed
- **OKX contract size conversion**: `PlaceOrder` and `PlaceStopLoss` only converted base→contract when `ctVal > 1`, missing 181 contracts with ctVal < 1 (BTC, ETH, etc.) — changed to `ctVal != 1`
- **OKX LoadAllContracts StepSize/MinSize units**: Were stored in contract units instead of base asset units — now multiplied by ctVal for engine compatibility
- **OKX SizeDecimals**: Recalculated from base-unit step size instead of contract-unit lotSz

## [0.16.9] - 2026-03-25

### Fixed
- **Bitget GetUserTrades fee parsing**: API response uses `feeDetail[0].totalFee` and `baseVolume`, not `fee` and `size` — entry fees were being missed for Bitget legs
- **Bitget GetUserTrades JSON wrapper**: Added missing `data` envelope — `fillList` is at `data.fillList`, not root level
- **OKX GetClosePnL field mapping**: Removed non-existent fields (`realizedPnl`, `fee`, `fundingFee`, `direction`), mapped correct fields (`pnl`, `posSide`)
- **OKX GetClosePnL `begin` parameter**: Removed unsupported `begin` param (OKX silently ignores it — positions-history only supports `after`/`before` for ID pagination)
- **OKX GetClosePnL `posSide="net"` handling**: One-way mode returns "net" instead of "long"/"short" — now infers direction from `openMaxPos` sign with price fallback
- **Gate.io dark mode link**: Changed URL parameter from `theme=dark_mode` to `theme=dark`

## [0.16.7] - 2026-03-25

### Added
- **Configurable Binance delist filter**: New `strategy.discovery.delist_filter` toggle in config.json (default `true` — existing behavior unchanged). When disabled: delist monitor goroutine won't start, scanner won't filter delisting coins, engine won't auto-close positions for delisting coins
- **Dashboard delist filter toggle**: Enable/disable switch in Config page Discovery section with i18n support (English + Chinese)

## [0.16.6] - 2026-03-25

### Fixed
- **Config save 400 error**: Changed `exchangeUpdate.Enabled` from `*string` to `*bool` — frontend sends boolean, was causing JSON decode failure
- **Config fields cleared after save**: Update frontend config state with backend response after successful save
- **Exchange toggle not triggering dirty flag**: Toggling enable/disable now shows "unsaved changes" indicator
- **WebSocket null positions white screen**: Guard against `null` from `GetActivePositions()` causing `null.map()` crash
- **Transfer error messages unclear**: `rawRequest` now parses response body to show actual backend error
- **Transfer chain selection mismatch**: Auto-switch chain when destination exchange only has APT address but default was BEP20

### Added
- **Exchange enable/disable persistence**: Added `Enabled *bool` field to Config struct, config.json, and API handler for independent exchange on/off control
- **`IsExchangeEnabled()` method**: Checks both API key presence and enabled flag, replacing key-only check

### Changed
- **Build order reminder in CLAUDE.md**: Emphasize frontend must build before Go binary (go:embed)

## [0.16.5] - 2026-03-25

### Added
- **Dashboard overhaul — tab-based config**: Config page restructured into 8 tabs (Exchanges / Fund Management / Schedule / Discovery / Persistence Filter / Entry / Exit & Rotation / Risk Control)
- **Exchange settings UI**: Configure API Key, Secret Key, Passphrase, enable/disable toggle per exchange — all from the dashboard
- **Exchange settings backend API**: GET returns key preview (first 6 chars), POST supports update/clear credentials, persisted via SaveJSON
- **Mobile responsive sidebar**: Sidebar hidden on mobile, replaced with drawer + hamburger menu; tables with overflow-x-auto
- **Risk control visualization**: Margin L3/L4/L5 thresholds rendered as range sliders with gradient bar
- **Max funding interval filter**: New `max_interval_hours` config to skip pairs with long funding intervals (e.g. 4h/8h)
- **Dark mode exchange links**: All exchange trading URLs default to dark theme
- **Entry fee tracking in PnL**: Query actual trading fees from exchange API after opening, include in real-time PnL instead of waiting for reconcile
- **Entry Fees column in Positions dashboard page**

### Changed
- **Removed deposit address fields from Config exchanges tab**: Address management consolidated in Transfers page

### Removed
- Removed deprecated `exit_mode` and `order_advance_min` ghost fields from Config UI

## [0.16.4] - 2026-03-25

### Changed
- **Backtest rate limit mitigation**: Entry scan backtest filter is now cache-only (no inline API calls). Background prefetch worker fetches uncached symbols after non-entry scans with random 3-30s delays between requests. Global 429 backoff (60s cooldown). Cache TTL increased from 6h to 24h. Single-worker mutex prevents overlapping prefetch runs.

### Added
- **Loris Tools credit**: Added "Funding rate data provided by Loris Tools" link to dashboard Overview page.

## [0.16.3] - 2026-03-25

### Fixed
- **Config page: fixed "Unsaved changes" always showing** — Added dirty-state tracking; bottom bar now only shows "Unsaved changes" when user has actually modified a field, clears on save

### Changed
- **Config page: BEP20 + APT address fields** — Exchange section now shows both BEP20 and APT deposit address inputs (previously only BEP20); on save, transforms flat overrides into nested address map format for the backend

## [0.16.2] - 2026-03-25

### Fixed
- **Gate.io unified balance — proper margin ratio by account mode**:
  - `single_currency` mode: uses per-currency USDT fields (`mm` / `margin_balance`) for margin ratio
  - `multi_currency`/`portfolio` mode: uses top-level fields (`total_maintenance_margin` / `unified_account_total_equity`)
  - Previously top-level fields were all 0 for `single_currency` mode, causing MarginRatio=0
  - The health monitor fallback formula (`1 - available/total`) then computed 1.0 when available=0, falsely triggering L5 emergency close
- **Health monitor fallback safety**: Added `bal.Available > 0` guard to the fallback margin ratio calculation in `health.go`; when Available=0, skips fallback (marginRatio stays 0 = safe) instead of computing 1.0

### Added
- **Gate.io permission tip in dashboard**: Added yellow tip box in Permissions page explaining Gate.io users need "保證金交易（新版統一帳戶）" (Unified Account) permission for accurate balance display
- **Gate.io gatetest tool**: New `cmd/gatetest/` for testing Gate.io unified endpoints

## [0.16.1] - 2026-03-25

### Added
- **Gate.io unified account support**: Detect unified mode on startup via `GET /unified/unified_mode`, branch `GetFuturesBalance` to use `/unified/accounts` for unified mode, no-op `TransferToFutures` for unified, fall back to classic if detection fails

### Fixed
- **Dashboard double-counting Gate.io balance**: Separated display concerns (always show as unified in dashboard) from rebalancing concerns (runtime `IsUnified()` detection for surplus calculation)
- **Rebalancing surplus double-count for unified exchanges**: OKX, Bybit, and Gate.io unified now use only futures balance instead of futures+spot for surplus calculation

## [0.16.0] - 2026-03-25

### Added
- **API key permission check on startup**: Checks Read, Futures Trade, Withdraw, and Transfer permissions for all 6 exchanges on startup. Results displayed in dashboard Overview page.
  - Binance/Bybit/BingX: direct API endpoints for authoritative results
  - Bitget/OKX: inferred via test calls with error code detection
  - Gate.io: marked unsupported (inference unreliable)
  - Tri-state results: granted/denied/unknown — no false confidence
  - `GET /api/permissions` endpoint (no keys or secrets exposed)
  - Startup warns on denied Withdraw/Transfer, logs CRITICAL for denied Read/Trade

## [0.15.4] - 2026-03-25

### Fixed
- **Binance rotation PnL: empty Side filter**: Binance `GetClosePnL` returns `Side=""` (income API has no position side info), causing `reconcileRotationPnL` to reject the record. Now accepts empty side, matching the existing `aggregateClosePnLBySide` fallback. Added diagnostic logging (side, closeTime, netPnL) for rejected records.

### Added
- **Exit spread column in History**: Shows `current_spread` (the spread at exit time) alongside entry spread. Entry spread column renamed from "Spread" to "Entry Spread". Red highlight when exit spread is negative (reversed).

## [0.15.3] - 2026-03-24

### Added
- **Editable deposit addresses in dashboard**: Transfers page now includes a "Deposit Addresses" section where users can view and edit APT/BEP20 addresses for all 6 exchanges. Saved to config.json via `POST /api/addresses`.

## [0.15.2] - 2026-03-24

### Added
- **Instant stop-loss fill detection**: When an SL order fills on any exchange, the engine immediately closes the remaining leg via emergency market close — eliminating the 5-minute consolidator delay.
  - Added `SetOrderCallback` to Exchange interface — all 6 adapters emit filled orders via WS
  - Engine maintains in-memory SL index (`exchange:orderID → posID+leg`), rebuilt on startup
  - Non-blocking buffered channel decouples WS handler from engine work
  - Preempts any running exit goroutine, removes both SL keys, sends dashboard alert
  - `registerSLOrders()` called after SL placement, `unregisterSLOrders()` called on cancel

## [0.15.1] - 2026-03-24

### Added
- **Binance delist detection & auto-protection**: Background goroutine polls Binance CMS API (`catalogId=161`) every 6h for delisting announcements.
  - Parses coin names from spot delist titles ("Will Delist X, Y on DATE") and futures delist titles ("XUSD Perpetual Contracts")
  - Maintains Redis blacklist (`arb:delist:{SYMBOL}`) with TTL until delist date + 7 days
  - Scanner filter rejects all opportunities with blacklisted symbols (all scan types)
  - Engine auto-exits active positions with Binance legs via emergency market close
  - Dashboard alert broadcast on delist-triggered emergency close
  - Exit goroutine preemption before emergency close (reuses L4/L5 safety pattern)

## [0.15.0] - 2026-03-24

### Added
- **Historical funding backtest filter**: Before opening a position, checks if the coin's funding spread has been historically profitable over the past X days using Loris historical API.
  - Fetches settlement data from `loris.tools/api/funding/historical`
  - Validates each leg independently using per-leg funding intervals
  - Rounds timestamps to nearest hour for jitter tolerance, deduplicates
  - Coverage check: skips filter if <50% of expected settlements found
  - Profitability: `netProfit = shortSumY - longSumY > BacktestMinProfit`
  - Redis cache with 6h TTL (stores raw sums, re-evaluates pass/fail on read)
  - Config: `BacktestDays` (default 3, 0=disabled), `BacktestMinProfit` (default 0)
  - Dashboard config support in entry section

## [0.14.3] - 2026-03-24

### Changed
- **Clickable exchange links on all pages**: Exchange names in Opportunities and Trade History pages are now clickable links that open the trading page (same as Positions page). Extracted `tradingUrl` and `ExchangeLink` to shared utility `web/src/utils/tradingUrl.tsx`.

## [0.14.2] - 2026-03-24

### Added
- **Rotation PnL reconciliation**: `rotateLeg()` now triggers async `reconcileRotationPnL()` to query the old exchange's `GetClosePnL()` and store authoritative PnL in `RotationPnL`. Previously rotation PnL was always 0, causing final reconciled PnL to miss the rotated leg's contribution entirely.
  - Narrow time window (±5min around rotation) to avoid picking up stale records
  - Idempotent (set, not +=) to prevent double-counting on retry
  - If position already closed when result arrives, recomputes `RealizedPnL`, updates stats and history

### Fixed
- **Bybit PnL reconciliation double-counting funding**: Bybit's `closedPnl` already includes funding fees, but the adapter was adding `totalFunding` on top via `GetFundingFees()`. This inflated reconciled PnL by the Bybit-side funding amount. Fix: `NetPnL = closedPnl` (no longer adds funding). Funding query kept for `FundingCollected` reconciliation.
- **BingX PnL reconciliation missing funding**: BingX's `netProfit` excludes funding, but the adapter was using it as-is for `NetPnL`. This deflated reconciled PnL by the BingX-side funding amount. Fix: `NetPnL = netProfit + totalFunding`.

## [0.14.0] - 2026-03-24

### Added
- **Exit price recording**: Positions now store `long_exit` and `short_exit` close prices alongside entry prices.
  - Depth-fill exit: VWAP close price from multi-chunk fills
  - Smart/emergency close: price from confirmFill avgPrice or BBO fallback
  - L4 reducePosition: captures avg price from reduce orders before full-close delegation
  - Reconciliation: overwrites exit prices only when exchange reports non-zero ExitPrice (Binance has none)
  - Dashboard History page shows "entry → exit" price columns for both legs

## [0.13.6] - 2026-03-24

### Fixed
- **Binance WS order status case mismatch**: Binance sends uppercase status (`FILLED`, `CANCELED`) but `confirmFill` checks lowercase — every Binance fill timed out, attempted unnecessary cancel + REST fallback (+300ms latency). Normalized to lowercase in `ws_private.go` orderStore. Could have caused imbalance if REST also failed.
- **Bitget WS order status normalization**: Same class of exposure — raw status stored without normalization. Added `strings.ToLower()` for consistency with other adapters (Bybit, BingX, OKX, Gate.io already normalize).

## [0.13.5] - 2026-03-23

### Fixed
- **Gate.io spot→futures transfer**: Added missing `settle: "usdt"` field to `/wallet/transfers` request body. Gate.io API requires this for futures transfers — was failing with `INVALID_PARAM_VALUE: Invalid settle`.
- **Dashboard white page after login**: Fixed React Rules of Hooks violation — `useCallback` hooks for update banner were placed after conditional early return, causing React to crash with "Rendered fewer hooks than expected". Moved all hooks before the early return.

## [0.13.4] - 2026-03-23

### Added
- **Dashboard auto-update system**: Check for new versions from git remote and update from the dashboard.
  - `GET /api/check-update` — compares local `VERSION` vs `origin/main:VERSION`, returns changelog
  - `POST /api/update` — performs `git fetch` + `git reset --hard` + `make build`, then exits for systemd respawn
  - Blue notification banner when update available, dismissible for 24h per version (localStorage)
  - Update modal with changelog preview and progress output
  - Auto-checks on login and every 30 minutes

## [0.13.3] - 2026-03-23

### Changed
- **Duplicate coin entry blocked**: Engine now rejects opening a position on any symbol that already has an active position, regardless of exchange pair. Previously, same-pair add-ons were merged into existing positions — this caused issues with OKX delayed WS fill reports creating phantom positions and imbalances.

### Added
- **Balance enforcer in consolidator**: When consolidator detects >5% size imbalance between long and short legs after sync, it now trims the excess side via confirmed reduce-only market order. After trim: cancels old stop losses, updates local record, re-places SLs with correct sizes, broadcasts to dashboard.
- **confirmFill timeout logging**: Detailed logs when confirmFill times out — shows WS status, REST query result, and cancel attempt. Helps diagnose OKX WS delayed fill reports.
- **OKX REST avgPrice in GetOrderFilledQty**: REST fallback now parses `avgPx` and `state` from OKX order query and stores in orderStore, so confirmFill gets correct price even when WS is delayed.

### Fixed
- **Consolidator race protection**: `consolidatePosition` now checks `busySymbols` (entry + exit active) before processing. Previously only orphan closer checked this — size sync and balance enforcement could race with active trades.

## [0.13.2] - 2026-03-23

### Fixed
- **OKX IOC orders**: OKX adapter was ignoring the `Force` (IOC) field — all limit orders were placed as GTC, causing resting orders to fill later untracked and creating phantom positions. Now maps `req.Force` to OKX's `ordType` field (`ioc`, `fok`, `post_only`).
- **confirmFill cancel-on-timeout**: When the 5s fill confirmation deadline passes without a terminal state, the order is now cancelled before returning. Prevents resting orders from filling later and creating position imbalances. After cancel, re-queries fill state to capture any partial fills.
- **Shared exchange PnL split**: When multiple local positions share the same exchange+symbol on one side, reconciliation now splits the exchange-reported PnL evenly among siblings closed within 5 minutes.

## [0.13.1] - 2026-03-23

### Added
- **Force open position**: When manual open is rejected by risk checks (e.g. capital cap exceeded), the dashboard now shows a "Force Open (Skip Risk)" button to override the rejection. Risk approval still runs for sizing; only the approval gate is bypassed.

## [0.13.0] - 2026-03-23

### Changed
- **Exchange adapters moved to `pkg/exchange/`**: All 6 exchange adapters (Binance, Bybit, Gate.io, Bitget, OKX, BingX) moved from `internal/exchange/` to `pkg/exchange/`, making them importable by external projects. All import paths updated across 54 files.
- **Config: cooldown fields moved to entry section**: `loss_cooldown_hours` and `re_enter_cooldown_hours` moved from `strategy.discovery.persistence` to `strategy.entry` in config.json, API, and dashboard.
- **Config: missing persistence fields added to dashboard**: Lookback, min count, stability ratio, and OI rank settings for 1h/4h/8h intervals now visible and editable in the Config page.

### Fixed
- **BingX REST fill price backfill**: When BingX WS is disconnected (every ~1h), `GetOrderFilledQty` now also parses and stores `avgPrice` from the REST response, so `confirmFill` gets a correct close price instead of falling back to stale BBO data. Prevents garbage PnL calculations.
- **EnsureOneWayMode verify-then-suppress**: OKX, Bybit, and Bitget now verify the current position mode when the set call fails. If already in one-way mode, the error is suppressed.

## [0.12.1] - 2026-03-22

### Added
- **Log rotation**: Log file moved to `logs/arb.log`. Daily at 00:10 system time, the current log is archived to `logs/arb_YYYYMMDD.log` (yesterday's date) and a fresh log file is created. Thread-safe writer survives rotation seamlessly.

## [0.12.0] - 2026-03-22

### Added
- **AI Diagnose**: Dashboard Overview page has an "AI Diagnose" button that collects recent error/warn logs, active positions, trade history, and exchange balances, sends them to a configurable AI endpoint for analysis, and displays the result in a modal. Responds in Traditional Chinese.
- **AI config**: New `ai` section in config.json (endpoint, api_key, model, max_tokens) — editable from the Config dashboard page and persisted to config.json.
- **Clickable exchange links**: Exchange names in the Positions table are now clickable links that open the exchange's futures trading page for that coin in a new browser tab. Supports all 6 exchanges (Binance, Bybit, Gate.io, Bitget, OKX, BingX).

### Fixed
- **WriteTimeout**: Increased Go HTTP server `WriteTimeout` from 15s to 180s to prevent connection drops during slow AI diagnosis responses.

### Fixed
- **Funding collection missing one side**: When one exchange provides `FundingFee` in position data (Gate.io, Bitget, OKX) and the other doesn't (Binance, Bybit), the old code skipped the fallback for the second exchange. Now each side is queried independently — fast-path if available, `GetFundingFees` history fallback otherwise.
- **BingX GetClosePnL JSON parse error**: `avgClosePrice` field was typed as `float64` but BingX returns a string. Changed to `string` with `ParseFloat`.
- **PnL sanity check**: Both exit paths now reject PnL exceeding 2x position notional value (indicates garbage close prices) and fall back to funding-only PnL to prevent absurd values like -77,323.
- **Gate.io unified balance with open positions**: Dashboard showed only free margin (excluding margin locked in positions) for unified exchanges. Now saves equity (`Total`) instead of `Available` for all exchanges.
- **Gate.io near-zero total fallback**: Gate.io API returns `total` as a tiny positive number (~4e-9) instead of 0 when no positions exist, bypassing the `total <= 0` fallback. Changed to `total < available` so the fallback always picks the meaningful value.
- **Gate.io balance source**: Gate.io unified account uses spot balance as true equity (futures balance is only the futures-allocated portion).

## [0.11.10] - 2026-03-22

(Superseded by 0.11.11)

## [0.11.9] - 2026-03-21

### Added
- **Position merging**: When a second trade opens for the same symbol on the same exchange pair, it merges into the existing position instead of creating a duplicate. Sizes are summed, entry prices are weighted-averaged, entry spread is weighted-averaged. Stop-loss orders are cancelled and replaced with the combined size.
- **`findSiblingPosition`**: Finds an existing active position matching symbol + exchange pair.
- **`mergeIntoPosition`**: Atomically merges new fills into an existing position via `UpdatePositionFields`, re-reads from DB before broadcasting, then cancel + reattach SLs.
- **`MergeExistingDuplicates`**: One-time startup function that detects and merges any pre-existing duplicate positions (same symbol + exchange pair). Keeps the earliest position, absorbs all others' sizes/funding/entry prices, cancels absorbed positions' SLs.

### Changed
- **ManualOpen duplicate check relaxed**: Same-symbol + same-exchange-pair opens are now allowed (they merge during execution). Same-symbol on different exchanges is still blocked.
- **Capacity check**: Same-pair add-ons no longer consume a new `MaxPositions` slot.

## [0.11.8] - 2026-03-21

### Fixed
- **Gate.io balance double-counting**: Gate.io is in unified account mode but the API reports separate spot/futures balances. Total Balance card was summing both ($276 + $291 = $568 vs actual ~$296). Unified exchanges now only count the futures/trading equity in the total.
- **Exchange balance display clarity**: Each exchange now shows its account type:
  - **Unified** (OKX, Bybit, Gate.io): displays `Unified: $xxx`
  - **Separate** (Binance, Bitget, BingX): displays `Futures: $xxx` / `Spot: $xxx`
- Added `account_type` field (`"unified"` or `"separate"`) to `/api/exchanges` response. i18n labels for EN and zh-TW.

## [0.11.7] - 2026-03-21

### Fixed
- **Transfer history now tracks all engine-initiated transfers**: Added `recordTransfer` helper to engine. All automatic transfers are now saved to Redis and displayed on the dashboard Transfers page:
  - **Rebalance**: futures→spot (prep), cross-exchange withdrawal, spot→futures (receive)
  - **L3 health**: inter-exchange internal transfer
  - **L5 safety**: futures→spot emergency transfer
  - **Rotation**: spot→futures auto-margin on new exchange
- Each record includes source, destination, amount, reason label (rebalance/L3-health/L5-safety/rotation), tx ID (for withdrawals), and timestamp.

## [0.11.6] - 2026-03-21

### Fixed
- **PnL reconciliation rewrite**: Replaced fragile cash-flow reconstruction (`GetUserTrades` + manual sum) with exchange-native position close PnL APIs. 4 of 6 exchanges (Bitget, OKX, Gate.io, BingX) now provide pre-calculated all-inclusive net PnL in a single call. Bybit's `closedPnl` is supplemented with funding fees. Binance aggregates income API records (`REALIZED_PNL` + `COMMISSION` + `FUNDING_FEE`).
- **New `GetClosePnL` interface method**: Added to Exchange interface with implementations across all 6 adapters. Returns `[]ClosePnL` with `PricePnL`, `Fees`, `Funding`, `NetPnL`, entry/exit prices, size, and normalized side.
- **Partial close aggregation**: `aggregateClosePnLBySide` sums all matching close records per side, handling exchanges that return multiple records for partial closes.
- **Reconcile no longer overwrites with bad data**: If either exchange leg fails to return close PnL, reconciliation retries instead of falling back to the fragile trade-reconstruction method. After 3 failed attempts, local PnL is kept as-is.
- **Bybit PnL accuracy**: Uses `cumEntryValue`/`cumExitValue` for price PnL calculation. Funding attached only to first record to prevent double-counting during aggregation. Funding query failure now fails the reconciliation (was silently ignored).

### Added
- **`Exchange_close_pnl_api.md`**: API reference documenting all 6 exchanges' close PnL endpoints, request params, response fields, formulas, and caveats.

## [0.11.5] - 2026-03-20

### Added
- **Manual position open from dashboard**: New Open button on Opportunities page with confirmation dialog. `POST /api/positions/open` endpoint triggers `Engine.ManualOpen` which runs risk checks synchronously and executes depth fill asynchronously (returns 202). Frontend modal shows symbol, exchanges, spread, and inline errors. i18n labels for EN and zh-TW.
- **`executeTradeV2WithPos` variant**: Accepts a pre-created pending position to avoid double-creation when ManualOpen reserves a slot before async execution.
- **`removePendingPosition` helper**: Marks a pending position as closed and broadcasts the update immediately, so the frontend drops phantom rows without waiting for a full refresh cycle.

### Fixed
- **Capacity race condition**: Added `capacityMu` global mutex to serialize capacity checks in both `ManualOpen` and `executeArbitrage`. Prevents concurrent requests (manual-vs-manual or manual-vs-automated) from over-subscribing `MaxPositions`.
- **Auth hardening for trade endpoints**: Mutating endpoints (`/api/positions/close`, `/api/positions/open`, `/api/transfer`) now require a configured `DASHBOARD_PASSWORD` — returns 403 when no password is set. Read-only endpoints still bypass auth in dev mode.
- **Phantom pending position cleanup**: `cleanupFailedPosition` now broadcasts position removal via WebSocket so the frontend immediately drops orphaned rows instead of waiting for a full positions refresh.
- **Error-to-status mapping**: `"dry run"` and `"risk check failed"` errors now return 422 (Unprocessable Entity) instead of falling through to 500 (Internal Server Error).
- **Frontend `openPosition` auth handling**: Restored 401 auto-clear-token + reload behavior and server error message preservation (was lost when using inline `fetch` instead of `rawRequest` pattern).
- **Modal Escape key dismiss**: Open-confirmation dialog on Opportunities page can now be dismissed with the Escape key.

## [0.11.4] - 2026-03-20

### Added
- **Manual position close from dashboard**: New Close button on Positions page with confirmation dialog. `POST /api/positions/close` endpoint delegates to engine via callback pattern (avoids import cycle). Engine `ManualClose` performs atomic `active → exiting` status transition to prevent race conditions with automated exits. Frontend shows inline errors on failure. i18n labels for EN and zh-TW.

## [0.11.3] - 2026-03-20

### Added
- **Spot balance on dashboard**: Overview page now shows spot balance below futures balance for each exchange (only when > 0). Total balance card includes both futures + spot. Balance refresh (60s) now caches spot balances to Redis alongside futures balances. API `GET /api/exchanges` response includes new `spot_balance` field.

## [0.11.2] - 2026-03-20

### Fixed
- **Reconcile PnL sanity check**: Added guard to reject PnL corrections where diff exceeds position notional — prevents incomplete trade data from exchange APIs from corrupting stats. Returns `false` to trigger retry instead of accepting bad data. BARDUSDT had its PnL falsely corrected from +$0.24 to -$395.22 because Bybit only returned exit trades (buys) without entry trades (sells), making the cash flow appear as a full-notional loss.
- **Reconcile notional fallback**: When position sizes are zeroed (depth-exit path zeroes them before async reconciliation), uses `abs(totalCashFlow)` as notional proxy instead of a hardcoded $100 fallback.
- **Balance CLI missing BingX**: Added BingX case to `cmd/balance/main.go`.

## [0.11.1] - 2026-03-20

### Fixed
- **BingX SetLeverage idempotency**: "Already set" errors were not suppressed (checked unreachable `code == 0`). Now matches other adapters — ignores "no need to change" / "already" messages.
- **BingX WS depth reconnect symbol format**: Stale depth data was deleted using BingX format (`BTC-USDT`) but stored under internal format (`BTCUSDT`), causing stale orderbooks to survive reconnect.
- **BingX WS old socket leak**: Both public and private WS now close the old connection before assigning the new one on reconnect.
- **ARCHITECTURE.md sync**: Fixed interface method count (29/30 → 31, added `GetFundingFees`), corrected config table defaults (`SCAN_MINUTES`, `EXIT_SCAN_MINUTE`, `ENTRY_SCAN_MINUTE`, `ROTATE_SCAN_MINUTE`, `VALUE_OF_TIME_HOURS`), distinguished env-overridable vs JSON-only params, added Redis cooldown keys.

## [0.11.0] - 2026-03-20

### Added
- **BingX exchange adapter**: 6th exchange integration. Full implementation with 4 files (~1,750 LOC):
  - `internal/exchange/bingx/client.go` — REST client with HMAC-SHA256 signing, retry logic, response envelope parsing
  - `internal/exchange/bingx/adapter.go` — All 26 Exchange interface methods (orders, positions, balance, leverage, margin, funding, orderbook, contracts, transfers, stop-loss, trade history)
  - `internal/exchange/bingx/ws.go` — Public WebSocket (BBO + depth streams, gzip decompression, Ping/Pong)
  - `internal/exchange/bingx/ws_private.go` — Private WebSocket (order updates via listenKey, auto-renewal)
- **BingX config integration**: API key/secret fields, env var overrides, factory wiring, discovery scanner, fee schedule (maker 0.016%, taker 0.04%)
- **BingX symbol mapping**: `BTCUSDT` internal ↔ `BTC-USDT` for BingX API (hyphenated)
- **Live test**: 15/15 non-order tests passing

## [0.10.11] - 2026-03-20

### Added
- **Config field tooltips**: Each config field in the dashboard now has a `?` icon showing a styled popover with an explanation (EN + zh-TW). Hover or click to view.

### Fixed
- **OKX `GetUserTrades` unmarshal error**: The OKX client already unwraps the `{code, data}` envelope, but `GetUserTrades` tried to parse a second `data` wrapper. Fixed to unmarshal directly into the array. This was causing all OKX trade reconciliation to fail.
- **Price gap filter bypassed on extreme gaps**: When the long exchange ask was lower than the short exchange bid (negative gap), the gap was clamped to 0 as "favorable". This allowed entries like EDGEUSDT (Gate.io=0.13, Binance=0.67) with 8000+ bps gap to pass. Now uses absolute gap value for the hard cap check — any gap exceeding `max_price_gap_bps` in either direction is rejected.

### Changed
- **Dashboard config section order**: Fund Management and Schedule moved to top for quick access.

## [0.10.10] - 2026-03-19

### Added
- **Anti-spike filters** (entry filters to prevent SAHARA-pattern repeated losses):
  - **Spread Volatility (CV)**: Rejects opportunities where the spread's coefficient of variation (stddev/mean) across recent scans exceeds a threshold (default 0.5). Catches erratic rate spikes that normalize post-settlement.
  - **Loss Cooldown**: After closing a position at a loss, blacklists that symbol for N hours (default 4h). Prevents re-entering the same symbol on different exchange pairs. Persisted to Redis (`arb:lossCooldown:{symbol}`) with TTL — survives restarts.
  - **Re-Enter Cooldown**: After closing any position (win or loss), blocks re-entry on the same symbol for N hours (default 0, disabled). Persisted to Redis (`arb:reEnterCooldown:{symbol}`) with TTL.
- **Spread Reversal Tolerance**: Configurable `spread_reversal_tolerance` (default 0) allows N spread reversals before triggering exit. Tolerates transient post-settlement noise. Reversal count persisted on position in Redis.
- **Configurable Funding Window**: `funding_window_min` (default 30) replaces the hardcoded 30-minute funding imminence check. Configurable via dashboard.
- **Scan minute fool-proofing**: `EnsureScanMinutes()` auto-adds `rebalance_scan_minute`, `exit_scan_minute`, `entry_scan_minute`, `rotate_scan_minute` to `scan_minutes` if missing. Runs at startup and on dashboard config updates.

### Removed
- **`evaluateExitCost` and `ExitMaxRecoveryHours`**: Removed the VWAP gap-based exit cost evaluation. Exit is now solely driven by spread reversal detection (with configurable tolerance). The recovery-hours logic was redundant since all spread <= 0 cases are already reversals.
- **`settlement_blackout_min`**: Removed settlement blackout filter (superseded by configurable funding window).

### Changed
- **Cooldowns use Redis**: Symbol cooldowns (loss + re-enter) now persisted to Redis with automatic TTL expiry instead of in-memory maps. Survive process restarts.
- **Position model**: Added `reversal_count` field for tracking spread reversal occurrences.

## [0.10.8] - 2026-03-19

### Added
- **Current spread on Positions page**: Risk monitor persists `CurrentSpread` (bps/h) every cycle. Dashboard shows entry and current spread side-by-side, color-coded (green positive, red negative).
- **Multi-language dashboard (i18n)**: Full zh-TW (繁體中文) and English support. Language switcher in sidebar, persisted to localStorage. Default: zh-TW. All pages translated including config field labels.
- **Consolidator race protection**: Consolidator now checks `exitActive` and `entryActive` maps before acting on positions. Prevents two critical race conditions:
  - Exit race: consolidator closing partially-exited legs as "orphans" while exit goroutine is still running (caused ATHUSDT double-close and circuit breaker).
  - Entry race: consolidator closing freshly-filled legs before the depth fill loop saves the position record (caused DEGOUSDT immediate loss of long leg).

### Changed
- **Rebalance moved to fixed scan minute**: Replaced dynamic `Scheduler` (computed next funding snapshot via exchange API polling) with `RebalanceScanMinute` (default `:20`). Rebalance now fires as a `RebalanceScan` type in the discovery scan loop, consistent with exit/entry/rotate. Deleted `scheduler.go`. Config field renamed from `rebalance_advance_min` to `rebalance_scan_minute` (old name accepted as fallback).
- **PnL reconciliation retry with better diagnostics**: `reconcilePnL` now retries 3 times (5s, 15s, 30s delays) instead of single attempt. Rejects 0-trade responses as incomplete. Logs attempt number and per-exchange funding breakdown. Also writes actual `FundingCollected` from exchange APIs back to position record.

### Fixed
- **OKX `GetUserTrades` limit error**: OKX fills-history endpoint max is 100, but reconciliation passed 200. Capped all adapters to their exchange-specific maximums.
- **OKX `GetUserTrades` contract size**: `fillSz` from OKX is in contracts, not base units. Now multiplied by `ctVal` to convert to base units, matching all other OKX methods.
- **Gate.io `GetUserTrades` unmarshal error**: Gate.io returns `fee` as a string, not float64. Changed struct field type and added `strconv.ParseFloat`.
- **All adapters**: Added `limit > 100` cap to Bybit, Bitget, Gate.io `GetUserTrades` (defensive).

## [0.10.7] - 2026-03-19

### Changed
- **Actual funding fees replace estimated values**: `FundingCollected` now uses real exchange data instead of the inaccurate `unrealizedPnL - pricePnL` estimation.
  - **Fast path** (Bitget, OKX, Gate.io): Parses accumulated funding fee from position endpoint (`totalFee`, `fundingFee`, `pnl_fund`).
  - **Fallback** (Binance, Bybit): Queries dedicated funding history APIs (`/fapi/v1/income`, `/v5/account/transaction-log`).
  - All 5 exchanges implement `GetFundingFees(symbol, since)` via their respective bill/income/transaction-log endpoints.
- **Funding tracker schedule**: Changed from every 5 minutes to once on startup + every hour at HH:10:00 UTC.
- **Reconciliation includes funding**: `reconcilePnL()` now adds funding fee history to total cash flow when computing final PnL after position close.

### Added
- `FundingFee` field on `exchange.Position` struct (populated by Bitget, OKX, Gate.io).
- `FundingPayment` type and `GetFundingFees` method on `Exchange` interface.

## [0.10.6] - 2026-03-19

### Changed
- **Dashboard overhaul**: Fixed incorrect data display and added missing information across all pages.
  - **Spread display fixed**: Removed erroneous `× 10000` multiplier — `entry_spread` and opportunity `spread` are already in bps/h.
  - **Config page**: Updated stale `max_gap_recovery_hours` field to `max_gap_recovery_intervals`. Added schedule config section (entry/exit/rotate scan minutes).
  - **History duration fixed**: Now shows actual hold time (`updated_at - created_at`) instead of time since creation.
  - **Overview**: Added exchange balance cards with 60s auto-refresh. Replaced stale "Trade Count" card with "Total Balance". Color-coded long (green) / short (red) exchange names.
  - **Positions**: Added rotation PnL column, SL status indicator (ON/- with order IDs in tooltip). Removed non-existent `exit_mode` column. Smart price formatting for small-cap coins.
  - **History**: Added rotation PnL and rotation count columns. Split exchanges into separate long/short columns.
  - **Opportunities**: Added individual long/short rate columns, next funding countdown, source indicator (L=Loris, CG=CoinGlass), opportunity count in header.
  - **Types updated**: Added `updated_at`, `rotation_pnl`, `all_exchanges`, `long_sl_order_id`, `short_sl_order_id` to Position. Added `next_funding`, `source`, `timestamp` to Opportunity. Added `ExchangeInfo` type.
- **Noisy depth tick logs demoted to Debug**: Spread oscillation messages (`waiting...` / `recovered`) during entry fill loop changed from Info to Debug to reduce log noise.
- **Buffered file logging**: Log file writer now uses 64KB buffer with 2s flush interval, reducing per-line syscall overhead during high-frequency operations.

## [0.10.5] - 2026-03-19

### Added
- **Stop-loss protection for arbitrage positions**: Places protective stop-loss (conditional) orders on both legs after entry. SL distance formula: `90% / leverage` (e.g. 3x→30%, 5x→18%). Long SL triggers on price drop (sell), short SL triggers on price rise (buy). SL orders are cancelled on exit and updated on rotation (old exchange SL cancelled, new exchange SL placed at new entry price). Implemented across all 5 exchanges: Binance (`algoOrder` STOP_MARKET), Bybit (`StopOrder`), Gate.io (`price_orders`), Bitget (`normal_plan`), OKX (`order-algo`). Position model tracks SL order IDs (`long_sl_order_id`, `short_sl_order_id`). SL failures are logged but never block entry/exit.
- **Livetest SL tests (19-22)**: Open min-size position, place SL, cancel SL, close position. Automatically selects a cheap symbol (DOGE) when BTC min notional exceeds exchange limits.

### Fixed
- **Binance stop orders use Algo Order API**: Binance migrated conditional orders (STOP_MARKET) from `/fapi/v1/order` to `/fapi/v1/algoOrder` (since 2025-12-09). Place returns `algoId`, cancel via `DELETE /fapi/v1/algoOrder`.
- **Gate.io `SetLeverage` switched to isolated mode**: Was sending `leverage=N` which switches from cross to isolated margin. Now sends `leverage=0, cross_leverage_limit=N` to stay in cross margin mode at the desired leverage.
- **Gate.io `GetUserTrades` 404**: Path had double `/api/v4` prefix (`/api/v4/futures/usdt/my_trades`). Base URL already includes `/api/v4`; fixed to `/futures/usdt/my_trades`.

## [0.10.4] - 2026-03-19

### Fixed
- **OKX adapter: switch to one-way (net) mode**: Removed `posSide` from `PlaceOrder` and `SetLeverage` — hedge mode fields are invalid in net mode. Position mode is now set to `net_mode` once during `LoadAllContracts` via `ensureNetMode()`. `SetMarginMode` separated from position mode — it's now a no-op since `SetLeverage` already passes `mgnMode=cross`.
- **Gate.io `SetMarginMode("cross")` was a no-op**: Previously returned immediately for cross mode, assuming it was the default. Now actively sets `cross_leverage_limit=5, leverage=0` to switch from isolated to cross margin.

## [0.10.3] - 2026-03-19

### Added
- **Settlement frequency penalty for rotation**: When rotating a leg to a higher-frequency settlement exchange (e.g. 8h→1h), computes the extra settlements before the old exchange's next funding and penalizes the spread improvement accordingly. Only applies when the leg is on the paying side — collecting legs benefit from more settlements and get no penalty. Prevents rotations that look good in bps/h but incur real losses from extra funding payments.

### Fixed
- **Stale `NextFunding` after rotation**: `rotateLeg()` now fetches `NextFunding` from the new exchange and updates the position record. Previously the timestamp stayed stale until the monitor loop caught up.

## [0.10.2] - 2026-03-19

### Changed
- **Interval-based gap recovery limit**: Replaced fixed `MaxGapRecoveryHours` (24h) with `MaxGapRecoveryIntervals` (default 1.0). Gap recovery is now evaluated relative to the coin's funding interval — a 4.5h recovery is 4.5 intervals for a 1h coin (rejected) but ~1.1 intervals for a 4h coin (borderline). Unknown intervals default to 8h (conservative). Config key renamed from `max_gap_recovery_hours` to `max_gap_recovery_intervals`, env var from `MAX_GAP_RECOVERY_HOURS` to `MAX_GAP_RECOVERY_INTERVALS`.

## [0.10.1] - 2026-03-19

### Changed
- **Separate `RotateScan` from `ExitScan`**: Rotation checks now run on `:45` (`RotateScan`) instead of `:25` with exits. Exit checks on `:25` only, rotation checks on `:45` only. New configurable `rotate_scan_minute` (default 45).

## [0.10.0] - 2026-03-19

### Added
- **Post-close PnL reconciliation**: After every position close, queries actual trade history from all involved exchanges via `GetUserTrades` and recomputes realized PnL from real fill data. Corrects the position record and stats if the difference exceeds $0.01. Runs asynchronously with a 3s delay for exchange history to settle.
- **`GetUserTrades` interface method**: New Exchange interface method for fetching fill history. Implemented for all 5 exchanges (Binance `/fapi/v1/userTrades`, Bybit `/v5/execution/list`, Gate.io `/api/v4/futures/usdt/my_trades`, Bitget `/api/v2/mix/order/fill-history`, OKX `/api/v5/trade/fills-history`).
- **`AllExchanges` position field**: Tracks every exchange a position has used across its lifecycle (including rotated-away exchanges), ensuring reconciliation queries all relevant trade histories.
- **`ScanType` enum**: Replaces `IsLastScan` bool with `NormalScan`, `ExitScan`, `EntryScan` for explicit scan purpose classification.
- **Configurable scan schedule**: `scan_minutes`, `entry_scan_minute` (default 35), `exit_scan_minute` (default 25) — all configurable via JSON and dashboard API.
- **Bybit WS private order logging**: Added missing order update log (symbol, orderID, status, filled, avg) matching other exchanges' format.

### Fixed
- **Duplicate-symbol consolidator bug**: When multiple positions share the same symbol on the same exchange, consolidator now compares exchange totals against aggregate local totals instead of per-position. Prevents incorrect size syncing.
- **Duplicate-symbol exit bug**: Exit, reduce, and emergency close now use local recorded sizes (`pos.LongSize`/`pos.ShortSize`) instead of querying exchange totals that include sibling positions. Fixes `position is zero` errors and inflated PnL from multiplying close price by combined size.
- **Persistence filter rejection logging**: `isPersistent` now returns a descriptive reason string (e.g., "seen 1/2 times in 15m0s lookback" or "spread unstable ...") instead of a bare bool.

### Changed
- **Exit/rotation checks**: Now only run on `:25` (`ExitScan`) and `:35` (`EntryScan`), not every scan.
- **Scheduler simplified**: Removed empty `OnBeforeSnapshot` execute phase and `fireCallbacks`. Scheduler is now single-phase (rebalance only).

### Removed
- **`ExitMode` config**: Removed from Config, position model, dashboard API, JSON parsing, env var. Was stored but never read to control behavior — exit logic is purely spread-reversal + cost-based.
- **`OrderAdvanceMin` config**: Removed — only controlled timing for the empty `OnBeforeSnapshot` callbacks. Execution is now scan-driven via `EntryScan`.

## [0.9.1] - 2026-03-19

### Added
- **Spread stability filter**: For low-liquidity (high OI rank) opportunities, requires spread magnitude to be stable across historical scans before qualifying for execution. Prevents entering trades where the spread persists in direction but is volatile in magnitude (e.g., LYNUSDT oscillating 45-94 bps/h across scans). Configurable per interval tier with ratio threshold and OI rank gate.
- **scanRecord struct**: Scan history now stores `{Time, Spread}` pairs instead of bare timestamps, enabling spread analysis across scans.
- **Config**: 6 new fields under `persistence` — `spread_stability_ratio_1h` (default 0.5), `spread_stability_oi_rank_1h` (default 100), plus 4h/8h variants (disabled by default). Dashboard API updated for read/write.

### Changed
- **isPersistent()**: Now computes min/max spread ratio across the lookback window. If `minSpread/maxSpread < stabilityRatio` and `OIRank >= oiRankThreshold`, opportunity is rejected as unstable. High-OI (liquid) pairs skip the check entirely.

## [0.9.0] - 2026-03-18

### Added
- **Scan-aligned exit strategy**: Exit checks now run on every scan result (~10min intervals) instead of a fixed 300s timer. Evaluates each active position's funding spread and VWAP exit cost to decide whether to hold or close.
- **VWAP exit cost evaluation**: When funding spread goes negative, computes the cross-exchange price gap cost to exit (reversed sides: sell on long exchange bids, buy on short exchange asks). Calculates `recoveryHours = gapBps / abs(spread)` — if recovery <= `ExitMaxRecoveryHours`, exits immediately; otherwise holds until next scan.
- **Depth-fill exit loop**: Exit execution uses a 200ms depth-fill loop (same pattern as entry) with reduce-only IOC limit orders, VWAP accumulation, and market order fallback after `ExitDepthTimeoutSec` timeout. Circuit breaker at 5 consecutive failures.
- **L4/L5 exit preemption**: Exit goroutines use `context.Context` for cancellation. L4 `handleReduce` and L5 `handleEmergencyClose` cancel any running exit goroutine before proceeding with their own close logic. 500ms grace period for cleanup.
- **StatusExiting position state**: New `exiting` status between `active` and `closed`. Position slot remains occupied while exit goroutine runs.
- **Config**: `ExitMaxRecoveryHours` (default 4.0), `ExitDepthTimeoutSec` (default 45).

### Changed
- **Exit decision logic**: Replaced PnL-based exit (unrealized + funding >= $50) with cost-based: positive spread → hold, negative spread → evaluate VWAP gap recovery time. Spread reversal remains as safety override.
- **Rotation checks**: Now triggered on every scan result (alongside exit checks) instead of the removed timer loop.
- **Exported VWAPFromLevels**: `risk.VWAPFromLevels` now public for use by exit cost evaluation.

### Removed
- **Timer-based exit manager**: `StartExitManager`, `runExitManager`, `checkExits` — replaced by scan-aligned `checkExitsV2`.
- **PnL-based wait exit**: `evaluateWaitExit` (waited for PnL >= $50 after funding) — replaced by spread + cost evaluation.
- **ExitCheckIntervalSec config**: No longer needed; exit timing follows scanner schedule.

## [0.8.0] - 2026-03-18

### Added
- **Persistence filter**: Opportunities must appear consistently across multiple recent scans before qualifying for execution. Time-based lookback windows vary by funding interval (1h: 15min/2 appearances, 4h: 30min/4, 8h: 40min/5). Prevents entering trades on transient funding rate spikes. 6 new config fields under `strategy.discovery.persistence`.
- **NextFunding on Opportunity**: Verifier now populates `NextFunding` on each opportunity using the earliest funding time from both exchange legs, enabling scan-level funding window filtering.
- **GapBPS on RiskApproval**: Risk manager returns calculated cross-exchange price gap in approval response, passed through to `executeTradeV2` for dynamic spread threshold.
- **Funding window filter**: On the :35 scan, opportunities with next funding >30min away are filtered out (replaces per-opportunity timing guard in `executeArbitrage`).
- **Configurable monitor intervals**: `RiskMonitorIntervalSec` (default 300s, was hardcoded 30s) and `ExitCheckIntervalSec` (default 30s, was hardcoded) now configurable via JSON and dashboard.

### Changed
- **Scan-driven execution**: Replaced scheduler-based execution callback (T-OrderAdvanceMin) with scanner :35 scan trigger. Engine's main loop receives `ScanResult` with `IsLastScan` flag and triggers `executeArbitrage` directly. Simplifies timing — execution is always tied to the scan that validated the opportunities.
- **Fixed scan schedule**: Scanner now fires at :05,:15,:25,:35,:45,:55 each hour (was variable-interval polling). The :35 scan is the execution trigger; all others build persistence history and feed the dashboard.
- **Dynamic spread threshold in executeTradeV2**: Now uses `gapBPS` from risk approval as the spread ceiling instead of computing it inline from `MaxSpreadBPS` and recovery hours.
- **Config JSON restructure**: Added nested `persistence` group under `strategy.discovery`. Risk monitor interval moved under `risk` group.
- **Dashboard Config page**: Updated to display persistence filter settings in grouped UI.
- **simtrade CLI**: Updated `SimExecuteTradeV2` signature to accept `gapBPS` parameter.

### Removed
- **Immediate exit mode** (`evaluateImmediateExit`): Removed — closes too early, often before funding economics play out.
- **Timed exit mode** (`evaluateTimedExit`): Removed — arbitrary hold duration doesn't align with funding-based strategy.
- **MaxHoldHours**: Removed config field, default, JSON parsing, env var handler, and force-exit safety check. MaxHoldTime monitor alert (`checkMaxHoldTime`) also removed.
- **Per-opportunity funding time guard**: Removed from `executeArbitrage` — replaced by scanner's funding window filter at :35.
- **Scheduler OnBeforeSnapshot callback**: Execution no longer triggered by scheduler; only rebalance callback remains.
- **checkRotations in exit manager tick**: Rotation checks removed from the exit manager's periodic loop.

## [0.7.1] - 2026-03-17

### Fixed
- **confirmFill race condition**: `confirmFill` returned on the first WS update with any non-zero fill, not waiting for terminal state. On exchanges with rapid partial fills (e.g. OKX sending 820→830→...→16680 in 37ms), it returned a partial quantity (830) while the order was still filling (16680). The second leg was then sized to the partial, and closeLeg only recovered the partial — leaving the remainder as an orphan. Now waits for `filled`/`cancelled` terminal state before returning. Poll interval reduced from 500ms to 50ms for IOC orders.
- **Missing error in second-leg IOC failure log**: `depth tick: long IOC failed` and `short IOC failed` log messages did not include the actual error. Added `%v` format for the error in both short-first and long-first code paths.

### Changed
- **Dynamic spread threshold in depth fill loop**: Replaced fixed `MaxSpreadBPS` (40 bps) check with recovery-time-based threshold: `maxSpread = max(MaxSpreadBPS, opp.Spread * MaxGapRecoveryHours)`. High-spread opportunities (e.g. 82.5 bps/h) can now tolerate proportionally higher entry cost instead of being rejected by the fixed 40 bps cap. Same formula the risk manager uses. Falls back to `MaxSpreadBPS` as floor for zero/negative spread.

## [0.7.0] - 2026-03-17

### Changed
- **Parallel opportunity execution**: `executeArbitrage` now approves candidates sequentially (risk checks, funding guards) then launches all approved trades as parallel goroutines. Previously sequential execution meant each 5-minute depth fill loop blocked the next — with 2 opportunities, total time was 10min (past the funding snapshot). Now both execute simultaneously within the same 5-minute window.

## [0.6.1] - 2026-03-17

### Fixed
- **Gate.io WS quanto multiplier**: Private WebSocket order updates reported fill volumes in raw contracts instead of base asset units. `confirmFill` via WS returned 189 (contracts) while REST returned 18900 (base units), causing the depth fill loop to accumulate mismatched sizes between legs. Now applies `quanto_multiplier` to WS fills, consistent with REST.
- **Depth fill accumulation bug**: When the second leg partially filled or got 0 fill, `closeLeg` trimmed the excess but the accumulator still counted the full first-leg fill. This caused `confirmedShort` or `confirmedLong` to inflate, leading to massive imbalances (e.g. long=18900 short=37800). Now only the matched portion is accumulated.

### Added
- **Position consolidator**: Background goroutine (every 5min) reconciles local position records with actual exchange positions. Detects: missing legs (→ closes position), size mismatches >1% (→ syncs to exchange), orphan exchange positions not tracked locally (→ warns). Runs once at startup after 10s delay.

## [0.6.0] - 2026-03-17

### Added
- **Pre-execution cross-exchange fund rebalancing**: Two-phase scheduler fires rebalance at T-10min, then execution at T-6min. Analyzes capital needs per exchange from discovered opportunities, performs same-exchange spot→futures transfers (instant), then cross-exchange withdrawals via APT/BEP20 for remaining deficits. Prevents repeated `INSUFFICIENT_AVAILABLE` margin errors when batch execution consumes shared exchange funds.
- **Config**: `rebalance_advance_min` (default 10) — minutes before funding snapshot to start rebalancing.

### Changed
- **Scheduler**: Two-phase callback system with `OnBeforeRebalance` (rebalance phase) and `OnBeforeSnapshot` (execute phase).
- **Reverted pre-exec transfer**: Removed the per-trade spot→futures transfer block from `executeTradeV2` — superseded by the broader rebalancing approach.

## [0.5.3] - 2026-03-17

### Fixed
- **Wait exit closing positions before funding collection**: `evaluateWaitExit` had only a 5-minute minimum hold, causing positions to close on any positive unrealized PnL before collecting any funding payments. Now requires holding past at least one `NextFunding` snapshot (or `MinHoldTime` if NextFunding is unset) before allowing PnL-based exit.

### Changed
- **Dashboard Config page**: Replaced flat sorted key list with grouped sections (Strategy, Entry Execution, Exit, Price Gap, Rotation, Margin Health, Discovery, System) with human-readable labels and units.
- **Dashboard Positions page**: Added next funding countdown timer, exit mode badge, and rotation indicator (count + last rotated from).
- **Dashboard Overview page**: Added exit mode badges, next funding countdown, total funding collected stat, and win/loss breakdown.
- **Dashboard Opportunities page**: Added funding interval column (1h/4h/8h).

### Added
- **Config API**: 12 missing fields now exposed via GET/POST `/api/config` and persisted to Redis: `entry_timeout_sec`, `max_spread_bps`, `min_chunk_usdt`, `price_gap_free_bps`, `max_price_gap_bps`, `max_gap_recovery_hours`, `margin_l3_threshold`, `margin_l4_threshold`, `margin_l5_threshold`, `l4_reduce_fraction`, `rotation_threshold_bps`, `rotation_cooldown_min`.
- **Position types**: Added `last_rotated_from`, `last_rotated_at`, `rotation_count` to frontend Position type.
- **Opportunity types**: Added `interval_hours` to frontend Opportunity type.

## [0.5.2] - 2026-03-16

### Fixed
- **Gate.io quanto multiplier bug**: `GetPosition`, `GetAllPositions`, and `GetOrderFilledQty` returned raw contract counts instead of base asset units. When the engine passed these sizes back to `PlaceOrder` (which divides by the multiplier), orders were 10x too small for IRUSDT (mult=10). Close/rotation orders that should send 88 contracts were sending 8. Fixed by multiplying position sizes by the quanto multiplier on read, making Gate.io consistent with all other exchanges.

### Added
- Unit test `TestQuantoMultiplierRoundTrip` verifying the full cycle: `GetPosition` returns base units → `PlaceOrder` converts back to contracts → `GetOrderFilledQty` returns base units.

## [0.5.1] - 2026-03-16

### Fixed
- **Rotation settlement guard**: Block rotations within 10 minutes before or after a funding settlement. Pre-settlement rate spikes caused a false rotation trigger at 19:59 (1 min before 20:00 settlement), leaving an orphan position on Gate.io.
- **BBO unavailable log**: IOC close fallback now logs which exchange is missing BBO data (`long/bitget=true short/binance=false`) instead of a generic message.

## [0.5.0] - 2026-03-16

### Changed
- **Entry execution rewrite: depth-driven sequential IOC**. Replaces the concurrent fire-both-legs approach with an orderbook-aware incremental fill loop:
  1. **Subscribe to WS depth** on both exchanges before execution.
  2. **On each 100ms tick**: read top-of-book, check cross-exchange spread, size order to available liquidity (`min(remaining, bidQty, askQty)`).
  3. **Less-liquid leg goes first**: The side with higher `order_size / available_qty` ratio (more likely to fail) executes first. If it gets 0 fill, abort for free — no position to unwind.
  4. **Second leg fills confirmed quantity**: Only orders the exact amount the first leg filled.
  5. **VWAP entry prices**: Tracks weighted average across micro-fills for accurate PnL.
  6. **Incremental position updates**: Dashboard shows partial fills during the execution window.
- **New config parameters**: `entry_timeout_sec` (default 60), `max_spread_bps` (default 15), `min_chunk_usdt` (default 5).
- Old `executeTrade()` kept in codebase as reference.

### Added
- **WebSocket orderbook depth streaming** on all 5 exchanges (Binance, Bybit, Gate.io, Bitget, OKX). Three new methods on `Exchange` interface: `SubscribeDepth`, `UnsubscribeDepth`, `GetDepth`. Streams top-5 levels via the existing WS connection, on-demand during execution windows.
- **Depth live test** (test 18) in `cmd/livetest/` — validates subscribe, 5 bid + 5 ask levels, and unsubscribe cleanup on all exchanges.

## [0.4.1] - 2026-03-16

### Fixed
- **Rotation partial fill leak**: Single `closeLeg` only partially fills on low-liquidity pairs (e.g. Gate.io IRUSDT: 256 opened, only 26 closed back, leaking 230 per attempt). Replaced with `closeFullyWithRetry` — loops up to 10 market IOC attempts over 30s until fully closed.
- **Rotation pre-check**: Skip rotation if position notional < $10 or open fill < 50% of target size. Prevents repeated small partial fills that accumulate orphan positions.

## [0.4.0] - 2026-03-16

### Changed
- **Rotation rewrite: sequential open-first execution**. The old concurrent fire approach caused compounding partial fills across multiple retries, leaving orphan positions on 3 exchanges. New design:
  1. **Open new leg first** — if this fails, abort immediately. Original position is untouched.
  2. **Close old leg second** — only close the exact quantity that was opened. If IOC doesn't fill, retry with market. The old leg must fully close (reduce-only).
  3. **Cooldown on all outcomes** — `LastRotatedAt` is set on every attempt (success or failure) via `defer`, preventing rapid retry loops that compound errors.
  4. **No reversal logic** — the old code tried to "undo" partial fills with complex reversal paths that used `closeLeg` (reduce-only) where a non-reduce-only order was needed. The new code never needs reversal because nothing irreversible happens until the open is confirmed.
- **Extracted helpers**: `getBBOWithFallback()` (WS → REST orderbook) and `closeOldLegMarket()` (reduce-only market IOC).

## [0.3.3] - 2026-03-16

### Fixed
- **Rotation margin**: Check futures balance on new exchange before rotation, auto-transfer from spot if insufficient. Fixes `INSUFFICIENT_AVAILABLE` error when rotating to an exchange without pre-funded margin.

## [0.3.2] - 2026-03-16

### Fixed
- **Rotation BBO**: Subscribe new exchange to symbol price stream before rotation, with orderbook REST fallback when WebSocket BBO is not yet available. Fixes "BBO unavailable" error when rotating to an exchange that hasn't streamed the symbol yet.

## [0.3.1] - 2026-03-16

### Fixed
- **Bitget funding interval**: `GetFundingRate` now reads `fundingRateInterval` directly from the funding rate API response instead of relying on the contracts API which omits the field for some symbols (e.g. IRUSDT). Was defaulting to 8h instead of 4h, causing the risk monitor to report halved spread values (8.7 bps/h instead of 21 bps/h).

## [0.3.0] - 2026-03-16

### Added
- **Leg rotation**: When discovery finds a better exchange for one leg of an active position, the engine rotates just that leg via simultaneous IOC orders (close old + open new concurrently). Shared leg stays untouched — 2 trades instead of 4.
- **Anti-churn**: 30-minute cooldown between rotations, 2× threshold hysteresis to prevent oscillation back to a previously-abandoned exchange.
- **Config**: `RotationThresholdBPS` (default 20 bps/h), `RotationCooldownMin` (default 30min).
- **Position fields**: `LastRotatedFrom`, `LastRotatedAt`, `RotationCount` for rotation state tracking.
- **Unit test**: `TestClassifyRotation` covering all 4 leg-matching cases.

## [0.2.0] - 2026-03-16

### Changed
- **IOC entry execution**: Replaced three-phase BBO-tracking entry (limit orders + market fallback + consolidation) with single-phase simultaneous IOC limit orders. Both legs fire concurrently with slippage-capped prices, trimmed to smaller leg. Eliminates BBO tracking loop that caused 2.5% slippage.
- **IOC exit execution**: Replaced BBO-tracking smart close (~330 lines) with simultaneous IOC limit orders (~100 lines). Both legs fire concurrently with slippage buffer; unfilled remainder retries with market orders. No abort path — positions always close fully.
- **Discovery poll interval**: Reduced from 60s to 5min. Funding rates are slow-moving; execution uses real-time BBO snapshots regardless. Cuts API calls by 80%.
- **OrderAdvanceMin default**: Reduced from 16min to 3min to match IOC execution speed (no BBO tracking phase needed).
- **Fee model**: Entry now uses taker fees (IOC) instead of mixed maker/taker. Ranker and scanner updated to account for 4x taker fees round-trip.
- **Removed**: Three-phase execution (BBO tracking, market fallback, consolidation), `isStillProfitableWithMarketFees()`, gap guard in smart close, `SmartCloseDeadlineSec`/`SmartCloseGapMaxBPS` config fields (deprecated, unused).

### Fixed
- **Binance WebSocket**: Fixed JSON unmarshaling failure caused by `EventTime` field collision on the `e` JSON tag, which silently dropped all private stream messages.
- **Gate.io WebSocket**: Fixed `fill_price` JSON type mismatch (string vs number) in order update parser that caused zero fill prices.
- **Gate.io WebSocket**: Added app logger for diagnostics (was using stdlib log).
- **Binance WebSocket**: Added app logger for diagnostics (was using stdlib log).
- **OKX/Bitget WebSocket**: Added app logger for diagnostics.
- **Order sizing (wstest)**: Dynamic sizing to meet 5 USDT minimum notional; OKX exemption for large lot sizes.

## [0.1.0] - 2026-03-15

Initial release: funding rate arbitrage bot with discovery, execution, exit management, health monitoring, and React dashboard across 5 exchanges (Binance, Bybit, Gate.io, Bitget, OKX).
