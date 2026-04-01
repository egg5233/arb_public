# Feature Landscape

**Domain:** Funding Rate Arbitrage (Perp-Perp + Spot-Futures) with Capital Allocation
**Researched:** 2026-04-01
**Overall Confidence:** MEDIUM-HIGH

## Table Stakes

Features users expect from a professional funding rate arbitrage system. Missing = system feels incomplete or untrustworthy for real capital.

### Spot-Futures Engine

| Feature | Why Expected | Complexity | Current Status | Notes |
|---------|--------------|------------|----------------|-------|
| Positive carry (buy spot, short futures) | Core strategy of every exchange-native arb bot (Binance, Bitget, Pionex, OKX all offer this) | Med | Dir B working on Bybit only | Must work on all 5 margin exchanges |
| Reverse carry (sell spot/borrow, long futures) | Bitget launched reverse mode Jan 2026; Binance supports negative carry. Expected in any serious bot | Med | Dir A working on Bybit only | Must work on all 5 margin exchanges |
| Auto-borrow/auto-repay | Every exchange-native bot handles this transparently. Manual borrow is a dealbreaker | Low | Working on Bybit (v0.22.49) | Need to verify on Binance, Gate.io, Bitget, OKX |
| Margin health monitoring + emergency close | Binance monitors uniMMR continuously; Pionex manages liquidation risk automatically | Med | Working (L0-L5 tiers exist) | Solid implementation already in risk/health.go |
| Basis/spread control on entry and exit | Binance default -0.1%/+0.1% spread control; Bitget has configurable entry/exit basis. Table stakes to avoid slippage on open/close | Med | Partial (entry spread checked, exit uses spread reversal) | Exit basis control needs explicit threshold config |
| Auto-discovery of opportunities | All exchange bots auto-scan available pairs. Manual selection is unacceptable | Low | Working (CoinGlass scraper + scoring) | Already implemented |
| Full position lifecycle (open -> monitor -> close) | Every bot manages the complete lifecycle. Hanging positions that need manual intervention are failures | High | Partially working (Bybit only, pending recovery exists but untested on other exchanges) | Priority 1 blocker |

### Perp-Perp Engine

| Feature | Why Expected | Complexity | Current Status | Notes |
|---------|--------------|------------|----------------|-------|
| Delta-neutral position management | Fundamental to the strategy. Both legs must stay balanced | Low | Working | Core feature, solid |
| Automated entry based on funding rate spreads | Standard in all cross-exchange arb systems | Low | Working (executeArbitrage pipeline) | Solid |
| Automated exit with multiple triggers | Spread reversal, time-based, rate flip -- all standard | Med | Working (checkExitsV2, spread reversal, zero-spread, min-hold) | Solid multi-trigger system |
| Position rotation between exchanges | Standard for maximizing yield when rates shift | Med | Working (checkRotations) | Competitive feature |
| Stop-loss / liquidation detection | Every trading bot needs SL. Exchange-side liquidation must be detected | Med | Working (dual SL detection: slIndex + ReduceOnly) | Good implementation |
| Funding fee tracking per position | Binance shows total funding accumulated; every bot tracks this | Low | Working (trackFunding goroutine) | Basic implementation |
| Exchange balance monitoring | Must know available capital per exchange | Low | Working (60s refresh cycle) | Basic but functional |

### Dashboard & Operational

| Feature | Why Expected | Complexity | Current Status | Notes |
|---------|--------------|------------|----------------|-------|
| Real-time position view | Every trading platform shows live positions | Low | Working (Overview page, WebSocket) | Solid |
| Trade history with PnL | Altrady, TradesViz, every exchange shows this. Must track per-trade PnL | Low | Working (History page with 15 columns) | Has funding collected, rotation PnL, realized PnL |
| Opportunity scanner view | Standard for rate arb tools (CoinGlass, Loris, P2P.Army) | Low | Working (Opportunities page, perp + spot tabs) | Good |
| Live log streaming | Operational necessity for a self-hosted bot | Low | Working (Logs page) | Good |
| Configuration management via UI | Any bot with a dashboard lets you change settings | Low | Working (Config page, 4 strategy sections) | Good |
| Rejection/filter transparency | Users need to understand why opportunities were skipped | Low | Working (Rejections page) | Uncommon feature, good competitive edge |
| Authentication | Dashboard access must be protected | Low | Working (auth.go, Bearer tokens) | Basic but functional |

### Risk Management

