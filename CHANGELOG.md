# Changelog

All notable changes to this project will be documented in this file.

## 0.38.1 — 2026-05-02

### Phase 16 Plan 01 — Paper-mode realized-slippage fix (PG-FIX-01)

**Bug fix.** The Phase-9 `RealizedSlipBps = ModeledSlipBps` paper-mode override at `internal/pricegaptrader/monitor.go:242-244` (originally PG-VAL-03 v0.34.10 closure) is removed. The slip formula now uses a true exit-side mid (`LongMidAtExit` / `ShortMidAtExit`, captured from live BBO at close with a `*MidAtDecision` fallback when BBO is stale) as the exit reference, and sums magnitudes via `math.Abs` so paper-mode synth contributions are additive. Realized slippage in paper mode is now non-zero and reflects modeled drift + actual mid movement between decision and close — Pitfall-7-class machine-zero closed.

#### Added

- `models.PriceGapPosition.LongMidAtExit` + `.ShortMidAtExit` fields with `omitempty` json tags (legacy decode safe).
- `internal/pricegaptrader/realized_slip_test.go` regression tests: `TestRealizedSlip_PaperNonZero` (paper close with mid drift produces non-zero slip ≠ ModeledSlipBps), `TestRealizedSlip_PaperBBOStaleFallback` (BBO-stale fallback collapses slip to ModeledSlipBps), `TestRealizedSlip_LiveStillWorks` (live formula unchanged, no override leak).

#### Changed

- `internal/pricegaptrader/monitor.go` `closePair` samples `sampleLegs` BBO before placing the close legs and stamps `LongMidAtExit` / `ShortMidAtExit` (D-01); `*MidAtDecision` fallback applies when `sampleLegs` returns an error or zero mid (Pitfall 10).
- Realized-slip formula exit term now references `*MidAtExit` (entry term continues to use `*MidAtDecision`); magnitudes summed via `math.Abs` per leg.

#### Removed

- Paper-mode override block at `monitor.go:242-244` and its preceding `PG-VAL-03 v0.34.10 closure` comment.
- Synth path in `execution.go` is intentionally UNCHANGED (D-04 — fix is in measurement, not synthesis). Paper-mode chokepoints in `placeCloseLegIOC` + `closeLegMarketForPos` remain (required for paper synth fills).

#### Reconciler

- `internal/pricegaptrader/reconciler.go` anomaly detector at `:268` (compares against `cfg.PriceGapAnomalySlippageBps`) automatically benefits from real values; no reconciler change required (D-03).

## 0.38.0 — 2026-05-01

### Phase 15 — Drawdown Circuit Breaker (PG-LIVE-02)

**New feature, default OFF.** Strategy 4 gains a realized-PnL drawdown breaker that auto-flips the engine to paper mode when rolling 24h realized PnL drops below `pricegap_drawdown_limit_usdt`. Bybit `:04-:05:30` blackout suppresses evaluation. Two-strike rule (≥5 min apart). Recovery requires explicit operator action via `pg-admin breaker recover --confirm` or dashboard Recover button.

#### Added (Plan 15-04 — operations surface, version-bump ship)

- **3 new pg-admin subcommands** (`cmd/pg-admin/breaker.go`):
  - `pg-admin breaker show` — read-only status + last 10 trip records (Asia/Taipei timestamps).
  - `pg-admin breaker recover --confirm` — typed-phrase-guarded operator recovery; prompts for literal `RECOVER` from stdin (case-sensitive) before invoking `BreakerController.Recover(ctx, "pg-admin")`.
  - `pg-admin breaker test-fire --confirm [--dry-run]` — typed-phrase-guarded synthetic fire; prompts for `TEST-FIRE`. Default behavior is REAL TRIP (operator sees `WARNING: default behavior is REAL TRIP` line); `--dry-run` opts into preview-only (no mutations).
- **3 new REST endpoints** (`internal/api/pricegap_breaker_handlers.go`), all bearer-auth gated:
  - `GET /api/pg/breaker/state` — snapshot + most-recent trip + derived `armed`/`tripped` flags.
  - `POST /api/pg/breaker/recover` — body `{confirmation_phrase: "RECOVER", operator?: string}`. 400 on phrase mismatch, 409 when sticky=0, 500 on controller error.
  - `POST /api/pg/breaker/test-fire` — body `{confirmation_phrase: "TEST-FIRE", dry_run?: bool}`. 400 on phrase mismatch, 409 when already tripped (T-15-16 bounded recursion), 500 on controller error.
- **2 new Telegram critical-bucket methods** (`internal/notify/pricegap_breaker.go`):
  - `NotifyPriceGapBreakerTrip(record)` — D-17 critical alert with 24h PnL, threshold, ramp stage, paused candidate count, Asia/Taipei trip time, recovery instruction line. Bypasses cooldown via new `sendCritical` helper.
  - `NotifyPriceGapBreakerRecovery(record, operator)` — recovery alert with operator name, recovery time, original trip context.
- **`BreakerController.TestFire(ctx, dryRun)`** (`internal/pricegaptrader/breaker_test_fire.go`):
  - `dryRun=false` → REAL trip via the same D-15 ordering as evalTick; source label `test_fire`.
  - `dryRun=true` → computes 24h PnL but writes NOTHING (no SaveBreakerState, no AppendBreakerTrip, no Telegram, no WS, no candidate pause); source label `test_fire_dry_run`.
- **`BreakerController.Recover(ctx, operator)`** (`internal/pricegaptrader/breaker_recovery.go`):
  - 5-step inverse of trip — sticky cleared LAST so partial earlier failure leaves engine in safer state (still paper).
  - Step 1: `Registry.ClearAllPausedByBreaker()` (operator-set `Disabled` untouched per D-11).
  - Step 2: `UpdateBreakerTripRecovery(0, ts, op)` LSet backfill on most-recent trip.
  - Step 3 (CRITICAL): `SaveBreakerState` with sticky=0 + pending=0 + strike1_ts=0; failure returns error.
  - Step 4: Telegram critical-bucket recovery alert.
  - Step 5: WS broadcast `pg.breaker.recover` with operator + recovery_ts + trip payload.
  - Returns explicit `breaker not tripped (sticky=0)` error when called against an armed-but-not-tripped state.
- **`*database.Client.LoadBreakerTripAt(idx)`** (`internal/database/pricegap_state.go`) — `(record, exists, err)` shape mirrors `LoadPriceGapPosition` (Phase 14); used by Recover for alert payload + by `breaker show` for the recent-trips tail.
- **`Server.SetPgBreaker` / `Server.SetPgBreakerTrips` + `BreakerControllerAPI` / `BreakerTripsReader` narrow interfaces** in `internal/api/server.go` — same Plan 12-03 D-15 boundary precedent (pricegaptrader stays import-free of internal/api).
- **`Hub.BroadcastPriceGapBreakerEvent`** in `internal/api/ws.go` satisfies the narrow `BreakerWSBroadcaster` interface for production wiring.
- **`cmd/main.go` full bootstrap wiring** — aggregator + breaker controller + setters (WS / Ramp / Registry / Positions) + tracker.SetBreakerController + apiSrv.SetPgBreaker + apiSrv.SetPgBreakerTrips. Daemon spawned by tracker.Start when `cfg.PriceGapBreakerEnabled=true`.
- **Synthetic full-cycle integration test** (`TestSyntheticFireFullCycle`) — exercises trip → paper-flip → operator recovery in-process, no engine restart. Success-criterion #5 evidence.
- **Widened `CandidatePauser` interface** with `ClearAllPausedByBreaker()` so the recovery chokepoint goes through `*Registry`.
- **Widened `BreakerNotifier` interface** with `NotifyPriceGapBreakerRecovery(record, operator) error`.
- **Widened `BreakerStateStore` interface** with `LoadBreakerTripAt` + `UpdateBreakerTripRecovery` so Recover can read-modify-write the most-recent trip without a separate store interface.
- **`/api/pg/breaker/{recover,test-fire}` added to `isMutatingEndpoint`** (`internal/api/auth.go`) so the routes always require auth in production deployments where DASHBOARD_PASSWORD is unset.

#### Tests

- 7 unit tests in `internal/pricegaptrader` (Telegram + TestFire + Recover); 9 sub-tests in `cmd/pg-admin/breaker_test.go`; 11 sub-tests in `internal/api/pricegap_breaker_handlers_test.go`. Full Phase 15 test suite (Plans 01+02+03+04 combined) green; pricegaptrader (324) + notify (35) + database (50) + api (112) + cmd/pg-admin (42) all pass.

#### Migration

Existing operators set `enable_pricegap_breaker: true` + `pricegap_drawdown_limit_usdt: <negative threshold>` via dashboard config or pg-admin tooling — `config.json` reload not required for runtime toggle.

### Added

- **Phase 15 Plan 03 (PG-LIVE-02) — BreakerController state machine + D-15 trip ordering (Task 2):**
  - **`BreakerController`** (`internal/pricegaptrader/breaker_controller.go`) — drawdown circuit-breaker daemon. Run() → 5-min ticker → evalTick(). evalTick implements D-01 (5-min cadence) + D-03 (whole-tick Bybit blackout suppression — REUSES `inBybitBlackout` from scanner.go, no parallel impl) + D-04 (pending strike survives blackout) + D-05 (HASH persistence on every state change) + D-08 (PnL recovery clears Strike-1) + boot guard (missing state → fresh init, permissive unlike Phase 14 ramp) + defensive ≥5min check (`5*60*1000` ms) independent of ticker interval.
  - **D-15 trip ordering with load-bearing safety property** — Step 1 persists `PaperModeStickyUntil = math.MaxInt64` to `pg:breaker:state` FIRST. Step 1 failure aborts the trip (Steps 2-5 skipped, error returned). Steps 2-5 (candidate pause via chokepoint, AppendBreakerTrip, NotifyPriceGapBreakerTrip critical alert, WS broadcast) are best-effort with logged failures. Step 2 runs before Step 3 so the trip record carries `PausedCandidateCount`. Comment block in `trip()` locks the ordering against future regressions.
  - **5 narrow interfaces** for D-15 module-boundary preservation: `BreakerStateStore` (Load/Save/Append), `BreakerNotifier`, `BreakerWSBroadcaster`, `CandidatePauser`, `ActivePositionLister`. Production wires `*database.Client` + `*notify.TelegramNotifier` + `*api.Hub` + `*Registry`; tests inject in-memory fakes.
  - **`Tracker.SetBreakerController`** + daemon spawn — `Tracker.Start` launches `runBreakerDaemon` goroutine which adapts `t.stopCh` into a `context.Context` (BreakerController.Run expects ctx.Done()). nil-safe; spawned only when `*BreakerController` wired (Plan 15-04 wires production via cmd/main.go).
  - **15 unit tests** covering all D-01..D-15 behaviors:
    - `TestBreaker_DisabledByDefault` — daemon no-op when cfg disabled.
    - `TestBreaker_FreshBootInit` / `TestBreaker_BootGuard_PreservesExistingTrip` — missing-state vs sticky-MaxInt64 boot paths.
    - `TestBreaker_SingleStrike_NoTrip` / `TestBreaker_TwoStrikeTrips` — two-strike state machine.
    - `TestBreaker_TwoStrikeRequiresTwoSeparateEvaluations` — defensive <5min skip.
    - `TestBreaker_RecoveryClearsPendingStrike` — D-08 PnL recovery clear.
    - `TestBreaker_BlackoutSuppression` / `TestBreaker_PendingSurvivesBlackout` — D-03/D-04 blackout pair.
    - `TestBreaker_StateSurvivesRestart` — kill-9 mid-strike, fresh controller fires Strike-2 (Pitfall 2).
    - **`TestBreaker_TripOrdering_StickyFirstWhenStepsFail`** — load-bearing D-15 anchor: Steps 2-5 all fail, sticky still persisted.
    - **`TestBreaker_TripOrdering_Step1FailureAborts`** — Step 1 fail returns error, Steps 2-5 not invoked.
    - `TestBreaker_TripIncludesRampStage` / `TestBreaker_TripPausedCandidateCount` / `TestBreaker_WSBroadcastFiredOnTrip` / `TestBreaker_Snapshot` — record-population + broadcast + read-only Snapshot lockdown.

### Changed

- `internal/pricegaptrader/tracker.go` — `Start()` spawns the breaker daemon goroutine when `*BreakerController` is wired (in addition to existing scanner / reconcile / ramp daemons).

### Safety

- **D-15 trip atomicity locked by test.** `TestBreaker_TripOrdering_StickyFirstWhenStepsFail` deliberately fails Steps 2-5 and verifies the sticky flag persisted. Any future reorder of `trip()` will fail this test.
- **Bybit blackout suppression delegates to scanner.go.** No parallel implementation of `inBybitBlackout` — Pitfall 4 (single source of truth) enforced via grep gate.

