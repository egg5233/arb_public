# Phase 5: Capital Allocation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-04
**Phase:** 05-capital-allocation
**Areas discussed:** Risk profile definition, Allocation algorithm, Capital pool model, Dynamic rebalancing
**Mode:** auto (all areas auto-selected, recommended defaults chosen)

---

## Risk Profile Definition

| Option | Description | Selected |
|--------|-------------|----------|
| Static presets only | Three fixed profiles, no customization | |
| Static presets with tunable overrides | Profiles set defaults, user can tweak individual fields | auto |
| Fully custom only | No presets, user configures everything | |

**User's choice:** [auto] Static presets with tunable overrides (recommended default)
**Notes:** Keeps it simple for most users while allowing power-user customization. Profile switches to "custom" when any field is manually changed.

---

## Allocation Algorithm

| Option | Description | Selected |
|--------|-------------|----------|
| Fixed ratio from profile | Static perp/spot split, no performance adjustment | |
| Performance-weighted with floor/ceiling | Base split tilted by trailing APR, bounded by min/max | auto |
| Fully dynamic (100% performance-driven) | No fixed ratio, allocation entirely driven by returns | |

**User's choice:** [auto] Performance-weighted with floor/ceiling (recommended default)
**Notes:** Leverages Phase 4 analytics data while preventing runaway concentration. Floor/ceiling (20%/80%) provides safety bounds.

---

## Capital Pool Model

| Option | Description | Selected |
|--------|-------------|----------|
| Keep separate per-engine configs | No change, manual CapitalPerLeg + SpotFuturesCapital | |
| Single pool with percentage splits | One TotalCapitalUSDT, existing Pct fields become ceilings | auto |
| Tiered pool (total → strategy → exchange) | Three-level hierarchy | |

**User's choice:** [auto] Single pool with percentage splits (recommended default)
**Notes:** Extends existing CapitalAllocator pattern. Backward-compatible: TotalCapitalUSDT=0 falls back to manual config.

---

## Dynamic Rebalancing

| Option | Description | Selected |
|--------|-------------|----------|
| Evaluate on scan cycle | Check allocation each scan, shift unused to other strategy | auto |
| Dedicated rebalance timer | Separate goroutine with configurable interval | |
| Manual only | User triggers rebalance from dashboard | |

**User's choice:** [auto] Evaluate on scan cycle (recommended default)
**Notes:** Aligns with existing scan-driven execution model. No new goroutines or timers needed.

---

## Claude's Discretion

- Config field naming, struct layout, JSON keys
- Dashboard UI placement
- Performance weighting formula specifics
- Redis key structure

## Deferred Ideas

None
