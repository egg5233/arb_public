# Changelog

All notable changes to this project will be documented in this file.

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

