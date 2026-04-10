# Plan: Capital Maximizer — Unified Transfer Oracle + Joint B&B Evaluation (v12)

## Goal
Maximize capital utilization across 6 exchanges without any exchange hitting L4 (0.80 margin ratio). Donor threshold relaxed from L3 (0.50) to L4 (0.80).

## Problem Statement
The current pool allocator evaluates transfer costs per-choice independently, then validates the combined plan post-hoc. When an exchange is both a trading leg and a donor, combined demand exceeds capacity → entire plan rejected → no money moves. Four separate transfer evaluation paths use different logic and different donor thresholds, causing solver/executor disagreements.

## Design

### Core Change: Shared `dryRunTransferPlan`
One function replaces 4 separate transfer evaluation paths. Used by both B&B solver (at leaf nodes) and post-solver validation.

```go
type transferStep struct {
    From, To string
    Amount   float64
    Fee      float64
    Chain    string
}

type dryRunResult struct {
    Feasible   bool
    TotalFee   float64
    Steps      []transferStep
    PostRatios map[string]float64
}

func (e *Engine) dryRunTransferPlan(
    choices []allocatorChoice,
    balances map[string]rebalanceBalanceInfo,
    feeCache map[string]feeEntry,
) dryRunResult
```

### dryRunTransferPlan Logic (executor-parity)
Must mirror `executeRebalanceFundingPlan` (allocator.go:1392+) semantics exactly:

1. Clone balances into `sim` map
2. Aggregate margin needs per exchange from all choices
3. For each exchange with need > 0:
   a. **Recipient spot→futures** (split-account only, skip if `IsUnified()`):
      Mirror executor logic at allocator.go:1470, 1487, 1497:
      - Compute deficit from `sim[exch].futures` directly (NOT from `rebalanceAvailable` which includes spot and would hide the real deficit):
        `marginDeficit = max(0, need - sim[exch].futures)`
      - Also compute ratioDeficit (same formula as step 3b)
      - `deficit = max(marginDeficit, ratioDeficit)`
      - `localMove = min(sim[exch].spot, deficit)`
      - Skip guard: if `localMove < 1.0` skip local transfer (mirrors executor at allocator.go:1497)
      - Update sim: `futures += localMove`, `spot -= localMove`, `futuresTotal += localMove`
      - Recompute remaining deficit after local move (for cross-exchange transfer)
   b. **Compute deficit** = `max(marginDeficit, ratioDeficit)`:
      - `marginDeficit = max(0, need - rebalanceAvailable(exch, sim[exch]))`
      - `actualMargin = need / MarginSafetyMultiplier`
      - `targetRatio = MarginL4Threshold - 0.005` (marginEpsilon)
      - `freeTarget = 1 - targetRatio`
      - `ratioDeficit = max(0, (freeTarget*futuresTotal - futures + actualMargin) / targetRatio)`
      - `deficit = max(marginDeficit, ratioDeficit)`
   c. **While deficit > 0**, find best donor:
      - For each exchange != exch in sim:
        - SKIP if `sim[donor].marginRatio >= MarginL4Threshold` ← unified threshold
        - Compute donor capacity via `allocatorDonorGrossCapacity` pattern:
          - Unified: `futuresAvail = sim[donor].futures`
          - Split: `futuresAvail = sim[donor].maxTransferOut` (or fallback to futures with safety cap)
          - `surplus = futuresAvail + sim[donor].spot (split only) - needs[donor]`
        - If `donor.hasPositions`: `surplus = min(surplus, capByMarginHealth(sim[donor]))` ← uses L4
        - Pick donor with **max surplus** (matches executor at allocator.go:1554+)
      - If no donor found: return `{Feasible: false}`
      - **Donor futures→spot simulation** (matches executor at allocator.go:1668, 1719, 1820-1821):
        - Only for split-account donors: compute `movedToSpot` to move from futures to spot
        - Cap by `freshMaxTransferOut * 0.99` (executor pattern at allocator.go:1699-1704)
        - Cap by `capByMarginHealth - needs[donor]` (executor pattern at allocator.go:1685)
        - Simulate internal move: `sim[donor].futures -= movedToSpot`, `sim[donor].spot += movedToSpot`
        - NOTE: The cross-exchange withdrawal comes out of SPOT, not futures.
          Do NOT debit futures again in the transfer step below.
      - **Fee lookup** from `feeCache`: determine chain (APT > BEP20), get fee + minWd
        - Fee-mode aware: check `WithdrawFeeInclusive()` for gross vs net (executor at allocator.go:1741-1751)
        - **Deduct netAmount + fee from donor sim balance** (mirrors executor at allocator.go:1604, 1817) to prevent overcommit on repeated transfers
      - `move = min(deficit, donorAvailable)`
      - Floor to minWd (fee-mode aware, mirrors executor at allocator.go:1629, 1769):
        - If `WithdrawFeeInclusive()`: effective floor = `minWd - fee` (net amount)
        - Else: effective floor = `minWd`
        - If `move < effectiveFloor` and `donorAvailable >= effectiveFloor`: `move = effectiveFloor`
        - Else if `donorAvailable < effectiveFloor`: skip this donor, try next
      - Apply to sim (mirrors executor at allocator.go:1820-1821):
        - **Split-account donor**: `sim[donor].spot -= move` (withdrawal from spot, NOT futures). Do NOT reduce `futuresTotal` — split-account spot withdrawal does not affect futures total.
        - **Unified-account donor**: `sim[donor].futures -= move`, `sim[donor].futuresTotal -= move` (unified pool, debit from futures directly, no spot involved)
        - **Recipient**: `sim[exch].futures += move`, `sim[exch].futuresTotal += move`
      - Record step, accumulate `totalFee += fee`
      - `deficit -= move`
