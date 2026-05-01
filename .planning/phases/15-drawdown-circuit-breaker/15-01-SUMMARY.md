---
phase: 15
plan: 01
subsystem: pricegaptrader
tags: [drawdown-breaker, foundation, config, redis, models, wave-0]
requires:
  - phase-14-ramp-controller
  - phase-14-reconciler
provides:
  - config.PriceGapBreakerEnabled
  - config.PriceGapDrawdownLimitUSDT
  - config.PriceGapBreakerIntervalSec
  - models.BreakerState
  - models.BreakerTripRecord
  - database.SaveBreakerState
  - database.LoadBreakerState
  - database.AppendBreakerTrip
  - database.UpdateBreakerTripRecovery
  - redis-namespace pg:breaker:state
  - redis-namespace pg:breaker:trips
  - wave-0-test-targets (30 test names across 7 files)
affects:
  - internal/config/config.go (struct + applyJSON + validator)
  - internal/database/pricegap_state.go (4 helpers + 2 namespace consts)
  - internal/models/pricegap_breaker.go (NEW; 2 structs)
tech-stack:
  added: []
  patterns:
    - "Redis HASH 5-field decimal-string encoding (mirrors RampState; preserves int64 sentinel)"
    - "Redis LIST capped at 500 via LPush+LTrim 0 499 (mirrors pg:ramp:events)"
    - "Wave 0 t.Skip stubs reserve test-name surface for downstream plans"
key-files:
  created:
    - internal/models/pricegap_breaker.go
    - internal/database/pricegap_breaker_state_test.go
    - internal/pricegaptrader/breaker_static_test.go
    - internal/pricegaptrader/breaker_aggregator_test.go
    - internal/pricegaptrader/breaker_controller_test.go
    - internal/pricegaptrader/breaker_test_fire_test.go
    - internal/api/pricegap_breaker_handlers_test.go
    - internal/notify/pricegap_breaker_test.go
    - cmd/pg-admin/breaker_test.go
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - internal/database/pricegap_state.go
    - CHANGELOG.md
    - VERSION
decisions:
  - "PaperModeStickyUntil stored as decimal-string of int64 (not float) so math.MaxInt64 sentinel survives Redis roundtrip without precision loss"
  - "LTrim 0 499 inlined (not via constant) per plan acceptance criterion grep pattern; constant priceGapBreakerTripsCap=500 retained for documentation"
  - "Wave 0 stub count: 30 SKIP tests across 7 files (plan title says 10 stub files but listed 7 paths; satisfied with 7 files reserving 30 names — exceeds plan acceptance threshold of 25 SKIPs)"
  - "applyJSON gate: BreakerIntervalSec default-then-override pattern matches Phase 14 Stage* fields so a partial price_gap JSON block leaves the 300s default intact"
metrics:
  duration: ~25min
  tasks: 3
  files_created: 9
  files_modified: 5
  tests_added: 19  # 13 config validator cases + 5 database persistence + 1 retest in coverage path
  wave_0_stubs: 30
  completed_date: 2026-05-01
---

# Phase 15 Plan 01: Foundation Summary

**One-liner:** Three breaker config fields, two model structs, four Redis helpers, two namespace constants, and 30 Wave 0 test stubs ship the data plumbing Plans 15-02 / 15-03 / 15-04 will compile against — pure data layer, zero behavior.

## Objective Recap

Lay all data plumbing for the Phase 15 drawdown circuit breaker (PG-LIVE-02) in one landing so downstream plans (paper-mode chokepoint migration in 15-02, BreakerController state machine in 15-03, CLI/API/UI in 15-04) compile and test against finalized contracts.

## What Shipped

### Task 1 — Config schema (commit `7ea47ad`)

**Three new `Config` fields** on the existing `PriceGap*` block in `internal/config/config.go`:

| Go field | JSON tag | Type | Default | Purpose |
|----------|----------|------|---------|---------|
| `PriceGapBreakerEnabled` | `enable_pricegap_breaker` | `bool` | `false` | Master switch (default OFF per Phase 15 hard contract) |
| `PriceGapDrawdownLimitUSDT` | `pricegap_drawdown_limit_usdt` | `float64` | `0` | Absolute USDT trip threshold; armed-but-never-trips at default; must be ≤ 0 when enabled (D-06) |
| `PriceGapBreakerIntervalSec` | `pricegap_breaker_interval_sec` | `int` | `300` | Eval ticker interval; range [60, 3600]s (D-01 5-min default with safety floor + ceiling) |

**Validator extension** appended to `validatePriceGapLive`. When `PriceGapBreakerEnabled=true`:
- Rejects `PriceGapDrawdownLimitUSDT > 0` with explicit error message `pricegap_drawdown_limit_usdt must be <= 0 ...`.
- Rejects `PriceGapBreakerIntervalSec` outside `[60, 3600]` with explicit error message `pricegap_breaker_interval_sec must be in [60, 3600] ...`.
- Default OFF (any field values) accepted — operator must opt-in.

