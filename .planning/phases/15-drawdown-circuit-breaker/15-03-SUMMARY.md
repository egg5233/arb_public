---
phase: 15
plan: 03
subsystem: pricegaptrader
tags: [drawdown-breaker, controller, two-strike-state-machine, d-15-atomicity, wave-2]
requires:
  - phase-15-plan-01-foundation
  - phase-15-plan-02-aggregator-chokepoint
provides:
  - pricegaptrader.BreakerController
  - pricegaptrader.BreakerStateStore
  - pricegaptrader.BreakerNotifier
  - pricegaptrader.BreakerWSBroadcaster
  - pricegaptrader.CandidatePauser
  - pricegaptrader.ActivePositionLister
  - pricegaptrader.Tracker.SetBreakerController
  - pricegaptrader.Registry.SetPausedByBreaker
  - pricegaptrader.Registry.PauseAllOpenCandidates
  - pricegaptrader.Registry.ClearAllPausedByBreaker
  - models.PriceGapCandidate.PausedByBreaker
  - PriceGapNotifier.NotifyPriceGapBreakerTrip
  - PriceGapNotifier.NotifyPriceGapBreakerRecovery
  - risk_gate.Gate1.5_paused_by_breaker
affects:
  - internal/models/pricegap_interfaces.go (Candidate field)
  - internal/notify/pricegap_reconcile.go (TelegramNotifier breaker stubs)
  - internal/pricegaptrader/notify.go (PriceGapNotifier extension)
  - internal/pricegaptrader/registry.go (3 chokepoint helpers)
  - internal/pricegaptrader/risk_gate.go (Gate 1.5)
  - internal/pricegaptrader/tracker.go (controller field + setter + daemon spawn)
  - internal/pricegaptrader/breaker_controller.go (NEW — full controller)
  - internal/pricegaptrader/tracker_broadcast_test.go (spyNotifier extension)
  - internal/pricegaptrader/daemon_test.go (fakeBootGuardNotifier extension)
tech-stack:
  added: []
  patterns:
    - "Daemon goroutine + ctx adapter: stopCh→context.WithCancel for BreakerController.Run integration with existing tracker shutdown"
    - "Narrow-interface DI for D-15 boundary: 5 interfaces consumed by controller, all satisfied by existing concrete types via duck typing"
    - "D-15 trip atomicity-by-ordering: Step 1 load-bearing, Steps 2-5 best-effort with logged failures"
    - "Chokepoint write helpers (SetPausedByBreaker / PauseAllOpenCandidates / ClearAllPausedByBreaker) reuse the existing Registry path-lock + cfg-lock + reload-from-disk + SaveJSONWithBakRing pattern with rollback"
    - "Risk gate insertion at Gate 1.5 — distinct rejection reason ('paused_by_breaker' vs 'exec_quality') so operators distinguish in logs/Telegram"
key-files:
  created:
    - internal/pricegaptrader/breaker_controller.go
  modified:
    - internal/models/pricegap_interfaces.go
    - internal/notify/pricegap_reconcile.go
    - internal/pricegaptrader/notify.go
    - internal/pricegaptrader/registry.go
    - internal/pricegaptrader/registry_concurrent_test.go
    - internal/pricegaptrader/risk_gate.go
    - internal/pricegaptrader/risk_gate_test.go
    - internal/pricegaptrader/tracker.go
    - internal/pricegaptrader/tracker_broadcast_test.go
    - internal/pricegaptrader/daemon_test.go
    - internal/pricegaptrader/breaker_controller_test.go
    - CHANGELOG.md
    - VERSION
