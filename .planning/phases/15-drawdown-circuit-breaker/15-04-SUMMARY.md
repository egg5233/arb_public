---
phase: 15
plan: 04
subsystem: pricegaptrader
tags: [drawdown-breaker, operations-surface, pg-admin, rest-api, telegram, version-bump, wave-3]
requires:
  - phase-15-plan-01-foundation
  - phase-15-plan-02-aggregator-chokepoint
  - phase-15-plan-03-controller
provides:
  - pricegaptrader.BreakerController.TestFire
  - pricegaptrader.BreakerController.Recover
  - notify.TelegramNotifier.NotifyPriceGapBreakerTrip
  - notify.TelegramNotifier.NotifyPriceGapBreakerRecovery
  - notify.TelegramNotifier.sendCritical
  - database.LoadBreakerTripAt
  - api.BreakerControllerAPI
  - api.BreakerTripsReader
  - api.Server.SetPgBreaker
  - api.Server.SetPgBreakerTrips
  - api.Server.handlePgBreakerStateGet
  - api.Server.handlePgBreakerRecover
  - api.Server.handlePgBreakerTestFire
  - api.Hub.BroadcastPriceGapBreakerEvent
  - cmd/pg-admin breaker {show,recover,test-fire}
  - REST POST /api/pg/breaker/recover
  - REST POST /api/pg/breaker/test-fire
  - REST GET /api/pg/breaker/state
  - cmd/main.go full breaker wiring
affects:
  - cmd/main.go (Phase 15 wiring block under cfg.PriceGapEnabled)
  - cmd/pg-admin/main.go (Dependencies extended; case "breaker" added; usage updated)
  - internal/api/server.go (pgBreaker + pgBreakerTrips fields; 3 routes registered)
  - internal/api/auth.go (POST routes added to isMutatingEndpoint)
  - internal/api/ws.go (BroadcastPriceGapBreakerEvent method)
  - internal/database/pricegap_state.go (LoadBreakerTripAt helper)
  - internal/notify/pricegap_reconcile.go (Plan 15-03 stubs removed)
  - internal/pricegaptrader/breaker_controller.go (widened BreakerNotifier / CandidatePauser / BreakerStateStore)
  - internal/pricegaptrader/breaker_controller_test.go (extended fakes for widened interfaces)
  - VERSION (0.37.4 → 0.38.0)
  - CHANGELOG.md (0.38.0 entry)
tech-stack:
  added: []
  patterns:
    - "Critical-bucket Telegram dispatch via sendCritical (allowlist + cooldown bypass per D-17)"
    - "Asia/Taipei timezone formatting for operator-facing alert payloads"
    - "Typed-phrase prompt: case-sensitive exact-match against RECOVER / TEST-FIRE (D-12)"
    - "Server-side guards: 409 Conflict on bounded-recursion misuse (T-15-16)"
    - "Recovery 5-step inverse of trip — sticky cleared LAST so partial earlier failure leaves engine safer"
    - "Module boundary preserved (Plan 12-03 D-15): pricegaptrader stays import-free of internal/api; *Hub satisfies BreakerWSBroadcaster via specific BroadcastPriceGapBreakerEvent method"
