---
phase: 06-spot-futures-risk-hardening
verified: 2026-04-05T15:30:00Z
status: human_needed
score: 4/4 must-haves verified
re_verification: false
human_verification:
  - test: "Enable SpotFuturesEnableMaintenanceGate in config and trigger a spot-futures auto-entry on a high-maintenance-rate contract"
    expected: "Entry is rejected with reason containing 'maintenance_survivable_' in the log"
    why_human: "Requires live exchange + funded account; cannot verify end-to-end rejection in integration without running the bot"
  - test: "With live position on a contract near liquidation, confirm liq_distance_emergency trigger fires and closes the position"
    expected: "exit_manager checkExitTriggers returns liq_distance_emergency, SpotEngine closes position"
    why_human: "Requires live position at specific price proximity; cannot simulate orderbook mid-price and real maintenance rates together"
  - test: "Verify dashboard Opportunities table renders the MR column with color-coded values for known high/low maintenance rate symbols"
    expected: "Red for GUAUSDT-class contracts (>= 10%), amber for moderate (5-10%), gray for normal (< 5%), dash for unknown"
    why_human: "Visual rendering requires a running dashboard with active discovery scan data"
---

# Phase 6: Spot-Futures Risk Hardening Verification Report

**Phase Goal:** Prevent spot-futures positions that are immediately overleveraged relative to the contract's maintenance requirements, and detect/act on liquidation proximity at runtime across all 5 margin exchanges
**Verified:** 2026-04-05T15:30:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (from ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Pre-entry check fetches per-contract maintenance_rate and rejects positions where max survivable price drop < configured threshold | ✓ VERIFIED | risk_gate.go check 6: `survivableDrop := (1.0/leverage) - maintenanceRate`, `entryThreshold := 0.90/leverage`, rejects when `survivableDrop < entryThreshold` with reason `maintenance_survivable_X%<Y%` |
| 2 | Runtime monitor calculates liquidation distance using current mark price + maintenance_rate, triggers exit when distance falls below threshold | ✓ VERIFIED | exit_manager.go trigger 2b: `estLiqPrice = pos.FuturesEntry * (1 - (1/leverage) + maintenanceRate)`, graduated response at 10%/20%/50% of entry threshold, returns `liq_distance_emergency` or `liq_distance_exit` |
| 3 | Health monitor (L3/L4/L5) evaluates spot-futures positions — not just perp-perp | ✓ VERIFIED | health.go checkAll() calls `h.db.GetActiveSpotPositions()`, L4 handleL4 populates `HealthAction.SpotPositions` with largest spot position, L5 handleL5 includes ALL spot positions; engine.go consumeHealthActions dispatches via `spotCloseCallback` |
| 4 | Discovery displays maintenance_rate per contract for user observation (scoring deferred per D-15) | ✓ VERIFIED | SpotArbOpportunity.MaintenanceRate populated in both Dir A and Dir B discovery paths via `lookupMaintenanceRateForDisplay()`; dashboard Opportunities.tsx renders MR/Maint % column with red/amber/gray color coding; no scoring or filtering applied |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/exchange/types.go` | MaintenanceRate field on ContractInfo | ✓ VERIFIED | Line 111: `MaintenanceRate float64 // tier-1 maintenance margin rate as decimal` |
| `internal/spotengine/maintenance.go` | getMaintenanceRate helper with TTL cache | ✓ VERIFIED | 162 lines; `maintenanceRateProvider` interface, `maintenanceRateCache` with TTL, `getMaintenanceRate` with 4-level fallback chain |
| `internal/config/config.go` | SpotFuturesEnableMaintenanceGate + threshold fields | ✓ VERIFIED | Lines 216-218: 3 fields with defaults OFF/0.05/60; 6-touch-point convention (struct field, jsonSpotFutures tag, default value, applyJSON, persistToRedis, loadEnvOverrides) |
| `pkg/exchange/gateio/adapter.go` | GetMaintenanceRate + MaintenanceRate in LoadAllContracts | ✓ VERIFIED | maintenance_rate parsed inline in LoadAllContracts; GetMaintenanceRate calls /futures/usdt/risk_limit_tiers |
| `pkg/exchange/bybit/adapter.go` | GetMaintenanceRate + div-by-100 normalization | ✓ VERIFIED | `mm / 100.0` on line 563 and 613; GetMaintenanceRate with tier matching |
| `pkg/exchange/binance/adapter.go` | GetMaintenanceRate using authenticated leverageBracket | ✓ VERIFIED | `b.client.Get("/fapi/v1/leverageBracket", ...)` — client.Get signs request (USER_DATA endpoint) |
| `pkg/exchange/okx/adapter.go` | GetMaintenanceRate via position-tiers | ✓ VERIFIED | GetMaintenanceRate calls /api/v5/public/position-tiers with instFamily extraction |
| `pkg/exchange/bitget/adapter.go` | GetMaintenanceRate via query-position-lever | ✓ VERIFIED | GetMaintenanceRate parses keepMarginRate from /api/v2/mix/market/query-position-lever |
| `internal/spotengine/risk_gate.go` | Check 6 maintenance gate before check 7 dry-run | ✓ VERIFIED | `// 6. Maintenance rate:` at line 79, `// 7. Dry-run:` at line 107; contains `maintenance_survivable_` reason string |
| `internal/spotengine/execution.go` | Warning for manual opens on high-maintenance-rate contracts | ✓ VERIFIED | Log line: `%s on %s maintenance_rate=%.1f%%, survivable=%.1f%% < threshold=%.1f%% — proceeding per manual override` |
| `internal/api/spot_handlers.go` | maintenance_rate_warning in manual open API response | ✓ VERIFIED | Lines 239-241: checks `s.spotMaintenanceWarning != nil` and adds `resp["maintenance_rate_warning"]` |
| `internal/spotengine/exit_manager.go` | Trigger 2b: liq distance emergency/exit/warn | ✓ VERIFIED | `// 2b. Maintenance-rate-aware liquidation distance` at line 207; contains `liq_distance_emergency`, `liq_distance_exit`; thresholds at 0.10/0.20/0.50 fractions |
| `internal/risk/health.go` | SpotPositions on HealthAction + GetActiveSpotPositions in checkAll | ✓ VERIFIED | Line 67: `SpotPositions []*models.SpotFuturesPosition`; line 161: `h.db.GetActiveSpotPositions()`; L4/L5 handlers accept spotPositions param |
| `internal/engine/engine.go` | SetSpotCloseCallback + dispatchSpotHealthAction | ✓ VERIFIED | Lines 165-170: SetSpotCloseCallback setter; lines 1158-1178: dispatchSpotHealthAction dispatches to spotCloseCallback goroutines |
| `internal/spotengine/discovery.go` | MaintenanceRate on SpotArbOpportunity, populated in both scanner paths | ✓ VERIFIED | Line 29: `MaintenanceRate float64` with json tag `maintenance_rate`; `lookupMaintenanceRateForDisplay()` called, result assigned to both Dir A (line 262) and Dir B (line 289) |
| `web/src/types.ts` | maintenance_rate on SpotOpportunity | ✓ VERIFIED | Line 171: `maintenance_rate: number;` |
| `web/src/pages/Opportunities.tsx` | MR/Maint % column with color coding | ✓ VERIFIED | Lines 145-149 (compact) and 194-198 (full): renders with `text-red-400` for >= 0.10, `text-amber-400` for >= 0.05, `text-gray-400` otherwise |
| `web/src/i18n/en.ts` | spot.maintenanceRate + spot.maintenanceRateTooltip | ✓ VERIFIED | Lines 111-112: `'spot.maintenanceRate': 'Maint %'` and tooltip key |
| `web/src/i18n/zh-TW.ts` | spot.maintenanceRate + spot.maintenanceRateTooltip | ✓ VERIFIED | Lines 113-114: `'spot.maintenanceRate': '維持率'` and Chinese tooltip |
| `cmd/main.go` | SetSpotCloseCallback + SetSpotMaintenanceWarning wired | ✓ VERIFIED | Lines 408, 410: both callbacks wired after engine + spotengine initialization |
| `VERSION` | Bumped to reflect Phase 6 | ✓ VERIFIED | 0.28.3 |
| `CHANGELOG.md` | Spot-Futures Risk Hardening entries for all 4 plans | ✓ VERIFIED | 4 entries present for Plans 01-04 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `pkg/exchange/gateio/adapter.go` | `pkg/exchange/types.go` | ContractInfo.MaintenanceRate in LoadAllContracts | ✓ WIRED | `maintenance_rate` json field parsed inline, set on ContractInfo |
| `pkg/exchange/bybit/adapter.go` | `pkg/exchange/types.go` | ContractInfo.MaintenanceRate via loadMaintenanceRates | ✓ WIRED | loadMaintenanceRates called from LoadAllContracts, div-by-100 normalization |
| `internal/spotengine/maintenance.go` | `pkg/exchange/gateio/adapter.go` | maintenanceRateProvider type-assertion | ✓ WIRED | `exch.(maintenanceRateProvider)` in getMaintenanceRate; all 5 adapters implement GetMaintenanceRate |
| `internal/spotengine/risk_gate.go` | `internal/spotengine/maintenance.go` | `e.getMaintenanceRate(symbol, exchName, plannedNotional)` | ✓ WIRED | Line 95 in risk_gate.go |
| `internal/spotengine/execution.go` | `internal/spotengine/maintenance.go` | manual open warning via `e.getMaintenanceRate` | ✓ WIRED | Via `e.MaintenanceWarning()` method which calls getMaintenanceRate |
| `internal/spotengine/exit_manager.go` | `internal/spotengine/maintenance.go` | `e.getMaintenanceRate` for liq distance calc | ✓ WIRED | Called in trigger 2b block in checkExitTriggers |
| `internal/risk/health.go` | `internal/database/spot_state.go` | `h.db.GetActiveSpotPositions()` | ✓ WIRED | Line 161 in checkAll() |
| `internal/engine/engine.go` | `internal/spotengine/execution.go` | `e.spotCloseCallback` -> SpotEngine.ClosePosition | ✓ WIRED | Lines 1158, 1162 call `dispatchSpotHealthAction`; callback set via `SetSpotCloseCallback(spotEng.ClosePosition)` in cmd/main.go:410 |
| `internal/spotengine/discovery.go` | `pkg/exchange/types.go` | ContractInfo.MaintenanceRate via lookupMaintenanceRateForDisplay | ✓ WIRED | lookupMaintenanceRateForDisplay calls getMaintenanceRate which falls back to ContractInfo |
| `web/src/pages/Opportunities.tsx` | `web/src/types.ts` | SpotOpportunity.maintenance_rate rendered | ✓ WIRED | `opp.maintenance_rate` used in JSX; `SpotOpportunity` interface includes `maintenance_rate: number` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `Opportunities.tsx` | `opp.maintenance_rate` | SpotArbOpportunity.MaintenanceRate via WebSocket broadcast | Yes — populated from ContractInfo (Gate.io/Binance/Bybit at startup) or GetMaintenanceRate on-demand (OKX/Bitget), with 0.05 default fallback; never hardcoded empty | ✓ FLOWING |
| `internal/spotengine/risk_gate.go` check 6 | `maintenanceRate` | `e.getMaintenanceRate()` | Yes — 4-level chain: cache hit -> adapter GetMaintenanceRate -> ContractInfo -> conservative default 0.05; rate=0 never silently passes | ✓ FLOWING |
| `internal/spotengine/exit_manager.go` trigger 2b | `maintenanceRate` | `e.getMaintenanceRate()` | Yes — same 4-level chain, uses current mark price from orderbook (ob.Bids[0]+ob.Asks[0])/2 | ✓ FLOWING |
| `internal/risk/health.go` HealthAction.SpotPositions | `spotPositions` | `h.db.GetActiveSpotPositions()` -> Redis | Yes — real Redis query; fail-open on error (continues with nil, perp-perp monitoring unblocked) | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| All exchange maintenance tests pass | `go test ./pkg/exchange/... -run TestMaintenance -count=1` | 7 tests pass | ✓ PASS |
| SpotEngine maintenance helper tests pass | `go test ./internal/spotengine/ -run TestMaintenance -count=1` | 15 tests pass | ✓ PASS |
| Risk gate maintenance gate tests pass | `go test ./internal/spotengine/ -run TestMaintenanceRateGate -count=1` | 14 tests pass | ✓ PASS |
| Liq distance trigger tests pass | `go test ./internal/spotengine/ -run TestLiqDistance -count=1` | 7 tests pass | ✓ PASS |
| Health monitor spot integration tests pass | `go test ./internal/risk/ -run TestHealthMonitor -count=1` | 4 tests pass | ✓ PASS |
| Discovery maintenance tests pass | `go test ./internal/spotengine/ -run TestDiscoveryMaintenance -count=1` | 3 tests pass | ✓ PASS |
| Full spotengine test suite | `go test ./internal/spotengine/ -count=1` | 190 tests pass | ✓ PASS |
| Full risk+engine test suites | `go test ./internal/risk/ ./internal/engine/ -count=1` | 87 tests pass | ✓ PASS |
| go vet clean on all modified packages | `go vet ./pkg/exchange/... ./internal/...` | No issues | ✓ PASS |
| Full binary build | `go build ./cmd/main.go` | Succeeds | ✓ PASS |

### Requirements Coverage

The plan frontmatter references requirement IDs SF-RISK-01 through SF-RISK-04. These IDs are **not defined in REQUIREMENTS.md** — they are not listed in the v1 requirements section and do not appear in the traceability table. The traceability table only tracks Phase 1-5 requirements (SF-01..07, PP-01..04, AN-01..06, CA-01..04).

The Phase 6 work implements against ROADMAP.md success criteria, which serve as the de-facto requirements for this phase:

| Requirement ID (plan frontmatter) | ROADMAP Success Criterion | Plan Coverage | Status |
|-----------------------------------|--------------------------|---------------|--------|
| SF-RISK-01 | "Pre-entry check rejects positions where survivable drop < threshold" | Plan 01 (data) + Plan 02 (gate) | ✓ SATISFIED — code verified |
| SF-RISK-02 | "Runtime monitor calculates liq distance, triggers exit below threshold" | Plan 01 (data) + Plan 03 Task 1 | ✓ SATISFIED — code verified |
| SF-RISK-03 | "Health monitor evaluates spot-futures positions (L3/L4/L5)" | Plan 03 Task 2 | ✓ SATISFIED — code verified |
| SF-RISK-04 | "Discovery displays maintenance_rate per contract" | Plan 04 | ✓ SATISFIED — code verified |

**Documentation gap (non-blocking):** SF-RISK-01..04 are not added to REQUIREMENTS.md traceability table. The requirements exist only in ROADMAP.md and plan frontmatter. The implementation is correct, but REQUIREMENTS.md should be updated to include Phase 6 requirements for traceability completeness.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None | — | No TODO/FIXME/placeholder/stub patterns found in any of the 22 modified files | — | — |

### Human Verification Required

#### 1. Pre-Entry Gate End-to-End Rejection

**Test:** Enable `enable_maintenance_gate: true` in config, identify a high-maintenance-rate contract (e.g., GUAUSDT if available for spot-futures on Gate.io), trigger auto-discovery scan at :35 minute mark
**Expected:** Auto-entry attempt is logged with `maintenance_survivable_X%<Y%` reason and entry is NOT made; no open position appears in dashboard
**Why human:** Requires live exchange connectivity, funded account, and a real high-maintenance-rate contract to be discovered — cannot replicate live exchange conditions or real tiered rate data programmatically

#### 2. Liquidation Distance Trigger Activation

**Test:** With an active spot-futures position approaching liquidation (or by temporarily lowering SpotFuturesMaintenanceDefault to simulate), monitor exit_manager logs during a monitor cycle
**Expected:** `exit trigger: SYMBOL liq distance X% < Y%` log line appears at warn/exit/emergency level based on actual distance
**Why human:** Requires a live position and controlled price movement; simulating mark price via orderbook manipulation is not possible in integration testing

#### 3. Dashboard Maintenance Rate Column Display

**Test:** Load the Opportunities page on the running dashboard when discovery has returned results for multiple contracts
**Expected:** Maintenance Rate column shows color-coded values: red for contracts with >= 10% maintenance rate, amber for 5-10%, gray for < 5%, dash for zero/unknown; column header shows "MR" (compact) or "Maint %" (full view) with tooltip on hover
**Why human:** Visual rendering requires a browser and live dashboard; cannot verify column rendering or color accuracy programmatically

### Gaps Summary

No blocking gaps found. All 4 ROADMAP success criteria are fully implemented, wired, and unit-tested. The full build compiles cleanly, all 43+ maintenance-related tests pass across 6 test suites, and no anti-patterns are present in any of the 22 modified files.

One documentation gap exists: SF-RISK-01..04 are not added to REQUIREMENTS.md — they appear only in ROADMAP.md and plan frontmatter. This does not affect runtime behavior but breaks traceability completeness.

---

_Verified: 2026-04-05T15:30:00Z_
_Verifier: Claude (gsd-verifier)_
