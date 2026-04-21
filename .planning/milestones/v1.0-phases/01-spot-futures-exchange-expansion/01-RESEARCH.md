# Phase 1: Spot-Futures Exchange Expansion - Research

**Researched:** 2026-04-01
**Domain:** Exchange margin adapter verification, spot-futures engine cross-exchange compatibility
**Confidence:** HIGH

## Summary

Phase 1 expands the spot-futures engine from its Bybit-only prototype to all 5 margin exchanges (Bybit, Binance, Bitget, Gate.io, OKX). All 5 margin adapters already exist in `pkg/exchange/{name}/margin.go` and implement the `SpotMarginExchange` interface with compile-time checks. The spotengine execution code (`executeBorrowSellLong`, `executeBuySpotShort`) is generic -- it takes `SpotMarginExchange` + `Exchange` params with no exchange-specific branching. The livetest harness already has 4 margin tests (23-26) that cover read-only probes and optional borrow/repay/transfer tests.

The primary risk is adapter-level bugs: wrong API parameter formatting, incorrect response parsing, exchange-specific behavioral quirks around auto-borrow/auto-repay flags, and precision/rounding issues. The v0.22.44-49 Bybit debugging precedent (integer precision, margin health check, auto-borrow bugs) is the best predictor -- budget 3-5 adapter bugs per exchange. One critical finding: **OKX's margin adapter does NOT handle `AutoBorrow`/`AutoRepay` flags in `PlaceSpotMarginOrder`**, meaning Dir A auto-borrow-on-sell will not work without a fix. OKX uses account-level `autoLoan` setting, so the adapter either needs to set this flag or perform a separate manual borrow before the sell order.

**Primary recommendation:** Execute exchange-by-exchange (Bybit, Binance, Bitget, Gate.io, OKX), livetest first, live trades second. Fix adapter bugs as discovered. OKX will require engine-level divergence for borrow/repay handling.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Execution order: Bybit -> Binance -> Bitget -> Gate.io -> OKX. Easiest/most-validated first, hardest (OKX manual borrow/repay, Gate.io unified mode) last.
- **D-02:** Bybit is NOT fully verified -- it is the first exchange in line, not done. Previous work (v0.22.44-49) got it partially working but edge cases remain untested.
- **D-03:** Thorough verification per exchange: Manual open+close Dir A, Manual open+close Dir B, auto-borrow on entry, auto-repay on exit, edge cases (partial fills, repay retry, emergency close, blackout windows).
- **D-04:** Two-phase testing: Livetest CLI first (per margin API method), then full lifecycle live trades.
- **D-05:** Engine-level exchange branching (`if exchange == "okx"` style) is acceptable in `internal/spotengine/` when exchanges behave fundamentally differently. No purity requirement.

### Claude's Discretion
- Position sizing for test trades
- Bug-fix sequencing within each exchange (Dir A before Dir B or interleave)
- Livetest coverage per margin method (edge cases in livetest vs live trades)

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SF-01 | Dir A (borrow-sell-long) full lifecycle works on all 5 exchanges | All 5 margin adapters exist with `MarginBorrow`, `PlaceSpotMarginOrder(AutoBorrow)`, `MarginRepay`. OKX needs per-order auto-borrow fix. SpotEngine `executeBorrowSellLong` is generic. |
| SF-02 | Dir B (buy-spot-short) full lifecycle works on all 5 exchanges | All 5 adapters implement `PlaceSpotMarginOrder` with market BUY (QuoteSize). SpotEngine `executeBuySpotShort` is generic. No exchange-specific issues expected. |
| SF-03 | Auto-borrow/auto-repay verified per exchange using exchange-native margin order flags | Binance: `sideEffectType=AUTO_BORROW_REPAY/AUTO_REPAY`. Bybit: `isLeverage=1`. Bitget: `loanType=autoLoan/autoRepay`. Gate.io: `auto_borrow=true/auto_repay=true`. OKX: NO per-order flags -- uses account-level `autoLoan` or requires manual `MarginBorrow`/`MarginRepay` calls. |
</phase_requirements>

## Standard Stack

No new libraries needed. This phase works entirely within the existing codebase.

### Core (already in place)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib | 1.26 | All backend logic | Runtime already in use |
| `redis/go-redis/v9` | 9.18.0 | State persistence (spot positions) | Already handles all position CRUD |
| `alicebob/miniredis/v2` | 2.37.0 | In-memory Redis for unit tests | Already used in `execution_test.go` |

