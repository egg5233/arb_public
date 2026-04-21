# Phase 5: Capital Allocation - Context

**Gathered:** 2026-04-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Unify capital management across both engines so the user deposits USDT once, selects a risk preference, and the system allocates across perp-perp and spot-futures on all exchanges automatically. Replaces the current manual per-engine capital configuration (CapitalPerLeg, SpotFuturesCapitalSeparate/Unified) with a single pool that dynamically distributes based on performance data and risk profile.

</domain>

<decisions>
## Implementation Decisions

### Risk Profile Definition
- **D-01:** Three static presets — Conservative, Balanced, Aggressive — each bundling: max positions, leverage, entry threshold (min BPS), allocation weights (perp vs spot split), and position sizing multiplier
- **D-02:** Presets are the starting point; each bundled parameter remains individually overridable in the dashboard Config page. Profile selection sets the values, user can then tweak any field
- **D-03:** Profile selection stored as a config field (`RiskProfile string` — "conservative", "balanced", "aggressive", "custom"). Selecting a named profile overwrites the bundled fields; changing any field after switches profile to "custom"
- **D-04:** Default profile: "balanced" — provides a safe middle-ground for new deployments

### Allocation Algorithm
- **D-05:** Performance-weighted allocation with configurable floor/ceiling. Base split comes from profile (e.g., balanced = 50/50), then tilted by trailing APR from Phase 4 analytics data
- **D-06:** Floor/ceiling per strategy (e.g., min 20%, max 80%) prevents runaway concentration even if one strategy massively outperforms
- **D-07:** Performance lookback window configurable (default 7 days) — uses `ComputeStrategySummary` from `internal/analytics/aggregator.go`
- **D-08:** When analytics data is insufficient (< 3 closed positions per strategy), fall back to profile's base split — no performance tilt applied

### Capital Pool Model
- **D-09:** Single total USDT pool configured as `TotalCapitalUSDT` in config. Replaces per-engine `CapitalPerLeg` and `SpotFuturesCapitalSeparate/Unified` as the primary capital source
- **D-10:** Percentage-based splits: `MaxPerpPerpPct` and `MaxSpotFuturesPct` (existing fields in allocator) become the ceilings, with effective allocation computed from profile + performance weighting
- **D-11:** Per-exchange ceiling via existing `MaxPerExchangePct` — no change needed, already in `CapitalAllocator`
- **D-12:** Position sizing: `CapitalPerLeg` becomes derived from `TotalCapitalUSDT / maxPositions / 2` (two legs per position) rather than a manual input. Manual override still available
- **D-13:** Backward compatibility: if `TotalCapitalUSDT` is 0 (unset), fall back to existing `CapitalPerLeg` behavior — no disruption for current deployments

### Dynamic Rebalancing
- **D-14:** Evaluate allocation on each scan cycle (existing scan-driven model). When discovery finds no opportunities for a strategy, its unused allocation becomes available to the other strategy for that cycle
- **D-15:** No cross-exchange fund transfers triggered by allocation — only available balance is considered. Transfers remain the responsibility of the rebalance scan (existing L3 handler)
- **D-16:** Dashboard shows current allocation state: pool total, per-strategy committed/available, per-exchange exposure — visible on Overview or a new Allocation section

### Claude's Discretion
- Config field naming, struct layout, and JSON keys
- Dashboard UI placement (new tab vs section in existing page)
- Exact performance weighting formula (linear interpolation, exponential decay, etc.)
- Redis key structure for allocation state

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Capital Allocation (existing code)
- `internal/risk/allocator.go` — Existing CapitalAllocator with Redis-backed reservations, strategy/exchange exposure tracking
- `internal/risk/allocator_test.go` — Tests for existing allocator
- `internal/engine/capital.go` — Engine-side capital helpers for perp-perp

### Analytics (Phase 4 outputs)
- `internal/analytics/aggregator.go` — ComputeStrategySummary, ComputeExchangeMetrics, CalculateAPR — performance data source for allocation weighting
- `internal/analytics/store.go` — SQLite PnL time-series store

### Config Pattern
- `internal/config/config.go` — 6-touch-point convention for new fields (see EnableLossLimits, EnableAnalytics as references)
- `internal/api/handlers.go` — buildConfigResponse and handlePostConfig patterns for dashboard config

### Risk Management
- `internal/risk/manager.go` — Pre-trade approval (11-point check) — allocation must integrate with existing risk gates
- `internal/risk/health.go` — Health monitoring tiers — allocation must respect margin health state

### Requirements
- `.planning/REQUIREMENTS.md` — CA-01 through CA-04

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `CapitalAllocator` (risk/allocator.go): Already implements Reserve/Commit/Release with Redis-backed state, strategy/exchange percentage limits. Phase 5 extends this rather than replacing it
- `ComputeStrategySummary` (analytics/aggregator.go): Returns per-strategy APR, PnL, trade count — direct input for performance-weighted allocation
- `MaxPerpPerpPct`, `MaxSpotFuturesPct`, `MaxPerExchangePct` config fields: Already wired end-to-end (config → allocator → dashboard). Phase 5 makes these dynamic based on profile + performance
- Config 6-touch-point pattern: Well-established for EnableAnalytics, EnableLossLimits — follow for new profile fields

### Established Patterns
- Scan-driven execution: Discovery fires at scheduled minutes, engine consumes opportunities. Allocation evaluation fits naturally into the scan cycle
- Config toggle gating: All new features default OFF with EnableX bool + dashboard toggle
- Interface-driven DI: Engine depends on `models.Discoverer`, `models.RiskChecker` — allocation may need a similar interface

### Integration Points
- `cmd/main.go`: Wire new allocation logic between CapitalAllocator creation and engine start
- `internal/engine/engine.go`: Entry execution reads CapitalPerLeg for position sizing — must read from allocation instead
- `internal/spotengine/engine.go`: Spot-futures uses SpotFuturesCapitalSeparate/Unified — must read from allocation
- `internal/api/handlers.go`: Dashboard config GET/POST for new profile fields
- `web/src/pages/Config.tsx`: Risk profile selector and allocation display

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches. The existing `CapitalAllocator` provides a strong foundation; Phase 5 primarily adds profile-driven parameterization and performance-weighted dynamic allocation on top.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 05-capital-allocation*
*Context gathered: 2026-04-04*
