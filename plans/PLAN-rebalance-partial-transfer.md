# Plan: Rebalance Partial Transfer — 3 bugs from 2026-04-17 14:45 UTC incident
Version: v4
Date: 2026-04-18
Status: REVIEWING

## Incident

At 14:45 UTC on v0.32.18, rotate scan selected SOONUSDT binance/bingx. bingx needed 116.82 USDT cross-exchange. 4 donors attempted, only bitget succeeded (33.84 USDT via APT). Override was dropped (kept=0). Entry scan at 14:55 re-ranked SOONUSDT as gateio/bingx (scanner's fresh top), gateio had no capital (avail=23.37), risk rejected post-trade margin 0.98 > L4 0.80. Net result: bitget's 33.84 USDT sits on bingx unused cross-scan; no position opened.

## Three Distinct Bugs

### Bug 1 — Binance split-account donor ignores authoritative maxTransferOut=0

Log: `donor binance capacity: futures=201.46 total=201.46 marginRatio=0.0548 maxTransferOut=0.00 futuresAvail=187.66 surplus=159.83 unified=false hasPos=true` → `TransferToSpot 24.7885 failed code=-5013 insufficient`.

**Root cause:** Two sites apply the same "maxTransferOut=0 → fall back to raw futures with L4-safety estimate" logic:
- Scan-time donor capacity: `allocatorDonorGrossCapacity` at allocator.go:695-706
- Executor pre-withdraw re-cap: at allocator.go:1537-1549

The shared contract at `pkg/exchange/types.go:143` documents `MaxTransferOut: 0 = unknown`. For Binance with open positions, `maxWithdrawAmount=0` is NOT "unknown" — it is the exchange authoritatively reporting "no withdrawable collateral". The fallback-to-estimate path is intended for adapters that don't populate the field at all.

**Fix direction (v2 — revised from v1):** Add an explicit `MaxTransferOutAuthoritative bool` flag to `exchange.Balance`, following the existing `MarginRatioUnavailable` precedent. Only adapters that guarantee populating the field (Binance, Bybit) set it `true`. Other adapters leave it `false` (default), preserving current "0 = unknown" contract. Both allocator sites gate the "zero means unavailable" rule on this flag, not on `hasPositions` alone.

### Bug 2 — moveAmt formatted to "0.0000" triggers bitget code=40020

Log: `bitget capping moveAmt 0.4214 to fresh maxTransferOut 0.0000` → `bitget futures->spot 0.0000 USDT` → `ERROR code=40020 Parameter amount error`.

**Root cause:** `freshBal.MaxTransferOut` can be a tiny positive (e.g. 0.00003) that passes the `> 0` guard at allocator.go:1566 and the `moveAmt <= 0` guard at L1572, but rounds to `"0.0000"` via `%.4f` at L1577. Bitget rejects zero-amount transfers.

**Codex confirmed:** other same-file formatting paths (L1257, L1340) already enforce `>= 1.0` pre-format, so this guard is needed ONLY at L1577.

### Bug 3 — Override pair dropped despite receiver leg funded

Log: `14:48:15 dropped 1/1 allocator overrides after executor outcome (kept=0)`. Entry scan at 14:55 re-ranked SOONUSDT to gateio/bingx using scanner's fresh top (ranker.go:161 Score sort), gateio rejected on margin ratio.

**Root cause (Codex-confirmed):**
- `keepFundedChoices` at allocator.go:155-165 replays choices against `PostBalances` using `canReserveAllocatorChoice` (L121-138)
- `PostBalances` is computed from the executor's internal bookkeeping, not from a fresh balance fetch after all transfers settle
- When receiver leg (bingx) got 33.84 USDT via APT + spot→futures at 14:48:15, the executor's PostBalances may under-reflect the actual futures balance, causing the `long.futures < requiredMargin` check to fail even when live balance would pass
- Override-drop path triggers even when the PAIR is actually feasible post-partial-transfer
- Once `kept=0`, entry scan falls to default scanner ranking — re-picks a different pair (gateio/bingx) that matches no funded-receiver

**Fix direction (v2 — revised):** When `keepFundedChoices` would drop a choice whose receiver leg is in `FundedReceivers`, re-fetch LIVE balance for both legs of that choice and re-run `canReserveAllocatorChoice` with live data. If it passes, retain the override. This uses the existing `applyAllocatorOverrides` → `opp.Alternatives` patch path (engine.go:1036-1098) which already handles pair-mismatched overrides correctly.

## Changes

### Change 1 — Adapter-level authoritative signal + allocator gate (Bug 1)

#### 1a. `pkg/exchange/types.go:133-144` — add flag

Before:
```go
type Balance struct {
    Total          float64
    Available      float64
    Frozen         float64
    Currency       string
    MarginRatio    float64
    MarginRatioUnavailable bool
    MaxTransferOut float64 // max amount that can be transferred out; 0 = unknown (use Available as fallback)
}
```

After:
```go
type Balance struct {
    Total          float64
    Available      float64
    Frozen         float64
    Currency       string
    MarginRatio    float64
    MarginRatioUnavailable bool
    MaxTransferOut float64 // max amount that can be transferred out; 0 = unknown unless MaxTransferOutAuthoritative=true
    // MaxTransferOutAuthoritative: when true, MaxTransferOut=0 means the
    // exchange has explicitly reported no withdrawable collateral (e.g. all
    // equity reserved as position margin), not "field missing". Only adapters
    // that always populate the underlying exchange field should set this.
    MaxTransferOutAuthoritative bool
}
```

#### 1b. `pkg/exchange/binance/adapter.go:734-744` — populate flag

Binance's `/fapi/v3/balance` always returns `maxWithdrawAmount`. Add `MaxTransferOutAuthoritative: true` to the returned Balance.

Verify: the field is present in the asset struct at L706-708. If `strconv.ParseFloat(asset.MaxWithdrawAmount, 64)` succeeds, set flag true regardless of value (including zero).

#### 1c. `pkg/exchange/bybit/adapter.go` — populate flag

Bybit already fetches `/v5/account/withdrawal` in `GetFuturesBalance`; set `MaxTransferOutAuthoritative: true` ONLY when that endpoint succeeds. If the endpoint errors/is unavailable, leave flag false (current v0.32.10 behavior).

Location: the section that assigns `MaxTransferOut` from availableWithdrawal. Exact line verified at implementation time (likely ~L821-841 per Codex v1 review).

#### 1d. `internal/engine/allocator.go` — propagate flag into `rebalanceBalanceInfo`

Add field:
```go
type rebalanceBalanceInfo struct {
    ...existing fields...
    maxTransferOutAuthoritative bool
}
```

Populate at the snapshot site `internal/engine/engine.go:609-614` (add one line next to the other futBal copies):
```go
if futBal, err := exch.GetFuturesBalance(); err == nil {
    bi.futures = futBal.Available
    bi.futuresTotal = futBal.Total
    bi.marginRatio = futBal.MarginRatio
    bi.maxTransferOut = futBal.MaxTransferOut
    bi.maxTransferOutAuthoritative = futBal.MaxTransferOutAuthoritative  // v4
}
```

The helper `fetchLiveRebalanceBalance` (Change 3e) already copies this field.

#### 1e. `internal/engine/allocator.go:695-706` — gate scan-time fallback

Before:
```go
} else {
    futuresAvail = bal.maxTransferOut
    if futuresAvail <= 0 {
        futuresAvail = bal.futures
        if bal.marginRatio > 0 && bal.futuresTotal > 0 && e.cfg.MarginL4Threshold > 0 {
            safeMax := bal.futuresTotal * (1.0 - bal.marginRatio/e.cfg.MarginL4Threshold)
            if safeMax < futuresAvail { futuresAvail = safeMax }
        }
    }
}
```

After:
```go
} else {
    futuresAvail = bal.maxTransferOut
    if futuresAvail <= 0 {
        if bal.maxTransferOutAuthoritative {
            // Exchange authoritatively reports zero withdrawable. Don't
            // override with an estimate; skip this donor.
            e.log.Debug("allocator: donorGross %s split-account authoritative maxTransferOut=0, treating as unavailable", name)
            return 0
        }
        // Field not populated — fall back to raw futures with L4-safety estimate.
        futuresAvail = bal.futures
        if bal.marginRatio > 0 && bal.futuresTotal > 0 && e.cfg.MarginL4Threshold > 0 {
            safeMax := bal.futuresTotal * (1.0 - bal.marginRatio/e.cfg.MarginL4Threshold)
            if safeMax < futuresAvail { futuresAvail = safeMax }
        }
    }
}
```

#### 1f. `internal/engine/allocator.go:1537-1549` — gate executor pre-withdraw fallback

Before:
```go
maxMove := donorBal.maxTransferOut
if maxMove <= 0 {
    maxMove = donorBal.futures - needs[bestDonor]
    if donorBal.marginRatio > 0 && donorBal.futuresTotal > 0 && e.cfg.MarginL4Threshold > 0 {
        safeMax := donorBal.futuresTotal * (1.0 - donorBal.marginRatio/e.cfg.MarginL4Threshold)
        safeMax -= needs[bestDonor]
        if safeMax < maxMove { maxMove = safeMax }
    }
} else {
    maxMove -= needs[bestDonor]
}
```

After:
```go
maxMove := donorBal.maxTransferOut
if maxMove <= 0 {
    if donorBal.maxTransferOutAuthoritative {
        e.log.Info("rebalance: %s donor skipped — authoritative maxTransferOut=0", bestDonor)
        surplus[bestDonor] = 0
        continue
    }
    maxMove = donorBal.futures - needs[bestDonor]
    if donorBal.marginRatio > 0 && donorBal.futuresTotal > 0 && e.cfg.MarginL4Threshold > 0 {
        safeMax := donorBal.futuresTotal * (1.0 - donorBal.marginRatio/e.cfg.MarginL4Threshold)
        safeMax -= needs[bestDonor]
        if safeMax < maxMove { maxMove = safeMax }
    }
} else {
    maxMove -= needs[bestDonor]
}
```

#### 1g. `internal/engine/allocator.go:1014-1020` — gate dryRunTransferPlan fallback (added v3)

Third split-account fallback site used by allocator feasibility/greedy selection (flows from L57-64, L582-600). Identified by Codex v2 review.

Before:
```go
maxMove := donorBal.maxTransferOut
if maxMove <= 0 {
    maxMove = donorBal.futures
}
maxMove -= needs[bestDonor]
if donorBal.maxTransferOut > 0 && maxMove > donorBal.maxTransferOut {
    maxMove = donorBal.maxTransferOut
}
```

After:
```go
maxMove := donorBal.maxTransferOut
if maxMove <= 0 {
    if donorBal.maxTransferOutAuthoritative {
        // Exchange authoritatively reports zero transferable collateral.
        // Do not synthesize a donor budget from raw futures.
        triedDonors[bestDonor] = true
        continue
    }
    maxMove = donorBal.futures
}
maxMove -= needs[bestDonor]
if donorBal.maxTransferOut > 0 && maxMove > donorBal.maxTransferOut {
    maxMove = donorBal.maxTransferOut
}
```

### Change 2 — Post-format zero guard (Bug 2)

`internal/engine/allocator.go:1577-1583`

Before:
```go
moveStr := fmt.Sprintf("%.4f", moveAmt)
e.log.Info("rebalance: %s futures->spot %s USDT", bestDonor, moveStr)
if err := e.exchanges[bestDonor].TransferToSpot("USDT", moveStr); err != nil {
    e.log.Error("rebalance: %s futures->spot failed: %v", bestDonor, err)
    surplus[bestDonor] = 0
    continue
}
```

After:
```go
moveStr := fmt.Sprintf("%.4f", moveAmt)
// Post-format zero guard: moveAmt can be a tiny positive (e.g. 0.00003)
// that passes float checks but rounds to "0.0000" via %.4f. Exchanges
// reject zero-amount transfers (bitget code=40020).
if parsed, _ := strconv.ParseFloat(moveStr, 64); parsed <= 0 {
    e.log.Warn("rebalance: %s moveAmt %.6f rounds to zero string, skipping", bestDonor, moveAmt)
    surplus[bestDonor] = 0
    continue
}
e.log.Info("rebalance: %s futures->spot %s USDT", bestDonor, moveStr)
if err := e.exchanges[bestDonor].TransferToSpot("USDT", moveStr); err != nil {
    e.log.Error("rebalance: %s futures->spot failed: %v", bestDonor, err)
    surplus[bestDonor] = 0
    continue
}
```

Note: verify `strconv` import exists in allocator.go (it does — `strconv.FormatFloat` usage elsewhere).

### Change 3 — Retain override via live-balance recheck when receiver leg funded (Bug 3)

Use the existing v0.32.9 `rebalanceExecutionResult` structure. Do NOT add parallel state on Engine struct; thread the receiver info through keepFundedChoices.

#### 3a. `internal/engine/allocator.go` — extend `rebalanceExecutionResult`

Existing struct (from v0.32.9 at top of file):
```go
type rebalanceExecutionResult struct {
    PostBalances map[string]rebalanceBalanceInfo
    Unfunded     map[string]float64
    SkipReasons  map[string]string
}
```

Add one field:
```go
type rebalanceExecutionResult struct {
    PostBalances    map[string]rebalanceBalanceInfo
    Unfunded        map[string]float64
    SkipReasons     map[string]string
    // FundedReceivers: exchanges that received a cross-exchange deposit AND
    // successfully moved it to futures this cycle. Used as a trigger for the
    // live-balance recheck in keepFundedChoices — not a bias for entry scan.
    FundedReceivers map[string]float64
}
```

#### 3b. `internal/engine/allocator.go` — populate `FundedReceivers`

After the receiver `TransferToFutures` success block at allocator.go:1845-1890, record the credited amount using the existing `totalPending` variable (v4 correction — `creditedAmount` did not exist):
```go
if result.FundedReceivers == nil {
    result.FundedReceivers = make(map[string]float64)
}
result.FundedReceivers[recipient] += totalPending
```

#### 3c. `internal/engine/allocator.go:155-165` — keepFundedChoices live-recheck (v3 revision fixes reservation bug)

Before:
```go
func (e *Engine) keepFundedChoices(choices []allocatorChoice, post map[string]rebalanceBalanceInfo) map[string]allocatorChoice {
    work := cloneRebalanceBalances(post)
    kept := make(map[string]allocatorChoice, len(choices))
    for _, c := range choices {
        if !e.canReserveAllocatorChoice(work, c) {
            continue
        }
        e.reserveAllocatorChoice(work, c)
        kept[c.symbol] = c
    }
    return kept
}
```

After (signature change + correct reservation against liveWork, then promote liveWork to work):
```go
func (e *Engine) keepFundedChoices(choices []allocatorChoice, post map[string]rebalanceBalanceInfo, funded map[string]float64) map[string]allocatorChoice {
    work := cloneRebalanceBalances(post)
    kept := make(map[string]allocatorChoice, len(choices))

    for _, c := range choices {
        if e.canReserveAllocatorChoice(work, c) {
            e.reserveAllocatorChoice(work, c)
            kept[c.symbol] = c
            continue
        }

        _, longFunded := funded[c.longExchange]
        _, shortFunded := funded[c.shortExchange]
        if !longFunded && !shortFunded {
            continue
        }

        // Start from the CURRENT reserved working view so any prior kept choices
        // remain deducted. Refresh only the two legs involved in this choice.
        liveWork := cloneRebalanceBalances(work)

        longBal, okLong := e.fetchLiveRebalanceBalance(c.longExchange)
        if !okLong {
            continue
        }
        shortBal, okShort := e.fetchLiveRebalanceBalance(c.shortExchange)
        if !okShort {
            continue
        }

        liveWork[c.longExchange] = longBal
        liveWork[c.shortExchange] = shortBal

        if !e.canReserveAllocatorChoice(liveWork, c) {
            continue
        }

        // Reserve against LIVE refreshed balances, then promote that reserved
        // live view to be the new working state for subsequent choices.
        e.reserveAllocatorChoice(liveWork, c)
        work = liveWork
        kept[c.symbol] = c

        e.log.Info(
            "allocator: override %s retained via live-balance recheck (funded receiver: long=%v short=%v)",
            c.symbol, longFunded, shortFunded,
        )
    }

    return kept
}
```

#### 3e. New helper `fetchLiveRebalanceBalance` (v3 revision — complete implementation)

Mirrors the rebalance snapshot site at engine.go:606-618 (which already does `GetFuturesBalance + GetSpotBalance` unconditionally for all exchanges — no unified-vs-split branch). Add in `internal/engine/engine.go` near other rebalance helpers:

```go
func (e *Engine) fetchLiveRebalanceBalance(name string) (rebalanceBalanceInfo, bool) {
    exch, ok := e.exchanges[name]
    if !ok {
        return rebalanceBalanceInfo{}, false
    }

    // Mirror the rebalance snapshot site exactly:
    // - always read GetFuturesBalance()
    // - always read GetSpotBalance()
    // - no unified-vs-split special case here
    futBal, err := exch.GetFuturesBalance()
    if err != nil {
        return rebalanceBalanceInfo{}, false
    }

    spotBal, err := exch.GetSpotBalance()
    if err != nil {
        return rebalanceBalanceInfo{}, false
    }

    hasPositions := false
    active, err := e.db.GetActivePositions()
    if err != nil {
        return rebalanceBalanceInfo{}, false
    }
    for _, p := range active {
        if p.Status == models.StatusClosed {
            continue
        }
        if p.LongExchange == name || p.ShortExchange == name {
            hasPositions = true
            break
        }
    }

    return rebalanceBalanceInfo{
        futures:                     futBal.Available,
        spot:                        spotBal.Available,
        futuresTotal:                futBal.Total,
        marginRatio:                 futBal.MarginRatio,
        maxTransferOut:              futBal.MaxTransferOut,
        maxTransferOutAuthoritative: futBal.MaxTransferOutAuthoritative,
        hasPositions:                hasPositions,
    }, true
}
```

Verify `e.db.GetActivePositions()` signature at implementation; if it uses a different name (e.g. `GetOpenPositions`), adapt accordingly. Also verify `rebalanceBalanceInfo` struct field names match (tag-free Go struct literal).

#### 3d. `internal/engine/engine.go:632-646` — caller update

Update the single call site of `keepFundedChoices` to pass the new `funded` param (v4 correction — variable is `allocSel`, not `selection`):
```go
kept := e.keepFundedChoices(allocSel.choices, result.PostBalances, result.FundedReceivers)
```

If `result.FundedReceivers` is nil (nothing got funded this cycle), keepFundedChoices sees nil map and its lookups short-circuit — behavior unchanged for the "no partial success" case.

## Why

1. **Bug 1 fix respects the shared exchange contract.** The `0 = unknown` semantics of `MaxTransferOut` remain intact for adapters that don't populate the field. Only Binance and Bybit — which authoritatively return the real withdrawable — gate the new behavior. Both the scan-time site (L695) and the executor site (L1537) must be fixed; fixing only one leaves the other path still vulnerable.

2. **Bug 2 is a targeted formatting guard** at the single site where pre-format `>= 1.0` is not enforced. Zero-risk defensive code.

3. **Bug 3 preserves the override pair** rather than biasing entry scan. Since `applyAllocatorOverrides` already handles pair-mismatch via `opp.Alternatives`, retaining the override at keepFundedChoices is sufficient — entry scan's existing Alternatives-patch path then recovers the funded pairing. Live-balance recheck is gated on funded-receiver (narrow scope, doesn't change behavior for no-partial-success case).