4. **Simulate position opening**: for each choice, deduct `requiredMargin / MarginSafetyMultiplier` from both leg exchanges
5. **Post-trade L4 check** on ALL involved exchanges (legs + donors whose balance decreased):
   `ratio = 1 - max(0, sim[exch].futures) / sim[exch].futuresTotal`
   If `ratio >= MarginL4Threshold`: return `{Feasible: false}`
6. Return `{Feasible: true, TotalFee, Steps, PostRatios}`

---

## Implementation Steps

### Step 1: Unify L3→L4 threshold (20 occurrences across 2 files)
All `MarginL3Threshold` references in donor eligibility and health-cap contexts change to `MarginL4Threshold`.

**File: `internal/engine/allocator.go`** (14 occurrences):
- Line 136: `TransferablePerExchange` — donor skip
- Line 174: `TransferablePerExchange` — second donor skip
- Line 681: `findAllocatorDonor` — donor skip
- Line 682: log message L3 label → L4
- Line 817: `cheapestTransferFee` — donor skip
- Line 818: log message L3 label → L4
- Line 1036: `estimateAllocatorTransferCost` — donor skip (will be deleted in Step 5, but change for consistency until then)
- Line 1037: log message L3 label → L4
- Line 1160: `capByMarginHealth` — zero guard
- Line 1167: `capByMarginHealth` — cap formula divisor
- Line 1267: `simulateTransferPlan` — donor skip (will become wrapper in Step 3)
- Line 1550: `executeRebalanceFundingPlan` — donor skip
- Line 1551: log message L3 label → L4
- Line 1688: log message L3 label → L4

**File: `internal/engine/engine.go`** (6 occurrences):
- Line 722: sequential fallback donor exclusion
- Line 733: sequential fallback health-cap guard
- Line 735: sequential fallback health-cap formula divisor
- Line 810: sequential fallback donor exclusion (second block)
- Line 814: sequential fallback health-cap guard (second block)
- Line 816: sequential fallback health-cap formula divisor (second block)

**Pseudo-diff**: Each is `e.cfg.MarginL3Threshold` → `e.cfg.MarginL4Threshold` (or log string "L3" → "L4")

**Risk**: LOW — purely relaxes donor eligibility.

### Step 2: Implement `dryRunTransferPlan`
**File: `internal/engine/allocator.go`**

Add new structs `transferStep`, `dryRunResult` and function `dryRunTransferPlan` as specified in the Design section.

Key implementation details:
- Reuse `rebalanceAvailable()` for idle capital computation
- Reuse `capByMarginHealth()` (after Step 1 updates it to L4)
- Use `feeCache` parameter — `feeCache` is a local `map[string]feeEntry` created in `runPoolAllocator` (line 427) and `isAllocatorFundingFeasible` (line 610). Pass it as parameter, NOT as `e.feeCache` (no such field).
- Donor capacity computation must mirror `allocatorDonorGrossCapacity` (line 1092): unified vs split, futuresAvail from maxTransferOut, spot inclusion for split accounts
- Donor futures→spot prep must mirror executor (line 1668-1719): cap by maxTransferOut*0.99, cap by capByMarginHealth minus needs
- Fee deducted from donor sim balance to prevent overcommit
- Check `IsUnified()` via type assertion `interface{ IsUnified() bool }`

