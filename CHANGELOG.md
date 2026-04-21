# Changelog

All notable changes to this project will be documented in this file.

## [0.33.0] - 2026-04-21

### Added (Phase 8 — Price-Gap Tracker Core, v2.0 milestone)
- New isolated subsystem `internal/pricegaptrader/` — cross-exchange price-gap event detection and delta-neutral IOC execution (Strategy 4 MVP). Default OFF (`PriceGapEnabled=false`).
- Config surface: 11 new `PriceGap*` fields (PG-05, PG-OPS-06), read once at startup from `config.json`.
- Redis namespace `pg:*` for position persistence (PG-04), exec-quality flags (PG-RISK-03), and slippage rolling windows. No collision with `arb:*` or `arb:spot_*`.
- Pre-entry risk gates: Gate concentration 50%, max concurrent, per-position notional cap, budget, delist/halt/staleness checks, exec-quality disable (PG-RISK-01..05).
- `cmd/pg-admin` — operator CLI for `enable|disable|status|positions list` (D-20). Reversal path for PG-RISK-03 auto-disable until Phase 9 dashboard ships. Operates entirely on Redis; config.json untouched.
- Circuit breaker on 5 consecutive PlaceOrder failures skips full ticks (D-10).
- Startup rehydration with orphan detection (§Pitfall 3): active positions re-enroll in monitors; zero-size ghost positions are closed with `ExitReasonOrphan`.
- 49 new tests in `internal/pricegaptrader/` (44 unit + 5 E2E) and 4 in `cmd/pg-admin/`. Full suite green under `-race -count=1`.

### Notes
- Perp-perp and spot-futures engines are byte-for-byte unaffected when `PriceGapEnabled=false`; safety-property test asserts zero `pg:*` Redis writes on the default-off path.
- Phase 9 will add a Dashboard tab, paper-mode toggle, runtime enable switch, and Telegram alerts; until then, operator workflow is: edit `config.json` + restart + `pg-admin` for disable reversals.

## [0.32.44] - 2026-04-21

### Added
- **Price-gap tracker — cmd/main.go conditional wiring (Phase 08 Plan 07, Task 3)** (`cmd/main.go`). Completes the tracker's main-binary integration:
  - **Startup (D-03):** AFTER `spotEng.Start()`. Guarded by `if cfg.PriceGapEnabled`; when true, constructs `pricegaptrader.NewTracker(exchanges, db, scanner, cfg)` and calls `Start()`. `*discovery.Scanner` satisfies `models.DelistChecker` via its existing `IsDelisted` method — no new interface surface needed.
  - **Shutdown (D-03 reverse):** BEFORE `spotEng.Stop()`. Since the tracker started AFTER SpotEngine, it stops FIRST under the reverse-order rule so its monitor goroutines wind down while dependencies (db, exchanges) are still fully live.
  - **Safety property (PG-OPS-06, T-08-27):** `PriceGapEnabled=false` (default) path logs `"Price-gap tracker disabled (cfg.PriceGapEnabled=false)"` and does NOT call `NewTracker` — zero goroutines spawn, zero `pg:*` Redis reads occur. Byte-for-byte isolation from perp-perp and spot-futures engines is preserved; the existing `spotEng`/`eng`/`apiSrv` wiring is unchanged.
  - Phase 08 Plan 07 complete — tracker is now end-to-end integrated. Phase 09 will add the dashboard enable toggle; today the switch lives in `config.json` only and is read once at startup.

## [0.32.43] - 2026-04-21

### Added
- **Price-gap tracker — end-to-end test suite (Phase 08 Plan 07, Task 2)** (`internal/pricegaptrader/tracker_test.go`). 5 tests driving the full tickLoop pipeline against a miniredis-backed real `*database.Client`:
  - `TestTrackerE2E_HappyCycle` — 4 bars of +253 bps spread feeds the ring → `runTick` opens the position via `openPair`, enrolls monitor, persists to `pg:positions`+`pg:positions:active`; then spread reversion + `checkAndMaybeExit` drives a clean close with `ExitReasonReverted`.
  - `TestTrackerE2E_GateBlocksOnBudget` — budget=500 < per-position=1000 → fired detection is blocked at the budget gate, zero orders placed on either leg, zero positions persisted.
  - `TestTrackerE2E_DisabledFlagBlocks` — pre-seeded `pg:candidate:disabled:SOON` blocks at Gate 1 (exec-quality), zero orders placed.
  - `TestTrackerE2E_RehydrationOnRestart` — pre-seeded active position + nonzero exchange positions → `Start()`→`rehydrate()` re-enrolls the monitor; wide BBO + long poll interval prevent false reversion-close during the test window.
  - `TestPriceGapEnabled_DefaultOff_NoTrackerInstantiated` — safety property (PG-OPS-06 / T-08-27): with `PriceGapEnabled=false`, the main.go-equivalent guard block does NOT call `NewTracker`; miniredis key scan asserts zero `pg:*` writes.



### Added
- **Price-gap tracker — real tickLoop dispatch + Start() rehydrate (Phase 08 Plan 07, Task 1)** (`internal/pricegaptrader/tracker.go`). Replaced the stub `tickLoop` with a real detect → gate → enter → monitor dispatch:
  - `tickLoop` fires the first tick ~7s after startup (RESEARCH §Pitfall 2 — offsets off the Bybit :04–:05:30 blackout window on fresh boot) then runs at `PriceGapPollIntervalSec` cadence via `time.Ticker`.
  - `runTick` iterates every configured `PriceGapCandidate`: `detectOnce` → on fired, `preEntry` gates → on approved, converts notional USDT to per-leg base-asset size via `mid = (MidLong+MidShort)/2`, calls `openPair` + `startMonitor`. Circuit-breaker-aware: skips the whole tick when `isCircuitOpen()` is true.
  - `Start()` now calls `rehydrate()` BEFORE spawning the tick goroutine so any restored positions are already enrolled in monitors before the first tick reads the active set for budget gating.
  - Local `active` view is appended after each successful `openPair` so back-to-back candidates in the same tick see up-to-date concurrency state.

## [0.32.41] - 2026-04-21

### Added
- **Price-gap tracker — exit monitor, exec-quality override, and startup rehydration (Phase 08 Plan 06)** (`internal/pricegaptrader/{monitor,slippage,rehydrate}.go`). Completes PG-03 + PG-04 + PG-RISK-03:
  - **Per-position exit monitor** — `startMonitor` spawns a goroutine per open position; `monitorPosition` polls at `PriceGapPollIntervalSec`. `closePair` fires on reversion (`|spread| ≤ ThresholdBps × PriceGapExitReversionFactor`, D-11) or max-hold (`now − OpenedAt ≥ PriceGapMaxHoldMin`). Exit places simultaneous IOC ReduceOnly orders on both legs; any shortfall retries as MARKET ReduceOnly (D-12 §Pitfall 4 — positions must close fully). Persists `ExitReason`, `RealizedPnL`, `RealizedSlipBps` (D-21 round-trip bps) and moves to `pg:positions:history`.
  - **Exec-quality auto-disable (PG-RISK-03)** — `recordSlippageAndMaybeDisable` appends one sample per closed trade to a rolling 10-trade window; if `mean(realized) > 2 × mean(modeled)` (strict `>`, D-19), sets `pg:candidate:disabled:<symbol>` so subsequent preEntry Gate 1 blocks with `ErrPriceGapCandidateDisabled`. Divide-by-zero guard: `meanMod ≤ 0` short-circuits, preventing the pathological "zero modeled → auto-disable everything" trap.
  - **Startup rehydration (PG-04)** — `rehydrate` iterates `pg:positions:active`; for each, queries `GetPosition(symbol)` on both legs and orphans any zero-size position with `ExitReasonOrphan` (RESEARCH §Pitfall 3 ghost-position replay). Survivors re-enroll in monitor goroutines. Idempotent: `startMonitor` cancels+replaces prior handles via an atomic seq token, so calling `rehydrate` twice leaves exactly one live monitor per position (race-safe under `-race`).
  - **Test coverage** — 14 new tests across `monitor_test.go` (6), `slippage_test.go` (5), `rehydrate_test.go` (3). All 44 pricegaptrader tests green under `-race -count=5`.
  - Still off by default (`PriceGapEnabled=false`). Phase 9 (dashboard) will expose `/api/pricegap/positions` and the enable toggle; Plan 07 wires `rehydrate()` into `Start()`.

## [0.32.40] - 2026-04-21

### Added
- **Price-gap tracker — simultaneous IOC entry path with unwind-to-match (Phase 08 Plan 05 Task 1)** (`internal/pricegaptrader/execution.go`). Implements the live-trading-critical `openPair` function for Strategy 4: concurrent IOC market orders on both legs, D-10 unwind-to-match partial-fill reconciliation via MARKET (not IOC) orders, zero-fill-on-one-leg → immediate market-close of the other leg, 5-consecutive-failures circuit breaker, per-symbol Redis lock via `AcquirePriceGapLock`, and D-15 position-ID format (`pg_{symbol}_{longExch}_{shortExch}_{unixNano}`). Fill price stamping uses the mid-at-decision from `DetectionResult` because the adapter's `GetOrderFilledQty(orderID, symbol) (float64, error)` returns only qty — realized slippage is computed at exit in Plan 06. Off by default (depends on `PriceGapEnabled=false` in Plan 01 + Plan 04 preEntry gate + this plan's circuit breaker). Tests in follow-up tasks.

## [0.32.39] - 2026-04-21

### Fixed
- **`isAlreadyFlatError` missed Binance `-2022 ReduceOnly Order is rejected`** (`internal/spotengine/execution.go`). After deploying v0.32.38, the DEGO monitor retry attempted a futures BUY-to-close against an already-flat futures position and got `-2022`. The existing pattern `"reduce only order"` (with space) didn't match the Binance message `"ReduceOnly Order is rejected"` (no space). Consequence: the idempotent-flat branch was skipped, `futErr` was returned, and `emergencyClose` returned that error even though the spot leg succeeded with the v0.32.38 rounding fix. The monitor then looped forever. Added two patterns to `isAlreadyFlatError`: `"reduceonly order"` and `"code=-2022"`. Test cases added to `TestIsAlreadyFlatError`.

## [0.32.38] - 2026-04-21

### Fixed
- **Emergency close path also needs LOT_SIZE step rounding** (`internal/spotengine/execution.go`, `emergencyClose`). Codex review of v0.32.36 caught a gap: delist-triggered exits enter `monitor.go:102-110` with `emergency=true` → `ClosePosition(..., true)` → `emergencyClose()` (parallel leg-close path). That path also formatted raw `remaining` before `PlaceSpotMarginOrder`, so the FIRST close attempt of an unaligned-`SpotSize` position still failed with Binance `-1013 LOT_SIZE` — only later non-emergency monitor retries healed it. Now `emergencyClose()` applies the same `utils.RoundToStep(remaining, rules.QtyStep)` logic (SELL side only; Dir-A buyback unchanged to preserve residual-borrow semantics). Additionally, after the rounded fill confirms, the known dust residue between `sellQty` and original `remaining` is marked as complete instead of being reported as a partial-fill error, so the emergency path now closes cleanly on its first attempt rather than leaving the position stuck until the next 3-minute monitor tick. Regression test: `TestEmergencyClose_SpotSellRoundsToLotStep` in `dust_close_test.go`.

## [0.32.37] - 2026-04-21

### Fixed
- **ManualOpen bypassed the delist filter** (`internal/spotengine/execution.go`). Root cause of the DEGO incident: the user manually opened `DEGOUSDT` via `POST /api/spot/manual_open` while the discovery poller had already written `arb:delist:DEGOUSDT` to Redis. Three entry paths exist for spot-futures — `autoentry` (covered by `risk_gate.go:125`), `monitor` active-position check (`monitor.go:102`), and `ManualOpen` (previously unchecked). `ManualOpen` only consulted `opp.FilterStatus` which the scanner never populates from delist signals; the delist check lives one layer deeper in `risk_gate.go` which `ManualOpen` skips entirely. Now `ManualOpen` calls `e.db.IsDelisted(symbol)` directly, gated on the same shared `DelistFilterEnabled` flag (default true) used by autoentry + monitor. Closes the accidental-open window that triggered the v0.32.36 LOT_SIZE stuck-exit incident. Override = toggle the existing flag off; no new config key added. Regression test: `TestManualOpen_RejectsDelistedSymbol`.

## [0.32.36] - 2026-04-21

