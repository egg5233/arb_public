# Phase 6: Spot-Futures Risk Hardening - Research

**Researched:** 2026-04-05
**Domain:** Spot-futures liquidation risk, per-contract maintenance margin rates, exchange API tiered risk limits
**Confidence:** HIGH

## Summary

Phase 6 hardens the spot-futures engine against liquidation risk by integrating per-contract maintenance margin rates into four subsystems: pre-entry rejection, runtime liquidation distance monitoring, the perp-perp health monitor, and discovery display. The motivating incident (GUAUSDT on Gate.io, 30% maintenance_rate, only 10% buffer to liquidation) proves that the existing fixed-percentage price-spike exit is insufficient for high-maintenance-rate contracts.

The core technical challenge is that each of the 5 exchanges exposes maintenance margin data through different API endpoints, response formats, and tier structures. Gate.io includes `maintenance_rate` directly in the contract info response, but the other 4 exchanges require separate API calls to tiered risk-limit endpoints. Each exchange returns maintenance rates in different formats (decimal fraction vs percentage string) and with different tier boundary semantics (notional USDT ranges vs position count ranges).

**Primary recommendation:** Implement a `GetMaintenanceRate(symbol string, notional float64) (float64, error)` method pattern in each adapter that queries the exchange-specific risk-limit endpoint, matches the appropriate tier for the given notional size, and returns a normalized decimal fraction (e.g., 0.005 = 0.5%). Cache results per-symbol with a configurable TTL (default 1 hour) since maintenance rate tiers change infrequently. The `ContractInfo.MaintenanceRate` field stores the tier-1 (lowest position size) rate from `LoadAllContracts`, which is sufficient for discovery display. The runtime and pre-entry paths use `GetMaintenanceRate()` with actual notional for accurate tier matching.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Maintenance_rate check inserts as check 6 in `checkRiskGate()`, before dry-run (which becomes check 7)
- **D-02:** Leverage-scaled threshold: `min_survivable_drop = 90% / leverage`. Defaults: 2x->45%, 3x->30%, 4x->22.5%, 5x->18%
- **D-03:** Auto-entry: hard reject with logged reason. Manual open: allow bypass with warning
- **D-04:** Manual open via dashboard allows bypass with warning response
- **D-05:** Same threshold for all exchanges (no unified vs separate distinction for pre-entry)
- **D-06:** Enhance spotengine's exit trigger #2 with maintenance_rate-aware liq distance (additional trigger, not replacement)
- **D-07:** Liquidation distance = `|mark_price - estimated_liq_price| / mark_price` using per-symbol maintenance_rate
- **D-08:** Graduated response: warn at 50% of entry threshold, exit at 20%, emergency at 10%
- **D-09:** Examples at 3x leverage (30% threshold): warn at 15%, exit at 6%, emergency at 3%
- **D-10:** Add `GetActiveSpotPositions()` to `HealthMonitor.checkAll()` for L4/L5 actions
- **D-11:** Same L3/L4/L5 tiers and thresholds as perp-perp (no separate tier system)
- **D-12:** L3 transfer: same as perp-perp, no separate-account futures->margin transfer
- **D-13:** Fetch maintenance_rate during discovery, add to `SpotArbOpportunity` struct
- **D-14:** Display maintenance_rate in dashboard opportunities table
- **D-15:** No filter or scoring penalty on maintenance_rate for now (display only)
- **D-16:** Add `MaintenanceRate float64` to `ContractInfo` in `pkg/exchange/types.go`
- **D-17:** Per-exchange API sources verified (see Architecture Patterns section)
- **D-18:** For tiered rates, use tier matching position's notional size
- **D-19:** Config convention: `Enable{Feature}` bool (default OFF) + JSON tag + dashboard toggle
- **D-20:** Config fields go in existing `spot_futures` JSON section

### Claude's Discretion
- Specific implementation of tiered maintenance_rate lookup (cache strategy, refresh interval)
- Dashboard UI layout for displaying maintenance_rate in opportunities table
- Exact config field naming within the established convention
- Plan decomposition and wave ordering

