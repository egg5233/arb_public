# Plan: Rebalance Partial-Success — 4 bugs from post-v0.32.23 3-round audit
Version: v5
Date: 2026-04-18
Status: REVIEWING

## Context

Post-v0.32.23 (07:59 UTC), 3 rebalance/entry rounds between 08:45-10:55 UTC. Round 2 opened SOONUSDT ✓. Rounds 1 and 3 had transfers execute but no position opened. Codex audit confirmed 5 bugs; Bug E (over-solicitation) deferred as optimization. Bug D merged into Bug A (shared code site). Net scope: 4 bugs, 3 separate code changes.

## Round summary

| Round | Allocator | Transfers | Outcome |
|-------|-----------|-----------|---------|
| 1 | SOONUSDT binance/gateio | bitget→binance 42.64 ✓, bybit→gateio 85.62 ✗ timeout | Override dropped, entry tier-3, rejected |
| 2 | SOONUSDT bybit/gateio | gateio→bitget 42.79 ✓ | **Position opened** via Alternatives patch |
| 3 | Sequential SIRENUSDT/SIGNUSDT | binance/bitget→gateio ✓, bybit/okx failed, **0 to bingx** | No override stored, tier-3, rejected |

---

## Bug A (merged with Bug D) — Deadline-fallback missing Unfunded/SkipReasons + unified uses wrong API

