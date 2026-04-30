---
status: partial
phase: 12-auto-promotion
source: [12-VERIFICATION.md]
started: 2026-04-30T00:00:00Z
updated: 2026-04-30T00:00:00Z
---

## Current Test

[awaiting human testing — staging environment with real Redis, Telegram bot, and operational `arb` daemon]

## Tests

### 1. WS push render in dashboard timeline
expected: `LPUSH pg:promote:events '{...}'` on staging Redis updates the Discovery → Promote/Demote Timeline within ≤1 cycle, no page reload required.
why_human: Live WS subscription rendering can't be asserted by unit test.
result: [pending]

### 2. Telegram per-event message format and 5-min cooldown
expected: `[PG promote] {symbol} {long}↔{short} (bidirectional) score=N streak=N` — single message per event; subsequent events for the same candidate within 5min are suppressed; distinct candidates pass through unaffected.
why_human: Requires real Telegram bot token + channel.
result: [pending]

### 3. Cold restart 6-cycle baseline (D-03 in-memory streak)
expected: After daemon kill at streak counter=5 and restart, controller starts at 0 and needs 6 fresh consecutive cycles before firing a promote. (D-03: streak is in-memory only.)
why_human: Requires daemon kill + observation across cycles.
result: [pending]

### 4. `.bak` rotation on auto-promote write
expected: `config.json.bak.{ts}` file appears after the first auto-promote with prior file content preserved.
why_human: Filesystem side-effect requires a real promote event.
result: [pending]

## Summary

total: 4
passed: 0
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps
