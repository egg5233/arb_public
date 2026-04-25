---
phase: 09-price-gap-dashboard-paper-live-operations
plan: 11
type: execute
status: complete
completed: 2026-04-25
duration: 12 hours (across 2 sessions, multiple in-session gap closures)
tasks: 3/3
---

# Plan 09-11 — UAT Rewalk Summary

## Objective

Carry the three human-only verification items from `09-VERIFICATION.md` through
to a signed-off state after Gap #1 (09-09) and Gap #2 (09-10) landed. Walk
the 14 UAT rows blocked in the 2026-04-22 attempt under aggressive thresholds.

## Outcome

**PASS — Phase 9 UAT signed off 2026-04-25.**

All three checkpoint tasks completed:

- Task 1: 14-row UAT walkthrough on live dev host — all rows verified against
  the live paper-mode fire on SOONUSDT (08:12:16–08:12:47).
- Task 2: PG-OPS-01 Disable/Re-enable modal Redis parity — verified.
- Task 3: A11y `role=dialog` + Esc close on both modals — verified.

## Bonus gap closures discovered + fixed during UAT

The UAT cycle exposed three additional bugs that were fixed in-session before
sign-off — collectively shipping as v0.34.6 → v0.34.8. None of these would
have been caught by the headless test suite; they only surface under live
end-to-end exercise of the pricegap pipeline.

| v | Commit | Bug |
|---|--------|-----|
| 0.34.6 | `5e5bb9e` | BingX WS bookTicker case-insensitive JSON overwrote bid/ask price with quantity |
| 0.34.7 | `7e8548a` | Gate.io adapter rejected market orders (`closeLegMarket`) without `tif=ioc` |
| 0.34.8 | `5a0d214` | Paper-mode boundary leak in `closeLegMarket` + missing per-candidate re-entry gate (Gate 0) |

## Live-fire validation evidence

Paper-mode fire on SOONUSDT at 2026-04-25 08:12:16:

- Position ID: `pg_SOONUSDT_gateio_bingx_1777075936234666224`
- Mode: paper (stamped at entry, immutable through close)
- Entry: spread 14.1 bps, notional $50, long_size=276 short_size=276
- Synth fills: long mid 0.18115 → fill 0.18142 (+15.0 bps), short mid 0.18090 → fill 0.18062 (−15.0 bps)
- Close: 31s later, reason=reverted, pnl=−$0.30
- Duplicate fire attempted at 08:12:45 — Gate 0 blocked: `pricegap: SOONUSDT gate-blocked: duplicate_candidate`
- Real exchange `PlaceOrder` calls in the 5-minute window: **zero**
- Final state: 0 ghost positions in `pg:positions:active`

## Known issues (deferred, not blocking sign-off)

1. **`realized_slippage_bps ≈ 0` in paper mode** — calculation appears to be
   `actual_slip - modeled_slip`, which is bit-exact zero when synth uses
   modeled exactly. Pitfall 7's intent (non-zero realized to exercise
   analytics) is partially met because the absolute slip IS happening (15bps
   each leg), but the persisted field reads near-zero. Fix candidate: store
   absolute slip, or add noise to the synth.
2. **`paper_mode=false` flipped on dashboard load** at 2026-04-25 07:47 with
   no user click. No React useEffect auto-POST was found. Capture browser
   DevTools Network on next dashboard open to identify source.
3. **`cmd/bingxprobe/main.go`** untracked diagnostic tool — useful for future
   BingX WS debugging. Decide: commit as utility or delete.

## Files

- `.planning/phases/09-price-gap-dashboard-paper-live-operations/09-UAT-REWALK.md` — evidence doc, signed off

## Next

`/gsd-verify-work` to mark Phase 9 verified and complete.
