# BingX API Annotations

## Gotchas & Implementation Notes

### clientOrderId casing (WARN)
- **API expects**: `clientOrderId` (lowercase `d`) for both request param and response field
- **Code uses**: `clientOrderID` (uppercase `D`) in both `PlaceOrder` params and `GetPendingOrders` JSON tag
- Go JSON decoder with explicit tags is case-sensitive — `json:"clientOrderID"` will NOT match `clientOrderId` from API
- Files: `adapter.go:174` (request), `adapter.go:225` (response parsing)

### Signature computed on URL-encoded values (WARN)
- API docs say: signature must be computed from **unencoded** signing string
- `buildParamString()` applies `url.QueryEscape()` before signing
- Works for alphanumeric params but will break for values containing `[`, `{`, spaces, `+`
- File: `client.go:85`

### SetLeverage side=BOTH (WARN)
- API docs say side param accepts `LONG` or `SHORT` only
- Code sends `BOTH` — undocumented but appears to work in production
- File: `adapter.go:389`

### Transfer endpoint uses undocumented params
- Code uses `transferAccountType` / `targetAccountType` on `/openApi/wallets/v1/capital/innerTransfer/apply`
- Doc lists different params: `userAccountType`, `userAccount`, `walletType`
- Standard self-transfer endpoint is `/openApi/api/v3/asset/transfer` with `type` enum (e.g. `FUND_SFUTURES`)
- File: `adapter.go:626-651`

### Listen key response format
- Listen key endpoints do NOT use the standard `{code, data}` wrapper
- Response is bare `{"listenKey":"..."}` — must use `DoRequestRaw` not `Get`/`Post`
- File: `ws_private.go:86-105`

### GetFundingInterval hardcoded
- Returns 8h always, but BingX supports 1h/2h/4h/8h per contract
- `GetFundingRate` already parses `fundingIntervalHours` correctly
- File: `adapter.go:548-552`

### Private WS sends unsolicited Pong
- `pingLoop` sends "Pong" every 10min proactively (ws_private.go:143-163)
- `readLoop` already handles server Ping -> Pong reactively
- Harmless but incorrect — should be "Ping" if intent is client keepalive

---
date: 2026-03-26
type: correction
---
GET /openApi/swap/v2/trade/fillHistory: Response wrapper key is `fill_history_orders` (NOT `fill_orders` as documented in official docs). The `filledTime` field is a datetime string like `"2026-03-26T16:53:55.000+08:00"` (NOT a ms timestamp). Field names in response: `qty` (not `volume`), `commission` (negative = cost), `commissionAsset`, `tradeId`, `orderId`, `price`, `side`, `symbol`. Parse `filledTime` with Go format `"2006-01-02T15:04:05.000-07:00"`.

