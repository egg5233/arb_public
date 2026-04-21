---
phase: 06-spot-futures-risk-hardening
plan: 01
subsystem: exchange, config, spotengine
tags: [maintenance-rate, risk, exchange-adapter, cache, config]

# Dependency graph
requires: []
provides:
  - ContractInfo.MaintenanceRate field populated by 5 exchange adapters
  - GetMaintenanceRate(symbol, notional) method on 5 adapters with tier matching
  - SpotEngine getMaintenanceRate() helper with TTL cache and fallback chain
  - Config fields for maintenance gate enable, default rate, cache TTL
affects: [06-02, 06-03, 06-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "maintenanceRateProvider optional interface for adapter type assertion"
    - "4-level maintenance rate fallback: cache -> adapter -> ContractInfo -> default"
    - "Notional bucket rounding for cache key efficiency"

key-files:
  created:
    - internal/spotengine/maintenance.go
    - internal/spotengine/maintenance_test.go
    - pkg/exchange/binance/adapter_test.go
    - pkg/exchange/bybit/adapter_test.go
    - pkg/exchange/bitget/adapter_test.go
    - pkg/exchange/okx/maintenance_test.go
  modified:
    - pkg/exchange/types.go
    - pkg/exchange/gateio/adapter.go
    - pkg/exchange/gateio/adapter_test.go
    - pkg/exchange/bybit/adapter.go
    - pkg/exchange/binance/adapter.go
    - pkg/exchange/okx/adapter.go
    - pkg/exchange/bitget/adapter.go
    - internal/config/config.go
    - internal/spotengine/engine.go

key-decisions:
  - "GetMaintenanceRate not added to Exchange interface -- kept adapter-specific via maintenanceRateProvider optional interface (BingX excluded)"
  - "OKX MaintenanceRate=0 in LoadAllContracts (fetched on demand) since OKX requires per-instFamily API calls"
  - "Bitget MaintenanceRate=0 in LoadAllContracts (fetched on demand) since Bitget requires per-symbol API calls"
  - "Lazy cache initialization in getMaintenanceRate to avoid constructor changes"

patterns-established:
  - "maintenanceRateProvider interface: type-assert exchange adapter to check for tiered rate support"
  - "Bybit maintenance normalization: always divide by 100 (percentage to decimal)"
  - "Binance leverageBracket requires authenticated client (USER_DATA endpoint)"

requirements-completed: [SF-RISK-01, SF-RISK-02, SF-RISK-03, SF-RISK-04]

# Metrics
duration: 16min
completed: 2026-04-05
---

# Phase 6 Plan 01: Maintenance Rate Foundation Summary

**Per-contract maintenance margin rates from 5 exchange adapters with tiered tier-matching, TTL cache, conservative fallback, and 6-touch-point config**

## Performance

- **Duration:** 16 min
- **Started:** 2026-04-05T13:55:49Z
- **Completed:** 2026-04-05T14:12:06Z
- **Tasks:** 2
- **Files modified:** 15

## Accomplishments
- ContractInfo.MaintenanceRate field added and populated by all 5 adapters (Gate.io inline, Bybit/Binance in LoadAllContracts, OKX/Bitget on-demand)
- GetMaintenanceRate(symbol, notional) methods on 5 adapters with exchange-specific tier matching and normalization
- Bybit percentage-to-decimal normalization verified by unit test (div 100)
- Binance authenticated leverageBracket endpoint verified by header check in test
- SpotEngine getMaintenanceRate() helper with maintenanceRateCache (configurable TTL), 4-level fallback chain
- maintenanceRate=0 always returns conservative default (0.05 = 5%), never silently passes (threat T-06-03)
- 21 total test cases across 7 test files (12 adapter + 9 spotengine)
- Config fields follow 6-touch-point convention with defaults OFF/0.05/60

## Task Commits

Each task was committed atomically:

1. **Task 1: ContractInfo.MaintenanceRate + 5 adapter GetMaintenanceRate + tests** - `8e387c4` (feat)
2. **Task 2: Config fields + SpotEngine getMaintenanceRate helper with cache** - `f9adf77` (feat)

## Files Created/Modified
- `pkg/exchange/types.go` - Added MaintenanceRate float64 field to ContractInfo
- `pkg/exchange/gateio/adapter.go` - Parse maintenance_rate from contracts endpoint; GetMaintenanceRate via risk_limit_tiers
- `pkg/exchange/bybit/adapter.go` - loadMaintenanceRates with pagination; GetMaintenanceRate with div-by-100 normalization
- `pkg/exchange/binance/adapter.go` - loadMaintenanceRates from authenticated leverageBracket; GetMaintenanceRate with bracket matching
- `pkg/exchange/okx/adapter.go` - GetMaintenanceRate via position-tiers with instFamily extraction
- `pkg/exchange/bitget/adapter.go` - GetMaintenanceRate via query-position-lever with keepMarginRate parsing
- `pkg/exchange/gateio/adapter_test.go` - 3 tests: normalization, tier matching, bounds check
- `pkg/exchange/bybit/adapter_test.go` - 2 tests: normalization (div 100), tier matching
- `pkg/exchange/binance/adapter_test.go` - 2 tests: normalization (already decimal), tier matching with auth check
- `pkg/exchange/okx/maintenance_test.go` - 2 tests: normalization (mmr decimal), tier matching
- `pkg/exchange/bitget/adapter_test.go` - 2 tests: normalization (keepMarginRate), bounds check
- `internal/config/config.go` - 3 new fields with 6-touch-point convention
- `internal/spotengine/engine.go` - Added maintCache field to SpotEngine
- `internal/spotengine/maintenance.go` - maintenanceRateCache, getMaintenanceRate helper, maintenanceRateProvider interface
- `internal/spotengine/maintenance_test.go` - 5 tests: cache hit/miss, cache dedup, fallback default, bounds check, ContractInfo fallback

## Decisions Made
- GetMaintenanceRate is NOT on the Exchange interface (anti-pattern per research: BingX doesn't support spot-futures). Used optional maintenanceRateProvider interface with type assertion instead.
- OKX and Bitget set MaintenanceRate=0 in LoadAllContracts since their APIs require per-symbol calls. Rates are fetched on demand via GetMaintenanceRate and cached.
- Gate.io is the only exchange that populates MaintenanceRate directly in LoadAllContracts (the contracts endpoint includes it).
- Lazy cache initialization in getMaintenanceRate() avoids modifying the SpotEngine constructor.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- OKX test file initially named `adapter_test_maintenance.go` which Go doesn't recognize as a test file (must end in `_test.go`). Renamed to `maintenance_test.go`.

## User Setup Required
None - no external service configuration required. New config fields default to OFF.

## Next Phase Readiness
- All 5 adapters ready for Plan 02 (pre-entry risk gate) to call getMaintenanceRate()
- Config field SpotFuturesEnableMaintenanceGate ready for Plan 02 to check
- Cache and fallback chain ready for Plan 03 (runtime monitor) and Plan 04 (discovery display)
- No blockers

## Self-Check: PASSED

- All 16 key files verified present
- Both task commits verified (8e387c4, f9adf77)
- All 21 tests pass across 7 test files
- go vet clean on all modified packages
- Full build (go build ./cmd/main.go) succeeds

---
*Phase: 06-spot-futures-risk-hardening*
*Completed: 2026-04-05*