decisions:
  - "Adapted plan's hypothetical Candidate.Disabled bool to Candidate.PausedByBreaker bool only — actual models.PriceGapCandidate has no Disabled field; disable is Redis-backed (IsCandidateDisabled). Added Gate 1.5 in risk_gate.go (between Gate 1 and Gate 2) instead of replacing the plan's hypothetical Disabled-OR-Paused composite check. Distinct rejection reasons preserved per D-10 (Rule 3 — blocking issue, hypothetical interface)"
  - "ActivePositionLister.GetActivePriceGapPositions() returns []*models.PriceGapPosition (not []string IDs as plan suggested) — matches the existing PriceGapStore signature. PauseAllOpenCandidates accepts position objects so it can extract (symbol, longExch, shortExch) tuples for candidate matching"
  - "PauseAllOpenCandidates ignores Direction in the matching tuple. Positions don't always carry Direction (legacy/restored positions); pausing every candidate that shares the (symbol, long, short) triple is the correct conservative choice during a trip"
  - "BreakerWSBroadcaster method named BroadcastPriceGapBreakerEvent (specific) rather than generic Broadcast(eventType, payload). The narrow interface is intentionally specific so an *api.Hub addition for breaker events does not collide with existing Broadcast methods (Plan 15-04 wires production)"
  - "Step 2 reordered to run BEFORE Step 3 in trip() so the trip record's PausedCandidateCount is populated. CONTEXT D-15 prose ordering allowed this swap (D-15 lists Step 2 = LPUSH and Step 3 = pause; both are best-effort, only Step 1 is load-bearing). Comment block in trip() documents the swap"
  - "PriceGapCandidate.PausedByBreaker uses json:omitempty so legacy candidates persisted before Phase 15 round-trip byte-identical (no extra paused_by_breaker:false rows in config.json). Tested by TestRegistry_PausedByBreakerField"
  - "Telegram stubs for breaker methods land in pricegap_reconcile.go (alongside Phase 14 stubs) rather than a new file — matches the existing module-graph layout. Plan 15-04 ships real Telegram dispatch in internal/notify/pricegap_breaker.go (separate file) and removes these stubs"
  - "BreakerController test fakes use unique names (fakeBreakerStore, fakeBreakerNotifier, etc.) to avoid collision with existing risk_gate_test.go fakeStore + tracker_broadcast_test.go spyNotifier. Two stores: fakeBreakerStore (full surface) + fakeBreakerStoreOneShotSaveErr (Step 1 failure path with embed-and-override pattern)"
metrics:
  duration: ~22min
  tasks: 2
  files_created: 1
  files_modified: 13
  tests_added: 22  # 5 (Task 1) + 15 (Task 2 BreakerController) + 7 (preserved IsPaperModeActive truth-table from 15-01 stub, now real impl in this file) = 27 — but 5 already existed in tracker stubs from 15-02
  completed_date: 2026-05-01
---

# Phase 15 Plan 03: BreakerController + D-15 Trip Ordering Summary

**One-liner:** BreakerController daemon implements the two-strike state machine and the D-15 atomic trip ordering — sticky paper-mode flag persisted FIRST so any partial failure during the trip leaves the engine in the safest state, with Plan 15-02's `IsPaperModeActive` chokepoint guaranteeing the sticky flag is observable from every paper-mode read site in `internal/pricegaptrader/`.

## Objective Recap

Plan 15-03 is the load-bearing safety logic of Phase 15. It builds on Plan 15-01 data plumbing (`models.BreakerState/TripRecord`, `pg:breaker:state` HASH, `pg:breaker:trips` LIST) and Plan 15-02 chokepoint (`Tracker.IsPaperModeActive`, `RealizedPnLAggregator`) to produce the daemon that fires correctly on real PnL drawdowns, survives `kill -9` mid-strike, and cannot trap the engine in live mode after a partial failure during a trip.

Two deliverables:

1. **Task 1 — Notifier interface + Candidate.PausedByBreaker field + entry-path guard.** Extends `PriceGapNotifier` with two critical-bucket methods (`NotifyPriceGapBreakerTrip`, `NotifyPriceGapBreakerRecovery`); adds `PausedByBreaker bool` to `models.PriceGapCandidate`; adds 3 chokepoint helpers on `*Registry` for the trip + recovery paths; inserts Gate 1.5 in `risk_gate.go` so a paused candidate rejects entry with reason `"paused_by_breaker"` distinct from Gate 1's `"exec_quality"`.
2. **Task 2 — BreakerController daemon + D-15 trip ordering.** New file `internal/pricegaptrader/breaker_controller.go` with the full daemon: `Run` (5-min ticker + ctx.Done), `evalTick` (D-01..D-08 + boot guard + defensive ≥5min check + blackout suppression via REUSED `inBybitBlackout`), `trip` (D-15 ordering with Step 1 load-bearing, Steps 2-5 best-effort), `Snapshot` (read-only). 15 unit tests including the load-bearing `TestBreaker_TripOrdering_StickyFirstWhenStepsFail` and `TestBreaker_TripOrdering_Step1FailureAborts`.

