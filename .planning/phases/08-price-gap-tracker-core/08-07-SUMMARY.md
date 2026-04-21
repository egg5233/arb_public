---
phase: 08-price-gap-tracker-core
plan: 07
subsystem: pricegaptrader
tags: [pricegap, strategy4, tickloop, dispatch, wiring, main, d-03, d-05, pg-02, pg-03, pg-ops-06]

requires:
  - (t *Tracker).detectOnce + candidateBars ring + freshness gate (08-03)
  - (t *Tracker).preEntry + 6 deterministic gates (08-04)
  - (t *Tracker).openPair + placeLeg + sizeDecimals + circuit breaker (08-05)
  - (t *Tracker).startMonitor + monitorPosition + checkAndMaybeExit + rehydrate (08-06)
  - cfg.PriceGapEnabled / PriceGapPollIntervalSec / PriceGapCandidates / PriceGapBudget (08-01)
  - *discovery.Scanner.IsDelisted satisfying models.DelistChecker (existing)
provides:
  - (t *Tracker).tickLoop — real detect→gate→enter→monitor dispatch at PriceGapPollIntervalSec cadence
  - (t *Tracker).runTick — single-cycle evaluator (extracted for test injection)
  - (t *Tracker).Start — now calls rehydrate() before spawning the tick goroutine
  - chanTick — nil-safe ticker channel helper for startup-offset first tick
  - cmd/main.go conditional tracker wiring — D-03 ordering preserved
affects: [08-08]

tech-stack:
  added: []
  patterns:
    - "Startup offset tick (7s) to avoid Bybit :04–:05:30 blackout on fresh boot — first tick fires off-boundary, subsequent ticks run on steady cadence"
    - "Nil-safe select channel (chanTick) — lets tickLoop select on a ticker that hasn't been constructed yet"
    - "runTick extracted from tickLoop so E2E tests drive one full dispatch cycle synchronously without spawning the goroutine"
    - "Local active-view append after openPair — same-tick back-to-back candidates see updated concurrency for gate math"
    - "Conditional module instantiation: NewTracker never called when cfg.PriceGapEnabled=false → byte-for-byte isolation guarantee"

key-files:
  created:
    - internal/pricegaptrader/tracker_test.go
    - .planning/phases/08-price-gap-tracker-core/deferred-items.md
  modified:
    - internal/pricegaptrader/tracker.go
    - cmd/main.go
    - CHANGELOG.md
    - VERSION

key-decisions:
  - "First-tick offset via time.After(7s) instead of time.NewTicker at boot: RESEARCH §Pitfall 2 noted that a pure ticker spawned at certain minute boundaries could land the first BBO read in the Bybit :04–:05:30 blackout window. Used time.After for the initial fire then a steady time.Ticker for the remainder — subsequent cadence is system-clock-driven, so only the first tick's offset matters."
  - "runTick extracted from tickLoop: tests want to drive one full detect→gate→enter cycle without waiting 7+ seconds or racing the goroutine. Extracting the per-cycle work into runTick keeps tickLoop thin (select-only) and lets TestTrackerE2E_HappyCycle / GateBlocksOnBudget / DisabledFlagBlocks run synchronously in microseconds."
  - "rehydrate BEFORE go tickLoop(): if the tick goroutine spawns first, it races rehydration to read the active set for budget gating — the first tick could see an empty active set and briefly allow over-budget entries until rehydrate finishes. Running rehydrate synchronously in Start() closes that window."
  - "Local active-view append after each openPair in runTick: back-to-back candidates in a single tick (e.g., two pairs both crossing threshold this second) would otherwise each see the pre-tick active set and both pass the budget/max-concurrent/gate-concentration gates. Appending pos to the local slice after a successful openPair gives the next candidate the up-to-date view without a Redis round-trip."
  - "chanTick(nil) returns nil: Go's select blocks forever on a nil channel, which is exactly the startup semantics we want — before the first time.After fires, select only watches stopCh + firstTick. After firstTick, ticker is non-nil and select watches its .C. One nil-safe helper function; no atomic flags, no extra state."
  - "Scanner (not engine or a new wrapper) as DelistChecker: *discovery.Scanner already has `IsDelisted(symbol) bool` and satisfies models.DelistChecker at compile-time. Passing scanner directly into NewTracker preserves D-02 module boundary (pricegaptrader imports only models) while reusing the existing Binance delist monitor."
  - "PG-OPS-06 safety test replicates main.go guard pattern in-process: cmd/main.go is not unit-testable as a binary, so TestPriceGapEnabled_DefaultOff_NoTrackerInstantiated mirrors the `if cfg.PriceGapEnabled` block and asserts zero pg:* miniredis keys afterwards. Gives us the invariant coverage main.go can't provide itself."
  - "Circuit-breaker check at top of runTick (not per-candidate): if 5 consecutive PlaceOrder failures opened the breaker, we skip the entire tick rather than probing each candidate. Prevents the breaker's 'stop trading' semantics from being undermined by optimistic retries across multiple symbols in the same cycle."

