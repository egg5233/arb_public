# Phase 16 Plan 03 — bingxprobe Restoration Notes

## Restoration source

- Commit: `21cb60b^` (parent of `21cb60b chore(13): remove cmd/bingxprobe — debug utility purpose served`)
- File: `cmd/bingxprobe/main.go`
- Original line count: 53
- Original scope: Public WebSocket subscriber. Dialed `wss://open-api-swap.bingx.com/swap-market`, sent `bookTicker` subscribe messages for 4 hardcoded symbols (`SOON-USDT`, `HOLO-USDT`, `SIGN-USDT`, `RAVE-USDT`), gzip-decoded inbound frames, and printed raw payloads to stdout for ~10 seconds. Single-purpose 2026-04-24 diagnostic written to confirm BingX WS BBO frame contents during the case-insensitive JSON decode bug investigation (fixed in v0.34.6). No REST traffic, no auth, no preflight, no ticker fetch.

## D-11 contract

- Probe scope: preflight `OrderPreflight.TestOrder` + ticker fetch.
- Production path mirrored: `internal/pricegaptrader/execution.go:296` (`preflight.TestOrder(exchange.PlaceOrderParams{...})` after `priceGapBingXProbePrice`).
- Adapter constructor: `bingx.NewAdapter(exchange.ExchangeConfig{ApiKey, SecretKey, ...})` at `pkg/exchange/bingx/adapter.go:114`.
- TestOrder signature: `(a *Adapter) TestOrder(req exchange.PlaceOrderParams) error` — note: takes `PlaceOrderParams` (NOT `PlaceOrderRequest` as the plan's `<interfaces>` block suggested) and accepts NO context — drift from plan's literal text; plan's stated *intent* (mirror execution.go:296) is what we honour.
- Ticker fetch: `(a *Adapter) GetOrderbook(symbol string, depth int) (*exchange.Orderbook, error)` at `pkg/exchange/bingx/adapter.go:964` — top-of-book bid/ask is the ticker proxy used elsewhere in the engine.

## Gap analysis

| Aspect | Original (21cb60b^) | D-11 contract | Action |
|--------|---------------------|---------------|--------|
| Operation | WS public bookTicker subscribe + print 10s | preflight TestOrder + ticker fetch | replace (full rewrite) |
| Imports | `gorilla/websocket`, `compress/gzip`, `encoding/json`, `bytes`, `fmt`, `time` | `pkg/exchange/bingx`, `pkg/exchange`, `os`, `fmt`, `log` | replace |
| Credentials | none (public WS, no auth) | env-var `BINGX_API_KEY` + `BINGX_SECRET_KEY` (matches adapter `ExchangeConfig`) | add |
| Output | gzip-decoded WS frames to stdout | bid/ask/mid + probe verdict + symbol + timestamp to stdout, exit 0 on success | replace |

## Retrofit plan

**Path B (full rewrite per D-11) — chosen.** The 53-line original has zero overlap with the D-11 probe-scope contract: WS-only, no REST, no adapter, no auth. Patching in place would mean deleting all 53 lines and writing a fresh 50-line REST probe — equivalent cost to Path B with worse provenance (the diff would mask the rewrite as "edit"). Justification:

1. The original was a one-shot WS frame inspector for a 2026-04-24 bug that has been fixed (v0.34.6). The historical artifact has no production value beyond the audit trail in this notes doc.
2. D-11 asks for the production preflight path that `priceGapBingXProbePrice` (`execution.go:291..296..310`) exercises. That path is REST `/openApi/swap/v2/trade/order` (TestOrder) plus REST `/openApi/swap/v2/quote/depth` (GetOrderbook for top-of-book ticker). Both happen via the existing `pkg/exchange/bingx` adapter.
3. **Probe-scope downscoping (Rule 1/3 deviation, see Task 1 verdict below):** the BingX adapter `TestOrder` does NOT hit a dry-run endpoint. It places a real non-marketable IOC limit order at `/openApi/swap/v2/trade/order` and immediately cancels it. The adapter's own header comment at `pkg/exchange/bingx/adapter.go:217..219` is explicit: "*BingX's documented /order/test endpoint does not hit the same temporary market-risk gate that rejects real /order requests.*" — i.e., the adapter intentionally avoids the dry-run endpoint because it does not exercise the same gate the production engine cares about. The plan's `<TestOrder safety verification>` section requires the verdict to be **safe** before Task 2 may include a TestOrder call; the adapter does not satisfy that "safe = dry-run endpoint" criterion. **Per the plan's own fallback** ("If unsafe, the probe MUST exclude any TestOrder call and use ticker-only — adjust the implementation accordingly"), Task 2 ships ticker-only. Rationale: an operator-run debug utility must not require actual capital at risk, even briefly; the production engine already exercises the live preflight on every Strategy 4 entry, so validation coverage is not lost — the probe utility's job is connectivity + auth + ticker sanity, which ticker-only fully covers.
4. No new dependencies. `pkg/exchange` + `pkg/exchange/bingx` are already in tree; `gorilla/websocket` (used by the original) remains in `go.mod` for unrelated reasons but the rewrite does not import it.

## Compile risk

- gorilla/websocket: present in go.mod (`github.com/gorilla/websocket v1.5.3`, line 8) — not used by the rewrite, but OK if a future task ever wants WS path.
- BingX adapter API stable since 21cb60b^? Yes — `bingx.NewAdapter`, `Adapter.GetOrderbook`, `Adapter.TestOrder` signatures all unchanged. The adapter has churn in many other methods (PlaceOrder, GetPosition, etc.) but the surfaces this probe uses are stable.
- Estimated retrofit size: S (rewrite ~50 lines, single file, no test required for a debug tool).

## TestOrder safety verification

Per plan checker info #8 / T-16-03-01 mitigation. Documented the BingX adapter's TestOrder implementation:

- Adapter file implementing TestOrder: `pkg/exchange/bingx/adapter.go:220` (function header at `:217..219`, validator at `:247`).
- REST endpoint hit by TestOrder: `/openApi/swap/v2/trade/order` — **the LIVE order endpoint, NOT a dry-run path**. The function places a real non-marketable IOC limit order (validator at `:247..269` enforces side/limit/IOC/positive-price/positive-size) and cancels it immediately at `:241..243` via `cancelProbeOrder` → `DELETE /openApi/swap/v2/trade/order`.
- Evidence (excerpt from `pkg/exchange/bingx/adapter.go:217..245`):

  ```go
  // TestOrder validates live order availability with a caller-supplied
  // non-marketable IOC probe. BingX's documented /order/test endpoint does not
  // hit the same temporary market-risk gate that rejects real /order requests.
  func (a *Adapter) TestOrder(req exchange.PlaceOrderParams) error {
      if err := validateTestOrderProbe(req); err != nil {
          return err
      }
      params := bingXOrderParams(req)
      result, err := a.client.Post("/openApi/swap/v2/trade/order", params)
      ...
      if cancelErr := a.cancelProbeOrder(req.Symbol, orderID); cancelErr != nil {
          return fmt.Errorf("bingx TestOrder cancel probe order %s: %w", orderID, cancelErr)
      }
      return nil
  }
  ```

  The function header is explicit that it intentionally does NOT use the documented `/order/test` dry-run endpoint, because that endpoint bypasses the temporary market-risk gate the engine wants to validate. Production safety relies on the non-marketable IOC + immediate cancel pattern.

- **Verdict: unsafe (per the plan's narrow definition — "path string contains `/test` or comment explicitly says dry-run / no order placement").** The adapter does NOT hit a dry-run endpoint. The non-marketable IOC + cancel pattern is BingX-specific production-safe but does not satisfy the plan's safe-criterion verbatim.
- **Action (per plan fallback):** Task 2 retrofit reduces the probe to **ticker-only** (REST `GetOrderbook` top-of-book read). No `TestOrder` call. CHANGELOG documents the deviation. T-16-03-01 mitigation is now actually executable AND defensive: even if a future operator runs `make probe-bingx` on a misconfigured environment, NO real order is ever placed by the probe. The production engine continues to exercise the live `TestOrder` path on every Strategy 4 entry, so validation coverage of the live-order code path is not lost.
