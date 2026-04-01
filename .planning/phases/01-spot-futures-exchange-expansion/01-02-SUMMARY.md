---
phase: 01-spot-futures-exchange-expansion
plan: 02
status: complete
started: 2026-04-01T17:20:00+08:00
completed: 2026-04-01T23:10:00+08:00
---

## Summary

Verified and fixed Binance and Bitget margin adapters for full Dir A and Dir B spot-futures lifecycle. Found and fixed 12 bugs during live trade verification, including fundamental issues with spot vs margin endpoint routing, fee deduction tracking, and Bitget's flash-repay integration.

## Tasks Completed

| Task | Name | Status |
|------|------|--------|
| 1 | Run livetest on Binance and Bitget, fix adapter and engine bugs | Complete |
| 2 | Verify Binance and Bitget Dir A and Dir B with live trades | Complete (checkpoint approved) |

## Key Findings

### Livetest (Task 1)
- All 28 tests PASS on both Binance and Bitget with `--test-margin`
- Bitget `GetMarginBalance` returned error on empty margin account (fixed: return zero-balance struct)
- Engine missing `TransferToMargin`/`TransferFromMargin` for separate-account exchanges (added `needsMarginTransfer()`)
- Livetest Test 11 minimum notional: Binance requires 100 USDT (fixed dynamic size calculation)

### Live Verification Bugs Found (Task 2) — 12 total

**Spot vs Margin Routing (critical)**
1. **Dir B assets in wrong wallet** — `PlaceSpotMarginOrder` always used margin endpoint; Dir B (plain buy) needs regular spot endpoint. Fixed: route to `/api/v3/order` (Binance) / `/api/v2/spot/trade/place-order` (Bitget) when no auto-borrow/repay
2. **Dir B transfer routing** — `TransferToMargin` wrong for Dir B; need `TransferToSpot` (futures→spot) for entry, `TransferToFutures` (spot→futures) for exit
3. **Binance TransferToSpot was no-op** — existing method returned nil; fixed to do real `UMFUTURE_MAIN` transfer

**Fee Deduction (critical)**
4. **Spot BUY fee deducted from received coin** — Binance/Bitget deduct trading fee from BTC on buy; engine recorded gross fill but tried to sell full amount on exit. Fixed: query trade fills to get actual fee, store net received as SpotSize, floor to spot step (0.00001)
5. **All 5 adapters: fee fill queries** — Added trade fill queries to get FeeDeducted: Binance `/api/v3/myTrades`, Bybit `/v5/execution/list`, Bitget `/api/v2/spot/trade/fills`, OKX `/api/v5/trade/fills`, Gate.io `/spot/my_trades`

**Bitget-specific**
6. **Flash-repay for Dir A close** — Bitget's spot buyback hit ~2% price ceiling on limit orders. Root cause: market BUY only accepts quoteSize (USDT), giving approximate fill with dust. Fix: use Bitget's flash-repay (`/api/v2/margin/crossed/account/flash-repay`) to repay borrow without a buy order
7. **Bitget quoteSize param name** — margin BUY needs `quoteSize`, spot BUY needs `size` (was using `size` for both)
8. **Bitget spot order query format** — spot endpoints return `data[].baseVolume`, margin returns `data.orderList[].size`; added `getSpotTradeOrder` parser

**Engine Sequencing**
9. **TransferToMargin before MaxBorrowable** — transfer was after size computation; MaxBorrowable returned near-zero because USDT hadn't been moved to margin yet
10. **Dir A buyback stale price** — quoteEst used entry price (could be hours old); now uses futures exit price (just filled = current market)
11. **Dir A buyback price ceiling** — derived limit price exceeded Bitget's 2% ceiling; adapter now queries live spot ticker for current ask

**Consolidator**
12. **Orphan detection false positive** — perp-perp consolidator flagged spot-futures futures legs as orphans and would auto-close them; added exclusion set from active spot positions

## Commits

- `34bbef0` feat(01-02): Binance and Bitget pass all 28 livetest tests with margin
- `c1548f2` fix(01-02): Binance/Bitget Dir A+B live verification — spot/margin routing, flash-repay, fee deduction

## Key Files

### Modified
- `pkg/exchange/binance/adapter.go` — TransferToSpot real implementation
- `pkg/exchange/binance/margin.go` — spot/margin endpoint routing, dual-endpoint order query, trade fill fee query
- `pkg/exchange/bitget/margin.go` — spot/margin routing, flash-repay, live ticker price, spot order parser, trade fill fee query
- `pkg/exchange/bybit/margin.go` — trade fill fee query (populateBybitFeeDeducted)
- `pkg/exchange/gateio/margin.go` — trade fill fee query
- `pkg/exchange/okx/margin.go` — trade fill fee query
- `pkg/exchange/types.go` — FeeDeducted field, FlashRepayer interface
- `internal/spotengine/execution.go` — Dir B fee deduction, transfer sequencing, Dir A buyback price
- `internal/spotengine/exit_manager.go` — Dir B TransferToFutures on exit
- `internal/engine/consolidate.go` — spot-futures orphan exclusion

## Self-Check: PASSED

- [x] Binance Dir A: open, close, no residual borrows
- [x] Binance Dir B: open, close, USDT returned to futures
- [x] Bitget Dir A: open, close via flash-repay, no residual borrows
- [x] Bitget Dir B: open, close, USDT returned to futures
- [x] All 28 livetest tests pass on both exchanges
- [x] Unit tests pass (`go test ./internal/spotengine/ ./pkg/exchange/bitget/`)
- [x] Full build passes (`go build ./...`)
- [x] Consolidator no longer flags spot-futures positions as orphans
