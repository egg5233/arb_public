---
phase: 15
plan: 05
subsystem: pricegaptrader
tags: [drawdown-breaker, frontend, dashboard, typed-phrase-modal, i18n-lockstep, wave-3]
requires:
  - phase-15-plan-04-operations-surface
provides:
  - hooks.usePgBreaker
  - components.Ramp.BreakerSubsection
  - components.Ramp.BreakerConfirmModal
  - i18n.pricegap.breaker.* (22 keys lockstep en + zh-TW)
  - dashboard-widget:breaker-subsection-in-ramp-reconcile-section
affects:
  - web/src/i18n/en.ts (22 breaker keys appended)
  - web/src/i18n/zh-TW.ts (22 breaker keys appended; Traditional Chinese values)
  - web/src/hooks/usePgBreaker.ts (NEW)
  - web/src/components/Ramp/BreakerSubsection.tsx (NEW)
  - web/src/components/Ramp/BreakerConfirmModal.tsx (NEW)
  - web/src/components/Ramp/RampReconcileSection.tsx (mounts BreakerSubsection at bottom)
tech-stack:
  added: []
  patterns:
    - "Self-fetching FC pattern with own /ws connection (mirrors usePgDiscovery + Phase 14 RampReconcileSection — no token prop drilling)"
    - "Re-seed on WS reconnect to recover state drift during disconnect window"
    - "Case-sensitive typed-phrase gate with EXACT match (no trim, no toUpperCase) — T-15-13 mitigation"
    - "Magic strings RECOVER + TEST-FIRE LITERAL in code AND i18n strings (D-12)"
    - "i18n parity test (parity.test.ts) gates compile-time lockstep — zero diff between en + zh-TW key sets"
    - "Dot-notation i18n keys (pricegap.breaker.*) matches existing pricegap.ramp.* / pricegap.discovery.* convention"
key-files:
  created:
    - web/src/hooks/usePgBreaker.ts
    - web/src/components/Ramp/BreakerSubsection.tsx
    - web/src/components/Ramp/BreakerConfirmModal.tsx
  modified:
    - web/src/i18n/en.ts
    - web/src/i18n/zh-TW.ts
    - web/src/components/Ramp/RampReconcileSection.tsx
decisions:
  - "Component path: web/src/components/Ramp/ (matches existing Phase 14 RampReconcileSection.tsx location). Plan literal said web/src/components/PriceGap/ but that directory does not exist in this codebase — Rule 3 blocking adaptation"
  - "Hook signature: usePgBreaker() takes no args (reads localStorage.getItem('arb_token') internally). Plan literal said usePgBreaker(token) but Phase 14 RampReconcileSection / DiscoverySection precedent uses self-fetching FC pattern; consistency wins"
  - "i18n key naming: dot-notation pricegap.breaker.* (matches existing pricegap.ramp.* + pricegap.discovery.* + pricegap.reconcile.* convention). Plan literal showed camelCase keys (breakerArmed) — adapted to project convention"
  - "Magic strings RECOVER + TEST-FIRE embedded as TS literals in usePgBreaker + BreakerConfirmModal props (NOT i18n keys) — D-12 invariant. zh-TW prompt strings preserve them literally inside the surrounding Chinese prose"
  - "WS event name match: pg.breaker.trip + pg.breaker.recover (matches Hub.BroadcastPriceGapBreakerEvent calls in Plan 15-03 trip + Plan 15-04 recovery paths)"
  - "Status badge tri-state: gray (Disabled — cfg flag false), red (Tripped — sticky != 0), green (Armed — enabled && !tripped). Maps to CONTEXT 'Specifics' color spec; cell falls through to gray when neither armed nor tripped"
  - "Recover button visible only when state.tripped === true; Test-fire button always rendered but disabled={state.tripped} (T-15-16 client-side guard mirrors server 409)"
  - "Dry-run defaults to FALSE inside the test-fire modal (operator must opt INTO dry-run). The safer default is the explicit one — REAL TRIP warning is shown until checkbox is checked. Pitfall 7 mitigation"
  - "REAL-TRIP warning banner conditional on (action===test-fire && !dryRun) — flips on/off as operator toggles checkbox in real-time"
  - "Modal Confirm button submits via onConfirm({dryRun}) callback — recover variant ignores dryRun; test-fire passes through. Single shared component for two actions, distinct title + prompt + magic phrase via props"
  - "Asia/Taipei timestamp: formatAsiaTaipei(ms) uses toLocaleString('sv-SE',{timeZone:'Asia/Taipei'}) — matches Phase 14 RampReconcileSection's formatNullableDate pattern"
