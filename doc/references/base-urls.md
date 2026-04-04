# Exchange Base URLs

Shared reference for the curated BingX docs in this repository.

## BingX

- REST: `https://open-api.bingx.com`
- Public WebSocket: check the latest BingX market-data docs for the current swap and spot stream hosts
- Private WebSocket: check the latest BingX authenticated stream docs for the current host and auth flow

## Repo Notes

- Repo symbol format usually omits separators: `BTCUSDT`
- BingX vendor docs often show symbols with a hyphen: `BTC-USDT`
- For this codebase, authoritative adapter behavior lives in:
  - `pkg/exchange/bingx/client.go`
  - `pkg/exchange/bingx/adapter.go`
  - `pkg/exchange/bingx/ws.go`
  - `pkg/exchange/bingx/ws_private.go`
