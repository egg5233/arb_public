---
phase: 08-price-gap-tracker-core
verified: 2026-04-21T00:00:00Z
status: human_needed
score: 5/5 must-haves verified
gaps: []
deferred: []
human_verification:
  - test: "Build the full repo and confirm no compile errors"
    expected: "go build ./... succeeds with zero errors"
    why_human: "Cannot run go build in sandbox; compile-time assertion var _ models.PriceGapStore = (*Client)(nil) provides strong signal but full build must be confirmed"
  - test: "Run go test ./internal/pricegaptrader/... ./internal/database/... -race -count=1"
    expected: "All unit tests pass (detector, execution, monitor, rehydrate, risk_gate, slippage, tracker, pricegap_state)"
    why_human: "Tests require Redis (pricegap_state_test.go) and cannot run offline"
  - test: "Start binary with PriceGapEnabled=false (default config), observe startup log"
    expected: "Log line: 'Price-gap tracker disabled (cfg.PriceGapEnabled=false)' appears; no pricegaptrader goroutines launched; perp-perp and spot-futures engines start and operate normally"
    why_human: "Requires live binary execution against a running Redis instance"
  - test: "Start binary with PriceGapEnabled=true and one candidate; observe tick loop"
    expected: "Ticker fires every PollIntervalSec seconds; detector logs BBO samples; no panics; existing engine logs are unchanged"
    why_human: "Requires live binary with config.json edit and running exchanges/Redis"
---

# Phase 8: Price-Gap Tracker Core — Verification Report

**Phase Goal:** A new isolated tracker subsystem detects cross-exchange price-gap events on a static candidate list and opens/closes delta-neutral positions under hard risk gates — gated behind a dashboard-off default switch
**Verified:** 2026-04-21
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `PriceGapEnabled` in config gates the entire subsystem; OFF by default; no tracker code paths run when OFF | VERIFIED | `cmd/main.go:468` — `if cfg.PriceGapEnabled { ... }` conditional; `log.Info("Price-gap tracker disabled...")` on false path; config.go field defaults to zero-value false |
| 2 | When spread crosses threshold T and persists ≥4 consecutive 1m bars with fresh BBO (<90s), simultaneous IOC market orders placed on both legs and position recorded to Redis | VERIFIED | `detector.go` — 4-slot barRing with `allExceed(T)` guard; `execution.go:72-76` — goroutines launch `placeLeg(..., "ioc", ...)` on both legs simultaneously; `database/pricegap_state.go:56` — `pipe.HSet(ctx, keyPricegapPositions, ...)` |
| 3 | Open positions auto-close when spread reverts to ≤T/2 OR 4h max-hold elapses; closed positions persist with PnL and exit reason; survive restart via Redis rehydration | VERIFIED | `monitor.go:104-127` — `maxHold` check and `reversionFactor` (default 0.5) check; `monitor.go:201` — realized PnL computed; `ExitReason` set before save; `rehydrate.go` — `GetActivePriceGapPositions()` + `startMonitor` re-enrollment on startup |
| 4 | Pre-entry risk gates block: concentration >50% Gate budget, max 3 concurrent, per-candidate notional cap, delist/halt/stale kline | VERIFIED | `risk_gate.go:34` — `preEntry()` with 5 checks; `risk_gate_test.go` — 13 test cases covering all gate conditions; typed errors in `errors.go` |
| 5 | After 10 closed trades, if mean realized slippage >2× modeled, tracker auto-disables candidate | VERIFIED | `slippage.go:32` — `recordSlippageAndMaybeDisable()`; `GetSlippageWindow(candidateID, 10)` + ratio check; `SetCandidateDisabled()` called on breach; `slippage_test.go` — TestSlippage_TenSamples_JustOverThreshold and boundary tests |

