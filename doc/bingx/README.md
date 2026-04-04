# BingX Docs Index

This folder holds the raw BingX vendor documentation used by the adapter in:

- `pkg/exchange/bingx/adapter.go`
- `pkg/exchange/bingx/client.go`
- `pkg/exchange/bingx/ws.go`
- `pkg/exchange/bingx/ws_private.go`

## Use This Folder For

- Perpetual swap trading and market data
- Spot market data where needed for symbol conventions
- Account and transfer behavior

## Document Map

- `bingx-swap-api-docs.md`
  - Primary futures/swap REST reference
  - Orders, positions, funding, balances, leverage
- `bingx-spot-api-docs.md`
  - Spot endpoints and symbol conventions
- `bingx-other-api-docs.md`
  - Account, wallet, and miscellaneous API behavior
- `bingx-cswap-api-docs.md`
  - Supplemental contract trading reference where the main swap docs are incomplete

## Repo Notes

- Symbol format in repo: `BTCUSDT`
- BingX uses one-way mode in this project
- BingX is not used as a spot-margin exchange in the spot-futures engine
- Private and public WebSocket behavior is implemented separately in the repo; confirm both docs before changing order-event parsing
