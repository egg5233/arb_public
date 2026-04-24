---
phase: 09-price-gap-dashboard-paper-live-operations
plan: 10
subsystem: pricegaptrader
tags: [gap-closure, subscription, bbo, observability]
gap_closure: true
requirements: [PG-OPS-02, PG-OPS-03, PG-OPS-04, PG-OPS-05, PG-VAL-01, PG-VAL-02]
requires:
  - internal/pricegaptrader/tracker.go (post-09-09 state — lastNoFireLogged, shouldLogNoFire, PriceGapDebugLog already present)
  - pkg/exchange.Exchange.SubscribeSymbol (idempotent at adapter layer)
  - pkg/exchange.Exchange.GetBBO (ok=false when no subscription)
provides:
  - internal/pricegaptrader/tracker.go: subscribeCandidates() + subscribeOneLeg() fan-out (fail-soft); assertBBOLiveness() + checkOneLegBBO() (fail-loud) + priceGapBBOAssertionGrace = 15s
  - internal/pricegaptrader/tracker_subscribe_test.go: 10 new tests covering fan-out correctness, fail-soft paths, WARN content, BBO liveness, shutdown-safety
  - internal/pricegaptrader/fakes_test.go: stubExchange.subs counter + optional subscribeFn override + subscribeCount helper
affects:
  - Tracker.Start() gains two new responsibilities before the tick goroutine spawns: subscribe candidates, then later (15s post-boot) emit BBO-liveness WARN for any silent leg. No change to existing openPair / closePair / runTick / rehydrate paths.
tech-stack:
  added: []
  patterns:
    - engine.go:4675-4676 fail-soft SubscribeSymbol pattern (copied inline, no shared helper extracted)
    - utils.Subscribe()/Unsubscribe() log-broadcast subscriber (used in tests for WARN-content assertion)
key-files:
  created:
    - internal/pricegaptrader/tracker_subscribe_test.go (10 tests, 327 lines)
    - .planning/phases/09-price-gap-dashboard-paper-live-operations/09-10-SUMMARY.md
  modified:
    - internal/pricegaptrader/tracker.go (+~95 lines: subscribeCandidates, subscribeOneLeg, priceGapBBOAssertionGrace, assertBBOLiveness, checkOneLegBBO; Start() now calls subscribeCandidates before rehydrate and launches assertBBOLiveness after tickLoop)
    - internal/pricegaptrader/fakes_test.go (+~20 lines: subs, subscribeFn, subscribeCount; SubscribeSymbol now records each call and optionally delegates to subscribeFn)
    - VERSION 0.34.2 → 0.34.3
    - CHANGELOG.md (0.34.3 entry prepended)
decisions:
  - "Combined Task 1 + Task 2 in a single commit. Rationale: both tasks modify the same two files (tracker.go + tracker_subscribe_test.go) and form one logical Gap #2 closure unit per the plan's <objective> (subscription loop + startup assertion together eliminate the silent-failure mode). Splitting would either create an intermediate broken state or require surgical mid-file reverts that leave the file churn noisier than a single-commit diff. Matches recent project norm (09-09 Task 3 shipped dashboard + i18n + VERSION as one commit 6f4de44)."
  - "PriceGapCandidate field names verified: Symbol, LongExch, ShortExch (from internal/models/pricegap_interfaces.go:7-14). No guesses."
  - "Grace = 15s. Rationale: matches the plan's recommended value and is long enough for all 6 adapters' WS handshakes to complete under normal network conditions (typical Binance/Bybit BBO stream first-message latency is 2-4s; 15s gives 3-4x headroom). Not operator-configurable to keep the config surface minimal; promote to a field only if operational experience demands it."
  - "SubscribeSymbol invoked twice when LongExch == ShortExch. Relies on adapter-layer idempotency rather than deduping at the Tracker. Matches engine.go precedent and keeps the tracker-layer logic O(2N) and side-effect-trivial."
  - "stubExchange.SubscribeSymbol still returns true by default (no behavior change for all pre-existing tests). New per-symbol counter + optional subscribeFn only activate when tests explicitly set them."
  - "assertBBOLiveness uses t.wg.Add(1) + defer wg.Done(). Stop() already calls wg.Wait() after close(stopCh), so the goroutine is shutdown-safe without a separate lifecycle. Verified under -race (TestAssertBBOLiveness_StopBeforeGrace)."
  - "No dedup logic added at Tracker level even though repeated Start() calls would stack subs counts on the stub. Rationale: the real Tracker is never Start()ed twice in the same process lifecycle; the only dedup requirement comes from adapter-layer idempotency, which is SubscribeSymbol's documented contract."
  - "cmd/main.go untouched. Plan explicitly called out Option A clean boundary; this keeps the tracker module owner of its own subscription lifecycle and avoids coupling cmd/main.go to yet another per-engine startup step."
  - "VERSION 0.34.2 → 0.34.3. Patch-level bump because the change is a pure bug/gap-closure with no new user-visible config knobs. Grace duration is not a config field."
metrics:
  duration: 30min
  completed: 2026-04-24T12:00:00Z
---

# Phase 9 Plan 10: BBO Subscription Gap Closure Summary

Closes Verification Gap #2 — the zero-caller status of `pkg/exchange.SubscribeSymbol` inside `internal/pricegaptrader/`. `Tracker.Start()` now fans out `SubscribeSymbol` across both leg exchanges for every configured candidate before `rehydrate()` and the first tick, and launches a 15s-delayed BBO-liveness assertion goroutine that WARNs on any leg that silently failed to deliver a BBO. Combined with 09-09 (det.Reason logger), the pricegap tracker no longer has a silent-failure mode for candidates outside the perp-perp scanner's subscription universe.

## Task-by-task outcomes

### Task 1 — subscribeCandidates + Start() wiring

- Added `subscribeCandidates()` helper (iterates `cfg.PriceGapCandidates`, two calls per entry).
- Added `subscribeOneLeg(exchName, symbol, legLabel)` sub-helper that handles the three branches: missing exchange key → WARN, `SubscribeSymbol` returning false → WARN, success → INFO.
- Modified `Start()`: inserted `t.subscribeCandidates()` BEFORE `t.rehydrate()`. Rationale: rehydrate may spawn monitor goroutines that read BBO; those reads only succeed if subscriptions are established first.
- Extended `stubExchange` (in `fakes_test.go`) with `subs map[string]int` counter, optional `subscribeFn func(string) bool` override, and `subscribeCount(symbol)` helper. Default behavior (return true) unchanged so no existing test regressed.
- Tests (7): FanOut (exact per-(exchange, symbol) counts across 2 candidates × 3 exchanges), UnknownExchange (missing bybit → no panic, binance still called), SubscribeFalse (byb rejects, second candidate still attempted on both legs), Empty (zero candidates → zero calls), RestartIdempotent (two subscribeCandidates calls → count 2 each), SameExchBothLegs (LongExch==ShortExch → 2 calls on same exchange), WarnOnMissing (asserts the WARN log line names the missing exchange and candidate symbol via `utils.Subscribe()` capture).

### Task 2 — assertBBOLiveness fail-loud goroutine

- Added `priceGapBBOAssertionGrace = 15 * time.Second` package constant.
- Added `assertBBOLiveness()` goroutine launched from `Start()` via `wg.Add(1)+go`. Select-waits for either the grace timer or `stopCh`; on grace-elapsed, walks every candidate leg and calls `checkOneLegBBO`.
- Added `checkOneLegBBO(exchName, symbol, legLabel)` — silently skips missing exchanges (already WARNed during subscribe), emits `pricegap: BBO NOT LIVE after 15s — SYM@EXCH (LEG leg)` on `ok=false` or zero-price BBO, emits `pricegap: BBO live SYM@EXCH (LEG leg) bid=X ask=Y` on success.
- Tests (3): AllLive (both legs prepopulated → no NOT-LIVE WARN), DeadLeg (only one leg has BBO → NOT-LIVE WARN names the silent leg and symbol), StopBeforeGrace (close stopCh, wg.Wait completes within 2s — guards against goroutine leak under `-race`).

## Log format reference

```
pricegap: subscribed SOONUSDT@binance (long leg)
pricegap: subscribed SOONUSDT@bybit (short leg)
pricegap: subscribe skipped — unknown exchange "okx" for short leg of VICUSDT
pricegap: subscribe returned false for RAVEUSDT@gateio (short leg) — BBO may be unavailable
pricegap: BBO live SOONUSDT@binance (long leg) bid=0.999 ask=1.001
pricegap: BBO NOT LIVE after 15s — VICUSDT@bybit (short leg). Check subscription / WS health.
```

All lines carry the `pricegap:` prefix consistent with the existing module convention (openPair, closePair, gate-blocked, no-fire from 09-09). `.Info` level for successes and `.Warn` for failures so operators on default journalctl see both without extra flags.

## Deviations from plan

None functional. One structural deviation:

### Combined commit instead of per-task commit

