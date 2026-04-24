# PLAN: Dir B Primary Strategy (Feature-Toggle Pivot)

Version: v2
Date: 2026-04-24
Status: DRAFT (meta-plan — coordinates sub-plans)

## Changelog
- **v2** (2026-04-24): Codex review round 1 — NEEDS-REVISION 9 blockers. Fixed:
  - **#1 Dir A vs Dir B not distinguished**: spot engine discovers BOTH
    `borrow_sell_long` (Dir A) and `buy_spot_short` (Dir B). v1's toggle
    conflated them. v2 redefines the toggle surface to be **direction-
    aware** — the primary slot is Dir B, Dir A explicitly secondary
    (existing-positions-only when toggle=`dir_b_first`). See new `Toggle
    Semantics (v2)` table.
  - **#2 "Next scan cycle" not implementable**: spot engine has its own
    `discoveryLoop` ticker (`spotengine/engine.go:168-249`), perp engine
    has its own scan tick (`engine/engine.go:1003+`). Mid-flip can be seen
    at different boundaries by each. v2 introduces a **shared
    `StrategyEpoch` atomic** (uint64 counter) advanced on each config
    reload. Each engine snapshots the epoch + mode at tick start and
    operates against that frozen view for the duration of the tick. This
    replaces the fragile "next cycle" language.
  - **#3 Step 4 scoped against wrong dispatcher**: v1 said reorder inside
    `engine/engine.go`, but spot-futures runs from its own ticker. v2
    replaces Step 4 with **Step 4a (Coordinator seam)** and **Step 4b
    (dispatch order within each engine via Coordinator)**. The
    Coordinator is a thin struct holding strategy-epoch + per-cycle
    winner reservations; both engines consult it rather than calling each
    other. This preserves engine independence while giving both a
    deterministic priority view.
  - **#4 Step 3 self-contradiction + EV formula thin**: v1 body said
    "blocks lower EV", summary said "logs, no gating". v2 splits Step 3
    into **Step 3a (EV computation + logging only, no gating)** and
    **Step 3b (EV-based gating)**. Step 3a ships first for observation;
    Step 3b ships after Coordinator (Step 4a). v2 writes the explicit EV
    formula with hold-hours normalization, asymmetric spot-vs-futures
    fees, and rotation-cost amortization. Dir A gets its own formula
    (includes borrow APR cost).
  - **#5 Capital allocation prereqs underspecified**: shared allocator
    is optional today (`EnableCapitalAllocator`). v2 explicitly requires
    `EnableCapitalAllocator=true` before Dir B-primary capital effects
    apply; when OFF, toggle still affects discovery + entry ordering,
    but capital remains per-engine siloed. Behavior documented in the
    semantics table.
  - **#6 Cross-engine hard blocks preservation**: existing hard-blocks
    (PP's `filterArgmaxOpps` excluding SF-occupied slots; SF's
    `crossEngineBlocked` at `risk_gate.go:47`; consolidator cross-engine
    mismatch checks) v2 explicitly lists these as **post-open safety
    guards to preserve**, not invert. Step 3b adds a pre-entry
    coordinator-based winner-selection rule on TOP of the existing
    post-open blocks.
  - **#7 Config persistence — config.json not Redis**: per
    `CLAUDE.md:29` "Do not modify config.json" and
    `doc/../AGENTS.md` config contract, the toggle value lives in
    `config.json` with dashboard-handler writeback. v2 corrects Step 1
    scaffold wording.
  - **#8 BingX Dir B under-scoped**: spot engine indexes only
    `SpotMarginExchange` (`spotengine/engine.go:80-90`), discovery uses
    `e.spotMargin`, risk gate prices via `spotMargin`, execution
    rejects non-margin. v2 expands Step 5 to 6 explicit sub-tasks
    covering engine init, discovery dual-index, risk gate fork,
    execution fork, close fork, and SpotOrderRules/fee plumbing.
  - **#9 Acceptance not verifiable**: "PnL parity" is market-dependent.
    v2 replaces with **deterministic signals**: identical candidate-
    ordering logs under toggle=OFF (before/after diff = empty), explicit
    winner-selection log line `[coordinator] cycle=N chose=dir_b
    symbol=X ev_net=1.23 bps/h` visible on every cycle, cycle-boundary
    log line `[coordinator] cycle=N started epoch=M strategy_priority=X`
    visible at each engine's tick start, rollback trigger = any cycle
    where PP opens a position with EV < concurrent Dir B candidate's
    EV (SLO breach counter in Redis monitoring).

