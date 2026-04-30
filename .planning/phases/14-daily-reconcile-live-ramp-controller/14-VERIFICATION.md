---
phase: 14-daily-reconcile-live-ramp-controller
verified: 2026-04-30T00:00:00Z
status: human_needed
score: 6/6 must-haves verified
human_verification:
  - test: "Load the dashboard Pricegap-tracker tab and confirm the Ramp + Reconcile widget renders with live-capital badge (color-coded red/gray), ramp stage fields, and last-reconcile summary. No mutation buttons should be visible."
    expected: "Widget shows all 5 ramp state fields (current_stage, clean_day_counter, last_eval_ts, last_loss_day_ts, demote_count), a last-reconcile panel (or 'no data' placeholder for new installs), and a color-coded LIVE CAPITAL badge. No force-promote/demote/reset buttons present in the UI."
    why_human: "Dashboard visual rendering and badge color behavior cannot be verified programmatically. Operator UAT was conducted during execution (14-05 checkpoint approved); this item is recorded for completeness per verification protocol."
---

# Phase 14: Daily Reconcile + Live Ramp Controller Verification Report

**Phase Goal:** Strategy 4 has a daily PnL reconcile job producing per-day realized-PnL output AND a Redis-persisted ramp controller that gates live-capital sizing through 100 → 500 → 1000 USDT/leg stages with asymmetric ratchet — the first phase to touch live capital.
**Verified:** 2026-04-30
**Status:** human_needed (automated checks passed; one dashboard visual item recorded per protocol — already UAT-approved by operator during execution)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Daily reconcile fires 30+ min after UTC 00:00, aggregates by `(position_id, version)`, idempotent (byte-equality), writes `pg:reconcile:daily:{date}` | ✓ VERIFIED | `reconcileLoop` goroutine in tracker.go:431; UTC 00:30 fire documented in daemon.go; byte-equality check pattern confirmed in reconciler.go:106,278 |
| 2 | Ramp controller is Redis-persisted with 5 explicit fields; kill-9 + restart resumes correct state | ✓ VERIFIED | `ramp_controller.go` has `Eval`, `ForcePromote`, `ForceDemote`, `Reset`, `Snapshot`; `database/pricegap_state.go:407` implements `SavePriceGapRampState` writing 5-field HASH; `LoadPriceGapRampState` round-trips state |
| 3 | Asymmetric ratchet: single losing day resets clean-day counter to 0 AND demotes one stage if above stage 1 | ✓ VERIFIED | `ramp_controller.go:168` — `NotifyPriceGapRampDemote` fires on demote; ratchet logic in `Eval`; `ForcePromote` does NOT zero counter (intentional per plan); `ForceDemote` and `Reset` do zero counter |
| 4 | Hard ceiling enforced as `min(stage_size, hard_ceiling)` at sizer call site | ✓ VERIFIED | `sizer.go:36,45-47` — `func (s *Sizer) Cap` enforces `min(notional, stage_size, hard_ceiling)` with `PriceGapHardCeilingUSDT` check; risk_gate.go:151-153 provides defense-in-depth second layer |
| 5 | Ramp gate integrated into `risk_gate.go` as Gate 6 (Gate 7 = delist/staleness); returns `ErrPriceGapRampExceeded` | ✓ VERIFIED | `risk_gate.go:32-33` — comment explicitly states "Phase 14 inserts Gate 6 (ramp) AFTER Gate 5; previous Gate 6 is now Gate 7"; `ErrPriceGapRampExceeded` returned with `Reason: "ramp"` |
| 6 | Boot guard: `PriceGapLiveCapital=true` + missing ramp state → panic + Telegram critical | ✓ VERIFIED | `tracker.go:392-407` — boot guard panics with "live_capital=true requires valid pg:ramp:state"; `t.notifier.NotifyPriceGapReconcileFailure("BOOT_GUARD", ...)` fires before panic |

