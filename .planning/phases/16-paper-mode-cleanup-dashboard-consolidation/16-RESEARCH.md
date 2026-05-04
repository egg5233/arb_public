# Phase 16: Paper-Mode Cleanup + Dashboard Consolidation — Research

**Researched:** 2026-05-01
**Domain:** Strategy 4 (pricegap) paper-mode metrics, dashboard config write surface, Go CLI utility restoration, React tab consolidation + i18n lockstep
**Confidence:** HIGH (all anchors verified in current source tree)

## Summary

Phase 16 ships four narrowly-scoped Strategy 4 cleanups against an already-stable v2.2 substrate (Phase 11 chokepoint, Phase 12 auto-promote, Phase 14 ramp+reconcile, Phase 15 breaker — all complete). Every code anchor named in `16-CONTEXT.md` was verified in the current tree:

- `internal/pricegaptrader/monitor.go:242-244` paper-mode override exists verbatim (the band-aid being removed).
- `internal/api/handlers.go:1601-1668` is the single config-write surface for `price_gap_paper_mode` (both nested `price_gap.paper_mode` and flat shortcut land here; nested wins).
- `cmd/bingxprobe/main.go` is recoverable from `git show 21cb60b^` (53-line WS probe; needs scope retrofit per D-11).
- `web/src/pages/PriceGap.tsx` already has `togglePaper`, `postConfig`, and a `paperMode` useState with the `setPaperMode(d.paper_mode)` hydration line at L272 — that line is the most likely auto-flip suspect (state is set from `/api/pg/state` GET, then any subsequent `postConfig` round-trip could echo it).
- `web/src/components/Ramp/BreakerConfirmModal.tsx` is the Phase 15 typed-phrase precedent — directly extensible to `ENABLE-LIVE-CAPITAL` / `ENABLE-BREAKER`.
- All `PriceGap*` config fields named in CONTEXT D-17 already exist in `internal/config/config.go` with validators in place.

**Primary recommendation:** Treat this phase as a refactor + UI consolidation, not a feature build. Plans 16-01/02/03 are tightly bounded (single-file or near-single-file diffs); plan 16-04 (PG-OPS-09) is the one with real surface area — frontend Configuration card + i18n lockstep + Config.tsx legacy delete. Sequence 16-01 → 16-02 → 16-03 → 16-04 so the load-bearing backend fixes ship and bake before the frontend reorg.

## User Constraints (from CONTEXT.md)

### Locked Decisions
**PG-FIX-01: Realized-Slippage Formula Fix (Plan 16-01)**
- **D-01:** Capture true exit mid + drop the override. Add `pos.LongMidAtExit float64` and `pos.ShortMidAtExit float64` fields to `models.PriceGapPosition`. Stamp them at close time (`monitor.closePair`) from live BBO; fallback to last-known mid if BBO is stale. Synth fills continue to fill at `exit_mid ± modeled/2`, but the formula at `monitor.go:222-231` now uses `MidAtExit` (not `MidAtDecision`) as the exit reference — so realized = modeled drift + actual mid movement between decision and close, which is non-zero. Delete the paper-mode override at `monitor.go:242-244`.
- **D-02:** Regression test asserts non-zero AND ≠ ModeledSlipBps. New test in `internal/pricegaptrader/paper_mode_test.go` (or sibling): synth a paper trade with non-trivial mid drift between entry and exit; assert `|RealizedSlipBps| > 0` AND `|RealizedSlipBps - ModeledSlipBps| > epsilon`. Catches both the machine-zero bug and any future regression to the override pattern. Keep the existing `paper_to_live_test.go` `RealizedSlipBps == 0` guards passing.
- **D-03:** Reconciler anomaly detector is read-only consumer. `internal/pricegaptrader/reconciler.go:268` (anomaly threshold against `cfg.PriceGapAnomalySlippageBps`) automatically benefits from real values. No reconciler change required.
- **D-04:** Paper-mode invariant preserved. `placeLeg` synth path (`execution.go:237-251`) is unchanged. The fix is purely in the **measurement** path at close, not the **synthesis** path at entry. Paper mode still makes zero `PlaceOrder` calls.

**PG-FIX-02: Dashboard Auto-POST Flip (Plan 16-02)**
- **D-05:** Audit-first diagnosis. HAR capture + grep `useEffect→postConfig` + handler audit; document findings before implementing the guard. Establishing the negative is required.
- **D-06:** Server-side handler reject (chokepoint guard). Reject any write to `price_gap_paper_mode` (or nested `price_gap.paper_mode`) unless the request body carries an explicit `operator_action: true` marker. Toggle button (`PriceGap.tsx:togglePaper`) sends the marker. Returns HTTP 409 on rejected attempts.
- **D-07:** Phase 9 `pos.Mode` immutability untouched. Guard sits at the **config write** path (engine-wide flag), NOT the position-level Mode field.
- **D-08:** Sticky-flag interaction documented. Breaker `pg:breaker:state.paper_mode_sticky_until` forces paper mode regardless. New handler guard is independent.
- **D-09:** Regression test (browser-side). Smoke test (Playwright if available, else 16-02-HUMAN-UAT.md): fresh page load → zero POST `/api/config` carrying `price_gap_paper_mode`. HAR archived under `.planning/phases/16-.../uat-evidence/`.

**DEV-01: bingxprobe Restoration + Make Target (Plan 16-03)**
- **D-10:** Restore from git history (`git checkout 21cb60b^ -- cmd/bingxprobe/`).
- **D-11:** Probe scope = preflight `OrderPreflight.TestOrder` + ticker. Restored utility hits BingX with the same `OrderPreflight.TestOrder()` that `priceGapBingXProbePrice` uses (`execution.go:296`).
- **D-12:** `make probe-bingx` Makefile target; `go run ./cmd/bingxprobe/`; place under existing `.PHONY` block.
- **D-13:** CHANGELOG entry referencing original deletion commit `21cb60b`.
- **D-14:** Acceptance: human-verify successful probe (16-03-HUMAN-UAT.md).