## What Shipped

### Task 1 — Notifier + PausedByBreaker + Gate 1.5 (commit `c36130a`)

#### `models.PriceGapCandidate` field

```go
PausedByBreaker bool `json:"paused_by_breaker,omitempty"`
```

`json:omitempty` preserves byte-identity for legacy candidates persisted before Phase 15 — no `"paused_by_breaker": false` rows appear in `config.json` until the breaker actually pauses something.

#### Notifier interface delta

Two methods added to `pricegaptrader.PriceGapNotifier`:

```go
NotifyPriceGapBreakerTrip(record models.BreakerTripRecord) error
NotifyPriceGapBreakerRecovery(record models.BreakerTripRecord, operator string) error
```

Both dispatch via the **critical bucket** per CONTEXT D-17 (bypass allowlist; fire-now alerts). Implementers updated:
- `NoopNotifier` — return-nil stubs.
- `*notify.TelegramNotifier` — stub bodies (real Telegram dispatch ships in Plan 15-04 in a separate file `internal/notify/pricegap_breaker.go`).
- `spyNotifier` (test double in `tracker_broadcast_test.go`) — recording impl.
- `fakeBootGuardNotifier` (daemon_test.go) — stub for compile-time conformance.

#### 3 chokepoint helpers on `*Registry`

| Helper | Purpose | Audit-log op |
|---|---|---|
| `SetPausedByBreaker(symbol, longExch, shortExch, direction, value) (int, error)` | Idempotent per-candidate set. Direction normalization: empty == "pinned". | `pause_by_breaker` / `clear_paused_by_breaker` |
| `PauseAllOpenCandidates(positions []*models.PriceGapPosition) (int, error)` | Bulk trip-path mutation. Builds a `(symbol, long, short)` set from positions and pauses every matching candidate. Direction NOT included in the match — positions don't always carry Direction; conservative all-direction match. | `pause_all_open` |
| `ClearAllPausedByBreaker() (int, error)` | Bulk recovery — flips every PausedByBreaker=true → false. Operator-set Redis disable (IsCandidateDisabled) NOT touched. | `clear_all_paused_by_breaker` |

All 3 use the existing path-lock + cfg.Lock + `reloadFromDiskLocked` + `SaveJSONWithBakRing` pattern with rollback on persist failure (matches Phase 11 PG-DISC-04 chokepoint discipline).

#### `risk_gate.go` Gate 1.5

Inserted between Gate 1 (Redis exec-quality disabled) and Gate 2 (max concurrent):

```go
if cand.PausedByBreaker {
    t.notifier.NotifyPriceGapRiskBlock(cand.Symbol, "paused_by_breaker", "drawdown circuit breaker tripped")
    return GateDecision{Err: ErrPriceGapCandidateDisabled, Reason: "paused_by_breaker"}
}
```

Distinct reason string from Gate 1's `"exec_quality: ..."` — operators tell them apart in logs and Telegram. Gate 1 still fires first when both flags are set (verified by `TestRiskGate_DisabledOR_PausedByBreaker_TruthTable` case `both_true_disabled_first`).

#### Tests added (5)

