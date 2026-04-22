# PLAN: Bitget Error Handling + Full Non-ASCII Symbol Support

Version: v17
Date: 2026-04-22
Status: REVIEWING

## Direction change from v1

v1 kept v0.32.42's ASCII symbol guards (blocked non-ASCII at source). User rejected this: **"不要擋中文代幣 這是不正確的方式"**. Direction corrected: support non-ASCII symbols (e.g. `龙虾USDT`) end-to-end. Exchanges list these symbols on their side — blocking in our bot loses real arbitrage opportunities.

## Problem A: Bitget client swallows all errors

`pkg/exchange/bitget/client.go:doRequest` returns `(body, nil)` regardless of HTTP status code OR response `code` field. When bitget server returns `{"code":"40009","msg":"sign signature error","data":null}` (HTTP 400), client treats it as success. Downstream adapter methods that lack per-method `resp.Code` check unmarshal the error body into a struct where `Data.Bills`/`Data.XXX` is nil, returning `(empty slice, nil error)` — indistinguishable from "confirmed no data".

Affects any bitget API call during any transient failure. Other 5 exchanges all check either HTTP status OR API code:
- Binance: `resp.StatusCode >= 400` at `pkg/exchange/binance/client.go:245`
- Bybit: `apiResp.RetCode != 0` at `pkg/exchange/bybit/client.go:170`
- OKX: `okxResp.Code != "0"` at `pkg/exchange/okx/client.go:188`
- Gate.io: `resp.StatusCode >= 400` at `pkg/exchange/gateio/client.go:186`
- BingX: `apiResp.Code != 0` at `pkg/exchange/bingx/client.go:233`

### Concrete impact observed on live position 龙虾usdt-1776800704405

- `funding_collected = -0.06900134` matches ONLY binance long leg's 2 funding payments
- Bitget short leg actually credited 2 funding events totaling **+0.914 USDT** (verified via bitget API without symbol filter)
- `queryEntryFees` log at entry also showed `short=bitget:0.0000` — same root cause via `GetUserTrades`

### Call-site exposure (Codex-verified)

| Method | Adapter | Callers |
|--------|---------|---------|
| `GetFundingFees` | `adapter.go:1171-1209` | `engine.go:1783, 1802`, `handlers.go:224` |
| `GetUserTrades` | `adapter.go:1118-1168` | `engine.go:4357, 4367` |
| `GetOrderFilledQty` | `adapter.go:243-264` | `engine.go:2630, 2682, 4278, 4309, 4587`, `spotengine/execution.go:679` |

**`GetOrderFilledQty` is highest severity** — error returning `(0, nil)` can cause duplicate orders (2× exposure).

Not affected (Codex-verified):
- `GetPosition` — `parsePositions` at `adapter.go:319` already checks `resp.Code != "00000"`
- `GetFuturesBalance` / `GetSpotBalance` — have code check at `adapter.go:690` / `740`

## Problem B: Non-ASCII symbol in GET query fails bitget HMAC

Bitget server rejects GET requests whose query string contains URL-encoded non-ASCII (e.g. `symbol=%E9%BE%99%E8%99%BEUSDT`) with `40009 "sign signature error"`. POST works (PlaceOrder with body-encoded `symbol=龙虾USDT` succeeded). GET without symbol filter also works (verified).

After Problem A fix, calls with non-ASCII symbols surface 40009 error — correct but no data retrieved. Need fallback: use no-symbol-filter endpoint + local filter.

## Problem C: Revert v0.32.42 ASCII guards

v0.32.42 (just deployed to GitHub, not yet to VPS) blocks non-ASCII symbols across discovery, API handlers, and cache load. Per user direction, remove these blocks. Legitimate symbols that exchanges list should flow through.

## Problem D: Verify other 5 exchanges handle non-ASCII correctly

Only bitget has been empirically proven broken for non-ASCII. Other exchanges have unknown behavior:

- **Binance** — empirically works (龙虾USDT funding returned via GetFundingFees)
- **Bybit** — untested; passes symbol raw; unknown if bybit HMAC has same issue
- **OKX** — untested; transforms via `toOKXInstID` → `龙虾-USDT-SWAP`; unknown
- **Gate.io** — untested; transforms via `toGateSymbol` → `龙虾_USDT`; unknown
- **BingX** — safe by design (no symbol in GET query; fetches all and filters locally)

Need empirical tests before trusting. If any peer has same signing issue, apply same fallback pattern.

## Not Changed

- Other 5 exchanges' client error checking (already correct)
- Discovery ranking logic (works with non-ASCII — verified in logs: `[9] 龙虾USDT (loris) | ...`)
- Risk approve / entry execution (POST bodies work — verified: `PlaceOrder`, `PlaceStopLoss`, `PlaceTakeProfit` all succeeded for 龙虾USDT)
- WS subscriptions (verified from logs: `binance-ws-priv` and `bitget-ws-priv` both emit `symbol=龙虾USDT` order updates correctly)

## Changes

### 1. `pkg/exchange/bitget/client.go:doRequest` — add HTTP status + API code check

Mirror pattern from `binance/client.go:245-254` and `gateio/client.go:186-196`.

BEFORE (`pkg/exchange/bitget/client.go:165-177`):
```go
resp, err := c.httpClient.Do(req)
if err != nil {
	return "", err
}
defer resp.Body.Close()

data, err := io.ReadAll(resp.Body)
if err != nil {
	return "", err
}

return string(data), nil
```

AFTER:
```go
resp, err := c.httpClient.Do(req)
if err != nil {
	return "", err
}
defer resp.Body.Close()

data, err := io.ReadAll(resp.Body)
if err != nil {
	return "", err
}

// Parse envelope once for downstream branching.
var envelope struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}
_ = json.Unmarshal(data, &envelope)

// Idempotent codes that existing adapter methods treat as success:
// - 40872 "margin mode already set" (SetMarginMode adapter.go:175)
// - 43011 "order already cancelled/finalized" (CancelOrder adapter.go:400,
//   CancelStopLoss adapter.go:1376)
// - 43025 same semantic as 43011 in some contexts
// Pass these through untouched so existing idempotent handling keeps working.
if bitgetPassThroughCodes[envelope.Code] {
	return string(data), nil
}

// Non-2xx HTTP is always error. Synthesize APIError from status if body has
// no usable code (e.g. HTML response, empty body).
// IMPORTANT: assign to the local `err` variable captured by the metrics defer
// at client.go:97-103 so bitget API errors are recorded for observability.
if resp.StatusCode >= 400 {
	if envelope.Code != "" {
		err = &APIError{Code: envelope.Code, Msg: envelope.Msg}
		return "", err
	}
	err = &APIError{Code: strconv.Itoa(resp.StatusCode), Msg: fmt.Sprintf("bitget HTTP %d: %s", resp.StatusCode, string(data))}
	return "", err
}

// HTTP 2xx but logical failure (code != "00000").
if envelope.Code != "" && envelope.Code != "00000" {
	err = &APIError{Code: envelope.Code, Msg: envelope.Msg}
	return "", err
}

return string(data), nil
```

**Metrics note** — `doRequest` has **unnamed** return values `(string, error)` (verified at client.go:96), but declares a local `var err error` at line 98 that the defer at lines 99-103 captures. All new HTTP/API error paths MUST assign to this local `err` before `return "", err`, so metrics record bitget errors. The sample AFTER code above already does this (`err = &APIError{...}` before `return "", err`).

Add package-level declaration:
```go
// bitgetPassThroughCodes lists bitget error codes whose semantics are treated
// as success by existing adapter call sites (idempotent operations). Each must
// have a documented call site that relies on the code being preserved.
var bitgetPassThroughCodes = map[string]bool{
	"40872": true, // SetMarginMode: margin mode already set (adapter.go:175)
	"43011": true, // CancelOrder / CancelStopLoss: order already finalized (adapter.go:400, 1376)
	"43025": true, // Same semantic family
}
```

