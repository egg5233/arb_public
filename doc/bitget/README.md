# Bitget Docs Index

This folder holds the raw Bitget vendor documentation used by the adapter in:

- `pkg/exchange/bitget/adapter.go`
- `pkg/exchange/bitget/client.go`
- `pkg/exchange/bitget/margin.go`
- `pkg/exchange/bitget/ws.go`
- `pkg/exchange/bitget/ws_private.go`

## Use This Folder For

- Futures trading and market data
- Cross-margin spot-futures operations
- Loan and borrow behavior for spot-futures

## Document Map

- `bitget-futures-api-docs.md`
  - Primary futures reference
  - Orders, positions, funding, leverage, balances
- `bitget-spot-api-docs.md`
  - Spot trading and market data behavior
- `bitget-margin-api-docs.md`
  - Cross-margin balances, orders, borrow, repay, transfer
  - Primary reference for the spot-futures spot leg
- `bitget-earn-loan-api-docs.md`
  - Supplemental loan and borrow references

## Repo Notes

- Symbol format in repo: `BTCUSDT`
- Bitget spot-futures support includes separate-account transfers between futures and spot/margin
- Verify quote-size vs base-size semantics carefully on market buys; Bitget differs between endpoints
