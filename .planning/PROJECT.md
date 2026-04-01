# Arb Bot — Unified Arbitrage System

## What This Is

A funding rate arbitrage system that monitors funding rate differentials across 6 exchanges (Binance, Bybit, Gate.io, Bitget, OKX, BingX) and executes delta-neutral positions. Two strategy engines — perp-perp (live since mid-March 2026) and spot-futures (first trade 2026-04-01, partially working). The system needs to evolve from two independent engines into a unified arbitrage platform with intelligent capital allocation.

## Core Value

"I deposit USDT, select my risk preference, and the system automatically finds opportunities across both strategies, opens positions, collects funding, exits when profitable, and I can see exactly how much each position earned."

## Requirements

### Validated

- ✓ Perp-perp engine running live on 6 exchanges — existing
- ✓ Discovery → Risk → Execute → Monitor → Exit pipeline — existing
- ✓ Dashboard with real-time positions, opportunities, config — existing
- ✓ Spot-futures Dir A (borrow-sell-long) working on Bybit — v0.22.49
- ✓ Spot-futures Dir B (buy-spot-short) working on Bybit — v0.22.46
- ✓ 5 margin adapters (Binance, Bybit, Gate.io, Bitget, OKX) — v0.22.46
- ✓ Auto-borrow/auto-repay on entry and exit — v0.22.49
- ✓ Margin health monitoring and emergency close — v0.22.49
- ✓ Scheduler, scan-driven execution, WebSocket feeds — existing

### Active

**Spot-Futures Engine (Priority 1)**
- [ ] Dir A + Dir B full lifecycle on all 5 exchanges (Binance, Bybit, Gate.io, Bitget, OKX)
- [ ] Auto-discovery → auto-open pipeline for spot-futures
- [ ] Auto-exit with all edge cases (blackout windows, partial fills, emergency)
- [ ] Single-exchange: both directions verified on each supported exchange
- [ ] Cross-exchange spot-futures (borrow on X, sell on Y / buy on X, short on Y)

**Perp-Perp Improvements (Priority 2)**
- [ ] Dashboard: detailed trade history with per-position breakdown
- [ ] Dashboard: UI improvements for better operational visibility
- [ ] Performance metrics: profit tracking by strategy, exchange, config
- [ ] Backtesting: replay historical funding data with different configs
- [ ] A/B dashboard: compare config profiles side by side
- [ ] Paper trading mode: simulate strategies without real orders

**Capital Allocation Engine (Priority 3)**
- [ ] Unified capital pool across both strategies
- [ ] Risk preference selection (user-configurable)
- [ ] Smart allocation: calculate optimal split between spot-futures and perp-perp
- [ ] Dynamic rebalancing: shift capital when one strategy is unavailable
- [ ] Risk parameter auto-adjustment based on allocation

### Out of Scope

- BingX for spot-futures — no margin/borrow support
- Binance Portfolio Margin — reverted (v0.20.1), too complex for limited benefit
- New exchange integrations — 6 exchanges is sufficient for now
- Mobile app — web dashboard is the interface

## Context

- **Live system**: perp-perp engine trading real money since mid-March 2026
- **Spot-futures**: first successful trade 2026-04-01, multiple bugs fixed same day (integer precision, margin health check, auto-borrow). Only Bybit verified so far.
- **Pain point**: previous agent team produced technically-detailed code that failed first contact with production. Adopting GSD framework for product-level planning with evaluation.
- **Experimentation needed**: optimal entry/exit params for perp-perp require real-world testing over time — metrics and tooling needed to validate.
- **Fund management**: currently separated — each engine has its own capital config. Goal 3 unifies this.
- **Codebase**: ~15k+ lines Go, React dashboard, 6 exchange adapters. See `.planning/codebase/` for full analysis.

## Constraints

- **Live system**: changes must not break existing perp-perp trading
- **Tech stack**: Go 1.22+, Redis DB 2, React/Vite/Tailwind, systemd deployment
- **npm lockdown**: axios compromise — only `npm ci`, never `npm install`
- **Build order**: frontend must build before Go binary (go:embed)
- **Exchange APIs**: rate limits, blackout windows (Bybit :04-:05:30), per-exchange quirks
- **Single server**: runs on one machine under systemd

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Single-exchange spot-futures first, cross-exchange later | Cross-exchange is fundamentally different architecture (two margin pools, coordination, harder rollback) | — Pending |
| Auto-borrow/repay via exchange margin API (not manual borrow+sell) | Eliminates precision issues, risk limit rejections, simplifies rollback | ✓ Good (v0.22.49) |
| BingX excluded from spot-futures | No borrow/margin support | ✓ Good |
| BingX stays in capital allocation | Still useful for perp-perp | — Pending |
| GSD framework for development | Previous agent teams failed to deliver working product — technical sharding without product validation | — Pending |
| Start with better metrics before backtesting/paper trading | Need measurement before experimentation | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition:**
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone:**
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-01 after initialization*
