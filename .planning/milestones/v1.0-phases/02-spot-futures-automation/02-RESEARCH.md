# Phase 2: Spot-Futures Automation - Research

**Researched:** 2026-04-02
**Domain:** Go backend (spot-futures engine automation), React dashboard (config toggles)
**Confidence:** HIGH

## Summary

Phase 2 transforms the spot-futures engine from a manual-trigger system into a fully autonomous pipeline. The existing codebase already has most of the automation infrastructure built: `attemptAutoEntries()`, `checkRiskGate()`, `ManualOpen()`, `monitorPosition()`, `checkExitTriggers()`, and the full exit lifecycle. The primary work areas are: (1) building a native discovery scanner to replace the CoinGlass Chrome scraper as primary data source, (2) adding min-hold and settlement-window guards to the exit pipeline, (3) adding basis/spread gating at entry and exit, and (4) exposing new config fields in the dashboard.

The code surface is well-defined. All four requirements (SF-04 through SF-07) have clear integration points in existing files. The native scanner replaces `runDiscoveryScan()` in `discovery.go`, exit guards insert into `checkExitTriggers()` in `exit_manager.go`, basis/spread checks touch `risk_gate.go` (entry) and `exit_manager.go` (exit), and config fields follow the established `SpotFutures*` pattern in `config.go`. The existing test infrastructure (miniredis stubs, exchange stubs) covers all backend changes.

