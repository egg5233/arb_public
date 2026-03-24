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
- Loris rates are already bps/h (used directly); exchange API rates convert via `rate * 10000 / interval_hours`
- The scheduler fires **every hour** (0–23 UTC) to support 1h-interval tokens
- A per-opportunity timing guard skips tokens whose next funding is not imminent

### 1.2 Dual-Source Discovery & Scoring

**Source 1 — Loris API** (primary, every 60s)
- `GET https://api.loris.tools/funding` — rates + per-symbol intervals
- Rates already in bps/hour (used directly, no re-normalization)

**Source 2 — CoinGlass** (secondary, from Redis key `coinGlassArb`)
- Pre-scraped arbitrage data with OI metrics
- Staleness cutoff: skip if >60min old
- Spread derived from `annualYield / 8760` to get bps/h (FundingRate interval varies)
- OI-based rank approximation: >=10M→10, >=1M→50, >=100K→150, else→500

**Merge & Rank**:
1. Poll both sources, convert to `[]Opportunity`
2. Deduplicate by (symbol, longExchange, shortExchange), keep highest score
3. Sort by composite score: `score = spread * (1.0 + 1.0/oiRank)`
4. Take top N (default 5)
5. Verify against exchange-native APIs (direction-only, no magnitude tolerance). Verifier also populates `NextFunding` per opportunity.
6. **Persistence filter** (on entry scan only): Require opportunities to appear consistently across multiple recent scans before qualifying for execution. Key = `symbol|longExchange|shortExchange` (direction-sensitive). Thresholds vary by funding interval:
   - 1h interval: 2 appearances in 15min lookback
   - 4h interval: 4 appearances in 30min lookback
   - 8h interval: 5 appearances in 40min lookback
   - **Spread stability**: For low-liquidity pairs (OIRank >= threshold), also checks that `minSpread / maxSpread >= stabilityRatio` across scans in the lookback window. Prevents volatile-magnitude spreads that persist in direction but collapse after entry. Configurable per interval tier (1h default: ratio=0.5, OIRank>=100; 4h/8h disabled).
7. **Spread volatility filter** (on entry scan only): Computes coefficient of variation (population stddev / mean) of spread values across scans within the tier-appropriate lookback. Rejects if CV > `SpreadVolatilityMaxCV` (default 0.5). Requires `SpreadVolatilityMinSamples` (default 3) data points. Catches erratic rate spikes.
8. **Symbol cooldown filter** (on entry scan only): Rejects if the symbol is in any cooldown. Two types: **Loss cooldown** (`LossCooldownHours`, default 4.0) — set after closing at a loss. **Re-enter cooldown** (`ReEnterCooldownHours`, default 0, disabled) — set after any close. Both persisted to Redis with TTL (`arb:lossCooldown:{symbol}`, `arb:reEnterCooldown:{symbol}`).
9. **Funding window filter** (on entry scan only): Drop opportunities with `NextFunding > FundingWindowMin` (default 30min) away
10. **Historical backtest filter** (on entry scan only): Fetches X days (default 3, `BacktestDays`) of historical funding settlement data from Loris API. Validates each leg independently against its own settlement schedule, sums cash flows, and rejects if `netProfit = shortSumY - longSumY <= BacktestMinProfit`. Results cached in Redis with 6h TTL.

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

**Scan-driven execution**: The scanner fires at configurable minutes (default :00, :10, :20, :30, :35, :40, :50) each hour. Each scan has a type: **NormalScan** (builds persistence history, feeds dashboard), **RebalanceScan** (:20 — fund rebalancing), **ExitScan** (:30 — exit checks), **EntryScan** (:40 — trade execution), or **RotateScan** (:35 — rotation checks). Schedule is configurable via `scan_minutes`, `rebalance_scan_minute`, `entry_scan_minute`, `exit_scan_minute`, `rotate_scan_minute`.

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

:20 scan   RebalanceScan → rebalanceFunds()
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
- **Capital exposure cap**: 60% of total capital per single exchange (hard limit)

**Position Sizing** (two modes):
1. **Fixed** (`CAPITAL_PER_LEG > 0`): `notional = capitalPerLeg * leverage`, clamped to available balance
2. **Auto** (`CAPITAL_PER_LEG = 0`): `notional = (min(balA, balB) * leverage) / (remainingSlots * 2)` — 50% safety buffer

After sizing: round to contract step size, enforce min size, enforce 10 USDT minimum margin per leg.

**Pre-Execution Fund Rebalancing**: On the rebalance scan (default :20), `rebalanceFunds()` analyzes capital needs across all discovered opportunities and ensures each exchange has sufficient margin:
1. Same-exchange spot→futures transfers (instant, free)
2. Cross-exchange withdrawals via APT (preferred) or BEP20 for remaining deficits
3. Only withdraws from L0/L1/L2 exchanges (margin ratio below L3 threshold)
4. Needs capped to `max_positions × capital_per_leg` to avoid over-allocation