**PG-OPS-09: Tab Consolidation (Plan 16-04)**
- **D-15:** Extend PriceGap.tsx in place. No new tab; no inner sub-tabs. Collapsible "Configuration" card at the TOP.
- **D-16:** Full cutover migration — move from `Config.tsx`: `strategy.discovery.price_gap_free_bps`, `strategy.discovery.max_price_gap_bps`, `spot_futures.enable_price_gap_gate`, `spot_futures.max_price_gap_pct`. Delete originals.
- **D-17:** New config sections: Scanner (5 fields), Breaker (3 fields, `ENABLE-BREAKER` typed-phrase guard), Ramp (read-mostly), Live-capital + risk (`ENABLE-LIVE-CAPITAL` typed-phrase guard).
- **D-18:** Typed-phrase pattern carried from Phase 15 D-12. Magic strings `ENABLE-LIVE-CAPITAL`, `ENABLE-BREAKER` are NOT translated.
- **D-19:** Phase 14/15 widgets stay where they are (read-mostly status, not editable config).
- **D-20:** i18n lockstep — every new label/placeholder/modal/validation/error in BOTH `web/src/i18n/en.ts` AND `web/src/i18n/zh-TW.ts`. `TranslationKey` type catches missing keys at build.
- **D-21:** POST envelope follows existing `postConfig` pattern; writes through `/api/config` (PG-FIX-02 D-06 guard); breaker/live-capital toggles include typed-phrase confirmation in POST body.

### Hard Contracts (Locked)
- Module isolation: code in `internal/pricegaptrader/`, `internal/api/handlers.go`, `cmd/bingxprobe/`, `web/src/pages/PriceGap.tsx` + `web/src/i18n/*.ts`. NO imports of `internal/engine` or `internal/spotengine`.
- Phase 9 `pos.Mode` immutability — unchanged.
- CandidateRegistry chokepoint (Phase 11) — unchanged.
- Default-OFF for any new config flag — none expected (all referenced fields pre-existing).
- `pg:*` Redis namespace — no new keys this phase.
- Commit + VERSION + CHANGELOG together.
- i18n lockstep.
- No `npm install` — only `npm ci`.
- DO NOT modify `config.json`.

### Claude's Discretion
- Internal naming for new struct fields (`LongMidAtExit` vs `LongExitMid`) — match nearest neighbor in `models.PriceGapPosition`.
- Exact validator clamp ranges for new UI number inputs — base on existing Phase 14/15 validators.
- HAR storage path under `.planning/phases/16-.../uat-evidence/`.
- Bundle 4 plans into single CHANGELOG entry vs one per plan — planner's call.
- React component decomposition for new Configuration card.
- Whether restored bingxprobe needs cleanup (read `git show 21cb60b^:cmd/bingxprobe/` first).
- Test file organization (extend `paper_mode_test.go` vs new `realized_slip_test.go`).
- HTTP 409 vs 422 on rejected paper_mode write — match existing handler conventions.
- Boot-time validation: do nothing new (existing `validatePriceGapLive` covers).
- Telegram alert on PG-FIX-02 reject — probably overkill; skip unless planner sees a clear signal.

### Deferred Ideas (OUT OF SCOPE)
- Cross-strategy / per-symbol breaker (deferred from Phase 15)
- Auto-recovery of breaker (sticky must be operator-cleared)
- Any change to Phase 9 `pos.Mode` immutability
- Any change to CandidateRegistry chokepoint
- Live-capital ramp behavior changes (Phase 14 owns; UI exposure only here)
- v1.0 tech-debt sweep (Phase 17)
- New scanner gates or score-formula changes
- Any non-Strategy-4 dashboard reshuffle
- Move Phase 14/15 widgets into Configuration card (revisit v2.3)
- Inner sub-tabs in PriceGap.tsx
- Dedicated `/api/pg/paper-mode` endpoint
- Frontend ESLint rule disallowing top-level postConfig calls (defer)
- Restore richer bingxprobe (full read-only sweep) — chose preflight + ticker
- Telegram alert on PG-FIX-02 reject
- i18n key lint pre-commit hook
- PriceGapHub.tsx as separate consolidated config page
- Auto-tune breaker/ramp from PnL history
- Cross-strategy paper-mode flip (v3.0)
- Migrating `sf_enable_price_gap_gate` BACK to Spot-Futures tab (revisit v2.3)
- Restore other deleted debug utilities (only `cmd/bingxprobe/` this phase)

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PG-FIX-01 | Fix `realized_slippage_bps` machine-zero in paper mode | `monitor.go:222-244` formula + override verified; `models.PriceGapPosition` has `LongMidAtDecision`/`ShortMidAtDecision` (need parallel `*MidAtExit` fields per D-01); `paper_to_live_test.go` + `paper_mode_test.go` exist as test homes |
| PG-FIX-02 | Diagnose + fix dashboard auto-POST that flips `paper_mode=false` on page load | `handlers.go:1602-1668` is single config-write surface (both nested + flat shortcut land here); `PriceGap.tsx:272 setPaperMode(d.paper_mode)` is hydration line — only `togglePaper` (L363-375) currently POSTs; `IsPaperModeActive` chokepoint at `tracker.go:283` (Phase 15 D-07) preserved |
| DEV-01 | `make probe-bingx` Makefile target | `cmd/bingxprobe/main.go` recoverable from `21cb60b^` (53 lines, gorilla/websocket + gzip decode); current `Makefile` has 5 `.PHONY` targets — clean insertion point; `OrderPreflight.TestOrder` interface defined at `pkg/exchange/types.go:79-83` |
| PG-OPS-09 | Consolidate ALL Strategy 4 config in PriceGap tab | All fields named in D-17 exist in `internal/config/config.go` with validators (`PriceGapDiscoveryEnabled`, `PriceGapAutoPromoteScore`, `PriceGapMaxCandidates`, `PriceGapLiveCapital`, `PriceGapAnomalySlippageBps`, `PriceGapBreakerEnabled`, `PriceGapDrawdownLimitUSDT`, `PriceGapBreakerIntervalSec`); typed-phrase precedent at `web/src/components/Ramp/BreakerConfirmModal.tsx`; legacy controls at `Config.tsx:746,753,1393,1454,1466` confirmed |

## Project Constraints (from CLAUDE.md / CLAUDE.local.md)

- **npm security lockdown:** never run `npm install`/`update`/`upgrade`/`npx`/`pnpm install`. Only `npm ci`. No new packages. [VERIFIED: CLAUDE.local.md]
- **Do not modify `config.json`:** runtime config with API keys; UI POSTs flow through `/api/config` handler with `.bak` backup. [VERIFIED: CLAUDE.local.md]
- **Build order:** frontend MUST build BEFORE Go binary (go:embed). `make build-frontend` → `make build`. Reverse order serves stale JS. [VERIFIED: CLAUDE.local.md, Makefile inspected]
- **Two-locale i18n:** `en` and `zh-TW`. `web/src/i18n/en.ts` + `web/src/i18n/zh-TW.ts` MUST stay in sync. `TranslationKey` derived from English file enforces compile-time. [VERIFIED: en.ts:973, both files 974 lines]
- **Module boundaries:** `pkg/exchange/` and `pkg/utils/` standalone — no `internal/` imports. `internal/pricegaptrader/` cannot import `internal/engine` or `internal/spotengine`. [VERIFIED: CLAUDE.local.md]
- **Versioning:** every commit updates BOTH `CHANGELOG.md` AND `VERSION`. [VERIFIED: CLAUDE.local.md, current `VERSION=0.38.0`]
- **Live trading risk:** changes must not break perp-perp, spot-futures, or pricegap-paper engines. [VERIFIED: CLAUDE.local.md]
- **Delegation mode:** coordinator delegates to teammates for any task touching 2+ files or non-trivial logic; uses Sonnet4.6/Opus4.6 only. [VERIFIED: CLAUDE.local.md]
- **graphify routing:** start with `graphify-publish/AI_ROUTER.md` (or `AI_REVIEW_ROUTER.md` for review) before grep. [VERIFIED: CLAUDE.local.md]

