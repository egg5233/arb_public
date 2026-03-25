# Changelog

All notable changes to this project will be documented in this file.

## [0.16.5] - 2026-03-25

### Added
- **Entry fee tracking in PnL**: Query actual trading fees from exchange API after opening position, include in real-time PnL calculation instead of waiting for reconcile
- **Entry Fees column in Positions dashboard page**

### Changed
- **Removed deposit address fields from Config exchanges tab**: Address management consolidated in Transfers page

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
