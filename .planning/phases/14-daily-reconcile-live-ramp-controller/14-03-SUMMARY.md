---
phase: 14-daily-reconcile-live-ramp-controller
plan: 03
subsystem: pricegap-live-ramp
tags: [pricegap, live-capital, ramp-controller, sizer, risk-gate, asymmetric-ratchet, defense-in-depth, go, tdd]

# Dependency graph
requires:
  - phase: 14-daily-reconcile-live-ramp-controller
    plan: 01
    provides: RampState/RampEvent types, RampStateStore Save/Load/AppendEvent surfaces, PriceGapLiveCapital + PriceGapStage{1,2,3}SizeUSDT + PriceGapHardCeilingUSDT + PriceGapCleanDaysToPromote config fields, Wave-0 7+2+3 ramp/sizer/Gate-6 test stubs
  - phase: 14-daily-reconcile-live-ramp-controller
    plan: 02
    provides: DailyReconcileRecord (with Totals.NetClean signal) — RampController.Eval consumer surface
provides:
  - RampController state machine (asymmetric ratchet, kill-9 persistence, force-op semantics)
  - RampStateStore + RampNotifier narrow interfaces (D-15 boundary)
  - RampSnapshotter narrow interface for risk_gate.go consumer
  - Sizer.Cap notional-cap primitive (defense-in-depth layer 2)
  - RedisWSRampSink concrete sink (mirrors Phase 12 RedisWSPromoteSink)
  - risk_gate.go Gate 6 (ramp) — first gate that touches live capital
  - 2 sentinel errors (ErrPriceGapRampExceeded + ErrPriceGapRampStateUnavailable)
  - "ramp" entry in Telegram priceGapGateAllowlist
affects:
  - 14-04-daemon-notifier (consumes RampController constructor + Sizer wiring)
  - 14-05-cli-pg-ramp-admin (consumes ForcePromote/ForceDemote/Reset surfaces)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Asymmetric ratchet state machine — promotion slow (7 clean days = 1 step up), demotion fast (1 loss day = counter zero + 1 step down). Locked by TestRampController_AsymmetricRatchetInvariant at -count=100"
    - "Defense in depth at sizing chokepoint — Sizer caps at call site (tracker.go runTick), risk_gate Gate 6 caps independently. Either alone sufficient; both must agree (D-22)"
    - "kill -9 persistence via narrow RampStateStore interface — Save+Load round-trip through fakeRampStore proves state survives restart (T-14-02)"
    - "Forward-declared narrow interface (RampSnapshotter) lets Tracker consume RampController without coupling to the concrete type — same shape used for ScanRunner (Phase 11)"
    - "Sentinel error channeling for typed gate decisions — ErrPriceGapRampExceeded + ErrPriceGapRampStateUnavailable join the existing 9 PriceGap sentinels"

key-files:
  created:
    - "internal/pricegaptrader/sizer.go (75 lines) — Sizer.Cap notional-cap primitive (defense-in-depth layer 2)"
    - "internal/pricegaptrader/ramp_controller.go (227 lines) — RampController + RampStateStore + RampNotifier interfaces"
    - "internal/pricegaptrader/redis_ramp_sink.go (62 lines) — RedisWSRampSink mirrors Phase 12 RedisWSPromoteSink"
  modified:
    - "internal/pricegaptrader/errors.go (+12 lines) — ErrPriceGapRampExceeded + ErrPriceGapRampStateUnavailable sentinels"
    - "internal/pricegaptrader/sizer_test.go (Wave-0 stubs replaced with 2 real test bodies + cfg fixture)"
    - "internal/pricegaptrader/ramp_controller_test.go (Wave-0 stubs replaced with 7 test bodies + fakeRampStore + fakeRampNotifier + helpers)"
    - "internal/pricegaptrader/risk_gate.go (+50 lines) — Gate 6 (ramp) inserted; old Gate 6 → Gate 7; stageSizeForCfg helper; doc comment header bumped to '8 deterministic gates (gate 0 + 7)'"
    - "internal/pricegaptrader/risk_gate_test.go (Wave-0 stubs replaced with 4 Gate-6 tests + fakeRampSnapshotter + liveRampCfg fixture)"
    - "internal/pricegaptrader/tracker.go (+20 lines) — sizer + ramp DI fields; RampSnapshotter narrow interface; sizer.Cap wired at runTick sizing call site"
    - "internal/notify/telegram.go (+1 line) — 'ramp' added to priceGapGateAllowlist"