**Stop-Loss Protection**: After position activation, protective stop-loss orders are placed on both legs. SL distance = `90% / leverage` (e.g. 3x→30%). Lifecycle:
- **Entry**: `attachStopLosses()` places SL on both exchanges, stores order IDs in position
- **Exit**: `cancelStopLosses()` cancels both SLs before closing position
- **Rotation**: `updateRotationStopLoss()` cancels old leg's SL, places new SL at new entry price
- SL failures are logged but never block entry/exit/rotation

| Exchange | Place endpoint | Cancel endpoint |
|----------|---------------|-----------------|
| Binance | `POST /fapi/v1/algoOrder` (STOP_MARKET) | `DELETE /fapi/v1/algoOrder` |
| Bybit | `POST /v5/order/create` (StopOrder) | `POST /v5/order/cancel` (StopOrder) |
| Gate.io | `POST /futures/usdt/price_orders` | `DELETE /futures/usdt/price_orders/{id}` |
| Bitget | `POST /api/v2/mix/order/place-plan-order` | `POST /api/v2/mix/order/cancel-plan-order` |
| OKX | `POST /api/v5/trade/order-algo` | `POST /api/v5/trade/cancel-algos` |

**Position Consolidator**: Background goroutine (every 5min) reconciles local position records with actual exchange positions:
- Missing leg detected → closes remaining leg and marks position closed
- Size mismatch >1% → syncs local record to exchange reality
- **Balance enforcer**: After sync, if long/short differ by >5%, trims excess side via confirmed reduce-only market order, then cancels and re-places stop losses
- Orphan exchange positions (not tracked locally) → auto-closes via reduce-only market IOC
- **Race protection**: Skips positions with active exit/entry goroutines (`exitActive`, `entryActive`, `busySymbols`) to prevent conflicts with in-progress trades

### 1.5 Exit Strategy

**Scan-aligned evaluation**: Exit checks run on ExitScan (:25) and EntryScan (:35) only. No timer-based polling.

**Decision flow per position**:

```
1. Safety: checkSpreadReversal() → exit if entry spread was positive but current is negative
   (with optional tolerance: allow N reversals before triggering, configurable via
   `SpreadReversalTolerance`, default 0 = immediate exit on first reversal)
2. Skip evaluation within ±10min of funding settlement (rates unreliable)
```

**Exit execution — Depth-Fill (non-emergency)**:
1. Set position status to `exiting` (slot remains occupied)
2. Spawn cancellable goroutine (`context.Context`)
3. Subscribe to WS depth on both exchanges
4. Depth-fill loop (200ms tick, `ExitDepthTimeoutSec` timeout):
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
| `ScanMinutes` | [5,15,25,35,45,55] | Minutes within each hour when scans fire (auto-includes special minutes) |
| `REBALANCE_SCAN_MINUTE` (env) | 20 | Minute mark that triggers fund rebalancing |
| `EntryScanMinute` | 35 | Minute mark that triggers trade execution |
| `ExitScanMinute` | 25 | Minute mark that triggers exit checks |
| `RotateScanMinute` | 45 | Minute mark that triggers rotation checks |

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
internal/
  config/config.go                 JSON + env config
  engine/
    engine.go                      Main loop, parallel trade execution, funding tracker, rebalancing
    consolidate.go                 Position consolidator: syncs exchange state with local records (5min), race-safe
    exit.go                        Scan-aligned exit (VWAP cost eval, depth-fill loop), L4 reduction, leg rotation
    engine_test.go                 Tests: effectiveAdvanceMin, classifyRotation
  discovery/
    scanner.go                     Fixed-schedule polling (Loris + CoinGlass), merge, persistence filter & broadcast
    verifier.go                    Exchange-native rate verification (direction-only)
    ranker.go                      Profitability filter, composite scoring, fee schedules
    ranker_test.go                 Ranking tests
  database/
    redis.go                       Redis client wrapper (DB 2)
    state.go                       Positions, history, funding snapshots, stats, transfers
    locks.go                       Distributed locking (SET NX + TTL)
  risk/
    manager.go                     11-point pre-trade risk approval, position sizing
    monitor.go                     5-dimension active position monitoring (30s)
    health.go                      5-tier margin health system (L0–L5), protective actions
    limits.go                      Hard constraints: 5x leverage, 60% exposure, 10 USDT floor
    health_test.go                 Health level tests
  models/
    interfaces.go                  Discoverer, RiskChecker, StateStore abstractions
    opportunity.go                 Opportunity struct (with IntervalHours, Source)
    position.go                    ArbitragePosition, status constants
    funding.go                     FundingRate, FundingSnapshot, LorisResponse, CoinGlassResponse
  api/
    server.go                      HTTP + WebSocket server
    handlers.go                    REST endpoints (positions, history, config, exchanges, transfers)
    ws.go                          WebSocket hub & client management
    auth.go                        Session-based auth (24h TTL tokens)
    frontend.go                    React SPA embedding & routing
