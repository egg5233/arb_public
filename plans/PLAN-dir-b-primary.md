# PLAN: Dir B Primary Strategy (Feature-Toggle Pivot)

Version: v1
Date: 2026-04-24
Status: DRAFT (meta-plan — coordinates sub-plans)

## Goal

Shift the bot's primary strategy from perp-perp (funding-rate spread across two
perps) to Dir B (buy_spot_short — buy spot, short futures, collect positive
funding on the perp leg). Perp-perp becomes fallback. Behavior is gated behind
a single config toggle so production can migrate or revert without code
deploys.

### Rationale (user)

Dir B has higher typical profit rate than perp-perp:
- Dir B earns absolute funding (positive funding paid to shorts) net of taker
  fees; no rotation cost, no cross-exchange transfer cost for the spot leg.
- Perp-perp earns the differential (short-leg funding minus long-leg funding)
  net of 2× taker fees plus rotation costs when the cheap-funding leg shifts
  to a different exchange.
- When funding is extreme on any one exchange, Dir B captures it directly;
  perp-perp only captures the part that exceeds the other exchange's funding.

### Constraints

- Live system — no regression for existing perp-perp trading when toggle is
  OFF (per `feedback_feature_toggle.md`).
- Dashboard-controllable toggle — no restart required to change mode.
- All new features default OFF.
- Sub-plans must each pass the standard review loop before implementation.

## The Toggle

### Config field

```go
// In internal/config/config.go
type StrategyPriority string

const (
    StrategyPriorityPerpPerpFirst  StrategyPriority = "perp_perp_first"   // current (default)
    StrategyPriorityDirBFirst      StrategyPriority = "dir_b_first"       // new target
    StrategyPriorityDirBOnly       StrategyPriority = "dir_b_only"        // monitor-only for perp-perp
    StrategyPriorityPerpPerpOnly   StrategyPriority = "perp_perp_only"    // monitor-only for spot-futures
)

type Config struct {
    ...
    StrategyPriority StrategyPriority `json:"strategy_priority"` // default "perp_perp_first"
}
```

### Semantics

| Value | perp-perp entry | spot-futures entry | Capital allocation | Monitor existing |
|-------|-----------------|-------------------|---------------------|------------------|
| `perp_perp_first` (default) | normal | after perp-perp | perp-perp priority | both |
| `dir_b_first`   | runs AFTER Dir B discovery and EV gate | runs FIRST in scan | Dir B priority | both |
| `dir_b_only`    | no new entries (existing positions managed) | runs normally | Dir B only | both |
| `perp_perp_only`| runs normally | no new entries | perp-perp only | both |

### Runtime effects

- Scan schedule unchanged — only the *handler dispatch order* changes within
  a scan tick.
- Existing positions are always monitored/exited by their original engine,
  regardless of toggle.
- Toggle changes apply at the next scan cycle (no in-flight entry is
  preempted).

### Dashboard

- Single dropdown in Config page: `Strategy Priority: [perp_perp_first ▾]`
- Readout on Overview: "Primary: Dir B · Fallback: Perp-Perp" (or mirror).
- Per-strategy KPI split: active positions count, realized PnL today,
  funding collected, capital deployed — existing cards plus a "primary"
  badge on the currently-prioritized row.

## Sub-Plans (execution order + dependencies)

Each sub-plan is its own file in `plans/`. Review loop per CLAUDE.md
plan-first rule applies to each.

### Step 1 — Config toggle scaffolding (no behavior change)
- File: `plans/PLAN-dir-b-toggle-scaffold.md` (new)
- Adds `StrategyPriority` field, JSON + env + defaults, Redis persistence,
  dashboard dropdown, Config handler validation.
- Behavior: no code path reads the value yet. Safe to merge + deploy.
- Acceptance: dropdown flips the Redis value; config reload preserves it.
- Depends on: nothing.

### Step 2 — Capital Maximizer (prereq)
- File: `plans/PLAN-capital-maximizer.md` (exists, v12 ALL PASS, pending
  implementation).
