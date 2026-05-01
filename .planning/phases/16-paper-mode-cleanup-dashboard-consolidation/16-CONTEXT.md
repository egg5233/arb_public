# Phase 16: Paper-Mode Cleanup + Dashboard Consolidation - Context

**Gathered:** 2026-05-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Four tightly-scoped Strategy 4 (pricegap) cleanups that close v2.0 paper-mode debt and consolidate the dashboard surface ahead of v2.2 ship:

1. **PG-FIX-01** — Replace the paper-mode `RealizedSlipBps = ModeledSlipBps` band-aid override at `internal/pricegaptrader/monitor.go:242-244` with a real formula fix that produces non-zero realized slippage and exercises the full Phase 8 modeled-vs-realized pipeline.
2. **PG-FIX-02** — Diagnose + close the dashboard auto-POST that flipped `paper_mode=false` on page load (one-time v2.0 UAT observation), with both audit-driven root-cause and a server-side chokepoint guard. Phase 9 `pos.Mode` immutability invariant remains intact.
3. **DEV-01** — Restore the deleted `cmd/bingxprobe/` (removed in commit `21cb60b` Phase 13) from git history and add a `make probe-bingx` Makefile target. Probe scope: BingX preflight `OrderPreflight.TestOrder` + ticker fetch.
4. **PG-OPS-09** — Consolidate ALL Strategy 4 configuration into the existing top-level `PriceGap.tsx` tab (extended in place, not a new tab). Full cutover migration of legacy controls scattered across `Config.tsx` (Spot-Futures + Strategy.discovery sections). New config sections added for Scanner, Breaker, Ramp, Live-capital, and risk thresholds — currently config.json-only, now exposed in UI.

Plan structure: **4 plans, 1 per requirement** (16-01 PG-FIX-01, 16-02 PG-FIX-02, 16-03 DEV-01, 16-04 PG-OPS-09). ROADMAP "5 plans" hint = 4 + 1 verification/UAT. Each plan ships its own commit + VERSION + CHANGELOG bump.

**Out of scope this phase:**
- Cross-strategy breaker / per-symbol breaker (deferred from Phase 15)
- Auto-recovery of breaker (REQUIREMENTS Out-of-Scope; sticky must be operator-cleared)
- Any change to Phase 9 `pos.Mode` immutability semantics
- Any change to the CandidateRegistry chokepoint (Phase 11)
- Live-capital ramp behavior changes (Phase 14 owns; UI exposure only here)
- v1.0 tech-debt sweep (Phase 17)
- New scanner gates or score-formula changes
- Any non-Strategy-4 dashboard reshuffle

</domain>

<decisions>
## Implementation Decisions

### PG-FIX-01: Realized-Slippage Formula Fix (Plan 16-01)
- **D-01:** **Capture true exit mid + drop the override.** Add `pos.LongMidAtExit float64` and `pos.ShortMidAtExit float64` fields to `models.PriceGapPosition`. Stamp them at close time (`monitor.closePair`) from live BBO; fallback to last-known mid if BBO is stale. Synth fills continue to fill at `exit_mid ± modeled/2`, but the formula at `monitor.go:222-231` now uses `MidAtExit` (not `MidAtDecision`) as the exit reference — so realized = modeled drift + actual mid movement between decision and close, which is non-zero. Delete the paper-mode override at `monitor.go:242-244`.
- **D-02:** **Regression test asserts non-zero AND ≠ ModeledSlipBps.** New test in `internal/pricegaptrader/paper_mode_test.go` (or sibling): synth a paper trade with non-trivial mid drift between entry and exit; assert `|RealizedSlipBps| > 0` AND `|RealizedSlipBps - ModeledSlipBps| > epsilon`. Catches both the machine-zero bug and any future regression to the override pattern. Keep the existing `paper_to_live_test.go` `RealizedSlipBps == 0` guards passing.
- **D-03:** **Reconciler anomaly detector is read-only consumer.** `internal/pricegaptrader/reconciler.go:268` (anomaly threshold against `cfg.PriceGapAnomalySlippageBps`) automatically benefits from real values. No reconciler change required.
- **D-04:** **Paper-mode invariant preserved.** `placeLeg` synth path (`execution.go:237-251`) is unchanged. The fix is purely in the **measurement** path at close, not the **synthesis** path at entry. Paper mode still makes zero `PlaceOrder` calls.

