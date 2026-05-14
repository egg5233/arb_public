# Plan — Bybit Contract-Agreement Skip-List (110126)

**Status:** DRAFT — review before any code is touched
**Date:** 2026-05-14
**Driver:** follow-up to v0.39.3 (Bybit entry preflight)
**Config default:** ON (deviates from "default OFF" convention — this fixes a real
orphan-risk retry loop, not a new risk filter; documented deviation)

---

## Problem

When a Bybit entry preflight hits API error `110126`
("You must sign the required agreement before trading this contract"),
`engine.go:3831/3839` applies a generic **30-minute `SetReEnterCooldown`**.

Consequences:
- The symbol is retried every 30 min and re-fails forever — the agreement
  only clears via a one-time manual sign on the Bybit UI.
- No persistence beyond the 30-min TTL.
- No discovery exclusion — the symbol keeps getting ranked and dispatched.
- No operator visibility — nobody is told *which* symbol needs a manual sign.

## Goal

Persistently exclude agreement-gated Bybit symbols from discovery until the
operator signs on Bybit and clears the block. Surface the blocked set on the
dashboard. Self-heal when a real order later succeeds.

---

## Design (mirrors the existing `IsDelisted` blacklist pattern in `delist.go`)

### 1. Classify the error — `internal/engine/`
- Add `isBybitAgreementError(err error) bool` — case-insensitive match on
  `110126` and/or "sign the required agreement". Keep alongside the existing
  `isBingXAPIOrdersTemporarilyDisabled` helper.
- In `preflightEntryOrder` (`engine.go:2759`): no behavior change here — keep
  returning the wrapped error. Classification happens at the call site so the
  leg/exchange context is available.

### 2. Route at the preflight call sites — `internal/engine/engine.go:3827-3842`
- For the long and short preflight blocks: if the failing exchange is `bybit`
  **and** `isBybitAgreementError(err)` **and** `cfg.EnableAgreementSkiplist`:
  call `e.discovery.SetAgreementBlock("bybit", opp.Symbol, reason)` instead of
  the 30-min `SetReEnterCooldown`.
- Otherwise: unchanged (generic 30-min cooldown).
- Log at WARN with the symbol + "needs manual sign on Bybit UI".

### 3. Persist — `internal/database/state.go`
New Redis-backed methods on `*database.Client`, modeled on the delist block:
- key prefix: `keyAgreementBlockPrefix = "arb:agreement_block:"`
  → full key `arb:agreement_block:<exchange>:<symbol>`
- `SetAgreementBlock(exchange, symbol, reason string) error` — **no TTL**
  (value = JSON `{reason, blocked_at}`).
- `IsAgreementBlocked(exchange, symbol string) bool`
- `ListAgreementBlocks() []AgreementBlock` — scan `arb:agreement_block:*`
- `ClearAgreementBlock(exchange, symbol string) error`
- New type `AgreementBlock{Exchange, Symbol, Reason string; BlockedAt time.Time}`.

### 4. Scanner surface — `internal/discovery/scanner.go` (or new `agreement.go`)
Thin pass-throughs on `*discovery.Scanner` (engine holds the concrete type —
**no `models.Discoverer` interface change needed**, confirmed at `engine.go:68`):
- `SetAgreementBlock(exchange, symbol, reason string)`
- `IsAgreementBlocked(exchange, symbol string) bool`
- `ListAgreementBlocks() []database.AgreementBlock`
- `ClearAgreementBlock(exchange, symbol string)`

### 5. Discovery exclusion filter — `internal/discovery/scanner.go`
Add a filter step next to the delist filter (currently `:1073`, "step 8"):
```
// 9. Filter agreement-blocked symbols (all scan types).
if s.cfg.EnableAgreementSkiplist {
    // drop any opp where LongExchange or ShortExchange is agreement-blocked
    // for opp.Symbol; record rejection in rejStore with reason.
}
```
Note: `110126` is **per-contract on one exchange**, so only drop the opp when
the *blocked* exchange is actually a leg of that opp — a Bybit block on
`HOODUSDT` must not exclude a Binance/Gate `HOODUSDT` pair.

### 6. Dashboard alert — `internal/api/` + `web/`
When a symbol gets newly agreement-blocked, surface a **dashboard alert** so the
operator knows a manual Bybit sign is needed.
- Backend: include the agreement-block set in the WS state push (or a dedicated
  alert field) so a new block appears without a manual refresh.
