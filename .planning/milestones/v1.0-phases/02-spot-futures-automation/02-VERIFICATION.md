---
phase: 02-spot-futures-automation
verified: 2026-04-02T09:08:05Z
status: human_needed
score: 6/7 must-haves verified
re_verification: false
human_verification:
  - test: "Build system, open dashboard, verify all 9 config fields in correct tabs with correct defaults"
    expected: "Discovery tab: Native Scanner toggle (ON) + Entry Basis Gate + Max Basis %. Exit tab: Min Hold Gate (OFF, 8h) + Settlement Window Guard (OFF, 10min) + Exit Spread Gate (OFF, 0.3%). Conditional dimming active on associated number fields when parent toggle is OFF."
    why_human: "Visual layout and toggle behavior require browser inspection; automated checks confirm presence in code but not rendered position, default values, or opacity behavior"
  - test: "Toggle language to zh-TW in dashboard and verify all 9 new config labels render in Traditional Chinese"
    expected: "All new labels show Chinese text (e.g., '原生掃描器', '最低持倉門檻', '開倉基差門檻', '平倉價差門檻')"
    why_human: "i18n rendering in browser cannot be verified by static analysis"
  - test: "With SpotFuturesAutoEnabled=true and SpotFuturesDryRun=true, observe logs for auto-entry dry-run messages"
    expected: "Log shows: 'auto-entry: BLOCKED XXXUSDT on exchange — dry_run' confirming SF-05 pipeline executed"
    why_human: "Requires running system with live or injected opportunities; log output is runtime behavior"
  - test: "Opportunities page shows Source indicator when spot opportunities are present"
    expected: "'Source: Native' label visible in the summary bar when native scanner is ON and finds opportunities"
    why_human: "Requires running system with live Loris data; static analysis confirms the conditional render code is correct"
  - test: "Config change persists through restart (Redis persistence)"
    expected: "Set Min Hold Hours to 12, restart system, verify value is still 12"
    why_human: "Requires running system with Redis; cannot verify SetConfigField persistence without executing the server"
---

# Phase 02: Spot-Futures Automation Verification Report

**Phase Goal:** The spot-futures engine autonomously discovers, opens, and exits positions without manual intervention
**Verified:** 2026-04-02T09:08:05Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Success Criteria from ROADMAP.md

The ROADMAP.md defines 4 observable success criteria for Phase 2:

1. System automatically discovers and ranks spot-futures opportunities across all 5 exchanges, visible in the Opportunities dashboard
2. System auto-opens the best spot-futures opportunity when it meets entry thresholds, without user clicking anything
3. Positions auto-exit when exit conditions are met, handling blackout windows, partial fills, and emergency close scenarios
4. Entry is rejected when basis/spread is too wide, and exit is gated by spread threshold — user can see the spread reason in logs

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Native scanner produces ranked SpotArbOpportunity list from Loris API across all exchanges | VERIFIED | `runNativeDiscoveryScan()` in discovery.go polls `https://api.loris.tools/funding`, iterates all `spotMargin` exchanges, produces Dir A + Dir B opportunities ranked by NetAPR |
| 2 | Net yield calculated as fundingAPR - borrowAPR - feeAPR | VERIFIED | discovery.go line 179: `rawRate / 8.0` normalization confirmed; formula verified in 10 passing unit tests including TestNativeScannerLorisNormalization |
| 3 | Both Dir A and Dir B opportunities generated when funding positive | VERIFIED | Source: "native" + "borrow_sell_long" + "buy_spot_short" all confirmed present; TestNativeScannerBothDirections passes |
| 4 | CoinGlass is fallback when Loris unavailable | VERIFIED | `runDiscoveryScan()` routes to `runCoinGlassFallback()` when `SpotFuturesNativeScannerEnabled=false` or on Loris error |
| 5 | Opportunities visible in dashboard (SC1) | VERIFIED | `discoveryLoop()` in engine.go calls `e.api.SetSpotOpportunities()` + `BroadcastSpotOpportunities()` after every scan; Opportunities.tsx renders `spotOpportunities` prop with source indicator |
| 6 | Auto-open fires without user intervention (SC2) | VERIFIED | `discoveryLoop()` → `filterPassed()` → `attemptAutoEntries()` → `checkRiskGate()` → `ManualOpen()` chain confirmed in engine.go; TestAutoEntry_DryRun passes confirming wiring |
| 7 | Auto-exit handles blackout windows, partial fills, emergency close, pending repay retry (SC3) | VERIFIED | `exit_manager.go`: emergency-first trigger ordering (Phase 1 safety > Phase 2 guards > Phase 3 yield); `monitor.go`: `retryPendingRepay()` handles Bybit blackout deferral and stuck exit retry; `initiateExit()` handles PendingRepay persistence |
| 8 | Entry rejected when basis too wide; user sees reason in logs (SC4) | VERIFIED | `checkRiskGate()` check 5 returns `basis_%.2f%%>%.2f%%` reason; `attemptAutoEntries()` logs `BLOCKED %s on %s — %s`; TestBasisGateEntry all 6 subtests pass including fail-closed on error |
| 9 | Exit gated by spread threshold | VERIFIED | `checkExitTriggers()` Phase 2 guard: `SpotFuturesEnableExitSpreadGate` → `estimateUnwindSlippage()` → defer if above threshold; TestExitSpreadGate passes |
| 10 | Min-hold gate blocks young positions from yield-based exit | VERIFIED | `checkExitTriggers()` Phase 2: `SpotFuturesEnableMinHold` check before yield triggers; TestMinHoldGate all 3 subtests pass |
| 11 | Settlement window guard defers exits during settlement | VERIFIED | `isInSettlementWindowAt()` uses hour%8 arithmetic; TestSettlementGuard 9 subtests all pass |
| 12 | Emergency exits bypass all guards | VERIFIED | Phase 1 (price spike, margin health) evaluated BEFORE Phase 2 guards in `checkExitTriggers()`; TestEmergencyBypass passes |