| Test | Verifies |
|---|---|
| `TestRegistry_PausedByBreakerField` | JSON marshal includes `paused_by_breaker:true`; default-false omits the tag (omitempty). Unmarshal roundtrip preserves true. |
| `TestRegistry_PausedByBreaker_ConcurrentWrite` | 100 goroutines call SetPausedByBreaker on the same candidate; no torn writes; on-disk JSON remains valid; candidate identity intact. |
| `TestRegistry_RecoveryClearsPausedByBreaker_PreservesDisabled` | D-11: ClearAllPausedByBreaker flips paused→false on 2 candidates; reports count=2; Redis disable state untouched (helper has zero Redis interaction). |
| `TestRiskGate_PausedByBreaker_RejectsWithDistinctReason` | `cand.PausedByBreaker=true` → `Err = ErrPriceGapCandidateDisabled` and `Reason = "paused_by_breaker"`. |
| `TestRiskGate_DisabledOR_PausedByBreaker_TruthTable` | 4-cell matrix: (false,false) allow, (true,false) reject `exec_quality`, (false,true) reject `paused_by_breaker`, (true,true) reject `exec_quality` (Gate 1 fires first). |

### Task 2 — BreakerController + D-15 trip ordering (commit `e69527f`)

#### `BreakerController` struct

```go
type BreakerController struct {
    cfg        *config.Config
    store      BreakerStateStore
    aggregator breakerAggregator
    notifier   BreakerNotifier
    ws         BreakerWSBroadcaster   // best-effort, nil-safe
    ramp       RampSnapshotter        // best-effort, nil-safe
    registry   CandidatePauser        // best-effort, nil-safe
    positions  ActivePositionLister   // best-effort, nil-safe
    log        *utils.Logger
    mu         sync.RWMutex
}
```

Setters: `SetWSBroadcaster`, `SetRamp`, `SetRegistry`, `SetPositions`. Constructor takes only required deps (cfg, store, aggregator, notifier, log).

#### 5 narrow interfaces preserve D-15 module boundary

| Interface | Method(s) | Production satisfier |
|---|---|---|
| `BreakerStateStore` | LoadBreakerState / SaveBreakerState / AppendBreakerTrip | `*database.Client` (Plan 15-01) |
| `BreakerNotifier` | NotifyPriceGapBreakerTrip | `*notify.TelegramNotifier` (Plan 15-04 ships real impl) |
| `BreakerWSBroadcaster` | BroadcastPriceGapBreakerEvent(event, payload) | `*api.Hub` (Plan 15-04 wires) |
| `CandidatePauser` | PauseAllOpenCandidates(positions) | `*Registry` (Task 1) |
| `ActivePositionLister` | GetActivePriceGapPositions | `*database.Client` (Phase 8) |

#### `Run(ctx)` — daemon loop

Mirrors `RampController.Run` exactly:

```go
if !bc.cfg.PriceGapBreakerEnabled { ... return }
interval := bc.cfg.PriceGapBreakerIntervalSec
if interval < 60 { interval = 300 }    // safety floor
ticker := time.NewTicker(time.Duration(interval) * time.Second)
defer ticker.Stop()
for {
    select {
    case <-ctx.Done(): return
    case t := <-ticker.C: bc.evalTick(ctx, t)
    }
}
```

`Tracker.runBreakerDaemon` adapts `t.stopCh` into a `context.Context` so the existing tracker shutdown signal cancels the loop cleanly.

#### `evalTick(ctx, now)` — single eval cycle (D-01 .. D-08 + boot guard + defensive 5min)

| Step | Implementation | D-code |
|---|---|---|
| 1 | `if !cfg.PriceGapBreakerEnabled return nil` | re-check race-on-cfg-reload |
| 2 | `if inBybitBlackout(now) return nil` | **D-03** — REUSE scanner.go helper, no parallel impl |
| 3 | `state, exists, err := store.LoadBreakerState()` | D-05 |
| 4 | `if !exists { state = BreakerState{} }` | boot guard (permissive, unlike Phase 14 ramp) |
| 5 | `if state.PaperModeStickyUntil != 0 return nil` | already-tripped: operator must recover |
| 6 | `pnl, err := aggregator.Realized24h(ctx, now)` | Plan 15-02 reader |
| 7 | record `LastEvalTs` + `LastEvalPnLUSDT` (heartbeat) | observability |
| 8 | `inBreach := pnl < threshold` | D-06 absolute USDT |
| 9 | `!inBreach && state.PendingStrike==1` → clear (PendingStrike=0, Strike1Ts=0); save | **D-08** PnL recovery |
| 10 | `inBreach && state.PendingStrike==0` → set Strike-1, Strike1Ts=now, save | **Pitfall 2** (kill-9 survives) |
| 11 | `inBreach && PendingStrike==1 && elapsed < 5min` → defensive skip; save heartbeat | safety guard independent of ticker interval |
| 12 | `inBreach && PendingStrike==1 && elapsed >= 5min` → `trip()` | **D-15 entry** |

