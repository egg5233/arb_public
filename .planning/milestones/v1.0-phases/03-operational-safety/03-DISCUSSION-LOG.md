# Phase 3: Operational Safety - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-02
**Phase:** 03-operational-safety
**Areas discussed:** Telegram event scope, Circuit breaker design, Loss limit enforcement, Dashboard surface

---

## Telegram Event Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Critical only | SL triggered, emergency close, error threshold. 1-3 msgs/day | ✓ |
| Trade lifecycle | All entries, exits, SL, emergency, errors. 5-15 msgs/day | |
| Everything | Entries, exits, rotations, rebalances, funding, errors. 20+ msgs/day | |

**User's choice:** Critical only. Also requested extensible design so spot-futures events can be added easily.

### Error Threshold

| Option | Description | Selected |
|--------|-------------|----------|
| Consecutive failures | Alert after N consecutive API errors on same exchange | ✓ |
| Rate-based | Alert when error rate exceeds X% over time window | |
| You decide | Claude picks | |

**User's choice:** 3 consecutive failures on same exchange.

### Rate Limiting

| Option | Description | Selected |
|--------|-------------|----------|
| Cooldown per event type | Same event type can only fire once per N minutes | ✓ |
| Daily digest | Batch non-critical alerts into periodic summaries | |
| No throttle | See every alert as it happens | |
| You decide | Claude picks | |

**User's choice:** Cooldown per event type (5 min).

---

## Circuit Breaker Design

**User's question:** "I don't remember what circuit breaker was for, to stop the whole system?"

**Clarification provided:** Per-exchange only — pauses new entries on failing exchange while other exchanges continue. Existing positions still managed.

**User's insight:** "If an exchange's API starts failing, then opening position will also not be possible, perhaps the circuit breaker is not needed?"

| Option | Description | Selected |
|--------|-------------|----------|
| Keep it | Protects against flaky-but-not-dead API. Dashboard visibility. | |
| Drop it | API failures already prevent entries. Telegram alert covers awareness. | ✓ |
| Defer it | Move to backlog, revisit if problems occur. | |

**User's choice:** Drop it. Simplifies Phase 3 scope.

---

## Loss Limit Enforcement

### Time Window

| Option | Description | Selected |
|--------|-------------|----------|
| Calendar day/week | Resets at UTC midnight / Monday 00:00 | |
| Rolling window | Last 24h / last 7 days sliding | ✓ |
| You decide | Claude picks | |

**User's choice:** Rolling window.

### Breach Action

| Option | Description | Selected |
|--------|-------------|----------|
| Stop new entries only | Existing positions continue. Operator gets alert. | ✓ |
| Stop entries + force-exit all | Nuclear option, close everything. | |
| Stop entries + notify, operator decides | Halt entries, alert, operator manually decides. | |

**User's choice:** Stop new entries only.

### Risk Tier Interaction

| Option | Description | Selected |
|--------|-------------|----------|
| Independent systems | Loss limits and risk tiers operate separately. | ✓ |
| Coordinated | Loss limit breach escalates risk tier. | |

**User's choice:** Independent systems.

### Dashboard Visibility

| Option | Description | Selected |
|--------|-------------|----------|
| Banner/badge on existing pages | Loss status on Overview, turns red near threshold | ✓ |
| Dedicated safety section | New section grouping safety indicators | |
| You decide | Claude picks | |

**User's choice:** Banner on Overview page.

---

## Claude's Discretion

- Default threshold values for daily/weekly loss limits
- TelegramNotifier refactoring approach
- Redis key design for rolling loss windows
- Plan ordering

## Deferred Ideas

- Circuit breaker (PP-02) — dropped, revisit if flaky API issues occur
- Non-critical Telegram events (entries, exits, rotations) — add later as configurable levels