**Lines**: ~130 new lines
**Risk**: MEDIUM — must achieve executor parity

### Step 3: Refactor `simulateTransferPlan` into wrapper
**File: `internal/engine/allocator.go`**

Replace body of `simulateTransferPlan` (lines 1179-1377) with delegation to `dryRunTransferPlan`. The wrapper creates a local feeCache (same pattern as line 610):

```go
func (e *Engine) simulateTransferPlan(choices []allocatorChoice, balances map[string]rebalanceBalanceInfo) (feasible bool, totalFee float64) {
    feeCache := map[string]feeEntry{}
    result := e.dryRunTransferPlan(choices, balances, feeCache)
    return result.Feasible, result.TotalFee
}
```

**Lines**: ~200 lines deleted, ~5 lines new
**Risk**: LOW — behavior preserved by delegation

### Step 4: Replace B&B solver internals with joint eval
**File: `internal/engine/allocator.go`**

Changes in `solveAllocator` (line 423-515):
1. **Remove per-choice fee gating**: Currently `estimateChoiceTransferFee` (line 460) gates each choice and mutates `baseValue`. Remove this — let dryRunTransferPlan handle transfer feasibility at leaf level.
2. **Replace leaf evaluation**: Currently `allocatorChoiceValue` (line 442). Replace with:
```go
result := e.dryRunTransferPlan(current, balances, feeCache)
if !result.Feasible {
    return
}
value := 0.0
for _, c := range current {
    value += c.baseValue  // original baseValue, not fee-adjusted
}
value -= result.TotalFee
```
3. **Same changes in `greedyAllocatorSeed`** (line 517+): remove per-choice `estimateChoiceTransferFee` gating (line 528), use dryRunTransferPlan for seed value.

**Lines**: ~40 modified
**Risk**: MEDIUM — changes solver search behavior. The removal of per-choice pruning may increase search space, but timeout bounds runtime.

### Step 5: Delete dead code
**File: `internal/engine/allocator.go`**

After Steps 3-4, most functions are dead. But `isAllocatorFundingFeasible` still has callers at line 251 and 278 in `runPoolAllocator`.

**Sub-step 5a**: Replace callers of `isAllocatorFundingFeasible` (line 251, 278) with `dryRunTransferPlan`. Note: `feeCache` does not exist in `runPoolAllocator` scope, so create a local one:
```go
// At top of runPoolAllocator, after balance fetching:
feeCache := map[string]feeEntry{}

// Then replace callers:
// OLD: if !e.isAllocatorFundingFeasible(selected, balances) {
// NEW:
preCheck := e.dryRunTransferPlan(selected, balances, feeCache)
if !preCheck.Feasible {
```
The `feeCache` local variable is created once in `runPoolAllocator` and passed to both `solveAllocator` (which already creates its own at line 427 — change to accept as parameter) and `dryRunTransferPlan` calls. This ensures fee lookups are cached across all calls within one rebalance cycle.
This makes `isAllocatorFundingFeasible` truly dead.

**Sub-step 5b**: Delete functions with no remaining callers:
- `isAllocatorFundingFeasible` (~60 lines, line 591)
- `findAllocatorDonor` (line 656) — only called by isAllocatorFundingFeasible
- `estimateChoiceTransferFee` (~30 lines, line 767)
- `cheapestTransferFee` (line 790) — only called by estimateChoiceTransferFee
- `allocatorChoiceValue` (~25 lines, line 876)
- `estimateAllocatorTransferCost` (~100 lines, line 902)
- `findAllocatorDonorWithCache` (line 1011) — only called by estimateAllocatorTransferCost
- `firstSettlementProfit` (line 761) — only used in per-choice fee gating path (removed in Step 4)
- `xferFeeDeducted` field in `allocatorChoice` struct (line 64) — no longer populated or read after Step 4

