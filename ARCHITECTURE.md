# Funding Rate Arbitrage Bot — Architecture

## 1. Strategy Overview

Delta-neutral funding rate arbitrage across 6 CEXes: **Binance, Bybit, Gate.io, Bitget, OKX, BingX**.

**Core idea**: Long on the exchange paying you funding, Short on the exchange charging least (or paying you). Collect net funding spread every period.

### 1.1 Multi-Interval Funding Awareness

Funding intervals are **per-symbol per-exchange**, NOT static:
- Binance/Bybit/OKX: default 4h (some pairs 8h or 1h)
- Gate.io/Bitget: default 8h (some pairs vary)
- The bot fetches and tracks actual intervals via Loris API `FundingIntervals` map
- All rates standardized to **bps per hour** as the universal internal unit
- Loris rates are normalized to 8h equivalent (`bps_per_period × 8/interval`); ranker divides by 8 to get bps/h. Exchange API rates convert via `rate * 10000 / interval_hours`
- The scheduler fires **every hour** (0–23 UTC) to support 1h-interval tokens
- A per-opportunity timing guard skips tokens whose next funding is not imminent

### 1.2 Dual-Source Discovery & Scoring

**Source 1 — Loris API** (primary, every 60s)
- `GET https://api.loris.tools/funding` — rates + per-symbol intervals
- Rates normalized to 8h equivalent; ranker divides by 8 to get bps/h

**Source 2 — CoinGlass** (secondary, from Redis key `coinGlassArb`)
- Pre-scraped arbitrage data with OI metrics
- Staleness cutoff: skip if >60min old
- Spread derived from `annualYield / 8760` to get bps/h (FundingRate interval varies)
- OI-based rank approximation: >=10M→10, >=1M→50, >=100K→150, else→500

**Source 3 — CoinGlass Spot-Futures Arb** (from Redis key `coinGlassSpotArb`)
- Scraped by embedded `internal/scraper` goroutine using headless Chrome (chromedp)
- Targets CoinGlass ArbitrageList page for spot-sell + futures-long opportunities
- Runs on configurable schedule (default: minutes 15, 35 each hour) + once at startup
- Writes JSON payload (timestamp, count, opportunities) to Redis key `coinGlassSpotArb`
- Config: `config.json` → `spot_arb.enabled`, `spot_arb.schedule`, `spot_arb.chrome_path`; env overrides `SPOT_ARB_ENABLED`, `SPOT_ARB_SCHEDULE`, `SPOT_ARB_CHROME_PATH`
- Chrome binary auto-detected from Puppeteer/Playwright caches and system paths
- Standalone CLI: `cmd/spotarb/main.go` (imports shared package, supports `--cron`, `--no-redis`, `--json`)

**Merge & Rank**:
1. Poll both sources, convert to `[]Opportunity`
2. Deduplicate by (symbol, longExchange, shortExchange), keep highest score
3. Sort by composite score: `score = spread * (1.0 + 1.0/oiRank)`
4. Take top N (default 5)
5. Verify against exchange-native APIs. Verifier performs 4 checks per opportunity (all skipped for CoinGlass sources):
   - **Direction check**: Reject if exchange rate sign contradicts Loris direction
   - **Interval mismatch**: Reject if Loris interval vs exchange interval differ by >0.5h (catches stale interval data inflating normalized rates)
   - **Rate magnitude**: Reject if per-leg Loris rate vs exchange rate differ by >50%
   - **Spread magnitude**: Reject if net spread (Loris) vs net spread (exchange) differ by >50%
   - **Funding rate cap**: Reject rates exceeding exchange-reported MaxRate/MinRate caps (±150 bps/h fallback)
   Verifier also populates `NextFunding` per opportunity.
6. **Persistence filter** (on entry scan only): Require opportunities to appear consistently across multiple recent scans before qualifying for execution. Key = `symbol|longExchange|shortExchange` (direction-sensitive). Thresholds vary by funding interval:
   - 1h interval: 2 appearances in 15min lookback
   - 4h interval: 4 appearances in 30min lookback
   - 8h interval: 5 appearances in 40min lookback
   - **Spread stability**: For low-liquidity pairs (OIRank >= threshold), also checks that `minSpread / maxSpread >= stabilityRatio` across scans in the lookback window. Prevents volatile-magnitude spreads that persist in direction but collapse after entry. Configurable per interval tier (1h default: ratio=0.5, OIRank>=100; 4h/8h disabled).
7. **Spread volatility filter** (on entry scan only): Computes coefficient of variation (population stddev / mean) of spread values across scans within the tier-appropriate lookback. Rejects if CV > `SpreadVolatilityMaxCV` (default 0.5). Requires `SpreadVolatilityMinSamples` (default 3) data points. Catches erratic rate spikes.
8. **Symbol cooldown filter** (on entry scan only): Rejects if the symbol is in any cooldown. Two types: **Loss cooldown** (`LossCooldownHours`, default 4.0) — set after closing at a loss. **Re-enter cooldown** (`ReEnterCooldownHours`, default 0, disabled) — set after any close. Both persisted to Redis with TTL (`arb:lossCooldown:{symbol}`, `arb:reEnterCooldown:{symbol}`).
9. **Funding window filter** (on entry scan only): Drop opportunities with `NextFunding > FundingWindowMin` (default 30min) away
10. **Historical backtest filter** (on entry scan only): Fetches X days (default 3, `BacktestDays`) of historical funding settlement data from Loris API. Validates each leg independently against its own settlement schedule, sums cash flows, and rejects if `netProfit = shortSumY - longSumY <= BacktestMinProfit`. Results cached in Redis with 6h TTL.
11. **Delist filter** (all scan types, configurable via `strategy.discovery.delist_filter`, default `true`): Two complementary signal paths write to the same `arb:delist:{SYMBOL}` Redis key:
    - **Article scraper** (`internal/discovery/delist.go`): polls Binance announcement API every 6h, parses coin names from delist announcement titles, stores blacklist with TTL until delist date + 7 days.
    - **deliveryDate poller** (`internal/discovery/contract_refresh.go`, v0.31.0): `StartContractRefresh` runs a 1h-cadence background goroutine (interval configurable via `contract_refresh_min`, default 60, 0 disables) that calls `LoadAllContracts` for every configured exchange and writes any near-future `DeliveryDate`-flagged perpetual to the same Redis key. Currently populates `DeliveryDate` for Binance (`deliveryDate` field) and Bybit (`deliveryTime` field on LinearPerpetual contracts). Title-format-independent — catches batch delists using generic announcement titles.
    - Both writes are idempotent; the key format is shared. Scanner rejects blacklisted symbols. Engine auto-exits active positions regardless of which leg's exchange is involved (generalized from Binance-only in v0.31.0) via emergency market close + dashboard alert. When disabled: neither goroutine starts, scanner won't filter, engine won't auto-close.

**Profitability Filter**:
```
cost_ratio = total_fees_bps / (spread_bps_h * value_of_time_hours)

Where:
  total_fees_bps = (maker_A + taker_A + maker_B + taker_B) * 100
  spread_bps_h   = short_rate - long_rate (bps per hour)

PASS if cost_ratio < VALUE_OF_RATIO (default: 0.50)
```

**Exchange Fee Schedule** (with 20% rebate):
| Exchange | Maker | Taker |
|----------|-------|-------|
| Binance  | 0.016% | 0.040% |
| Bybit    | 0.016% | 0.044% |
| OKX      | 0.016% | 0.040% |
| Bitget   | 0.016% | 0.048% |
| Gate.io  | 0.012% | 0.040% |
| BingX    | 0.016% | 0.040% |

