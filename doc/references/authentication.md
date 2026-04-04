# Exchange Authentication Notes

Shared reference for the curated BingX docs in this repository.

## BingX REST Signing

- Signature algorithm: HMAC-SHA256
- Common signed request fields:
  - `timestamp`
  - `recvWindow`
  - `signature`
- Requests also require the API key header expected by the product family being used

## Repo Notes

- Treat this file as a quick navigation aid, not the final source of truth
- Before changing signing logic, verify the current implementation in:
  - `pkg/exchange/bingx/client.go`
- For WebSocket authentication, follow the specific private-stream docs and compare against:
  - `pkg/exchange/bingx/ws_private.go`
