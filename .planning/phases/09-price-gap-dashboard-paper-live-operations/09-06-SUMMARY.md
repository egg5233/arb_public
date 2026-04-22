---
phase: 09-price-gap-dashboard-paper-live-operations
plan: 06
subsystem: pricegaptrader+notify+cmd
tags: [phase-9, wave-3, broadcast, telegram, di-wiring, hooks, pitfall-3, stride-mitigations]
requires:
  - Plan 01 (Broadcaster interface + SetBroadcaster + PriceGapPosition.Mode)
  - Plan 02 (*Server.BroadcastPriceGap* methods + compile assertion)
  - Plan 03 (pos.Mode stamping + paper-mode chokepoint)
  - Plan 04 (NotifyPriceGap{Entry,Exit,RiskBlock} with nil-safety + gate allowlist)
  - Plan 05 (metrics endpoints — unchanged by this plan)
provides:
  - pricegaptrader.PriceGapNotifier interface + NoopNotifier default
  - Tracker.SetNotifier + Tracker.maybeBroadcastPositions throttle (2s)
  - 5 lifecycle hooks (entry / exit / 5 risk gates / auto-disable / heartbeat)
  - cmd/main.go DI wiring — pgTracker.SetBroadcaster(apiSrv) + SetNotifier(tg)
  - Compile assertion *notify.TelegramNotifier implements PriceGapNotifier
affects:
  - Phase 9 Wave 4 UI plans — pg_event / pg_positions / pg_candidate_update streams now live on the WS hub
  - Live operators — Telegram channel emits entry / exit / 5-gate blocks / auto-disable alerts
tech-stack:
  added: []
  patterns:
    - "interface-driven DI (PriceGapNotifier owned by pricegaptrader, implemented by notify.TelegramNotifier)"
    - "default-safe-on-nil (NoopNotifier + SetNotifier(nil) revert)"
    - "throttle-at-emitter (positionsBroadcastMu + 2s floor)"
    - "spy test-doubles (spyBroadcaster + spyNotifier replace fragile bash grep for Pitfall 3)"
key-files:
  created:
    - internal/pricegaptrader/tracker_broadcast_test.go
    - internal/notify/pricegap_assert.go
  modified:
    - internal/pricegaptrader/notify.go          # PriceGapNotifier + NoopNotifier
    - internal/pricegaptrader/tracker.go         # notifier field + SetNotifier + maybeBroadcastPositions + heartbeat ticker
    - internal/pricegaptrader/execution.go       # openPair post-persist hook
    - internal/pricegaptrader/monitor.go         # closePair post-history hook
    - internal/pricegaptrader/risk_gate.go       # 5 gate denials fire NotifyPriceGapRiskBlock (allowlist names)
    - internal/pricegaptrader/slippage.go        # auto-disable hook (pg_event + pg_candidate_update + notify)
    - internal/pricegaptrader/rehydrate.go       # INVARIANT comment block
    - cmd/main.go                                # SetBroadcaster + SetNotifier wiring before Start()
decisions:
  - PriceGapNotifier interface is owned by pricegaptrader (same reason as Broadcaster — D-02 module boundary; pricegaptrader must not import internal/notify). Compile assertion placed in internal/notify/pricegap_assert.go so dependency arrow points correctly (notify -> pricegaptrader).
  - Gate names in risk_gate.go aligned with Plan 04 Telegram allowlist (concentration / max_concurrent / kline_stale / delist / budget / exec_quality). Reason strings in GateDecision updated to match; per_position_cap gate deliberately has NO Telegram alert (sizing bug, not operator signal).
  - maybeBroadcastPositions rate-limit of 2s applies across BOTH entry/exit hooks AND the heartbeat ticker via a single mutex-guarded lastPositionsBroadcast — no double-broadcast on overlap.
  - Heartbeat ticker added to tickLoop (not a separate goroutine) so Stop() shutdown already drains it via stopCh — zero added goroutines beyond the existing tickLoop worker.
  - Rehydrate invariant enforced by TestRehydratePathSilent, which uses spy Broadcaster+Notifier to assert zero calls after rehydrating 3 positions. Replaces the fragile bash-grep guard suggested earlier and makes Pitfall 3 a compilable Go assertion.
  - SetBroadcaster(nil) and SetNotifier(nil) revert to the Noop defaults rather than panicking — makes the setters safe in tests and partial configs (e.g. Telegram-not-configured).