patterns-established:
  - "Startup-offset first tick to dodge exchange blackout windows — applies equally to any future minute-boundary-sensitive scheduler"
  - "Extract per-cycle dispatch as a pure(ish) runTick function so E2E tests drive the pipeline synchronously; goroutine itself only manages timing"
  - "Synchronous rehydrate-before-goroutine-spawn in Start() — any subsystem with persisted state must restore it before its first active-set read"
  - "Conditional module instantiation in cmd/main.go with a default-off test that asserts zero side effects — a reusable pattern for every new optional engine"

decisions:
  - summary: "Tracker startup goes AFTER SpotEngine (D-03); shutdown goes BEFORE SpotEngine (reverse order). pgTracker.Stop() must complete while db + exchanges are still live."
  - summary: "PriceGapEnabled=false is the default; flipping it requires editing config.json and restart. Runtime toggle is deferred to Phase 9 dashboard work (T-08-30 accept)."
  - summary: "pg:* namespace is enforced — no tracker code writes to arb:* or arb:spot_*, and with default-off flag no reads happen either."

metrics:
  duration: "~45 minutes"
  completed: "2026-04-21"

threat_model:
  - { id: T-08-27, category: EoP, disposition: mitigate, note: "TestPriceGapEnabled_DefaultOff_NoTrackerInstantiated asserts no pg:* keys when disabled; cmd/main.go guard is the production-side mitigation." }
  - { id: T-08-28, category: DoS, disposition: mitigate, note: "Tracker.Stop() closes stopCh + cancels all monitor CancelFuncs + exitWG.Wait(); pgTracker.Stop() runs before spotEng.Stop() so db/exchanges are live while monitors wind down." }
  - { id: T-08-29, category: Repudiation, disposition: mitigate, note: "Startup logs tracker state (enabled/disabled + candidate count + budget); runTick logs every gate block and every opened position; Stop() logs start+finish." }
  - { id: T-08-30, category: Spoofing, disposition: accept, note: "Config read once at Start(); runtime toggle deferred to Phase 9. In-flight positions continue to close under existing rules if user flips Enabled→false mid-trade." }
---

# Phase 8 Plan 7: Tracker Integration Summary

## One-liner
End-to-end wires the price-gap tracker: real detect→gate→enter→monitor dispatch in tickLoop, rehydrate-before-goroutine-spawn in Start(), and a conditional cmd/main.go integration that respects D-03 startup/shutdown order while guaranteeing byte-for-byte isolation when `PriceGapEnabled=false`.

## What Shipped

### Task 1 — tickLoop + Start rehydrate (commit 87732ba)
- Replaced the stub `tickLoop` (which just drained stopCh) with a real dispatch loop that fires the first tick 7s after startup (Bybit blackout mitigation) then runs at `PriceGapPollIntervalSec` cadence via `time.Ticker`.
- Extracted `runTick(now)` — single-cycle evaluator iterating every `PriceGapCandidate`: `detectOnce` → on fired, `preEntry` → on approved, convert notional USDT to per-leg base-asset size via `mid = (MidLong+MidShort)/2`, call `openPair` + `startMonitor`. Circuit-breaker-aware (skips entire tick when `isCircuitOpen()`).
- Added `chanTick(ticker)` nil-safe helper so `select` can watch a ticker that hasn't been constructed yet (before first startup-offset fire).
- `Start()` now calls `rehydrate()` BEFORE spawning the tick goroutine so restored positions are enrolled in monitors before the first tick reads the active set.

