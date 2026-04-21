---
phase: 08-price-gap-tracker-core
plan: 03
subsystem: pricegaptrader
tags: [pricegap, strategy4, tracker, detector, bbo, pg-01]

requires:
  - PriceGapPosition / PriceGapCandidate / SlippageSample (from 08-01)
  - PriceGapStore interface (from 08-01)
  - Config fields PriceGapEnabled / PriceGapBudget / PriceGapBarPersistence / PriceGapKlineStalenessSec / PriceGapCandidates (from 08-01)
provides:
  - internal/pricegaptrader package skeleton (Tracker + NewTracker + Start/Stop)
  - BBO 1m bar aggregator (barRing, 4-slot fixed circular buffer)
  - 4-bar persistence detector with same-sign invariant (PG-01)
  - DelistChecker interface (added to internal/models) for cross-module DI
  - stubExchange + fakeClock shared test fakes for all Phase 8 subsequent plans
affects: [08-04, 08-05, 08-06, 08-07, 08-08]

tech-stack:
  added: []
  patterns:
    - "In-memory rolling-window detector (fixed barRing, T-08-10 DoS mitigation)"
    - "Compile-time interface assertion (var _ exchange.Exchange = (*stubExchange)(nil))"
    - "Same-sign persistence invariant (T-08-11 mitigation: threshold-cross-and-revert cannot fire)"
    - "DI via two small interfaces (PriceGapStore + DelistChecker) to preserve D-02 module boundary"

key-files:
  created:
    - internal/pricegaptrader/tracker.go
    - internal/pricegaptrader/detector.go
    - internal/pricegaptrader/fakes_test.go
    - internal/pricegaptrader/detector_test.go
  modified:
    - internal/models/pricegap_interfaces.go

key-decisions:
  - "DelistChecker interface added to internal/models — satisfied by *discovery.Scanner.IsDelisted; keeps pricegaptrader boundary clean (no internal/discovery import)"
  - "BBO has no UpdatedAt field — staleness gate repurposed to wall-clock interval between successive successful samples per candidate. When the gap exceeds PriceGapKlineStalenessSec the ring is reset and result reports stale_bbo"
  - "candidateBars forward-stub in tracker.go during Task 1 removed in Task 2 (detector.go has the real definition)"
  - "stubExchange uses compile-time interface assertion so any future pkg/exchange method addition breaks this file first"

patterns-established:
  - "pricegaptrader module boundary verified: zero imports of internal/engine or internal/spotengine (grep asserted)"
  - "Tracker.tickLoop is a stub draining stopCh — Plan 05 wires the real dispatch"
  - "Shared test helpers (newDetectorTestTracker, setMids, pushBarsAt250Bps, defaultCandidate) reusable by 04/05/06 tests"

requirements-completed: [PG-01]

duration: 15min
completed: 2026-04-21
---

# Phase 08 Plan 03: Tracker skeleton + BBO detector (PG-01) Summary

**Wave-3 core: internal/pricegaptrader package established with Tracker lifecycle + 4-bar persistence detector. PG-01 event detection fires only when four consecutive same-sign 1m bars exceed the per-candidate threshold and BBO samples are fresh. Module boundary (D-02) enforced; no imports of internal/engine or internal/spotengine.**

## Performance

- **Duration:** ~15 min
- **Tasks:** 4
- **Files created:** 4
- **Files modified:** 1

## Accomplishments

- `internal/pricegaptrader/tracker.go` — Tracker struct (exchanges map, PriceGapStore, DelistChecker, config, logger, stopCh, wg, exitWG, bars map, monitors map, circuit breaker state, entry-in-flight map); `NewTracker` constructor; `Start`/`Stop` lifecycle with goroutine wait-group; tickLoop stub draining stopCh
- `internal/pricegaptrader/detector.go` — barRing (fixed 4-slot circular buffer), push-with-dedup, allExceed with same-sign invariant, computeSpreadBps implementing D-06 formula, sampleLegs, detectOnce with freshness gate + persistence gate + reason diagnostics
- `internal/pricegaptrader/fakes_test.go` — stubExchange implementing full 41-method Exchange interface with compile-time assertion; fakeClock with Now/Advance
- `internal/pricegaptrader/detector_test.go` — 8 tests (5 barRing unit + 3 detectOnce integration), all pass under -race
- `internal/models/pricegap_interfaces.go` — DelistChecker interface added (satisfied by *discovery.Scanner)