### 1.3 Execution Timeline

**Scan-driven execution**: The scanner fires at configurable minutes (default :10, :20, :30, :35, :40, :45, :50) each hour. Each scan has a type: **NormalScan** (builds persistence history, feeds dashboard), **RebalanceScan** (default :10, production overrides to :20 — fund rebalancing), **ExitScan** (:30 — exit checks), **RotateScan** (:35 — rotation checks), or **EntryScan** (:40 — trade execution). Schedule is configurable via `scan_minutes`, `rebalance_scan_minute`, `entry_scan_minute`, `exit_scan_minute`, `rotate_scan_minute`.

All actions (rebalance, exit, entry, rotate) are triggered by scan types in the main engine loop — no separate scheduler needed.

```
Continuous  Scanner fires at configured minutes each hour
              1. Poll Loris + CoinGlass, merge & rank top N
              2. Verify rates via exchange APIs (populates NextFunding)
              3. Record in persistence history (every scan)
              4. On entry scan only:
                 a. Apply persistence filter (see §1.2)
                 b. Apply funding window filter (NextFunding <= 30min)
                 c. Send ScanResult{Type: EntryScan} to engine

:20 scan   RebalanceScan → rebalanceFunds()  (code default :10, production override :20)
             1. Analyze capital needs per exchange (capped to max_positions × capital_per_leg)
             2. Same-exchange spot→futures transfers (instant)
             3. Cross-exchange withdrawals for remaining deficits (APT/BEP20)
             4. Wait for deposits (up to 3min polling)

:40 scan   Engine receives EntryScan → executeArbitrage()
           Phase 1 — Sequential pre-filter:
             For each opp (best-scored first, up to available slots):
               - Acquire per-symbol Redis lock
               - Risk approval (11-point check, see §2.4)
               - Collect approved candidates

           Phase 2 — Parallel execution (sync.WaitGroup):
             Launch all approved candidates as goroutines:
               go executeTradeV2(opp1, size1, price1, gapBPS1)
               go executeTradeV2(opp2, size2, price2, gapBPS2)
             Wait for all goroutines to complete

           Each executeTradeV2() — depth-driven sequential IOC:
           1. Setup: set leverage + margin mode on both exchanges
           2. Subscribe to WS depth on both exchanges, wait up to 2s
           3. Fill loop (every 100ms, up to EntryTimeoutSec):
              a. Read depth from both exchanges
              b. Spread threshold: uses gapBPS from risk approval as ceiling
              c. Top-of-book spread check: (bestAsk/bestBid - 1)*10000 < gapBPS
              d. Aggregate depth across all levels within spread threshold
              e. Size = min(remaining, aggregatedBidQty, aggregatedAskQty), rounded to step
              f. Skip if notional < MinChunkUSDT
              g. Less-liquid leg first (higher size/qty ratio → riskier)
              h. Place IOC limit on first leg, confirmFill waits for terminal state (filled/cancelled)
              i. If 0 fill → free abort, try next tick
              j. Place IOC limit on second leg for confirmed qty
              k. Trim excess if second leg partially fills; only count matched portion
              l. Accumulate fills + VWAP entry prices
              m. Circuit breaker: abort after 5 consecutive PlaceOrder failures on either exchange
           4. Post-loop: trim to min(long, short), activate position
           5. Unsubscribe depth on both exchanges

T-1min     Position(s) active (or aborted cleanly)
T-0        Funding snapshot — collect spread
T+?        EXIT (per exit mode, see §1.5)
```

**Entry fee model**: Both legs pay taker fees (IOC at counterparty price crosses the spread). The profitability filter in discovery accounts for 4× taker fees round-trip.

### 1.4 Position Management

- **Max concurrent positions**: configurable (default 1). Duplicate symbol entries are blocked — only one active position per symbol regardless of exchange pair
- **Leverage**: configurable, hard-capped at 5x
- **Margin mode**: Cross, USDT-margined only
- **Exchange diversity**: Soft preference — warn if >60% of positions on same exchange
- **Capital exposure cap**: 60% of total capital per single exchange (hard limit); bypassed when the cross-strategy allocator is active
- **Cross-strategy capital allocator** (`internal/risk/allocator.go`): optional Redis-backed module that enforces shared USDT budgets across perp-perp and spot-futures. Configure via `enable_capital_allocator` (default `false`). Supports `max_total_exposure_usdt`, per-strategy caps (`max_perp_perp_pct`, `max_spot_futures_pct`), per-exchange cap (`max_per_exchange_pct`), and configurable reservation TTL. Reserve → Commit → Release lifecycle; reservations auto-expire if a trade never commits.
- **Unified Capital Allocation** (`internal/risk/allocator.go`, Phase 5): optional single-pool system that derives `CapitalPerLeg` from a shared `TotalCapitalUSDT` budget. Enable via `enable_unified_capital` (default `false`). Supports risk profile presets (`conservative`, `balanced`, `aggressive`), performance-weighted strategy splits (tilts toward higher-APR strategy using analytics data, CA-03), and dynamic capital shifting — idle strategy's unused allocation freed to active strategy each scan cycle (CA-04). When enabled, the Allocation tab in Config drives all sizing; Fund and Risk tabs show "Managed by Allocation" badges on affected fields. Manual `CapitalPerLeg > 0` always takes precedence.

**Position Sizing** (two modes):
1. **Fixed** (`CAPITAL_PER_LEG > 0`): `notional = capitalPerLeg * leverage`, clamped to available balance
2. **Auto** (`CAPITAL_PER_LEG = 0`): `notional = (min(balA, balB) * leverage) / (remainingSlots * 2)` — 50% safety buffer

After sizing: round to contract step size, enforce min size, enforce 10 USDT minimum margin per leg.

**Pre-Execution Fund Rebalancing**: On the rebalance scan (code default :10, production :20), `rebalanceFunds()` analyzes capital needs across all discovered opportunities and ensures each exchange has sufficient margin:
1. Same-exchange spot→futures transfers (instant, free)
2. Cross-exchange withdrawals via APT (preferred) or BEP20 for remaining deficits
3. Only withdraws from exchanges with margin ratio below L4 threshold (allows L3 donors)
4. Needs capped to `max_positions × capital_per_leg` to avoid over-allocation

**Stop-Loss Protection**: After position activation, protective stop-loss orders are placed on both legs. SL distance = `90% / leverage` (e.g. 3x→30%). Lifecycle:
- **Entry**: `attachStopLosses()` places SL on both exchanges, stores order IDs in position
- **Exit**: `cancelStopLosses()` cancels both SLs before closing position
- **Rotation**: `updateRotationStopLoss()` cancels old leg's SL, places new SL at new entry price
- SL failures are logged but never block entry/exit/rotation
- **Instant SL fill detection** (two methods): All exchange WS adapters parse `ReduceOnly` and `Symbol` from order fill events and send fills to a buffered channel via `SetOrderCallback`. Engine goroutine `consumeSLFills` checks each fill via two methods:
  1. **slIndex lookup** (original): In-memory reverse index (`exchange:orderID → posID+leg`). On match, triggers `triggerEmergencyClose`.
  2. **ReduceOnly detection** (fallback): When a reduce-only fill arrives for a symbol we hold, checks it's not a bot-initiated order (tracked in `ownOrders` sync.Map) and not during entry (`entryActive` guard), then verifies the leg is actually flat on the exchange via `getExchangePositionSize`. Only triggers emergency close if the leg is confirmed gone. Catches SL/TP/liquidation fills even when the exchange issues a new order ID on trigger (which the slIndex would miss).
  Both methods call `triggerEmergencyClose` which preempts any exit goroutine, sends dashboard alert, and fires emergency close of the remaining leg.