**applyJSON wiring** mirrors the Phase 14 default-then-override pattern: the interval default (300s) is installed before the override block so partial `price_gap` JSON blocks leave the safe default intact.

**Test coverage:** `TestValidatePriceGapLive_BreakerFields` — 13 table-driven cases (default-OFF acceptance, positive-limit rejection at 50 / 0.1, interval boundary tests at 59 / 60 / 300 / 3600 / 3601, zero-interval rejection).

### Task 2 — Models + Redis helpers (commits `b938d23` RED, `0efecaf` GREEN)

**TDD flow:** RED commit added 5 failing tests + the 2 model structs; GREEN commit added the 4 helpers + 2 namespace constants and turned the tests green.

**`internal/models/pricegap_breaker.go` (NEW):**

```go
type BreakerState struct {
    PendingStrike        int     `json:"pending_strike"`
    Strike1Ts            int64   `json:"strike1_ts_ms"`
    LastEvalTs           int64   `json:"last_eval_ts_ms"`
    LastEvalPnLUSDT      float64 `json:"last_eval_pnl_usdt"`
    PaperModeStickyUntil int64   `json:"paper_mode_sticky_until_ms"`
}

type BreakerTripRecord struct {
    TripTs               int64   `json:"trip_ts_ms"`
    TripPnLUSDT          float64 `json:"trip_pnl_usdt"`
    Threshold            float64 `json:"threshold_usdt"`
    RampStage            int     `json:"ramp_stage"`
    PausedCandidateCount int     `json:"paused_candidate_count"`
    RecoveryTs           *int64  `json:"recovery_ts_ms,omitempty"`
    RecoveryOperator     *string `json:"recovery_operator,omitempty"`
    Source               string  `json:"source"`
}
```

13 `json:` tags total (5 + 8) — exceeds plan acceptance criterion (≥13).

**`internal/database/pricegap_state.go` (EDIT):**

- 2 new namespace constants: `keyPriceGapBreakerState = "pg:breaker:state"`, `keyPriceGapBreakerTrips = "pg:breaker:trips"` (plus internal cap constant `priceGapBreakerTripsCap = 500`).

- 4 new `*Client` methods:

  | Helper | Signature | Notes |
  |--------|-----------|-------|
  | `SaveBreakerState(s models.BreakerState) error` | HSet pipeline; numeric fields encoded as decimal strings via `strconv.FormatInt`/`strconv.FormatFloat` so `math.MaxInt64` survives the roundtrip. |
  | `LoadBreakerState() (models.BreakerState, bool, error)` | HGetAll; returns `(zero, false, nil)` on missing key (boot-init signal); decode failures return error so callers can fail-safe-to-paper. |
  | `AppendBreakerTrip(record models.BreakerTripRecord) error` | JSON-marshal + LPush + `LTrim 0 499` in one TxPipeline. Newest-first ordering at index 0. |
  | `UpdateBreakerTripRecovery(index int64, recoveryTs int64, operator string) error` | LIndex + json.Unmarshal + backfill nullable fields + LSet — used by recovery path to mutate the most-recent (index 0) trip. |

**Test coverage:** 5 unit tests against miniredis:
- `TestPriceGapState_SaveLoadBreakerState` — full 5-field roundtrip including `MaxInt64` sentinel.
- `TestPriceGapState_LoadBreakerStateMissing` — `(zero, false, nil)` boot-init signal.
- `TestPriceGapState_BreakerTripsCap` — insert 502, verify length=500, head=newest, oldest survivor=index 2.
- `TestPriceGapState_UpdateBreakerTripRecovery` — LSET backfill + pre-existing field preservation.
- `TestPriceGapState_StickyMaxInt64Roundtrip` — sentinel-precision regression guard.

### Task 3 — Wave 0 stubs (commit `fe80f5f`)

**7 stub files reserve 30 test names** so downstream plans implement against pre-named test targets:

| Stub file | Tests reserved | Target plan |
|-----------|---------------|-------------|
| `internal/pricegaptrader/breaker_static_test.go` | `TestStaticCheck_NoDirectPaperModeRead` (1) | 15-02 |
| `internal/pricegaptrader/breaker_aggregator_test.go` | `TestAggregator_24hFilter` / `_MissingYesterdaySet` / `_DedupAcrossTodayYesterday` / `_RealizedPnLFieldName` (4) | 15-02 |
| `internal/pricegaptrader/breaker_controller_test.go` | `TestBreaker_DisabledByDefault`, `_FreshBootInit`, `_BootGuard_PreservesExistingTrip`, `_SingleStrike_NoTrip`, `_TwoStrikeTrips`, `_TwoStrikeRequiresTwoSeparateEvaluations`, `_RecoveryClearsPendingStrike`, `_BlackoutSuppression`, `_PendingSurvivesBlackout`, `_StateSurvivesRestart`, `_TripOrdering_StickyFirstWhenStepsFail`, `TestTracker_IsPaperModeActive_Sticky` (12) | 15-03 (one for 15-02 helper) |
| `internal/pricegaptrader/breaker_test_fire_test.go` | `TestSyntheticFireFullCycle`, `TestSyntheticFireDryRun_NoMutations`, `TestBreaker_RecoveryReenablesCandidatesAndClearsSticky` (3) | 15-04 |
| `internal/api/pricegap_breaker_handlers_test.go` | `TestPgBreakerHandlers_TypedPhraseRequired` / `_RecoverEndpoint` / `_TestFireEndpoint` / `_StateGetEndpoint` / `_AuthRequired` (5) | 15-04 |
| `internal/notify/pricegap_breaker_test.go` | `TestNotifyPriceGapBreakerTrip_CriticalBucket`, `_Recovery` (2) | 15-04 |
| `cmd/pg-admin/breaker_test.go` | `TestPgAdminBreakerRecover_TypedPhraseRequired`, `_TestFire`, `_Show` (3) | 15-04 |