| Feature | Why Expected | Complexity | Current Status | Notes |
|---------|--------------|------------|----------------|-------|
| Position count limits | Standard risk control in all bots | Low | Working (MaxPositions in 11-point check) | Solid |
| Per-exchange exposure caps | Standard for multi-exchange systems | Low | Working (60% cap, bypassed when allocator active) | Solid |
| Slippage estimation and limits | PixelPlex guide: "Size positions appropriately relative to order book depth" | Low | Working (binary search for max tradeable size) | Good |
| Cross-exchange price gap check | Prevents entering during flash crashes or depegs | Low | Working (hard cap + directional recovery limit) | Good |
| Minimum margin floor | Prevents dust-sized positions | Low | Working (10 USDT floor) | Good |

## Differentiators

Features that set the system apart. Not universally expected, but high value for a self-hosted professional tool.

### Capital Allocation Engine

| Feature | Value Proposition | Complexity | Current Status | Notes |
|---------|-------------------|------------|----------------|-------|
| Unified capital pool across strategies | No exchange-native bot does cross-strategy allocation. Binance/Bitget bots are single-strategy | High | Partially built (CapitalAllocator in risk/allocator.go with Reserve/Commit/Release, strategy % caps) | Core infrastructure exists but not actively driving decisions |
| Strategy-weighted allocation (perp-perp vs spot-futures) | Allocate more capital to whichever strategy has better risk-adjusted returns right now | High | Config exists (MaxPerpPerpPct, MaxSpotFuturesPct) but static | Needs dynamic adjustment based on performance |
| Per-exchange capital caps with allocator override | Prevents over-concentration while allowing allocator to optimize | Med | Working (MaxPerExchangePct, allocator checkCaps) | Already implemented |
| Capital reservation with TTL | Prevents double-booking during concurrent entries | Med | Working (5-min TTL reservations in Redis) | Sophisticated, unusual for arb bots |
| Reconciliation from live positions | Self-healing allocator state | Med | Working (Reconcile() rebuilds from active positions) | Good resilience feature |
| Dynamic rebalancing across exchanges | Move capital from idle exchanges to active opportunities | High | Partial (rebalanceFunds exists for perp-perp, not unified) | Cross-strategy rebalancing is the differentiator |

### Performance Analytics

| Feature | Value Proposition | Complexity | Current Status | Notes |
|---------|-------------------|------------|----------------|-------|
| Per-position PnL breakdown (entry fees, funding, exit fees, basis PnL) | TradesViz offers 50+ stats per trade. For arb, need: entry cost, funding earned, exit cost, basis gain/loss, net PnL | Med | Partial (funding_collected, realized_pnl, entry_fees exist; no per-component breakdown in dashboard) | Data mostly captured in Redis, needs dashboard decomposition |
| Strategy-level performance comparison | Compare perp-perp vs spot-futures returns over time. No exchange bot offers this because they only run one strategy | Med | Not built (stats are separate: keyStats for perp, spot:stats for spot) | Need unified analytics layer |
| Exchange-level performance metrics | Which exchanges are most profitable? Which have most slippage? Informes capital allocation | Med | Not built (data exists in trade history but not aggregated) | Can be computed from history |
| Cumulative PnL chart over time | Standard in TradesViz, Altrady. Visual trend of performance | Med | Not built (only total_pnl counter in Stats) | Need time-series storage |
| Win rate and average win/loss by strategy | Basic performance metrics that every trading dashboard shows | Low | Partial (win_count, loss_count, trade_count exist; no per-strategy breakdown) | Needs segmentation |
| APR/APY calculation per position | Pionex and Bitget show estimated APR prominently. Key metric for funding arb | Low | Working for spot-futures (current_net_yield_apr); not computed for perp-perp | Perp-perp needs APR based on funding collected vs capital deployed vs hold time |
| Funding rate heatmap or history | CoinGlass offers this; useful for understanding rate patterns | Med | Not built | Could pull from Loris API historical data |

### Backtesting & Paper Trading

| Feature | Value Proposition | Complexity | Current Status | Notes |
|---------|-------------------|------------|----------------|-------|
| Historical funding rate replay | Test parameter changes (entry thresholds, exit triggers, min-hold time) against real historical data | High | Scaffold exists (internal/discovery/backtest.go) but not connected to engine | Major differentiator -- no exchange-native bot offers parametric backtesting |
| Paper trading mode | Run the full engine pipeline with virtual orders. FundingView recommends "run on testnet first." 3Commas guide says "paper trade 3-6 months before live" | High | Not built | Requires order simulation layer that mirrors exchange behavior |
| A/B config comparison | Run two config profiles side-by-side and compare. Listed in PROJECT.md as active requirement | Med | Not built | Could be implemented as paper vs live, or two paper instances |
| Historical performance attribution | After a losing trade, understand what went wrong: was it basis loss? Rate flip? Slippage? | Med | Not built (exit_reason exists but no detailed attribution) | Useful for parameter tuning |