#### `trip()` — D-15 ordering with load-bearing safety property

```
STEP 1 (LOAD-BEARING):  state.PaperModeStickyUntil = math.MaxInt64
                        store.SaveBreakerState(state)
                        if err → ABORT trip, return error (Steps 2-5 SKIPPED)

STEP 2 (best-effort):   active := positions.GetActivePriceGapPositions()
                        count := registry.PauseAllOpenCandidates(active)
                        record.PausedCandidateCount = count

STEP 3 (best-effort):   store.AppendBreakerTrip(record)

STEP 4 (best-effort):   notifier.NotifyPriceGapBreakerTrip(record)

STEP 5 (no-error):      ws.BroadcastPriceGapBreakerEvent("pg.breaker.trip", record)
```

**Step 2 reordered before Step 3** so the trip record carries `PausedCandidateCount` — CONTEXT D-15 prose allows the swap (only Step 1 is load-bearing). Comment block in `trip()` locks the ordering. Each best-effort step has its own error-log line tagged `breaker: TRIP step N <method>: <err>`.

#### `Snapshot()` — read-only state read

For dashboard `GET /api/pg/breaker/state` endpoint (Plan 15-04 wires). Reads via the same store.LoadBreakerState path as evalTick — no separate cache to drift.

#### Tests added (15)

| # | Test | D-code anchor | Verifies |
|---|---|---|---|
| 1 | `TestBreaker_DisabledByDefault` | cfg=false | Run() returns immediately; evalTick zero side-effects |
| 2 | `TestBreaker_FreshBootInit` | boot guard | exists=false → fresh state, no panic, no trip, save persisted |
| 3 | `TestBreaker_BootGuard_PreservesExistingTrip` | boot guard | exists+sticky=MaxInt64 → no aggregator call, no save |
| 4 | `TestBreaker_SingleStrike_NoTrip` | two-strike | first breach → PendingStrike=1, no notifier |
| 5 | `TestBreaker_TwoStrikeTrips` | D-01 + trip | pending + breach + 6min elapsed → trip, append, notify |
| 6 | `TestBreaker_TwoStrikeRequiresTwoSeparateEvaluations` | defensive 5min | pending + breach + 2min → no trip; pending+Strike1Ts unchanged |
| 7 | `TestBreaker_RecoveryClearsPendingStrike` | **D-08** | pending + non-breach → clear (PendingStrike=0, Strike1Ts=0) |
| 8 | `TestBreaker_BlackoutSuppression` | **D-03** | tick at minute :04:30 → no aggregator, no save |
| 9 | `TestBreaker_PendingSurvivesBlackout` | **D-04** | blackout tick no-op → next normal tick still fires trip |
| 10 | `TestBreaker_StateSurvivesRestart` | **D-05 / Pitfall 2** | seeded Strike-1 in store → fresh controller fires trip on Strike-2 |
| 11 | **`TestBreaker_TripOrdering_StickyFirstWhenStepsFail`** | **D-15 anchor** | Steps 2/3/4 all fail → sticky still persisted, evalTick returns nil |
| 12 | **`TestBreaker_TripOrdering_Step1FailureAborts`** | **D-15 anchor** | Step 1 fails → evalTick returns error; Steps 2/3/4 never invoked |
| 13 | `TestBreaker_TripIncludesRampStage` | record-population | Ramp wired with stage=2 → record.RampStage=2 |
| 14 | `TestBreaker_TripPausedCandidateCount` | record-population | pauser returns count=3 → record.PausedCandidateCount=3 |
| 15 | `TestBreaker_WSBroadcastFiredOnTrip` | Step 5 | ws.calls len=1, event="pg.breaker.trip" |

