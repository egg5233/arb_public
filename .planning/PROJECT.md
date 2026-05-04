# Arb Bot — Unified Arbitrage System

## What This Is

A multi-strategy arbitrage platform that monitors funding rate differentials and cross-exchange price dislocations across 6 exchanges (Binance, Bybit, Gate.io, Bitget, OKX, BingX) and executes delta-neutral positions. **v1.0** shipped a unified platform with two working engines (perp-perp + spot-futures), performance analytics, unified capital allocation, and spot-futures risk hardening. **v2.0** added a third engine (cross-exchange price-gap arbitrage) as a paper-mode tracker. **v2.1** shipped dashboard candidate CRUD and bidirectional candidate mode. **v2.2** promotes Strategy 4 from paper to live capital with conservative ramp, ships the deferred auto-discovery + auto-promotion pipeline, closes paper-mode bugs, and clears v1.0 tech debt completely.

## Core Value

"I deposit USDT, select my risk preference, and the system automatically finds opportunities across multiple strategies, opens positions, collects yield, exits when profitable, and I can see exactly how much each position earned — with capital shifting between strategies as opportunities shift."

## Current Milestone: v2.2 Auto-Discovery & Live Strategy 4

**Goal:** Promote Strategy 4 from paper observation to live capital with a conservative ramp, ship the deferred auto-discovery + auto-promotion pipeline (Phases 11+12 from v2.1 backlog), close out remaining paper-mode bugs, and clear v1.0 tech debt completely.

**Target features:**
- Strategy 4 live capital — conservative ramp: 100 USDT/leg → 500 USDT/leg after 7 clean days → hard ceiling 1000 USDT/leg for v2.2. Telegram alert per fill, daily PnL reconcile.
- Auto-discovery scanner (PG-DISC-01) — surfaces candidates from market data with score + reasoning. Default OFF behind config switch.
- Auto-promotion (PG-DISC-02) — score ≥ `PriceGapAutoPromoteScore` auto-appends to `cfg.PriceGapCandidates`, capped by new `PriceGapMaxCandidates` (default 12). Persisted to `config.json`. Telegram + WS broadcast on promote/demote.
- Discovery telemetry (PG-DISC-03) — scanner cycle metrics, candidate score history, dashboard visibility.
- Paper-mode bug closure — fix `realized_slippage_bps` machine-zero, diagnose paper_mode auto-flip on dashboard load, promote `cmd/bingxprobe/` to `make probe-bingx`.
- v1.0 tech-debt full sweep — Phase 07 retrospective VERIFICATION.md + VALIDATION.md (SF-RISK-01); Nyquist Wave-0 for phases 01/03/04/06; browser confirmations for phases 02/03/05/06.

**Key context:**
- Strategy 4 has been in paper mode since v2.0 (2026-04-25) — observation window has 3+ days of data; live ramp is gated on conservative caps, not just promotion alone.
- Auto-discovery deferred twice (v2.1 → v2.2). Phase numbering continues from v2.1 → v2.2 starts at **Phase 14** (Phases 11+12 reused per original deferred numbering, 13 already used by v2.1 closure).
- v1.0 tech debt has carried through v2.0 + v2.1; this milestone closes it.
- Live trading risk: existing perp-perp + spot-futures + Strategy 4 paper engines must remain undisturbed.

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

### Active (v2.2 candidates)

**Strategy 4 Live Capital (Priority 1)**
- [ ] Conservative ramp budget controller — start 100 USDT/leg, scale to 500 USDT/leg after 7 clean days, hard ceiling 1000 USDT/leg for v2.2
- [ ] Telegram alert per Strategy 4 live fill (entry + exit, both legs)
- [ ] Daily PnL reconcile job for Strategy 4 live positions
- [x] PG-LIVE-02 — Drawdown circuit breaker — auto-revert to paper if 24h realized PnL drawdown exceeds threshold. _Validated in Phase 15: drawdown circuit breaker (v0.38.0, 2026-05-01)._

**Auto-Discovery & Promotion (Priority 1)**
- [ ] PG-DISC-01 — Auto-discovery scanner surfaces candidates with score + reasoning. Default OFF.
- [x] PG-DISC-02 — Auto-promotion to `cfg.PriceGapCandidates` with score gate + max cap. Persisted to `config.json`. WS + Telegram on promote/demote. _Validated in Phase 12: auto-promotion (v0.36.0, 2026-04-30)._
- [ ] PG-DISC-03 — Discovery telemetry: scanner cycle metrics, candidate score history, dashboard observability.

**Paper-Mode Bug Closure (Priority 2)**
- [x] PG-FIX-01 — Fix `realized_slippage_bps` machine-zero in paper mode (formula bug). _Validated in Phase 16: BBO mid-at-exit replaces ModeledSlipBps band-aid (v0.38.1, 2026-05-02)._
- [x] PG-FIX-02 — Diagnose + fix dashboard auto-POST flipping `paper_mode=false` on page load. _Validated in Phase 16: HTTP 409 server guard requires `operator_action: true` (v0.38.2, 2026-05-02)._
- [x] DEV-01 — Promote `cmd/bingxprobe/` to `make probe-bingx`. _Validated in Phase 16: ticker-only safe variant (TestOrder rejected as unsafe per D-11 fallback) (v0.38.3, 2026-05-02)._
- [x] PG-OPS-09 — Strategy 4 Configuration card consolidation on Price-Gap dashboard tab. _Validated in Phase 16: 4-subsection ConfigCard, 4 legacy Config.tsx controls migrated, EN+zh-TW lockstep (v0.39.0, 2026-05-04)._

**v1.0 Tech Debt — Full Sweep (Priority 2)**
- [ ] Phase 07 retrospective VERIFICATION.md + VALIDATION.md (SF-RISK-01)
- [ ] Nyquist Wave-0 validations for phases 01, 03, 04, 06
- [ ] Browser confirmations for phases 02, 03, 05, 06 (human_needed verif cleanup)

**Documentation (Priority 3)**
- [ ] `/gsd-docs-update` pass to refresh CHANGELOG / ARCHITECTURE post-v2.2 (CLAUDE.local.md remains hand-owned)

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

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-05-04 — Phase 16 (Paper-Mode Cleanup + Dashboard Consolidation) shipped on v0.39.0. PG-FIX-01 replaces the paper-mode `RealizedSlipBps = ModeledSlipBps` band-aid with BBO mid-at-exit (v0.38.1); PG-FIX-02 installs a 409 server guard at `/api/config` requiring `operator_action: true` for paper_mode writes (v0.38.2); DEV-01 restores `cmd/bingxprobe/` as a ticker-only probe (TestOrder rejected as unsafe per D-11 fallback) with `make probe-bingx` (v0.38.3); PG-OPS-09 consolidates ALL Strategy 4 configuration into a 4-subsection card at the top of the Price-Gap tab, migrating 4 legacy Config.tsx controls and wiring `togglePaper` with `operator_action: true` (v0.39.0). EN + zh-TW i18n lockstep, npm + config.json lockdown preserved. Operator UATs (16-03, 16-04) approved 2026-05-04 with 2 partial human-verif items (HAR re-capture, probe stdout) tracked in `16-HUMAN-UAT.md`.*
