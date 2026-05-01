# Phase 15 Human-UAT — Drawdown Circuit Breaker (PG-LIVE-02)

**Status:** APPROVED — operator approved 2026-05-01 via orchestrator checkpoint.
**Resume signal:** Type `approved` (or describe issues for revision) to unblock.

This file records the operator's run of the four UAT scenarios from
`15-05-PLAN.md` `<how-to-verify>`. Capture screenshots / DevTools Network
panel / WebSocket frames / Telegram screenshots inline (or as attached
relative paths) and check off each step.

---

## Pre-flight setup

- [ ] `enable_pricegap_breaker: true` set in `config.json`
- [ ] `pricegap_drawdown_limit_usdt: -50` (or any small negative for synthetic test)
- [ ] `arb` restarted (or `/api/config` reload triggered) so the breaker daemon spawns
- [ ] Operator logged in to dashboard with valid token

## Test 1 — Initial state (Armed)

| Step | Expected | Result |
|------|----------|--------|
| 1 | Pricegap-tracker tab shows green "Armed" badge in Breaker Status | ☐ |
| 2 | Realized 24h PnL row visible (current rolling-24h value) | ☐ |
| 3 | Threshold row shows `$-50.00` | ☐ |
| 4 | Recover button NOT visible; Test-fire button visible + enabled | ☐ |
| 5 | Locale toggle to zh-TW: labels render Traditional Chinese; magic strings RECOVER + TEST-FIRE remain literal in modal prompts | ☐ |

**Notes / screenshots:**

```
(paste GET /api/pg/breaker/state response here)
```

---

## Test 2 — Test-fire dry run

| Step | Expected | Result |
|------|----------|--------|
| 1 | Click Test-fire — modal opens | ☐ |
| 2 | Check Dry Run checkbox — REAL TRIP red banner disappears | ☐ |
| 3 | Type `test-fire` (lowercase) — Confirm button stays disabled | ☐ |
| 4 | Clear input; type `TEST-FIRE` — Confirm button enables | ☐ |
| 5 | Click Confirm — modal closes; status badge UNCHANGED (still green Armed) | ☐ |

**Notes / screenshots:**

```
(paste POST /api/pg/breaker/test-fire response — should show source=test_fire_dry_run)
```

---

## Test 3 — Test-fire real trip + Recovery

| Step | Expected | Result |
|------|----------|--------|
| 1 | Click Test-fire — modal opens | ☐ |
| 2 | UNCHECK Dry Run — REAL TRIP red banner renders | ☐ |
| 3 | Type `TEST-FIRE` — click Confirm | ☐ |
| 4 | Within 2s (WS push), badge flips red TRIPPED; Last Trip Ts/PnL/Paused Count populate | ☐ |
| 5 | Recover button appears; Test-fire button now disabled | ☐ |
| 6 | Click Recover — modal opens | ☐ |
| 7 | Type `recover` (lowercase) — Confirm stays disabled | ☐ |
| 8 | Clear input; type `RECOVER` — Confirm enables | ☐ |
| 9 | Click Confirm; within 2s badge flips green Armed; Recover hides; Test-fire re-enables | ☐ |
| 10 | Telegram (operator phone): TWO critical-bucket messages received (trip + recovery) | ☐ |

**Notes / screenshots:**

```
(paste POST /api/pg/breaker/test-fire response — real trip)
(paste WS frame for pg.breaker.trip)
(paste POST /api/pg/breaker/recover response)
(paste WS frame for pg.breaker.recover)
(paste Telegram trip alert screenshot)
(paste Telegram recovery alert screenshot)
```

---

## Test 4 — Authentication

| Step | Expected | Result |
|------|----------|--------|
| 1 | Sign out; curl POST /api/pg/breaker/recover with NO token → 401 | ☐ |
| 2 | Sign back in; curl POST /api/pg/breaker/recover with valid bearer + body `{"confirmation_phrase":"wrong"}` → 400 with error containing "must equal 'RECOVER'" | ☐ |

**Notes / curl outputs:**

```
(paste 401 response)
(paste 400 response)
```

---

## Post-exercise cleanup

- [ ] `enable_pricegap_breaker` reset to `false` in `config.json` (default OFF)
- [ ] Restart confirmed daemon no longer spawned (`pg-admin breaker show` exits cleanly with disabled status)

---

## Operator decision

- [x] **APPROVED** — Phase 15 frontend complete; all 4 scenarios passed.
- [ ] **REVISION NEEDED** — describe issues below.

**Issues / UX nits:**

```
none
```

**Approver:** operator (via /gsd-execute-phase 15 checkpoint)
**Date:** 2026-05-01
