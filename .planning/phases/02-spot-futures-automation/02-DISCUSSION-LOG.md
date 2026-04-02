# Phase 2: Spot-Futures Automation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-02
**Phase:** 02-spot-futures-automation
**Areas discussed:** Discovery resilience, Basis/spread gating, Exit safeguards, Go-live strategy

---

## Discovery Resilience

| Option | Description | Selected |
|--------|-------------|----------|
| A) Keep CoinGlass, better staleness handling | Quick fix, minimal work, still fragile | |
| B) Native scanner from Loris + exchange APIs, CoinGlass fallback | More work, much more reliable, uses existing building blocks | ✓ |
| C) Research CoinGlass paid API first | Unknown availability/cost, delays decision | |

**User's choice:** B — Build native discovery from Loris funding rates + exchange borrow APIs. Keep CoinGlass as optional fallback.
**Notes:** User confirmed CoinGlass is the only external source they know of. The codebase already has borrow rate fetching per exchange and Loris API for funding rates, making native discovery feasible.

---

## Basis/Spread Gating (SF-07)

| Option | Description | Selected |
|--------|-------------|----------|
| Configurable with defaults | All thresholds as config fields, dashboard toggles, reasonable defaults | ✓ |
| Hardcoded thresholds | Simpler but inflexible | |
| Depth-aware (like perp-perp) | More complex, overkill for spot-futures | |

**User's choice:** Configurable with reasonable defaults.
**Notes:** Applies to all gating — entry basis check, exit spread gate. Follows project convention for risk filters.

---

## Exit Safeguards

| Option | Description | Selected |
|--------|-------------|----------|
| Configurable min-hold + settlement guard | Add both as config fields with defaults and dashboard toggles | ✓ |
| No new guards (continuous monitor only) | Current behavior, no settlement protection | |

**User's choice:** Same as basis/spread — configurable with defaults.
**Notes:** Min-hold gate and settlement-window guard adapted from perp-perp patterns.

---

## Go-Live Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Simple flip | Deploy → dry-run → observe → enable. Uses existing config knobs | ✓ |
| Graduated ramp (new code) | Automated position count increase over time | |
| Automated validation | Track would-have entries, compare profitability | |

**User's choice:** Simple flip. Manual dry-run observation + gradual `MaxPositions` ramp using existing config fields.
**Notes:** User asked for explanation of options. Confirmed that existing `auto_enabled`, `dry_run`, `max_positions` knobs are sufficient. No new transition code needed.

---

## Scope Clarification

**User asked:** "Is risk management in this phase?"
**Answer:** No — heavy risk features (circuit breakers, loss limits, Telegram alerts) are Phase 3. Phase 2 includes gating thresholds (SF-07) and exit safeguards as part of making automation safe, but not a new risk framework.

## Claude's Discretion

- Specific threshold default values for all configurable gates
- Native scanner polling interval and data merging strategy
- Implementation plan ordering

## Deferred Ideas

- Cross-exchange spot-futures (v2 requirement)
- Automated dry-run validation (nice-to-have)