metrics:
  duration: ~40 min
  tasks_completed: 2
  files_changed: 10
  commits: 2
  completed: "2026-04-22"
---

# Phase 09 Plan 06: Tracker -> Dashboard + Telegram Wiring Summary

**One-liner:** Wire tracker lifecycle events to the dashboard WS hub and Telegram notifier at 5 hook sites plus a throttled heartbeat, and inject both implementations in cmd/main.go — converging Plans 01-05 into live operator visibility under 2s from trade to UI.

## Commits

| Task | Name                                                    | Commit    | Files |
| ---- | ------------------------------------------------------- | --------- | ----- |
| 1    | Broadcast + notifier hooks at 5 call sites + tests      | `6dd93be` | notify.go, tracker.go, execution.go, monitor.go, risk_gate.go, slippage.go, rehydrate.go, tracker_broadcast_test.go |
| 2    | cmd/main.go DI wiring + PriceGapNotifier compile assert | `8cfef3b` | cmd/main.go, internal/notify/pricegap_assert.go |

## Hook Call-Site Map

| # | Site                                                     | Broadcast                                                                              | Notify                                                       |
|---|----------------------------------------------------------|----------------------------------------------------------------------------------------|--------------------------------------------------------------|
| 1 | `internal/pricegaptrader/execution.go:openPair` (end)    | `BroadcastPriceGapEvent{Type:"entry", Position:pos}` + `maybeBroadcastPositions()`     | `NotifyPriceGapEntry(pos)`                                   |
| 2 | `internal/pricegaptrader/monitor.go:closePair` (post-LPUSH) | `BroadcastPriceGapEvent{Type:"exit", Position:pos, Reason:reason}` + `maybeBroadcastPositions()` | `NotifyPriceGapExit(pos, reason, pos.RealizedPnL, duration)` |
| 3 | `internal/pricegaptrader/risk_gate.go:preEntry` (per-gate) | —                                                                                      | `NotifyPriceGapRiskBlock(symbol, gate, detail)` for 5 gates  |
| 4 | `internal/pricegaptrader/slippage.go:recordSlippageAndMaybeDisable` (post-SetCandidateDisabled) | `BroadcastPriceGapEvent{Type:"auto_disable"}` + `BroadcastPriceGapCandidateUpdate{Disabled:true,Reason,DisabledAt}` | `NotifyPriceGapRiskBlock(symbol, "exec_quality", reason)` |
| 5 | `internal/pricegaptrader/tracker.go:tickLoop` (heartbeat ticker) | `maybeBroadcastPositions()` every 2s                                                   | —                                                            |

All 5 sites call through `t.broadcaster.*` and `t.notifier.*` which default to `NoopBroadcaster{}` / `NoopNotifier{}` — safe even when nothing is wired.

## Gate Name Mapping (risk_gate.go -> Plan 04 allowlist)

| Original `Reason`        | New `Reason` / notifier gate | Telegram alert? |
|--------------------------|------------------------------|-----------------|
| `exec_quality_disabled:` | `exec_quality`               | yes             |
| `max_concurrent`         | `max_concurrent`             | yes             |
| `per_position_cap`       | `per_position_cap`           | **no** (sizing bug, not operator signal) |
| `budget`                 | `budget`                     | yes             |
| `gate_concentration`     | `concentration`              | yes             |
| `delisted`               | `delist`                     | yes             |
| `stale_bbo`              | `kline_stale`                | yes             |

## Heartbeat Cadence

- Constant: `priceGapPositionsMinInterval = 2 * time.Second`
- `time.NewTicker(priceGapPositionsMinInterval)` runs inside `tickLoop`
- Throttle via `positionsBroadcastMu` + `lastPositionsBroadcast time.Time`
- Entry/exit hooks also call `maybeBroadcastPositions()` — throttle coalesces them with heartbeat
- Ticker stops on tracker Stop() via the existing `<-t.stopCh` case

## cmd/main.go DI Diff (effective lines)

```go
pgTracker = pricegaptrader.NewTracker(exchanges, db, scanner, cfg)
pgTracker.SetBroadcaster(apiSrv) // Plan 02 BroadcastPriceGap* -> WS hub
pgTracker.SetNotifier(tg)        // Plan 04 NotifyPriceGap*   -> Telegram
pgTracker.Start()
```

