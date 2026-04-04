# Gate.io Docs Index

This folder holds the raw Gate.io vendor documentation used by the adapter in:

- `pkg/exchange/gateio/adapter.go`
- `pkg/exchange/gateio/client.go`
- `pkg/exchange/gateio/margin.go`
- `pkg/exchange/gateio/ws.go`
- `pkg/exchange/gateio/ws_private.go`

## Use This Folder For

- USDT perpetual futures trading
- Spot trading and unified-account margin behavior
- Loan and collateral behavior where unified docs are split

## Document Map

- `gate-perpetual-futures-api-docs.md`
  - Primary futures reference
  - Orders, positions, funding, risk, WebSocket conventions
- `gate-spot-api-docs.md`
  - Spot trading and market data
- `gate-isolated-margin-api-docs.md`
  - Margin trading reference used as supplemental behavior guide
- `gate-multi-collateral-loan-api-docs.md`
  - Loan and collateral details relevant to unified account flows

## Repo Notes

- Repo symbol format: `BTCUSDT`
- Gate API symbol format often differs by product; futures commonly use `BTC_USDT`
- The repo also has top-level Gate unified references in:
  - `doc/EXCHANGEAPI_GATEIO_MARGIN.md`
  - `doc/EXCHANGEAPI_GATEIO_UNIFIED.md`
- Confirm account-mode assumptions before changing margin endpoints; Gate behavior differs sharply between classic and unified flows
