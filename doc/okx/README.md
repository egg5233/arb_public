# OKX Docs Index

This folder holds the raw OKX vendor documentation used by the adapter in:

- `pkg/exchange/okx/adapter.go`
- `pkg/exchange/okx/client.go`
- `pkg/exchange/okx/margin.go`
- `pkg/exchange/okx/ws.go`
- `pkg/exchange/okx/ws_private.go`

## Use This Folder For

- Trading account and funding account behavior
- Perpetual futures trading and market data
- Spot and margin market conventions for the spot-futures engine

## Document Map

- `okx-order-book-trading-api-docs.md`
  - Primary trading reference
  - Orders, fills, algo orders, close behavior
- `okx-trading-account-api-docs.md`
  - Balances, positions, leverage, account configuration
- `okx-funding-account-api-docs.md`
  - Transfers and funding-account asset movement
- `okx-public-data-api-docs.md`
  - Instruments, funding rate, mark price, public market metadata
- `okx-financial-product-api-docs.md`
  - Secondary reference for product/account behavior when needed

## Repo Notes

- Repo symbol format: `BTCUSDT`
- OKX spot instrument format: `BTC-USDT`
- OKX swap instrument format: `BTC-USDT-SWAP`
- The client unwraps the standard `{code, msg, data}` envelope before most adapter logic sees the response
- Missing spot instruments can surface as `51001`; treat that as "spot market does not exist" rather than a transient API failure