### Task 2 — End-to-end test suite (commit 01fb6c0)
Created `internal/pricegaptrader/tracker_test.go` with 5 E2E tests driving the full pipeline against a miniredis-backed real `*database.Client`:
1. `TestTrackerE2E_HappyCycle` — 4 bars of +253 bps spread → runTick opens position + enrolls monitor + persists to `pg:positions`+`pg:positions:active`; reversion + checkAndMaybeExit closes cleanly with `ExitReasonReverted`.
2. `TestTrackerE2E_GateBlocksOnBudget` — budget=500 < per-position=1000 → fired detection blocked; zero orders on either leg; zero positions persisted.
3. `TestTrackerE2E_DisabledFlagBlocks` — pre-seeded `pg:candidate:disabled:SOON` → Gate 1 blocks; zero orders.
4. `TestTrackerE2E_RehydrationOnRestart` — pre-seeded active position + nonzero exchange positions → `Start()` re-enrolls monitor; wide BBO + long poll interval prevent false reversion during the test.
5. `TestPriceGapEnabled_DefaultOff_NoTrackerInstantiated` — safety property (PG-OPS-06 / T-08-27): miniredis key scan asserts zero `pg:*` writes when `PriceGapEnabled=false`.

All 5 green under `-race -count=1`.

### Task 3 — cmd/main.go wiring (commit 161a685)
- Added import `"arb/internal/pricegaptrader"` to `cmd/main.go`.
- Inserted conditional `NewTracker + Start` block AFTER `spotEng.Start()` (line 467–475 per D-03 startup order); uses `scanner` as the `models.DelistChecker` (satisfied by `*discovery.Scanner.IsDelisted`, no new surface needed).
- Inserted `pgTracker.Stop()` BEFORE `spotEng.Stop()` in the shutdown block (reverse order).
- Default-off branch logs `"Price-gap tracker disabled (cfg.PriceGapEnabled=false)"` — operator can confirm isolation state from logs.

## Verification

| Check | Result |
|-------|--------|
| `go build ./internal/pricegaptrader/...` | PASS |
| `go test ./internal/pricegaptrader -race -count=1` | PASS — all 49 tests green (44 pre-existing + 5 new E2E) |
| `go build ./...` (full repo incl. cmd/main.go) | PASS |
| `go vet` on touched files (`./internal/pricegaptrader/... ./cmd`) | CLEAN |
| 7 acceptance-grep checks (Task 1) | 7/7 PASS |
| 4 acceptance-grep checks + 2 test-name greps (Task 2) | 6/6 PASS |
| 5 acceptance-grep checks + order check (Task 3) | 6/6 PASS |

Line-order invariant verified: `pgTracker = NewTracker` at line 469 (> `spotEng.Start()` at line 452); `pgTracker.Stop()` at line 489 (< `spotEng.Stop()` at line 494).

## Deviations from Plan

**None.** The three tasks executed as written; no Rule 1/2/3 deviations or Rule 4 checkpoints triggered.

## Deferred Issues

- `cmd/gatecheck/main.go:48` `fmt.Println` redundant-newline vet warning is pre-existing (last touched in commit `cba254f`, unrelated to Phase 08). Recorded in `.planning/phases/08-price-gap-tracker-core/deferred-items.md`. Does not affect the price-gap tracker path.

## Follow-ups for Plan 08

- Dashboard UI (`web/`) will need to read `GET /api/pricegap/positions` and surface the enable toggle — the server-side endpoint is still to be added in Plan 08.
- Runtime (not restart-only) PriceGapEnabled toggle deferred to Phase 9 per T-08-30 accept disposition.

## Known Stubs

None — tracker is fully wired end-to-end; all data paths are connected.

## Self-Check: PASSED

- FOUND: internal/pricegaptrader/tracker.go (modified)
- FOUND: internal/pricegaptrader/tracker_test.go (created)
- FOUND: cmd/main.go (modified)
- FOUND: .planning/phases/08-price-gap-tracker-core/deferred-items.md (created)
- FOUND commit 87732ba (Task 1)
- FOUND commit 01fb6c0 (Task 2)
- FOUND commit 161a685 (Task 3)
