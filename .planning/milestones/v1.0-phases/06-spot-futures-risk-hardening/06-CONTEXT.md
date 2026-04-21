# Phase 6: Spot-Futures Risk Hardening - Context

**Gathered:** 2026-04-05
**Status:** Ready for planning

<domain>
## Phase Boundary

Prevent spot-futures positions that are immediately overleveraged relative to the contract's maintenance requirements, detect and act on liquidation proximity at runtime, and extend the perp-perp health monitor to include spot-futures positions. Covers all 5 margin exchanges (Binance, Bybit, Gate.io, Bitget, OKX).

</domain>

<decisions>
## Implementation Decisions

### Pre-Entry Rejection (SF-RISK-01)
- **D-01:** Add maintenance_rate-aware survivable-drop check as a new gate in `checkRiskGate()` (after existing check 5 price gap, before dry-run check 6).
- **D-02:** Leverage-scaled threshold formula: `min_survivable_drop = 90% / leverage`. Concrete defaults: 2x→45%, 3x→30%, 4x→22.5%, 5x→18%.
- **D-03:** For auto-entry: hard reject with logged reason (e.g. `maintenance_survivable_4.5%<30%`). Follows existing `filterStatus` / `RiskGateResult.Reason` pattern.
- **D-04:** For manual open (dashboard): allow bypass. Add a warning in the response but do not block. User accepts the risk explicitly.
- **D-05:** Same threshold for all exchanges — no unified vs separate-account distinction for pre-entry.

### Runtime Liquidation Monitor (SF-RISK-02)
- **D-06:** Enhance spotengine's existing exit trigger #2 (margin health) with a new maintenance_rate-aware liquidation distance calculation. This is an additional trigger in `checkExitTriggers()`, not a replacement.
- **D-07:** Liquidation distance = `|mark_price - estimated_liq_price| / mark_price`, where estimated_liq_price uses the contract's per-symbol `maintenance_rate`.
- **D-08:** Graduated response linked to the pre-entry threshold via configurable fractions:
  - Warn at 50% of entry threshold (log + dashboard alert)
  - Exit at 20% of entry threshold (normal depth-fill exit)
  - Emergency at 10% of entry threshold (market IOC, parallel close)
- **D-09:** Concrete examples at 3x leverage (entry threshold 30%): warn at 15% distance, exit at 6%, emergency at 3%.

### Health Monitor Integration (SF-RISK-03)
- **D-10:** Add `GetActiveSpotPositions()` to the existing perp-perp `HealthMonitor.checkAll()` so spot-futures positions are visible as candidates for L4 (reduce) and L5 (emergency close) actions.
- **D-11:** Same L3/L4/L5 tiers and thresholds — no separate tier system for spot-futures. The purpose is to let the health monitor pick the best position to close across both engines when an exchange is under pressure.
- **D-12:** L3 transfer behavior: same as perp-perp (transfer USDT from healthiest exchange). No separate-account futures→margin transfer for now.

### Discovery Scoring (SF-RISK-04)
- **D-13:** Fetch `maintenance_rate` per contract during discovery and add it to `SpotArbOpportunity` struct as a display field.
- **D-14:** Display maintenance_rate in the dashboard opportunities table so the user can observe real values.
- **D-15:** No filter or scoring penalty on maintenance_rate for now. Defer scoring/filtering decisions until after live observation with the displayed data.

### Maintenance Rate Data Source
- **D-16:** Add `MaintenanceRate float64` to `ContractInfo` struct in `pkg/exchange/types.go`. Populate it in each adapter's `LoadAllContracts()` implementation.
- **D-17:** Per-exchange API sources (researcher to verify exact endpoints/fields):
  - Gate.io: `GET /futures/usdt/contracts` → `maintenance_rate` (direct in contract info)
  - Bybit: `GET /v5/market/risk-limit` → `maintenanceMargin` (separate endpoint, tiered)
  - OKX: `GET /api/v5/public/position-tiers` → `mmr` (separate endpoint, tiered)
  - Bitget: `GET /api/v2/mix/market/query-position-lever` → `keepMarginRate` (separate endpoint, tiered)
  - Binance: tier bracket endpoint (deprecated `maintMarginPercent` in exchangeInfo unreliable)
- **D-18:** For tiered maintenance rates, use the tier matching the position's notional size. For pre-entry, use the tier matching the planned position size.

### Config Convention
- **D-19:** All new risk features follow project convention: `Enable{Feature}` bool (default OFF) + JSON tag + dashboard toggle. Matches `feedback_risk_configurable_switch.md`.
- **D-20:** Config fields go in the existing `spot_futures` JSON section, not a new top-level section.

### Claude's Discretion
- Specific implementation of tiered maintenance_rate lookup (cache strategy, refresh interval)
- Dashboard UI layout for displaying maintenance_rate in opportunities table
- Exact config field naming within the established convention
- Plan decomposition and wave ordering

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Risk Design Spec
- `doc/DESIGN_SPOT_FUTURES_RISK.md` — Extreme condition risk design: tiers 0-5, emergency exit procedures, per-exchange account type risk differences

### Spot Engine (runtime monitor + exit triggers)
- `internal/spotengine/risk_gate.go` — Pre-entry risk gate, 6 checks (maintenance_rate check inserts here as check 6, dry-run becomes check 7)
- `internal/spotengine/exit_manager.go` — `checkExitTriggers()` 7-trigger priority system, trigger #2 is margin health (enhance with maintenance_rate-aware liq distance)
- `internal/spotengine/monitor.go` — `monitorLoop()`, `monitorTick()`, `monitorPosition()` — runtime monitoring loop
- `internal/spotengine/discovery.go` — `SpotArbOpportunity` struct (add MaintenanceRate field), `runNativeDiscovery()`, scoring/sorting