| Exchange | Place endpoint | Cancel endpoint |
|----------|---------------|-----------------|
| Binance | `POST /fapi/v1/algoOrder` (STOP_MARKET) | `DELETE /fapi/v1/algoOrder` |
| Bybit | `POST /v5/order/create` (StopOrder) | `POST /v5/order/cancel` (StopOrder) |
| Gate.io | `POST /futures/usdt/price_orders` | `DELETE /futures/usdt/price_orders/{id}` |
| Bitget | `POST /api/v2/mix/order/place-plan-order` | `POST /api/v2/mix/order/cancel-plan-order` |
| OKX | `POST /api/v5/trade/order-algo` | `POST /api/v5/trade/cancel-algos` |

**Position Consolidator**: Background goroutine (every 5min) reconciles local position records with actual exchange positions:
- Missing leg detected → closes both legs (including "missing" one via re-query + local size fallback) and marks position closed. BingX legs require 3 consecutive misses before acting (guards against transient empty-position API responses)
- **PnL on close**: `markPositionClosed` queries `GetClosePnL` from both exchanges to populate `LongExit`, `ShortExit`, `RealizedPnL` (price PnL + fees + funding), and calls `UpdateStats` so consolidator-closed positions count in win/loss tracking
- Size mismatch >1% → syncs local record to exchange reality
- **Balance enforcer**: After sync, if long/short differ by >5%, trims excess side via confirmed reduce-only market order, then cancels and re-places stop losses
- Orphan exchange positions (not tracked locally) → auto-closes via reduce-only market IOC
- **Race protection**: Skips positions with active exit/entry goroutines (`exitActive`, `entryActive`, `busySymbols`) to prevent conflicts with in-progress trades

### 1.5 Exit Strategy

**Scan-aligned evaluation**: Exit checks run on ExitScan (:30) and EntryScan (:40) only. No timer-based polling.

**Decision flow per position**:

```
1. Safety: checkSpreadReversal() → exit if entry spread was positive but current is negative
   (with optional tolerance: allow N reversals before triggering, configurable via
   `SpreadReversalTolerance`, default 0 = immediate exit on first reversal)
2. Interval monitor: checkIntervalChanges() → queries live rates+intervals via GetFundingRate().
   Spread-aware: if intervals diverge but live spread is still positive (collecting side benefits
   from shorter interval), keeps position open and updates NextFunding. Only exits if spread
   goes negative. Skips within ±10min of funding settlement (same guard as spread reversal).
   Controlled by `AllowMixedIntervals` config (default false) at ranking time.
3. Skip evaluation within ±10min of funding settlement (rates unreliable)
4. Min-hold gate: suppress exit before first funding settlement (`NextFunding`); manual close and margin emergencies bypass
```

**Exit execution — Depth-Fill (non-emergency)**:
1. Set position status to `exiting` (slot remains occupied)
2. Spawn cancellable goroutine (`context.Context`)
3. Subscribe to WS depth on both exchanges
4. **Gap-gated** depth-fill loop (200ms tick, `ExitDepthTimeoutSec` timeout). Cross-exchange gap checked before each order pair; gap ceiling ramps from `ExitMaxGapBPS` (default 10 bps) to 3× over timeout:
   - Sell on long exchange: reduce-only IOC limit at `bid * (1 - slippage)`
   - Buy on short exchange: reduce-only IOC limit at `ask * (1 + slippage)` for matched qty
   - VWAP accumulation across fills
   - Circuit breaker: 5 consecutive failures on either exchange
5. After timeout → market IOC for remainder (must close fully)
6. Compute realized PnL, save to history, broadcast
7. **Async reconciliation** (3 retries at 5s/15s/30s): Query exchange-native close PnL APIs (`GetClosePnL`) on both current legs. Uses position-level endpoints (Bitget `history-position`, OKX `positions-history`, Gate.io `position_close`, BingX `positionHistory`, Bybit `closed-pnl` + funding, Binance income API aggregation). Aggregates all records per side, includes `RotationPnL` from rotated legs. Corrects if diff > $0.01. Also updates `FundingCollected`.

**Exit execution — Emergency (L4 reduce / L5 close)**:
Reduce-only IOC market orders on both legs, 60s fill timeout. L4/L5 handlers cancel any running exit goroutine first (500ms grace period), then proceed.

Realized PnL = longLegNetPnL + shortLegNetPnL + RotationPnL (exchange-reported, includes fees + funding; reconciled post-close via `GetClosePnL`).

| Caller | Mode | Reason |
|--------|------|--------|
| `checkExitsV2()` | Depth-fill + market fallback | Scan-aligned, cost-evaluated |
| `handleEmergencyClose` (L5) | Market | Critical, preempts exit goroutine |
| `reducePosition` full-close | Market | Already reduced via market |
| Consolidator | Market close | Missing leg detected |

### 1.6 Funding Collection Tracking

`trackFunding()` runs once on startup, then every hour at HH:10:00 UTC. Updates `FundingCollected` for all active positions using actual exchange data:

- **Fast path** (Bitget, OKX, Gate.io): Reads accumulated funding fee directly from position endpoint fields (`totalFee`, `fundingFee`, `pnl_fund`).
- **Fallback** (Binance, Bybit, or if position field empty): Queries dedicated funding history endpoints (`/fapi/v1/income`, `/v5/account/transaction-log`).
- All adapters implement `GetFundingFees(symbol, since)` returning `[]FundingPayment` with signed amounts.
- `reconcilePnL()` (post-close) uses exchange-native `GetClosePnL` APIs for authoritative PnL figures (replaces cash-flow reconstruction).
- Recomputes `NextFunding` from exchange APIs when previous snapshot passes.
- Broadcasts updates to dashboard via WebSocket.

### 1.7 Leg Rotation

When a better exchange pair exists for the same symbol with one shared leg, the engine
rotates only the inferior leg instead of closing and reopening the full position.

Rotation check flow:

1. Compare each active position's **live funding spread** against the latest discovery opportunity for the same symbol
2. If one leg matches and spread improvement > `RotationThresholdBPS` (default 20 bps/h):
   - **Open new leg first** (IOC limit) — if fails, abort cleanly, original position untouched
   - **Close old leg second** (IOC limit, reduce-only) — only close the amount that was opened
   - If close IOC doesn't fill, retry with market (must close fully)
   - Update position record atomically (exchange, entry price, spread)
3. Anti-churn protections:
   - **Cooldown**: No rotation within `RotationCooldownMin` (default 30min) of last rotation
   - **Hysteresis**: 2× threshold (40 bps/h) to rotate back to a previously-abandoned exchange
   - **One-leg-only**: Both legs different → skip (would need full close+reopen)
   - **Settlement guard**: Skip rotation within 10min before/after funding settlement
   - **Settlement frequency penalty**: When moving to a higher-frequency exchange (e.g. 8h→1h), the extra settlements incur costs if the leg is on the paying side. Penalty = `extraSettlements × payingRate` amortized as bps/h, subtracted from spread improvement. Only applies when the rotating leg pays funding on the new exchange; collecting legs get no penalty.
   - **Min notional**: Skip if position notional < $10
   - **Min fill**: Skip if open fill < 50% of target size
4. After rotation completes, `NextFunding` is updated from the new exchange to prevent stale timestamps.
5. **Rotation PnL reconciliation**: After close, `reconcileRotationPnL()` asynchronously queries old exchange's `GetClosePnL()` (3 retries, narrow ±5min time window around rotation) and stores the authoritative PnL in `RotationPnL`. If position is already closed when the result arrives, recomputes `RealizedPnL`, updates stats and history.

