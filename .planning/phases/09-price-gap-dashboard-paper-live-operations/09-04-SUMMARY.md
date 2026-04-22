---
phase: 09-price-gap-dashboard-paper-live-operations
plan: 04
subsystem: notify
tags: [telegram, pricegap, notifications, paper-mode, regression]
requires:
  - internal/notify/telegram.go (checkCooldown, formatExitReason, formatDuration, send)
  - internal/models/pricegap_position.go (PriceGapPosition fields incl. Mode)
provides:
  - NotifyPriceGapEntry(pos *models.PriceGapPosition)
  - NotifyPriceGapExit(pos, reason, pnl, duration)
  - NotifyPriceGapRiskBlock(symbol, gate, detail)
  - sanitizeForTelegram(s, max int) string
affects:
  - internal/notify/telegram.go (added 3 methods + helper, replaced hard-coded URL with telegramAPIBase var)
tech-stack:
  added: []
  patterns: [nil-receiver-safe methods, checkCooldown dedup, httptest-stubbed transport]
key-files:
  created:
    - internal/notify/telegram_pricegap_test.go
    - internal/notify/telegram_regression_test.go
  modified:
    - internal/notify/telegram.go
    - VERSION
    - CHANGELOG.md
decisions:
  - Used package-level `telegramAPIBase` var (replacing hard-coded URL) as the httptest seam — minimal blast radius vs. interface-typed transport.
  - Rejected unknown gate names silently rather than alerting on them (T-09-17 chose to stay quiet over making noise on malformed input).
  - Paper prefix matches the plan spec exactly (`📝 PAPER `) including the emoji + trailing space.
metrics:
  completed: 2026-04-22
  duration: ~25min
  tasks: 2
  tests_added: 18 (12 pricegap + 6 regression test functions)
  notify_suite: 30/30 green under -race
---

# Phase 09 Plan 04: Price-Gap Telegram Notifications Summary

Three new `*TelegramNotifier` methods (`NotifyPriceGapEntry`, `NotifyPriceGapExit`, `NotifyPriceGapRiskBlock`) ship with paper-mode prefix parity, per-(gate, symbol) cooldown, allowlisted gate names, and control-character detail sanitization — plus a regression guardrail pinning byte-for-byte output of every pre-existing notifier.

## Commits

| Task | Description | Hash |
|------|-------------|------|
| 1 | Add PriceGap notify methods + sanitizer + tests | `03e4843` |
| 2 | Byte-for-byte regression fixtures for existing Notify* methods | `b969424` |

## New Method Signatures

```go
func (t *TelegramNotifier) NotifyPriceGapEntry(pos *models.PriceGapPosition)
func (t *TelegramNotifier) NotifyPriceGapExit(pos *models.PriceGapPosition, reason string, pnl float64, duration time.Duration)
func (t *TelegramNotifier) NotifyPriceGapRiskBlock(symbol, gate, detail string)
```

All three are nil-receiver-safe; entry/exit are also nil-pos-safe.

## Risk-Block Gate Allowlist

Unknown gate names are silently rejected before cooldown lookup (T-09-17, prevents cooldown-bypass via crafted keys):

```
concentration, max_concurrent, kline_stale, delist, budget, exec_quality
```

## Cooldown Key Format

`pg_risk:<gate>:<symbol>` — routes through the existing `checkCooldown` helper (5-minute window, shared with other per-event-type dedup). Distinct (gate, symbol) pairs do NOT share cooldown, so a spinning denial gate on one symbol never suppresses alerts on another.

## Paper-Mode Parity (D-22)

When `pos.Mode == "paper"`, messages are prefixed with `"📝 PAPER "` and tagged `[PAPER]` in the header line. Live messages remain unprefixed with `[LIVE]` tag.

## Detail Sanitizer

`sanitizeForTelegram(s, max)` strips all C0 control characters except `\t` and `\n`, then caps the result to `max` bytes. Applied with `max=256` to the RiskBlock `detail` argument (T-09-18).

## Regression Coverage

`telegram_regression_test.go` pins the byte-for-byte output of:

| Method | Variants |
|--------|----------|
| NotifyAutoEntry | Dir A + Dir B labels |
| NotifyAutoExit | positive + negative PnL sign |
| NotifyEmergencyClose | standard format |
| NotifySLTriggered | standard format |
| NotifyEmergencyClosePerp | standard format |
| NotifyConsecutiveAPIErrors | with err + nil-err |
| NotifyLossLimitBreached | standard format |
| NotifySpotHedgeBroken | standard format |
| NotifySpotCloseBlocked | standard format |
| TestRegression_AllNilSafe | all 9 methods + Send() |

Any shared-helper refactor that alters a perp-perp or spot-futures alert body will now fail `go test ./internal/notify/...`.

## Test Seam

Replaced the hard-coded `"https://api.telegram.org"` in `send()` with a package-level `telegramAPIBase` var. Tests swap it for `httptest.NewServer.URL` and restore via `t.Cleanup`. Production behavior unchanged.

## Verification

- `go test ./internal/notify/... -count=1 -race` → 30/30 green
- `go test ./internal/pricegaptrader/... -count=1 -race` → green (no cross-package regressions)
- `go build ./...` → success
- `go vet ./...` → clean

## Threat Mitigations Applied

| Threat | Mitigation |
|--------|-----------|
| T-09-17 Spoofing (crafted gate) | Map-based allowlist; unknown → return without send |
| T-09-18 Tampering (detail injection) | `sanitizeForTelegram` strips C0 controls + 256B cap |
| T-09-19 DoS (gate spam) | `pg_risk:<gate>:<symbol>` cooldown via existing helper |
| T-09-20 Elevation (regression) | Task 2 golden-string fixtures cover all 9 existing methods |
| T-09-21 Repudiation (untagged paper) | `📝 PAPER ` prefix + `[PAPER]` tag, enforced by test |

## Deviations from Plan

- **[Rule 1 - Bug]** Plan interface block had unused `_ = pnl` pattern; implementation uses `%.1f bps` with a zero-notional guard (`pos.NotionalUSDT > 0`) to avoid NaN/Inf divide. Covered by `TestNotifyPriceGapExit_ZeroNotionalSafe`.
- **[Rule 2 - Critical]** Added `nil pos` guard to `NotifyPriceGapEntry` / `NotifyPriceGapExit` (not just nil receiver) — the plan's interface stub dereferenced `pos` without a nil check. Matches the existing pattern on `NotifySpotHedgeBroken` / `NotifySpotCloseBlocked`.
- Plan suggested `telegram_regression_test.go` cover "5+" existing methods; implementation covers **9** to include `NotifySpotHedgeBroken`, `NotifySpotCloseBlocked`, `NotifyLossLimitBreached` — no existing method left unpinned.

## Self-Check: PASSED

- internal/notify/telegram.go — modified, 3 methods present (grep-verified)
- internal/notify/telegram_pricegap_test.go — created (file exists)
- internal/notify/telegram_regression_test.go — created (file exists)
- Commits: `03e4843`, `b969424` both present in `git log --oneline`
- All acceptance greps satisfied: `NotifyPriceGap{Entry,Exit,RiskBlock}`, `📝 PAPER `, `checkCooldown("pg_risk:`, `"concentration"`
- Full notify suite green (30 tests), full build green, vet clean