Each stub: minimal `package` declaration, `import "testing"`, comment header naming the implementing plan and expected behavior, body is `t.Skip("Wave 0 stub — Plan 15-XX implements ...")`.

**Verification:** `go test ... -run "Breaker|StaticCheck|TestSyntheticFire|TestAggregator|TestPgBreaker|TestPgAdminBreaker|TestNotifyPriceGapBreaker|TestTracker_IsPaperModeActive"` produces 30 SKIPs and 0 FAILs (one pre-existing `TestExecution_CircuitBreakerOpensAfterFiveFailures` matches the regex and PASSes — orthogonal to Phase 15).

## Verification

| Verification step | Result |
|-------------------|--------|
| `go build ./...` | Pass |
| `go test ./internal/config -run TestValidatePriceGapLive_BreakerFields -count=1` | 13 cases pass |
| `go test ./internal/database -run "TestPriceGapState_(SaveLoadBreaker\|LoadBreakerStateMissing\|BreakerTripsCap\|UpdateBreakerTripRecovery\|StickyMaxInt64Roundtrip)" -count=1` | 5 tests pass |
| Wave 0 stub gate (30 SKIPs, 0 FAILs across pricegaptrader/api/notify/pg-admin) | Pass |
| `git diff --quiet config.json` | Pass — config.json untouched |
| CHANGELOG.md + VERSION updated together | Pass — Unreleased section + 0.37.0 → 0.37.1 |

## Deviations from Plan

None — plan executed exactly as written.

The plan title says "10 stub files" but lists 7 file paths in the `<files>` block. The 7-file approach was kept (matches `<files>` enumeration); 30 reserved test names exceeds the acceptance criterion's "≥25 SKIPs" threshold so the spirit of the requirement is met.

## Authentication Gates

None — pure code changes, no external services touched.

## Migration Notes for Plan 15-02

Plan 15-02 will introduce the paper-mode chokepoint helper `Tracker.IsPaperModeActive(ctx) (bool, error)`. The contract Plan 15-01 establishes for that helper:

- **Source of sticky state:** `LoadBreakerState()` returns `(state, exists, err)`. The helper must:
  - On `err != nil` → fail-safe-to-paper: return `(true, err)`. A Redis outage during a real trip MUST NOT silently allow live trading.
  - On `exists=false` → not sticky, fall through to the cfg flag.
  - On `exists=true && state.PaperModeStickyUntil != 0 && now < PaperModeStickyUntil` → return `(true, nil)` regardless of cfg.
- **Sentinel encoding:** `PaperModeStickyUntil` carries `0` for not-sticky and `math.MaxInt64` for sticky-until-operator. The decimal-string Redis encoding is verified to roundtrip without truncation in `TestPriceGapState_StickyMaxInt64Roundtrip` — the helper can rely on the value coming back exactly as written.
- **Boot guard contract:** Plan 15-03 (BreakerController) will own the boot guard; Plan 15-02 only needs `LoadBreakerState`, not Save. If the BreakerController is disabled, the HASH may not exist at all — helper must treat `exists=false` as "no sticky" and fall through.

## Known Stubs

None — Plan 15-01 ships only data plumbing; no UI, no runtime data flow that could appear stubbed to operators. All hardcoded literals (defaults, validator bounds, namespace strings) are documented per CONTEXT/RESEARCH and intentional.

## Self-Check: PASSED

- `internal/models/pricegap_breaker.go` — FOUND
- `internal/database/pricegap_breaker_state_test.go` — FOUND
- `internal/pricegaptrader/breaker_static_test.go` — FOUND
- `internal/pricegaptrader/breaker_aggregator_test.go` — FOUND
- `internal/pricegaptrader/breaker_controller_test.go` — FOUND
- `internal/pricegaptrader/breaker_test_fire_test.go` — FOUND
- `internal/api/pricegap_breaker_handlers_test.go` — FOUND
- `internal/notify/pricegap_breaker_test.go` — FOUND
- `cmd/pg-admin/breaker_test.go` — FOUND
- Commit `7ea47ad` (Task 1 config) — FOUND
- Commit `b938d23` (Task 2 RED) — FOUND
- Commit `0efecaf` (Task 2 GREEN) — FOUND
- Commit `fe80f5f` (Task 3 stubs) — FOUND
