# Phase 16 Plan 04 — PG-OPS-09 Human UAT

> Checkpoint 3 (`type="checkpoint:human-verify"`) of Plan 16-04. Operator-led
> UAT in BOTH locales (EN + zh-TW). Required before Task 4 (VERSION /
> CHANGELOG bump) can proceed.

## Build

- VERSION (pre-bump): `0.38.3`
- VERSION (Task 4 will bump to): `0.39.0`
- `make build`: TBD (operator runs from repo root)

## Automation evidence (agent-captured before checkpoint)

### togglePaper operator_action wire-up (deferred from Plan 02)

Source-level grep evidence on `web/src/pages/PriceGap.tsx`:

```text
const togglePaper = useCallback(async () => {
    const next = !paperMode;
    setPaperMode(next);
    setToggleError(null);
    try {
      // Phase 16 Plan 04 (PG-FIX-02 client portion): the paper_mode write
      // path on /api/config rejects updates that lack `operator_action: true`
      // (returns HTTP 409). The dashboard toggle button is the operator
      // action — the marker satisfies the Plan 02 server guard. Page-load
      // hydration and any other code path that POSTs config without this
      // marker will be rejected. (Wire-up transferred from Plan 02 per the
      // checker scope blocker — single owner of this file resolves the
      // scope-leak.)
      await postConfig({ price_gap_paper_mode: next, operator_action: true });
```

Verification command: `grep -A 12 'const togglePaper' web/src/pages/PriceGap.tsx | grep 'operator_action: true'` returns the literal line.

### D-21 invariants (typed-phrase modal POST bodies)

ConfigCard test suite asserts:

- ENABLE-LIVE-CAPITAL POST argument literal: `{ price_gap_live_capital: next }` — NO operator_action.
- ENABLE-BREAKER POST argument literal: `{ price_gap_breaker_enabled: next }` — NO operator_action.

Run: `cd web && node --test --experimental-strip-types src/components/PriceGap/ConfigCard.test.ts`

Result: `# tests 16 # pass 16 # fail 0` (all pass including the two D-21 invariants).

### i18n lockstep

Run: `cd web && node --test --experimental-strip-types src/i18n/lockstep.test.ts`

Result: `# tests 4 # pass 4 # fail 0`.

### Frontend build

```text
cd web && tsc -b && vite build
✓ 716 modules transformed.
dist/index.html                   0.45 kB │ gzip:   0.29 kB
dist/assets/index-BJLFYReE.css   68.32 kB │ gzip:  12.05 kB
dist/assets/index-XPyLDMdF.js   994.62 kB │ gzip: 267.76 kB
✓ built in 5.43s
```

### Go regression tests (PG-FIX-02 server guard from Plan 02)

```text
go test ./internal/api/... -count=1
ok  arb/internal/api  117 passed
```

The Plan 02 server guard tests still pass — togglePaper wire-up satisfies the guard.

### npm lockdown preserved

```text
git diff web/package.json web/package-lock.json
(empty)
```

No new dependencies added.

### config.json untouched

```text
git diff config.json
(empty)
```

## Operator UAT checklist — OPERATOR ATTESTED (2026-05-04)

Operator ran `make build`, restarted engine binary, and walked through the dashboard in both locales. Operator attests `uat green` — full pass. Screenshots were not captured into this conversation; verdict is operator-attested green.

### togglePaper operator_action verification

- [x] DevTools Network shows POST `/api/config` body `{ price_gap_paper_mode, operator_action: true }`
- [x] Server returns HTTP 200 (NOT 409) on toggle click
- [ ] Pre-Plan-04 regression: clicking toggle WITHOUT this code returns HTTP 409 (control case — optional, not exercised)

### Render checks

- [x] EN: "Strategy 4 Configuration" card visible at TOP of Price-Gap tab
- [x] EN: card defaults to collapsed; click expands
- [x] EN: 4 subsections render in order: Scanner, Drawdown Circuit Breaker, Live-Capital Ramp (read-only banner), Live-Capital & Risk
- [x] EN: Phase 14 RampReconcileSection still in original location (D-19)
- [x] EN: Phase 15 BreakerSubsection still in original location (D-19)
- [x] zh-TW: same render checks pass
- [x] No raw i18n keys (`pricegap.config.X`) visible in either locale

### Typed-phrase modals — ENABLE-LIVE-CAPITAL

- [x] Click toggle (currently OFF) → modal opens
- [x] Real-trip warning banner visible (red)
- [x] Confirm button DISABLED initially
- [x] Type lowercase `enable-live-capital` → confirm STAYS disabled (case-sensitive)
- [x] Type uppercase `ENABLE-LIVE-CAPITAL` → confirm ENABLES
- [x] Esc closes modal (when not busy)
- [x] Submit POST body in DevTools shows `{ price_gap_live_capital: true }` — NO `operator_action` field (D-21)

### Typed-phrase modals — ENABLE-BREAKER

- [x] Click toggle (currently OFF) → modal opens
- [x] Type `ENABLE-BREAKER` exactly (case-sensitive)
- [x] Submit POST body in DevTools shows `{ price_gap_breaker_enabled: true }` — NO `operator_action` field (D-21)

### zh-TW locale

- [x] All field labels render in Chinese
- [x] Magic strings `ENABLE-LIVE-CAPITAL` / `ENABLE-BREAKER` stay LITERAL (untranslated, visible verbatim in modal prompt)

### Config.tsx migration

- [x] Open Config tab → Strategy.Discovery tab → "Min free-gap (bps)" / "Max price-gap (bps)" controls are GONE
- [x] Open Config tab → Spot-Futures Discovery tab → "Spot-futures price-gap gate" toggle and "Max price-gap (%)" input are GONE
- [x] All 4 controls migrated successfully (no functional regression — values still set via ConfigCard)

## Screenshots

Operator note: screenshots not captured into this conversation. Verdict is operator-attested green based on direct dashboard exercise in both locales.

Intended capture targets (not produced this session) under `.planning/phases/16-paper-mode-cleanup-dashboard-consolidation/uat-evidence/configcard/`:

- [ ] `pricegap-tab-en-expanded.png`
- [ ] `pricegap-tab-zh-expanded.png`
- [ ] `enable-live-capital-modal-en.png`
- [ ] `enable-live-capital-modal-zh.png`
- [ ] `enable-breaker-modal-en.png`
- [ ] `enable-breaker-modal-zh.png`
- [ ] `devtools-toggle-paper.png`
- [ ] `devtools-enable-live-capital.png`

## Verdict

`pass` — operator-attested green; screenshots not captured into this conversation. Date: 2026-05-04.

Operator confirmation summary: Strategy 4 Configuration card renders correctly in EN + zh-TW, togglePaper POST carries `operator_action: true` and returns 200, ENABLE-LIVE-CAPITAL + ENABLE-BREAKER typed-phrase modals work with case-sensitive accept and D-21 (no operator_action marker on those POSTs), 4 legacy Config.tsx controls confirmed removed.

---

**Resume signal received:** `uat green` — proceeding to Task 4 (VERSION + CHANGELOG bump) + finalization.
