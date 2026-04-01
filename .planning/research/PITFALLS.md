# Domain Pitfalls

**Domain:** Funding Rate Arbitrage Feature Development
**Researched:** 2026-04-01

## Critical Pitfalls

Mistakes that cause rewrites or major issues.

### Pitfall 1: Building Analytics Before Both Engines Are Stable
**What goes wrong:** Analytics code is built around one engine's data model, then the other engine's model doesn't fit. Or worse, metrics are collected from a partially-working spot-futures engine and guide bad capital allocation decisions.
**Why it happens:** Analytics feels like quick wins (dashboards are visible progress). But analytics on unreliable data is misleading.
**Consequences:** Capital allocation weights derived from incomplete spot-futures performance. Refactoring analytics when both engines are stable. False confidence in strategy comparison.
**Prevention:** Complete spot-futures lifecycle on all 5 exchanges first. Only then build cross-strategy analytics.
**Detection:** If you find yourself adding "if strategy == spot_futures { skip }" conditionals in analytics code, the engine isn't ready.

### Pitfall 2: Activating Capital Allocator Without Performance Data
**What goes wrong:** The allocator splits capital 50/50 between strategies, but one strategy has 3x better risk-adjusted returns. Or one strategy is consistently losing to basis drag, but the allocator keeps feeding it capital.
**Why it happens:** The allocator infrastructure exists (Reserve/Commit/Release) and it's tempting to "just turn it on." But static percentage splits (MaxPerpPerpPct, MaxSpotFuturesPct) without performance feedback are arbitrary.
**Consequences:** Capital locked in underperforming strategy. Missed opportunities in better strategy. In the worst case, the losing strategy draws down principal while the winning strategy is capital-starved.
**Prevention:** Collect at least 2-4 weeks of per-strategy performance data before activating dynamic allocation. Start with manual percentage splits informed by actual numbers.
**Detection:** If allocation weights haven't changed since initial config and you have 20+ trades of history, the allocator is running blind.

### Pitfall 3: Exchange API Behavior Differences in Spot-Futures
**What goes wrong:** Borrow mechanics that work on Bybit fail on Binance or Gate.io. Each exchange has different auto-borrow modes, margin modes, borrow rate APIs, repay timing, and error message formats.
**Why it happens:** The project already experienced this: v0.22.44-v0.22.49 were a stream of bugs found only during live testing (integer precision, margin health, auto-borrow). The first live trade on 2026-04-01 revealed multiple issues same day.
**Consequences:** Positions open but can't close. Borrows taken but repay fails. Margin calculations wrong, leading to liquidation.
**Prevention:** Per-exchange integration testing with real (small) capital before enabling production-size trades. The 6-agent parallel audit pattern (from user_team_patterns.md) for each exchange's margin API surface.
**Detection:** First 3-5 trades on each new exchange should be manually monitored with small capital.

### Pitfall 4: Time-Series Storage Bolt-On Instead of Design-First
**What goes wrong:** PnL history, cumulative charts, and performance metrics are hacked into Redis lists or strings. Redis is fast for key-value state but poor for time-series queries (range scans, aggregations, retention policies).
**Why it happens:** Redis is already the persistence layer, and adding another database feels like over-engineering. But history grows unbounded (currently capped at 1000 entries), and aggregation queries get expensive.
**Consequences:** Dashboard performance degrades as history grows. Can't query "last 30 days of perp-perp PnL by exchange" without loading all history. Storage cap silently drops old data.
**Prevention:** When building analytics, decide on time-series storage upfront: Redis TimeSeries module, SQLite file, or even structured JSON files. Don't retrofit.
**Detection:** If you find yourself writing Go code to iterate Redis lists and compute aggregations that SQL could do in one query, you need a proper datastore.

## Moderate Pitfalls