## Standard Stack

### Core (existing — no new dependencies allowed per npm lockdown / Go module isolation)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.26.0 | Backend | `go version go1.26.0 linux/amd64` [VERIFIED: shell] |
| gorilla/websocket | (per `go.mod`) | BingX probe WS dial | Already used in restored bingxprobe source [VERIFIED: `git show 21cb60b^:cmd/bingxprobe/main.go`] |
| React + Vite + Tailwind | (per `web/package.json`) | Dashboard | Existing stack — `npm ci` only [CITED: CLAUDE.local.md] |
| pricegaptrader narrow interfaces | n/a | Module isolation | `BreakerStateLoader`, `RampSnapshotter`, `ReconcileRecordLoader`, `ActivePositionLister` — D-15 module boundary preserved [VERIFIED: tracker.go:156] |

### Supporting (existing)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `web/src/components/Ramp/BreakerConfirmModal.tsx` | n/a | Phase 15 typed-phrase modal | Direct extension target for PG-OPS-09 `ENABLE-LIVE-CAPITAL` / `ENABLE-BREAKER` [VERIFIED: file exists, RECOVER + TEST-FIRE strings present] |
| `postConfig` helper | n/a | Generic config POST envelope | New PG-OPS-09 controls reuse; PG-FIX-02 guard rides on top [VERIFIED: PriceGap.tsx:337] |
| `OrderPreflight.TestOrder` | n/a | Production preflight path | DEV-01 probe must exercise this exact path [VERIFIED: pkg/exchange/types.go:82-83, execution.go:296] |
| `IsPaperModeActive` (tracker) | Phase 15 | Single paper-mode read chokepoint | PG-FIX-02 guard does NOT bypass this [VERIFIED: tracker.go:261-309] |
| Recharts | (per `package.json`) | Dashboard charts | PG-OPS-09 shouldn't need new charts (config inputs only) [CITED: CONTEXT.md] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Server-side `operator_action` marker on `/api/config` | Dedicated `/api/pg/paper-mode` endpoint | Bigger API surface change; CONTEXT D-deferred rejected this for v2.2 |
| Restore bingxprobe from git | Recreate from scratch | Loses original-utility-exercised path; CONTEXT D-10 explicitly chose restore |
| New top-level "Price-Gap Config" tab | Inline Configuration card in PriceGap.tsx (D-15) | Operator muscle memory; single React route preserved |
| Move Phase 14/15 widgets into Configuration card | Leave widgets where they are (D-19) | Avoids re-laying out shipped UI mid-phase |

**Installation:** None. Phase 16 adds zero dependencies. `make probe-bingx` runs against existing `gorilla/websocket` already in `go.mod`. [VERIFIED: existing import in restored bingxprobe source]