| Caller | Trigger | Action |
|--------|---------|--------|
| `checkRotations()` | spread improvement > threshold | `rotateLeg()` — sequential open-first, then close |

### 1.8 Configurable Parameters

Parameters marked with (env) have env var overrides; others are JSON/dashboard only.

| Parameter | Default | Description |
|---|---|---|
| `VALUE_OF_TIME_HOURS` (env) | 16h | Minimum hold time (MinHoldTime) |
| `VALUE_OF_RATIO` (env) | 0.50 | Max fee/revenue ratio (MaxCostRatio) |
| `MAX_POSITIONS` (env) | 1 | Max concurrent arb positions |
| `LEVERAGE` (env) | 3 | Default leverage (hard cap: 5) |
| `SLIPPAGE_LIMIT_BPS` (env) | 5 | Max acceptable orderbook slippage |
| `TOP_OPPORTUNITIES` (env) | 5 | Max opportunities from discovery |
| `CAPITAL_PER_LEG` (env) | 0 | Fixed USDT per leg; 0 = auto from balance |
| `PRICE_GAP_FREE_BPS` (env) | 40 | Below this gap, skip price gap checks |
| `MAX_PRICE_GAP_BPS` (env) | 250 | Hard reject above this gap (absolute, both directions) |
| `MAX_GAP_RECOVERY_INTERVALS` (env) | 1.0 | Max funding intervals to recover gap (e.g. 1.0 = one funding period) |
| `DRY_RUN` (env) | false | Skip actual trade execution |
| `ENTRY_TIMEOUT_SEC` (env) | 60 | Max seconds for depth-driven fill loop |
| `MIN_CHUNK_USDT` (env) | 5 | Min notional per micro-order (avoids dust) |
| `ExitDepthTimeoutSec` | 45 | Depth-fill exit loop timeout before market fallback |
| `RiskMonitorIntervalSec` | 300 | Interval between risk monitor checks (seconds) |
| `MarginL3Threshold` | 0.50 | Trigger fund transfer (margin ratio) |
| `MarginL4Threshold` | 0.80 | Trigger position reduction |
| `MarginL5Threshold` | 0.95 | Trigger emergency close |
| `L4ReduceFraction` | 0.50 | Fraction to reduce at L4 |
| `RotationThresholdBPS` | 20 | Min spread improvement (bps/h) to trigger leg rotation |
| `RotationCooldownMin` | 30 | Cooldown after rotation before next allowed |
| `PersistLookback1h` | 15min | Persistence lookback window for 1h-interval pairs |
| `PersistMinCount1h` | 2 | Min appearances in lookback for 1h pairs |
| `PersistLookback4h` | 30min | Persistence lookback window for 4h-interval pairs |
| `PersistMinCount4h` | 4 | Min appearances in lookback for 4h pairs |
| `PersistLookback8h` | 40min | Persistence lookback window for 8h-interval pairs |
| `PersistMinCount8h` | 5 | Min appearances in lookback for 8h pairs |
| `SpreadStabilityRatio1h` | 0.5 | Min min/max spread ratio for 1h pairs (0=disabled) |
| `SpreadStabilityOIRank1h` | 100 | Only check spread stability when OIRank >= this (1h) |
| `SpreadStabilityRatio4h` | 0 | Min min/max spread ratio for 4h pairs (disabled) |
| `SpreadStabilityOIRank4h` | 0 | OI rank threshold for 4h stability check (disabled) |
| `SpreadStabilityRatio8h` | 0 | Min min/max spread ratio for 8h pairs (disabled) |
| `SpreadStabilityOIRank8h` | 0 | OI rank threshold for 8h stability check (disabled) |
| `SpreadVolatilityMaxCV` | 0.5 | Max spread coefficient of variation across scans (0=disabled) |
| `SpreadVolatilityMinSamples` | 3 | Min scan records needed to evaluate CV |
| `FundingWindowMin` | 30 | Max minutes before funding to allow entry |
| `LossCooldownHours` | 4.0 | Hours to blacklist symbol after loss close (Redis TTL) |
| `ReEnterCooldownHours` | 0 | Hours to block re-entry on same symbol after any close (0=disabled, Redis TTL) |
| `SpreadReversalTolerance` | 0 | Allow N spread reversals before triggering exit (0=immediate) |
| `ScanMinutes` | [10,20,30,35,40,45,50] | Minutes within each hour when scans fire (auto-includes special minutes) |
| `REBALANCE_SCAN_MINUTE` (env) | 10 | Minute mark that triggers fund rebalancing (production config overrides to 20) |
| `EntryScanMinute` | 40 | Minute mark that triggers trade execution |
| `ExitScanMinute` | 30 | Minute mark that triggers exit checks |
| `RotateScanMinute` | 35 | Minute mark that triggers rotation checks |

Config loading priority: defaults → `config.json` → environment variables → `EnsureScanMinutes()` auto-adds special minutes.

---

## 2. Architecture

### 2.1 Module Overview

```
cmd/
  main.go                          Entry point, wires all components, exchange factory
  livetest/main.go                 Live exchange API test suite (22 tests per exchange)
  simtrade/main.go                 Trade simulation CLI (dry run depth analysis + live execution)
  balance/main.go                  Balance check CLI
  transfer/main.go                 Fund transfer CLI
  transfertest/main.go             Transfer test CLI
  fundmove/main.go                 Spot ↔ Futures transfer CLI
  wstest/                          WebSocket test CLI
  spotarb/main.go                  Standalone spot-futures arb scraper CLI (--cron, --json, --no-redis)
internal/
  scraper/
    spotarb.go                     CoinGlass spot-futures arb scraper (headless Chrome via chromedp)
  config/config.go                 JSON + env config
  engine/
    engine.go                      Main loop, parallel trade execution, funding tracker, rebalancing
    consolidate.go                 Position consolidator: syncs exchange state with local records (5min), race-safe
    exit.go                        Scan-aligned exit (VWAP cost eval, depth-fill loop), L4 reduction, leg rotation
    engine_test.go                 Tests: effectiveAdvanceMin, classifyRotation
  discovery/
    scanner.go                     Fixed-schedule polling (Loris + CoinGlass), merge, persistence filter & broadcast
    verifier.go                    Exchange-native rate verification (direction, interval, magnitude, spread, caps)
    ranker.go                      Profitability filter, composite scoring, fee schedules
    ranker_test.go                 Ranking tests
    contract_refresh.go            1h-cadence deliveryDate poller: writes near-expiry perpetuals to arb:delist:{SYMBOL}
  database/
    redis.go                       Redis client wrapper (DB 2)
    state.go                       Positions, history, funding snapshots, stats, transfers
    locks.go                       Distributed locking (SET NX + TTL)
  risk/
    manager.go                     11-point pre-trade risk approval, position sizing (includes spread stability gate)
    monitor.go                     5-dimension active position monitoring (30s)
    health.go                      5-tier margin health system (L0–L5), protective actions
    limits.go                      Hard constraints: 5x leverage, 60% exposure, 10 USDT floor
    health_test.go                 Health level tests
    spread_stability.go            Redis-backed spread CV gate (SpreadStabilityChecker) called at entry time
    spread_stability_test.go       Spread stability gate tests
    loss_limit.go                  Rolling-window loss limit checker (24h/7d) via Redis sorted sets
  models/
    interfaces.go                  Discoverer, RiskChecker, StateStore abstractions
    opportunity.go                 Opportunity struct (with IntervalHours, Source)
    position.go                    ArbitragePosition, status constants
    funding.go                     FundingRate, FundingSnapshot, LorisResponse, CoinGlassResponse
  api/
    server.go                      HTTP + WebSocket server
    handlers.go                    REST endpoints (positions, history, config, exchanges, transfers, check-update)
    handlers_test.go               Tests: check-update semver comparison, versionNewer helper
    config_handlers_test.go        Config endpoint tests
    drift_monitor.go               Binary drift monitor: detects stale process after deploy, triggers systemd restart
    ws.go                          WebSocket hub & client management
    auth.go                        Session-based auth (24h TTL tokens)
    frontend.go                    React SPA embedding & routing
    spot_handlers.go               Spot-futures REST + WebSocket endpoints
    spot_stats_test.go             Spot stats cold-start / partial-hash regression tests
web/
  embed.go                         React dist embedding via go:embed
pkg/
  exchange/                        Reusable exchange adapter package (importable by external projects)
    exchange.go                    Unified Exchange interface (35 methods)
    types.go                       Shared types (Order, Position, Balance, Orderbook, etc.)
    factory.go                     Exchange factory (logic in cmd/main.go to avoid cycles)
    binance/                       adapter.go, client.go, ws.go, ws_private.go, margin.go
    bitget/                        adapter.go, client.go, ws.go, ws_private.go, margin.go
    bybit/                         adapter.go, client.go, ws.go, ws_private.go, margin.go
    gateio/                        adapter.go, client.go, ws.go, ws_private.go, adapter_test.go, margin.go
    okx/                           adapter.go, client.go, ws.go, ws_private.go, margin.go
    bingx/                         adapter.go, client.go, ws.go, ws_private.go
  utils/
    math.go                        RoundToStep, EstimateSlippage, RateToBpsPerHour, GenerateID
    logging.go                     Structured logging with module context + file output + daily rotation
```