- **v1** (2026-04-24): initial meta-plan — 4 toggle modes, 9 sub-plans,
  4 rollout phases. Codex review: 9 blockers (listed above).

## Goal

Shift the bot's **primary entry strategy** from perp-perp (funding-rate
spread across two perps) to Dir B (`buy_spot_short` — buy spot, short
futures, collect positive funding on the perp leg). Perp-perp and Dir A
become fallback. Behavior is gated behind a single config toggle so
production can migrate or revert without code deploys.

### Rationale (user)

Dir B has higher typical profit rate than perp-perp:
- Dir B earns absolute funding (positive funding paid to shorts) net of
  taker fees; no rotation cost, no cross-exchange transfer cost for the
  spot leg (spot stays put until exit).
- Perp-perp earns only the **differential** (short-leg funding minus
  long-leg funding) net of 2× taker fees plus rotation costs.
- When funding is extreme on any one exchange, Dir B captures it
  directly; perp-perp captures only the excess above the other
  exchange's funding.

### Constraints

- Live system — no regression for existing perp-perp trading when toggle
  is OFF (`feedback_feature_toggle.md`).
- Dashboard-controllable toggle — no restart required to change.
- All new features default OFF.
- Sub-plans must each pass the standard review loop before implementation.
- Config persistence is `config.json` only (per `CLAUDE.md:29` lockdown).

## Toggle Semantics (v2)

### Config field

```go
// In internal/config/config.go
type StrategyPriority string

const (
    StrategyPriorityPerpPerpFirst StrategyPriority = "perp_perp_first" // (default)
    StrategyPriorityDirBFirst     StrategyPriority = "dir_b_first"     // new target
    StrategyPriorityDirBOnly      StrategyPriority = "dir_b_only"      // monitor-only for PP + Dir A
    StrategyPriorityPerpPerpOnly  StrategyPriority = "perp_perp_only"  // monitor-only for SF (both directions)
)

type Config struct {
    ...
    StrategyPriority StrategyPriority `json:"strategy_priority"` // default "perp_perp_first"
}
```

### Semantics (direction-aware, v2)

| Value | PP entry | Dir B entry | Dir A entry | Capital allocation (when `EnableCapitalAllocator=true`) | Capital allocation (when OFF) | Monitor existing |
|-------|----------|-------------|-------------|--------------------------------------------------------|-------------------------------|------------------|
| `perp_perp_first` (default) | normal | after PP (existing order) | after PP (existing order) | PP priority | per-engine silos unchanged | all three |
| `dir_b_first` | runs AFTER Dir B admits (per Coordinator) | runs FIRST | still runs, but **after** Dir B winner locks | Dir B priority; PP + Dir A share remainder | per-engine silos unchanged; warning logged that priority is only effective with allocator ON | all three |
| `dir_b_only` | no new entries; existing managed normally | runs normally | no new entries; existing managed normally | Dir B only when allocator ON; SF silo only when OFF | per-engine silos unchanged | all three |
| `perp_perp_only` | runs normally | no new entries; existing managed normally | no new entries; existing managed normally | PP only when allocator ON | per-engine silos unchanged | all three |

**Key v2 clarifications:**
- Dir B is the **primary slot** in the `dir_b_first` mode; Dir A runs
  after Dir B as secondary within SF engine.
- When `EnableCapitalAllocator=false`, the toggle still reorders discovery
  and entry decisions inside each engine, but CAPITAL stays siloed — so
  "Dir B priority" for capital is not enforced; a warning is logged at
  startup/config-reload.

### Runtime boundary: StrategyEpoch

- New shared atomic field `Coordinator.epoch uint64` (see Step 4a).
- Incremented on each config reload that touches `StrategyPriority` OR
  `EnableCapitalAllocator`.
- Each engine, at the start of every scan tick, calls
  `Coordinator.snapshot() (strategyPriority StrategyPriority, epoch
  uint64)` and holds that view for the entire tick.
- If the epoch advances mid-tick, the CURRENT tick completes with the
  old view; the NEXT tick sees the new view.
- Mid-tick in-flight entries complete under their snapshot; no preemption.
- Log line at tick start: `[coordinator] cycle=<tick-id> started
  epoch=<n> strategy_priority=<mode>`.

### Dashboard

- Config page: `Strategy Priority: [perp_perp_first ▾]` dropdown.
- Overview page: "Primary: Dir B · Fallback: Perp-Perp + Dir A" (or
  mirror based on current value).
- Per-strategy KPI split: per-strategy active positions count, today's
  realized PnL, funding collected, capital deployed.

## Sub-Plans (execution order + dependencies)

### Step 1 — Config toggle scaffolding (no behavior change)

- File: `plans/PLAN-dir-b-toggle-scaffold.md` (new).
- Adds `StrategyPriority` field, JSON read/write, env var, defaults,
  dashboard dropdown, Config handler validation.
- Persistence: **`config.json` via Dashboard API writeback** (per repo
  config contract; NOT Redis). The dashboard config handler reads JSON,
  applies predicate, writes JSON with `.bak` backup.
- Behavior: no code path reads the value yet.
- Acceptance: dropdown flips the `config.json` value; config reload
  preserves it across restart; log line confirms read-at-boot value.
- Depends on: nothing.

### Step 2 — Capital Maximizer (existing plan, pending implementation)

- File: `plans/PLAN-capital-maximizer.md` (v12 ALL PASS — implementation
  pending).
- Unblocks donor-leg-conflict rejection so allocator can actually move
  capital cross-exchange efficiently. Both engines benefit.
- Prereq for Step 3b (EV-based gating needs coherent capital accounting).
- Depends on: nothing.

### Step 3a — EV computation + logging (no gating)

- File: `plans/PLAN-ev-logging.md` (new).
- Adds per-candidate `EstimatedEV` field to opportunity models (PP +
  SF). Computation:

  **Dir B (buy_spot_short) EV per USDT per hour:**
  ```
  EV_dirB = funding_rate_bps_per_hour * (1 - 2 * taker_fee_bps_futures / 10000)
         - (taker_fee_bps_spot_open + taker_fee_bps_spot_close) / (10000 * expected_hold_hours)
         - (taker_fee_bps_futures_open + taker_fee_bps_futures_close) / (10000 * expected_hold_hours)
  ```
  Notes:
  - Spot buy has no borrow cost (own USDT).
  - Spot leg fees amortized over `expected_hold_hours` (new config; default
    24h per recent Dir B holds).
  - Futures-side funding payment is bps/hour; spot leg has no funding.
  - Returned in bps per hour (same units as funding).

  **Dir A (borrow_sell_long) EV per USDT per hour:**
  ```
  EV_dirA = funding_rate_bps_per_hour * (1 - 2 * taker_fee_bps_futures / 10000)
         - borrow_rate_apr_bps / (365 * 24)
         - spot/futures amortized fees (same shape as Dir B)
  ```
  Notes:
  - Borrow APR from `MarginInterestRate` API divided by 365*24 to get
    bps/hour.

  **Perp-perp EV per USDT per hour:**
  ```
  spread_bps_h = short_funding_rate_bps_per_hour - long_funding_rate_bps_per_hour
  EV_pp = spread_bps_h * (1 - fee_asymmetry_factor)
       - (fee_bps_short_leg_open + fee_bps_short_leg_close) / (10000 * expected_hold_hours)
       - (fee_bps_long_leg_open + fee_bps_long_leg_close) / (10000 * expected_hold_hours)
       - rotation_cost_bps_per_hour_estimate  // amortized historical rotation fees per hour
  ```
  Notes:
  - `rotation_cost_bps_per_hour_estimate` read from a new Redis rolling
    window (`arb:pp:rotation_cost_7d`) populated by existing rotation
    PnL writes.
  - `fee_asymmetry_factor` is zero if both exchanges have same taker
    fee, otherwise `(fee_short - fee_long) / (fee_short + fee_long)`.

- Logged at scan time: `[ev] candidate=<sym> strategy=<dirB|dirA|pp>
  funding=<...> ev_net=<...> bps/h expected_hold_h=<h>`.
- **No gating decision is made**; Step 3b adds that.
- Acceptance: EV values appear in logs for every candidate; on a
  known reference day (captured test vector), values match expected to
  0.01 bps/h.
