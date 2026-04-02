---
phase: 01-spot-futures-exchange-expansion
verified: 2026-04-02T02:46:38Z
status: human_needed
score: 9/10 must-haves verified
human_verification:
  - test: "Run test-lifecycle on Gate.io and OKX live and confirm structured JSON report shows all steps ok"
    expected: "DirA.Open.Status=ok, DirA.VerifyOpen.Borrowed.Found=true, DirA.VerifyClose.Borrowed.Found=false, DirB.Open.Status=ok, DirB.VerifyOpen.SpotBalance.Found=true, DirB.VerifyClose.SpotBalance.Amount<=initial on both exchanges"
    why_human: "01-03-SUMMARY has status:checkpoint-pending for Gate.io and OKX Task 2 (live trade human checkpoint). Test-lifecycle commit (6d83f7e) claims BTCUSDT passes all 5 exchanges both directions, but the formal checkpoint approval was never recorded in the summary. Human needs to confirm the test-lifecycle automated run constitutes sufficient approval or re-run manually."
---

# Phase 1: Spot-Futures Exchange Expansion Verification Report

**Phase Goal:** Both spot-futures directions (Dir A borrow-sell-long, Dir B buy-spot-short) work reliably on all 5 margin exchanges
**Verified:** 2026-04-02T02:46:38Z
**Status:** human_needed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| #  | Truth                                                                                     | Status      | Evidence                                                                                                 |
|----|-------------------------------------------------------------------------------------------|-------------|----------------------------------------------------------------------------------------------------------|
| 1  | Livetest harness has PlaceSpotMarginOrder (Dir A + Dir B) and GetSpotMarginOrder tests   | VERIFIED    | `totalTests=28`, tests 27-28 present in `cmd/livetest/main.go` (lines 913, 1101)                       |
| 2  | Bybit passes all 28 livetest tests including margin trade tests                           | VERIFIED    | 01-01-SUMMARY self-check PASSED; commits dd7bbc7, 3575bdf, b45c199 confirmed in git                    |
| 3  | Bybit Dir A and Dir B manually verified with live trades (checkpoint approved)            | VERIFIED    | 01-01-SUMMARY: Task 3 "Complete (checkpoint approved)"                                                  |
| 4  | Binance passes all 28 livetest tests including margin trade tests                         | VERIFIED    | 01-02-SUMMARY self-check PASSED; commit 34bbef0 confirmed in git                                       |
| 5  | Bitget passes all 28 livetest tests including margin trade tests                          | VERIFIED    | 01-02-SUMMARY self-check PASSED; commit 34bbef0 confirmed in git                                       |
| 6  | Binance and Bitget Dir A and Dir B manually verified with live trades (checkpoint approved) | VERIFIED  | 01-02-SUMMARY: Task 2 "Complete (checkpoint approved)"                                                  |
| 7  | Gate.io passes all 28 livetest tests including margin trade tests                         | VERIFIED    | 01-03-SUMMARY: "Gate.io passes all 28 livetest tests"; commit b8a93d8 confirmed in git                 |
| 8  | OKX passes 27/28 livetest tests (Test 25 MarginBorrow expected fail in Futures mode)     | VERIFIED    | 01-03-SUMMARY: "OKX passes 27/28"; Test 25 by design — autoLoan replaces explicit borrow for OKX      |
| 9  | All 5 exchanges pass BTCUSDT test-lifecycle for both Dir A and Dir B                     | VERIFIED    | commit 6d83f7e: "Verified: BTCUSDT test-lifecycle passes all 5 exchanges, both directions"             |
| 10 | Gate.io and OKX Dir A and Dir B human checkpoint formally approved in 01-03-SUMMARY      | ? UNCERTAIN | 01-03-SUMMARY status=checkpoint-pending; test-lifecycle run serves as functional equivalent but formal approval not recorded |

**Score:** 9/10 truths verified (1 uncertain — formal Gate.io/OKX checkpoint approval)

---

### Required Artifacts