### Supporting (already in place)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Exchange adapters | in-tree | 5x `margin.go` implementations | Every margin API call |
| `pkg/utils` | in-tree | `FormatSize`, `GenerateID`, logging | Formatting, IDs |

### No New Dependencies
This phase requires zero new packages. All work is within existing adapter code and engine logic.

## Architecture Patterns

### Existing Structure (no changes needed)
```
pkg/exchange/
  binance/margin.go     # SpotMarginExchange impl (270 lines)
  bybit/margin.go       # SpotMarginExchange impl (338 lines) -- reference
  bitget/margin.go      # SpotMarginExchange impl (387 lines)
  gateio/margin.go      # SpotMarginExchange impl (288 lines)
  okx/margin.go         # SpotMarginExchange impl (279 lines) -- needs auto-borrow fix
  types.go              # SpotMarginExchange interface, SpotMarginOrderParams

internal/spotengine/
  execution.go          # executeBorrowSellLong(), executeBuySpotShort(), ClosePosition()
  exit_manager.go       # checkExitTriggers(), initiateExit(), completeExit()
  monitor.go            # monitorLoop(), retryPendingRepay(), updateBorrowCost()
  engine.go             # NewSpotEngine() -- auto-discovers SpotMarginExchange via type assertion

cmd/livetest/main.go    # Tests 23-26: margin read + borrow/repay/transfer
```

### Pattern 1: SpotMarginExchange Interface
**What:** All margin operations go through the `SpotMarginExchange` interface. The SpotEngine discovers implementations via type assertion on startup.
**When to use:** Always -- this is the established pattern. No direct exchange API calls from engine code.
**Key methods:**
```go
type SpotMarginExchange interface {
    MarginBorrow(params MarginBorrowParams) error
    MarginRepay(params MarginRepayParams) error
    PlaceSpotMarginOrder(params SpotMarginOrderParams) (orderID string, err error)
    GetMarginInterestRate(coin string) (*MarginInterestRate, error)
    GetMarginBalance(coin string) (*MarginBalance, error)
    TransferToMargin(coin string, amount string) error
    TransferFromMargin(coin string, amount string) error
}
```

### Pattern 2: Auto-Borrow via Order Flags (4 exchanges) vs Manual Borrow (OKX)
**What:** Four exchanges (Bybit, Binance, Bitget, Gate.io) support per-order auto-borrow flags. OKX uses account-level `autoLoan` or requires explicit `MarginBorrow` -> sell -> `MarginRepay` calls.
**When to use:** The engine's `executeBorrowSellLong` currently passes `AutoBorrow: true` which works for 4 of 5 exchanges. OKX needs special handling.
**Implementation options for OKX:**
1. Check/set OKX account-level `autoLoan=true` before trading (one-time config), then `tdMode: cross` auto-borrows
2. Engine-level branch: if OKX, call `MarginBorrow` explicitly before the sell order, and `MarginRepay` explicitly after buyback
3. Add `autoLoan` flag handling in the OKX adapter's `PlaceSpotMarginOrder` -- set the account-level flag before placing orders

### Pattern 3: Livetest-First Verification
**What:** Run `go run ./cmd/livetest/ --exchange {name} --test-margin` to verify each margin API method individually before attempting full lifecycle trades.
**When to use:** First step for each exchange. Tests 23-26 cover read-only (interest rate, balance) and write (borrow+repay, transfer) operations.

### Pattern 4: Two-Phase Exit (trade legs then repay)
**What:** Exit closes futures first, then spot, then repays residual borrow. If repay fails (e.g., Bybit blackout), position stays in "exiting" with `PendingRepay=true` and monitor retries.
**When to use:** This is the existing pattern. Works for all exchanges. Only Bybit has the `:04-:05:30` blackout window sentinel error (`ErrRepayBlackout`).