- Unblocks donor-leg-conflict rejection so allocator can actually move
  capital to Dir B's spot accounts.
- Behavior: allocator efficiency change; both engines benefit.
- Depends on: nothing.

### Step 3 — Cross-engine EV comparison gate
- File: `plans/PLAN-cross-engine-ev-gate.md` (new)
- When both engines have opportunities on the same `exchange:symbol` OR when
  capital is constrained:
  - Compute `EV per USDT per hour` for each candidate:
    - Dir B: `funding_bps_per_hour × (1 - taker_fee_bps/1e4 × 2)` (1 spot buy
      + 1 futures open at entry; spot sell + futures close at exit, averaged
      over hold time via config-supplied expected hold hours).
    - Perp-perp: `spread_bps_per_hour × (1 - taker_fee_bps/1e4 × 2)` using
      spread = short_funding - long_funding.
  - When `StrategyPriority=dir_b_first`, allow Dir B entry first; then
    compare remaining perp-perp candidates against remaining unallocated
    Dir B EV and only admit perp-perp if its EV exceeds Dir B's marginal
    EV (else block).
  - When `perp_perp_first`, mirror.
- Reads `StrategyPriority` via config.
- Depends on: Step 1 (toggle), Step 2 (capital accounting consistency).

### Step 4 — Scheduler dispatch order
- File: `plans/PLAN-dir-b-scheduler-order.md` (new)
- Changes the scan handler so spot-futures discovery/entry fires before
  perp-perp entry when `dir_b_first` is active.
- Affects `internal/engine/engine.go`'s scan handlers at the minute
  classifications (per memory `project_scan_schedule.md`: real scan minutes
  are :35 rebalance+discovery, :45 entry open).
- Depends on: Step 1 (toggle), Step 3 (EV gate).

### Step 5 — BingX Dir B spot leg (new SpotExchange interface)
- File: `plans/PLAN-bingx-dir-b-spot-leg.md` (new)
- Per memory `project_bingx_dirb_spot_leg.md`.
- New `SpotExchange` interface (Buy/Sell/QueryFill/Balance/Fees, no
  Borrow/Repay).
- BingX adapter: `pkg/exchange/bingx/spot.go`.
- spotengine capability routing: choose margin or non-margin path per
  exchange capability.
- Discovery expansion: include BingX as valid Dir B spot leg when the
  chosen target has regular spot trading.
- Depends on: independent of toggle — but Dir B primary mode benefits
  directly, so deploy after Step 4 so the new capacity actually gets used.

### Step 6 — 1h funding exit redesign
- File: `plans/PLAN-1h-exit-redesign.md` (new; see memory
  `project_1h_exit_design.md` for agreed design from Claude+Codex
  discussion 2026-04-02).
- Implements rolling reversal ratio + asymmetric settlement skip
  (T-3m to T+5m for 1h) + funding-loss hard stop.
- Critical for Dir B primary because most extreme-funding Dir B
  candidates are on 1h cycles — today those get stuck.
- Depends on: independent; can ship before or after Step 4.

### Step 7 — Transfer bugs v8
- File: `plans/PLAN-transfer-bugs.md` (exists, at v7, needs v8 per memory
  `project_transfer_bugs.md`).
- Four-location deficit formula mismatch + no min-withdraw check.
- Prereq for efficient Dir B-primary capital movement (Dir B needs USDT
  on spot/margin accounts, not futures).
- Depends on: Step 2 (Capital Maximizer) — same subsystem.

### Step 8 — Stats race in reconcile (post-egg-fix)
- File: `plans/PLAN-reconcile-stats-race.md` (new)
- Per round-7 independent Codex finding: `needsPnLUpdate` and `statsDiff`
  use stale `oldPnL = pos.RealizedPnL`. When rotation path updates DB
  RealizedPnL before close retry fires, close retry's `AdjustPnL` double-
  adjusts stats.
