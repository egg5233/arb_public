# Phase 5: Capital Allocation - Research

**Researched:** 2026-04-04
**Domain:** Cross-strategy capital management, risk profile parameterization, performance-weighted allocation
**Confidence:** HIGH

## Summary

Phase 5 transforms the arb bot from two independently-configured capital pools into a unified allocation system. The existing `CapitalAllocator` in `internal/risk/allocator.go` already provides Redis-backed reservation/commit/release mechanics with strategy and exchange percentage caps. Phase 5 extends this foundation by adding: (1) a `TotalCapitalUSDT` pool that derives per-leg sizing instead of manual `CapitalPerLeg`, (2) risk profile presets that bundle multiple config parameters, (3) performance-weighted dynamic allocation using Phase 4's `ComputeStrategySummary` APR data, and (4) per-scan-cycle capital shifting when one strategy has no opportunities.

The implementation involves seven main touch-points: config struct fields (6-touch-point pattern), a new `ProfileManager` or similar component that applies preset bundles, enhancement of the existing `CapitalAllocator.checkCaps` to use dynamic percentages instead of static config values, integration into both engines' entry paths, a new API endpoint for allocation status, dashboard UI for profile selection and allocation display, and backward-compatible fallback when `TotalCapitalUSDT` is unset.

**Primary recommendation:** Extend the existing `CapitalAllocator` with a `ComputeEffectiveAllocation` method that reads analytics data and profile config to produce dynamic strategy percentages, then replace static `strategyPct()` calls. Do NOT build a separate allocation engine -- the allocator already has the right Redis state model.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Three static presets -- Conservative, Balanced, Aggressive -- each bundling: max positions, leverage, entry threshold (min BPS), allocation weights (perp vs spot split), and position sizing multiplier
- **D-02:** Presets are the starting point; each bundled parameter remains individually overridable in the dashboard Config page. Profile selection sets the values, user can then tweak any field
- **D-03:** Profile selection stored as a config field (`RiskProfile string` -- "conservative", "balanced", "aggressive", "custom"). Selecting a named profile overwrites the bundled fields; changing any field after switches profile to "custom"
- **D-04:** Default profile: "balanced" -- provides a safe middle-ground for new deployments
- **D-05:** Performance-weighted allocation with configurable floor/ceiling. Base split comes from profile (e.g., balanced = 50/50), then tilted by trailing APR from Phase 4 analytics data
- **D-06:** Floor/ceiling per strategy (e.g., min 20%, max 80%) prevents runaway concentration even if one strategy massively outperforms
- **D-07:** Performance lookback window configurable (default 7 days) -- uses `ComputeStrategySummary` from `internal/analytics/aggregator.go`
- **D-08:** When analytics data is insufficient (< 3 closed positions per strategy), fall back to profile's base split -- no performance tilt applied
- **D-09:** Single total USDT pool configured as `TotalCapitalUSDT` in config. Replaces per-engine `CapitalPerLeg` and `SpotFuturesCapitalSeparate/Unified` as the primary capital source
- **D-10:** Percentage-based splits: `MaxPerpPerpPct` and `MaxSpotFuturesPct` (existing fields in allocator) become the ceilings, with effective allocation computed from profile + performance weighting
- **D-11:** Per-exchange ceiling via existing `MaxPerExchangePct` -- no change needed, already in `CapitalAllocator`
- **D-12:** Position sizing: `CapitalPerLeg` becomes derived from `TotalCapitalUSDT / maxPositions / 2` (two legs per position) rather than a manual input. Manual override still available
- **D-13:** Backward compatibility: if `TotalCapitalUSDT` is 0 (unset), fall back to existing `CapitalPerLeg` behavior -- no disruption for current deployments
- **D-14:** Evaluate allocation on each scan cycle (existing scan-driven model). When discovery finds no opportunities for a strategy, its unused allocation becomes available to the other strategy for that cycle
- **D-15:** No cross-exchange fund transfers triggered by allocation -- only available balance is considered. Transfers remain the responsibility of the rebalance scan (existing L3 handler)
- **D-16:** Dashboard shows current allocation state: pool total, per-strategy committed/available, per-exchange exposure -- visible on Overview or a new Allocation section