### Anti-Patterns to Avoid
- **Testing with large positions:** Use minimum viable amounts. Previous debugging wasted real money on bugs.
- **Assuming auto-borrow "just works":** Each exchange implements auto-borrow differently. Verify the actual borrowed amount appears in `GetMarginBalance` after entry.
- **Skipping residual borrow checks on exit:** Even with auto-repay, rounding/interest can leave residual borrows. Always check `GetMarginBalance` after exit.
- **Ignoring transfer requirements:** Binance and Bitget have separate margin accounts requiring `TransferToMargin`/`TransferFromMargin`. Bybit, Gate.io, and OKX have unified accounts (transfers are no-ops).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Margin order placement | Custom HTTP calls per exchange | `SpotMarginExchange.PlaceSpotMarginOrder()` | Already handles side mapping, quote/base size, auto-borrow flags, TIF per exchange |
| Borrow/repay lifecycle | Manual borrow tracking | Engine's `retryPendingRepay()` + `PendingRepay` flag | Handles blackout windows, deficit purchases, monitor retry |
| Position persistence | Custom Redis operations | `database.SaveSpotPosition()`, `GetActiveSpotPositions()` | Already handles atomic read-modify-write with locking |
| Fill confirmation | Custom polling logic | `confirmSpotFill()`, `confirmFuturesFill()` | WS-first with REST fallback, timeout handling, SpotMarginOrderQuerier support |

## Common Pitfalls

### Pitfall 1: OKX Auto-Borrow Not Working
**What goes wrong:** `executeBorrowSellLong` passes `AutoBorrow: true` but OKX adapter ignores it. The sell order fails or sells from existing balance instead of borrowed coins.
**Why it happens:** OKX uses account-level `autoLoan` setting, not per-order flags. The adapter's `PlaceSpotMarginOrder` does not handle `AutoBorrow`/`AutoRepay` params.
**How to avoid:** Either (a) add account-level autoLoan check/set in OKX adapter, or (b) add engine-level branch for OKX that calls `MarginBorrow` explicitly before sell, or (c) configure OKX account to have `autoLoan=true` and verify it works with `tdMode: cross`.
**Warning signs:** Sell order succeeds but `GetMarginBalance(baseCoin).Borrowed == 0` after entry.

### Pitfall 2: Binance/Bitget Transfer Requirements
**What goes wrong:** Spot margin order fails because funds are in the futures account, not the margin account.
**Why it happens:** Binance and Bitget have separate margin and futures wallet accounts. USDT must be transferred to the margin account before it can serve as collateral for borrowing.
**How to avoid:** The engine may need to call `TransferToMargin` before entry and `TransferFromMargin` after exit for these exchanges. Check if the current flow handles this.
**Warning signs:** "Insufficient margin" or "insufficient balance" errors on margin orders despite having USDT in futures.

### Pitfall 3: Integer Precision on Borrow Amounts
**What goes wrong:** Borrow request rejected because amount has too many decimal places.
**Why it happens:** Each exchange has different precision requirements. Bybit required `math.Floor(amt)` (whole numbers). Others may have different rules.
**How to avoid:** Check each exchange's minimum borrow amount and precision requirements. Use `utils.FormatSize` with appropriate decimal places.
**Warning signs:** "precision must be an integer multiple" or "invalid amount" errors from borrow API.

### Pitfall 4: Gate.io Unified vs Classic Account Mode
**What goes wrong:** Gate.io margin API calls fail because the account is in classic (non-unified) mode.
**Why it happens:** Gate.io has two account modes. The adapter uses unified endpoints (`/unified/loans`, `account: "unified"`). Classic mode uses different endpoints.
**How to avoid:** The adapter already has `DetectUnifiedMode()` (noted in CONTEXT.md). Verify this works and errors clearly if non-unified.
**Warning signs:** 400 errors or "account mode not supported" from Gate.io API.

### Pitfall 5: Market Buy QuoteSize vs Size Confusion
**What goes wrong:** Market BUY order buys wrong amount because exchange interprets `size` as quote currency or vice versa.
**Why it happens:** Each exchange has different conventions for market BUY orders. Some take base coin quantity, others take quote (USDT) amount.
**How to avoid:** Each adapter already handles this mapping (Binance: `quoteOrderQty`, Bybit: `marketUnit=baseCoin`, Bitget: `quoteSize`, Gate.io: amount in quote for market BUY, OKX: `tgtCcy`). Verify by checking filled amount matches expected base quantity.
**Warning signs:** Order fills for much more or less than expected.

### Pitfall 6: Residual Borrows After Exit
**What goes wrong:** Position closes but a tiny borrow remains (interest accrued between last check and repay), slowly accumulating interest.
**Why it happens:** Auto-repay covers the principal but interest accrues between the moment of buyback and repay. Rounding errors compound this.
**How to avoid:** The `closeDirectionA` step 3 already queries `GetMarginBalance` for actual liability and repays `Borrowed + Interest`. Verify this works on each exchange.
**Warning signs:** After position close, `GetMarginBalance(baseCoin).Borrowed > 0`.