**Score:** 12/12 automated truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/config.go` | 9 new SpotFutures* config fields with 6 touch-points | VERIFIED | Struct fields, JSON tags, defaults, apply, toJSON, fromEnv — all 9 confirmed at lines 1063-1087 (apply), 1328-1336 (toJSON), 1541-1575 (fromEnv) |
| `internal/spotengine/discovery.go` | Native Loris scanner replacing CoinGlass as primary | VERIFIED | `pollLoris()`, `runNativeDiscoveryScan()`, `runCoinGlassFallback()`, `runDiscoveryScan()` router all confirmed; `rawRate / 8.0` normalization present |
| `internal/spotengine/native_scanner_test.go` | Unit tests, min 100 lines | VERIFIED | 545 lines, 10 test functions covering Dir A/B, normalization, fallback, filter status, source field, config defaults |
| `internal/spotengine/exit_manager.go` | Min-hold gate, settlement window guard, exit spread gate | VERIFIED | All 3 guards at lines 212-248; `isInSettlementWindow()` + `isInSettlementWindowAt()` + `estimateUnwindSlippage()` helpers present |
| `internal/spotengine/risk_gate.go` | Basis gate as check 5, before dry-run check 6 | VERIFIED | `SpotFuturesEnableBasisGate` at line 59; `calculateEntryBasis()` at line 92; `basis_check_error` fail-closed; `basis_%.2f%%>%.2f%%` rejection format |
| `internal/spotengine/exit_guards_test.go` | Tests for min-hold, settlement window, emergency bypass, min 80 lines | VERIFIED | 475 lines; TestMinHoldGate, TestSettlementGuard, TestEmergencyBypass, TestExitSpreadGate, TestEstimateUnwindSlippage, TestIsInSettlementWindow all present |
| `internal/spotengine/basis_spread_test.go` | Tests for entry basis gate, exit spread gate, min 60 lines | VERIFIED | 302 lines; TestBasisGateEntry, TestBasisGateBeforeDryRun, TestBasisGateDryRunPassthrough, TestCalculateEntryBasis, TestCalculateEntryBasis_ExchangeNotFound all present |
| `internal/api/spot_handlers.go` | Extended GET/POST handlers for 9 new config fields | VERIFIED | `spotAutoConfigResponse()` returns all 9 fields; POST req struct has 9 pointer fields; all 9 call `s.db.SetConfigField()` for Redis persistence |
| `internal/spotengine/autoentry_test.go` | TestAutoEntry_DryRun confirming SF-05 pipeline wiring | VERIFIED | TestAutoEntry_DryRun passes — verifies `checkRiskGate()` returns `dry_run` reason; TestAutoEntry_Disabled + TestAutoEntry_AtCapacity also present |
| `web/src/pages/Config.tsx` | 5 toggles + 4 number fields in sf-discovery and sf-exit tabs | VERIFIED | `native_scanner_enabled`, `enable_basis_gate`, `enable_min_hold`, `enable_settlement_guard`, `enable_exit_spread_gate` toggles confirmed; `opacity-50` conditional dimming at lines 1368, 1517, 1543, 1569 |
| `web/src/i18n/en.ts` | ~20 new translation keys | VERIFIED | All keys confirmed: `cfg.sf.enableMinHold`, `cfg.sf.nativeScannerEnabled`, `cfg.sf.enableBasisGate`, `cfg.sf.enableExitSpreadGate`, `spot.sourceNative` etc. |
| `web/src/i18n/zh-TW.ts` | Matching Traditional Chinese translations | VERIFIED | All matching keys confirmed: '原生掃描器', '最低持倉門檻', '原生' etc. |
| `web/src/pages/Opportunities.tsx` | Data source indicator in summary bar | VERIFIED | Lines 202-210: conditional render of `t('spot.sourceNative')` / `t('spot.sourceCoinGlass')` / `t('spot.sourceFallback')` based on `spotOpportunities[0].source` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/spotengine/discovery.go` | `internal/config/config.go` | `e.cfg.SpotFuturesNativeScannerEnabled` | VERIFIED | Line 73: gates native vs CoinGlass routing |
| `internal/spotengine/discovery.go` | `https://api.loris.tools/funding` | HTTP GET poll in `pollLorisFromURL()` | VERIFIED | `lorisURL` const at line 68; HTTP client decode to `models.LorisResponse` |
| `internal/spotengine/discovery.go` | `getCachedBorrowRate` | function call per symbol+exchange | VERIFIED | Line 196: `e.getCachedBorrowRate(exchName, baseCoin, smExch)` for Dir A |
| `internal/spotengine/engine.go` | `internal/spotengine/autoentry.go` | `attemptAutoEntries(passed)` | VERIFIED | Lines 155-160, 168-173, 183-186: three call sites in discoveryLoop |
| `internal/spotengine/exit_manager.go` | `internal/config/config.go` | `e.cfg.SpotFuturesEnableMinHold`, `...SettlementGuard`, `...ExitSpreadGate` | VERIFIED | Lines 212, 224, 239 in `checkExitTriggers()` |
| `internal/spotengine/risk_gate.go` | `internal/config/config.go` | `e.cfg.SpotFuturesEnableBasisGate` | VERIFIED | Line 59 in `checkRiskGate()` |
| `web/src/pages/Config.tsx` | `/api/spot/config/auto` | POST fetch for config updates | VERIFIED | `handleBoolChange()` / `handleChange()` → `['spot_futures', 'native_scanner_enabled']` path wired |
| `internal/api/spot_handlers.go` | `internal/config/config.go` | `s.cfg.SpotFutures*` field reads and writes | VERIFIED | `spotAutoConfigResponse()` reads all 9 fields; POST handler writes all 9 fields |
| `web/src/i18n/en.ts` | `web/src/pages/Config.tsx` | `t()` translation function | VERIFIED | All new `cfg.sf.*` keys used via `t('cfg.sf.enableMinHold')` pattern in Config.tsx |
| `internal/spotengine/autoentry.go` | `internal/spotengine/risk_gate.go` | `checkRiskGate()` for each opportunity | VERIFIED | Line 23: `result := e.checkRiskGate(opp)` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `discovery.go: runNativeDiscoveryScan()` | `loris.FundingRates`, `loris.Symbols` | HTTP GET to `api.loris.tools/funding` → `models.LorisResponse` | Yes — live API JSON decoded into struct | FLOWING |
| `engine.go: discoveryLoop()` | `opps []SpotArbOpportunity` | `runDiscoveryScan()` → native scanner output | Yes — real Loris data + exchange borrow rates | FLOWING |
| `Opportunities.tsx` | `spotOpportunities` prop | WebSocket `spot_opportunities` push + `SetSpotOpportunities` REST seed | Yes — populated by `BroadcastSpotOpportunities()` in engine after each scan | FLOWING |
| `spot_handlers.go: spotAutoConfigResponse()` | 9 new config fields | `s.cfg.SpotFutures*` live config struct | Yes — reads from in-memory config populated from Redis/env on startup | FLOWING |
| `autoentry.go: attemptAutoEntries()` | `opps []SpotArbOpportunity` | `filterPassed(runDiscoveryScan())` | Yes — same live scan output | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| All spotengine tests pass | `go test ./internal/spotengine/... -count=1` | 152 tests, 0 failures | PASS |
| Go binary compiles | `go build ./...` | Exit 0, no errors | PASS |
| TestAutoEntry_DryRun: SF-05 pipeline wiring | `-run TestAutoEntry_DryRun -v` | PASS (0.01s) — dry_run gate reached | PASS |
| TestBasisGateEntry: 6 scenarios including fail-closed | `-run TestBasisGateEntry -v` | PASS — all 6 subtests | PASS |
| TestMinHoldGate: blocks young, allows old, skips when OFF | `-run TestMinHoldGate -v` | PASS — all 3 subtests | PASS |
| TestSettlementGuard: window detection | `-run TestSettlementGuard -v` | PASS — all 9 subtests | PASS |
| TestEmergencyBypass: emergency fires despite min-hold | `-run TestEmergencyBypass -v` | PASS | PASS |
| TestNativeScanner*: Dir A/B, normalization, fallback | `-run TestNativeScanner -v` | PASS — all 10 functions | PASS |
| Dashboard visual/functional inspection | Browser + running server | Not tested — see human verification | SKIP (needs human) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|---------|
| SF-04 | 02-01 (primary), 02-03 | Auto-discovery pipeline finds and ranks opportunities across all 5 exchanges | SATISFIED | `runNativeDiscoveryScan()` polls Loris, iterates all `spotMargin` exchanges, ranks by NetAPR; opportunities broadcast to dashboard via `BroadcastSpotOpportunities()` |
| SF-05 | 02-03 | Auto-open pipeline executes best opportunities without manual intervention | SATISFIED | `discoveryLoop()` → `filterPassed()` → `attemptAutoEntries()` → `checkRiskGate()` → `ManualOpen()` chain; TestAutoEntry_DryRun confirms wiring |
| SF-06 | 02-02 (primary) | Auto-exit handles all edge cases (blackout windows, partial fills, emergency close, pending repay retry) | SATISFIED | `checkExitTriggers()` emergency-first ordering; `initiateExit()` PendingRepay/blackout handling; `monitorLoop()` → `retryPendingRepay()` for stuck exits; existing exit_manager coverage |
| SF-07 | 02-02 (primary), 02-03 | Basis/spread control on entry (reject if too wide) and exit (threshold-gated close) | SATISFIED | Basis gate as `checkRiskGate()` check 5 with fail-closed; exit spread gate in `checkExitTriggers()` Phase 2; TestBasisGateEntry + TestExitSpreadGate both pass |