**Total**: 107 Go files + 21 TS/TSX frontend files

### 2.2 Data Flow

```
        ┌─────────────┐     ┌──────────────┐
        │  Loris API  │     │  CoinGlass   │
        │  (HTTP 60s) │     │  (Redis key) │
        └──────┬──────┘     └──────┬───────┘
               │                   │
               └───────┬───────────┘
                       │ merge & deduplicate
                ┌──────▼──────┐
                │  Discovery  │──── verify ──── Exchange APIs
                │  (Scanner)  │                 (direction-only)
                └──────┬──────┘
                       │ ranked opportunities (top N)
              ┌────────┼────────┐
              │                 │
       ┌──────▼──────┐  ┌──────▼──────┐
       │  Dashboard  │  │   Engine    │ Dispatches by scan type:
       │ (broadcast) │  │  (main loop)│ :10/:20 rebalance, :30 exit, :35 rotate, :40 entry
       └─────────────┘  └──────┬──────┘
                               │
                        ┌──────▼──────┐  RebalanceScan: rebalanceFunds()
                        │  Rebalance  │  spot→futures + cross-exchange withdrawals
                        └──────┬──────┘
                               │
                        ┌──────▼──────┐  EntryScan: executeArbitrage()
                        │    Risk     │◄──── Redis (capital, positions)
                        │  (11-point) │  Sequential pre-filter
                        └──────┬──────┘
                               │ approved candidates
                        ┌──────▼──────┐
                        │   Engine    │  Parallel execution (WaitGroup)
                        │  (depth-v2) │  Depth-driven sequential IOC per trade
                        └──────┬──────┘
                              ╱ ╲
                       ┌─────╱   ╲─────┐
                       │  Long    Short │  Per goroutine:
                       │  Exch    Exch  │  1. Subscribe depth
                       └────┬─────┬────┘  2. Aggregate & fill loop
                            │     │       3. Confirm + activate
                     ┌──────▼─────▼──────┐
                     │      Redis        │  State, history, locks, stats
                     │     (DB 2)        │
                     └──────────┬────────┘
                                │
            ┌───────────────────┼───────────────────┐
            │                   │                   │
     ┌──────▼──────┐    ┌──────▼──────┐    ┌───────▼───────┐
     │ Risk Monitor│    │   Health    │    │  Exit Manager │
     │  (5 checks) │    │  Monitor   │    │  (4 modes +   │
     │  30s loop   │    │  (L0–L5)   │    │   rotation)   │
     └──────┬──────┘    └──────┬──────┘    └───────┬───────┘
            │                  │                   │
     ┌──────▼──────┐    ┌──────▼──────┐    ┌──────▼───────┐
     │   Alerts    │    │  Actions    │    │ Consolidator │
     │  (WS push)  │    │ (transfer,  │    │ (5min sync)  │
     └─────────────┘    │  reduce,    │    └──────────────┘
                        │  close)     │
                        └─────────────┘
```

### 2.3 Exchange Interface

35 methods across 15 categories:

```go
type Exchange interface {
    // Identity
    Name() string

    // Orders
    PlaceOrder(req PlaceOrderParams) (orderID string, err error)
    CancelOrder(symbol, orderID string) error
    GetPendingOrders(symbol string) ([]Order, error)
    GetOrderFilledQty(orderID, symbol string) (float64, error)

    // Positions
    GetPosition(symbol string) ([]Position, error)
    GetAllPositions() ([]Position, error)

    // Account Config
    SetLeverage(symbol string, leverage string, holdSide string) error
    SetMarginMode(symbol string, mode string) error

    // Contract Info
    LoadAllContracts() (map[string]ContractInfo, error)

    // Funding Rate
    GetFundingRate(symbol string) (*FundingRate, error)
    GetFundingInterval(symbol string) (time.Duration, error)

    // Account & Transfers
    GetFuturesBalance() (*Balance, error)
    GetSpotBalance() (*Balance, error)
    Withdraw(params WithdrawParams) (*WithdrawResult, error)
    TransferToSpot(coin string, amount string) error
    TransferToFutures(coin string, amount string) error

    // Orderbook
    GetOrderbook(symbol string, depth int) (*Orderbook, error)

    // WebSocket: Prices (BBO)
    StartPriceStream(symbols []string)
    SubscribeSymbol(symbol string) bool
    GetBBO(symbol string) (BBO, bool)
    GetPriceStore() *sync.Map

    // WebSocket: Depth (orderbook, on-demand)
    // Levels per exchange: Binance 20, Bitget 15, Bybit 50, Gate.io 20, OKX 5
    SubscribeDepth(symbol string) bool
    UnsubscribeDepth(symbol string) bool
    GetDepth(symbol string) (*Orderbook, bool)

    // WebSocket: Private
    StartPrivateStream()
    GetOrderUpdate(orderID string) (OrderUpdate, bool)
    SetOrderCallback(fn func(OrderUpdate))

    // Stop-Loss (conditional orders)
    PlaceStopLoss(params StopLossParams) (orderID string, err error)
    CancelStopLoss(symbol, orderID string) error

    // Trade History
    GetUserTrades(symbol string, startTime time.Time, limit int) ([]Trade, error)
    GetFundingFees(symbol string, since time.Time) ([]FundingPayment, error)
    GetClosePnL(symbol string, since time.Time) ([]ClosePnL, error)

    // Account Setup
    EnsureOneWayMode() error

    // Lifecycle
    Close()
}
```

**Symbol format mapping**:
- Gate.io: `BTCUSDT` ↔ `BTC_USDT` (underscore)
- OKX: `BTCUSDT` ↔ `BTC-USDT-SWAP` (hyphenated with -SWAP)
- BingX: `BTCUSDT` ↔ `BTC-USDT` (hyphenated)
- Others: `BTCUSDT` as-is

