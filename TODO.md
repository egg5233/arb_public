# TODO

## Open Issues (2026-03-17)

### POLYXUSDT instant spread reversal
- **Symptom**: Opened at 23:54, spread reversed from +165.6 to -203.0 bps/h within 19 seconds, closed at 23:59 with -$1.79 loss and zero funding collected.
- **Root cause**: 8h funding interval — funding snapshot is far away, but the discovered spread was stale or market moved during entry. The position had no time to collect any funding before the spread flipped.
- **Action**: Consider requiring a minimum hold-to-funding-ratio (e.g. don't enter 8h-interval positions when >4h from snapshot) or verify spread with a second sample before executing.

### KITEUSDT entered with positive spread (unfavorable)
- **Symptom**: `spread=158.0bps` (ask > bid = we pay to enter), but the dynamic threshold allowed it (`maxSpread = max(40, opp.Spread * 24)`).
- **Root cause**: Dynamic threshold is very permissive for high-spread opportunities. For KITEUSDT the funding spread justified the entry cost on a recovery-time basis, but the cross-exchange spread was unfavorable at entry moment.
- **Action**: Investigate whether the position profited despite the unfavorable entry. May need a sanity cap on the dynamic threshold (e.g. `min(dynamicMax, MaxPriceGapBPS)`) to prevent entering at extreme spreads.

### Multiple bot instances running concurrently
- **Symptom**: Triple-duplicated log lines at 23:00. `lock busy` messages for PIPPINUSDT, KITEUSDT, POLYXUSDT. Scheduler fired 3 callbacks simultaneously.
- **Root cause**: Manual `nohup ./arb` restart at 21:11 didn't kill the systemd-managed instance cleanly. Second manual restart at 22:01 added a third instance. All three shared the same Redis locks, causing contention and duplicate order attempts.
- **Action**: Always use `systemctl restart arb` for restarts. Consider adding a PID file or Redis-based singleton lock at startup to prevent multiple instances.

### Logger MultiWriter causes double lines with nohup
- **Symptom**: Every log line appears twice when bot is started with `nohup ./arb >> arb.log`.
- **Root cause**: Logger uses `io.MultiWriter(os.Stdout, logFile)` writing to both stdout and arb.log. `nohup >> arb.log` redirects stdout into the same file.
- **Action**: Either skip stdout when log file is open, or always start via systemd (which captures stdout separately in journal).

---

# Archived — Exchange Adapter Bugs (livetest 2026-03-15)

## Binance
- [ ] **PlaceOrder min notional**: Binance requires min notional 100 USDT for non-reduce-only orders. With capital_per_leg=40 USDT, cheap tokens may fail. Either increase capital_per_leg or add min notional check in risk manager.
- [ ] **Orphaned LYNUSDT position**: Long size=540, leverage=2x still open on Binance from failed trade. Needs manual close.

## Bybit
- [x] **SetMarginMode on unified account**: Fixed — code=100028 now treated as no-op (same as "already set").

## Gate.io
- [x] **WebSocket BBO no data**: Fixed — Go json case-insensitive matching caused `"b"` (price string) and `"B"` (volume int) to collide. Switched to raw map parsing.
- [x] **GetPosition/GetAllPositions JSON unmarshal**: Fixed — `cross_leverage_limit` changed from `int` to `json.Number`.

## Earlier fixes (this session)
- [x] **Gate.io SetMarginMode**: Was sending leverage=0 overriding SetLeverage. Now no-op for cross mode.
- [x] **Bitget PlaceOrder**: Missing `marginMode` param. Added `"marginMode": "crossed"`.
- [x] **All exchanges tick size**: Engine was formatting prices with hardcoded 8 decimals. Now rounds to exchange tick size via contract info.

## Final livetest scores (post-fix)
| Exchange | Score | Notes |
|----------|-------|-------|
| Bitget   | 17/17 | Clean |
| OKX      | 17/17 | Clean |
| Bybit    | 17/17 | Fixed SetMarginMode |
| Gate.io  | 17/17 | Fixed WS + unmarshal |
| Binance  | 15/17 | PlaceOrder min notional (test issue + config issue) |