### Claude's Discretion
- Config field naming, struct layout, and JSON keys
- Dashboard UI placement (new tab vs section in existing page)
- Exact performance weighting formula (linear interpolation, exponential decay, etc.)
- Redis key structure for allocation state

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CA-01 | Unified capital pool -- single deposit, system allocates across both strategies and all exchanges | Existing `CapitalAllocator` has Reserve/Commit/Release with Redis-backed strategy+exchange tracking. Add `TotalCapitalUSDT` as pool source, derive `CapitalPerLeg` from it. Backward-compatible when unset (D-13). |
| CA-02 | Risk preference profiles (conservative/balanced/aggressive) that bundle position size, max positions, entry thresholds, and allocation weights | New `RiskProfile` config field + profile preset map. Applies bundled values to existing config fields (`MaxPositions`, `Leverage`, `MaxCostRatio`, `MaxPerpPerpPct`, `MaxSpotFuturesPct`, etc.). Switches to "custom" on manual override (D-03). |
| CA-03 | Strategy-weighted allocation dynamically adjusts perp-perp vs spot-futures split based on recent performance metrics | `ComputeStrategySummary` returns per-strategy APR. New `ComputeEffectiveAllocation` method tilts base split by trailing APR ratio. Minimum 3 trades threshold (D-08). Floor/ceiling enforced (D-06). |
| CA-04 | Dynamic rebalancing shifts capital to available strategies when one strategy has no opportunities | Allocation evaluated per scan cycle (D-14). When discovery returns 0 opportunities for a strategy, its unused (uncommitted) share becomes available to the other. No fund transfers (D-15). |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **npm lockdown**: No `npm install` -- only `npm ci` for frontend deps. No new npm packages.
- **Delegation mode**: Coordinator delegates subtasks for 2+ file changes.
- **Build order**: Frontend BEFORE Go binary (`make build`).
- **Live system**: Changes must not break existing perp-perp trading.
- **Config toggle gating**: New features default OFF with `EnableX` bool + dashboard toggle.
- **Versioning**: Every commit must update `CHANGELOG.md` and `VERSION`.
- **i18n**: All user-facing strings via translation keys in both `en.ts` and `zh-TW.ts`.
- **6-touch-point config pattern**: struct field, JSON struct, defaults, apply, toJSON, fromEnv.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `testing` | 1.26 | All backend tests | Project convention |
| `alicebob/miniredis/v2` | 2.37.0 | In-memory Redis for allocation tests | Already used by `allocator_test.go` |
| `redis/go-redis/v9` | 9.18.0 | Redis state for allocation tracking | Already the persistence layer |
| React | 19.2.0 | Dashboard UI | Existing frontend |
| Tailwind CSS | 4.2.1 | Dashboard styling | Existing frontend |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `modernc.org/sqlite` | (existing) | Analytics data source for performance weighting | Already used by Phase 4 analytics store |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Extending `CapitalAllocator` | New `AllocationEngine` struct | Unnecessary complexity; allocator already has Redis state model, reserve/commit/release lifecycle, and strategy/exchange tracking |
| Redis for allocation state | SQLite | Redis is already the state store for all trading data; allocation state changes every scan cycle and must be fast |
| Static profile map in Go | Config-driven profile definitions | Profiles are static presets (3 named profiles); Go map is simpler and compile-time safe |

**No new dependencies required.** Phase 5 uses only existing libraries.

## Architecture Patterns

### Recommended Project Structure
```
internal/risk/
    allocator.go          # Extend: ComputeEffectiveAllocation, dynamic strategyPct
    allocator_test.go     # Extend: tests for dynamic allocation
    profiles.go           # NEW: RiskProfile preset definitions + ApplyProfile logic
    profiles_test.go      # NEW: profile application tests
internal/config/
    config.go             # Extend: TotalCapitalUSDT, RiskProfile, AllocationLookbackDays,
                          #   AllocationFloorPct, AllocationCeilingPct, EnableUnifiedCapital
internal/engine/
    capital.go            # Modify: derive CapitalPerLeg from TotalCapitalUSDT when set
internal/spotengine/
    engine.go             # Modify: capitalForExchange reads from unified pool when enabled
internal/api/
    handlers.go           # Extend: allocation summary endpoint, profile selection in config
web/src/pages/
    Config.tsx            # Extend: profile selector, unified capital fields
    Overview.tsx          # Extend: allocation summary display (or new section)
web/src/i18n/
    en.ts                 # Extend: new translation keys
    zh-TW.ts              # Extend: new translation keys
```