**Version verification:** Current `VERSION=0.38.0` [VERIFIED: file]. Phase 15 shipped 2026-05-01. Phase 16 plans bump per CONTEXT (1 per plan or bundled — planner's call).

## Architecture Patterns

### Recommended Project Structure (existing — confirmation only)

```
internal/pricegaptrader/
├── monitor.go               # PG-FIX-01: edit closePair (lines 147-244)
├── execution.go             # unchanged (synth path D-04)
├── reconciler.go            # unchanged (read-only consumer D-03)
├── tracker.go               # unchanged (Phase 15 IsPaperModeActive untouched)
├── paper_mode_test.go       # PG-FIX-01: extend (or sibling realized_slip_test.go)
└── paper_to_live_test.go    # existing assertions must continue to pass

internal/models/
└── pricegap_position.go     # PG-FIX-01: add LongMidAtExit + ShortMidAtExit fields

internal/api/
└── handlers.go              # PG-FIX-02: add operator_action guard at lines 1601-1668

cmd/bingxprobe/              # DEV-01: restore from 21cb60b^
└── main.go

web/src/pages/
├── PriceGap.tsx             # PG-OPS-09: extend with Configuration card at top
└── Config.tsx               # PG-OPS-09: delete migrated controls (lines 746,753,1393,1454,1466)

web/src/components/Ramp/     # Phase 15 typed-phrase precedent dir
├── BreakerConfirmModal.tsx  # extension model
└── BreakerSubsection.tsx    # subsection model

web/src/i18n/
├── en.ts                    # PG-OPS-09: add new keys
└── zh-TW.ts                 # PG-OPS-09: lockstep additions

Makefile                     # DEV-01: add probe-bingx target
```

### Pattern 1: Measurement vs Synthesis Separation (PG-FIX-01)

**What:** The bug at `monitor.go:242-244` is in the *measurement* layer (computing realized slip after close). The synthesis layer (placing synth fills at `mid ± modeled/2` in `execution.go`) is correct. Fix the measurement, leave the synthesis alone.

**When to use:** Any future Pitfall-7-class fix where derived metrics show machine-zero. Address the input data (capture exit mid), not the output column (override RealizedSlipBps).

**Example:**
```go
// Source: VERIFIED — internal/pricegaptrader/monitor.go:222-244 (current bug)
// CURRENT (bug): formula uses LongMidAtDecision as exit reference,
// so synth fills (which already reference exit_mid) cancel out.
if pos.LongMidAtDecision > 0 {
    entryLongBps = (pos.LongFillPrice - pos.LongMidAtDecision) / pos.LongMidAtDecision * 10_000.0
    exitLongBps  = (longExitPrice    - pos.LongMidAtDecision) / pos.LongMidAtDecision * 10_000.0
}
// ...
if pos.Mode == models.PriceGapModePaper {
    pos.RealizedSlipBps = pos.ModeledSlipBps  // <-- band-aid (D-01 deletes)
}

// AFTER (D-01): exit reference becomes LongMidAtExit / ShortMidAtExit,
// stamped at close from live BBO; override deleted.
if pos.LongMidAtExit > 0 {
    exitLongBps = (longExitPrice - pos.LongMidAtExit) / pos.LongMidAtExit * 10_000.0
}
```

### Pattern 2: Server-Side Chokepoint Guard (PG-FIX-02)

**What:** Reject any write to a load-bearing config field unless the request body carries an explicit operator-intent marker. Frontend audit fix is belt; server reject is suspenders.

**When to use:** Engine-wide config flags that flip system risk posture (paper→live). Same pattern as Phase 9's `pos.Mode` immutability, but at the config-write layer instead of the position layer.

**Example:**
```go
// Source: PROPOSED based on internal/api/handlers.go:1601-1668 (verified)
// Pseudo-pattern; planner finalizes signature
if upd.PriceGapPaperMode != nil || (upd.PriceGap != nil && upd.PriceGap.PaperMode != nil) {
    if !upd.OperatorAction {  // new field: `operator_action bool json:"operator_action"`
        s.cfg.Unlock()
        writeJSON(w, http.StatusConflict, Response{
            Error: "price_gap_paper_mode write requires operator_action=true (Phase 16 D-06)",
        })
        return
    }
}
```
[VERIFIED: `operator_action` does not currently exist anywhere in the codebase — clean introduction. Verified via grep.]

### Pattern 3: Typed-Phrase Confirm Modal (PG-OPS-09 from Phase 15)

**What:** Operator types a literal magic-string phrase (uppercase, hyphenated, never translated) before high-blast-radius mutation submits. Surrounding labels ARE translated.

**When to use:** Any toggle that flips system from safer→less-safe state. Phase 15 established for `RECOVER`/`TEST-FIRE`; Phase 16 extends to `ENABLE-LIVE-CAPITAL` + `ENABLE-BREAKER`.

**Example:**
```typescript
// Source: VERIFIED — web/src/components/Ramp/BreakerConfirmModal.tsx:1-25
// Phase 15 precedent — magic phrase RECOVER / TEST-FIRE LITERAL.
// PG-OPS-09 reuses this component shape, parameterized by the new phrase.
type Phrase = 'RECOVER' | 'TEST-FIRE' | 'ENABLE-LIVE-CAPITAL' | 'ENABLE-BREAKER';
```

### Pattern 4: Read-Mostly Status Widget vs Editable Configuration Card (PG-OPS-09)

**What:** Read-mostly status displays (Phase 14 ramp + Phase 15 breaker widgets) live near positions/history. Editable config inputs live in the new Configuration card at top. Cognitive split: "what is the system doing" vs "what knobs control it."

**When to use:** Whenever a tab grows multiple read+write surfaces. D-19 explicitly chose this split.

### Anti-Patterns to Avoid

- **Override the output column instead of fixing the input data.** The `monitor.go:242-244` band-aid is the example. Future Pitfall-7-class fixes must address why the formula collapses, not paint over the result. (Specific Idea 1 in CONTEXT)
- **Translate magic-string typed phrases.** `ENABLE-LIVE-CAPITAL` is a literal; localizing it would defeat the operator-typing safeguard.
- **Inner sub-tabs inside PriceGap.tsx.** Rejected for v2.2 per Deferred Ideas — extend in place is simpler.
- **Background `useEffect` that POSTs config.** PG-FIX-02 audit specifically hunts for these. The legitimate write surface is event-handler-only (`togglePaper`).
- **Touching `pos.Mode` per-position immutability.** Phase 9 chokepoint stamps once at entry. PG-FIX-02 guards the engine-wide flag, not per-position.
- **Cross-coupling Phase 14/15 widgets with Configuration card layout.** D-19 keeps them physically separated to avoid mid-phase re-layout regression.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| BingX WS probe | New WS client + gzip decoder | Restored `cmd/bingxprobe/main.go` from `21cb60b^` (53 lines, gorilla/websocket + gzip stdlib) | Original utility's exercised path is preserved; D-10 explicit |
| Order preflight call | Custom HTTP request to BingX | `OrderPreflight.TestOrder` (`pkg/exchange/types.go:82`) — same path `priceGapBingXProbePrice` uses | Production code path under test, not parallel implementation |
| Paper-mode read | Custom `cfg.PriceGapPaperMode` reads | `Tracker.IsPaperModeActive(ctx)` (`tracker.go:283`) — Phase 15 D-07 chokepoint, sticky-flag-aware | Single source of truth; PG-FIX-02 guard does not bypass |
| Config POST envelope | New endpoint for paper_mode | Existing `/api/config` POST (`handlers.go:1601-1668`) + `operator_action` marker | Minimal API surface change; CONTEXT deferred ideas explicitly rejected `/api/pg/paper-mode` |
| i18n key parity check | Pre-commit hook | `TranslationKey = keyof typeof en` (`en.ts:973`) — TS compile error if zh-TW.ts is missing a key referenced by code | Compile-time enforcement already exists; pre-commit hook deferred per CONTEXT |
| Typed-phrase confirm modal | New component | `web/src/components/Ramp/BreakerConfirmModal.tsx` — parameterize Phrase union | Phase 15 D-12 pattern; PG-OPS-09 D-18 extends |
| `.bak` rotation on config writes | Custom backup logic | Existing `SaveJSON` `.bak` rotation in config persistence | Phase 11/12 chokepoint already does this |
| Candidate CRUD writes | Direct `cfg.PriceGapCandidates` mutation | `s.registry.Replace()` (Phase 11 chokepoint at `handlers.go:1605-1613`) | PG-DISC-04 invariant; PG-OPS-09 must NOT regress |

**Key insight:** Phase 16 is almost entirely composition of existing primitives. The ONLY new abstraction is the `operator_action` marker on the config POST envelope. Everything else (typed-phrase modal, postConfig, OrderPreflight, IsPaperModeActive, Registry chokepoint) is reuse.

## Common Pitfalls

### Pitfall 1: PG-FIX-01 — Reintroducing the override after a failing test
**What goes wrong:** Engineer writes the new `MidAtExit` formula, runs tests, sees `RealizedSlipBps` still zero in some path, panics, re-adds the override "just for that path."
**Why it happens:** Synth fills at exit are at `MidAtExit ± modeled/2`. If `MidAtExit` is captured at the SAME timestamp as the entry decision (e.g., zero mid drift in a unit test), the formula still collapses to modeled-only contribution. The realized-non-zero property comes from REAL mid drift between decision and close.
**How to avoid:** D-02 regression test must inject non-trivial mid drift between entry and exit (e.g., advance the fake exchange's BBO before invoking close). Assert `|RealizedSlipBps - ModeledSlipBps| > epsilon`, not just `> 0`.
**Warning signs:** Test passes only because epsilon is generous; tests where mid is identical at entry+exit show machine-zero; engineer reaches for `pos.Mode == paper` branch in measurement code.

### Pitfall 2: PG-FIX-02 — Auditing only the legitimate write surface
**What goes wrong:** Engineer audits `togglePaper`, confirms it's wired correctly, declares the audit done. Misses a `useEffect` in `Config.tsx` that fires on mount and posts a generic config snapshot containing `paper_mode=false` from a stale React-state-default.
**Why it happens:** The bug was a one-time UAT observation; current grep shows no obvious offender. But CONTEXT D-05 explicitly requires "establishing the negative" — audit must be exhaustive even if reproduce fails.
**How to avoid:** Grep `useEffect` AND `useLayoutEffect` AND `componentDidMount`-equivalent across ALL `web/src/` paths (not just `PriceGap.tsx`). Inspect every `postConfig`/`fetch.*config.*POST` site. Capture HAR from a fresh-session page load (not a refresh — different code path). Server guard at handler is the safety net.
**Warning signs:** "I couldn't reproduce" without HAR; grep narrows to PriceGap.tsx only; audit skips Config.tsx and other tabs.

### Pitfall 3: PG-FIX-02 — Server guard breaks existing tests
**What goes wrong:** `internal/api/pricegap_handlers_test.go:387-426` already round-trips `price_gap_paper_mode=false` writes (verified by grep). Adding the `operator_action` requirement makes those tests fail.
**Why it happens:** Existing tests were written before the chokepoint existed.
**How to avoid:** Update existing tests to include `operator_action: true` in their POST bodies, then add NEW tests (rejected without marker → 409, accepted with marker → 200). Document in test file comment why the marker exists.
**Warning signs:** PR shows red CI on `TestConfig_PaperMode` family; engineer disables tests instead of updating them.

### Pitfall 4: DEV-01 — Restored source has compile-blocking drift
**What goes wrong:** `git checkout 21cb60b^ -- cmd/bingxprobe/` brings back source written 2026-04-24. If gorilla/websocket API or BingX WS endpoint shape changed since, `make probe-bingx` fails to compile or runs but prints garbage.
**Why it happens:** Six months of adapter evolution between deletion and restoration. CONTEXT discretion item explicitly acknowledges this risk.
**How to avoid:** Read `git show 21cb60b^:cmd/bingxprobe/main.go` BEFORE adding the Make target. Confirm imports compile against current `go.mod`. If trivial drift (e.g., method rename), patch in plan 16-03; if non-trivial (e.g., scope mismatch with D-11 OrderPreflight contract), report gap to user before proceeding.
**Warning signs:** Restored source uses removed types; `go vet` warnings; `make probe-bingx` returns non-zero exit on a healthy network.

### Pitfall 5: DEV-01 — Probe scope drift from D-11
**What goes wrong:** Restored source is a WS subscriber (verified — it dials swap-market WS and reads bookTicker). D-11 specifies probe scope as "preflight `OrderPreflight.TestOrder` + ticker." These are DIFFERENT operations.
**Why it happens:** The original 2026-04-24 utility diagnosed BBO decode bugs; current Phase 16 wants to validate the production preflight path. Restored source doesn't exercise `OrderPreflight.TestOrder`.
**How to avoid:** Read restored source first (Pitfall 4). If scope mismatches D-11, the planner must decide: (a) extend the restored utility to add a `TestOrder` call, or (b) replace WS-only scope with REST preflight + ticker fetch. CONTEXT D-11 says "If restored source has a different scope, trim/extend to match this contract." Plan 16-03 must include this assessment as task 1.
**Warning signs:** Plan jumps straight to "add Make target" without inspecting restored source contents.

### Pitfall 6: PG-OPS-09 — i18n key drift
**What goes wrong:** Engineer adds 30 new EN keys, ships, dashboard renders raw key strings (`pricegap.config.enable_live_capital`) in zh-TW because they forgot to mirror.
**Why it happens:** EN locale is the source of truth; zh-TW is the lockstep mirror. `TranslationKey` type catches missing keys when CODE references an unknown key, but it does NOT catch missing translations (i.e., zh-TW omitting a key that EN has). The lockstep is by convention + diff review, not type system.
**How to avoid:** Mechanical step in plan 16-04: add ALL keys to en.ts FIRST, then copy block-for-block to zh-TW.ts with translations. Diff-check key sets (`jq` or sed) before commit. Both files are 974 lines today (verified via `wc -l`) — equal line count is a weak smoke test.
**Warning signs:** PR shows en.ts +N lines, zh-TW.ts +M lines, N != M; reviewer skims zh-TW assuming "Chinese-translation-by-LLM is fine"; build succeeds because no CODE references the missing keys.

### Pitfall 7: PG-OPS-09 — Config.tsx legacy delete strands references
**What goes wrong:** D-16 deletes 4 controls from `Config.tsx`. If something else in the file (or another component) reads those state slots, the delete causes a runtime undefined-access or silent broken UI.
**Why it happens:** Migration assumes single producer/consumer.
**How to avoid:** Before deleting, grep `web/src/` for ALL references to `strategy.discovery.price_gap_free_bps`, `strategy.discovery.max_price_gap_bps`, `spot_futures.enable_price_gap_gate`, `spot_futures.max_price_gap_pct`. If references exist outside Config.tsx, rewire them to the new PriceGap.tsx Configuration card. UAT verifies no missed references (D-16 explicit).
**Warning signs:** Delete touches 4 lines, doesn't touch any other file; reviewer doesn't open zh-TW.ts; smoke test only renders PriceGap tab.

### Pitfall 8: PG-OPS-09 — Recharts/React npm install temptation
**What goes wrong:** Engineer wants a fancy new input widget, runs `npm install some-input-package`, breaks lockfile lockdown.
**Why it happens:** Forgetting the npm lockdown rule under deadline pressure.
**How to avoid:** All new inputs must compose from existing primitives in PriceGap.tsx + Config.tsx. No new packages. Plan 16-04 acceptance check: `web/package.json` and `web/package-lock.json` are unchanged in the diff.
**Warning signs:** PR shows `web/package-lock.json` modifications; new import path from `node_modules/`.

### Pitfall 9: General — Build order regression
**What goes wrong:** `go build` runs before `npm run build`; binary embeds stale JS via `go:embed`; UAT shows new Configuration card "missing" because the bundled assets don't include it.
**Why it happens:** CLAUDE.local.md flags this as a project pitfall; agents repeatedly forget under stress.
**How to avoid:** Use `make build` (which runs `make build-frontend` first). UAT manual step: `rm -rf arb web/dist && make build` to force clean rebuild before testing PG-OPS-09 frontend changes.
**Warning signs:** UAT report says "feature missing" but code merged; binary timestamp newer than `web/dist/` timestamp.

### Pitfall 10: PG-FIX-01 — `LongMidAtExit` zero in cold-start / BBO-stale path
**What goes wrong:** `MidAtExit` capture in `closePair` reads live BBO. If the WS feed is stale or the close path runs before BBO seed, `LongMidAtExit` is 0 → guard at line 223 (`if pos.LongMidAtDecision > 0`) → exit slip term collapses.
**Why it happens:** Live BBO is best-effort; close path can race ahead of WS warm-up.
**How to avoid:** D-01 explicit fallback: "fallback to last-known mid if BBO is stale." Implement as: capture `MidAtExit` from BBO; if BBO unavailable or returns 0, fall back to `MidAtDecision` and FLAG the position with an anomaly counter (existing reconciler hook at `reconciler.go:268` handles slippage anomalies — extend or piggyback). Document the fallback in the new struct field doc-comment.
**Warning signs:** New regression test passes only with mocked BBO; no test for stale-BBO path; production logs show `LongMidAtExit=0` in a non-trivial fraction of closes.

## Code Examples

### Capture exit mid in closePair (PG-FIX-01)

```go
// Source: PROPOSED based on VERIFIED monitor.go:147-244
// Add after `decS := t.sizeDecimals(...)` and BEFORE the placeCloseLegIOC goroutines:

// PG-FIX-01 D-01: capture exit mid from live BBO before placing close legs.
// Used as the exit reference in the realized-slip formula below (replaces
// LongMidAtDecision/ShortMidAtDecision as exit reference). Fallback to
// last-known mid if BBO is stale.
midL, midS, _, err := sampleLegs(t.exchanges, models.PriceGapCandidate{
    Symbol:    pos.Symbol,
    LongExch:  pos.LongExchange,
    ShortExch: pos.ShortExchange,
})
if err != nil || midL == 0 {
    midL = pos.LongMidAtDecision
}
if err != nil || midS == 0 {
    midS = pos.ShortMidAtDecision
}
pos.LongMidAtExit = midL
pos.ShortMidAtExit = midS
```

### New struct fields (PG-FIX-01)

```go
// Source: PROPOSED for internal/models/pricegap_position.go
// Match nearest-neighbor naming: existing fields are LongMidAtDecision /
// ShortMidAtDecision (verified at lines 72-73 of pricegap_position.go).
LongMidAtExit  float64 `json:"long_mid_at_exit,omitempty"`  // PG-FIX-01 D-01: BBO mid at close, fallback to *MidAtDecision
ShortMidAtExit float64 `json:"short_mid_at_exit,omitempty"` // PG-FIX-01 D-01: BBO mid at close, fallback to *MidAtDecision
```

### Server guard for paper_mode write (PG-FIX-02)

```go
// Source: PROPOSED based on VERIFIED handlers.go:1601-1668
// Add new field to upd struct (top of handler):
//   OperatorAction bool `json:"operator_action"`
//
// Add guard before applying paper_mode change:
paperModeRequested := upd.PriceGapPaperMode != nil ||
    (upd.PriceGap != nil && upd.PriceGap.PaperMode != nil)
if paperModeRequested && !upd.OperatorAction {
    s.cfg.Unlock()
    writeJSON(w, http.StatusConflict, Response{
        Error: "price_gap_paper_mode write requires operator_action=true (Phase 16 PG-FIX-02 D-06; " +
               "Phase 9 chokepoint: pos.Mode immutability is per-position, this guard is engine-wide)",
    })
    return
}
```

### Frontend togglePaper update (PG-FIX-02)

```typescript
// Source: PROPOSED based on VERIFIED PriceGap.tsx:363-375
const togglePaper = useCallback(async () => {
    const next = !paperMode;
    setPaperMode(next);
    try {
        await postConfig({ price_gap_paper_mode: next, operator_action: true });
    } catch (e) {
        setPaperMode(!next);
        // ...existing error handling
    }
}, [paperMode, postConfig, t]);
```

### Makefile target (DEV-01)

```makefile
# Source: PROPOSED for Makefile
# Insert in existing .PHONY block (line 4):
.PHONY: build-frontend build run dev clean probe-bingx

# Insert after `clean:` target:
probe-bingx:
	go run ./cmd/bingxprobe/
```

### Typed-phrase modal extension (PG-OPS-09)

```typescript
// Source: PROPOSED extending VERIFIED web/src/components/Ramp/BreakerConfirmModal.tsx:24
// Magic phrases stay LITERAL — never translated, never lowercased, never trimmed.
type Phrase = 'RECOVER' | 'TEST-FIRE' | 'ENABLE-LIVE-CAPITAL' | 'ENABLE-BREAKER';
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Paper-mode override at `monitor.go:242-244` | Capture `MidAtExit`, drop override | This phase (PG-FIX-01) | Realized slip becomes meaningful in paper mode |
| Engine-wide `paper_mode` writable from any POST | `operator_action` marker required | This phase (PG-FIX-02) | Page-load hydration cannot silently flip paper→live |
| `cmd/bingxprobe/` deleted (Phase 13, v0.34.6) | Restored + Make target | This phase (DEV-01) | Reproducible probe for ops |
| Strategy 4 config scattered across Config.tsx + PriceGap.tsx | Single Configuration card in PriceGap.tsx | This phase (PG-OPS-09) | Operator one-stop for Strategy 4 |
| Direct `cfg.PriceGapCandidates` mutation | `Registry.Replace` chokepoint | Phase 11 (PG-DISC-04) | Phase 16 must NOT regress |
| Per-position `pos.Mode` mutable | Stamped once at entry, immutable | Phase 9 | Phase 16 must NOT regress |
| Direct `cfg.PriceGapPaperMode` reads | `IsPaperModeActive(ctx)` chokepoint | Phase 15 (D-07) | Phase 16 PG-FIX-02 does NOT bypass |
| Phase 15 typed-phrase pattern (`RECOVER`, `TEST-FIRE`) | Extended to `ENABLE-LIVE-CAPITAL`, `ENABLE-BREAKER` | This phase (PG-OPS-09 D-18) | Wider safety net on UI mutations |

**Deprecated/outdated:**
- `monitor.go:233-244` comment block ("PG-VAL-03 v0.34.10 closure"): comment justifies the override; both comment and override removed in PG-FIX-01.
- Strategy 4 controls in Config.tsx (4 locations): deleted in PG-OPS-09 D-16.
- Phase 13 deletion of `cmd/bingxprobe/`: reversed in DEV-01.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Phase 14/15 widgets stay where they are (D-19 settled) and won't need to move during Phase 16 | Architecture Patterns / D-19 | Mid-phase re-layout if operator UAT objects |
| A2 | The PG-FIX-02 auto-flip bug may not reproduce in current code; audit ships HAR + grep negative as evidence anyway (CONTEXT D-05) | Pitfalls / D-05 | If bug is in fact reproducible, audit-first wastes time vs. patch-first |
| A3 | Restored `cmd/bingxprobe/main.go` from `21cb60b^` will compile against current `go.mod` (gorilla/websocket pinned, BingX endpoint stable) | DEV-01 / Pitfall 4 | Compile/runtime failure → planner adds adapter-fix scope to plan 16-03 |
| A4 | Restored bingxprobe scope (WS bookTicker subscriber) needs retrofit to D-11 (preflight TestOrder + ticker) — they are different operations | DEV-01 / Pitfall 5 | If unaddressed, `make probe-bingx` doesn't satisfy success criterion #3 |
| A5 | The four Config.tsx legacy controls (lines 746/753/1393/1454/1466) have no other consumers in `web/src/` | PG-OPS-09 / Pitfall 7 | Delete strands references; planner must grep before merging plan 16-04 |
| A6 | i18n line-count parity (en.ts + zh-TW.ts each 974 lines today) is preserved — N new EN keys = N new zh-TW keys | PG-OPS-09 / Pitfall 6 | Silent missing translation in zh-TW; reviewer must compare key sets, not just line counts |
| A7 | The `operator_action` POST field name is acceptable wire-protocol-wise (no current users; clean introduction) | Code Examples / D-06 | Bikeshed-only; planner can rename without spec change |
| A8 | `IsPaperModeActive` chokepoint (Phase 15 D-07) handles all engine reads; PG-FIX-02 guard is sufficient at the write side | Don't Hand-Roll / D-07 | If a code path reads `cfg.PriceGapPaperMode` directly, write guard alone leaves a window |

## Open Questions

1. **PG-FIX-02 reproduction status**
   - What we know: CONTEXT calls this a "one-time observation during v2.0 UAT"; current grep shows only `togglePaper` POSTs `price_gap_paper_mode`.
   - What's unclear: whether the bug is currently latent (a regressed-then-undone code path) or persistently lurking in a non-PriceGap component.
   - Recommendation: Plan 16-02 task 1 = audit-first per D-05; ship HAR + grep evidence regardless of reproduction.

2. **Restored bingxprobe scope retrofit size**
   - What we know: Original utility is 53 lines, WS bookTicker subscriber. D-11 requires preflight `TestOrder` + ticker.
   - What's unclear: whether retrofit is a 10-line tweak or effective rewrite.
   - Recommendation: Plan 16-03 task 1 = read `git show 21cb60b^:cmd/bingxprobe/main.go`, assess gap, report to user if non-trivial.

3. **Plan 16-04 commit cadence**
   - What we know: PG-OPS-09 is the largest plan. Frontend Configuration card + 4 Config.tsx deletions + ~30 i18n keys + typed-phrase modal extension.
   - What's unclear: whether to ship as one plan-16-04 commit or split into sub-commits (e.g., backend wiring → Configuration card → legacy delete → i18n).
   - Recommendation: planner splits if any single sub-step is independently verifiable; otherwise single commit. CONTEXT discretion item allows either.

4. **Telegram alert on PG-FIX-02 reject**
   - What we know: CONTEXT defers as "probably overkill; skip unless planner sees a clear signal."
   - What's unclear: whether a single rejected POST is a normal operator double-click vs an attack/regression signal.
   - Recommendation: skip for v2.2; revisit if rejects ever fire in production logs.

5. **Phase 14/15 widget relocation operator preference**
   - What we know: D-19 keeps widgets where they are; Deferred Ideas lists "revisit in v2.3."
   - What's unclear: if operator UAT for plan 16-04 surfaces navigation pain.
   - Recommendation: leave for v2.3 unless 16-04-HUMAN-UAT.md flags it.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | DEV-01, PG-FIX-01, PG-FIX-02 | ✓ | go1.26.0 | — |
| gorilla/websocket | DEV-01 | ✓ (in go.mod via existing usage) | per go.mod | — |
| Node.js >= 22 | PG-OPS-09 (frontend rebuild) | ✓ (per Makefile nvm path) | 22.13.0 | nvm fallback in Makefile |
| `npm ci` (lockfile-only install) | PG-OPS-09 | ✓ (mandated) | n/a | none — `npm install` FORBIDDEN |
| BingX WS reachability (`wss://open-api-swap.bingx.com`) | DEV-01 acceptance | runtime — operator verifies | n/a | UAT, not CI |
| Redis (DB 2) | All Phase 16 plans run in existing engine context | ✓ (project default) | n/a | — |
| Playwright | PG-FIX-02 D-09 (browser smoke test) | unknown — planner verifies | n/a | manual HAR capture per CONTEXT D-09 |
| `git` history at `21cb60b^` | DEV-01 | ✓ (commit confirmed via `git show`) | n/a | — |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:**
- Playwright: if not installed in `web/`, fall back to documented manual HAR capture per CONTEXT D-09 (acceptable per spec).

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework (Go) | stdlib `testing` + go1.26 (no fancy framework) |
| Framework (TS) | Vitest (per `web/package.json`) — verified by existence of `web/src/pages/PriceGap.candidates.test.ts` |
| Config file | none for Go (default `go test`); `vite.config.ts` for TS |
| Quick run command | `go test ./internal/pricegaptrader/... -run TestRealized -count=1` |
| Full suite command | `go test ./... -count=1` then `cd web && npm test` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PG-FIX-01 | Synth paper trade with mid drift produces non-zero RealizedSlipBps AND ≠ ModeledSlipBps | unit | `go test ./internal/pricegaptrader/ -run TestRealizedSlip_PaperNonZero -v` | ❌ Wave 0 (extend `paper_mode_test.go` or sibling) |
| PG-FIX-01 (regression) | Existing paper_to_live RealizedSlipBps assertions still pass | unit | `go test ./internal/pricegaptrader/ -run TestPaperToLive -v` | ✅ `paper_to_live_test.go` |
| PG-FIX-02 | POST `/api/config` with `price_gap_paper_mode` and no `operator_action` returns 409 | unit | `go test ./internal/api/ -run TestConfig_PaperMode_RequiresOperatorAction -v` | ❌ Wave 0 (extend `pricegap_handlers_test.go`) |
| PG-FIX-02 | Existing PaperMode round-trip tests still pass after marker added | unit | `go test ./internal/api/ -run TestConfig_PaperMode -v` | ✅ `pricegap_handlers_test.go:387` (must be updated) |
| PG-FIX-02 | Fresh page load → no auto-POST of `price_gap_paper_mode` | manual / Playwright | (manual HAR capture; Playwright optional) | ❌ manual UAT |
| DEV-01 | `make probe-bingx` runs, prints successful BingX response | manual | `make probe-bingx` | ❌ manual UAT (operator) |
| DEV-01 | `cmd/bingxprobe/main.go` compiles | unit | `go build ./cmd/bingxprobe/` | ✅ once restored |
| PG-OPS-09 | Configuration card renders + posts via existing envelope | manual / Playwright | manual UAT | ❌ manual UAT |
| PG-OPS-09 | i18n keys present in BOTH locales | unit | `node scripts/check-i18n-parity.js` (proposed) OR diff-check in code review | ❌ Wave 0 (script TBD; CI compile catches CODE-referenced keys via TranslationKey) |
| PG-OPS-09 | Typed-phrase modal accepts ENABLE-LIVE-CAPITAL / ENABLE-BREAKER | manual | manual UAT | ❌ manual UAT |
| PG-OPS-09 | Legacy Config.tsx controls deleted, no orphan references | unit | `! grep -r 'price_gap_free_bps\|max_price_gap_bps' web/src/ | grep -v PriceGap.tsx` | ❌ Wave 0 (planner adds grep guard) |

### Sampling Rate
- **Per task commit:** `go test ./internal/pricegaptrader/... -run TestRealizedSlip -v && go test ./internal/api/... -run TestConfig_PaperMode -v`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full Go suite green + manual UAT artifacts attached for PG-FIX-02 (HAR), DEV-01 (probe output), PG-OPS-09 (browser screenshot + zh-TW visual)

### Wave 0 Gaps
- [ ] `internal/pricegaptrader/paper_mode_test.go` (extend) OR `internal/pricegaptrader/realized_slip_test.go` (new) — covers PG-FIX-01 D-02
- [ ] `internal/api/pricegap_handlers_test.go` (extend) — adds operator_action accept + reject tests for PG-FIX-02
- [ ] `scripts/check-i18n-parity.js` (proposed; planner's call) — diff-check key sets between en.ts and zh-TW.ts
- [ ] Grep-guard test for Config.tsx legacy delete (PG-OPS-09 / Pitfall 7) — can be a CI shell check
- [ ] Manual UAT files: `16-02-HUMAN-UAT.md` (HAR), `16-03-HUMAN-UAT.md` (probe output), `16-04-HUMAN-UAT.md` (Configuration card screenshots, both locales)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (existing) | Bearer token in localStorage `arb_token` (CLAUDE.local.md) — unchanged |
| V3 Session Management | yes (existing) | Bearer token; no session creation in scope |
| V4 Access Control | yes | `operator_action: true` marker on `/api/config` paper_mode write — Phase 16 D-06 explicit |
| V5 Input Validation | yes | New PG-OPS-09 number inputs validated at handler (existing `validatePriceGapLive` covers); typed-phrase magic strings validated server-side too (per D-21) |
| V6 Cryptography | no | None hand-rolled this phase; existing BingX adapter HMAC unchanged; bingxprobe is read-only WS |

### Known Threat Patterns for Go + React + Bearer-token + Single-server stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Silent paper→live flip via background POST | Tampering | `operator_action: true` marker requirement (PG-FIX-02 D-06) — server rejects 409 without |
| Operator mis-click on `ENABLE-LIVE-CAPITAL` toggle | Tampering | Typed-phrase confirm modal (PG-OPS-09 D-18) — operator types literal magic string |
| Replay of legitimate paper-mode POST (CSRF-style) | Tampering | Bearer-token auth already in place; same-origin SPA — replay window is operator's session |
| Stale frontend state hydrating with `paper_mode=false` and overwriting live config | Tampering | Server guard (D-06) is the chokepoint; frontend `togglePaper`-only POST is belt |
| Restored bingxprobe leaks API keys | Information Disclosure | Restored utility is read-only WS subscriber + preflight TestOrder (NOT live order); credentials read from existing env path; no new key surface |
| `OrderPreflight.TestOrder` accidentally placing real order | Tampering | TestOrder is the BingX `/dry-run` equivalent — verify in adapter docs that it is non-marketable; planner confirms in plan 16-03 |
| i18n key omission rendering raw key strings as labels | Information Disclosure (low) | TranslationKey type catches CODE-referenced misses; lockstep diff catches missing-locale misses (Pitfall 6) |

## Sources

### Primary (HIGH confidence)
- `git show 21cb60b^:cmd/bingxprobe/main.go` — restored bingxprobe source (53 lines, gorilla/websocket WS subscriber)
- `internal/pricegaptrader/monitor.go:1-280` — Read tool, formula + override verified verbatim
- `internal/api/handlers.go:1590-1668` — Read tool, paper_mode write surface verified
- `pkg/exchange/types.go:79-83` — grep, OrderPreflight interface verified
- `internal/pricegaptrader/execution.go:296` — grep, `preflight.TestOrder` call site verified
- `internal/config/config.go:54-105, 392-437` — grep, all PG-OPS-09 fields exist with validators
- `web/src/pages/PriceGap.tsx:222, 272, 363-393` — grep, paperMode/togglePaper/postConfig/debug_log surfaces verified
- `web/src/pages/Config.tsx:746,753,1393,1454,1466` — grep, legacy Strategy 4 controls verified
- `web/src/components/Ramp/BreakerConfirmModal.tsx:1-25` — grep, Phase 15 typed-phrase precedent verified
- `web/src/i18n/en.ts:973` + line counts — grep + `wc -l`, TranslationKey type + lockstep parity verified
- `internal/pricegaptrader/tracker.go:156, 261-309` — grep, `IsPaperModeActive` chokepoint verified
- `Makefile` — Read tool, current 5-target structure verified
- `VERSION` (0.38.0), `CHANGELOG.md` (Phase 15 v0.38.0 ship 2026-05-01) — Read tool
- `go version go1.26.0` + `go.mod` — shell verification
- `.planning/REQUIREMENTS.md`, `.planning/STATE.md`, `.planning/ROADMAP.md`, `.planning/phases/16-.../16-CONTEXT.md` — Read tool

### Secondary (MEDIUM confidence)
- `.planning/research/PITFALLS.md` Pitfall 7 (regressions during tech-debt closure) — referenced in CONTEXT but not directly applicable; quoted only as analogy
- Phase 14/15 plan summaries (per STATE.md decisions log) — used to confirm narrow-interface + typed-phrase precedents

### Tertiary (LOW confidence)
- None. All claims verified against code or CONTEXT.md decisions.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — existing stack, zero new dependencies
- Architecture: HIGH — all anchors verified in current source tree
- Pitfalls: HIGH — derived from CONTEXT decisions + verified code anchors

**Research date:** 2026-05-01
**Valid until:** 2026-05-31 (30 days for stable code anchors); revisit if Phase 17 lands first or if any of `monitor.go`, `handlers.go`, `PriceGap.tsx`, `Config.tsx` are touched by an unrelated change.
