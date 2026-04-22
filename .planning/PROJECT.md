# Arb Bot — Unified Arbitrage System

## What This Is

A multi-strategy arbitrage platform that monitors funding rate differentials and cross-exchange price dislocations across 6 exchanges (Binance, Bybit, Gate.io, Bitget, OKX, BingX) and executes delta-neutral positions. **v1.0 shipped** a unified platform with two working engines (perp-perp + spot-futures), performance analytics, unified capital allocation with risk profiles, and spot-futures risk hardening. **v2.0** evolves the platform toward a third engine (cross-exchange price-gap arbitrage) and addresses v1.0's accumulated documentation/verification tech debt.

## Core Value

"I deposit USDT, select my risk preference, and the system automatically finds opportunities across multiple strategies, opens positions, collects yield, exits when profitable, and I can see exactly how much each position earned — with capital shifting between strategies as opportunities shift."

## Current Milestone: v2.0 Multi-Strategy Expansion

**Goal:** Ship Strategy 4 (cross-exchange price-gap arbitrage) as a minimal live tracker, validate edge over 4–8 weeks, and address v1.0's accumulated documentation tech-debt.

**Target features:**
- Minimal price-gap tracker (`internal/pricegaptrader/`): static candidate config, delta-neutral IOC entry/exit, Gate concentration cap, hard denylist layer, exec-quality override, paper mode → $5k live budget
- Live validation gate (4–8 weeks) to decide upgrade to full Gated Discovery (D) architecture
- v1.0 tech-debt cleanup: Phase 07 retrospective VERIFICATION.md + Nyquist Wave-0 validations for phases 01/03/04/06

**Key context:**
- Phase 0/1/round-2 scoping complete (`/tmp/phase0-pricegap/`)
- Codex design review complete (dispatch task `4ecfbf85`) — (D) adopted as target, MVP-first as v2.0 scope
- Edge universe narrow (~5 pairs, SOON-dominated, Gate-centric) → live validation before scale-up is non-negotiable
- Phase numbering continues from v1.0 → v2.0 starts at **Phase 8**

## Requirements

### Validated (shipped in v1.0)

**Foundation (pre-v1.0):**
- ✓ Perp-perp engine running live on 6 exchanges — existing
- ✓ Discovery → Risk → Execute → Monitor → Exit pipeline — existing
- ✓ Dashboard with real-time positions, opportunities, config — existing
- ✓ Scheduler, scan-driven execution, WebSocket feeds — existing
- ✓ Spot-futures Dir A/B proof on Bybit — v0.22.x

**Spot-Futures Engine (v1.0 Phase 1-2, 6):**
- ✓ Dir A + Dir B full lifecycle on all 5 margin exchanges — v1.0 (SF-01, SF-02)
- ✓ Auto-borrow/auto-repay verified per exchange — v1.0 (SF-03)
- ✓ Auto-discovery pipeline ranks spot-futures opportunities — v1.0 (SF-04)
- ✓ Auto-open pipeline executes best opportunities — v1.0 (SF-05)
- ✓ Auto-exit handles edge cases (blackout windows, partial fills, emergency close, pending repay retry) — v1.0 (SF-06)
- ✓ Basis/spread gating on entry and exit — v1.0 (SF-07)
- ✓ Maintenance margin rates per-contract from all adapters — v1.0 (SF-RISK-02)
- ✓ Liquidation distance trigger with graduated response — v1.0 (SF-RISK-03)
- ✓ Maintenance gate dashboard wiring — v1.0 (SF-RISK-01, verification missing but code live as v0.29.0)

**Operational Safety (v1.0 Phase 3):**
- ✓ Telegram notifications for perp-perp critical events — v1.0 (PP-01, partial scope per D-01)
- ✓ Daily/weekly loss limits halt new entries — v1.0 (PP-03)
- ⊘ Circuit breaker per-exchange — v1.0 DROPPED (D-05 decision) (PP-02)

**Performance Analytics (v1.0 Phase 4):**
- ✓ Trade history dashboard with per-position breakdown — v1.0 (PP-04)
- ✓ Per-position PnL decomposition (fees, funding, basis) — v1.0 (AN-01)
- ✓ Strategy-level comparison (perp-perp vs spot-futures) — v1.0 (AN-02)
- ✓ Exchange-level metrics (profit, slippage, fill rate, error rate) — v1.0 (AN-03)
- ✓ Cumulative PnL chart with SQLite time-series — v1.0 (AN-04)
- ✓ APR calculation for perp-perp — v1.0 (AN-05)
- ✓ Win rate segmented by strategy and exchange — v1.0 (AN-06)

**Capital Allocation (v1.0 Phase 5):**
- ✓ Unified capital pool — v1.0 (CA-01)
- ✓ Risk preference profiles (conservative/balanced/aggressive) — v1.0 (CA-02)
- ✓ Strategy-weighted allocation based on recent performance — v1.0 (CA-03)
- ✓ Dynamic rebalancing when a strategy has no opportunities — v1.0 (CA-04)

### Active (v2.0 candidates)