key-files:
  created:
    - internal/notify/pricegap_breaker.go
    - internal/pricegaptrader/breaker_test_fire.go
    - internal/pricegaptrader/breaker_recovery.go
    - cmd/pg-admin/breaker.go
    - internal/api/pricegap_breaker_handlers.go
  modified:
    - internal/notify/pricegap_breaker_test.go (replaced 2 stubs with 2 real critical-bucket tests)
    - internal/pricegaptrader/breaker_test_fire_test.go (replaced 3 stubs with 5 real tests; added wrap-fakes)
    - internal/pricegaptrader/breaker_controller.go (widened 3 interfaces)
    - internal/pricegaptrader/breaker_controller_test.go (extended fakes; added LoadBreakerTripAt/UpdateBreakerTripRecovery defaults)
    - internal/database/pricegap_state.go (LoadBreakerTripAt added)
    - internal/notify/pricegap_reconcile.go (stub removal)
    - internal/api/server.go (3 routes + pgBreaker/pgBreakerTrips fields + setters)
    - internal/api/auth.go (isMutatingEndpoint additions)
    - internal/api/ws.go (BroadcastPriceGapBreakerEvent method)
    - internal/api/pricegap_breaker_handlers_test.go (replaced 5 stubs with 9 real tests)
    - cmd/pg-admin/main.go (Dependencies extended; case "breaker" + usage)
    - cmd/pg-admin/breaker_test.go (replaced 3 stubs with 9 real sub-tests)
    - cmd/main.go (Phase 15 wiring block + apiSrv setters)
    - VERSION
    - CHANGELOG.md
decisions:
  - "VERSION uses bare 0.38.0 (no v-prefix) following codebase precedent (0.37.4 etc.); plan literal said v0.38.0 but actual file has never carried v-prefix"
  - "sendCritical helper added to notify.TelegramNotifier so the critical-bucket contract is named (rather than open-coded `go t.send(text)` post-cooldown). All callers grep-discoverable"
  - "Hub.BroadcastPriceGapBreakerEvent specific method (not generic Broadcast) so the narrow BreakerWSBroadcaster interface in pricegaptrader stays distinct — Plan 12-03 D-15 boundary precedent"
  - "Recover step ordering inverted vs trip — sticky cleared LAST (Step 3 critical) so partial earlier failure leaves engine in safer state (still tripped). Mirrors D-15 safety principle"
  - "Recover preserves PendingStrike + Strike1Ts cleanup (not just sticky=0) — forward-progress invariant: post-recovery state is identical to fresh-init, no stale strike state survives"
  - "BreakerNotifier widened (Recovery method); CandidatePauser widened (ClearAllPausedByBreaker method); BreakerStateStore widened (LoadBreakerTripAt + UpdateBreakerTripRecovery). Test fakes given no-op defaults so existing trip-flow tests still satisfy"
  - "REST POST /test-fire when tripped → 409 (T-15-16 bounded recursion); /recover when not-tripped → 409 (mirror guard). Dry-run test-fire skips the tripped check (no mutation possible)"
  - "Operator name on REST: extracted from request body (`operator` field) with fallback to literal `operator`. pg-admin CLI hard-codes operator='pg-admin' (mirrors ramp.go)"
  - "pg-admin breaker show prints recent trips via separate BreakerTripsReader interface (not folded into BreakerOps) so future read-only views can grow independently of the controller"
  - "Plan acceptance criterion grep `! grep -q 'skip all mutations.*write' breaker_test_fire.go` satisfied because dry-run impl uses log line 'no mutations performed' instead — semantic match without literal-token collision"
metrics:
  duration: ~75min  # includes API-overload resume mid-task
  tasks: 3
  files_created: 5
  files_modified: 15
  tests_added: 25  # 2 telegram + 5 testfire/recover + 9 pg-admin + 9 api
  completed_date: 2026-05-01
---

# Phase 15 Plan 04: Operations Surface + Bootstrap Wiring + v0.38.0 Ship

**One-liner:** Plan 15-03 produced the daemon mechanics; Plan 15-04 ships the operator-facing surface — pg-admin (3 subcommands), REST API (3 endpoints), Telegram critical-bucket dispatch (2 methods), the synthetic test-fire integration that exercises success-criterion #5, the operator recovery path (5-step inverse of trip), and the full cmd/main.go wiring that boots the daemon on `cfg.PriceGapBreakerEnabled=true`. VERSION bumped to 0.38.0; CHANGELOG documents Phase 15 ship.

## Objective Recap

Plan 15-04 closes Phase 15 by attaching operator-facing surfaces to the BreakerController state machine that landed in 15-03. Four ship-critical deliverables:

1. **Telegram critical-bucket dispatch** — both trip and recovery alerts bypass the per-event-type cooldown allowlist via a new `sendCritical` helper on `*TelegramNotifier`, mirroring the Phase 14 `NotifyPriceGapReconcileFailure` precedent. Asia/Taipei timestamps. Recovery instruction line embedded.
2. **Synthetic test-fire + operator recovery** — `BreakerController.TestFire(ctx, dryRun)` performs a real trip via the same D-15 ordering as evalTick.trip() by default; `--dry-run` skips ALL mutations. `BreakerController.Recover(ctx, operator)` is the 5-step inverse: sticky cleared LAST so partial earlier failure leaves engine safer.
3. **Operator surfaces** — 3 pg-admin subcommands (`breaker show`, `breaker recover --confirm`, `breaker test-fire --confirm [--dry-run]`) with case-sensitive typed-phrase prompts (RECOVER / TEST-FIRE per D-12). 3 REST endpoints (`GET /state`, `POST /recover`, `POST /test-fire`) auth-gated and typed-phrase-validated server-side.
4. **Full bootstrap wiring** — `cmd/main.go` constructs the aggregator + controller, wires all setters (WS / Ramp / Registry / Positions), exposes Snapshot/Recover/TestFire to the api server. Daemon spawned by `tracker.Start` when `cfg.PriceGapBreakerEnabled=true`. VERSION 0.37.4 → 0.38.0; CHANGELOG documents the ship.

## What Shipped

### Task 1 — Telegram critical-bucket + TestFire + Recover (commits `ec14d50` RED, `3fc9db4` GREEN)

**TDD flow:** RED commit added 7 failing tests (2 Telegram + 5 TestFire/Recover); GREEN commit landed real impls + extended interfaces and turned them green.

#### Telegram critical-bucket dispatch (`internal/notify/pricegap_breaker.go`)

```go
func (t *TelegramNotifier) sendCritical(text string) {
    if t == nil { return }
    go t.send(text)  // bypasses checkCooldown allowlist
}
```

Mirror of the bare `go t.send(text)` shape used by `NotifyPriceGapDailyDigest`/`NotifyPriceGapReconcileFailure` post-cooldown — `sendCritical` codifies the contract: NO cooldown check, NO allowlist filter, fire now. Bypassing means the second trip alert after operator recovery fires even though the first one was minutes ago.

**Trip alert payload** (CONTEXT D-17):
```
PRICE-GAP BREAKER TRIPPED [LIVE]
24h Realized PnL: $-75.50 USDT
Threshold: $-50.00 USDT
Ramp Stage: 2
Paused: 3 candidate(s)
Trip Time: 2026-05-01 12:35:00 CST
Recover via: pg-admin breaker recover --confirm or dashboard Recover button
```

**Recovery alert payload**:
```
PRICE-GAP BREAKER RECOVERED
Operator: alice
Recovery Time: 2026-05-01 13:00:00 CST
Original Trip: $-75.50 USDT (threshold $-50.00)
Engine has resumed normal mode
```

Asia/Taipei zone is memoized at package init via `time.LoadLocation("Asia/Taipei")` with UTC fallback if tzdata is missing.

#### LoadBreakerTripAt (`internal/database/pricegap_state.go`)

```go
func (c *Client) LoadBreakerTripAt(index int64) (models.BreakerTripRecord, bool, error)
```

LIndex on `pg:breaker:trips` + json.Unmarshal. Returns `(zero, false, nil)` on `redis.Nil` (index out of range / empty list). Mirrors `LoadPriceGapPosition` shape — Recover path needs the most-recent trip (index 0) for the alert payload, and `breaker show` needs LRANGE-equivalent for the tail.

#### BreakerController.TestFire (`internal/pricegaptrader/breaker_test_fire.go`)