- Frontend: visible alert banner / notification — "Bybit <SYMBOL> needs
  agreement sign-off — sign on Bybit UI, then clear here". Links to / highlights
  the Safety-tab blocked list.
- Confirm the exact alert mechanism during implementation (reuse whatever
  existing dashboard alert/notification surface exists, if any).
- **No self-heal:** blocks clear only via the operator's manual "Clear" button
  (per user decision — keep it explicit, no auto-clear on order success).

### 7. Config switch — `internal/config/config.go`
Per the rollout pattern, with **default ON**:
- struct field `EnableAgreementSkiplist bool` (near `DelistFilterEnabled`)
- JSON tag `enable_agreement_skiplist` in the discovery sub-block
  (alongside `delist_filter` — overlay struct, apply-overlay, two
  serialize sites at `:1919` and `:2424`, and env override if pattern requires)
- default `true` in the defaults constructor (`:823` area)
- **Deviation note:** convention is "default OFF"; documented here because the
  feature corrects a defect rather than adding optional risk behavior.

### 8. Dashboard — `web/` (Safety tab)
- Toggle: "Bybit agreement skip-list" bound to `enable_agreement_skiplist` via
  `/api/config` (in-memory + `config.json` + `.bak`, standard path).
- Blocked-symbols list: symbol, exchange, reason, blocked-at (Asia/Taipei).
- Per-row "Clear" button → new authenticated endpoint.
- i18n: add keys to **both** `web/src/i18n/en.ts` and `web/src/i18n/zh-TW.ts`.

### 9. API — `internal/api/handlers.go` + `server.go`
- `GET  /api/agreement-blocks`        → `ListAgreementBlocks()`
- `POST /api/agreement-blocks/clear`  → body `{exchange, symbol}` →
  `ClearAgreementBlock()`. Bearer-token auth, standard `{ok,data,error}` envelope.
- Register routes in `server.go`.
- Optionally include the block list in the WS state push (defer if scope-heavy).

---

## Files touched

| File | Change |
|------|--------|
| `internal/engine/engine.go` | `isBybitAgreementError`, reroute preflight call sites |
| `internal/database/state.go` | `AgreementBlock` type + 4 methods + key prefix |
| `internal/discovery/scanner.go` | 4 pass-throughs + exclusion filter step |
| `internal/config/config.go` | `EnableAgreementSkiplist` field, JSON overlay, default, serialize |
| `internal/api/handlers.go` | 2 handlers |
| `internal/api/server.go` | 2 routes |
| `web/src/...` (Safety tab) | toggle + blocked list + clear button |
| `web/src/i18n/en.ts`, `zh-TW.ts` | new keys (kept in sync) |

## Tests

- `engine`: `isBybitAgreementError` true/false cases; preflight `110126` →
  `SetAgreementBlock` (not 30-min cooldown); non-agreement bybit error →
  still generic cooldown; self-heal clears block on successful order.
- `database`: set/is/list/clear round-trip; no-TTL persistence.
- `discovery`: agreement-blocked Bybit leg dropped; same symbol on a
  non-Bybit pair **kept**; filter no-op when switch OFF.
- `api`: list + clear handlers, auth protection; block set present in WS push.
- `web`: static/i18n sync checks for the new keys; alert renders on new block.

## Out of scope (explicitly deferred / decided)

- **Auto-signing** the Bybit agreement via API — deferred per user. Needs a
  live Bybit V5 docs check (Playwright, per verify-before-implement rule);
  local `doc/bybit/` shows no such endpoint. If/when added, mirror the existing
  Binance `TradFiSigner` interface pattern.
- **Self-heal / auto-clear** — dropped per user decision. Blocks clear only via
  the manual dashboard "Clear" button.
- **Spot-futures engine** — NOT covered per user decision. This plan is
  perp-perp engine only; spotengine's own preflight path
  (`internal/spotengine/execution*.go`) is untouched.
- Telegram alert for blocked symbols — Telegram is spot-futures-only today;
  the dashboard alert (section 6) + WARN log is the visibility surface.

## Resolved (previously open)

1. ~~Self-heal insertion point~~ — dropped; replaced by dashboard alert (§6).
2. ~~Env-var override for `enable_agreement_skiplist`~~ — no; `delist_filter`
   has no env override, new switch matches it (config.json only).
3. ~~Spot-futures coverage~~ — no; perp-perp only.

## Versioning

- Bump `VERSION` + `CHANGELOG.md` (every commit — project rule).
- Suggested: `0.40.0` (new feature + config field + API + dashboard).