**Code-based branching NOT covered by pass-through** (must migrate to `errors.As(*APIError)` pattern after fix):
- `CheckPermissions` uses code `40009` to detect "permission denied" (must rewrite — see #16)
- `EnsureOneWayMode` uses code `40774` (must rewrite if current pattern relies on string match of raw body; OK if it only needs `err.Error()` to contain the code)

Grep for all uses of `body.Code == "..."` / `resp.Code == "..."` patterns in adapter.go during implementation and migrate each to `errors.As(err, &apiErr); apiErr.Code == "..."`.

Requires `APIError` struct (verified: does NOT exist in bitget package — must add):
```go
type APIError struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("bitget API error %s: %s", e.Code, e.Msg)
}
```

**Import additions required:**

BEFORE (client.go:3-18, actual HEAD):
```go
import (
	"arb/pkg/exchange"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)
```

AFTER:
```go
import (
	"arb/pkg/exchange"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)
```

Two additions: `errors` (for `errors.As` in updated `isRetryable`), `strconv` (for `strconv.Itoa(resp.StatusCode)`).

**Retry interaction:** `retryDo` uses `isRetryable(err, rawResp)`. After fix, rawResp is `""` when error returned; isRetryable must also inspect `*APIError.Code`:
```go
func isRetryable(err error, rawResp string) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) && retryableCodes[apiErr.Code] {
		return true
	}
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "connection refused") ||
			strings.Contains(errMsg, "EOF") ||
			strings.Contains(errMsg, "connection reset") {
			return true
		}
	}
	// Existing rawResp inspection retained for backward compat if rawResp
	// ever returned (e.g. passthrough codes above).
	if rawResp != "" {
		var base struct {
			Code string `json:"code"`
		}
		if json.Unmarshal([]byte(rawResp), &base) == nil && retryableCodes[base.Code] {
			return true
		}
	}
	return false
}
```

**5xx HTTP retry:** After fix, 5xx errors surface as `APIError{Code: strconv.Itoa(status)}`. `retryableCodes` map currently only lists bitget error codes (not HTTP codes). Add generic 5xx handling so server maintenance/overload retries work:
```go
// Inside isRetryable, after errors.As apiErr check:
if apiErr != nil {
	if n, convErr := strconv.Atoi(apiErr.Code); convErr == nil && n >= 500 && n <= 599 {
		return true
	}
}
```

### 2-4. bitget adapter.go — per-method code check (defense-in-depth)

Even with #1, adapter methods should validate their unmarshaled envelope. Pattern (must include `Msg` field in struct):
```go
var resp struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data struct { ... } `json:"data"`
}
if err := json.Unmarshal([]byte(raw), &resp); err != nil {
	return <zero>, err
}
if resp.Code != "" && resp.Code != "00000" {
	return <zero>, &APIError{Code: resp.Code, Msg: resp.Msg}
}
```

Apply to:
- **#2** `GetOrderFilledQty` (adapter.go:231-264) — return `(0, err)`. Also needs #9 fallback because this endpoint uses `symbol` in query.
- **#3** `GetUserTrades` (adapter.go:1103-1168) — return `(nil, err)`. Also needs #7 fallback.
- **#4** `GetFundingFees` (adapter.go:1172-1209) — return `(nil, err)`. Also needs #5 fallback.

### 5. `pkg/exchange/bitget/adapter.go:GetFundingFees` — non-ASCII fallback

When symbol contains non-ASCII char, call `/account/bill` without symbol filter and filter locally.

**Key corrections from v3 review:**
- `limit` max is **100** per bitget docs (`doc/bitget/bitget-futures-api-docs.md:3333`), not 500.
- Endpoint requires `endTime` to pair with `startTime`; 30-day span cap.
- `containsNonASCII` lives in `pkg/exchange/bitget` (not `pkg/utils`). After #10 deletes `pkg/utils/symbol.go`, utils would have no non-ASCII helpers left.

```go
// Add to bitget package (e.g. bitget/util.go or top of adapter.go):
func containsNonASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}
```

```go
func (a *Adapter) GetFundingFees(symbol string, since time.Time) ([]exchange.FundingPayment, error) {
	if containsNonASCII(symbol) {
		return a.getFundingFeesViaNoFilter(symbol, since)
	}
	// existing symbol-filtered path with added code check (from #4) ...
}

func (a *Adapter) getFundingFeesViaNoFilter(symbol string, since time.Time) ([]exchange.FundingPayment, error) {
	params := map[string]string{
		"productType":  "USDT-FUTURES",
		"businessType": "contract_settle_fee",
		"startTime":    strconv.FormatInt(since.UnixMilli(), 10),
		"endTime":      strconv.FormatInt(time.Now().UnixMilli(), 10),
		"limit":        "100", // bitget max per docs; local filter trims irrelevant rows
	}
	body, err := a.client.Get("/api/v2/mix/account/bill", params)
	if err != nil {
		return nil, fmt.Errorf("GetFundingFees (no-filter): %w", err)
	}
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Bills []struct {
				Amount string `json:"amount"`
				CTime  string `json:"cTime"`
				Symbol string `json:"symbol"`
			} `json:"bills"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("GetFundingFees (no-filter) unmarshal: %w", err)
	}
	if resp.Code != "" && resp.Code != "00000" {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	out := make([]exchange.FundingPayment, 0)
	for _, b := range resp.Data.Bills {
		if !strings.EqualFold(b.Symbol, symbol) {
			continue
		}
		amt, _ := strconv.ParseFloat(b.Amount, 64)
		ms, _ := strconv.ParseInt(b.CTime, 10, 64)
		out = append(out, exchange.FundingPayment{
			Amount: amt,
			Time:   time.UnixMilli(ms),
		})
	}
	return out, nil
}
```

**Pagination required** (per independent review — 100-row limit causes data loss for busy accounts even in 24h window):

```go
func (a *Adapter) getFundingFeesViaNoFilter(symbol string, since time.Time) ([]exchange.FundingPayment, error) {
	out := make([]exchange.FundingPayment, 0)
	var idLessThan string
	endTime := time.Now().UnixMilli()
	startTime := since.UnixMilli()
	for {
		params := map[string]string{
			"productType":  "USDT-FUTURES",
			"businessType": "contract_settle_fee",
			"startTime":    strconv.FormatInt(startTime, 10),
			"endTime":      strconv.FormatInt(endTime, 10),
			"limit":        "100",
		}
		if idLessThan != "" {
			params["idLessThan"] = idLessThan
		}
		body, err := a.client.Get("/api/v2/mix/account/bill", params)
		if err != nil {
			return nil, fmt.Errorf("GetFundingFees (no-filter page): %w", err)
		}
		var resp struct {
			Code string `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Bills []struct {
					BillID string `json:"billId"`
					Amount string `json:"amount"`
					CTime  string `json:"cTime"`
					Symbol string `json:"symbol"`
				} `json:"bills"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(body), &resp); err != nil {
			return nil, fmt.Errorf("GetFundingFees (no-filter) unmarshal: %w", err)
		}
		if resp.Code != "" && resp.Code != "00000" {
			return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
		}
		if len(resp.Data.Bills) == 0 {
			break
		}
		var oldestCTime int64
		for _, b := range resp.Data.Bills {
			if !strings.EqualFold(b.Symbol, symbol) {
				if ct, _ := strconv.ParseInt(b.CTime, 10, 64); ct < oldestCTime || oldestCTime == 0 {
					oldestCTime = ct
				}
				continue
			}
			amt, _ := strconv.ParseFloat(b.Amount, 64)
			ms, _ := strconv.ParseInt(b.CTime, 10, 64)
			out = append(out, exchange.FundingPayment{Amount: amt, Time: time.UnixMilli(ms)})
			if ms < oldestCTime || oldestCTime == 0 {
				oldestCTime = ms
			}
		}
		// Stop paginating once oldest returned bill is older than our since window.
		if oldestCTime != 0 && oldestCTime < startTime {
			break
		}
		if len(resp.Data.Bills) < 100 {
			break
		}
		// Per bitget docs (bitget-futures-api-docs.md:3330, 3367, 3377), use
		// data.endId as next-page cursor. Fall back to last row's billId if
		// endId is empty. Detect cursor deadlock (same cursor as previous).
		nextCursor := strings.TrimSpace(resp.Data.EndID)
		if nextCursor == "" {
			nextCursor = strings.TrimSpace(resp.Data.Bills[len(resp.Data.Bills)-1].BillID)
		}
		if nextCursor == "" || nextCursor == idLessThan {
			return nil, fmt.Errorf("GetFundingFees no-filter pagination stalled: cursor=%q rows=%d", idLessThan, len(resp.Data.Bills))
		}
		idLessThan = nextCursor
	}
	return out, nil
}
```

**Note on cursor field:** Response struct must include `EndID string \`json:"endId"\`` alongside `Bills` array:
```go
Data struct {
	EndID string `json:"endId"`
	Bills []struct {
		BillID string `json:"billId"`
		Amount string `json:"amount"`
		CTime  string `json:"cTime"`
		Symbol string `json:"symbol"`
	} `json:"bills"`
} `json:"data"`
```

**30-day window cap**: if `since` > 30 days ago, bitget docs cap query span. In practice our usage queries from `pos.CreatedAt` which is always recent. If needed, iterate in 30-day chunks — deferred to implementation.

Same pagination pattern applies to GetUserTrades (#7), GetClosePnL (#8), and GetOrderFilledQty fallback (#9) when they use no-symbol endpoints. Apply consistently.

### 6. `pkg/exchange/bitget/adapter.go:GetPosition` — non-ASCII fallback

Currently uses `/api/v2/mix/position/single-position` with symbol — fails 40009 on non-ASCII. Fallback to `GetAllPositions` + local filter:

```go
func (a *Adapter) GetPosition(symbol string) ([]exchange.Position, error) {
	if containsNonASCII(symbol) {
		all, err := a.GetAllPositions()
		if err != nil {
			return nil, err
		}
		filtered := make([]exchange.Position, 0, len(all))
		for _, p := range all {
			if strings.EqualFold(p.Symbol, symbol) {
				filtered = append(filtered, p)
			}
		}
		return filtered, nil
	}
	// existing path ...
}
```

### 7. `pkg/exchange/bitget/adapter.go:GetUserTrades` — non-ASCII fallback

**Concrete implementation** (per Codex v3 review: bitget docs confirm `symbol` is optional on `/api/v2/mix/order/fills` at `doc/bitget/bitget-futures-api-docs.md:1798`):

```go
func (a *Adapter) GetUserTrades(symbol string, since time.Time, limit int) ([]exchange.Trade, error) {
	params := map[string]string{
		"productType": "USDT-FUTURES",
		"startTime":   strconv.FormatInt(since.UnixMilli(), 10),
		"endTime":     strconv.FormatInt(time.Now().UnixMilli(), 10),
		"limit":       strconv.Itoa(limit),
	}
	if !containsNonASCII(symbol) {
		params["symbol"] = symbol
	}
	raw, err := a.client.Get("/api/v2/mix/order/fills", params)
	if err != nil {
		return nil, fmt.Errorf("GetUserTrades: %w", err)
	}
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			FillList []struct {
				// existing fields ...
				Symbol string `json:"symbol"` // MUST add for local filter
				// ... TradeID, OrderID, Side, Price, BaseVolume, CTime, FeeDetail ...
			} `json:"fillList"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("GetUserTrades unmarshal: %w", err)
	}
	if resp.Code != "" && resp.Code != "00000" {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	trades := make([]exchange.Trade, 0, len(resp.Data.FillList))
	for _, t := range resp.Data.FillList {
		// When non-ASCII path taken, filter locally.
		if containsNonASCII(symbol) && !strings.EqualFold(t.Symbol, symbol) {
			continue
		}
		// existing trade mapping ...
	}
	return trades, nil
}
```

### 8. `pkg/exchange/bitget/adapter.go:GetClosePnL` — non-ASCII fallback

Per Codex v3 review: bitget `/api/v2/mix/position/history-position` supports no-symbol query (`symbol` optional per `doc/bitget/bitget-futures-api-docs.md:3836`).

Current implementation needs the response struct extended with `Symbol string` for local filtering. Apply same dual-path pattern as #7:
```go
if !containsNonASCII(symbol) {
	params["symbol"] = symbol
}
// ... unmarshal with Symbol in HistoryList items ...
if containsNonASCII(symbol) && !strings.EqualFold(item.Symbol, symbol) {
	continue
}
```

### 9. `pkg/exchange/bitget/adapter.go:GetOrderFilledQty` — non-ASCII fallback

Per Codex v3 review: endpoint `/order/detail` **requires** symbol (docs `bitget-futures-api-docs.md:1682`). For non-ASCII, fallback to `/api/v2/mix/order/fills` (same as #7) with `orderId` param (no symbol), sum `baseVolume` across matching fills:

```go
func (a *Adapter) GetOrderFilledQty(orderID, symbol string) (float64, error) {
	if containsNonASCII(symbol) {
		return a.getOrderFilledQtyViaFills(orderID)
	}
	// existing /order/detail path with added code check ...
}

