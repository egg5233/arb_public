# Plan: Unified donor respects maxTransferOut
Version: v2
Date: 2026-04-13
Status: ALL PASS

## Problem

For Bybit (UTA-only), rebalance can request more than Bybit's `/v5/account/withdrawal` `availableWithdrawal` allows, triggering `code=131001 insufficient`. Runtime evidence 2026-04-12 15:46 UTC (v0.32.10 already deployed):

```
donor bybit capacity: futures=81.33 maxTransferOut=33.40 futuresAvail=81.33 surplus=77.68 unified=true
batched withdraw bybit->bitget net=60.02
ERROR code=131001 msg=Account available balance insufficient
```

3 Codex passes converged on: unified donors skip both `maxTransferOut` checks that split donors already honour.

- **Scan-time capacity** (`allocator.go:684-685`): unified branch uses raw `bal.futures`, ignoring `bal.maxTransferOut`.
- **Pre-withdraw re-cap** (`allocator.go:1610`): unified re-fetches balance but compares only `Available`, not `MaxTransferOut`.

Split donors already have matching checks at `allocator.go:687-697` (scan) and `allocator.go:1557-1560` (fresh re-cap before `TransferToSpot`). This patch extends the same invariant to unified donors.

## Changes (2 locations, single file)

### 1. `internal/engine/allocator.go:684-685` — scan-time capacity

Before:
```go
if isUnified {
    futuresAvail = bal.futures
} else {
    // split: uses maxTransferOut ...
}
```

After:
```go
if isUnified {
    futuresAvail = bal.futures
    if bal.maxTransferOut > 0 && bal.maxTransferOut < futuresAvail {
        futuresAvail = bal.maxTransferOut
    }
} else {
    // split: uses maxTransferOut ...
}
```

Guard `> 0`: `MaxTransferOut=0` means "unknown / API failed" per `pkg/exchange/types.go`, not "zero available". Without the guard, an adapter probe failure would incorrectly reject all unified withdrawals.

### 2. `internal/engine/allocator.go:1610` — pre-withdraw re-cap for unified donors

Before:
```go
if donorSpotBal.Available < grossRequired {
    netAmount = donorSpotBal.Available - fee
    ...
}
```

After:
```go
// For unified donors, also respect MaxTransferOut (the exchange's
// authoritative withdrawable cap, which reflects risk-delay freezes
// and position collateral not captured in Available).
effectiveAvail := donorSpotBal.Available
if donorSpotBal.MaxTransferOut > 0 && donorSpotBal.MaxTransferOut < effectiveAvail {
    effectiveAvail = donorSpotBal.MaxTransferOut
}
if effectiveAvail < grossRequired {
    netAmount = effectiveAvail - fee
    e.log.Debug("rebalance: adjusted netAmount %s: avail=%.2f (effective, maxTransferOut=%.2f) gross=%.2f net=%.2f",
        bestDonor, effectiveAvail, donorSpotBal.MaxTransferOut, grossRequired, netAmount)
}
```

This mirrors the split donor fresh re-cap at `allocator.go:1557-1560` that runs before `TransferToSpot`. For unified donors the fresh re-cap naturally goes in the pre-withdraw path since they skip the futures→spot prep.

## Why these two and not more

Codex identified 4 other secondary locations (`allocator.go:625`, `:1037`, `engine.go:744`, `:822`). All are planning/dry-run paths whose outputs flow through `allocatorDonorGrossCapacity()` at `:684-685` anyway; fixing the primary there closes the main over-commit path. The pre-withdraw re-cap at `:1610` adds robustness against mid-cycle drops (same pattern split donors already have). Wider cleanup is out of scope — would be "nice to have" consistency, not fixing today's production incident.

## Impact

- **Bybit** (UTA-only): fixed. Actual production incident.
- **OKX unified** (acctLv 3/4): fixed. Currently latent (prod is acctLv=2 so `IsUnified()=false`).
- **Binance Portfolio Margin**: fixed. Currently latent (PM not enabled).
- **Gate.io unified**: no-op. Adapter does not populate `MaxTransferOut`, so `> 0` guard bypasses the clamp.
- **Split accounts**: untouched.

## Risk

- **`MaxTransferOut=0` semantic ambiguity** (known limitation, accepted): Bybit adapter cannot distinguish "API failed / unknown" from "Bybit authoritatively returned 0 (UTA fully locked)". Both leave `maxTransferOut=0`. The `> 0` guard skips clamping in both cases — so an authoritative zero still falls through to raw futures and could theoretically trigger 131001. Mitigation in practice: an authoritative-zero scenario implies marginRatio is high (UTA 100% utilized), so `capByMarginHealth()` at `allocator.go:713-717` would usually limit donor surplus first. This edge case is deferred to a follow-up that adds an explicit validity flag (e.g. `Balance.MaxTransferOutValid bool`); not required for today's production incident where Bybit returns non-zero (33.40).
- **Manual transfer `/api/transfer`** (out of scope): `handlers.go:1825-1836` invokes `Withdraw()` directly without passing through the allocator's `effectiveAvail` clamp, so it remains vulnerable to 131001 on Bybit. Pre-existing bug, not a regression from this patch. Follow-up would extract a shared pre-withdraw helper. Production impact is low (manual transfers are rare operator actions).
- **Over-restriction**: impossible — clamp only lowers requested amount, never raises. Worst case is partial fill, which the existing `Unfunded` / `SkipReasons` tracking handles.
- **L4 interaction**: no new L4 risk. Clamping to a smaller withdrawal reduces futures balance less, keeps margin ratio further from L4.
- **Gate.io regression**: verified. `pkg/exchange/gateio/adapter.go:746-816` unified branch does not set `MaxTransferOut`; clamp is no-op there.

## Review History
- v1: Codex (gpt-5.4 high) — ALL PASS on scope / guards / split-path / Gate.io no-op.
- v2 (post-implementation adversarial, fresh thread) — 2 findings both accepted as known limitations out of scope: (1) authoritative-zero sentinel ambiguity [HIGH], (2) manual `/api/transfer` not protected [MEDIUM]. Neither is a regression from this patch; both deferred to follow-ups.