**Primary recommendation:** Implement discovery first (SF-04), then exit hardening (SF-06, SF-07), then auto-entry enablement (SF-05). Discovery feeds all downstream decisions; exit safeguards must be in place before enabling auto-entry.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Build a native discovery scanner using Loris API (funding rates) + exchange borrow rate APIs (already available via `getCachedBorrowRate`). This replaces CoinGlass Chrome scraper as the primary source.
- **D-02:** Keep CoinGlass scraper as an optional fallback data source, not the primary. The native scanner is more reliable (no headless Chrome dependency, no page structure breakage risk).
- **D-03:** Net yield calculation: `funding APR - borrow APR - trading fees`. Rank opportunities by net yield descending across all 5 exchanges.
- **D-04:** Reuse existing `attemptAutoEntries()` -> `checkRiskGate()` -> `ManualOpen()` path. The code is already built and wired -- Phase 2 enables and hardens it.
- **D-05:** One entry per scan cycle (existing sequential design). Prevents over-committing before Redis reflects the new position.
- **D-06:** Add configurable min-hold gate (don't exit before first funding settlement). Reasonable default, dashboard toggle.
- **D-07:** Add configurable settlement-window guard (skip exit evaluation during funding settlement periods when rates are temporarily unreliable). Reasonable default, dashboard toggle.
- **D-08:** Existing edge case handling stays: blackout windows (Bybit :04-:05:30), pending repay retry, stuck exit escalation to emergency after 5 retries.
- **D-09:** All gating thresholds are configurable with reasonable defaults and dashboard toggles. This follows the project convention (`feedback_risk_configurable_switch.md`).
- **D-10:** Entry basis/spread check: reject when spot-futures basis is too wide. Percentage-based threshold (configurable).
- **D-11:** Exit spread gate: threshold-gated close so exits don't fire at unfavorable spread.
- **D-12:** Simple flip approach. Deploy -> enable dry-run -> observe logs -> disable dry-run -> live. Uses existing config knobs: `auto_enabled`, `dry_run`, `max_positions`. No new code needed for the transition itself.
- **D-13:** Graduated ramp via existing `MaxPositions=1` -> increase after first successful trade cycle.

### Claude's Discretion
- Specific threshold default values for basis/spread gating, min-hold duration, settlement window timing
- Native scanner polling interval and data merging strategy with CoinGlass fallback
- Implementation ordering of plans (discovery first vs exit safeguards first)

### Deferred Ideas (OUT OF SCOPE)
- **Cross-exchange spot-futures** (borrow on X, trade on Y) -- v2 requirement (XSF-01, XSF-02, XSF-03), fundamentally different architecture
- **Automated dry-run validation** (track "would-have" entries and compare profitability) -- nice-to-have, not needed for Phase 2
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SF-04 | Auto-discovery pipeline finds and ranks spot-futures opportunities across all 5 exchanges | Native scanner using Loris API + borrow rate APIs; produces `SpotArbOpportunity` struct; replaces CoinGlass as primary in `runDiscoveryScan()` |
| SF-05 | Auto-open pipeline executes best opportunities without manual intervention | Existing `attemptAutoEntries()` -> `checkRiskGate()` -> `ManualOpen()` pipeline; enable via config; add basis/spread check to risk gate |
| SF-06 | Auto-exit handles all edge cases (blackout windows, partial fills, emergency close, pending repay retry) | Min-hold gate and settlement-window guard insert into `checkExitTriggers()`; existing blackout/repay/retry mechanisms stay |
| SF-07 | Basis/spread control on entry (reject if spread too wide) and exit (threshold-gated close) | New risk gate check in `checkRiskGate()` for entry; spread gate before exit trigger evaluation in `checkExitTriggers()` |
</phase_requirements>

## Architecture Patterns

### Existing Code Structure (spotengine package)
```
internal/spotengine/
  engine.go          -- SpotEngine struct, discoveryLoop(), monitorLoop(), lifecycle
  discovery.go       -- runDiscoveryScan(), SpotArbOpportunity, borrow rate cache
  autoentry.go       -- attemptAutoEntries(), sequential one-per-scan
  risk_gate.go       -- checkRiskGate(), 5 pre-entry checks
  execution.go       -- ManualOpen(), trade leg execution (1678 lines)
  exit_manager.go    -- checkExitTriggers() (5 triggers), initiateExit(), completeExit()
  monitor.go         -- monitorLoop(), monitorTick(), updateBorrowCost(), retryPendingRepay()
  rate_velocity.go   -- RateVelocityDetector, borrow spike detection
  capital.go         -- Capital reservation/release helpers
```

### Pattern 1: Native Discovery Scanner (replacing CoinGlass)
**What:** Replace `runDiscoveryScan()` body (currently reads CoinGlass Redis key) with a Loris API poll + exchange borrow rate merge.
**When to use:** Every scan cycle in `discoveryLoop()`.
**Integration point:** `internal/spotengine/discovery.go:69` -- `runDiscoveryScan()`.

The native scanner must:
1. Poll Loris API at `https://api.loris.tools/funding` (same as perp-perp scanner in `internal/discovery/scanner.go:220`)
2. Extract per-symbol funding rates for each exchange
3. Normalize Loris rates (8h-equivalent, divide by 8 for bps/h, then annualize)
4. For each symbol+exchange: check SpotMarginExchange availability, fetch borrow rate via `getCachedBorrowRate()`
5. Calculate net yield: `fundingAPR - borrowAPR - feeAPR`
6. Produce `[]SpotArbOpportunity` with same struct as today (zero downstream changes)
7. Fall back to CoinGlass Redis key if Loris is unavailable/stale

**Key detail -- Loris rate normalization for spot-futures:**
- Loris returns 8h-equivalent rates in basis points
- Perp-perp divides by 8 to get bps/hour (`internal/discovery/ranker.go:75`)
- For spot-futures, annualize: `bpsPerHour * 8760 / 10000` to get decimal APR
- Loris also provides `funding_intervals` per exchange/symbol (hours between settlements)

**Key detail -- direction determination:**
- CoinGlass provides "Sell BTC" / "Buy ETH" in Portfolio field, parsed by `parsePortfolio()`
- Native scanner must determine direction from funding rate sign:
  - Positive funding = longs pay shorts -> Dir A (borrow-sell-long futures) is profitable
  - Negative funding = shorts pay longs -> Dir B (buy-spot-short futures) is profitable
- For Dir A: must check borrow availability and rate
- For Dir B: no borrow needed (borrowAPR = 0)

**Key detail -- symbol enumeration:**
- Loris provides all symbols across all exchanges (same as perp-perp)
- Filter to symbols that have both: (a) futures contract, (b) spot margin availability
- Existing `spotMargin` map + `GetMarginBalance()` check already does this

```go
// Pseudocode for native scanner core loop
func (e *SpotEngine) runNativeDiscoveryScan() []SpotArbOpportunity {
    loris, err := e.pollLoris()  // reuse Poll() pattern from discovery/scanner.go
    if err != nil {
        e.log.Error("native scan: Loris unavailable: %v", err)
        return e.runCoinGlassFallback()  // existing CoinGlass code path
    }

    var opps []SpotArbOpportunity
    for _, baseSym := range loris.Symbols {
        symbol := strings.ToUpper(baseSym) + "USDT"
        for exchName, smExch := range e.spotMargin {
            rates, ok := loris.FundingRates[exchName]
            if !ok { continue }
            rawRate, ok := rates[baseSym]
            if !ok { continue }

            // Normalize: Loris 8h-equiv -> bps/h -> APR
            bpsPerHour := rawRate / 8.0
            fundingAPR := bpsPerHour * 8760 / 10000

            // Determine direction from funding sign
            if fundingAPR > 0 {
                // Dir A: borrow-sell-long (longs pay shorts, we earn from shorts)
                borrowRate, err := e.getCachedBorrowRate(exchName, baseSym, smExch)
                // ... calculate netAPR = fundingAPR - borrowAPR - feeAPR
            }
            // Dir B: buy-spot-short (always possible when funding > 0 from futures side)
            // ... calculate netAPR = fundingAPR - 0 - feeAPR
        }
    }
    return opps
}
```

### Pattern 2: Min-Hold Gate (exit safeguard)
**What:** Prevent exits before a position has collected at least one funding settlement.
**When to use:** Insert as a pre-check in `checkExitTriggers()`, before triggers 1-4.
**Integration point:** `internal/spotengine/exit_manager.go:22` -- top of `checkExitTriggers()`.

**Reference pattern from perp-perp:**
- Perp-perp uses `MinHoldTime` (default 16h) in `internal/config/config.go:30`
- Applied in `internal/engine/exit.go:161` -- but note: spread reversal bypasses min-hold (safety override)

**Spot-futures adaptation:**
- Default: 8 hours (covers one standard funding period)
- Emergency exits (price spike, margin health) MUST bypass min-hold
- Normal yield-based exits respect min-hold

```go
// In checkExitTriggers(), before existing trigger checks:
if e.cfg.SpotFuturesMinHoldHours > 0 {
    holdDuration := time.Since(pos.CreatedAt)
    minHold := time.Duration(e.cfg.SpotFuturesMinHoldHours) * time.Hour
    if holdDuration < minHold {
        // Still allow emergency triggers to bypass
        // But skip yield/borrow-cost triggers
        // Only check price spike and margin health
    }
}
```

### Pattern 3: Settlement Window Guard (exit safeguard)
**What:** Skip exit evaluation during the funding settlement window when rates are temporarily unreliable.
**When to use:** Before yield-based exit triggers fire.
**Integration point:** Same as min-hold gate in `checkExitTriggers()`.

**Funding settlement windows by exchange:**
| Exchange | Settlement times (UTC) | Duration |
|----------|----------------------|----------|
| Binance | :00 (every 8h) | ~1-2 min |
| Bybit | :00 (every 8h), blackout :04-:05:30 | ~5 min |
| OKX | :00 (some 8h, some 4h, some 1h) | ~1-2 min |
| Bitget | :00 (every 8h) | ~1-2 min |
| Gate.io | :00 (every 8h) | ~1-2 min |

**Recommended defaults:**
- Guard window: 10 minutes around settlement (5 min before, 5 min after :00)
- Only blocks yield-based triggers; emergency exits still fire

```go
// Settlement window check
func isInSettlementWindow(exchName string, windowMin int) bool {
    now := time.Now().UTC()
    minute := now.Minute()
    // Standard: funding settles at :00 every 8h (00:00, 08:00, 16:00)
    hour := now.Hour()
    isSettlementHour := hour%8 == 0 || hour%8 == 7 // also check end of previous period
    if isSettlementHour && minute < windowMin {
        return true
    }
    if isSettlementHour && minute >= 60-windowMin {
        return true
    }
    return false
}
```

**Note:** Bybit has an additional repay blackout at :04-:05:30 which is already handled in `retryPendingRepay()` via `ErrRepayBlackout`. The settlement window guard is about rate reliability, not API availability.

### Pattern 4: Basis/Spread Gating (entry)
**What:** Reject auto-entry when the spot-futures basis (price difference between spot and futures) is too wide, indicating unfavorable entry conditions.
**When to use:** New check #6 in `checkRiskGate()` (after persistence, before dry-run).
**Integration point:** `internal/spotengine/risk_gate.go:18` -- add after check 4.

**Basis calculation:**
```go
// basis = (futuresPrice - spotPrice) / spotPrice * 100
// For Dir A (short spot, long futures): positive basis means we pay more on futures entry
// For Dir B (long spot, short futures): positive basis means we get more on futures entry
func (e *SpotEngine) checkBasis(symbol, exchName string) (float64, error) {
    exch := e.exchanges[exchName]
    ob, err := exch.GetOrderbook(symbol, 5)
    if err != nil { return 0, err }
    futuresMid := (ob.Bids[0].Price + ob.Asks[0].Price) / 2

    smExch := e.spotMargin[exchName]
    spotBal, err := smExch.GetMarginBalance(/* needs spot price, not balance */)
    // Alternative: use BBO from price stream for spot price
    // spotBBO, ok := exch.GetBBO(symbol)
    // The basis check needs spot price -- but spot margin API doesn't give price.
    // Use futures orderbook as proxy (delta-neutral positions have ~same price)
    // OR: compare mark price vs last price from the exchange
    return 0, nil
}
```

**Important consideration:** The spot-futures engine currently does NOT maintain separate spot and futures price feeds. The existing `GetOrderbook()` returns futures orderbook. For basis calculation, we need both spot and futures mid prices. Options:
1. Use `GetMarginBalance()` to get an implicit spot price (not reliable)
2. Add a spot orderbook call to the exchange interface (significant work)
3. Use the basis implied from the position entry/exit slippage (not available pre-entry)
4. **Recommended:** Use the funding rate as a proxy for basis -- high funding rates correlate with high basis. The net yield calculation already accounts for funding. The "basis check" can be implemented as a minimum net yield threshold (already exists: `SpotFuturesMinNetYieldAPR`).

**Revised approach per D-10:** The entry basis check should compare futures price vs. spot price at time of entry. Since the engine already calls `GetOrderbook()` for futures prices, and the spot margin order execution gets fills at market, the practical approach is:
1. Fetch futures mid price from orderbook
2. Estimate spot price from futures price (they track closely for USDT pairs)
3. Calculate basis percentage
4. Reject if basis > threshold

**Recommended default:** Entry basis threshold: 0.5% (reject entries where basis > 0.5%)

### Pattern 5: Spread Gate (exit)
**What:** Don't trigger yield-based exits when the current spread (cost to close) is unfavorable.
**When to use:** Wrap yield-based exit triggers (borrow cost, funding rate drop) with a spread check.
**Integration point:** `internal/spotengine/exit_manager.go` -- around triggers 1 and 2.

The "spread" for spot-futures is the cost of unwinding:
- Dir A: buy back spot + close long futures -- spread = bid-ask spread on both legs
- Dir B: sell spot + close short futures -- spread = bid-ask spread on both legs

**Recommended approach:** Calculate total unwind slippage estimate from orderbook depth. If slippage exceeds threshold, defer yield-based exit to next monitor cycle. Emergency exits always bypass.

**Recommended default:** Exit spread threshold: 0.3% (don't exit if estimated slippage > 0.3%)

### Anti-Patterns to Avoid
- **Modifying CoinGlass data flow:** The CoinGlass scraper (`internal/scraper/spotarb.go`) writes to Redis key `coinGlassSpotArb`. The native scanner should NOT write to this same key. Instead, native results should be used directly in `runDiscoveryScan()`, with CoinGlass as a fallback read-only source.
- **Breaking the one-entry-per-scan invariant:** `attemptAutoEntries()` returns after the first successful entry (line 55). This is intentional -- don't parallelize entries.
- **Bypassing config toggles:** Every new feature MUST have an `Enable*` config field (default OFF) and dashboard toggle. This is a hard project convention per `feedback_risk_configurable_switch.md`.
- **Ignoring Loris rate normalization:** Loris rates are 8h-equivalent. Forgetting to divide by 8 before annualizing produces 8x inflated APR values.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Loris API polling | Custom HTTP client | Reuse pattern from `internal/discovery/scanner.go:220` (Poll method) | Same API, same auth, same error handling |
| Borrow rate caching | New cache system | Existing `borrowCache` sync.Map + `getCachedBorrowRate()` in `discovery.go:250` | Already built, 5-min TTL, thread-safe |
| Config field pattern | Custom config handling | Follow `SpotFutures*` pattern in `config.go` with `jsonSpotFutures` struct | Consistent JSON tags, dashboard binding, Redis persistence |
| Exit state tracking | Custom locks | Existing `exitState` + `exitMu` + `isExiting()` in `exit_manager.go` | Prevents double-trigger, tested |
| Persistence counting | Custom counters | Existing `IncrSpotPersistence` / `GetSpotPersistence` in database | Redis-backed, already wired |
| Dashboard config API | New endpoints | Extend existing `handleSpotAutoConfig` in `spot_handlers.go:261` | Same pattern, same auth |

## Common Pitfalls

### Pitfall 1: Loris Rate Units
**What goes wrong:** Using raw Loris rates as-is produces 8x inflated funding APR values, leading to false-positive entries on unprofitable opportunities.
**Why it happens:** Loris normalizes all rates to 8h-equivalent basis points, regardless of the exchange's actual funding interval (which may be 1h, 4h, or 8h).
**How to avoid:** Always divide by 8 first (`rawRate / 8.0` gives bps/hour), then convert to APR (`bpsPerHour * 8760 / 10000`). Reference: `internal/discovery/ranker.go:75` and `project_loris_normalization.md`.
**Warning signs:** Opportunities showing 40%+ APR for stable coins.

### Pitfall 2: Direction Determination Without CoinGlass
**What goes wrong:** CoinGlass explicitly tells you "Sell BTC" (Dir A) or "Buy ETH" (Dir B) via the Portfolio field. Without CoinGlass, the native scanner must derive direction from funding rate sign.
**Why it happens:** Positive funding means longs pay shorts. For spot-futures: if you're short spot + long futures (Dir A), you receive funding. If funding is positive AND borrow cost is manageable, Dir A is profitable. But Dir B (buy spot, short futures) also earns positive funding differently.
**How to avoid:** For each symbol+exchange, generate BOTH directions as separate opportunities, each with their own net yield calculation. Let the ranking sort them naturally. The CoinGlass approach of showing only one direction per symbol was an optimization, not a requirement.
**Warning signs:** Only seeing Dir A or only Dir B opportunities in scan results.

### Pitfall 3: Exit Guard Priority
**What goes wrong:** Min-hold gate blocks emergency exits, causing positions to blow up during price spikes.
**Why it happens:** Inserting min-hold at the top of `checkExitTriggers()` without exception handling.
**How to avoid:** Structure exit triggers with clear priority:
1. ALWAYS check: price spike (emergency), margin health (emergency)
2. GATED by min-hold + settlement window: borrow cost drift, funding rate drop
3. GATED by spread threshold: same yield-based triggers

**Warning signs:** Emergency exit logs not firing during volatile periods for young positions.

### Pitfall 4: CoinGlass Fallback Staleness
**What goes wrong:** Falling back to CoinGlass data that's hours old, leading to entries based on stale rates.
**Why it happens:** CoinGlass scraper runs on a schedule (minutes 15, 35) and writes to Redis. If the scraper fails or Chrome crashes, the Redis data persists but becomes stale.
**How to avoid:** The existing staleness check in `runDiscoveryScan()` (line 89) already rejects data older than 60 minutes. Keep this check for the fallback path.
**Warning signs:** Discovery scan logs showing "data is X min old, skipping".

### Pitfall 5: Config Field Symmetry
**What goes wrong:** Adding a Go config field but forgetting one of: JSON tag, dashboard handler, Redis persistence, env var, dashboard UI toggle, i18n keys.
**Why it happens:** The config system has 6 touch points for each field.
**How to avoid:** Checklist for each new config field:
1. `internal/config/config.go` -- struct field + default + JSON struct + apply function + toJSON + fromEnv
2. `internal/api/spot_handlers.go` -- GET/POST handler for dashboard updates
3. `web/src/pages/Config.tsx` -- UI component in appropriate sf-* tab
4. `web/src/i18n/en.ts` + `web/src/i18n/zh-TW.ts` -- translation keys
5. Redis persistence key in `SetConfigField` call
**Warning signs:** Config changes from dashboard not surviving restart.

### Pitfall 6: Bybit Blackout Window Interaction
**What goes wrong:** Exit trigger fires at :04 UTC but the actual close sequence runs into Bybit's repay blackout (:04-:05:30), leaving the position in a stuck "exiting + pending repay" state.
**Why it happens:** `checkExitTriggers()` fires the trigger, `initiateExit()` calls `ClosePosition()`, which tries to repay and gets `ErrRepayBlackout`.
**How to avoid:** This is ALREADY handled by the existing code in `exit_manager.go:319-342` (PendingRepay path) and `monitor.go:289-403` (retryPendingRepay). No new code needed -- just don't break the existing pattern.
**Warning signs:** `retryPendingRepay` logs showing repeated blackout deferrals.

## Code Examples

### Example 1: Config Field Pattern (complete lifecycle)

Each new config field must follow this pattern. Using `SpotFuturesMinHoldHours` as example:

```go
// 1. config.go -- struct field
type Config struct {
    // ...
    SpotFuturesMinHoldHours       int     // minimum hours before yield-based exit (default: 8)
    SpotFuturesEnableMinHold      bool    // enable min-hold gate (default: false)
}

// 2. config.go -- JSON struct
type jsonSpotFutures struct {
    // ...
    MinHoldHours       *int  `json:"min_hold_hours"`
    EnableMinHold      *bool `json:"enable_min_hold"`
}

// 3. config.go -- defaults
func DefaultConfig() *Config {
    return &Config{
        SpotFuturesMinHoldHours:  8,
        SpotFuturesEnableMinHold: false,
    }
}

// 4. config.go -- apply from JSON
if sf.MinHoldHours != nil && *sf.MinHoldHours > 0 {
    c.SpotFuturesMinHoldHours = *sf.MinHoldHours
}
if sf.EnableMinHold != nil {
    c.SpotFuturesEnableMinHold = *sf.EnableMinHold
}

// 5. config.go -- toJSON
sf["min_hold_hours"] = c.SpotFuturesMinHoldHours
sf["enable_min_hold"] = c.SpotFuturesEnableMinHold

// 6. spot_handlers.go -- dashboard POST handler
if req.MinHoldHours != nil && *req.MinHoldHours > 0 {
    s.cfg.SpotFuturesMinHoldHours = *req.MinHoldHours
    s.db.SetConfigField("spot_futures_min_hold_hours", strconv.Itoa(*req.MinHoldHours))
}

// 7. i18n/en.ts
'cfg.sf.minHoldHours': 'Min Hold Hours',
'cfg.sf.minHoldHoursDesc': 'Minimum hours to hold before yield-based exits trigger. 0 = disabled.',
```

### Example 2: Risk Gate Check Pattern (entry)

```go
// In risk_gate.go, after check 4 (persistence), before check 5 (dry-run):

// 5. Basis gate: reject entry if spot-futures basis is too wide.
if e.cfg.SpotFuturesEnableBasisGate {
    basisPct, err := e.calculateBasis(symbol, exchName)
    if err != nil {
        e.log.Warn("risk-gate: basis check failed for %s on %s: %v", symbol, exchName, err)
        // Fail-open or fail-closed is a design choice. Recommend fail-closed.
        return RiskGateResult{Allowed: false, Reason: "basis_check_error"}
    }
    maxBasis := e.cfg.SpotFuturesMaxBasisPct
    if maxBasis <= 0 {
        maxBasis = 0.5 // default 0.5%
    }
    if basisPct > maxBasis {
        return RiskGateResult{Allowed: false, Reason: fmt.Sprintf("basis_%.2f%%>%.2f%%", basisPct, maxBasis)}
    }
}

// 6. Dry-run (renumbered from 5)
```

### Example 3: Exit Guard Structure

```go
func (e *SpotEngine) checkExitTriggers(pos *models.SpotFuturesPosition) (string, bool) {
    // --- ALWAYS-ON: Safety triggers (bypass all guards) ---

    // 3. Price Spike (existing, renumber as first check)
    // 4. Margin Health (existing, renumber as second check)

    // --- GUARDED: Yield-based triggers ---

    // Min-hold gate
    if e.cfg.SpotFuturesEnableMinHold {
        minHold := time.Duration(e.cfg.SpotFuturesMinHoldHours) * time.Hour
        if time.Since(pos.CreatedAt) < minHold {
            return "", false // too young for yield-based exit
        }
    }

    // Settlement window guard
    if e.cfg.SpotFuturesEnableSettlementGuard {
        windowMin := e.cfg.SpotFuturesSettlementWindowMin
        if windowMin <= 0 { windowMin = 10 }
        if isInSettlementWindow(pos.Exchange, windowMin) {
            return "", false // rates unreliable during settlement
        }
    }

    // Exit spread gate
    if e.cfg.SpotFuturesEnableExitSpreadGate {
        slippagePct := e.estimateUnwindSlippage(pos)
        maxSlippage := e.cfg.SpotFuturesExitSpreadPct
        if maxSlippage <= 0 { maxSlippage = 0.3 }
        if slippagePct > maxSlippage {
            return "", false // too expensive to close right now
        }
    }

    // 1. Borrow Cost Drift (existing)
    // 2. Funding Rate Drop (existing)

    return "", false
}
```

## New Config Fields Required

| Field | Type | Default | Purpose | Enable Toggle |
|-------|------|---------|---------|---------------|
| `SpotFuturesEnableMinHold` | bool | false | Enable min-hold gate | Yes |
| `SpotFuturesMinHoldHours` | int | 8 | Hours before yield-based exit allowed | N/A (uses enable toggle) |
| `SpotFuturesEnableSettlementGuard` | bool | false | Skip exit eval during settlement window | Yes |
| `SpotFuturesSettlementWindowMin` | int | 10 | Minutes around settlement to guard | N/A |
| `SpotFuturesEnableBasisGate` | bool | false | Reject entry when basis too wide | Yes |
| `SpotFuturesMaxBasisPct` | float64 | 0.5 | Maximum basis % for entry | N/A |
| `SpotFuturesEnableExitSpreadGate` | bool | false | Gate exits by unwind slippage | Yes |
| `SpotFuturesExitSpreadPct` | float64 | 0.3 | Maximum slippage % to allow exit | N/A |
| `SpotFuturesNativeScannerEnabled` | bool | true | Use Loris-based native scanner (false = CoinGlass only) | Yes |

**All follow the established pattern:** `Enable{Feature}` bool field (default OFF per project convention), JSON tag in `jsonSpotFutures`, Redis persistence, dashboard toggle.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| CoinGlass Chrome scraper | Native Loris API + borrow rate APIs | Phase 2 (this phase) | Eliminates headless Chrome dependency, reduces latency, improves reliability |
| Manual dashboard open | Auto-discovery + auto-entry pipeline | Phase 2 (this phase) | Full automation, no manual intervention needed |
| No min-hold / no settlement guard | Configurable guards | Phase 2 (this phase) | Prevents premature exits during rate volatility |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `github.com/alicebob/miniredis/v2` |
| Config file | None (Go stdlib testing, no config file) |
| Quick run command | `go test ./internal/spotengine/ -run TestName -v -count=1` |
| Full suite command | `go test ./internal/spotengine/ -v -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SF-04 | Native scanner produces opportunities from Loris data | unit | `go test ./internal/spotengine/ -run TestNativeScanner -v -count=1` | Wave 0 |
| SF-04 | CoinGlass fallback activates when Loris unavailable | unit | `go test ./internal/spotengine/ -run TestCoinGlassFallback -v -count=1` | Wave 0 |
| SF-04 | Net yield ranking: funding - borrow - fees, descending | unit | `go test ./internal/spotengine/ -run TestNetYieldRanking -v -count=1` | Wave 0 |
| SF-05 | Auto-entry triggers via existing pipeline when enabled | unit | `go test ./internal/spotengine/ -run TestAutoEntry -v -count=1` | Partial (risk_gate_test.go exists) |
| SF-06 | Min-hold gate blocks yield exits for young positions | unit | `go test ./internal/spotengine/ -run TestMinHoldGate -v -count=1` | Wave 0 |
| SF-06 | Settlement window guard blocks yield exits | unit | `go test ./internal/spotengine/ -run TestSettlementGuard -v -count=1` | Wave 0 |
| SF-06 | Emergency exits bypass all guards | unit | `go test ./internal/spotengine/ -run TestEmergencyBypass -v -count=1` | Wave 0 |
| SF-07 | Entry rejected when basis too wide | unit | `go test ./internal/spotengine/ -run TestBasisGateEntry -v -count=1` | Wave 0 |
| SF-07 | Exit gated by spread threshold | unit | `go test ./internal/spotengine/ -run TestExitSpreadGate -v -count=1` | Wave 0 |
| SF-07 | Spread reason visible in logs | unit | `go test ./internal/spotengine/ -run TestSpreadLogging -v -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/spotengine/ -run TestSpecificChange -v -count=1`
- **Per wave merge:** `go test ./internal/spotengine/ -v -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/spotengine/native_scanner_test.go` -- covers SF-04 (Loris polling, fallback, direction determination, net yield ranking)
- [ ] `internal/spotengine/exit_guards_test.go` -- covers SF-06 (min-hold, settlement window, emergency bypass)
- [ ] `internal/spotengine/basis_spread_test.go` -- covers SF-07 (entry basis gate, exit spread gate, logging)

*(Existing test infrastructure: miniredis stubs, exchange stubs already in `execution_test.go` and `exit_triggers_test.go` -- reuse for new tests.)*

## Integration Points Summary

| Requirement | Primary File | Function | Change Type |
|-------------|-------------|----------|-------------|
| SF-04 | `internal/spotengine/discovery.go` | `runDiscoveryScan()` | Replace body with native scanner, keep CoinGlass as fallback |
| SF-04 | `internal/spotengine/engine.go` | `discoveryLoop()` | No change needed (calls runDiscoveryScan) |
| SF-05 | `internal/spotengine/risk_gate.go` | `checkRiskGate()` | Add basis gate check (new check 5) |
| SF-05 | `internal/spotengine/autoentry.go` | `attemptAutoEntries()` | No change needed (already wired) |
| SF-06 | `internal/spotengine/exit_manager.go` | `checkExitTriggers()` | Add min-hold gate, settlement window guard, restructure priority |
| SF-07 | `internal/spotengine/risk_gate.go` | `checkRiskGate()` | Basis gate (same as SF-05) |
| SF-07 | `internal/spotengine/exit_manager.go` | `checkExitTriggers()` | Add exit spread gate |
| All | `internal/config/config.go` | Config struct | Add 9 new fields |
| All | `internal/api/spot_handlers.go` | `handleSpotAutoConfig()` | Extend GET/POST for new fields |
| All | `web/src/pages/Config.tsx` | Spot-futures config tabs | Add toggles/inputs for new fields |
| All | `web/src/i18n/en.ts` + `zh-TW.ts` | Translation keys | Add ~20 new keys |

## Recommended Implementation Order

1. **Wave 1: Native Discovery Scanner (SF-04)** -- most foundational, feeds all downstream
   - Build Loris polling in spotengine (reuse perp-perp pattern)
   - Direction determination from funding sign
   - Net yield calculation with borrow rate merge
   - CoinGlass fallback path
   - Config toggle: `SpotFuturesNativeScannerEnabled`
   - Tests

2. **Wave 2: Exit Safeguards (SF-06)** -- must be in place before enabling auto-entry
   - Min-hold gate with config
   - Settlement window guard with config
   - Restructure trigger priority (emergency first)
   - Dashboard toggles
   - Tests

3. **Wave 3: Basis/Spread Gating (SF-07)** -- entry and exit quality control
   - Entry basis gate in risk_gate.go
   - Exit spread gate in exit_manager.go
   - Config fields and dashboard toggles
   - Tests

4. **Wave 4: Auto-Entry Enablement (SF-05)** -- final piece, all safeguards in place
   - Verify existing pipeline works with native scanner
   - Dashboard config for go-live controls
   - Integration test with dry-run mode
   - Go-live documentation

## Open Questions

1. **Spot price source for basis calculation**
   - What we know: Futures orderbook is readily available via `GetOrderbook()`. Spot price is not directly available without an additional API call.
   - What's unclear: Whether to add a spot orderbook method to the exchange interface, use the BBO price stream (currently futures-only), or approximate basis from the funding rate itself.
   - Recommendation: For initial implementation, use futures mid-price as a proxy (spot and futures prices for USDT pairs track closely, typically within 0.1%). If more precision is needed, add a `GetSpotOrderbook()` method in a later iteration. The basis gate threshold (0.5%) provides enough buffer.

2. **Loris API symbol coverage for spot-margin-capable symbols**
   - What we know: Loris covers all major futures symbols across all 6 exchanges. Not all futures symbols have spot margin support.
   - What's unclear: Whether Loris covers enough symbols that also have spot margin availability, or if many opportunities will be filtered out.
   - Recommendation: Log and track "symbol has funding data but no margin support" cases in the first week of native scanner operation. This will inform whether a supplementary data source is needed.

3. **Settlement window timing for non-8h intervals**
   - What we know: Most symbols have 8h funding intervals, but some (especially on OKX) have 4h or 1h intervals. Loris provides `funding_intervals` per exchange/symbol.
   - What's unclear: Whether the settlement window guard should dynamically adjust based on the symbol's actual funding interval.
   - Recommendation: Start with a simple approach (guard around :00 of settlement hours). Use `funding_intervals` from Loris to determine which hours are settlement hours for each symbol. Store this per-position at entry time so the guard can reference it.

## Sources

### Primary (HIGH confidence)
- **Codebase inspection** -- all files in `internal/spotengine/`, `internal/discovery/`, `internal/config/`, `internal/api/`, `pkg/exchange/`
- **Existing test files** -- `discovery_test.go`, `execution_test.go`, `exit_triggers_test.go`, `risk_gate_test.go`, `rate_velocity_test.go`
- **CONTEXT.md** -- user decisions D-01 through D-13
- **REQUIREMENTS.md** -- SF-04 through SF-07

### Secondary (MEDIUM confidence)
- **Perp-perp patterns** -- `internal/discovery/scanner.go` (Loris polling), `internal/engine/exit.go` (min-hold, settlement patterns)
- **Project memory** -- `project_loris_normalization.md`, `project_spot_futures_engine.md`, `feedback_risk_configurable_switch.md`

### Tertiary (LOW confidence)
- **Settlement window timing** -- estimated from exchange documentation in memory, needs validation against live behavior during first operational cycle

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries already in use (Go stdlib, go-redis, miniredis, gorilla/websocket)
- Architecture: HIGH -- all integration points identified, patterns verified in existing code
- Pitfalls: HIGH -- derived from actual codebase bugs and project memory
- Basis/spread calculation: MEDIUM -- spot price source needs design decision

**Research date:** 2026-04-02
**Valid until:** 2026-04-30 (stable domain, no external dependencies changing)