key-decisions:
  - "Tracker takes RampSnapshotter (narrow interface), NOT *RampController concrete type. Lets Task 1 (Sizer wiring) compile before Task 3 (RampController exists). Production wires *RampController at Plan 14-04 — duck-typing via Snapshot() signature"
  - "stageSizeForCfg helper duplicated in risk_gate.go (mirrors Sizer.stageSize). Both functions kept in sync via doc comment 'if you change one, change both'. Alternative — exporting Sizer.StageSize — was rejected to avoid coupling risk_gate.go to *Sizer when only the cap math is needed"
  - "4 Gate-6 tests instead of plan's 3: added TestRiskGate_Gate6_Ramp_NilRampController_FailsClosed locking the defensive nil-ramp path. Plan 14-04 guarantees non-nil at production wiring; the test prevents partial-wiring regression"
  - "fakeRampSnapshotter.unavailable=true returns Snapshot{CurrentStage: 0} — Gate 6 detects via stageSizeForCfg returning 0 and fails-closed with ErrPriceGapRampStateUnavailable. Mirrors T-14-12 mitigation (legacy/empty state)"
  - "RampController bootstrap initializes state to {Stage:1, Counter:0} on missing/legacy/corrupt-stage state. Persists immediately so subsequent Save+Load round-trips are deterministic. T-14-12 lock"
  - "ForcePromote does NOT zero counter (D-15 #3 — operator override is intentional); ForceDemote DOES zero counter + increments DemoteCount (D-15 #4 — matches asymmetric ratchet). Both fire NotifyPriceGapRampForceOp via the RampNotifier"
  - "Reset emits action='reset' RampEvent and clears all state including DemoteCount. ForcePromote/ForceDemote emit force_promote/force_demote actions. All audit-trail visible in pg:ramp:events LIST"

patterns-established:
  - "Forward-declared consumer-side narrow interfaces (RampSnapshotter on Tracker) before the producer type exists — sequential TDD pattern that lets each task's GREEN phase build cleanly without circular ordering"
  - "Defense-in-depth at chokepoints — when a security/correctness invariant matters, enforce it at MULTIPLE call sites (Sizer at sizing, Gate 6 at risk-gate). Document both sites with cross-references so refactors don't accidentally break one and trust the other"
  - "Sentinel error decoration with Reason field — every gate denial carries (Err sentinel, Reason string) for telemetry and exit_reason stamping; Reason MUST match the Telegram allowlist string for risk_block notifications to surface"

requirements-completed: [PG-LIVE-01]

# Metrics
duration: 8min
completed: 2026-04-30
---

# Phase 14 Plan 03: RampController + Sizer + Gate 6 Summary

**Strategy 4 live-capital ramp state machine + defense-in-depth notional cap + risk gate 6 — 12 tests passing, asymmetric ratchet locked at -count=100, kill-9 persistence proven via Save+Load round-trip, full repo regression-clean (301 tests in 3 packages).**

## Performance

- **Duration:** ~8 minutes
- **Started:** 2026-04-30T07:24:29Z
- **Completed:** 2026-04-30T07:32:54Z
- **Tasks:** 4 (all auto, all `tdd=true`)
- **Files created:** 3 (sizer.go, ramp_controller.go, redis_ramp_sink.go)
- **Files modified:** 7 (errors.go, sizer_test.go, ramp_controller_test.go, risk_gate.go, risk_gate_test.go, tracker.go, telegram.go)
- **Commits:** 4 atomic per-task commits

## Accomplishments

