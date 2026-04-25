---
phase: 10-dashboard-candidate-crud
plan: 02
subsystem: i18n
tags: [pricegap, candidate-crud, i18n, en, zh-TW, lockstep]

# Dependency graph
requires:
  - phase: 09-price-gap-dashboard-paper-live-operations
    provides: existing pricegap.* namespace (67 keys per locale, parity-verified)
  - phase: 10-dashboard-candidate-crud
    plan: 01
    provides: backend error/HTTP-status taxonomy that the new i18n keys label
provides:
  - 23 pricegap.candidates.* keys in en.ts (modal labels, validation errors, confirmDelete dialog)
  - 23 pricegap.candidates.* keys in zh-TW.ts (Traditional Chinese, lockstep with en.ts)
  - TranslationKey type now includes the full pricegap.candidates.* surface — Plan 10-03 modal can call t('pricegap.candidates.*') with full type-safety
affects: [10-03-frontend-modal, 10-05-vitest-and-e2e]

# Tech tracking
tech-stack:
  added: []  # zero new dependencies — pure additive on existing locale objects
  patterns:
    - "Flat dotted keys (NOT nested objects) per RESEARCH.md Pitfall 5 + en.ts:766+ existing convention"
    - "Lockstep additions in same wave (en.ts + zh-TW.ts both edited; structurally enforced because zh-TW.ts is typed Record<TranslationKey, string>)"
    - "Interpolation placeholders {symbol}/{long}/{short}/{bps} — text-node renders only, no innerHTML (T-10-10 mitigation)"

key-files:
  created: []
  modified:
    - web/src/i18n/en.ts
    - web/src/i18n/zh-TW.ts

key-decisions:
  - "Flat dotted keys ('pricegap.candidates.modal.add.title') NOT nested objects — matches RESEARCH.md Pitfall 5 verbatim and existing pricegap.* block at en.ts:766+"
  - "Single-step delete confirm body interpolates 4 placeholders (symbol, long, short, bps) so the operator sees full candidate detail before destructive action — D-15 satisfied"
  - "zh-TW translation typography (Traditional Chinese conventions): substituted plan's ASCII colon/comma/question-mark with fullwidth equivalents (：，？) inside the Chinese values for typography consistency with existing pricegap.err.toggleFailed (U+FF1A verified). Key strings stay byte-identical to en.ts"
  - "Task 1 typecheck is necessarily RED in isolation because zh-TW.ts is typed Record<TranslationKey, string> (where TranslationKey = keyof typeof en); structural lockstep is enforced at the type level, even safer than the plan's grep-diff invariant. Both commits land in the same wave so the green state is restored within the plan boundary"

patterns-established:
  - "Pattern: Append new keys at the END of the existing namespace block in BOTH files, using the same insertion anchor (line preceding `} as const;` in en.ts and `};` in zh-TW.ts) for visual contiguity and easy review"
  - "Pattern: Verify lockstep with `diff <(grep -oE \"'pricegap\\\\.candidates\\\\.[a-zA-Z0-9_.]+'\" web/src/i18n/en.ts | sort -u) <(grep -oE … zh-TW.ts | sort -u)` — empty output is the gate"

requirements-completed: [PG-OPS-07]

# Metrics
duration: 6min
completed: 2026-04-25
---

# Phase 10 Plan 02: i18n Locale Keys Summary

**Both `web/src/i18n/en.ts` and `web/src/i18n/zh-TW.ts` now contain the 23 `pricegap.candidates.*` keys that Plan 10-03's modal will reference — modal labels, on-submit field validation errors, and the single-step confirmDelete dialog — with TypeScript-level lockstep enforcement and frontend build green.**

## Final 23-Key List (EN ↔ zh-TW)