### Deferred Ideas (OUT OF SCOPE)
- Discovery scoring/filtering by maintenance_rate (display first, decide later)
- Separate-account futures->margin profit transfer (complex cross-account, future phase)
- Price monitor goroutine (10-second fast-poll, separate enhancement from DESIGN_SPOT_FUTURES_RISK.md Tier 1)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SF-RISK-01 | Pre-entry check fetches per-contract maintenance_rate and rejects positions where max survivable price drop < configured threshold | Exchange API research confirms per-exchange maintenance_rate endpoints; `checkRiskGate()` insertion point verified at line 79 of risk_gate.go; formula `survivable_drop = 90% / leverage` validated against leverage mechanics |
| SF-RISK-02 | Runtime monitor calculates liquidation distance using mark price + maintenance_rate, triggers exit when distance falls below threshold | Existing `checkExitTriggers()` Phase 1 safety section verified; `checkLiquidationDistance()` in risk/monitor.go provides reference pattern; graduated response (50%/20%/10% of threshold) maps to warn/exit/emergency |
| SF-RISK-03 | Health monitor evaluates spot-futures positions for L3/L4/L5 actions | `HealthMonitor.checkAll()` at health.go:153 calls only `GetActivePositions()`; adding `GetActiveSpotPositions()` is straightforward; `HealthAction` struct already supports all needed action types |
| SF-RISK-04 | Discovery scoring penalizes coins with high maintenance_rate (>10%) | Decision D-15 defers scoring to display-only; `SpotArbOpportunity` struct at discovery.go:20 needs `MaintenanceRate float64` field; dashboard display requires i18n keys and table column |
</phase_requirements>

## Standard Stack

### Core
No new libraries are needed. This phase uses the existing Go standard library, exchange adapters, and the established project patterns.

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `testing` | 1.26 | Unit tests for new risk gates and triggers | Already used throughout project |
| `github.com/alicebob/miniredis/v2` | 2.37.0 | In-memory Redis for test fixtures | Already used in spotengine and risk test suites |
| `github.com/redis/go-redis/v9` | 9.18.0 | State persistence (contract info cache) | Already used as sole persistence layer |

[VERIFIED: go.mod in repo]

### Supporting
No new supporting libraries needed. All work is within existing packages.

## Architecture Patterns

### Recommended Project Structure (changes only)

```
pkg/exchange/
  types.go            # Add MaintenanceRate to ContractInfo
  exchange.go         # No changes needed (LoadAllContracts already in interface)
  binance/adapter.go  # Add leverageBracket parsing + GetMaintenanceRate
  bybit/adapter.go    # Add risk-limit parsing + GetMaintenanceRate
  gateio/adapter.go   # Parse maintenance_rate from existing contract endpoint
  okx/adapter.go      # Add position-tiers parsing + GetMaintenanceRate
  bitget/adapter.go   # Add query-position-lever parsing + GetMaintenanceRate

internal/spotengine/
  risk_gate.go        # Insert check 6 (maintenance_rate), shift dry-run to 7
  exit_manager.go     # Insert liq distance trigger in Phase 1 safety section
  discovery.go        # Add MaintenanceRate field to SpotArbOpportunity
  monitor.go          # No structural change (uses checkExitTriggers which gains new trigger)

internal/risk/
  health.go           # Add GetActiveSpotPositions() call in checkAll()

internal/config/
  config.go           # Add EnableMaintenanceRateGate, related threshold fields

web/src/
  pages/Positions.tsx  # Display maintenance_rate in spot opportunities table
  i18n/en.ts          # New translation keys for maintenance rate display
  i18n/zh-TW.ts       # New translation keys for maintenance rate display
```

### Pattern 1: Per-Exchange Maintenance Rate Data Source

**What:** Each exchange provides maintenance margin rate data through different endpoints and formats.
**When to use:** Whenever loading contract specifications or evaluating position risk.

[VERIFIED: Exchange API docs in doc/ directory]