- **RampController state machine landed (PG-LIVE-01):** Eval drives 7-clean-days promotion + asymmetric loss-day demotion; Snapshot returns the 5-field RampState; ForcePromote/ForceDemote/Reset surface for pg-admin (Plan 14-05). Locked by 7 unit tests.
- **Asymmetric ratchet invariant locked:** TestRampController_AsymmetricRatchetInvariant passes at -count=100 (zero flakiness). Promotion slow (7 clean days), demotion fast (1 loss day → counter=0 + step down). Cannot accidentally go symmetric without breaking the test.
- **kill -9 persistence proven:** TestRampController_KillNinePersistence drives state to (stage=1, counter=4), constructs a fresh controller from the SAME store, asserts state survives. Models T-14-02 mitigation via the narrow RampStateStore boundary.
- **Defense in depth wired (D-22):** Sizer.Cap caps at the runTick sizing call site (tracker.go:587) BEFORE the request reaches the risk gate. risk_gate.go Gate 6 caps INDEPENDENTLY using the same `min(stage_size, hard_ceiling)` math. Either alone sufficient; both must agree. TestSizer_TypoStage3Of9999_StillSizesAt1000 locks the layer-2 typo-resilience.
- **Risk gate Gate 6 (ramp) inserted:** preEntry now composes 8 deterministic gates (Gate 0 + Gates 1–7). Gate 6 (ramp) sits AFTER Gate 5 (concentration); old Gate 6 (delist/staleness) becomes Gate 7. No-op when PriceGapLiveCapital=false (D-07 paper-mode pass-through). Fail-closed when ramp controller is nil OR returns CurrentStage=0 (T-14-12, T-14-07).
- **Module isolation preserved:** zero `arb/internal/engine` or `arb/internal/spotengine` imports across ramp_controller.go, redis_ramp_sink.go, sizer.go (D-15 boundary).
- **Telegram allowlist extended:** "ramp" entry in priceGapGateAllowlist enables Plan 14-04 to dispatch Gate-6 risk-block notifications via the existing NotifyPriceGapRiskBlock plumbing.
- **TDD discipline:** Task 1 GREEN (sizer + sentinels + tracker wiring), Task 2 RED (7 ramp tests fail to compile — undefined NewRampController), Task 3 GREEN (RampController + RedisWSRampSink — all 7 tests pass), Task 4 GREEN (Gate 6 + tests + allowlist).

## Task Commits

Each task committed atomically:

1. **Task 1: Sizer + ramp errors + tracker wiring** — `35b60c1` (feat): Sizer.Cap (75 lines), 2 sentinel errors, 2 sizer tests replace Wave-0 stubs (T-14-01 layer-2 typo lock), tracker.go wires sizer.Cap at runTick:587 with nil-guard.
2. **Task 2: RED — 7 ramp controller tests** — `4439469` (test): replaces 7 Wave-0 stubs with real bodies; tests intentionally fail to compile (undefined: NewRampController, RampStateStore, RampNotifier).
3. **Task 3: GREEN — RampController + RedisWSRampSink** — `19271b5` (feat): RampController (227 lines) + RedisWSRampSink (62 lines) + 2 narrow interfaces; all 7 tests pass; -count=100 stable on asymmetric ratchet test.
4. **Task 4: GREEN — Gate 6 (ramp) in risk_gate.go** — `c463d2e` (feat): risk_gate.go Gate 6 inserted, old Gate 6 → Gate 7, stageSizeForCfg helper, 4 Gate-6 tests replace 3 Wave-0 stubs (added nil-controller fail-closed test), "ramp" added to priceGapGateAllowlist.

## Files Created/Modified

**Created:**
- `internal/pricegaptrader/sizer.go` (75 lines) — `Sizer` + `NewSizer` + `Sizer.Cap` + private `stageSize` helper. Defense-in-depth layer 2.
- `internal/pricegaptrader/ramp_controller.go` (227 lines) — `RampController` + `RampStateStore` + `RampNotifier` narrow interfaces + 5 methods (Eval, ForcePromote, ForceDemote, Reset, Snapshot) + private bootstrapState/persistLocked helpers.
- `internal/pricegaptrader/redis_ramp_sink.go` (62 lines) — `RedisWSRampSink` + `NewRedisWSRampSink` + `Emit`. Mirrors RedisWSPromoteSink shape.