| Artifact                              | Expected                                        | Status      | Details                                                                 |
|---------------------------------------|-------------------------------------------------|-------------|-------------------------------------------------------------------------|
| `cmd/livetest/main.go`                | Tests 27-28 for PlaceSpotMarginOrder/GetSpotMarginOrder | VERIFIED | `totalTests=28`, AutoBorrow:true, AutoRepay:true, SpotMarginOrderQuerier assertion present |
| `pkg/exchange/bybit/margin.go`        | PlaceSpotMarginOrder with marketUnit=baseCoin   | VERIFIED    | marketUnit=baseCoin at line 114; compile-time SpotMarginExchange check  |
| `pkg/exchange/binance/margin.go`      | sideEffectType AUTO_BORROW_REPAY + quoteOrderQty | VERIFIED   | sideEffect="AUTO_BORROW_REPAY" at line 69; quoteOrderQty at line 78; /api/v3/myTrades fee query |
| `pkg/exchange/bitget/margin.go`       | loanType autoLoan + FlashRepayer interface      | VERIFIED    | loanType mapping at lines 111-115; FlashRepay method at line 44         |
| `pkg/exchange/gateio/margin.go`       | auto_borrow/auto_repay bool + ioc for market orders | VERIFIED | auto_borrow=true at line 99; time_in_force=ioc at line 77               |
| `pkg/exchange/okx/margin.go`          | EnsureAutoLoan + tdMode=cross + ccy=USDT        | VERIFIED    | EnsureAutoLoan at line 35; tdMode=cross at line 116; ccy=USDT at line 117 |
| `pkg/exchange/okx/adapter.go`         | lotSz cache for fractional contract sizing      | VERIFIED    | lotSzCache sync.Map at line 47; math.Floor(contracts/lotSz)*lotSz at line 186 |
| `internal/spotengine/execution.go`    | needsMarginTransfer, both Size+QuoteSize, fee deduction, collateral polling | VERIFIED | needsMarginTransfer at line 26; QuoteSize+Size both passed at line 509; FeeDeducted at line 545; MaxBorrowable polling at line 205 |
| `internal/spotengine/exit_manager.go` | TransferToFutures for Dir B exit on separate-account exchanges | VERIFIED | TransferToFutures call at line 472 |
| `internal/engine/consolidate.go`      | Spot-futures futures legs excluded from orphan detection | VERIFIED | exclusion set built at line 131-163 |
| `pkg/exchange/types.go`               | FeeDeducted field + FlashRepayer interface      | VERIFIED    | FeeDeducted at line 319; FlashRepayer at line 361                       |
| `internal/api/spot_handlers.go`       | /api/spot/test-lifecycle automated lifecycle test | VERIFIED  | handleSpotTestLifecycle at line 427; registered in server.go line 115   |
| `CHANGELOG.md`                        | Entries for v0.22.51 and v0.22.54               | VERIFIED    | v0.22.51 at line 87; v0.22.54 at line 47                                |
| `VERSION`                             | Bumped per convention (now 0.24.1)              | VERIFIED    | Bumped multiple times; currently 0.24.1                                 |

---

### Key Link Verification

