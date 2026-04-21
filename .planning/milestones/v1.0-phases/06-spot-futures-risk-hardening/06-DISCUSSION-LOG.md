# Phase 6: Spot-Futures Risk Hardening - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-05
**Phase:** 06-spot-futures-risk-hardening
**Areas discussed:** Pre-entry rejection thresholds, Health monitor integration, Runtime liquidation response, Discovery maintenance_rate handling

---

## Pre-entry Rejection Thresholds

### Minimum survivable price drop

| Option | Description | Selected |
|--------|-------------|----------|
| 20% | Aggressive, allows more coins but tighter margin | |
| 30% | Moderate, the value from success criteria | |
| 40% | Conservative, filters out most mid-cap coins | |
| Leverage-scaled | 90%/leverage: 2x→45%, 3x→30%, 4x→22.5%, 5x→18% | ✓ |

**User's choice:** Leverage-scaled formula (90%/leverage)
**Notes:** User specified the exact values per leverage level. Pattern naturally scales — higher leverage = tighter threshold.

### Unified vs separate-account exchange thresholds

| Option | Description | Selected |
|--------|-------------|----------|
| Same threshold for all | Simpler, one knob | ✓ |
| Stricter for separate accounts | e.g. 40% for Binance/Bitget, 30% for unified | |

**User's choice:** Same threshold, follows the leverage-scaled formula for all exchanges.

### Hard reject vs size reduction

| Option | Description | Selected |
|--------|-------------|----------|
| Hard reject | Don't enter at all | |
| Auto-reduce size | Shrink position until it fits | |
| Hard reject + logged reason (auto) / bypass for manual | Auto: hard reject with reason. Manual: allow bypass with warning. | ✓ |

**User's choice:** Dual behavior — auto-entry hard-rejects with logged reason, manual open allows bypass.

---

## Health Monitor Integration

### Same HealthMonitor vs separate system

| Option | Description | Selected |
|--------|-------------|----------|
| Same HealthMonitor | Add spot-futures to existing checkAll(), same L3/L4/L5 tiers | ✓ |
| Separate spot-futures health loop | New goroutine in spotengine with own thresholds | |
| Hybrid | HealthMonitor collects both, different thresholds per engine | |

**User's choice:** Same HealthMonitor. Initially unsure — after discussion about what the spotengine already monitors (exit triggers) vs what HealthMonitor does (exchange-level L4/L5 position reduction), user understood the gap: L4 needs to see spot-futures positions as candidates for reduction alongside perp-perp.
**Notes:** User confirmed the purpose: "let L4 reduce not only perp-perp, but also spot-perp dir A/B."

### L3 transfer behavior for spot-futures

| Option | Description | Selected |
|--------|-------------|----------|
| Transfer USDT in (same as perp-perp) | Simple, works on unified accounts | ✓ |
| Transfer futures profit to margin | Separate-account exchanges only | |
| Both depending on exchange type | Most complete but complex | |

**User's choice:** Same as perp-perp (transfer USDT in). Separate-account profit transfer deferred.

---

## Runtime Liquidation Response

### Graduated vs single threshold

| Option | Description | Selected |
|--------|-------------|----------|
| Graduated | Warn/exit/emergency at different distances | ✓ |
| Single threshold | Just exit when too close | |

**User's choice:** Graduated, linked to entry threshold.

### Independent vs linked thresholds

| Option | Description | Selected |
|--------|-------------|----------|
| Independent | Separate config knobs for pre-entry and runtime | |
| Linked | Runtime thresholds derived as fractions of entry threshold | ✓ |

**User's choice:** Linked. User iterated through several fraction sets:
- 50/33/17 — initial proposal
- 60/30/10 — user's first try
- 65/25/10 — second iteration
- 50/15/10 — third iteration (exit and emergency too close)
- **50/20/10** — final choice

**Notes:** User confirmed the fractions don't need to add to 100% — they're independent trigger lines. Final: warn at 50% of entry threshold, exit at 20%, emergency at 10%.

---

## Discovery Maintenance Rate Handling

### Filter/penalty approach

| Option | Description | Selected |
|--------|-------------|----------|
| Hard filter | Reject above threshold | |
| Soft penalty | Reduce NetAPR score | |
| Both | Hard filter + soft penalty | |
| Display only (defer scoring) | Fetch and show in dashboard, no filter/penalty yet | ✓ |

**User's choice:** Display only. Fetch maintenance_rate during discovery, show in dashboard opportunities table, but no filter or scoring penalty. User wants to observe real values in production before deciding on a filtering strategy.
**Notes:** User asked whether all exchanges have maintenance_rate data (yes, all 5 do — via different endpoints). User also asked about current scoring model (purely NetAPR sort) and how penalties would work. After seeing examples, decided to defer the scoring decision.

---

## Claude's Discretion

- Tiered maintenance_rate lookup implementation (cache strategy, refresh interval)
- Dashboard UI layout for maintenance_rate column
- Config field naming within established convention
- Plan decomposition and wave ordering

## Deferred Ideas

- Discovery scoring/filtering by maintenance_rate — defer until after live observation
- Separate-account futures→margin profit transfer for L3 mitigation
- Dedicated fast-poll price monitor goroutine (DESIGN_SPOT_FUTURES_RISK.md Tier 1)