metrics:
  duration: ~25min
  tasks: 1   # Task 2 is operator-led human-verify; not counted in code duration
  files_created: 3
  files_modified: 3
  i18n_keys_added: 22
  completed_date: 2026-05-01
---

# Phase 15 Plan 05: Drawdown Breaker Dashboard Subsection Summary

**One-liner:** BreakerSubsection lands inside the Phase 14 RampReconcileSection on the Pricegap-tracker tab — usePgBreaker hook (REST seed + WS subscribe), BreakerConfirmModal (case-sensitive typed-phrase gate with REAL-TRIP warning), 22 i18n keys lockstep en + zh-TW. Operator can now visually monitor breaker state (Armed/Tripped/Disabled badge), inspect last-trip stats (Asia/Taipei), and exercise the recover/test-fire flow without touching pg-admin CLI. Phase 15 frontend feature-complete behind the existing PriceGapBreakerEnabled config flag.

## Objective Recap

Plan 15-05 closes Phase 15's frontend by attaching the operator-facing UI to the BreakerController + REST surface that landed in Plans 15-03 / 15-04. Three deliverables:

1. **`usePgBreaker` hook** — REST seed via GET /api/pg/breaker/state on mount + WS subscription on /ws (own connection, mirrors usePgDiscovery pattern) listening for `pg.breaker.trip` and `pg.breaker.recover` events from the BreakerController. Mutators `recover()` and `testFire(dryRun)` POST the typed-phrase REST endpoints with the LITERAL magic strings.
2. **`BreakerConfirmModal`** — shared typed-phrase modal used for both recover + test-fire. Enforces EXACT case-sensitive match against `RECOVER` / `TEST-FIRE` (no trim, no toUpperCase) before enabling Confirm. Test-fire variant exposes a Dry-Run checkbox; when unchecked, the prominent red REAL-TRIP warning banner renders above the input.
3. **`BreakerSubsection` + parent integration** — status badge (red Tripped / green Armed / gray Disabled), 24h PnL + threshold heartbeat, last-trip stats (Asia/Taipei timestamp + PnL + paused count), Recover button (visible only when Tripped), Test-fire button (always visible; disabled when Tripped per T-15-16). Mounted at the bottom of `RampReconcileSection.tsx` so Phase 16 PG-OPS-09 can lift it as-is.

## What Shipped

### Task 1 — i18n keys + breaker hook + sub-components + parent integration (commit `d1c8766`)

#### i18n: 22 new keys lockstep en + zh-TW