### Perp-Perp Health Monitor
- `internal/risk/health.go` — `HealthMonitor`, `checkAll()` (add `GetActiveSpotPositions()`), L3/L4/L5 tiers, `HealthAction` types
- `internal/risk/monitor.go` — `checkLiquidationDistance()` (reference pattern for maintenance_rate-aware calculation)

### Exchange Interface + Contract Info
- `pkg/exchange/types.go` — `ContractInfo` struct (add `MaintenanceRate float64`), `SpotMarginExchange` interface
- `pkg/exchange/exchange.go` — `Exchange` interface, `LoadAllContracts()` method

### Per-Exchange Adapters (maintenance_rate data sources)
- `pkg/exchange/gateio/adapter.go` — Gate.io contract info (maintenance_rate already in API response)
- `pkg/exchange/bybit/adapter.go` — Bybit adapter (needs risk-limit endpoint)
- `pkg/exchange/okx/adapter.go` — OKX adapter (needs position-tiers endpoint)
- `pkg/exchange/bitget/adapter.go` — Bitget adapter (needs position-lever endpoint)
- `pkg/exchange/binance/adapter.go` — Binance adapter (needs tier bracket endpoint)

### Config
- `internal/config/config.go` — `SpotFutures*` fields (lines 176-201), 6-touch-point config pattern

### Models + Database
- `internal/models/spot_position.go` — `SpotFuturesPosition` model
- `internal/database/spot_state.go` — `GetActiveSpotPositions()`, spot-futures position CRUD

### Exchange API Docs (for adapter implementation)
- `doc/gate/gate-perpetual-futures-api-docs.md` — Gate.io maintenance_rate in contract info
- `doc/bybit/bybit-market-api-docs.md` — Bybit risk-limit endpoint
- `doc/okx/okx-public-data-api-docs.md` — OKX position-tiers endpoint
- `doc/bitget/bitget-futures-api-docs.md` — Bitget position-lever endpoint
- `doc/binance/binance-usds-futures-api-docs.md` — Binance tier bracket endpoint

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`checkRiskGate()`** (risk_gate.go): 6-check pre-entry gate — new maintenance_rate check inserts as check 6
- **`checkExitTriggers()`** (exit_manager.go): 7-trigger priority system — new liq distance trigger inserts in Phase 1 safety triggers
- **`checkLiquidationDistance()`** (risk/monitor.go): perp-perp liq distance pattern — estimates liq price from leverage, alerts at 15%/10% distance. Reference implementation for the maintenance_rate-aware version
- **`LoadAllContracts()`** (exchange.go): already in Exchange interface, all 5 adapters implement it — natural place to add MaintenanceRate
- **`ContractInfo`** (types.go): currently has Symbol, MinSize, StepSize, MaxSize, SizeDecimals, PriceStep, PriceDecimals — no maintenance rate yet
- **`HealthMonitor.checkAll()`** (health.go): calls `GetActivePositions()` only — add `GetActiveSpotPositions()` here

### Established Patterns
- Risk gate checks: numbered, ordered cheap→expensive, return `RiskGateResult{Allowed, Reason}`
- Exit triggers: priority-ordered, Phase 1 safety triggers bypass all guards
- Config: `Enable{Feature}` bool default OFF + JSON tag + dashboard toggle
- `filterStatus` pattern in discovery: empty string = passed, non-empty = human-readable rejection reason

### Integration Points
- `risk_gate.go:77` — insert new maintenance_rate check before dry-run (check 6 → 7)
- `exit_manager.go:101` — insert new liq distance trigger in Phase 1 safety triggers section
- `health.go:153` — `checkAll()` add `GetActiveSpotPositions()` call
- `discovery.go:20` — `SpotArbOpportunity` struct add `MaintenanceRate float64` field
- `types.go:103` — `ContractInfo` struct add `MaintenanceRate float64` field
- `config.go` — new `SpotFutures*` fields for all thresholds and enable toggles

</code_context>

<specifics>
## Specific Ideas

- The GUAUSDT incident on Gate.io (2026-04-05) is the motivating case: 30% maintenance_rate on a low-cap coin, 3x leverage, only 10% buffer to liquidation. No risk system caught it.
- Pre-entry formula: `survivable_drop = f(equity, notional, maintenance_rate, leverage)` — reject if below `90% / leverage`
- Runtime fractions (50%/20%/10% of entry threshold) give graduated response that scales with leverage automatically
- Tiered maintenance rates (most exchanges): use tier matching planned or current position size
- Gate.io scraped risk control docs available at `.firecrawl/gate-risk-control.md` for reference

</specifics>

<deferred>
## Deferred Ideas

- **Discovery scoring/filtering by maintenance_rate** — display maintenance_rate first, decide on filter/penalty after observing real data in production
- **Separate-account futures→margin profit transfer** (Binance/Bitget L3 mitigation) — complex cross-account transfer, defer to future phase
- **Price monitor goroutine** (DESIGN_SPOT_FUTURES_RISK.md Tier 1) — 10-second price monitoring. Existing `checkExitTriggers` runs on monitor interval. Dedicated fast-poll goroutine is a separate enhancement.

</deferred>

---

*Phase: 06-spot-futures-risk-hardening*
*Context gathered: 2026-04-05*
