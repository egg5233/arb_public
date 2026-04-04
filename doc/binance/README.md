# Binance Docs Index

This folder holds the raw Binance vendor documentation used by the adapter in:

- `pkg/exchange/binance/adapter.go`
- `pkg/exchange/binance/client.go`
- `pkg/exchange/binance/margin.go`
- `pkg/exchange/binance/ws.go`
- `pkg/exchange/binance/ws_private.go`

## Use This Folder For

- USDS-M futures trading and market data
- Spot and wallet endpoints used by transfers and spot-futures flows
- Cross-margin and loan behavior for the spot-futures engine
- CMS delist announcements

## Document Map

- `binance-usds-futures-api-docs.md`
  - Primary futures REST and trading reference
  - Order placement, funding, leverage, position mode, exchange info
- `binance-spot-api-docs.md`
  - Public spot market data and standard spot endpoints
  - Relevant for spot ticker and book-ticker behavior
- `binance-margin-trading-api-docs.md`
  - Cross-margin borrowing, balances, and margin order behavior
  - Primary reference for spot-futures margin leg operations
- `binance-wallet-api-docs.md`
  - Internal transfers, withdraw, and asset movement
- `binance-cms-api-docs.md`
  - Delist/news announcements used by the delist filter
- `binance-crypto-loan-api-docs.md`
  - Secondary reference for loan-specific behavior when needed
- `binance-portfolio-margin-api-docs.md`
  - Historical reference only; this repo currently uses classic `fapi` paths, not PM routing

## Repo Notes

- Futures base URL: `https://fapi.binance.com`
- Spot/wallet base URL: `https://api.binance.com`
- Symbol format in repo: `BTCUSDT`
- `GetSpotBBO()` uses the unsigned public spot `bookTicker` endpoint; do not sign public spot market-data requests
