# Plan: Transfer Bug Fix — Deficit Formula + Min Withdraw + Donor Capacity Unification
Version: v16
Date: 2026-04-07
Status: IMPLEMENTING

## Changes

### Bug A — Unify deficit formula (3 sites)
- `internal/engine/allocator.go:832-840` (estimateAllocatorTransferCost): replace `shortfall + l4Extra`
- `internal/engine/allocator.go:1327-1335` (executeRebalanceFundingPlan): replace `shortfall + l4Extra`
- `internal/engine/engine.go:781-789` (sequential fallback): replace `shortfall + l4Extra`

### Bug B — Min withdraw + donor capacity (multiple files)

#### FIX 1: GetWithdrawFee interface change
- `pkg/exchange/exchange.go:47`: `(float64, error)` → `(float64, float64, error)`
- 6 adapters: add minWithdraw field to response structs
  - `pkg/exchange/binance/adapter.go:898`: WithdrawMin
  - `pkg/exchange/bingx/adapter.go:805`: WithdrawMin
  - `pkg/exchange/bitget/adapter.go:938`: MinWithdrawAmount
  - `pkg/exchange/okx/adapter.go`: MinWd
  - `pkg/exchange/bybit/adapter.go:948`: WithdrawMin
  - `pkg/exchange/gateio/adapter.go:1037`: WithdrawAmountMini
- 4 test stubs return `(0, 0, nil)`:
  - `internal/discovery/test_helpers_test.go:67`
  - `internal/spotengine/execution_test.go:100`
  - `internal/spotengine/exit_triggers_test.go:77`
  - `internal/api/funding_history_test.go:37`
- 4 direct callers: allocator.go L625, L750, L937, L1448

#### FIX 2: feeCache → feeEntry struct
```go
type feeEntry struct {
    fee   float64
    minWd float64
    valid bool // replaces -1 sentinel
}
```
Locations: L393, L483, L692, L715, L747, L751-755, L757, L781, L807, L866-870, L903, L934, L939, L942, L944

#### FIX 3: Fee-mode aware donor filter
- L625 (findAllocatorDonor), L937 (findAllocatorDonorWithCache), L715 (cheapestTransferFee):
  - fee-inclusive: skip if `avail < minWd`
  - net-fee: skip if `avail < minWd + fee`

#### FIX 4: isAllocatorFundingFeasible minWd floor
- allocator.go L557: deficit < 10.0 → floor to 10.0 (conservative hardcode)

#### FIX 5: simulateTransferPlan minWd floor
- allocator.go L1062: move < 10.0 → floor to 10.0; bestSurplus < 10.0 → return infeasible

#### FIX 6a: Fee-mode cap — keep guard, add fee-inclusive path
- allocator.go L1458-1468: KEEP the `!WithdrawFeeInclusive` guard for net-fee path (existing).
- ADD fee-inclusive path: for fee-inclusive exchanges, the fee is embedded in contribution, but total debit is still contribution (gross). Cap contribution to bestSurplus:
  ```go
  isGross := e.exchanges[bestDonor].WithdrawFeeInclusive()
  if !isGross && balances[bestDonor].hasPositions {
      // net-fee: actual debit = contribution + fee (existing logic)
      if contribution+fee > bestSurplus {
          contribution = bestSurplus - fee
      }
  } else if isGross && balances[bestDonor].hasPositions {
      // fee-inclusive: actual debit = contribution (fee embedded)
      if contribution > bestSurplus {
          contribution = bestSurplus
      }
  }
  if contribution <= 0 { surplus[bestDonor] = 0; continue }
  ```

#### FIX 7: Unify donor capacity
- `allocatorDonorGrossCapacity` (L966) rewrite to:
  ```go
  func (e *Engine) allocatorDonorGrossCapacity(name string, bal rebalanceBalanceInfo, localNeed float64) float64 {
      // futures可用: for unified accounts, rebalanceAvailable returns futures only (no double-count)
      // for split accounts, use min(maxTransferOut, futures) as transferable futures
      var futuresAvail float64
      isUnified := false
      if uc, ok := e.exchanges[name].(interface{ IsUnified() bool }); ok && uc.IsUnified() {
          isUnified = true
      }
      if isUnified {
          futuresAvail = bal.futures
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

      // surplus = futures可用 + spot可用 - 自己需要的
      // unified: spot is part of futures pool, don't add separately
      var surplus float64
      if isUnified {
          surplus = futuresAvail - localNeed
      } else {
          surplus = futuresAvail + bal.spot - localNeed
      }
      if surplus <= 0 { return 0 }

      // cap by 健康上限
      if bal.hasPositions {
          healthCap := e.capByMarginHealth(bal)
          if healthCap < surplus { surplus = healthCap }
      }
      if surplus < 0 { return 0 }
      return surplus
  }
  ```