**Verify**: For each function, `grep -n 'functionName' allocator.go` must return only the definition line (no callers). Delete in dependency order: leaf functions first, then their callers. For `xferFeeDeducted`, verify no references remain after Step 4 changes.

**Lines**: ~215 deleted
**Risk**: LOW — dead code removal, compiler will catch any missed references

### Step 6: No solver rerun — rely on sequential fallthrough (Step 7)
**File: `internal/engine/allocator.go`**

**Decision**: Do NOT add a solver rerun after revalidation/drop-worst.

**Reasons**:
1. The solver now uses `dryRunTransferPlan` for all feasibility, which does not consult the `capacity` map. Reducing capacity has no effect on a deterministic solver.
2. When the pool allocator fails (all choices dropped), Step 7 falls through to the sequential path at engine.go:652+, which evaluates opportunities one-by-one with per-leg donor rescue. This naturally avoids the accumulated-reservation conflicts.
3. The sequential path already uses the same L4 donor threshold (Step 1) and benefits from all the other improvements.

**Lines**: 0 new
**Risk**: LOW — no new code, rely on existing sequential fallback

### Step 7: Infeasible fallthrough to sequential
**File: `internal/engine/engine.go`**

At line 647, replace early return with fallthrough:
```go
// OLD:
} else {
    e.log.Warn("rebalance: pool allocator infeasible, skipping rebalance")
    return
}

// NEW:
} else {
    e.log.Warn("rebalance: pool allocator infeasible, falling through to sequential")
    // Fall through to sequential path below
}
```

**Lines**: ~3 modified
**Risk**: LOW — sequential path already exists and works

### Step 8: ensureFuturesBalance guard
**File: `internal/risk/manager.go`**

At line 970 (after `transferAmt` is floored), add guard:
```go
transferAmt = math.Floor(transferAmt*10000) / 10000
if transferAmt <= 0 {
    return // nothing to transfer, avoid API error
}
```

**Lines**: ~3 new
**Risk**: LOW — pure guard, prevents 0-amount API calls

### Step 9: Increase solver timeout significantly
**File: `internal/config/config.go`**