### Pattern 1: Profile Preset Application
**What:** A map of named profiles where each profile is a struct of config overrides. Selecting a profile writes its values to the corresponding config fields. Any manual change to a bundled field sets `RiskProfile = "custom"`.
**When to use:** When the user selects a risk profile from the dashboard.
**Example:**
```go
// internal/risk/profiles.go
type ProfilePreset struct {
    MaxPositions      int
    Leverage          int
    MaxCostRatio      float64 // perp-perp entry threshold
    MinNetYieldAPR    float64 // spot-futures entry threshold
    PerpPerpPct       float64 // base allocation to perp-perp
    SpotFuturesPct    float64 // base allocation to spot-futures
    SizeMultiplier    float64 // multiplier on derived CapitalPerLeg
}

var Profiles = map[string]ProfilePreset{
    "conservative": {
        MaxPositions:   1,
        Leverage:       2,
        MaxCostRatio:   0.30,
        MinNetYieldAPR: 0.15,
        PerpPerpPct:    0.60,
        SpotFuturesPct: 0.40,
        SizeMultiplier: 0.7,
    },
    "balanced": {
        MaxPositions:   3,
        Leverage:       3,
        MaxCostRatio:   0.50,
        MinNetYieldAPR: 0.10,
        PerpPerpPct:    0.50,
        SpotFuturesPct: 0.50,
        SizeMultiplier: 1.0,
    },
    "aggressive": {
        MaxPositions:   5,
        Leverage:       5,
        MaxCostRatio:   0.70,
        MinNetYieldAPR: 0.05,
        PerpPerpPct:    0.40,
        SpotFuturesPct: 0.60,
        SizeMultiplier: 1.3,
    },
}
```

### Pattern 2: Performance-Weighted Allocation
**What:** On each allocation evaluation, fetch recent strategy summaries from Phase 4 analytics, compute APR ratio, and tilt base split accordingly within floor/ceiling bounds.
**When to use:** During entry scan cycles when `EnableUnifiedCapital` is true and analytics data is sufficient.
**Example:**
```go
// internal/risk/allocator.go (new method)
func (a *CapitalAllocator) ComputeEffectiveAllocation(
    perpAPR, spotAPR float64,
    basePerpPct, baseSpotPct float64,
    floorPct, ceilingPct float64,
) (perpPct, spotPct float64) {
    totalAPR := perpAPR + spotAPR
    if totalAPR <= 0 {
        return basePerpPct, baseSpotPct
    }
    // Linear tilt: weight by APR contribution
    perpWeight := perpAPR / totalAPR
    spotWeight := spotAPR / totalAPR
    
    // Blend: 50% base + 50% performance-weighted
    perpPct = 0.5*basePerpPct + 0.5*perpWeight
    spotPct = 0.5*baseSpotPct + 0.5*spotWeight
    
    // Enforce floor/ceiling
    perpPct = clamp(perpPct, floorPct, ceilingPct)
    spotPct = clamp(spotPct, floorPct, ceilingPct)
    
    // Normalize to sum to 1.0
    total := perpPct + spotPct
    perpPct /= total
    spotPct /= total
    return
}
```

### Pattern 3: Derived Capital Per Leg
**What:** When `TotalCapitalUSDT > 0` (unified capital enabled), derive per-leg capital automatically. Manual `CapitalPerLeg` override still respected.
**When to use:** In engine entry paths and risk manager sizing.
**Example:**
```go
// internal/engine/capital.go (enhanced)
func (e *Engine) effectiveCapitalPerLeg() float64 {
    if e.cfg.CapitalPerLeg > 0 {
        return e.cfg.CapitalPerLeg // manual override
    }
    if e.cfg.TotalCapitalUSDT <= 0 {
        return 0 // auto from balance (existing behavior)
    }
    maxPos := e.cfg.MaxPositions
    if maxPos <= 0 {
        maxPos = 1
    }
    return e.cfg.TotalCapitalUSDT / float64(maxPos) / 2.0
}
```