| Key | en | zh-TW |
|---|---|---|
| `pricegap.breaker.status` | Breaker Status | 熔斷器狀態 |
| `pricegap.breaker.armed` | Armed | 武裝中 |
| `pricegap.breaker.tripped` | TRIPPED | 已觸發 |
| `pricegap.breaker.disabled` | Disabled | 已停用 |
| `pricegap.breaker.realized24hPnl` | Realized 24h PnL | 24小時已實現損益 |
| `pricegap.breaker.threshold` | Threshold | 閾值 |
| `pricegap.breaker.lastTripTs` | Last Trip | 上次觸發時間 |
| `pricegap.breaker.lastTripPnl` | Last Trip PnL | 上次觸發時損益 |
| `pricegap.breaker.pausedCount` | Paused Candidates | 已暫停候選 |
| `pricegap.breaker.recoverButton` | Recover | 恢復 |
| `pricegap.breaker.testFireButton` | Test Fire | 測試觸發 |
| `pricegap.breaker.confirmRecoverPrompt` | Type 'RECOVER' to confirm operator-initiated recovery: | 輸入 'RECOVER' 確認操作員啟動恢復: |
| `pricegap.breaker.confirmTestFirePrompt` | Type 'TEST-FIRE' to confirm synthetic breaker test-fire: | 輸入 'TEST-FIRE' 確認測試觸發: |
| `pricegap.breaker.dryRunCheckbox` | Dry run (compute would-trip PnL, no mutations) | 空跑(計算 would-trip PnL,不執行寫入) |
| `pricegap.breaker.realTripWarning` | ⚠️ Default behavior is REAL TRIP — engine will flip to paper mode. Check Dry Run for simulation. | ⚠️ 預設行為為真實觸發 — 引擎將切換為紙上交易模式。勾選空跑進行模擬。 |
| `pricegap.breaker.recoveryInstruction` | Use the Recover button to clear sticky paper mode after operator review. | 操作員審查後使用恢復按鈕清除黏滯紙上交易旗標。 |
| `pricegap.breaker.confirmTitleRecover` | Confirm Operator Recovery | 確認操作員恢復 |
| `pricegap.breaker.confirmTitleTestFire` | Confirm Synthetic Test-Fire | 確認合成測試觸發 |
| `pricegap.breaker.confirmCancel` | Cancel | 取消 |
| `pricegap.breaker.confirmSubmit` | Confirm | 確認 |
| `pricegap.breaker.confirmTypePlaceholder` | Type the magic phrase exactly | 請完全照寫魔法字串 |
| `pricegap.breaker.errorRequest` | Request failed: {error} | 請求失敗:{error} |

**Magic-string preservation verified:** `grep -q "RECOVER" web/src/i18n/zh-TW.ts` and `grep -q "TEST-FIRE" web/src/i18n/zh-TW.ts` both return matches inside the surrounding Chinese prose. Operators using zh-TW locale type the same English magic phrases — D-12 invariant.

#### `usePgBreaker` hook (`web/src/hooks/usePgBreaker.ts`)

```typescript
export function usePgBreaker(): {
  state: BreakerState | null;
  recover: () => Promise<MutatorResult>;
  testFire: (dryRun: boolean) => Promise<MutatorResult>;
  refresh: () => Promise<void>;
  wsConnected: boolean;
  seedError: string | null;
  busy: boolean;
};
```

State shape mirrors `handlePgBreakerStateGet` Go response:
```typescript
interface BreakerState {
  enabled: boolean;
  pending_strike: number;
  strike1_ts_ms: number;
  sticky_until_ms: number;
  last_eval_pnl_usdt: number;
  last_eval_ts_ms: number;
  threshold_usdt: number;
  armed: boolean;
  tripped: boolean;
  last_trip: BreakerTripRecord | null;
}
```

**REST seed:** `GET /api/pg/breaker/state` on mount with `Authorization: Bearer ${token}` from localStorage. Re-seed on WS reconnect (false→true edge) to recover state drift during the disconnect window.

**WebSocket subscription:** opens own `/ws?token=...` connection. Switches on msg.type:
- `pg.breaker.trip` → marks `tripped=true, armed=false, sticky_until_ms=trip_ts_ms, last_trip=payload`
- `pg.breaker.recover` → marks `tripped=false, armed=enabled, sticky_until_ms=0, pending_strike=0, strike1_ts_ms=0, last_trip=payload??prev`

WS connection independent from the page-level `usePriceGapWebSocket` in PriceGap.tsx — clean unmount semantics; future Phase 16 lift-out is a pure relocation.

**Mutators:** `recover()` POSTs `/api/pg/breaker/recover` with `{confirmation_phrase: "RECOVER"}`; `testFire(dryRun)` POSTs `/api/pg/breaker/test-fire` with `{confirmation_phrase: "TEST-FIRE", dry_run}`. Both call `void seed()` after success as belt+suspenders since the WS push handles the canonical state transition. Error path returns `{ok:false, error}` for the modal to surface.

