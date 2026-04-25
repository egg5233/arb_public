# PLAN: Dir B Primary Strategy (Feature-Toggle Pivot)

Version: v3
Date: 2026-04-24
Status: DRAFT (meta-plan — coordinates sub-plans)

## Changelog
- **v3** (2026-04-24): Codex review round 2 — NEEDS-REVISION 5 blockers. Fixed:
  - **#1 EV formula dimensional error**: v2 formulas multiplied `funding_bps_h *
    (1 - fee_bps/10000)` which is nonsensical (bps/h × dimensionless fraction
    ≈ funding with 0.02% haircut, understates fees 10000×). And fee
    amortization `/ (10000 * hours)` produces decimal/hour not bps/h. v3
    rewrites as additive bps/h throughout: `EV_bps_h = funding_bps_h -
    total_fees_bps / hold_hours - borrow_bps_h - rotation_bps_h`. All terms
    in bps/hour. Complete rewrite of formulas for Dir B, Dir A, perp-perp.
  - **#2 StrategyEpoch snapshot not race-safe**: v2 kept `*config.Config.
    StrategyPriority` and `Coordinator.epoch` as separate atomics. Config
    POST handler mutates `*cfg` first (handlers.go:983), then notifies
    (handlers.go:1755), and subscribers (notifier.go:30) can observe
    `new_mode + old_epoch` or `old_mode + new_epoch`. v3 moves the mode
    value INTO the Coordinator behind a single atomic.Value holding
    `struct{mode StrategyPriority; epoch uint64; capAllocOn bool}`. Engines
    always snapshot via `Coordinator.Snapshot()` which returns the bundled
    struct atomically. Config handler calls `Coordinator.UpdatePriority
    (newMode, capAllocOn)` which does a single atomic.Value.Store. `cfg.
    StrategyPriority` is still written to config.json for persistence but
    runtime reads ALWAYS go through the coordinator.
  - **#3 Cycle protocol undefined**: v2 Step 4b referenced `IsCycleDone`
    / "PP's cycle for this minute" without Step 4a defining them; the two
    engines run on separate tickers with no shared minute boundary. v3
    replaces the "wait for the other engine" pattern with a
    **reservation-based protocol**. Coordinator API:
    - `TryReserve(exchSymbol string, strategy string, ev_bps_h float64)
      (granted bool, reason string)` — compares against existing
      reservations; mode + EV + strategy determine priority.
    - `ReleaseReservation(exchSymbol, strategy, orderID string)` — called
      when entry completes or fails.
    - Reservations have TTL (5 minutes default) auto-expiring if caller
      crashes.
    - Reservations are per-exchange:symbol. No shared cycle ID needed.
    - Priority rule: when mode=`dir_b_first` AND strategy=`dir_b`,
      incoming reservation can bump an existing PP or Dir A reservation
      (PP/DirA entry not yet placed). When mode=`perp_perp_first`, PP
      bumps SF. No bumping once order placed (tracked via `BindOrder`).
    - Engines no longer block on each other; they just query
      reservations + compare priority.
  - **#4 Dependency ordering contradiction**: v2 had Step 2 as Step 3b
    prereq in intro but omitted from Step 3b's own dependency list, and
    rollout said "toggle meaningful after 4b" while Step 3b already
    gates. v3 reconciles:
    - Step 3b depends on Step 1 + Step 2 + Step 3a + Step 4a
      (coordinator exists; EV computed; Capital Maximizer lands so
      allocator state is consistent).
    - Rollout: toggle becomes **meaningful at entry-gate level** after
      Step 3b (EV-based admission respects mode). Step 4b adds
      **scan-scheduling order** on top but is NOT required for mode
      effect on entries. Rollout summary updated.
  - **#5 Signal 7 not deterministic**: v2 said "1-hour live log diff
    byte-for-byte identical" — market state changes, impossible. v3
    replaces with a **unit-test equivalence check**: fixed-input test
    cases (captured candidate sets from a real scan snapshot) run
    through the pre-coordinator code path and post-coordinator code
    path with `mode=perp_perp_first`. Candidate ordering + EV values
    + winner selection outputs must match byte-for-byte. This is a
    compile-time property, not a live-market property.
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

  **All formulas output bps/hour. All inputs in bps. Fees are amortized
  over expected hold time. No multiplicative fee factors — fees are a
  separate subtraction term.**

  **Dir B (buy_spot_short) EV_bps_h:**
  ```
  total_fees_bps = taker_fee_bps_spot_open      // spot market BUY taker
                 + taker_fee_bps_spot_close     // spot market SELL taker
                 + taker_fee_bps_futures_open   // futures SHORT open
                 + taker_fee_bps_futures_close  // futures SHORT close

  EV_dirB_bps_h = funding_rate_bps_h               // earned on short futures
                - total_fees_bps / expected_hold_hours  // amortized fees
  ```
  Rationale:
  - Spot buy uses own USDT — zero borrow cost.
  - Funding earned only on the futures leg (spot has no funding).
  - All four taker fees paid once each over the position lifetime; dividing
    total by hold hours gives the hourly cost rate in bps/h.

  **Dir A (borrow_sell_long) EV_bps_h:**
  ```
  total_fees_bps = taker_fee_bps_spot_open      // spot margin SELL
                 + taker_fee_bps_spot_close     // spot margin BUY (repay)
                 + taker_fee_bps_futures_open   // futures LONG open
                 + taker_fee_bps_futures_close  // futures LONG close

  borrow_cost_bps_h = borrow_apr_bps_per_year / (365 * 24)

  EV_dirA_bps_h = funding_rate_bps_h               // earned on long futures
                - total_fees_bps / expected_hold_hours
                - borrow_cost_bps_h               // charged continuously
  ```
  Rationale:
  - Spot SELL is a borrow-sell — continuous borrow interest accrues.
  - `borrow_apr_bps_per_year` pulled from `GetMarginInterestRate(asset)`
    per exchange.

  **Perp-perp EV_bps_h:**
  ```
  spread_bps_h = short_funding_bps_h - long_funding_bps_h  // net per-hour income

  total_fees_bps = taker_fee_bps_short_open
                 + taker_fee_bps_short_close
                 + taker_fee_bps_long_open
                 + taker_fee_bps_long_close

  rotation_bps_h = rotation_7d_total_bps / (7 * 24)  // amortized rotation cost

  EV_pp_bps_h = spread_bps_h
              - total_fees_bps / expected_hold_hours
              - rotation_bps_h
  ```
  Rationale:
  - `rotation_7d_total_bps` from rolling 7-day Redis key `arb:pp:rotation_
    cost_7d` summing rotation fees in bps of notional over past 7d.
  - No fee-asymmetry multiplicative factor — fees are simply summed per
    leg.

  **`expected_hold_hours` value:**
  - New config field `ExpectedHoldHours float64`, default `24`.
  - One value shared across Dir B / Dir A / PP for consistency.
  - Future improvement: per-strategy observed median hold hours from
    history; deferred as data-driven tuning.

- Logged at scan time: `[ev] candidate=<sym> strategy=<dirB|dirA|pp>
  funding_bps_h=<...> fees_bps_h=<...> borrow_bps_h=<...>
  rotation_bps_h=<...> ev_net_bps_h=<...>`.
- **No gating decision is made**; Step 3b adds that.
- Acceptance: EV values appear in logs for every candidate; fixed-input
  test vector with captured funding/fee values produces expected
  `ev_net_bps_h` to ±0.01 bps/h tolerance.
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
- Coordinator emits `[coordinator] symbol=<sym> chose=<strategy>
  ev_net=<...> bps/h reason=<admitted|bumped:<prior>|blocked:<why>>`
  per candidate.
- Depends on: Step 1 (toggle field), Step 2 (Capital Maximizer — allocator
  state consistent), Step 3a (EV values), Step 4a (Coordinator exists).

### Step 4a — Coordinator seam

- File: `plans/PLAN-coordinator.md` (new).
- New type `strategy.Coordinator` in `internal/strategy/coordinator.go`:

  ```go
  // snapshot bundles all coordinator mode state atomically.
  type snapshot struct {
      mode                  StrategyPriority
      epoch                 uint64
      capitalAllocatorOn    bool
  }

  type Reservation struct {
      ExchSymbol string
      Strategy   string   // "dir_b" | "dir_a" | "pp"
      EVbpsH     float64
      Priority   uint8    // computed from mode + strategy
      OrderID    string   // empty until BindOrder()
      ExpiresAt  time.Time
  }

  type Coordinator struct {
      mu            sync.Mutex
      state         atomic.Value  // holds snapshot (replaces separate mode/epoch fields)
      reservations  map[string]*Reservation  // exchSymbol -> latest winner
      reservationTTL time.Duration  // default 5min
      log           *utils.Logger
  }

  // Snapshot returns the current mode + epoch atomically as a bundle.
  // Engines call this at tick start and hold the returned value for
  // the full tick.
  func (c *Coordinator) Snapshot() snapshot

  // UpdatePriority stores a new mode + bumps epoch + capAllocOn in a
  // single atomic.Value.Store. Called by the config handler AFTER it
  // writes config.json, BEFORE Notify() fans out. Race-safe.
  func (c *Coordinator) UpdatePriority(mode StrategyPriority, capAllocOn bool)

  // TryReserve attempts to claim exchSymbol for `strategy` with the given
  // EV. Returns granted=true if accepted; false with reason if rejected.
  // Accept/reject rules:
  //   1. If no prior reservation → accept.
  //   2. If prior reservation has OrderID bound (entry in-flight) → reject.
  //   3. Else compute priority of incoming vs existing from current mode:
  //        - mode=dir_b_first: dir_b=1, dir_a=2, pp=2
  //        - mode=perp_perp_first: pp=1, dir_b=2, dir_a=2
  //        - mode=dir_b_only: dir_b=1, others=DENY (reject outright)
  //        - mode=perp_perp_only: pp=1, others=DENY
  //      Lower priority number wins. If incoming priority < existing →
  //      bump existing (release); accept incoming.
  //   4. Same priority → tiebreak by EV (higher wins).
  //   5. Existing wins otherwise → reject incoming.
  func (c *Coordinator) TryReserve(exchSymbol, strategy string, ev float64) (granted bool, reason string)

  // BindOrder promotes a pending reservation to an order-bound state.
  // Reservations bound to orders cannot be bumped.
  func (c *Coordinator) BindOrder(exchSymbol, strategy, orderID string) error

  // Release drops a reservation (on order completion OR on order failure).
  func (c *Coordinator) Release(exchSymbol, strategy string)

  // gcExpired walks reservations older than TTL and releases them. Called
  // by Snapshot() or a background ticker.
  func (c *Coordinator) gcExpired()
  ```

- **Race-safety pattern**: mode + epoch + capAllocOn live inside a
  single `atomic.Value` snapshot struct. The config handler calls
  `Coordinator.UpdatePriority()` which does ONE `atomic.Value.Store`.
  Engines call `Snapshot()` which does ONE `atomic.Value.Load`. No
  subscriber-ordering race possible: whatever the engine reads, it
  reads all three fields together.
- **Wiring order in handler**: the dashboard config POST handler MUST
  call `Coordinator.UpdatePriority()` **before** `Notify()` fans out.
  This ensures any engine that reacts to the notifier immediately sees
  the new mode via the snapshot.
- Coordinator is INJECTED via constructor to both engines and to the
  config handler. Not a global. Unit-testable.
- Log line: `[coordinator] state updated mode=<m> epoch=<n>
  capital_allocator=<on|off>`.

**Acceptance**:
- Unit test racing `UpdatePriority()` vs `Snapshot()` × 1000 iterations
  shows the returned snapshot always has a consistent triple (no
  torn reads).
- Unit test `TryReserve` with both modes demonstrates correct
  priority-bump behavior and order-bound immunity.
- Integration test: two goroutines simulate both engines reserving
  the same exchSymbol; the expected strategy wins under each mode.

- Depends on: nothing.

### Step 4b — Engine entry paths use Coordinator reservations

- File: `plans/PLAN-engine-dispatch-coordinator.md` (new).
- Replaces v2's "wait for the other engine to finish its cycle"
  design (not workable across separate tickers) with the
  **reservation-based flow**:

  For each engine at entry time (after candidate ranking but before
  placing order):
  1. Snapshot mode via `Coordinator.Snapshot()`.
  2. For each ranked candidate:
     a. Call `TryReserve(exchSymbol, strategy, ev_bps_h)`.
     b. If `granted=true`: proceed to `BindOrder()` after placing the
        order. Then place the order. On success, bind OrderID. On
        failure, call `Release()`.
     c. If `granted=false`: skip candidate, log `[coordinator]
        reserve-denied strategy=<s> symbol=<x> reason=<r>
        conflict_ev=<...>`. Move to next candidate.

- No engine waits on the other. The reservation table itself provides
  serialization and priority. Both engines run their tickers
  independently.
- Out-of-order ticking is handled: if SF ticks first and reserves,
  PP ticks later and finds slot taken (by higher-priority reservation
  if mode=dir_b_first). Or if PP ticks first and reserves with lower
  priority (mode=dir_b_first), SF's later reservation BUMPS PP's (only
  if PP hasn't yet bound an order — if it has, SF yields).
- Existing cross-engine hard blocks (`filterArgmaxOpps` etc.) remain
  as post-open safety guards. Coordinator is the pre-entry layer.

**Acceptance (Step 4b-specific)**:
- Unit test: two mock engines race to `TryReserve` same exchSymbol;
  under each mode, the correct winner holds the reservation.
- Integration test: SF reserves first (mode=dir_b_first), PP later
  tries, gets denied with reason=`dir_b_has_higher_priority`.
- Log signal: every denied reservation emits `[coordinator] reserve-
  denied` with ev values for before/after diff.

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
| 3b | EV gating | After 1 + 2 + 3a + 4a | Winner-selection logs |
| 4b | Engine dispatch via Coordinator | After 4a | Dir B runs first when `dir_b_first` |
| 5 | BingX Dir B spot leg | After 4b (or parallel) | BingX in Dir B opp list |
| 6 | 1h exit redesign | Any time | 1h positions exit cleanly |
| 7 | Transfer v8 | After 2 | Capital can push to spot accounts |
| 8 | Reconcile stats race | Any time | Stats matches row |
| 9 | Dashboard strategy view | After 1 + 3a | Strategy-aware UI |

Toggle mode affects **entry admission via EV gate** after Step 3b ships
(reservations respect priority). Step 4b is not strictly required for
toggle meaningfulness — it's a refinement; without it both engines still
run their own tickers, but the reservation protocol prevents conflicts
and enforces priority.

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
- **Signal 7 — Pre-vs-post equivalence under toggle=OFF (unit test, v3)**:
  a fixed-input test suite `TestOffModeEquivalence` feeds a captured
  candidate set (JSON of 20 real scan candidates) through (a) the
  pre-coordinator code path and (b) the post-coordinator code path
  with `mode=perp_perp_first`. Assertions: same ordering, same EV
  values to 0.01 bps/h, same winner per symbol. Deterministic —
  market-state independent. Runs in CI + pre-deploy.

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