### Pattern 4: Dynamic Opportunity-Based Shifting (CA-04)
**What:** When one strategy finds no opportunities in a scan cycle, its uncommitted allocation becomes available to the other strategy for that cycle only.
**When to use:** During entry execution, before checking caps in `CapitalAllocator.Reserve`.
**Example:**
```go
// The allocator's checkCaps already reads strategy percentages via strategyPct().
// Make strategyPct() accept an "effective" override from the scan context:
//   - If spot-futures has 0 opportunities this cycle, perp-perp can use up to ceiling
//   - Committed capital (existing positions) still counts against the strategy cap
//   - Only the UNUSED portion of the other strategy's allocation is freed
```

### Anti-Patterns to Avoid
- **Duplicating capital state**: Do NOT create a second tracking system alongside the existing `CapitalAllocator`. All capital state must flow through the allocator's Redis keys.
- **Replacing CapitalPerLeg inline**: Do NOT remove `CapitalPerLeg` from config. It serves as the manual override (D-12, D-13). Only derive it when `TotalCapitalUSDT > 0`.
- **Eager profile application**: Do NOT auto-apply a profile on startup if `RiskProfile` is "custom". Only apply when the user explicitly selects a named profile.
- **Blocking analytics queries in trade path**: Analytics `ComputeStrategySummary` reads from Redis history. Cache the result and refresh on scan cycles (every ~10 min), never inline during order execution.
- **Hard-coding profile values in config defaults**: Profile presets belong in `profiles.go`, not scattered across config defaults. The config only stores the *result* of applying a profile (the individual field values).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Capital reservation tracking | Custom in-memory map | `CapitalAllocator` (existing) | Already handles Redis-backed reserve/commit/release with TTL and version-retried transactions |
| Strategy performance aggregation | Manual Redis position scanning | `analytics.ComputeStrategySummary` (Phase 4) | Already computes per-strategy APR, PnL, win rate from position history |
| Config field management | Ad-hoc struct fields | 6-touch-point pattern (existing convention) | Ensures JSON, defaults, apply, toJSON, fromEnv all stay in sync |
| Concurrent capital access | Custom mutex locking | `CapitalAllocator.withVersionRetry` + Redis WATCH | Already implements optimistic concurrency with 8-retry loop |

**Key insight:** The biggest risk in this phase is building new infrastructure instead of extending what exists. The `CapitalAllocator` already has 90% of the state management needed. The main work is making its percentage caps dynamic (driven by profiles + performance) rather than static config values.

## Common Pitfalls

### Pitfall 1: Profile-Config Desynchronization
**What goes wrong:** User selects "balanced" profile, manually tweaks leverage to 4, then later re-selects "balanced" -- expectation is leverage resets to 3, but code doesn't know which fields were overridden.
**Why it happens:** Profile application is one-way (profile -> config), but detection of "custom" requires tracking original profile values.
**How to avoid:** When user selects a named profile, unconditionally overwrite ALL bundled fields. When any bundled field is changed manually via POST /api/config, set `RiskProfile = "custom"`. The detection is simple: `handlePostConfig` checks if any profile-bundled field path was included in the update payload.
**Warning signs:** `RiskProfile` says "balanced" but actual values don't match the preset.

### Pitfall 2: Division by Zero in Derived Capital
**What goes wrong:** `TotalCapitalUSDT / MaxPositions / 2` crashes when MaxPositions is 0.
**Why it happens:** Config default for MaxPositions is 1, but user could set it to 0 via API.
**How to avoid:** Always clamp MaxPositions to >= 1 before division. The `effectiveCapitalPerLeg` function must guard against zero.
**Warning signs:** Panic in sizing logic.

### Pitfall 3: Stale Performance Data Causing Oscillation
**What goes wrong:** Performance weighting swings allocation 80/20 one cycle, then back 20/80 the next, because a single closed position changes the APR.
**Why it happens:** With few positions (< 10), a single large win/loss dominates the APR calculation.
**How to avoid:** (1) Require minimum 3 closed positions per strategy before applying performance tilt (D-08). (2) Use the floor/ceiling bounds (D-06, e.g., 20%/80%) to limit swing magnitude. (3) Consider exponential moving average or blending factor (50% base + 50% performance) rather than pure APR ratio.
**Warning signs:** Allocation percentages oscillating more than 10% per cycle.