### Pitfall 7: Bybit Repay Blackout Window
**What goes wrong:** Repay fails during Bybit's :04-:05:30 UTC window each hour.
**Why it happens:** Bybit blocks repayment during settlement processing.
**How to avoid:** Already handled: Bybit adapter returns `ErrRepayBlackout` with `RetryAfter`, monitor retries. This is Bybit-specific and already working.
**Warning signs:** Repay error with blackout message. Should auto-resolve.

## Per-Exchange Analysis

### Bybit (Reference Implementation)
- **Account model:** Unified (UTA) -- margin, futures, spot all in one account
- **Auto-borrow mechanism:** `isLeverage=1` on spot order triggers auto-borrow
- **Transfer:** No-op (unified account)
- **Blackout:** :04-:05:30 UTC each hour (repay blocked)
- **Borrow precision:** Integer amounts (`math.Floor`)
- **Known bugs fixed:** v0.22.44-49 (integer precision, margin health, auto-borrow)
- **Confidence:** HIGH -- most tested, reference for others

### Binance
- **Account model:** Separate margin account (cross margin)
- **Auto-borrow mechanism:** `sideEffectType=AUTO_BORROW_REPAY` on margin order
- **Transfer:** Required -- `TransferToMargin` (UMFUTURE_MARGIN) and `TransferFromMargin` (MARGIN_UMFUTURE)
- **Blackout:** None known
- **Order placement:** `/sapi/v1/margin/order` endpoint
- **Market BUY:** Uses `quoteOrderQty` (USDT amount), not `quantity`
- **Confidence:** MEDIUM -- adapter exists but untested in production

### Bitget
- **Account model:** Separate cross margin account
- **Auto-borrow mechanism:** `loanType=autoLoan` / `autoRepay` / `autoLoanAndRepay`
- **Transfer:** Required -- `usdt_futures` <-> `crossed_margin` via `/api/v2/spot/wallet/transfer`
- **Blackout:** None known
- **Order placement:** `/api/v2/margin/crossed/place-order` endpoint
- **Market BUY:** Uses `quoteSize` (USDT amount)
- **Passphrase:** Requires `BITGET_PASSPHRASE` in addition to API key/secret
- **Confidence:** MEDIUM -- adapter exists but untested in production

### Gate.io
- **Account model:** Unified account (similar to Bybit)
- **Auto-borrow mechanism:** `auto_borrow=true` / `auto_repay=true` on spot order with `account=unified`
- **Transfer:** No-op (unified account)
- **Blackout:** None known
- **Order placement:** `/spot/orders` with `account: unified`
- **Market BUY:** Amount is in quote currency (USDT) when side=buy
- **Unified mode detection:** `DetectUnifiedMode()` exists in adapter
- **Response format:** JSON body (vs query string for other exchanges)
- **Confidence:** MEDIUM -- adapter uses unified endpoints, needs verification of `auto_borrow`/`auto_repay` behavior

### OKX (Most Complex)
- **Account model:** Unified account, but manual borrow/repay
- **Auto-borrow mechanism:** Account-level `autoLoan` setting (not per-order). `PlaceSpotMarginOrder` uses `tdMode=cross` but DOES NOT pass auto-borrow flags.
- **Transfer:** No-op (unified account)
- **Blackout:** None known
- **Borrow endpoint:** `/api/v5/account/spot-manual-borrow-repay` (explicit borrow/repay)
- **Contract size:** OKX uses `ctVal` (contract value multiplier) for futures -- adapter handles conversion
- **Symbol format:** Spot = `BTC-USDT`, Futures = `BTC-USDT-SWAP`
- **CRITICAL GAP:** `PlaceSpotMarginOrder` ignores `AutoBorrow`/`AutoRepay`. For Dir A, the engine must either:
  1. Pre-configure OKX account with `autoLoan=true` (via `POST /api/v5/account/set-auto-loan`)
  2. Or add engine-level `MarginBorrow` call before sell order for OKX
- **Confidence:** MEDIUM-LOW -- most likely to need engine-level branching and adapter fixes

## Code Examples

### Example 1: Livetest Margin Verification
```bash
# Phase 1: Livetest per exchange (read-only tests always run)
go run ./cmd/livetest/ --exchange bybit
go run ./cmd/livetest/ --exchange binance
go run ./cmd/livetest/ --exchange bitget
go run ./cmd/livetest/ --exchange gateio
go run ./cmd/livetest/ --exchange okx

# Phase 2: With write tests (uses real funds -- borrow 1 USDT, repay, transfer)
go run ./cmd/livetest/ --exchange bybit --test-margin
go run ./cmd/livetest/ --exchange binance --test-margin
# ... etc
```

### Example 2: Manual Open via Dashboard API
```bash
# Trigger Dir A on Binance for SEIUSDT
curl -X POST http://localhost:8080/api/spot/open \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"symbol":"SEIUSDT","exchange":"binance","direction":"borrow_sell_long"}'

# Trigger Dir B on Bitget for ETHUSDT
curl -X POST http://localhost:8080/api/spot/open \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"symbol":"ETHUSDT","exchange":"bitget","direction":"buy_spot_short"}'
```

### Example 3: OKX Auto-Borrow Fix Pattern (engine-level branch)
```go
// In executeBorrowSellLong, before placing spot sell order:
if pos.Exchange == "okx" {
    // OKX requires explicit borrow -- no per-order auto-borrow flag
    if err := smExch.MarginBorrow(exchange.MarginBorrowParams{
        Coin:   baseCoin,
        Amount: sizeStr,
    }); err != nil {
        return 0, 0, 0, 0, 0, fmt.Errorf("OKX manual borrow failed: %w", err)
    }
    // Then place sell without AutoBorrow flag
    spotOrderID, err = smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
        Symbol:    symbol,
        Side:      exchange.SideSell,
        OrderType: "market",
        Size:      sizeStr,
        // NO AutoBorrow -- already borrowed explicitly
    })
} else {
    // Other exchanges: auto-borrow on sell order
    spotOrderID, err = smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
        Symbol:     symbol,
        Side:       exchange.SideSell,
        OrderType:  "market",
        Size:       sizeStr,
        AutoBorrow: true,
    })
}
```