All 4 requirements for Phase 2 are SATISFIED. No orphaned requirements found (REQUIREMENTS.md traceability table already marks SF-04/05/06/07 as Complete for Phase 2).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | - | - | - | - |

No TODO/FIXME/placeholder comments found in any modified files. No hardcoded empty returns in production paths. No props hardcoded to empty arrays/objects. The `return "", false` at the end of `checkExitTriggers()` is a legitimate fallthrough (no trigger matched), not a stub.

### Human Verification Required

The automated checks have all passed. The following items need human visual and functional inspection.

#### 1. Dashboard Config Page Layout

**Test:** Build and run the system (`make build && make dev`), open the Config page, navigate to Spot-Futures > Discovery tab.
**Expected:** "Native Scanner" toggle appears at the top of the Discovery tab (default ON); "Entry Gates" section below persistence fields with "Entry Basis Gate" toggle and dimmed "Max Basis %" field. Navigate to Exit tab — "Exit Guards" section with Min Hold Gate (default OFF, 8h dimmed), Settlement Window Guard (default OFF, 10min dimmed), Exit Spread Gate (default OFF, 0.3% dimmed). Toggling each guard ON should un-dim the associated number field.
**Why human:** Visual layout and interactive toggle behavior (conditional opacity-50 dimming) require browser inspection. Static analysis confirms the code patterns are correct but cannot verify the rendered tab structure or interactive behavior.