- Delete: `allocatorWithdrawalUsesMainBalance` function (L1024-1033) — no longer needed, unified check is inline
- Keep: `maxTransferOut` field in `rebalanceBalanceInfo` struct — still used by executor at L1489-1538
- executor surplus map (L1374-1384) change to:
  ```go
  surplus := map[string]float64{}
  for name := range e.exchanges {
      surplus[name] = e.allocatorDonorGrossCapacity(name, balances[name], needs[name])
  }
  ```

#### FIX 8: minWd early bump + final guard
- Early bump (after L1468, before L1470):
  ```go
  if minWd > 0 {
      netFloor := minWd - fee
      if netFloor < 0 { netFloor = 0 }
      if contribution < netFloor {
          cappedFloor := math.Min(netFloor, bestSurplus-fee)
          if cappedFloor <= 0 { surplus[bestDonor] = 0; continue }
          contribution = cappedFloor
      }
  }
  ```
- Final guard (after L1578, before L1581):
  ```go
  if minWd > 0 && withdrawAmtForAPI < minWd {
      surplus[bestDonor] = 0; continue
  }
  ```

#### FIX 9: L1392 removal
- Delete `remaining < 1.0` skip. Intentional over-transfer when deficit < minWd.
- Keep L1341 (spot→futures internal `< 1.0`)

### New test
- `internal/engine/allocator_deficit_test.go`: table-driven test for max(marginDeficit, ratioDeficit)

## Why

**Bug A:** `simulateTransferPlan` uses `max(marginDeficit, ratioDeficit)` = 21.42, but executor uses `shortfall + l4Extra` = 11.50. The `l4Extra` clamp to `[0, shortfall]` artificially limits the transfer amount. This is a deliberate behavior fix.

**Bug B:** Hardcoded `< 1.0 USDT` threshold at L1392. Binance min is 10 USDT, Bybit 10, BingX 5-10, Bitget 5, OKX 0.1, Gate.io ~0.01. Sub-minimum transfers fail at exchange API every time. User wants: if deficit < min, still transfer at minimum amount.

**FIX 7:** solver (`allocatorDonorGrossCapacity`) and executor (surplus map) used different donor capacity formulas. Now unified: both use `min(futuresAvail + spot - localNeed, healthCap)` where `futuresAvail = maxTransferOut` for split accounts (respects actual transferable amount), `bal.futures` for unified accounts.

## How

See pseudo code in each FIX section above.

## Risk

- Interface signature change is atomic — all 6 adapters + 4 stubs must update together
- Bug A is deliberate behavior change (larger transfers than before)
- Gate.io `withdraw_amount_mini` needs runtime verification
- Over-transfer to minWd consumes extra donor surplus (recipient gets buffer — safe)
- FIX 7 unifies solver/executor capacity using maxTransferOut for split accounts — keeps existing accuracy while eliminating mismatch
- FIX 6a keeps !WithdrawFeeInclusive guard — fee-inclusive exchanges have fee embedded in contribution
- maxTransferOut field kept in struct — executor L1489-1538 still needs it for futures→spot internal move

## Graphify

Graph: 1906 nodes (1784 code + 122 doc), 2945 edges, 86 communities.
God nodes: Adapter (67 edges), Engine (56), Client (38), Server (29), Scanner (28).
Queried: allocator/transfer/deficit/withdraw/feeinclusive/donor/surplus — confirmed all relevant nodes in internal/engine/allocator.go.
WithdrawFeeInclusive: 4 test stubs + OKX adapter.

## Review History
- v1-v5: Codex (gpt-5.4) — NEEDS-REVISION — formula mismatch, wrong bump location, missing feeCache locations
- v6: Claude code-reviewer — NEEDS-REVISION — formula divergence for split accounts, feeEntry incomplete
- v7: Claude code-reviewer — NEEDS-REVISION — feeEntry type, donor filter ambiguity
- v8: Claude code-reviewer — NEEDS-REVISION — formula not equivalent (deliberate), feeCache missing sigs
- v9: Claude code-reviewer — NEEDS-REVISION — L1392 removal edge case, bump ordering
- v10: Codex — NEEDS-REVISION (wrong review type); re-reviewed correctly — NEEDS-REVISION: feeCache L757/L944, late caps, fee-mode awareness
- v11: Codex — NEEDS-REVISION: FIX 2 placement, fee-inclusive health cap, donor ranking
- v12: Codex — NEEDS-REVISION: FIX 6a use bestSurplus not healthCap, FIX 6b would double-subtract
- v13: Codex — NEEDS-REVISION: solver/executor donor capacity mismatch
- v14: Claude code-reviewer — NEEDS-REVISION — unified formula double-counts unified accounts, maxTransferOut deletion ambiguous, FIX 6a removes needed guard, non-unified futures not all transferable
- v15: Claude code-reviewer — NEEDS-REVISION — unified account double-counts spot in FIX 7
- v16: Claude code-reviewer — ALL PASS — implementing