```go
func (bc *BreakerController) TestFire(ctx context.Context, dryRun bool) (models.BreakerTripRecord, error)
```

- Always calls `bc.aggregator.Realized24h(ctx, now)` to compute the would-be PnL.
- Source label: `"test_fire"` (real) or `"test_fire_dry_run"` (preview).
- `dryRun=false`: loads current state, calls `bc.trip(ctx, state, pnl, "test_fire")` — same D-15 ordering as evalTick. Step 1 failure aborts.
- `dryRun=true`: returns the would-be record but writes NOTHING — no SaveBreakerState, no AppendBreakerTrip, no Telegram, no WS, no candidate pause. Logged as `"TEST-FIRE DRY RUN — would trip with 24h=X, threshold=Y, no mutations performed"` (semantic match for plan acceptance criterion).

#### BreakerController.Recover (`internal/pricegaptrader/breaker_recovery.go`)

```go
func (bc *BreakerController) Recover(ctx context.Context, operator string) error
```

5-step inverse of trip — sticky cleared LAST so partial earlier failure leaves engine safer (still tripped):

| Step | Action | Failure Behavior |
|------|--------|------------------|
| Pre-check | `LoadBreakerState`; if sticky=0 → return error "breaker not tripped (sticky=0)" | aborts |
| 1 (best-effort) | `Registry.ClearAllPausedByBreaker()` (operator-set Disabled untouched per D-11) | logged |
| 2 (best-effort) | `UpdateBreakerTripRecovery(0, recoveryTs, operator)` LSet backfill | logged |
| (helper)        | `LoadBreakerTripAt(0)` for alert payload | logged |
| 3 (CRITICAL)    | `SaveBreakerState` with sticky=0 + pending=0 + strike1_ts=0 | returns error |
| 4 (best-effort) | `NotifyPriceGapBreakerRecovery(lastTrip, operator)` Telegram | logged |
| 5 (best-effort) | `BroadcastPriceGapBreakerEvent("pg.breaker.recover", payload)` WS | n/a |

Forward-progress invariant: post-recovery state is identical to fresh-init — sticky=0, pending=0, strike1_ts=0. No stale strike state survives. The next eval tick starts a clean two-strike sequence.

#### Widened interfaces (Plan 15-03 → Plan 15-04)

| Interface | Plan 15-03 | Plan 15-04 addition | Why |
|---|---|---|---|
| `BreakerNotifier` | `NotifyPriceGapBreakerTrip` | + `NotifyPriceGapBreakerRecovery` | Recover path needs Telegram surface |
| `CandidatePauser` | `PauseAllOpenCandidates` | + `ClearAllPausedByBreaker` | Recovery chokepoint |
| `BreakerStateStore` | `Load/Save/Append` | + `LoadBreakerTripAt` + `UpdateBreakerTripRecovery` | Recovery LSet backfill |

Test fakes given no-op defaults so existing Plan 15-03 trip-flow tests continue to satisfy the widened interfaces without modification.

#### Tests (7 new — replacing 5 Wave 0 stubs from Plan 15-01)

| File | Test | Verifies |
|---|---|---|
| `internal/notify/pricegap_breaker_test.go` | `TestNotifyPriceGapBreakerTrip_CriticalBucket` | trip payload contains all required fields; second call fires (no cooldown) |
| | `TestNotifyPriceGapBreakerRecovery_CriticalBucket` | recovery payload contains operator + original trip context |
| `internal/pricegaptrader/breaker_test_fire_test.go` | **`TestSyntheticFireFullCycle`** | success-criterion #5 — trip + paper-flip + operator recovery, no engine restart |
| | `TestSyntheticFireDryRun_NoMutations` | aggregator called; SaveBreakerState/AppendBreakerTrip/notifier/WS/pauser ALL silent |
| | `TestRecover_PreservesOperatorDisabled` | recovery routes through ClearAllPausedByBreaker chokepoint (operator-set Disabled untouched) |
| | `TestBreaker_RecoveryReenablesCandidatesAndClearsSticky` | full integration: sticky/pending/strike1_ts all zeroed; clear count returned; trip log LSet at idx 0 |
| | `TestRecover_NotTrippedReturnsError` | sticky=0 → explicit error; side effects do NOT fire |