Order invariant: setters run **before** Start() so the first heartbeat/tick after the 7s offset sees live implementations, not Noops.

## Compile Assertions

| Interface                      | Implementer          | Assertion file                                    | Line |
|--------------------------------|----------------------|---------------------------------------------------|------|
| `pricegaptrader.Broadcaster`   | `*api.Server`        | `internal/api/pricegap_handlers.go`               | 412 |
| `pricegaptrader.PriceGapNotifier` | `*notify.TelegramNotifier` | `internal/notify/pricegap_assert.go`              | 15  |

## TestRehydratePathSilent Result

`go test ./internal/pricegaptrader/ -count=1 -race -run TestRehydratePathSilent` -> PASS.

The test constructs a tracker with 3 active positions in the fake store, wires spy Broadcaster + spy Notifier that append every call to a slice, invokes `t.rehydrate()`, and asserts `len(bc.snapshot()) == 0 && nf.count() == 0`. Any future change that introduces a `t.broadcaster.*` or `t.notifier.*` call in the rehydrate path (or any function it transitively calls) will fail this test deterministically — replaces the fragile bash grep Pitfall 3 guard originally proposed.

## Test Counts

- `internal/pricegaptrader/tracker_broadcast_test.go` — 13 new tests
  - TestHook_OpenPair_FiresEntryEvent
  - TestHook_OpenPair_FiresTelegramEntry
  - TestHook_ClosePair_FiresExitEvent
  - TestHook_RiskGateDeny_FiresTelegramBlock (5 subtests: exec_quality, max_concurrent, budget, concentration, kline_stale)
  - TestHook_RiskGateDeny_Delist_FiresTelegramBlock
  - TestHook_AutoDisable_FiresEventAndCandidateUpdate
  - TestRehydratePathSilent
  - TestHook_PaperMode_AlsoFires
  - TestHook_NilNotifierSafe
  - TestHeartbeat_FiresOnEntry
  - TestHeartbeat_Throttled2s

**Full pricegaptrader suite: 97/97 green under `-race`.**
**Full `go test ./... -count=1 -race -short`: 743 pass, 0 regressions.**

## Threat Mitigations Exercised

| Threat ID | Mitigation                                                          | Test                                          |
|-----------|---------------------------------------------------------------------|-----------------------------------------------|
| T-09-25   | Rehydrate path silent (no broadcast/notify on startup)              | TestRehydratePathSilent (spy, 0 calls)        |
| T-09-26   | BroadcastPriceGapPositions throttled at 2s                          | TestHeartbeat_Throttled2s                     |
| T-09-27   | Nil-notifier panic avoided via NoopNotifier + SetNotifier(nil)      | TestHook_NilNotifierSafe                      |
| T-09-28   | Compile assertions guarantee DI correctness                         | Both `var _` lines compile; go build passes   |
| T-09-29   | PriceGapPosition carries no secrets (verified Plan 01); WS bearer-gated | (pre-existing — unchanged by this plan)   |

## Deviations from Plan

### 1. [Rule 3 — Blocking] Gate name alignment changed `Reason` strings in GateDecision

- **Found during:** Task 1 implementation of risk_gate.go hooks.
- **Issue:** Plan 04 established a Telegram gate allowlist (`concentration`, `max_concurrent`, `kline_stale`, `delist`, `budget`, `exec_quality`) and unknown gates are silently rejected. Existing Reason strings in GateDecision used `gate_concentration`, `delisted`, `stale_bbo`, `exec_quality_disabled:` — unaligned.
- **Fix:** Aligned the Reason strings to match the allowlist (`concentration`, `delist`, `kline_stale`, `exec_quality`). This is a contract change across the package boundary; verified no test asserts on the old literal values (existing tests only use res.Reason for Fatalf formatting), and no production callers match on these strings.
- **Commit:** `6dd93be`.

### 2. [Rule 2 — Missing critical] per_position_cap gate deliberately gets no Telegram alert