## Risk

- **Change 1a-c (adapter flag)**: purely additive. Adapters that don't set it retain current behavior.
- **Change 1d-f (allocator gate)**: tightens split-account donor pool for Binance/Bybit when they have positions and report zero. This is the intended behavior — currently the allocator picks them and the executor fails. Net: fewer failed executor attempts, equivalent or better capital deployment.
- **Change 2**: no behavior change except on the pathological tiny-positive moveAmt case.
- **Change 3**: adds one live-balance fetch per funded-receiver pair during override replay. Pair count is typically 1-5 per rebalance cycle, receiver funding is uncommon. Extra latency: ~500ms worst case (one exchange API per leg). No concurrency change — runs in the existing synchronous rebalance path.
- **Interaction with v0.32.9 (override-guard)**: Change 3 extends the structure but does not remove the drop path — drops still fire when no funded-receiver or when live recheck fails.
- **Interaction with v0.32.11 (unified-donor-maxtransferout)**: Change 1 touches the split-account branch only; unified branch untouched.
- **Concurrency**: keepFundedChoices runs synchronously in `rebalanceFunds` (engine.go:632-646). No new locks needed. `allocOverrideMu` (engine.go:~115-119; verified at implementation) continues to guard the override map.

## Files Touched (summary)

- `pkg/exchange/types.go` — 1 new field on `Balance`
- `pkg/exchange/binance/adapter.go` — set flag (1 line)
- `pkg/exchange/bybit/adapter.go` — set flag (1 line)
- `internal/engine/allocator.go` — 5 edit blocks (1d-f, 3a-c)
- `internal/engine/engine.go` — 1 caller update (3d)