### Pitfall 4: Breaking Existing Perp-Perp Trading
**What goes wrong:** Enabling `EnableUnifiedCapital` overrides `CapitalPerLeg` and suddenly changes position sizing for live positions.
**Why it happens:** Existing positions were opened with old sizing; new derived sizing could be different.
**How to avoid:** (1) Feature defaults to OFF (D-13 backward compatibility). (2) Unified capital only affects NEW entries -- existing positions retain their original sizing. (3) `CapitalPerLeg > 0` always takes precedence (explicit manual override).
**Warning signs:** Position sizes unexpectedly changing after config update.

### Pitfall 5: Opportunity-Based Shifting Without Committed Tracking
**What goes wrong:** Strategy A has no opportunities, so all capital shifts to strategy B. But strategy A still has committed positions consuming capital. The shift gives B more than actually available.
**Why it happens:** "No opportunities" doesn't mean "no committed capital."
**How to avoid:** Dynamic shifting only frees the UNCOMMITTED portion of a strategy's allocation. Formula: `available_for_B = B_ceiling - B_committed + max(0, A_allocation - A_committed)`. The existing `allocatorState.byStrategy` already tracks committed amounts.
**Warning signs:** Total exposure exceeding `TotalCapitalUSDT`.

### Pitfall 6: Config JSON Section Placement
**What goes wrong:** New allocation config fields placed in the wrong JSON section, causing dashboard config save to silently drop them.
**Why it happens:** The config has nested sections (fund, risk, strategy, spot_futures, safety, analytics). Placing a field in the wrong section means `applyJSON` doesn't read it.
**How to avoid:** Follow the CONTEXT.md pattern: `TotalCapitalUSDT` goes in a new `"allocation"` JSON section (or in `"fund"` alongside `CapitalPerLeg`). `RiskProfile` could go in `"risk"` or a new `"allocation"` section. Decision: use a new `"allocation"` section for all Phase 5 fields to keep them grouped.
**Warning signs:** Config values reverting to defaults after restart.

## Code Examples

### Example 1: 6-Touch-Point Pattern for New Config Field
```go
// TOUCH 1: Config struct field (internal/config/config.go, ~line 232)
TotalCapitalUSDT float64 // Total USDT pool for unified capital allocation (0 = disabled, use CapitalPerLeg)
RiskProfile      string  // "conservative", "balanced", "aggressive", "custom"
EnableUnifiedCapital bool // Master on/off for unified capital allocation (default OFF)

// TOUCH 2: JSON struct (jsonAllocation nested struct)
type jsonAllocation struct {
    EnableUnifiedCapital    *bool    `json:"enable_unified_capital"`
    TotalCapitalUSDT        *float64 `json:"total_capital_usdt"`
    RiskProfile             *string  `json:"risk_profile"`
    AllocationLookbackDays  *int     `json:"allocation_lookback_days"`
    AllocationFloorPct      *float64 `json:"allocation_floor_pct"`
    AllocationCeilingPct    *float64 `json:"allocation_ceiling_pct"`
}

// TOUCH 3: Defaults
EnableUnifiedCapital: false,
TotalCapitalUSDT:     0,
RiskProfile:          "balanced",
AllocationLookbackDays: 7,
AllocationFloorPct:    0.20,
AllocationCeilingPct:  0.80,

// TOUCH 4: applyJSON
if alloc.EnableUnifiedCapital != nil {
    c.EnableUnifiedCapital = *alloc.EnableUnifiedCapital
}
if alloc.TotalCapitalUSDT != nil {
    c.TotalCapitalUSDT = *alloc.TotalCapitalUSDT
}
if alloc.RiskProfile != nil {
    c.RiskProfile = *alloc.RiskProfile
}

// TOUCH 5: toJSON
alloc := getMap(raw, "allocation")
alloc["enable_unified_capital"] = c.EnableUnifiedCapital
alloc["total_capital_usdt"] = c.TotalCapitalUSDT
alloc["risk_profile"] = c.RiskProfile

// TOUCH 6: fromEnv
if v := os.Getenv("TOTAL_CAPITAL_USDT"); v != "" {
    if f, err := strconv.ParseFloat(v, 64); err == nil { c.TotalCapitalUSDT = f }
}
```