**Strategy 4 — Cross-Exchange Price-Gap Arbitrage (Priority 1)**
- [x] Minimal price-gap tracker (Phase 8, v0.33.0): `internal/pricegaptrader/` with delta-neutral IOC entry/exit, 5-gate pre-entry risk (Gate concentration, max concurrent, kline staleness, delist veto, budget), 4h max-hold + T/2 reversion exit, exec-quality auto-disable, startup rehydration, pg-admin CLI for reversal. Default OFF per PG-OPS-06. Paper mode + live budget come in Phase 9.
- [ ] Phase 9: Dashboard UI, paper mode, Telegram alerts, rolling metrics (PG-OPS-01..05, PG-VAL-01..02)
- [ ] Validate edge on live data for 4–8 weeks (gate to (D) full architecture)
- [ ] (Deferred) Full (D) Gated Discovery architecture: scanner + registry + state machine — see `/tmp/phase0-pricegap/STRATEGY_DESIGN.md`

**v1.0 Tech Debt (Priority 2)**
- [ ] Phase 07 retrospective VERIFICATION.md + VALIDATION.md (SF-RISK-01)
- [ ] Nyquist Wave-0 validations for phases 01, 03, 04, 06
- [ ] Browser confirmations for phases 02, 03, 05, 06 (human_needed verif cleanup)

**Documentation / Observability (Priority 3)**
- [ ] `/gsd-docs-update` pass to refresh CHANGELOG / ARCHITECTURE post-v1.0 (CLAUDE.local.md is hand-owned — do not re-bloat)

### Out of Scope

- BingX for spot-futures — no margin/borrow support
- Binance Portfolio Margin — reverted (v0.20.1), too complex for limited benefit
- New CEX integrations beyond current 6 — sufficient breadth for now
- Mobile app — web dashboard is the interface
- Pairs / cross-asset arbitrage (e.g., ETH/BTC ratio) — confirmed out for Strategy 4 per user
- OKX + BingX in Strategy 4 v1 — deferred; may join if (D) architecture is built

## Context

- **Live system**: perp-perp since mid-March 2026; spot-futures live since 2026-04-01
- **Current version**: 0.33.0 (Phase 8 shipped — price-gap tracker core, default OFF)
- **Codebase**: ~15k+ Go, React dashboard, 6 exchange adapters. See `.planning/codebase/` for full analysis.
- **v1.0 metrics**: 7 phases, 21 plans, 381 commits over 30 days (2026-03-23 → 2026-04-21)
- **v1.0 audit**: classified `tech_debt` — 10/24 clean-passed requirements, 12 partial (human_needed verif), 1 dropped, 1 unsatisfied (Phase 07 missing VERIFICATION.md, code live). All code shipped and working in production; debt is documentation/verification only.
- **Strategy 4 scoping done**: Phase 0 + Phase 1 depth/slippage + Round-2 discovery complete. Edge universe is narrow (~5 pairs, SOON-dominated, Gate-centric). MVP-first approach chosen; full architecture deferred.
- **Design review by Codex** (2026-04-21): agreed with MVP-first direction; surfaced Gate concentration, correlation crowding, hard denylist, exec-quality override as first-class risk controls.

## Constraints

- **Live system**: changes must not break existing perp-perp or spot-futures trading
- **Tech stack**: Go 1.26+ required by go.mod (install.sh still provisions 1.22 — drift), Redis DB 2, React/Vite/Tailwind, systemd deployment
- **npm lockdown**: axios compromise — only `npm ci`, never `npm install`
- **Build order**: frontend must build before Go binary (go:embed)
- **Exchange APIs**: rate limits, blackout windows (Bybit :04–:05:30), per-exchange quirks, Gate 10k-candle 1m history cap
- **Single server**: runs on one machine under systemd
- **config.json is sole config source of truth** (Redis HSET persistence removed 2026-04-05)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Single-exchange spot-futures first, cross-exchange later | Cross-exchange spot-futures is different architecture | ✓ Good (v1.0 shipped single-exchange) |
| Auto-borrow/repay via exchange margin API | Eliminates precision issues, simplifies rollback | ✓ Good (v0.22.49) |
| BingX excluded from spot-futures | No borrow/margin support | ✓ Good |
| GSD framework for development | Prior agent teams failed to deliver working product | ✓ Good (v1.0 shipped 7 phases) |
| Unified capital pool with risk profiles | User wants single-deposit UX | ✓ Good (v1.0 Phase 5) |
| Performance-weighted allocation | Capital follows edge | ✓ Good (CA-03) |
| Config.json as sole config source | Redis persistence was wiping config | ✓ Good (2026-04-05) |
| Maintenance-rate survivable-drop gate | GUAUSDT incident exposed overleverage | ✓ Good (Phase 6) |
| Strategy 4 MVP-first (Option C), (D) deferred | Narrow edge universe (~5 pairs) doesn't justify 2-week full-architecture build; MVP validates live edge in ~1 week | — Pending (v2.0) |
| Circuit breaker per-exchange (PP-02) | Dropped in v1.0 per D-05 — low priority vs other safety work | ⊘ Dropped |
| Close v1.0 with `tech_debt` status | Code is live; documentation debt carried as v2.0 backlog, not worth blocking Strategy 4 for retrospective docs | — Pending (v2.0) |

## Evolution

This document evolves at phase transitions and milestone boundaries.

---
*Last updated: 2026-04-22 after Phase 8 (price-gap tracker core) shipped at v0.33.0*