- **Phase 15 Plan 03 (PG-LIVE-02) — Notifier + Candidate.PausedByBreaker + entry-path guard (Task 1):**
  - **`PriceGapNotifier` extended with 2 critical-bucket methods** — `NotifyPriceGapBreakerTrip(record)` and `NotifyPriceGapBreakerRecovery(record, operator)`. `NoopNotifier` and `*notify.TelegramNotifier` (stub bodies, real impl in Plan 15-04) satisfy the interface. `spyNotifier` (test double) extended for compile-time conformance.
  - **`models.PriceGapCandidate.PausedByBreaker bool`** (`json:"paused_by_breaker,omitempty"`) — Phase 15 D-10. Distinct from operator-set Redis disable (Phase 9 PG-RISK-03). Set by trip path (D-15 step 3); cleared by recovery path. JSON `omitempty` preserves byte-identity for legacy candidates.
  - **3 new chokepoint helpers on `*Registry`** — `SetPausedByBreaker(symbol, longExch, shortExch, direction, value)` (idempotent per-candidate); `PauseAllOpenCandidates(positions)` (bulk trip-path mutation, derives candidates from active position tuples); `ClearAllPausedByBreaker()` (bulk recovery, count of true→false flips). All 3 use the existing path-lock + cfg.Lock + reload-from-disk + SaveJSONWithBakRing pattern; rollback on persist failure; audit-log entries `pause_by_breaker` / `clear_paused_by_breaker` / `pause_all_open` / `clear_all_paused_by_breaker`.
  - **`risk_gate.go` Gate 1.5** — new entry-path check between Gate 1 (Redis exec-quality disabled) and Gate 2 (max concurrent). When `cand.PausedByBreaker=true`, returns `ErrPriceGapCandidateDisabled` with reason string `"paused_by_breaker"` (distinct from Gate 1's `"exec_quality: ..."` so operators distinguish). Telegram risk-block dispatched with gate=`paused_by_breaker`.
  - **5 new tests:** `TestRegistry_PausedByBreakerField` (JSON roundtrip + omitempty), `TestRegistry_PausedByBreaker_ConcurrentWrite` (100 goroutines, no torn writes), `TestRegistry_RecoveryClearsPausedByBreaker_PreservesDisabled` (D-11 — Redis disable untouched), `TestRiskGate_PausedByBreaker_RejectsWithDistinctReason`, `TestRiskGate_DisabledOR_PausedByBreaker_TruthTable` (4-cell truth table: both-false allow / disabled-only / paused-only / both-true Gate-1-first).

- **Phase 15 Plan 02 (PG-LIVE-02) — Aggregator + paper-mode chokepoint:**
  - **`RealizedPnLAggregator`** (`internal/pricegaptrader/breaker_aggregator.go`) — rolling 24h sum of `pos.RealizedPnL` over positions closed in `[now-24h, now]`. Reads the Phase 14 `pg:positions:closed:{YYYY-MM-DD}` SET indices (today ∪ yesterday); dedupes IDs across the two sets (clock-skew SADD edge); uses `ExchangeClosedAt` with `ClosedAt` fallback (mirrors `reconciler.go:255`); resilient to missing keys and load-not-found (skips, does not abort). 8 unit tests green.
  - **`Tracker.IsPaperModeActive(ctx)`** — single paper-mode chokepoint per Phase 15 D-07. Order: cfg flag short-circuit → nil-store legacy fallback → `LoadBreakerState` consultation. **Fail-safes to paper on Redis error** (Pitfall 8) — Redis outage during a real trip MUST NOT silently allow live trading. Sticky truth table: `0` (live), `MaxInt64` (sticky-until-operator), `now < sticky_until` (timed sticky active).
  - **`Tracker.SetBreakerStore`** + **`BreakerStateLoader`** narrow interface — preserves D-15 module boundary; `*database.Client` satisfies via existing `LoadBreakerState`. Plan 15-04 wires production.
  - **`execution.go` 4-site migration** — all four direct `cfg.PriceGapPaperMode` reads (lines 170 / 227 / 269 / 362) routed through `IsPaperModeActive`. Site labels (`entry-order-placement`, `entry-synth-fill`, `entry-bingx-guard`, `close-leg-paper`) appear in WARN logs on Redis-error fail-safe so operators can trace which path applied paper-mode silently.
  - **`TestStaticCheck_NoDirectPaperModeRead`** — regex-walk regression guard mirroring `scanner_static_test.go`. Forbids any non-`tracker.go` production file from reading `cfg.PriceGapPaperMode` directly. Mitigates T-15-05 (future paper-mode reader bypass).

### Changed

- `internal/pricegaptrader/execution.go` — paper-mode reads no longer touch `t.cfg.PriceGapPaperMode` directly; all routes through `Tracker.IsPaperModeActive`. Behavior is byte-identical when no breaker store is wired (legacy nil-store fallback to cfg flag); production wiring lands in Plan 15-04.

### Safety

- **Sticky paper-mode is now uncircumventable from within `internal/pricegaptrader/`.** Static test fails the build if any future code path reads `cfg.PriceGapPaperMode` outside the tracker.go helper.
- **Redis outage = paper, not live.** `IsPaperModeActive` returns `(true, err)` on `LoadBreakerState` error; caller logs at WARN. Operator sees outage in journalctl + Telegram.

- **Phase 15 Plan 01 (PG-LIVE-02) — Drawdown circuit breaker foundation:**
  - **3 new Config fields** — `PriceGapBreakerEnabled` (bool, default false), `PriceGapDrawdownLimitUSDT` (float64, default 0 / armed-but-never-trips), `PriceGapBreakerIntervalSec` (int, default 300 / 5-min ticker). All default OFF; JSON tags `enable_pricegap_breaker`, `pricegap_drawdown_limit_usdt`, `pricegap_breaker_interval_sec`.
  - **`validatePriceGapLive` extended** — when `PriceGapBreakerEnabled=true`, rejects positive limits (D-06: limit is absolute USDT, must be ≤ 0) and out-of-band intervals ([60, 3600]s). Disabled-state defaults bypass validation.
  - **2 new Redis namespace constants** — `pg:breaker:state` (5-field HASH per D-05) and `pg:breaker:trips` (LIST capped at 500 per D-18).
  - **2 new model structs** — `models.BreakerState` (5 fields including `PaperModeStickyUntil` int64 sentinel encoding) and `models.BreakerTripRecord` (8 fields including nullable `RecoveryTs`/`RecoveryOperator`).
  - **4 new database helpers** — `SaveBreakerState` / `LoadBreakerState` (HSet pipeline + decimal-string encoding to preserve `math.MaxInt64` sentinel), `AppendBreakerTrip` (LPush + LTrim 0 499), `UpdateBreakerTripRecovery` (LIndex + LSet to backfill recovery fields).
  - **7 Wave 0 stub test files** — 30 test names reserved across `internal/pricegaptrader/`, `internal/api/`, `internal/notify/`, `cmd/pg-admin/` so Plans 15-02 / 15-03 / 15-04 implement against pre-named test targets. All stubs compile and skip; `go build ./...` green.

### Safety

- **Default OFF:** breaker is dormant on existing installs; operator must explicitly toggle `enable_pricegap_breaker=true` and set a negative `pricegap_drawdown_limit_usdt` to arm.
- **`config.json` untouched:** Phase 15 Plan 01 is pure Go-struct + helper plumbing; runtime mutations land via `POST /api/config` in Plan 15-04.
- **Module boundary preserved:** all new code under `internal/config/`, `internal/database/`, `internal/models/`, plus test stubs in pricegap subpackages. Zero imports of `internal/engine` / `internal/spotengine`.

## [0.37.0] - 2026-04-30

### Added

- **Phase 14 (PG-LIVE-01 + PG-LIVE-03) — Daily reconcile daemon + live ramp controller (backend complete):**
  - **Reconcile daemon** — fires daily at UTC 00:30, aggregates closed Strategy 4 positions for the previous UTC date keyed by `(position_id, version)`, writes byte-identical `pg:reconcile:daily:{date}` payload (D-04 idempotency proven at `-count=100`), 3-retry on transient failure (5s/15s/30s, imitated from `internal/engine/exit.go:1119`), triple-fail dispatches `NotifyPriceGapReconcileFailure` and skips the day. Boot-time catchup runs immediately if Tracker starts past 01:00 UTC and yesterday is missing (RESEARCH Q14). Per-day timeout 10 min (T-14-14).
  - **Live ramp controller** — Redis-persisted state machine with 5 fields (`pg:ramp:state`), 3 stages (100 / 500 / 1000 USDT/leg at v2.2 defaults), asymmetric ratchet (PriceGapCleanDaysToPromote consecutive clean days = 1 step up, ANY single loss day demotes one stage AND zeroes the counter), bounded RampEvent LIST `pg:ramp:events` (RPUSH + LTRIM 500) for full audit trail.
  - **Risk Gate 6 (ramp)** — defense-in-depth layer 2: when `cfg.PriceGapLiveCapital=true`, both Sizer (caps at sizing call site BEFORE Gate enters) and `risk_gate.go` Gate 6 (independent `min(stage_size, hard_ceiling)` check) reject over-budget proposals. No-op in paper mode (D-07 paper/live parity preserved).
  - **`pg-admin` Phase 14 subcommands** — `reconcile run --date=YYYY-MM-DD`, `reconcile show --date=YYYY-MM-DD`, `ramp show`, `ramp reset --reason=...`, `ramp force-promote --reason=...`, `ramp force-demote --reason=...`. Force ops emit RampEvents with `operator="pg-admin"` for audit visibility.
  - **4 new Telegram dispatch methods on `*TelegramNotifier`** — `NotifyPriceGapDailyDigest` (non-critical, one-shot per UTC day, cooldown key `pg_digest:{date}`), `NotifyPriceGapReconcileFailure` (CRITICAL, cooldown key `pg_reconcile_failure:{date}`), `NotifyPriceGapRampDemote` (CRITICAL on capital reduction), `NotifyPriceGapRampForceOp` (CRITICAL on operator override).
  - **Boot guard** — `Tracker.Start` panics + dispatches `BOOT_GUARD` critical Telegram if `cfg.PriceGapLiveCapital=true` and `pg:ramp:state` is missing or invalid (`CurrentStage < 1`). Refuses to ramp without a valid state signal (CONTEXT "Specific Ideas" #6, T-14-04).
  - **Telegram allowlist extended** — `"ramp"` entry added to `priceGapGateAllowlist` so Gate-6 risk-block notifications surface via existing `NotifyPriceGapRiskBlock` plumbing.
  - **`*database.Client.LoadPriceGapPosition`** — new `(pos, exists, error)` adapter satisfies `pricegaptrader.ReconcileStore`; distinguishes missing-id (exists=false, err=nil) from real Redis errors so `T-14-11` skipped-position counting works correctly.

### Safety

- **Default OFF:** entire Phase 14 path is dormant unless operator flips `cfg.PriceGapLiveCapital=true`. Reconciler runs in paper mode too — clean-day signal accumulates regardless of capital mode (D-07).
- **Module boundary preserved:** `internal/pricegaptrader` imports zero `internal/engine` / `internal/spotengine` paths. Reconciler imitates the engine's 3-retry shape locally rather than importing it.
- **Layered defense:** config-load validator (Plan 14-01) rejects malformed stage sizes / hard-ceiling typos at boot; Sizer + Gate 6 catch anything that escapes layer 1 (D-22).
- **Operator audit trail:** every state-changing operation (daemon eval, force-promote, force-demote, reset) appends to `pg:ramp:events` with operator + reason + prior/next stages. Force-op operator is hard-coded `"pg-admin"`.
- **Threat model documented:** T-14-01..T-14-16 in plan files; STRIDE entries cover boot-guard refusal, force-op auditability, daemon timeout, future-date validation, concurrent eval/force-op serialization.

### Tests

- `internal/pricegaptrader`: 5 nextUTCFireTime subcases + 1 boot-catchup + 1 graceful-shutdown + 1 boot-guard panic + 3 PriceGapNotifier conformance/static-allowlist = 11 new tests.
- `internal/notify`: 4 dispatch-shape tests (digest, reconcile-failure, ramp-demote, ramp-force-op) on httptest server.
- `cmd/pg-admin`: 6 reconcile + 7 ramp = 13 new subcommand tests covering happy-path / invalid-date / future-date / not-found / nil-dep / unknown-subcommand.
- Full pricegaptrader suite remains green (5.9s, idempotency lock test still PASS at -count=100).

## [0.36.0] - 2026-04-30

### Added

- **Phase 12 (PG-DISC-02) auto-promotion controller** — bidirectional pricegap candidates with score >= `PriceGapAutoPromoteScore` for >=6 consecutive scanner cycles auto-promote into `cfg.PriceGapCandidates` through the Phase 11 `*Registry` chokepoint. Symmetric demote: candidates below threshold (or absent from cycle records) for >=6 consecutive cycles auto-demote, gated by an active-position guard that consults `pg:positions:active` SMEMBERS — match → demote held until the position closes (D-05 fail-safe also blocks on Redis read errors).
- **Cap-full silent skip telemetry** — when promotion would exceed `cfg.PriceGapMaxCandidates`, the controller HOLDs the streak at threshold and increments `cap_full_skips:{symbol}` HASH field on `pg:scan:metrics` (no Telegram, no event). Operators can `redis-cli HGETALL pg:scan:metrics` to see WHICH symbols are queued behind a sustained cap.
- **Idempotent dedupe** — duplicate `(Symbol, LongExch, ShortExch, Direction)` tuples coming from a race between dashboard / pg-admin / controller are silently skipped (controller HOLDs streak at threshold; no spurious event).
- **Per-event Telegram alert** with cooldown key `"pg_promote:{action}:{symbol}:{long}:{short}:{direction}"` — distinct events bypass each other's 5-minute cooldown; same-candidate flap is throttled.
- **WebSocket event `pg_promote_event`** — broadcast on every promote/demote for live dashboard updates.
- **Redis LIST `pg:promote:events`** (RPush + LTrim 1000) plus REST seed `GET /api/pg/discovery/promote-events` (newest-first, Bearer-authed) — feeds the discovery dashboard timeline.
- **`DBActivePositionChecker`** in `internal/pricegaptrader` — production `ActivePositionChecker` impl; matches the configured tuple via `CandidateLongExch / CandidateShortExch` (PG-DIR-01) with fallback to wire-side roles for legacy positions (Pitfall 5).
- **`Server.Hub()` accessor** in `internal/api` — exposes the WS hub so cmd/main.go can pass it into `pricegaptrader.NewRedisWSPromoteSink` without leaking `internal/api` into `pricegaptrader` (D-15 module boundary preserved).

### Changed

- **`pricegaptrader.NewScanner` constructor signature (Phase 12 D-17 swap):** `registry` parameter widened from `RegistryReader` (interface) to `*Registry` (concrete) so the new `PromotionController` can call `registry.Add` / `registry.Delete` via the chokepoint. New `promotion *PromotionController` parameter added before `log` (nil-safe — `RunCycle` checks). The `RegistryReader` interface remains in `registry_reader.go` for read-only consumers.
- **`internal/pricegaptrader/scanner_static_test.go` (Phase 12 D-17 relaxation):** the original `\*Registry` and `registry.(Add|Update|Delete|Replace)\(` forbidden-token regexes are removed; the new (relaxed) invariant is a single regex `PriceGapCandidates\s*=` that forbids only RAW `cfg.PriceGapCandidates =` assignment. Chokepoint discipline is preserved by `PromotionController` having no `*config.Config` mutation surface.

### Safety

- **Default OFF:** the entire Phase 12 auto-promotion path is gated by the existing `cfg.PriceGapDiscoveryEnabled` flag. When `false`, the controller is never constructed, `s.promotion` is nil, and `Scanner.RunCycle` skips the `Apply` call. No new flag introduced.
- **Module boundary preserved:** `internal/pricegaptrader` does not import `internal/api` — `RedisWSPromoteSink` accepts a narrow `WSBroadcaster` interface that `*api.Hub` satisfies via duck-typing at the cmd/main.go wiring site.
- **Threat model documented:** cap-full / dedupe / active-position guard / chokepoint discipline / Telegram fan-out each have STRIDE entries in `.planning/phases/12-auto-promotion/12-0{1,2,3}-PLAN.md` `<threat_model>` blocks. All severities medium-or-lower; no high blockers.
- **Rollback:** set `cfg.PriceGapDiscoveryEnabled=false` via `POST /api/config` (NEVER edit `config.json` directly — CLAUDE.local.md). Restart `arb`. Optional cleanup: `redis-cli DEL pg:promote:events` (clears timeline) and `redis-cli HDEL pg:scan:metrics cap_full_skips:*` (clears counters). Existing manually-promoted candidates remain in `cfg.PriceGapCandidates` — only the AUTO-promotion path is disabled.

## [0.35.2] - 2026-04-28

### Added

- Replaced the legacy installer with a root-friendly Ubuntu/Debian one-click `install.sh` that installs Go 1.26+, Node.js 22, Redis, clones/updates the repo, creates a safe dry-run config when absent, builds frontend before the Go binary, installs a systemd service, and only starts the service when `ARB_START=1` is explicitly set.

## [0.35.1] - 2026-04-27

### Removed

- Removed the Dir B strategy-priority plan scaffold, including coordinator/SLO persistence, dashboard SLO visibility, strategy-priority config fields, reservation metadata, and the obsolete plan document.

### Fixed

- Kept the BingX probe-cancel hardening so cancel responses that say the probe order no longer exists are treated as successful cleanup, while API-disabled responses still fail closed.

## [0.35.0] - 2026-04-27

### Added

- **Pricegap tracker (PG-DIR-01, Phase 999.1):** `PriceGapCandidate.Direction` field (default `"pinned"`). Bidirectional candidates (`direction: "bidirectional"`) fire on either sign of the spread; the executor swaps wire-side leg roles for inverse fires while the lock key, position ID, and Phase 10 active-position guard continue to use the CONFIGURED tuple. Operator UX: dashboard Add/Edit candidate modal exposes a Direction radio toggle with Pinned (default) and Bidirectional options, with i18n labels in EN and zh-TW.
- **Pricegap position observability (Phase 999.1):** `FiredDirection` ("forward"|"inverse"), `CandidateLongExch`, `CandidateShortExch` fields persisted on `PriceGapPosition` for closed-log analytics and Phase 10 D-11 active-position guard tuple matching when wire-side roles diverge from configured.

### Changed

- **BREAKING (behavior, not API) — pinned-mode sign filter (Phase 999.1 Plan 01):** Pinned mode now requires positive-direction sign continuity to fire. Previously `barRing.allExceed` used `math.Abs` and silently fired bidirectional in detection (only execution was direction-locked), placing wrong-side trades on inverse spreads. Operators with candidates intentionally exploiting this latent bug will see them stop firing — verify your candidate list with `pg-admin list` and flip any intentionally-symmetric candidates to `direction: "bidirectional"` via the dashboard modal (or the candidates JSON). No JSON migration required for pinned-only candidates.

### Fixed

- **Pricegap risk-gate concentration (Phase 999.1 A5):** Gate-concentration check is role-blind — counts notional on either leg — required for correct concentration accounting under bidirectional inverse fires where wire-side roles diverge from the configured tuple.

## [0.34.13] - 2026-04-27

### Fixed

- Hardened BingX live order preflights so probes use the actual entry side, reject unsafe probe parameters, fail closed on missing probe order IDs, and report filled-before-cancel probes as failures.
- Added BingX futures preflight coverage to spot-futures Dir B entry and pending-entry recovery before any spot leg is opened.
- Added BingX preflight coverage to price-gap live entry before concurrent leg placement so a BingX API order disable cannot leave the peer leg opened first.

### Added

- Added a Dir B strategy-priority flow HTML reference diagram under `docs/`.

## [0.34.12] - 2026-04-27

### Fixed

- Added a BingX futures entry preflight using a signed, non-marketable IOC probe on the live `/openApi/swap/v2/trade/order` endpoint, so temporary BingX API order disables are detected before either real arbitrage leg is placed.
- Fixed planned allocator transfer sizing so donor min-withdraw constraints no longer cap a transfer below the remaining deficit.
- Made PnL reconciliation refresh and apply close stats atomically to avoid stale-position races during concurrent reconciliation.

### Added

- Added strategy-priority SLO backend/dashboard visibility for the existing `/api/strategy-priority` endpoint.
- Added spot-only Dir B plumbing for BingX behind the disabled-by-default spot-only exchange rollout flag.

## [0.34.11] - 2026-04-25

### Fixed

- **PG-VAL-03 (paper-mode realized_slippage_bps machine-zero):** in `internal/pricegaptrader/monitor.go` ClosePosition, paper synth fills are exactly `mid ± ModeledSlip/2` on both legs at both entry and exit. With no real price drift, entry slip (`+ModeledSlip`) and exit slip (`-ModeledSlip` because the formula references `LongMidAtDecision`/`ShortMidAtDecision` for both entry AND exit) cancel algebraically and `pos.RealizedSlipBps` collapsed to machine-zero — hiding what the operator modeled. Override added: when `pos.Mode == PriceGapModePaper`, set `pos.RealizedSlipBps = pos.ModeledSlipBps`. Live positions unaffected.

### Removed

- `cmd/bingxprobe/` debug utility (PG-DEBT-01) — purpose served when it diagnosed the case-insensitive JSON decode bug fixed in v0.34.6. 53-line one-shot, no callers, recoverable from git history if needed again.

### Added

- `cmd/shutdown_order_test.go` — Nyquist gap-fill for Phase 8: pins `pgTracker.Stop()` before `spotEng.Stop()` (D-03) and `pgTracker.Start()` only when `cfg.PriceGapEnabled` (PG-OPS-06). Static source-order tests so future refactors of `cmd/main.go` can't silently regress shutdown ordering.

### Documentation

- Phase 08 (price-gap tracker core) marked Nyquist-compliant after audit — VALIDATION.md frontmatter `nyquist_compliant: true`, all 28 task rows green. Phase 8 status: `Needs Review` → `Complete`.
- Phase 13 (v2.0 deferred closure) tightened: PG-OPS-08 closed by v0.34.10 (real cause was test wipe, not auto-POST), PG-DEBT-01 closed by this release, PG-VAL-03 closed by this release. Phase 13 effectively complete; no remaining v2.0 deferred items.

## [0.34.10] - 2026-04-25

### Fixed

- **Critical: live config.json wipe by `go test ./internal/api/`.** Root cause identified during Phase 10 UAT (audit syscall trace pinned `comm="api.test"` writing to `/var/solana/data/arb/config.json` at the wipe timestamp 2026-04-25 11:56:40 UTC). The chain: `Config.SaveJSONWithExchangeSecretOverrides()` had an absolute-path fallback `/var/solana/data/arb/config.json`. Tests in subpackages run with `cwd=internal/api/` (no local `config.json`), and the test helper `newPriceGapTestServer` constructed a near-zero `&config.Config{}`. When tests POSTed to handlers that called `SaveJSON()`, the absolute fallback resolved to the live production file and overwrote it with all-zero defaults — explaining every "mystery wipe" recorded in `config_watchdog.log` over the past several days (always the same fields: bool=true → omitted via omitempty, numerics → 0/1/0.5 defaults). Two-layer fix:
  1. `SaveJSONWithExchangeSecretOverrides` now requires either `CONFIG_FILE` env var OR `config.json` in cwd. No absolute fallback. Production unaffected (systemd `WorkingDirectory=/var/solana/data/arb` makes the relative path resolve correctly).
  2. `newPriceGapTestServer` now sets `CONFIG_FILE` to a `t.TempDir()` sandbox path (belt-and-suspenders).
- Verification: `go test ./...` (844 tests across 44 packages) all pass; live `config.json` byte-equal at 8870 bytes before and after the full suite. Previously the same test invocation reproduced the wipe deterministically.
- Added `cmd/configprobe` debug utility — runs `Load()` (and optionally `SaveJSON()`) against a sandbox config to verify load/save symmetry.

## [0.34.9] - 2026-04-25

### Fixed

- **VPS binary lineage — rebuild v0.34.8 to actually include e59be3d4 reconcile fix.** Observed: VPS v0.34.8 binary (built 2026-04-24 14:08 UTC) had VERSION=0.34.8 but did NOT contain `CloseSizeUnknown` or the Gate.io `getContractMult()` conversion in `GetClosePnL`. UPUSDT gateio↔okx closed 2026-04-24 16:02 UTC failed reconcile 4× with `longRawClose=10.000000/100.000000` — identical symptom to the PORTALUSDT binance↔bingx case, but via the Gate.io multiplier side of the same root cause. Root cause on VPS: the 0.34.8 artifact was built from a branch state that predates commit `e59be3d4` in its ancestry. No code change in v0.34.9 — just a clean rebuild from main HEAD (which does include e59be3d4 via merge `b737f958`) + republish + VPS redeploy so the on-disk binary actually contains both `ClosePnL.CloseSizeUnknown` and the gateio quanto_multiplier conversion. Verification: `strings arb | grep CloseSizeUnknown` post-deploy must return ≥1 match (v0.34.8 returned 0).

## [0.34.8] - 2026-04-24

### Fixed

- **Paper-mode boundary leak in `closeLegMarket` (high-severity).** `placeLeg` correctly synthesized fills when `PriceGapPaperMode=true`, but `openPair`'s unwind-to-match and defensive-close paths called `closeLegMarket`, which unconditionally invoked `ex.PlaceOrder` — sending real `reduceOnly=true` market orders against positions that only existed in our synth state. Observed during the 2026-04-24 UAT attempt: two SOONUSDT "opens" each preceded by `bingx API error code=101290 "Reduce Only order can only decrease position, not open"`. Reduce-only semantics saved us from opening unintended positions, but the tracker accumulated ghost Redis rows every 30s while live. Fix: single-line paper-mode guard at the top of `closeLegMarket` — restores D-12 / Pitfall 2 "paper mode is a single chokepoint at `ex.PlaceOrder`" invariant. Regression: `paper_close_leg_test.go` (paper: zero PlaceOrder calls; live: one PlaceOrder call with expected params).
- **Missing per-candidate re-entry gate (high-severity).** `preEntry` had 6 gates (exec-quality, max-concurrent, per-position cap, budget, gate-concentration, delist/staleness) but never checked "does this exact `(symbol, long_exch, short_exch)` slot already have an active position?". Observed during 2026-04-24 UAT: SOONUSDT opened at 21:25:04 and fired again at 21:25:34 under the same candidate tuple — capital exposure could have compounded in live mode. Fix: new Gate 0 at the top of `preEntry` that matches active positions on the full tuple and blocks with `ErrPriceGapDuplicateCandidate` / reason `duplicate_candidate`. No Telegram alert (this is sustained market signal, not an operational event). Regression tests: `TestRiskGate_DuplicateCandidate_Blocks` and `TestRiskGate_DuplicateCandidate_DifferentSymbol_Passes` in `risk_gate_test.go`.

### Added

- **`pg-admin positions purge <id>`** — operator CLI for dropping an entry from `pg:positions:active` without touching the exchange. Intended use: clean up synth/ghost positions orphaned by bugs or crashes. Position record stays in `pg:positions` hash for analytics. Not needed for tonight's two ghosts — Plan 08's startup rehydrate already orphaned them via the "zero real exchange position" check (Pitfall 3), confirming the self-healing path works for this failure mode.

### Provenance

- Failure surface emerged during controlled 09-11 UAT walkthrough after temporarily lowering SOONUSDT's `threshold_bps` to 10 to force a fire. Threshold reverted to 20 before any of these fixes landed; no live orders held capital.

## [0.34.7] - 2026-04-24

### Fixed

- **Gate.io adapter rejected pricegap `closeLegMarket` with `market order without IOC or FOK`.** Gate.io futures rejects market orders (price "0") unless `tif` is `ioc` or `fok`, but the adapter unconditionally defaulted `tif="gtc"` when `req.Force` was empty. pricegap's `closeLegMarket` submits `OrderType="market"` without setting `Force` (by design per §Pitfall 4), so every defensive unwind / monitor-exit market close via Gate.io was instantly rejected with `label=INVALID_ARGUMENT`. Surfaced on SOONUSDT and HOLOUSDT unwind-to-match during Phase 9 live tests. Fix: `pkg/exchange/gateio/adapter.go` defaults `tif="ioc"` when `req.OrderType == "market"`; explicit caller `Force` (including `fok`) still wins. Regression tests in `pkg/exchange/gateio/place_order_market_test.go` pin both the new default and the "explicit Force still wins" guarantee.

### Observations (not fixed — turned out to be same-root symptoms)

- Bybit `code=10001 "The number of contracts exceeds minimum limit allowed"` and BingX `code=109400 "parameter quantity or quoteOrderQty is must"` on SIGNUSDT openPair were downstream symptoms of the 0.34.6 BingX BBO bug — polluted mid price caused `sizeBase = notional / mid ≈ 0.01` which rounded to 0, and both exchanges legitimately rejected a zero-size order. Gone after 0.34.6. No adapter change needed.

## [0.34.6] - 2026-04-24

### Fixed

- **BingX public WS bookTicker: bid/ask quantity was being stored as price (case-insensitive JSON overwrite).** Surfaced by Phase 9 Gap #2 (09-10) once `assertBBOLiveness` started actually reading BingX BBO and logged nonsense values like `SOON@bingx bid=24565 ask=28345` when real SOON is ~$0.18. Root cause: BingX's payload carries both `b` (bid price) and `B` (bid qty), and likewise `a`/`A`. The handler struct tagged only `"b"` and `"a"`. Go's `encoding/json` matches keys case-insensitively, and when a lowercase tag matches both lowercase and uppercase JSON keys the later-emitted key wins — so `B` overwrote `b` and `A` overwrote `a`. Live WS probe confirmed BingX itself emits correct prices; the bug was wholly on our side. Fix: claim the uppercase fields as explicit `BidQty` / `AskQty` slots so every JSON key has exactly one exact-tag match. Regression test: `pkg/exchange/bingx/ws_booktick_test.go` feeds a real BingX payload and asserts stored `Bid=0.18140, Ask=0.18160`. Fails on pre-fix struct, passes after.

## [0.34.5] - 2026-04-24

### Merged

- Merged `origin/main` (collaborator's 0.33.x series through 0.33.3) into local v2.0 Phase 9 line. VERSION resolved as merge bump above the higher side. CHANGELOG interleaved semver-descending; both sides of the existing `[0.32.40]` `### Added` block preserved.

## [0.34.4] - 2026-04-24

### Fixed

- **Test isolation: two `internal/api/config_handlers_test.go` tests were writing to the live `/var/solana/data/arb/config.json` on every `go test ./internal/api/...` run.** `TestHandleConfig_SpotFuturesAutoDryRunRoundTrip` and `TestHandleConfig_WithdrawMinIntervalMs` called `s.handlePostConfig`, which invokes `s.cfg.SaveJSON()`. `SaveJSON` honors `CONFIG_FILE` env var first and falls through to `["config.json", "/var/solana/data/arb/config.json"]` otherwise. Both tests set up a Server but never seeded a TempDir config + `t.Setenv("CONFIG_FILE", …)`, so SaveJSON hit the production config file. Fixed by adding the standard `t.TempDir()` + `t.Setenv("CONFIG_FILE", …)` pattern used by the other eight tests in the file. Verified: tests still pass; `config.json` inode/size/mtime unchanged after both runs. Root cause of the 2026-04-24 02:33 UTC / 05:45 UTC stray writes to live config.

## [0.34.3] - 2026-04-24

### Fixed

- **Phase 9 Verification Gap #2 closed** — `Tracker.Start()` now fans out `SubscribeSymbol` on both leg exchanges for every `cfg.PriceGapCandidates` entry before `rehydrate()` and the first detection tick (`internal/pricegaptrader/tracker.go`). Previously `pkg/exchange.SubscribeSymbol` had zero callers in `internal/pricegaptrader/` or `cmd/main.go`, so pricegap candidates only received BBO when they coincidentally overlapped the perp-perp funding-rate scanner's subscription universe. Any candidate outside that overlap (e.g. VICUSDT) silently produced no BBO and `detectOnce` returned `sample_error` — which Gap #1 previously hid entirely. Fail-soft per `engine.go:4675-4676`: missing exchange key or `SubscribeSymbol` returning false emits a WARN and the loop continues.

### Added

- **Startup BBO-liveness assertion (fail-loud diagnostic)** — `assertBBOLiveness()` goroutine launched from `Start()`. 15s after startup, every configured candidate leg is probed via `GetBBO`; any `ok=false` or zero-price BBO emits a `pricegap: BBO NOT LIVE` WARN naming `(symbol, exchange, leg)`. Diagnostic beacon only — does not retry or abort. Respects `stopCh` so shutdown during the grace window is clean. Tracked via `wg` so `Stop()` blocks until it exits.
- **Unit tests** — `internal/pricegaptrader/tracker_subscribe_test.go` (10 tests). Task 1 fan-out: exact per-(leg, symbol) call counts, unknown-exchange fail-soft, SubscribeSymbol-false fail-soft, empty-candidate no-op, restart idempotency, same-exchange-both-legs edge case, WARN-message content asserted via `utils.Subscribe()` log capture. Task 2 BBO liveness: all-live silence, dead-leg WARN content, Stop-before-grace wg.Wait completes within 2s (goroutine leak guard under `-race`). Extends `stubExchange` in `fakes_test.go` with `subs map[string]int` counter + optional `subscribeFn` override.

### Boundary

- Option A clean boundary preserved (09-VERIFICATION.md) — subscription lifecycle lives inside `Tracker`; `cmd/main.go` unchanged. `internal/engine` and `internal/spotengine` not imported.

## [0.34.2] - 2026-04-24

### Fixed

- **Phase 9 Verification Gap #1 closed** — detector non-fire reasons now surface in logs when `price_gap.debug_log` is ON (`internal/pricegaptrader/tracker.go:287-296`). Previously the bare `if !det.Fired { continue }` discarded `det.Reason` values (`sample_error: ...`, `insufficient_persistence`, `stale_bbo`), making the detector a black box in production and blocking the 14 live-detection UAT rows (PG-OPS-02/03/05, PG-VAL-01/02). Rate-limited per-(symbol, reason) at a 60s cooldown so steady-state logging cannot flood journalctl (4 candidates × 3 reasons × 2/min = up to 24 dupe lines/min without the throttle).

### Added

- **`PriceGapDebugLog` config field** — `internal/config/config.go`. JSON key `price_gap.debug_log` with applyJSON + SaveJSON round-trip. Default OFF per the project new-feature rollout pattern (CLAUDE.local.md). Flat shortcut intentionally not introduced — nested form only.
- **Dashboard toggle** — `web/src/pages/PriceGap.tsx` header next to the Paper/Live pill. Amber tint when ON signals "diagnostic logging active". POSTs `{ price_gap: { debug_log: newValue } }` via the existing `postConfig` helper. Reflected on next seed.
- **i18n lockstep** — `pricegap.debugLog` + `pricegap.debugLogTooltip` added to `en.ts` and `zh-TW.ts`; `scripts/check-i18n-sync.sh` green at 758 keys.
- **Unit tests** — `internal/pricegaptrader/tracker_debug_log_test.go` (10 tests): empty-reason rejected, first-emit, cooldown suppression, cooldown-boundary refill, reason-independence, symbol-independence, flood-guard (1-of-100), flag-OFF zero-map-growth, flag-ON one-entry. `internal/config/config_test.go` gains 5 tests: default OFF, applyJSON true/false, absent-preserves, SaveJSON round-trip.

## [0.34.1] - 2026-04-24

### Fixed

- **`realized_pnl` over-reported on Binance+Gate.io closes.** Reconcile's Tier-1 completeness gate (`internal/engine/exit.go:1170-1178`) compared raw `ClosePnL.CloseSize` across adapters as if all were in token units. Binance's aggregated income record returned `CloseSize=0` (no per-close volume in that API) and Gate.io returned `AccumSize` in contract units (e.g. `2681 = 268.1 × quanto_multiplier`), so the gate rejected the authoritative exchange-PnL on every retry and persisted a stale depth-exit estimate. Example: `promusdt-1776743108692` closed at `+19.62` vs actual `+6.16` (overstatement ≈ `$13.45`, driven by a bad long-leg VWAP in the market-fallback close path). Fix: Gate.io adapter applies `getContractMult()` to `CloseSize` (mirrors OKX `getCtVal()` pattern); Binance adapter sets new `ClosePnL.CloseSizeUnknown=true` flag; reconcile gate and `aggregateClosePnLBySide` skip size-comparison for unknown-size legs. `pkg/exchange/types.go`, `pkg/exchange/gateio/adapter.go`, `pkg/exchange/binance/adapter.go`, `internal/engine/exit.go`.

## [0.34.0] - 2026-04-22

### Added (Phase 09 — Price-Gap Dashboard & Paper→Live Operations, v2.0 milestone)

- **Price-Gap dashboard tab (PG-OPS-01)** — `web/src/pages/PriceGap.tsx`. Candidate list with Disable / Re-enable confirmation modals, Live Positions table with Entry / Current / Hold / PnL columns, Closed Log with Modeled / Realized / Delta columns (color-coded by sign), Rolling Metrics section showing 24h / 7d / 30d bps/day per candidate, default-sorted by 30d descending. Nav placement between spot-positions and history.
- **Paper-mode toggle (PG-OPS-04)** — header pill flips between violet PAPER and green LIVE. Persists to `config.json` via POST `/api/config`. Default TRUE when `price_gap.paper_mode` key absent (safety-first default per D-12, Pitfall 1). Flat `price_gap_paper_mode` root field accepted alongside nested `price_gap.paper_mode`; nested wins on conflict.
- **Mode immutability (PG-OPS-04 / Pitfall 2)** — `PriceGapPosition.Mode` field stamped once at entry in `execution.go` and never re-read from the global flag. Monitor and close paths read `pos.Mode` exclusively; flipping the runtime toggle mid-life cannot flip an in-flight position between paper and live.
- **Synth fill slippage (PG-VAL-01 / Pitfall 7)** — paper-mode fills synthesize prices at `mid ± (modeled/2)/10_000` on both entry and exit legs. Guarantees non-zero `RealizedSlipBps` on paper closes so the Phase 8 realized-vs-modeled pipeline is fully exercised without real orders.
- **Telegram alerts for price-gap (PG-OPS-05)** — three new `*TelegramNotifier` methods: `NotifyPriceGapEntry`, `NotifyPriceGapExit`, `NotifyPriceGapRiskBlock`. Cooldown keyed per `pg_risk:<gate>:<symbol>` with 1h window. Paper-mode messages carry a 📝 PAPER prefix so operators cannot mistake paper traffic for live. Allowlisted gate names: `concentration`, `max_concurrent`, `kline_stale`, `delist`, `budget`, `exec_quality`. Detail sanitization strips C0 control chars and truncates to 256 bytes (T-09-18).
- **Rolling metrics aggregator (PG-VAL-02)** — pure function with caller-supplied clock; cumulative 24h / 7d / 30d windows computed from `pg:history` alone (D-24 simplification — no new write paths added). Zero-activity rows padded from `cfg.PriceGapCandidates` at the handler level.
- **WebSocket topics** — `pg_positions` (full active-positions snapshot, throttled to 2s), `pg_event` (entry / exit / auto_disable, unthrottled), `pg_candidate_update` (disable state changes).
- **REST API** — `/api/pricegap/{state,candidates,positions,closed,metrics}` read endpoints + `/api/pricegap/candidate/{sym}/disable` and `/api/pricegap/candidate/{sym}/enable` mutating endpoints (auth-gated via `isMutatingEndpoint`, T-09-06).
- **Candidate disable persistence (Pitfall 6)** — `IsCandidateDisabled` now returns `(bool, reason, disabledAtUnixSec, err)`. New writes use JSON `{reason, disabled_at}`; legacy plain-string values are read transparently for backward compatibility.
- **i18n sync gate** — `scripts/check-i18n-sync.sh` asserts every key present in `en.ts` is also present in `zh-TW.ts` and vice versa; CI-style check runs as an acceptance gate on every Phase 9 plan.
- **Paper→live cutover integration tests (Plan 09-08)** — `TestPaperToLiveCutover` and 3 siblings drive the full lifecycle: paper entry → flag flip → live entry → paper close (synth path, no `PlaceOrder`) → live close (real path). Asserts `pg:history` carries both rows with correct Mode stamps, non-zero `RealizedSlipBps` on the paper row, and correct Telegram notifier call counts.
- **09-UAT.md** — human-operator walkthrough checklist with 22 rows covering every PG-OPS-0X and PG-VAL-0X requirement plus i18n and accessibility. Exit criteria: every row checked blocks Phase 9 sign-off.

### Changed

- `PriceGapPosition` gained a `Mode` field (`"paper" | "live"`), stamped at entry and immutable through the position's lifecycle. History rows carry it; the dashboard Closed Log reads it to apply the PAPER / LIVE badge.
- `SetCandidateDisabled` now persists JSON `{reason, disabled_at}`; the backward-compatible reader handles legacy plain-string values written by pre-Phase-9 `pg-admin`.
- `isMutatingEndpoint` extended to cover `/api/pricegap/candidate/` paths so POST disable / enable enforce auth even when `DashboardPassword` is unset (T-09-06 hardening).
- Plan 06 aligned `risk_gate.go` reason strings with the Plan 04 Telegram allowlist; the per-position-cap gate intentionally does not emit a Telegram alert (operator-configured cap, not a market event).

### Unchanged (regression-pinned)

- Perp-perp Telegram messages — `internal/notify/telegram_regression_test.go` (Plan 04) pins byte-for-byte output for every pre-Phase-9 `Notify*` method.
- Spot-futures notification paths — same regression pinning.
- Phase 8 tracker core behavior when `PriceGapPaperMode=false` — `paper_mode_test.go` `TestLiveMode_OpenPair_Unchanged` and sibling tests assert live-path byte-for-byte parity with pre-Phase-9 behavior.
- The safety property that `PriceGapEnabled=false` produces zero `pg:*` Redis writes (Phase 8 Plan 07 `TestPriceGapEnabled_DefaultOff_NoTrackerInstantiated`).

## [0.33.2] - 2026-04-22

### Added (Phase 09 Plan 04 — Regression Guardrail, Task 2, T-09-20)
- `internal/notify/telegram_regression_test.go` pins byte-for-byte output for every pre-Phase-9 `Notify*` method: `NotifyAutoEntry` (Dir A + Dir B), `NotifyAutoExit` (positive + negative PnL), `NotifyEmergencyClose`, `NotifySLTriggered`, `NotifyEmergencyClosePerp`, `NotifyConsecutiveAPIErrors` (with + without error), `NotifyLossLimitBreached`, `NotifySpotHedgeBroken`, `NotifySpotCloseBlocked`.
- `TestRegression_AllNilSafe` pins the nil-receiver invariant for all existing methods so future refactors of the shared `send`/`checkCooldown` helpers cannot silently break paging.
- Any accidental format drift on a perp-perp or spot-futures alert body will now fail a unit test (`go test ./internal/notify/... -count=1 -race`, 30/30 green).

## [0.33.1] - 2026-04-22

### Added (Phase 09 Plan 04 — Price-Gap Telegram Notifications, Task 1, PG-OPS-05)
- Three new notifier methods on `*TelegramNotifier` in `internal/notify/telegram.go`:
  - `NotifyPriceGapEntry(pos *models.PriceGapPosition)`
  - `NotifyPriceGapExit(pos, reason, pnl, duration)`
  - `NotifyPriceGapRiskBlock(symbol, gate, detail)` — allowlisted gate names (`concentration`, `max_concurrent`, `kline_stale`, `delist`, `budget`, `exec_quality`), cooldown keyed per `pg_risk:<gate>:<symbol>`, detail sanitized (control chars stripped, 256-byte cap).
- Paper-mode parity (D-22): when `pos.Mode == "paper"` messages are prefixed with `📝 PAPER ` and tagged `[PAPER]` so operators cannot mistake paper traffic for live.
- `sanitizeForTelegram(s, max)` helper — strips C0 control characters except `\n` and `\t`, truncates to `max` bytes (T-09-18).
- `telegramAPIBase` package-level var replaces hard-coded `https://api.telegram.org` URL to make `send()` httptest-stubbable without touching production behavior.
- 12 new tests in `internal/notify/telegram_pricegap_test.go` covering live/paper prefixing, nil-receiver + nil-pos safety, cooldown windowing, per-key independence, unknown-gate rejection (T-09-17), detail sanitization, and zero-notional divide-safety. Full notify suite green under `-race -count=1` (20/20).



### Added (Phase 8 — Price-Gap Tracker Core, v2.0 milestone)
- New isolated subsystem `internal/pricegaptrader/` — cross-exchange price-gap event detection and delta-neutral IOC execution (Strategy 4 MVP). Default OFF (`PriceGapEnabled=false`).
- Config surface: 11 new `PriceGap*` fields (PG-05, PG-OPS-06), read once at startup from `config.json`.
- Redis namespace `pg:*` for position persistence (PG-04), exec-quality flags (PG-RISK-03), and slippage rolling windows. No collision with `arb:*` or `arb:spot_*`.
- Pre-entry risk gates: Gate concentration 50%, max concurrent, per-position notional cap, budget, delist/halt/staleness checks, exec-quality disable (PG-RISK-01..05).
- `cmd/pg-admin` — operator CLI for `enable|disable|status|positions list` (D-20). Reversal path for PG-RISK-03 auto-disable until Phase 9 dashboard ships. Operates entirely on Redis; config.json untouched.
- Circuit breaker on 5 consecutive PlaceOrder failures skips full ticks (D-10).
- Startup rehydration with orphan detection (§Pitfall 3): active positions re-enroll in monitors; zero-size ghost positions are closed with `ExitReasonOrphan`.
- 49 new tests in `internal/pricegaptrader/` (44 unit + 5 E2E) and 4 in `cmd/pg-admin/`. Full suite green under `-race -count=1`.

### Notes
- Perp-perp and spot-futures engines are byte-for-byte unaffected when `PriceGapEnabled=false`; safety-property test asserts zero `pg:*` Redis writes on the default-off path.
- Phase 9 will add a Dashboard tab, paper-mode toggle, runtime enable switch, and Telegram alerts; until then, operator workflow is: edit `config.json` + restart + `pg-admin` for disable reversals.

## [0.32.44] - 2026-04-21

### Added
- **Price-gap tracker — cmd/main.go conditional wiring (Phase 08 Plan 07, Task 3)** (`cmd/main.go`). Completes the tracker's main-binary integration:
  - **Startup (D-03):** AFTER `spotEng.Start()`. Guarded by `if cfg.PriceGapEnabled`; when true, constructs `pricegaptrader.NewTracker(exchanges, db, scanner, cfg)` and calls `Start()`. `*discovery.Scanner` satisfies `models.DelistChecker` via its existing `IsDelisted` method — no new interface surface needed.
  - **Shutdown (D-03 reverse):** BEFORE `spotEng.Stop()`. Since the tracker started AFTER SpotEngine, it stops FIRST under the reverse-order rule so its monitor goroutines wind down while dependencies (db, exchanges) are still fully live.
  - **Safety property (PG-OPS-06, T-08-27):** `PriceGapEnabled=false` (default) path logs `"Price-gap tracker disabled (cfg.PriceGapEnabled=false)"` and does NOT call `NewTracker` — zero goroutines spawn, zero `pg:*` Redis reads occur. Byte-for-byte isolation from perp-perp and spot-futures engines is preserved; the existing `spotEng`/`eng`/`apiSrv` wiring is unchanged.
  - Phase 08 Plan 07 complete — tracker is now end-to-end integrated. Phase 09 will add the dashboard enable toggle; today the switch lives in `config.json` only and is read once at startup.

## [0.32.43] - 2026-04-21

### Added
- **Price-gap tracker — end-to-end test suite (Phase 08 Plan 07, Task 2)** (`internal/pricegaptrader/tracker_test.go`). 5 tests driving the full tickLoop pipeline against a miniredis-backed real `*database.Client`:
  - `TestTrackerE2E_HappyCycle` — 4 bars of +253 bps spread feeds the ring → `runTick` opens the position via `openPair`, enrolls monitor, persists to `pg:positions`+`pg:positions:active`; then spread reversion + `checkAndMaybeExit` drives a clean close with `ExitReasonReverted`.
  - `TestTrackerE2E_GateBlocksOnBudget` — budget=500 < per-position=1000 → fired detection is blocked at the budget gate, zero orders placed on either leg, zero positions persisted.
  - `TestTrackerE2E_DisabledFlagBlocks` — pre-seeded `pg:candidate:disabled:SOON` blocks at Gate 1 (exec-quality), zero orders placed.
  - `TestTrackerE2E_RehydrationOnRestart` — pre-seeded active position + nonzero exchange positions → `Start()`→`rehydrate()` re-enrolls the monitor; wide BBO + long poll interval prevent false reversion-close during the test window.
  - `TestPriceGapEnabled_DefaultOff_NoTrackerInstantiated` — safety property (PG-OPS-06 / T-08-27): with `PriceGapEnabled=false`, the main.go-equivalent guard block does NOT call `NewTracker`; miniredis key scan asserts zero `pg:*` writes.



### Added
- **Price-gap tracker — real tickLoop dispatch + Start() rehydrate (Phase 08 Plan 07, Task 1)** (`internal/pricegaptrader/tracker.go`). Replaced the stub `tickLoop` with a real detect → gate → enter → monitor dispatch:
  - `tickLoop` fires the first tick ~7s after startup (RESEARCH §Pitfall 2 — offsets off the Bybit :04–:05:30 blackout window on fresh boot) then runs at `PriceGapPollIntervalSec` cadence via `time.Ticker`.
  - `runTick` iterates every configured `PriceGapCandidate`: `detectOnce` → on fired, `preEntry` gates → on approved, converts notional USDT to per-leg base-asset size via `mid = (MidLong+MidShort)/2`, calls `openPair` + `startMonitor`. Circuit-breaker-aware: skips the whole tick when `isCircuitOpen()` is true.
  - `Start()` now calls `rehydrate()` BEFORE spawning the tick goroutine so any restored positions are already enrolled in monitors before the first tick reads the active set for budget gating.
  - Local `active` view is appended after each successful `openPair` so back-to-back candidates in the same tick see up-to-date concurrency state.

## [0.32.41] - 2026-04-21

### Added
- **Price-gap tracker — exit monitor, exec-quality override, and startup rehydration (Phase 08 Plan 06)** (`internal/pricegaptrader/{monitor,slippage,rehydrate}.go`). Completes PG-03 + PG-04 + PG-RISK-03:
  - **Per-position exit monitor** — `startMonitor` spawns a goroutine per open position; `monitorPosition` polls at `PriceGapPollIntervalSec`. `closePair` fires on reversion (`|spread| ≤ ThresholdBps × PriceGapExitReversionFactor`, D-11) or max-hold (`now − OpenedAt ≥ PriceGapMaxHoldMin`). Exit places simultaneous IOC ReduceOnly orders on both legs; any shortfall retries as MARKET ReduceOnly (D-12 §Pitfall 4 — positions must close fully). Persists `ExitReason`, `RealizedPnL`, `RealizedSlipBps` (D-21 round-trip bps) and moves to `pg:positions:history`.
  - **Exec-quality auto-disable (PG-RISK-03)** — `recordSlippageAndMaybeDisable` appends one sample per closed trade to a rolling 10-trade window; if `mean(realized) > 2 × mean(modeled)` (strict `>`, D-19), sets `pg:candidate:disabled:<symbol>` so subsequent preEntry Gate 1 blocks with `ErrPriceGapCandidateDisabled`. Divide-by-zero guard: `meanMod ≤ 0` short-circuits, preventing the pathological "zero modeled → auto-disable everything" trap.
  - **Startup rehydration (PG-04)** — `rehydrate` iterates `pg:positions:active`; for each, queries `GetPosition(symbol)` on both legs and orphans any zero-size position with `ExitReasonOrphan` (RESEARCH §Pitfall 3 ghost-position replay). Survivors re-enroll in monitor goroutines. Idempotent: `startMonitor` cancels+replaces prior handles via an atomic seq token, so calling `rehydrate` twice leaves exactly one live monitor per position (race-safe under `-race`).
  - **Test coverage** — 14 new tests across `monitor_test.go` (6), `slippage_test.go` (5), `rehydrate_test.go` (3). All 44 pricegaptrader tests green under `-race -count=5`.
  - Still off by default (`PriceGapEnabled=false`). Phase 9 (dashboard) will expose `/api/pricegap/positions` and the enable toggle; Plan 07 wires `rehydrate()` into `Start()`.

## [0.33.3] - 2026-04-23

### Fixed
- **Rebalance transfer timing for split-account donors** — `executeRebalanceFundingPlan` called `TransferToSpot` as fire-and-forget and then immediately read `GetSpotBalance`, which for binance/bitget-class split-account donors returned stale balance before the internal futures→spot settled. The subsequent `netAmount = spot - fee < 0` guard silently skipped the outbound withdraw, leaving recipients (e.g. bingx) unfunded. Observed 2026-04-23 02:35 UTC binance→bingx for SIGNUSDT. Fix:
  - Added `*Engine.waitForSpotBalance(donor, required, timeout)` helper returning `(*exchange.Balance, error)` that polls `GetSpotBalance` every 1s until `Available >= required` or the 10s timeout elapses (`internal/engine/allocator.go`). Stop-aware via `select` on `e.stopCh`.
  - Added `*Engine.captureSpotBalanceForTransfer(donor, snapshotSpot)` helper that reads the donor's current spot balance BEFORE `TransferToSpot`. On read failure it returns an error and the caller skips the donor — no snapshot fallback because an earlier-in-cycle donor prep could have credited spot above the snapshot, making the post-transfer wait target too low.
  - Captured pre-transfer spot balance before `TransferToSpot`, waited for `preTransferSpot + movedToSpot` using the parsed submitted transfer amount (`parsedMove` from `moveStr`), and reused the returned `*Balance` for the split-account `donorSpotBal` read — avoiding a redundant `GetSpotBalance` that could hit stale cache after a successful wait.
  - On wait timeout: pessimistically debit donor `futures`/`futuresTotal` by `movedToSpot` and credit `spot` so subsequent iterations don't over-commit the donor (funds are in spot but we can't observe them in time).
  - Regression tests covering eventual success, transient `GetSpotBalance` error, pre-existing spot below target, timeout, unknown donor, stop-aware shutdown, and captureSpotBalanceForTransfer success/read-error/unknown-donor cases (`internal/engine/wait_spot_balance_test.go`, 9 tests).
  - Codex independent audits (local companion tasks `b1f1og3j7`, `bqvv15ukz`, `bqtfw2xow`, `ba6l7l0eg`, `brshav1ag`, `bam03kjb5`, `bjz5cvnu5`, `bdz321szb`, `bfiv2sy20`, `buac3qov4`, gpt-5.4 xhigh) confirmed root cause and iterated the plan to v9 PASS with 8/10 production-readiness.

## [0.33.2] - 2026-04-23

### Added
- **Dashboard version display** — sidebar now shows `v<version>` next to the connection indicator so deploy status is visible at a glance. Version is injected at build time from the repo `VERSION` file via Vite `define` (`web/vite.config.ts`, `web/src/components/Sidebar.tsx`).

## [0.33.1] - 2026-04-22

### Fixed
- **Rebalance top-up routing bug** — simulator's cross-exchange top-up inflated `pair.bal.Available` to pass margin/L4 checks, but the resulting `approval.Approved=true` was routed into Pass-1 case (a) which schedules NO transfer. Subsequent entry-scan executor rejected on real (unrestocked) balance. 2026-04-22 METUSDT gateio/bitget incident: 11:35 rebalance reserved 199.93/199.71, 11:45 entry rejected with post-trade margin ratio 0.87 on gateio (>L4 0.80). Fix:
  - Added `RiskApproval.TopUpApplied map[string]float64` recording per-exchange simulator borrows (`internal/models/interfaces.go`).
  - `approveInternal` populates the field only when `dryRun=true` (`internal/risk/manager.go`).
  - Pass-1 switch at `internal/engine/rebalance.go:210` routes `Approved && len(TopUpApplied) > 0` into the case-(b) rescue-candidate path, unifying with existing `RejectionKindCapital` handling so a real transfer is planned and the post-transfer replay re-validates against `TransferablePerExchange=nil`.
  - Regression tests in `internal/risk/manager_topup_test.go` and `internal/engine/rebalance_topup_test.go`.
  - Independent review via dispatch-mcp (task `baa9d706`) confirmed the bug before fix.

## [0.33.0] - 2026-04-22

### Fixed
- **Bitget error swallowing + non-ASCII symbol support (plan `plans/PLAN-bitget-error-handling.md` v22 ALL PASS — approved by remote dispatch-mcp codex review task 58772305)** — bitget client's `doRequest` previously returned `(body, nil)` regardless of HTTP status or response `code`, masking every transient API failure as "no data". Concrete symptom: position `龙虾usdt-1776800704405` `funding_collected=-0.069` tracked only binance long leg, missing bitget short leg's +0.914 USDT funding (verified via no-symbol-filter bitget API). Root cause: non-ASCII symbol (`龙虾USDT`) in signed GET query fails bitget HMAC with 40009, client swallowed the error. Four coordinated phases across 5 parallel worktrees:
  - **Phase A — bitget adapter strict errors + non-ASCII fallbacks**: `doRequest` checks HTTP status AND `envelope.Code` with pass-through map for idempotent codes (40872, 43011, 43025); `isRetryable` inspects `*APIError` via `errors.As` + 5xx class retry. Non-ASCII fallback endpoints added: `GetFundingFees`, `GetFundingRate`, `GetPosition`, `GetUserTrades`, `GetClosePnL`, `GetOrderFilledQty`. `CheckPermissions` migrated to `errors.As` with class branching (40009→PermDenied, retryable/5xx→PermUnknown, other→PermGranted). `CancelAllOrders` surfaces both errors via `errors.Join`. `populateBitgetFeeDeducted` logs both spot-fills and margin-fills errors.
  - **Phase B — engine caller safety**: `retrySecondLeg` signature adds `error` return; 4 callers (engine.go:3688/3727/3932/3971) persist `StatusPartial` + `FailureReason="second_leg_fill_unknown"` on unknown-state error to prevent duplicate orders (2× exposure risk). `confirmFill` adds error propagation; all callers in `exit.go` + `consolidate.go` audited. `consolidate.go` logs silent continue on `GetClosePnL` failure.
  - **Phase C — spotengine pending-futures recovery**: new `pendingFuturesEntryError` struct + `PendingFuturesEntryOrderID` field + `reconcilePendingFuturesEntry` monitor method. `confirmFuturesFill` returns error on unknown REST state; Dir A (borrow_sell_long) persists `spotFilled*spotAvg` gross, Dir B (buy_spot_short) persists `spotNetReceived*spotAvg` net to match HEAD final state after outer ManualOpen overwrite at line 402. Defensive gate in `reconcilePendingEntry` routes pending-futures to its own reconciler.
  - **Phase D — revert v0.32.42 ASCII guards + frontend partial-legs UI**: removed `IsValidBaseQuoteSymbol` guards from discovery/ranker, discovery/scanner, spotengine/discovery, spotengine/engine, api/spot_handlers (v0.32.42 blocked non-ASCII at source — wrong approach; exchanges list these symbols, so blocking loses real arbitrage opportunities). Deleted `pkg/utils/symbol.go` (`containsNonASCII` moved to `pkg/exchange/bitget/util.go`). Backend `handleGetPositionFunding` logs failed legs + sets `X-Partial-Legs` header. Frontend `getPositionFunding` bypass preserves 401 handling, exposes header; `Positions.tsx` renders warning banner; new `pos.fundingPartial` i18n key (EN + zh-TW).
- **`cmd/peertest/` (#11)** — read-only CLI empirically tests Bybit/OKX/Gate/BingX/Binance for same HMAC issue using dynamically discovered non-ASCII symbols via `LoadAllContracts`.

## [0.32.42] - 2026-04-22

### Fixed
- **ASCII symbol guard across discovery pipeline + spot API handlers (plan `plans/PLAN-ascii-symbol-guard.md` v4 ALL PASS on Codex normal + independent review + post-implementation review)** — Loris API returns non-ASCII symbols (e.g. `龙虾`, `币安人生`) that previously entered the ranker, passed `hasContract()` on some exchanges, passed 22/23 verification checks, and could reach entry scan causing one-legged positions, wasted API calls, and infinite retry loops. Added `utils.IsValidBaseSymbol(^[A-Z0-9]+$)` guard at 9 entry points: Loris/CoinGlass ranker (silent skip, fires every cycle), `InjectTestOpportunity` (WARN log), `loadCachedOpps` startup filter, and 4 API handlers (`/spot/manual-open`, `/spot/test-inject`, `/spot/test-lifecycle`, `/spot/backtest`) return HTTP 400 with "expected ASCII USDT symbol" error.
- **Pool Allocator double-rebalance fix** — when `EnablePoolAllocator=true`, `rebalanceFunds()` previously ran twice per cycle (at RebalanceScan :10/:20 AND RotateScan :35), causing ~15-25-min-apart double transfers, wasted fees (e.g. gateio→okx $1), and suboptimal fund distribution. RebalanceScan handler now skips when allocator enabled (symmetric with existing RotateScan guard). Both guards also check `RotateScanMinute != RebalanceScanMinute` to handle same-minute misconfiguration — if user sets both minutes equal, scanner's else-if dispatch picks RebalanceScan and RotateScan never fires, so skipping would strand rebalancing entirely.

### Added
- **`pkg/utils/symbol.go`** — shared `IsValidBaseSymbol` helper with compiled regex (zero-alloc per match). Used by `internal/discovery`, `internal/spotengine`, and `internal/api`.
- **`pkg/utils/symbol_test.go`** — unit test covering 7 valid symbols (BTC, 1000SATS, 1MBABYDOGE, 0G, 1INCH, 10000000AIDOGE, etc.) and 7 invalid (Chinese chars, slashes, underscores, empty, whitespace, accented chars).

## [0.32.41] - 2026-04-21

### Fixed
- **Stage 2 DB-failure fund stranding (codex independent review `d1ac2563` Finding #1 HIGH)** — when `db.GetActivePositions()` failed transiently between Stage 1 and Stage 2, `applyAllocatorOverrides` had already drained `e.allocOverrides` but Stage 2 bailed without executing, and the existing salvage block excluded in-scan symbols via `inScan[sym]`, stranding rebalance-funded capital until the next cycle. Plan `plans/PLAN-stage2-db-failure-salvage.md` v6 ALL PASS on normal + independent Codex review. Two complementary fixes:
  - **Retry helper** (`getActivePositionsWithRetry`, 2 attempts / 300ms backoff) — survives transient Redis blips at the two critical Stage 2 + salvage call sites.
  - **Replaced `inScan[sym]` salvage exclusion with `stage2Attempted[sym]` tracking** — salvage now correctly rescans symbols Stage 2 never got to attempt (in-scan AND out-of-scan), instead of assuming Stage 2 always had a chance.

### Added
- **`activePositionsGetter` interface + `Engine.activePosSource` field** (`internal/engine/engine.go`) — minimal injection seam routing the two Stage 2 + salvage `GetActivePositions` reads through the retry helper. Wired to `e.db` in `NewEngine`; other 14+ `e.db.GetActivePositions` call sites unchanged (zero-blast-radius scope).
- **`runStage2AndSalvage` method** (`internal/engine/engine.go`) — extracted the Stage 2 + salvage block from the inline run loop into a testable method; caller-site unchanged in behavior.
- **9 regression tests** (`internal/engine/engine_stage2_salvage_test.go`) — T1 happy path, T2a/T2b retry/fallback variants (proves `calls==3` with shared-seam accounting), T3 out-of-scan salvage, T4 both-DB-fail clean skip, T5 duplicate-skip, T6 all-stale, T7 multi-symbol partial, T8 mixed validity. `failingActivePosGetter` total-call budget stub; retry timing verified (T2b 0.31s, T4 0.61s).

### Safety (salvage guardrails)
- **Stage 2 DB failure falls through to salvage** instead of early-returning — if the retry still fails, salvage can still recover in-scan override symbols via `RescanSymbols`.
- **`stage2Attempted` populated BEFORE the `openedS1` duplicate check** — symbols filtered as already-open still count as attempted, so salvage does not re-open them.
- **Salvage's own `GetActivePositions` read also goes through retry helper** — symmetric protection.

## [0.32.40] - 2026-04-21

### Added
- **Price-gap tracker — simultaneous IOC entry path with unwind-to-match (Phase 08 Plan 05 Task 1)** (`internal/pricegaptrader/execution.go`). Implements the live-trading-critical `openPair` function for Strategy 4: concurrent IOC market orders on both legs, D-10 unwind-to-match partial-fill reconciliation via MARKET (not IOC) orders, zero-fill-on-one-leg → immediate market-close of the other leg, 5-consecutive-failures circuit breaker, per-symbol Redis lock via `AcquirePriceGapLock`, and D-15 position-ID format (`pg_{symbol}_{longExch}_{shortExch}_{unixNano}`). Fill price stamping uses the mid-at-decision from `DetectionResult` because the adapter's `GetOrderFilledQty(orderID, symbol) (float64, error)` returns only qty — realized slippage is computed at exit in Plan 06. Off by default (depends on `PriceGapEnabled=false` in Plan 01 + Plan 04 preEntry gate + this plan's circuit breaker). Tests in follow-up tasks.
- **Enumerate-and-argmax rebalance (feature-flagged `EnableArgmaxRebalance`, default OFF)** — plan `plans/PLAN-enumerate-argmax-rebalance.md` v8 ALL PASS on normal + independent Codex review. `rebalanceFunds()` dispatches to `rebalanceFundsArgmax` that enumerates feasible subsets of approved AND capital-rescue candidates, scores by net PnL = sum(baseValue) − dryRun.TotalFee, picks the globally best. Fixes the 2026-04-20 15:45 bug where 288 USDT donor pool sat idle because tail-prune dropped the BingX-constrained opp before enumeration.
- **Post-transfer replay helper (`replayArgmaxApprovalAfterFunding`)** — re-runs approval with projected post-transfer balances; admits rescue only when replay approves, uses post-transfer Size/Price for scoring. Closes both "size resize upward" and "non-capital gates bypass" risks.
- **`pricedCapitalRejection` helper (`internal/risk/manager.go`)** — all 4 `RejectionKindCapital` sites populate Size/Price/RequiredMargin/Long/ShortMarginNeeded. `ManualOpen(force=true)` rejects rejected approvals explicitly.
- **Argmax dashboard controls** — 4 new fields (`enable_argmax_rebalance`, `rebalance_min_net_pnl_usdt`=0.50, `rebalance_donor_floor_pct`=0.05, `rebalance_subset_size_cap`=0) wired through config/API/Redis/frontend/i18n + env vars.

### Changed
- **Rebalance wrapper** — dispatches to `rebalanceFundsArgmax` or `rebalanceFundsLegacy`; clears stale `allocOverrides` before argmax to prevent early-return leakage.
- **Rotate-scan gate (`engine.go:1471`)** — fires when either `EnablePoolAllocator` or `EnableArgmaxRebalance` is on.
- **Force Open button removed (`Opportunities.tsx`)** — backend now always rejects force on rejected approvals; button was dead.

### Safety (argmax guardrails)
- **Reserved-aware combo re-validation** — `reservedValidateArgmaxCombo` replays `SimulateApprovalForPair` sequentially over the chosen combo with accumulating reserved-margin map; picks that fail when earlier picks reserve capital on the same exchange are dropped, and the subset re-runs `dryRunTransferPlan` + netPnL scoring. Mirrors the legacy allocator guard at `allocator.go:393-426`. Fixes the race where two individually-approved picks share a constrained exchange and the second one fails at `executeArbitrage` time after donor capital was already moved.
- **Feature-flag emergency brake** — re-read `EnableArgmaxRebalance` under `capacityMu` twice (pre-execute, pre-publish) with the final check inside `allocOverrideMu`. `POST /api/config` invokes `Engine.ClearAllocOverrides()` on flag true→false transition.
- **Loss-limiter gate** — argmax calls `lossLimiter.CheckLimits()` at top mirroring `runPoolAllocator`, including dashboard broadcast + Telegram.
- **Post-execute capacity recheck** — if `ManualOpen` consumed slot during transfer window, drop the overrides (no publish).
- **Spot-futures occupancy gate** — `filterArgmaxOpps` excludes `exchange:symbol` occupied by the spot-futures engine, mirroring `ppCrossEngineBlocked`. Per-attempt re-check inside `buildArgmaxCandidates` covers alternative pairs too.
- **Deterministic tie-break** — netPnL desc → step-count asc → `comboSymbolsKey` lex asc; candidate sort uses `sort.SliceStable` + 3-level symbol/long/short lex tie-break.
- **Streaming best combo** — `*scoredCombo` tracker instead of `[]scoredCombo` slice; O(1) memory regardless of feasible subset count.

### Tests
- `rebalance_argmax_test.go` — pure helpers, `buildArgmaxCandidates` rescue admission, end-to-end smoke (override publish, no-opps clear, capacity-at-max early-return, capital-rescue replay with bitget→bingx step), targeted replay tests (`PromotesRescue` + `RejectsWhenStillInfeasible`).
- `manager_capital_rejection_test.go` — pins priced rejection fields non-zero at all 4 sites.

## [0.32.39] - 2026-04-21

### Fixed
- **`isAlreadyFlatError` missed Binance `-2022 ReduceOnly Order is rejected`** (`internal/spotengine/execution.go`). After deploying v0.32.38, the DEGO monitor retry attempted a futures BUY-to-close against an already-flat futures position and got `-2022`. The existing pattern `"reduce only order"` (with space) didn't match the Binance message `"ReduceOnly Order is rejected"` (no space). Consequence: the idempotent-flat branch was skipped, `futErr` was returned, and `emergencyClose` returned that error even though the spot leg succeeded with the v0.32.38 rounding fix. The monitor then looped forever. Added two patterns to `isAlreadyFlatError`: `"reduceonly order"` and `"code=-2022"`. Test cases added to `TestIsAlreadyFlatError`.

## [0.32.38] - 2026-04-21

### Fixed
- **Emergency close path also needs LOT_SIZE step rounding** (`internal/spotengine/execution.go`, `emergencyClose`). Codex review of v0.32.36 caught a gap: delist-triggered exits enter `monitor.go:102-110` with `emergency=true` → `ClosePosition(..., true)` → `emergencyClose()` (parallel leg-close path). That path also formatted raw `remaining` before `PlaceSpotMarginOrder`, so the FIRST close attempt of an unaligned-`SpotSize` position still failed with Binance `-1013 LOT_SIZE` — only later non-emergency monitor retries healed it. Now `emergencyClose()` applies the same `utils.RoundToStep(remaining, rules.QtyStep)` logic (SELL side only; Dir-A buyback unchanged to preserve residual-borrow semantics). Additionally, after the rounded fill confirms, the known dust residue between `sellQty` and original `remaining` is marked as complete instead of being reported as a partial-fill error, so the emergency path now closes cleanly on its first attempt rather than leaving the position stuck until the next 3-minute monitor tick. Regression test: `TestEmergencyClose_SpotSellRoundsToLotStep` in `dust_close_test.go`.

## [0.32.37] - 2026-04-21

### Fixed
- **ManualOpen bypassed the delist filter** (`internal/spotengine/execution.go`). Root cause of the DEGO incident: the user manually opened `DEGOUSDT` via `POST /api/spot/manual_open` while the discovery poller had already written `arb:delist:DEGOUSDT` to Redis. Three entry paths exist for spot-futures — `autoentry` (covered by `risk_gate.go:125`), `monitor` active-position check (`monitor.go:102`), and `ManualOpen` (previously unchecked). `ManualOpen` only consulted `opp.FilterStatus` which the scanner never populates from delist signals; the delist check lives one layer deeper in `risk_gate.go` which `ManualOpen` skips entirely. Now `ManualOpen` calls `e.db.IsDelisted(symbol)` directly, gated on the same shared `DelistFilterEnabled` flag (default true) used by autoentry + monitor. Closes the accidental-open window that triggered the v0.32.36 LOT_SIZE stuck-exit incident. Override = toggle the existing flag off; no new config key added. Regression test: `TestManualOpen_RejectsDelistedSymbol`.

## [0.32.36] - 2026-04-21

### Fixed
- **Spot-futures Dir B exit stuck in 'exiting' when SpotSize is not aligned to Binance LOT_SIZE stepSize** (`internal/spotengine/execution.go`). Incident: `sf-degousdt-1776738361635` opened with `SpotSize=1258.1406`, futures leg closed successfully on delist-trigger exit, but every spot SELL retry was rejected by Binance with `-1013 Filter failure: LOT_SIZE` because `1258.1406` is not a multiple of DEGOUSDT's `stepSize=0.01`. Two root-cause fixes:
  1. **Entry path** — `openDirBBuySpotShort` previously hardcoded `math.Floor(spotNetReceived*1e5) / 1e5` (fixed 5dp) when flooring net-received size after fee deduction. For coins whose LOT_SIZE stepSize is coarser than 5dp (DEGO=0.01), this left `SpotSize` stepSize-unaligned. Now floors to `rules.QtyStep` fetched from `SpotOrderRules(symbol)`, with 1e-5 as fallback if the rules lookup fails.
  2. **Exit path (Dir B)** — `dirB-spot-sell` retryLeg closure now applies `utils.RoundToStep(remaining, rules.QtyStep)` before submitting the SELL order. If flooring produces zero (residue < step), short-circuits and marks the spot leg closed (dust residue ignored) instead of submitting a doomed order. Protects existing stuck positions — after deploy, the next 3-minute monitor retry for DEGO will successfully sell 1258.14 and dust-close the 0.0006 residue.
  
  Regression test added: `TestCloseDirectionB_SpotSellRoundsToLotStep` in `dust_close_test.go` reproduces the exact DEGO scenario (SpotSize=1258.1406, stepSize=0.01) and asserts the first `PlaceSpotMarginOrder` Size is `"1258.140000"` (step-aligned) and the position closes cleanly via the residue dust path.

## [0.32.35] - 2026-04-20

### Added
- **Per-recipient/per-donor debug trace logs in `dryRunTransferPlan`** (`internal/engine/allocator.go`) — 5 Debug-level log points for diagnosing rank-first Tier-1 prune decisions. Previously, `dryRun: no donor for X deficit=N` was the only visible infeasibility marker; it was impossible to trace from logs alone which donor sent how much to which recipient, whether min-withdraw overfunding consumed capacity, or whether post-trade-ratio headroom inflated the true transfer need. Five new logs:
  1. Entry: `dryRun: start — choices=N recipients=M needs=<map>`
  2. Per-recipient start: `dryRun: recipient X starts: need=... marginDeficit=... ratioDeficit=... transferNeed=...`
  3. Per donor→recipient move applied: `dryRun: moved ... from Y to X (gross=... fee=... netCredit=... donorBudgetAfter=... recipientDeficitAfter=...)`
  4. Per-recipient closeout (funded or infeasible with residual)
  5. Function exit: `dryRun: end — feasible=T/F reason=...` before each return site
  
  Debug-level only (requires `DEBUG=true` env var) to avoid log pollution — rotate scan runs every 60 min; Info-level would add ~500-1200 lines/day. Plan file: `plans/PLAN-dryrun-transferplan-logs.md` v2 (ALL PASS on Codex review).

## [0.32.34] - 2026-04-20

### Added
- **Rank-first rebalance — invert pool allocator from primary to fallback** (plan v29 ALL PASS on normal + independent Codex review). `rebalanceFunds()` now runs the sequential rank-first planner as the PRIMARY path; the pool allocator runs only as FALLBACK when Tier-1 produces zero selected choices. Rank represents expected profitability from `discovery`, so capital now preferentially funds the highest-rank feasible combination rather than the `baseValue`-maximizing bundle.
  - **New typed rejection taxonomy** (`internal/models/interfaces.go`): `RejectionKind` enum with `None/Capital/Capacity/Market/Spread/Health/Config`. `RiskApproval` carries `Kind` so callers can decide rescue eligibility without string matching.
  - **Per-site annotations** (`internal/risk/manager.go`): all 22 rejection returns in `approveInternal` now set `Kind`. Notional-floor sites `:438`/`:503` use new `notionalRejectionKind(effectiveCap, leverage)` helper — tags as `Config` (unrescueable) when `effectiveCap * leverage < 10`, else `Capital`. Helper and call site use a single effectiveCap + leverage snapshot computed once in `approveInternal` and threaded into `calculateSizeWithPrice` as parameters, eliminating concurrent-cap-change race.
  - **Shared transferable cache** (`internal/engine/allocator.go`): inline donor-surplus builder extracted into `(e *Engine) buildTransferableCache(balances)` — used by both pool allocator and rank-first Tier-1. `revalCache` in `runPoolAllocator` now reuses the extracted cache.
  - **Per-symbol pair walk** (`internal/engine/engine.go`): Tier-1 evaluates primary + each `opp.Alternatives` pair before falling to rank-2. Mirrors pool allocator's alt enumeration. `firstErr` is diagnostic-only and never gates the outer loop.
  - **Tier-1 / Tier-2 inversion**: rank-first runs first; pool allocator fallback inserted after the summary log, before Phase 1 balance mutation. `EnablePoolAllocator=false` skips fallback entirely. RotateScan dispatcher unchanged.
  - **Alt-override gate extension**: `altOverrideNeeded` flag set when chosen attempt is alt or differs from primary; override-storage gates (no-cross and executor paths) store `allocOverrides` even without transfer so alt-pair selections reach the entry scan via `applyAllocatorOverrides` (fixes silent-revert-to-primary semantic gap).
  - **Rejection taxonomy test coverage**: `internal/risk/manager_test.go` (new, 948 lines) pinning every rejection site to its Kind. `internal/engine/allocator_transferable_parity_test.go` (new, 287 lines) asserting `buildTransferableCache` matches pre-extraction inline logic. `internal/engine/engine_rank_first_test.go` (new, 924 lines) covering 20 plan scenarios including notional-floor sub-cases, alt-pair coverage (no-transfer + with-transfer), symmetric-replay drop, and RotateScan gate.

### Notes
- Throughput tradeoff (accepted per user intent): once Tier-1 keeps even one ranked choice, pool allocator is not invoked that cycle — it cannot backfill remaining slots with lower-ranked positive-value bundles. Total deployed capital per cycle may be lower than pool-allocator-first behavior when feasible bundle value > top-N rank value. Tier-1 success log `tier1 selected N opps, skipping pool allocator fallback` makes this observable.
- Known approximation: notional-floor Kind classification uses `effectiveCap * leverage` alone; sizing-zero from step rounding / short-leg price discount / min-size / invalid price are tagged `Capital` (not `Config`) in that regime, triggering a futile donor-rescue attempt (no correctness bug, just wasted work). `false-Config` under the chosen condition is mathematically impossible. Follow-up marker: if live logs show > 5 futile rescues/week, open a sizing-reason extension plan to return structured reject causes from `calculateSizeWithPrice`.

## [0.32.33] - 2026-04-20

### Fixed
- **Binance Dir A borrow-cost under-counting** (`pkg/exchange/binance/margin.go`) — codex re-review `0b5f1811` caught this pre-existing bug exposed by the new borrow-coverage floor. Binance's `/sapi/v1/margin/interestRateHistory` returns one record per day, but the adapter was emitting one `MarginInterestRatePoint` per record with `HourlyRate = daily/24`. The engine sums `HourlyRate × 10000` across points to compute total borrow cost in bps — so a 7-day backtest summed only 7 points × (daily/24) × 10000 instead of the correct 7 × daily × 10000 (24× under-count). With the new coverage floor, 7 daily points over a 7-day window was 4% coverage, correctly failing the floor but masking the real bug. Adapter now expands each daily record into 24 hourly-equivalent points within `[start, end]`, preserving the interface contract and producing correct bps totals. New `TestGetMarginInterestRateHistoryBinanceDailyExpansionCostAccuracy` asserts `Σ HourlyRate × 10000 == daily × 7 × 10000` over 7 days as regression guard.
- **Dir A backtest frontend/backend policy drift** (`web/src/pages/Opportunities.tsx`) — codex audit `14064872` finding 1. The Backtest button on OKX/Bitget Dir A rows was always disabled because `SPOT_DIR_A_BACKTEST_EXCHANGES` hard-coded the native-only list, even when `SpotFuturesBacktestCoinGlassFallback` was on and the backend was ready to serve the request. Removed the frontend capability gate; the UI now always allows clicking Backtest, and the backend's 400 response (with a clear message from `RunSpotBacktestOnDemand` or the 400 from `insufficient borrow history`) surfaces in the modal's error area.
- **Dir A sparse-borrow bootstrap trap** (`internal/spotengine/backtest.go`) — codex audit `14064872` finding 2. `fetchAndCacheSpotBacktestDirA` and `runBacktestDirAOnDemand` now enforce a ≥50% borrow-history coverage floor (`len(borrowSeries) / days*24`). During CoinGlass fallback bootstrap, the scraper accumulates one sample per hour; 24h of data over a 7-day window would previously produce misleading NetBps (missing hours default to zero via the borrowByHour map lookup) and cache for 24h, potentially allowing OKX/Bitget Dir A opportunities to false-pass the MinProfit filter. The prefetch path now fails open (no cache write) below the floor; the on-demand path returns a descriptive error so the modal displays an actionable "data source still accumulating" message.
- 3 new regression tests in `dir_a_coverage_test.go` cover sparse (24/168h → no cache), full (168/168h → cache), and on-demand sparse (10h → error) paths.

## [0.32.32] - 2026-04-20

### Added
- **SaveJSON tripwire for critical numeric fields** (`internal/config/config.go`) — defense-in-depth against config-clearing incidents. The `keepNonZero` helper now guards `spot_futures.max_positions`, `spot_futures.leverage`, `spot_futures.capital_unified_usdt`, and `spot_futures.capital_separate_usdt` on every SaveJSON write. If a save attempts to zero a currently-non-zero on-disk value for any of these, the tripwire logs a loud `[config] TRIPWIRE: refusing to zero ...` line (including the Go caller) and preserves the disk value. Legitimate operational changes flip `*Enabled` booleans rather than zeroing capital/leverage, so an incoming zero is almost always a bug (stale dashboard state, missing guard in an update path, etc.). The tripwire is narrow — it does not block the save, only preserves the specific field.
- 10 unit tests covering disk-preserved, disk-missing, type-mixing, and normal-update cases.

### Fixed
- Live incident recovery: between v0.32.30 deploy (13:34) and v0.32.31 deploy (13:48), `spot_futures.max_positions`, `spot_futures.leverage`, and `spot_futures.capital_unified_usdt` were silently zeroed during a SaveJSON write with an unidentified caller. Manual restore from `config.bk` was required. The tripwire above makes future occurrences visible via logs and prevents the zeroing.

## [0.32.31] - 2026-04-20

### Fixed
- **Spot-Futures backtest toggles not saving from dashboard** (`internal/api/handlers.go`) — latent bug present since v0.32.21 (Dir B) and uncovered by v0.32.30 (Dir A CoinGlass fallback). The `spotFuturesUpdate` request struct and `configSpotFuturesResponse` response struct in the `/api/config` handler were both missing the four backtest fields (`backtest_enabled`, `backtest_days`, `backtest_min_profit`, `backtest_coinglass_fallback`). Dashboard POSTs parsed into a struct with no matching fields so values were silently dropped — the toggle UI would flip back on refresh because the GET response also lacked the fields. Fixed by wiring all four fields through the GET response (struct + build block) and the POST request (struct + apply block). Underlying `SaveJSON`/`Load` paths in `internal/config/config.go` were already correct; only the API handler was unplumbed.

## [0.32.30] - 2026-04-20

### Added
- **Spot-Futures Dir A backtest — CoinGlass fallback for OKX and Bitget (Phase 2)**. The two exchanges lack public historical borrow-rate APIs; previously Dir A backtest was gated off for them. This release adds a fallback that reads from a hourly-accumulated Redis time series populated by `/var/solana/data/coinGlass/fetch_margin_fee.js` (new scraper, runs under PM2).
  - **New config field** (`internal/config/config.go`):
    - `SpotFuturesBacktestCoinGlassFallback bool` — JSON: `backtest_coinglass_fallback`, ENV: `SPOT_FUTURES_BACKTEST_COINGLASS_FALLBACK`, **default OFF**. Wire through `jsonSpotFutures`, JSON read, JSON write, env-var parsing, and defaults block.
  - **New database helper** (`internal/database/coinglass_margin_fee.go`): `GetCoinGlassMarginFeeHistory(exchange, coin, start, end) ([]CoinGlassMarginFeePoint, error)` reads the rolling Redis list `coinGlassMarginFee:hist:{exchange}:{coin}` (written by the scraper's hourly LPUSH + LTRIM 0 719), parses each entry, and filters by time window. Malformed entries skipped silently to survive scraper drift.
  - **New engine helpers** (`internal/spotengine/backtest.go`):
    - `exchangeSupportsCoinGlassDirAFallback(exch)` — returns true for `okx`, `bitget`.
    - `(e *SpotEngine) canRunDirABacktest(exch)` — layered gate: native list (`binance`/`bybit`/`gateio`) always true, fallback list gated by config flag.
    - `(e *SpotEngine) loadBorrowHistoryWithFallback(ctx, smExch, exchName, baseCoin, start, end)` — tries adapter's native `GetMarginInterestRateHistory` first; on `ErrHistoricalBorrowNotSupported` AND fallback flag enabled AND Redis has data, returns CoinGlass points converted to `MarginInterestRatePoint`. Preserves the sentinel when the fallback is off or empty so upstream fail-open logic stays intact.
  - **Discovery + prefetch hooks** now use `canRunDirABacktest` instead of the native-only `exchangeSupportsDirABacktest`, and both borrow-history fetch sites (`runBacktestDirAOnDemand`, `fetchAndCacheSpotBacktestDirA`) route through `loadBorrowHistoryWithFallback`.
  - **Bootstrap**: the CoinGlass scraper accumulates 1 sample per hour; 168 samples = 7 days of coverage. During bootstrap the fallback returns the sentinel (fail-open) and native-supported exchanges are unaffected.
  - **No UI change** — Dir A button already enables for supported exchanges at the engine layer.

### Fixed (review follow-ups on v0.32.30)
- **Dashboard toggle** for `SpotFuturesBacktestCoinGlassFallback` (`web/src/pages/Config.tsx`) — the config flag was previously wired only through `config.json` and env vars, violating the new-feature rollout rule (config + dashboard toggle + persistence). Now exposed as a toggle in the Spot-Futures → Dir B Backtest section, next to the existing backtest controls. i18n keys added to `web/src/i18n/en.ts` and `web/src/i18n/zh-TW.ts`. No API handler change needed — the generic `POST /api/config` path already accepts the new field via the existing `jsonSpotFutures.backtest_coinglass_fallback` wiring.
- **`GetCoinGlassMarginFeeHistory` JSON parsing** (`internal/database/coinglass_margin_fee.go`) — the parser declared a shared `entry` struct outside the loop. `encoding/json.Unmarshal` leaves fields unchanged when the input omits them, so a valid entry followed by `{}` could inherit the previous sample's `t` or `rate` and be returned as real data. Replaced with a fresh struct per iteration and pointer fields (`*int64`, `*float64`) so missing required fields now skip the entry instead of inheriting or zero-polluting. New regression test `TestGetCoinGlassMarginFeeHistory_PartialEntryDoesNotInheritPriorValues` locks this in.

## [0.32.29] - 2026-04-19

### Fixed
- **Spot-futures "stuck in exiting" dust lockup** — positions no longer loop forever when the spot exit leaves an untradeable residual (first hit GUAUSDT, recurred on GWEIUSDT 2026-04-19).
  - New `SpotOrderRules(symbol)` on `SpotMarginExchange` — returns `MinBaseQty`, `QtyStep`, `MinNotional` from each exchange's **spot market** endpoint (Binance `/api/v3/exchangeInfo`, Bybit `/v5/market/instruments-info?category=spot`, Gate.io `/spot/currency_pairs`, OKX `/api/v5/public/instruments?instType=SPOT`, Bitget `/api/v2/spot/public/symbols`). 5-min per-symbol cache inside each adapter.
  - Close paths (`closeDirectionB`, `closeDirectionA`, `emergencyClose` spot goroutine) now detect untradeable dust via `isSpotResidualDust(rules, remaining, price)` — dust if `floor(remaining/step)*step < MinBaseQty` or `effective * price < MinNotional`. Marks the spot leg closed with a `(spot dust residue ignored: X BASECOIN)` note on `ExitReason`.
  - Dir A dust short-circuit additionally verifies `GetMarginBalance.Borrowed == 0` before marking closed — prevents losing track of outstanding borrow liability.
  - Futures close is now idempotent on "empty position" / "position not exist" errors via `isAlreadyFlatError` + `verifyFuturesFlat(GetPosition)` double-check. Populates `FuturesExit` from orderbook mid when the exchange provided no fill price.
  - Retry-compatible: currently-stuck positions auto-heal on the next monitor retry after deploy. No Redis cleanup required.

### Fixed (review follow-ups on v0.32.29)
- **Bitget spot rules parser** (`pkg/exchange/bitget/spot_rules.go`) — the live Bitget v2 `/api/v2/spot/public/symbols` response uses `quantityPrecision` (not `quantityScale`). The original parser silently defaulted `QtyStep` to 1, which could cause `isSpotResidualDust` to flag a tradeable residual as dust on Bitget symbols with real sub-integer step precision. Parser now reads `quantityPrecision` with `quantityScale` retained as a legacy fallback. Golden fixture re-recorded from a live endpoint response; added a test that covers the legacy-field fallback path.
- **Bybit spot rules parser** (`pkg/exchange/bybit/spot_rules.go`) — the live Bybit v5 `/v5/market/instruments-info?category=spot` response nests `minOrderAmt` inside `lotSizeFilter`, not inside a separate `quoteFilter` object. The original parser read only `quoteFilter.minOrderAmt`, so `MinNotional` silently parsed as 0 — meaning `isSpotResidualDust` would never catch a sub-notional residual via the notional check and instead keep attempting doomed orders (the opposite failure mode from the Bitget bug: preserving a dust lockup rather than skipping a tradeable residual). Parser now reads `lotSizeFilter.minOrderAmt` first, with `quoteFilter.minOrderAmt` retained as a legacy fallback. Golden fixture trimmed to the current v5 shape (lotSizeFilter only); added `TestSpotOrderRulesBybitMinNotionalInLotSizeOnly` (would fail under the old parser) and `TestSpotOrderRulesBybitLegacyQuoteFilterFallback` (covers version drift).

## [0.32.28] - 2026-04-19

### Added
- **Spot-Futures Dir A (`borrow_sell_long`) backtest capability** for Binance, Bybit, and Gate.io. OKX and Bitget remain deferred (OKX has CSV-only export; Bitget's history endpoint is user-scoped, not public market rates).
  - **New `SpotMarginExchange` interface method** (`pkg/exchange/types.go`): `GetMarginInterestRateHistory(ctx, coin, start, end) ([]MarginInterestRatePoint, error)`. Adapters that lack a public historical API return `ErrHistoricalBorrowNotSupported`; callers fail-open on this sentinel (same semantics as a cache miss).
    - Binance: `GET /sapi/v1/margin/interestRateHistory` — daily granularity, paginates in 30-day windows; `HourlyRate = dailyInterestRate / 24`.
    - Bybit: `GET /v5/spot-margin-trade/interest-rate-history` — hourly, paginates in 30-day windows, VIP level `No VIP`.
    - Gate.io: `GET /unified/history_loan_rate` — hourly, page/limit pagination with client-side `[start, end]` filtering (endpoint has no date params).
    - OKX + Bitget: return `ErrHistoricalBorrowNotSupported` immediately.
  - **`backtestDirA` filter** (`internal/spotengine/backtest.go`): computes `netBps = −Σ fundingBps − Σ borrowBps` over the configured lookback window. Cache key: `arb:spot_backtest:{symbol}:{exchange}:borrow_sell_long:{days}` with 24 h TTL. Cache miss → fail-open (opportunity passes filter). Unsupported exchange → no-op (no cache write). Shares existing Loris 429 backoff from `discovery.TriggerLorisBackoff`.
  - **`exchangeSupportsDirABacktest` helper** gates the filter in one place — currently `binance`, `bybit`, `gateio`.
  - **Discovery hook** (`internal/spotengine/discovery.go`): native-scan and CoinGlass-scan Dir A paths call `backtestDirA` when `SpotFuturesBacktestEnabled && exchangeSupportsDirABacktest(exch)` and the position is not already active.
  - **`RunSpotBacktestOnDemand`** now accepts `direction=borrow_sell_long` on supported exchanges (previously rejected with 400). Returns extended `SpotBacktestReport` with optional `funding_bps` and `borrow_bps` breakdown fields (zero-valued for Dir B — backward compatible).
  - **Dashboard** (`web/src/pages/Opportunities.tsx`): Backtest button is enabled on Dir A rows for `binance`/`bybit`/`gateio`; rows for unsupported exchanges show a per-exchange tooltip (`spotBacktest.modal.notSupportedExchange`). Modal renders two additional stat tiles — funding bps and borrow bps — alongside the existing net-bps tile.
  - **i18n** (`web/src/i18n/en.ts` + `zh-TW.ts`): `spotBacktest.modal.fundingBps`, `spotBacktest.modal.borrowBps`, `spotBacktest.modal.netBps`, `spotBacktest.modal.notSupportedExchange` (with `{exchange}` placeholder).
  - **No new config** — reuses `SpotFuturesBacktestEnabled`, `SpotFuturesBacktestDays`, `SpotFuturesBacktestMinProfit`.

### Fixed (review follow-ups on v0.32.28)
- **API handler now accepts Dir A** (`internal/api/spot_handlers.go`) — `handleSpotBacktest` previously rejected every direction other than `buy_spot_short` with a 400 before reaching the engine, making the new Dir A modal unreachable from the dashboard. The handler now accepts both `buy_spot_short` and `borrow_sell_long`, forwards to the engine, and surfaces engine errors (e.g. "unsupported on okx") as 400 so the UI can render them inline. Two new tests cover the routing and the error-surfacing behavior.
- **Frontend Dir-A detection uses `opp.direction`** (`web/src/pages/Opportunities.tsx`) — previously checked `result.funding_bps !== undefined`, but Go's `omitempty` would drop a legitimate zero-valued `funding_bps` and cause the modal to fall back to the Dir B layout. Now keys off the already-known `opp.direction` instead; the Dir A tile values use `?? 0` defensively.
- **`TestBacktestDirASignMath` strengthened** (`internal/spotengine/backtest_test.go`) — now asserts the exact expected values (`FundingBps=15`, `BorrowBps=24`, `NetBps=-39`) in addition to sign direction. Catches magnitude regressions, not just sign flips.

## [0.32.27] - 2026-04-18

### Added
- **Spot-Futures Dir B backtest capability** (integrated from egg's `feat/spot-futures-dir-b-backtest`, originally tagged v0.32.21 on their branch — renumbered here to preserve our sequential numbering) — historical funding filter for `buy_spot_short` opportunities + on-demand UI backtest modal. Dir A (`borrow_sell_long`) is not yet supported (pending historical borrow-rate source).
  - **Background filter** (`internal/spotengine/backtest.go`, new): `backtestDirB` checks N days of historical Loris funding data after the net-APR check. Cache miss = fail-open (same as perp-perp). `prefetchSpotBacktestData` prefetches for passing opps after each scan. `RunSpotBacktestOnDemand` runs a fresh uncached fetch for the UI.
  - **Discovery hook** (`internal/spotengine/discovery.go`): if `SpotFuturesBacktestEnabled` and direction is `buy_spot_short`, calls `backtestDirB`; sets `FilterStatus` to reason on fail. Active positions bypass. Dir A unchanged.
  - **New config fields** (`internal/config/config.go`):
    - `SpotFuturesBacktestEnabled bool` — JSON: `backtest_enabled`, ENV: `SPOT_FUTURES_BACKTEST_ENABLED`, **default OFF**
    - `SpotFuturesBacktestDays int` — JSON: `backtest_days`, ENV: `SPOT_FUTURES_BACKTEST_DAYS`, default 7
    - `SpotFuturesBacktestMinProfit float64` — JSON: `backtest_min_profit`, ENV: `SPOT_FUTURES_BACKTEST_MIN_PROFIT`, default 0 bps
  - **New endpoint** (`internal/api/spot_handlers.go`): `POST /api/spot/backtest` — parses `{symbol, exchange, direction, days}`, rejects Dir A with 400, calls `RunSpotBacktestOnDemand`, returns standard `{ok, data}` envelope.
  - **Dashboard config controls** (`web/src/pages/Config.tsx`): enable toggle, days input, min-profit-bps input under Spot-Futures tab, mirroring perp-perp backtest controls.
  - **On-demand modal** (`web/src/pages/SpotPositions.tsx`): Backtest button on each Dir B opportunity row; modal shows funding sum bps, APR projection, settlement count, coverage %, and per-day breakdown. Dir A button disabled with tooltip.
  - **i18n**: matching keys added to `web/src/i18n/en.ts` and `web/src/i18n/zh-TW.ts`.

### Fixed (review follow-ups merged in)
- **Backend/frontend JSON contract** — `SpotBacktestReport` now emits the field names the dashboard expects: `projected_apr`, `settlement_count`, `coverage_pct` (0–100 percent, not 0–1 ratio), and `days: [{date, bps}]`. Prevented the on-demand modal from crashing at `result.days.length` on first successful response.
- **Stop-aware prefetch goroutine** — the spot backtest prefetch goroutine is now tracked in `SpotEngine.wg` via `launchBacktestPrefetch`, with a `sleepInterruptible` helper and `e.stopping()` checks inside the loop. `Stop()` waits for it to finish before returning.
- **Shared Loris 429 backoff** — moved the rate-limit cooldown to package-level `discovery.LorisBackoffUntil` / `TriggerLorisBackoff`, shared between the perp-perp `Scanner` and the spot-futures `SpotEngine`. `FetchLorisHistoricalSeries` auto-triggers on 429, so a 429 seen by either engine now suppresses the other instead of each engine keeping its own cooldown.

## [0.32.26] - 2026-04-18

### Fixed
- **Pool allocator executor now honors simulation's Steps (planner/executor parity).** At 2026-04-18 14:45 UTC on v0.32.25, pool allocator selected SIGNUSDT gateio/bingx. Dry-run `simulation passed`, gateio→bingx 132.55 USDT executed successfully, but override dropped (`kept=0`) with empty shortReason and entry at 14:55 rejected (gateio post-trade ratio 0.91 > L4 0.80). Codex deep-dive confirmed: `dryRunTransferPlan` produces `Steps []transferStep` (donor→recipient assignments with fee/chain/minWd), but `executeRebalanceFundingPlan` threw away `Steps` and re-ran greedy donor selection from `needs` only. Same bug class as v0.32.24 Bug B (sequential planner/executor mismatch), in the pool allocator path. When the same exchange served as both donor AND trading leg, executor's re-greedy routing drained more than sim validated → post-exec state failed `keepFundedChoices` replay. Plan v3 ALL PASS + Codex post-review PASS. Changes:
  - **`transferStep` struct** (`internal/engine/allocator.go:68-75`): added `MinWithdraw float64` field. `dryRunTransferPlan` now populates `MinWithdraw: minWd` (line 1228-1233) so executor's late min-withdraw safety check can use sim's validated value, not rediscover it.
  - **`allocatorSelection` struct** (`:257-263`): added `plan dryRunResult` field. `runPoolAllocator` assigns `plan: simResult` before returning the feasible selection.
  - **`executeRebalanceFundingPlan` signature** (`:1300`): added `plannedSteps []transferStep` parameter.
  - **Pool allocator call site** (`internal/engine/engine.go:633`): passes `allocSel.plan.Steps` instead of `nil`.
  - **Sequential fallback call site** (`:1097`): passes `nil` (sequential has its own dry-run prune per v0.32.24 Bug B).
  - **Executor loop** (`allocator.go:1477-1616`): builds `plannedByRecipient` map; for each recipient's cross-deficit, if `plannedSteps != nil`, dequeues steps from the map (using step.From/Fee/MinWithdraw/Chain directly) instead of running greedy donor selection. Greedy fallback preserved when `plannedSteps == nil`. Variable scoping renamed to avoid Go redeclaration (chain/fee/minWd now declared at loop top).
- Net effect: pool allocator's simulation is now binding — if sim validates a specific donor→recipient→amount routing as feasible, executor will execute exactly that plan. Prevents donor-leg overlap divergence where greedy re-allocation drained a donor's own-leg capacity.

## [0.32.25] - 2026-04-18

### Fixed
- **Cross-exchange withdraw rate-limit throttle.** At 2026-04-18 12:45:10 UTC, rebalance for SIRENUSDT triggered two gateio withdraws 1ms apart (bingx + binance recipients); second hit `code=TOO_FAST Withdrawal frequency is limited to 10s`. gateio had 314 USDT available — not a balance issue. Bot's batched-withdraw executor (`internal/engine/allocator.go:1826`) had no per-donor timing gate. Added `lastWithdrawAt map[string]time.Time` in the batched-withdraw loop: before each `Withdraw()` API call, if `time.Since(lastWithdrawAt[donor]) < WithdrawMinIntervalMs`, sleep the remaining delta; timing anchor is request-dispatch (not response-completion). New config `WithdrawMinIntervalMs` (default 11000ms = 11s, 1s buffer above gateio's 10s hard limit; bybit documented same 10s-per-coin/chain limit) fully wired through `Config` / `jsonRisk` / defaults / `applyJSON` / `SaveJSON` in `internal/config/config.go` plus `configRiskResponse` / `buildConfigResponse` / `riskUpdate` / `handlePostConfig` / Redis flat-map in `internal/api/handlers.go`. Dashboard: new `NumberField` in Config → Appearance (actually Risk) tab, i18n keys added to `en.ts` and `zh-TW.ts`. Regression test at `internal/api/config_handlers_test.go::TestHandleConfig_WithdrawMinIntervalMs` covers GET, POST, in-memory update, Redis persistence. Plan v5 ALL PASS + Codex post-review PASS (with missing test finding addressed).

## [0.32.24] - 2026-04-18

### Fixed
- **Rebalance partial-success: 4 bugs from post-v0.32.23 3-round log audit.** Plan v5 ALL PASS + Codex post-review PASS. Round 2 of 3 rounds opened SOONUSDT successfully, but Rounds 1 and 3 still had transfers execute without positions opening. Codex 2097-line audit surfaced 4 additional bugs (Bug E over-solicitation deferred as optimization):
  - **Bug A+D (allocator.go:2024-2067) — Deadline-fallback missing accounting + unified uses wrong balance API.** v0.32.23's Bug 5 deadline-fallback credited spot but never wrote `result.Unfunded[recipient]` or `result.SkipReasons[recipient]`, so `complete (unfunded=0)` log was misleading. Also: for unified recipients (bybit UTA, gateio unified), `GetSpotBalance()` always returns 0, so late-arriving deposits on unified exchanges could not be recovered. Fix: deadline-fallback now branches on `IsUnified()` — unified uses `GetFuturesBalance()` and credits `bi.futures`/`bi.futuresTotal`; split uses `GetSpotBalance()` and credits `bi.spot`. Shortfall (not covered by late arrival) is written to `result.Unfunded` and `result.SkipReasons`.
  - **Bug A2 (allocator.go:1941) — Unified baseline `startBal` fallback used `.spot`.** When `GetFuturesBalance()` fails during deposit-baseline setup for unified recipient, old code fell back to `bi.spot` which is always 0 for unified → later poll math over-credited any late deposit as "arrived". Fix: fallback now uses `bi.futures` which represents the unified pool.
  - **Bug B (engine.go:695-941) — Sequential planner/executor mismatch stranded legs.** Sequential planner computed donor→recipient assignments into `plannedTransfers`, but executor ignored it and greedy-allocated donors from `needs` in fixed order. Round 3: gateio consumed all donors first, bingx got zero transfers despite being in rescue plan. Fix: planner-side validation via new `dryRunTransferPlan` prune loop. Added `sequentialChoice` struct + `selectedChoices []sequentialChoice` tracking both rescue AND normal selections. Dry-run verifies whole plan feasibility; while infeasible, pops the last-added choice (restoring `needs`/`reserved`/`selectedSymbols`) until feasible subset remains.
  - **Bug C (engine.go:981, 1062, 1077-1084, 1097-1105) — Sequential fallback never stored overrides.** The fallback path discarded `executeRebalanceFundingPlan` result with `_ = ...`, so partial rescue success (e.g. gateio funded but bingx not) could never steer entry at :55 — always went tier-3 tier-3-rank-based. Fix: tracked `localTransferHappened` flag in sequential flow scope (set on each successful local spot→futures); `crossDeficits==0` early return now stores overrides via `keepFundedChoices(selectedChoices, balances, nil)` when local transfers succeeded; cross-exchange path consumes `result` and gates override storage on `localTransferHappened || result.LocalTransferHappened || result.CrossTransferHappened`.
- Bug E (over-solicitation of ratioDef when marginDef is smaller) deferred — optimization, not correctness issue.

## [0.32.23] - 2026-04-18

### Fixed
- **Rebalance deposit race — 5 bugs + log fix from 2026-04-18 06:45 UTC SIGNUSDT incident.** Plan v3 ALL PASS + Codex post-review PASS. Root story: bingx alone could cover 111.42 USDT deficit to binance; bitget was unnecessarily bumped +10 to meet its 10-USDT min-withdraw floor, so target became 121.42. bingx landed in 40s, bitget late. 90% threshold falsely declared "all deposits confirmed" (bal=111.42 ≥ 109.28), then `TransferToFutures("121.42")` failed `code=-5013`. No retry despite 1m43s remaining rebalance lifetime. Non-unified credit path skipped on error → override dropped. At :55 entry, auto-transfer only covered the 67-USDT deficit, leaving 54.48 idle — post-trade L4 ratio hit 0.91 > 0.80 due to artificially low total → rejected. Fixes:
  - **Bug 2 (replay + credit for split-account).** `canReserveAllocatorChoice` now runs balances through new `replayUsableAllocatorBalance` helper that simulates split-account spot→futures (unified exchanges unchanged), so stranded spot no longer fails the override replay. Deadline-fallback credits `bi.spot` (not `bi.futures`) so replay handles the sweep.
  - **Bug 3 (sweep all in fixed-capital).** `internal/risk/manager.go` `ensureFuturesBalance` now uses `sweepTarget=1e9` in both fixed-capital and auto-size branches; previously fixed-capital left spot idle → L4 projection used artificially low futures total.
  - **Bug 4 (min-withdraw overfund guard).** When >= 90% of the original deficit is already scheduled from prior donors, the min-withdraw bump at `allocator.go:1589` is skipped (marks residual as `Unfunded`, no over-solicitation).
  - **Bug 5 (retry-to-deadline + Bug 1 cap + log).** Receiver block at `allocator.go:1977-2023` now: (a) caps `moveAmt` to `min(totalPending, arrivedAmt)` instead of transferring 100% target; (b) unified fast-path credits directly without `TransferToFutures` API call (mirrors v0.32.21 Section A); (c) split-account path on error uses `continue` to retry next poll iteration with fresh `GetSpotBalance()` until `pollDeadline`; (d) on deadline hit, credits `bi.spot` for override retention via Bug 2 replay. Log renamed `all deposits confirmed` → `deposit threshold met (90%)` to stop misleading triage.

## [0.32.22] - 2026-04-18

### Fixed
- **Gate.io `EnsureOneWayMode` used JSON body; Gate.io actually requires query parameter.** After v0.32.21 hard-failed startup on `EnsureOneWayMode` error, the bot entered a crash loop because Gate.io returned `MISSING_REQUIRED_PARAM: dual_mode` every call. Investigation: `/futures/{settle}/dual_mode` endpoint rejects JSON body and expects `dual_mode` as a query parameter. Previous adapter comment/test asserting "JSON body" was a wrong assumption (doc spec did not match runtime behavior). Fix: pass `dual_mode=false` in query string, empty body. Test renamed `UsesJSONBody` → `UsesQueryParam` and asserts query + empty body. Verified in production: all 6 exchanges now report "one-way mode confirmed".

### Integration
- Merged `origin/main` (egg's v0.32.20 Appearance toggle, frontend-only) into branch. VERSION bumped to 0.32.22 to resolve duplicate v0.32.20 commit collision (both branches had diverging v0.32.20 tags). CHANGELOG consolidates both v0.32.20 entries under the same section below.

## [0.32.21] - 2026-04-18

### Fixed
- **Post-v0.32.20 log audit — 7 bugs.** Open-ended Codex log audit over 11 rotate/entry cycles (since v0.32.20 deploy at 16:22 UTC) surfaced 7 additional issues affecting transfer-no-open reliability. Plan v2 ALL PASS + Codex post-review PASS. Shipped as single patch:
  - **Bug β — Bybit unified receiver TransferToFutures call was inappropriate.** `allocator.go` receiver credit path unconditionally called `TransferToFutures()` after deposit confirmation. For Bybit UTA, this returned `code=131212 user insufficient balance` because UTA has no separate spot wallet — deposit lands directly in unified pool. Failure skipped the credit path, leaving `PostBalances` and `FundedReceivers` stale; `keepFundedChoices` then dropped the override despite successful deposit. Gate.io unified already handles this via adapter no-op; Bybit needed gate at allocator call site. Added unified-check branch before `TransferToFutures` that credits balances directly + sets `CrossTransferHappened=true`.
  - **Bug 3 — Rebalance L4 headroom too tight.** All 5 deficit-sizing sites (`allocator.go`, `engine.go`) computed `targetRatio := MarginL4Threshold - marginEpsilon` where marginEpsilon=0.005. Transfers sized to hit L4 exactly; any tiny intervening price move pushed post-trade ratio to 0.81+ and entry rejected. Centralized via new `rebalanceTargetRatio()` helper using new `MarginL4Headroom` config (default 0.05 = 5% headroom below L4). Full config wiring added.
  - **Bug 2 — Binance -2019 insufficient margin wasted 5 IOC retries.** First-leg IOC returned `code=-2019` when cached balance diverged from live. Engine retried 5× same size → circuit breaker trip → trade aborted. Added `refetchMarginCappedSize()` helper; first-leg IOC handlers (short + long) now detect margin error via existing `isMarginError`, refetch live balance, downsize once, retry; if refetched size < minSize, abort immediately. Second-leg handlers unchanged (require rollback/match, not downsize).
  - **Bug 4 — Override stored even when no transfer occurred.** `rebalanceFunds` stored allocator override whenever it selected a pair, regardless of whether actual transfer happened (`crossDeficits empty after local fund relief` case). Next entry scan hit "all overrides stale" → tier-3 fallback. Extended `rebalanceExecutionResult` with `LocalTransferHappened` + `CrossTransferHappened` bools. Set on successful local/cross transfer. Engine short-circuits override storage if neither flag is set.
  - **Bug 6 — BingX 109400 "order not exist" cancel spam.** `CancelOrder` returned `code=109400` when order already completed; handler logged WARN and fell through. Added 109400 to adapter-level idempotent code list (alongside existing 80018/80016); engine cancel-path falls through cleanly to the existing `GetOrderFilledQty` REST re-query for terminal state.
  - **Bug 1 — Gate.io EnsureOneWayMode startup error not hard-failed.** Every startup, gateio `EnsureOneWayMode` logged error and bot continued assuming one-way. If account actually in hedge mode, trades fail silently. Changed startup loop to `os.Exit(1)` on EnsureOneWayMode failure.
  - **Bug 7 — Donor capacity log spam.** Single INFO line emitted 30+ times per rebalance cycle from branch-and-bound paths. Downgraded to DEBUG.
- Bug 5 (allocator symbol cooldown on repeated rejection) intentionally deferred for separate patch.

## [0.32.20] - 2026-04-18

### Fixed
- **Rebalance partial-transfer: 3 bugs exposed by 2026-04-17 14:45 UTC SOONUSDT incident.** Rotate scan selected SOONUSDT binance/bingx; bingx needed 116.82 USDT cross-exchange. Four donors attempted, only bitget succeeded (33.84 USDT). Allocator override dropped (kept=0), entry scan re-ranked SOONUSDT as gateio/bingx, gateio had no capital, risk rejected at post-trade margin 0.98 > L4 0.80. Net: bitget's 33.84 USDT sat unused on bingx for a full scan cycle; no position opened. Fixes (plan v4 ALL PASS, Codex post-review PASS):
  - **Bug 1 — Binance split-account donor ignored authoritative `maxWithdrawAmount=0`.** Log showed `futuresAvail=187.66 maxTransferOut=0.00` yet `TransferToSpot 24.79` returned `code=-5013 insufficient`. Allocator's fallback-to-L4-safety-estimate path is intended for adapters that don't populate the field at all; Binance's `maxWithdrawAmount=0` with open positions is authoritative ("no withdrawable collateral"). Added `MaxTransferOutAuthoritative bool` to `exchange.Balance` (pkg/exchange/types.go), set `true` by Binance (always populated) and by Bybit (only when `/v5/account/withdrawal` endpoint succeeds — preserves v0.32.10 fallback for endpoint failures). Three allocator sites now gate on the flag: `allocatorDonorGrossCapacity` scan-time capacity (internal/engine/allocator.go), `dryRunTransferPlan` feasibility path, and executor pre-withdraw re-cap. Non-authoritative adapters keep the existing `0 = unknown` contract.
  - **Bug 2 — `moveAmt` formatted to `"0.0000"` string triggered bitget `code=40020`.** `freshBal.MaxTransferOut` could be a tiny positive (e.g. 0.00003) that passed `> 0` and `moveAmt <= 0` guards but rounded to `"0.0000"` via `%.4f`. Added post-format reparse guard before `TransferToSpot` — if the formatted string parses back to zero, skip with warning.
  - **Bug 3 — Override pair dropped despite receiver leg funded cross-exchange.** `keepFundedChoices` replayed choices against executor's `PostBalances`, which under-reflects reality when receiver's `spot→futures` has been applied but the post-balance projection lags. Once `kept=0`, entry scan fell to scanner's fresh ranking (SOONUSDT re-picked as gateio/bingx), ignoring that bingx just received 33.84 USDT via APT. Extended `rebalanceExecutionResult` with `FundedReceivers map[string]float64` populated after successful receiver `TransferToFutures`. `keepFundedChoices` now takes a third `funded` param; when a pair would drop AND either leg is in `funded`, re-fetches live balance via new `fetchLiveRebalanceBalance` helper (mirrors snapshot site — `GetFuturesBalance + GetSpotBalance + db.GetActivePositions()`) and re-runs the reservation check against live data. If feasible, retains the override — existing `applyAllocatorOverrides` then uses `opp.Alternatives` to patch the current scan back to the funded pair (binance/bingx), preventing cross-scan capital stranding.

### Changed
- **Dashboard design toggle — users can choose Classic (pre-v0.32.18) or New (Binance-inspired) design.** Default is now **Classic**, preserving the look that predates the v0.32.18 redesign for users who didn't like the rebrand. The New design is opt-in via Config → **Appearance** tab.
  - **`web/src/theme/index.ts`** (new): `ThemeContext` + `useTheme()` hook, mirrors the `LocaleContext` pattern. localStorage-backed (`arb-theme` key), default `'classic'`. No backend config — UI preference is per-browser, not per-deployment.
  - **`web/src/index.css`**: Binance color tokens moved out of global `@theme` into `:root[data-theme="new"]`, so Tailwind v4 utility classes (`bg-gray-400`, `text-gray-900`, etc.) resolve to Binance values only when the new theme is active. When `data-theme="classic"` (default), the Tailwind v4 built-in palette applies — restoring the pre-v0.32.18 look. `.btn-primary` / `.btn-secondary` / focus rings / body background all gate on `[data-theme]`.
  - **`web/src/App.tsx`**: wraps the app in `<ThemeContext.Provider>`, syncs `document.documentElement.dataset.theme` on every theme change via `useEffect` (no page reload needed). Mobile header, update banner, TradFi banner, and update modal each branch on theme to match the classic palette.
  - **`web/src/components/{Sidebar,StatusBadge,TimeRangeSelector}.tsx` + `pages/Login.tsx`**: each component reads `useTheme()` and renders the classic (pre-ae8c563) or new markup accordingly. Props and behavior identical — only visuals differ.
  - **`web/src/pages/Config.tsx`**: new rightmost strategy tab `Appearance` with a two-option toggle (Classic / New Design). Theme switches live on click.
  - **i18n**: 5 new keys synced in `en.ts` and `zh-TW.ts` (`cfg.tab.appearance`, `cfg.appearance.theme`, `cfg.appearance.theme.{new,classic,desc}`).
  - **Frontend-only change** — no Go code, backend, or `config.json` touched. No new npm dependencies.

## [0.32.19] - 2026-04-17

### Fixed
- **Bybit `GetClosePnL` had two PnL bugs — one sign flip + one missing funding term** (`pkg/exchange/bybit/adapter.go`). Reported against `nomusdt-1776033907476`; dashboard Close P/L showed `-73.26` and trade Total P/L showed `35.38`, both wrong. Live reconcile logs (`[reconcile-debug] ... shortAgg Fees=0.294327 Funding=-28.451038 PricePnL=-73.259710 NetPnL=72.965383`) confirmed both issues.
  - **Bug 1 — `PricePnL` sign-flipped for shorts**: `pricePnL := cumExit - cumEntry` is only correct for longs. Bybit's `cumEntryValue` / `cumExitValue` are unsigned notionals (no direction), so winning shorts had `ShortClosePnL` stored with the wrong sign (right magnitude). All other adapters (Bitget / Gate / BingX / OKX / Binance) pull a pre-signed PnL directly from their respective endpoints — only Bybit re-derived from cumulative notionals. Fix: flip sign after side normalization when `side == "short"`. Downstream field affected: `BasisGainLoss` (`exit.go:1255`, `consolidate.go:678` — sum of long + short PricePnL).
  - **Bug 2 — `NetPnL` missing funding**: prior code and comments ("closedPnl already includes funding") were wrong. Empirically `closedPnl = pricePnL − openFee − closeFee` with no funding term, but every other adapter returns `NetPnL = price + fees + funding`. The reconcile formula `long.NetPnL + short.NetPnL` therefore silently dropped Bybit's funding leg for *every* position where Bybit was a leg — regardless of long/short. For `nomusdt-...907476`: stored `RealizedPnL = 35.38 = −37.58 (Binance) + 72.97 (Bybit, no funding) + 0`; correct is `35.38 + (−28.45) = 6.93`. Fix: `NetPnL: closedPnl + totalFunding`, with matching comment/doc correction.
  - Backfill: `scripts/backfill_bybit_short_pnl.py` corrects existing `arb:history` entries in place for **both** bugs — flips `short_close_pnl` sign + recomputes `basis_gain_loss` for Bybit-short entries, and adjusts `realized_pnl` by `+bybit_leg_funding` for any reconciled entry where Bybit is a leg (either side). Dry-run by default; pass `--apply` to write. On current prod state: 3 Bybit-short + 6 Bybit-long reconciled entries affected for the `realized_pnl` adjustment; 3 Bybit-short entries additionally need the sign flip.

## [0.32.18] - 2026-04-17

### Changed
- **Binance-inspired design system (Phase 1 — foundation)** — establishes the visual/typographic contract documented in `DESIGN.md` (new, 500+ lines) and installs it via Tailwind v4 `@theme` tokens in `web/src/index.css`. The dashboard keeps its dark-first trading-terminal posture but rebrands to Binance's two-tone foundation: Binance Yellow (`#F0B90B`) as the singular accent for primary actions, Binance Dark (`#222126`) panels on deep-ink (`#0b0e11`) canvas, Crypto Green (`#0ECB81`) / Crypto Red (`#F6465D`) preserved for PnL semantics.
  - **`web/src/index.css`** (+323 lines): remapped Tailwind `gray` / `slate` / `zinc` / `neutral` scales to the Binance neutral palette via `@theme`, so existing utility classes (`bg-gray-900`, `text-gray-400`, etc.) automatically pick up the new brand without page-level rewrites. Also installs `--font-sans` / `--font-mono` stacks preferring `BinancePlex` with data-dense system fallbacks (Inter, JetBrains Mono).
  - **`web/src/components/Sidebar.tsx`** (+56/−30): yellow diamond brand mark, active-nav indicator rail (3px yellow bar on the left edge), pill-shaped locale switcher, uppercase-tracked nav labels. Connected/disconnected status dot shifts from generic green/red to Crypto Green / Crypto Red.
  - **`web/src/pages/Login.tsx`** (+61/−22): ambient gold radial glows on the dark canvas, brand-mark diamond + wordmark lockup, Binance-Dark card with `#f0b90b` focus ring on inputs.
  - **`web/src/App.tsx`** (+52/−28): mobile top-bar (`< md` only) with hamburger, brand diamond, and page title in the deep-ink surface; integrates with the Sidebar's drawer.
  - **`web/src/components/StatusBadge.tsx`** (+21/−7) + **`TimeRangeSelector.tsx`** (+7/−3): restyled to the new accent + neutral system.
- This is a **foundation-only** pass — remaining pages (Overview, Config, Analytics, Exchanges, Risk, Safety, Allocation, Spot Positions, Transfers, Rejections, Logs, Permissions, Trade History) still render through auto-remapped token colors but haven't been touched component-by-component. Their look is noticeably improved "for free" via the `@theme` scale remap, but follow-up passes are expected.
- No behavior changes, no new npm dependencies (respecting the axios lockdown), no backend changes.

## [0.32.17] - 2026-04-16

### Changed
- **Mobile UX overhaul for Opportunities / Positions / History pages** — all three used single wide tables with `overflow-x-auto` as the sole mobile strategy. Quantified audit at 390×844 (iPhone 14 Pro): Opportunities table 634px (1.6× viewport), Positions 795px (2.0×), History 1317px (3.4×). On Positions specifically, only 3 of 12 columns were visible — the close button, PnL, and next-funding countdown all off-screen, making the page functionally unusable on a phone. Replaced with dual-layout pattern via Tailwind `md:` breakpoint (≥768px keeps the existing table unchanged; <768px renders a card list):
  - **`web/src/pages/Positions.tsx`** (+260 lines): per-position card with symbol+rotation+SL badge header, long/short leg rows (exchange+size@price), 2×2 metric grid (Entry, Current, Funding Collected, Rot PnL), meta row (Age, Next Fund, fees), full-width Block/Close action buttons. Expand reveals Unrealized PnL per leg and grouped Funding History (per-day `<details>` with per-event rows).
  - **`web/src/pages/History.tsx`** (+171 lines): per-trade card with symbol+status badge+PnL (color-coded, prominent top-right), long→short exchanges+duration, 3-col grid (Entry Spread, Funding Collected, Rot PnL), red border+failure-reason preview on failed trades. Expand reveals full timestamps, entry/exit prices per leg, exit reason, and PnLBreakdown table.
  - **`web/src/pages/Opportunities.tsx`** (+491 lines): new `SpotCard` component + mobile card lists on both Perp tab and all Spot sections (compact split-view and full single-source). Perp cards show rank+symbol+spread, long/short exchanges with rates, interval/next-fund/OI meta row, and full-width Block+Open actions. Spot cards show direction badge, net APR prominent, 4-col metric grid (funding/borrow/fees/MR), Gap+Borrow lazy-check results inline.
- No desktop (≥md) visual changes — existing tables render identically. Mobile cards and desktop tables share the same state (expand, pagination, filter), rendered from the same source data.
- Verified at 390×844 (iPhone 14 Pro) via Playwright: all three pages now report `documentOverflow=false` (was 1.6–3.4× viewport before). Expand interactions confirmed working (PnLBreakdown, funding history).

## [0.32.16] - 2026-04-16

### Fixed
- **Cross-engine entry gate: prevent PP/SF from opening on same (exchange, symbol)** — v0.32.13–15 added post-execution `sfSubtract` at 20 reconciliation sites, but all are *same-side only* and *after PlaceOrder*. Neither engine's entry path consulted the other. Incident: SF dir-A held LONG 648.1 BARDUSDT on Bybit; PP discovery scored a short-Bybit opportunity and opened SHORT 991. Bybit's one-way mode (forced at startup via `EnsureOneWayMode()` on all 6 exchanges) netted `+648.1 − 991 = −342.9`, silently consuming SF's futures hedge. Consolidator's side-keyed `sfSubtract` returned 0 for the short-side query, synced PP to 342.9, and trimmed OKX long from 991→343. PP became internally consistent, but SF's record (648.1 long) became a time-bomb — its next `ClosePosition` would sell 648.1 into the −342.9 net, driving Bybit to −991 and re-corrupting PP. Three coordinated fixes:
  - **Fix A — Bidirectional entry gates** (`engine.go:2118-2128,2146-2153,2297-2304` + `spotengine/risk_gate.go:39-49,214-228`): PP's `executeArbitrage` now builds a `spotFuturesOccupied` map from `buildSpotFuturesMaps()` (stripping the side suffix), and `ppCrossEngineBlocked()` rejects any opportunity whose long or short exchange+symbol overlaps an active SF position. SF's `checkRiskGate` now queries `GetActivePositions()` (PP) and `spotEntryBlockedByPerp()` refuses entry when PP holds a leg on the target exchange+symbol. Both gates are hard blocks — no size offset, no cooldown needed (Redis reads are ~1ms, filter naturally clears when the other engine exits).
  - **Fix B — SF futures-size reality check** (`spotengine/monitor.go:113-124,197-280`): new `reconcileHedge()` piggy-backs the per-position monitor loop (~60s cadence). Calls `GetPosition(symbol)` on the exchange adapter, matches against `pos.FuturesSide`, and flags divergence when: (a) exchange reports opposite side, (b) exchange reports zero/flat, or (c) `|exchange − recorded| > max(1%×FuturesSize, stepSize)`. On detection: `MarkHedgeBroken()` persists `hedge_broken=true` via `lockedUpdatePosition`, logs ERROR, fires Telegram alert (`NotifySpotHedgeBroken`). New `HedgeBroken bool` field on `SpotFuturesPosition` (`models/spot_position.go`) — JSON-persisted with `omitempty` (absent = false = intact). In-memory mirror `HedgeIntact` (`json:"-"`) synced via `SyncHedgeState()` called on every DB read/write.
  - **Fix C — SF close-path guard** (`spotengine/exit_manager.go:429-435`, `spotengine/execution.go:1274-1280`, `spotengine/monitor.go:73-76,99-103,184-189`): `initiateExit` checks `HedgeBroken` before setting `StatusExiting`, preventing the stuck-exiting trap (review finding: status was set before `ClosePosition` abort). The stuck-exit retry path skips retrying when hedge is broken. Delist exits log ERROR + Telegram alert instead of issuing a doomed close. `ClosePosition` and the yield-exit dispatch retain their guards as defense-in-depth. Manual intervention required to recover.

## [0.32.15] - 2026-04-15

### Fixed
- **sfSizeOffset missed SF positions in SpotStatusExiting window** (codex review of v0.32.14, dispatch task `cbcc7ff8`) — `buildSpotFuturesMaps()` only counted `SpotStatusActive` positions toward the size offset, inherited from codex's v0.32.13 review ("pending/exiting may have flat or stale FuturesSize"). But SF flips `pos.Status = SpotStatusExiting` at `spotengine/exit_manager.go:446` BEFORE calling `ClosePosition(pos)` at `:467`, so during the exit execution window (seconds to minutes) the futures leg is still open on the exchange but excluded from the offset. Every v0.32.14 patched PP site then gets `sfSubtract = size - 0` and mishandles SF's still-live hedge — consolidator could close it, verifier could import its size into PP's record, entry fallback could absorb it as PP size. The two codex reviews gave contradictory advice: v0.32.13 (conservative — exclude exiting) opened a window the v0.32.14 findings aimed to close. Fix: remove the `SpotStatusActive` gate in `buildSpotFuturesMaps()` (`internal/engine/consolidate.go:283`) — include all non-closed SF positions (pending/active/exiting). `sfSubtract`'s zero-clamp handles the rare stuck-exit edge case (FuturesSize > exchange remainder) safely. Pending positions have `FuturesSize=0` so contribute harmlessly. This is a release blocker for v0.32.14 — without it, the 7 HIGH fixes have a timing gap that hits on every SF exit.

## [0.32.14] - 2026-04-15

### Fixed
- **Cross-engine interference at 7 HIGH sites (audit 260415-34e follow-up)** — v0.32.13 fixed 2 cross-engine interference bugs; the audit found 21 more (7 HIGH, 9 MEDIUM, 5 LOW). This release applies the same `sfSizeOffset` subtraction pattern at all 7 HIGH sites so the perp-perp engine never confuses a spot-futures futures leg for its own position on the same `(exchange, symbol, side)`:
  - **Shared helper** (`consolidate.go:252-307`): extracted `buildSpotFuturesMaps()` and `sfSubtract()` from the v0.32.13 inline code. `buildSpotFuturesMaps` returns both the orphan-exclusion set (all non-closed SF positions) and the size-offset map (SpotStatusActive only); `sfSubtract` performs the offset lookup with a zero-clamp.
  - **Finding 1** — SL method-2 verify (`engine.go:1577-1588`): an SF reduce-only fill on an exchange where PP also holds the same side could false-trigger `triggerEmergencyClose` on a just-closed PP position, causing duplicate bookkeeping. Now subtracts SF offset before the `remaining > 0` skip check.
  - **Finding 2** — orphan-cleanup verify (`engine.go:1826-1838`): `handleOrphanClose` post-close verification sees SF leg and reverts PP to Active with `LongSize/ShortSize = SF.size`. Next consolidator cycle would try to close SF's leg. Now subtracts SF offset before the dust check.
  - **Findings 3+4** — `markPositionClosed` close-leg + verify (`consolidate.go:511-587`): without the subtraction, (a) the close command's quantity includes SF's futures leg and could flatten SF's hedge, and (b) post-close verification sees SF size and leaves PP stuck in a ghost Active state forever. Now subtracts SF offset before both the close call and the dust check.
  - **Finding 5** — `reconcilePartialPosition` (`consolidate.go:898-904`): a `StatusPartial` PP position on a symbol where SF also holds a leg would promote to Active with SF's size imported as PP's `LongSize`/`ShortSize`, and the trim path would close SF's hedge. Now subtracts SF offset before promote/trim logic.
  - **Finding 6** — `closePositionWithMode` verify (`exit.go:1893-1909`): direct twin of the fixed rotation verify at `exit.go:2775`. Without the subtraction, a closed PP position would be reverted to Active with SF's size and stale SLs reattached against a phantom position. Now subtracts SF offset before the `notFlat` check.
  - **Finding 7** — entry `confirmFillSafe` fallback at 4 depth-fill paths (`engine.go:3624-3630`, `:3688-3693`, `:3855-3861`, `:3912-3918`): when confirmFill fails and the code falls back to reading exchange position size, any pre-existing SF leg on the same side was being written directly as PP's `LongSize`/`ShortSize`. Now subtracts SF offset so PP only records its own fill.

No source additions beyond the 7 fix sites + 2 helper functions. `go build` clean, `go vet` clean.

## [0.32.13] - 2026-04-14

### Fixed
- **Consolidator cross-engine size mismatch** — the perp-perp consolidator's size-mismatch check (`consolidate.go:280-281`) read the total exchange position for a symbol, which includes spot-futures futures legs. When both engines hold the same symbol on the same exchange and side, the consolidator flagged a false mismatch (e.g. BARDUSDT: perp-perp local=919, exchange=1225.6 including 306.6 from spot-futures dir A). Fix: build a `sfSizeOffset` map from active spot-futures positions and subtract from exchange totals before comparing. Only `SpotStatusActive` positions contribute to the offset — pending/exiting positions with stale `FuturesSize` are excluded (per Codex review).
- **Rotation step-size rounding causes unhedged position** — when rotating a long leg from exchange A to B, the rotation handler formatted the size for the new exchange (`formatSize(B, sym, 349)` → 300 due to step size), opened 300 on B, closed 300 on A, then the post-close verification found 49 remaining on A (349−300) and treated it as a failure. The abort path (a) overwrote `LongSize=49` in the position record, (b) rolled back the new B leg, but (c) did NOT re-open the 300 already closed on A — leaving the position 49 long vs 349 short (86% imbalanced). Incident: ARIAUSDT `ariausdt-1776041102139` on 2026-04-14 20:40 UTC, escalated to SL trigger at 23:48 UTC. Three fixes:
  - **Pre-check** (`exit.go:2648-2656`): if `formatSize` rounds more than 1% down, skip the rotation with a warning instead of proceeding to a partial rotation that will fail verification.
  - **Re-open on abort** (`exit.go:2819-2860`): when the "NOT flat" check fires, the handler now (1) closes the new leg (same as before), (2) re-opens the old leg via IOC order for `actualClosed = closeQty - remainingOnExch` (only the exposure actually lost, per Codex review), (3) does NOT overwrite position sizes — lets the consolidator reconcile.
  - **Diagnostic improvement**: distinguishes expected remainder (step-size) from unexpected remainder (real failure) in the log message.

## [0.32.12] - 2026-04-13

### Fixed
- **Stuck-active dust position chain (NATGASUSDT incident)** — Position `natgasusdt-1775865302363` sat Active for 3 days with `LongSize=ShortSize=7.1e-15` floating-point dust; two manual closes from dashboard failed; consolidator skipped recovery every 5 min. Six coordinated fixes after 10 rounds of Codex review (+ i5 fresh-eyes ALL PASS):
  - **Fix A — depth-exit dust snap** (`internal/engine/exit.go:899-915`): finalizer now snaps `longRemainder`/`shortRemainder` to 0 via new `isDust` helper (`internal/engine/dust.go`) using `max(stepSize, minSize)` as threshold, with `formatSize=0` fallback and `1e-10` floor when contract metadata is unavailable. `closedLong`/`closedShort` are NOT snapped (preserves VWAP/PnL math). Mirrors depth-loop done-criteria at `exit.go:402-417, 515-523`.
  - **Fix B — PnL sanity fallback** (`internal/engine/exit.go:920-940` + `:1898-1914`): when residual sizes drive notional to 0, fall back to stored `EntryNotional` (`models/position.go:48`) or skip the gate entirely. Logs WARN. Previous code zeroed real PnL on dust retries because `notional <= 0` made `Abs(pnl) > 0*2` falsely true.
  - **Fix C — consolidator UpdatedAt bypass** (`internal/engine/consolidate.go:317-336`): drop the 60s "recently updated" guard for the exchange-flat branch. `UpdatePositionFields` auto-bumps `UpdatedAt` on every successful mutate, and risk-monitor / funding tracker / exit-check write to active positions every few minutes for non-size reasons (CurrentSpread, NextFunding, ReversalCount, UnrealizedPnL), so the guard was permanently tripped — see NATGAS log `exchange-flat but recently updated (10s ago), skipping` repeating every 5 min for 3 days. Safety now provided by Fix F's per-position close lock + exchange-flat re-verify in `markPositionClosed`.
  - **Fix D — NextFunding stale on zero-rate** (`internal/engine/engine.go:1995-2024`): `NextFunding` advancement was nested inside `if fundingChanged`, which is `false` whenever both legs report rate=0. TSLAUSDT showed `next_funding=08:00:00Z` 30+ min after settlement passed during quiet hours. Lifted advancement out of the inner block; `FundingCollected` write stays gated to avoid overwriting with stale 0. Edge case: when both `fundingChanged` and `uplChanged` are false, the outer block is skipped — accepted as rare and tracked as follow-up.
  - **Fix E — `markPositionClosed` verify gate dust semantics** (`internal/engine/consolidate.go:495-507`): replace raw `> 0` check with `isDust()` so exchange-side rounding (e.g. `1e-7` in close response) doesn't block recovery. Reuses Fix A helper for consistent semantics.
  - **Fix F — per-position close lock + status claim + reread-with-fallback** (`internal/engine/consolidate.go:425-696` + `internal/engine/exit.go:1751-2002`): three-layer serialization to prevent double bookkeeping (history, stats, releasePerpPosition, reconcilePnL) when `markPositionClosed` (consolidator) races `closePositionWithMode` (L4 reduce → emergency close, delist, SL preemption). Outer: `AcquireOwnedLock("close:"+posID, 30s)` — Redis SET NX + Lua release + auto-renew (`internal/database/locks.go`). Middle: `UpdatePositionFields` predicate as inner stale-copy guard — `markPositionClosed` accepts only `Active|Exiting`, `closePositionWithMode` rejects only `Closing|Closed` (so preemption from `Pending|Partial` per `checkDelistPositions` filter still works). Bookkeeping: re-read persisted truth with `retry(3, 50/100/150ms)` + fallback to local pos on Redis transient failure (preserves all existing post-close side effects exactly once). Phase-2 close write strictly mirrors `exit.go:1902-1918`: only `LongExit`/`ShortExit` (>0 guards), `RealizedPnL`, `Status`. `ExitFees`/`LongClosePnL`/`ShortClosePnL` are NOT written here — they belong to `reconcilePnL`; pre-writing trips `InferHasReconciled` (`models/position.go:73-83`) and skips the real reconcile pass. `CancelAllOrders` kept BEFORE phase-2 save to prevent canceling orders on a re-used symbol after close.

## [0.32.11] - 2026-04-13

### Fixed
- **Unified donor rebalance ignored `maxTransferOut`** — For unified-account donors (Bybit UTA), the allocator's capacity calculation and pre-withdraw check used raw futures balance instead of the exchange-reported withdrawable cap. Bybit's `/v5/account/withdrawal` returns `availableWithdrawal` reflecting risk-delay freezes and position collateral that raw balance misses, so the allocator could request more than Bybit allows, triggering `code=131001 insufficient` even after v0.32.10's `accountType=UTA` fix. Production evidence 2026-04-12 15:46 UTC: `maxTransferOut=33.40` but allocator tried `60.02`. Added `maxTransferOut` clamps at `internal/engine/allocator.go:684-697` (scan-time capacity) and `:1610-1624` (pre-withdraw `effectiveAvail`), mirroring the existing split-account logic at `:1557-1560`. Gate.io unified unaffected (adapter does not populate `MaxTransferOut`).

## [0.32.10] - 2026-04-12

### Fixed
- **Bybit withdraw missing required `accountType`** — `pkg/exchange/bybit/adapter.go` `Withdraw()` did not send the `accountType` field that `POST /v5/asset/withdraw/create` lists as required (`doc/EXCHANGEAPI_BYBIT.md:1318`). Every automated rebalance donor withdrawal from Bybit was therefore failing with `code=131001 Account available balance insufficient` (7+ failures observed in VPS journalctl between 2026-04-09 and 2026-04-12; zero successful `bybit [rebalance] -> X` entries in Redis `arb:transfers`). Added `accountType: "UTA"` so Bybit internally transfers the required amount UNIFIED → Funding and withdraws in one call, matching the rebalance flow where donor funds live in UNIFIED. Prior BingX fix in v0.32.4 addressed the same pattern.

## [0.32.9] - 2026-04-12

### Fixed
- **Rebalance allocator override guard** — `rebalanceFunds()` previously stored allocator overrides unconditionally after calling a void `executeRebalanceFundingPlan()`, so overrides could survive even when the executor never actually moved funds. Next entry scan then attempted trades on unfunded exchanges and got rejected for L4 margin breach (2026-04-12 03:45–:55 incident: SIRENUSDT post-trade margin 0.94 on bitget after rebalance failed to transfer). The executor now returns `rebalanceExecutionResult{PostBalances, Unfunded, SkipReasons}`; the caller filters `allocSel.choices` via new `keepFundedChoices` helper (deterministic replay against post-execution balances) before storing overrides. Donor bookkeeping is now account-type aware (unified vs split), with rollback/refetch/pessimistic-zero fallback on batched-withdraw failure, merged-batch fee reconciliation on success, and `futuresTotal` tracked across all relief paths. 7 new regression tests in `allocator_override_test.go`. Verified via 10 Codex review passes.
- **Sequential rebalance relief missing `futuresTotal` update** — Same-exchange spot→futures relief in the sequential fallback path (`engine.go:982-989`) omitted `bi.futuresTotal += actualTransfer` while the sufficient-futures branch and allocator path both updated all three fields. Post-relief L4 ratio calculations on that branch used a stale `futuresTotal`.

## [0.32.8] - 2026-04-12

### Added
- **Override fallback when discovery returns 0** — When entry scan discovery returns 0 opportunities but allocator overrides from the :45 rotate scan exist (funds already transferred cross-exchange), the entry handler now re-scans those specific symbols through the full scanner pipeline (poll fresh rates → rank → verify → filters) to salvage the transferred capital. Previously the overrides were silently discarded at the next rebalance cycle, wasting the transfer fees. New `Scanner.RescanSymbols()` method and `models.SymbolRescanner` interface.

### Refactored
- Extracted entry filter chain (persistence, volatility, cooldown, interval, funding window, backtest, delist) into `Scanner.applyEntryFilters()` for reuse by both `runCycleInternal` and `RescanSymbols`.

## [0.32.7] - 2026-04-11

### Fixed
- **Spot-relief continue skipping L4 check** — When an exchange had spot balance > 0 (even dust like 0.001 USDT), the rebalance spot→futures relief branch unconditionally `continue`d past the post-trade L4 margin ratio check. Exchanges like OKX with tiny spot dust never got a crossDeficit entry even when post-trade ratio would exceed L4, causing wasted transfers to the other leg with no entry possible. Fixed in both pool allocator (allocator.go) and sequential rebalance (engine.go) paths. Also fixed futuresTotal not being updated after spot→futures transfers.

## [0.32.6] - 2026-04-11

### Fixed
- **Step size sync — coarse-first ordering + top-up alignment** — When two exchanges have vastly different step sizes (e.g., BingX 0.01 vs Gate.io 100), partial fills on the fine-step exchange produce amounts the coarse exchange cannot match (ARIAUSDT imbalance bug: long=176.18 vs short=100). Entry now overrides leg ordering so coarse-step exchange goes first. Second leg uses commonTradeableSize to verify alignment; if misaligned, tops up first leg to next common step instead of rolling back (saves fees). Exit keeps risk-leg-first ordering with same top-up logic; falls back to market close only if top-up fails.
- **Merge same-direction rebalance withdrawals** — When the rebalance loop picks the same donor for the same recipient multiple times (e.g., bingx→bitget 15.19 + 5.00), each withdrawal incurred a separate on-chain fee. Now accumulates withdrawals per donor→recipient pair and executes merged withdrawals after the loop, saving one fee per merged pair.

### Added
- `RoundUpToStep()` utility function for top-up step alignment calculations.

## [0.32.5] - 2026-04-10

### Fixed
- **Allocator appendChoice missing backtest validation** — Alternative exchange pairs were accepted without running CheckPairFilters (backtest/persistence/volatility). Could select unprofitable pairs, plan useless cross-exchange transfers. Now validates alt pairs before adding to candidates.
- **Tier-3 entry blocked by stale overrides** — When allocator overrides failed at entry, tier-3 fallback was blocked ("avoid unfunded entries"). Profitable opps like ARIAUSDT/SIRENUSDT were skipped. Now falls through to tier-3; risk.Approve still gates margins.
- **Rotation missing pair filter validation** — checkRotations could rotate into pairs that fail backtest/persistence. Now calls CheckPairFilters on rotation target before rotateLeg.

## [0.32.4] - 2026-04-09

### Fixed
- **BingX withdraw missing walletType** — Withdraw API requires walletType parameter (1=Fund, 2=Standard, 3=Perpetual). TransferToSpot lands funds in Fund account but Withdraw was called without walletType, causing "Insufficient balance" error. Added walletType=1.

## [0.32.3] - 2026-04-09

### Fixed
- **Gate.io unified balance double-count** — GetSpotBalance for unified accounts returned overlapping value from /spot/accounts (same money as /unified/accounts). Now returns zero for unified mode. Cross-exchange withdrawal donor and deposit polling use GetFuturesBalance for unified accounts.
- **Exit check at :40 removed** — v0.32.0 added exit checks in EntryScan handler, which defeated SpreadReversalTolerance=1 (tolerance requires full scan cycles, not 10-min gaps). Exits now only run at :30 as designed.

## [0.32.1] - 2026-04-09

### Debug
- **History/reconcile debug logging** — Added comprehensive debug logs to trace per-leg field writing through the entire flow: tryReconcilePnL aggregate values, needsBreakdownUpdate comparison, UpdatePositionFields mutator, GetPosition re-read, UpdateHistoryEntry, and AddToHistory. Also logs JSON content checks for `long_total_fees` and `has_reconciled` presence.

## [0.32.0] - 2026-04-09

### Fixed (Critical)
- **Rollback/trim one-sided exposure** — In-loop rollback and trim branches now correctly account for surviving exposure via `abortFillLoop` flag, ensuring VWAP stays in sync before breaking the fill loop. Telegram alerts for orphan exposure events.
- **rotateLeg DB swap silent failure** — Re-reads position after `UpdatePositionFields` to verify swap applied. On failure, writes `StatusPartial` with actual leg state (CAS-guarded against concurrent close). Broadcasts partial to dashboard.
- **Rotation not cancellable by L4/L5** — `rotateLeg` now registers full exit lifecycle (`exitCancels`, `exitDone`, context). Three cancel checkpoints at each critical stage with safe cleanup. New leg protected from consolidator orphan scan via `entryActive`.

### Fixed (High)
- **First-leg confirm zeroing real fill** — Detects actual exchange position via `getExchangePositionSize` when `confirmFillSafe` fails. Updates existing pending position to `StatusPartial` instead of archiving as zero.
- **Leverage clamp inconsistency** — Entry now uses `effectiveLev = min(cfg.Leverage, MaxLeverage())` consistently for SetLeverage calls and margin calculations.
- **Micro-order sizing short-only validation** — Looks up contract specs for both exchanges. New `commonTradeableSize` helper iteratively converges to a size both exchanges can represent.
- **entryActive rotation target check** — `checkRotations` now checks `entryActive` on the candidate replacement exchange before proceeding.
- **ManualClose strands StatusExiting** — `spawnExitGoroutine` returns `bool` and is sole authority for `active→exiting` CAS. ManualClose no longer pre-sets status.
- **Depth exit allocator leak** — Added `releasePerpPosition` call in depth-exit fully-flat branch.
- **FormatSize hardcoded 6 decimals** — L4 reduce and rotation open now use `e.formatSize(exchName, symbol, size)` instead of `utils.FormatSize(size, 6)`.
- **PnL double-count on rotate-back** — `tryReconcilePnL` queries from last `RotationHistory.Timestamp` instead of `CreatedAt`.
- **Reconcile retry missing 30s** — Added third retry at 30s to the delays slice.

### Optimizations
- **Depth stream pre-warming** — Subscribes depth WS at :35 for top candidates. Ref-counted subscriptions with 5s freshness check. Entry at :40 skips 3-8s wait when data is fresh.
- **Parallel market fallback close** — Both legs closed concurrently via goroutines in market fallback, reducing naked exposure time.
- **Risk-leg-first depth exit** — Closes the leg with thinner depth first instead of always long-first.
- **Parallel SetLeverage/SetMarginMode** — Both exchanges' setup calls run concurrently.
- **Exit check at :40** — `checkIntervalChanges` + `checkExitsV2` now run before `executeArbitrage` on EntryScan, reducing worst-case exit detection latency.
- **Per-position PnL lock** — Replaced global `pnlReconcileMu` with per-position locks via `sync.Map`. 2s sleep moved outside lock. Both `reconcilePnL` and `reconcileRotationPnL` updated.
- **Approval cache ActivePositions** — `PrefetchCache` now includes `ActivePositions` with incremental delta-update during batch approval.
- **SCAN replaces KEYS** — All 4 `Keys()` calls in allocator.go replaced with iterative `SCAN` (cursor-based, batch 100).
- **Exit priority ordering** — `checkExitsV2` now sorts by worst `CurrentSpread` first, largest notional as tiebreaker.

### Improved (Medium)
- **$10 per-leg floor in pre-trade** — `approveInternal` checks per-leg notional using per-exchange mid prices. Post-loop check also uses per-leg fill VWAPs.
- **Per-leg margin math** — `RiskApproval` extended with `LongMarginNeeded`/`ShortMarginNeeded`. Approval, buffer checks, projected ratio, and reservation all use per-exchange values.
- **Strategy-aware CapitalPerLeg** — `EffectiveCapitalPerLeg` accepts optional strategy param. Per-strategy dynamic caps prevent perp cap from bleeding into spot-futures sizing.

## [0.31.2] - 2026-04-09

### Fixed
- **RotateScan :35 risk bypass** — `rotateLeg()` now calls `risk.ApproveRotation()` with leverage clamp, MarginSafetyMultiplier, projected L4 margin ratio, exchange health scoring, and per-exchange exposure cap. Auto-transfer trigger uses buffered margin instead of bare requirement.
- **False-positive rotation on rate lookup failure** — `computeLiveSpread()` returns `(float64, bool)`; `checkRotations()` skips position when live spread is unavailable instead of treating failure as zero spread.
- **Nil-deref after rotation auto-transfer** — `GetFuturesBalance()` error after spot→futures transfer is now handled (was silently ignored, could panic).
- **Rotation race with exit/SL/consolidator** — `checkRotations()` now skips positions with active exit or entry; `rotateLeg()` claims `exitActive` for the duration of the rotation. L5/delist emergency close intentionally not blocked (known limitation).
- **Allocator pre-feasibility hardcoded floor** — `isAllocatorFundingFeasible()` always looks up exchange `minWithdraw` instead of only when deficit < $10; uses `feeCache` to avoid redundant API calls.

## [0.31.1] - 2026-04-08

### Fixed
- **ExitReason missing in position history** — 5 close paths wrote `AddToHistory(pos)` without setting `pos.ExitReason`, causing empty "平倉原因" on dashboard. Fixed: `spawnExitGoroutine` (depth exit + fallback close), `markPositionClosed` (consolidator), `triggerEmergencyClose` (SL/liquidation), `checkDelistPositions` (Binance delist), `reducePosition` (L4 full-flatten).

## [0.31.0] - 2026-04-08

### Added
- **deliveryDate-based delist detection** — primary delist signal: a 1h-cadence background poller (`internal/discovery/contract_refresh.go`) re-loads contract metadata via `LoadAllContracts` for every configured exchange and writes any near-future `DeliveryDate`-flagged perpetual to the existing `arb:delist:{SYMBOL}` Redis key. Catches batch delists where the article scraper's title regex misses (e.g. the 2026-04-08 OLUSDT/HIPPOUSDT/RLSUSDT/PUFFERUSDT batch announced under a generic title) — Binance updates `deliveryDate` at announcement time per their docs.
- **Bybit `deliveryTime` parsing** — `pkg/exchange/bybit/adapter.go` now extracts `deliveryTime` for `LinearPerpetual` contracts into `ContractInfo.DeliveryDate`, enabling Bybit delist detection symmetrically with Binance.
- **`ContractInfo.DeliveryDate` field** — new `time.Time` field on `pkg/exchange/types.go ContractInfo`, populated by Binance and Bybit adapters for true perpetuals being delisted (excludes the year-2100 sentinel and dated quarterlies).
- **Spot-futures delist parity** — spot-futures risk gate now has a delist check (step 7, before dry-run) and the monitor loop force-exits any active position whose futures symbol lands on the blacklist. Both engines now consume the same `arb:delist:{SYMBOL}` Redis key for a single source of truth.
- **`Database.IsDelisted` helper** — leaf-package read of the delist blacklist (avoids the import cycle that would result from spot-engine depending on `internal/discovery`).
- **`ContractRefreshInterval` config** — new `time.Duration` config field (`contract_refresh_min` JSON, default 60 minutes, 0 disables) controlling the new poller's cadence. Reuses `DelistFilterEnabled` as the on/off toggle.
- **Targeted regression test** — `TestLoadAllContracts_Binance_DeliveryDateParsing` pins live (zero), delisting (populated), and quarterly (zero) cases for the new `DeliveryDate` field.

### Fixed
- **Article-scraper-only delist detection blind spot** — root cause of the 2026-04-08 RLSUSDT loss. The scraper at `internal/discovery/delist.go` only parsed announcement titles; the batch delist used a generic title with no symbol tokens, so neither regex pattern fired and the bot entered RLSUSDT after the announcement. The new `deliveryDate` poller is title-format-independent and reads the same field from the same endpoint Binance updates at announcement time.
- **Binance-only auto-exit guard in `checkDelistPositions`** — `internal/engine/engine.go` now triggers emergency close on a delisted position regardless of which leg's exchange is involved. Previously, a delisting symbol with no Binance leg was logged and skipped, leaving the position to settle unfavorably.
- **Test stub compilation failures** — `internal/discovery/test_helpers_test.go` and `internal/api/funding_history_test.go` were failing to compile since v0.29.2 because their stub `Exchange` implementations (`stubExchange`, `fundingStubExchange`) were never updated when `CancelAllOrders` was added to the `exchange.Exchange` interface. Added the missing no-op method to both stubs so `go vet ./...` and `go test ./internal/discovery/... ./internal/api/...` compile cleanly. Tests were previously unrunnable in those two packages; now 45 tests pass across both.

### Changed
- **`Scanner.contracts` access is now lock-protected** — added `contractsMu sync.RWMutex` to guard the contracts cache, since the new contract refresh poller introduces concurrent writes alongside the existing reads from `hasContract`. `SetContracts`, `hasContract`, and the new `replaceContractsForExchange` all go through the lock.
- **Article scraper retained as belt-and-suspenders** — `internal/discovery/delist.go` is unchanged. When both signal paths write the same key with the same date, the operation is idempotent.

### Notes
- Out of scope for this release (deferred): Gate.io/Bitget/BingX/OKX `deliveryDate` equivalents (no equivalent fields on their contract endpoints per the doc survey), symbol-disappearance detection, and a dashboard surface for `deliveryDate`-blacklisted symbols.
- Merged in parallel with v0.30.0 (PnL reconcile safety + entry scan hardening). Both feature sets are part of this release.

## [0.30.0] - 2026-04-08

### Fixed
- **CRITICAL: Consolidator force-close writes partial PnL as reconciled** — added PartialReconcile field; InferHasReconciled skips inference for partial data; async reconcilePnL retries after force-close; AdjustWinLoss corrects win/loss counts on PnL sign change
- **CRITICAL: Trim-back failure activates position with unmatched exposure** — sentinel errPartialEntry pattern; force checkpoint before trim; post-trim sizes tracked accurately; caller skips cleanup on partial success
- **HIGH: Sequential rebalance passes deficits as needs** — promoted rebalanceDeficit to package-level type; added precomputedDeficits parameter to skip upper-half recalculation while preserving donor surplus math
- **HIGH: StatusPartial positions stranded after crash** — consolidator now reconciles StatusPartial: query exchange, trim to matched, promote or close; markPartialClosed zeros sizes
- **HIGH: Post-fill SavePosition failure leaves orphan fills** — 3-retry save; falls back to errPartialEntry with valid StatusPartial checkpoint in DB
- **HIGH: confirmFill treats unknown as zero-fill** — confirmFillSafe wrapper distinguishes REST failure from confirmed zero; first-leg unknown freezes depth loop; second-leg unknown saves partial with entry prices
- **HIGH: Allocator override stale falls through to unfunded tier-3** — applyAllocatorOverrides returns (filtered, hadOverrides); tier-3 blocked when allocator ran but all overrides stale
- **MEDIUM: No-diff reconciliation doesn't update history entry** — UpdateHistoryEntry (not AddToHistory) in no-diff branch; clears PartialReconcile
- **MEDIUM: Allocator commit failure not handled** — triggers allocator.Reconcile() on persistent commit failure
- **LOW: PnLBreakdown rotation_pnl not in hasDecomposition gate** — rotation-only positions now render breakdown instead of "data unavailable"

### Added
- `PartialReconcile` field on ArbitragePosition (partial data marker)
- `AdjustWinLoss()` in database/state.go (pipelined win/loss count correction)
- `confirmFillSafe()` entry-only wrapper with error awareness
- `reconcilePartialPosition()` + `markPartialClosed()` in consolidator
- `rebalanceDeficit` package-level type in allocator
- `errPartialEntry` sentinel error for partial entry success
- Aggregator test coverage for HasReconciled flag (3 new test cases)
- `partial_reconcile` field in frontend Position type

### Changed
- `executeRebalanceFundingPlan` accepts optional precomputedDeficits parameter
- `applyAllocatorOverrides` returns ([]Opportunity, bool) tuple
- `GetWithdrawFee` exchange interface returns (fee, minWd, error) — all adapters updated

### Removed
- `buildOppsFromAllocatorChoices` dead code function

## [0.29.3] - 2026-04-07

### Fixed
- **Allocator re-validation TransferablePerExchange bug** — revalCache now includes TransferablePerExchange so transfer-dependent candidates are no longer rejected by re-validation before reaching simulateTransferPlan
- **Sizing/margin check inconsistency** — calculateSizeWithPrice and CalculateSize now divide maxFromBalance by MarginSafetyMultiplier, preventing sizing from creating positions that the subsequent margin buffer check always rejects
- **DryRun spot balance for split accounts** — approveInternal dryRun unconditionally adds spot balance for non-unified exchanges, matching rebalanceAvailable semantics; unified accounts (gateio) correctly skip spot addition

### Added
- **Comprehensive debug logging for :35 rebalance path** — 57+ Debug logs across approveInternal (19 rejection points), allocator (appendChoice success/reject, deficit breakdown, capacity map, totalDonorSurplus, greedy/B&B selection, budget exceeded, cheapestTransferFee, simulateTransferPlan), and sizing strategy
- **Allocator solver comparison log** — greedy vs B&B result comparison (improved=true/false)
- **Re-validation reserved accumulation log** — shows reserved map when re-validation rejects candidates

### Removed
- **50% deviation guard** — removed spread deviation filter from allocator alternatives that blocked valid exchange pairs when primary pair had unusually high spread