### Example 4: OKX Auto-Borrow Fix Pattern (adapter-level -- preferred)
```go
// In okx/margin.go PlaceSpotMarginOrder, handle AutoBorrow:
func (a *Adapter) PlaceSpotMarginOrder(params exchange.SpotMarginOrderParams) (string, error) {
    if params.AutoBorrow {
        // OKX: Borrow explicitly before the sell order
        if err := a.MarginBorrow(exchange.MarginBorrowParams{
            Coin:   extractBaseCoin(params.Symbol),
            Amount: params.Size,
        }); err != nil {
            return "", fmt.Errorf("PlaceSpotMarginOrder auto-borrow: %w", err)
        }
    }
    // ... existing order placement code ...
    // If params.AutoRepay, repay after fill confirmation
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Separate borrow then sell | Auto-borrow on sell order | v0.22.49 | Simpler, fewer race conditions, exchange manages borrow internally |
| Exit: sell then manual repay | Exit: auto-repay on buyback + residual check | v0.22.49 | Cleaner exit, `retryPendingRepay` handles failures |
| Single exchange (Bybit) | 5 margin adapters | v0.22.44+ | Adapters exist but untested beyond Bybit |

## Open Questions

1. **OKX auto-borrow: adapter-level vs engine-level fix?**
   - What we know: OKX does not support per-order auto-borrow. It has account-level `autoLoan` and manual `MarginBorrow`/`MarginRepay`.
   - What's unclear: Does setting `autoLoan=true` at account level make `tdMode: cross` orders auto-borrow? If so, a one-time account config is sufficient. If not, explicit borrow calls are needed.
   - Recommendation: Test with `--test-margin` on OKX first. Check if OKX account already has `autoLoan=true`. If not, test both approaches and pick the simpler one.

2. **Binance/Bitget TransferToMargin timing**
   - What we know: Both exchanges have separate margin accounts requiring fund transfers.
   - What's unclear: Does the current spotengine flow call `TransferToMargin` before entry? The `ManualOpen` code does not appear to call it.
   - Recommendation: Verify. If missing, add transfer calls in the entry flow for exchanges that need them. A simple check: `if exchange is binance or bitget, call TransferToMargin(USDT, amount) before placing margin orders`.

3. **Gate.io unified mode prerequisite**
   - What we know: Adapter uses unified endpoints. Gate.io has a `DetectUnifiedMode()` function.
   - What's unclear: Is it called during startup? Does the engine fail gracefully if the account is in classic mode?
   - Recommendation: Verify unified mode detection on Gate.io livetest run.

## Environment Availability

Step 2.6: No new external dependencies -- all tools (Go, Redis, exchange API keys) are already in use for the perp-perp engine. The `cmd/livetest/` harness exists and has margin tests.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `miniredis` for integration |
| Config file | None (standard `go test`) |
| Quick run command | `go test ./internal/spotengine/ -run TestClose -count=1 -v` |
| Full suite command | `go test ./internal/spotengine/ -count=1 -v` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SF-01 | Dir A lifecycle on all 5 exchanges | integration (live) | `go run ./cmd/livetest/ --exchange {name} --test-margin` | Partial (tests 23-26 exist, need PlaceSpotMarginOrder test) |
| SF-02 | Dir B lifecycle on all 5 exchanges | integration (live) | Manual via dashboard API | No automated test |
| SF-03 | Auto-borrow/auto-repay verification | integration (live) | `go run ./cmd/livetest/ --exchange {name} --test-margin` + manual inspection | Partial (borrow+repay test exists, auto-borrow test missing) |

### Sampling Rate
- **Per task commit:** `go test ./internal/spotengine/ -count=1 -v` + `go build ./...`
- **Per wave merge:** Full livetest on changed exchange + `go test ./... -count=1`
- **Phase gate:** All 5 exchanges pass livetest + successful manual Dir A and Dir B trades

### Wave 0 Gaps
- [ ] Livetest test for `PlaceSpotMarginOrder` (Dir A sell with auto-borrow, Dir B buy) -- currently no test for this critical path
- [ ] Livetest test for `GetSpotMarginOrder` (fill reconciliation) -- not tested
- [ ] No automated Dir A/Dir B lifecycle test (manual trades are the verification, per D-04)

## Project Constraints (from CLAUDE.md)

- **npm lockdown:** No `npm install` -- `npm ci` only. Phase 1 has no frontend changes, so this is not directly relevant.
- **Delegation mode:** Coordinator/team lead pattern. For multi-file fixes, delegate to teammates.
- **Build order:** Frontend before Go binary. Phase 1 is backend-only but builds still need `make build`.
- **Live system:** Changes must not break existing perp-perp trading. All changes are in spotengine and exchange adapters, which are independent from the perp-perp engine.
- **Changelog/VERSION:** Every commit must update both files.
- **Skill: `/local-api-docs`:** Agent teams MUST load this skill when working on exchange API code. Local docs at `doc/{exchange}/` are the authoritative source.
- **Skill: `/sdebug`:** Agent teams should load this skill when debugging adapter bugs. Four-phase framework: Root Cause Investigation, Pattern Analysis, Hypothesis Testing, Implementation.

## Sources

### Primary (HIGH confidence)
- Source code: `pkg/exchange/{binance,bybit,bitget,gateio,okx}/margin.go` -- all 5 adapters read in full
- Source code: `internal/spotengine/execution.go` -- entry and exit flows read in full
- Source code: `internal/spotengine/exit_manager.go` -- exit triggers and repay retry read in full
- Source code: `internal/spotengine/monitor.go` -- monitor loop and borrow cost tracking read in full
- Source code: `cmd/livetest/main.go` -- margin tests 23-26 verified
- Source code: `pkg/exchange/types.go` -- SpotMarginExchange interface and params

### Secondary (MEDIUM confidence)
- Local API docs: `doc/okx/okx-trading-account-api-docs.md` -- confirmed `autoLoan` is account-level, not per-order
- Local API docs: `doc/gate/gate-spot-api-docs.md` -- confirmed unified account supports `auto_borrow`/`auto_repay` per-order
- Local API docs: `doc/binance/binance-margin-trading-api-docs.md` -- Binance margin endpoints confirmed
- Local API docs: `doc/bitget/bitget-margin-api-docs.md` -- Bitget margin endpoints confirmed

### Tertiary (LOW confidence)
- None -- all findings verified against source code or local API docs

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new libraries, all existing code
- Architecture: HIGH -- patterns established in v0.22.44-49, code read in full
- Pitfalls: HIGH -- drawn from v0.22.44-49 debugging history + code analysis + OKX gap found in source code
- Per-exchange analysis: MEDIUM -- adapter code verified but not live-tested (that is the phase's work)

**Research date:** 2026-04-01
**Valid until:** 2026-05-01 (stable codebase, no external API changes expected)
