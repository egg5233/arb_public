# Phase 1: Spot-Futures Exchange Expansion - Context

**Gathered:** 2026-04-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Verify and fix Dir A (borrow-sell-long) and Dir B (buy-spot-short) full lifecycle on all 5 margin exchanges: Bybit, Binance, Bitget, Gate.io, OKX. Bybit is partially working but not fully verified. Each exchange must pass thorough verification before moving to the next.

</domain>

<decisions>
## Implementation Decisions

### Exchange Priority Order
- **D-01:** Execution order: Bybit → Binance → Bitget → Gate.io → OKX. Easiest/most-validated first, hardest (OKX manual borrow/repay, Gate.io unified mode) last.
- **D-02:** Bybit is NOT fully verified — it is the first exchange in line, not done. Previous work (v0.22.44-49) got it partially working but edge cases remain untested.

### Verification Definition
- **D-03:** Thorough verification per exchange. Each exchange must pass:
  1. Manual open + close for Dir A (borrow-sell-long)
  2. Manual open + close for Dir B (buy-spot-short)
  3. Auto-borrow on entry works correctly using exchange-native margin order flags
  4. Auto-repay on exit works correctly, no residual borrows left
  5. Edge cases: partial fills handled, repay retry on failure, emergency close, blackout window handling (where applicable)

### Testing Approach
- **D-04:** Two-phase testing per exchange:
  1. **Livetest CLI first** — Use `cmd/livetest/` harness to verify each margin API method individually (MarginBorrow, MarginRepay, PlaceSpotMarginOrder, GetSpotMarginOrder, GetMarginBalance, GetMarginInterestRate)
  2. **Full lifecycle live trades** — Small real-money positions testing both Dir A and Dir B end-to-end, then edge cases

### Per-Exchange Divergence
- **D-05:** Engine-level exchange branching (`if exchange == "okx"` style checks) is acceptable in `internal/spotengine/` when exchanges behave fundamentally differently. No requirement to force all differences into the adapter layer — pragmatism over purity.

### Claude's Discretion
- Position sizing for test trades — Claude picks appropriate small amounts per exchange
- Bug-fix sequencing within each exchange — Claude decides whether to fix Dir A fully before Dir B or interleave
- Livetest coverage per margin method — Claude decides which edge cases to add to livetest harness vs. test via live trades

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Exchange Adapter Interface
- `pkg/exchange/types.go` — `SpotMarginExchange` interface definition (line 322), `MarginBorrowParams`, `MarginRepayParams`, `SpotMarginOrderParams` structs
- `pkg/exchange/exchange.go` — Core `Exchange` interface (35 methods)

### Existing Margin Adapters (all 5)
- `pkg/exchange/bybit/margin.go` — Reference implementation (most tested)
- `pkg/exchange/binance/margin.go` — Binance spot margin adapter
- `pkg/exchange/bitget/margin.go` — Bitget spot margin adapter
- `pkg/exchange/gateio/margin.go` — Gate.io spot margin adapter (unified/classic mode)
- `pkg/exchange/okx/margin.go` — OKX spot margin adapter (manual borrow/repay)

### Spot Engine Execution
- `internal/spotengine/execution.go` — `executeBorrowSellLong()` (line 309), `executeBuySpotShort()` (line 395), `ManualOpen()` (line 25)
- `internal/spotengine/exit_manager.go` — Exit triggers, repay retry, emergency close
- `internal/spotengine/monitor.go` — Stuck exit retry, repay retry monitoring
- `internal/spotengine/risk_gate.go` — Pre-entry risk checks

### Livetest Harness
- `cmd/livetest/` — Live exchange API test suite (extend with margin method tests)

### Models
- `internal/models/spot_position.go` — SpotFuturesPosition model

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **5 margin adapters** (`margin.go` per exchange): All implement `SpotMarginExchange` interface — MarginBorrow, MarginRepay, PlaceSpotMarginOrder, GetSpotMarginOrder, GetMarginInterestRate, GetMarginBalance
- **SpotEngine execution** (`internal/spotengine/execution.go`): `executeBorrowSellLong()` and `executeBuySpotShort()` — generic, takes `SpotMarginExchange` + `Exchange` params
- **Livetest harness** (`cmd/livetest/`): 22 tests per exchange, extensible for margin method testing
- **ManualOpen()**: Dashboard-triggered entry point that routes to Dir A or Dir B

### Established Patterns
- Adapter pattern: each exchange normalizes its API into the common interface
- Error wrapping: `fmt.Errorf("MethodName: %w", err)` throughout
- Exchange-specific `APIError` structs in each `client.go`
- Rollback helpers: `rollbackBorrow()`, `rollbackSpotOrder()` in execution.go
- Confirm-fill pattern: `confirmFuturesFill()`, `confirmSpotFill()` for order verification

### Integration Points
- `cmd/main.go` — Exchange factory `newExchange()` switch at line 343
- `internal/spotengine/engine.go` — SpotEngine lifecycle, exchange map initialization
- `internal/database/spot_state.go` — SpotFuturesPosition CRUD in Redis
- `internal/api/spot_handlers.go` — Dashboard API routes for spot-futures

### Known Per-Exchange Complexity
- **Gate.io**: Unified vs classic account mode auto-detection (`DetectUnifiedMode()`)
- **OKX**: Manual borrow/repay via `/api/v5/account/spot-manual-borrow-repay`, `ctVal` contract size conversion
- **Bitget**: Passphrase auth, `fetchMaxBorrowable()` helper
- **Binance**: Margin borrow/repay via `/sapi/v1/margin/borrow-repay`

</code_context>

<specifics>
## Specific Ideas

- Budget 3-5 adapter bugs per exchange based on v0.22.44-49 precedent (STATE.md blocker note)
- Bybit was the prototype — lessons from its debugging (integer precision, margin health check, auto-borrow bugs) should inform what to watch for on other exchanges

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-spot-futures-exchange-expansion*
*Context gathered: 2026-04-01*
