---
status: partial
phase: 11-auto-discovery-scanner-chokepoint-telemetry
source: [11-06-PLAN.md, 11-06-SUMMARY.md]
started: "2026-04-28T15:30:00.000Z"
updated: "2026-04-28T15:30:00.000Z"
---

## Current Test

[awaiting human testing]

## Tests

### 1. Visual UI verification — Discovery section across 3 scanner states + locale switching

**Origin:** Plan 11-06 Task 3 (`<how-to-verify>` section)

**expected:**
- Run `./arb` against live runtime (no Phase 11 config edits required for this test).
- Visit `https://localhost:8443` → PriceGap page → Discovery section.
- State 1 (default OFF — `PriceGapDiscoveryEnabled=false`): banner shows "Discovery: OFF" in en, "自動探索：關閉" in zh-TW. CycleStatsCard, ScoreHistoryCard, WhyRejectedCard render in placeholder/empty form.
- State 2 (errored — manipulate Redis DB 2: `redis-cli -n 2 SET pg:scan:error "test error"`): banner switches to error state with message visible in both locales.
- State 3 (ON with cycles — set `PriceGapDiscoveryEnabled=true` in `config.json`, restart, wait ≥75s): banner shows "ON", CycleStatsCard populates, ScoreHistoryCard line chart renders, WhyRejectedCard shows tallies.
- Locale switch (top-right toggle): every Discovery component swaps between en and zh-TW with no untranslated keys leaking through.

result: [pending]

### 2. Live BBO join — all 6 exchanges × BTCUSDT

**Origin:** Plan 11-06 Task 4 (Phase 11 Success Criterion #5, `<how-to-verify>` section)

**expected:**
- Edit `config.json` (operator only — Claude must not touch this file):
  - `PriceGapDiscoveryEnabled = true`
  - `PriceGapDiscoveryUniverse = ["BTCUSDT"]`
  - `PriceGapDiscoveryIntervalSec = 60`
- Restart `arb` (systemctl restart arb-bot or equivalent).
- Wait ≥75 seconds.
- Run `redis-cli -n 2 LRANGE pg:scan:cycles -1 -1` and inspect the most recent CycleRecord.
- Verify it contains records covering all 30 cross-pair permutations across {binance, bybit, gate, bitget, okx, bingx} for BTCUSDT (i.e., scanner subscribed to live BBO from all 6 exchanges).
- After verification, optionally set `PriceGapDiscoveryEnabled = false` again to keep system OFF until ramp-up phase.

result: [pending]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