| Exchange | Endpoint | Auth | Field | Format | Tier Granularity |
|----------|----------|------|-------|--------|------------------|
| Gate.io | `GET /futures/usdt/contracts` | No | `maintenance_rate` | Decimal string ("0.005" = 0.5%) | Single rate per contract (no tiers in contract endpoint) |
| Gate.io | `GET /futures/usdt/risk_limit_tiers` | No | `maintenance_rate` per tier | Decimal string per tier | Tiered by `risk_limit` (notional USDT) |
| Bybit | `GET /v5/market/risk-limit` | No | `maintenanceMargin` | Percentage string ("0.5" = 0.5%) | Tiered by `riskLimitValue`, paginated (15 symbols/page, or use `symbol` param) |
| OKX | `GET /api/v5/public/position-tiers` | No | `mmr` | Decimal string ("0.03" = 3%) | Tiered by `maxSz` (contract count), requires `instFamily` + `tdMode=cross` |
| Bitget | `GET /api/v2/mix/market/query-position-lever` | No | `keepMarginRate` | Decimal string ("0.004" = 0.4%) | Tiered by `startUnit`/`endUnit` (notional USDT) |
| Binance | `GET /fapi/v1/leverageBracket` | **Yes** (USER_DATA) | `maintMarginRatio` | Decimal number (0.0065 = 0.65%) | Tiered by `notionalFloor`/`notionalCap` (USDT) |

**Critical normalization:**
- Gate.io: `maintenance_rate` is already a decimal (0.005 = 0.5%). Use directly.
- Bybit: `maintenanceMargin` is a PERCENTAGE string ("0.5" = 0.5%). Divide by 100 to get decimal.
- OKX: `mmr` is a decimal string ("0.03" = 3%). Parse to float, use directly.
- Bitget: `keepMarginRate` is a decimal string ("0.004" = 0.4%). Parse to float, use directly.
- Binance: `maintMarginRatio` is a decimal number (0.0065 = 0.65%). Use directly. **Auth required.**

### Pattern 2: Two-Level Maintenance Rate Strategy

**What:** Use `ContractInfo.MaintenanceRate` (tier-1 baseline) for discovery display, and a runtime `GetMaintenanceRate(symbol, notional)` for accurate risk calculations.
**When to use:** Always. Discovery needs a quick baseline; pre-entry and runtime need size-accurate rates.

```
Discovery flow (fast, bulk):
  LoadAllContracts() → ContractInfo.MaintenanceRate = tier-1 rate
  Used for: dashboard display, opportunity filtering (deferred)

Pre-entry + Runtime flow (accurate, per-symbol):
  GetMaintenanceRate(symbol, notional) → exact tier rate for position size
  Used for: survivable-drop check, liquidation distance calculation
```