Each adapter: `adapter.go` (interface impl), `client.go` (REST), `ws.go` (public WS), `ws_private.go` (private WS), `margin.go` (spot margin, optional).

#### SpotMarginExchange (optional interface)

Spot margin support for borrow-sell-buyback-repay strategies. Use type assertion (`exch.(exchange.SpotMarginExchange)`) to check support. BingX does not implement this interface.

```go
type SpotMarginExchange interface {
    MarginBorrow(params MarginBorrowParams) error
    MarginRepay(params MarginRepayParams) error
    PlaceSpotMarginOrder(params SpotMarginOrderParams) (orderID string, err error)
    GetMarginInterestRate(coin string) (*MarginInterestRate, error)
    GetMarginBalance(coin string) (*MarginBalance, error)
    TransferToMargin(coin string, amount string) error
    TransferFromMargin(coin string, amount string) error
}
```

Implemented by: Binance, Bybit, OKX, Gate.io (unified `/unified/*` cross margin endpoints), Bitget.

#### FlashRepayer (optional interface)

Optional interface for exchanges that support one-step collateral conversion and borrow repayment. Used by Direction A close to skip the spot buyback order entirely — the exchange converts USDT collateral and repays the borrow in one atomic step, avoiding price-ceiling and LOT_SIZE constraints.

```go
type FlashRepayer interface {
    FlashRepay(coin string) (repayID string, err error)
}
```

Implemented by: Bitget (via `/api/v2/margin/crossed/account/flash-repay`), Gate.io (via Flash Swap API `/flash_swap/orders` + `MarginRepay(repaid_all=true)`). Use type assertion `exch.(exchange.FlashRepayer)` to check support.

#### SpotMarginOrderQuerier (optional interface)

Optional interface for querying the status of a spot margin order after placement. Allows the engine to reconcile fill quantities and fee deductions (`FeeDeducted` field on `SpotMarginOrderStatus`).

```go
type SpotMarginOrderQuerier interface {
    GetSpotMarginOrder(orderID, symbol string) (*SpotMarginOrderStatus, error)
}
```

Implemented by: all five `SpotMarginExchange` adapters. Use type assertion `exch.(exchange.SpotMarginOrderQuerier)` to check support.

**Gate.io quanto multiplier**: Gate.io uses contract-based sizing. The adapter converts between base units and contracts using `quanto_multiplier` in PlaceOrder (divide), GetPosition/GetOrderFilledQty (multiply), and WS order updates (multiply). All other code works in base asset units.

**Gate.io unified account**: On startup, the adapter detects unified account mode via `GET /unified/unified_mode`. In unified mode, `GetFuturesBalance` reads from `/unified/accounts` instead of `/futures/usdt/accounts`, and `TransferToFutures` is a no-op (funds are shared). Falls back to classic mode if detection fails. Gate.io unified mode also exposes two additional methods on the adapter: `GetIsolatedMarginUSDT()` (queries `GET /margin/accounts`, sums available+locked USDT across all isolated pairs) and `SweepIsolatedMarginUSDT()` (transfers idle USDT with no borrows from isolated margin pairs back to spot via `POST /wallet/transfers`). The SpotEngine calls `SweepIsolatedMarginUSDT()` once per scan cycle to reclaim residual USDT left in isolated margin accounts after positions close.

### 2.4 Risk Management

#### Pre-Trade: 11-Point Risk Approval

1. Position count vs MaxPositions
2. Both exchanges exist in adapter map
3. Capital check + auto spot→futures transfer if needed
4. Leverage clamp to hard cap (5x)
5. Orderbook non-empty on long exchange
6. Position size calculation (CalculateSize)
7. Margin per leg >= 10 USDT floor
8. Slippage check on both exchanges (asks for long, bids for short) — if slippage exceeds the limit at full size, binary-searches for the largest tradeable size within slippage constraints (`findMaxSizeForSlippage`, ~10 iterations) instead of rejecting outright
9. Cross-exchange price gap check (absolute gap hard cap + directional recovery limit)
10. Per-exchange capital exposure cap (60% of total) — bypassed when cross-strategy allocator is active
11. Cross-strategy capital allocator reserve (if `enable_capital_allocator` is true)
12. Exchange concentration warning (>60% of positions)
13. Rolling loss limit gate: reject if 24h or 7d realized PnL exceeds configured thresholds (`EnableLossLimits`, `DailyLossLimitUSDT`, `WeeklyLossLimitUSDT`)

#### Active Monitoring: Risk Monitor (configurable interval, default 300s)

4 dimensions per active position:
1. **Funding rate**: Spread reversal (critical) or >50% narrowing (warning)
2. **Position balance**: >5% leg imbalance (warning)
3. **Unrealized PnL**: >5% loss (critical), >2% loss (warning)
4. **Liquidation distance**: <10% (critical), <15% (warning)

Alerts pushed to dashboard via WebSocket.

#### Margin Health: 5-Tier System (30s loop)

Per-exchange health levels based on margin ratio (0=safe, 1=liquidation):

| Level | Condition | Action |
|-------|-----------|--------|
| L0-None | No positions | — |
| L1-Safe | Positions, PnL >= 0 | — |
| L2-Low | PnL < 0, ratio < 0.50 | — |
| L3-Medium | PnL < 0, ratio >= 0.50 | Transfer funds from healthiest exchange |
| L4-High | PnL < 0, ratio >= 0.80 | Reduce positions by 50% (worst PnL first) |
| L5-Critical | ratio >= 0.95 | Emergency close all positions on exchange |

L3 transfer: targets ratio at 50% of L3 threshold, picks donor by L0 > L1 > highest balance.

### 2.5 Key Structs

```go
type Opportunity struct {
    Symbol        string
    LongExchange  string
    ShortExchange string
    LongRate      float64       // bps per hour
    ShortRate     float64       // bps per hour
    Spread        float64       // ShortRate - LongRate (bps/h)
    CostRatio     float64       // fees / (spread_bps_h * hold_hours)
    OIRank        int
    Score         float64       // spread * (1 + 1/oiRank)
    IntervalHours float64       // 1, 4, or 8
    NextFunding   time.Time    // earliest next funding snapshot across both legs
    Source        string        // "loris" or "coinglass"
    Timestamp     time.Time
}

type ArbitragePosition struct {
    ID               string
    Symbol           string
    LongExchange     string
    ShortExchange    string
    LongOrderID      string
    ShortOrderID     string
    LongSize         float64
    ShortSize        float64
    LongEntry        float64
    ShortEntry       float64
    Status           string     // pending → partial → active → exiting → closed
    EntrySpread      float64
    FundingCollected float64
    RealizedPnL      float64
    CreatedAt        time.Time
    UpdatedAt        time.Time
    NextFunding      time.Time
    LongSLOrderID    string     // stop-loss order ID on long exchange
    ShortSLOrderID   string     // stop-loss order ID on short exchange
    ExitMode         string     // wait, immediate, timed, spread_reversal
    LastRotatedFrom  string     // exchange we rotated away from
    LastRotatedAt    time.Time  // when last rotation happened
    RotationCount    int        // total rotations for this position
    ReversalCount    int        // spread reversal occurrences (for tolerance)
}
```

### 2.6 Redis Schema (DB 2)