#### `BreakerConfirmModal` (`web/src/components/Ramp/BreakerConfirmModal.tsx`)

Shared typed-phrase modal driven by props:

```typescript
interface Props {
  open: boolean;
  action: 'recover' | 'test-fire';
  magicPhrase: 'RECOVER' | 'TEST-FIRE';
  promptKey: TranslationKey;
  busy: boolean;
  errorMessage?: string | null;
  onClose: () => void;
  onConfirm: (opts: { dryRun: boolean }) => void;
}
```

**Validation:** `valid = (typed === magicPhrase)` — EXACT case-sensitive match. No `.trim()`, no `.toUpperCase()`. Confirm button stays disabled until valid OR busy.

**Test-fire variant only:** Dry-Run checkbox above the prompt; defaults to FALSE (operator must opt INTO dry-run). REAL-TRIP red-banner warning conditional on `(action==='test-fire' && !dryRun)` — toggles in real-time as operator clicks the checkbox.

**Confirm button color:** red (real trip) when `showRealTripWarning`, blue otherwise. The visual change reinforces the danger context per CONTEXT "Pitfall 7" mitigation.

**A11y:** `role="dialog"`, `aria-labelledby="pg-breaker-confirm-title"`, ESC closes (gated on busy). Click outside the panel closes. Phase 10 modal pattern parity.

#### `BreakerSubsection` (`web/src/components/Ramp/BreakerSubsection.tsx`)

Renders the breaker card inside RampReconcileSection. Visible elements:

| Field | Source | Notes |
|---|---|---|
| Status badge | `state.enabled` + `state.tripped` | gray Disabled / red TRIPPED / green Armed |
| 24h Realized PnL | `state.last_eval_pnl_usdt` | Color-coded; `$X.XX` format |
| Threshold | `state.threshold_usdt` | `$X.XX` format |
| Last Trip Ts | `state.last_trip?.trip_ts_ms` | `formatAsiaTaipei` (UTC+8) |
| Last Trip PnL | `state.last_trip?.trip_pnl_usdt` | Color-coded |
| Paused Count | `state.last_trip?.paused_candidate_count` | integer |
| Recover button | rendered when `state.tripped` | yellow accent |
| Test-fire button | always rendered; `disabled={state.tripped}` | blue; T-15-16 client guard |

`recovery_ts_ms` / `recovery_operator` from the trip record are accessible on `state.last_trip` for future surfacing; this iteration shows trip metadata only (recovery is implied when tripped flips to false).

#### Parent integration — `RampReconcileSection.tsx`

```tsx
import { BreakerSubsection } from './BreakerSubsection.tsx';

// inside the RampReconcileSection FC, after the reconcile block:
<BreakerSubsection />
```

The breaker subsection appears at the bottom of the existing widget on the Pricegap-tracker tab. Phase 16 PG-OPS-09 will move both `RampReconcileSection` and `BreakerSubsection` into the new top-level Pricegap tab without restructuring either component.

## Verification

| Verification step | Result |
|---|---|
| `cd web && npm run build` | Pass — TypeScript strict-mode green; bundle 972 kB (within size budget) |
| `go build ./...` | Pass — go:embed picks up the new `web/dist/` |
| `node --test --experimental-strip-types web/src/i18n/parity.test.ts` | 5/5 pass — en + zh-TW key counts identical, every key in both, direction namespace intact |
| `grep -c "pricegap.breaker" web/src/i18n/en.ts` | 22 (≥14 acceptance threshold) |
| `grep -c "pricegap.breaker" web/src/i18n/zh-TW.ts` | 22 (≥14 acceptance threshold) |
| `grep "'pricegap.breaker.armed'" web/src/i18n/zh-TW.ts` | `武裝中` — Chinese value, NOT English copy |
| `grep "RECOVER" web/src/i18n/zh-TW.ts` | matches inside Chinese prompt — magic string preserved literally |
| `grep "TEST-FIRE" web/src/i18n/zh-TW.ts` | matches inside Chinese prompt — magic string preserved literally |
| `test -f web/src/hooks/usePgBreaker.ts` | exists |
| `test -f web/src/components/Ramp/BreakerSubsection.tsx` | exists |
| `test -f web/src/components/Ramp/BreakerConfirmModal.tsx` | exists |
| `grep -q "<BreakerSubsection" web/src/components/Ramp/RampReconcileSection.tsx` | Pass — parent integration |
| `git diff --quiet config.json` | Pass — config.json untouched |
| No `npm install\|npm update\|npx` invocations | Confirmed — only `npm run build` (which calls tsc + vite, no install) |