### Fixed
- **Spot-futures Dir B exit stuck in 'exiting' when SpotSize is not aligned to Binance LOT_SIZE stepSize** (`internal/spotengine/execution.go`). Incident: `sf-degousdt-1776738361635` opened with `SpotSize=1258.1406`, futures leg closed successfully on delist-trigger exit, but every spot SELL retry was rejected by Binance with `-1013 Filter failure: LOT_SIZE` because `1258.1406` is not a multiple of DEGOUSDT's `stepSize=0.01`. Two root-cause fixes:
  1. **Entry path** — `openDirBBuySpotShort` previously hardcoded `math.Floor(spotNetReceived*1e5) / 1e5` (fixed 5dp) when flooring net-received size after fee deduction. For coins whose LOT_SIZE stepSize is coarser than 5dp (DEGO=0.01), this left `SpotSize` stepSize-unaligned. Now floors to `rules.QtyStep` fetched from `SpotOrderRules(symbol)`, with 1e-5 as fallback if the rules lookup fails.
  2. **Exit path (Dir B)** — `dirB-spot-sell` retryLeg closure now applies `utils.RoundToStep(remaining, rules.QtyStep)` before submitting the SELL order. If flooring produces zero (residue < step), short-circuits and marks the spot leg closed (dust residue ignored) instead of submitting a doomed order. Protects existing stuck positions — after deploy, the next 3-minute monitor retry for DEGO will successfully sell 1258.14 and dust-close the 0.0006 residue.
  
  Regression test added: `TestCloseDirectionB_SpotSellRoundsToLotStep` in `dust_close_test.go` reproduces the exact DEGO scenario (SpotSize=1258.1406, stepSize=0.01) and asserts the first `PlaceSpotMarginOrder` Size is `"1258.140000"` (step-aligned) and the position closes cleanly via the residue dust path.

## [0.32.35] - 2026-04-20

### Added
- **Per-recipient/per-donor debug trace logs in `dryRunTransferPlan`** (`internal/engine/allocator.go`) — 5 Debug-level log points for diagnosing rank-first Tier-1 prune decisions. Previously, `dryRun: no donor for X deficit=N` was the only visible infeasibility marker; it was impossible to trace from logs alone which donor sent how much to which recipient, whether min-withdraw overfunding consumed capacity, or whether post-trade-ratio headroom inflated the true transfer need. Five new logs:
  1. Entry: `dryRun: start — choices=N recipients=M needs=<map>`
  2. Per-recipient start: `dryRun: recipient X starts: need=... marginDeficit=... ratioDeficit=... transferNeed=...`
  3. Per donor→recipient move applied: `dryRun: moved ... from Y to X (gross=... fee=... netCredit=... donorBudgetAfter=... recipientDeficitAfter=...)`
  4. Per-recipient closeout (funded or infeasible with residual)
  5. Function exit: `dryRun: end — feasible=T/F reason=...` before each return site
  
  Debug-level only (requires `DEBUG=true` env var) to avoid log pollution — rotate scan runs every 60 min; Info-level would add ~500-1200 lines/day. Plan file: `plans/PLAN-dryrun-transferplan-logs.md` v2 (ALL PASS on Codex review).

## [0.32.34] - 2026-04-20

### Added
- **Rank-first rebalance — invert pool allocator from primary to fallback** (plan v29 ALL PASS on normal + independent Codex review). `rebalanceFunds()` now runs the sequential rank-first planner as the PRIMARY path; the pool allocator runs only as FALLBACK when Tier-1 produces zero selected choices. Rank represents expected profitability from `discovery`, so capital now preferentially funds the highest-rank feasible combination rather than the `baseValue`-maximizing bundle.
  - **New typed rejection taxonomy** (`internal/models/interfaces.go`): `RejectionKind` enum with `None/Capital/Capacity/Market/Spread/Health/Config`. `RiskApproval` carries `Kind` so callers can decide rescue eligibility without string matching.
  - **Per-site annotations** (`internal/risk/manager.go`): all 22 rejection returns in `approveInternal` now set `Kind`. Notional-floor sites `:438`/`:503` use new `notionalRejectionKind(effectiveCap, leverage)` helper — tags as `Config` (unrescueable) when `effectiveCap * leverage < 10`, else `Capital`. Helper and call site use a single effectiveCap + leverage snapshot computed once in `approveInternal` and threaded into `calculateSizeWithPrice` as parameters, eliminating concurrent-cap-change race.
  - **Shared transferable cache** (`internal/engine/allocator.go`): inline donor-surplus builder extracted into `(e *Engine) buildTransferableCache(balances)` — used by both pool allocator and rank-first Tier-1. `revalCache` in `runPoolAllocator` now reuses the extracted cache.
  - **Per-symbol pair walk** (`internal/engine/engine.go`): Tier-1 evaluates primary + each `opp.Alternatives` pair before falling to rank-2. Mirrors pool allocator's alt enumeration. `firstErr` is diagnostic-only and never gates the outer loop.
  - **Tier-1 / Tier-2 inversion**: rank-first runs first; pool allocator fallback inserted after the summary log, before Phase 1 balance mutation. `EnablePoolAllocator=false` skips fallback entirely. RotateScan dispatcher unchanged.
  - **Alt-override gate extension**: `altOverrideNeeded` flag set when chosen attempt is alt or differs from primary; override-storage gates (no-cross and executor paths) store `allocOverrides` even without transfer so alt-pair selections reach the entry scan via `applyAllocatorOverrides` (fixes silent-revert-to-primary semantic gap).
  - **Rejection taxonomy test coverage**: `internal/risk/manager_test.go` (new, 948 lines) pinning every rejection site to its Kind. `internal/engine/allocator_transferable_parity_test.go` (new, 287 lines) asserting `buildTransferableCache` matches pre-extraction inline logic. `internal/engine/engine_rank_first_test.go` (new, 924 lines) covering 20 plan scenarios including notional-floor sub-cases, alt-pair coverage (no-transfer + with-transfer), symmetric-replay drop, and RotateScan gate.

### Notes
- Throughput tradeoff (accepted per user intent): once Tier-1 keeps even one ranked choice, pool allocator is not invoked that cycle — it cannot backfill remaining slots with lower-ranked positive-value bundles. Total deployed capital per cycle may be lower than pool-allocator-first behavior when feasible bundle value > top-N rank value. Tier-1 success log `tier1 selected N opps, skipping pool allocator fallback` makes this observable.
- Known approximation: notional-floor Kind classification uses `effectiveCap * leverage` alone; sizing-zero from step rounding / short-leg price discount / min-size / invalid price are tagged `Capital` (not `Config`) in that regime, triggering a futile donor-rescue attempt (no correctness bug, just wasted work). `false-Config` under the chosen condition is mathematically impossible. Follow-up marker: if live logs show > 5 futile rescues/week, open a sizing-reason extension plan to return structured reject causes from `calculateSizeWithPrice`.

## [0.32.33] - 2026-04-20

### Fixed
- **Binance Dir A borrow-cost under-counting** (`pkg/exchange/binance/margin.go`) — codex re-review `0b5f1811` caught this pre-existing bug exposed by the new borrow-coverage floor. Binance's `/sapi/v1/margin/interestRateHistory` returns one record per day, but the adapter was emitting one `MarginInterestRatePoint` per record with `HourlyRate = daily/24`. The engine sums `HourlyRate × 10000` across points to compute total borrow cost in bps — so a 7-day backtest summed only 7 points × (daily/24) × 10000 instead of the correct 7 × daily × 10000 (24× under-count). With the new coverage floor, 7 daily points over a 7-day window was 4% coverage, correctly failing the floor but masking the real bug. Adapter now expands each daily record into 24 hourly-equivalent points within `[start, end]`, preserving the interface contract and producing correct bps totals. New `TestGetMarginInterestRateHistoryBinanceDailyExpansionCostAccuracy` asserts `Σ HourlyRate × 10000 == daily × 7 × 10000` over 7 days as regression guard.
- **Dir A backtest frontend/backend policy drift** (`web/src/pages/Opportunities.tsx`) — codex audit `14064872` finding 1. The Backtest button on OKX/Bitget Dir A rows was always disabled because `SPOT_DIR_A_BACKTEST_EXCHANGES` hard-coded the native-only list, even when `SpotFuturesBacktestCoinGlassFallback` was on and the backend was ready to serve the request. Removed the frontend capability gate; the UI now always allows clicking Backtest, and the backend's 400 response (with a clear message from `RunSpotBacktestOnDemand` or the 400 from `insufficient borrow history`) surfaces in the modal's error area.
- **Dir A sparse-borrow bootstrap trap** (`internal/spotengine/backtest.go`) — codex audit `14064872` finding 2. `fetchAndCacheSpotBacktestDirA` and `runBacktestDirAOnDemand` now enforce a ≥50% borrow-history coverage floor (`len(borrowSeries) / days*24`). During CoinGlass fallback bootstrap, the scraper accumulates one sample per hour; 24h of data over a 7-day window would previously produce misleading NetBps (missing hours default to zero via the borrowByHour map lookup) and cache for 24h, potentially allowing OKX/Bitget Dir A opportunities to false-pass the MinProfit filter. The prefetch path now fails open (no cache write) below the floor; the on-demand path returns a descriptive error so the modal displays an actionable "data source still accumulating" message.
- 3 new regression tests in `dir_a_coverage_test.go` cover sparse (24/168h → no cache), full (168/168h → cache), and on-demand sparse (10h → error) paths.

## [0.32.32] - 2026-04-20

### Added
- **SaveJSON tripwire for critical numeric fields** (`internal/config/config.go`) — defense-in-depth against config-clearing incidents. The `keepNonZero` helper now guards `spot_futures.max_positions`, `spot_futures.leverage`, `spot_futures.capital_unified_usdt`, and `spot_futures.capital_separate_usdt` on every SaveJSON write. If a save attempts to zero a currently-non-zero on-disk value for any of these, the tripwire logs a loud `[config] TRIPWIRE: refusing to zero ...` line (including the Go caller) and preserves the disk value. Legitimate operational changes flip `*Enabled` booleans rather than zeroing capital/leverage, so an incoming zero is almost always a bug (stale dashboard state, missing guard in an update path, etc.). The tripwire is narrow — it does not block the save, only preserves the specific field.
- 10 unit tests covering disk-preserved, disk-missing, type-mixing, and normal-update cases.

### Fixed
- Live incident recovery: between v0.32.30 deploy (13:34) and v0.32.31 deploy (13:48), `spot_futures.max_positions`, `spot_futures.leverage`, and `spot_futures.capital_unified_usdt` were silently zeroed during a SaveJSON write with an unidentified caller. Manual restore from `config.bk` was required. The tripwire above makes future occurrences visible via logs and prevents the zeroing.

## [0.32.31] - 2026-04-20

### Fixed
- **Spot-Futures backtest toggles not saving from dashboard** (`internal/api/handlers.go`) — latent bug present since v0.32.21 (Dir B) and uncovered by v0.32.30 (Dir A CoinGlass fallback). The `spotFuturesUpdate` request struct and `configSpotFuturesResponse` response struct in the `/api/config` handler were both missing the four backtest fields (`backtest_enabled`, `backtest_days`, `backtest_min_profit`, `backtest_coinglass_fallback`). Dashboard POSTs parsed into a struct with no matching fields so values were silently dropped — the toggle UI would flip back on refresh because the GET response also lacked the fields. Fixed by wiring all four fields through the GET response (struct + build block) and the POST request (struct + apply block). Underlying `SaveJSON`/`Load` paths in `internal/config/config.go` were already correct; only the API handler was unplumbed.

## [0.32.30] - 2026-04-20