### Example 2: Integration with Existing Risk Manager Sizing
```go
// In risk/manager.go CalculateSize and calculateSizeWithPrice:
// Replace: if m.cfg.CapitalPerLeg > 0 { ... }
// With: effective := m.effectiveCapitalPerLeg()
//        if effective > 0 { ... }

func (m *Manager) effectiveCapitalPerLeg() float64 {
    if m.cfg.CapitalPerLeg > 0 {
        return m.cfg.CapitalPerLeg
    }
    if !m.cfg.EnableUnifiedCapital || m.cfg.TotalCapitalUSDT <= 0 {
        return 0
    }
    maxPos := m.cfg.MaxPositions
    if maxPos <= 0 {
        maxPos = 1
    }
    // D-12: TotalCapitalUSDT / maxPositions / 2 (two legs per position)
    derived := m.cfg.TotalCapitalUSDT / float64(maxPos) / 2.0
    // Apply profile size multiplier if set
    // (SizeMultiplier stored in config after profile application)
    return derived
}
```

### Example 3: Dashboard Profile Selector Component Pattern
```typescript
// Follows existing Config.tsx patterns (ToggleSwitch, NumberField, handleChange)
const PROFILE_OPTIONS = ['conservative', 'balanced', 'aggressive', 'custom'] as const;

// In renderAllocationTab():
<div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
  <label className="text-sm font-medium">{t('cfg.alloc.riskProfile')}</label>
  <select
    className="mt-1 block w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm"
    value={getByPath(config, ['allocation', 'risk_profile']) || 'balanced'}
    onChange={(e) => handleProfileChange(e.target.value)}
  >
    {PROFILE_OPTIONS.map(p => (
      <option key={p} value={p}>{t(`cfg.alloc.profile.${p}`)}</option>
    ))}
  </select>
</div>
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual `CapitalPerLeg` per engine | Still current (Phase 5 changes this) | Phase 5 | Unified pool derives per-leg capital |
| Static `MaxPerpPerpPct`/`MaxSpotFuturesPct` | Still current (Phase 5 changes this) | Phase 5 | Dynamic allocation from performance data |
| No risk profiles | Still current (Phase 5 adds this) | Phase 5 | One-click config bundling |

**Current state of codebase (as of Phase 4 completion):**
- `CapitalAllocator` fully functional with Redis-backed state, strategy/exchange caps, reservation TTL
- `ComputeStrategySummary` returns APR, PnL, win rate per strategy
- Config 6-touch-point pattern well-established (EnableAnalytics, EnableLossLimits as references)
- Dashboard Config page has 6 strategy tabs (exchanges, perp, spot, risk, safety, analytics)

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `alicebob/miniredis/v2` |
| Config file | None (tests use in-code config setup) |
| Quick run command | `go test ./internal/risk/ -run TestCapitalAllocator -count=1 -v` |
| Full suite command | `go test ./internal/risk/ ./internal/analytics/ ./internal/config/ ./internal/engine/ ./internal/spotengine/ -count=1 -v` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CA-01 | Unified pool derives CapitalPerLeg correctly | unit | `go test ./internal/risk/ -run TestEffectiveCapitalPerLeg -count=1 -v` | Wave 0 |
| CA-01 | Backward compat: TotalCapitalUSDT=0 falls back to CapitalPerLeg | unit | `go test ./internal/risk/ -run TestCapitalFallback -count=1 -v` | Wave 0 |
| CA-02 | Profile application overwrites bundled fields | unit | `go test ./internal/risk/ -run TestProfileApplication -count=1 -v` | Wave 0 |
| CA-02 | Manual override sets RiskProfile to "custom" | unit | `go test ./internal/risk/ -run TestProfileCustomOnOverride -count=1 -v` | Wave 0 |
| CA-03 | Performance weighting tilts allocation within floor/ceiling | unit | `go test ./internal/risk/ -run TestPerformanceWeightedAllocation -count=1 -v` | Wave 0 |
| CA-03 | Insufficient data falls back to base split | unit | `go test ./internal/risk/ -run TestAllocationInsufficientData -count=1 -v` | Wave 0 |
| CA-04 | Zero opportunities shifts unused allocation | unit | `go test ./internal/risk/ -run TestDynamicShifting -count=1 -v` | Wave 0 |
| CA-04 | Committed capital not double-freed during shift | unit | `go test ./internal/risk/ -run TestShiftingRespectsCommitted -count=1 -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/risk/ -count=1 -v`
- **Per wave merge:** `go test ./internal/risk/ ./internal/analytics/ ./internal/config/ ./internal/engine/ ./internal/spotengine/ -count=1 -v`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/risk/profiles_test.go` -- covers CA-02 (profile preset application, custom detection)
- [ ] `internal/risk/allocator_test.go` -- extend with CA-01 (derived capital), CA-03 (performance weighting), CA-04 (dynamic shifting)
- [ ] Test helper: analytics mock that returns controllable `StrategySummary` for allocation tests