## Task Commits

1. **Task 1: Tracker skeleton + DelistChecker interface** — `65099ef` (feat)
2. **Task 2: BBO bar aggregator + 4-bar persistence detector** — `839f9b8` (feat)
3. **Task 3: stubExchange + fakeClock shared test fakes** — `c4a35c0` (test)
4. **Task 4: 8 detector unit tests (PG-01)** — `90c6d1b` (test)

_Plan metadata commit (SUMMARY + state + roadmap) follows this summary._

## Files Created/Modified

- `internal/pricegaptrader/tracker.go` (created, 117 lines) — Tracker + NewTracker + Start/Stop + tickLoop stub; Option A design rationale in file header
- `internal/pricegaptrader/detector.go` (created, 170 lines) — barRing, candidateBars, computeSpreadBps, sampleLegs, detectOnce
- `internal/pricegaptrader/fakes_test.go` (created, 163 lines) — full Exchange interface stub with compile-time assertion + fakeClock
- `internal/pricegaptrader/detector_test.go` (created, 178 lines) — 8 tests covering push dedup, allExceed empty/below/mixed-sign/pass, detectOnce insufficient/stale/fire
- `internal/models/pricegap_interfaces.go` (modified, +6 lines) — DelistChecker interface

## Decisions Made

- **DelistChecker interface added to internal/models.** The plan specified `models.DelistChecker` but the interface didn't exist in Plan 01's scope. Added the minimal `IsDelisted(symbol string) bool` contract — exactly matching `*discovery.Scanner.IsDelisted`. Keeps the D-02 boundary intact (pricegaptrader still has no internal/discovery import).
- **Wall-clock staleness gate (not BBO timestamp).** `pkg/exchange.BBO` has only `Bid` and `Ask` fields — no `UpdatedAt`. The plan's sample code referenced `BidPrice`, `AskPrice`, `UpdatedAt` which don't exist. I preserved the freshness semantics by measuring the wall-clock interval between successive successful samples for a candidate; gap ≥ `PriceGapKlineStalenessSec` resets the ring and reports `stale_bbo`. This is tighter than a BBO-timestamp check in some ways (picks up detector-side stalls too) and looser in others (a paused tick loop looks the same as a paused stream). Acceptable for Phase 8 scope; if future work adds timestamps to BBO we can switch.
- **candidateBars forward-stub in Task 1.** Task 1's acceptance criteria require `go build ./internal/pricegaptrader/...` to succeed in isolation, but Tracker references `*candidateBars` which is defined in detector.go (Task 2). Added an empty placeholder type in tracker.go to close the Task 1 build; Task 2 removed the stub when the real detector.go landed.
- **Compile-time Exchange interface assertion.** `var _ exchange.Exchange = (*stubExchange)(nil)` forces any future interface addition (e.g., if a `GetKlines` method were later added) to fail the stub build rather than silently letting tests pass a half-implemented fake.

## Deviations from Plan

**[Rule 1 — Fix] BBO type mismatch.** The plan's detector/stub sample code referenced `BBO.BidPrice`, `BBO.AskPrice`, and `BBO.UpdatedAt`. Actual type in `pkg/exchange/types.go:170` has only `Bid` and `Ask` — no timestamp field. Fixed by:
- Using `BBO.Bid` / `BBO.Ask` throughout detector.go and fakes_test.go
- Repurposing the freshness gate to wall-clock interval between samples (see Decisions). Design decision locked in detector.go file header comment.

