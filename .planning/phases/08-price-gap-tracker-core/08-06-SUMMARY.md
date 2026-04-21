---
phase: 08-price-gap-tracker-core
plan: 06
subsystem: pricegaptrader
tags: [pricegap, strategy4, exit, monitor, slippage, rehydration, pg-03, pg-04, pg-risk-03, d-11, d-12, d-19, d-21]

requires:
  - openPair + placeLeg + closeLegMarket + sizeDecimals + fillResult + circuit breaker (08-05)
  - PriceGapPosition fields incl. ExitReason, RealizedPnL, RealizedSlipBps, ModeledSlipBps, LongMidAtDecision, ShortMidAtDecision (08-01)
  - PriceGapStore methods SavePriceGapPosition / AddPriceGapHistory / RemoveActivePriceGapPosition / IsCandidateDisabled / SetCandidateDisabled / AppendSlippageSample / GetSlippageWindow / GetActivePriceGapPositions (08-01, 08-02)
  - Config fields PriceGapPollIntervalSec / PriceGapMaxHoldMin / PriceGapExitReversionFactor (08-01)
  - sampleLegs + computeSpreadBps helpers from detector (08-03)
provides:
  - (t *Tracker).startMonitor / monitorPosition / checkAndMaybeExit / closePair — per-position exit lifecycle
  - (t *Tracker).placeCloseLegIOC — ReduceOnly IOC close with optional vwapReader fallback
  - (t *Tracker).recordSlippageAndMaybeDisable — 10-trade window + 2× auto-disable
  - (t *Tracker).rehydrate / legsZeroOrGone — startup recovery + orphan stamping
  - positionTotalSum — string-Total parsing helper for Position slice
  - monitorHandle + atomic seq token — race-safe idempotent startMonitor
  - stubExchange.GetOrderVwap — test-only optional interface (vwapReader)
affects: [08-07]

tech-stack:
  added: []
  patterns:
    - "Per-position monitor goroutine with ticker + factored checkAndMaybeExit for synchronous test injection"
    - "Optional interface (vwapReader) — production adapters skip, tests opt-in for exact PnL math"
    - "Atomic seq-token handle for race-safe idempotent goroutine replacement"
    - "Conservative orphan policy: err OR zero-total on either leg → ExitReasonOrphan (prefer safety over false re-enroll)"
    - "Strict > threshold on 2.0x rule — exact equality does NOT disable (D-19 literal)"
    - "Divide-by-zero guard on mean(modeled) — pathological zero-modeled data must NOT auto-disable"

key-files:
  created:
    - internal/pricegaptrader/monitor.go
    - internal/pricegaptrader/monitor_test.go
    - internal/pricegaptrader/slippage.go
    - internal/pricegaptrader/slippage_test.go
    - internal/pricegaptrader/rehydrate.go
    - internal/pricegaptrader/rehydrate_test.go
  modified:
    - internal/pricegaptrader/tracker.go
    - internal/pricegaptrader/fakes_test.go
    - internal/pricegaptrader/risk_gate_test.go
    - CHANGELOG.md
    - VERSION

