# Plan: Rank-First Rebalance — Invert Pool Allocator from Primary to Fallback

Version: v26
Date: 2026-04-20
Status: DRAFT

## Review History
- v1 (2026-04-20): Codex NEEDS-REVISION — rejection taxonomy misclassified sites; treated post-trade-ratio as final; executor contract wrong; override replay mismatch; overlapped 5bugs plan.
- v2 (2026-04-20): Codex NEEDS-REVISION (7 findings) — `ApproveDryRun` method assumed but does not exist; `$10` notional sites (438/503) misclassified as `Capacity`; `buildRebalancePrefetchCache` delegated to `allocatorDonorGrossCapacity` (not parity with allocator's inline math which uses `rebalanceAvailable`); asymmetric replay/prune contract still real; fallback branch point vague; line numbers drifted after 5bugs.
- v3 (2026-04-20): Codex 6/7 PASS, NEEDS-REVISION on residuals — F7 used `~938` which is still inside the prune loop (`selected--`); stale `ApproveDryRun` mention at line ~205; asymmetric-replay rationale overstated fallback safety; version example behind current `VERSION`.
- v4 (2026-04-20): NEEDS-REVISION on 4 text-only residuals — `:1065` block mislabeled; line 430 Risks-table wording still contradicted the corrected asymmetry section; stale `v3`/`v4` tokens in Dependencies/Rollout/Out-of-Scope sections; review-history retained `ApproveDryRun` tokens.
- v5 (2026-04-20): Final re-review NEEDS-REVISION on 6 residual issues — Change 3 compile-time hole (`runPoolAllocator` still uses local `transferable` for `revalCache` at `allocator.go:430-432`); fallback gate wording "at :945" imprecise (`:945` IS the summary log, gate goes AFTER it); stale `348-376` range should be `348-381`; allocator objective citation points to `greedyAllocatorSeed` (`:723-746`) instead of real objective sites (`:639-652, :688-695`); `manager.go:649` label "spread stability CV gate" too narrow; `$10` notional explanation missed the `:367-368` mutation anchor; missing tests for notional-floor rescue and shared-cache overcommit prune; missing throughput regression note.
- v6 (2026-04-20): Final re-review 6/6 R PASS + 2/3 A PASS — A3 INACCURATE (throughput mitigation claimed a log line that Change 5 never instructed adding); "12 scenarios" token stale in 2 places after adding Tests 13+14.
- v7 (2026-04-20): Final ALL PASS in dependent re-review.
- v8 (2026-04-20): Alt-pair walk added but used wrong `CheckPairFilters` signature.
- v9 (2026-04-20): CheckPairFilters signature fixed, but altOpp NextFunding assignment was guarded `if !IsZero()` — diverges from live pattern (allocator.go:583-592, engine.go:1219-1225 set it unconditionally) and would inherit primary's funding window when alt has zero NextFunding, misclassifying the alt pair in the funding-window gate.
- v10 (2026-04-20): Independent fresh review found 7 issues — (1) Change 4 pair-walk only remembered first rejection, masking alt Capital rejections under primary non-Capital rejection (no rescue for viable alt); (2) no nil-approval guard when all attempts error/filtered; (3) missing pool allocator's alt pre-approval guards (`exchange exists`, `alt.Spread > 0`); (4) `rebalanceFunds()` line drift `:527` → `:530`; (5) missing tests for primary-non-capital+alt-capital, nil-approval, alt-zero-NextFunding; (6) v9/v10 history overstated NextFunding parity (allocator filter is unconditional but engine entry-patching is conditional — distinction not articulated); (7) rollout version text stale.
- v11 (2026-04-20): 7/7 regular PASS; new issue: per-attempt `err = aerr` poisoned the outer loop's legacy `if err != nil { continue }` gate at `engine.go:728-732`, so a primary-pair error could skip the symbol even when a later alt would succeed.
- v12 (2026-04-20): Change 4 now explicitly REPLACES the two-line BEFORE block (approval call + `if err != nil` gate) — not just the call. Per-attempt errors are renamed `firstErr` (diagnostic only, surfaced in the `default:` skip log) and never gate the outer loop, mirroring `allocator.go:570-579` appendChoice behavior. New Test 19 covers primary-errors + alt-succeeds.
- v13 (2026-04-20): Codex review of v12 found 4 residual issues. (1) `$10` notional-floor curability overstated — in fixed-capital mode when `CapitalPerLeg * leverage < 10`, no transfer top-up can raise size because `manager.go:876-878` caps `maxPositionValue = ecl2 * leverage` BEFORE the balance cap. Notional rejection at `:438`/`:503` must be `RejectionKindConfig` (not rescueable) when the config ceiling is below $10; `RejectionKindCapital` only when top-up can plausibly raise size. New Test 13b pins the uncurable-ceiling case. (2) 697/702 cap-rescue rationale corrected — transfer DOES mutate `pair.bal.Total` at `:367-368`, which feeds `totalCapital := longBal.Total + shortBal.Total` at `:673`; keeping caps as non-rescueable stands but the reason must not claim "transfer doesn't change total exposure math". (3) Symmetric-replay carry-forward claim weakened — for the SAME `allocatorChoice` the gate is identical, but fallback's alternative-pair evaluation at `allocator.go:617-618` can still select a DIFFERENT feasible choice (alt pair or lower-rank bundle). Risks-table wording updated to not claim guaranteed non-recovery. (4) Line-anchor drift fixed: `:882-908` → `:896-909`; `:983` → `:984-987`; `:1065` → `:1066-1069`. Also Test 11's regression-guard framing clarified.
- v14 (2026-04-20): Codex review of v13 found 1 residual issue. `notionalRejectionKind` helper was gated on raw `cfg.CapitalPerLeg` but the actual sizing cap at `manager.go:876-878` uses `ecl2 := m.effectiveCapitalPerLeg(string(StrategyPerpPerp))` (manager.go:847). `effectiveCapitalPerLeg` (defined at manager.go:54-61) returns the allocator-derived per-leg cap for unified capital mode even when `cfg.CapitalPerLeg == 0` (see `allocator.go:750-753`). Fixed helper signature and call to use `effectiveCapitalPerLeg`.
- v15 (2026-04-20): Codex review of v14 found 1 residual issue. v14's Change 2 snippet claimed `ecl2` was "already in scope" at `:438`/`:503`, but `ecl2` is local to `calculateSizeWithPrice` (defined at `:842`/`:847`), while the notional-rejection returns live in `approveInternal` (`:207-onwards`). v15 replaced the snippet with unconditional local re-derivation.
- v16 (2026-04-20): Codex review of v15 found 1 residual issue: non-history plan text still referenced `ecl2` in taxonomy conditions. v16 normalized to `effectiveCap` and reserved `ecl2` for code-speak. v16 normal review returned ALL PASS.
- v17 (2026-04-20): Codex INDEPENDENT review of v16 found 4 residual issues (taxonomy approximation documented as known limitation; concurrency race eliminated by plumbing effectiveCap through calculateSizeWithPrice; Test 13/13b fixtures fixed; allocator.go anchor corrected).
- v18 (2026-04-20): Codex review of v17 found 4 residual issues (stale re-derive wording; wrong calculateSizeWithPrice signature in snippet; Test 13b fixture used non-existent active-position formula; v17 Review History narrative about Test 13 was inaccurate). v18 fixed all four with the correct single-snapshot pattern, real signature, correct allocator formula, and history correction.
- v19 (2026-04-20): Codex review of v18 found 1 residual issue — raw `m.cfg.Leverage` vs clamped `leverage` at classifier. v19 fixed with symmetric plumbing (compute clamped leverage in approveInternal, pass through sizing, reuse at classification).
- v20 (2026-04-20): Codex review of v19 found 2 residual issues (test fixtures used raw cfg.Leverage; "Other callers" instruction only mentioned effectiveCap not both new args). v20 fixed both. v20 normal review returned ALL PASS.
- v21 (2026-04-20): Codex INDEPENDENT review of v20 found 2 residual issues. (1) Call-site snippet used placeholder variable names (refPrice/cachedActive) instead of the real caller's midPrice/active. (2) Substantive gap: alt-pair selections without transfer never reached entry via allocOverrides because override storage gates were localTransferHappened-only. v21 fixed both via real variable names + new altOverrideNeeded flag + extended override-storage gates + Test 15 sub-cases 15a/15b + new Risks row.
- v22 (2026-04-20): Codex review of v21 found 1 residual issue — executor-path keepFundedChoices passed nil,nil for the last two args but real contract passes result.FundedReceivers, result.PendingDeposits. v22 fixed both paths and added precision paragraph.
- v23 (2026-04-20): Codex review of v22 found 1 residual issue — documented signature post/pending/return types were wrong. v23 corrected post (`rebalanceBalanceInfo`), pending (`rebalancePending`), return (`map[string]allocatorChoice`), and struct name (`rebalanceExecutionResult`). BUT v23 ALSO over-corrected by changing the FIRST parameter from slice to map, which was a mistake.
- v24 (2026-04-20): Corrected first parameter of keepFundedChoices from map to slice.
- v25 (2026-04-20): Aligned plan text with pushed-branch source (pre-dependency-merge): 3-arg keepFundedChoices, marginRatio check, corrected line anchors. 12/15 PASS on Codex review.
- v26 (this draft): Codex review of v25 (task 98353e1d at pinned HEAD a02eeb0e) found 3 residual spots that missed the v25 sweep. (1) Files-to-Change table row at plan line 110 still cited stale `348-381` and `:430-432` (the detailed Change 3 section was fixed in v25 but the summary table row at line 110 was missed). v26 updates both anchors in that row to `282-315` and `:363-366`. (2) Plan line 627 (paragraph above the signature code block) still said "accounting for funded receivers and pending deposits" and cited `allocator.go:246`; v26 updates to "accounting for funded receivers" only, anchors to `allocator.go:188`. (3) Plan line 641 (paragraph after the signature block) still described "the last two arguments carry cross-exchange pending-funding intent" and cited `allocator.go:129-137` for struct fields; v26 rewrites to "the funded argument carries the cross-exchange funded-receiver signal" and re-anchors struct fields to `allocator.go:91-100`.

<details>
<summary>v8 description (retained for history)</summary> Independent reviewer caught 4 issues that dependent reviews missed — (1) Tier-1 only tried primary pair via `alt=nil`, missing `opp.Alternatives` evaluation that current pool allocator does (would lose feasible alt-pair selections for rank-N when primary fails); (2) plan said "extend manager_test.go" but that file does not exist; (3) tests 4+13 omitted fixed-capital precondition needed for cache top-up branch; (4) minor anchor drift `allocator.go:929-937` → `936-940`. v8 adds per-symbol pair walk (primary + alternatives) in Change 4, fixes manager_test.go instruction to "create new", adds `CapitalPerLeg > 0` precondition to tests 4/13, corrects the `936-940` anchor, adds Test 15 for alt-pair coverage.
</details>

## Dependencies (Strict ordering)

**MUST land AFTER:**
1. `plans/PLAN-rebalance-partial-5bugs.md` v5 — provides `selectedChoices`, `buildChoices`, dry-run prune, override store, `localTransferHappened`.
2. `plans/PLAN-margin-ratio-unify.md` v3 — provides `donorHasHeadroom`, `capByMarginHealth`. THIS plan consumes whichever formulas are current at merge time.

Both are referenced by exact line numbers in the current tree below.

If either dependency has not landed, **stop — do not implement this plan**.

## Problem Statement

`rebalanceFunds()` (engine.go:530) runs pool allocator first whenever `EnablePoolAllocator=true` (engine.go:628-675), falling through to the sequential rank-ordered planner only on failure/infeasibility. Pool allocator maximizes bundled `baseValue = funding_value − fees` with no rank tiebreak (see objective/tiebreak in `allocator.go:639-652` and `:688-695`; `:723-746` is the `greedyAllocatorSeed` that feeds the same baseValue-ordered solver). Selected symbols are stored as `allocOverrides` (engine.go:648-652) and consumed by the next entry scan (engine.go:1174-1276, 1394-1415).

Consequence: a rank-1 opportunity that is only marginally short on margin can be displaced in favor of a non-rank-1 bundle with higher `baseValue` sum. Rank — the expected-profit ordering from `discovery` — is destroyed inside the value-maximizer.

User intent: rank represents expected profitability; capital should preferentially fund the highest-rank feasible combination, falling back to value-optimization only when no rank-ordered opportunity is feasible (with or without transfer rescue) due to non-capital reasons.

## Design

Two-tier planner inside `rebalanceFunds`:

```
rebalanceFunds(opps):
  balances = snapshot()
  filter active+blacklist
  cache = buildTransferableCache(balances)   # NEW shared helper, extracted from allocator.go:282-315

  # ---- Tier 1: Rank-First with Capital Rescue (PRIMARY) ----
  # Uses the existing sequential planner delivered by 5bugs Bug B,
  # upgraded to call SimulateApprovalForPair(..., cache) for
  # transfer-aware approval (post-trade-ratio sites can be cured).
  tier1 := run sequential planner with cache
  if len(tier1.selectedChoices) > 0:
      execute transfers, store overrides (5bugs Bug C path)
      return

  # ---- Tier 2: Pool Allocator (FALLBACK) ----
  if !EnablePoolAllocator: return
  allocSel := runPoolAllocator(opps, balances, remainingSlots)
  if allocSel.feasible:
      execute, store overrides (existing pool allocator path)
```

Rejection classification used by Tier-1's rescue decision:

| Kind | Rescueable | Sites in manager.go |
|---|---|---|
| `Capital` | **YES** | 430, 438*, 450, 465, 503*, 512, 525 |
| `Capacity` | NO | 697, 702 |
| `Market` | NO | 421, 490 |
| `Spread` | NO | 563, 588, 613, 627, 639, 649 |
| `Health` | NO | 657, 663 |
| `Config` | NO | 227, 234, 240, 438**, 503** |

In the rest of this section and in the rejection-site table below, `effectiveCap` denotes the single-snapshot local computed ONCE at the top of `approveInternal` (before sizing) and threaded through both `calculateSizeWithPrice` (via a new parameter) and the notional-floor classifier — see Change 2 for the snippet. The `ecl2` token is reserved for describing the same-named local inside `calculateSizeWithPrice` where sizing consumes the plumbed value at `manager.go:876-878`; after v18 plumbing, `ecl2` is the passed-in parameter, so `ecl2 == effectiveCap` by construction, with no concurrent-mutation window.

\* `438`/`503` are `Capital` when the per-leg capital ceiling permits a size ≥ `$10` after top-up — i.e., `effectiveCap == 0` (no per-leg ceiling applied; sizing is balance-bound) OR `effectiveCap * leverage ≥ 10` (ceiling high enough that top-up can bridge the balance gap).
\*\* `438`/`503` are `Config` when `effectiveCap > 0 && effectiveCap * leverage < 10`, because `manager.go:876-878` fixes `maxPositionValue = ecl2 * leverage` BEFORE the balance cap at `:879-881` (and `ecl2 == effectiveCap` by construction — v17 eliminates re-derivation by plumbing the value as a parameter). In that regime, no transfer top-up can raise sizing; the rejection must be treated as non-rescueable. `effectiveCap` is produced by `Manager.effectiveCapitalPerLeg` (`manager.go:54-61`), which falls back to `cfg.CapitalPerLeg` when allocator-derived. The allocator-derived unified-capital path lives at `allocator.go:746-779` overall (with the manual-override short-circuit at `:750-753` and the derived unified-capital branch at `:760-779`). Gating on raw `cfg.CapitalPerLeg` would miss the derived-cap case.

**Why `$10` notional (438, 503) can be `Capital`**: the dry-run cache top-up branch starts at `manager.go:330`, and the actual `pair.bal.Available` / `pair.bal.Total` mutations happen at `manager.go:367-368` — both BEFORE the sizing call at `:427`. When `maxPositionValue` is balance-bound (`effectiveCap == 0`, or `effectiveCap * leverage ≥ 10` where the ceiling doesn't bind), a sufficient top-up lets `calculateSizeWithPrice` return a larger size, which passes the notional floor at `:435-438` / `:499-503`.

**Why `$10` notional can ALSO be `Config`**: when `effectiveCap > 0 && effectiveCap * leverage < 10`, the cap at `manager.go:877-878` fixes `maxPositionValue` strictly below $10 regardless of balance. Transfer top-up cannot cure this because the subsequent balance cap at `:879-881` only LOWERS `maxPositionValue`. Classifying this sub-case as `Config` (rather than blindly `Capital`) prevents Tier-1 from initiating a pointless rescue attempt on an opp whose rejection no cross-exchange transfer can fix. This applies to BOTH fixed-capital (`cfg.CapitalPerLeg > 0`) and unified-capital modes where the allocator derives a small per-leg cap.

**Why per-exchange `%` cap (697, 702) is `Capacity`, not rescued by this plan**: that check compares opened position value against the per-exchange cap on total deployed capital. Transfer DOES mutate `pair.bal.Total` at `:367-368`, and that flows into `totalCapital := longBal.Total + shortBal.Total` at `:673`, so a transfer CAN change the denominator of the ratio. However, rescuing cap rejections would require per-pair cap-ratio math this plan does not implement. Cap-rescue is therefore out of scope; the `RejectionKindCapacity` tag means "this plan does not attempt rescue", not "transfer cannot in principle affect cap math".

## Files to Change

| File | Change |
|---|---|
| `internal/models/interfaces.go` | Add `RejectionKind` enum; add `Kind RejectionKind` field to `RiskApproval`. |
| `internal/risk/manager.go` | At each of the 22 rejection returns in `approveInternal`, set `Kind:`. Additionally: (a) add private helper `notionalRejectionKind(effectiveCap, leverage)`; (b) change `calculateSizeWithPrice` signature to accept `effectiveCap float64` AND `leverage int` as new parameters (remove both the internal `m.effectiveCapitalPerLeg(...)` call at `:847` AND the leverage clamp at `:857-859`); (c) in `approveInternal` compute `effectiveCap := m.effectiveCapitalPerLeg(string(StrategyPerpPerp))` AND `leverage := m.cfg.Leverage` with `MaxLeverage()` clamp ONCE before sizing and pass both in; reuse the SAME locals at the `:438`/`:503` classification. Parallel update at `:265`: `needed := m.effectiveCapitalPerLeg(...)` → `needed := effectiveCap`. Update all other callers of `calculateSizeWithPrice` to pass the new arguments (the only current direct caller is `:427`). No behavior change beyond eliminating internal re-derivations (strictly safer under concurrent `dynamicCaps` updates and under any future change to the leverage clamp). |
| `internal/engine/allocator.go` | Extract inline `transferable` builder (lines 282-315) into new `func (e *Engine) buildTransferableCache(balances map[string]rebalanceBalanceInfo) *risk.PrefetchCache`. `runPoolAllocator` calls the helper AND the downstream `revalCache` at `:363-366` is updated to reuse the same cache (see Change 3 details). |
| `internal/engine/engine.go` | In `rebalanceFunds`: (a) call `buildTransferableCache` once near top; (b) switch Tier-1 approval call from `SimulateApproval` to `SimulateApprovalForPair(opp, opp.LongExchange, opp.ShortExchange, reserved, nil, cache)`; (c) replace `isMarginRejection` substring match with `approval.Kind == models.RejectionKindCapital`; (d) invert primary/fallback by wrapping the existing pool allocator block with a `len(selectedChoices) == 0` gate, placed BEFORE any balance-mutating step. |
| `internal/engine/engine_rank_first_test.go` | New test file — 20 scenarios (1–13, 13b, 14–19) below. |
| `internal/risk/manager_test.go` | **Create new** (file does not exist in tree) `internal/risk/manager_test.go` with one assertion per rejection site that `Kind` matches the taxonomy table. |

## Concrete Changes

### Change 1: `internal/models/interfaces.go`

ADD above `type RiskApproval struct`:

```go
// RejectionKind classifies why a risk approval was rejected so callers can
// decide whether the rejection is curable by cross-exchange transfer.
// Zero value (RejectionKindNone) applies to approved opportunities.
type RejectionKind int

const (
    RejectionKindNone     RejectionKind = iota // Approved=true
    RejectionKindCapital                        // curable by transfer: insufficient capital / margin buffer / post-trade ratio / notional floor
    RejectionKindCapacity                       // NOT curable: per-exchange capital cap
    RejectionKindMarket                         // NOT curable: empty orderbook
    RejectionKindSpread                         // NOT curable: slippage / gap / stability
    RejectionKindHealth                         // NOT curable: exchange health scorer
    RejectionKindConfig                         // NOT curable: max positions / exchange not configured
)

func (k RejectionKind) String() string {
    switch k {
    case RejectionKindCapital:  return "capital"
    case RejectionKindCapacity: return "capacity"
    case RejectionKindMarket:   return "market"
    case RejectionKindSpread:   return "spread"
    case RejectionKindHealth:   return "health"
    case RejectionKindConfig:   return "config"
    default:                    return "none"
    }
}
```

MODIFY `RiskApproval`:

```go
type RiskApproval struct {
    Approved          bool           `json:"approved"`
    Kind              RejectionKind  `json:"kind"`            // NEW
    Size              float64        `json:"size"`
    Reason            string         `json:"reason"`
    Price             float64        `json:"price"`
    GapBPS            float64        `json:"gap_bps"`
    RequiredMargin    float64        `json:"required_margin"`
    LongMarginNeeded  float64        `json:"long_margin_needed"`
    ShortMarginNeeded float64        `json:"short_margin_needed"`
}
```

### Change 2: `internal/risk/manager.go` — annotate all 22 rejection sites

| Line | Reason fragment | Kind |
|---|---|---|
| 227 | max positions reached | `RejectionKindConfig` |
| 234 | long exchange not configured | `RejectionKindConfig` |
| 240 | short exchange not configured | `RejectionKindConfig` |
| 421 | empty orderbook on long exchange | `RejectionKindMarket` |
| 430 | insufficient capital for minimum position size | `RejectionKindCapital` |
| 438 | long-leg notional $10 minimum | `RejectionKindCapital` if `effectiveCap == 0 \|\| effectiveCap*float64(leverage) >= 10`, else `RejectionKindConfig`. Both `effectiveCap` and `leverage` are single-snapshot locals computed once at the top of `approveInternal` (`effectiveCap := m.effectiveCapitalPerLeg(string(StrategyPerpPerp))`, `leverage := m.cfg.Leverage` then clamped by `MaxLeverage()`) and threaded into sizing as new parameters; classification reuses the SAME locals, no rejection-site re-derivation. |
| 450 | insufficient margin buffer on long | `RejectionKindCapital` |
| 465 | post-trade margin ratio (long leg) | `RejectionKindCapital` |
| 490 | empty orderbook on short exchange | `RejectionKindMarket` |
| 503 | short-leg notional $10 minimum | `RejectionKindCapital` if `effectiveCap == 0 \|\| effectiveCap*float64(leverage) >= 10`, else `RejectionKindConfig`. Same `effectiveCap` and clamped `leverage` locals as the long-leg row — single snapshot, no re-derivation. |
| 512 | insufficient margin buffer on short | `RejectionKindCapital` |
| 525 | post-trade margin ratio (short leg) | `RejectionKindCapital` |
| 563 | slippage too high (no adaptive path) | `RejectionKindSpread` |
| 588 | slippage too high at minimum size | `RejectionKindSpread` |
| 613 | price gap too high (hard cap) | `RejectionKindSpread` |
| 627 | price gap with zero funding spread | `RejectionKindSpread` |
| 639 | gap recovery intervals exceeded | `RejectionKindSpread` |
| 649 | spread stability rejection (any reason from `SpreadStabilityChecker.Check()`: insufficient history, non-positive mean spread, or CV instability) | `RejectionKindSpread` |
| 657 | exchange health too low (long) | `RejectionKindHealth` |
| 663 | exchange health too low (short) | `RejectionKindHealth` |
| 697 | would exceed % capital cap (long) | `RejectionKindCapacity` |
| 702 | would exceed % capital cap (short) | `RejectionKindCapacity` |

Edit pattern for each site:
```go
// BEFORE
return &models.RiskApproval{Approved: false, Reason: reason}, nil
// AFTER
return &models.RiskApproval{Approved: false, Kind: models.RejectionKind<X>, Reason: reason}, nil
```

Notional-floor sites (`:438`, `:503`) use a small helper to pick `Kind`. The helper takes `effectiveCapitalPerLeg` (the same value sizing uses), NOT raw `cfg.CapitalPerLeg` — otherwise unified-capital mode (where `cfg.CapitalPerLeg == 0` but the allocator derives a positive per-leg cap via `allocator.go:760-779`) would be misclassified.

**Concurrency-safe plumbing**: re-deriving `effectiveCapitalPerLeg` at the rejection site is NOT strictly safe under concurrent runtime config changes because `Manager.effectiveCapitalPerLeg` (`manager.go:54-61`) takes no lock, and `CapitalAllocator.EffectiveCapitalPerLeg` only locks `dynamicCaps` (`allocator.go:770-776`), not `cfg`. A `dynamicCaps` update between sizing at `manager.go:847` and classification at `:438`/`:503` could yield different values and flip the Kind. To eliminate the race entirely, `approveInternal` computes `effectiveCap` ONCE before sizing, passes it into `calculateSizeWithPrice` as a new parameter, and reuses the same local at classification.

This requires a signature change on `calculateSizeWithPrice`. The current real signature at `manager.go:842` is:

```go
// BEFORE (manager.go today, line 842):
func (m *Manager) calculateSizeWithPrice(opp models.Opportunity, balances map[string]float64, refPrice float64, cachedActive []*models.ArbitragePosition) float64 {
    ...
    ecl2 := m.effectiveCapitalPerLeg(string(StrategyPerpPerp))  // :847 — remove this derivation
    // :857-859 clamp leverage locally:
    leverage := m.cfg.Leverage
    if leverage > MaxLeverage() { leverage = MaxLeverage() }
    if ecl2 > 0 {
        maxPositionValue = ecl2 * float64(leverage)  // :877-878 uses clamped leverage local
        ...
    }
    ...
}

// AFTER — caller passes BOTH the effectiveCap snapshot and the clamped leverage; sizing no longer derives either internally:
func (m *Manager) calculateSizeWithPrice(opp models.Opportunity, balances map[string]float64, refPrice float64, cachedActive []*models.ArbitragePosition, effectiveCap float64, leverage int) float64 {
    ...
    // ecl2 and leverage are now passed-in snapshots; drop the m.effectiveCapitalPerLeg call at :847
    // and drop the local leverage clamp at :857-859.
    ecl2 := effectiveCap
    if ecl2 > 0 {
        maxPositionValue = ecl2 * float64(leverage)
        ...
    }
    ...
}
```

The helper and rejection-site pattern. Both `effectiveCap` and clamped `leverage` are computed ONCE in `approveInternal` and threaded into sizing and classification:

```go
// helper at top of manager.go (private):
func notionalRejectionKind(effectiveCap float64, leverage int) models.RejectionKind {
    if effectiveCap > 0 && effectiveCap*float64(leverage) < 10 {
        return models.RejectionKindConfig
    }
    return models.RejectionKindCapital
}

// inside approveInternal (around :207 onward) — compute BOTH locals ONCE at the top,
// mirroring the clamp logic that calculateSizeWithPrice currently performs at :857-859:
effectiveCap := m.effectiveCapitalPerLeg(string(StrategyPerpPerp))
leverage := m.cfg.Leverage
if leverage > MaxLeverage() {
    leverage = MaxLeverage()
}

// pass both into sizing (the real caller variables at manager.go:423-427 are named
// `midPrice` and `active`, not `refPrice`/`cachedActive`; use the actual names):
size := m.calculateSizeWithPrice(opp, balances, midPrice, active, effectiveCap, leverage)

// at :438 / :503 — reuse the SAME locals, no re-derivation:
return &models.RiskApproval{Approved: false, Kind: notionalRejectionKind(effectiveCap, leverage), Reason: reason}, nil
```

**Parallel update at `manager.go:265`**: the dry-run auto-sweep path has `needed := m.effectiveCapitalPerLeg(string(StrategyPerpPerp))` which performs a second re-derivation that could return a different value if `dynamicCaps` mutates. Replace with `needed := effectiveCap` to reuse the same snapshot — this assumes `approveInternal`'s `effectiveCap` local is in scope at `:265`. If `:265` is inside a helper called from `approveInternal`, pass `effectiveCap` through as an additional argument too, OR snapshot `effectiveCap` at a shared call site and inject into both paths. Verify the call-site tree at implementation time via `grep -n effectiveCapitalPerLeg internal/risk/` and ensure EVERY use resolves to the single approveInternal snapshot.

Other callers of `calculateSizeWithPrice`: the only direct caller in the current tree is `manager.go:423-427` (inside `approveInternal`, reads `size := m.calculateSizeWithPrice(opp, balances, midPrice, active)`). The helper at `manager.go:842` is the only definition. No test or other package calls it. Update the single call site at `:427` to pass BOTH new arguments using the actual caller-side variable names: `m.calculateSizeWithPrice(opp, balances, midPrice, active, effectiveCap, leverage)`.

No `Reason` text change; the only behavior change is removing an internal re-derivation — mathematically identical when `dynamicCaps` is stable, strictly safer under concurrent cap updates.

This correctly tags the uncurable `effectiveCap * leverage < 10` regime as `Config` across both fixed-capital and unified-capital modes, while preserving `Capital` when the ceiling permits rescue (`effectiveCap == 0` or `effectiveCap * leverage ≥ 10`).

**Known approximation** (from v16 independent review Finding 1): the Kind for `:430`/`:438`/`:503` is computed from `effectiveCap * leverage < 10` alone, but `calculateSizeWithPrice` can return 0 for non-capital reasons — step-size rounding (`manager.go:893`), short-leg price discount (`:893` uses long-side `refPrice` while short notional at `:499-503` uses `shortMid`), min-size constraints (`:829-831`), or invalid prices (`:889-890`). When any of these fires WITH `effectiveCap * leverage >= 10`, the rejection is tagged `Capital` but transfer top-up cannot cure the underlying cause (step, short-price, min-size, market). The direction of this misclassification is always `false-Capital` — Tier-1 will attempt a donor rescue that cannot succeed for step/market-bound cases, wasting one iteration but introducing no correctness bug. The opposite direction (`false-Config`) is impossible under this condition: `effectiveCap * leverage < 10` mathematically guarantees sizing cannot produce a size ≥ $10 regardless of balance, step, or price, so Config is always a correct tag. Proper discrimination of step/market-bound zero-size cases requires extending `calculateSizeWithPrice` to return a structured rejection reason — deferred to a follow-up plan. **Follow-up marker**: if live logs after this plan ships show rescue-attempt latency or donor-thrash from futile step/market rescues > 5 events/week, open a sizing-reason extension plan.

### Change 3: `internal/engine/allocator.go` — extract shared transferable builder

CURRENT inline block at `allocator.go:282-315`:
```go
transferable := make(map[string]float64, len(balances))
for recipient := range e.exchanges {
    var totalSurplus float64
    for donor, bal := range balances {
        if donor == recipient { continue }
        if bal.marginRatio >= e.cfg.MarginL4Threshold {
            continue // skip unhealthy donors
        }
        surplus := e.rebalanceAvailable(donor, bal)
        if bal.hasPositions {
            healthCap := e.capByMarginHealth(bal)
            if healthCap == math.MaxFloat64 {
                e.log.Debug("allocator: capByHealth %s: MaxFloat64 (no positions)", donor)
            }
            if surplus > healthCap { surplus = healthCap }
        }
        if surplus > 0 { totalSurplus += surplus }
    }
    transferable[recipient] = totalSurplus
}
e.log.Debug("allocator: transferable: %v", transferable)

cache := &risk.PrefetchCache{
    TransferablePerExchange: transferable,
}
```

(Note: margin-ratio-unify plan, when merged, replaces the `bal.marginRatio >= e.cfg.MarginL4Threshold` check with a call to `donorHasHeadroom(bal)`. This plan's extracted helper mirrors whatever the pool allocator's inline block uses at merge time — the parity test in the Tests section guards against divergence.)

AFTER — extract into helper, keep both call sites semantically identical:

Add new function (place near `rebalanceAvailable` in `allocator.go`):
```go
// buildTransferableCache computes per-exchange donor surplus identically to
// the pool allocator's long-standing inline logic, and returns it as a
// PrefetchCache that SimulateApprovalForPair can consume.
// Extracted so both pool allocator and rank-first Tier-1 use the same math.
func (e *Engine) buildTransferableCache(balances map[string]rebalanceBalanceInfo) *risk.PrefetchCache {
    transferable := make(map[string]float64, len(balances))
    for recipient := range e.exchanges {
        var totalSurplus float64
        for donor, bal := range balances {
            if donor == recipient { continue }
            if bal.marginRatio >= e.cfg.MarginL4Threshold {
                continue // skip unhealthy donors
            }
            surplus := e.rebalanceAvailable(donor, bal)
            if bal.hasPositions {
                healthCap := e.capByMarginHealth(bal)
                if healthCap == math.MaxFloat64 {
                    e.log.Debug("allocator: capByHealth %s: MaxFloat64 (no positions)", donor)
                }
                if surplus > healthCap { surplus = healthCap }
            }
            if surplus > 0 { totalSurplus += surplus }
        }
        transferable[recipient] = totalSurplus
    }
    e.log.Debug("allocator: transferable: %v", transferable)
    return &risk.PrefetchCache{
        TransferablePerExchange: transferable,
    }
}
```

REPLACE the inline block at `allocator.go:282-315` with:
```go
cache := e.buildTransferableCache(balances)
candidates := e.buildAllocatorCandidates(opps, cache)
```

**Downstream dependency fix (CRITICAL)**: `runPoolAllocator` also reuses the local `transferable` identifier at `allocator.go:363-366` to build `revalCache`:

```go
// BEFORE (allocator.go:363-366)
revalCache := &risk.PrefetchCache{
    TransferablePerExchange: transferable, // same transfer-aware view as buildAllocatorCandidates
}

// AFTER — reuse the extracted cache
revalCache := cache
```

After extraction, the local `transferable` identifier is gone; leaving the old line would be a compile error. The extracted `cache` returned by `buildTransferableCache` is the same `*risk.PrefetchCache` shape, safe to reuse for revalidation (both the initial `buildAllocatorCandidates` and revalidation want the same `TransferablePerExchange` view).

Parity is guaranteed — it's the same function body.

### Change 4: `internal/engine/engine.go` — wire Tier-1 to use shared cache and typed kind

In `rebalanceFunds`, after the balance snapshot / active-symbol filter (before the current pool allocator block at `:628`), add:
```go
// Build transfer-aware cache once, used by both Tier-1 approval calls and
// (transitively) the pool allocator fallback via its own re-construction.
// Keeping this at the top lets Tier-1 consume the same TransferablePerExchange
// math the pool allocator uses.
prefetchCache := e.buildTransferableCache(balances)
```

**Switch Tier-1 approval call** at `engine.go:728` (post-5bugs) and **preserve alternative-pair evaluation**:

The current `SimulateApproval` call only tries the primary `(opp.LongExchange, opp.ShortExchange)` pair. Pool allocator (the current primary planner) evaluates `opp.Alternatives` too via `buildAllocatorCandidates` (`allocator.go:557-633`), so Tier-1 replacing the primary planner MUST also walk alternatives — otherwise a rank-1 opp whose primary pair is not approvable but has a valid alternative pair would be lost (no fallback: Tier-1 has already selected something lower-ranked, so pool allocator fallback gate `len(selectedChoices)==0` stays closed).

Introduce a per-symbol pair walk that tracks THREE distinct outcomes so the rescue path can act on the correct candidate:

**CRITICAL**: the old call site at `engine.go:728-732` is TWO lines of control flow:
```go
approval, err := e.risk.SimulateApproval(opp, reserved)
if err != nil {
    e.log.Info("rebalance: skip %s — simulate error: %v", opp.Symbol, err)
    continue
}
```
The replacement below REPLACES BOTH — not just the approval call. The old `if err != nil { continue }` gate must be removed entirely because individual pair attempts can error without that implying the whole symbol must be skipped. Per-attempt errors are swallowed inside the pair-walk loop as local bookkeeping; only the resolved `approval` and `chosenAttempt` gate downstream behavior.

```go
// BEFORE (post-5bugs, engine.go:728-732 — REPLACE THIS ENTIRE BLOCK, including the err gate)
approval, err := e.risk.SimulateApproval(opp, reserved)
if err != nil {
    e.log.Info("rebalance: skip %s — simulate error: %v", opp.Symbol, err)
    continue
}

// AFTER — try primary first, then alternatives in alt order.
// Per-attempt errors are LOCAL (firstErr, diagnostic only); they do NOT gate the
// outer loop. That matches allocator.go's appendChoice behavior (allocator.go:570-579)
// where a pair error is logged at Debug and the loop keeps trying other attempts.
type pairAttempt struct {
    longExch  string
    shortExch string
    alt       *models.AlternativePair  // nil for primary
}
attempts := []pairAttempt{
    {opp.LongExchange, opp.ShortExchange, nil},
}
for i := range opp.Alternatives {
    a := &opp.Alternatives[i]
    attempts = append(attempts, pairAttempt{a.LongExchange, a.ShortExchange, a})
}

// Track three separate outcomes:
// - approved: first pair that passed approval (winner, no rescue needed)
// - firstCapital: first pair whose rejection is RejectionKindCapital (rescue candidate)
// - firstAny: first rejection of ANY kind (for skip log if nothing else found)
var approvedAttempt *pairAttempt
var approvedApproval *models.RiskApproval
var firstCapitalAttempt *pairAttempt
var firstCapitalApproval *models.RiskApproval
var firstAnyAttempt *pairAttempt
var firstAnyApproval *models.RiskApproval
var firstErr error   // diagnostic only; does NOT gate outer loop

for i := range attempts {
    att := attempts[i]
    if att.alt != nil {
        // Mirror pool allocator's pre-approval alt guards (allocator.go:621, :625, :629).
        if _, ok := e.exchanges[att.longExch]; !ok { continue }
        if _, ok := e.exchanges[att.shortExch]; !ok { continue }
        if att.alt.Spread <= 0 { continue }

        // CheckPairFilters takes an Opportunity and returns a rejection reason
        // (empty = passed). Build altOpp mirroring allocator.go:582-595.
        // NOTE: allocator's filter construction uses unconditional assignment;
        // entry-patching sites (engine.go:1248, scanner.go:1185) are conditional,
        // but this is the FILTER path so we match allocator.
        altOpp := opp
        altOpp.LongExchange = att.longExch
        altOpp.ShortExchange = att.shortExch
        altOpp.Spread = att.alt.Spread
        altOpp.IntervalHours = att.alt.IntervalHours
        altOpp.NextFunding = att.alt.NextFunding
        if reason := e.discovery.CheckPairFilters(altOpp); reason != "" {
            continue
        }
    }
    a, aerr := e.risk.SimulateApprovalForPair(opp, att.longExch, att.shortExch, reserved, att.alt, prefetchCache)
    if aerr != nil {
        if firstErr == nil { firstErr = aerr }
        e.log.Debug("rebalance: pair simulate error %s %s/%s: %v", opp.Symbol, att.longExch, att.shortExch, aerr)
        continue  // local bookkeeping only; keep trying remaining attempts
    }
    if a == nil {
        continue
    }
    if a.Approved {
        approvedAttempt = &attempts[i]
        approvedApproval = a
        break
    }
    if firstAnyApproval == nil {
        firstAnyAttempt = &attempts[i]
        firstAnyApproval = a
    }
    if a.Kind == models.RejectionKindCapital && firstCapitalApproval == nil {
        firstCapitalAttempt = &attempts[i]
        firstCapitalApproval = a
    }
}

// Resolve attempt outcome:
var approval *models.RiskApproval
var chosenAttempt pairAttempt
switch {
case approvedAttempt != nil:
    approval = approvedApproval
    chosenAttempt = *approvedAttempt
case firstCapitalAttempt != nil:
    // No approval, but at least one attempt was Capital-rejected — use it for donor rescue below.
    approval = firstCapitalApproval
    chosenAttempt = *firstCapitalAttempt
case firstAnyAttempt != nil:
    // All attempts failed non-Capital; skip with representative rejection for logging.
    approval = firstAnyApproval
    chosenAttempt = *firstAnyAttempt
default:
    // All attempts errored or were filtered out — nothing to act on, skip symbol.
    if firstErr != nil {
        e.log.Info("rebalance: skip %s — no viable pair attempts (firstErr=%v, all filtered or errored)", opp.Symbol, firstErr)
    } else {
        e.log.Info("rebalance: skip %s — no viable pair attempts (all filtered)", opp.Symbol)
    }
    continue
}
// Guard against nil deref before subsequent approval.Approved / approval.Kind usage:
if approval == nil {
    e.log.Info("rebalance: skip %s — no approval result after pair walk", opp.Symbol)
    continue
}
// If approval.Approved, downstream record uses chosenAttempt's long/short to
// populate selectedChoices (not opp.LongExchange/opp.ShortExchange directly).
```

**Downstream changes needed** (inside the rest of the rank-first loop at `engine.go:744-874`):
- When appending to `selectedChoices` (5bugs Bug B3 at `:896-909`), use `chosenAttempt.longExch` / `chosenAttempt.shortExch` instead of `opp.LongExchange` / `opp.ShortExchange`. Similarly for `reserved[]` keys and `plannedTransfers`.
- If `chosenAttempt.alt != nil`, the opp's `Spread`/`IntervalHours`/`Score` are overridden by the alt; `SimulateApprovalForPair` already does this internally (`manager.go:107-119`), but any log lines that reference `opp.Spread` should prefer the alt's fields for accuracy.
- Rescue path (Change 4 below) applies to `chosenAttempt`'s legs, not the primary pair's legs.
- **`altOverrideNeeded` flag** (new in v21): set a local `altOverrideNeeded := true` (initialized to `false` per rebalance pass) whenever the chosen attempt is an alternative, i.e. `chosenAttempt.alt != nil` OR `chosenAttempt.longExch != opp.LongExchange` OR `chosenAttempt.shortExch != opp.ShortExchange`. This flag is consumed by Change 5's override-storage gate so the alt selection survives even when no transfer is needed. Without this, an alt-pair selection with zero deficit would silently revert to the primary pair at the subsequent entry scan (`applyAllocatorOverrides` empty-path at `engine.go:1174-1181`). See Finding 2 in v21 Review History.

**Rationale**: this keeps "rank first" semantics — the opp at rank position N still wins over rank N+1 — while not regressing the alternative-pair search pool allocator currently does. If the primary pair for rank-1 fails but an alternative for rank-1 passes, Tier-1 picks rank-1 (alternative) before moving to rank-2. Matches user intent "rank > 排除所有風控".

`SimulateApprovalForPair` exists at `manager.go:107` and takes `(opp, longExch, shortExch, reserved, alt, cache)`. Cache is honored for both primary and alternative calls.

**Note on `e.discovery.CheckPairFilters`**: the method exists as `Scanner.CheckPairFilters(opp models.Opportunity) string` at `internal/discovery/scanner.go:776-780`, returning a rejection reason (empty string = passed). The pool allocator uses it with the same altOpp-building pattern at `allocator.go:582-595`.

**Replace `isMarginRejection`** at `engine.go:735-738`:
```go
// BEFORE
isMarginRejection := strings.Contains(approval.Reason, "insufficient margin buffer") ||
    strings.Contains(approval.Reason, "post-trade margin ratio") ||
    strings.Contains(approval.Reason, "insufficient capital")
if !isMarginRejection {
    e.log.Info("rebalance: skip %s — %s", opp.Symbol, approval.Reason)
    continue
}

// AFTER
if approval.Kind != models.RejectionKindCapital {
    e.log.Info("rebalance: skip %s — %s (kind=%s)", opp.Symbol, approval.Reason, approval.Kind)
    continue
}
```

`RejectionKindCapital` now covers the exact same set of rescueable reasons as the old substring match, plus the two `$10` notional sites that the substring match was silently missing. (That is an intentional, small behavior improvement: previously a notional-floor rejection fell into the "skip" branch; now it is considered for rescue.)

**Note on `strings` import**: remove if unused after this edit.

### Change 5: Invert primary/fallback order

Current structure (engine.go:628-950 after 5bugs):
```
rebalanceFunds() {
    ... balance snapshot, active filter ...

    if EnablePoolAllocator {
        // PRIMARY today
        allocSel = runPoolAllocator(...)
        if feasible { execute + store overrides; return }
    }

    // Sequential planner (fallback today -> will become Tier-1)
    for each opp { selectedChoices.append(...) }
    dry-run prune
    if selectedChoices == 0 { return }
    local transfer phase (mutates balances via spot->futures ratio relief)
    compute crossDeficits
    if crossDeficits == 0 { store overrides; return }
    executeRebalanceFundingPlan + store overrides
}
```

Target structure after this plan:
```
rebalanceFunds() {
    ... balance snapshot, active filter ...

    prefetchCache = buildTransferableCache(balances)

    // ---- Tier 1: Rank-First (PRIMARY) ----
    for each opp { selectedChoices.append(...) using SimulateApprovalForPair(...,cache) }
    dry-run prune

    if len(selectedChoices) == 0 {
        // Tier 1 infeasible → fall back to pool allocator.
        // Branch point MUST be here, BEFORE any balance-mutating step below.
        if !EnablePoolAllocator {
            log "tier1 infeasible, pool allocator disabled"
            log "rebalance: complete"
            return
        }
        log "tier1 infeasible, falling back to pool allocator"
        allocSel, err = runPoolAllocator(opps, balances, remainingSlots)
        if err != nil || allocSel == nil || !allocSel.feasible {
            log "rebalance: complete"
            return
        }
        // existing pool allocator execute + override store (engine.go:634-671)
        // NOTE: rebuilds its own cache; that's fine — same helper.
        log "rebalance: complete"
        return
    }

    // Tier 1 selected >=1 opp — proceed with existing post-5bugs flow.
    // Add an explicit log so the throughput tradeoff (pool allocator skipped
    // this cycle) is observable in live logs:
    log "tier1 selected N opps, skipping pool allocator fallback"

    local transfer phase (mutates balances)   // may set localTransferHappened = true
    compute crossDeficits
    if crossDeficits == 0 {
        // v21: store overrides when local transfer happened OR alt selection needs patching
        if localTransferHappened || altOverrideNeeded {
            allocOverrides = keepFundedChoices(buildChoices(selectedChoices), balances, nil)
            // existing override storage at engine.go:1077-1084 extends here
        }
        return
    }
    executeRebalanceFundingPlan
    // v21: cross-path override gate — store when any transfer occurred OR alt patch needed
    // v22/v25: pass result.FundedReceivers (matches current pushed-branch 3-arg signature)
    if localTransferHappened || result.LocalTransferHappened || result.CrossTransferHappened || altOverrideNeeded {
        allocOverrides = keepFundedChoices(buildChoices(selectedChoices), result.PostBalances, result.FundedReceivers)
        // existing override storage at engine.go:1099-1106 extends here
    }
    log "rebalance: complete"
}
```

**Override-storage gate update** (new in v21, addresses independent-review Finding 2): the existing `localTransferHappened`-only gates at `engine.go:1077-1084` (no-cross-deficit path) and `engine.go:1099-1106` (executor path) would drop Tier-1 alt-pair selections that need NO transfer. Entry scan would then consume `allocOverrides == nil` via `applyAllocatorOverrides` empty-path at `engine.go:1174-1181` and fall back to `opp.LongExchange`/`opp.ShortExchange` — silently losing the alt.

Fix: extend both gates with the new `altOverrideNeeded` flag (set in Change 4 when any selectedChoice used a non-primary pair). Stored choices run through `keepFundedChoices` which drops any unfunded entries, so storing overrides on the no-transfer path is safe even when post-state balances equal the original snapshot.

The `keepFundedChoices` helper is provided by 5bugs (dependency); its current pushed-branch contract is "drop any choice whose required margin exceeds the post-state per-leg balance, accounting for funded receivers". Real signature at `allocator.go:188`:

```go
func (e *Engine) keepFundedChoices(
    choices []allocatorChoice,
    post map[string]rebalanceBalanceInfo,
    funded map[string]float64,
) map[string]allocatorChoice
```

Takes a SLICE of `allocatorChoice` as the first argument (not a map). Returns a `map[string]allocatorChoice` keyed by symbol. `post` is `rebalanceBalanceInfo` (not `*balanceInfo`). `funded` is `map[string]float64`. `buildChoices(selectedChoices)` at `engine.go:707` has signature `func(src []sequentialChoice) []allocatorChoice` — returns a slice that feeds the first argument directly.

(Note: the 5bugs dependency plan, when merged, may extend this signature with a fourth `pending map[string]rebalancePending` parameter for submitted-but-not-confirmed cross-exchange deposit intent. This plan's Change 5 override-storage call sites should then pass `result.PendingDeposits` as the new fourth arg. Current pushed-branch state is 3-arg; v25 documents the 3-arg form so review checks pass against HEAD. At implementation time, update the call sites to whatever shape 5bugs provides.)

The `funded` argument carries the cross-exchange funded-receiver signal set by `executeRebalanceFundingPlan` on the result struct (`rebalanceExecutionResult` — note the lowercase; fields at `allocator.go:91-100`) and must be preserved in the executor-path gate.

**Argument differences between the two override-storage paths** (v22 precision, v25 types corrected against current 3-arg pushed-branch signature):
- **No-cross-deficit path** (`engine.go:1077-1084`): no executor ran → `keepFundedChoices(buildChoices(selectedChoices), balances, nil)` is correct (matches current pre-plan semantics). Here `balances` is the local `map[string]rebalanceBalanceInfo` already present in `rebalanceFunds`.
- **Executor/cross path** (`engine.go:1099-1106`): executor ran → `result.FundedReceivers` (`map[string]float64`) carries the funded-receiver signal that `keepFundedChoices` consumes → `keepFundedChoices(buildChoices(selectedChoices), result.PostBalances, result.FundedReceivers)`. `rebalanceExecutionResult` has `FundedReceivers` but no `PendingDeposits` field at pushed HEAD. When the 5bugs dependency plan merges and extends `rebalanceExecutionResult` with `PendingDeposits map[string]rebalancePending` plus extending `keepFundedChoices` with a fourth `pending` arg, update both call sites to pass that field. For an alt pair that fit under pre-state balances and needed no transfer, `result.PostBalances == balances`, `FundedReceivers` is empty, so the alt choice is retained.

**Fallback branch point inserted AFTER `engine.go:944` and BEFORE `:946`**. `:944` itself is the `rebalance: analyzed ... selected ... needs: ...` summary log line (after the dry-run prune block which spans `:915-943`). `:946` is the first line of the local-transfer ratio-relief phase. The new `if len(selectedChoices) == 0 { ... fallback ... return }` block slots between those two lines.

Balance mutation anchors downstream of the branch point (all strictly AFTER `:945`):
- local spot→futures ratio-relief mutates `balances` entries at `engine.go:984-987` (the post-transfer `bi := balances[name]; bi.futures += extra; bi.spot -= extra; bi.futuresTotal += extra; balances[name] = bi` block);
- `executeRebalanceFundingPlan` at `engine.go:1098` performs further mutations internally via its `PostBalances` alias;
- further same-exchange spot→futures transfer logic at `engine.go:1066-1069` block (still pre-executor, but it mutates `balances` entries as same-exchange relief fires).

The state that Tier-1 mutates before the `:945` branch point — `reserved`, `plannedTransfers`, `needs`, `selectedSymbols`, `selectedChoices` — is local planner state ONLY and never touches `balances`. `buildTransferableCache(balances)` reads balances but does not mutate. Pool allocator fallback therefore receives the untouched `balances` snapshot that Tier-1 originally saw.

### Change 6: Remove `EnablePoolAllocator` guard around current pool-allocator block

The current pool-allocator block at `engine.go:628-675` is wrapped in `if e.cfg.EnablePoolAllocator { ... }`. After the inversion, that block becomes the body of the fallback branch. Remove the redundant wrapper since the outer fallback branch already gates on `EnablePoolAllocator`.

### Change 7: RotateScan interaction — no dispatcher change

`engine.go:1348-1355` already gates `rebalanceFunds()` call from RotateScan on `EnablePoolAllocator=true`. After this plan:
- `EnablePoolAllocator=true`: RotateScan → `rebalanceFunds` → Tier-1 runs first; pool allocator fallback if needed.
- `EnablePoolAllocator=false`: RotateScan does NOT call `rebalanceFunds` (unchanged).

**No dispatcher change.** This is the existing intended behavior — rotate-scan rebalance exists for value-optimization opportunities only. When pool allocator is disabled, the only planned rebalance is the dedicated rebalance-scan minute.

Add a clarifying comment at `engine.go:1352`:
```go
// NOTE: When EnablePoolAllocator=false, rotate-scan intentionally skips
// rebalance; only the dedicated rebalance-scan minute triggers it.
// Tier-1 rank-first is invoked inside rebalanceFunds regardless.
```

## Asymmetric per-leg margin — explicit conservative stance

`approval.LongMarginNeeded` and `approval.ShortMarginNeeded` can differ by the long/short mid-price gap (typically <1% for delta-neutral perp-perp pairs). 5bugs Bug B tracks them asymmetrically on `sequentialChoice.longNeed/shortNeed` for local planner bookkeeping, but `buildChoices()` maps each `sequentialChoice` to `allocatorChoice{requiredMargin: approval.RequiredMargin}` — where `RequiredMargin = max(LongMarginNeeded, ShortMarginNeeded)` per `manager.go:715`.

Downstream, `dryRunTransferPlan` (`allocator.go:936-940`) and `canReserveAllocatorChoice` (`allocator.go:181-200`) both reserve `requiredMargin` symmetrically on BOTH legs. This means the short leg receives a reservation slightly larger than its actual need when `long_margin > short_margin`.

**Impact**: in rare edge cases (e.g., one exchange has exactly enough for short_margin but not long_margin, cross-exchange transfer plan relies on that exact margin), a feasible rank-ordered selection can be conservatively dropped. In practice for perp-perp delta-neutral, the asymmetry is <1% and this drop is extremely unlikely.

**Decision**: accept the conservative symmetric replay as a carry-forward of existing semantics. Do NOT extend `allocatorChoice` with per-leg fields in this plan. Rationale:
- Changing `allocatorChoice` touches `dryRunTransferPlan` (`allocator.go:939`), `canReserveAllocatorChoice` (`allocator.go:181-200`), `reserveAllocatorChoice` (`allocator.go:212`), `keepFundedChoices`, and the pool allocator's solver — cross-cutting risk disproportionate to the win.
- For delta-neutral perp-perp pairs, the per-leg asymmetry is typically <1%, making conservative drops rare in practice.

**Fallback is not guaranteed-equivalent recovery**: if Tier-1 conservatively drops a choice due to symmetric over-reservation, the pool allocator fallback uses the SAME symmetric `requiredMargin` replay path (`allocator.go:181, 212, 939`) and for the SAME `allocatorChoice` applies the same feasibility gate. Fallback therefore does NOT "self-heal" the exact same choice. However, pool allocator separately evaluates `opp.Alternatives` (`allocator.go:617-618`) and re-sorts the full choice set by `baseValue` (`:639-652, :688-695`), so the fallback can still select a DIFFERENT feasible choice — an alternative pair, a lower-rank bundle whose totals fit inside `balances` under the symmetric gate. This is an accepted carry-forward of the symmetric contract with the clarification that fallback may produce a different selection than Tier-1 considered, not that it never produces one.

**Explicit follow-up marker**: if live logs after this plan ships show ≥5 events/week of Tier-1 dropping choices that pool allocator would also drop but a hypothetical asymmetric planner would have kept, open a separate per-leg extension plan to extend `allocatorChoice` with `longMargin/shortMargin` and thread them through all three downstream call sites. This extension explicitly changes behavior in BOTH Tier-1 and pool allocator simultaneously.

## Tests

### `internal/risk/manager_test.go` — create new file
Table-driven test with 22 rows, one per rejection site. Each row:
- input: a minimal opportunity + exchange setup that triggers the target site
- expected: `approval.Approved == false && approval.Kind == expected`

Additionally: an approved-case row asserts `Kind == RejectionKindNone`.

### `internal/engine/engine_rank_first_test.go` — new file, 20 scenarios (1–13, 13b, 14–19)

1. **rank-1 approved, rank-2 approved**: Tier-1 selects both, no transfers, pool allocator NOT invoked.
2. **rank-1 Capital-rejected but rescueable via donor transfer, rank-2 approved**: Tier-1 selects both; rank-1 via `plannedTransfers`; pool allocator NOT invoked.
3. **rank-1 Spread-rejected, rank-2 approved**: Tier-1 selects only rank-2 (rank-1 skipped without rescue attempt); pool allocator NOT invoked.
4. **rank-1 post-trade-ratio-rejected, prefetchCache has sufficient `TransferablePerExchange`** (fixed-capital mode, `CapitalPerLeg > 0` required so `manager.go:298-374` top-up branch runs): `SimulateApprovalForPair(...,cache)` top-up cures the rejection; Tier-1 selects rank-1; pool allocator NOT invoked.
5. **rank-1 post-trade-ratio-rejected, prefetchCache insufficient but live donor can satisfy deficit**: Tier-1 falls back to donor rescue path (same block as buffer rejection); succeeds; pool allocator NOT invoked.
6. **all rank opps Health-rejected or Spread-rejected**: Tier-1 produces 0 selectedChoices; fallback gate fires; pool allocator invoked with untouched `balances`.
7. **all rank opps Capital-rejected with zero donor surplus**: Tier-1 produces 0; fallback gate fires; pool allocator invoked.
8. **EnablePoolAllocator=false, rank-1 approved**: Tier-1 selects rank-1; no fallback; behavior identical to pre-plan sequential-only path.
9. **EnablePoolAllocator=false, all rank opps rejected**: Tier-1 empty; return without fallback; no pool allocator invocation.
10. **RotateScan with EnablePoolAllocator=false**: assert dispatcher at `engine.go:1352-1355` does NOT call `rebalanceFunds` — pre-existing behavior preserved (regression guard).
11. **Symmetric replay under asymmetric approval** (regression guard for the conservative stance): long_margin=50, short_margin=48 → `buildChoices` stores `requiredMargin=50`; `dryRunTransferPlan` reserves 50 on both legs. Fixture setup: the short leg exchange has `total=48` (exactly enough for short_margin=48 but insufficient for the symmetric 50 reservation), no donor headroom and no `TransferablePerExchange` entry (so neither dry-run top-up nor live donor rescue masks the effect). Assert the choice is dropped by the symmetric feasibility gate at `allocator.go:181-200` / `:936-940`, and the drop is logged. **This is not a "passes" test — it pins the current conservative semantic under conditions where the drop is unambiguously driven by `requiredMargin` over-reservation (not by missing funding setup), so a later follow-up plan can track if live logs show excessive drops.**
12. **Donor double-count across rescues**: rank-1 and rank-2 both rescue from the same donor; second rescue sees `plannedTransfers[donor+"_out"]` from first; total out ≤ donor surplus. Covered by 5bugs Bug B v4 but re-asserted here under rank-first gating.
13. **Notional-floor rescue via cache top-up** (new in v6, fixture tightened in v17 per independent review Finding 3, leverage-clamp constraint added in v20): opp whose first-pass sizing would land below `$10` notional (`manager.go:438` / `:503`) with sufficient `TransferablePerExchange` — `SimulateApprovalForPair(...,cache)` top-up raises `Available`/`Total` at `:367-368`, sizing at `:427` produces size ≥ `$10` notional; Tier-1 selects; pool allocator NOT invoked. Asserts `approval.Kind == RejectionKindCapital` on the first-pass rejection (precondition for the rescue branch). **Fixture requirements**: `cfg.CapitalPerLeg > 0` AND `cfg.Leverage <= MaxLeverage()` (so `leverage == cfg.Leverage` after the clamp at `approveInternal`, keeping fixture-level predicates unambiguous) AND `cfg.CapitalPerLeg * cfg.Leverage ≥ 10` (equivalently, under the clamp constraint, `effectiveCap * leverage ≥ 10` — this is the condition the classifier actually evaluates with its single-snapshot locals). The cache top-up branch at `manager.go:298` is gated by `needed > 0` where `needed` comes from `effectiveCapitalPerLeg` (`:265`, `:54-61`) — so the cache-top-up path ONLY fires when the effective cap is positive. Shared-capital mode (`cfg.CapitalPerLeg == 0` with no allocator-derived cap) would skip the cache top-up branch entirely and exercise a different code path (donor rescue via `plannedTransfers` outside `SimulateApprovalForPair`); that path is covered separately by Test 5 (post-trade-ratio with insufficient cache but viable live donor) and Test 16 (alt-pair Capital rescue via donor). Keep Test 13 focused on the cache-top-up path to avoid muddling branches.
13b. **Notional-floor UNCURABLE under per-leg ceiling** (new in v13, helper signature corrected in v14, snippet scope fixed in v15, token normalized in v16, fixture realized in v17 per independent review Finding 3, leverage-clamp constraint added in v20): configure a real `CapitalAllocator` (no stubbing — `Manager.allocator` is a concrete `*CapitalAllocator`, not an interface; `manager.go:29-34, :40`) with `cfg` tuned so that `m.effectiveCapitalPerLeg(string(StrategyPerpPerp)) * leverage < 10` (where `leverage` is the clamped single-snapshot local; under the fixture precondition below, `leverage == cfg.Leverage`). Two sub-cases, BOTH using a real allocator:
  - **Sub-case (a) — fixed-capital**: `cfg.CapitalPerLeg > 0` AND `cfg.Leverage <= MaxLeverage()` (keeps `leverage == cfg.Leverage` after the clamp) AND `cfg.CapitalPerLeg * cfg.Leverage < 10` (e.g., `CapitalPerLeg=5, Leverage=1` — the manual-override short-circuit at `allocator.go:750-753` returns `5.0` directly). No unified-capital config needed.
  - **Sub-case (b) — unified-capital derived**: `cfg.CapitalPerLeg == 0`, `cfg.EnableUnifiedCapital == true`, `cfg.Leverage <= MaxLeverage()` (so `leverage == cfg.Leverage` after the clamp), and `cfg.TotalCapitalUSDT`, `cfg.MaxPositions`, `cfg.SizeMultiplier`, `cfg.Leverage` tuned so the allocator's real formula at `allocator.go:760-779` yields a small per-leg cap. The exact derivation (from reading `allocator.go:746-779`) is `derived = TotalCapitalUSDT / MaxPositions / 2.0`, then `derived *= SizeMultiplier` (default 1.0 if ≤0), then optionally multiplied by `dynamicCaps[strategy]` (a pct in range 0-1 if configured via `DynamicStrategyPct`). NO active-position count input. Concrete example: `TotalCapitalUSDT=10, MaxPositions=5, SizeMultiplier=1, Leverage=1` → `derived = 10/5/2 = 1.0` → `1.0 * 1 (Leverage) = 1 < 10` → `RejectionKindConfig` is correct. Test should NOT use `dynamicCaps` in the baseline row (to isolate the derivation); an additional row may exercise the `dynamicCaps` multiplier if desired. Verify pre-condition: under the clamp constraint, compute `effectiveCap := m.effectiveCapitalPerLeg(string(StrategyPerpPerp))` and assert `effectiveCap * float64(cfg.Leverage) < 10` (equivalent to `effectiveCap * leverage < 10` since `leverage == cfg.Leverage`).
  
  First-pass sizing at `manager.go:876-878` fixes `maxPositionValue = ecl2 * leverage < 10` (under the v17 plumbing, `ecl2` is the passed-in `effectiveCap`; same value), which the balance cap at `:879-881` can only lower. Notional rejection fires at `:438` / `:503`. Assert `approval.Kind == RejectionKindConfig` (NOT `Capital`); assert Tier-1 does NOT attempt donor rescue (the Capital branch is skipped); assert the opp is sequentially skipped. Both sub-cases guard against the v13 bug where the helper only checked `cfg.CapitalPerLeg`. This pins the v14 taxonomy correction preventing futile rescue attempts in uncurable ceiling regimes regardless of capital mode.
14. **Shared-cache overcommit prune** (new in v6): two ranked opps both depend on the same `TransferablePerExchange` headroom for approval (cache top-up lets both pass `SimulateApprovalForPair` independently, but only one can actually be funded because the donor surplus is shared). Dry-run prune at `engine.go:915-943` must drop the later-ranked one; assert rank-1 is kept, rank-2 is pruned, and the log records the drop. Covers the new Tier-1 cache + prune interaction.
15. **Alternative-pair coverage** (new in v8, extended in v21 per independent-review Finding 2): rank-1 opp whose primary pair fails (e.g., `bybit/gateio` — primary leg has exchange-health issue) but an alternative pair `(okx, binance)` in `opp.Alternatives` passes approval; rank-2 opp is also approvable on its primary. Assert (A) Tier-1 selects rank-1's ALTERNATIVE pair (not rank-2, not nothing); `selectedChoices[0].longExchange/shortExchange` match the alt, not the primary; (B) `altOverrideNeeded` is set to `true` inside the rebalance loop; (C) pool allocator NOT invoked. **Sub-case 15a — no transfer needed**: alt pair fits under existing balances without any donor transfer (both legs already have `total >= requiredMargin`). Assert `allocOverrides` is populated (stored via the v21 gate extension at `engine.go:1077-1084` / `:1099-1106`) and that `applyAllocatorOverrides` patches the subsequent entry-scan execution to use alt legs, not primary legs. Guards against the v21-discovered semantic gap where a zero-transfer alt selection would silently revert to the primary pair at entry. **Sub-case 15b — transfer needed**: pre-existing v8 coverage (alt pair requires donor rescue). Both transfer gates AND `altOverrideNeeded` are true; assert override storage and alt entry patch still work. Guards against the alternative-pair regression that v7 was blind to.
16. **Alt-pair Capital rescue with primary non-Capital rejection** (new in v11): rank-1 opp's primary pair rejected `Spread` (slippage too high); rank-1's alternative rejected `Capital` (insufficient margin buffer) with a viable donor; rank-2 opp approvable on primary. Assert Tier-1 picks rank-1 ALT via donor rescue (`plannedTransfers` records the transfer; `selectedChoices[0]` uses alt legs). Guards against the v10 bug where the pair walk remembered only the first rejection and would have skipped rank-1 entirely.
17. **All pair attempts filtered or errored — nil-approval guard** (new in v11): rank-1 opp with primary pair where `SimulateApprovalForPair` returns `(nil, err)` and every alternative is filtered out by `CheckPairFilters`. Assert Tier-1 logs `no viable pair attempts` and proceeds to rank-2 without panicking on nil dereference.
18. **Alt-pair with zero NextFunding not inheriting primary's window** (new in v11): primary pair has far-future `NextFunding`; alternative has `NextFunding = time.Time{}` (zero). Assert that `CheckPairFilters(altOpp)` sees the alt's zero NextFunding (not the primary's), and the funding-window gate at `scanner.go:792-795` behaves accordingly. Catches the v10-discovered NextFunding-inheritance risk.
19. **Primary pair errors, alternative succeeds** (new in v12): `SimulateApprovalForPair(opp, primary, reserved, nil, cache)` returns `(nil, someErr)` for the primary pair (e.g., orderbook fetch transient failure); an alternative pair approves cleanly. Assert Tier-1 uses the alternative (not skip symbol because of the primary error). Guards against the v11 bug where per-attempt errors poisoned the outer loop's `err` gate and incorrectly skipped symbols that had a later recoverable attempt.

### Parity test — `internal/engine/allocator_transferable_parity_test.go` (new)
Assert `buildTransferableCache(balances)` produces exactly the `TransferablePerExchange` map that the current inline pool-allocator block produces, for a fixture set of `balances`. Guards against accidental divergence during the extract.

### Integration replay
If a post-v0.32.24 live log exists where pool allocator demoted rank-1 (ranked by discovery score) to rank-3 due to bundled `baseValue`, replay it and assert Tier-1 selects rank-1. If no such log exists at implementation time, construct a deterministic fixture with 3 opps where rank-1 funding_rate is highest but rank-2 + rank-3 bundled `baseValue` is higher.

## Risks

| Risk | Mitigation |
|---|---|
| Extracting `buildTransferableCache` silently diverges from allocator semantics | Parity test above. |
| `Kind` field typo or missing annotation at a rejection site | 22-row table test in manager_test.go pins every site. |
| Taxonomy approximation at `:430/:438/:503`: sizing can return 0 for step/min-size/short-price/invalid-price reasons, but these are classified `Capital` when `effectiveCap * leverage ≥ 10`. | Direction of misclassification is always `false-Capital` → futile donor rescue (one wasted iteration, no correctness bug). `false-Config` is mathematically impossible under the chosen condition. Explicit follow-up marker (Change 2 "Known approximation" paragraph): if live logs show >5 futile rescues/week, open sizing-reason extension plan to return structured reject causes from `calculateSizeWithPrice`. |
| `calculateSizeWithPrice` signature change (added `effectiveCap float64` and `leverage int` parameters) could break other callers | Enumerate all callers via `grep -n calculateSizeWithPrice internal/risk/` during implementation (current tree has only one direct caller at `manager.go:427`) and update each; reviewer must verify no caller is left passing the old signature. Parity test: Test 13/13b pre-condition assertion verifies both `effectiveCap` and `leverage` reach sizing via the new parameters (not via internal re-derivation). |
| Concurrent `dynamicCaps` update or future leverage-clamp change could yield different values between sizing and classification, flipping `Kind` | v17/v19 eliminate re-derivation by computing BOTH `effectiveCap` and `leverage` (clamped) ONCE in `approveInternal` and plumbing them as parameters. Classification and sizing share the same snapshot for both. |
| **Alt-pair selection without transfer silently reverts to primary pair at entry** (v21 independent-review Finding 2) | Current sequential-planner override-storage gates at `engine.go:1077-1084` and `:1099-1106` fire only when a transfer occurred; an approved alt pair that fit under existing balances would leave `allocOverrides == nil` and `applyAllocatorOverrides` at `:1174-1181` empty-path would restore the opp's primary pair. v21 adds `altOverrideNeeded` flag set in Change 4 (when any selectedChoice uses a non-primary pair), consumed by Change 5's extended override-storage gate. `keepFundedChoices` drops any unfunded entries, so storing overrides on the no-transfer path is safe even when post-state balances equal the pre-state snapshot. Test 15 sub-case 15a pins the no-transfer-alt path end-to-end. |
| Conservative symmetric replay may drop rank-ordered selections due to per-leg over-reservation | Explicit carry-forward of existing semantics: for the SAME `allocatorChoice`, pool allocator fallback uses the SAME symmetric `requiredMargin` replay path (`allocator.go:181, 212, 939`) and applies the same feasibility gate. However, fallback is NOT guaranteed-equivalent — pool allocator evaluates `opp.Alternatives` at `allocator.go:617-618` and sorts bundles by `baseValue` at `:639-652, :688-695`, so it CAN still pick a different feasible choice (an alternative pair, a lower-rank bundle) that Tier-1 rejected under the symmetric gate. Fallback is therefore "may self-heal into a different choice", not "guaranteed recovery of the same choice". Follow-up marker present; if live logs show Tier-1 drops that fallback also fails to cover, open a separate per-leg extension plan that updates BOTH planners simultaneously. |
| Pool allocator builds its own cache via extracted helper — if merge reorders so allocator inline is still present alongside helper, double cache construction | Reviewer must ensure the inline block is deleted from `runPoolAllocator` when the helper is added. |
| RotateScan gate is pre-existing; users may expect rebalance to fire regardless | Pre-existing; add the clarifying comment in Change 7. |
| `SimulateApprovalForPair` with `alt=nil` has never been exercised from this call site | Unit test 4 covers it. |
| 5bugs not merged at implementation time | Implementation halts per Dependencies section. |
| Throughput regression: once Tier-1 keeps even ONE ranked choice, pool allocator is never invoked this cycle — it will not backfill remaining position slots with lower-ranked positive-value bundles that the allocator would have picked | Accepted by design per user intent ("rank-first, allocator only when rank produces zero"). Total deployed capital / aggregate `baseValue` per cycle may be lower than current pool-allocator-first behavior when feasible bundle value > rank-ordered top-N value. Explicit log line `tier1 selected N opps, skipping pool allocator fallback` added to Change 5's success path makes this observable so live data can quantify the tradeoff post-merge. |

## Rollout

- **Version**: next sequence after 5bugs + margin-unify merge. Current `VERSION` file at repo root reads `0.32.27`; this plan ships as whatever version comes after both dependencies merge — likely `v0.32.29` or `v0.32.30` depending on dependency merge order. Implementer bumps both `VERSION` and `CHANGELOG.md` at commit time.
- **CHANGELOG**: "rebalance: rank-first sequential planner is now primary; pool allocator moved to fallback when no rank-ordered opportunity is feasible".
- **No new config flag** in this plan. If rank-first causes a regression in live logs, a follow-up plan can add an `EnableRankFirstRebalance` toggle.
- **Rollback**: single-commit revert restores pool-allocator-first.

## Out of Scope

- Extending `allocatorChoice` with per-leg margin fields (see conservative-stance section; deferred to a separate follow-up plan if live logs show excessive drops).
- Changing donor gating formulas (owned by margin-unify).
- Changing discovery/ranking.
- Spot-futures engine.
- Frontend/i18n.
- Deduplicating :35 rotate + :45 rebalance capital movement (deferred).