## Integration Points (Detailed)

### Backend Integration Map

| File | Current Behavior | Phase 5 Change | Risk Level |
|------|-----------------|----------------|------------|
| `internal/config/config.go` | ~100+ fields, 6-touch-point pattern | Add ~8 new fields (TotalCapitalUSDT, RiskProfile, EnableUnifiedCapital, AllocationLookbackDays, AllocationFloorPct, AllocationCeilingPct, SizeMultiplier) + new `jsonAllocation` struct | LOW -- additive only |
| `internal/risk/allocator.go` | Static `strategyPct()` reads config | `strategyPct()` becomes dynamic, reading from cached allocation state. New `ComputeEffectiveAllocation` method | MEDIUM -- core path but well-tested |
| `internal/risk/manager.go:152` | `needed := m.cfg.CapitalPerLeg` | Replace with `effectiveCapitalPerLeg()` that checks TotalCapitalUSDT first | MEDIUM -- directly affects trade sizing |
| `internal/risk/manager.go:515-624` | `if m.cfg.CapitalPerLeg > 0` in both CalculateSize variants | Same `effectiveCapitalPerLeg()` replacement | MEDIUM -- same sizing path |
| `internal/engine/engine.go:569,640` | `e.cfg.CapitalPerLeg * e.cfg.MarginSafetyMultiplier` | Replace with `e.effectiveCapitalPerLeg() * e.cfg.MarginSafetyMultiplier` | MEDIUM -- margin check path |
| `internal/engine/capital.go` | Reserve/commit helpers for perp capital | Add `effectiveCapitalPerLeg()` method on Engine | LOW -- helper addition |
| `internal/spotengine/engine.go:377` | `capitalForExchange()` reads SpotFuturesCapitalSeparate/Unified | When unified capital enabled, derive from TotalCapitalUSDT/MaxPositions | MEDIUM -- spot-futures sizing |
| `internal/api/handlers.go` | Config GET/POST, no allocation endpoint | Add allocation section to config, new `/api/allocation` GET endpoint, profile change handler | LOW -- additive |
| `cmd/main.go:150` | `NewCapitalAllocator(db, cfg)` | Optionally pass analytics store for performance data access | LOW -- constructor change |

### Frontend Integration Map

| File | Current State | Phase 5 Change |
|------|---------------|----------------|
| `web/src/pages/Config.tsx` | 6 strategy tabs, risk-alloc sub-tab | New "Allocation" strategy tab with profile selector, unified capital fields, allocation display |
| `web/src/pages/Overview.tsx` | No allocation display | Add allocation summary section (pool total, per-strategy split, per-exchange exposure) |
| `web/src/i18n/en.ts` | ~300 keys | Add ~25 new keys for allocation UI |
| `web/src/i18n/zh-TW.ts` | ~300 keys | Same ~25 keys in Traditional Chinese |

## Performance Weighting Formula Recommendation

Given the constraints (D-05 through D-08), the recommended formula is a **linear blend with configurable blend factor**:

```
Given:
  perpAPR, spotAPR  = trailing APR from ComputeStrategySummary (lookback window)
  basePerpPct, baseSpotPct = from profile (e.g., 0.50, 0.50 for balanced)
  floor, ceiling = from config (e.g., 0.20, 0.80)

If either strategy has < 3 closed positions:
  return basePerpPct, baseSpotPct  (no performance tilt)

perfRatio_perp = perpAPR / (perpAPR + spotAPR)
perfRatio_spot = spotAPR / (perpAPR + spotAPR)

// 50/50 blend of base and performance
effectivePerpPct = 0.5 * basePerpPct + 0.5 * perfRatio_perp
effectiveSpotPct = 0.5 * baseSpotPct + 0.5 * perfRatio_spot

// Clamp to floor/ceiling
effectivePerpPct = clamp(effectivePerpPct, floor, ceiling)
effectiveSpotPct = clamp(effectiveSpotPct, floor, ceiling)

// Normalize so they sum to 1.0
total = effectivePerpPct + effectiveSpotPct
effectivePerpPct /= total
effectiveSpotPct /= total
```

**Edge cases:**
- Both APRs negative: fall back to base split (performance data unhelpful)
- One APR negative, one positive: the positive strategy gets all performance weight, but floor protects the negative one
- Both APRs zero: fall back to base split

**Why linear blend over exponential decay:** Simpler to understand, debug, and explain in dashboard. The floor/ceiling already prevents extreme concentration. Exponential decay would be over-engineering for 2 strategies.

## Open Questions

1. **Config section placement**
   - What we know: Phase 5 needs ~8 new config fields. Options: new `"allocation"` JSON section, or split across existing `"fund"` and `"risk"` sections.
   - What's unclear: Whether a new top-level section or reusing existing sections is preferred.
   - Recommendation: New `"allocation"` JSON section. It groups all Phase 5 fields together, follows the precedent of `"safety"` and `"analytics"` being separate sections. Add it to `jsonConfig`, `configUpdate`, and `configResponse`.

2. **Dashboard tab vs section**
   - What we know: Config page currently has 6 strategy tabs. Overview page shows positions and stats.
   - What's unclear: Whether allocation display belongs on Overview or as a separate strategy tab.
   - Recommendation: Add "Allocation" as a 7th strategy tab in Config (for configuration), and add an allocation summary card on Overview (for monitoring). This follows the safety/analytics pattern.

3. **Performance data caching strategy**
   - What we know: `ComputeStrategySummary` reads from Redis position history. Scan cycles happen every 10 minutes.
   - What's unclear: Whether to cache the allocation computation result or recompute on each scan.
   - Recommendation: Cache the effective allocation percentages in memory (or a Redis key), refresh once per scan cycle. The allocator's `checkCaps` reads cached values. This keeps the trade path fast and avoids repeated history queries.

## Sources

### Primary (HIGH confidence)
- `internal/risk/allocator.go` -- Full source code analysis of existing CapitalAllocator
- `internal/risk/allocator_test.go` -- Test patterns for allocator
- `internal/analytics/aggregator.go` -- ComputeStrategySummary, CalculateAPR implementation
- `internal/config/config.go` -- Full config struct, 6-touch-point pattern
- `internal/engine/capital.go` -- Engine-side capital helpers
- `internal/spotengine/engine.go:374-390` -- capitalForExchange implementation
- `internal/risk/manager.go:152,515,623` -- CapitalPerLeg usage in sizing
- `internal/engine/engine.go:569,640` -- CapitalPerLeg usage in margin checks
- `internal/api/handlers.go` -- Config GET/POST patterns, configResponse/configUpdate structs
- `web/src/pages/Config.tsx` -- Dashboard tab structure, existing risk-alloc sub-tab

### Secondary (MEDIUM confidence)
- `.planning/phases/05-capital-allocation/05-CONTEXT.md` -- All 16 locked decisions
- `.planning/REQUIREMENTS.md` -- CA-01 through CA-04 requirement definitions

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- No new dependencies, all existing libraries
- Architecture: HIGH -- Extends well-understood existing patterns (allocator, config, API)
- Pitfalls: HIGH -- Derived from direct code analysis of integration points
- Performance formula: MEDIUM -- Recommendation based on domain analysis, not live testing

**Research date:** 2026-04-04
**Valid until:** 2026-05-04 (stable codebase, no external dependency changes expected)