func (a *Adapter) getOrderFilledQtyViaFills(orderID string) (float64, error) {
	params := map[string]string{
		"productType": "USDT-FUTURES",
		"orderId":     orderID,
	}
	raw, err := a.client.Get("/api/v2/mix/order/fills", params)
	if err != nil {
		return 0, fmt.Errorf("GetOrderFilledQty (fills): %w", err)
	}
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			FillList []struct {
				OrderID    string `json:"orderId"`
				BaseVolume string `json:"baseVolume"`
			} `json:"fillList"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return 0, err
	}
	if resp.Code != "" && resp.Code != "00000" {
		return 0, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	var total float64
	for _, f := range resp.Data.FillList {
		if f.OrderID != orderID {
			continue
		}
		v, _ := strconv.ParseFloat(f.BaseVolume, 64)
		total += v
	}
	return total, nil
}
```

### 10. REVERT v0.32.42 ASCII guards — remove 11 changes

Revert changes from `plans/PLAN-ascii-symbol-guard.md` (v0.32.42 commit `3700945d`):

- `internal/discovery/ranker.go` — remove `utils.IsValidBaseSymbol` guard + import
- `internal/discovery/scanner.go` — remove CoinGlass perp guard
- `internal/spotengine/discovery.go` — remove Loris + CoinGlass guards + import
- `internal/spotengine/engine.go:InjectTestOpportunity` — remove guard
- `internal/spotengine/engine.go:loadCachedOpps` — remove filter
- `internal/api/spot_handlers.go:handleSpotTestInject` — remove guard
- `internal/api/spot_handlers.go:handleSpotTestLifecycle` — remove guard
- `internal/api/spot_handlers.go:handleSpotManualOpen` — remove guard
- `internal/api/spot_handlers.go:handleSpotBacktest` — remove guard
- Delete `pkg/utils/symbol.go` and `pkg/utils/symbol_test.go` (unused after revert)

**Do NOT revert** `VERSION` / `CHANGELOG.md` from v0.32.42 — that's committed history. Instead this plan's implementation will bump VERSION to the next number (e.g. v0.32.43) and add a new CHANGELOG entry documenting the bitget fixes + guard removal.

Keep `internal/engine/engine.go:rebalanceScan` / `rotateScan` dedup (v0.32.42 Problem B) — that's unrelated to Chinese symbol support, should stay.

### 11. Empirical tests for other 5 exchanges — verify non-ASCII handling

Before trusting other exchanges, run empirical test script on VPS. Dynamic symbol discovery (not hardcoded):

```go
// cmd/peertest/main.go (new CLI under cmd/)
// For each of binance, bybit, okx, gateio, bingx:
// 1. Call adapter.LoadAllContracts() to get listed symbols
// 2. Filter for non-ASCII symbols (using containsNonASCII helper from bitget package,
//    or re-use — acceptable minor duplication since cmd/ depends on pkg/)
// 3. For each non-ASCII symbol found, call:
//    - GetFundingRate(sym) (read-only, safe)
//    - GetFundingFees(sym, 24h_ago)  (read-only auth required)
//    - GetPosition(sym) — expect empty if no position, MUST NOT return silent (empty, nil) on signing error
// 4. Assert: either (data, nil) or (_, err != nil). Never (empty, nil) from signing failure.
```

Test symbol sources:
- Binance: already known OK (龙虾USDT returned valid funding)
- Bybit, OKX, Gate.io, BingX: discover via each adapter's `LoadAllContracts` method
- If none list non-ASCII symbols → acknowledge in test output ("no test case available for this exchange"); acceptable, no code change needed
- If any lists non-ASCII and silently fails → follow up with same fallback pattern as bitget (separate sub-task)

Place test CLI under `cmd/peertest/` to avoid touching existing `cmd/livetest/` (which has order-placement paths). This keeps peer tests read-only.

### 12. `internal/risk/health.go:getLegPnL` — log silent swallow

BEFORE (line 358-360 verified at HEAD):
```go
positions, err := exch.GetPosition(symbol)
if err != nil {
	return 0
}
```

AFTER (ASCII-only log string to avoid any downstream logging issues):
```go
positions, err := exch.GetPosition(symbol)
if err != nil {
	h.log.Warn("getLegPnL: GetPosition(%s) on %s failed: %v - counting as 0", symbol, exchName, err)
	return 0
}
```

Minimal change: add log. Don't change return-0 behavior (margin health is a hot path; changing semantics is risky; separate task if needed).

### 13. `internal/api/handlers.go:handleGetPositionFunding` — log + partial flag

Backend change:

BEFORE (line 224-227):
```go
fees, err := exch.GetFundingFees(pos.Symbol, leg.start)
if err != nil {
	continue
}
```

AFTER:
```go
fees, err := exch.GetFundingFees(pos.Symbol, leg.start)
if err != nil {
	s.log.Warn("handleGetPositionFunding: GetFundingFees(%s) on %s failed: %v", pos.Symbol, leg.name, err)
	partialLegs = append(partialLegs, leg.name)
	continue
}
```

Response envelope changes shape from `[]fundingEvent` to `{events, partial_legs}`:
```go
writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]interface{}{
	"events":       events,
	"partial_legs": partialLegs,  // [] when complete
}})
```

**Frontend updates required (shape change is breaking):**
- `web/src/types.ts` — add `FundingHistoryResponse { events: FundingEvent[]; partial_legs: string[] }` type
- `web/src/hooks/useApi.ts:161` — update `useFundingHistory` hook return type to unwrap `response.data.events` (currently expects `FundingEvent[]` directly)
- `web/src/pages/Positions.tsx` — use `data.events` for rendering, show warning banner if `data.partial_legs.length > 0`
- `web/src/i18n/en.ts` + `zh-TW.ts` — add translation keys: `positions.fundingPartial = "Funding history incomplete — exchange data unavailable for: {legs}"`

**Chosen approach: `X-Partial-Legs` response header** (less breaking than response shape change).

Backend — must preserve existing empty-array response pattern (HEAD returns empty array literal when `events == nil`):

BEFORE (current HEAD pattern):
```go
if events == nil {
	writeJSON(w, http.StatusOK, Response{OK: true, Data: []interface{}{}})
	return
}
writeJSON(w, http.StatusOK, Response{OK: true, Data: events})
```

AFTER:
```go
if events == nil {
	events = []fundingEvent{}
}
if len(partialLegs) > 0 {
	w.Header().Set("X-Partial-Legs", strings.Join(partialLegs, ","))
}
writeJSON(w, http.StatusOK, Response{OK: true, Data: events})
```

Collapsing to a single `writeJSON` after normalization lets the header apply uniformly whether events are present or empty. Return shape stays `[]fundingEvent{}` on the wire (no visible behavior change for existing clients).

**Frontend — concrete because `request()` helper unwraps JSON and discards headers at `useApi.ts:48`:**

The current generic `request<T>()` can't expose headers. `getPositionFunding` must bypass it (or `request()` extended). Bypass is simpler — only this hook needs headers. **Must preserve 401 handling** that `request()` currently performs (token clear + reload) — inline it in the bypass:

```ts
// web/src/types.ts — add:
export interface FundingHistoryResult {
	events: FundingEvent[];
	partialLegs: string[];
}

// web/src/hooks/useApi.ts — replace getPositionFunding:
const getPositionFunding = useCallback(async (positionId: string): Promise<FundingHistoryResult> => {
	const token = getToken();
	const headers: Record<string, string> = { 'Content-Type': 'application/json' };
	if (token) headers.Authorization = `Bearer ${token}`;

	const res = await fetch(`/api/positions/${positionId}/funding`, { headers });
	if (res.status === 401) {
		clearToken();
		window.location.reload();
		throw new Error('Unauthorized');
	}
	if (!res.ok) {
		const body = await res.json().catch(() => null) as { error?: string } | null;
		throw new Error(body?.error || `Funding history failed (${res.status})`);
	}

	const partialLegs = (res.headers.get('X-Partial-Legs') || '')
		.split(',')
		.map(s => s.trim())
		.filter(Boolean);

	const body = await res.json() as ApiResponse<FundingEvent[]>;
	if (!body.ok) throw new Error(body.error || 'Funding history failed');

	return { events: body.data ?? [], partialLegs };
}, []);
```

`getToken` and `clearToken` helpers must match the names used elsewhere in `useApi.ts`; grep the file during implementation to use the exact identifiers (`getToken` / `clearToken` per current convention; adjust if they're named differently).

**Caller updates required (not just the hook):**
- `web/src/pages/Positions.tsx` — `PositionsProps.onFetchFunding` signature changes from `Promise<FundingEvent[]>` to `Promise<FundingHistoryResult>`
- `toggleExpand()` in Positions.tsx — use `result.events` for events list, `result.partialLegs` for warning banner conditional
- `web/src/i18n/en.ts` + `zh-TW.ts` — add `positions.fundingPartial = "Funding history incomplete — unavailable for: {legs}"` and the Chinese equivalent

### 14. `consolidate.go:615, 621` — add log on silent continue

Per Codex v3 review: `exit.go:1128, 1135, 3124` ALREADY log failures (verified at line 1130 and 3126 at HEAD). Only `consolidate.go` lacks the log.

BEFORE (consolidate.go:615):
```go
if longExch, ok := e.exchanges[pos.LongExchange]; ok {
	pnls, err := longExch.GetClosePnL(pos.Symbol, pos.CreatedAt)
	if err == nil {
		longPnLs = pnls
	}
}
```

AFTER:
```go
if longExch, ok := e.exchanges[pos.LongExchange]; ok {
	pnls, err := longExch.GetClosePnL(pos.Symbol, pos.CreatedAt)
	if err != nil {
		e.log.Warn("consolidate GetClosePnL long=%s %s failed: %v", pos.LongExchange, pos.Symbol, err)
	} else {
		longPnLs = pnls
	}
}
```

Same pattern for short leg at consolidate.go:621. Behavior unchanged (empty longPnLs triggers the same downstream flow).

### 15. Caller safety for `GetOrderFilledQty` — prevent duplicate orders on error

**Critical safety fix flagged by Codex review.** After #2 + #9, `GetOrderFilledQty` returns `(0, err)` on bitget API errors instead of silent `(0, nil)`. Existing callers assume `(0, nil)` means "definitely zero filled" — some continue to place another order. If the underlying order actually filled, this causes 2× exposure.

**Concrete signature/caller changes per site (not left to implementation):**

#### 15a. `internal/engine/engine.go:retrySecondLeg` (called from 2630 and 2682)

Change signature to surface error:
```go
// BEFORE (inside retrySecondLeg)
filledQty, _ := exch.GetOrderFilledQty(orderID, symbol)
if filledQty == 0 { continue /* retry */ }

// AFTER
filledQty, qErr := exch.GetOrderFilledQty(orderID, symbol)
if qErr != nil {
	return totalFilled, avgPrice, fmt.Errorf("second leg fill state unknown for %s on %s: %w", orderID, exchName, qErr)
}
if filledQty == 0 { continue }
```

Function signature if currently `(float64, float64)` must become `(float64, float64, error)`. All callers updated:

```go
// Caller pattern (engine.go around 2630, 2682)
retryFilled, retryAvg, retryErr := e.retrySecondLeg(...)
if retryErr != nil {
	// Do NOT place another order — state unknown. Mark position partial,
	// let consolidator reconcile on next cycle.
	pos.Status = models.StatusPartial
	pos.FailureReason = "second_leg_fill_unknown"
	pos.UpdatedAt = time.Now().UTC()
	_ = e.db.SavePosition(pos)
	return errPartialEntry // new sentinel or reuse existing
}
```

#### 15b. `internal/engine/engine.go:confirmFill` — add error return + propagate

Verified at HEAD: line 4278 is inside `confirmFill` function (not a loose call site). Change its signature so callers can distinguish confirmed-zero from unknown:

BEFORE (signature at engine.go around line 4250):
```go
func (e *Engine) confirmFill(exch exchange.Exchange, orderID, symbol string) (float64, float64) {
	...
	filled, _ := exch.GetOrderFilledQty(orderID, symbol)
	...
	return filled, avg
}
```

AFTER:
```go
func (e *Engine) confirmFill(exch exchange.Exchange, orderID, symbol string) (float64, float64, error) {
	...
	filled, qErr := exch.GetOrderFilledQty(orderID, symbol)
	if qErr != nil {
		return 0, 0, fmt.Errorf("fill state unknown for %s on %s: %w", orderID, exch.Name(), qErr)
	}
	...
	return filled, avg, nil
}
```

**All direct `confirmFill` callers** (line 4278 is one internal use, plus external callers in exit/rotation paths — grep for `confirmFill(` during implementation to find every site). Each caller's shape:

```go
// BEFORE
filled, avg := e.confirmFill(exch, orderID, symbol)
if filled == 0 { /* treat as not filled */ }

// AFTER
filled, avg, cfErr := e.confirmFill(exch, orderID, symbol)
if cfErr != nil {
	e.log.Warn("confirmFill unresolved %s %s: %v", orderID, symbol, cfErr)
	// Do NOT treat as zero. Caller-specific: skip this cycle, mark
	// position for recovery, or bubble error up to consolidator.
	return /* or continue, site-dependent */
}
if filled == 0 { /* confirmed not filled */ }
```

**Sister function `confirmFillSafe`** (if exists — common pattern: a "safe" wrapper that returns zero-on-error): must be audited and updated to propagate error instead of swallowing. Implementation will identify via grep.

#### 15c. Other reconciliation call sites at `engine.go:4309, 4587`

After #15b, most of these sites call `confirmFill` and inherit the signature change. Any direct `GetOrderFilledQty` calls (bypass `confirmFill`) use the same pattern:
```go
filled, qErr := exch.GetOrderFilledQty(orderID, symbol)
if qErr != nil {
	return fmt.Errorf("order fill state unknown: %w", qErr)
}
```
Exact handling per site:
- 4309: skip reconciliation this cycle, retry next
- 4587: surrounding context already logs error — add explicit error check + return/continue

#### 15c. `internal/spotengine/execution.go:confirmFuturesFill` (line 679)

Change signature to return error:
```go
// BEFORE
func (e *SpotEngine) confirmFuturesFill(exch exchange.Exchange, orderID, symbol string) (float64, float64) {
	...
	actual, _ := exch.GetOrderFilledQty(orderID, symbol)
	if actual == 0 { return 0, 0 }
	...
}

// AFTER
func (e *SpotEngine) confirmFuturesFill(exch exchange.Exchange, orderID, symbol string) (float64, float64, error) {
	...
	actual, qErr := exch.GetOrderFilledQty(orderID, symbol)
	if qErr != nil {
		return 0, 0, fmt.Errorf("futures fill state unknown for %s: %w", orderID, qErr)
	}
	if actual == 0 {
		return 0, 0, nil // confirmed zero
	}
	...
	return filled, avg, nil
}
```

**Caller change** (entry flow that currently rolls back on zero):
```go
filled, avg, cfErr := e.confirmFuturesFill(futExch, futOrderID, symbol)
if cfErr != nil {
	// State unknown — do NOT rollback spot leg; futures may have filled.
	// Mark as pending-recovery using existing pattern.
	return &pendingFuturesEntryError{
		posID:   pos.ID,
		orderID: futOrderID,
		err:     cfErr,
	}
}
if filled == 0 {
	// Confirmed zero — safe to rollback spot.
	// (existing rollback logic)
}
```

**If `pendingFuturesEntryError` type doesn't exist**, mirror existing `pendingSpotEntryError` (execution.go ~line 564) — same struct/handling pattern.

**CRITICAL — `pendingFuturesEntryError` must be handled in ManualOpen's top-level error branch at execution.go:352**. Current code at that line ONLY catches `*pendingSpotEntryError`. If a new `*pendingFuturesEntryError` falls through, ManualOpen calls `abandonPendingEntry` which is WRONG for unknown-futures-fill state.

BEFORE (execution.go:352):
```go
var pendingErr *pendingSpotEntryError
if errors.As(err, &pendingErr) {
	recoverySaveFailed := false
	// existing pending-spot recovery logic ...
}
e.abandonPendingEntry(entryPos, "entry_failed")
```

AFTER:
```go
var pendingSpotErr *pendingSpotEntryError
if errors.As(err, &pendingSpotErr) {
	// existing pending-spot recovery logic ...
	return err
}

var pendingFutErr *pendingFuturesEntryError
if errors.As(err, &pendingFutErr) {
	// Save pending-futures entry so consolidator can reconcile when bitget
	// recovers. Symmetric with pending-spot pattern: persist pos with
	// status=pending, commit capital reservation, broadcast, return.
	// Full mirror of pending-spot branch (~execution.go:352), including
	// nil-guards, recoverySaveFailed tracking, and wrapped error return.
	recoverySaveFailed := false
	targetPos := entryPos
	if pendingFutErr.pendingPos != nil {
		targetPos = pendingFutErr.pendingPos
	}
	if saveErr := e.persistPendingFuturesEntry(targetPos, pendingFutErr.orderID); saveErr != nil {
		e.log.Error("failed to persist pending futures entry: %v", saveErr)
		recoverySaveFailed = true
	}
	// Commit the spot-side capital reservation. Prefer pendingFutErr.capitalAmount
	// if supplied (reflects actual spot fill), fall back to plannedNotional.
	if reservation != nil {
		amount := plannedNotional
		if pendingFutErr.capitalAmount > 0 {
			amount = pendingFutErr.capitalAmount
		}
		if cmErr := e.commitSpotCapital(reservation, targetPos.ID, amount); cmErr != nil {
			e.log.Error("commitSpotCapital after pending-futures persist failed: %v", cmErr)
		}
	}
	// Guard broadcast — don't broadcast if api is nil or save failed (avoid
	// broadcasting stale state that isn't persisted).
	if e.api != nil && !recoverySaveFailed {
		e.api.BroadcastSpotPositionUpdate(targetPos)
	}
	if recoverySaveFailed {
		return fmt.Errorf("pending-futures persistence failed: %w", err)
	}
	return err
}

e.abandonPendingEntry(entryPos, "entry_failed")
```

**`persistPendingFuturesEntry` — must NOT reuse `PendingEntryOrderID` field** (independent review flag). Existing `PendingEntryOrderID` on the position struct means SPOT order ID, and `reconcilePendingEntry` calls `confirmSpotFill` on it. Overloading it with futures order ID would mis-reconcile.

Two safe implementation options:

**Option A: Add new field `PendingFuturesEntryOrderID`** (cleaner separation)
- Add field to `SpotFuturesPosition` struct in `internal/models/spot_position.go`
- `persistPendingFuturesEntry` stores futOrderID in this new field. **Caller must clear `PendingEntryOrderID` BEFORE calling** (spot is confirmed at this point — see entry paths 506/619 pattern above). `persistPendingFuturesEntry` itself does NOT touch `PendingEntryOrderID`; enforcement is at caller level to keep the method focused.
- Add `reconcilePendingFuturesEntry` method (mirror of `reconcilePendingEntry`) that calls `confirmFuturesFill` on `PendingFuturesEntryOrderID`
- Monitor's scan cycle calls both reconciles when their respective field is non-empty. Defensive gate in `reconcilePendingEntry` skips when `PendingFuturesEntryOrderID != ""` (belt-and-suspenders).

**Option B: Persist confirmed spot fields with `PendingEntryOrderID = ""`** (leverage existing)
- Spot leg already confirmed (by the time we hit futures error, spot filled successfully)
- Save position with spot details, leave `PendingEntryOrderID` empty so existing `reconcilePendingEntry` skips
- Add separate recovery via `pendingEntryFuturesPosition` helper that queries the futures exchange for matching position and fills in futures details if found
- Less new code but more fragile (relies on futures position detection, which may miss if order placed but not yet confirmed on exchange)

**Chosen: Option A** (new field + dedicated reconciler). Safer semantics, no overloading. Adds:
- `models.SpotFuturesPosition.PendingFuturesEntryOrderID string \`json:"pending_futures_entry_order_id,omitempty"\`` (new field, omitempty for backward compat with existing JSON)
- `spotengine.persistPendingFuturesEntry(pos, orderID)` method
- `spotengine.reconcilePendingFuturesEntry(pos)` method called from monitor loop alongside existing `reconcilePendingEntry`
- Redis schema is position JSON with the new field — no separate key needed

**`confirmFuturesFill` all-callers audit** (corrected after v8 review):

`internal/spotengine/execution.go:679` is INSIDE `confirmFuturesFill` (not a caller). Actual callers at HEAD:

| Line | Context | Category | Handling |
|------|---------|----------|----------|
| 506 | Entry path (Dir A — borrow_sell_long) | **entry** | Pending-futures persist |
| 619 | Entry path (Dir B — buy_spot_short) | **entry** | Pending-futures persist |
| 1097 | `reconcilePendingEntry` (places hedge for already-pending entry) | **entry-retry** | Pending-futures persist — NOT bubble-error |
| 1418 | `ClosePosition` Dir A futures close | close | Bubble error, monitor retries next cycle |
| 1701 | `ClosePosition` Dir B futures close | close | Bubble error |
| 1909 | `emergencyClose` | close | Bubble error |

**Category A (entry 506, 619):** return `*pendingFuturesEntryError` so `ManualOpen` top-level catch handles pending-recovery.

**Category A-retry (reconcilePendingEntry 1097):** function is **void** (return nothing). Cannot return an error. Must inline-handle: persist `PendingFuturesEntryOrderID`, nil-guard broadcast, log failures, `return` (bare).

`pendingFuturesEntryError` struct (mirror `pendingSpotEntryError`):
```go
type pendingFuturesEntryError struct {
	posID         string
	orderID       string
	pendingPos    *models.SpotFuturesPosition // optional — set when entry flow has constructed pos
	capitalAmount float64                      // optional — actual spot fill amount for capital commit
	err           error
}

func (e *pendingFuturesEntryError) Error() string { return e.err.Error() }
func (e *pendingFuturesEntryError) Unwrap() error { return e.err }
```

**Entry paths (506, 619) — error-return pattern with spot-state handoff:**

CRITICAL: at these points, `PendingEntryOrderID` (= spot pending order ID) was SET earlier at execution.go:460/557 before spot confirmation, and is NORMALLY CLEARED after futures confirmation succeeds at execution.go:513/626. Our new error path returns BEFORE those clears — so the position ends up with both `PendingEntryOrderID` AND `PendingFuturesEntryOrderID` set. If we don't clear the spot one, `reconcilePendingEntry` will reprocess the already-confirmed spot leg and may place another hedge.

Fix: before returning pendingFuturesEntryError, persist spot-side confirmed state and clear `PendingEntryOrderID`:

```go
// BEFORE
futFilled, futAvg := e.confirmFuturesFill(futExch, orderID, pos.Symbol)
if futFilled <= 0 {
	return fmt.Errorf("futures entry got 0 fill (order %s)", orderID)
}

// AFTER (entry paths 506 Dir A / 619 Dir B)
futFilled, futAvg, cfErr := e.confirmFuturesFill(futExch, orderID, pos.Symbol)
if cfErr != nil {
	// Spot leg already confirmed above; persist its confirmed fields and
	// clear spot pending-order so existing reconcilePendingEntry does NOT
	// re-process the spot leg. Only PendingFuturesEntryOrderID remains,
	// routing to reconcilePendingFuturesEntry on next monitor cycle.

	// Field names confirmed at models/spot_position.go: SpotSize (NOT
	// SpotFilledQty), SpotEntryPrice, NotionalUSDT, FuturesSide, BorrowAmount.

	// Dir A (line 506):
	pos.PendingEntryOrderID = ""
	pos.SpotSize = spotFilled              // gross spot qty (Dir A borrows + sells)
	pos.SpotEntryPrice = spotAvg
	pos.NotionalUSDT = spotFilled * spotAvg
	pos.FuturesSide = "long"
	// BorrowAmount already set earlier after spot confirmation, no-op here
	// FuturesSize/FuturesEntry intentionally NOT set — futures leg state is unknown

	// Dir B (line 619) — use net received (matches HEAD's successful Dir B persistence
	// formula `actualNotional := spotFilledQty * spotEntryPrice` where the caller
	// passes `spotFilledQty = spotNetReceived` per return signature
	// `return spotAvg, futAvg, spotNetReceived, futFilled, nil`):
	// pos.SpotSize = spotNetReceived           // NET after fee deduction
	// pos.NotionalUSDT = spotNetReceived * spotAvg
	// pos.FuturesSide = "short"
	// (SpotEntryPrice, PendingEntryOrderID clear — same as Dir A)

	return &pendingFuturesEntryError{
		posID:         pos.ID,
		orderID:       orderID,
		pendingPos:    pos,
		capitalAmount: spotFilled * spotAvg, // actual filled notional (pre-fee for capital accounting)
		err:           cfErr,
	}
}
if futFilled <= 0 {
	return fmt.Errorf("futures entry got 0 fill (order %s)", orderID)
}
```

**Defensive gate in existing `reconcilePendingEntry`**: also add an early-return when pos has `PendingFuturesEntryOrderID` set. Prevents spot-reconciler stepping on futures-reconciler territory even if the clear above is missed. **Do NOT add `if pos.PendingEntryOrderID == "" { return }` gate** — existing recovery checkpoints may legitimately have empty `PendingEntryOrderID` (spot leg already confirmed, futures hedge still needs recovery), and skipping those would break established recovery paths.

```go
func (e *SpotEngine) reconcilePendingEntry(pos *models.SpotFuturesPosition) {
	// Skip only if this position's pending state belongs to the futures leg.
	// Do not return merely because PendingEntryOrderID is empty: existing
	// recovery supports durable checkpoints where the spot leg is already
	// confirmed and the futures hedge still needs to be recovered or placed.
	if pos.PendingFuturesEntryOrderID != "" {
		return
	}

	// existing body unchanged ...
}
```

**Entry-retry path (1097, inside void `reconcilePendingEntry`) — inline void-handling pattern:**
```go
// BEFORE (in reconcilePendingEntry)
futFilled, futAvg := e.confirmFuturesFill(futExch, orderID, pos.Symbol)
if futFilled <= 0 {
	// ... existing zero-handling ...
}

// AFTER (same function, void — cannot return error)
futFilled, futAvg, cfErr := e.confirmFuturesFill(futExch, orderID, pos.Symbol)
if cfErr != nil {
	// Persist pending-futures state directly (no error propagation via return).
	if saveErr := e.persistPendingFuturesEntry(pos, orderID); saveErr != nil {
		e.log.Error("reconcilePendingEntry %s: persistPendingFuturesEntry failed: %v", pos.ID, saveErr)
		// Don't broadcast stale state; just return so monitor retries next cycle.
		return
	}
	if e.api != nil {
		e.api.BroadcastSpotPositionUpdate(pos)
	}
	e.log.Warn("reconcilePendingEntry %s: futures fill unknown, pending-futures persisted, will retry next cycle: %v", pos.ID, cfErr)
	return
}
if futFilled <= 0 {
	// ... existing zero-handling ...
}
```

**Category B (close/exit/rotation — 1418, 1701, 1909):** fill-state-unknown on close means "position may be closed." Next monitor cycle checks actual exchange state; retry close is safe because close is idempotent (closing an already-closed position is a no-op / gets the correct error).

```go
// BEFORE
futFilled, futAvg := e.confirmFuturesFill(futExch, orderID, pos.Symbol)
if futFilled <= 0 {
	return fmt.Errorf("futures close got 0 fill (order %s)", orderID)
}

// AFTER (close paths)
futFilled, futAvg, cfErr := e.confirmFuturesFill(futExch, orderID, pos.Symbol)
if cfErr != nil {
	return fmt.Errorf("futures fill state unknown for order %s: %w", orderID, cfErr)
}
if futFilled <= 0 {
	return fmt.Errorf("futures close got 0 fill (order %s)", orderID)
}
```

**Related fix at `reconcilePendingEntry`:** v8 reviewer flagged that `pendingEntryFuturesPosition` failure at `execution.go:1049` currently logs and continues toward placing a new hedge. Under strict bitget errors, "pending entry's futures position check failed" must NOT fall through to new hedge placement (would double-hedge).

**Note:** `reconcilePendingEntry` currently has no return value at HEAD. Do NOT use `return fmt.Errorf(...)` — that would compile error. Just log + bare `return`:

```go
// BEFORE (~ execution.go:1049)
futPos, err := e.pendingEntryFuturesPosition(...)
if err != nil {
	e.log.Warn("...: %v", err)
	// falls through to place new hedge
}

// AFTER
futPos, err := e.pendingEntryFuturesPosition(...)
if err != nil {
	// Can't confirm existing hedge state — don't risk double-hedging.
	// Exit so next monitor cycle retries. Existing pending-entry record
	// remains in place; no new hedge placed.
	e.log.Warn("reconcilePendingEntry %s: hedge check failed, skipping this cycle: %v", pos.ID, err)
	return
}
```

If later evolution adds return value to `reconcilePendingEntry` (e.g. for tests), switch to `return fmt.Errorf(...)`. For now the signature is void — stay consistent.

**Implementation checklist:** after changing `confirmFuturesFill` signature, `go build ./...` compile errors pinpoint each caller. Apply category A (entry/entry-retry) or category B (close) pattern as documented above.

#### Test coverage required

Unit tests per call site:
- Mock exchange returns `(0, nil)` → caller treats as confirmed zero (rollback / retry)
- Mock exchange returns `(0, *APIError)` → caller marks partial/pending, does NOT rollback, does NOT place new order
- Mock exchange returns `(N, nil)` where N > 0 → caller uses actual value

**Regression test for the "spot-confirmed + futures-unknown" bug** (covers v11 review finding).

**Setup:**
- Mock futures exchange returns `(0, *APIError{Code: "40009"})` on `GetOrderFilledQty` for the target orderID.
- Spot leg mock confirms normally.
- Track observable call counters on both mocks: `GetSpotMarginOrderCalls`, `GetOrderFilledQtyCalls`.

**Dir A case:**
- `spotFilled = 1.234` (arbitrary nonzero), `spotAvg = 5.0`.
- Execute Dir A entry path (line 506 site).
- Exact assertions on resulting position state:
  - `pos.PendingEntryOrderID == ""` (spot cleared)
  - `pos.PendingFuturesEntryOrderID == orderID` (futures set)
  - `pos.SpotSize == 1.234` (exactly gross spotFilled — NOT just `> 0`)
  - `pos.SpotEntryPrice == 5.0` (exactly spotAvg)
  - `pos.NotionalUSDT == 6.17` (spotFilled * spotAvg)
  - `pos.FuturesSide == "long"`
  - `pos.BorrowAmount == 1.234` (already set earlier, still present)

**Dir B case:**
- Mock `GetSpotMarginOrder` with `FeeDeducted = 0.001` (nonzero); `SpotOrderRules.QtyStep = 0.01` so floor rounding applies.
- `spotFilled = 1.234`, `spotAvg = 5.0`, expect `spotNetReceived = 1.23` (after fee + step floor).
- Dir B requires TWO spot-order query responses: one for `confirmSpotFill`, one for fee deduction. Mock setup must provide both.
- Execute Dir B entry path (line 619 site).
- Exact assertions:
  - `pos.PendingEntryOrderID == ""`
  - `pos.PendingFuturesEntryOrderID == orderID`
  - `pos.SpotSize == 1.23` (exactly spotNetReceived — protects against accidental gross persistence)
  - `pos.SpotEntryPrice == 5.0`
  - `pos.NotionalUSDT == 6.15` (spotNetReceived * spotAvg = 1.23 * 5.0 — matches HEAD's successful Dir B `actualNotional := spotFilledQty * spotEntryPrice` where the caller passes `spotFilledQty = spotNetReceived`)
  - `pos.FuturesSide == "short"`
  - `capitalAmount == 6.17` preserved in pendingFuturesEntryError (spotFilled * spotAvg = 1.234 * 5.0 for capital accounting)

**Next monitor cycle:**
- Save the updated position to mock DB.
- Invoke monitor scan / reconcile flow.
- Assert observable counters:
  - `GetSpotMarginOrderCalls` unchanged (spot confirmation NOT re-invoked → proves `reconcilePendingEntry` early-returned on `PendingFuturesEntryOrderID != ""` gate)
  - `GetOrderFilledQtyCalls` incremented by 1 on futures mock (proves `reconcilePendingFuturesEntry` attempted)
  - OR: terminal futures WS update for `PendingFuturesEntryOrderID` observed
- After second reconcile attempt (this time mock returns `(filled, nil)` success): assert `pos.PendingFuturesEntryOrderID == ""` and `pos.FuturesSize > 0`.

**Risk:** Changing `GetOrderFilledQty` caller behavior is live-system-touching. Test on VPS with small-size bitget position before full deploy.

### 16. Additional bitget-silent-error call sites (from independent review)

Independent review found 3 additional sites where bitget silent errors cause problems:

#### 16a. `pkg/exchange/bitget/adapter.go:CheckPermissions` — migrate to `errors.As`

Currently uses `body.Code == "40009"` pattern on raw body to detect permission denied. After #1, the client returns `*APIError` instead of raw body, so this pattern breaks.

BEFORE (conceptual — exact lines to grep during implementation):
```go
raw, err := a.client.Get(...)
if err != nil { return exchange.PermUnknown }
var body struct { Code string }
json.Unmarshal([]byte(raw), &body)
if body.Code == "40009" { return exchange.PermDenied }
```

**CRITICAL — preserve "endpoint reached" inference**: current HEAD treats any non-nil error as "probe unknown" only because the legacy client returns raw body on all responses; in practice, a non-40009 HTTP 4xx from the permission endpoint meant the probe reached bitget (auth worked, permission check responded), which equals `PermGranted` (the endpoint exists and responded; 40009 is the only signal for `PermDenied`). Under strict errors (#1), every non-2xx becomes `*APIError`, so the naive migration below would flip expected validation failures to `PermUnknown` and break permission UI on bitget.

Must branch on code class: retryable codes (transient) → `PermUnknown`; 5xx HTTP status → `PermUnknown`; 40009 → `PermDenied`; any other `*APIError` (non-retryable, non-5xx, non-40009) → `PermGranted` (endpoint reached, just rejected for other reason).

AFTER:
```go
raw, err := a.client.Get(...)
var apiErr *APIError
if errors.As(err, &apiErr) {
	if apiErr.Code == "40009" {
		return exchange.PermDenied
	}
	if retryableCodes[apiErr.Code] {
		return exchange.PermUnknown
	}
	if n, convErr := strconv.Atoi(apiErr.Code); convErr == nil && n >= 500 && n <= 599 {
		return exchange.PermUnknown
	}
	return exchange.PermGranted
}
if err != nil {
	return exchange.PermUnknown
}
// existing success path
```

`retryableCodes` refers to the package-level map defined in `pkg/exchange/bitget/client.go` (used by `isRetryable` per #1). `strconv.Atoi` handles HTTP-status-synthesized APIError codes (#1 sets `apiErr.Code = strconv.Itoa(resp.StatusCode)` when body has no code). Import `strconv` in `adapter.go` if not already imported.

Apply same pattern to any other code-based branching (e.g. `EnsureOneWayMode` code `40774`). Grep for `body.Code ==` and `resp.Code ==` patterns in adapter.go during implementation.

#### 16b. `pkg/exchange/bitget/adapter.go:CancelAllOrders` — surface errors

Current HEAD adapter discards BOTH Post errors (ignores return values entirely) and returns `nil`. After #1, the client returns `*APIError` but the adapter still throws it away. Must capture both errors, join them, and return.

BEFORE:
```go
func (a *Adapter) CancelAllOrders(symbol string) error {
	a.client.Post("/api/v2/mix/order/cancel-plan-order", map[string]string{
		"symbol": symbol, "productType": "USDT-FUTURES",
	})
	a.client.Post("/api/v2/mix/order/batch-cancel-orders", map[string]string{
		"symbol": symbol, "productType": "USDT-FUTURES",
	})
	return nil
}
```

AFTER:
```go
func (a *Adapter) CancelAllOrders(symbol string) error {
	var errs []error
	if _, err := a.client.Post("/api/v2/mix/order/cancel-plan-order", map[string]string{
		"symbol": symbol, "productType": "USDT-FUTURES",
	}); err != nil {
		errs = append(errs, fmt.Errorf("cancel plan orders: %w", err))
	}
	if _, err := a.client.Post("/api/v2/mix/order/batch-cancel-orders", map[string]string{
		"symbol": symbol, "productType": "USDT-FUTURES",
	}); err != nil {
		errs = append(errs, fmt.Errorf("batch cancel orders: %w", err))
	}
	return errors.Join(errs...)
}
```

Requires `errors` import in adapter.go (add if not present). `errors.Join` is stdlib since Go 1.20 — confirmed available per `go.mod` (Go 1.26).

**Caller updates** — grep `CancelAllOrders(` in `internal/engine/consolidate.go` and `internal/engine/exit.go`. Each call site must capture the returned error and log it with position ID, exchange, and symbol context. For call sites inside goroutines or defer blocks, wrap in an anonymous function that logs the error instead of discarding:

```go
// BEFORE (generic pattern at caller)
exch.CancelAllOrders(symbol)

// AFTER
if err := exch.CancelAllOrders(symbol); err != nil {
	e.log.Warn("CancelAllOrders %s/%s (pos %s) failed: %v", exchName, symbol, posID, err)
}
```

Errors should be logged and continue (do not abort surrounding operation), matching existing patterns for cleanup calls elsewhere in engine.

#### 16c. `pkg/exchange/bitget/margin.go:populateBitgetFeeDeducted` — log fill fetch errors

Function lives in `margin.go` (not `adapter.go`). Currently swallows fee-fill query errors via `err == nil` guard. After #1, errors propagate but this helper's caller expects "fee unknown = 0 fee" semantic preserved. Add log on error path while keeping existing behavior (do not abort caller).

BEFORE (current pattern in margin.go — grep `populateBitgetFeeDeducted` to locate exact line):
```go
if raw, err := a.client.Get("/api/v2/spot/trade/fills", fillParams); err == nil {
	var resp fillResp
	if json.Unmarshal([]byte(raw), &resp) == nil && resp.Code == "00000" {
		for _, f := range resp.Data {
			if strings.EqualFold(f.FeeDetail.FeeCoin, baseCoin) {
				fee, _ := strconv.ParseFloat(f.FeeDetail.TotalFee, 64)
				totalFee += fee
				found = true
			}
		}
	}
}
```

AFTER:
```go
if raw, err := a.client.Get("/api/v2/spot/trade/fills", fillParams); err == nil {
	var resp fillResp
	if json.Unmarshal([]byte(raw), &resp) == nil && resp.Code == "00000" {
		for _, f := range resp.Data {
			if strings.EqualFold(f.FeeDetail.FeeCoin, baseCoin) {
				fee, _ := strconv.ParseFloat(f.FeeDetail.TotalFee, 64)
				totalFee += fee
				found = true
			}
		}
	}
} else {
	log.Printf("[bitget] populateBitgetFeeDeducted spot fills failed order=%s symbol=%s: %v", orderID, symbol, err)
}
```

Import requirement: add `"log"` to `pkg/exchange/bitget/margin.go` imports if not already present (grep `import` block to verify). Diagnostic-only; preserves existing "fee unknown → 0 fee" semantic on error.

### 17. `confirmFill` callers in exit.go and consolidate.go

#15b covers `confirmFill` signature change. This item ensures all callers outside engine.go are updated. Independent review flagged that plan focused on engine.go callers but didn't explicitly cover:

- `internal/engine/exit.go` — grep for `confirmFill(` during implementation, apply same caller pattern
- `internal/engine/consolidate.go` — same
- `internal/spotengine/execution.go` — `confirmFuturesFill` is different function; its callers covered in #15c

Implementation checklist: after changing `confirmFill` signature, compile check will flag every caller. Fix each to handle the new `error` return. No other code discovery needed.

## How (build + test plan)

**Build verification:**
```
go build ./...
go vet ./... (should show only pre-existing cmd/gatecheck warning)
```

**Unit tests:**
- `pkg/exchange/bitget/client_test.go` — mock httpClient, simulate HTTP 400 + error body, verify `*APIError` returned with correct code/msg
- `pkg/exchange/bitget/adapter_test.go` — test `GetFundingFees` non-ASCII fallback: mock client returns mixed-symbol bills, verify local filter returns correct subset
- Regression: existing bitget adapter tests must still pass

**Integration test (manual, requires VPS deploy):**
- Unit tests can't reach live bitget. Deploy to VPS under controlled window:
  1. Verify existing ASCII positions' funding tracking regresses (shouldn't)
  2. Check 龙虾USDT position's `funding_collected` updates correctly in next tracker cycle (should jump by +0.91 USDT or more)
  3. Check UI `/api/positions/{id}/funding` shows both binance AND bitget legs

## Risk

### Surfacing previously hidden errors (Problem A fix)

Changes #1 + #2-4 surface previously silent bitget errors. Grep all bitget callers to verify they handle errors (not swallow into misbehavior):
- `engine.go:1783, 1802` GetFundingFees — `if err == nil { ... gotShort = true }` — err case correctly skips
- `handlers.go:224` — `continue` on err — correct
- `engine.go:2630, 2682, 4278, 4309, 4587` GetOrderFilledQty — **caller inspection required**; if caller assumes `err == nil` means "definitive 0", may need to handle differently. Possible incident: engine retries order placement because "filled=0 means not filled"; if error surfaced, caller should retry query, not the order.

Mitigation: inspect each GetOrderFilledQty caller to ensure error handling doesn't trigger unintended retries. Add test for each call site.

### Retry behavior regression

`retryDo` loop currently checks `isRetryable(err, rawResp)`. After #1, rawResp is empty on error. Update `isRetryable` to inspect `*APIError.Code` (via `errors.As`) for retryable codes. Otherwise, previously-retrying transient errors become terminal.

### Non-ASCII fallback data volume

`/account/bill` without symbol limit=500 returns up to 500 bills across all symbols. For active traders, may still miss target symbol's events if window is wide. Accept this trade-off: fallback is only used for non-ASCII which should be rare.

### No-filter endpoint rate limit

Calling no-filter version is more data-intensive. Bitget per-endpoint rate limits may trigger if used heavily. Mitigation: cache the no-filter result for 30s similar to BingX's batch cache (`fundingFeesCacheMu`). Evaluate if needed after deploy.

### Reverting v0.32.42 guards (change #10)

Removing guards means non-ASCII symbols flow through entire pipeline. Other 5 exchanges' non-ASCII behavior is untested (change #11). If any silently fails like bitget did, will take longer to surface. Mitigation: run #11 empirical tests BEFORE reverting guards, or revert + monitor + fix next iteration.

Alternative: keep guards temporarily, revert in second phase after #11 validates peer exchanges. Defer choice to review.

### 龙虾USDT existing position

Current `funding_collected=-0.069` is stale. After fix:
- Next tracker cycle queries both legs correctly → sees larger total → updates funding_collected
- Historical missed events cannot be reconstructed (tracker uses current snapshot from exchange, not event log)
- Close PnL at exit uses fresh GetClosePnL query — should be accurate after fix

No manual intervention required — fix will auto-correct on next tracker cycle (runs hourly at HH:10).

### Interaction with in-flight orders

If deploy coincides with a bitget order being placed, `GetOrderFilledQty` behavior change may affect that order's tracking. Low risk: deploy during low-activity window, or add feature flag `EnableBitgetStrictErrors` default OFF, toggle on after monitoring.

## Feature Flag (decided: none)

No feature flag for strict error handling. Rationale:
- Other 5 exchanges do not have a flag for their already-correct error checking; treating bitget as exception institutionalizes the bug
- Default-OFF flag would leave buggy behavior as production default, the fix would only activate when someone remembers to toggle it
- This is a bug fix, not a feature; user preference for dashboard toggles applies to new features (`CLAUDE.md` convention)
- Rollback path is `git revert` of the commit; no in-band toggle needed

Risk mitigation is handled via staged deploy: deploy to VPS off-peak, monitor logs for previously-hidden bitget errors over a few hours, roll back if unexpected cascade.

## Review History

- v1: initial plan — kept v0.32.42 guards (blocked non-ASCII at source).
- v2: REVISED direction per user ("不要擋中文代幣"). Added revert plan (#10), empirical peer tests (#11), and full non-ASCII support across bitget adapter. Removed feature flag option per user's "direct fix" direction.
- v3: flag decision finalized (no flag). Codex first review → NEEDS-REVISION: 11 of 14 items need revision, plus critical safety gap (GetOrderFilledQty caller misuse).
- v4: (this version) — incorporates all v3 findings:
  * #1 client.go: added `bitgetPassThroughCodes` map for idempotent codes (40872, 43011, 43025) that existing adapter methods rely on; synthesize APIError.Code from HTTP status when body has no code; updated isRetryable to inspect `*APIError` via errors.As
  * #2-4: added `Msg` field to response structs
  * #5 GetFundingFees fallback: corrected limit to 100 (bitget max per docs), added `endTime` param, moved `containsNonASCII` to bitget package
  * #6: use `strings.EqualFold` for symbol filter
  * #7 GetUserTrades: concrete impl confirmed per bitget docs 1798 (`symbol` optional on `/order/fills`)
  * #8 GetClosePnL: concrete impl confirmed per bitget docs 3836 (`symbol` optional on `/history-position`); must add `Symbol` to response struct for local filter
  * #9 GetOrderFilledQty: resolved — `/order/detail` requires symbol, so fallback via `/order/fills` with `orderId` and sum `baseVolume`
  * #10 revert: added note NOT to revert VERSION/CHANGELOG (bump new version instead)
  * #11 peer tests: new `cmd/peertest/` CLI using dynamic symbol discovery via LoadAllContracts, read-only
  * #13 handler: switched to X-Partial-Legs header approach (less breaking, no response shape change)
  * #14: narrowed scope to consolidate.go only (exit.go already logs)
  * **#15 NEW**: caller safety for GetOrderFilledQty at engine.go:2630/2682/4278/4309/4587 and spotengine/execution.go:679 — prevent duplicate orders on (0, err)
- v5: addresses v4 Codex findings (named err, concrete frontend, concrete #15).
- v6: addresses v5 Codex findings (imports, confirmFill, pendingFuturesEntryError).
- v6 Codex normal review: ALL PASS.
- v6 Codex independent review (fresh thread, xhigh): NEEDS-REVISION — 5 findings.
- v7: addresses v6 independent review findings (imports, pagination, pendingFuturesEntryError, 5xx retry, CheckPermissions/CancelAllOrders/populateBitgetFeeDeducted).
- v7 Codex independent re-review (fresh thread, xhigh): NEEDS-REVISION — 2 findings.
- v8: addresses v7 independent review (pagination cursor via endId, confirmFuturesFill non-entry callers).
- v8 Codex independent re-review (fresh thread): NEEDS-REVISION — 2 findings.
- v9: addresses v8 independent review (confirmFuturesFill caller categorization, pendingEntryFuturesPosition strict return).
- v9 Codex independent re-review (fresh thread): NEEDS-REVISION — 2 findings.
- v10: addresses v9 findings (reconcilePendingEntry void, commitSpotCapital signature).
- v10 Codex independent re-review (fresh thread): NEEDS-REVISION — 2 findings.
- v11: addresses v10 findings (ManualOpen pending-futures mirror, reconcilePendingEntry void handling).
- v11 Codex independent re-review: NEEDS-REVISION — 1 finding.
- v12: addresses v11 findings (spot clear + defensive gate).
- v12 Codex independent re-review: NEEDS-REVISION — 3 findings (field names, Dir A/B specifics, plan conflict).
- v13: addresses v12 (SpotSize field name, Dir A/B values, persistPendingFuturesEntry clarification).
- v13 Codex independent re-review: NEEDS-REVISION — 2 test spec precision findings.
- v14: addresses v13:
  * Regression test spec tightened with exact value assertions: Dir A `SpotSize == spotFilled`, Dir B `SpotSize == spotNetReceived` (prevents accidental gross persistence in Dir B). Specific `spotFilled=1.234`, `spotAvg=5.0`, `FeeDeducted=0.001`, `QtyStep=0.01` values documented.
  * Monitor-pass assertions now track observable counters: `GetSpotMarginOrderCalls` unchanged (proves spot reconciler gated), `GetOrderFilledQtyCalls` incremented (proves futures reconciler ran).
  * Added recovery-success assertion: after second reconcile with mock returning `(filled, nil)`, `PendingFuturesEntryOrderID` cleared and `FuturesSize > 0`.
- v14 Codex independent re-review (fresh thread, xhigh): NEEDS-REVISION — 2 findings (#15 defensive gate too strict; #16a CheckPermissions needs retryable/5xx branching).
- v15: addresses v14:
  * #15 defensive gate: removed `if pos.PendingEntryOrderID == "" { return }` check. Existing recovery checkpoints may legitimately have empty `PendingEntryOrderID` (spot leg confirmed, futures hedge still pending recovery). Only gate on `PendingFuturesEntryOrderID != ""`. Added explanatory comment in AFTER block.
  * #16a CheckPermissions: added "endpoint reached" inference preservation. Under strict errors, differentiate `*APIError` classes: 40009 → `PermDenied`; `retryableCodes[code]` → `PermUnknown`; 5xx HTTP status (via `strconv.Atoi`) → `PermUnknown`; any other `*APIError` → `PermGranted` (endpoint reached, rejected for non-auth reason). Documented `retryableCodes` reference to `client.go` and `strconv` import requirement.
- v15 Codex independent re-review (resume thread): ALL PASS (delta-only).
- v15 Codex FRESH-thread full independent audit (xhigh): NEEDS-REVISION — 2 findings not caught by resume-thread review:
  * #1 metrics note: plan claimed `doRequest` uses named returns, actual HEAD uses unnamed returns + local `var err error`. Text was inaccurate.
  * #16b CancelAllOrders: "audit call sites" too vague; bitget adapter still silently discards both Post errors (returns nil unconditionally). Needed concrete BEFORE/AFTER + caller update pattern.
- v16: addresses v15 fresh-audit findings:
  * #1 metrics note: rewritten to describe unnamed return + local `var err error` pattern; clarified new error paths must assign to local `err` before returning.
  * #16b CancelAllOrders: added concrete BEFORE/AFTER adapter code using `errors.Join`; `errors` import note; caller update pattern with log+continue semantics.
- v16 Codex fresh-thread full independent audit (xhigh): NEEDS-REVISION — 4 findings:
  * #1: imports BEFORE/AFTER block was abbreviated (5 lines); actual HEAD has 15 lines. Also the inline comment in client.go AFTER still said "named `err` return var" after v16 fixed the prose.
  * #13: backend snippet didn't preserve existing `[]fundingEvent{}` empty-array pattern; frontend bypass dropped existing 401 handling (token clear + reload).
  * #15: Dir B NotionalUSDT used gross `spotFilled * spotAvg`; actual HEAD successful Dir B uses net `spotNetReceived * spotAvg` (caller passes `spotFilledQty = spotNetReceived`).
  * #16c: function lives in `margin.go`, not `adapter.go`; plan lacked concrete BEFORE/AFTER and `log` import note.
- v17: (this version) — addresses v16 fresh-audit findings:
  * #1: expanded imports BEFORE/AFTER to match actual HEAD (15 → 17 imports, adding `errors` and `strconv`); updated inline code comment from "named `err` return var" to "local `err` variable captured by the metrics defer".
  * #13 backend: switched from two `writeJSON` branches to single `writeJSON` after normalizing `events = []fundingEvent{}` when nil. Preserves on-wire empty-array behavior.
  * #13 frontend: added `res.status === 401 → clearToken() + reload()` branch to bypass, plus JSON-error-body extraction on non-401 non-ok responses. Noted `getToken`/`clearToken` name verification during implementation.
  * #15 Dir B: changed test-spec `pos.NotionalUSDT == 6.17` to `== 6.15` (spotNetReceived * spotAvg = 1.23 * 5.0); added `capitalAmount == 6.17` preservation assertion; tightened plan comment on Dir B persistence formula referencing HEAD `actualNotional := spotFilledQty * spotEntryPrice` with `spotFilledQty = spotNetReceived`.
  * #16c: retitled section to `margin.go`; added concrete BEFORE/AFTER code from HEAD with `log.Printf` on error path; noted `"log"` import requirement.
- v13 original: (kept for history)
  * Field name corrected: `SpotSize` (NOT `SpotFilledQty`) per HEAD models/spot_position.go
  * Dir A vs Dir B specific fields documented: Dir A uses `SpotSize = spotFilled` (gross, borrowed+sold); Dir B uses `SpotSize = spotNetReceived` (net after fee deduction). Each sets correct `FuturesSide`.
  * Fields persisted before return: SpotSize, SpotEntryPrice (= spotAvg), NotionalUSDT (= spotFilled*spotAvg), FuturesSide. BorrowAmount already set earlier.
  * Plan conflict resolved: `persistPendingFuturesEntry` does NOT clear `PendingEntryOrderID`; enforcement is at CALLER level (entry paths 506/619). Defensive gate in reconcilePendingEntry is belt-and-suspenders.
  * Added regression test spec: asserts spot cleared + futures set + SpotSize/SpotEntryPrice/NotionalUSDT populated; next monitor pass does NOT call confirmSpotFill.
- v12 original: (kept for history)
  * Entry paths 506/619 now explicitly clear `pos.PendingEntryOrderID` and persist confirmed spot-leg fields (SpotEntryPrice, SpotFilledQty, etc.) BEFORE returning `pendingFuturesEntryError`. Without this, position ends up with both PendingEntryOrderID and PendingFuturesEntryOrderID set, causing `reconcilePendingEntry` to reprocess already-confirmed spot leg and potentially place duplicate hedge.
  * Added defensive gate in `reconcilePendingEntry`: early-return if `pos.PendingFuturesEntryOrderID != ""`. Delegates routing to `reconcilePendingFuturesEntry` for positions in futures-pending state.
- v11 original: (kept for history)
  * ManualOpen pending-futures branch fully mirrors pending-spot pattern: adds `recoverySaveFailed` tracking, `pendingFutErr.pendingPos` override, `pendingFutErr.capitalAmount` override, nil-guard broadcast (`e.api != nil`), wrapped error return on persistence failure.
  * Separated Category A into A (entry, 506/619 — error-return) vs A-retry (1097, void `reconcilePendingEntry` — inline handling). A-retry uses direct `persistPendingFuturesEntry` + nil-guard broadcast + bare `return` pattern instead of `return &pendingFuturesEntryError{...}`.
  * `pendingFuturesEntryError` struct fields documented: posID, orderID, pendingPos, capitalAmount, err. Mirrors `pendingSpotEntryError` fields.
- v10 original: (kept for history)
  * `reconcilePendingEntry` has no return value at HEAD — v9 `return fmt.Errorf(...)` would fail compile. Changed to `log + bare return` with note that value-returning refactor is separate.
  * `e.commitEntryCapital(entryPos)` does not exist — existing code uses `commitSpotCapital(reservation, posID, amount)`. Updated pending-futures branch to use the real signature with error handling (mirror of pending-spot branch pattern).
- v9 original: (kept for history)
  * confirmFuturesFill line numbers corrected: entry callers are 506 (Dir A) and 619 (Dir B), not 679 (which is inside the function itself). Added table mapping each of 506/619/1097/1418/1701/1909 to category (entry / entry-retry / close) with distinct handling.
  * execution.go:1097 is `reconcilePendingEntry` (entry-retry), NOT close — reclassified to category A (pending-futures persist) to prevent double-hedge on retry.
  * Added fix for `pendingEntryFuturesPosition` failure at execution.go:1049 — current code logs and continues to place new hedge; under strict errors must return to prevent double-hedging.
- v8 original: (kept for history)
  * #2 client.go: documented code-based branches NOT covered by pass-through (`CheckPermissions` 40009, `EnsureOneWayMode` 40774); noted migration to `errors.As(*APIError)` required
  * #1 retry: added 5xx HTTP status retry handling in `isRetryable`
  * #5 GetFundingFees fallback: pagination via `idLessThan` with stop conditions (empty response, oldest-cTime crossed threshold, less than 100 rows). Applies to #7/#8/#9 no-symbol endpoints
  * #15c pendingFuturesEntryError: switched from overloaded `PendingEntryOrderID` to dedicated `PendingFuturesEntryOrderID` field + `reconcilePendingFuturesEntry` method. Prevents spot reconciler mis-handling futures order ID.
  * #16 NEW: `CheckPermissions` migration to `errors.As`; `CancelAllOrders` / `populateBitgetFeeDeducted` error surfacing
  * #17 NEW: confirmFill callers in exit.go / consolidate.go / spotengine — compile-check-driven audit after signature change
