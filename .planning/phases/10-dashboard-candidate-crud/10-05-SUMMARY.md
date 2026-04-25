# Plan 10-05 — Manual UAT Summary

**Status:** ✓ APPROVED by operator
**Date:** 2026-04-25
**Test binary:** `/tmp/arb-phase10` built from commits through Plan 10-04 (`55ff6ea`) + validator-loosening fix
**UAT operator:** user (egg5233)

## What was tested

16-step manual UAT against the test binary running on port 8080 with the live `config.json` (sandbox protected by pre-UAT backup `config.json.preuat-phase10`). Steps covered:

1-5  | PG-OPS-08 carryover — zero auto-POST `/api/config` on tab nav, modal open, Cancel, backdrop, ESC | **PASS**
6    | Add candidate `TESTUATUSDT` round-trip (gateio/bingx, threshold 300, max 1000, slip 10) | **PASS** (after validator fix below)
7    | Reload page — candidate persists | **PASS**
8    | `.bak` backup file created with older mtime | **PASS**
9    | Edit candidate (threshold 300 → 500), reload, persists | **PASS**
10-11| Delete confirm dialog correct, Cancel keeps row, zero POST | **PASS**
12   | Delete + Confirm, reload, row gone | **PASS**
13   | (skipped — no candidate had active position to test 409 against)
14   | zh-TW locale: modal labels, no overflow, server error in Chinese | **PASS**
15   | Phase-9 Disable button still opens Phase-9 modal (not new modal) | **PASS**
16   | Operator sign-off | **APPROVED**

## Defect found and fixed during UAT

**Defect:** Plan 10-01's validator bounds (D-07/D-08/D-09 in CONTEXT.md) were aspirational — `threshold_bps [50,1000]`, `max_position_usdt [100,50000]`, `modeled_slippage_bps [0,100]`. Live production candidates (SOONUSDT, RAVEUSDT, HOLOUSDT, SIGNUSDT) all use `threshold_bps=20` and `max_position_usdt=50`, **outside the original bounds**. Step 6 Save POSTed the full candidates array (per D-16 last-write-wins) → backend rejected all 4 existing candidates as "out of range".

**Fix shipped (single commit):** Loosened bounds in both backend and frontend to `(0, 10000]`, `(0, 500000]`, `[0, 1000]`. Updated:

- `internal/api/pricegap_handlers.go` — validator
- `internal/api/pricegap_candidates_validate_test.go` — test inputs (used `threshold_bps=0`, `max_position_usdt=0`, `modeled_slippage_bps=1001` to stay out-of-range)
- `web/src/pages/PriceGap.tsx` — client validation mirror
- `web/src/pages/PriceGap.candidates.test.ts` — frontend test mirror
- `web/src/i18n/en.ts` + `zh-TW.ts` — error message text (lockstep)
- `.planning/phases/10-dashboard-candidate-crud/10-CONTEXT.md` — D-07/D-08/D-09 amended with rationale

All tests green after fix:
- Go: `arb/internal/api` 0.066s OK
- Frontend native: 17/17 PASS, 164ms
- TypeScript: `tsc -b --noEmit` exit 0
- Vite build: success, dist regenerated for go:embed

## Ambient bug discovered (NOT Plan 10's fault — separate hotfix needed)

During UAT, the wipe-bug-in-question fired live. config.json went from 8870 bytes → 8675 bytes (-195 bytes) at `2026-04-25T11:56:40Z`. Damage:

| Field | Pre-UAT (correct) | Post-UAT (degraded) |
|---|---|---|
| `analytics.enable_analytics` | `true` | MISSING |
| `price_gap.debug_log` | `true` | MISSING |
| `spot_futures.backtest_enabled` | `true` | MISSING |
| `spot_futures.backtest_coinglass_fallback` | `true` | MISSING |
| `fund.capital_per_leg` | `200` | **`0`** |
| `fund.max_positions` | `8` | **`1`** |
| `risk.margin_l3_threshold` | `0.65` | `0.5` |

**Writer identified:** the test binary's own SaveJSON path (`handlers.go:1783` — same code as live arb's pre-Phase-10 line 1749). Pattern matches the previous mystery wipes (today 07:23 UTC, yesterday 13:36 + 21:36 UTC).

**Root cause hypothesis:** `Config.Load()` and `Config.SaveJSON()` are not symmetric:

- Some plain `bool` fields with `omitempty` JSON tags get stripped on save when value is `false`
- Some numeric fields are loaded as `0` (struct/JSON tag mismatch?) and written back as `0`, replacing the on-disk non-zero value

The wipe fires whenever ANY dashboard POST hits `handlePostConfig` → `SaveJSONWithExchangeSecretOverrides()`. This explains why every `Save` action — including the user's legitimate dashboard interactions — degrades the file.

**Recovery during UAT:** `cp config.json.preuat-phase10 config.json && sudo systemctl start arb` — restored 8870-byte clean state and resumed live trading.

**Audit-rule blind spot also exposed:** the syscall rule `-F path=/var/solana/data/arb/config.json` only matches the literal path string. Arb uses the **relative** path `"config.json"` (cwd is `/var/solana/data/arb`), so the rule misses arb's own writes. The complementary inode-watch rule (`-w /var/solana/data/arb/config.json`) is what should catch this regardless of path used. Worth re-enabling that form.

## Recommended follow-ups (NOT part of Phase 10)

Open as a separate hotfix or Phase 13 addition:

1. **Audit `Config.Load()` vs `SaveJSON()` for field symmetry.** Every field declared on `Config` should be:
   - Read from JSON in Load (no missing tags)
   - Written back in SaveJSON (no `omitempty` on plain `bool` where `false` is a valid persistent value)
2. **Add a Wave-0 round-trip test:** `LoadConfig() → SaveJSON() → LoadConfig()` and assert byte-equal. Would catch the bug instantly.
3. **Replace audit rule:** prefer `-w /var/solana/data/arb/config.json -p wa -k arb-config-write` (path-watch by inode) over the syscall rule with `-F path=`.
4. **Persist audit rules across reboot:** copy into `/etc/audit/rules.d/arb.rules`.

## Files modified (this plan)

- `.planning/phases/10-dashboard-candidate-crud/10-CONTEXT.md` (D-07/D-08/D-09 amended)
- `.planning/phases/10-dashboard-candidate-crud/10-05-SUMMARY.md` (this file, new)
- `internal/api/pricegap_handlers.go`
- `internal/api/pricegap_candidates_validate_test.go`
- `web/src/pages/PriceGap.tsx`
- `web/src/pages/PriceGap.candidates.test.ts`
- `web/src/i18n/en.ts`
- `web/src/i18n/zh-TW.ts`

## Sign-off

UAT operator: user — `approved` — 2026-04-25 ~12:00 UTC
Post-UAT: live arb resumed (PID 3506705 → restart on backup-restored config), `/tmp/arb-phase10` killed, port 8080 freed.
Phase 10 manual UAT plan complete.
