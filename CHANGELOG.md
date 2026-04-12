# Changelog

All notable changes to this project will be documented in this file.

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

