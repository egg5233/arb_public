# Graph Report - ./doc  (2026-04-07)

## Corpus Check
- Large corpus: 54 files · ~842,714 words. Semantic extraction will be expensive (many Claude tokens). Consider running on a subfolder, or use --no-semantic to run AST-only.

## Summary
- 122 nodes · 183 edges · 8 communities detected
- Extraction: 87% EXTRACTED · 13% INFERRED · 0% AMBIGUOUS · INFERRED: 23 edges (avg confidence: 0.82)
- Token cost: 0 input · 0 output

## God Nodes (most connected - your core abstractions)
1. `Docs Index` - 16 edges
2. `Bitget Docs Index` - 14 edges
3. `Gate.io Docs Index` - 10 edges
4. `Spot-Futures Arbitrage Risk Design` - 8 edges
5. `BingX Perpetual Swap (USDT-M) API Documentation` - 8 edges
6. `Binance Docs Index` - 7 edges
7. `Bybit Docs Index` - 7 edges
8. `OKX Docs Index` - 6 edges
9. `BingX Coin-M Futures API Documentation` - 6 edges
10. `Gate.io Perpetual Futures API Documentation` - 6 edges

## Surprising Connections (you probably didn't know these)
- `Binance Spot API Documentation` --semantically_similar_to--> `Bybit Market Data API`  [INFERRED] [semantically similar]
  doc/binance/binance-spot-api-docs.md → doc/bybit/bybit-market-api-docs.md
- `Binance Wallet API Documentation` --semantically_similar_to--> `BingX Account, Wallet & Sub-Account API Documentation`  [INFERRED] [semantically similar]
  doc/binance/binance-wallet-api-docs.md → doc/bingx/bingx-other-api-docs.md
- `Bybit Order API` --semantically_similar_to--> `BingX Perpetual Swap (USDT-M) API Documentation`  [INFERRED] [semantically similar]
  doc/bybit/bybit-order-api-docs.md → doc/bingx/bingx-swap-api-docs.md
- `Spot trading and spot leg behavior for spot-futures` --semantically_similar_to--> `Spot-market metadata, balances, and transfer support`  [INFERRED] [semantically similar]
  doc/gate/README.md → doc/bitget/README.md
- `Gate account-mode split between classic and unified flows` --semantically_similar_to--> `Cross-margin spot-futures operations`  [INFERRED] [semantically similar]
  doc/gate/README.md → doc/bitget/README.md

## Hyperedges (group relationships)
- **Unified Margin Exchange Family** — exchangeapi_okx_margin, exchangeapi_gateio_unified, exchangeapi_gateio_margin, exchangeapi_bybit_margin [INFERRED 0.86]
- **Separate Margin Shorting Family** — exchangeapi_binance_margin, spec_binance_spot_margin, exchangeapi_bitget_margin [INFERRED 0.83]
- **Spot-Futures Risk Flow** — spot_futures_negative_funding_synthetic_short, spot_futures_positive_funding_long_spot_short_perp, spot_futures_account_level_cross_leg_protection [INFERRED 0.85]
- **BingX Shared Reference Stack** — exchange_base_urls, exchange_authentication_notes, bingx_perpetual_swap_usdt_m_api, bingx_spot_trading_api, bingx_coin_m_futures_api, bingx_account_wallet_sub_account_api [INFERRED 0.88]
- **Binance Spot-Futures Support Stack** — binance_usdt_m_futures_api, binance_spot_api, binance_margin_trading_api, binance_wallet_api [INFERRED 0.78]
- **Bybit UTA Spot-Futures Stack** — bybit_market_data_api, bybit_order_api, bybit_position_api, bybit_account_api, bybit_spot_margin_uta_api, bybit_websocket_api [INFERRED 0.82]
- **Gate margin-aware spot-futures support stack** — gate_spot_api_documentation, gate_isolated_margin_api_documentation, gate_multi_collateral_loan_api_documentation [INFERRED 0.83]
- **Bitget spot-futures support stack** — bitget_spot_api_documentation, bitget_margin_api_documentation, bitget_futures_api_documentation [INFERRED 0.84]

## Communities

### Community 0 - "Margin Model References"
Cohesion: 0.09
Nodes (29): Binance Cross Margin Borrow/Repay, Binance Cross vs Isolated Margin, Binance Universal Transfer, BingX One-Way Position Mode, BingX Docs Index, Bitget Cross Margin Borrow/Repay, Bitget Product Types, Bybit Repay Conversion Blackout (+21 more)