| From                                    | To                                              | Via                                        | Status   | Details                                                                      |
|-----------------------------------------|-------------------------------------------------|--------------------------------------------|----------|------------------------------------------------------------------------------|
| `cmd/livetest/main.go`                  | `pkg/exchange/types.go`                         | SpotMarginExchange + SpotMarginOrderQuerier | WIRED   | PlaceSpotMarginOrder call + SpotMarginOrderQuerier type assertion present     |
| `internal/spotengine/execution.go`      | `pkg/exchange/bybit/margin.go`                  | PlaceSpotMarginOrder(AutoBorrow:true)       | WIRED    | AutoBorrow:true in executeBorrowSellLong; marketUnit=baseCoin in Bybit adapter |
| `internal/spotengine/execution.go`      | `pkg/exchange/binance/margin.go`                | PlaceSpotMarginOrder(AutoBorrow:true) + TransferToMargin | WIRED | needsMarginTransfer("binance")=true; sideEffectType=AUTO_BORROW_REPAY wired |
| `internal/spotengine/execution.go`      | `pkg/exchange/bitget/margin.go`                 | PlaceSpotMarginOrder(AutoBorrow:true) + FlashRepayer | WIRED | needsMarginTransfer("bitget")=true; loanType=autoLoan; FlashRepay for Dir A close |
| `internal/spotengine/execution.go`      | `pkg/exchange/gateio/margin.go`                 | PlaceSpotMarginOrder(AutoBorrow:true)       | WIRED    | auto_borrow=true; market ioc time-in-force; unified account                  |
| `pkg/exchange/okx/margin.go`            | OKX API /api/v5/account/set-auto-loan          | EnsureAutoLoan() called on AutoBorrow       | WIRED    | EnsureAutoLoan called at PlaceSpotMarginOrder line 107                       |
| `internal/spotengine/execution.go`      | `pkg/exchange/okx/margin.go`                   | tdMode=cross + ccy=USDT                     | WIRED    | PlaceSpotMarginOrder uses tdMode=cross+ccy=USDT; handles borrow implicitly   |
| `internal/engine/consolidate.go`        | `internal/database/spot_state.go`              | exclusion set from active spot positions    | WIRED    | GetActiveSpotPositions() at line 132; exclusion checked at line 163          |

---

### Data-Flow Trace (Level 4)

| Artifact                              | Data Variable     | Source                                    | Produces Real Data | Status   |
|---------------------------------------|-------------------|-------------------------------------------|--------------------|----------|
| `internal/api/spot_handlers.go`       | lifecycleReport   | spotOpenPosition + spotClosePosition callbacks | Yes — real exchange orders | FLOWING |
| `internal/spotengine/execution.go`    | SpotSize          | PlaceSpotMarginOrder fill - FeeDeducted   | Yes — from GetSpotMarginOrder fills query | FLOWING |
| `pkg/exchange/binance/margin.go`      | FeeDeducted       | /api/v3/myTrades or /sapi/v1/margin/myTrades | Yes — live trade fills | FLOWING |
| `pkg/exchange/bitget/margin.go`       | FeeDeducted       | /api/v2/spot/trade/fills or margin fills  | Yes — live trade fills | FLOWING |
| `pkg/exchange/bybit/margin.go`        | FeeDeducted       | /v5/execution/list                        | Yes — live execution list | FLOWING |
| `pkg/exchange/gateio/margin.go`       | FeeDeducted       | /spot/my_trades                           | Yes — live trade fills | FLOWING |
| `pkg/exchange/okx/margin.go`          | FeeDeducted       | /api/v5/trade/fills                       | Yes — live trade fills | FLOWING |

---

### Behavioral Spot-Checks

| Behavior                              | Command                                                    | Result                                     | Status |
|---------------------------------------|------------------------------------------------------------|--------------------------------------------|--------|
| Full project builds                   | `go build ./...`                                           | Exit 0, no output                          | PASS   |
| Bybit unit tests pass                 | `go test ./pkg/exchange/bybit/ -count=1`                  | ok arb/pkg/exchange/bybit 0.021s           | PASS   |
| Bitget unit tests pass                | `go test ./pkg/exchange/bitget/ -count=1`                 | ok arb/pkg/exchange/bitget 0.018s          | PASS   |
| Gate.io unit tests pass               | `go test ./pkg/exchange/gateio/ -count=1`                 | ok arb/pkg/exchange/gateio 0.024s          | PASS   |
| OKX unit tests pass                   | `go test ./pkg/exchange/okx/ -count=1`                    | ok arb/pkg/exchange/okx 0.024s             | PASS   |
| SpotEngine unit tests pass            | `go test ./internal/spotengine/ -count=1`                 | ok arb/internal/spotengine 58.947s         | PASS   |
| totalTests = 28 in livetest           | grep in cmd/livetest/main.go                               | `const totalTests = 28`                    | PASS   |
| All 5 adapters implement SpotMarginExchange | compile-time check `var _ exchange.SpotMarginExchange = (*Adapter)(nil)` | Present in all 5 margin.go files | PASS |

