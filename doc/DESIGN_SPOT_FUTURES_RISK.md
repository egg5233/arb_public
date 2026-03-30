# Spot-Futures Arbitrage: Extreme Condition & Risk Design

## User Experience Reference

> "I borrowed on exchange A and deposited on exchange B. The coin short squeezed, price went 10x,
> and my collateral on A was gone." — This happened in a cross-exchange carry trade.

Our strategy runs both legs on the SAME exchange, which helps (unified margin can offset),
but doesn't eliminate the risk entirely.

---

## Two Trade Directions

### Direction A: Negative Funding (Borrow-Sell + Long Perp)
- Borrow coin → sell on spot (synthetic short)
- Long perpetual futures
- Collect: funding from long futures
- Pay: borrow interest

**Extreme risk**: Coin price 10x → spot margin liability explodes → liquidation

### Direction B: Positive Funding (Buy Spot + Short Perp)
- Buy coin on spot (hold)
- Short perpetual futures
- Collect: funding from short futures
- Pay: nothing (just capital locked)

**Extreme risk**: Coin price 10x → futures short liquidation → spot holding can't offset fast enough

---

## Risk Tiers

### Tier 0: Pre-Entry Filters (Prevention)

| Filter | Purpose | Default |
|--------|---------|---------|
| Min market cap | Avoid micro-caps that can squeeze | $10M |
| Min open interest | Avoid illiquid contracts | $500K |
| Max borrow utilization | Don't borrow when pool is almost empty (squeeze signal) | 80% |
| Max position size vs OI | Don't be a significant % of open interest | 5% of OI |
| Coin blacklist | Manual ban list for known volatile coins | configurable |
| Max leverage on futures leg | Lower leverage = wider liquidation distance | 3x |

### Tier 1: Real-Time Price Monitoring (Early Warning)

Run a goroutine that monitors price movement every 10 seconds for all active spot-futures positions:

```
priceChangePercent = (currentPrice - entryPrice) / entryPrice * 100
```

| Price Move | Severity | Action |
|------------|----------|--------|
| >10% in 1h | Warning | Log alert, notify dashboard |
| >20% in 1h | Critical | Begin exit immediately |
| >30% in 1h | Emergency | Market-close both legs NOW (skip depth fill) |
| >50% in 1h | Panic | Same + halt all new entries for this coin |

For Direction A (borrow-sell), upward moves are dangerous — the borrowed-and-sold
spot must be bought back at a higher price, and margin utilization increases.

For Direction B (buy-spot + short), upward moves are dangerous — the short futures
position faces liquidation risk on a squeeze. Down moves are actually profitable
for the futures short and do NOT trigger price-spike exits. (The spot long is safe;
you own the coin outright with no leverage.)

### Tier 2: Margin Health Monitoring

#### Direction A (Borrow-Sell):
- Monitor spot margin utilization: `borrowed_value / collateral_value`
- Exchange will auto-liquidate the borrow if margin ratio hits threshold
- Our trigger: exit BEFORE exchange liquidation

| Margin Utilization | Action |
|-------------------|--------|
| >70% | Warning alert |
| >85% | Begin exit |
| >95% | Emergency close — market orders |

How to check: `GetMarginBalance(coin)` → compute `Borrowed * currentPrice / Available`

#### Direction B (Buy-Spot + Short):
- Monitor futures margin ratio (existing health monitor handles this)
- Futures short liquidation is the main risk
- The spot holding is safe (you own the coin, no leverage)

### Tier 3: Emergency Exit Procedure

When an extreme move is detected, the exit must be FAST, not depth-optimal:

```
EMERGENCY EXIT (Direction A — borrow-sell + long perp):

Step 1: Close futures long IMMEDIATELY (market IOC, reduce-only)
        → This realizes the profit that offsets the borrow loss

Step 2: Buy back coin on spot IMMEDIATELY (market IOC)
        → Even at inflated price — the futures profit covers it

Step 3: Repay loan
        → If we can't buy enough coin, repay what we can
        → Remaining debt will be auto-liquidated by exchange

TOTAL TIME TARGET: <5 seconds for steps 1-2
```

```
EMERGENCY EXIT (Direction B — buy spot + short perp):

Step 1: Close futures short IMMEDIATELY (market IOC, reduce-only)
        → Stop the bleeding on the losing side

Step 2: Sell spot holding (market IOC)
        → Realize the offsetting profit

Step 3: No loan to repay — done

TOTAL TIME TARGET: <5 seconds for steps 1-2
```