**Score:** 5/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/models/pricegap_position.go` | PriceGapPosition struct + ExitReason enum | VERIFIED | Contains `type PriceGapPosition struct`, all 5 ExitReason constants including ExitReasonReverted, ExitReasonMaxHold, ExitReasonRiskGate, ExitReasonExecQuality |
| `internal/models/pricegap_interfaces.go` | PriceGapStore DI interface + PriceGapCandidate + SlippageSample | VERIFIED | All three types present; interface has 11 methods |
| `internal/config/config.go` | 11 PriceGap* fields + applyJSON mapping | VERIFIED | Fields at lines 268-277; applyJSON at lines 1303-1330; PriceGapEnabled defaults false |
| `internal/pricegaptrader/detector.go` | BBO-sampled 1m bar detector | VERIFIED | 4-slot barRing, `detectOnce()`, staleness check |
| `internal/pricegaptrader/risk_gate.go` | 5-check pre-entry gate | VERIFIED | `preEntry()` with concentration, concurrent, notional, delist/stale checks |
| `internal/pricegaptrader/execution.go` | Simultaneous IOC entry + unwind-to-match + circuit breaker | VERIFIED | Goroutine pair for both legs, circuit breaker counter, MARKET fallback for remainder |
| `internal/pricegaptrader/monitor.go` | Per-position exit monitor + PnL | VERIFIED | `startMonitor()`, `checkAndMaybeExit()`, `closePair()`, realized PnL math |
| `internal/pricegaptrader/rehydrate.go` | Redis rehydration on startup | VERIFIED | Re-enrolls active positions, orphan detection |
| `internal/pricegaptrader/slippage.go` | Exec-quality auto-disable | VERIFIED | 10-sample window, 2× threshold, `SetCandidateDisabled()` |
| `internal/pricegaptrader/tracker.go` | Tracker orchestrator + tickLoop | VERIFIED | `NewTracker()`, `Start()`, `Stop()`, tick loop wiring |
| `internal/database/pricegap_state.go` | Redis persistence under pg:* namespace | VERIFIED | `pg:positions` HSET, `pg:positions:active` SET, `pg:history` LIST, compile-time `var _ models.PriceGapStore = (*Client)(nil)` |
| `cmd/pg-admin/main.go` | Redis-only admin CLI | VERIFIED | `status`, `enable`, `disable`, `positions list` subcommands; comment: "config.json is NEVER written" |
| `cmd/main.go` | Conditional startup + D-03 shutdown order | VERIFIED | `if cfg.PriceGapEnabled { pgTracker.Start() }` at line 468; shutdown at line 487 before spotEng (line 494) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cmd/main.go` | `pricegaptrader.Tracker` | `if cfg.PriceGapEnabled` conditional | WIRED | Lines 467-474; import at line 19 |
| `pricegaptrader` | `models.PriceGapStore` | DI injection in `NewTracker` | WIRED | Tracker holds store interface; `database.Client` satisfies it via compile-time assertion |
| `pricegaptrader` | `pkg/exchange` adapters | Exchange map passed to `NewTracker` | WIRED | `execution.go` looks up `t.exchanges[cand.LongExch]` |
| `database.Client` | `models.PriceGapStore` | compile-time assertion | WIRED | `var _ models.PriceGapStore = (*Client)(nil)` line 35 |
| `pricegaptrader` | `internal/engine` or `internal/spotengine` | D-02 isolation | NOT WIRED (correct) | Only a comment references those packages; no import found |
| `monitor.go` | `slippage.go` | `recordSlippageAndMaybeDisable()` call after close | WIRED | Called in closePair exit path |
| `rehydrate.go` | `startMonitor()` | re-enrollment on startup | WIRED | Calls `t.startMonitor(pos)` for each active position |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|--------------------|--------|
| `detector.go` | `barRing` spread samples | `exchange.GetBBO()` on both legs | Yes — live BBO from exchange adapters | FLOWING |
| `execution.go` | fill price / fill qty | `PlaceOrder()` + `GetOrderFilledQty()` | Yes — exchange adapter returns real fill | FLOWING |
| `monitor.go` | current spread for exit check | `exchange.GetBBO()` polled in tick | Yes — live BBO | FLOWING |
| `database/pricegap_state.go` | position JSON | Redis HGET/HSET | Yes — real Redis queries | FLOWING |
| `slippage.go` | slippage window | `GetSlippageWindow()` from Redis LIST | Yes — real Redis LRange | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| pricegaptrader package builds | `go build ./internal/pricegaptrader/` | Cannot run offline | SKIP — needs go toolchain |
| PriceGapStore compile-time assertion | `grep "var _ models.PriceGapStore = " internal/database/pricegap_state.go` | Found at line 35 | PASS (static) |
| D-02 boundary: no engine/spotengine import | `grep -r "internal/engine\|internal/spotengine" internal/pricegaptrader/*.go` | Only comment in tracker.go | PASS (static) |
| Default OFF: PriceGapEnabled zero-value | `grep "PriceGapEnabled.*bool" internal/config/config.go` | Found at line 268 — bool defaults false | PASS (static) |
| pg-admin Redis-only | `grep "config.json" cmd/pg-admin/main.go` | "config.json is NEVER written" comment only | PASS (static) |
| D-03 shutdown order | pgTracker.Stop() before spotEng.Stop() | Lines 487 vs 494 | PASS (static) |