```
arb:config                          HASH    Runtime config overrides
arb:positions                       HASH    positionID → JSON(ArbitragePosition)
arb:positions:active                SET     Active position IDs
arb:history                         LIST    Completed trades (FIFO, max 1000)
arb:funding:latest                  HASH    symbol → JSON(rates per exchange)
arb:funding:snapshots:{symbol}      LIST    Historical snapshots (max 100 per symbol)
arb:locks:{resource}                STRING  Distributed lock (SET NX + TTL)
arb:exchange:{name}:balance         STRING  Cached futures balance
arb:exchange:{name}:spotBalance     STRING  Cached spot balance (0 for unified Gate.io — avoids double-count)
arb:exchange:{name}:marginBalance   STRING  Cached margin USDT (cross-margin for Binance/Bitget; isolated margin for Gate.io unified)
arb:stats                           HASH    total_pnl, trade_count, win_count, loss_count
arb:transfers                       HASH    transferID → JSON(TransferRecord)
arb:transfers:history               LIST    Transfer IDs (FIFO, max 200)
arb:lossCooldown:{symbol}           STRING  Loss cooldown flag (auto-expires via TTL)
arb:reEnterCooldown:{symbol}        STRING  Re-entry cooldown flag (auto-expires via TTL)
arb:loss_events                     ZSET    Realized PnL events (score=timestamp, pruned >8d)
arb:spot_opportunities_cache        STRING  Last spot-futures scan results (JSON); written after each discovery cycle, no TTL; stale results served until next scan
```

### 2.7 Dashboard & API

**Backend**: Go HTTP server (same binary, separate goroutine)
**Frontend**: React (Vite + Tailwind) — embedded via `go:embed`
**Auth**: Session-based (24h TTL tokens). Read-only endpoints bypassed if `DASHBOARD_PASSWORD` empty; mutating endpoints (close, open, transfer) always require auth — returns 403 when no password configured

**REST Endpoints**:
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/login` | Authenticate, returns token |
| GET | `/api/positions` | Active positions |
| GET | `/api/history` | Closed positions (default 50) |
| GET | `/api/opportunities` | Current discovery results |
| GET | `/api/stats` | PnL, win rate |
| GET/POST | `/api/config` | View/update runtime config |
| GET | `/api/exchanges` | Exchange balances — returns `{name, balance, spot_balance, margin_balance, account_type}` per exchange; `margin_balance` is cross-margin USDT for Binance/Bitget, isolated margin USDT for Gate.io |
| POST | `/api/positions/close` | Manual close active position |
| POST | `/api/positions/open` | Manual open from opportunity |
| GET | `/api/positions/{id}/funding` | Funding history for a position |
| POST | `/api/transfer` | Execute cross-exchange withdrawal |
| GET | `/api/transfers` | Transfer history |
| GET | `/api/addresses` | Deposit addresses |
| GET | `/api/permissions` | API key permission check results |
| GET | `/api/logs` | Recent log lines |
| GET | `/api/rejections` | Recent trade rejection log |
| GET | `/api/diagnose` | System diagnostics snapshot |
| GET | `/api/check-update` | Compare local vs remote VERSION; returns semver `hasUpdate`, `binaryDrift` bool, and full `runtime` provenance (pid, exePath, binaryModTime, driftReason, restartSupported) |
| POST | `/api/update` | Git pull + make build + restart |

**WebSocket Messages** (`/ws`):
- `position_update` — single position change
- `positions` — full active position list
- `opportunities` — discovery results
- `stats` — PnL stats
- `alert` — risk alerts
- `binary_drift` — broadcast when drift monitor detects stale running binary
- `loss_limits` — rolling loss limit status (24h/7d used, thresholds, breached flags)

### 2.8 Startup Sequence (cmd/main.go)

1. Load config (JSON + env overrides + EnsureScanMinutes)
2. Connect to Redis (DB 2)
3. Initialize exchange adapters (require min 2)
4. Load contract specs for all exchanges
5. Start WebSocket streams (public + private)
6. Initialize: Scanner, RiskManager, RiskMonitor, HealthMonitor, API Server, Engine
7. Start balance refresh goroutine (60s) — fetches futures balance for all exchanges; spot balance only for non-unified exchanges (Gate.io unified skipped to avoid double-count); margin balance from `GetMarginBalance("USDT")` for Binance/Bitget, isolated margin from `GetIsolatedMarginUSDT()` for Gate.io unified
8. Start all components: Scanner → RiskMon → HealthMon → API (incl. binary drift monitor) → Engine (incl. Consolidator)
9. If `SpotFuturesEnabled`: create and start SpotEngine
10. Wait for SIGINT/SIGTERM → graceful shutdown (reverse order, SpotEngine stops first)
    - If drift monitor triggered the shutdown: exit non-zero so systemd `Restart=on-failure` fires

---

## 3. Spot-Futures Arbitrage Engine

A separate engine (`internal/spotengine/`) that runs **alongside** the perp-perp engine. It targets delta-neutral positions combining a spot margin leg and a futures hedge on the **same exchange**.

### 3.1 Two Directions

| Direction | Spot Leg | Futures Leg | Collects | Pays |
|-----------|----------|-------------|----------|------|
| **A** — Negative funding (`borrow_sell_long`) | Borrow coin → sell on spot | Long perpetual | Funding from long | Borrow interest |
| **B** — Positive funding (`buy_spot_short`) | Buy coin on spot (hold) | Short perpetual | Funding from short | Nothing (capital locked) |

### 3.2 Package Layout

| Package | Purpose |
|---------|---------|
| `internal/spotengine/engine.go` | `SpotEngine` struct, lifecycle (`Start`/`Stop`), discovery loop, Redis persistence counters, per-exchange capital limits, `sweepIsolatedMargin()` called each scan cycle to reclaim idle Gate.io isolated margin USDT |
| `internal/spotengine/discovery.go` | Reads CoinGlass spot arb data from Redis, queries live borrow rates, scores opportunities, preserves active positions in top-N |
| `internal/spotengine/execution.go` | Trade entry/exit with per-leg retry (`retryLeg`), idempotency guards (`FuturesExit > 0`), emergency escalation |
| `internal/spotengine/exit_manager.go` | Exit triggers (spread, yield, price spike, margin health for Dir A + Dir B), `initiateExit`, `completeExit` |
| `internal/spotengine/monitor.go` | Per-tick monitoring: stuck exit retry, `retryPendingRepay` for Bybit blackout, fresh borrow rate fetch |
| `internal/spotengine/risk_gate.go` | Pre-entry risk gate: capacity, cooldown, duplicate (per-symbol), persistence filter |
| `internal/models/spot_position.go` | `SpotFuturesPosition` data model — spot leg, futures leg, borrow tracking, P&L, `PendingRepay`, `ExitRetryCount` |
| `internal/database/spot_state.go` | Redis CRUD — positions, history, stats, cooldown, Redis-backed persistence counters with 20-min TTL, spot-market availability cache (`arb:spot_market_exists:*`, 24h TTL) |
| `internal/notify/telegram.go` | `TelegramNotifier` — auto-entry, auto-exit, emergency close, manual close alerts via Telegram Bot API |
| `internal/api/spot_handlers.go` | Dashboard REST endpoints + WebSocket broadcasts for spot positions |
| `doc/DESIGN_SPOT_FUTURES_RISK.md` | Extreme condition & risk design (10x squeeze, liquidation cascades) |

### 3.3 Discovery Flow

```
Primary path:
Loris funding API
  ↓