### PG-FIX-02: Dashboard Auto-POST Flip (Plan 16-02)
- **D-05:** **Audit-first diagnosis.** Before any code change: (1) load dashboard with fresh session, capture HAR of all Network requests during page mount; (2) grep `web/src/` for ANY `useEffect` that calls `postConfig` outside an explicit event handler; (3) audit `internal/api/handlers.go` config POST handler chain for paper_mode key paths (`price_gap_paper_mode` flat AND nested `price_gap.paper_mode`). Document findings in `16-02-PLAN.md` "Investigation" section before implementing the guard. If the bug currently fails to reproduce, the audit must still complete to establish the negative.
- **D-06:** **Server-side handler reject (chokepoint guard).** In `handlers.go` config POST handler, reject any write to `price_gap_paper_mode` (or nested `price_gap.paper_mode`) unless the request body carries an explicit `operator_action: true` marker. The toggle button (`PriceGap.tsx:togglePaper`) is updated to send the marker; all other code paths (page-load hydration, generic config persists) cannot accidentally flip the flag. Returns HTTP 409 with explanatory error on rejected attempts. Belt-and-suspenders with the frontend audit fix.
- **D-07:** **Phase 9 `pos.Mode` immutability untouched.** The guard sits at the **config write** path (engine-wide `paper_mode` flag), NOT the position-level `Mode` field. ROADMAP success-criterion #2 explicitly requires this invariant remain intact. Add comment block in handler citing Phase 9 chokepoint.
- **D-08:** **Sticky-flag interaction documented.** When `pg:breaker:state.paper_mode_sticky_until` is non-zero, the breaker forces paper mode regardless. The new handler guard is independent — it prevents the engine-wide config flag from being silently overwritten via dashboard load, which is a separate failure mode from the breaker sticky path.
- **D-09:** **Regression test (browser-side).** Add a smoke test (Playwright if available, otherwise a documented manual check in 16-02-HUMAN-UAT.md): fresh page load → verify zero POST requests to `/api/config` carrying `price_gap_paper_mode`. Captured HAR archived under `.planning/phases/16-paper-mode-cleanup-dashboard-consolidation/uat-evidence/`.