### Advanced Risk & Position Sizing

| Feature | Value Proposition | Complexity | Current Status | Notes |
|---------|-------------------|------------|----------------|-------|
| Risk preference profiles (conservative/balanced/aggressive) | Pionex offers moderate/aggressive modes. Users want one-click risk profiles | Med | Not built (all params individually configured) | Bundle position size, leverage, entry threshold, max positions into named profiles |
| Adaptive position sizing based on order book depth | PixelPlex: "A $10K opportunity in thin market may only support $2K." Current system does this | Low | Working (binary search slippage estimator) | Already a differentiator |
| Exchange health scoring | Weight opportunities by exchange reliability (latency, error rates, fill rates) | Med | Working (internal/risk/exchange_scorer.go) | Unusual for arb bots |
| Spread stability gate (CV-based) | Reject opportunities with volatile spreads | Med | Working (internal/risk/spread_stability.go, Redis-backed CV) | Uncommon, good risk feature |
| Circuit breaker for exchange-wide failures | Pause trading on degraded exchanges. PixelPlex 2026 guide lists this as essential | Med | Not built (identified in CONCERNS.md as missing) | Important operational safety |
| Daily/weekly loss limits | 3Commas guide: "daily loss thresholds." Standard in professional risk management | Low | Not built | Simple but valuable safety net |
| Correlation monitoring across positions | Multiple positions on same asset across strategies could compound risk | Med | Not built | Important when running both strategies on same symbol |

### Operational Resilience

| Feature | Value Proposition | Complexity | Current Status | Notes |
|---------|-------------------|------------|----------------|-------|
| Telegram notifications for critical events | PixelPlex: "Send notifications when human attention is required." Currently only spot engine has Telegram | Low | Partial (spot engine only, perp engine logs + WebSocket only) | Identified in CONCERNS.md as missing for perp |
| Healthcheck endpoint | Standard for production services behind monitoring (Prometheus, uptime checks) | Low | Not built (identified in CONCERNS.md) | Trivial to add |
| Redis state backup/export | Disaster recovery. Identified as missing critical feature in CONCERNS.md | Med | Not built | Important for a system managing real money |
| Graceful degradation during exchange maintenance | Continue operating on remaining exchanges when one goes down | Med | Partial (positions tracked per-exchange, but no exchange-level circuit breaker) | Ties to circuit breaker feature |

## Anti-Features

Features to explicitly NOT build. These are traps that add complexity without proportional value.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Binance Portfolio Margin integration | Already tried and reverted (v0.20.1). Too complex for limited benefit. Margin requirements are manageable with cross margin | Stick with cross margin on all exchanges. Decision is validated |
| DEX/on-chain funding rate arbitrage | Entirely different architecture (MEV, gas, smart contracts, on-chain settlement). Research shows DEX funding rates are noisier and less predictable | Stay CEX-only. The 6 exchanges provide sufficient opportunity space |
| AI/ML-based rate prediction | Multiple sources warn that funding rates are regime-shifting and noisy. Kelly criterion is "often too aggressive" for crypto. Simple threshold-based entries outperform overfit models | Use historical statistics (spread stability CV, rate velocity) not predictions. Current approach is correct |
| Triangular or statistical arbitrage | Different strategy entirely. Would require new discovery, execution, and risk systems. Dilutes focus | Keep laser focus on funding rate arbitrage (perp-perp + spot-futures). These two strategies share infrastructure naturally |
| Mobile app | Web dashboard accessed via browser is sufficient. Mobile adds build/deploy complexity for a single-user system | Keep web dashboard, ensure it works on mobile browsers (responsive design) |
| Multi-user support | This is a personal trading system. Auth is for access control, not multi-tenancy | Single user, single config. Keep architecture simple |
| Copy trading / social features | Adds regulatory complexity, liability, and infrastructure for minimal value in a personal system | Not applicable to a self-operated bot |
| New exchange integrations (beyond 6) | 6 exchanges already provide comprehensive coverage of funding rate opportunities. Each new exchange adds adapter maintenance burden | Only add exchanges if a specific, recurring opportunity gap is identified |
| Real-time order book visualization | Impressive but expensive (WebSocket bandwidth, rendering). The depth-fill algorithm already handles this programmatically | Keep programmatic depth analysis. Log key depth metrics for post-hoc review |
| Automated parameter optimization (genetic algorithms, etc.) | Overfitting risk is extreme with limited historical funding rate data. Parameters are sensitive to market regime | Manual A/B testing with paper trading mode is safer and more interpretable |

