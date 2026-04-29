---
phase: 12
slug: auto-promotion
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-29
---

# Phase 12 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none — `go.mod` provides toolchain |
| **Quick run command** | `go test ./internal/pricegaptrader/... -run "TestPromotion|TestAutoPromote|TestStreak" -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~30s quick / ~3-5min full |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/pricegaptrader/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green; frontend `npm run build` must succeed
- **Max feedback latency:** 30 seconds (quick) / 300 seconds (full)

---

## Per-Task Verification Map

> Filled in by gsd-planner after task IDs are finalized. Each task referenced below maps to a Validation Architecture entry from 12-RESEARCH.md.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 12-01-* | 01 | 1 | PG-DISC-02 | — | streak counter increments only on accepted cycles ≥ score | unit | `go test ./internal/pricegaptrader/ -run TestPromotionStreak -count=1` | ❌ W0 | ⬜ pending |
| 12-01-* | 01 | 1 | PG-DISC-02 | — | streak resets on miss / non-accepted cycle | unit | `go test ./internal/pricegaptrader/ -run TestPromotionStreakReset -count=1` | ❌ W0 | ⬜ pending |
| 12-01-* | 01 | 1 | PG-DISC-02 | — | promote fires Registry.Add at counter ≥ 6 with source="scanner-promote" | unit | `go test ./internal/pricegaptrader/ -run TestPromotionFires -count=1` | ❌ W0 | ⬜ pending |
| 12-01-* | 01 | 1 | PG-DISC-02 | — | cap-full skip silent, increments cap_full_skips, holds streak at 6 | unit | `go test ./internal/pricegaptrader/ -run TestPromotionCapFull -count=1` | ❌ W0 | ⬜ pending |
| 12-01-* | 01 | 1 | PG-DISC-02 | — | duplicate dedupe skip silent (Registry returns ErrDuplicate) | unit | `go test ./internal/pricegaptrader/ -run TestPromotionDedupe -count=1` | ❌ W0 | ⬜ pending |
| 12-01-* | 01 | 1 | PG-DISC-02 | — | symmetric demote streak fires Registry.Delete at counter ≥ 6 | unit | `go test ./internal/pricegaptrader/ -run TestDemoteStreak -count=1` | ❌ W0 | ⬜ pending |
| 12-01-* | 01 | 1 | PG-DISC-02 | — | active-position guard blocks demote, holds counter at 6 | unit | `go test ./internal/pricegaptrader/ -run TestDemoteBlockedByGuard -count=1` | ❌ W0 | ⬜ pending |
| 12-01-* | 01 | 1 | PG-DISC-02 | — | promote/demote events written to pg:promote:events LIST + LTrim 1000 | unit | `go test ./internal/pricegaptrader/ -run TestPromoteEventLog -count=1` | ❌ W0 | ⬜ pending |
| 12-01-* | 01 | 1 | PG-DISC-02 | — | Telegram cooldown key uses unique per-event format | unit | `go test ./internal/pricegaptrader/ -run TestPromoteTelegramKey -count=1` | ❌ W0 | ⬜ pending |
| 12-01-* | 01 | 1 | PG-DISC-02 | — | WS broadcast fires pg_promote_event with full payload | unit | `go test ./internal/pricegaptrader/ -run TestPromoteWSBroadcast -count=1` | ❌ W0 | ⬜ pending |
| 12-02-* | 02 | 2 | PG-DISC-02 | — | Scanner.RunCycle calls controller.Apply after telemetry.WriteCycle | unit | `go test ./internal/pricegaptrader/ -run TestScannerCallsController -count=1` | ❌ W0 | ⬜ pending |
| 12-02-* | 02 | 2 | PG-DISC-02 | — | scanner_static_test.go updated to forbid raw cfg.PriceGapCandidates write | static | `go test ./internal/pricegaptrader/ -run TestScannerNoMutation -count=1` | ✅ exists | ⬜ pending |
| 12-02-* | 02 | 2 | PG-DISC-02 | — | bootstrap wires PromotionController in cmd/main.go | integration | `go build -o /tmp/arb-test ./cmd && go vet ./...` | ✅ exists | ⬜ pending |
| 12-03-* | 03 | 2 | PG-DISC-02 | — | GET /api/pg/discovery/promote-events returns LIST contents | integration | `go test ./internal/api/ -run TestPromoteEventsEndpoint -count=1` | ❌ W0 | ⬜ pending |
| 12-04-* | 04 | 3 | PG-DISC-02 | — | Discovery dashboard timeline subscribes to pg_promote_event | manual+e2e | `npm run build && npm run typecheck` (web/) | ✅ exists | ⬜ pending |
| 12-04-* | 04 | 3 | PG-DISC-02 | — | EN + zh-TW i18n keys added in lockstep, parity test passes | static | `npm run test -- parity.test.ts` (web/) | ✅ exists | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/pricegaptrader/promotion_test.go` — stubs for streak, cap-full, dedupe, demote, guard-block, event log, telegram-key, ws-broadcast
- [ ] `internal/pricegaptrader/testhelpers_promotion.go` — fakes for `*Registry`, `ActivePositionChecker`, `PromoteNotifier`, `WSBroadcaster`, `Telemetry`
- [ ] `internal/api/pricegap_promote_events_test.go` — REST seed endpoint test
- [ ] No new framework needed — existing `go test` infrastructure covers all needs

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Discovery dashboard timeline renders newest-first with promote/demote events on live WS | PG-DISC-02 | Visual rendering + WS subscription cannot be asserted by unit test | 1. Build frontend (`npm run build` in web/), build Go binary 2. Start arb on a staging Redis 3. Manually `LPUSH pg:promote:events '{...}'` and verify timeline updates within 1 cycle without page reload 4. Toggle `PriceGapDiscoveryEnabled` and verify scanner+controller stop together |
| Telegram message wording matches D-14 format | PG-DISC-02 | Telegram delivery requires real bot token and channel | 1. Configure dev Telegram bot 2. Run promotion against a fake scanner that yields a candidate above threshold for 6 cycles 3. Confirm `[PG promote] {symbol} {long}↔{short} ({direction}) score={n} streak={n}` arrives once 4. Force a flap within 5 minutes; confirm second message suppressed by cooldown 5. Force a different candidate within same window; confirm distinct message arrives |
| Cold restart 6-cycle baseline (D-03) | PG-DISC-02 | Requires daemon kill + restart timing observation | 1. Run scanner with a candidate at counter=5 2. Kill daemon mid-cycle 3. Restart 4. Confirm counter resets to 0 and 6 fresh cycles required before promote |
| `.bak` rotation on auto-promote write | PG-DISC-02 | Filesystem side-effect of `Registry.Add` → `SaveJSON` flow; covered by Phase 11 tests but worth one live confirmation | After first auto-promote, confirm `config.json.bak.{ts}` exists with prior content |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (`promotion_test.go`, fakes file, REST endpoint test)
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s for quick run, < 300s for full
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