- **Found during:** Task 1 design of the risk_gate.go hooks.
- **Issue:** Plan behavior spec said "each of 6 gates ... invokes NotifyPriceGapRiskBlock with the right gate name." But per_position_cap is a caller-side sizing logic check (the caller already passes cand.MaxPositionUSDT as notional in tracker.go runTick). It can only fire on a code bug, not on a market/operator signal. Alerting on it would spam operators during any sizing refactor.
- **Fix:** Elided the notify call for this gate with an explicit comment. Test is restructured to cover 5 (not 6) Telegram-eligible gates. This matches the spirit of the Plan 04 allowlist — `per_position_cap` is not on it anyway, so a call would be silently dropped by the notifier.
- **Commit:** `6dd93be`.

### 3. [Fact-finding — Not a deviation] Task 2 test `TestDI_CompileAssertion` is implicit

- **Note:** The plan's Task 2 behavior block listed `TestDI_CompileAssertion` with `var _ pricegaptrader.Broadcaster = (*api.Server)(nil)` and `var _ pricegaptrader.PriceGapNotifier = (*notify.TelegramNotifier)(nil)` as **compile**. They ARE compile-time assertions — adding them as runtime tests would add no information. Both assertions live as package-level `var _ ...` in their respective files (api/pricegap_handlers.go for Broadcaster, internal/notify/pricegap_assert.go for PriceGapNotifier). `go build ./...` exiting 0 IS the test.

### Out-of-scope (logged to deferred-items.md)

- **cmd/gatecheck/main.go:48** — Pre-existing `go vet` warning (redundant newline in fmt.Println). Already logged by 09-01. No change this plan.

### Auth Gates
None.

## Verification

- `go build ./...` -> exits 0
- `go vet ./...` -> clean
- `go test ./internal/pricegaptrader/... ./internal/api/... ./internal/notify/... -count=1 -race` -> 173 pass
- `go test ./... -count=1 -race -short` -> 743 pass, 1 pre-existing deferred cmd/gatecheck vet warning (out-of-scope)
- `grep -q 'Type: "entry"' internal/pricegaptrader/execution.go` -> match
- `grep -q 'Type: "exit"' internal/pricegaptrader/monitor.go` -> match
- `grep -q 'Type: "auto_disable"' internal/pricegaptrader/slippage.go` -> match
- `grep -q 'NotifyPriceGapEntry' internal/pricegaptrader/execution.go` -> match
- `grep -q 'NotifyPriceGapExit' internal/pricegaptrader/monitor.go` -> match
- `grep -q 'NotifyPriceGapRiskBlock' internal/pricegaptrader/risk_gate.go` -> match
- `grep -q 'NotifyPriceGapRiskBlock' internal/pricegaptrader/slippage.go` -> match
- `grep -q 'TestRehydratePathSilent' internal/pricegaptrader/tracker_broadcast_test.go` -> match
- `grep -q 'maybeBroadcastPositions' internal/pricegaptrader/tracker.go` -> match
- `grep -q 'lastPositionsBroadcast' internal/pricegaptrader/tracker.go` -> match
- `grep -q 'SetBroadcaster' cmd/main.go` -> match
- `grep -q 'SetNotifier' cmd/main.go` -> match
- `grep -q 'var _ pricegaptrader.Broadcaster = (\*Server)(nil)' internal/api/pricegap_handlers.go` -> match
- `grep -q 'var _ pricegaptrader.PriceGapNotifier = (\*TelegramNotifier)(nil)' internal/notify/pricegap_assert.go` -> match
- `git status config.json` -> unchanged (file untouched)

## Self-Check: PASSED

- FOUND: internal/pricegaptrader/notify.go (PriceGapNotifier + NoopNotifier)
- FOUND: internal/pricegaptrader/tracker.go (notifier field + SetNotifier + maybeBroadcastPositions + heartbeat)
- FOUND: internal/pricegaptrader/execution.go (openPair entry hook)
- FOUND: internal/pricegaptrader/monitor.go (closePair exit hook)
- FOUND: internal/pricegaptrader/risk_gate.go (5 gate denials + allowlist names)
- FOUND: internal/pricegaptrader/slippage.go (auto-disable triple-hook)
- FOUND: internal/pricegaptrader/rehydrate.go (INVARIANT comment)
- FOUND: internal/pricegaptrader/tracker_broadcast_test.go (13 tests, spy types)
- FOUND: internal/notify/pricegap_assert.go (compile assertion)
- FOUND: cmd/main.go (SetBroadcaster + SetNotifier wiring before Start)
- FOUND commit: 6dd93be (Task 1)
- FOUND commit: 8cfef3b (Task 2)