- Fix: re-read `pos.RealizedPnL` (and RotationPnL) at top of
  `tryReconcilePnL`, use appliedReconciledPnL-from-predicate for statsDiff.
- Depends on: nothing. Can ship independently.

### Step 9 — Dashboard strategy view
- File: `plans/PLAN-strategy-dashboard.md` (new)
- Overview page shows current strategy priority, per-strategy capital and
  PnL split, toggle history (last 5 changes with timestamp).
- Opportunities page sorts by EV-adjusted score with strategy tag.
- Depends on: Step 1 (toggle field) + Step 3 (EV computation).

## Implementation order summary

| Order | Sub-plan | Can start when | User-visible change |
|-------|----------|----------------|---------------------|
| 1 | Toggle scaffold | Now | Dropdown visible, no-op |
| 2 | Capital Maximizer | Now (parallel) | Allocator more efficient |
| 3 | Cross-engine EV gate | After 1 & 2 | EV comparison logged, no gating yet |
| 4 | Scheduler order | After 3 | Dir B runs first when toggle=dir_b_first |
| 5 | BingX spot leg | After 4 (or parallel) | BingX appears in Dir B opportunities |
| 6 | 1h exit redesign | Any time | 1h Dir B positions exit cleanly |
| 7 | Transfer v8 | After 2 | Allocator can push capital to spot accounts |
| 8 | Stats race | Any time | Stats row matches position row post-rotation |
| 9 | Dashboard | After 1 & 3 | Strategy-aware UI |

The toggle flip from `perp_perp_first` → `dir_b_first` becomes safe
**after Step 4**. Before that, the toggle is cosmetic (Steps 1-3 land but
don't gate entry ordering).

## Rollout

### Phase A (Steps 1-2)
- Merge & deploy. No behavior change. Users can see the dropdown but
  flipping it does nothing user-visible yet.
- Observe for 1 full cycle.

### Phase B (Steps 3-4)
- Merge & deploy with toggle still defaulting to `perp_perp_first`. EV gate
  and scheduler order now respect the toggle.
- Manually flip toggle to `dir_b_first` on VPS → observe ≥ 2 full scan
  cycles → revert if unexpected.
- Once stable, user decides when to leave toggle on `dir_b_first`
  permanently.

### Phase C (Steps 5-7)
- Ship expansion (BingX spot, 1h exit, Transfer v8).
- Each can deploy independently.

### Phase D (Steps 8-9)
- Ship reconcile stats fix and dashboard enhancements.
- No change in trading behavior.

## Risk assessment

- **Default off** — every sub-plan's feature toggle defaults to current
  behavior (`perp_perp_first`). Blast radius capped.
- **Independent revertability** — toggle flip is Redis-only; no rebuild.
- **Review loop** — each sub-plan goes through standard review (normal
  loop to PASS, then independent review) before implementation.
- **Monitoring** — each sub-plan must specify what to observe on VPS
  post-deploy (logs, dashboard metrics) and a rollback trigger.

## Acceptance for the meta-plan

- All 9 sub-plans exist in `plans/` and reach ALL PASS before their
  implementation merges.
- After Phase B ships, the toggle can be flipped between
  `perp_perp_first` and `dir_b_first` in production and the bot's
  strategy priority changes at the next scan cycle without restart.
- No regression in existing perp-perp positions across the entire
  rollout (verified by per-cycle PnL parity against the pre-Step-1
  baseline when toggle=`perp_perp_first`).

## Out of scope (for this meta-plan)

- Cross-exchange Dir A (`project_cross_exchange_dira.md`) — deferred per
  existing memory note.
- Real-time unrealized PnL display (`project_pnl_realtime_todo.md`) —
  UX improvement, not required for the strategy pivot.
- FFUSDT orphan close bug (`project_pending_issues.md`) — separate
  investigation.
- Binance reconcile Tier 1 per-leg skip — already fixed in HEAD by
  `CloseSizeUnknown` mechanism (egg commit e59be3d4); VPS needs deploy,
  not code work.