### DEV-01: bingxprobe Restoration + Make Target (Plan 16-03)
- **D-10:** **Restore from git history.** `git checkout 21cb60b^ -- cmd/bingxprobe/` to restore the deleted utility. ROADMAP success-criterion #3 requires the path `make probe-bingx` runs end-to-end. The Phase 13 deletion ("debug utility purpose served") is reversed: ops still wants reproducible probe per DEV-01.
- **D-11:** **Probe scope = preflight TestOrder + ticker.** Restored utility hits BingX with the same `OrderPreflight.TestOrder()` that `priceGapBingXProbePrice` uses (`execution.go:296`), prints ticker + non-marketable test order roundtrip. Validates the production preflight path, which is the path that Phase 9 paper mode and Phase 11+12 scanner depend on. If restored source has a different scope, trim/extend to match this contract.
- **D-12:** **`make probe-bingx` Makefile target.** Add to `Makefile`:
  ```makefile
  .PHONY: probe-bingx
  probe-bingx:
      go run ./cmd/bingxprobe/
  ```
  Keep simple — no flags, no nvm gymnastics (it's pure Go). Place under existing `.PHONY` block alongside `build-frontend`, `build`, etc.
- **D-13:** **README/CHANGELOG note.** CHANGELOG entry under v0.X.Y "Restored cmd/bingxprobe/ utility (Phase 16, DEV-01)." Reference the original deletion commit so the audit trail is complete. No new docs file — operators discover via `make help` or this CHANGELOG entry.
- **D-14:** **Acceptance: human-verify successful probe.** Operator runs `make probe-bingx`, captures output, attaches to `16-03-HUMAN-UAT.md` showing successful BingX response. Required per ROADMAP success-criterion #3 ("prints a successful BingX probe response").

### PG-OPS-09: Tab Consolidation (Plan 16-04)
- **D-15:** **Extend PriceGap.tsx in place.** No new tab; no inner sub-tabs. Add a collapsible "Configuration" card at the TOP of the existing PriceGap.tsx layout (above Discovery / Candidates / Positions / History sections). Operator muscle memory preserved. Single React route.
- **D-16:** **Full cutover migration of legacy controls.** Move from `Config.tsx`:
  - `strategy.discovery.price_gap_free_bps` → Price-Gap Configuration card
  - `strategy.discovery.max_price_gap_bps` → Price-Gap Configuration card
  - `spot_futures.enable_price_gap_gate` → Price-Gap Configuration card (cross-strategy gate, but it's a Strategy-4 reference — REQUIREMENTS PG-OPS-09 wording supports moving it: "ALL Strategy 4 configuration in one place")
  - `spot_futures.max_price_gap_pct` → Price-Gap Configuration card
  Delete the originals from Config.tsx. Single source of truth. UAT verifies no missed references.
- **D-17:** **New config sections (UI for previously config.json-only fields).**
  - **Scanner section**: `PriceGapDiscoveryEnabled` (toggle), `PriceGapScanIntervalSec` (number input, validated [60, 3600]), `PriceGapMaxUniverse` (number, [1, 50]), `PriceGapAutoPromoteScore` (number, [0, 100]), `PriceGapMaxCandidates` (number, [1, 50]).
  - **Breaker section**: `PriceGapBreakerEnabled` (toggle, requires confirm-modal + typed phrase `ENABLE-BREAKER`), `PriceGapDrawdownLimitUSDT` (number, validated [-1e6, 0]), `PriceGapBreakerIntervalSec` (number, [60, 3600]). ROADMAP success-criterion #4 explicitly requires "breaker threshold input."
  - **Ramp section**: `PriceGapRampStageSizesUSDT` (3-element list inputs), hard ceiling, clean-day target. Read-mostly; edits go through normal POST.
  - **Live-capital + risk section**: `PriceGapLiveCapital` (toggle, requires confirm-modal + typed phrase `ENABLE-LIVE-CAPITAL` per Phase 15 D-12 pattern), `PriceGapAnomalySlippageBps`, exec-quality limits. The live-capital toggle is the highest blast-radius switch in the system — typed-phrase guard is mandatory.
- **D-18:** **Typed-phrase confirm-modal pattern (carry forward from Phase 15 D-12).** Any toggle that flips the system from a safer state to a less-safe state requires the operator to type a literal magic-string phrase before the mutation submits. Magic strings (uppercase, hyphenated): `ENABLE-LIVE-CAPITAL`, `ENABLE-BREAKER`. Phrases are NOT translated; surrounding modal labels ARE translated via i18n keys.
- **D-19:** **Subsume Phase 14 + Phase 15 widgets.** The existing Phase 14 ramp+reconcile widget and Phase 15 breaker widget (currently inline in PriceGap.tsx) move into the new Configuration card as their own subsections OR remain in their current location and the new Configuration card contains only NEW controls (D-17). **Decision: keep Phase 14/15 widgets where they are** (they are read-mostly status displays, not config) — the new Configuration card is for editable config only. Avoids re-laying out shipped widgets mid-phase.
- **D-20:** **i18n lockstep.** Every new label, placeholder, modal text, validation message, and error string MUST be added to BOTH `web/src/i18n/en.ts` AND `web/src/i18n/zh-TW.ts`. CI / `TranslationKey` type catches missing keys at build time. Magic-string typed phrases (`ENABLE-LIVE-CAPITAL`, `ENABLE-BREAKER`) are NOT translated.
- **D-21:** **POST envelope.** Each new config field follows existing `postConfig` envelope pattern (`PriceGap.tsx:340`); writes go through `/api/config` (with PG-FIX-02 D-06 guard for paper_mode); breaker/live-capital toggles include the typed-phrase confirmation in the POST body so the server can re-verify if needed. CandidateRegistry chokepoint (Phase 11) handles candidate CRUD writes — unchanged.

### Hard Contracts (Carried Forward — Locked by ROADMAP/REQUIREMENTS/Prior Phases)
These are NOT gray areas; pinned by spec or prior decisions. Listed for downstream agents:

- **Module isolation:** all code in `internal/pricegaptrader/` (PG-FIX-01), `internal/api/handlers.go` (PG-FIX-02 server guard), `cmd/bingxprobe/` (DEV-01 restored), `web/src/pages/PriceGap.tsx` + `web/src/i18n/*.ts` (PG-OPS-09). NO imports of `internal/engine` or `internal/spotengine`.
- **Phase 9 `pos.Mode` immutability** — unchanged; PG-FIX-02 guards the engine-wide config flag, not position-level Mode.
- **CandidateRegistry chokepoint (Phase 11)** — unchanged; candidate CRUD continues to flow through the chokepoint.
- **Default-OFF for any NEW config flag** — none expected this phase (all referenced fields are pre-existing).
- **`pg:*` Redis namespace** — no new keys this phase.
- **Commit + VERSION + CHANGELOG together** (project rule from CLAUDE.local.md). Each of the 4 plans bumps its own minor version (e.g., v0.38.0 → v0.38.1 → v0.38.2 → v0.39.0), or one bundled bump on the final plan — planner's call.
- **i18n lockstep** — every new UI string in both `en.ts` and `zh-TW.ts`.
- **No `npm install`** — frontend dependency surface is locked; only `npm ci`. No new packages allowed.
- **DO NOT modify `config.json`** — runtime config; UI POSTs flow through existing `/api/config` handler that handles persistence.
- **Phase 15 typed-phrase pattern (D-12)** — extended to PG-OPS-09 high-blast-radius toggles (`ENABLE-LIVE-CAPITAL`, `ENABLE-BREAKER`).

### Claude's Discretion
- Internal naming for new struct fields (`LongMidAtExit` vs `LongExitMid` etc.) — match nearest neighbor in `models.PriceGapPosition`.
- Exact field validators (clamp ranges) for new UI number inputs in PG-OPS-09 — base on existing Phase 14 / Phase 15 validators.
- HAR capture format + storage path under `.planning/phases/16-.../uat-evidence/` — operator-friendly layout.
- Whether to bundle 4 plans into a single CHANGELOG section (e.g., "v0.39.0: Phase 16 paper-mode cleanup") or one section per plan — planner's call based on size of each diff.
- React component decomposition for the new Configuration card (one component vs subsections) — consistent with existing PriceGap.tsx style.
- Whether the BingX probe utility's restored source needs cleanup (e.g., updated Go module references, current adapter API) — planner reads `git show 21cb60b^:cmd/bingxprobe/` first to assess.
- Test file organization: extend existing `paper_mode_test.go` for PG-FIX-01 vs new `realized_slip_test.go` — co-locate with closest existing tests.
- Whether the PG-FIX-02 server guard returns HTTP 409 (Conflict) or 422 (Unprocessable Entity) on rejected paper_mode write — match existing handler conventions.
- Boot-time validation: do nothing new (existing `validatePriceGapLive` already covers the new fields at config-load).
- Telegram alert on PG-FIX-02 reject (i.e., if some background agent tries to flip paper_mode and gets rejected, alert ops?) — probably overkill; skip unless planner sees a clear signal.

### Folded Todos
None — `todo match-phase 16` returned 0 matches at session start.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Roadmap & Requirements
- `.planning/ROADMAP.md` §"Phase 16: Paper-Mode Cleanup + Dashboard Consolidation" — phase goal + 5 success criteria
- `.planning/REQUIREMENTS.md` §"Paper-Mode Bug Closure" §**PG-FIX-01** — realized-slip machine-zero contract verbatim
- `.planning/REQUIREMENTS.md` §"Paper-Mode Bug Closure" §**PG-FIX-02** — dashboard auto-POST flip + Phase 9 immutability invariant
- `.planning/REQUIREMENTS.md` §"Developer Experience" §**DEV-01** — make probe-bingx target requirement
- `.planning/REQUIREMENTS.md` §"Strategy 4 Live Capital" §**PG-OPS-09** — top-level Price-Gap dashboard tab consolidation contract

### Prior Phase Contexts (load all before planning — heavy reuse)
- `.planning/phases/15-drawdown-circuit-breaker/15-CONTEXT.md` — typed-phrase confirm-modal pattern (D-12), sticky-flag semantics (D-07), breaker config fields, dashboard widget extension precedent
- `.planning/phases/14-daily-reconcile-live-ramp-controller/14-CONTEXT.md` — ramp config fields, Redis HASH 5-field precedent, dashboard widget pattern, paper-mode toggle path
- `.planning/phases/12-auto-promotion/12-CONTEXT.md` — auto-promote score field, scanner config, candidate registry write pattern
- `.planning/phases/11-auto-discovery-scanner-chokepoint-telemetry/11-CONTEXT.md` — CandidateRegistry chokepoint, scanner config fields, telemetry surface

### v2.0 Retrospective (PG-FIX-01 root cause)
- `.planning/milestones/v2.0-MILESTONE-AUDIT.md` (if present) — Pitfall 7 documentation: realized-slippage machine-zero in paper mode
- `.planning/research/PITFALLS.md` — Pitfall 7 source

### Existing Code Anchors (read before modifying)

**PG-FIX-01 sites:**
- `internal/pricegaptrader/monitor.go:222-244` — current realized-slip formula + paper-mode override (the band-aid being removed)
- `internal/pricegaptrader/monitor.go:closePair` — site to add `MidAtExit` stamping at close
- `internal/pricegaptrader/execution.go:218-251` — `placeLeg` synth path (unchanged; fix is in measurement, not synthesis)
- `internal/models/pricegap_position.go` — add `LongMidAtExit`, `ShortMidAtExit` fields here
- `internal/pricegaptrader/paper_mode_test.go` — existing paper-mode tests; new regression test co-locates here
- `internal/pricegaptrader/paper_to_live_test.go:168-376` — existing assertions on `RealizedSlipBps != 0` (must continue to pass)
- `internal/pricegaptrader/reconciler.go:253, 268` — anomaly threshold consumer (read-only; benefits automatically)

**PG-FIX-02 sites:**
- `internal/api/handlers.go` — config POST handler chain; add server-side reject for unauthorized `price_gap_paper_mode` writes
- `web/src/pages/PriceGap.tsx:222, 272, 365-375` — `paperMode` state + `togglePaper` (the legitimate write surface)
- `web/src/pages/PriceGap.tsx:377-393` — `toggleDebugLog` precedent for nested vs flat envelope shape
- `web/src/pages/PriceGap.tsx` (full file) — audit for any `useEffect` that POSTs config
- `internal/pricegaptrader/tracker.go` — `IsPaperModeActive` (Phase 15 D-07; sticky-flag-aware paper-mode read site, untouched by PG-FIX-02)

**DEV-01 sites:**
- `Makefile` — add `probe-bingx` target alongside existing build-frontend / build / dev / clean
- Git: `git show 21cb60b -- cmd/bingxprobe/` — recover deleted source for restoration
- `pkg/exchange/bingx/` (or similar adapter location) — `OrderPreflight.TestOrder` implementation that the probe must exercise

**PG-OPS-09 sites:**
- `web/src/pages/PriceGap.tsx` — extend in place; add Configuration card at top
- `web/src/pages/Config.tsx:746-755, 1393, 1454-1466` — legacy Strategy 4 controls being migrated/deleted
- `web/src/i18n/en.ts` + `web/src/i18n/zh-TW.ts` — add ALL new label keys in lockstep; type derived from `en.ts` enforces lockstep at compile
- `internal/config/config.go` — `PriceGap*` block (used by validators); no schema changes this phase, but planner verifies all fields exposed in UI exist
- `internal/api/handlers.go` config POST — same handler used by PG-FIX-02; ensure no double-handling of paper_mode key

### v2.2 Research
- `.planning/research/ARCHITECTURE-v2.2.md` — Strategy 4 architecture overview
- `.planning/research/PITFALLS.md` — Pitfall 7 (realized-slip machine-zero), Pitfall 2 (CandidateRegistry chokepoint), context for not regressing prior work

### Cross-Project History
- Commit `21cb60b chore(13): remove cmd/bingxprobe — debug utility purpose served` — DEV-01 reversal target; restore from this commit's parent
- v0.37.0 (Phase 14 ship), v0.38.x (Phase 15 ship) — most recent precedent for adding controllers/widgets without disrupting live trading

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`PriceGap.tsx`** — already exists as a top-level tab; layout pattern (`postConfig`, `togglePaper`, `toggleDebugLog`) is the model for new toggle controls in PG-OPS-09.
- **`postConfig` helper** (`PriceGap.tsx:340`) — generic config POST envelope; new controls reuse it.
- **`/api/config` handler** (`internal/api/handlers.go`) — existing config write surface; both PG-FIX-02 guard and PG-OPS-09 new fields land here.
- **`OrderPreflight.TestOrder`** (BingX adapter) — production preflight path; DEV-01 probe re-uses for end-to-end exercise.
- **Phase 15 `IsPaperModeActive` helper** (`tracker.go`) — single source of truth for paper-mode reads; PG-FIX-02 guard does NOT bypass this.
- **Phase 15 typed-phrase confirm-modal pattern** (D-12) — directly extended to PG-OPS-09's `ENABLE-LIVE-CAPITAL` and `ENABLE-BREAKER` toggles.
- **Phase 14 widget + Phase 15 widget layouts** (PriceGap.tsx subsections) — pattern reference for read-mostly status displays; new Configuration card is for editable controls only (D-19).
- **`paper_to_live_test.go`** — existing test harness for paper-mode lifecycle; PG-FIX-01 regression test follows same setup style.
- **`reconciler.go` anomaly detector** — auto-benefits from PG-FIX-01 (no change required).
- **i18n `TranslationKey` type** — compile-time enforcement of EN/zh-TW lockstep.

### Established Patterns
- **Default-OFF master toggle.** Every existing `PriceGap*` flag follows this; PG-OPS-09 surfaces them in UI but defaults remain config-driven.
- **Strict `pg:*` Redis namespace.** No new keys this phase.
- **i18n lockstep.** Every UI string in both locale files; CI catches.
- **Commit + VERSION + CHANGELOG together** (project rule).
- **Module isolation.** No `internal/engine` or `internal/spotengine` imports from `internal/pricegaptrader/`.
- **Best-effort Redis writes with logged failure.** Carried over from prior phases; not directly invoked this phase.
- **Typed-phrase magic-string confirm-modals** (Phase 15 D-12). Extended to PG-OPS-09.
- **Server-side handler envelope** — `{ ok: boolean, data?: T, error?: string }` per CLAUDE.local.md "Dashboard API envelope".
- **`go:embed` build order** — `npm run build` BEFORE `go build`. PG-OPS-09 frontend changes require full rebuild before testing.

### Integration Points
- **`monitor.closePair`** — site of new `MidAtExit` stamping (PG-FIX-01).
- **`/api/config` POST handler** — site of new `paper_mode` write guard (PG-FIX-02).
- **`Makefile`** — add `probe-bingx` target (DEV-01).
- **`PriceGap.tsx`** — extend with Configuration card at top (PG-OPS-09).
- **`Config.tsx`** — delete migrated Strategy 4 controls (PG-OPS-09).
- **`web/src/i18n/en.ts` + `web/src/i18n/zh-TW.ts`** — add all new keys in lockstep (PG-OPS-09).
- **CHANGELOG.md + VERSION** — bump per plan, per project rule.

</code_context>

<specifics>
## Specific Ideas

- **"The override at monitor.go:242-244 is the bug, not the fix."** PG-VAL-03 closed the symptom (column never showed zero) but masked the root cause: synth fills cancel against entry mid because exit mid is never captured. PG-FIX-01 captures the exit mid and the formula naturally produces non-zero realized slip — no override needed. This is the pattern for future Pitfall-7-class fixes: address the input data, not the output column.
- **"Audit-first means we publish the HAR even if the bug doesn't reproduce."** PG-FIX-02 starts with documentation-driven investigation. If the auto-POST is no longer firing in current code, the HAR + grep audit + handler audit still ship as evidence in 16-02-PLAN.md so we're not relying on memory. Establishing the negative is required; "we couldn't reproduce" without evidence is not acceptable.
- **"Restore from git, don't recreate."** DEV-01: the deleted `cmd/bingxprobe/` source is recoverable from `21cb60b^`. Restoration preserves the original utility's exercised path and avoids drift. If the restored source has compile-blocking divergence from current adapter API, the planner reports the gap before patching — small adapter fix is acceptable; full rewrite requires re-discussion.
- **"PG-OPS-09 is config-only consolidation; status widgets stay put."** Phase 14 ramp widget and Phase 15 breaker widget are read-mostly status displays — they belong with positions/history, not with editable config. The new Configuration card is for inputs only. This avoids relaying out shipped UI mid-phase and keeps the cognitive split clean: "what is the system doing right now" vs "what knobs control it."
- **"Typed-phrase guards on every load-bearing toggle."** `ENABLE-LIVE-CAPITAL` and `ENABLE-BREAKER` flips system risk posture from safer to less-safe. Phase 15 D-12 established the pattern (RECOVER, TEST-FIRE); PG-OPS-09 extends it. Operator typing the literal phrase prevents shell-history one-shots, mis-clicks on touchscreen, and trivial replay.
- **"Single source of truth for paper_mode."** PG-FIX-02 server guard sits at `/api/config` POST handler; Phase 15 D-07 routes all reads through `IsPaperModeActive`. Together: the ONLY way to flip paper_mode is operator-driven UI toggle (POST with `operator_action: true`) or pg-admin CLI. Page loads, background hydration, and any other writer get rejected. Phase 9 `pos.Mode` immutability remains the per-position invariant.
- **"4 plans, 4 commits, 4 CHANGELOG entries."** Each plan is independent enough to ship and verify in isolation. If PG-OPS-09 takes longer than expected, PG-FIX-01/02 + DEV-01 can ship first. Planner decides version-bump cadence (one bump per plan vs bundled).

</specifics>

<deferred>
## Deferred Ideas

- **Move Phase 14 ramp widget + Phase 15 breaker widget into Configuration card** — D-19 chose to leave them in current locations. Revisit in v2.3 if operators report navigation pain.
- **Inner sub-tabs in PriceGap.tsx (Config / Discovery / Activity / History)** — rejected for v2.2 (extend in place is simpler). Revisit if the page grows past one viewport.
- **Dedicated `/api/pg/paper-mode` endpoint** — PG-FIX-02 alternative; rejected in favor of guard on existing `/api/config` to minimize API surface change. Revisit if other config keys need similar protection.
- **Frontend ESLint rule disallowing top-level postConfig calls** — could harden PG-FIX-02 further; deferred to v2.3 frontend tooling sweep.
- **Restore `cmd/bingxprobe/` as a richer probe (full read-only sweep)** — chose preflight + ticker scope for D-11. Operators can extend later if needed.
- **Telegram alert on PG-FIX-02 reject** — overkill for v2.2; deferred unless reject signal becomes meaningful.
- **i18n key lint pre-commit hook** — CI already enforces via TranslationKey type; pre-commit hook is convenience, not correctness. Defer.
- **PriceGapHub.tsx as a separate consolidated config page** — rejected; extending PriceGap.tsx in place is the right call.
- **Auto-tune of breaker / ramp parameters from PnL history** — out of scope per v2.2 REQUIREMENTS ("manual calibration with operator review").
- **Cross-strategy paper-mode flip (perp-perp + spot-futures + pricegap shared toggle)** — v3.0 unification milestone.
- **Migrating sf_enable_price_gap_gate FROM PriceGap tab BACK to Spot-Futures tab** — D-16 chose to move it into PriceGap. If operators report confusion (it's a Spot-Futures-engine setting that references Strategy 4 data), revisit in v2.3.
- **Restore other deleted debug utilities** — DEV-01 only restores `cmd/bingxprobe/`. Other Phase 13 deletions stay deleted.

### Reviewed Todos (not folded)
None — `todo match-phase 16` returned 0 matches at session start.

</deferred>

---

*Phase: 16-paper-mode-cleanup-dashboard-consolidation*
*Context gathered: 2026-05-01*