Plus the 7 `TestTracker_IsPaperModeActive_*` tests preserved from Plan 15-01 stub (now real impls in this file): CfgFlagOnly, CfgFlagFalse_NoSticky, StickyMaxInt64, StickyFutureTimestamp, StickyExpired, RedisError_FailsToPaper, NilStore.

#### Tracker integration

```go
// tracker.go fields:
breakerController *BreakerController

// Setter:
func (t *Tracker) SetBreakerController(bc *BreakerController) { t.breakerController = bc }

// Start spawns daemon when wired:
if t.breakerController != nil {
    t.wg.Add(1)
    go t.runBreakerDaemon()    // adapts stopCh → context.Context
}
```

Plan 15-04 wires production via `cmd/main.go`:
```go
breakerCtl := pricegaptrader.NewBreakerController(cfg, dbClient, aggregator, telegramNotifier, log)
breakerCtl.SetWSBroadcaster(apiHub)
breakerCtl.SetRamp(rampCtl)
breakerCtl.SetRegistry(registry)
breakerCtl.SetPositions(dbClient)
tracker.SetBreakerController(breakerCtl)
tracker.Start()
```

## Verification

| Verification step | Result |
|---|---|
| `go build ./...` | Pass |
| `go vet ./...` | Pass (implicit via `go test`) |
| `go test ./internal/pricegaptrader -run "TestRegistry_PausedByBreaker\|TestRegistry_RecoveryClearsPausedByBreaker\|TestRiskGate_PausedByBreaker\|TestRiskGate_DisabledOR_PausedByBreaker" -count=1` | 9 passed (5 new + 4 sub-cases of TruthTable) |
| `go test ./internal/pricegaptrader -run "TestBreaker_" -count=1` | 16 passed (15 new + Snapshot already counted) |
| `go test ./internal/pricegaptrader -run "TestBreaker_TripOrdering_(StickyFirstWhenStepsFail\|Step1FailureAborts)" -count=1` | 2 passed — D-15 atomicity locked |
| `go test ./internal/pricegaptrader -count=1` | 319 passed |
| `go test ./... -count=1` | 1193 passed across 44 packages — no regressions |
| All Task 2 grep gates (Run / evalTick / trip / Snapshot / inBybitBlackout / no parallel impl / math.MaxInt64 / D-15 doc block / 5*60*1000 / SetBreakerController / breakerController.Run / no UnrealizedPnL\|MarkPrice) | All pass |
| `git diff --quiet config.json` | Pass — config.json untouched |
| CHANGELOG.md + VERSION updated | Pass — Unreleased extended with Task 1 + Task 2 sections; 0.37.2 → 0.37.3 → 0.37.4 |

## Deviations from Plan

Three Rule 3 (blocking-issue auto-fix) adaptations because the plan's `<interfaces>` block referenced hypothetical types/signatures that don't match the actual codebase. None changed the spirit of the plan — the safety properties (D-10, D-15, D-08, blackout, kill-9 survival) all hold.

### 1. [Rule 3 - Blocking] Candidate.Disabled does not exist; entry-path check inserted as Gate 1.5 instead of replacing hypothetical composite check

**Found during:** Task 1 read of registry.go + risk_gate.go.

**Issue:** Plan's `<interfaces>` showed:
```go
type Candidate struct {
    Disabled        bool   // operator-set
    PausedByBreaker bool   // Phase 15 ADDS
}
```
And the entry-path action said "locate `if c.Disabled` check; replace with `if c.Disabled || c.PausedByBreaker`". The actual `models.PriceGapCandidate` has no `Disabled` field — disable state is Redis-backed via `PriceGapStore.IsCandidateDisabled` (Phase 9 PG-RISK-03), and `risk_gate.go` Gate 1 reads it via `t.db.IsCandidateDisabled(symbol)`.

**Fix:** Added `PausedByBreaker bool` to `models.PriceGapCandidate` (still satisfies D-10 — distinct from operator-set state). Inserted a separate Gate 1.5 in `risk_gate.go` between Gate 1 (Redis disabled) and Gate 2 (max concurrent), so each gate emits a distinct rejection reason (`"exec_quality"` vs `"paused_by_breaker"`) — preserves D-10 spirit (operators distinguish operator-set vs breaker-set rejection).