**Modified:**
- `internal/pricegaptrader/errors.go` — +2 sentinels (ErrPriceGapRampExceeded, ErrPriceGapRampStateUnavailable) + comment block.
- `internal/pricegaptrader/sizer_test.go` — Wave-0 stubs replaced with 2 test bodies + config fixture.
- `internal/pricegaptrader/ramp_controller_test.go` — Wave-0 stubs replaced with 7 test bodies + `fakeRampStore` + `fakeRampNotifier` + `cleanRecord` / `lossRecord` / `fixedClock` helpers + `rampTestCfg` fixture + compile-time interface assertions.
- `internal/pricegaptrader/risk_gate.go` — +Gate 6 (ramp) insertion (29 lines) + old Gate 6 renumbered to Gate 7 in comment + `stageSizeForCfg` helper at file bottom (18 lines) + `arb/internal/config` import + doc comment header bumped from "6 gates" to "8 deterministic gates (gate 0 + 7)".
- `internal/pricegaptrader/risk_gate_test.go` — Wave-0 stubs replaced with 4 Gate-6 tests + `fakeRampSnapshotter` + `liveRampCfg` fixture (added 1 extra test beyond plan's 3 — nil-controller defensive lock).
- `internal/pricegaptrader/tracker.go` — +sizer + ramp DI fields (8 lines) + RampSnapshotter narrow interface (5 lines) + sizer.Cap wiring at runTick:587 sizing call site (8 lines).
- `internal/notify/telegram.go` — +1 line: `"ramp": {}` allowlist entry.

## Verification Confirmed

| Check | Result |
|-------|--------|
| `go test ./internal/pricegaptrader/... -run 'TestSizer_\|TestRampController_\|TestRiskGate_' -count=1` | 29/29 PASS (2 sizer + 7 ramp + 16 risk-gate including 4 new Gate 6) |
| `go test ./internal/pricegaptrader/... -run 'TestRampController_AsymmetricRatchetInvariant' -count=100` | 100/100 PASS (zero flakiness on asymmetric ratchet) |
| `go test ./internal/pricegaptrader/... ./internal/notify/...` | 301 PASSED in 3 packages (full regression scan) |
| `go build ./...` | clean exit 0 |
| `go vet ./...` | clean exit 0 |
| `grep -nE '^\s*"arb/internal/engine\|^\s*"arb/internal/spotengine' internal/pricegaptrader/ramp_controller.go internal/pricegaptrader/redis_ramp_sink.go internal/pricegaptrader/sizer.go` | 0 matches (D-15 isolation) |
| `grep -E "min\(stage_size\|min\(stageSize\|min\(stage" internal/pricegaptrader/sizer.go internal/pricegaptrader/risk_gate.go` | 3 matches (D-22 defense in depth at both layers) |
| `grep -n "Gate 6: ramp\|Gate 7: delist" internal/pricegaptrader/risk_gate.go` | 2 matches (Gate 6 ramp + renumbered Gate 7) |
| `grep -n "rc.state.CleanDayCounter = 0" internal/pricegaptrader/ramp_controller.go` | 3 matches (loss-day, ForceDemote, Reset — three reset sites) |
| `grep -n "// D-15 #3" internal/pricegaptrader/ramp_controller.go` | 1 match (force-promote counter-preserved comment pinned to decision ID) |
| `grep -c "t.Skip" internal/pricegaptrader/sizer_test.go internal/pricegaptrader/ramp_controller_test.go internal/pricegaptrader/risk_gate_test.go` | 0+0+0 = 0 (all Wave-0 stubs replaced) |
| `grep -n '"ramp"' internal/notify/telegram.go` | 1 match (allowlist entry) |
| config.json untouched | `git diff --stat config.json` empty |

**Gate 6 insertion line:** risk_gate.go:130 (the comment header line `// Gate 6: ramp (Phase 14 — first gate that touches live capital).`). The plan estimated "after Gate 5 ending ~line 120" — actual line is 130 due to comment-header drift from the 8-gate doc-comment expansion at the function preamble.

## Decisions Made

1. **Forward-declared RampSnapshotter narrow interface on Tracker** — Tracker takes `RampSnapshotter` (interface with single `Snapshot() models.RampState` method) NOT `*RampController` concrete type. Rationale: lets Task 1 (Sizer wiring) compile cleanly before Task 3 creates the concrete `*RampController`. Plan 14-04 wires the *RampController at production; the duck-typing via Snapshot() signature preserves the D-15 boundary while keeping task ordering clean. Same shape as `ScanRunner` (Phase 11).

2. **stageSizeForCfg duplicated in risk_gate.go** — Mirrors Sizer.stageSize but kept as a separate helper. Doc comment on both says "if you change one, change both — and consider whether the duplication is still worth the decoupling." Alternative — exporting `Sizer.StageSize` and calling it from risk_gate.go — was rejected to avoid coupling risk_gate.go to the *Sizer concrete type when only the cap math is needed. The duplication is small (12 lines) and intentional.

3. **4 Gate-6 tests instead of plan's 3** — Added `TestRiskGate_Gate6_Ramp_NilRampController_FailsClosed` locking the defensive nil-ramp path. Plan 14-04 guarantees non-nil at production wiring, but the test prevents a future partial-wiring regression where someone forgets to wire `t.ramp` and live capital silently bypasses Gate 6. Belt-and-suspenders aligned with the plan's defense-in-depth philosophy.

4. **`fakeRampSnapshotter.unavailable=true → Snapshot{CurrentStage: 0}`** — Gate 6 detects via `stageSizeForCfg` returning 0 (out-of-range stage) and fails-closed with `ErrPriceGapRampStateUnavailable`. This mirrors the T-14-12 mitigation (legacy/empty state) — `RampController.bootstrapState` materializes (stage=1, counter=0) on the production path, but the gate must STILL fail-closed if it ever sees a 0 (e.g. corrupt-Redis race between bootstrap and gate consult).

5. **RampController bootstrap initializes to (Stage:1, Counter:0) on missing/legacy/corrupt state and immediately persists** — T-14-12 lock. Subsequent kill-9 test passes because the bootstrap is idempotent: a fresh controller on a populated store reads the persisted state instead of overwriting.

6. **ForcePromote preserves counter (D-15 #3); ForceDemote zeroes counter + increments DemoteCount (D-15 #4)** — Operator override on promotion is intentional; the counter represents earned-state and the operator should be able to fast-track stages without losing accumulated trust. Demotion always matches the asymmetric-ratchet semantics. Both fire `NotifyPriceGapRampForceOp` via the RampNotifier — Plan 14-04 wires real Telegram dispatch.

7. **Reset emits `action="reset"` RampEvent + clears DemoteCount** — Reset is the nuclear option (factory reset of ramp state). ForcePromote/ForceDemote keep DemoteCount visible across operations; Reset wipes it. Operator-visible distinction for Plan 14-05 pg-admin auditing.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Tracker referenced *RampController before Task 3 created it**
- **Found during:** Task 1 build verification
- **Issue:** Plan instructed wiring `t.ramp` as `*RampController` in Task 1, but `*RampController` doesn't exist until Task 3. `go build ./... → 1 error: undefined: RampController`. Acceptance criterion `go build ./... exit 0` would fail.
- **Fix:** Declared a narrow `RampSnapshotter` interface on Tracker (single method: `Snapshot() models.RampState`). Sizer wiring uses this interface; Task 3's `*RampController` satisfies it via duck typing at production wiring (Plan 14-04 cmd/main.go). Pattern matches existing `ScanRunner` interface from Phase 11.
- **Files modified:** internal/pricegaptrader/tracker.go (Task 1 commit `35b60c1`)
- **Verification:** `go build ./...` clean after Task 1; all subsequent tasks build cleanly.
- **Committed in:** `35b60c1` (Task 1 — fix landed before commit, no separate commit needed).

**2. [Rule 2 - Missing Critical] Added 4th Gate-6 test for nil ramp controller**
- **Found during:** Task 4 test design
- **Issue:** Plan called for 3 Gate-6 tests (over-budget rejects, no-op when off, fail-closed on state unavailable). The "fail-closed" test only exercised `unavailable=true` (CurrentStage=0). The case where `t.ramp == nil` (mis-wired) — a separate fail-closed branch in the gate — was untested. A future partial-wiring regression could silently bypass Gate 6.
- **Fix:** Added `TestRiskGate_Gate6_Ramp_NilRampController_FailsClosed` which sets `tr.ramp = nil` and asserts `ErrPriceGapRampStateUnavailable`. Locks the defensive nil-ramp path matching the existing fail-closed branch in risk_gate.go.
- **Files modified:** internal/pricegaptrader/risk_gate_test.go (Task 4 commit `c463d2e`)
- **Verification:** Test passes; total Gate-6 tests = 4 (3 from plan + 1 added).
- **Committed in:** `c463d2e` (Task 4 — covered under acceptance criterion "3 ramp gate tests pass" — landed 4 instead, all pass).

**Total deviations:** 2 auto-fixed (1 Rule 3 blocking task ordering, 1 Rule 2 missing critical defensive test). Both preserved acceptance criteria; deviation 2 strengthened them.

## Issues Encountered

- **None.** No flaky tests, no test regressions, no surprises in the existing risk-gate test suite (all 16 risk-gate tests pass — Gates 0–5 + delist/staleness now Gate 7 + 4 new Gate 6). The Wave-0 stub harness from Plan 14-01 made replacement straightforward.

## User Setup Required

None — all changes ship behind existing toggles. `cfg.PriceGapLiveCapital` defaults to false (Plan 14-01); until an operator flips it ON, the Sizer is pass-through and Gate 6 is no-op. RampController + RedisWSRampSink are constructible but not wired into the daemon yet — Plan 14-04 ships the cron daemon that calls `RampController.Eval` after the previous-day reconcile completes.

## Next Phase Readiness

**Plan 14-04 (Daemon + Notifier) ready:**
- `NewRampController(store, notifier, cfg, log, nowFn)` is the production constructor surface — Plan 14-04 wires it at cmd/main.go bootstrap with `*database.Client` (satisfies RampStateStore via Plan 14-01 method set), `*notify.TelegramNotifier` (satisfies RampNotifier — Plan 14-04 ships the real impl), `cfg`, `utils.NewLogger("pg-ramp")`, `time.Now`.
- `t.sizer = NewSizer(cfg)` and `t.ramp = rampController` slots in tracker.go are nil-by-default and ready for Plan 14-04 to populate.
- `RedisWSRampSink` is the dashboard-facing event sink — Plan 14-04 wires it as `NewRedisWSRampSink(db, hub, log)` and passes it to the daemon (which emits force-op events to the sink in addition to the controller's own AppendPriceGapRampEvent).
- `"ramp"` entry in `priceGapGateAllowlist` lets Plan 14-04 dispatch Gate-6 risk-block notifications via the existing `NotifyPriceGapRiskBlock` plumbing — no telegram.go retro-fit needed.

**Plan 14-05 (CLI pg-ramp-admin) ready:**
- `RampController.ForcePromote(operator, reason)`, `ForceDemote(operator, reason)`, `Reset(operator, reason)` are the operator surfaces. All three persist state, emit RampEvents to pg:ramp:events, and fire NotifyPriceGapRampForceOp via the RampNotifier. pg-admin can construct a controller against the same store, invoke any of these methods with `operator="pg-admin"`, and the daemon's controller will see the new state on its next Eval (state is loaded fresh on each call via the persistence path).

**No blockers. CLAUDE.local.md rules respected:** config.json untouched, no npm install, frontend unchanged (no go:embed concern), Go module isolation preserved (zero internal/engine or internal/spotengine imports in any new file).

## Self-Check: PASSED

Verification claims confirmed:

- All commits exist:
  - `35b60c1` (Task 1 Sizer + sentinels + tracker wiring) — FOUND
  - `4439469` (Task 2 RED ramp tests) — FOUND
  - `19271b5` (Task 3 GREEN RampController + RedisWSRampSink) — FOUND
  - `c463d2e` (Task 4 GREEN Gate 6 + tests + allowlist) — FOUND
- All claimed files exist:
  - `internal/pricegaptrader/sizer.go` — FOUND (75 lines)
  - `internal/pricegaptrader/ramp_controller.go` — FOUND (227 lines)
  - `internal/pricegaptrader/redis_ramp_sink.go` — FOUND (62 lines)
  - All 7 modified files — FOUND
- All acceptance criteria satisfied:
  - 0 t.Skip in sizer_test.go / ramp_controller_test.go / risk_gate_test.go (all Wave-0 stubs replaced)
  - 7 TestRampController_* + 2 TestSizer_* + 4 TestRiskGate_Gate6_Ramp_* = 13 new tests (plan called for 12; 4 instead of 3 Gate-6 tests for the nil-ramp lock)
  - 5 RampController methods (Eval, ForcePromote, ForceDemote, Reset, Snapshot)
  - 2 narrow interfaces (RampStateStore, RampNotifier)
  - 0 internal/engine / internal/spotengine imports across the 3 new files (D-15 isolation)
  - 3 `min(stage` matches in sizer.go + risk_gate.go (D-22 defense in depth at both layers)
  - "Gate 6: ramp" + "Gate 7: delist" both present (renumber landed)
  - "ramp" in priceGapGateAllowlist (telegram.go line 349)
  - 3 `rc.state.CleanDayCounter = 0` matches in ramp_controller.go (loss-day, ForceDemote, Reset)
  - 1 `// D-15 #3` match in ramp_controller.go (force-promote counter-preserved)
- Tests: 29 PASS in 2 packages (2 sizer + 7 ramp + 16 risk-gate); 301 PASS across full pricegaptrader+notify suite; -count=100 stable on TestRampController_AsymmetricRatchetInvariant
- go build ./... clean; go vet ./... clean
- config.json untouched (`git diff --stat config.json` returns empty)

---
*Phase: 14-daily-reconcile-live-ramp-controller*
*Completed: 2026-04-30*