**1. [Process deviation — documented]** Tasks 1 and 2 share the same two source files and form one logical unit; splitting would either produce an intermediate broken tree (Task-1 commit references symbols Task-2 introduces) or require surgical partial-staging that makes the diff harder to review. Committed as a single `feat(09-10): Tracker.Start() subscribes candidates + BBO-liveness assertion` commit with both task summaries in the body. Matches the recent project norm (09-09 commit `6f4de44` shipped dashboard + i18n + VERSION bump as one commit).

- **Commit:** `3eec59f`

## Known stubs

None. All new code is wired end-to-end: `Start()` → `subscribeCandidates` → `subscribeOneLeg` → live adapter `SubscribeSymbol`; `Start()` → `assertBBOLiveness` → `checkOneLegBBO` → live adapter `GetBBO`.

## Threat flags

None. No new network endpoint, no auth path, no file access pattern, no schema change. The added log output is `.Info` / `.Warn` to the systemd journal only (no remote shipping). SubscribeSymbol was already called for perp-perp; this plan merely extends existing callers, not the adapter surface.

## Verification against plan's success criteria

- [x] Gap #2 closed: `SubscribeSymbol` now has 2 call sites inside `internal/pricegaptrader/` (via `subscribeOneLeg`), up from zero.
- [x] Subscription ownership inside Tracker (Option A clean boundary); `cmd/main.go` untouched.
- [x] Fail-soft on per-leg failures (WARN + continue per engine.go precedent).
- [x] Fail-loud on post-grace BBO silence (`BBO NOT LIVE` WARN).
- [x] All automated acceptance criteria satisfied (see grep outputs below).
- [x] No import of `internal/engine` or `internal/spotengine` (D-02 boundary preserved).
- [x] `go test ./internal/pricegaptrader/... -count=1 -race -short` green (all 121 tests pass — 111 previous + 10 new).
- [x] `go vet ./internal/pricegaptrader/...` green.
- [x] `go build ./...` green.
- [x] `git diff config.json` empty (CLAUDE.local.md hard rule honored).

## Acceptance-criteria grep evidence

| Criterion | Result |
|-----------|--------|
| `SubscribeSymbol` in tracker.go (≥2 expected) | 5 matches (2 call sites + 3 doc references) |
| `subscribeCandidates` in tracker.go (≥2 expected) | 5 matches (1 def + 1 Start call + 3 doc refs) |
| `subscribeOneLeg` in tracker.go (≥3 expected) | 6 matches (1 def + 2 call sites + 3 doc refs) |
| `pricegap: subscribe` in tracker.go (≥3 expected) | 3 matches (unknown WARN, false-return WARN, success INFO) |
| `assertBBOLiveness` in tracker.go (≥2 expected) | 4 matches (1 def + 1 go-launch + 2 doc refs) |
| `checkOneLegBBO` in tracker.go (≥3 expected) | 4 matches (1 def + 2 call sites + 1 doc ref) |
| `priceGapBBOAssertionGrace` in tracker.go (≥1 expected) | 4 matches (1 def + 3 usages) |
| `BBO NOT LIVE` in tracker.go (≥1 expected) | 1 match |
| `case <-t.stopCh:` in assertBBOLiveness | present (select-block on lines inside assertBBOLiveness body) |
| `tracker_subscribe_test.go` exists | yes, 327 lines |
| `grep -n SubscribeSymbol cmd/main.go` for pricegap | 0 matches (correct — wiring stays in Tracker) |

## Self-Check: PASSED

Created files:
- FOUND: /var/solana/data/arb/internal/pricegaptrader/tracker_subscribe_test.go
- FOUND: /var/solana/data/arb/.planning/phases/09-price-gap-dashboard-paper-live-operations/09-10-SUMMARY.md

Commits:
- FOUND: 3eec59f (Tasks 1+2 — subscription loop + BBO-liveness assertion + 10 new tests)
- TO-COMMIT: VERSION + CHANGELOG + SUMMARY (docs commit)

Tests:
- FOUND: 10 new subscribe/BBO tests pass under `-race -short`
- FOUND: Full pricegaptrader package green (`ok arb/internal/pricegaptrader 1.232s`)
- FOUND: `go vet ./internal/pricegaptrader/...` clean
- FOUND: `go build ./...` clean

Unblocks: Plan 09-11 (UAT walkthrough of the 14 blocked rows). Gap #1 + Gap #2 together eliminate the silent-failure mode — any future detector problem will surface in logs (Gap #1) and any future subscription failure will surface as a `BBO NOT LIVE` WARN (Gap #2).