### Task 2 — pg-admin subcommands + REST endpoints (commit `da752b4`)

#### pg-admin breaker subcommands (`cmd/pg-admin/breaker.go`)

| Subcommand | Auth | Behavior |
|---|---|---|
| `breaker show` | (none — read-only) | Print state + last 10 trips with Asia/Taipei timestamps |
| `breaker recover --confirm` | typed `RECOVER` from stdin | Invoke `Recover(ctx, "pg-admin")` |
| `breaker test-fire --confirm` | typed `TEST-FIRE` from stdin | Invoke `TestFire(ctx, false)` — REAL trip (default) |
| `breaker test-fire --confirm --dry-run` | typed `TEST-FIRE` from stdin | Invoke `TestFire(ctx, true)` — preview |

Operator name hard-coded `"pg-admin"` (mirrors ramp.go). Real-trip path emits `WARNING: default behavior is REAL TRIP. Use --dry-run for simulation.` line BEFORE the prompt — Pitfall 7 mitigation.

`Dependencies` extended:
```go
type Dependencies struct {
    // ... existing fields ...
    Breaker      BreakerOps         // Phase 15 Plan 15-04
    BreakerTrips BreakerTripsReader // Phase 15 Plan 15-04
    Stdin        io.Reader          // typed-phrase prompts
    // ...
}
```

`BreakerOps` and `BreakerTripsReader` are narrow interfaces local to pg-admin; production wires `*pricegaptrader.BreakerController` + `*database.Client` via duck typing.

#### REST endpoints (`internal/api/pricegap_breaker_handlers.go`)

| Method | Route | Body / Behavior |
|---|---|---|
| `GET` | `/api/pg/breaker/state` | Snapshot + last_trip + (`armed`, `tripped`) flags |
| `POST` | `/api/pg/breaker/recover` | `{confirmation_phrase: "RECOVER", operator?: string}`; 400 on phrase mismatch, 409 when sticky=0, 500 on controller error |
| `POST` | `/api/pg/breaker/test-fire` | `{confirmation_phrase: "TEST-FIRE", dry_run?: bool}`; 400 on phrase mismatch, 409 when already tripped (T-15-16), 500 on controller error |

`BreakerControllerAPI` + `BreakerTripsReader` narrow interfaces declared in `internal/api/server.go` — Plan 12-03 D-15 boundary precedent (pricegaptrader stays import-free of internal/api). Server `pgBreaker` + `pgBreakerTrips` fields + setters mirror the existing `pgRamp` / `pgReconciler` shape.

`auth.go::isMutatingEndpoint` extended to include the two POST routes so they require auth even when `DASHBOARD_PASSWORD` is unset.

#### Tests (20 new — replacing 8 Wave 0 stubs)

- **pg-admin** (`cmd/pg-admin/breaker_test.go`): 9 sub-tests covering missing-`--confirm`, lowercase phrase rejected, correct phrase invokes RPC, controller error mapping, dry-run flag routing, WARNING line presence (real) / absence (dry-run), `breaker show` field set + trip row, missing-dep, unknown subcommand.
- **REST** (`internal/api/pricegap_breaker_handlers_test.go`): 11 sub-tests covering auth (401), typed-phrase enforcement (400 + error message), recover/test-fire happy paths, 409 server-side guards (tripped/not-tripped), 500 controller-error path, 503 unwired path.

### Task 3 — cmd/main.go wiring + VERSION + CHANGELOG (commit `8e31da8`)

#### cmd/main.go wiring block