**Critical**: In emergency mode, do NOT use depth-fill loops. Use single market orders.
Accept slippage — survival > optimization.

### Tier 4: Account-Level Cross-Leg Protection

#### Unified Account Exchanges (Bybit, OKX, Gate.io):
- Futures PnL automatically offsets spot margin liability
- A 10x move on futures long = +900% profit, which covers the spot margin borrow loss
- The exchange's risk engine sees the COMBINED position
- Lower risk — the exchange won't liquidate spot margin if futures profit covers it
- **BUT**: if the exchange's risk calculation has delays, the spot margin leg might get
  liquidated before the futures profit is recognized

#### Separate Account Exchanges (Binance, Bitget):
- Futures profit sits in futures account, spot margin liability in margin account
- They DON'T automatically offset
- A squeeze can liquidate spot margin while futures profit is trapped
- **HIGHER RISK** — must exit manually before liquidation
- **Mitigation**: Set lower position limits on these exchanges
- **Mitigation**: Transfer futures profit to margin account periodically (every hour)
  - But transfers take time and may not be instant during extreme moves

### Tier 5: Worst-Case Scenarios & Responses

#### Scenario 1: 10x Squeeze in 5 Minutes (Direction A)
- Entry: Borrowed 1000 COIN at $1, sold for $1000 USDT
- Now: COIN = $10, borrow liability = $10,000
- Futures long: entry $1, now $10, unrealized PnL = +$9,000
- Net: should be fine (+$9000 - $9000 = ~breakeven minus fees)
- **RISK**: Exchange liquidates spot margin at 5x ($5) before we can close futures
- **Response**: Tier 1 price monitor catches >100% move → emergency exit at 2-3x
- **Config**: `max_price_move_pct: 100` → exit at 2x, not wait for 10x

#### Scenario 2: Flash Crash then Recovery (Direction B)
- Entry: Bought 1000 COIN at $1, shorted futures at $1
- Flash crash: COIN = $0.10 in 2 minutes
- Futures short: +$900 profit (good)
- Spot holding: -$900 loss (bad, but you still hold the coins)
- Recovery: COIN back to $0.80 in 10 minutes
- If we panicked and sold spot at $0.10, we realized the loss
- **Response**: For Direction B, spot is just holding — no liquidation risk
- Only futures side has liquidation risk, and that's profitable in a crash
- **Direction B is naturally safer** for flash crashes

#### Scenario 3: Borrow Rate Spikes to 1000% APR
- Exchange raises borrow rate because everyone is borrowing
- Our hourly cost jumps from $0.01 to $1.00
- **Response**: Tier 2 borrow rate monitor catches it → trigger exit within 1 check interval
- **Config**: `max_borrow_rate_hourly: 0.005` (= 43.8% APR) → exit above this

#### Scenario 4: Exchange Halts Withdrawals/Trading
- Can't close positions during maintenance
- **Response**: Nothing we can do — same risk as perp-perp arb
- **Mitigation**: Don't hold spot-futures positions over known maintenance windows

---

## Configuration

```json
{
  "spot_futures": {
    "risk": {
      "min_market_cap_usd": 10000000,
      "min_oi_usd": 500000,
      "max_borrow_utilization_pct": 80,
      "max_position_pct_of_oi": 5,
      "max_leverage": 3,
      "price_monitor_interval_sec": 10,
      "price_alert_pct_1h": 10,
      "price_exit_pct_1h": 20,
      "price_emergency_pct_1h": 30,
      "margin_warning_pct": 70,
      "margin_exit_pct": 85,
      "margin_emergency_pct": 95,
      "max_borrow_rate_hourly": 0.005,
      "emergency_exit_timeout_sec": 10,
      "separate_account_max_notional": 200,
      "unified_account_max_notional": 500,
      "coin_blacklist": []
    }
  }
}
```

---

## Summary: Risk Priority by Exchange Type

| Exchange | Account Type | Max Recommended Size | Key Risk |
|----------|-------------|---------------------|----------|
| Bybit | Unified (UTA) | $500 | Low — cross-leg offset works |
| OKX | Unified (multi-currency) | $500 | Low — cross-leg offset works |
| Gate.io | Unified (multi-currency) | $500 | Low — cross-leg offset works |
| Binance | Separate (margin ≠ futures) | $200 | HIGH — no auto-offset between accounts |
| Bitget | Separate (margin ≠ futures) | $200 | HIGH — no auto-offset between accounts |

**Recommendation**: Start with unified-account exchanges (Bybit/OKX/Gate.io) only.
Add Binance/Bitget later with lower limits and hourly profit-to-margin transfers.