- Depends on: Step 1.

### Step 3b — EV-based gating (Coordinator-driven)

- File: `plans/PLAN-ev-gating.md` (new).
- Uses the Coordinator (Step 4a) to compare EV across candidates and
  gate entries per toggle mode.
- When `StrategyPriority=dir_b_first`:
  1. Dir B candidates admitted in rank order (by EV) up to capital
     limit.
  2. For each remaining `exchange:symbol` slot, PP candidates are
     compared against the MARGINAL Dir B EV (what the next Dir B would
     have earned with remaining capital). PP admits only if its EV >
     that marginal.
  3. Dir A candidates treated identically to PP in ordering (earn slot
     vs marginal Dir B) but ranked against PP on EV.
- When `StrategyPriority=perp_perp_first`: mirror.
- Coordinator emits `[coordinator] cycle=<n> chose=<dirB|dirA|pp>
  symbol=<sym> ev_net=<...> bps/h reason=<admitted|blocked:<why>>` per
  candidate.
- Depends on: Step 1, Step 3a, Step 4a.

### Step 4a — Coordinator seam

- File: `plans/PLAN-coordinator.md` (new).
- New type `strategy.Coordinator` in `internal/strategy/coordinator.go`:

  ```go
  type Coordinator struct {
      mu       sync.RWMutex
      epoch    atomic.Uint64
      cfg      *config.Config
      reservations map[string]*Reservation // exchange:symbol -> strategy
      cycleLog     []CycleRecord
  }
  func (c *Coordinator) Snapshot() (StrategyPriority, uint64)
  func (c *Coordinator) AdvanceEpoch()
  func (c *Coordinator) Reserve(cycle uint64, strategy string, exchangeSymbol string) (ok bool)
  func (c *Coordinator) Release(exchangeSymbol string)
  func (c *Coordinator) StartCycle(strategy string) uint64  // returns cycle ID
  ```

- Both engines (`engine/engine.go` and `spotengine/engine.go`) accept a
  `*strategy.Coordinator` via constructor. Each engine at tick start
  calls `c.StartCycle(strategyName)` and `c.Snapshot()` to freeze the
  per-cycle view.
- Coordinator is INJECTED, not a global. Allows unit tests to stub.
- On config reload at `cmd/main.go`, call
  `coordinator.AdvanceEpoch()` to bump.
- Log line: `[coordinator] epoch advanced <old>->=<new>
  (strategy_priority=<mode> capital_allocator=<on|off>)`.
- Acceptance: two engines both use the same coordinator instance;
  racing `AdvanceEpoch()` against `Snapshot()` is exercised in a
  unit test with 100 iterations.
- Depends on: nothing.

### Step 4b — Engine dispatch order uses Coordinator snapshot

- File: `plans/PLAN-engine-dispatch-coordinator.md` (new).
- Each engine's scan loop uses `Coordinator.Snapshot()` at tick start
  and routes its entry logic accordingly:
  - `spotengine/engine.go discoveryLoop` at `:168`: when snapshot =
    `dir_b_first` or `dir_b_only`, runs immediately without deferring
    to PP. When snapshot = `perp_perp_first` or `perp_perp_only`, does
    NOT open new positions until PP's cycle for this minute has
    released its reservations (polls `Coordinator.IsCycleDone(cycle)`).
  - `engine/engine.go` scan handlers at `:1003+`: mirror for PP.
- This does NOT move handlers into a shared tick; each engine keeps
  its own ticker, only the **decision ordering** is coordinated.
- Depends on: Step 4a.

### Step 5 — BingX Dir B spot leg (new SpotExchange interface)

- File: `plans/PLAN-bingx-dir-b-spot-leg.md` (new).
- Per memory `project_bingx_dirb_spot_leg.md`.

**Sub-tasks (v2 expanded):**

1. **New interface `exchange.SpotExchange`** in `pkg/exchange/exchange.go`
   — subset of SpotMarginExchange: `PlaceSpotOrder`, `QuerySpotOrder`,
   `GetSpotBalance`, `SpotOrderRules`, `GetSpotFee`. No Borrow/Repay.
2. **BingX adapter**: `pkg/exchange/bingx/spot.go` implementing
   `SpotExchange`. Wires `/openApi/spot/v1/trade/order`,
   `/openApi/spot/v1/trade/query`, `/openApi/spot/v1/account/balance`.