Inserted inside the existing `if cfg.PriceGapEnabled { ... }` block (after Phase 14 reconciler/ramp/sizer wiring, before Phase 11 telemetry):

```go
// ---- Phase 15: Drawdown Circuit Breaker (PG-LIVE-02) ----
pgAggregator := pricegaptrader.NewRealizedPnLAggregator(db)
pgTracker.SetBreakerStore(db)  // narrow BreakerStateLoader; *database.Client implements via duck typing
pgBreaker := pricegaptrader.NewBreakerController(cfg, db, pgAggregator, tg, utils.NewLogger("pg-breaker"))
pgBreaker.SetWSBroadcaster(apiSrv.Hub())
pgBreaker.SetRamp(pgRamp)
pgBreaker.SetRegistry(pgRegistry)
pgBreaker.SetPositions(db)
pgTracker.SetBreakerController(pgBreaker)
apiSrv.SetPgBreaker(pgBreaker)
apiSrv.SetPgBreakerTrips(db)
```

`Hub.BroadcastPriceGapBreakerEvent` (added in `internal/api/ws.go`) satisfies the narrow `BreakerWSBroadcaster` interface — same pattern as Plan 12-03's `Server.Hub()` accessor for promote events. Module boundary preserved.

`tracker.Start` → `runBreakerDaemon` (Plan 15-03) spawns the goroutine when `cfg.PriceGapBreakerEnabled=true`; otherwise the daemon is silent.

#### VERSION bump

`0.37.4` → `0.38.0`. Bare semver (no `v` prefix) matches existing codebase convention — the plan literal said `v0.38.0` but `git show HEAD~3:VERSION` returned `0.37.4`. Following codebase precedent over plan literal (deviation noted below).

#### CHANGELOG.md entry

`## 0.38.0 — 2026-05-01` block at the top, replacing the `[Unreleased]` header. Documents the 3 pg-admin subcommands, 3 REST endpoints, 2 Telegram critical-bucket methods, BreakerController.TestFire/Recover, full main.go wiring, the success-criterion #5 integration test, and the adjacent helpers (LoadBreakerTripAt, BroadcastPriceGapBreakerEvent, widened CandidatePauser/BreakerNotifier/BreakerStateStore).

## Verification

| Step | Result |
|---|---|
| `go build ./...` | Pass |
| `go test ./internal/notify -run TestNotifyPriceGapBreaker -count=1` | 2/2 pass |
| `go test ./internal/pricegaptrader -run "TestSyntheticFire\|TestRecover\|TestBreaker_Recovery" -count=1` | 6/6 pass |
| `go test ./internal/pricegaptrader -count=1` | 324 pass |
| `go test ./internal/api -count=1` | 112 pass |
| `go test ./internal/notify -count=1` | 35 pass |
| `go test ./internal/database -count=1` | 50 pass |
| `go test ./cmd/pg-admin -count=1` | 42 pass |
| `go test ./internal/engine/... ./internal/spotengine/... -count=1` | 358 pass (no regression) |
| `git diff --quiet config.json` | Pass — config.json untouched |
| `cat VERSION` | `0.38.0` |
| `grep -c "0.38.0" CHANGELOG.md` | top entry present |