### Requirements Coverage

| Requirement | Description | Status | Evidence |
|------------|-------------|--------|----------|
| PG-01 | 1m kline detector with ≥4-bar persistence filter | SATISFIED | `detector.go` barRing, `allExceed(T)`, 4-bar minimum |
| PG-02 | IOC delta-neutral 2-leg entry via existing adapters | SATISFIED | `execution.go` goroutine pair, `PlaceOrder(..., "ioc", ...)` |
| PG-03 | Exit on spread ≤T/2 OR 4h max-hold | SATISFIED | `monitor.go` `reversionFactor` + `maxHold` checks |
| PG-04 | Redis persistence under pg:* namespace, survive restart | SATISFIED | `pricegap_state.go` pg:positions HSET; `rehydrate.go` startup re-enrollment |
| PG-05 | Candidate list configurable via config.json | SATISFIED | `PriceGapCandidates []models.PriceGapCandidate` in config; `applyJSON` mapping |
| PG-RISK-01 | Gate concentration cap ≤50% of PriceGapBudget | SATISFIED | `risk_gate.go` preEntry Gate check; `risk_gate_test.go` TestRiskGate_GateConcentration_* |
| PG-RISK-02 | Delist/halt/stale kline pre-entry block | SATISFIED | `risk_gate.go` delist + staleness check; `risk_gate_test.go` TestRiskGate_Delisted, TestRiskGate_StaleBBO |
| PG-RISK-03 | Exec-quality auto-disable after 10 trades with 2× slippage | SATISFIED | `slippage.go` + `slippage_test.go` boundary tests |
| PG-RISK-04 | Max 3 concurrent positions cap | SATISFIED | `risk_gate.go` TestRiskGate_MaxConcurrent_AtLimit |
| PG-RISK-05 | Per-candidate notional cap from config | SATISFIED | `risk_gate.go` per-position cap check using `cand.MaxPositionUSDT` |
| PG-OPS-06 | `PriceGapEnabled` default OFF, round-trip via API | SATISFIED | `cmd/main.go:468` conditional; bool zero-value = false; config applyJSON pattern |

**All 11 Phase 8 requirements: SATISFIED**

### Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| `fakes_test.go` | Multiple `return nil` | Info | Test stubs only — not production code paths |
| `execution.go:164` | `return nil` on nil result | Info | Legitimate circuit-breaker early return, not a stub |

No production stubs, hardcoded empty data, or TODO/FIXME found in non-test files.

### Human Verification Required

#### 1. Full Repo Build

**Test:** Run `go build ./...` from repo root
**Expected:** Zero compile errors; binary produced
**Why human:** Cannot execute go toolchain in sandbox. The compile-time assertion `var _ models.PriceGapStore = (*Client)(nil)` strongly signals correctness, but full build must confirm no import cycles or missing symbols.

#### 2. Unit Test Suite

**Test:** Run `go test ./internal/pricegaptrader/... ./internal/database/... -race -count=1`
**Expected:** All tests pass: detector (3), execution (multiple), monitor (6), rehydrate (3), risk_gate (13+), slippage (5), tracker, pricegap_state
**Why human:** pricegap_state_test.go requires a live Redis instance (DB 2).

#### 3. Default-OFF Regression Test

**Test:** Start the binary with default config (PriceGapEnabled absent or false); observe startup log and engine behavior for 5 minutes
**Expected:** Log shows "Price-gap tracker disabled"; perp-perp scan cycle fires at :40; no new Redis keys under `pg:*`; spot-futures engine unaffected
**Why human:** Requires live binary + Redis + exchange connectivity.

#### 4. Enabled Startup Smoke Test

**Test:** Set `"price_gap": {"enabled": true, "budget": 100, "candidates": [...]}` in config, restart binary
**Expected:** Log shows "Price-gap tracker started (candidates=N budget=100)"; tickLoop fires at PollIntervalSec intervals; no panics; existing engines unaffected
**Why human:** Requires live binary execution with config edit.

### Gaps Summary

No automated gaps found. All 5 success criteria verified against actual codebase. All 11 requirements have implementation evidence. Module boundary (D-02), default-OFF (PG-OPS-06), startup/shutdown order (D-03), and Redis-only admin (D-20) all confirmed correct.

Human verification is required to confirm compile success and test-suite green status before declaring the phase fully passed.

---

_Verified: 2026-04-21_
_Verifier: Claude (gsd-verifier)_