No test stub updates needed (stubs synthesize `exchange.Balance` with flag defaulting to false = preserving existing behavior).

## Review History
- v1: Codex NEEDS-REVISION — (a) Change 1 violated "0=unknown" contract and skipped L1537-1566 executor fallback; (b) Change 3's stable-sort bias couldn't recover binance/bingx when scanner re-ranked to gateio/bingx — overlooked that `applyAllocatorOverrides` already patches via `opp.Alternatives`; (c) referenced non-existent `e.allocMu`.
- v2: Codex NEEDS-REVISION — (a) Change 1 missed third fallback at `allocator.go:1014-1020` in `dryRunTransferPlan` (allocator feasibility/greedy path); (b) Change 3 live-recheck had a reservation bug (reserved on `work` then overwrote with unreserved `liveWork`, losing deduction for downstream choices), `fetchLiveRebalanceBalance` underspecified; (c) Change 2 PASSED.
- v3: Codex NEEDS-REVISION — three naming/symbol gaps: (a) Change 3b used non-existent `creditedAmount` instead of existing `totalPending`; (b) Change 3d used non-existent `selection.choices` instead of actual `allocSel.choices`; (c) Change 1d did not explicitly show the snapshot-site copy line for `maxTransferOutAuthoritative`.
- v4: REVIEWING — fixed all 3 naming gaps with exact correct identifiers.