SpotEngine.discoveryLoop (every scan_interval_min minutes)
  1. Poll Loris funding payload, derive per-exchange Dir A + Dir B rows
  2. For each symbol+exchange:
     a. Normalize exchange name, check allowed list
     b. Verify exchange implements SpotMarginExchange interface
     c. Check spot-market availability via GetSpotBBO()
        - cache result in Redis key arb:spot_market_exists:{exchange}:{symbol}
        - TTL 24h so discovery survives restart without re-probing every symbol
        - rows with no spot market remain visible but carry `filter_status="spot market unavailable"`
     d. Direction A only: fetch borrow rate (5min cache), filter by max_borrow_apr
     e. Calculate fee APR: 4 taker legs × (365 / 30-day assumed hold)
     f. Net APR = fundingAPR − borrowAPR − feeAPR
     g. Filter by min_net_yield_apr
  3. Rank by net APR descending, limit to top N (always preserving entries for active positions)
  4. Update Redis persistence counters (per-symbol, 20-min TTL)
  5. Cache full results to Redis key `arb:spot_opportunities_cache` (no TTL) for instant dashboard display after restart
  6. Push results to API server (GET /api/spot/opportunities)

Scanner mode (config: `scanner_mode`):
  - `"native"`: Loris only
  - `"coinglass"`: CoinGlass only
  - `"both"`: merge both sources, each capped at 100 entries independently before merge to prevent one source from crowding out the other

Fallback path:
CoinGlass scraper → Redis (coinGlassSpotArb)
                        ↓
Used only when native Loris discovery is disabled or unavailable
  - Same spot-market availability filter applies before borrowability checks
  - Prevents impossible rows (for example missing OKX spot markets) from appearing actionable
```

### 3.4 Configuration

JSON `config.json` → `spot_futures` section:

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | false | Enable the spot-futures engine |
| `max_positions` | 1 | Maximum concurrent spot-futures positions |
| `capital_separate_usdt` | 200 | Capital per position for separate-wallet exchanges (Binance, Bitget) |
| `capital_unified_usdt` | 500 | Capital per position for unified-account exchanges (Bybit, OKX, Gate.io) |
| `leverage` | 3 | Futures leg leverage |
| `monitor_interval_sec` | 300 | Position monitoring interval |
| `min_net_yield_apr` | 0.10 | Minimum net APR after costs (10%) |
| `max_borrow_apr` | 0.50 | Maximum borrow APR for Direction A (50%) |
| `margin_exit_pct` | 85.0 | Margin utilization % that triggers normal exit |
| `margin_emergency_pct` | 95.0 | Margin utilization % that triggers emergency exit |
| `exchanges` | [] | Exchanges to consider (empty = all SpotMargin-capable) |
| `scan_interval_min` | 10 | Discovery scan interval in minutes |
| `persistence_scans` | 2 | Consecutive scans a symbol must appear before auto-entry |
| `auto_enabled` | false | Enable automated entry from discovery loop |
| `auto_dry_run` | true | Log auto-entry decisions without executing |

JSON `config.json` → `telegram` section:

| Field | Description |
|-------|-------------|
| `bot_token` | Telegram Bot API token |
| `chat_id` | Target chat/channel ID for alerts |

Env overrides: `SPOT_FUTURES_ENABLED`, `SPOT_FUTURES_MAX_POSITIONS`, `SPOT_FUTURES_LEVERAGE`, `SPOT_FUTURES_MONITOR_INTERVAL`, `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`.

### 3.5 Dashboard API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/spot/positions` | GET | Active spot-futures positions |
| `/api/spot/history` | GET | Closed positions (default limit 50) |
| `/api/spot/stats` | GET | Win/loss/PnL counters |
| `/api/spot/opportunities` | GET | Latest discovery scan results with `filter_status` reason per row |
| `/api/spot/open` | POST | Manual open a spot-futures position |
| `/api/spot/close` | POST | Manual close a spot-futures position |
| `/api/spot/positions/{id}/health` | GET | Live health snapshot: `last_borrow_rate_check`, `negative_yield_since`, current margin utilization |
| `/api/spot/config/auto` | GET/POST | View/update auto-entry config; response includes `max_positions`, `capital_separate_usdt`, and `capital_unified_usdt` guardrail fields |
| `/api/spot/test-inject` | POST | Inject synthetic Dir A + Dir B opportunities for lifecycle testing when no real opportunity is available |
| `/api/spot/test-lifecycle` | POST | Automated Dir A + Dir B lifecycle verification: opens two test positions, waits for fills, closes both, and returns a structured pass/fail report |
| `/api/spot/batch-check-gap` | POST | Check live price gaps for all provided opportunities at once; returns `{symbol, exchange, gap_bps, error}` per item; unlisted symbols return `error: true` |
| `/api/spot/batch-check-borrowable` | POST | Check max borrowable quantity for all provided Dir A items at once; Binance -3045 and OKX "coin not found" normalized to `max_borrowable=0` |

WebSocket events: `spot_position_update` (single position), `spot_positions` (full active list), `spot_opportunities` (discovery results broadcast), `spot_position_health` (real-time health tick).

### 3.6 Exit Flow

```
initiateExit(pos, reason, isEmergency)
  ├── markExiting(pos.ID)                 // prevent concurrent exits
  ├── lockedUpdatePosition → status=exiting, ExitRetryCount=0 (fresh) or keep (retry)
  ├── ClosePosition
  │     ├── closeDirectionA:
  │     │     step 1: retryLeg("futures-close", 3, 2s) → confirmFuturesFill
  │     │     step 2: retryLeg("spot-buyback", 3, 2s) → confirmSpotFill(expectedQty)
  │     │     step 3: MarginRepay → on failure: pos.PendingRepay=true
  │     │     [retry exhausted] → emergencyClose (concurrent legs)
  │     └── closeDirectionB:
  │           step 1: retryLeg("futures-close", 3, 2s)
  │           step 2: retryLeg("spot-sell", 3, 2s)
  │           [retry exhausted] → emergencyClose (concurrent legs)
  ├── if pos.PendingRepay: persist flag, return (monitor retries repay)
  └── completeExit → record PnL, close Redis position, Telegram alert

monitorTick (every monitor_interval_sec):
  ├── PendingRepay=true  → retryPendingRepay (GetMarginBalance → buy deficit → MarginRepay)
  ├── status=exiting (>2min, no goroutine) → increment ExitRetryCount, re-trigger initiateExit
  │     ExitRetryCount+1 >= 5 → isEmergency=true
  └── status=active → checkExitTriggers (spread, yield, price spike, margin health)
```

**Price-spike triggers (canonical rule):**
- **Direction A** (`borrow_sell_long`): adverse move is price **UP** — long futures profits but borrowed-and-sold spot must be bought back at a higher price; also increases margin utilization on the borrow.
- **Direction B** (`buy_spot_short`): adverse move is price **UP** — short futures faces liquidation risk on a squeeze. The spot holding (long) is safe but cannot offset fast enough if the short is liquidated. Down moves are profitable for the futures short and do **not** trigger price-spike exits.

Both directions use the same check: `movePct = (currentPrice − futuresEntry) / futuresEntry × 100`; exit fires when `movePct > price_exit_pct` (default 20%), emergency when `movePct > price_emergency_pct` (default 30%).

**Margin health triggers:**
- **Direction A**: `borrowed/available * 100 > margin_exit_pct` (spot margin utilization)
- **Direction B**: `GetFuturesBalance().MarginRatio * 100 > margin_exit_pct` (futures account margin ratio)

### 3.7 Integration with Perp-Perp Engine

The spot-futures engine is **fully independent** — separate goroutines, separate Redis keys, separate API routes. It shares:
- Exchange adapter instances (via `exchange.SpotMarginExchange` interface)
- Redis client connection
- API server (for route registration and WebSocket hub)
- Config loader
- TelegramNotifier instance (shared; perp-perp engine uses it for SL/emergency/API-error/loss-limit alerts with per-event-type cooldown)

No coordination is needed between the two engines; they can run concurrently without interference.

---
