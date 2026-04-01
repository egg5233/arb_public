# Requirements: Arb Bot -- Unified Arbitrage System

**Defined:** 2026-04-01
**Core Value:** "I deposit USDT, select my risk preference, and the system automatically finds opportunities across both strategies, opens positions, collects funding, exits when profitable, and I can see exactly how much each position earned."

## v1 Requirements

Requirements for Milestone 1. Each maps to roadmap phases.

### Spot-Futures Engine

- [ ] **SF-01**: Dir A (borrow-sell-long) full lifecycle works on all 5 exchanges (Binance, Bybit, Gate.io, Bitget, OKX)
- [ ] **SF-02**: Dir B (buy-spot-short) full lifecycle works on all 5 exchanges
- [ ] **SF-03**: Auto-borrow/auto-repay verified and working per exchange (using exchange-native margin order flags)
- [ ] **SF-04**: Auto-discovery pipeline finds and ranks spot-futures opportunities across all 5 exchanges
- [ ] **SF-05**: Auto-open pipeline executes best opportunities without manual intervention
- [ ] **SF-06**: Auto-exit handles all edge cases (blackout windows, partial fills, emergency close, pending repay retry)
- [ ] **SF-07**: Basis/spread control on entry (reject if spread too wide) and exit (threshold-gated close)

### Perp-Perp Operational Safety

- [ ] **PP-01**: Telegram notifications for perp-perp engine critical events (entry, exit, errors, SL triggers)
- [ ] **PP-02**: Circuit breaker pauses trading on an exchange when error rate or latency exceeds threshold
- [ ] **PP-03**: Daily and weekly loss limits halt new entries when threshold breached
- [ ] **PP-04**: Trade history dashboard shows per-position detailed breakdown (entry/exit prices, fees, funding collected, hold time, PnL components)

### Performance Analytics

- [ ] **AN-01**: Per-position PnL decomposition: entry fees, funding earned, exit fees, basis gain/loss, net PnL
- [ ] **AN-02**: Strategy-level performance comparison: perp-perp vs spot-futures returns over configurable time window
- [ ] **AN-03**: Exchange-level metrics: profit, slippage, fill rate, error rate per exchange
- [ ] **AN-04**: Cumulative PnL chart over time with SQLite time-series storage
- [ ] **AN-05**: APR calculation for perp-perp positions (funding collected vs capital deployed vs hold time)
- [ ] **AN-06**: Win rate and average win/loss segmented by strategy and exchange

### Capital Allocation

- [ ] **CA-01**: Unified capital pool -- single deposit, system allocates across both strategies and all exchanges
- [ ] **CA-02**: Risk preference profiles (conservative/balanced/aggressive) that bundle position size, max positions, entry thresholds, and allocation weights
- [ ] **CA-03**: Strategy-weighted allocation dynamically adjusts perp-perp vs spot-futures split based on recent performance metrics
- [ ] **CA-04**: Dynamic rebalancing shifts capital to available strategies when one strategy has no opportunities (e.g., spot-futures unavailable -> increase perp-perp allocation)

## v2 Requirements

Deferred to next milestone. Tracked but not in current roadmap.

### Cross-Exchange Spot-Futures

- **XSF-01**: Borrow on exchange A, sell spot on exchange A, long futures on exchange B
- **XSF-02**: Buy spot on exchange A, short futures on exchange B
- **XSF-03**: Coordinated exit across two exchanges with rollback safety

### Backtesting & Paper Trading

- **BT-01**: Historical funding rate replay with configurable parameters against real Loris/CoinGlass data
- **BT-02**: Paper trading mode with simulated order fills (Exchange interface wrapping)
- **BT-03**: A/B config comparison -- run two parameter profiles and compare results
- **BT-04**: Bias-aware backtest reporting (survivorship bias, look-ahead bias, fee model warnings)

## Out of Scope

| Feature | Reason |
|---------|--------|
| Binance Portfolio Margin | Tried and reverted (v0.20.1) -- too complex for limited benefit |
| DEX/on-chain arbitrage | Entirely different architecture (MEV, gas, smart contracts) |
| AI/ML rate prediction | Funding rates are regime-shifting; threshold-based entries outperform overfit models |
| Triangular/statistical arbitrage | Different strategy, would dilute focus |
| Mobile app | Web dashboard sufficient for single-user system |
| Multi-user / copy trading | Personal trading system, not a platform |
| New exchange integrations | 6 exchanges provide sufficient coverage |
| Automated parameter optimization | Overfitting risk with limited historical data |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| SF-01 | Phase 1 | Pending |
| SF-02 | Phase 1 | Pending |
| SF-03 | Phase 1 | Pending |
| SF-04 | Phase 2 | Pending |
| SF-05 | Phase 2 | Pending |
| SF-06 | Phase 2 | Pending |
| SF-07 | Phase 2 | Pending |
| PP-01 | Phase 3 | Pending |
| PP-02 | Phase 3 | Pending |
| PP-03 | Phase 3 | Pending |
| PP-04 | Phase 4 | Pending |
| AN-01 | Phase 4 | Pending |
| AN-02 | Phase 4 | Pending |
| AN-03 | Phase 4 | Pending |
| AN-04 | Phase 4 | Pending |
| AN-05 | Phase 4 | Pending |
| AN-06 | Phase 4 | Pending |
| CA-01 | Phase 5 | Pending |
| CA-02 | Phase 5 | Pending |
| CA-03 | Phase 5 | Pending |
| CA-04 | Phase 5 | Pending |

**Coverage:**
- v1 requirements: 21 total
- Mapped to phases: 21
- Unmapped: 0

---
*Requirements defined: 2026-04-01*
*Last updated: 2026-04-01 after roadmap creation*
