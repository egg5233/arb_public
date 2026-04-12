# Plan: Bybit Withdraw — add required `accountType` field
Version: v2
Date: 2026-04-12
Status: ALL PASS

## Problem

`pkg/exchange/bybit/adapter.go:917-926` calls `POST /v5/asset/withdraw/create` without the `accountType` field. Per local doc `doc/EXCHANGEAPI_BYBIT.md:1318`, this field is **required**:

> | accountType | true | string | Wallet to withdraw from. `FUND`: Funding wallet. `UTA`: system transfers to Funding first. `FUND,UTA`: combo (Funding first, then UTA for remainder) |

Production impact: every Bybit donor rebalance withdrawal fails with `code=131001 Account available balance insufficient` because the request is missing a required field. Verified via 6+ VPS journalctl entries 2026-04-09 through 2026-04-12, and zero successful `bybit [rebalance] -> X` entries in Redis `arb:transfers`.

Prior art: v0.32.4 commit `14c0ffae` added `walletType=1` to BingX Withdraw for the same shape of bug.

## Change (single-file, one added field)

### `pkg/exchange/bybit/adapter.go:917-926` — add `accountType: "UTA"` to the request body

```go
reqParams := map[string]string{
    "coin":        params.Coin,
    "chain":       chain,
    "address":     params.Address,
    "amount":      params.Amount,
    "timestamp":   fmt.Sprintf("%d", time.Now().UnixMilli()),
    "accountType": "UTA", // required by API. UTA = system transfers UNIFIED->Funding before withdraw
}
```

## Why `UTA` (value choice per docs)

The bot's Bybit adapter hardcodes `IsUnified()=true` (`pkg/exchange/bybit/adapter.go:48-49`), and the rebalance allocator skips donor-side `TransferToSpot` for unified exchanges (`internal/engine/allocator.go:1519-1526`), so funds live in the UNIFIED pool at withdraw time. `UTA` is the documented value for "system transfers to Funding first" — Bybit handles the internal move in one API call, matching our existing flow without code changes.

`FUND` would require a working spot wallet balance that the allocator doesn't prep. `FUND,UTA` would consume any FUND dust first (unmodeled in allocator bookkeeping). Neither fits the current flow as cleanly as plain `UTA`.

## Effect on other callers

`pkg/exchange/bybit/adapter.go:Withdraw` is shared by rebalance (`internal/engine/allocator.go:1712-1717`) and manual transfer (`internal/api/handlers.go:1825-1836`). This patch changes both call paths:

- **Rebalance (primary target)**: funds already live in UNIFIED; `UTA` directs Bybit to internally transfer UTA→Funding and withdraw. Matches the existing flow exactly.
- **Manual transfer (secondary effect)**: the handler currently calls `TransferToSpot()` (UTA→FUND) before `Withdraw()`. Per doc (`doc/EXCHANGEAPI_BYBIT.md:1318`), `UTA` mode "system transfers to Funding first". With a prior FUND pre-fill, the interaction with plain `UTA` is NOT documented explicitly (only `FUND,UTA` is documented as "Funding first, then UTA remainder"). Manual path may therefore double-handle funds or leave pre-fill stranded in FUND. This is an improvement over the current state (where Withdraw returns 131001 and the call fails outright), but it is not necessarily optimal.

If post-deploy observation shows manual Bybit transfers still misbehaving, a follow-up patch should consider either:
- Switching handler's Bybit branch to `AccountType="FUND,UTA"` (doc-explicit fallback semantics), or
- Skipping the handler's `TransferToSpot()` for Bybit entirely (UTA mode handles the move).

Either follow-up is out of scope for THIS patch, which addresses the documented-required-field violation that blocks rebalance in production today.

## Other Bybit endpoints

`GetWithdrawFee` / transfer-log endpoints verified — they don't take `accountType`. No additional required-field violations found in this adapter.

## Risk

- **Classic (non-UTA) account incompatibility**: the codebase hardcodes Bybit as unified (`IsUnified()=true`), so a classic account would already be mis-handled by other logic. No regression here.
- **Undocumented behavior**: local Bybit doc only documents `FUND` / `UTA` / `FUND,UTA`. Online docs show `EARN` option added post-doc-snapshot. `UTA` semantics are stable across versions.
- **Backward compatibility**: all prior Bybit Withdraw calls were rejected with 131001 anyway. Adding the required field makes the API call well-formed — it does not change the success path for any previously-working call.

## Review History
- v1: Codex (gpt-5.4 high) — NEEDS-REVISION (wording only) — "Scope NOT included" misstated that manual transfer path is separable. The shared `Withdraw()` is called by both rebalance and manual path, so the adapter change affects both.
- v2: Codex (gpt-5.4 high, resumed) — **ALL PASS** — v2 correctly frames the shared Withdraw, limits the patch to the one doc-mandated field, and treats manual-path interaction as follow-up out of scope.