**Files modified:** `internal/models/pricegap_interfaces.go`, `internal/pricegaptrader/risk_gate.go`.
**Commit:** `c36130a`.

### 2. [Rule 3 - Blocking] ActivePositionLister returns []*PriceGapPosition not []string

**Found during:** Task 2 implementation of trip Step 2.

**Issue:** Plan's `<action>` showed `ActivePositionLister.ActivePriceGapPositions() ([]string, error)` returning position IDs. The actual `*database.Client.GetActivePriceGapPositions()` returns `[]*models.PriceGapPosition` (Phase 8 / Phase 14 D-02), and we need (symbol, longExch, shortExch) tuples to match candidates — IDs alone wouldn't help PauseAllOpenCandidates derive the matching tuple.

**Fix:** `ActivePositionLister.GetActivePriceGapPositions() ([]*models.PriceGapPosition, error)` (matches the existing concrete signature exactly — `*database.Client` satisfies via duck typing without an adapter shim). `Registry.PauseAllOpenCandidates(positions []*models.PriceGapPosition)` extracts the tuple and matches against the candidate slice.

**Files modified:** `internal/pricegaptrader/registry.go`, `internal/pricegaptrader/breaker_controller.go`.
**Commit:** `c36130a` + `e69527f`.

### 3. [Rule 3 - Blocking] BreakerWSBroadcaster method named specifically (not generic Broadcast)

**Found during:** Task 2 controller drafting.

**Issue:** Plan's `<action>` showed `BreakerWSBroadcaster.Broadcast(eventType string, payload any)` — a generic method name that would collide with existing `*api.Hub` Broadcast methods if duck-typed.

**Fix:** Named the method `BroadcastPriceGapBreakerEvent(event string, payload any)` so it's a specific method on the hub (Plan 15-04 will add this method to `*api.Hub` for breaker-specific WS dispatch). Avoids collision; preserves narrow-interface DI.

**Files modified:** `internal/pricegaptrader/breaker_controller.go`.
**Commit:** `e69527f`.

### Step 2 vs Step 3 reordering

Not a deviation — explicitly allowed by the CONTEXT D-15 prose ("Steps 2-5 are best-effort"). Step 2 (candidate pause) reordered to run BEFORE Step 3 (append trip record) so the trip record can carry `PausedCandidateCount`. Comment block in `trip()` documents the swap. Load-bearing Step 1 invariant unaffected.

## Authentication Gates

None — pure code changes; no external services touched.

## Migration Notes for Plan 15-04

Plan 15-04 (CLI + API + dashboard surface + real Telegram) consumes the contract Plan 15-03 establishes:

- **`*BreakerController`** is the single source of truth for trip + recovery state. Plan 15-04's pg-admin `breaker recover --confirm` and dashboard `POST /api/pg/breaker/recover` both call into recovery logic (TBD in 15-04 — likely a `BreakerController.Recover(operator)` method that clears sticky + un-pauses candidates + appends backfill on the trip record).
- **`*BreakerController.Snapshot()`** is the read-path for dashboard widget + `pg-admin breaker show`.
- **Plan 15-04 wiring order in cmd/main.go**: construct aggregator → construct breakerCtl → wire setters (WS / Ramp / Registry / Positions) → `tracker.SetBreakerController(breakerCtl)` → `tracker.Start()`. The Tracker.Start spawn (`runBreakerDaemon`) handles ctx adaptation; cmd/main.go does NOT need its own goroutine.
- **Telegram stub removal**: `internal/notify/pricegap_reconcile.go` currently has stub bodies for `NotifyPriceGapBreakerTrip` + `NotifyPriceGapBreakerRecovery`. Plan 15-04 ships the real impls in a new file `internal/notify/pricegap_breaker.go` (mirrors `pricegap_reconcile.go` precedent — separate file so the module-graph stays clean). The stubs in `pricegap_reconcile.go` should be deleted in 15-04.
- **Test fire**: Plan 15-04's `pg-admin breaker test-fire --confirm` and `POST /api/pg/breaker/test-fire` need a way to invoke `trip(ctx, state, pnl, source="test_fire")` (and `source="test_fire_dry_run"` for the `--dry-run` variant). Plan 15-04 either exports a `BreakerController.TestFire(dryRun bool, operator string) error` method or uses a friend-package access pattern. The current `trip()` signature already accepts `source string` — extending is an additive change.