### Community 1 - "Binance Support Surface"
Cohesion: 0.12
Nodes (21): Binance CMS API Documentation, Binance Crypto Loan API Documentation, Binance Crypto Loan Percent-Encoding Notice, Binance Crypto Loan Separate From Cross-Margin Note, Binance Docs Index, Binance Margin ListenKey Deprecation Notice, Binance Margin Trading API Documentation, Binance Portfolio Margin API Documentation (+13 more)

### Community 2 - "Gate Support Stack"
Cohesion: 0.22
Nodes (17): Futures trading and market data, Gate account-mode split between classic and unified flows, Confirm account-mode assumptions before margin changes, Contract sizing and quanto_multiplier, Gate.io Isolated Margin API Documentation, Margin account state, borrow/repay, and transfer behavior, Gate.io Multi-Collateral Loan API Documentation, Multi-collateral is not a direct substitute for classic spot-margin borrow endpoints (+9 more)

### Community 3 - "Market Data and Auth Rules"
Cohesion: 0.22
Nodes (16): Binance Futures WebSocket Split Notice, Binance Spot API Documentation, Binance Spot Unsigned Public Market Data Rule, Binance USDT-M Futures API Documentation, BingX Account, Wallet & Sub-Account API Documentation, BingX Coin-M Futures API Documentation, BingX Coin-M Market Data Timestamp Note, BingX One-Way Mode Note (+8 more)

### Community 4 - "Bitget Trading Stack"
Cohesion: 0.24
Nodes (15): Cross-margin spot-futures operations, Bitget Futures Trading API Documentation, Futures contract metadata, leverage, and margin mode, Main trading path is futures plus cross-margin, Bitget Margin Trading API Documentation, Bitget market-buy semantics differ across endpoints, Check quantity-vs-notional semantics across futures and spot, Quantity-vs-notional semantics across futures and spot (+7 more)

### Community 5 - "OKX Capability Surface"
Cohesion: 0.17
Nodes (12): OKX Earn/Lending Surface, OKX Financial Product API, OKX Funding Account API, OKX Funding Account Transfers, OKX Order Book Sequence Maintenance, OKX Order Book Trading API, OKX Public Data API, OKX Public Market Metadata (+4 more)

### Community 6 - "Spot-Futures Risk Design"
Cohesion: 0.31
Nodes (9): Binance Spot Margin Borrow-and-Sell, Separate Account Exchanges Higher Risk, Spot-Futures Arbitrage Risk Design, Start With Unified-Account Exchanges, Unified Account Exchanges Lower Risk, Account-Level Cross-Leg Protection, Spot-Futures Arbitrage Risk Model, Negative Funding: Borrow-Sell + Long Perp (+1 more)

### Community 7 - "Bitget Earn Loan"
Cohesion: 1.0
Nodes (3): Bitget Earn Loan API Documentation, Earn-loan borrowing is operationally different from cross-margin borrow, Bitget earn-loan is reference-only

## Knowledge Gaps
- **30 isolated node(s):** `Binance API Documentation Reference`, `Bybit API Documentation Reference`, `Gate.io API Documentation Reference`, `Spot-Futures Arbitrage Risk Model`, `Positive Funding: Buy Spot + Short Perp` (+25 more)
  These have ≤1 connection - possible missing edges or undocumented components.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Docs Index` connect `Margin Model References` to `OKX Capability Surface`, `Spot-Futures Risk Design`?**
  _High betweenness centrality (0.140) - this node is a cross-community bridge._
- **Why does `OKX Docs Index` connect `OKX Capability Surface` to `Margin Model References`?**
  _High betweenness centrality (0.064) - this node is a cross-community bridge._
- **Why does `Spot-Futures Arbitrage Risk Design` connect `Spot-Futures Risk Design` to `Margin Model References`?**
  _High betweenness centrality (0.042) - this node is a cross-community bridge._
- **Are the 3 inferred relationships involving `BingX Perpetual Swap (USDT-M) API Documentation` (e.g. with `Binance USDT-M Futures API Documentation` and `Bybit Order API`) actually correct?**
  _`BingX Perpetual Swap (USDT-M) API Documentation` has 3 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Binance API Documentation Reference`, `Bybit API Documentation Reference`, `Gate.io API Documentation Reference` to the rest of the system?**
  _30 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Margin Model References` be split into smaller, more focused modules?**
  _Cohesion score 0.09 - nodes in this community are weakly interconnected._
- **Should `Binance Support Surface` be split into smaller, more focused modules?**
  _Cohesion score 0.12 - nodes in this community are weakly interconnected._