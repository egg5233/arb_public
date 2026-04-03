# Roadmap: Arb Bot -- Unified Arbitrage System

## Overview

This milestone takes the system from "perp-perp is live, spot-futures barely works on one exchange" to "both engines are production-stable on all exchanges, protected by safety nets, with full performance visibility and intelligent capital allocation." The ordering follows a strict dependency chain: exchanges must work before automation, engines must be stable before analytics are meaningful, and analytics must exist before capital allocation can be informed.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Spot-Futures Exchange Expansion** - Dir A and Dir B full lifecycle verified on all 5 margin exchanges
- [ ] **Phase 2: Spot-Futures Automation** - Auto-discovery, auto-open, auto-exit pipeline for spot-futures
- [ ] **Phase 3: Operational Safety** - Telegram alerts, circuit breakers, and loss limits protecting the live system
- [ ] **Phase 4: Performance Analytics** - SQLite-backed PnL decomposition, strategy comparison, and dashboard charts
- [ ] **Phase 5: Capital Allocation** - Unified capital pool with risk profiles and dynamic strategy-weighted allocation

## Phase Details

### Phase 1: Spot-Futures Exchange Expansion
**Goal**: Both spot-futures directions (Dir A borrow-sell-long, Dir B buy-spot-short) work reliably on all 5 margin exchanges
**Depends on**: Nothing (Bybit already works, expanding to remaining 4)
**Requirements**: SF-01, SF-02, SF-03
**Success Criteria** (what must be TRUE):
  1. User can manually trigger a Dir A (borrow-sell-long) position on each of Binance, Gate.io, Bitget, and OKX, and it opens successfully with correct margin and borrow behavior
  2. User can manually trigger a Dir B (buy-spot-short) position on each of Binance, Gate.io, Bitget, and OKX, and it opens successfully
  3. Auto-borrow on entry and auto-repay on exit work correctly on each exchange using exchange-native mechanisms (per-order flags on Bybit/Binance/Bitget/Gate.io; account-level autoLoan on OKX via API)
  4. Each exchange's positions can be cleanly exited (repay completes, no residual borrows, margin released)
**Plans**: 3 plans

Plans:
- [x] 01-01-PLAN.md -- Enhance livetest harness + verify Bybit (reference implementation)
- [x] 01-02-PLAN.md -- Verify and fix Binance + Bitget (separate margin accounts)
- [x] 01-03-PLAN.md -- Verify and fix Gate.io + OKX (unified accounts, OKX account-level autoLoan)

### Phase 2: Spot-Futures Automation
**Goal**: The spot-futures engine autonomously discovers, opens, and exits positions without manual intervention
**Depends on**: Phase 1
**Requirements**: SF-04, SF-05, SF-06, SF-07
**Success Criteria** (what must be TRUE):
  1. System automatically discovers and ranks spot-futures opportunities across all 5 exchanges, visible in the Opportunities dashboard
  2. System auto-opens the best spot-futures opportunity when it meets entry thresholds, without user clicking anything
  3. Positions auto-exit when exit conditions are met, handling blackout windows (e.g., Bybit :04-:05:30), partial fills, and emergency close scenarios
  4. Entry is rejected when basis/spread is too wide, and exit is gated by spread threshold -- user can see the spread reason in logs
**Plans**: TBD

Plans:
- [ ] 02-01: TBD
- [ ] 02-02: TBD
- [ ] 02-03: TBD

### Phase 3: Operational Safety
**Goal**: The live trading system has safety nets that prevent catastrophic losses and keep the operator informed
**Depends on**: Nothing (protects existing perp-perp engine; independent of spot-futures phases)
**Requirements**: PP-01, PP-02, PP-03
**Success Criteria** (what must be TRUE):
  1. User receives Telegram notifications for perp-perp critical events: position opened, position closed, SL triggered, errors exceeding threshold
  2. When an exchange API starts failing repeatedly (error rate or latency spike), the circuit breaker pauses new entries on that exchange and the dashboard shows the exchange as "circuit open"
  3. When daily or weekly realized loss exceeds the configured threshold, the system halts all new entries and the user sees a "loss limit breached" status on the dashboard
**Plans**: TBD

Plans:
- [x] 03-01-PLAN.md -- Perp-perp Telegram notifications with per-event-type cooldown
- [x] 03-02-PLAN.md -- Rolling loss limit system (Redis sorted sets, pre-entry gate)
- [x] 03-03-PLAN.md -- Config 6-touch-points, Dashboard Safety tab, Overview banner, i18n, version bump

### Phase 4: Performance Analytics
**Goal**: The user can see exactly how much each position earned, compare strategy performance, and track cumulative PnL over time
**Depends on**: Phase 1, Phase 2 (needs stable engines producing reliable trade data)
**Requirements**: PP-04, AN-01, AN-02, AN-03, AN-04, AN-05, AN-06
**Success Criteria** (what must be TRUE):
  1. User can view any closed position's detailed PnL breakdown: entry fees, funding earned, exit fees, basis gain/loss, borrow cost (spot-futures), and net PnL
  2. User can compare perp-perp vs spot-futures returns over a configurable time window on the dashboard
  3. User can view per-exchange metrics (profit, slippage, fill rate, error rate) to identify which exchanges perform best
  4. Dashboard shows a cumulative PnL chart over time, backed by SQLite time-series storage
  5. User can see APR and win rate metrics segmented by strategy and exchange
**Plans**: 4 plans
**UI hint**: yes

Plans:
- [x] 04-01-PLAN.md -- Backend data layer: position model enrichment, SQLite time-series store, aggregator (APR, win rate, exchange metrics)
- [x] 04-02-PLAN.md -- Frontend dependency setup: add Recharts via controlled npm lockfile update (user approval checkpoint)
- [x] 04-03-PLAN.md -- Backend integration: analytics API endpoints, PnL decomposition in reconcilePnL, BBO slippage capture, snapshot writer, Redis backfill
- [x] 04-04-PLAN.md -- Frontend: Analytics page (PnL chart, strategy comparison, exchange metrics), History drill-down, sidebar nav, i18n

### Phase 5: Capital Allocation
**Goal**: The user deposits USDT once, selects a risk preference, and the system intelligently distributes capital across both strategies and all exchanges
**Depends on**: Phase 4 (needs performance data to inform allocation weights)
**Requirements**: CA-01, CA-02, CA-03, CA-04
**Success Criteria** (what must be TRUE):
  1. User configures a single capital pool and the system allocates across perp-perp and spot-futures on all exchanges -- no manual per-engine capital configuration needed
  2. User can select a risk profile (conservative/balanced/aggressive) and the system adjusts position sizes, max positions, entry thresholds, and allocation weights accordingly
  3. Allocation weights between strategies adjust based on recent performance metrics from Phase 4 analytics
  4. When one strategy has no opportunities (e.g., no spot-futures candidates), the system dynamically shifts available capital to the other strategy
**Plans**: TBD

Plans:
- [ ] 05-01: TBD
- [ ] 05-02: TBD
- [ ] 05-03: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3 -> 4 -> 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Spot-Futures Exchange Expansion | 0/3 | Not started | - |
| 2. Spot-Futures Automation | 0/3 | Not started | - |
| 3. Operational Safety | 3/3 | Complete (pending human verify) | 2026-04-03 |
| 4. Performance Analytics | 0/4 | Not started | - |
| 5. Capital Allocation | 0/3 | Not started | - |