## Feature Dependencies

```
Spot-Futures Full Lifecycle (all 5 exchanges)
  --> Auto-Discovery Pipeline (spot-futures)
  --> Auto-Exit with Edge Cases
  --> Cross-Exchange Spot-Futures (later)

Per-Position PnL Breakdown
  --> Cumulative PnL Chart (needs time-series data)
  --> Strategy-Level Performance Comparison
  --> Exchange-Level Performance Metrics
  --> Historical Performance Attribution

Unified Capital Pool
  --> Strategy-Weighted Allocation
  --> Dynamic Rebalancing Across Exchanges
  --> Risk Preference Profiles (need allocation logic first)

Historical Funding Rate Replay
  --> Paper Trading Mode (can share simulation infrastructure)
  --> A/B Config Comparison (can run two paper instances)

Circuit Breaker for Exchange Failures
  --> Graceful Degradation During Maintenance
  --> Daily/Weekly Loss Limits (both are safety controls)

Telegram Notifications (perp engine)
  --> Healthcheck Endpoint (both are operational monitoring)
  --> Redis State Backup (all operational resilience)
```

## MVP Recommendation (Next Milestone)

**Priority 1 -- Complete what's partially working:**
1. Spot-futures full lifecycle on all 5 margin exchanges (table stakes, currently only Bybit)
2. Telegram notifications for perp engine critical events (operational necessity, low complexity)
3. Circuit breaker for exchange-wide failures (risk safety, medium complexity)

**Priority 2 -- Performance visibility (needed before experimentation):**
4. Per-position PnL breakdown in dashboard (decompose funding, fees, basis)
5. APR calculation for perp-perp positions (parity with spot-futures which already has this)
6. Strategy-level performance comparison (perp-perp vs spot-futures aggregate stats)

**Priority 3 -- Capital allocation activation:**
7. Risk preference profiles (conservative/balanced/aggressive bundles)
8. Activate unified capital allocator for both strategies (infrastructure exists, needs integration)
9. Daily/weekly loss limits (simple safety net)

**Defer to Later Milestone:**
- Backtesting/paper trading: needs significant infrastructure, better served after metrics prove which parameters matter
- Cumulative PnL charts: needs time-series storage (consider PostgreSQL/SQLite before building)
- Cross-exchange spot-futures: fundamentally different architecture, save for dedicated milestone
- Dynamic rebalancing: needs performance data to know where to allocate

## Sources

- [Binance Funding Rate Arbitrage Bot FAQ](https://www.binance.com/en/support/faq/f330e17d6fc04679b9b21d6f9350e787) -- HIGH confidence (official docs)
- [Bitget Reverse Arbitrage Mode](https://www.bitget.com/news/detail/12560605130294) -- HIGH confidence (official announcement)
- [Pionex Spot-Futures Arbitrage Bot (Aggressive Mode)](https://www.pionex.com/blog/pionex-arbitrage-bot/) -- HIGH confidence (official blog)
- [PixelPlex Crypto Arbitrage Bot Development 2026](https://pixelplex.io/blog/crypto-arbitrage-bot-development/) -- MEDIUM confidence (industry analysis)
- [Amberdata Funding Rate Arbitrage Guide](https://blog.amberdata.io/the-ultimate-guide-to-funding-rate-arbitrage-amberdata) -- MEDIUM confidence (data provider analysis)
- [3Commas AI Trading Bot Risk Management Guide](https://3commas.io/blog/ai-trading-bot-risk-management-guide) -- MEDIUM confidence (competitor docs)
- [CoinGlass Funding Rates Arbitrage](https://www.coinglass.com/ArbitrageList) -- HIGH confidence (data platform)
- [FundingView Arbitrage Tool](https://fundingview.app) -- MEDIUM confidence (tool documentation)
- [DEV.to Funding Rate Arbitrage Technical Deep Dive](https://dev.to/27sphere/funding-rate-arbitrage-a-technical-deep-dive-3eed) -- MEDIUM confidence (engineering blog)
- [Gainium Crypto Strategy Testing](https://gainium.io/crypto-strategy-testing) -- MEDIUM confidence (competitor docs)
- [TradesViz Trading Analytics](https://www.tradesviz.com/) -- MEDIUM confidence (analytics platform)
- [Altrady Positions & PnL Tracking](https://www.altrady.com/features/positions-pnl) -- MEDIUM confidence (analytics platform)
- Codebase analysis of `/var/solana/data/arb/` -- HIGH confidence (direct inspection)
- PROJECT.md, ARCHITECTURE.md, CONCERNS.md -- HIGH confidence (project documentation)
