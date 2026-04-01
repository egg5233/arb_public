---
phase: 01-spot-futures-exchange-expansion
plan: 01
status: complete
started: 2026-04-01T15:10:00+08:00
completed: 2026-04-01T16:50:00+08:00
---

## Summary

Enhanced livetest harness with margin trade tests (27-28) and verified Bybit as the reference implementation for both Dir A (borrow-sell-long) and Dir B (buy-spot-short) lifecycle. Found and fixed 8 bugs during live verification.

## Tasks Completed

| Task | Name | Status |
|------|------|--------|
| 1 | Add PlaceSpotMarginOrder and GetSpotMarginOrder tests to livetest | Complete |
| 2 | Run Bybit livetest and fix adapter bugs | Complete |
| 3 | Verify Bybit Dir A and Dir B with live trades | Complete (checkpoint approved) |

## Key Findings

### Livetest (Task 1-2)
- All 28 tests PASS on Bybit with `--test-margin`
- Bybit UTA `isLeverage=1` triggers auto-borrow on sell but does NOT auto-repay on buy
- Minimum borrow threshold: 100 USDT (was 1), minimum spot order notional: $12 (was $5)

### Live Verification Bugs Found (Task 3)
1. **Futures qty step size** — spot fill 0.001456 rejected by futures (step=0.001). Fix: pre-align both legs to futures step size before spot order
2. **Rollback doesn't repay** — old rollback relied on auto-repay which Bybit ignores. Fix: explicit `MarginRepay` in `rollbackBorrowSell()`
3. **Dir B inexact fill** — QuoteSize market buy gave 0.000999 (under step). Fix: use Size (base qty) with `marketUnit=baseCoin`
4. **Bybit repay quota** — `/v5/account/no-convert-repay` with explicit amount fails when UTA locks spot balance. Fix: omit amount parameter, let Bybit repay available balance
5. **Monitor false deficit** — Used `Available` instead of `TotalBalance`, buying unnecessary BTC. Fix: use `TotalBalance`
6. **Missing error log** — ManualOpen rejection silently returned error. Fix: add `e.log.Warn()` before return
7. **Dashboard swallows errors** — Spot Open button fire-and-forget. Fix: loading state + error banner
8. **Auto-repay mode OFF** — Bybit wasn't configured for auto-repay. Fix: enabled via API as safety net

## Commits

- `dd7bbc7` feat(01-01): add PlaceSpotMarginOrder and GetSpotMarginOrder tests to livetest harness
- `3575bdf` fix(01-01): Bybit passes all 28 livetest tests, fix minimum borrow and notional thresholds
- `b45c199` fix(01-01): Bybit Dir A/B lifecycle bugs found during live verification

## Key Files

### Created
- `cmd/livetest/main.go` — tests 27-28 for margin trade verification

### Modified
- `internal/spotengine/execution.go` — step size alignment, rollback repay, Dir B base qty, settlement delay
- `internal/spotengine/monitor.go` — TotalBalance deficit check, auto-settle detection
- `pkg/exchange/bybit/margin.go` — omit amount in no-convert-repay
- `internal/api/server.go` — test-inject handler registration
- `internal/api/spot_handlers.go` — test-inject endpoint
- `internal/spotengine/engine.go` — InjectTestOpportunity method
- `cmd/main.go` — wire test-inject handler
- `web/src/pages/Opportunities.tsx` — spot open error display
- `CHANGELOG.md` — v0.22.51 entry
- `VERSION` — 0.22.51

## Self-Check: PASSED

- [x] All 28 livetest tests pass on Bybit
- [x] Dir A (borrow-sell-long): open, close, repay — all clean
- [x] Dir B (buy-spot-short): open, close — all clean
- [x] No residual borrows after exit
- [x] Unit tests pass (`go test ./internal/spotengine/ ./pkg/exchange/bybit/`)
- [x] Full build passes (`go build ./...`)