---

### Requirements Coverage

| Requirement | Source Plan         | Description                                                              | Status           | Evidence                                                                                        |
|-------------|---------------------|--------------------------------------------------------------------------|------------------|-------------------------------------------------------------------------------------------------|
| SF-01       | 01-01, 01-02, 01-03 | Dir A (borrow-sell-long) full lifecycle on all 5 exchanges               | SATISFIED        | All 5 adapters: PlaceSpotMarginOrder with AutoBorrow:true wired; BTCUSDT test-lifecycle passes all 5 exchanges Dir A |
| SF-02       | 01-01, 01-02, 01-03 | Dir B (buy-spot-short) full lifecycle on all 5 exchanges                 | SATISFIED        | All 5 adapters: PlaceSpotMarginOrder with QuoteSize/Size for market BUY; BTCUSDT test-lifecycle passes all 5 exchanges Dir B |
| SF-03       | 01-01, 01-02, 01-03 | Auto-borrow/repay via exchange-native mechanisms per exchange             | SATISFIED        | Bybit: isLeverage=1 + omit-amount repay; Binance: sideEffectType=AUTO_BORROW_REPAY; Bitget: loanType=autoLoan+flash-repay; Gate.io: auto_borrow/auto_repay bool in order body; OKX: EnsureAutoLoan (account-level, explicitly cited in SF-03 description as valid) |

All three requirement IDs (SF-01, SF-02, SF-03) are claimed by plans 01-01, 01-02, and 01-03. No orphaned requirements were found.

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | — | No TODO/FIXME/placeholder patterns in any margin adapter | — | — |

No empty implementations, hardcoded empty arrays, or orphaned state variables were found in the margin adapter or execution engine files verified.

---

### Human Verification Required

#### 1. Gate.io and OKX Formal Checkpoint Approval

**Test:** Run `POST /api/spot/test-lifecycle` with `{"symbol":"BTCUSDT","exchange":"gateio"}` and `{"symbol":"BTCUSDT","exchange":"okx"}` via the running bot. Inspect the returned JSON report for both Dir A and Dir B.

**Expected:**
- Dir A: `DirA.Open.Status = "ok"`, `DirA.VerifyOpen.Borrowed.Found = true` (borrow present after open), `DirA.VerifyClose.Borrowed.Found = false` (borrow cleared after close)
- Dir B: `DirB.Open.Status = "ok"`, `DirB.VerifyOpen.SpotBalance.Found = true` (spot coin present), `DirB.VerifyClose.SpotBalance.Amount = 0` (spot sold on exit)
- No `"error"` status fields in any step

**Why human:** The 01-03-SUMMARY has `status: checkpoint-pending` — Task 2 (human-verify live trades on Gate.io and OKX) was never formally marked approved. Commit `6d83f7e` states "Verified: BTCUSDT test-lifecycle passes all 5 exchanges, both directions," which is strong functional evidence, but the formal GSD checkpoint approval was not recorded. A human needs to either (a) run test-lifecycle and confirm the JSON report shows clean results, or (b) confirm the existing commit message constitutes sufficient sign-off.

---

## Gaps Summary

No code-level gaps were found. All three plans (01-01, 01-02, 01-03) completed their automated tasks with verified commits in git. The full build passes, unit tests pass, all 5 adapters implement the SpotMarginExchange interface, and the critical fixes (fee deduction, TransferToMargin sequencing, flash-repay, OKX EnsureAutoLoan, Gate.io ioc, Binance quoteOrderQty, collateral settlement polling, dust sweep, Dir B QuoteSize+Size dual pass, lotSz cache) are all present and wired in the codebase.

The single outstanding item is procedural, not a code deficiency: the formal human-checkpoint sign-off for Gate.io and OKX (01-03 Task 2) was not recorded in the summary. The automated test-lifecycle endpoint runs real live trades with exchange-state verification — this is the functional equivalent of the manual checkpoint described in the plan. A human should confirm this equivalence.

---

_Verified: 2026-04-02T02:46:38Z_
_Verifier: Claude (gsd-verifier)_