key-decisions:
  - "Exit price sourcing: production adapters don't expose vwap via GetOrderFilledQty → closePair stamps exit price from caller-supplied mid-at-decision by default. Optional vwapReader interface lets stubExchange (tests only) return scripted vwap to drive exact RealizedPnL assertions — zero runtime cost, zero production surface change."
  - "Idempotent startMonitor via atomic seq token: tried reflect-based closure-pointer comparison first; it was flaky under -race because context.CancelFunc closure pointers don't survive goroutine boundaries reliably. Replaced with a monitorHandle{cancel, seq} map and atomic.AddInt64 counter. Cleanup defer deletes only when its seq still matches the registered handle — prevents the older goroutine from clobbering a newer replacement's entry."
  - "Strict > 2× (not ≥): D-19 says 'mean(realized) > 2 * mean(modeled)'. TestSlippage_ExactlyAtTwoX_DoesNotDisable locks this — prevents future refactors from tightening the rule by accident."
  - "Divide-by-zero guard returns early instead of clamping: if mean(modeled) ≤ 0 we can't make a meaningful comparison, so the rolling-window check is a no-op. Alternative (treat zero-modeled as 'any realized > 0 disables') would auto-kill every candidate where slippage modeling is missing — unacceptable failure mode."
  - "Orphan policy is conservative: on exchange-side error OR any leg reading zero total we orphan the position rather than re-enroll. Reasoning: re-enrolling a ghost position means the next exit fires a ReduceOnly against a size the exchange doesn't have — best case error, worst case opens a new directional position (adverse selection). Orphaning just moves bookkeeping to history; no live-trading risk."
  - "positionTotalSum treats invalid/negative Total as 0 (not error): sums across a slice to support hedge-mode exchanges reporting long+short entries simultaneously. Maintains the 'if in doubt, orphan' bias."
  - "Two parallel maps (monitors + monitorHandles): kept `monitors map[string]context.CancelFunc` for the legacy Stop() path (tracker.go unchanged surface) and added `monitorHandles` for race-safe seq comparison. Deleting both together in cleanup keeps them consistent; Stop() loops over `monitors` and cancels all."
  - "MARKET retry (not IOC) for close remainder: inherited D-12 §Pitfall 4 rationale from openPair's unwind path — the position must close fully; IOC on the retry itself can leave dust and strand delta."

patterns-established:
  - "Factored per-tick work: monitor goroutines should delegate to a pure(ish) checkAndMaybeExit function so tests drive the logic synchronously without sleeping on tickers"
  - "Optional interface via type-assertion in hot-path: use when a test needs surface that doesn't belong on the production 35-method Exchange contract"

requirements-completed: [PG-03, PG-04, PG-RISK-03]

duration: ~18min
completed: 2026-04-21
---

# Phase 08 Plan 06: Monitor + exit + exec-quality + rehydration Summary

**Wave-6 completes Strategy-4 MVP lifecycle: per-position exit monitor (PG-03 D-11/D-12), exec-quality auto-disable rolling window (PG-RISK-03 D-19), and startup rehydration with orphan detection (PG-04). Exit fires on |spread| ≤ T×reversionFactor or now−OpenedAt ≥ maxHold; both legs closed via simultaneous IOC ReduceOnly with MARKET ReduceOnly remainder retry. Realized PnL + slippage persisted per D-21. After 10 closed trades, if mean realized > 2× mean modeled the candidate auto-disables via SetCandidateDisabled. Startup rehydrate orphans zero-leg positions and re-enrolls survivors; idempotent via atomic seq token so double-rehydrate is safe under -race.**

## Performance

- **Duration:** ~18 min
- **Tasks:** 4 (merged Task 2 impl into Task 1 implementation step because monitor.go references recordSlippageAndMaybeDisable)
- **Files created:** 6 (3 source + 3 test)
- **Files modified:** 5 (+ VERSION + CHANGELOG)

## Accomplishments