Keep the timeout mechanism (it's a valid safety bound), but increase the default significantly to allow near-complete search:

```go
// OLD: AllocatorTimeoutMs: 5,
// NEW: AllocatorTimeoutMs: 30000,
```

30 seconds — effectively "跑完為主"，但有安全上限防止極端情況卡住。

30 秒基本不會觸發，但作為安全上限存在。Live system 已有 `allocator_timeout_ms` dashboard toggle。

**Lines**: 1 modified
**Risk**: LOW — bounded, just more generous

### Step 10: Reduce sizing safety margins via config (no code change)
**File: no code change — config/dashboard adjustment only**

The sizing is controlled by existing config parameters:
- `MarginSafetyMultiplier` (default 2.0 at config.go:529) — multiplier on margin requirement
- `CapitalPerLeg` — fixed USDT per leg (0 = auto mode)
- Auto mode formula uses `remainingSlots * 2` divisor (50% buffer, documented in ARCHITECTURE.md:160)

**Decision**: Do NOT change the code or remove the safety buffer. These are intentional safety mechanisms that prevent over-leveraging. The L4 post-trade check is a REACTIVE safety net (reduces positions at 0.80), not a replacement for PRE-TRADE safety margins.

**Instead**: After deploying Steps 1-9, adjust `MarginSafetyMultiplier` via dashboard API from 2.0 to 1.5 (25% reduction, still conservative). Monitor for 1 week. If margin health stays good, reduce further to 1.2.

This is a tuning decision, not a code change. The existing dashboard toggle already supports it.

**Lines**: 0 (config adjustment only)
**Risk**: LOW — gradual tuning via existing config

### Step 11: Increase exchange pair breadth per symbol
**File: `internal/config/config.go`**

The real bottleneck for capital maximization is NOT `TopOpportunities` (default 25, already sufficient) but `TopPairsPerSymbol` (default 1 at config.go:517, enforced in ranker.go:114).

`TopPairsPerSymbol=1` means each symbol only keeps its BEST exchange pair. With 6 exchanges, there are up to 30 possible pairs per symbol. Keeping only 1 means the allocator can't try alternative pairs when the best one fails.

Change default from 1 to 10:
```go
// OLD: TopPairsPerSymbol: 1,
// NEW: TopPairsPerSymbol: 10,
```

This gives the B&B solver up to 10 exchange pairs per symbol to choose from, enabling it to route around bottleneck exchanges. Example: SIRENUSDT best pair is bitget/gateio, but if gateio is full, the solver can try bitget/bingx or binance/gateio.

**Note**: `TopOpportunities` stays at 25. With `TopPairsPerSymbol=10`, candidates ≈ 25 symbols × ~8 viable pairs = ~200 choices. Most get pruned by B&B, and with 30s timeout this completes easily.

**Lines**: 1 modified
**Risk**: LOW — more pairs = better routing, solver handles it

---

## Tonight Scenario Walkthrough

Balances: binance futures=21.55 spot=70.00 (surplus=91.55), bybit futures=0 spot=69.90 (unified, spot invisible to rebalanceAvailable), bingx futures=246.02 ratio=0.46, gateio futures=21.91 ratio=0.08, bitget futures=54.30, okx futures=19.74

Note: Bybit is unified (IsUnified()=true), so its spot=69.90 in FUND wallet is NOT included in rebalanceAvailable. Bybit contributes as donor only via its futures balance (0, or 60 after auto-transfer from FUND→UNIFIED during entry scan).

Top opportunity: SIRENUSDT bitget/gateio spread=7.34 bps/h

**With this plan:**
1. L4 threshold → bingx (ratio 0.46 < 0.80) is now a valid donor
2. B&B evaluates SIRENUSDT bitget/gateio at leaf → calls dryRunTransferPlan
3. dryRunTransferPlan: gateio needs ~60 margin, has 21.91 → deficit=38.09
4. Finds donor: binance surplus=91.55 (split account: futures 21.55 + spot 70.00). Donor futures→spot simulated.
5. Transfer 38.09 from binance to gateio, fee from GetWithdrawFee (APT chain, actual fee)
6. Post-trade check: binance ratio stays well below L4, gateio ratio OK
7. Feasible! value = baseValue - actual fee
8. B&B continues evaluating more positions with remaining capital (binance still has ~53, bingx has surplus, bitget has 54.30)
9. Solver returns optimal set → executor transfers funds → :40 entry opens positions
10. If pool allocator somehow fails → fallthrough to sequential → sequential also uses L4 donor threshold

**Result**: SIRENUSDT opens + potentially more positions with remaining capital.

---

## Testing Strategy

1. **Unit test: dryRunTransferPlan correctness** — mock balances, verify feasibility/infeasibility/fee computation matches expected
2. **Unit test: donor in (L3, L4) range accepted** — bingx ratio=0.46 must be accepted as donor
3. **Unit test: pool infeasible → sequential finds combo** — pool allocator drops all, sequential path picks up and opens position
4. **Unit test: pool infeasible → sequential fallthrough** — mock infeasible pool allocator, verify sequential runs
5. **Unit test: zero-amount ensureFuturesBalance** — spot=0, verify no API call made
6. **Unit test: dryRunTransferPlan vs executor parity** — same inputs, verify dryRun's feasibility matches what executor would actually do
7. **Integration: tonight scenario** — replay tonight's balances+opportunities, verify positions open
8. **Live: DryRun logging** — deploy with extra logging, verify dryRunTransferPlan decisions before enabling

---

## Summary

| Step | File | Lines | Risk |
|------|------|-------|------|
| 1. L3→L4 (20 occ) | allocator.go, engine.go | ~20 modified | LOW |
| 2. dryRunTransferPlan | allocator.go | ~130 new | MEDIUM |
| 3. simulateTransferPlan wrapper | allocator.go | ~200 deleted, ~5 new | LOW |
| 4. B&B joint eval + remove per-choice gating | allocator.go | ~40 modified | MEDIUM |
| 5. Delete dead code | allocator.go | ~215 deleted | LOW |
| 6. No rerun (use Step 7) | — | 0 | LOW |
| 7. Fallthrough | engine.go | ~3 modified | LOW |
| 8. Guard | manager.go | ~3 new | LOW |
| 9. Timeout 5ms→30s | config.go | ~1 modified | LOW |
| 10. Sizing via config tuning | — | 0 (config only) | LOW |
| 11. TopPairsPerSymbol 1→10 | config.go | ~1 modified | LOW |
| **Total** | | **+173 new, -415 deleted, ~64 modified** | |
| **Net** | | **-242 lines (code shrinks)** | |