3. **Engine init split**: `spotengine/engine.go:80-90` currently indexes
   only `SpotMarginExchange`. v2 adds a separate `spotOnly map[string]
   SpotExchange` index. Discovery iterates UNION of both maps.
4. **Discovery dual-index**: `spotengine/discovery.go:179,394` —
   `iterateSpotExchanges(fn)` helper that yields both margin and
   non-margin exchanges; Dir B-eligible on both, Dir A-eligible only on
   margin.
5. **Risk gate fork**: `spotengine/risk_gate.go:145` — when spot leg
   exchange is spot-only, skip borrow-related checks entirely;
   pre-entry validates only USDT balance sufficiency on spot account.
6. **Execution fork**: `spotengine/execution.go:123,583` — new branch
   `spotOnlyBuy` / `spotOnlySell` that uses `SpotExchange` rather than
   `SpotMarginExchange`. Close path mirrors.
- Default OFF via a separate `EnableSpotOnlyExchanges` config bool
  (Dir B primary is the main toggle; this is its expansion).
- Depends on: independent of Dir B toggle — but practically ships after
  Step 4b.

### Step 6 — 1h funding exit redesign

- File: `plans/PLAN-1h-exit-redesign.md` (new).
- Per memory `project_1h_exit_design.md` (design agreed 2026-04-02).
- Rolling reversal ratio + asymmetric T-3m/T+5m settlement skip for 1h
  coins + funding-loss hard stop.
- Applies to BOTH Dir B and perp-perp (both engines have 1h-cycle
  risk); not toggle-gated.
- Depends on: independent.

### Step 7 — Transfer bugs v8

- File: `plans/PLAN-transfer-bugs.md` (existing, at v7, needs v8).
- Four-location deficit formula mismatch + no min-withdraw check.
- Depends on: Step 2 (same subsystem).

### Step 8 — Reconcile stats race

- File: `plans/PLAN-reconcile-stats-race.md` (new).
- Per independent Codex review of the (now-moot) Binance reconcile plan:
  `needsPnLUpdate` at `exit.go:1265` and `statsDiff` at `exit.go:1353`
  use stale `oldPnL = pos.RealizedPnL`. When rotation path updates DB
  RealizedPnL before close retry fires, close retry double-adjusts
  stats. Fix: re-read `pos.RealizedPnL` + `pos.RotationPnL` at top of
  `tryReconcilePnL`; use `appliedReconciledPnL` (from predicate closure)
  for `statsDiff`.
- Depends on: nothing; can ship at any time.

### Step 9 — Strategy dashboard view

- File: `plans/PLAN-strategy-dashboard.md` (new).
- Overview: current strategy priority, per-strategy capital/PnL split,
  toggle-change history (last 5, timestamped).
- Opportunities: strategy tag per row + EV-adjusted score.
- Depends on: Step 1 (toggle field) + Step 3a (EV available).

## Cross-engine hard blocks — preserved, not inverted (v2)

Existing hard-blocks in code that this pivot **preserves**:

| Location | What it blocks | v2 stance |
|----------|----------------|-----------|
| `engine/engine.go` (`filterArgmaxOpps`) | PP won't open on `exchange:symbol` already held by SF | **Preserved** as post-winner safety guard; Coordinator does pre-winner selection. |
| `spotengine/risk_gate.go:47` (`crossEngineBlocked`) | SF won't open when PP holds same `exchange:symbol` | **Preserved**. |
| `consolidator` cross-engine checks (recent v0.32.13/14 fixes) | Consolidator cross-engine position mismatch detection | **Preserved** unchanged. |

v2 adds **on top**: Coordinator's `Reserve()` is a soft pre-entry
serialization layer; existing hard-blocks remain as the backstop.

## Implementation order summary (v2)