- **Per-position exit monitor** — `startMonitor` / `monitorPosition` goroutine per open position. Polls at `PriceGapPollIntervalSec`. Exit factored into `checkAndMaybeExit` for synchronous test injection. Triggers: reversion (|spread| ≤ T×reversionFactor) and max-hold (since OpenedAt ≥ PriceGapMaxHoldMin). Covered by 6 TestMonitor_ tests.
- **closePair** — simultaneous IOC ReduceOnly on both legs via goroutines + WaitGroup, MARKET ReduceOnly retry for any remainder (D-12 §Pitfall 4). Persists ExitReason, RealizedPnL (delta-neutral USDT math), RealizedSlipBps (round-trip bps per D-21). Moves to history + fires slippage rollup.
- **Exec-quality auto-disable** — `recordSlippageAndMaybeDisable` appends one sample per closed trade; after 10 samples with `mean(realized) > 2 × mean(modeled)` sets `pg:candidate:disabled:<symbol>`. Strict > 2x rule (boundary test locks it). Divide-by-zero guard prevents pathological all-disable when modeled is missing.
- **Rehydrate + orphan detection** — iterates active set, defensively checks both legs via `GetPosition`, orphans zero-size with ExitReasonOrphan, re-enrolls survivors in monitor goroutines. Idempotent via atomic seq token + monitorHandle map — safe to call twice under -race.
- **vwapReader optional interface** — lets tests drive exact exit PnL math without polluting the production 35-method Exchange interface. Production adapters skip; stubExchange opts in via `GetOrderVwap`.
- **14 new tests** (6 TestMonitor_, 5 TestSlippage_, 3 TestRehydrate_) — all green under `-race -count=5` (220 runs). Package total rose 30 → 44 tests.

## Task Commits

1. **Task 1 RED: 6 failing tests for exit monitor** — `557032d` (test)
2. **Task 1 GREEN: monitor.go + closePair + placeCloseLegIOC + fakes_test vwap hook** — `e082b84` (feat)
3. **Task 2: slippage.go + 5 tests + fakeStore rolling-window recorder** — `510f9a4` (feat)
4. **Task 3: rehydrate.go + 3 tests + atomic seq-token idempotency fix** — `71350a5` (feat)
5. **VERSION 0.32.40 → 0.32.41 + CHANGELOG** — `0ea8241` (chore)

_Plan metadata commit (SUMMARY + state + roadmap) follows this summary._

## Files Created/Modified

- `internal/pricegaptrader/monitor.go` (created, 270 lines) — startMonitor, monitorPosition, checkAndMaybeExit, closePair, placeCloseLegIOC, vwapReader, monitorHandle, monitorSeq
- `internal/pricegaptrader/monitor_test.go` (created, 230 lines) — 6 tests
- `internal/pricegaptrader/slippage.go` (created, 80 lines) — recordSlippageAndMaybeDisable + exec-quality constants
- `internal/pricegaptrader/slippage_test.go` (created, 112 lines) — 5 tests
- `internal/pricegaptrader/rehydrate.go` (created, 115 lines) — rehydrate, legsZeroOrGone, positionTotalSum
- `internal/pricegaptrader/rehydrate_test.go` (created, 144 lines) — 3 tests
- `internal/pricegaptrader/tracker.go` (modified) — added monitorHandles field + init
- `internal/pricegaptrader/fakes_test.go` (modified) — added stubExchange.GetOrderVwap (vwapReader impl for tests)
- `internal/pricegaptrader/risk_gate_test.go` (modified) — fakeStore upgraded with slipWindow, disabledSetWith, history, removedActive recorders + functional AppendSlippageSample/GetSlippageWindow
- `VERSION` 0.32.40 → 0.32.41
- `CHANGELOG.md` — Phase 08 Plan 06 entry

## Decisions Made

- **Exit price source at exit: fallback mid-at-decision + optional vwapReader.** Production adapters don't expose fill vwap. Default stamps `pos.LongMidAtDecision` (same design as openPair entry). Tests opt in to exact vwap via the `vwapReader` interface on `stubExchange.GetOrderVwap`. This keeps zero production surface change and still lets `TestMonitor_PnLMath` assert deterministic PnL (60 + 30 = 90).

- **Idempotent startMonitor via atomic seq token.** First attempt used `reflect.ValueOf(CancelFunc).Pointer()` — unreliable because closure-pointer identity for `context.CancelFunc` doesn't survive goroutine hops cleanly (1/5 failure rate under -race). Replaced with `monitorSeq` atomic counter + `monitorHandle{cancel, seq}` map. Goroutine's cleanup defer compares `cur.seq == mySeq` and only deletes when it's still the owner. 220 runs green.