### Problem
`internal/engine/allocator.go:2024-2042` (v0.32.23's Bug 5 deadline-fallback):
- Does not write `result.Unfunded` or `result.SkipReasons` on timeout → `complete (unfunded=0)` logged even when nothing arrived (Round 1 Gate.io case).
- Calls `GetSpotBalance()` for late-deposit detection, but unified recipients' spot is always 0 (`pkg/exchange/gateio/adapter.go:862-867`) → cannot recover late unified deposits.

### Change

BEFORE `internal/engine/allocator.go:2024-2042`:
```go
		if !arrived {
			// Deadline hit — check if spot actually received funds. If so, credit
			// spot-backed receiver state so keepFundedChoices can retain override
			// (replay will simulate spot→futures via Bug 2's replayUsableAllocatorBalance).
			if liveSpot, err := recipientExch.GetSpotBalance(); err == nil {
				arrivedAmt := liveSpot.Available - startBal
				if arrivedAmt > 1.0 {
					bi := balances[recipient]
					bi.spot += arrivedAmt
					balances[recipient] = bi
					if result.FundedReceivers == nil {
						result.FundedReceivers = make(map[string]float64)
					}
					result.FundedReceivers[recipient] += arrivedAmt
					result.CrossTransferHappened = true
					e.log.Info("rebalance: deadline hit on %s, credited %.2f spot-backed for override retention", recipient, arrivedAmt)
				}
			}
			e.log.Warn("rebalance: deposits on %s not confirmed within 5min, skipping spot->futures", recipient)
		}
```

AFTER (Codex-supplied, merges Bug A + Bug D):
```go
		if !arrived {
			creditedAmt := 0.0

			// Deadline hit: mirror the same balance source used by the poll loop,
			// then preserve any late-arriving funds for override replay.
			var liveBal *exchange.Balance
			var balErr error
			if uc, ok := recipientExch.(interface{ IsUnified() bool }); ok && uc.IsUnified() {
				liveBal, balErr = recipientExch.GetFuturesBalance()
			} else {
				liveBal, balErr = recipientExch.GetSpotBalance()
			}
			if balErr == nil {
				arrivedAmt := liveBal.Available - startBal
				if arrivedAmt > 1.0 {
					creditedAmt = math.Min(totalPending, arrivedAmt)

					bi := balances[recipient]
					if uc, ok := recipientExch.(interface{ IsUnified() bool }); ok && uc.IsUnified() {
						bi.futures += creditedAmt
						bi.futuresTotal += creditedAmt
					} else {
						bi.spot += creditedAmt
					}
					balances[recipient] = bi

					if result.FundedReceivers == nil {
						result.FundedReceivers = make(map[string]float64)
					}
					result.FundedReceivers[recipient] += creditedAmt
					result.CrossTransferHappened = true
					e.log.Info("rebalance: deadline hit on %s, credited %.2f late-arriving balance for override retention", recipient, creditedAmt)
				}
			}

			shortfall := totalPending - creditedAmt
			if shortfall > 0.0001 {
				result.Unfunded[recipient] = shortfall
				if creditedAmt > 0 {
					result.SkipReasons[recipient] = fmt.Sprintf("deposit timeout after partial arrival %.2f/%.2f", creditedAmt, totalPending)
				} else {
					result.SkipReasons[recipient] = "deposit timeout"
				}
			}

			e.log.Warn("rebalance: deposits on %s not confirmed within 5min, skipping spot->futures", recipient)
		}
```

### Notes
- Bug D resolved by the unified/split branch in the new block (unified uses `GetFuturesBalance`).
- `result.Unfunded` must be initialized as non-nil map when `rebalanceExecutionResult` is constructed (verify — likely already done).
- Does NOT change behavior when full deposit arrived within deadline (that path didn't call the fallback block).

### A2 — startBal baseline fallback (v4 addition per Codex re-review)

Additional site at `internal/engine/allocator.go:~1937`. When unified `GetFuturesBalance()` fails, current code falls back to `bi.spot` which is always 0 for unified → later poll math over-credits late deposits.

BEFORE:
```go
if uc, ok := e.exchanges[bw.recipient].(interface{ IsUnified() bool }); ok && uc.IsUnified() {
    if fb, err := e.exchanges[bw.recipient].GetFuturesBalance(); err == nil {
        pendingStartBal[bw.recipient] = fb.Available
    } else {
        e.log.Warn("rebalance: %s unified GetFuturesBalance for deposit baseline failed: %v, using spot fallback", bw.recipient, err)
        pendingStartBal[bw.recipient] = balances[bw.recipient].spot
    }
} else {
    pendingStartBal[bw.recipient] = balances[bw.recipient].spot
}
```

AFTER:
```go
if uc, ok := e.exchanges[bw.recipient].(interface{ IsUnified() bool }); ok && uc.IsUnified() {
    if fb, err := e.exchanges[bw.recipient].GetFuturesBalance(); err == nil {
        pendingStartBal[bw.recipient] = fb.Available
    } else {
        e.log.Warn("rebalance: %s unified GetFuturesBalance for deposit baseline failed: %v, using futures snapshot fallback", bw.recipient, err)
        pendingStartBal[bw.recipient] = balances[bw.recipient].futures
    }
} else {
    pendingStartBal[bw.recipient] = balances[bw.recipient].spot
}
```

### Test
Add regression: "receiver deposit times out, expect Unfunded[recipient] == totalPending, SkipReasons[recipient] == 'deposit timeout'".

---

## Bug B — Sequential rescue planner should dry-run and prune infeasible targets

### Problem
Sequential planner (`engine.go:~695-790`) builds rescue targets + `plannedTransfers` but executor (`allocator.go:1460-1511`) ignores `plannedTransfers` and greedy-allocates donors. Round 3: gateio consumed all donors first; bingx got zero despite being in rescue plan.

### Change (Codex-supplied concrete code)

#### B1 — Replace locals at `engine.go:695-701` to track ALL selections (v4: extended scope per Codex re-review)

BEFORE:
```go
needs := map[string]float64{}
selectedSymbols := make(map[string]bool)     // prevent same-batch duplicates
reserved := map[string]float64{}             // cumulative margin reserved per exchange
plannedTransfers := map[string]float64{}     // cross-exchange transfer plan: recipient → amount
selected := 0
```

AFTER (track all selected opps, not just rescue):
```go
needs := map[string]float64{}
selectedSymbols := make(map[string]bool)
reserved := map[string]float64{}
plannedTransfers := map[string]float64{}

type sequentialChoice struct {
    choice    allocatorChoice
    longNeed  float64
    shortNeed float64
}
selectedChoices := make([]sequentialChoice, 0, remainingSlots)

buildChoices := func(src []sequentialChoice) []allocatorChoice {
    out := make([]allocatorChoice, 0, len(src))
    for _, sc := range src {
        out = append(out, sc.choice)
    }
    return out
}

selected := 0
```

#### B2 — Append rescue selections to selectedChoices at `engine.go:~784-790`:
```go
selectedChoices = append(selectedChoices, sequentialChoice{
    choice: allocatorChoice{
        symbol:         opp.Symbol,
        longExchange:   opp.LongExchange,
        shortExchange:  opp.ShortExchange,
        requiredMargin: estMargin,
    },
    longNeed:  estMargin,
    shortNeed: estMargin,
})
needs[opp.LongExchange] += estMargin
needs[opp.ShortExchange] += estMargin
reserved[opp.LongExchange] += estMargin
reserved[opp.ShortExchange] += estMargin
selectedSymbols[opp.Symbol] = true
selected++
e.log.Info("rebalance: selected %s via cross-exchange rescue (margin=%.2f per leg)", opp.Symbol, estMargin)
continue
```

#### B3 — Append normal selections to selectedChoices at `engine.go:~868-874`:
```go
selectedChoices = append(selectedChoices, sequentialChoice{
    choice: allocatorChoice{
        symbol:         opp.Symbol,
        longExchange:   opp.LongExchange,
        shortExchange:  opp.ShortExchange,
        requiredMargin: approval.RequiredMargin,
    },
    longNeed:  longMarginNeeded,
    shortNeed: shortMarginNeeded,
})
needs[opp.LongExchange] += longMarginNeeded
needs[opp.ShortExchange] += shortMarginNeeded
reserved[opp.LongExchange] += longMarginNeeded
reserved[opp.ShortExchange] += shortMarginNeeded
selectedSymbols[opp.Symbol] = true
selected++
e.log.Info("rebalance: selected %s (longMargin=%.2f shortMargin=%.2f)", opp.Symbol, longMarginNeeded, shortMarginNeeded)
```

#### B4 — Dry-run prune over full selected set (v4: covers rescue AND normal):

Insert before the "analyzed N opportunities" log (`engine.go:~877`):
```go
if len(selectedChoices) > 0 {
    feeCache := map[string]feeEntry{}
    for len(selectedChoices) > 0 && !e.dryRunTransferPlan(buildChoices(selectedChoices), balances, feeCache).Feasible {
        dropped := selectedChoices[len(selectedChoices)-1]
        selectedChoices = selectedChoices[:len(selectedChoices)-1]
        needs[dropped.choice.longExchange] -= dropped.longNeed
        if needs[dropped.choice.longExchange] <= 0 {
            delete(needs, dropped.choice.longExchange)
        }
        needs[dropped.choice.shortExchange] -= dropped.shortNeed
        if needs[dropped.choice.shortExchange] <= 0 {
            delete(needs, dropped.choice.shortExchange)
        }
        reserved[dropped.choice.longExchange] -= dropped.longNeed
        if reserved[dropped.choice.longExchange] < 0 {
            reserved[dropped.choice.longExchange] = 0
        }
        reserved[dropped.choice.shortExchange] -= dropped.shortNeed
        if reserved[dropped.choice.shortExchange] < 0 {
            reserved[dropped.choice.shortExchange] = 0
        }
        delete(selectedSymbols, dropped.choice.symbol)
        if selected > 0 {
            selected--
        }
        e.log.Info("rebalance: prune %s %s/%s - dry-run infeasible",
            dropped.choice.symbol, dropped.choice.longExchange, dropped.choice.shortExchange)
    }
}

e.log.Info("rebalance: analyzed %d opportunities, selected %d, needs: %v",
    len(opps), selected, needs)
```

Note: `dryRunTransferPlan` signature is `(choices []allocatorChoice, balances map[string]rebalanceBalanceInfo, feeCache map[string]feeEntry) dryRunResult` per allocator.go:75-80, 841. `dryRunResult.Feasible` is the primary feasibility flag.

### Test
Add regression: "rescue plan with second leg unfundable, prune drops infeasible rescue target".

---

## Bug C — Sequential fallback should store overrides (Codex-supplied concrete code)

### Problem
`engine.go:1020-1023` comment says sequential fallback discards executor result. Partial rescue success cannot steer entry — always goes tier-3.

### Change

BEFORE `internal/engine/engine.go:1020-1023`:
```go
e.log.Info("rebalance: %d exchanges need cross-exchange funding, delegating to allocator executor", len(precomputed))
// Sequential fallback does not use allocOverrides; signature aligned but
// result not consumed.
_ = e.executeRebalanceFundingPlan(needs, balances, precomputed)
```

AFTER (v5: adds localTransferHappened tracking for sequential local spot→futures + handles `crossDeficits==0` early return):

#### C1 — Add `localTransferHappened` tracking variable

Declare in sequential flow scope (near `selectedChoices` at `engine.go:~695`):
```go
localTransferHappened := false
```

Set on each successful same-exchange spot→futures call at `engine.go:~958` (the local relief site). Find the `TransferToFutures` calls and set `localTransferHappened = true` on each success branch.

#### C2 — Handle early return when `crossDeficits==0`

Around `engine.go:~1011` (early return before executor delegation):

BEFORE:
```go
if len(crossDeficits) == 0 {
    e.log.Info("rebalance: all exchanges funded, no cross-exchange transfers needed")
    return
}
```

AFTER:
```go
if len(crossDeficits) == 0 {
    if localTransferHappened {
        kept := e.keepFundedChoices(buildChoices(selectedChoices), balances, nil)
        e.allocOverrideMu.Lock()
        e.allocOverrides = kept
        e.allocOverrideMu.Unlock()
        e.log.Info("rebalance: stored %d sequential allocator overrides (local-only) for entry scan", len(kept))
    }
    e.log.Info("rebalance: all exchanges funded, no cross-exchange transfers needed")
    return
}
```

#### C3 — Cross-exchange path at `engine.go:~1020-1023`

BEFORE:
```go
e.log.Info("rebalance: %d exchanges need cross-exchange funding, delegating to allocator executor", len(precomputed))
// Sequential fallback does not use allocOverrides; signature aligned but
// result not consumed.
_ = e.executeRebalanceFundingPlan(needs, balances, precomputed)
```

AFTER:
```go
e.log.Info("rebalance: %d exchanges need cross-exchange funding, delegating to allocator executor", len(precomputed))
result := e.executeRebalanceFundingPlan(needs, balances, precomputed)

if !(localTransferHappened || result.LocalTransferHappened || result.CrossTransferHappened) {
    e.allocOverrideMu.Lock()
    e.allocOverrides = nil
    e.allocOverrideMu.Unlock()
    e.log.Info("rebalance: no transfers executed, skipping sequential override store")
} else {
    kept := e.keepFundedChoices(buildChoices(selectedChoices), result.PostBalances, result.FundedReceivers)

    e.allocOverrideMu.Lock()
    e.allocOverrides = kept
    e.allocOverrideMu.Unlock()

    e.log.Info("rebalance: stored %d sequential allocator overrides for entry scan", len(kept))
}

e.log.Info("rebalance: complete")
```

### Mapping
`selectedChoices` (from Bug B v4) is `[]sequentialChoice`; `buildChoices` returns `[]allocatorChoice`. `keepFundedChoices` (`allocator.go:186`) takes that directly. Only the 4 fields `symbol, longExchange, shortExchange, requiredMargin` are used by replay — matches the Bug B construction.

### Scope note
v4 change covers BOTH rescue and normal selected opps. Previously v3 only tracked rescue, leaving normal-path overrides unstored — Codex re-review caught this.

### Test
Add regression: "sequential fallback with partial rescue success, expect override stored for kept subset, entry uses tier-2".

---

## Files Touched

| File | Bugs |
|------|------|
| `internal/engine/allocator.go` | A+D (deadline fallback replacement) |
| `internal/engine/engine.go` | B (planner validation), C (override store) |

## Version
Bump to `v0.32.24`.

## Review History
- v1: Codex — Bug A/D NEEDS-REVISION (provided full AFTER code); Bug B, C PASS with fix-direction guidance.
- v2: Codex — Bug A/D NEEDS-REVISION code pasted; Bug B, C PASS at direction but no concrete code yet.
- v3: Codex NEEDS-REVISION — two gaps: (1) Bug A+D missed unified baseline fallback at allocator.go:1937 (fallback to .spot always 0 for unified); (2) Bug B/C scope only covered rescue, not normal selected opps.
- v4: Codex NEEDS-REVISION — Bug C missed two cases: (1) sequential local spot→futures at engine.go:958 happens BEFORE executeRebalanceFundingPlan, its success not reflected in result.LocalTransferHappened; (2) `crossDeficits==0` early return at engine.go:1011 bypasses override storage entirely.
- v5: REVIEWING — Bug C adds `localTransferHappened` tracking variable; handles both `crossDeficits==0` early return (stores overrides using baseline balances map) and cross-exchange path (combines local+result flags in gate check).