web/
  embed.go                         React dist embedding via go:embed
pkg/
  exchange/                        Reusable exchange adapter package (importable by external projects)
    exchange.go                    Unified Exchange interface (33 methods)
    types.go                       Shared types (Order, Position, Balance, Orderbook, etc.)
    factory.go                     Exchange factory (logic in cmd/main.go to avoid cycles)
    binance/                       adapter.go, client.go, ws.go, ws_private.go
    bitget/                        adapter.go, client.go, ws.go, ws_private.go
    bybit/                         adapter.go, client.go, ws.go, ws_private.go
    gateio/                        adapter.go, client.go, ws.go, ws_private.go, adapter_test.go
    okx/                           adapter.go, client.go, ws.go, ws_private.go
    bingx/                         adapter.go, client.go, ws.go, ws_private.go
  utils/
    math.go                        RoundToStep, EstimateSlippage, RateToBpsPerHour, GenerateID
    logging.go                     Structured logging with module context + file output + daily rotation
```

**Total**: 64 Go files (~19,000 LOC) + 17 TS/TSX frontend files (~1,400 LOC)

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
       │ (broadcast) │  │  (main loop)│ :20 rebalance, :30 exit, :35 rotate, :40 entry
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

32 methods across 13 categories:

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

    // Stop-Loss (conditional orders)
    PlaceStopLoss(params StopLossParams) (orderID string, err error)
    CancelStopLoss(symbol, orderID string) error

    // Trade History
    GetUserTrades(symbol string, startTime time.Time, limit int) ([]Trade, error)
    GetFundingFees(symbol string, since time.Time) ([]FundingPayment, error)
    GetClosePnL(symbol string, since time.Time) ([]ClosePnL, error)
}
```

**Symbol format mapping**:
- Gate.io: `BTCUSDT` ↔ `BTC_USDT` (underscore)
- OKX: `BTCUSDT` ↔ `BTC-USDT-SWAP` (hyphenated with -SWAP)
- BingX: `BTCUSDT` ↔ `BTC-USDT` (hyphenated)
- Others: `BTCUSDT` as-is

Each adapter: `adapter.go` (interface impl), `client.go` (REST), `ws.go` (public WS), `ws_private.go` (private WS).

**Gate.io quanto multiplier**: Gate.io uses contract-based sizing. The adapter converts between base units and contracts using `quanto_multiplier` in PlaceOrder (divide), GetPosition/GetOrderFilledQty (multiply), and WS order updates (multiply). All other code works in base asset units.

### 2.4 Risk Management

#### Pre-Trade: 11-Point Risk Approval

1. Position count vs MaxPositions
2. Both exchanges exist in adapter map
3. Capital check + auto spot→futures transfer if needed
4. Leverage clamp to hard cap (5x)
5. Orderbook non-empty on long exchange
6. Position size calculation (CalculateSize)
7. Margin per leg >= 10 USDT floor
8. Slippage check on both exchanges (asks for long, bids for short)
9. Cross-exchange price gap check (absolute gap hard cap + directional recovery limit)
10. Per-exchange capital exposure cap (60% of total)
11. Exchange concentration warning (>60% of positions)

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
arb:stats                           HASH    total_pnl, trade_count, win_count, loss_count
arb:transfers                       HASH    transferID → JSON(TransferRecord)
arb:transfers:history               LIST    Transfer IDs (FIFO, max 200)
arb:lossCooldown:{symbol}           STRING  Loss cooldown flag (auto-expires via TTL)
arb:reEnterCooldown:{symbol}        STRING  Re-entry cooldown flag (auto-expires via TTL)
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
| GET | `/api/exchanges` | Exchange balances |
| POST | `/api/positions/close` | Manual close active position |
| POST | `/api/positions/open` | Manual open from opportunity |
| POST | `/api/transfer` | Execute cross-exchange withdrawal |
| GET | `/api/transfers` | Transfer history |
| GET | `/api/addresses` | Deposit addresses |
| GET | `/api/check-update` | Compare local vs remote VERSION |
| POST | `/api/update` | Git pull + make build + restart |

**WebSocket Messages** (`/ws`):
- `position_update` — single position change
- `positions` — full active position list
- `opportunities` — discovery results
- `stats` — PnL stats
- `alert` — risk alerts

### 2.8 Startup Sequence (cmd/main.go)

1. Load config (JSON + env overrides + EnsureScanMinutes)
2. Connect to Redis (DB 2)
3. Initialize exchange adapters (require min 2)
4. Load contract specs for all exchanges
5. Start WebSocket streams (public + private)
6. Initialize: Scanner, RiskManager, RiskMonitor, HealthMonitor, API Server, Engine
7. Start balance refresh goroutine (60s)
8. Start all components: Scanner → RiskMon → HealthMon → API → Engine (incl. Consolidator)
9. Wait for SIGINT/SIGTERM → graceful shutdown (reverse order)

---