## Deviations from Plan

Three Rule 3 (blocking-issue auto-fix) adaptations because the plan literals referenced paths/APIs that didn't match the actual codebase. None changed the spirit of the plan — the safety properties (typed-phrase gate, magic-string literal preservation, i18n lockstep, Asia/Taipei timestamps, color-coded badges) all hold.

### 1. [Rule 3 - Blocking] Component directory path

**Found during:** Task 1 read-first scan.

**Issue:** Plan `<files>` block named `web/src/components/PriceGap/BreakerSubsection.tsx` etc. The directory `web/src/components/PriceGap/` does NOT exist — the Phase 14 widget lives at `web/src/components/Ramp/RampReconcileSection.tsx`.

**Fix:** Placed the new components under `web/src/components/Ramp/` matching the existing convention. The naming is loose (Ramp/ holds both Ramp+Reconcile+Breaker now); Phase 16 PG-OPS-09 will reorganize the dashboard into a top-level Pricegap tab anyway, so renaming the directory now would create a churn that gets undone two phases later.

### 2. [Rule 3 - Blocking] Hook signature — no token prop

**Found during:** Task 1 hook drafting.

**Issue:** Plan literal `usePgBreaker(token: string)` and `<BreakerSubsection token={token} />` pattern. Phase 14 RampReconcileSection is a self-fetching FC reading `localStorage.getItem('arb_token')` directly inside `useEffect`. DiscoverySection / usePgDiscovery follow the same pattern.

**Fix:** `usePgBreaker()` takes no args; reads token from localStorage internally via shared `authHeaders()` helper. `<BreakerSubsection />` takes no props. Matches Phase 14 + Phase 11 precedent — consistency wins.

### 3. [Rule 3 - Blocking] i18n key naming convention

**Found during:** Task 1 en.ts edit.

**Issue:** Plan literal showed camelCase keys like `breakerStatus`, `breakerArmed`. Existing `pricegap.ramp.title`, `pricegap.discovery.title`, `pricegap.reconcile.title` use dot-notation. The `TranslationKey` type is derived from the literal `en` object, so the dot-notation strings are the convention.

**Fix:** Used `pricegap.breaker.*` dot-notation matching existing namespace. All 22 keys follow `pricegap.breaker.{field}` shape. The parity test passes; CHANGELOG-style key audits still grep for `pricegap.breaker.` cleanly.

### Bonus tracking — extra i18n keys beyond plan minimum

Plan listed 16 expected breaker keys; this implementation ships **22** (4 of the extras are modal scaffolding: `confirmTitleRecover`, `confirmTitleTestFire`, `confirmCancel`, `confirmSubmit`, `confirmTypePlaceholder`, `errorRequest`). All 22 are lockstep across en + zh-TW; the parity test enforces equality.

## Authentication Gates

None — pure code changes. The frontend reads the existing `arb_token` from localStorage; the dashboard authentication layer (Phase 1+ behavior) was already exercised. No new auth surface is introduced.

## Migration Notes