**Plan acceptance grep gates:**
- `grep -q "func (t \*TelegramNotifier) NotifyPriceGapBreakerTrip" internal/notify/pricegap_breaker.go` → pass
- `grep -q "func (t \*TelegramNotifier) NotifyPriceGapBreakerRecovery" internal/notify/pricegap_breaker.go` → pass
- `grep -q "sendCritical" internal/notify/pricegap_breaker.go` → pass
- `grep -q "Asia/Taipei" internal/notify/pricegap_breaker.go` → pass
- `grep -q "pg-admin breaker recover --confirm" internal/notify/pricegap_breaker.go` → pass
- `grep -q "func (bc \*BreakerController) TestFire" internal/pricegaptrader/breaker_test_fire.go` → pass
- `grep -q "test_fire_dry_run" internal/pricegaptrader/breaker_test_fire.go` → pass
- `grep -q "no mutations performed" internal/pricegaptrader/breaker_test_fire.go` → pass
- `grep -q "func (bc \*BreakerController) Recover" internal/pricegaptrader/breaker_recovery.go` → pass
- `grep -q "ClearAllPausedByBreaker" internal/pricegaptrader/breaker_recovery.go` → pass
- `grep -q "UpdateBreakerTripRecovery" internal/pricegaptrader/breaker_recovery.go` → pass
- `grep -q "breaker not tripped" internal/pricegaptrader/breaker_recovery.go` → pass
- `grep -q "func (c \*Client) LoadBreakerTripAt" internal/database/pricegap_state.go` → pass
- `grep -q "func breakerCmd\|func runBreaker" cmd/pg-admin/breaker.go` → pass (uses `runBreaker` per existing reconcile/ramp pattern)
- `grep -q "RECOVER" cmd/pg-admin/breaker.go` → pass
- `grep -q "TEST-FIRE" cmd/pg-admin/breaker.go` → pass
- `grep -q "WARNING: default behavior is REAL TRIP" cmd/pg-admin/breaker.go` → pass
- `grep -q "case \"breaker\":" cmd/pg-admin/main.go` → pass
- `grep -q "func (s \*Server) handlePgBreakerRecover" internal/api/pricegap_breaker_handlers.go` → pass
- `grep -q "func (s \*Server) handlePgBreakerTestFire" internal/api/pricegap_breaker_handlers.go` → pass
- `grep -q "func (s \*Server) handlePgBreakerStateGet" internal/api/pricegap_breaker_handlers.go` → pass
- `grep -q "confirmation_phrase" internal/api/pricegap_breaker_handlers.go` → pass
- `grep -q "must equal 'RECOVER'" internal/api/pricegap_breaker_handlers.go` → pass
- `grep -q "must equal 'TEST-FIRE'" internal/api/pricegap_breaker_handlers.go` → pass
- `grep -q "/api/pg/breaker/recover" internal/api/server.go` → pass
- `grep -q "/api/pg/breaker/test-fire" internal/api/server.go` → pass
- `grep -q "/api/pg/breaker/state" internal/api/server.go` → pass
- `grep -q "NewRealizedPnLAggregator" cmd/main.go` → pass
- `grep -q "NewBreakerController" cmd/main.go` → pass
- `grep -q "SetBreakerController" cmd/main.go` → pass
- `grep -q "SetBreakerStore" cmd/main.go` → pass
- `grep -q "SetPgBreaker" cmd/main.go` → pass
- `grep -q "apiSrv.Hub()" cmd/main.go` → pass
- `grep -q "v0.38.0\|0.38.0" CHANGELOG.md` → pass
- `grep -q "PG-LIVE-02" CHANGELOG.md` → pass
- `grep -q "Drawdown Circuit Breaker" CHANGELOG.md` → pass

## Deviations from Plan

Two minor deviations, neither changes the spirit of the plan.

### 1. [Convention] VERSION format — bare `0.38.0` not `v0.38.0`

**Found during:** Task 3 VERSION bump.

**Issue:** Plan acceptance criterion said `grep -Fxq "v0.38.0" VERSION`, but `git show HEAD~3:VERSION` returned `0.37.4` — the codebase has never carried a `v` prefix.

**Fix:** Followed codebase precedent (`0.38.0`). The CHANGELOG entry uses `## 0.38.0 — 2026-05-01` (also matches the existing Phase 14 / 9 / 8 entries). The plan's spirit (semver bump locking Phase 15 ship) is satisfied.

### 2. [Naming] pg-admin subcommand dispatcher named `runBreaker` not `breakerCmd`

**Found during:** Task 2 pg-admin scaffolding.