**Score:** 6/6 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/config.go` | `PriceGapLiveCapital` + 6 sizing fields + `validatePriceGapLive` | ✓ VERIFIED | Field at line 413, default false (line 883); `validatePriceGapLive` at line 74 rejects `live_capital=true` with `paper_mode=true` |
| `internal/models/pricegap_position.go` | `Version` + `ExchangeClosedAt` fields | ✓ VERIFIED | Both fields present with `omitempty` JSON tags (lines 93, 97) |
| `internal/models/pricegap_interfaces.go` | Extended `PriceGapStore` with 7 new methods | ✓ VERIFIED | `AddPriceGapClosedPositionForDate`, `SavePriceGapRampState`, `LoadPriceGapRampState` confirmed at lines 99, 103-104; full method set present |
| `internal/database/pricegap_state.go` | Concrete impl of all 7 store methods | ✓ VERIFIED | `AddPriceGapClosedPositionForDate` at line 358; `SavePriceGapRampState` at line 412; `LoadPriceGapRampState` present |
| `internal/pricegaptrader/monitor.go` | SADD `pg:positions:closed:{date}` in `closePair` | ✓ VERIFIED | `t.db.AddPriceGapClosedPositionForDate(pos.ID, closeDate)` at line 258 |
| `internal/pricegaptrader/reconciler.go` | 3-retry reconciler + idempotency | ✓ VERIFIED | 12.9K file; byte-equality idempotency documented in code; `NotifyPriceGapReconcileFailure` called on triple-fail |
| `internal/pricegaptrader/ramp_controller.go` | State machine with all 5 ops | ✓ VERIFIED | All 5 methods (`Eval`, `Snapshot`, `ForcePromote`, `ForceDemote`, `Reset`) confirmed at lines 114-243 |
| `internal/pricegaptrader/redis_ramp_sink.go` | `RedisWSRampSink` struct | ✓ VERIFIED | File exists (2.7K); mirrors Phase 12 `RedisWSPromoteSink` pattern |
| `internal/pricegaptrader/risk_gate.go` | Gate 6 (ramp) + Gate 7 (delist/staleness) + `ErrPriceGapRampExceeded` | ✓ VERIFIED | Gate 6 comment at line 32; `ErrPriceGapRampExceeded` returned with reason "ramp"; `PriceGapHardCeilingUSDT` defense-in-depth at line 151 |
| `internal/pricegaptrader/sizer.go` | `Cap(notional, stage)` → `min(stage_size, hard_ceiling)` | ✓ VERIFIED | `func (s *Sizer) Cap` at line 36; hard ceiling applied at lines 45-47 |
| `internal/pricegaptrader/tracker.go` | `reconcileLoop` daemon + boot guard + sizer/ramp wiring | ✓ VERIFIED | `reconcileLoop` goroutine launched at line 431; boot guard panic at lines 392-407 |
| `internal/notify/pricegap_reconcile.go` | 4 Telegram methods: DailyDigest, ReconcileFailure, RampDemote, RampForceOp | ✓ VERIFIED | All 4 concrete implementations confirmed at lines 38, 83, 104, 121; `TelegramNotifier` satisfies the interface |
| `cmd/pg-admin/reconcile.go` | `cmdReconcileRun` + `cmdReconcileShow` | ✓ VERIFIED | File exists (4.5K); both commands present |
| `cmd/pg-admin/ramp.go` | `cmdRampShow/Reset/ForcePromote/ForceDemote` | ✓ VERIFIED | File exists (4.4K); all 4 subcommands confirmed via help text in main.go:469-474 |
| `cmd/pg-admin/main.go` | Dispatch cases "reconcile" and "ramp" | ✓ VERIFIED | `case "reconcile"` at line 169; `case "ramp"` at line 171; production wires `pgReconciler` and `pgRamp` at lines 357-358 |
| `internal/api/pricegap_ramp_handlers.go` | `handlePgRampState` + `handlePgReconcileDay` | ✓ VERIFIED | File exists (5.0K); both routes registered in server.go:235-236 |
| `web/src/components/Ramp/RampReconcileSection.tsx` | Read-only React widget; fetches `/api/pg/ramp` + `/api/pg/reconcile/{date}` | ✓ VERIFIED | Fetches confirmed at lines 94, 110; read-only invariant comment at lines 10-11; no mutation UI |
| `web/src/pages/PriceGap.tsx` | `<RampReconcileSection />` wired after `<DiscoverySection />` | ✓ VERIFIED | Import at line 4; rendered at line 779 |
| `web/src/i18n/en.ts` + `web/src/i18n/zh-TW.ts` | 13 `pricegap.ramp.*` + `pricegap.reconcile.*` keys in lockstep | ✓ VERIFIED | `pricegap.ramp.title` confirmed in both locales; zh-TW shows "即時資金階梯" / "最後對帳" translations present |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `monitor.go closePair` | `database/pricegap_state.go` | `AddPriceGapClosedPositionForDate` | ✓ WIRED | Direct call at monitor.go:258 |
| `config.go validate chain` | `validatePriceGapLive` | function call | ✓ WIRED | `validatePriceGapLive` at config.go:74; rejects live+paper combo at line 75 |
| `RampController.Eval` | `SavePriceGapRampState` | interface call after every state change | ✓ WIRED | `persistLocked` method at ramp_controller.go:243 called after every mutation |
| `risk_gate.go Gate 6` | `Sizer.Cap` (defense-in-depth) | `min(stage_size, hard_ceiling)` at two layers | ✓ WIRED | Gate 6 in risk_gate.go; Sizer.Cap in sizer.go:36 |
| `risk_gate.go Gate 6` | `ErrPriceGapRampExceeded` | `GateDecision` return | ✓ WIRED | Confirmed at risk_gate.go |
| `Tracker.reconcileLoop` | `Reconciler.RunForDate → RampController.Eval → Notifier.NotifyPriceGapDailyDigest` | synchronous call chain | ✓ WIRED | daemon.go:29,54,67; tracker.go:431 |
| `cmd/pg-admin Run dispatch` | `cmdReconcileRun, cmdRampShow, cmdRampForcePromote, cmdRampForceDemote, cmdRampReset` | switch cases | ✓ WIRED | main.go:169,171; all 6 subcommands in help text |
| `RampReconcileSection.tsx` | `/api/pg/ramp` + `/api/pg/reconcile/{date}` | `fetch` on mount | ✓ WIRED | Confirmed at RampReconcileSection.tsx:94,110 |
| `PriceGap.tsx` | `<RampReconcileSection />` | JSX child rendering | ✓ WIRED | PriceGap.tsx:4 (import), 779 (render) |
| `cmd/main.go` | `pgTracker.SetNotifier(tg)` | `TelegramNotifier` satisfies 4-method interface | ✓ WIRED | cmd/main.go:486; `internal/notify/pricegap_reconcile.go` has all 4 concrete impls |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|--------------------|--------|
| `RampReconcileSection.tsx` | ramp state (5 fields) | `GET /api/pg/ramp` → `RampController.Snapshot()` → `SavePriceGapRampState` → Redis HASH | Yes — Redis HASH persisted on every state change | ✓ FLOWING |
| `RampReconcileSection.tsx` | reconcile record | `GET /api/pg/reconcile/{date}` → `Reconciler.LoadRecord()` → Redis | Yes — DB query, returns `DailyReconcileRecord` or 404 | ✓ FLOWING |

### Behavioral Spot-Checks

Step 7b: SKIPPED — API endpoints require running server; config changes require live Redis. All behavioral assertions verified through code tracing (gates, sizer, ramp state machine).

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| PG-LIVE-01 | 14-01, 14-03, 14-04, 14-05 | Conservative ramp controller: 3 stages, asymmetric ratchet, Redis-persisted, Gate 6, hard ceiling | ✓ SATISFIED | `ramp_controller.go` state machine; `risk_gate.go` Gate 6; `sizer.go` Cap; `tracker.go` boot guard; dashboard widget |
| PG-LIVE-03 | 14-01, 14-02, 14-04, 14-05 | Daily PnL reconcile: 30+ min after UTC 00:00, `(position_id, version)` keyed, idempotent, anomaly flagging | ✓ SATISFIED | `reconciler.go` with byte-equality idempotency; `daemon.go` UTC 00:30 fire; `pricegap_reconcile.go` DailyDigest + ReconcileFailure Telegram |

No orphaned requirements — REQUIREMENTS.md traceability table shows PG-LIVE-01 and PG-LIVE-03 both mapped to Phase 14, both marked Complete.

### Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| (none) | No TODO/FIXME/PLACEHOLDER/stub patterns found in key artifacts | — | Clean |

No stubs, no empty implementations, no hardcoded-empty data paths found in any Phase 14 artifact.

Known pre-existing flake: `TestScanner_RunCycleCallsPromotionApply` in `internal/pricegaptrader` — documented in deferred-items.md from Plans 14-01 and 14-02. Not a Phase 14 regression.

### Human Verification Required

#### 1. Dashboard Ramp + Reconcile Widget Visual

**Test:** Load the dashboard Pricegap-tracker tab; locate the Ramp + Reconcile widget.
**Expected:** Widget shows all 5 ramp state fields, last-reconcile summary (or empty-state placeholder), prominent color-coded LIVE CAPITAL badge (red when live, gray when off), and no mutation buttons.
**Why human:** Dashboard visual rendering and badge color-coding require browser inspection. Note: the operator already performed UAT approval of the 14-05 checkpoint during execution — this item is recorded for protocol completeness only.

### Gaps Summary

No gaps identified. All 6 roadmap success criteria are satisfied by concrete code:

1. Reconcile fires UTC 00:30 (`daemon.go`/`tracker.go`), aggregates by `(position_id, version)`, idempotent via byte-equality (`reconciler.go`), writes `pg:reconcile:daily:{date}` Redis keys (`database/pricegap_state.go`).
2. Anomaly flagging present in reconciler; `NotifyPriceGapReconcileFailure` + `NotifyPriceGapDailyDigest` Telegram methods implemented and wired (`pricegap_reconcile.go` + `cmd/main.go:486`).
3. Ramp state Redis-persisted in 5-field HASH (`SavePriceGapRampState`); `bootstrapState` in `ramp_controller.go` reloads on restart.
4. Asymmetric ratchet locked: loss day resets counter + demotes one stage (`ramp_controller.go:168`).
5. Hard ceiling as `min(stage_size, hard_ceiling)` at sizer call site (`sizer.go:45-47`) + risk_gate Gate 6 defense-in-depth.
6. Gate 6 integrated into `risk_gate.go` after Gate 5 concentration; previous Gate 6 renumbered to Gate 7 (`risk_gate.go:32-33`).

Status is `human_needed` (not `passed`) because the dashboard visual verification item exists — consistent with the protocol rule that any human_verification item prevents `passed` regardless of automated score.

---

_Verified: 2026-04-30_
_Verifier: Claude (gsd-verifier)_