**Operator opt-in unchanged from Plan 15-04:** Phase 15 still ships behind `cfg.PriceGapBreakerEnabled` (default OFF). When the operator flips it true and the daemon spawns:
1. Visit the Pricegap-tracker tab — the new `BreakerSubsection` renders below the existing Ramp + Reconcile cards.
2. Initial state: gray `Disabled` badge if cfg flag is false; green `Armed` if cfg=true and not tripped; red `TRIPPED` if sticky != 0.
3. Test-fire from UI (preferred over `pg-admin` CLI) for visual confidence — REAL trip dispatches the same Telegram alerts and routes through the same Plan 15-03 D-15 ordering.
4. After the operator-recovery flow is exercised once on staging, set `enable_pricegap_breaker: false` in config.json (matches CHANGELOG.md "default OFF" guidance).

**Phase 16 PG-OPS-09 lift-out plan:** When the new top-level Pricegap tab is built, both `RampReconcileSection` and `BreakerSubsection` move out of `web/src/pages/PriceGap.tsx` and into the new tab's container. Both components are dependency-clean (token from localStorage, own /ws connections, own hooks); the move is a pure file relocation with zero functional changes expected.

## Known Stubs

None. All breaker UI is wired to live data via the BreakerControllerAPI + LoadBreakerTripAt routes shipped in Plan 15-04. The HUMAN-UAT exercise (Task 2) verifies the full end-to-end flow including Telegram + WS + REST round-trip.

## Threat Flags

None. Plan 15-05 introduces:
- **No new server endpoints** — consumes the 3 routes shipped in Plan 15-04 (auth-gated + typed-phrase-validated server-side).
- **No new WS event types** — consumes `pg.breaker.trip` + `pg.breaker.recover` shipped in Plan 15-03 + 15-04.
- **No new auth paths** — Bearer token from existing localStorage `arb_token` (Phase 1+ behavior).

The threat register T-15-18 / T-15-19 / T-15-20 mitigations are reaffirmed:
- **T-15-18** (client-side typed-phrase bypass): Server re-validates `confirmation_phrase` exact-match in `handlePgBreakerRecover` + `handlePgBreakerTestFire` (Plan 15-04). Client modal is UX, not security.
- **T-15-19** (i18n string disclosure): All 22 breaker translation strings are operational labels — no secrets, no tokens.
- **T-15-20** (operator clicks wrong button): Typed-phrase requirement on EVERY mutation surface (D-12). Magic strings RECOVER + TEST-FIRE remain LITERAL even in zh-TW prose. REAL-TRIP banner prominently warns when test-fire dry-run is unchecked.

## Self-Check: PASSED

- `web/src/hooks/usePgBreaker.ts` — FOUND
- `web/src/components/Ramp/BreakerSubsection.tsx` — FOUND
- `web/src/components/Ramp/BreakerConfirmModal.tsx` — FOUND
- `web/src/components/Ramp/RampReconcileSection.tsx` — `<BreakerSubsection />` import + render present
- `web/src/i18n/en.ts` — 22 `pricegap.breaker.*` keys appended
- `web/src/i18n/zh-TW.ts` — 22 keys lockstep with Traditional Chinese values; RECOVER + TEST-FIRE preserved literally
- Frontend build (`cd web && npm run build`) — Pass
- Go build (`go build ./...`) — Pass
- i18n parity test (`node --test`) — 5/5 pass
- Commit `d1c8766` (Task 1) — FOUND

## Pending: Human-Verify Checkpoint (Task 2)

Operator must record the full end-to-end UAT flow per the `<how-to-verify>` block in 15-05-PLAN.md:
1. Initial state (Armed, locale switch zh-TW, magic strings literal)
2. Test-fire dry run (warning hidden, lowercase rejected, exact match enables Confirm)
3. Test-fire real trip + Recovery (badge red→trip stats populate→Recover→badge green; Telegram trip+recover messages received)
4. Authentication (401 without token; 400 with wrong phrase)

Outcomes recorded in `.planning/phases/15-drawdown-circuit-breaker/15-HUMAN-UAT.md` (template stubbed for operator). Reset `enable_pricegap_breaker=false` after the exercise (default OFF).
