---
phase: 15
slug: drawdown-circuit-breaker
status: ready
nyquist_compliant: true
wave_0_complete: false
created: 2026-05-01
updated: 2026-05-01
---

# Phase 15 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Populated by planner.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib + testify) |
| **Config file** | none — Go modules already configured |
| **Quick run command** | `go test ./internal/pricegaptrader/... -count=1 -short` |
| **Full suite command** | `go test ./... -count=1` |
| **Phase 15 suite** | `go test ./internal/pricegaptrader/... ./internal/api/... ./internal/notify/... ./internal/database/... ./internal/config/... ./cmd/pg-admin/... -run "Breaker\|TestPgBreaker\|TestPgAdminBreaker\|TestNotifyPriceGapBreaker\|TestSyntheticFire\|TestRecover\|TestRegistry_Paused\|TestStaticCheck_NoDirectPaperMode\|TestAggregator\|TestValidatePriceGapLive_BreakerFields\|TestPriceGapState_(SaveLoadBreaker\|LoadBreakerStateMissing\|BreakerTripsCap\|UpdateBreakerTripRecovery\|StickyMaxInt64Roundtrip)" -count=1` |
| **Estimated runtime** | ~30s quick / ~3min full |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/pricegaptrader/... -count=1 -short`
- **After every plan wave:** Run the Phase 15 suite command above (~30s)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

> Filled by planner. Every plan task that affects breaker behavior MUST appear here with an automated verify command, or be flagged Manual-Only with a Wave 0 stub.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 15-01-T1 | 15-01 | 1 | PG-LIVE-02 | T-15-01 | Config validator rejects positive limit when breaker enabled (limit ≤ 0); rejects interval outside [60, 3600] | unit | `go test ./internal/config -run TestValidatePriceGapLive_BreakerFields -count=1` | ❌ W0 stub created in 15-01-T3 | ⬜ pending |
| 15-01-T2 | 15-01 | 1 | PG-LIVE-02 | T-15-02 | BreakerState 5-field HASH save/load round-trip; sticky=MaxInt64 preserved; trips LIST capped at 500 entries | unit | `go test ./internal/database -run "TestPriceGapState_(SaveLoadBreaker\|LoadBreakerStateMissing\|BreakerTripsCap\|UpdateBreakerTripRecovery\|StickyMaxInt64Roundtrip)" -count=1` | ❌ W0 (created in same task) | ⬜ pending |
| 15-01-T3 | 15-01 | 1 | PG-LIVE-02 | — | 10 Wave 0 stub test files compile; all tests SKIP not FAIL | static | `go build ./... && go test ./internal/pricegaptrader/... ./internal/api/... ./internal/notify/... ./cmd/pg-admin/... -run "Breaker\|StaticCheck" -count=1` | ❌ W0 (this task creates them) | ⬜ pending |
| 15-02-T1 | 15-02 | 1 | PG-LIVE-02 | T-15-07 | RealizedPnLAggregator computes rolling-24h sum, dedupes across today/yesterday, ExchangeClosedAt preferred, missing-set resilient | unit | `go test ./internal/pricegaptrader -run "TestAggregator_(24hFilter\|MissingYesterdaySet\|DedupAcrossTodayYesterday\|RealizedPnLFieldName\|ExchangeClosedAtPreferredOverClosedAt\|BothKeysEmpty\|LoadPositionError_Skipped)" -count=1` | ❌ W0 (replaces 15-01 stub) | ⬜ pending |
| 15-02-T2 | 15-02 | 1 | PG-LIVE-02 | T-15-05, T-15-06 | All 4 cfg.PriceGapPaperMode reads in execution.go migrate to IsPaperModeActive; static regression guard fails build on direct reads; helper fail-safes to paper on Redis error | unit + static | `go test ./internal/pricegaptrader -run "TestTracker_IsPaperModeActive\|TestStaticCheck_NoDirectPaperModeRead" -count=1` | ❌ W0 (replaces 15-01 stub) | ⬜ pending |
| 15-03-T1 | 15-03 | 2 | PG-LIVE-02 | T-15-09 | Candidate.PausedByBreaker added; entry path rejects with distinct reason; chokepoint helpers serialize concurrent writes; recovery preserves operator-Disabled | integration | `go test ./internal/pricegaptrader -run "TestRegistry_(PausedByBreakerField\|EntryGuard_DisabledOR_PausedByBreaker\|PausedByBreaker_ConcurrentWrite\|RecoveryClearsPausedByBreaker_PreservesDisabled)" -count=1` | ❌ W0 | ⬜ pending |
| 15-03-T2 | 15-03 | 2 | PG-LIVE-02 | T-15-08, T-15-10, T-15-11 | BreakerController two-strike state machine + blackout suppression + boot guard + D-15 trip-ordering atomicity (sticky persisted FIRST even if steps 2-5 fail) + kill-9 survivability | unit | `go test ./internal/pricegaptrader -run "TestBreaker_(DisabledByDefault\|FreshBootInit\|BootGuard_PreservesExistingTrip\|SingleStrike_NoTrip\|TwoStrikeTrips\|TwoStrikeRequiresTwoSeparateEvaluations\|RecoveryClearsPendingStrike\|BlackoutSuppression\|PendingSurvivesBlackout\|StateSurvivesRestart\|TripOrdering_StickyFirstWhenStepsFail\|TripOrdering_Step1FailureAborts)" -count=1` | ❌ W0 (replaces 15-01 stub) | ⬜ pending |
| 15-04-T1 | 15-04 | 3 | PG-LIVE-02 | T-15-19 | Telegram critical-bucket dispatch for trip + recovery; TestFire real-trip default + dry-run skips all mutations; Recover preserves operator-Disabled; full-cycle synthetic test (success-criterion #5) | integration | `go test ./internal/notify -run "TestNotifyPriceGapBreaker(Trip\|Recovery)_CriticalBucket" -count=1 && go test ./internal/pricegaptrader -run "TestSyntheticFireFullCycle\|TestSyntheticFireDryRun_NoMutations\|TestRecover_PreservesOperatorDisabled\|TestBreaker_RecoveryReenablesCandidatesAndClearsSticky\|TestRecover_NotTrippedReturnsError" -count=1` | ❌ W0 | ⬜ pending |
| 15-04-T2 | 15-04 | 3 | PG-LIVE-02 | T-15-12, T-15-13, T-15-16, T-15-17 | pg-admin 3 subcommands enforce typed-phrase; REST 3 endpoints enforce auth + typed-phrase + state-conflict guards (409 on test-fire-while-tripped, recover-when-not-tripped) | integration | `go test ./cmd/pg-admin -run "TestPgAdminBreaker" -count=1 && go test ./internal/api -run "TestPgBreakerHandlers" -count=1` | ❌ W0 | ⬜ pending |
| 15-04-T3 | 15-04 | 3 | PG-LIVE-02 | — | cmd/main.go bootstrap wiring + VERSION → v0.38.0 + CHANGELOG entry; full Phase 15 suite green; adjacent engines untouched | build + integration | `go build ./... && grep -Fxq "v0.38.0" VERSION && grep -q "v0.38.0 — " CHANGELOG.md && go test ./internal/engine/... ./internal/spotengine/... -count=1` | ❌ W0 | ⬜ pending |
| 15-05-T1 | 15-05 | 3 | PG-LIVE-02 | T-15-18, T-15-20 | i18n keys lockstep en + zh-TW; BreakerSubsection extends RampReconcileSection; typed-phrase modal enforces literal RECOVER/TEST-FIRE; build-order: npm run build BEFORE go build | build | `cd web && npm run build && cd .. && go build ./... && comm -3 <(grep -oE "breaker[A-Za-z]+" web/src/i18n/en.ts \| sort -u) <(grep -oE "breaker[A-Za-z]+" web/src/i18n/zh-TW.ts \| sort -u) \| wc -l \| awk '$1 == 0 { exit 0 } { exit 1 }'` | ❌ W0 | ⬜ pending |
| 15-05-T2 | 15-05 | 3 | PG-LIVE-02 | — | Operator-recorded full UI cycle: trip via test-fire → see Tripped badge → click Recover → type RECOVER → see Armed badge; Telegram alerts received | manual-UAT | (recorded in 15-HUMAN-UAT.md per Phase 14 P05 precedent) | ❌ HUMAN | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

> Test files that must exist before Wave 1 starts. Plan 15-01 Task 3 creates these as compiling stubs; downstream plans replace stubs with real implementations.

- [ ] `internal/pricegaptrader/breaker_static_test.go` — TestStaticCheck_NoDirectPaperModeRead (Plan 15-02 Task 2 implements)
- [ ] `internal/pricegaptrader/breaker_aggregator_test.go` — 7 TestAggregator_* tests (Plan 15-02 Task 1 implements)
- [ ] `internal/pricegaptrader/breaker_controller_test.go` — 12 TestBreaker_* + TestTracker_IsPaperModeActive_* tests (Plan 15-02 Task 2 + Plan 15-03 Task 2 implement)
- [ ] `internal/pricegaptrader/breaker_test_fire_test.go` — TestSyntheticFireFullCycle + TestSyntheticFireDryRun_NoMutations + TestBreaker_RecoveryReenablesCandidatesAndClearsSticky + TestRecover_PreservesOperatorDisabled + TestRecover_NotTrippedReturnsError (Plan 15-04 Task 1 implements)
- [ ] `internal/api/pricegap_breaker_handlers_test.go` — 5+ TestPgBreakerHandlers_* tests (Plan 15-04 Task 2 implements)
- [ ] `internal/notify/pricegap_breaker_test.go` — TestNotifyPriceGapBreakerTrip_CriticalBucket + Recovery (Plan 15-04 Task 1 implements)
- [ ] `cmd/pg-admin/breaker_test.go` — TestPgAdminBreakerRecover/TestFire/Show_TypedPhraseRequired (Plan 15-04 Task 2 implements)
- [ ] `internal/database/pricegap_breaker_state_test.go` — 5 TestPriceGapState_* tests (Plan 15-01 Task 2 implements)
- [ ] `internal/config/config_test.go` — TestValidatePriceGapLive_BreakerFields extends existing file (Plan 15-01 Task 1 implements)
- [ ] `internal/pricegaptrader/registry_concurrent_test.go` — extended with 4 new TestRegistry_PausedByBreaker_* tests (Plan 15-03 Task 1 implements)
- [ ] `15-HUMAN-UAT.md` — operator-recorded full-cycle synthetic-fire dashboard exercise (Plan 15-05 Task 2 records)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Telegram critical alert delivery | PG-LIVE-02 SC#3 | Real Telegram API call; mock at boundary in unit tests | Set bot token + chat ID in config, run `pg-admin breaker test-fire --confirm` (NOT --dry-run), verify message arrives in Telegram with critical bucket fields populated. |
| Dashboard recovery button UX | PG-LIVE-02 SC#4 | UI confirmation flow + i18n rendering | Trip breaker via dashboard test-fire button, observe red "Tripped" badge appears within 2s (WS push), click Recover, type RECOVER literal in modal, observe badge flips back to green "Armed" without engine restart. Plan 15-05 Task 2 records. |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (Plan 15-01 Task 3 creates 10 stub files; downstream plans replace stubs)
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** ready for execute-phase