**Rationale:** Loading all tier data for all symbols at startup is expensive (especially Bybit's paginated endpoint). Tier-1 rates from `LoadAllContracts()` are sufficient for display purposes. For actual risk decisions, we query the specific symbol's tiers at entry time and cache them.

### Pattern 3: Survivable Drop Formula (Pre-Entry)

**What:** Calculate the maximum price drop a position can survive before liquidation.
**When to use:** Pre-entry risk gate (check 6).

Per D-02, the formula uses a leverage-scaled threshold:

```go
// entryThreshold = 90% / leverage
// For 3x leverage: 90% / 3 = 30%
entryThreshold := 0.90 / float64(leverage)

// Estimated liquidation distance from maintenance_rate:
// For an isolated position at X leverage:
//   liq_distance ~= (1/leverage) - maintenance_rate
// This represents how far price can move against you before
// maintenance margin is breached.
survivableDrop := (1.0 / float64(leverage)) - maintenanceRate

// Reject if survivable drop is less than threshold
if survivableDrop < entryThreshold {
    return RiskGateResult{
        Allowed: false,
        Reason:  fmt.Sprintf("maintenance_survivable_%.1f%%<%.1f%%",
            survivableDrop*100, entryThreshold*100),
    }
}
```

**Key insight:** The `1/leverage` part is the initial margin (e.g., 33% at 3x). The `maintenance_rate` eats into that margin. A high maintenance_rate (like GUAUSDT at 30%) at 3x leverage leaves only `33% - 30% = 3%` survivable drop -- far below the 30% threshold.

### Pattern 4: Liquidation Distance Calculation (Runtime)

**What:** Calculate current distance to estimated liquidation price using mark price and maintenance_rate.
**When to use:** Runtime monitor (exit trigger in Phase 1 safety section).

```go
// Estimated liquidation price for cross-margin:
// Long:  liqPrice = entryPrice * (1 - (1/leverage) + maintenanceRate)
// Short: liqPrice = entryPrice * (1 + (1/leverage) - maintenanceRate)
// 
// Distance as fraction of mark price:
// distance = |markPrice - liqPrice| / markPrice

// Graduated response (per D-08):
// warnThreshold    = entryThreshold * 0.50
// exitThreshold    = entryThreshold * 0.20
// emergThreshold   = entryThreshold * 0.10
```

### Pattern 5: Health Monitor Spot-Futures Integration

**What:** Include spot-futures positions in the HealthMonitor's per-exchange health assessment.
**When to use:** Every 30-second health check cycle.

```go
// In checkAll():
perpPositions, _ := h.db.GetActivePositions()
spotPositions, _ := h.db.GetActiveSpotPositions()

for name := range h.exchanges {
    h.checkExchangeHealth(name, perpPositions, spotPositions)
}
```

The `HealthAction` struct already supports all needed types (`"close"`, `"reduce"`, `"transfer"`). The engine that consumes these actions needs to dispatch spot-futures close/reduce to the `SpotEngine` instead of the perp-perp `Engine`.

**Integration concern:** The HealthMonitor currently emits `HealthAction` with `[]*models.ArbitragePosition`. For spot-futures positions, it needs a way to carry `[]*models.SpotFuturesPosition`. Options:
1. Add `SpotPositions []*models.SpotFuturesPosition` field to `HealthAction`
2. Use separate action type prefix ("spot_close", "spot_reduce")
3. Both (cleanest)

Recommended: Add `SpotPositions` field + use existing action types with a check on which field is populated.

### Anti-Patterns to Avoid
- **Don't query risk-limit endpoints on every monitor tick:** Cache with TTL (maintenance tiers change infrequently, typically only during exchange schedule updates). 1-hour cache TTL is reasonable.
- **Don't assume all exchanges have the same tier structure:** Gate.io's base contract endpoint has a single maintenance_rate; the risk_limit_tiers endpoint has tiered rates. Other exchanges only have tiered endpoints.
- **Don't use Binance's `maintMarginPercent` from exchangeInfo:** Per D-17, this field is deprecated/unreliable. Use the authenticated `leverageBracket` endpoint instead.
- **Don't add maintenance_rate to the `Exchange` interface as a required method:** Only 5 of 6 exchanges support spot-futures. BingX doesn't implement `SpotMarginExchange` and shouldn't need maintenance rate lookups. Keep it adapter-specific or add an optional interface.
- **Don't block on tiered data loading during startup:** Load tier-1 rates in `LoadAllContracts()` synchronously. Load full tier data lazily on first access or in background.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Maintenance rate normalization | Custom per-format parser for each response | Single `normalizeMaintenanceRate()` per adapter with unit test | Each exchange has a subtly different format (percentage string vs decimal string vs number); wrong normalization silently produces 100x wrong values |
| Tier matching | Linear scan through unsorted tiers | Sort tiers by boundary, binary search or iterate with early exit | Tier lists are small (5-10 tiers), but correctness is critical -- the right tier MUST be selected |
| Liquidation price estimation | Complex isolated-margin model | Simplified cross-margin formula (conservative estimate) | Cross-margin is used by all 5 exchanges in this system; isolated-margin formula would underestimate risk for cross-margin positions |

## Common Pitfalls

### Pitfall 1: Bybit Maintenance Margin is a Percentage, Not Decimal
**What goes wrong:** Parsing "0.5" as a decimal (0.5 = 50%) instead of a percentage (0.5% = 0.005 as decimal). This would make the maintenance rate appear 100x too high, rejecting all Bybit entries.
**Why it happens:** Other exchanges (Gate.io, OKX, Bitget, Binance) return decimal fractions where "0.005" means 0.5%. Bybit returns "0.5" meaning 0.5%.
**How to avoid:** Each adapter normalizes to decimal in its own parsing code. Unit test with known values for each exchange.
**Warning signs:** All Bybit entries rejected as "maintenance_survivable" with extremely small survivable drops.

### Pitfall 2: Bybit Risk-Limit Pagination
**What goes wrong:** Only getting first 15 symbols when querying Bybit's `/v5/market/risk-limit` without pagination, missing maintenance rates for many symbols.
**Why it happens:** Bybit returns 15 symbols per page for `category=linear`. Must use `cursor` for pagination.
**How to avoid:** For bulk loading, paginate. For per-symbol queries, pass `symbol` parameter to get specific symbol tiers directly (no pagination needed).
**Warning signs:** Maintenance rate data missing for symbols after the first 15 alphabetically.

### Pitfall 3: Binance leverageBracket Requires Authentication
**What goes wrong:** Calling `/fapi/v1/leverageBracket` without authentication returns a 401 error, failing silently in the adapter.
**Why it happens:** This is a `USER_DATA` endpoint requiring `timestamp` and `signature`. Unlike the other 4 exchanges whose risk-limit endpoints are public.
**How to avoid:** Use the adapter's existing authenticated `client.Get()` method (which adds auth headers). The Binance client already handles signing.
**Warning signs:** Binance maintenance rates always returning 0 or error.

### Pitfall 4: OKX Position-Tiers Requires instFamily, Not instId
**What goes wrong:** Passing `instId=BTC-USDT-SWAP` to position-tiers endpoint fails or returns empty.
**Why it happens:** For SWAP type, the endpoint requires `instFamily=BTC-USDT` (the underlying family), not the specific instrument ID. When `instType=SWAP`, `instId` is ignored per docs.
**How to avoid:** Extract instFamily from instId by removing the `-SWAP` suffix: `BTC-USDT-SWAP` -> `BTC-USDT`.
**Warning signs:** Empty tier data for OKX symbols.

### Pitfall 5: Mixing Up Position Size Units Across Exchanges
**What goes wrong:** Using base-asset units when the tier boundaries expect notional USDT, or vice versa.
**Why it happens:** Gate.io tiers use `risk_limit` in USDT. Bitget tiers use `startUnit`/`endUnit` in USDT. Bybit tiers use `riskLimitValue` (USDT). OKX tiers use `maxSz` in contract counts. Binance uses `notionalFloor`/`notionalCap` in USDT.
**How to avoid:** Convert the planned position size to each exchange's expected unit before tier matching. For USDT-denominated tiers, multiply base-asset size by current price. For OKX contract counts, divide by `ctVal`.
**Warning signs:** Wrong tier selected, leading to either overly permissive or overly restrictive maintenance rates.

### Pitfall 6: Health Monitor Action Dispatch to Wrong Engine
**What goes wrong:** Health monitor emits a "close" action for a spot-futures position, but the perp-perp engine tries to process it and fails because it can't find the position.
**Why it happens:** Currently `HealthAction.Positions` only contains `ArbitragePosition` objects. The engine handler doesn't distinguish spot-futures from perp-perp.
**How to avoid:** Add `SpotPositions []*models.SpotFuturesPosition` to `HealthAction`. The consumer checks which field is populated and dispatches to the correct engine.
**Warning signs:** Health actions logged as "position not found" when spot-futures positions are under stress.

### Pitfall 7: Cross-Margin vs Isolated-Margin Liquidation Formula
**What goes wrong:** Using isolated-margin liquidation formula when the position is on cross-margin, underestimating the actual survivable distance.
**Why it happens:** Cross-margin uses the entire account balance as collateral, not just the position's initial margin. The CONTEXT.md formula is simplified.
**How to avoid:** Use a conservative estimate (isolated-margin formula) which overestimates risk. This is safer -- it triggers exits earlier than strictly necessary for cross-margin, which is the correct bias for a safety system.
**Warning signs:** Unnecessary early exits on cross-margin exchanges. This is an acceptable trade-off for safety.

## Code Examples

### Adding MaintenanceRate to ContractInfo
```go
// Source: pkg/exchange/types.go
type ContractInfo struct {
    Symbol        string
    MinSize       float64
    StepSize      float64
    MaxSize       float64
    SizeDecimals  int
    PriceStep     float64
    PriceDecimals int
    MaintenanceRate float64 // tier-1 maintenance margin rate as decimal (0.005 = 0.5%)
}
```

### Gate.io: Parsing maintenance_rate from Existing Endpoint
```go
// Source: verified against doc/gate/gate-perpetual-futures-api-docs.md line 85
// Gate.io already returns maintenance_rate in the contract list response.
// Just add the field to the existing struct in LoadAllContracts():
var contracts []struct {
    Name             string      `json:"name"`
    QuantoMultiplier string      `json:"quanto_multiplier"`
    OrderSizeMin     json.Number `json:"order_size_min"`
    OrderSizeMax     json.Number `json:"order_size_max"`
    OrderPriceRound  string      `json:"order_price_round"`
    InTrade          bool        `json:"in_delisting"`
    MaintenanceRate  string      `json:"maintenance_rate"` // <-- ADD THIS
}
// Then parse: maintenanceRate, _ := strconv.ParseFloat(c.MaintenanceRate, 64)
// Gate.io format: "0.005" = 0.5% (already decimal)
```

### Bybit: Risk-Limit Endpoint for Per-Symbol Tiers
```go
// Source: verified against doc/bybit/bybit-market-api-docs.md line 3224
// GET /v5/market/risk-limit?category=linear&symbol=BTCUSDT
// CRITICAL: maintenanceMargin is a PERCENTAGE ("0.5" = 0.5%), not decimal!
// Must divide by 100 to normalize.
var resp struct {
    List []struct {
        ID                int    `json:"id"`
        Symbol            string `json:"symbol"`
        RiskLimitValue    string `json:"riskLimitValue"`    // notional USDT cap
        MaintenanceMargin string `json:"maintenanceMargin"` // percentage (0.5 = 0.5%)
        IsLowestRisk      int    `json:"isLowestRisk"`
    } `json:"list"`
}
// Normalize: maintenanceRate = parsedValue / 100.0
```

### Binance: Authenticated leverageBracket
```go
// Source: verified against doc/binance/binance-usds-futures-api-docs.md line 1649
// GET /fapi/v1/leverageBracket?symbol=ETHUSDT (requires auth: timestamp + signature)
var resp struct {
    Symbol   string `json:"symbol"`
    Brackets []struct {
        Bracket           int     `json:"bracket"`
        NotionalCap       float64 `json:"notionalCap"`
        NotionalFloor     float64 `json:"notionalFloor"`
        MaintMarginRatio  float64 `json:"maintMarginRatio"` // decimal (0.0065 = 0.65%)
        Cum               float64 `json:"cum"`
    } `json:"brackets"`
}
// Already decimal: use directly
```

### Pre-Entry Risk Gate Check
```go
// Insert as check 6 in checkRiskGate(), before dry-run (check 7):
// Pattern follows existing numbered check convention.

// 6. Maintenance rate: reject if survivable drop < leverage-scaled threshold.
if e.cfg.SpotFuturesEnableMaintenanceGate {
    maintenanceRate := e.getMaintenanceRate(symbol, exchName, plannedNotional)
    if maintenanceRate > 0 {
        leverage := float64(e.cfg.SpotFuturesLeverage)
        if leverage <= 0 { leverage = 3.0 }
        survivableDrop := (1.0 / leverage) - maintenanceRate
        entryThreshold := 0.90 / leverage
        if survivableDrop < entryThreshold {
            return RiskGateResult{
                Allowed: false,
                Reason: fmt.Sprintf("maintenance_survivable_%.1f%%<%.1f%%",
                    survivableDrop*100, entryThreshold*100),
            }
        }
    }
}
```

### Exit Trigger: Liquidation Distance
```go
// Insert in checkExitTriggers() Phase 1 safety section, after trigger 2 (margin health):
//
// 2b. Maintenance-rate-aware liquidation distance
if e.cfg.SpotFuturesEnableMaintenanceGate {
    maintenanceRate := e.getMaintenanceRate(pos.Symbol, pos.Exchange, pos.NotionalUSDT)
    if maintenanceRate > 0 && markPrice > 0 && pos.FuturesEntry > 0 {
        leverage := float64(e.cfg.SpotFuturesLeverage)
        if leverage <= 0 { leverage = 3.0 }

        var estLiqPrice float64
        if pos.FuturesSide == "long" {
            estLiqPrice = pos.FuturesEntry * (1 - (1/leverage) + maintenanceRate)
        } else {
            estLiqPrice = pos.FuturesEntry * (1 + (1/leverage) - maintenanceRate)
        }

        distance := math.Abs(markPrice - estLiqPrice) / markPrice
        entryThreshold := 0.90 / leverage

        emergThreshold := entryThreshold * 0.10
        exitThreshold  := entryThreshold * 0.20
        warnThreshold  := entryThreshold * 0.50

        if distance < emergThreshold {
            return "liq_distance_emergency", true
        }
        if distance < exitThreshold {
            return "liq_distance_exit", false
        }
        if distance < warnThreshold {
            e.log.Warn("exit trigger: %s liq distance %.1f%% < warn %.1f%%",
                pos.Symbol, distance*100, warnThreshold*100)
            // Warn only -- don't exit yet
        }
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Fixed price-move % exit (20%/30%) | Maintenance-rate-aware liquidation distance | This phase | Catches high-maintenance-rate contracts that the fixed % misses |
| Perp-perp only health monitor | Unified health monitor across both engines | This phase | L3/L4/L5 actions protect spot-futures positions too |
| No maintenance_rate in discovery | maintenance_rate displayed in opportunities | This phase | User can observe real maintenance rates before enabling scoring |

**Deprecated/outdated:**
- Binance `maintMarginPercent` in exchangeInfo: Deprecated/unreliable per D-17. Use authenticated `leverageBracket` endpoint instead. [VERIFIED: Binance docs show leverageBracket as the current recommended endpoint]

## Assumptions Log

> List all claims tagged [ASSUMED] in this research.

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Bybit risk-limit `maintenanceMargin` is a percentage string (0.5 = 0.5%), not decimal | Architecture Patterns, Pitfall 1 | 100x error in maintenance rate for all Bybit symbols -- would reject all or accept all incorrectly. Live verification against a known symbol (BTCUSDT with expected ~0.5% maintenance) is essential during implementation. |
| A2 | OKX position-tiers endpoint ignores `instId` when `instType=SWAP`, requiring `instFamily` instead | Pitfall 4 | Empty tier data for OKX, falling back to 0 maintenance rate (unsafe permissive) |
| A3 | Cross-margin liquidation formula (conservative isolated-margin estimate) is adequate for safety | Pattern 4, Pitfall 7 | May trigger unnecessary early exits on cross-margin exchanges, but this is the safe direction |
| A4 | 1-hour cache TTL is appropriate for maintenance rate tiers | Architecture Patterns | If exchange changes tiers more frequently (unlikely), stale data could be used for ~1 hour |

**Verification plan for A1:** During adapter implementation, query Bybit `GET /v5/market/risk-limit?category=linear&symbol=BTCUSDT` and verify that BTCUSDT's lowest-tier `maintenanceMargin` value is around "0.5" (meaning 0.5%). If it returns "0.005", the format is decimal like others.

## Open Questions (RESOLVED)

1. **Health monitor action dispatch for spot-futures positions** — RESOLVED: Add `SpotPositions []*models.SpotFuturesPosition` field to `HealthAction`. Engine's `consumeHealthActions()` checks which field is populated and dispatches to SpotEngine via `SetSpotCloseCallback` (Plan 03 Task 2).

2. **Manual open bypass mechanism** — RESOLVED: `checkRiskGate()` is auto-entry only; manual opens already bypass it. A separate `checkMaintenanceRateWarning()` in the manual open path returns a warning string without blocking (Plan 02 Task 1).

3. **Planned notional size for pre-entry tier matching** — RESOLVED: Use `capitalPerLeg * leverage` as planned notional. Conservative bias is acceptable since maintenance rates increase with position size (Plan 02 Task 1).

## Environment Availability

Step 2.6: SKIPPED (no external dependencies identified). This phase is purely code/config changes to the existing Go codebase and React frontend. All 5 exchange APIs are already integrated.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `github.com/alicebob/miniredis/v2` |
| Config file | None -- uses Go conventions (`*_test.go` co-located) |
| Quick run command | `go test ./internal/spotengine/ -run TestMaintenance -count=1 -v` |
| Full suite command | `go test ./internal/spotengine/ ./internal/risk/ ./pkg/exchange/... -count=1 -v` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SF-RISK-01 | Pre-entry rejects when survivable drop < threshold | unit | `go test ./internal/spotengine/ -run TestMaintenanceRateGate -count=1 -v` | Wave 0 |
| SF-RISK-01 | Pre-entry allows when survivable drop >= threshold | unit | `go test ./internal/spotengine/ -run TestMaintenanceRateGate -count=1 -v` | Wave 0 |
| SF-RISK-01 | Manual open bypasses gate with warning | unit | `go test ./internal/spotengine/ -run TestMaintenanceManualBypass -count=1 -v` | Wave 0 |
| SF-RISK-02 | Liq distance emergency triggers parallel close | unit | `go test ./internal/spotengine/ -run TestLiqDistanceEmergency -count=1 -v` | Wave 0 |
| SF-RISK-02 | Liq distance exit triggers normal close | unit | `go test ./internal/spotengine/ -run TestLiqDistanceExit -count=1 -v` | Wave 0 |
| SF-RISK-02 | Liq distance warn logs but does not exit | unit | `go test ./internal/spotengine/ -run TestLiqDistanceWarn -count=1 -v` | Wave 0 |
| SF-RISK-03 | Health monitor includes spot positions in checkAll | unit | `go test ./internal/risk/ -run TestHealthMonitorSpotPositions -count=1 -v` | Wave 0 |
| SF-RISK-04 | Discovery includes MaintenanceRate in opportunity | unit | `go test ./internal/spotengine/ -run TestDiscoveryMaintenanceRate -count=1 -v` | Wave 0 |
| All | Adapter maintenance_rate normalization per exchange | unit | `go test ./pkg/exchange/... -run TestMaintenanceRate -count=1 -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/spotengine/ ./internal/risk/ -count=1 -v`
- **Per wave merge:** `go test ./internal/spotengine/ ./internal/risk/ ./pkg/exchange/... -count=1 -v`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/spotengine/maintenance_rate_test.go` -- covers SF-RISK-01 (risk gate) and SF-RISK-02 (exit trigger) tests
- [ ] `internal/risk/health_spot_test.go` -- covers SF-RISK-03 (health monitor spot integration)
- [ ] Per-adapter maintenance rate parsing tests (add to existing `adapter_test.go` files or new files)

*(Existing test infrastructure covers framework needs -- no new framework install required)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | N/A (no new auth surfaces) |
| V3 Session Management | No | N/A |
| V4 Access Control | No | N/A (uses existing dashboard auth) |
| V5 Input Validation | Yes | Validate API response parsing: bounds-check maintenance rates (0 < rate < 1.0), reject negative or >100% values |
| V6 Cryptography | No | N/A |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Exchange returns manipulated maintenance rate | Tampering | Bounds check (0 < rate < 1.0); log anomalies; cache prevents rapid flip-flop |
| Stale cached maintenance rate used during exchange tier change | Information Disclosure | 1-hour TTL cache; force refresh on position entry; log cache age |
| API error causes maintenance rate = 0, bypassing safety gate | Elevation of Privilege | Treat maintenanceRate=0 as "unknown" and apply conservative default (e.g., 0.05 = 5%); never silently pass |

## Sources

### Primary (HIGH confidence)
- `doc/gate/gate-perpetual-futures-api-docs.md` -- Gate.io contract info and risk_limit_tiers endpoints, response format, `maintenance_rate` field [VERIFIED]
- `doc/bybit/bybit-market-api-docs.md` -- Bybit risk-limit endpoint, pagination, `maintenanceMargin` percentage format [VERIFIED]
- `doc/okx/okx-public-data-api-docs.md` -- OKX position-tiers endpoint, `mmr` field, `instFamily` requirement [VERIFIED]
- `doc/bitget/bitget-futures-api-docs.md` -- Bitget query-position-lever endpoint, `keepMarginRate` field [VERIFIED]
- `doc/binance/binance-usds-futures-api-docs.md` -- Binance leverageBracket endpoint, `maintMarginRatio`, authentication requirement [VERIFIED]
- Codebase: `pkg/exchange/types.go`, `pkg/exchange/exchange.go`, all 5 adapter `LoadAllContracts()` implementations [VERIFIED]
- Codebase: `internal/spotengine/risk_gate.go`, `exit_manager.go`, `discovery.go`, `monitor.go` [VERIFIED]
- Codebase: `internal/risk/health.go`, `internal/risk/monitor.go` [VERIFIED]
- Codebase: `internal/config/config.go` (jsonSpotFutures struct pattern) [VERIFIED]

### Secondary (MEDIUM confidence)
- `.firecrawl/gate-risk-control.md` -- Gate.io unified account risk control mechanics (scraped 2025-12-11) [CITED]
- `doc/DESIGN_SPOT_FUTURES_RISK.md` -- Original risk design spec, tier structure reference [VERIFIED: project doc]

### Tertiary (LOW confidence)
- None. All claims verified against exchange API docs and codebase.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new libraries, all existing packages
- Architecture: HIGH -- all exchange API formats verified against local docs; all code insertion points inspected
- Pitfalls: HIGH -- each pitfall derived from verified API doc discrepancies (e.g., Bybit percentage format)
- Security: MEDIUM -- threat model is straightforward but "exchange returns manipulated data" scenarios are theoretical

**Research date:** 2026-04-05
**Valid until:** 2026-05-05 (exchange APIs are stable; maintenance rate endpoints rarely change)
