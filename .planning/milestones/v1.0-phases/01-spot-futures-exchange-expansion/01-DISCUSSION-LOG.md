# Phase 1: Spot-Futures Exchange Expansion - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-01
**Phase:** 01-spot-futures-exchange-expansion
**Areas discussed:** Exchange priority order, Verification definition, Testing approach, Per-exchange divergence tolerance

---

## Exchange Priority Order

| Option | Description | Selected |
|--------|-------------|----------|
| Easiest first | Binance → Bitget → Gate.io → OKX — build confidence on simpler exchanges | |
| Hardest first | OKX → Gate.io → Bitget → Binance — tackle riskiest unknowns early | |
| Most profitable first | User picks order based on trading priorities | |
| All at once | One plan per exchange, parallel execution | |

**User's choice:** Custom order — Bybit → Binance → Bitget → Gate.io → OKX. User corrected that Bybit is not fully tested and must be first.
**Notes:** User explicitly stated "bybit is not fully tested" — this overrides the assumption from PROJECT.md that Bybit was verified.

---

## Verification Definition

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal | Manual open + close succeeds for both Dir A and Dir B | |
| Standard | Minimal + auto-borrow/repay works, no residual borrows | |
| Thorough | Standard + edge cases: partial fills, repay retry, emergency close, blackout windows | ✓ |
| You decide | Claude picks appropriate level per exchange | |

**User's choice:** Thorough
**Notes:** None

---

## Testing Approach

| Option | Description | Selected |
|--------|-------------|----------|
| Live trades only | Small real-money positions, fix bugs as they surface | |
| Unit tests first, then live | Mock adapter tests, then live validation | |
| Livetest CLI first, then trades | Verify each margin API method individually via livetest harness, then full lifecycle | ✓ |
| You decide | Claude picks mix per exchange | |

**User's choice:** Livetest CLI first, then trades
**Notes:** None

---

## Per-Exchange Divergence Tolerance

| Option | Description | Selected |
|--------|-------------|----------|
| Adapter absorbs all | All differences hidden in margin.go, engine stays exchange-agnostic | |
| Engine can branch | Allow exchange-name checks in spot engine when difference is fundamental | ✓ |
| Mix | Prefer adapter-level, engine branching as escape hatch with comments | |

**User's choice:** Engine can branch on exchange name
**Notes:** None

---

## Claude's Discretion

- Position sizing for test trades
- Bug-fix sequencing within each exchange (Dir A first vs interleaved)
- Livetest coverage per margin method

## Deferred Ideas

None — discussion stayed within phase scope
