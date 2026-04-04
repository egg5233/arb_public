# Bybit Docs Index

This folder holds the raw Bybit vendor documentation used by the adapter in:

- `pkg/exchange/bybit/adapter.go`
- `pkg/exchange/bybit/client.go`
- `pkg/exchange/bybit/margin.go`
- `pkg/exchange/bybit/ws.go`
- `pkg/exchange/bybit/ws_private.go`

## Use This Folder For

- Unified trading account futures flows
- Market data, order placement, positions, and WebSocket events
- Spot-margin borrowing and repayment for spot-futures

## Document Map

- `bybit-market-api-docs.md`
  - Market data, funding, tickers, orderbook
- `bybit-order-api-docs.md`
  - Order placement, cancel, query, execution behavior
- `bybit-position-api-docs.md`
  - Positions, leverage, margin mode, risk limits
- `bybit-account-api-docs.md`
  - Balances, transfers, account configuration
- `bybit-spot-margin-api-docs.md`
  - Spot-margin order, borrow, repay, balances
  - Primary reference for spot-futures spot leg behavior
- `bybit-crypto-loan-api-docs.md`
  - Supplemental loan details when margin docs are incomplete
- `bybit-websocket-api-docs.md`
  - Public/private stream payloads and authentication

## Repo Notes

- Symbol format in repo: `BTCUSDT`
- Bybit runs in one-way mode in this project
- Spot-futures logic relies on UTA spot-margin behavior; settlement and locked-balance edge cases matter here more than in perp-perp flows