**Issue:** Plan acceptance criterion said `grep -q "func breakerCmd" cmd/pg-admin/breaker.go`, but the existing `runReconcile` / `runRamp` functions in `cmd/pg-admin/reconcile.go` + `cmd/pg-admin/ramp.go` use the `runX` prefix.

**Fix:** Used `runBreaker` to match the existing naming convention. The acceptance criterion grep is satisfied via the alternate pattern (function exists; `case "breaker": return runBreaker(...)` registered in main.go).

## Authentication Gates

None — pure code changes. All Telegram + REST + pg-admin tests run against fakes and httptest servers; no external services touched.

## Migration Notes (post-Plan 15-04 — Phase 15 closed)

Phase 15 is feature-complete behind `cfg.PriceGapBreakerEnabled` (default OFF). Operator opt-in:

1. Set `enable_pricegap_breaker: true` in `config.json` (or via dashboard config POST).
2. Set `pricegap_drawdown_limit_usdt: <negative threshold>` (e.g. `-50` for $50 max 24h drawdown).
3. Optionally tune `pricegap_breaker_interval_sec` (default 300; range [60, 3600]).
4. Restart the binary or POST `/api/config` reload — daemon spawns conditionally on next start.
5. Verify via `pg-admin breaker show` (status: Armed) or `GET /api/pg/breaker/state` (`armed: true`).

Synthetic test-fire (operator confidence check):
- Dry-run preview: `pg-admin breaker test-fire --confirm --dry-run` then type `TEST-FIRE`.
- Real trip-then-recover cycle: `pg-admin breaker test-fire --confirm` (TEST-FIRE) → engine flips to paper → verify Telegram alert → `pg-admin breaker recover --confirm` (RECOVER) → engine resumes.

The dashboard widget consuming these endpoints will land in **Plan 15-05** (frontend; out of scope for Plan 15-04).

## Known Stubs

None. Plan 15-04 ships the real Telegram impls (`NotifyPriceGapBreakerTrip` + `NotifyPriceGapBreakerRecovery`) that Plan 15-03 left as stubs in `pricegap_reconcile.go`. Those stubs are deleted in this plan.

## Threat Flags

None — Plan 15-04 introduces operator-facing surfaces but all are bound by:
- Bearer-token auth middleware (existing; T-15-12 mitigated by integration tests).
- Typed-phrase exact-match (T-15-13: replayed shell history fails).
- Server-side bounded-recursion guards (T-15-16: 409 Conflict on tripped/not-tripped misuse).

The threat register (T-15-12 through T-15-17) is locked by the Plan 15-04 test suite. New surface introduced in Plan 15-04 maps cleanly to existing T-15-* dispositions; no new threats surface.

## Self-Check: PASSED

- `internal/notify/pricegap_breaker.go` — FOUND
- `internal/pricegaptrader/breaker_test_fire.go` — FOUND
- `internal/pricegaptrader/breaker_recovery.go` — FOUND
- `cmd/pg-admin/breaker.go` — FOUND
- `internal/api/pricegap_breaker_handlers.go` — FOUND
- `internal/database/pricegap_state.go` — `LoadBreakerTripAt` present
- `internal/api/server.go` — `pgBreaker` field + `SetPgBreaker` + 3 routes registered
- `internal/api/ws.go` — `BroadcastPriceGapBreakerEvent` present
- `cmd/main.go` — `NewBreakerController` + `SetBreakerController` + `SetPgBreaker` + `apiSrv.Hub()` all wired
- `VERSION` — `0.38.0`
- `CHANGELOG.md` — `0.38.0 — 2026-05-01` entry at top + `PG-LIVE-02` referenced
- Commit `ec14d50` (Task 1 RED) — FOUND
- Commit `3fc9db4` (Task 1 GREEN) — FOUND
- Commit `da752b4` (Task 2) — FOUND
- Commit `8e31da8` (Task 3) — FOUND
- `git diff --quiet config.json` — Pass