#### 2. Traditional Chinese Translations Render Correctly

**Test:** Switch dashboard language to zh-TW.
**Expected:** All 9 new field labels and descriptions appear in Traditional Chinese (e.g., '原生掃描器', '最低持倉門檻', '結算窗口保護', '開倉基差門檻', '平倉價差門檻').
**Why human:** i18n rendering in browser; static analysis confirmed key presence in zh-TW.ts but not visual rendering.

#### 3. Auto-Entry Dry-Run Pipeline Visible in Logs

**Test:** In Config, set SpotFuturesAutoEnabled=true, SpotFuturesDryRun=true. Run system. Check logs.
**Expected:** Log line: `auto-entry: BLOCKED XXXUSDT on exchange — dry_run` confirming the SF-05 pipeline ran.
**Why human:** Requires a running system with live Loris data and opportunities that pass all real risk gate checks (capacity, duplicate, cooldown, persistence). Market conditions may mean no opportunities are found during the inspection window — in that case verify at least the scan cycle runs without errors.

#### 4. Opportunities Page Source Indicator

**Test:** With native scanner enabled and opportunities present, open Opportunities page.
**Expected:** "Source: Native" label appears in the summary bar next to the actionable/filtered counts.
**Why human:** Requires live Loris API returning data; static analysis confirms the conditional render code is correct (`spotOpportunities[0].source === 'native'`).

#### 5. Config Persistence Through Restart

**Test:** Change Min Hold Hours to 12, save, restart the system, reopen Config.
**Expected:** Min Hold Hours shows 12 (not the default 8), confirming Redis persistence via `SetConfigField()`.
**Why human:** Requires running system with Redis; `SetConfigField()` calls are confirmed in code but persistence requires actual execution.

### Gaps Summary

No blocking gaps. All automated checks pass. The 5 human verification items above are standard operational checks that cannot be verified programmatically — they concern visual layout, browser interactivity, runtime log output, live API data, and Redis persistence across restarts.

Phase 2 goal achievement summary:
- SF-04 (auto-discovery): Native Loris scanner fully implemented, tested (10 unit tests), wired to dashboard broadcast
- SF-05 (auto-open): Full pipeline wired and confirmed by TestAutoEntry_DryRun
- SF-06 (auto-exit edge cases): Emergency-first trigger ordering + blackout deferral + repay retry all in place
- SF-07 (basis/spread gating): Basis gate (entry) + spread gate (exit) both implemented, tested, and configurable

The only outstanding item is the human approval checkpoint (Plan 03 Task 3) which was noted as "automated verification passed, human inspection pending" in the SUMMARY.

---

_Verified: 2026-04-02T09:08:05Z_
_Verifier: Claude (gsd-verifier)_