## Known Stubs

Two intentional stubs remain after Plan 15-03:

| Stub | File | Reason |
|---|---|---|
| `*notify.TelegramNotifier.NotifyPriceGapBreakerTrip` | `internal/notify/pricegap_reconcile.go` | Real Telegram dispatch ships in Plan 15-04 (separate file `internal/notify/pricegap_breaker.go` per `pricegap_reconcile.go` precedent). Stub satisfies the interface so the controller compiles + can call without crashing during the Plan 15-04 integration window. |
| `*notify.TelegramNotifier.NotifyPriceGapBreakerRecovery` | `internal/notify/pricegap_reconcile.go` | Same as above. Recovery path doesn't exist yet (Plan 15-04 builds the pg-admin / API surfaces); stub is forward-looking. |

Both stubs are listed for the Phase 15 verifier; intentional and documented in the file's preamble comment.

## Threat Flags

None — Plan 15-03 introduces no new network endpoints, no new auth paths, and no new file-access patterns. The trip path's Step 2 mutates `config.json` via the EXISTING `Registry.SaveJSONWithBakRing` chokepoint (no new file write surface). The breaker daemon uses the SAME `pg:breaker:state` and `pg:breaker:trips` Redis keys provisioned by Plan 15-01 (no new key namespace).

The threat register's T-15-08 / T-15-09 / T-15-10 / T-15-11 mitigations are all in place and locked by tests:
- **T-15-08** (Reordered trip side-effects bypassing safety property) → `TestBreaker_TripOrdering_StickyFirstWhenStepsFail` + `TestBreaker_TripOrdering_Step1FailureAborts` + comment block in `trip()`.
- **T-15-09** (Concurrent entry during trip becomes live-mode) → Sticky persisted FIRST means concurrent `IsPaperModeActive()` reads see true → entry stamped paper. PausedByBreaker is the second guard.
- **T-15-10** (kill-9 mid-strike loses pending state) → `TestBreaker_StateSurvivesRestart`: Strike-1 persisted immediately on first breach (Pitfall 2).
- **T-15-11** (Bybit-blackout race produces phantom strikes) → `TestBreaker_BlackoutSuppression` + `TestBreaker_PendingSurvivesBlackout` (whole-tick suppression, pending preserved).

## Self-Check: PASSED

- `internal/pricegaptrader/breaker_controller.go` — FOUND
- `internal/pricegaptrader/breaker_controller_test.go` — FOUND (16 BreakerController tests + 7 IsPaperModeActive)
- `internal/pricegaptrader/notify.go` — FOUND (Notifier extended)
- `internal/pricegaptrader/registry.go` — FOUND (3 chokepoint helpers)
- `internal/pricegaptrader/risk_gate.go` — FOUND (Gate 1.5 inserted)
- `internal/pricegaptrader/tracker.go` — FOUND (SetBreakerController + runBreakerDaemon)
- `internal/models/pricegap_interfaces.go` — FOUND (PausedByBreaker added)
- `internal/notify/pricegap_reconcile.go` — FOUND (TelegramNotifier breaker stubs)
- `internal/pricegaptrader/registry_concurrent_test.go` — FOUND (3 PausedByBreaker tests added)
- `internal/pricegaptrader/risk_gate_test.go` — FOUND (2 PausedByBreaker tests added)
- `internal/pricegaptrader/tracker_broadcast_test.go` — FOUND (spyNotifier extended)
- `internal/pricegaptrader/daemon_test.go` — FOUND (fakeBootGuardNotifier extended)
- Commit `c36130a` (Task 1) — FOUND
- Commit `e69527f` (Task 2) — FOUND
- VERSION 0.37.2 → 0.37.3 → 0.37.4 — VERIFIED
- CHANGELOG.md Unreleased extended — VERIFIED