### Added
- **Spot-Futures Dir A backtest — CoinGlass fallback for OKX and Bitget (Phase 2)**. The two exchanges lack public historical borrow-rate APIs; previously Dir A backtest was gated off for them. This release adds a fallback that reads from a hourly-accumulated Redis time series populated by `/var/solana/data/coinGlass/fetch_margin_fee.js` (new scraper, runs under PM2).
  - **New config field** (`internal/config/config.go`):
    - `SpotFuturesBacktestCoinGlassFallback bool` — JSON: `backtest_coinglass_fallback`, ENV: `SPOT_FUTURES_BACKTEST_COINGLASS_FALLBACK`, **default OFF**. Wire through `jsonSpotFutures`, JSON read, JSON write, env-var parsing, and defaults block.
  - **New database helper** (`internal/database/coinglass_margin_fee.go`): `GetCoinGlassMarginFeeHistory(exchange, coin, start, end) ([]CoinGlassMarginFeePoint, error)` reads the rolling Redis list `coinGlassMarginFee:hist:{exchange}:{coin}` (written by the scraper's hourly LPUSH + LTRIM 0 719), parses each entry, and filters by time window. Malformed entries skipped silently to survive scraper drift.
  - **New engine helpers** (`internal/spotengine/backtest.go`):
    - `exchangeSupportsCoinGlassDirAFallback(exch)` — returns true for `okx`, `bitget`.
    - `(e *SpotEngine) canRunDirABacktest(exch)` — layered gate: native list (`binance`/`bybit`/`gateio`) always true, fallback list gated by config flag.
    - `(e *SpotEngine) loadBorrowHistoryWithFallback(ctx, smExch, exchName, baseCoin, start, end)` — tries adapter's native `GetMarginInterestRateHistory` first; on `ErrHistoricalBorrowNotSupported` AND fallback flag enabled AND Redis has data, returns CoinGlass points converted to `MarginInterestRatePoint`. Preserves the sentinel when the fallback is off or empty so upstream fail-open logic stays intact.
  - **Discovery + prefetch hooks** now use `canRunDirABacktest` instead of the native-only `exchangeSupportsDirABacktest`, and both borrow-history fetch sites (`runBacktestDirAOnDemand`, `fetchAndCacheSpotBacktestDirA`) route through `loadBorrowHistoryWithFallback`.
  - **Bootstrap**: the CoinGlass scraper accumulates 1 sample per hour; 168 samples = 7 days of coverage. During bootstrap the fallback returns the sentinel (fail-open) and native-supported exchanges are unaffected.
  - **No UI change** — Dir A button already enables for supported exchanges at the engine layer.

### Fixed (review follow-ups on v0.32.30)
- **Dashboard toggle** for `SpotFuturesBacktestCoinGlassFallback` (`web/src/pages/Config.tsx`) — the config flag was previously wired only through `config.json` and env vars, violating the new-feature rollout rule (config + dashboard toggle + persistence). Now exposed as a toggle in the Spot-Futures → Dir B Backtest section, next to the existing backtest controls. i18n keys added to `web/src/i18n/en.ts` and `web/src/i18n/zh-TW.ts`. No API handler change needed — the generic `POST /api/config` path already accepts the new field via the existing `jsonSpotFutures.backtest_coinglass_fallback` wiring.
- **`GetCoinGlassMarginFeeHistory` JSON parsing** (`internal/database/coinglass_margin_fee.go`) — the parser declared a shared `entry` struct outside the loop. `encoding/json.Unmarshal` leaves fields unchanged when the input omits them, so a valid entry followed by `{}` could inherit the previous sample's `t` or `rate` and be returned as real data. Replaced with a fresh struct per iteration and pointer fields (`*int64`, `*float64`) so missing required fields now skip the entry instead of inheriting or zero-polluting. New regression test `TestGetCoinGlassMarginFeeHistory_PartialEntryDoesNotInheritPriorValues` locks this in.

## [0.32.29] - 2026-04-19

### Fixed
- **Spot-futures "stuck in exiting" dust lockup** — positions no longer loop forever when the spot exit leaves an untradeable residual (first hit GUAUSDT, recurred on GWEIUSDT 2026-04-19).
  - New `SpotOrderRules(symbol)` on `SpotMarginExchange` — returns `MinBaseQty`, `QtyStep`, `MinNotional` from each exchange's **spot market** endpoint (Binance `/api/v3/exchangeInfo`, Bybit `/v5/market/instruments-info?category=spot`, Gate.io `/spot/currency_pairs`, OKX `/api/v5/public/instruments?instType=SPOT`, Bitget `/api/v2/spot/public/symbols`). 5-min per-symbol cache inside each adapter.
  - Close paths (`closeDirectionB`, `closeDirectionA`, `emergencyClose` spot goroutine) now detect untradeable dust via `isSpotResidualDust(rules, remaining, price)` — dust if `floor(remaining/step)*step < MinBaseQty` or `effective * price < MinNotional`. Marks the spot leg closed with a `(spot dust residue ignored: X BASECOIN)` note on `ExitReason`.
  - Dir A dust short-circuit additionally verifies `GetMarginBalance.Borrowed == 0` before marking closed — prevents losing track of outstanding borrow liability.
  - Futures close is now idempotent on "empty position" / "position not exist" errors via `isAlreadyFlatError` + `verifyFuturesFlat(GetPosition)` double-check. Populates `FuturesExit` from orderbook mid when the exchange provided no fill price.
  - Retry-compatible: currently-stuck positions auto-heal on the next monitor retry after deploy. No Redis cleanup required.

### Fixed (review follow-ups on v0.32.29)
- **Bitget spot rules parser** (`pkg/exchange/bitget/spot_rules.go`) — the live Bitget v2 `/api/v2/spot/public/symbols` response uses `quantityPrecision` (not `quantityScale`). The original parser silently defaulted `QtyStep` to 1, which could cause `isSpotResidualDust` to flag a tradeable residual as dust on Bitget symbols with real sub-integer step precision. Parser now reads `quantityPrecision` with `quantityScale` retained as a legacy fallback. Golden fixture re-recorded from a live endpoint response; added a test that covers the legacy-field fallback path.
- **Bybit spot rules parser** (`pkg/exchange/bybit/spot_rules.go`) — the live Bybit v5 `/v5/market/instruments-info?category=spot` response nests `minOrderAmt` inside `lotSizeFilter`, not inside a separate `quoteFilter` object. The original parser read only `quoteFilter.minOrderAmt`, so `MinNotional` silently parsed as 0 — meaning `isSpotResidualDust` would never catch a sub-notional residual via the notional check and instead keep attempting doomed orders (the opposite failure mode from the Bitget bug: preserving a dust lockup rather than skipping a tradeable residual). Parser now reads `lotSizeFilter.minOrderAmt` first, with `quoteFilter.minOrderAmt` retained as a legacy fallback. Golden fixture trimmed to the current v5 shape (lotSizeFilter only); added `TestSpotOrderRulesBybitMinNotionalInLotSizeOnly` (would fail under the old parser) and `TestSpotOrderRulesBybitLegacyQuoteFilterFallback` (covers version drift).

## [0.32.28] - 2026-04-19

### Added
- **Spot-Futures Dir A (`borrow_sell_long`) backtest capability** for Binance, Bybit, and Gate.io. OKX and Bitget remain deferred (OKX has CSV-only export; Bitget's history endpoint is user-scoped, not public market rates).
  - **New `SpotMarginExchange` interface method** (`pkg/exchange/types.go`): `GetMarginInterestRateHistory(ctx, coin, start, end) ([]MarginInterestRatePoint, error)`. Adapters that lack a public historical API return `ErrHistoricalBorrowNotSupported`; callers fail-open on this sentinel (same semantics as a cache miss).
    - Binance: `GET /sapi/v1/margin/interestRateHistory` — daily granularity, paginates in 30-day windows; `HourlyRate = dailyInterestRate / 24`.
    - Bybit: `GET /v5/spot-margin-trade/interest-rate-history` — hourly, paginates in 30-day windows, VIP level `No VIP`.
    - Gate.io: `GET /unified/history_loan_rate` — hourly, page/limit pagination with client-side `[start, end]` filtering (endpoint has no date params).
    - OKX + Bitget: return `ErrHistoricalBorrowNotSupported` immediately.
  - **`backtestDirA` filter** (`internal/spotengine/backtest.go`): computes `netBps = −Σ fundingBps − Σ borrowBps` over the configured lookback window. Cache key: `arb:spot_backtest:{symbol}:{exchange}:borrow_sell_long:{days}` with 24 h TTL. Cache miss → fail-open (opportunity passes filter). Unsupported exchange → no-op (no cache write). Shares existing Loris 429 backoff from `discovery.TriggerLorisBackoff`.
  - **`exchangeSupportsDirABacktest` helper** gates the filter in one place — currently `binance`, `bybit`, `gateio`.
  - **Discovery hook** (`internal/spotengine/discovery.go`): native-scan and CoinGlass-scan Dir A paths call `backtestDirA` when `SpotFuturesBacktestEnabled && exchangeSupportsDirABacktest(exch)` and the position is not already active.
  - **`RunSpotBacktestOnDemand`** now accepts `direction=borrow_sell_long` on supported exchanges (previously rejected with 400). Returns extended `SpotBacktestReport` with optional `funding_bps` and `borrow_bps` breakdown fields (zero-valued for Dir B — backward compatible).
  - **Dashboard** (`web/src/pages/Opportunities.tsx`): Backtest button is enabled on Dir A rows for `binance`/`bybit`/`gateio`; rows for unsupported exchanges show a per-exchange tooltip (`spotBacktest.modal.notSupportedExchange`). Modal renders two additional stat tiles — funding bps and borrow bps — alongside the existing net-bps tile.
  - **i18n** (`web/src/i18n/en.ts` + `zh-TW.ts`): `spotBacktest.modal.fundingBps`, `spotBacktest.modal.borrowBps`, `spotBacktest.modal.netBps`, `spotBacktest.modal.notSupportedExchange` (with `{exchange}` placeholder).
  - **No new config** — reuses `SpotFuturesBacktestEnabled`, `SpotFuturesBacktestDays`, `SpotFuturesBacktestMinProfit`.

### Fixed (review follow-ups on v0.32.28)
- **API handler now accepts Dir A** (`internal/api/spot_handlers.go`) — `handleSpotBacktest` previously rejected every direction other than `buy_spot_short` with a 400 before reaching the engine, making the new Dir A modal unreachable from the dashboard. The handler now accepts both `buy_spot_short` and `borrow_sell_long`, forwards to the engine, and surfaces engine errors (e.g. "unsupported on okx") as 400 so the UI can render them inline. Two new tests cover the routing and the error-surfacing behavior.
- **Frontend Dir-A detection uses `opp.direction`** (`web/src/pages/Opportunities.tsx`) — previously checked `result.funding_bps !== undefined`, but Go's `omitempty` would drop a legitimate zero-valued `funding_bps` and cause the modal to fall back to the Dir B layout. Now keys off the already-known `opp.direction` instead; the Dir A tile values use `?? 0` defensively.
- **`TestBacktestDirASignMath` strengthened** (`internal/spotengine/backtest_test.go`) — now asserts the exact expected values (`FundingBps=15`, `BorrowBps=24`, `NetBps=-39`) in addition to sign direction. Catches magnitude regressions, not just sign flips.

## [0.32.27] - 2026-04-18

### Added
- **Spot-Futures Dir B backtest capability** (integrated from egg's `feat/spot-futures-dir-b-backtest`, originally tagged v0.32.21 on their branch — renumbered here to preserve our sequential numbering) — historical funding filter for `buy_spot_short` opportunities + on-demand UI backtest modal. Dir A (`borrow_sell_long`) is not yet supported (pending historical borrow-rate source).
  - **Background filter** (`internal/spotengine/backtest.go`, new): `backtestDirB` checks N days of historical Loris funding data after the net-APR check. Cache miss = fail-open (same as perp-perp). `prefetchSpotBacktestData` prefetches for passing opps after each scan. `RunSpotBacktestOnDemand` runs a fresh uncached fetch for the UI.
  - **Discovery hook** (`internal/spotengine/discovery.go`): if `SpotFuturesBacktestEnabled` and direction is `buy_spot_short`, calls `backtestDirB`; sets `FilterStatus` to reason on fail. Active positions bypass. Dir A unchanged.
  - **New config fields** (`internal/config/config.go`):
    - `SpotFuturesBacktestEnabled bool` — JSON: `backtest_enabled`, ENV: `SPOT_FUTURES_BACKTEST_ENABLED`, **default OFF**
    - `SpotFuturesBacktestDays int` — JSON: `backtest_days`, ENV: `SPOT_FUTURES_BACKTEST_DAYS`, default 7
    - `SpotFuturesBacktestMinProfit float64` — JSON: `backtest_min_profit`, ENV: `SPOT_FUTURES_BACKTEST_MIN_PROFIT`, default 0 bps
  - **New endpoint** (`internal/api/spot_handlers.go`): `POST /api/spot/backtest` — parses `{symbol, exchange, direction, days}`, rejects Dir A with 400, calls `RunSpotBacktestOnDemand`, returns standard `{ok, data}` envelope.
  - **Dashboard config controls** (`web/src/pages/Config.tsx`): enable toggle, days input, min-profit-bps input under Spot-Futures tab, mirroring perp-perp backtest controls.
  - **On-demand modal** (`web/src/pages/SpotPositions.tsx`): Backtest button on each Dir B opportunity row; modal shows funding sum bps, APR projection, settlement count, coverage %, and per-day breakdown. Dir A button disabled with tooltip.
  - **i18n**: matching keys added to `web/src/i18n/en.ts` and `web/src/i18n/zh-TW.ts`.

### Fixed (review follow-ups merged in)
- **Backend/frontend JSON contract** — `SpotBacktestReport` now emits the field names the dashboard expects: `projected_apr`, `settlement_count`, `coverage_pct` (0–100 percent, not 0–1 ratio), and `days: [{date, bps}]`. Prevented the on-demand modal from crashing at `result.days.length` on first successful response.
- **Stop-aware prefetch goroutine** — the spot backtest prefetch goroutine is now tracked in `SpotEngine.wg` via `launchBacktestPrefetch`, with a `sleepInterruptible` helper and `e.stopping()` checks inside the loop. `Stop()` waits for it to finish before returning.
- **Shared Loris 429 backoff** — moved the rate-limit cooldown to package-level `discovery.LorisBackoffUntil` / `TriggerLorisBackoff`, shared between the perp-perp `Scanner` and the spot-futures `SpotEngine`. `FetchLorisHistoricalSeries` auto-triggers on 429, so a 429 seen by either engine now suppresses the other instead of each engine keeping its own cooldown.

## [0.32.26] - 2026-04-18

### Fixed
- **Pool allocator executor now honors simulation's Steps (planner/executor parity).** At 2026-04-18 14:45 UTC on v0.32.25, pool allocator selected SIGNUSDT gateio/bingx. Dry-run `simulation passed`, gateio→bingx 132.55 USDT executed successfully, but override dropped (`kept=0`) with empty shortReason and entry at 14:55 rejected (gateio post-trade ratio 0.91 > L4 0.80). Codex deep-dive confirmed: `dryRunTransferPlan` produces `Steps []transferStep` (donor→recipient assignments with fee/chain/minWd), but `executeRebalanceFundingPlan` threw away `Steps` and re-ran greedy donor selection from `needs` only. Same bug class as v0.32.24 Bug B (sequential planner/executor mismatch), in the pool allocator path. When the same exchange served as both donor AND trading leg, executor's re-greedy routing drained more than sim validated → post-exec state failed `keepFundedChoices` replay. Plan v3 ALL PASS + Codex post-review PASS. Changes:
  - **`transferStep` struct** (`internal/engine/allocator.go:68-75`): added `MinWithdraw float64` field. `dryRunTransferPlan` now populates `MinWithdraw: minWd` (line 1228-1233) so executor's late min-withdraw safety check can use sim's validated value, not rediscover it.
  - **`allocatorSelection` struct** (`:257-263`): added `plan dryRunResult` field. `runPoolAllocator` assigns `plan: simResult` before returning the feasible selection.
  - **`executeRebalanceFundingPlan` signature** (`:1300`): added `plannedSteps []transferStep` parameter.
  - **Pool allocator call site** (`internal/engine/engine.go:633`): passes `allocSel.plan.Steps` instead of `nil`.
  - **Sequential fallback call site** (`:1097`): passes `nil` (sequential has its own dry-run prune per v0.32.24 Bug B).
  - **Executor loop** (`allocator.go:1477-1616`): builds `plannedByRecipient` map; for each recipient's cross-deficit, if `plannedSteps != nil`, dequeues steps from the map (using step.From/Fee/MinWithdraw/Chain directly) instead of running greedy donor selection. Greedy fallback preserved when `plannedSteps == nil`. Variable scoping renamed to avoid Go redeclaration (chain/fee/minWd now declared at loop top).
- Net effect: pool allocator's simulation is now binding — if sim validates a specific donor→recipient→amount routing as feasible, executor will execute exactly that plan. Prevents donor-leg overlap divergence where greedy re-allocation drained a donor's own-leg capacity.

## [0.32.25] - 2026-04-18

### Fixed
- **Cross-exchange withdraw rate-limit throttle.** At 2026-04-18 12:45:10 UTC, rebalance for SIRENUSDT triggered two gateio withdraws 1ms apart (bingx + binance recipients); second hit `code=TOO_FAST Withdrawal frequency is limited to 10s`. gateio had 314 USDT available — not a balance issue. Bot's batched-withdraw executor (`internal/engine/allocator.go:1826`) had no per-donor timing gate. Added `lastWithdrawAt map[string]time.Time` in the batched-withdraw loop: before each `Withdraw()` API call, if `time.Since(lastWithdrawAt[donor]) < WithdrawMinIntervalMs`, sleep the remaining delta; timing anchor is request-dispatch (not response-completion). New config `WithdrawMinIntervalMs` (default 11000ms = 11s, 1s buffer above gateio's 10s hard limit; bybit documented same 10s-per-coin/chain limit) fully wired through `Config` / `jsonRisk` / defaults / `applyJSON` / `SaveJSON` in `internal/config/config.go` plus `configRiskResponse` / `buildConfigResponse` / `riskUpdate` / `handlePostConfig` / Redis flat-map in `internal/api/handlers.go`. Dashboard: new `NumberField` in Config → Appearance (actually Risk) tab, i18n keys added to `en.ts` and `zh-TW.ts`. Regression test at `internal/api/config_handlers_test.go::TestHandleConfig_WithdrawMinIntervalMs` covers GET, POST, in-memory update, Redis persistence. Plan v5 ALL PASS + Codex post-review PASS (with missing test finding addressed).

## [0.32.24] - 2026-04-18

### Fixed
- **Rebalance partial-success: 4 bugs from post-v0.32.23 3-round log audit.** Plan v5 ALL PASS + Codex post-review PASS. Round 2 of 3 rounds opened SOONUSDT successfully, but Rounds 1 and 3 still had transfers execute without positions opening. Codex 2097-line audit surfaced 4 additional bugs (Bug E over-solicitation deferred as optimization):
  - **Bug A+D (allocator.go:2024-2067) — Deadline-fallback missing accounting + unified uses wrong balance API.** v0.32.23's Bug 5 deadline-fallback credited spot but never wrote `result.Unfunded[recipient]` or `result.SkipReasons[recipient]`, so `complete (unfunded=0)` log was misleading. Also: for unified recipients (bybit UTA, gateio unified), `GetSpotBalance()` always returns 0, so late-arriving deposits on unified exchanges could not be recovered. Fix: deadline-fallback now branches on `IsUnified()` — unified uses `GetFuturesBalance()` and credits `bi.futures`/`bi.futuresTotal`; split uses `GetSpotBalance()` and credits `bi.spot`. Shortfall (not covered by late arrival) is written to `result.Unfunded` and `result.SkipReasons`.
  - **Bug A2 (allocator.go:1941) — Unified baseline `startBal` fallback used `.spot`.** When `GetFuturesBalance()` fails during deposit-baseline setup for unified recipient, old code fell back to `bi.spot` which is always 0 for unified → later poll math over-credited any late deposit as "arrived". Fix: fallback now uses `bi.futures` which represents the unified pool.
  - **Bug B (engine.go:695-941) — Sequential planner/executor mismatch stranded legs.** Sequential planner computed donor→recipient assignments into `plannedTransfers`, but executor ignored it and greedy-allocated donors from `needs` in fixed order. Round 3: gateio consumed all donors first, bingx got zero transfers despite being in rescue plan. Fix: planner-side validation via new `dryRunTransferPlan` prune loop. Added `sequentialChoice` struct + `selectedChoices []sequentialChoice` tracking both rescue AND normal selections. Dry-run verifies whole plan feasibility; while infeasible, pops the last-added choice (restoring `needs`/`reserved`/`selectedSymbols`) until feasible subset remains.
  - **Bug C (engine.go:981, 1062, 1077-1084, 1097-1105) — Sequential fallback never stored overrides.** The fallback path discarded `executeRebalanceFundingPlan` result with `_ = ...`, so partial rescue success (e.g. gateio funded but bingx not) could never steer entry at :55 — always went tier-3 tier-3-rank-based. Fix: tracked `localTransferHappened` flag in sequential flow scope (set on each successful local spot→futures); `crossDeficits==0` early return now stores overrides via `keepFundedChoices(selectedChoices, balances, nil)` when local transfers succeeded; cross-exchange path consumes `result` and gates override storage on `localTransferHappened || result.LocalTransferHappened || result.CrossTransferHappened`.
- Bug E (over-solicitation of ratioDef when marginDef is smaller) deferred — optimization, not correctness issue.

## [0.32.23] - 2026-04-18

### Fixed
- **Rebalance deposit race — 5 bugs + log fix from 2026-04-18 06:45 UTC SIGNUSDT incident.** Plan v3 ALL PASS + Codex post-review PASS. Root story: bingx alone could cover 111.42 USDT deficit to binance; bitget was unnecessarily bumped +10 to meet its 10-USDT min-withdraw floor, so target became 121.42. bingx landed in 40s, bitget late. 90% threshold falsely declared "all deposits confirmed" (bal=111.42 ≥ 109.28), then `TransferToFutures("121.42")` failed `code=-5013`. No retry despite 1m43s remaining rebalance lifetime. Non-unified credit path skipped on error → override dropped. At :55 entry, auto-transfer only covered the 67-USDT deficit, leaving 54.48 idle — post-trade L4 ratio hit 0.91 > 0.80 due to artificially low total → rejected. Fixes:
  - **Bug 2 (replay + credit for split-account).** `canReserveAllocatorChoice` now runs balances through new `replayUsableAllocatorBalance` helper that simulates split-account spot→futures (unified exchanges unchanged), so stranded spot no longer fails the override replay. Deadline-fallback credits `bi.spot` (not `bi.futures`) so replay handles the sweep.
  - **Bug 3 (sweep all in fixed-capital).** `internal/risk/manager.go` `ensureFuturesBalance` now uses `sweepTarget=1e9` in both fixed-capital and auto-size branches; previously fixed-capital left spot idle → L4 projection used artificially low futures total.
  - **Bug 4 (min-withdraw overfund guard).** When >= 90% of the original deficit is already scheduled from prior donors, the min-withdraw bump at `allocator.go:1589` is skipped (marks residual as `Unfunded`, no over-solicitation).
  - **Bug 5 (retry-to-deadline + Bug 1 cap + log).** Receiver block at `allocator.go:1977-2023` now: (a) caps `moveAmt` to `min(totalPending, arrivedAmt)` instead of transferring 100% target; (b) unified fast-path credits directly without `TransferToFutures` API call (mirrors v0.32.21 Section A); (c) split-account path on error uses `continue` to retry next poll iteration with fresh `GetSpotBalance()` until `pollDeadline`; (d) on deadline hit, credits `bi.spot` for override retention via Bug 2 replay. Log renamed `all deposits confirmed` → `deposit threshold met (90%)` to stop misleading triage.

## [0.32.22] - 2026-04-18

### Fixed
- **Gate.io `EnsureOneWayMode` used JSON body; Gate.io actually requires query parameter.** After v0.32.21 hard-failed startup on `EnsureOneWayMode` error, the bot entered a crash loop because Gate.io returned `MISSING_REQUIRED_PARAM: dual_mode` every call. Investigation: `/futures/{settle}/dual_mode` endpoint rejects JSON body and expects `dual_mode` as a query parameter. Previous adapter comment/test asserting "JSON body" was a wrong assumption (doc spec did not match runtime behavior). Fix: pass `dual_mode=false` in query string, empty body. Test renamed `UsesJSONBody` → `UsesQueryParam` and asserts query + empty body. Verified in production: all 6 exchanges now report "one-way mode confirmed".

### Integration
- Merged `origin/main` (egg's v0.32.20 Appearance toggle, frontend-only) into branch. VERSION bumped to 0.32.22 to resolve duplicate v0.32.20 commit collision (both branches had diverging v0.32.20 tags). CHANGELOG consolidates both v0.32.20 entries under the same section below.

## [0.32.21] - 2026-04-18

### Fixed
- **Post-v0.32.20 log audit — 7 bugs.** Open-ended Codex log audit over 11 rotate/entry cycles (since v0.32.20 deploy at 16:22 UTC) surfaced 7 additional issues affecting transfer-no-open reliability. Plan v2 ALL PASS + Codex post-review PASS. Shipped as single patch:
  - **Bug β — Bybit unified receiver TransferToFutures call was inappropriate.** `allocator.go` receiver credit path unconditionally called `TransferToFutures()` after deposit confirmation. For Bybit UTA, this returned `code=131212 user insufficient balance` because UTA has no separate spot wallet — deposit lands directly in unified pool. Failure skipped the credit path, leaving `PostBalances` and `FundedReceivers` stale; `keepFundedChoices` then dropped the override despite successful deposit. Gate.io unified already handles this via adapter no-op; Bybit needed gate at allocator call site. Added unified-check branch before `TransferToFutures` that credits balances directly + sets `CrossTransferHappened=true`.
  - **Bug 3 — Rebalance L4 headroom too tight.** All 5 deficit-sizing sites (`allocator.go`, `engine.go`) computed `targetRatio := MarginL4Threshold - marginEpsilon` where marginEpsilon=0.005. Transfers sized to hit L4 exactly; any tiny intervening price move pushed post-trade ratio to 0.81+ and entry rejected. Centralized via new `rebalanceTargetRatio()` helper using new `MarginL4Headroom` config (default 0.05 = 5% headroom below L4). Full config wiring added.
  - **Bug 2 — Binance -2019 insufficient margin wasted 5 IOC retries.** First-leg IOC returned `code=-2019` when cached balance diverged from live. Engine retried 5× same size → circuit breaker trip → trade aborted. Added `refetchMarginCappedSize()` helper; first-leg IOC handlers (short + long) now detect margin error via existing `isMarginError`, refetch live balance, downsize once, retry; if refetched size < minSize, abort immediately. Second-leg handlers unchanged (require rollback/match, not downsize).
  - **Bug 4 — Override stored even when no transfer occurred.** `rebalanceFunds` stored allocator override whenever it selected a pair, regardless of whether actual transfer happened (`crossDeficits empty after local fund relief` case). Next entry scan hit "all overrides stale" → tier-3 fallback. Extended `rebalanceExecutionResult` with `LocalTransferHappened` + `CrossTransferHappened` bools. Set on successful local/cross transfer. Engine short-circuits override storage if neither flag is set.
  - **Bug 6 — BingX 109400 "order not exist" cancel spam.** `CancelOrder` returned `code=109400` when order already completed; handler logged WARN and fell through. Added 109400 to adapter-level idempotent code list (alongside existing 80018/80016); engine cancel-path falls through cleanly to the existing `GetOrderFilledQty` REST re-query for terminal state.
  - **Bug 1 — Gate.io EnsureOneWayMode startup error not hard-failed.** Every startup, gateio `EnsureOneWayMode` logged error and bot continued assuming one-way. If account actually in hedge mode, trades fail silently. Changed startup loop to `os.Exit(1)` on EnsureOneWayMode failure.
  - **Bug 7 — Donor capacity log spam.** Single INFO line emitted 30+ times per rebalance cycle from branch-and-bound paths. Downgraded to DEBUG.
- Bug 5 (allocator symbol cooldown on repeated rejection) intentionally deferred for separate patch.

## [0.32.20] - 2026-04-18

### Fixed
- **Rebalance partial-transfer: 3 bugs exposed by 2026-04-17 14:45 UTC SOONUSDT incident.** Rotate scan selected SOONUSDT binance/bingx; bingx needed 116.82 USDT cross-exchange. Four donors attempted, only bitget succeeded (33.84 USDT). Allocator override dropped (kept=0), entry scan re-ranked SOONUSDT as gateio/bingx, gateio had no capital, risk rejected at post-trade margin 0.98 > L4 0.80. Net: bitget's 33.84 USDT sat unused on bingx for a full scan cycle; no position opened. Fixes (plan v4 ALL PASS, Codex post-review PASS):
  - **Bug 1 — Binance split-account donor ignored authoritative `maxWithdrawAmount=0`.** Log showed `futuresAvail=187.66 maxTransferOut=0.00` yet `TransferToSpot 24.79` returned `code=-5013 insufficient`. Allocator's fallback-to-L4-safety-estimate path is intended for adapters that don't populate the field at all; Binance's `maxWithdrawAmount=0` with open positions is authoritative ("no withdrawable collateral"). Added `MaxTransferOutAuthoritative bool` to `exchange.Balance` (pkg/exchange/types.go), set `true` by Binance (always populated) and by Bybit (only when `/v5/account/withdrawal` endpoint succeeds — preserves v0.32.10 fallback for endpoint failures). Three allocator sites now gate on the flag: `allocatorDonorGrossCapacity` scan-time capacity (internal/engine/allocator.go), `dryRunTransferPlan` feasibility path, and executor pre-withdraw re-cap. Non-authoritative adapters keep the existing `0 = unknown` contract.
  - **Bug 2 — `moveAmt` formatted to `"0.0000"` string triggered bitget `code=40020`.** `freshBal.MaxTransferOut` could be a tiny positive (e.g. 0.00003) that passed `> 0` and `moveAmt <= 0` guards but rounded to `"0.0000"` via `%.4f`. Added post-format reparse guard before `TransferToSpot` — if the formatted string parses back to zero, skip with warning.
  - **Bug 3 — Override pair dropped despite receiver leg funded cross-exchange.** `keepFundedChoices` replayed choices against executor's `PostBalances`, which under-reflects reality when receiver's `spot→futures` has been applied but the post-balance projection lags. Once `kept=0`, entry scan fell to scanner's fresh ranking (SOONUSDT re-picked as gateio/bingx), ignoring that bingx just received 33.84 USDT via APT. Extended `rebalanceExecutionResult` with `FundedReceivers map[string]float64` populated after successful receiver `TransferToFutures`. `keepFundedChoices` now takes a third `funded` param; when a pair would drop AND either leg is in `funded`, re-fetches live balance via new `fetchLiveRebalanceBalance` helper (mirrors snapshot site — `GetFuturesBalance + GetSpotBalance + db.GetActivePositions()`) and re-runs the reservation check against live data. If feasible, retains the override — existing `applyAllocatorOverrides` then uses `opp.Alternatives` to patch the current scan back to the funded pair (binance/bingx), preventing cross-scan capital stranding.

### Changed
- **Dashboard design toggle — users can choose Classic (pre-v0.32.18) or New (Binance-inspired) design.** Default is now **Classic**, preserving the look that predates the v0.32.18 redesign for users who didn't like the rebrand. The New design is opt-in via Config → **Appearance** tab.
  - **`web/src/theme/index.ts`** (new): `ThemeContext` + `useTheme()` hook, mirrors the `LocaleContext` pattern. localStorage-backed (`arb-theme` key), default `'classic'`. No backend config — UI preference is per-browser, not per-deployment.
  - **`web/src/index.css`**: Binance color tokens moved out of global `@theme` into `:root[data-theme="new"]`, so Tailwind v4 utility classes (`bg-gray-400`, `text-gray-900`, etc.) resolve to Binance values only when the new theme is active. When `data-theme="classic"` (default), the Tailwind v4 built-in palette applies — restoring the pre-v0.32.18 look. `.btn-primary` / `.btn-secondary` / focus rings / body background all gate on `[data-theme]`.
  - **`web/src/App.tsx`**: wraps the app in `<ThemeContext.Provider>`, syncs `document.documentElement.dataset.theme` on every theme change via `useEffect` (no page reload needed). Mobile header, update banner, TradFi banner, and update modal each branch on theme to match the classic palette.
  - **`web/src/components/{Sidebar,StatusBadge,TimeRangeSelector}.tsx` + `pages/Login.tsx`**: each component reads `useTheme()` and renders the classic (pre-ae8c563) or new markup accordingly. Props and behavior identical — only visuals differ.
  - **`web/src/pages/Config.tsx`**: new rightmost strategy tab `Appearance` with a two-option toggle (Classic / New Design). Theme switches live on click.
  - **i18n**: 5 new keys synced in `en.ts` and `zh-TW.ts` (`cfg.tab.appearance`, `cfg.appearance.theme`, `cfg.appearance.theme.{new,classic,desc}`).
  - **Frontend-only change** — no Go code, backend, or `config.json` touched. No new npm dependencies.

## [0.32.19] - 2026-04-17

### Fixed
- **Bybit `GetClosePnL` had two PnL bugs — one sign flip + one missing funding term** (`pkg/exchange/bybit/adapter.go`). Reported against `nomusdt-1776033907476`; dashboard Close P/L showed `-73.26` and trade Total P/L showed `35.38`, both wrong. Live reconcile logs (`[reconcile-debug] ... shortAgg Fees=0.294327 Funding=-28.451038 PricePnL=-73.259710 NetPnL=72.965383`) confirmed both issues.
  - **Bug 1 — `PricePnL` sign-flipped for shorts**: `pricePnL := cumExit - cumEntry` is only correct for longs. Bybit's `cumEntryValue` / `cumExitValue` are unsigned notionals (no direction), so winning shorts had `ShortClosePnL` stored with the wrong sign (right magnitude). All other adapters (Bitget / Gate / BingX / OKX / Binance) pull a pre-signed PnL directly from their respective endpoints — only Bybit re-derived from cumulative notionals. Fix: flip sign after side normalization when `side == "short"`. Downstream field affected: `BasisGainLoss` (`exit.go:1255`, `consolidate.go:678` — sum of long + short PricePnL).
  - **Bug 2 — `NetPnL` missing funding**: prior code and comments ("closedPnl already includes funding") were wrong. Empirically `closedPnl = pricePnL − openFee − closeFee` with no funding term, but every other adapter returns `NetPnL = price + fees + funding`. The reconcile formula `long.NetPnL + short.NetPnL` therefore silently dropped Bybit's funding leg for *every* position where Bybit was a leg — regardless of long/short. For `nomusdt-...907476`: stored `RealizedPnL = 35.38 = −37.58 (Binance) + 72.97 (Bybit, no funding) + 0`; correct is `35.38 + (−28.45) = 6.93`. Fix: `NetPnL: closedPnl + totalFunding`, with matching comment/doc correction.
  - Backfill: `scripts/backfill_bybit_short_pnl.py` corrects existing `arb:history` entries in place for **both** bugs — flips `short_close_pnl` sign + recomputes `basis_gain_loss` for Bybit-short entries, and adjusts `realized_pnl` by `+bybit_leg_funding` for any reconciled entry where Bybit is a leg (either side). Dry-run by default; pass `--apply` to write. On current prod state: 3 Bybit-short + 6 Bybit-long reconciled entries affected for the `realized_pnl` adjustment; 3 Bybit-short entries additionally need the sign flip.

## [0.32.18] - 2026-04-17

### Changed
- **Binance-inspired design system (Phase 1 — foundation)** — establishes the visual/typographic contract documented in `DESIGN.md` (new, 500+ lines) and installs it via Tailwind v4 `@theme` tokens in `web/src/index.css`. The dashboard keeps its dark-first trading-terminal posture but rebrands to Binance's two-tone foundation: Binance Yellow (`#F0B90B`) as the singular accent for primary actions, Binance Dark (`#222126`) panels on deep-ink (`#0b0e11`) canvas, Crypto Green (`#0ECB81`) / Crypto Red (`#F6465D`) preserved for PnL semantics.
  - **`web/src/index.css`** (+323 lines): remapped Tailwind `gray` / `slate` / `zinc` / `neutral` scales to the Binance neutral palette via `@theme`, so existing utility classes (`bg-gray-900`, `text-gray-400`, etc.) automatically pick up the new brand without page-level rewrites. Also installs `--font-sans` / `--font-mono` stacks preferring `BinancePlex` with data-dense system fallbacks (Inter, JetBrains Mono).
  - **`web/src/components/Sidebar.tsx`** (+56/−30): yellow diamond brand mark, active-nav indicator rail (3px yellow bar on the left edge), pill-shaped locale switcher, uppercase-tracked nav labels. Connected/disconnected status dot shifts from generic green/red to Crypto Green / Crypto Red.
  - **`web/src/pages/Login.tsx`** (+61/−22): ambient gold radial glows on the dark canvas, brand-mark diamond + wordmark lockup, Binance-Dark card with `#f0b90b` focus ring on inputs.
  - **`web/src/App.tsx`** (+52/−28): mobile top-bar (`< md` only) with hamburger, brand diamond, and page title in the deep-ink surface; integrates with the Sidebar's drawer.
  - **`web/src/components/StatusBadge.tsx`** (+21/−7) + **`TimeRangeSelector.tsx`** (+7/−3): restyled to the new accent + neutral system.
- This is a **foundation-only** pass — remaining pages (Overview, Config, Analytics, Exchanges, Risk, Safety, Allocation, Spot Positions, Transfers, Rejections, Logs, Permissions, Trade History) still render through auto-remapped token colors but haven't been touched component-by-component. Their look is noticeably improved "for free" via the `@theme` scale remap, but follow-up passes are expected.
- No behavior changes, no new npm dependencies (respecting the axios lockdown), no backend changes.

## [0.32.17] - 2026-04-16

### Changed
- **Mobile UX overhaul for Opportunities / Positions / History pages** — all three used single wide tables with `overflow-x-auto` as the sole mobile strategy. Quantified audit at 390×844 (iPhone 14 Pro): Opportunities table 634px (1.6× viewport), Positions 795px (2.0×), History 1317px (3.4×). On Positions specifically, only 3 of 12 columns were visible — the close button, PnL, and next-funding countdown all off-screen, making the page functionally unusable on a phone. Replaced with dual-layout pattern via Tailwind `md:` breakpoint (≥768px keeps the existing table unchanged; <768px renders a card list):
  - **`web/src/pages/Positions.tsx`** (+260 lines): per-position card with symbol+rotation+SL badge header, long/short leg rows (exchange+size@price), 2×2 metric grid (Entry, Current, Funding Collected, Rot PnL), meta row (Age, Next Fund, fees), full-width Block/Close action buttons. Expand reveals Unrealized PnL per leg and grouped Funding History (per-day `<details>` with per-event rows).
  - **`web/src/pages/History.tsx`** (+171 lines): per-trade card with symbol+status badge+PnL (color-coded, prominent top-right), long→short exchanges+duration, 3-col grid (Entry Spread, Funding Collected, Rot PnL), red border+failure-reason preview on failed trades. Expand reveals full timestamps, entry/exit prices per leg, exit reason, and PnLBreakdown table.
  - **`web/src/pages/Opportunities.tsx`** (+491 lines): new `SpotCard` component + mobile card lists on both Perp tab and all Spot sections (compact split-view and full single-source). Perp cards show rank+symbol+spread, long/short exchanges with rates, interval/next-fund/OI meta row, and full-width Block+Open actions. Spot cards show direction badge, net APR prominent, 4-col metric grid (funding/borrow/fees/MR), Gap+Borrow lazy-check results inline.
- No desktop (≥md) visual changes — existing tables render identically. Mobile cards and desktop tables share the same state (expand, pagination, filter), rendered from the same source data.
- Verified at 390×844 (iPhone 14 Pro) via Playwright: all three pages now report `documentOverflow=false` (was 1.6–3.4× viewport before). Expand interactions confirmed working (PnLBreakdown, funding history).

## [0.32.16] - 2026-04-16

### Fixed
- **Cross-engine entry gate: prevent PP/SF from opening on same (exchange, symbol)** — v0.32.13–15 added post-execution `sfSubtract` at 20 reconciliation sites, but all are *same-side only* and *after PlaceOrder*. Neither engine's entry path consulted the other. Incident: SF dir-A held LONG 648.1 BARDUSDT on Bybit; PP discovery scored a short-Bybit opportunity and opened SHORT 991. Bybit's one-way mode (forced at startup via `EnsureOneWayMode()` on all 6 exchanges) netted `+648.1 − 991 = −342.9`, silently consuming SF's futures hedge. Consolidator's side-keyed `sfSubtract` returned 0 for the short-side query, synced PP to 342.9, and trimmed OKX long from 991→343. PP became internally consistent, but SF's record (648.1 long) became a time-bomb — its next `ClosePosition` would sell 648.1 into the −342.9 net, driving Bybit to −991 and re-corrupting PP. Three coordinated fixes:
  - **Fix A — Bidirectional entry gates** (`engine.go:2118-2128,2146-2153,2297-2304` + `spotengine/risk_gate.go:39-49,214-228`): PP's `executeArbitrage` now builds a `spotFuturesOccupied` map from `buildSpotFuturesMaps()` (stripping the side suffix), and `ppCrossEngineBlocked()` rejects any opportunity whose long or short exchange+symbol overlaps an active SF position. SF's `checkRiskGate` now queries `GetActivePositions()` (PP) and `spotEntryBlockedByPerp()` refuses entry when PP holds a leg on the target exchange+symbol. Both gates are hard blocks — no size offset, no cooldown needed (Redis reads are ~1ms, filter naturally clears when the other engine exits).
  - **Fix B — SF futures-size reality check** (`spotengine/monitor.go:113-124,197-280`): new `reconcileHedge()` piggy-backs the per-position monitor loop (~60s cadence). Calls `GetPosition(symbol)` on the exchange adapter, matches against `pos.FuturesSide`, and flags divergence when: (a) exchange reports opposite side, (b) exchange reports zero/flat, or (c) `|exchange − recorded| > max(1%×FuturesSize, stepSize)`. On detection: `MarkHedgeBroken()` persists `hedge_broken=true` via `lockedUpdatePosition`, logs ERROR, fires Telegram alert (`NotifySpotHedgeBroken`). New `HedgeBroken bool` field on `SpotFuturesPosition` (`models/spot_position.go`) — JSON-persisted with `omitempty` (absent = false = intact). In-memory mirror `HedgeIntact` (`json:"-"`) synced via `SyncHedgeState()` called on every DB read/write.
  - **Fix C — SF close-path guard** (`spotengine/exit_manager.go:429-435`, `spotengine/execution.go:1274-1280`, `spotengine/monitor.go:73-76,99-103,184-189`): `initiateExit` checks `HedgeBroken` before setting `StatusExiting`, preventing the stuck-exiting trap (review finding: status was set before `ClosePosition` abort). The stuck-exit retry path skips retrying when hedge is broken. Delist exits log ERROR + Telegram alert instead of issuing a doomed close. `ClosePosition` and the yield-exit dispatch retain their guards as defense-in-depth. Manual intervention required to recover.

## [0.32.15] - 2026-04-15

### Fixed
- **sfSizeOffset missed SF positions in SpotStatusExiting window** (codex review of v0.32.14, dispatch task `cbcc7ff8`) — `buildSpotFuturesMaps()` only counted `SpotStatusActive` positions toward the size offset, inherited from codex's v0.32.13 review ("pending/exiting may have flat or stale FuturesSize"). But SF flips `pos.Status = SpotStatusExiting` at `spotengine/exit_manager.go:446` BEFORE calling `ClosePosition(pos)` at `:467`, so during the exit execution window (seconds to minutes) the futures leg is still open on the exchange but excluded from the offset. Every v0.32.14 patched PP site then gets `sfSubtract = size - 0` and mishandles SF's still-live hedge — consolidator could close it, verifier could import its size into PP's record, entry fallback could absorb it as PP size. The two codex reviews gave contradictory advice: v0.32.13 (conservative — exclude exiting) opened a window the v0.32.14 findings aimed to close. Fix: remove the `SpotStatusActive` gate in `buildSpotFuturesMaps()` (`internal/engine/consolidate.go:283`) — include all non-closed SF positions (pending/active/exiting). `sfSubtract`'s zero-clamp handles the rare stuck-exit edge case (FuturesSize > exchange remainder) safely. Pending positions have `FuturesSize=0` so contribute harmlessly. This is a release blocker for v0.32.14 — without it, the 7 HIGH fixes have a timing gap that hits on every SF exit.

## [0.32.14] - 2026-04-15

### Fixed
- **Cross-engine interference at 7 HIGH sites (audit 260415-34e follow-up)** — v0.32.13 fixed 2 cross-engine interference bugs; the audit found 21 more (7 HIGH, 9 MEDIUM, 5 LOW). This release applies the same `sfSizeOffset` subtraction pattern at all 7 HIGH sites so the perp-perp engine never confuses a spot-futures futures leg for its own position on the same `(exchange, symbol, side)`:
  - **Shared helper** (`consolidate.go:252-307`): extracted `buildSpotFuturesMaps()` and `sfSubtract()` from the v0.32.13 inline code. `buildSpotFuturesMaps` returns both the orphan-exclusion set (all non-closed SF positions) and the size-offset map (SpotStatusActive only); `sfSubtract` performs the offset lookup with a zero-clamp.
  - **Finding 1** — SL method-2 verify (`engine.go:1577-1588`): an SF reduce-only fill on an exchange where PP also holds the same side could false-trigger `triggerEmergencyClose` on a just-closed PP position, causing duplicate bookkeeping. Now subtracts SF offset before the `remaining > 0` skip check.
  - **Finding 2** — orphan-cleanup verify (`engine.go:1826-1838`): `handleOrphanClose` post-close verification sees SF leg and reverts PP to Active with `LongSize/ShortSize = SF.size`. Next consolidator cycle would try to close SF's leg. Now subtracts SF offset before the dust check.
  - **Findings 3+4** — `markPositionClosed` close-leg + verify (`consolidate.go:511-587`): without the subtraction, (a) the close command's quantity includes SF's futures leg and could flatten SF's hedge, and (b) post-close verification sees SF size and leaves PP stuck in a ghost Active state forever. Now subtracts SF offset before both the close call and the dust check.
  - **Finding 5** — `reconcilePartialPosition` (`consolidate.go:898-904`): a `StatusPartial` PP position on a symbol where SF also holds a leg would promote to Active with SF's size imported as PP's `LongSize`/`ShortSize`, and the trim path would close SF's hedge. Now subtracts SF offset before promote/trim logic.
  - **Finding 6** — `closePositionWithMode` verify (`exit.go:1893-1909`): direct twin of the fixed rotation verify at `exit.go:2775`. Without the subtraction, a closed PP position would be reverted to Active with SF's size and stale SLs reattached against a phantom position. Now subtracts SF offset before the `notFlat` check.
  - **Finding 7** — entry `confirmFillSafe` fallback at 4 depth-fill paths (`engine.go:3624-3630`, `:3688-3693`, `:3855-3861`, `:3912-3918`): when confirmFill fails and the code falls back to reading exchange position size, any pre-existing SF leg on the same side was being written directly as PP's `LongSize`/`ShortSize`. Now subtracts SF offset so PP only records its own fill.

No source additions beyond the 7 fix sites + 2 helper functions. `go build` clean, `go vet` clean.

## [0.32.13] - 2026-04-14

### Fixed
- **Consolidator cross-engine size mismatch** — the perp-perp consolidator's size-mismatch check (`consolidate.go:280-281`) read the total exchange position for a symbol, which includes spot-futures futures legs. When both engines hold the same symbol on the same exchange and side, the consolidator flagged a false mismatch (e.g. BARDUSDT: perp-perp local=919, exchange=1225.6 including 306.6 from spot-futures dir A). Fix: build a `sfSizeOffset` map from active spot-futures positions and subtract from exchange totals before comparing. Only `SpotStatusActive` positions contribute to the offset — pending/exiting positions with stale `FuturesSize` are excluded (per Codex review).
- **Rotation step-size rounding causes unhedged position** — when rotating a long leg from exchange A to B, the rotation handler formatted the size for the new exchange (`formatSize(B, sym, 349)` → 300 due to step size), opened 300 on B, closed 300 on A, then the post-close verification found 49 remaining on A (349−300) and treated it as a failure. The abort path (a) overwrote `LongSize=49` in the position record, (b) rolled back the new B leg, but (c) did NOT re-open the 300 already closed on A — leaving the position 49 long vs 349 short (86% imbalanced). Incident: ARIAUSDT `ariausdt-1776041102139` on 2026-04-14 20:40 UTC, escalated to SL trigger at 23:48 UTC. Three fixes:
  - **Pre-check** (`exit.go:2648-2656`): if `formatSize` rounds more than 1% down, skip the rotation with a warning instead of proceeding to a partial rotation that will fail verification.
  - **Re-open on abort** (`exit.go:2819-2860`): when the "NOT flat" check fires, the handler now (1) closes the new leg (same as before), (2) re-opens the old leg via IOC order for `actualClosed = closeQty - remainingOnExch` (only the exposure actually lost, per Codex review), (3) does NOT overwrite position sizes — lets the consolidator reconcile.
  - **Diagnostic improvement**: distinguishes expected remainder (step-size) from unexpected remainder (real failure) in the log message.

## [0.32.12] - 2026-04-13

### Fixed
- **Stuck-active dust position chain (NATGASUSDT incident)** — Position `natgasusdt-1775865302363` sat Active for 3 days with `LongSize=ShortSize=7.1e-15` floating-point dust; two manual closes from dashboard failed; consolidator skipped recovery every 5 min. Six coordinated fixes after 10 rounds of Codex review (+ i5 fresh-eyes ALL PASS):
  - **Fix A — depth-exit dust snap** (`internal/engine/exit.go:899-915`): finalizer now snaps `longRemainder`/`shortRemainder` to 0 via new `isDust` helper (`internal/engine/dust.go`) using `max(stepSize, minSize)` as threshold, with `formatSize=0` fallback and `1e-10` floor when contract metadata is unavailable. `closedLong`/`closedShort` are NOT snapped (preserves VWAP/PnL math). Mirrors depth-loop done-criteria at `exit.go:402-417, 515-523`.
  - **Fix B — PnL sanity fallback** (`internal/engine/exit.go:920-940` + `:1898-1914`): when residual sizes drive notional to 0, fall back to stored `EntryNotional` (`models/position.go:48`) or skip the gate entirely. Logs WARN. Previous code zeroed real PnL on dust retries because `notional <= 0` made `Abs(pnl) > 0*2` falsely true.
  - **Fix C — consolidator UpdatedAt bypass** (`internal/engine/consolidate.go:317-336`): drop the 60s "recently updated" guard for the exchange-flat branch. `UpdatePositionFields` auto-bumps `UpdatedAt` on every successful mutate, and risk-monitor / funding tracker / exit-check write to active positions every few minutes for non-size reasons (CurrentSpread, NextFunding, ReversalCount, UnrealizedPnL), so the guard was permanently tripped — see NATGAS log `exchange-flat but recently updated (10s ago), skipping` repeating every 5 min for 3 days. Safety now provided by Fix F's per-position close lock + exchange-flat re-verify in `markPositionClosed`.
  - **Fix D — NextFunding stale on zero-rate** (`internal/engine/engine.go:1995-2024`): `NextFunding` advancement was nested inside `if fundingChanged`, which is `false` whenever both legs report rate=0. TSLAUSDT showed `next_funding=08:00:00Z` 30+ min after settlement passed during quiet hours. Lifted advancement out of the inner block; `FundingCollected` write stays gated to avoid overwriting with stale 0. Edge case: when both `fundingChanged` and `uplChanged` are false, the outer block is skipped — accepted as rare and tracked as follow-up.
  - **Fix E — `markPositionClosed` verify gate dust semantics** (`internal/engine/consolidate.go:495-507`): replace raw `> 0` check with `isDust()` so exchange-side rounding (e.g. `1e-7` in close response) doesn't block recovery. Reuses Fix A helper for consistent semantics.
  - **Fix F — per-position close lock + status claim + reread-with-fallback** (`internal/engine/consolidate.go:425-696` + `internal/engine/exit.go:1751-2002`): three-layer serialization to prevent double bookkeeping (history, stats, releasePerpPosition, reconcilePnL) when `markPositionClosed` (consolidator) races `closePositionWithMode` (L4 reduce → emergency close, delist, SL preemption). Outer: `AcquireOwnedLock("close:"+posID, 30s)` — Redis SET NX + Lua release + auto-renew (`internal/database/locks.go`). Middle: `UpdatePositionFields` predicate as inner stale-copy guard — `markPositionClosed` accepts only `Active|Exiting`, `closePositionWithMode` rejects only `Closing|Closed` (so preemption from `Pending|Partial` per `checkDelistPositions` filter still works). Bookkeeping: re-read persisted truth with `retry(3, 50/100/150ms)` + fallback to local pos on Redis transient failure (preserves all existing post-close side effects exactly once). Phase-2 close write strictly mirrors `exit.go:1902-1918`: only `LongExit`/`ShortExit` (>0 guards), `RealizedPnL`, `Status`. `ExitFees`/`LongClosePnL`/`ShortClosePnL` are NOT written here — they belong to `reconcilePnL`; pre-writing trips `InferHasReconciled` (`models/position.go:73-83`) and skips the real reconcile pass. `CancelAllOrders` kept BEFORE phase-2 save to prevent canceling orders on a re-used symbol after close.

## [0.32.11] - 2026-04-13

### Fixed
- **Unified donor rebalance ignored `maxTransferOut`** — For unified-account donors (Bybit UTA), the allocator's capacity calculation and pre-withdraw check used raw futures balance instead of the exchange-reported withdrawable cap. Bybit's `/v5/account/withdrawal` returns `availableWithdrawal` reflecting risk-delay freezes and position collateral that raw balance misses, so the allocator could request more than Bybit allows, triggering `code=131001 insufficient` even after v0.32.10's `accountType=UTA` fix. Production evidence 2026-04-12 15:46 UTC: `maxTransferOut=33.40` but allocator tried `60.02`. Added `maxTransferOut` clamps at `internal/engine/allocator.go:684-697` (scan-time capacity) and `:1610-1624` (pre-withdraw `effectiveAvail`), mirroring the existing split-account logic at `:1557-1560`. Gate.io unified unaffected (adapter does not populate `MaxTransferOut`).

## [0.32.10] - 2026-04-12

### Fixed
- **Bybit withdraw missing required `accountType`** — `pkg/exchange/bybit/adapter.go` `Withdraw()` did not send the `accountType` field that `POST /v5/asset/withdraw/create` lists as required (`doc/EXCHANGEAPI_BYBIT.md:1318`). Every automated rebalance donor withdrawal from Bybit was therefore failing with `code=131001 Account available balance insufficient` (7+ failures observed in VPS journalctl between 2026-04-09 and 2026-04-12; zero successful `bybit [rebalance] -> X` entries in Redis `arb:transfers`). Added `accountType: "UTA"` so Bybit internally transfers the required amount UNIFIED → Funding and withdraws in one call, matching the rebalance flow where donor funds live in UNIFIED. Prior BingX fix in v0.32.4 addressed the same pattern.

## [0.32.9] - 2026-04-12

### Fixed
- **Rebalance allocator override guard** — `rebalanceFunds()` previously stored allocator overrides unconditionally after calling a void `executeRebalanceFundingPlan()`, so overrides could survive even when the executor never actually moved funds. Next entry scan then attempted trades on unfunded exchanges and got rejected for L4 margin breach (2026-04-12 03:45–:55 incident: SIRENUSDT post-trade margin 0.94 on bitget after rebalance failed to transfer). The executor now returns `rebalanceExecutionResult{PostBalances, Unfunded, SkipReasons}`; the caller filters `allocSel.choices` via new `keepFundedChoices` helper (deterministic replay against post-execution balances) before storing overrides. Donor bookkeeping is now account-type aware (unified vs split), with rollback/refetch/pessimistic-zero fallback on batched-withdraw failure, merged-batch fee reconciliation on success, and `futuresTotal` tracked across all relief paths. 7 new regression tests in `allocator_override_test.go`. Verified via 10 Codex review passes.
- **Sequential rebalance relief missing `futuresTotal` update** — Same-exchange spot→futures relief in the sequential fallback path (`engine.go:982-989`) omitted `bi.futuresTotal += actualTransfer` while the sufficient-futures branch and allocator path both updated all three fields. Post-relief L4 ratio calculations on that branch used a stale `futuresTotal`.

## [0.32.8] - 2026-04-12

### Added
- **Override fallback when discovery returns 0** — When entry scan discovery returns 0 opportunities but allocator overrides from the :45 rotate scan exist (funds already transferred cross-exchange), the entry handler now re-scans those specific symbols through the full scanner pipeline (poll fresh rates → rank → verify → filters) to salvage the transferred capital. Previously the overrides were silently discarded at the next rebalance cycle, wasting the transfer fees. New `Scanner.RescanSymbols()` method and `models.SymbolRescanner` interface.

### Refactored
- Extracted entry filter chain (persistence, volatility, cooldown, interval, funding window, backtest, delist) into `Scanner.applyEntryFilters()` for reuse by both `runCycleInternal` and `RescanSymbols`.

## [0.32.7] - 2026-04-11

### Fixed
- **Spot-relief continue skipping L4 check** — When an exchange had spot balance > 0 (even dust like 0.001 USDT), the rebalance spot→futures relief branch unconditionally `continue`d past the post-trade L4 margin ratio check. Exchanges like OKX with tiny spot dust never got a crossDeficit entry even when post-trade ratio would exceed L4, causing wasted transfers to the other leg with no entry possible. Fixed in both pool allocator (allocator.go) and sequential rebalance (engine.go) paths. Also fixed futuresTotal not being updated after spot→futures transfers.

## [0.32.6] - 2026-04-11

### Fixed
- **Step size sync — coarse-first ordering + top-up alignment** — When two exchanges have vastly different step sizes (e.g., BingX 0.01 vs Gate.io 100), partial fills on the fine-step exchange produce amounts the coarse exchange cannot match (ARIAUSDT imbalance bug: long=176.18 vs short=100). Entry now overrides leg ordering so coarse-step exchange goes first. Second leg uses commonTradeableSize to verify alignment; if misaligned, tops up first leg to next common step instead of rolling back (saves fees). Exit keeps risk-leg-first ordering with same top-up logic; falls back to market close only if top-up fails.
- **Merge same-direction rebalance withdrawals** — When the rebalance loop picks the same donor for the same recipient multiple times (e.g., bingx→bitget 15.19 + 5.00), each withdrawal incurred a separate on-chain fee. Now accumulates withdrawals per donor→recipient pair and executes merged withdrawals after the loop, saving one fee per merged pair.

### Added
- `RoundUpToStep()` utility function for top-up step alignment calculations.

## [0.32.5] - 2026-04-10

### Fixed
- **Allocator appendChoice missing backtest validation** — Alternative exchange pairs were accepted without running CheckPairFilters (backtest/persistence/volatility). Could select unprofitable pairs, plan useless cross-exchange transfers. Now validates alt pairs before adding to candidates.
- **Tier-3 entry blocked by stale overrides** — When allocator overrides failed at entry, tier-3 fallback was blocked ("avoid unfunded entries"). Profitable opps like ARIAUSDT/SIRENUSDT were skipped. Now falls through to tier-3; risk.Approve still gates margins.
- **Rotation missing pair filter validation** — checkRotations could rotate into pairs that fail backtest/persistence. Now calls CheckPairFilters on rotation target before rotateLeg.

## [0.32.4] - 2026-04-09

### Fixed
- **BingX withdraw missing walletType** — Withdraw API requires walletType parameter (1=Fund, 2=Standard, 3=Perpetual). TransferToSpot lands funds in Fund account but Withdraw was called without walletType, causing "Insufficient balance" error. Added walletType=1.

## [0.32.3] - 2026-04-09

### Fixed
- **Gate.io unified balance double-count** — GetSpotBalance for unified accounts returned overlapping value from /spot/accounts (same money as /unified/accounts). Now returns zero for unified mode. Cross-exchange withdrawal donor and deposit polling use GetFuturesBalance for unified accounts.
- **Exit check at :40 removed** — v0.32.0 added exit checks in EntryScan handler, which defeated SpreadReversalTolerance=1 (tolerance requires full scan cycles, not 10-min gaps). Exits now only run at :30 as designed.

## [0.32.1] - 2026-04-09

### Debug
- **History/reconcile debug logging** — Added comprehensive debug logs to trace per-leg field writing through the entire flow: tryReconcilePnL aggregate values, needsBreakdownUpdate comparison, UpdatePositionFields mutator, GetPosition re-read, UpdateHistoryEntry, and AddToHistory. Also logs JSON content checks for `long_total_fees` and `has_reconciled` presence.

## [0.32.0] - 2026-04-09

### Fixed (Critical)
- **Rollback/trim one-sided exposure** — In-loop rollback and trim branches now correctly account for surviving exposure via `abortFillLoop` flag, ensuring VWAP stays in sync before breaking the fill loop. Telegram alerts for orphan exposure events.
- **rotateLeg DB swap silent failure** — Re-reads position after `UpdatePositionFields` to verify swap applied. On failure, writes `StatusPartial` with actual leg state (CAS-guarded against concurrent close). Broadcasts partial to dashboard.
- **Rotation not cancellable by L4/L5** — `rotateLeg` now registers full exit lifecycle (`exitCancels`, `exitDone`, context). Three cancel checkpoints at each critical stage with safe cleanup. New leg protected from consolidator orphan scan via `entryActive`.

### Fixed (High)
- **First-leg confirm zeroing real fill** — Detects actual exchange position via `getExchangePositionSize` when `confirmFillSafe` fails. Updates existing pending position to `StatusPartial` instead of archiving as zero.
- **Leverage clamp inconsistency** — Entry now uses `effectiveLev = min(cfg.Leverage, MaxLeverage())` consistently for SetLeverage calls and margin calculations.
- **Micro-order sizing short-only validation** — Looks up contract specs for both exchanges. New `commonTradeableSize` helper iteratively converges to a size both exchanges can represent.
- **entryActive rotation target check** — `checkRotations` now checks `entryActive` on the candidate replacement exchange before proceeding.
- **ManualClose strands StatusExiting** — `spawnExitGoroutine` returns `bool` and is sole authority for `active→exiting` CAS. ManualClose no longer pre-sets status.
- **Depth exit allocator leak** — Added `releasePerpPosition` call in depth-exit fully-flat branch.
- **FormatSize hardcoded 6 decimals** — L4 reduce and rotation open now use `e.formatSize(exchName, symbol, size)` instead of `utils.FormatSize(size, 6)`.
- **PnL double-count on rotate-back** — `tryReconcilePnL` queries from last `RotationHistory.Timestamp` instead of `CreatedAt`.
- **Reconcile retry missing 30s** — Added third retry at 30s to the delays slice.

### Optimizations
- **Depth stream pre-warming** — Subscribes depth WS at :35 for top candidates. Ref-counted subscriptions with 5s freshness check. Entry at :40 skips 3-8s wait when data is fresh.
- **Parallel market fallback close** — Both legs closed concurrently via goroutines in market fallback, reducing naked exposure time.
- **Risk-leg-first depth exit** — Closes the leg with thinner depth first instead of always long-first.
- **Parallel SetLeverage/SetMarginMode** — Both exchanges' setup calls run concurrently.
- **Exit check at :40** — `checkIntervalChanges` + `checkExitsV2` now run before `executeArbitrage` on EntryScan, reducing worst-case exit detection latency.
- **Per-position PnL lock** — Replaced global `pnlReconcileMu` with per-position locks via `sync.Map`. 2s sleep moved outside lock. Both `reconcilePnL` and `reconcileRotationPnL` updated.
- **Approval cache ActivePositions** — `PrefetchCache` now includes `ActivePositions` with incremental delta-update during batch approval.
- **SCAN replaces KEYS** — All 4 `Keys()` calls in allocator.go replaced with iterative `SCAN` (cursor-based, batch 100).
- **Exit priority ordering** — `checkExitsV2` now sorts by worst `CurrentSpread` first, largest notional as tiebreaker.

### Improved (Medium)
- **$10 per-leg floor in pre-trade** — `approveInternal` checks per-leg notional using per-exchange mid prices. Post-loop check also uses per-leg fill VWAPs.
- **Per-leg margin math** — `RiskApproval` extended with `LongMarginNeeded`/`ShortMarginNeeded`. Approval, buffer checks, projected ratio, and reservation all use per-exchange values.
- **Strategy-aware CapitalPerLeg** — `EffectiveCapitalPerLeg` accepts optional strategy param. Per-strategy dynamic caps prevent perp cap from bleeding into spot-futures sizing.

## [0.31.2] - 2026-04-09

### Fixed
- **RotateScan :35 risk bypass** — `rotateLeg()` now calls `risk.ApproveRotation()` with leverage clamp, MarginSafetyMultiplier, projected L4 margin ratio, exchange health scoring, and per-exchange exposure cap. Auto-transfer trigger uses buffered margin instead of bare requirement.
- **False-positive rotation on rate lookup failure** — `computeLiveSpread()` returns `(float64, bool)`; `checkRotations()` skips position when live spread is unavailable instead of treating failure as zero spread.
- **Nil-deref after rotation auto-transfer** — `GetFuturesBalance()` error after spot→futures transfer is now handled (was silently ignored, could panic).
- **Rotation race with exit/SL/consolidator** — `checkRotations()` now skips positions with active exit or entry; `rotateLeg()` claims `exitActive` for the duration of the rotation. L5/delist emergency close intentionally not blocked (known limitation).
- **Allocator pre-feasibility hardcoded floor** — `isAllocatorFundingFeasible()` always looks up exchange `minWithdraw` instead of only when deficit < $10; uses `feeCache` to avoid redundant API calls.

## [0.31.1] - 2026-04-08

### Fixed
- **ExitReason missing in position history** — 5 close paths wrote `AddToHistory(pos)` without setting `pos.ExitReason`, causing empty "平倉原因" on dashboard. Fixed: `spawnExitGoroutine` (depth exit + fallback close), `markPositionClosed` (consolidator), `triggerEmergencyClose` (SL/liquidation), `checkDelistPositions` (Binance delist), `reducePosition` (L4 full-flatten).

## [0.31.0] - 2026-04-08

### Added
- **deliveryDate-based delist detection** — primary delist signal: a 1h-cadence background poller (`internal/discovery/contract_refresh.go`) re-loads contract metadata via `LoadAllContracts` for every configured exchange and writes any near-future `DeliveryDate`-flagged perpetual to the existing `arb:delist:{SYMBOL}` Redis key. Catches batch delists where the article scraper's title regex misses (e.g. the 2026-04-08 OLUSDT/HIPPOUSDT/RLSUSDT/PUFFERUSDT batch announced under a generic title) — Binance updates `deliveryDate` at announcement time per their docs.
- **Bybit `deliveryTime` parsing** — `pkg/exchange/bybit/adapter.go` now extracts `deliveryTime` for `LinearPerpetual` contracts into `ContractInfo.DeliveryDate`, enabling Bybit delist detection symmetrically with Binance.
- **`ContractInfo.DeliveryDate` field** — new `time.Time` field on `pkg/exchange/types.go ContractInfo`, populated by Binance and Bybit adapters for true perpetuals being delisted (excludes the year-2100 sentinel and dated quarterlies).
- **Spot-futures delist parity** — spot-futures risk gate now has a delist check (step 7, before dry-run) and the monitor loop force-exits any active position whose futures symbol lands on the blacklist. Both engines now consume the same `arb:delist:{SYMBOL}` Redis key for a single source of truth.
- **`Database.IsDelisted` helper** — leaf-package read of the delist blacklist (avoids the import cycle that would result from spot-engine depending on `internal/discovery`).
- **`ContractRefreshInterval` config** — new `time.Duration` config field (`contract_refresh_min` JSON, default 60 minutes, 0 disables) controlling the new poller's cadence. Reuses `DelistFilterEnabled` as the on/off toggle.
- **Targeted regression test** — `TestLoadAllContracts_Binance_DeliveryDateParsing` pins live (zero), delisting (populated), and quarterly (zero) cases for the new `DeliveryDate` field.

### Fixed
- **Article-scraper-only delist detection blind spot** — root cause of the 2026-04-08 RLSUSDT loss. The scraper at `internal/discovery/delist.go` only parsed announcement titles; the batch delist used a generic title with no symbol tokens, so neither regex pattern fired and the bot entered RLSUSDT after the announcement. The new `deliveryDate` poller is title-format-independent and reads the same field from the same endpoint Binance updates at announcement time.
- **Binance-only auto-exit guard in `checkDelistPositions`** — `internal/engine/engine.go` now triggers emergency close on a delisted position regardless of which leg's exchange is involved. Previously, a delisting symbol with no Binance leg was logged and skipped, leaving the position to settle unfavorably.
- **Test stub compilation failures** — `internal/discovery/test_helpers_test.go` and `internal/api/funding_history_test.go` were failing to compile since v0.29.2 because their stub `Exchange` implementations (`stubExchange`, `fundingStubExchange`) were never updated when `CancelAllOrders` was added to the `exchange.Exchange` interface. Added the missing no-op method to both stubs so `go vet ./...` and `go test ./internal/discovery/... ./internal/api/...` compile cleanly. Tests were previously unrunnable in those two packages; now 45 tests pass across both.

### Changed
- **`Scanner.contracts` access is now lock-protected** — added `contractsMu sync.RWMutex` to guard the contracts cache, since the new contract refresh poller introduces concurrent writes alongside the existing reads from `hasContract`. `SetContracts`, `hasContract`, and the new `replaceContractsForExchange` all go through the lock.
- **Article scraper retained as belt-and-suspenders** — `internal/discovery/delist.go` is unchanged. When both signal paths write the same key with the same date, the operation is idempotent.

### Notes
- Out of scope for this release (deferred): Gate.io/Bitget/BingX/OKX `deliveryDate` equivalents (no equivalent fields on their contract endpoints per the doc survey), symbol-disappearance detection, and a dashboard surface for `deliveryDate`-blacklisted symbols.
- Merged in parallel with v0.30.0 (PnL reconcile safety + entry scan hardening). Both feature sets are part of this release.

## [0.30.0] - 2026-04-08

### Fixed
- **CRITICAL: Consolidator force-close writes partial PnL as reconciled** — added PartialReconcile field; InferHasReconciled skips inference for partial data; async reconcilePnL retries after force-close; AdjustWinLoss corrects win/loss counts on PnL sign change
- **CRITICAL: Trim-back failure activates position with unmatched exposure** — sentinel errPartialEntry pattern; force checkpoint before trim; post-trim sizes tracked accurately; caller skips cleanup on partial success
- **HIGH: Sequential rebalance passes deficits as needs** — promoted rebalanceDeficit to package-level type; added precomputedDeficits parameter to skip upper-half recalculation while preserving donor surplus math
- **HIGH: StatusPartial positions stranded after crash** — consolidator now reconciles StatusPartial: query exchange, trim to matched, promote or close; markPartialClosed zeros sizes
- **HIGH: Post-fill SavePosition failure leaves orphan fills** — 3-retry save; falls back to errPartialEntry with valid StatusPartial checkpoint in DB
- **HIGH: confirmFill treats unknown as zero-fill** — confirmFillSafe wrapper distinguishes REST failure from confirmed zero; first-leg unknown freezes depth loop; second-leg unknown saves partial with entry prices
- **HIGH: Allocator override stale falls through to unfunded tier-3** — applyAllocatorOverrides returns (filtered, hadOverrides); tier-3 blocked when allocator ran but all overrides stale
- **MEDIUM: No-diff reconciliation doesn't update history entry** — UpdateHistoryEntry (not AddToHistory) in no-diff branch; clears PartialReconcile
- **MEDIUM: Allocator commit failure not handled** — triggers allocator.Reconcile() on persistent commit failure
- **LOW: PnLBreakdown rotation_pnl not in hasDecomposition gate** — rotation-only positions now render breakdown instead of "data unavailable"

### Added
- `PartialReconcile` field on ArbitragePosition (partial data marker)
- `AdjustWinLoss()` in database/state.go (pipelined win/loss count correction)
- `confirmFillSafe()` entry-only wrapper with error awareness
- `reconcilePartialPosition()` + `markPartialClosed()` in consolidator
- `rebalanceDeficit` package-level type in allocator
- `errPartialEntry` sentinel error for partial entry success
- Aggregator test coverage for HasReconciled flag (3 new test cases)
- `partial_reconcile` field in frontend Position type

### Changed
- `executeRebalanceFundingPlan` accepts optional precomputedDeficits parameter
- `applyAllocatorOverrides` returns ([]Opportunity, bool) tuple
- `GetWithdrawFee` exchange interface returns (fee, minWd, error) — all adapters updated

### Removed
- `buildOppsFromAllocatorChoices` dead code function

## [0.29.3] - 2026-04-07

### Fixed
- **Allocator re-validation TransferablePerExchange bug** — revalCache now includes TransferablePerExchange so transfer-dependent candidates are no longer rejected by re-validation before reaching simulateTransferPlan
- **Sizing/margin check inconsistency** — calculateSizeWithPrice and CalculateSize now divide maxFromBalance by MarginSafetyMultiplier, preventing sizing from creating positions that the subsequent margin buffer check always rejects
- **DryRun spot balance for split accounts** — approveInternal dryRun unconditionally adds spot balance for non-unified exchanges, matching rebalanceAvailable semantics; unified accounts (gateio) correctly skip spot addition

### Added
- **Comprehensive debug logging for :35 rebalance path** — 57+ Debug logs across approveInternal (19 rejection points), allocator (appendChoice success/reject, deficit breakdown, capacity map, totalDonorSurplus, greedy/B&B selection, budget exceeded, cheapestTransferFee, simulateTransferPlan), and sizing strategy
- **Allocator solver comparison log** — greedy vs B&B result comparison (improved=true/false)
- **Re-validation reserved accumulation log** — shows reserved map when re-validation rejects candidates

### Removed
- **50% deviation guard** — removed spread deviation filter from allocator alternatives that blocked valid exchange pairs when primary pair had unusually high spread