| Key | EN value | zh-TW value |
|-----|----------|-------------|
| `pricegap.candidates.add.button` | `Add candidate` | `新增候選` |
| `pricegap.candidates.modal.add.title` | `Add candidate` | `新增候選` |
| `pricegap.candidates.modal.edit.title` | `Edit {symbol} ({long}/{short})` | `編輯 {symbol} ({long}/{short})` |
| `pricegap.candidates.modal.symbol.label` | `Symbol` | `交易對` |
| `pricegap.candidates.modal.symbol.placeholder` | `BTCUSDT` | `BTCUSDT` |
| `pricegap.candidates.modal.longExch.label` | `Long exchange` | `做多交易所` |
| `pricegap.candidates.modal.shortExch.label` | `Short exchange` | `做空交易所` |
| `pricegap.candidates.modal.thresholdBps.label` | `Threshold (bps)` | `觸發閾值 (bps)` |
| `pricegap.candidates.modal.maxPositionUSDT.label` | `Max position (USDT)` | `單筆上限 (USDT)` |
| `pricegap.candidates.modal.modeledSlippageBps.label` | `Modeled slippage (bps)` | `預估滑價 (bps)` |
| `pricegap.candidates.modal.save` | `Save` | `儲存` |
| `pricegap.candidates.modal.cancel` | `Cancel` | `取消` |
| `pricegap.candidates.errors.symbolFormat` | `Symbol must be uppercase, end with USDT` | `交易對必須為大寫，並以 USDT 結尾` |
| `pricegap.candidates.errors.exchangesEqual` | `Long and short exchanges must differ` | `做多與做空交易所不可相同` |
| `pricegap.candidates.errors.thresholdRange` | `Threshold must be 50–1000 bps` | `閾值需介於 50–1000 bps` |
| `pricegap.candidates.errors.maxPositionRange` | `Max position must be 100–50000 USDT` | `單筆上限需介於 100–50000 USDT` |
| `pricegap.candidates.errors.slippageRange` | `Modeled slippage must be 0–100 bps` | `滑價需介於 0–100 bps` |
| `pricegap.candidates.errors.tupleCollision` | `Candidate with this symbol+exchanges already exists` | `已存在相同的交易對+交易所組合` |
| `pricegap.candidates.errors.activePositionBlocksDelete` | `Cannot delete: candidate has an active position. Close it first.` | `無法刪除：此候選有未平倉的部位，請先平倉。` |
| `pricegap.candidates.confirmDelete.title` | `Delete candidate?` | `刪除候選？` |
| `pricegap.candidates.confirmDelete.body` | `{symbol}: {long} ↔ {short}, threshold {bps} bps. This cannot be undone.` | `{symbol}：{long} ↔ {short}，閾值 {bps} bps。此操作無法復原。` |
| `pricegap.candidates.confirmDelete.confirm` | `Delete` | `刪除` |
| `pricegap.candidates.confirmDelete.cancel` | `Cancel` | `取消` |

## Lockstep Diff — Empty (Zero Drift)

```bash
$ diff <(grep -oE "'pricegap\.candidates\.[a-zA-Z0-9_.]+'" web/src/i18n/en.ts | sort -u) \
       <(grep -oE "'pricegap\.candidates\.[a-zA-Z0-9_.]+'" web/src/i18n/zh-TW.ts | sort -u)
$ echo $?
0
```

Both locales contain the exact same 23 unique keys. Counts: `grep -c "pricegap.candidates"` returns `23` for both files.

Total `pricegap.*` namespace size after this plan: **90 keys per locale** (Phase 9's 67 + this plan's 23).

## Verification Results

| Check | Command | Result |
|-------|---------|--------|
| EN key count | `grep -c "pricegap.candidates" web/src/i18n/en.ts` | **23** |
| zh-TW key count | `grep -c "pricegap.candidates" web/src/i18n/zh-TW.ts` | **23** |
| Lockstep diff | `diff <(grep -oE … en.ts) <(grep -oE … zh-TW.ts)` | empty (rc=0) |
| TypeScript typecheck | `cd web && ./node_modules/.bin/tsc -b --noEmit` | exit 0 |
| Frontend build | `cd web && npm run build` | exit 0 (`vite v7.3.1 built in 4.72s`) |
| Traditional (not Simplified) zh values | manual visual | confirmed (儲存 not 保存; 無法 not 无法; 復原 not 复原; 閾值 not 阈值) |
| Modal title interpolation | inspection | `Edit {symbol} ({long}/{short})` placeholders present in both |
| Confirm body 4-way interpolation | inspection | `{symbol}` `{long}` `{short}` `{bps}` placeholders present in both |

## Tested Locale-Switch Behavior

Manual locale-switch testing is **deferred to Plan 10-05** (UAT pass). At this plan boundary:
- TypeScript `Record<TranslationKey, string>` typing on zh-TW.ts mathematically guarantees that any `t('pricegap.candidates.*')` call in Plan 10-03 will receive a string in BOTH locales (no English fallback at runtime — Pitfall 5 mitigated structurally, not just by lint).
- `vite build` succeeded with both files in their final state, so the bundled `web/dist/assets/index-*.js` already contains both translation tables for the embedded Go binary's next compile.

Plan 10-05 must run a manual locale-switch on the rendered modal to confirm:
1. zh-TW modal renders all labels in Traditional Chinese, no English bleed.
2. Layout does not wrap awkwardly (zh-TW labels are slightly longer than EN for some fields, e.g., 預估滑價 vs Modeled slippage).
3. Interpolation placeholders ({symbol}, {long}, {short}, {bps}) substitute cleanly in both locales.

## Note for Plan 10-03 (Frontend Modal)

**Complete key list available — modal MUST use `t('pricegap.candidates.*')` exclusively, no inline strings.**

Mapping from backend (Plan 10-01) error semantics to i18n keys for the modal's error display:

| Backend HTTP / error | i18n key |
|----------------------|----------|
| 400 — `symbol must match ^[A-Z0-9]+USDT$` | `pricegap.candidates.errors.symbolFormat` |
| 400 — `long_exch must differ from short_exch` | `pricegap.candidates.errors.exchangesEqual` |
| 400 — `threshold_bps out of range [50,1000]` | `pricegap.candidates.errors.thresholdRange` |
| 400 — `max_position_usdt out of range [100,50000]` | `pricegap.candidates.errors.maxPositionRange` |
| 400 — `modeled_slippage_bps out of range [0,100]` | `pricegap.candidates.errors.slippageRange` |
| 400 — `duplicate tuple` | `pricegap.candidates.errors.tupleCollision` |
| 409 — `candidate <SYM>/<long>/<short> has active position; close it first (position id=…)` | `pricegap.candidates.errors.activePositionBlocksDelete` |

Plan 10-03 modal flow:
1. Local on-submit validation (D-03) — emit the matching `errors.*` key per failed field.
2. On 400 from server — parse the collated `; `-joined error string and surface the matching `errors.*` keys (server message may be unlocalised, modal must NOT echo it raw to the user).
3. On 409 — show the dedicated `errors.activePositionBlocksDelete` copy AND link to the position row per D-14.
4. Confirm-delete dialog body MUST use `pricegap.candidates.confirmDelete.body` with all 4 interpolations supplied (`{symbol}`, `{long}`, `{short}`, `{bps}`).

## Deviations from Plan

### Auto-fixed Adaptations

**1. [Rule 1 — Bug] Translation typography for zh-TW values**
- **Found during:** Task 2 — Edit-tool string match failure on the apparent ASCII colon `:` in the existing `'pricegap.err.toggleFailed': '切換失敗:{error}'` line.
- **Issue:** Hex-dump (`xxd`) revealed the existing zh-TW.ts file uses U+FF1A FULLWIDTH COLON `：` (`efbc9a`) not the ASCII colon. The plan's RESEARCH.md key list at lines 497–522 used ASCII colon `:`, ASCII comma `,`, and ASCII question mark `?` inside the Chinese values.
- **Fix:** Substituted ASCII punctuation with fullwidth equivalents inside Chinese strings: `:` → `：` (U+FF1A), `,` → `，` (U+FF0C), `?` → `？` (U+FF1F). Affects 3 values: `errors.activePositionBlocksDelete` (1× colon, 1× comma); `confirmDelete.title` (1× question mark); `confirmDelete.body` (1× colon, 1× comma); plus `errors.symbolFormat` (1× comma). KEY STRINGS STAY BYTE-IDENTICAL TO EN.TS.
- **Why correct:** Traditional Chinese typographic convention places fullwidth punctuation between CJK characters (matches the surrounding existing zh-TW.ts text). Using ASCII punctuation here would render visibly inconsistent with the rest of the file's typography (jagged kerning, half-width gaps inside CJK runs).
- **Files modified:** `web/src/i18n/zh-TW.ts` only.
- **Commit:** `65c715f`.

### Other Deviations

None. Plan executed as written for all key strings, key counts, file targets, and verification commands.

## Authentication Gates

None — pure file edits with local typecheck/build verification.

## Self-Check: PASSED

- File `web/src/i18n/en.ts` modified (23 new keys appended at end of pricegap.* block) — VERIFIED via `grep -c "pricegap.candidates" web/src/i18n/en.ts` = 23.
- File `web/src/i18n/zh-TW.ts` modified (matching 23 keys with Traditional Chinese values) — VERIFIED via `grep -c "pricegap.candidates" web/src/i18n/zh-TW.ts` = 23.
- Commit `b9cb12e` (feat, Task 1 — en.ts) — VERIFIED via `git log --oneline | grep b9cb12e`.
- Commit `65c715f` (feat, Task 2 — zh-TW.ts) — VERIFIED via `git log --oneline | grep 65c715f`.
- Lockstep diff empty — VERIFIED (rc=0, zero-drift).
- `cd web && ./node_modules/.bin/tsc -b --noEmit` exit 0 — VERIFIED (TSC_EXIT=0).
- `cd web && npm run build` exit 0 — VERIFIED (BUILD_EXIT=0; vite built in 4.72s; web/dist regenerated).
- All 5 success_criteria from PLAN.md satisfied:
  1. 23 new keys in each file ✓
  2. Both files typecheck ✓
  3. Frontend build succeeds (web/dist/ regenerated for Plan 03 + Plan 05) ✓
  4. All Plan 03 modal labels, validation errors, and confirm copy reachable via t() with correct keys ✓ (key list provided above)
  5. EN/zh-TW key sets identical (zero drift verified by diff) ✓
