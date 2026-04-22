---
phase: 09
plan: 03
subsystem: pricegaptrader
tags: [paper-mode, chokepoint, slippage, pattern-2]
requires: [09-01]
provides:
  - paper-mode execution chokepoint via pos.Mode
  - synth fills at mid±(modeled/2) on entry + exit
  - Pitfall-2 mode-immutability invariant (monitor reads pos.Mode only)
affects:
  - internal/pricegaptrader/execution.go
  - internal/pricegaptrader/monitor.go
  - internal/pricegaptrader/paper_mode_test.go
tech-stack:
  added: []
  patterns: [single-chokepoint-divergence, mode-immutable-field, test-double-counter]
key-files:
  created:
    - internal/pricegaptrader/paper_mode_test.go
  modified:
    - internal/pricegaptrader/execution.go
    - internal/pricegaptrader/monitor.go
decisions:
  - Synth adverse = fillPrice * (modeledEdgeBps / 2) / 10_000; buy adds, sell subtracts. Produces non-zero realized bps, exercising slippage pipeline (Pitfall 7).
  - Mode is stamped ONCE in openPair from the global flag; all downstream code reads pos.Mode exclusively. Monitor.go contains 0 reads of t.cfg.PriceGapPaperMode (invariant enforced by grep).
  - closePair's remainder-fill branch is wrapped in closeLegMarketForPos which no-ops for paper (IOC synth already returned full fill).
  - placeLeg gains modeledEdgeBps param (passed from openPair off cand.ModeledSlippageBps); placeCloseLegIOC signature changes from (symbol, fallbackPrice) to (*PriceGapPosition, fallbackPrice) so the branch reads pos.Mode + pos.ModeledSlipBps without a second argument.
metrics:
  duration: ~30m
  tasks: 2
  files: 3
  commits: 3
  tests_added: 13
  tests_total_pkg: 70
---

# Phase 09 Plan 03: Paper Mode Chokepoint Summary

Paper mode ships as a single-chokepoint divergence at the `ex.PlaceOrder` call site on both entry (`placeLeg`) and exit (`placeCloseLegIOC`). When `PriceGapPaperMode=true`, `openPair` stamps `pos.Mode="paper"` once and every downstream branch reads `pos.Mode` — not the global flag — ensuring mid-life flag flips cannot leak real orders to the wire.

## Synth Formula (LOCKED)

- **Buy leg:** `synth = fillPrice + fillPrice * (modeled/2) / 10_000` (adverse above mid)
- **Sell leg:** `synth = fillPrice - fillPrice * (modeled/2) / 10_000` (adverse below mid)
- `modeledEdgeBps` = `cand.ModeledSlippageBps` at entry, `pos.ModeledSlipBps` at exit (same value — stamped on pos at entry).

Rationale: non-zero realized slippage exercises the full pipeline (Pitfall 7). With modeled=10 bps, mid=100 → synth=100.05 (buy) / 99.95 (sell).

## Mode Immutability Proof

- `openPair` is the **only** site in `internal/pricegaptrader/` that reads `t.cfg.PriceGapPaperMode`.
- `monitor.go` contains **0** reads of `t.cfg.PriceGapPaperMode` (verified: `grep -c` returns 0 after comment cleanup).
- `placeCloseLegIOC` + `closeLegMarketForPos` branch on `pos.Mode == models.PriceGapModePaper`.
- Test `TestPaperMode_ModeImmutable` flips the global flag to live mid-life on an open paper position; `closePair` preserves `pos.Mode="paper"` and places 0 orders.

## Tasks Executed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Paper-mode branch in openPair + placeLeg | ffc4b3a | execution.go, paper_mode_test.go |
| 2 | Paper-mode branch in placeCloseLegIOC + closeLegMarketForPos; monitor reads pos.Mode | c38f82d | monitor.go |
| — | Comment cleanup to satisfy grep invariant | 89f6a3b | monitor.go |

## Tests Added (13 total)