| Order | Sub-plan | Can start when | User-visible change |
|-------|----------|----------------|---------------------|
| 1 | Toggle scaffold | Now | Dropdown visible, no-op |
| 2 | Capital Maximizer | Now (parallel) | Allocator more efficient |
| 3a | EV logging | After 1 | EV values in logs |
| 4a | Coordinator seam | Now (parallel) | `[coordinator]` log lines |
| 3b | EV gating | After 3a + 4a | Winner-selection logs |
| 4b | Engine dispatch via Coordinator | After 4a | Dir B runs first when `dir_b_first` |
| 5 | BingX Dir B spot leg | After 4b (or parallel) | BingX in Dir B opp list |
| 6 | 1h exit redesign | Any time | 1h positions exit cleanly |
| 7 | Transfer v8 | After 2 | Capital can push to spot accounts |
| 8 | Reconcile stats race | Any time | Stats matches row |
| 9 | Dashboard strategy view | After 1 + 3a | Strategy-aware UI |

Toggle flip from `perp_perp_first` → `dir_b_first` becomes meaningful
**after Step 4b**. Before that, the toggle is scaffolding.

## Rollout (v2)

### Phase A (Steps 1, 2, 4a)
- Merge & deploy. No behavior change. Dropdown visible but inert;
  Coordinator logging visible.
- Observe 1 full day.

### Phase B (Steps 3a, 3b, 4b)
- Merge & deploy with toggle defaulting to `perp_perp_first`. EV logged
  + gated + dispatch ordering now respects toggle.
- On VPS, flip toggle to `dir_b_first` → observe ≥ 2 full scan cycles
  → revert if any unexpected log line appears.
- Once stable, leave on `dir_b_first` permanently.

### Phase C (Steps 5, 6, 7)
- Ship expansion (BingX spot leg, 1h exit, Transfer v8). Each deploys
  independently.

### Phase D (Steps 8, 9)
- Reconcile stats race, dashboard strategy view. No trading change.

## Risk assessment

- **Default off**: every sub-plan defaults to the current behavior.
- **Coordinator as single source of truth**: both engines observe the
  same epoch, so mid-flip races cannot produce divergent views.
- **Independent revertability**: toggle flip = `config.json` rewrite +
  dashboard save; no rebuild.
- **Hard blocks preserved**: existing cross-engine safety guards stay;
  Coordinator adds a soft pre-entry layer on top.
- **Review loop**: each sub-plan goes through standard review
  (normal-loop to PASS, then independent round) before implementation.

## Acceptance (v2, deterministic signals only)

- **Signal 1 — toggle visibility**: `config.json` contains
  `"strategy_priority": "perp_perp_first"` after Step 1; dashboard
  dropdown round-trips values.
- **Signal 2 — Coordinator observed in logs**: every engine tick emits
  `[coordinator] cycle=N started epoch=M strategy_priority=X` at tick
  start. Manual spot-check across 10 consecutive scan minutes in logs.
- **Signal 3 — EV computed for every candidate**: per-candidate
  `[ev] candidate=<sym> strategy=<...> ev_net=<...>` log line; count
  equals candidate count in the same scan log.
- **Signal 4 — Winner selection traceable**: after Step 3b,
  `[coordinator] cycle=N chose=<...>` line per candidate with
  `admitted` or `blocked:<reason>`. Blocked reasons enumerated.
- **Signal 5 — Toggle flip takes effect at cycle boundary**: flip
  toggle mid-cycle; observe current cycle completes under old mode
  (look at cycle's `started epoch=` line), next cycle uses new mode
  (next `started epoch=` shows bumped value).
- **Signal 6 — SLO breach counter**: new Redis key
  `arb:dirb:slo_breach_count` increments when PP opens a position with
  EV lower than a concurrent Dir B candidate's EV (post-Step 3b).
  Zero expected on happy path; non-zero triggers rollback.
- **Signal 7 — Pre-vs-post equivalence under toggle=OFF**: before and
  after Step 4b, run a 1-hour scan with toggle locked to
  `perp_perp_first`; candidate ordering logs must be byte-for-byte
  identical (via `diff` on captured logs).

## Out of scope (final, v2)

- Cross-exchange Dir A (`project_cross_exchange_dira.md`) — user
  deferred.
- Real-time unrealized PnL display — UX improvement, not required.
- FFUSDT orphan close bug (`project_pending_issues.md`) — separate
  investigation.
- Binance reconcile Tier 1 per-leg skip — already fixed in HEAD
  (`CloseSizeUnknown` mechanism, egg commit e59be3d4); VPS needs
  deploy, not code.
- Reconcile stats race is **Step 8 in scope**, NOT out of scope (v2
  consistency fix — v1 listed it in both).