### Pitfall 1: Overfitting Backtesting Parameters
**What goes wrong:** Backtest with historical funding data, find "optimal" parameters that worked perfectly in the past, deploy to production, and they fail. Funding rate regimes change.
**Why it happens:** Limited historical data + many tunable parameters = classic overfitting. A study spanning 26 exchanges found funding rate dynamics are regime-shifting.
**Prevention:** Use backtesting for sanity checks (does this parameter range not lose money?) not optimization (what's the exact optimal threshold?). Use walk-forward validation, not static backtesting. Full-Kelly is "often too aggressive" for crypto -- use half-Kelly at most.
**Detection:** If your backtest shows 100%+ APR but live performance is 15-20% APR, parameters are overfit.

### Pitfall 2: Risk Profiles That Don't Compose Well
**What goes wrong:** "Conservative" profile sets low leverage, small positions, high entry thresholds. But these interact: low leverage + high threshold means almost no entries. Or "aggressive" profile with 3x leverage + low threshold generates too many positions that all compete for capital.
**Why it happens:** Parameters are interdependent. Entry threshold, position size, max positions, leverage, and min-hold time form a system, not independent dials.
**Prevention:** Test each risk profile against historical data before exposing to user. Ensure each profile generates a reasonable number of entries and has a distinct risk/return character. Start with 2 profiles, not 3.
**Detection:** If a risk profile produces 0 entries in a week, it's too conservative. If it maxes out positions within hours, it's too aggressive.

### Pitfall 3: Ignoring Basis Drag in Performance Metrics
**What goes wrong:** PnL shows positive funding collected, but total position PnL is negative because the spot-futures basis moved against the position at entry and exit. Reporting only funding collected gives false impression of profitability.
**Why it happens:** Funding rate is the visible revenue stream. Basis cost is hidden in entry/exit spreads. Binance addresses this with explicit basis monitoring and -0.1%/+0.1% spread control.
**Prevention:** Always decompose PnL into: funding earned - borrow cost (spot-futures) - entry basis cost - exit basis cost - trading fees = net PnL. Dashboard must show all components.
**Detection:** If total funding collected is consistently positive but realized PnL is negative or near zero, basis drag is eating the funding.

### Pitfall 4: Paper Trading That Doesn't Simulate Execution Realities
**What goes wrong:** Paper trading uses mid-price fills with no slippage, no partial fills, no exchange errors, and no rate limits. Strategy looks great on paper, fails in production.
**Why it happens:** Simulating realistic execution is hard. Order book depth changes every millisecond. Fill probability depends on queue position, size, and volatility.
**Prevention:** Paper trading must include: realistic slippage estimation (use saved order book snapshots), simulated partial fills, fee deduction, and funding rate payment timing. Use actual order book data from WebSocket feeds, not just price data.
**Detection:** If paper PnL is 2x+ better than live PnL with same parameters, the simulation is too optimistic.

## Minor Pitfalls

### Pitfall 1: Dashboard Feature Creep
**What goes wrong:** Every new metric and chart gets added to the dashboard until it's overwhelming and slow.
**Prevention:** Each dashboard addition must answer a specific operational question ("should I change X?" or "is Y broken?"). If a metric doesn't drive a decision, it's noise.

### Pitfall 2: Alerting Fatigue from Telegram Notifications
**What goes wrong:** Too many Telegram notifications desensitize the operator. Critical alerts get lost in routine notifications.
**Prevention:** Categorize alerts into Critical (position at risk, emergency close, liquidation) and Informational (trade opened, funding collected). Only push Critical to Telegram. Show Informational in dashboard only.

### Pitfall 3: Concurrent Config Changes During Active Trades
**What goes wrong:** Changing risk parameters or allocation weights while positions are open can cause immediate exits, re-entries, or allocation violations.
**Prevention:** Config changes should only take effect at the next scan cycle, not immediately. Validate that new config doesn't violate current position state.

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Spot-futures completion | Each exchange has unique margin API quirks (v0.22.44-49 proved this on Bybit alone) | Per-exchange integration testing with small capital; 6-agent audit pattern |
| Operational safety | Circuit breaker thresholds that are too sensitive (pause on normal API errors) or too loose (don't catch real outages) | Start with conservative thresholds (3 consecutive failures = pause), tune based on observed error patterns |
| Performance metrics | Redis as time-series store doesn't scale for analytics queries | Decide on proper storage (SQLite, TimeSeries) upfront; don't retrofit |
| Capital allocation | Static allocation weights without performance feedback | Collect 2-4 weeks of per-strategy data before enabling dynamic allocation |
| Backtesting | Overfitting parameters to historical funding data | Walk-forward validation; backtest for sanity checks, not optimization; half-Kelly max |
| Paper trading | Over-optimistic simulation without execution realities | Use real order book snapshots, simulate slippage, deduct fees, model partial fills |

## Sources

- Project history (v0.22.44-v0.22.49 bug sequence during Bybit spot-futures launch) -- HIGH confidence
- CONCERNS.md analysis (TOCTOU race, missing circuit breaker, no Redis backup) -- HIGH confidence
- [ScienceDirect: Risk and Return Profiles of Funding Rate Arbitrage on CEX and DEX](https://www.sciencedirect.com/science/article/pii/S2096720925000818) -- HIGH confidence (academic research)
- [PixelPlex Crypto Arbitrage Bot Development 2026](https://pixelplex.io/blog/crypto-arbitrage-bot-development/) -- MEDIUM confidence
- [3Commas Comprehensive Backtesting Guide](https://3commas.io/blog/comprehensive-2025-guide-to-backtesting-ai-trading) -- MEDIUM confidence
- [Kelly Criterion for Crypto Trading](https://medium.com/@jpolec_72972/position-sizing-strategies-for-algo-traders-a-comprehensive-guide-c9a8fc2443c8) -- MEDIUM confidence
- [Binance Funding Rate Arbitrage Bot (spread control details)](https://www.binance.com/en/support/faq/f330e17d6fc04679b9b21d6f9350e787) -- HIGH confidence