**[Rule 3 — Fix] DelistChecker interface missing from models.** Plan 01 created `PriceGapStore` but not `DelistChecker`; Plan 03 constructor signature needs it for DI. Added the minimal interface to `internal/models/pricegap_interfaces.go` in Task 1 (six-line addition; no breakage to prior callers).

**[Minor] Fake PlaceOrder lock release.** To support `placeOrderFn` callbacks that might re-enter stubExchange, I release/reacquire the mutex around the user function call. Defensive; avoids a latent deadlock for Plan 05 tests.

No architectural (Rule 4) changes. No changes to config.json. No npm commands run.

## Issues Encountered

- BBO type mismatch caught at first build of Task 2 (would have been caught by graphify had I routed first). Minor rework.
- No test infrastructure changes needed — `go test` works out-of-the-box on the existing module.

## Verification

- `go build ./...` — succeeds across the entire repo (not just pricegaptrader)
- `go vet ./internal/pricegaptrader/...` — clean
- `go test ./internal/pricegaptrader/... -race -count=1` — 8/8 tests pass
- `go test ./internal/pricegaptrader/... -race -count=1 -run 'TestBarRing|TestDetectOnce'` — all 8 named tests pass
- D-02 boundary check — `grep -rnE '"(arb/)?internal/(engine|spotengine)"' internal/pricegaptrader/` returns zero matches
- Exchange interface compile-time assertion `var _ exchange.Exchange = (*stubExchange)(nil)` present in fakes_test.go:32
- Per-criteria grep counts all match spec (type Tracker: 1, NewTracker: 1, Start: 1, Stop: 1, PriceGapStore: 1, DelistChecker: 1, barRing: 1, computeSpreadBps: 1, allExceed: 1, firstSign: 3, PriceGapKlineStalenessSec: 1 in code + 1 in doc, TestBarRing_*: 5, TestDetectOnce_*: 3)
- CLAUDE.local.md honored — config.json untouched, no npm install

## Threat Flags

None. The threat surface is fully covered by the plan's threat model:
- T-08-09 (BBO freshness tampering) — mitigated via wall-clock staleness gate
- T-08-10 (unbounded barRing) — mitigated via fixed 4-slot array
- T-08-11 (spoofing via threshold-crossing reversion) — mitigated via same-sign check
- T-08-12 (cross-module boundary leak) — mitigated via grep assertion + DI interfaces

No new attack surface introduced beyond the plan scope.

## Next Plan Readiness

- **Plan 04 (risk gates)** can import `internal/pricegaptrader` test fakes (stubExchange) for entry-path unit tests; consume `DetectionResult` from detectOnce
- **Plan 05 (entry executor)** can wire `tickLoop` dispatch to `detectOnce → risk gates → PlaceOrder` using the already-exposed PlaceOrder path on stubExchange
- **Plan 06 (monitor/exit)** can spawn per-position goroutines via the existing `monitors` map and cancel them via Tracker.Stop's cancel-all loop
- **Plan 07 (wiring)** has the constructor signature frozen: `NewTracker(exchanges, store, delist, cfg)`; main.go wires `*database.Client` and `*discovery.Scanner` into those slots

## Self-Check: PASSED

- `internal/pricegaptrader/tracker.go` — FOUND
- `internal/pricegaptrader/detector.go` — FOUND
- `internal/pricegaptrader/fakes_test.go` — FOUND
- `internal/pricegaptrader/detector_test.go` — FOUND
- `internal/models/pricegap_interfaces.go` DelistChecker — FOUND
- Commit `65099ef` — FOUND on main
- Commit `839f9b8` — FOUND on main
- Commit `c4a35c0` — FOUND on main
- Commit `90c6d1b` — FOUND on main
- `go build ./...` — PASS
- `go test ./internal/pricegaptrader/... -race -count=1` — PASS (8/8)

---
*Phase: 08-price-gap-tracker-core*
*Completed: 2026-04-21*