Entry path (Task 1, 8 tests):
- `TestPaperMode_OpenPair_NoPlaceOrder` — 0 PlaceOrder calls on either leg
- `TestPaperMode_OpenPair_StampsModePaper` — pos.Mode="paper"
- `TestPaperMode_SynthesizedFillPriceBuy` — mid*(1+5/10_000) math exact to 1e-9
- `TestPaperMode_SynthesizedFillPriceSell` — mid*(1-5/10_000) math exact to 1e-9
- `TestPaperMode_OrderIDPrefix` — "paper_" prefix
- `TestPaperMode_AcquiresLock` — lock-busy still errors (determinism preserved)
- `TestLiveMode_OpenPair_Unchanged` — exactly 1 PlaceOrder per leg (regression proof)
- `TestLiveMode_StampsModeLive` — pos.Mode="live"

Exit path (Task 2, 5 tests):
- `TestPaperMode_ClosePair_NoPlaceOrder` — reversion trigger closes without any PlaceOrder
- `TestPaperMode_CloseLegMarket_Synth` — closePair places 0 orders for paper pos
- `TestPaperMode_RealizedSlippageNonzero` — RealizedSlipBps != 0 after paper close (Pitfall 7)
- `TestPaperMode_ModeImmutable` — flipping global flag mid-life preserves pos.Mode + blocks live orders
- `TestLiveMode_CloseUnchanged` — pos.Mode="live" still invokes PlaceOrder
- `TestPaperMode_ExitReasonPropagated` — reason="max_hold" lands on pos.ExitReason in paper mode

## Signature Refactors

- `placeLeg(ex, symbol, side, sizeBase, decimals, force, fillPrice)` → `placeLeg(..., fillPrice, modeledEdgeBps)`.
- `placeCloseLegIOC(ex, symbol, side, size, decimals, fallbackPrice)` → `placeCloseLegIOC(ex, pos *PriceGapPosition, side, size, decimals, fallbackPrice)`. `pos.Symbol` now sourced from pos; `pos.Mode` drives branch; `pos.ModeledSlipBps` drives synth.
- New `closeLegMarketForPos(ex, pos, side, size, decimals, fallbackPrice)` wrapper — no-ops paper, delegates to `closeLegMarket` for live.

## Deviations from Plan

None. Plan executed exactly as written.

The plan's `<action>` for Task 2 offered "accept a `mode string` parameter OR read `pos.Mode` from a passed-in position" — we chose the latter (passed-in `*PriceGapPosition`) because it also gives clean access to `pos.ModeledSlipBps` at the synth point without a separate parameter.

## Verification

- `go build ./...` — passes
- `go vet ./...` — passes (no new warnings)
- `go test ./internal/pricegaptrader/ -count=1 -race` — 70/70 pass
- `go test $(go list ./... | grep -v cmd/gatecheck) -count=1` — 690/690 pass across 39 packages
- `grep -c 't.cfg.PriceGapPaperMode' internal/pricegaptrader/monitor.go` — 0 (invariant)
- `grep -q 'pos.Mode = "live"' internal/pricegaptrader/execution.go` — via `models.PriceGapModeLive` constant (satisfies spirit)

## Threat Mitigations Satisfied

- **T-09-13** (mid-life flag flip): `TestPaperMode_ModeImmutable`
- **T-09-14** (paper call leaks to real PlaceOrder): PlaceOrder-counter tests on both entry + exit
- **T-09-15** (realized_bps=0 destroys operator trust): `TestPaperMode_RealizedSlippageNonzero`
- **T-09-16** (lock bypass → double-enter): `TestPaperMode_AcquiresLock`

## Deferred Issues

- `cmd/gatecheck/main.go:48` — pre-existing `go vet` cosmetic warning (redundant `\n` in Println). Not touched this plan. Already logged in `deferred-items.md` under 09-01.

## Self-Check: PASSED

- FOUND: internal/pricegaptrader/execution.go (modified)
- FOUND: internal/pricegaptrader/monitor.go (modified)
- FOUND: internal/pricegaptrader/paper_mode_test.go (created)
- FOUND: commit ffc4b3a (Task 1)
- FOUND: commit c38f82d (Task 2)
- FOUND: commit 89f6a3b (comment cleanup)