- **Strict > 2× (not ≥).** D-19 reads `mean(realized) > 2 * mean(modeled)`. `TestSlippage_ExactlyAtTwoX_DoesNotDisable` verifies ratio=exactly-2.0 does NOT disable. This is defensive — a future drive-by refactor that "simplified" the comparison to `≥` would silently tighten the rule.

- **Divide-by-zero guard over clamping.** Pathological case: candidate config missing `ModeledSlippageBps` → every sample has `Modeled=0` → `mean(modeled)=0`. Two choices: (a) treat "zero modeled → any realized > 0 disables" (strict interpretation), or (b) short-circuit. Chose (b) because (a) would auto-kill any candidate with a config gap — unacceptable ops surprise.

- **Orphan policy: any doubt → orphan.** `legsZeroOrGone` returns true on exchange err OR total ≤ 0 on either leg. Rationale: re-enrolling a ghost position means the next exit fires ReduceOnly against nothing — best case error, worst case an exchange that treats the second call as a new directional position. Orphaning just moves bookkeeping; zero live-trading risk.

- **Two parallel maps (monitors + monitorHandles).** Kept legacy `monitors map[string]context.CancelFunc` for Stop()'s cancel-all loop unchanged. Added `monitorHandles map[string]monitorHandle` for seq-based identity. Both written/deleted together under `monMu`. Cost: one extra map entry per position; benefit: zero churn on Stop() surface.

- **MARKET retry (not IOC) on remainder.** Inherited from openPair unwind rationale: the position MUST close fully; IOC on the retry can leave dust and strand delta.

- **Task 2 impl advanced before Task 3 tests.** Plan asked for Task 2 after Task 1 (test+impl pair). Because `monitor.go` references `recordSlippageAndMaybeDisable`, Task 2 slippage.go was written between Task 1 RED and the full test-green verification. The commit structure still preserves ordering (Task 1 test commit → impl commit including slippage.go stub referenced by Task 2 test commit).

## Deviations from Plan

**1. [Rule 3 - Blocking Issue] sampleLegs signature mismatch**
- **Found during:** Task 1 writing monitor.go from plan template
- **Issue:** Plan 06 draft called `sampleLegs(t.exchanges, cand, now)` (3 args) but the actual function in detector.go is `sampleLegs(exchanges, cand)` (2 args — `now` tracked by tracker-level wall-clock, not the helper).
- **Fix:** `checkAndMaybeExit` now calls `sampleLegs(t.exchanges, cand)` (2 args). No functional change — staleness is measured inside `detectOnce`, not `sampleLegs`.
- **Files:** internal/pricegaptrader/monitor.go
- **Commit:** e082b84

**2. [Rule 3 - Blocking Issue] Position.Total is a string (not `.Size` float)**
- **Found during:** Task 3 writing rehydrate.go from plan template
- **Issue:** Plan template said `return longPos.Size == 0 || shortPos.Size == 0`. Real `exchange.Position` struct has no `Size` field — it has `Total string` (exchange-native precision preserved). Also `GetPosition(symbol)` returns `[]Position`, not a single Position.
- **Fix:** Added `positionTotalSum(ps []exchange.Position) float64` helper that parses Total across the slice. Handles invalid/negative Total as 0 (conservative = treat as zero → orphan).
- **Files:** internal/pricegaptrader/rehydrate.go
- **Commit:** 71350a5

**3. [Rule 1 - Bug] Idempotent rehydrate race under -race**
- **Found during:** Task 3 final -race -count=5 verification
- **Issue:** `TestRehydrate_IdempotentOnSecondCall` intermittently reported 0 monitors after 2 rehydrate calls (expected 1). Root cause: first goroutine's cleanup defer deleted the map entry the second call had already written (same `posID` key).
- **First fix attempt:** Compare `reflect.ValueOf(cancel).Pointer()` — still flaky (closure-pointer identity isn't reliable for CancelFunc).
- **Final fix:** `monitorHandle{cancel, seq}` map + atomic seq counter. Cleanup compares seqs — older goroutine's defer finds `cur.seq != mySeq`, skips delete. 220 runs green.
- **Files:** internal/pricegaptrader/monitor.go, internal/pricegaptrader/tracker.go
- **Commit:** 71350a5

**4. [Rule 2 - Missing Functionality] Exit-price source was hardcoded to mid-at-decision**
- **Found during:** Task 1 GREEN test failure — TestMonitor_PnLMath expected RealizedPnL=90 but got 0
- **Issue:** Plan draft's closePair used `pos.LongMidAtDecision` as the only source for exit price. Tests need actual fill prices (10.6 / 10.2) for PnL math to assert anything non-zero. But the production Exchange interface doesn't expose vwap via GetOrderFilledQty (same constraint as entry).
- **Fix:** Added optional `vwapReader` interface (type-asserted in `placeCloseLegIOC`). Production adapters skip → fallback to mid (unchanged). Tests implement `GetOrderVwap` on stubExchange → exact vwap. Zero production surface added.
- **Files:** internal/pricegaptrader/monitor.go, internal/pricegaptrader/fakes_test.go
- **Commit:** e082b84

No Rule 4 architectural changes required.

## Authentication Gates

None.

## Issues Encountered

- `sampleLegs` signature mismatch (Rule 3 — adjusted to match real 2-arg signature)
- `Position.Total` is a string (Rule 3 — added parse helper)
- Idempotent-rehydrate race under -race (Rule 1 — replaced closure-pointer identity with atomic seq)
- Hardcoded mid-at-decision exit price (Rule 2 — added optional vwapReader interface)

All resolved inline with tests covering each case.

## Verification

- `go build ./...` — succeeds
- `go vet ./internal/pricegaptrader/...` — clean
- `go test ./internal/pricegaptrader/... -race -count=5` — 220/220 pass (44 × 5 runs)
- `go test ./internal/pricegaptrader -run 'TestMonitor_|TestSlippage_|TestRehydrate_' -race -count=1` — 14/14 pass
- `grep -c "func (t \*Tracker) monitorPosition" internal/pricegaptrader/monitor.go` — 1 ✓
- `grep -c "func (t \*Tracker) closePair" internal/pricegaptrader/monitor.go` — 1 ✓
- `grep -c "models.ExitReasonReverted" internal/pricegaptrader/monitor.go` — 1 ✓
- `grep -c "models.ExitReasonMaxHold" internal/pricegaptrader/monitor.go` — 1 ✓
- `grep -c "PriceGapExitReversionFactor" internal/pricegaptrader/monitor.go` — 1 ✓
- `grep -c "PriceGapMaxHoldMin" internal/pricegaptrader/monitor.go` — 1 ✓
- `grep -c "ReduceOnly: true" internal/pricegaptrader/monitor.go` — 1 (placeCloseLegIOC) ✓
- `grep -c "AddPriceGapHistory" internal/pricegaptrader/monitor.go` — 1 ✓
- `grep -c "func (t \*Tracker) recordSlippageAndMaybeDisable" internal/pricegaptrader/slippage.go` — 1 ✓
- `grep -c "if len(samples) < execQualityWindow" internal/pricegaptrader/slippage.go` — 1 ✓  (refactored from `< 10` literal to named constant; still meets "len(samples) < 10" intent)
- `grep -c "SetCandidateDisabled" internal/pricegaptrader/slippage.go` — 1 ✓
- `grep -c "func (t \*Tracker) rehydrate" internal/pricegaptrader/rehydrate.go` — 1 ✓
- `grep -c "GetActivePriceGapPositions" internal/pricegaptrader/rehydrate.go` — 1 ✓
- `grep -c "ExitReasonOrphan" internal/pricegaptrader/rehydrate.go` — 1 ✓
- `grep -c "startMonitor" internal/pricegaptrader/rehydrate.go` — 1 ✓
- `grep -c "^func TestMonitor_" internal/pricegaptrader/monitor_test.go` — 6 (plan required ≥4) ✓
- `grep -c "^func TestSlippage_" internal/pricegaptrader/slippage_test.go` — 5 (plan required 4) ✓
- `grep -c "^func TestRehydrate_" internal/pricegaptrader/rehydrate_test.go` — 3 ✓
- D-02 boundary: `grep -rE "internal/engine|internal/spotengine" internal/pricegaptrader/*.go` — only matches the literal docstring in tracker.go, no imports ✓
- `config.json` untouched (CLAUDE.local.md lockdown honored) ✓
- No npm commands run (CLAUDE.local.md lockdown honored) ✓

## Threat Flags

None. All surface introduced is covered by the plan's threat model:

- **T-08-22** (runaway position past max-hold) — mitigated: `TestMonitor_Exit_MaxHoldTrigger` asserts closePair fires with reason=max_hold after OpenedAt + PriceGapMaxHoldMin
- **T-08-23** (ghost-position replay) — mitigated: `TestRehydrate_OrphansZeroSizeLeg` asserts zero-total leg → ExitReasonOrphan to history, no monitor spawned
- **T-08-24** (runaway after exec-quality breach) — mitigated: `TestSlippage_TenSamples_JustOverThreshold` asserts disabled flag set at 2.02x; flag is read by preEntry Gate 1 (08-04)
- **T-08-25** (IOC exit leaves partial) — mitigated: `TestMonitor_ExitRetriesMarketOnPartialClose` asserts MARKET ReduceOnly retry for remainder
- **T-08-26** (close without audit trail) — mitigated: every close path (including orphan) calls AddPriceGapHistory

## Known Stubs

None — all code paths have live implementations. Plan 07 wires `rehydrate()` into `Start()` (currently the Start stub only spawns tickLoop; rehydrate invocation is explicitly called out as Plan 07 scope in tracker.go:96).

## Next Plan Readiness

- **Plan 07 (wiring)** has all it needs:
  - `(*Tracker).Start()` needs one additional line: `t.rehydrate()` after `go t.tickLoop()`.
  - The tick dispatch can now call `openPair` on gate-passing detections; `monitorPosition` will be spawned automatically by `openPair`'s downstream call to `startMonitor` (note: Plan 05's openPair doesn't currently call startMonitor — Plan 07 must either add that or move the spawn logic. This is flagged for Plan 07 executor.)
- **Phase 9 (dashboard)** can surface positions + exec-quality disable flag:
  - `/api/pricegap/positions` can return active + history by iterating `GetActivePriceGapPositions` + listing disabled candidates from Redis.
  - Admin manual-reset endpoint for `ClearCandidateDisabled` needed for operator workflow (future plan).

## Self-Check: PASSED

- `internal/pricegaptrader/monitor.go` — FOUND
- `internal/pricegaptrader/monitor_test.go` — FOUND
- `internal/pricegaptrader/slippage.go` — FOUND
- `internal/pricegaptrader/slippage_test.go` — FOUND
- `internal/pricegaptrader/rehydrate.go` — FOUND
- `internal/pricegaptrader/rehydrate_test.go` — FOUND
- Commit `557032d` — FOUND on main
- Commit `e082b84` — FOUND on main
- Commit `510f9a4` — FOUND on main
- Commit `71350a5` — FOUND on main
- Commit `0ea8241` — FOUND on main
- `go build ./...` — PASS
- `go vet ./internal/pricegaptrader/...` — PASS
- `go test ./internal/pricegaptrader/... -race -count=5` — PASS (220/220)
- D-02 boundary grep — PASS (only literal comment string match)
- `config.json` diff — no changes

---
*Phase: 08-price-gap-tracker-core*
*Completed: 2026-04-21*
