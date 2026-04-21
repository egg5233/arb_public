# Plan: Rebalance Deposit Race — 5 bugs from 2026-04-18 06:45 UTC SIGNUSDT incident
Version: v3
Date: 2026-04-18
Status: REVIEWING

## Incident

At 06:45 UTC on v0.32.22, allocator picked SIGNUSDT (binance/bybit). Rebalance needed to put 111.42 USDT into binance — bingx could cover the full amount alone. Instead:

1. bingx sent 111.42 USDT (arrived in 40s)
2. bitget was bumped from 0 → 10 USDT to meet its min-withdraw floor (unnecessary — deficit already covered)
3. Total target on binance: 121.42 USDT
4. 06:46:13 wait loop saw bal=111.42 ≥ 109.28 (90% threshold) → declared "all deposits confirmed" (misleading)
5. 06:46:14 `TransferToFutures("121.42")` → `code=-5013 insufficient balance` (spot only has 111.42)
6. No retry during the remaining 1m43s of rebalance lifetime (bitget's 10 actually arrived in that window)
7. Override dropped (non-unified credit path skips `CrossTransferHappened=true` on failure)
8. 06:55 entry scan: `auto-transfer 66.91 USDT spot→futures` only covers deficit; leaves 54.48 USDT idle in spot
9. Post-trade L4 ratio calc on reduced total (210 instead of 264) → 0.91 > L4 0.80 → rejected
10. Net stranded: 121 USDT arrived via rebalance but only 67 used

---

## Bug 1 — :45 TransferToFutures uses TARGET amount, not ACTUAL arrived (PASS)

### Problem
`internal/engine/allocator.go:1953-1980` hits 90% threshold then transfers 100% target.

### Change
Cap transfer to `min(totalPending, actualSpot - startBal)`. Consolidated into Bug 5's AFTER block (shared site).

---

## Bug 2 — Split-account receiver spot not simulated in replay; credit path skipped on failure (NEEDS-REVISION FIXED)

### Problem
`canReserveAllocatorChoice` at `allocator.go:140-169` rejects when `futures < requiredMargin`, projects L4 against futures-only. Split-account spot that is about to be swept (by Bug 3's fix + risk.Approve auto-transfer) is not counted. Also: non-unified receiver path at `allocator.go:1979-1999` skips PostBalances credit + `CrossTransferHappened=true` when TransferToFutures errors.

### Change

#### 2a. `internal/engine/allocator.go:140-169` — simulate split-account spot as usable futures

BEFORE:
```go
	if long.futures < c.requiredMargin || short.futures < c.requiredMargin {
		return false
	}
	margin := c.requiredMargin
	if e.cfg.MarginSafetyMultiplier > 0 {
		margin = c.requiredMargin / e.cfg.MarginSafetyMultiplier
	}
	return e.replayProjectedRatioOK(long, margin) && e.replayProjectedRatioOK(short, margin)
```

AFTER:
```go
func (e *Engine) canReserveAllocatorChoice(work map[string]rebalanceBalanceInfo, c allocatorChoice) bool {
	long, okL := work[c.longExchange]
	short, okS := work[c.shortExchange]
	if !okL || !okS {
		return false
	}
	long = e.replayUsableAllocatorBalance(c.longExchange, long)
	short = e.replayUsableAllocatorBalance(c.shortExchange, short)
	if long.futures < c.requiredMargin || short.futures < c.requiredMargin {
		return false
	}
	margin := c.requiredMargin
	if e.cfg.MarginSafetyMultiplier > 0 {
		margin = c.requiredMargin / e.cfg.MarginSafetyMultiplier
	}
	return e.replayProjectedRatioOK(long, margin) && e.replayProjectedRatioOK(short, margin)
}

func (e *Engine) replayUsableAllocatorBalance(name string, bal rebalanceBalanceInfo) rebalanceBalanceInfo {
	if uc, ok := e.exchanges[name].(interface{ IsUnified() bool }); ok && uc.IsUnified() {
		return bal
	}
	if bal.spot <= 0 {
		return bal
	}
	bal.futures += bal.spot
	bal.futuresTotal += bal.spot
	bal.spot = 0
	return bal
}
```

#### 2b. Non-unified receiver credit on TransferToFutures failure

Consolidated into Bug 5's AFTER block (same code site). When transfer fails, the Bug 5 retry loop handles it; if deadline hits with spot still there, credit spot-backed state via Bug 5's deadline-fallback branch.

### Tests
Add regression in `internal/engine/allocator_override_test.go`: "receiver funded only in spot, override retained after replay".

---

## Bug 3 — Fixed-capital auto-transfer only covers deficit, not sweep-all (NEEDS-REVISION FIXED)

### Problem
`internal/risk/manager.go:267-279`. Fixed-capital path uses `bufferedNeed`, leaves remaining spot idle. Later L4 projection at `:458-466, :518-526` uses `longBal.Total` — idle spot doesn't count → post-trade ratio artificially high → false L4 reject.

Auto-size mode already sweeps all (using `sweepTarget = 1e9`). Fixed-capital inconsistent.

### Change

BEFORE `internal/risk/manager.go:267-279`:
```go
		if needed > 0 {
			bufferedNeed := needed * m.cfg.MarginSafetyMultiplier
			m.ensureFuturesBalance(opp.LongExchange, longExch, longBal, bufferedNeed)
			m.ensureFuturesBalance(opp.ShortExchange, shortExch, shortBal, bufferedNeed)
		} else {
			const sweepTarget = 1e9
			m.ensureFuturesBalance(opp.LongExchange, longExch, longBal, sweepTarget)
			m.ensureFuturesBalance(opp.ShortExchange, shortExch, shortBal, sweepTarget)
		}
```

AFTER:
```go
		const sweepTarget = 1e9
		if needed > 0 {
			// Fixed-capital mode still needs the full split-account sweep; otherwise
			// post-trade L4 uses an artificially low futures total and can reject a
			// trade that already has idle spot available.
			m.ensureFuturesBalance(opp.LongExchange, longExch, longBal, sweepTarget)
			m.ensureFuturesBalance(opp.ShortExchange, shortExch, shortBal, sweepTarget)
		} else {
			m.ensureFuturesBalance(opp.LongExchange, longExch, longBal, sweepTarget)
			m.ensureFuturesBalance(opp.ShortExchange, shortExch, shortBal, sweepTarget)
		}
```

### Tests
Add risk test for fixed-capital approval where split-account spot must be swept to avoid a false L4 reject.

---

## Bug 4 — min-withdraw bump at :1577 overfunds when deficit already mostly scheduled (NEEDS-REVISION FIXED)

### Problem
The bump at `allocator.go:1577-1585` IS reachable (prior review was wrong that this is dead). In the incident: bingx was scheduled for 111.42 (covers 100% of deficit), then bitget contribution = 0 (already at 100% scheduled), bump pushed to 10 USDT → overfund.

### Change

BEFORE `internal/engine/allocator.go:1577-1585`:
```go
				if contribution < netFloor {
					cappedFloor := math.Min(netFloor, bestSurplus-fee)
					if cappedFloor <= 0 {
						e.log.Debug("rebalance: %s contribution %.2f below minWd net floor %.2f and no room to bump, skipping", bestDonor, contribution, netFloor)
						surplus[bestDonor] = 0
						continue
					}
					e.log.Info("rebalance: %s bumping contribution %.2f -> %.2f to meet minWd net floor %.2f", bestDonor, contribution, cappedFloor, netFloor)
					contribution = cappedFloor
				}
```

AFTER `internal/engine/allocator.go:1577-1590`:
```go
				if contribution < netFloor {
					scheduled := origDeficit - remaining
					if scheduled >= origDeficit*0.9 {
						e.log.Info("rebalance: residual %.2f on %s below %s minWd floor %.2f after %.2f/%.2f already scheduled; not overfunding",
							remaining, exchName, bestDonor, netFloor, scheduled, origDeficit)
						result.Unfunded[exchName] = remaining
						if _, ok := result.SkipReasons[exchName]; !ok {
							result.SkipReasons[exchName] = fmt.Sprintf("residual %.2f below %s minWd floor %.2f", remaining, bestDonor, netFloor)
						}
						break
					}
					cappedFloor := math.Min(netFloor, bestSurplus-fee)
					if cappedFloor <= 0 {
						surplus[bestDonor] = 0
						continue
					}
					contribution = cappedFloor
				}
```

Assumes `origDeficit` and `exchName` are already tracked in the loop scope. If not, thread them via existing variables (`remaining`, `need`, etc.).

### Tests
Add executor regression: "recipient already has ≥90% scheduled, second donor below minWd does NOT overfund".

---

## Bug 5 — TransferToFutures retry until poll deadline + merged with Bug 1 cap + Bug 2 credit (NEEDS-REVISION FIXED)

### Problem
`allocator.go:1953-1955, 1979-1981`. One-shot transfer at 100% target amount; on failure no retry despite `pollDeadline` (5min). Merges Bug 1 (cap to arrived), Bug 2 (spot credit on failure), Bug 5 (retry-to-deadline), and log fix.

### Change

BEFORE `internal/engine/allocator.go:1953-1981`:
```go
			if spotBal.Available >= startBal+totalPending*0.9 {
				arrived = true
				e.log.Info("rebalance: all deposits confirmed on %s (bal=%.2f)", recipient, spotBal.Available)
...
		transferStr := fmt.Sprintf("%.4f", totalPending)
		if err := recipientExch.TransferToFutures("USDT", transferStr); err != nil {
			e.log.Error("rebalance: %s spot->futures failed: %v", recipient, err)
```

AFTER (full block replacing the deposit-wait + transfer-call region at `:1953-1999`):
```go
			arrivedAmt := spotBal.Available - startBal
			if arrivedAmt < totalPending*0.9 {
				continue
			}
			e.log.Info("rebalance: deposit threshold met (90%%) on %s: %.2f arrived of %.2f target", recipient, arrivedAmt, totalPending)

			moveAmt := math.Min(totalPending, arrivedAmt)
			moveAmt = math.Floor(moveAmt*10000) / 10000
			if moveAmt <= 0 {
				continue
			}

			// Unified receivers (bybit UTA, gateio unified): deposit already
			// sits in the unified pool — no spot→futures API call needed.
			// Credit the PostBalances directly. Mirrors v0.32.21 Section A.
			if uc, ok := recipientExch.(interface{ IsUnified() bool }); ok && uc.IsUnified() {
				bi := balances[recipient]
				bi.futures += moveAmt
				bi.futuresTotal += moveAmt
				balances[recipient] = bi
				if result.FundedReceivers == nil {
					result.FundedReceivers = make(map[string]float64)
				}
				result.FundedReceivers[recipient] += moveAmt
				result.CrossTransferHappened = true
				arrived = true
				break
			}

			// Split-account receivers: actual spot→futures API call.
			transferStr := fmt.Sprintf("%.4f", moveAmt)
			if err := recipientExch.TransferToFutures("USDT", transferStr); err != nil {
				e.log.Warn("rebalance: %s spot->futures %s USDT failed before %s: %v", recipient, transferStr, pollDeadline.Format(time.RFC3339), err)
				continue
			}

			e.log.Info("rebalance: %s spot->futures %s USDT (rebalance deposit)", recipient, transferStr)
			e.recordTransfer(recipient+" spot", recipient, "USDT", "internal", transferStr, "0", "", "completed", "rebalance-recv")
			bi := balances[recipient]
			bi.futures += moveAmt
			bi.futuresTotal += moveAmt
			bi.spot = math.Max(0, spotBal.Available-moveAmt)
			balances[recipient] = bi
			if result.FundedReceivers == nil {
				result.FundedReceivers = make(map[string]float64)
			}
			result.FundedReceivers[recipient] += moveAmt
			result.CrossTransferHappened = true
			arrived = true
			break
```

This block runs inside the deposit poll loop. Failure at `TransferToFutures` now `continue`s (retries next iteration with fresh `GetSpotBalance`). On deadline hit, existing loop-exit path runs; Bug 2 deadline-fallback (below) credits spot-backed state.

### Deadline-fallback credit (Bug 2 integration)

AFTER the poll loop exits due to `pollDeadline` hit, before the existing "waiting..." log exits, add:

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
}
```

### Tests
Add: "partial arrival, transfer fails first, succeeds before pollDeadline".

---

## Log fix — "all deposits confirmed" → "deposit threshold met (90%)" (PASS)

Consolidated in Bug 5's AFTER block (single `e.log.Info` call updated).

---

## Files Touched

| File | Bugs |
|------|------|
| `internal/engine/allocator.go` | 1, 2 (canReserveAllocatorChoice + replayUsableAllocatorBalance), 4, 5, log |
| `internal/risk/manager.go` | 3 |
| `internal/engine/allocator_override_test.go` | 2 test |
| `internal/risk/manager_test.go` (or existing test file) | 3 test |

## Version
Bump to `v0.32.23`.

## Review History
- v1: Codex — Bugs 1, 3, log PASS; Bugs 2, 4, 5 NEEDS-REVISION. Provided concrete BEFORE/AFTER code for all NEEDS-REVISION items.
- v2: Codex NEEDS-REVISION — Bug 4 and Bug 3 paste OK. Bug 5's block removed the existing unified-recipient handling at `:1966` (UTA / gateio unified should skip TransferToFutures and credit directly). Bug 2/5 integration otherwise coherent.
- v3: REVIEWING — Added unified receiver fast-path branch inside Bug 5's AFTER block (before the split-account TransferToFutures call). Mirrors v0.32.21 Section A pattern: UTA / unified exchanges credit futures directly without the spot→futures API call.
