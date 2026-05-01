# Phase 16: Paper-Mode Cleanup + Dashboard Consolidation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-01
**Phase:** 16-paper-mode-cleanup-dashboard-consolidation
**Areas discussed:** Gray-area selection, plan structure, DEV-01 (bingxprobe), PG-FIX-01 (realized slip), PG-FIX-02 (auto-POST flip), PG-OPS-09 (tab consolidation)

---

## Gray-Area Selection (multiSelect)

| Option | Description | Selected |
|--------|-------------|----------|
| DEV-01 bingxprobe (CRITICAL) | cmd/bingxprobe/ deleted in Phase 13 (commit 21cb60b). Must resolve before planning. | ✓ |
| PG-FIX-01 realized-slip formula | monitor.go:242-244 override is band-aid. Need formula fix. | ✓ |
| PG-FIX-02 auto-POST flip | Diagnose-first vs guard-first; chokepoint location. | ✓ |
| PG-OPS-09 tab consolidation | Migration strategy; component structure. | ✓ |

**User's choice:** All 4 areas selected.

---

## Plan Structure

| Option | Description | Selected |
|--------|-------------|----------|
| 4 plans, 1 per requirement (Recommended) | 16-01 PG-FIX-01, 16-02 PG-FIX-02, 16-03 DEV-01, 16-04 PG-OPS-09. Maps to ROADMAP "5 plans" hint. | ✓ |
| Bundle into 2 plans + 1 consolidation | Backend fixes / paper-mode hardening / tab. | |
| Strictly serial sequence | Slowest, safest. | |

**User's choice:** 4 plans, 1 per requirement.

---

## DEV-01: bingxprobe Resolution

| Option | Description | Selected |
|--------|-------------|----------|
| Restore from git + make target (Recommended) | git checkout 21cb60b^ -- cmd/bingxprobe/. Honors REQUIREMENTS verbatim. | ✓ |
| Recreate as minimal new utility | Fresh cmd/probe-bingx/. No stale code import. | |
| Close DEV-01 as obsolete + amend REQUIREMENTS | Lowest code surface. | |

**User's choice:** Restore from git + make target.

## DEV-01: Probe Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Preflight TestOrder + ticker (Recommended) | Hits BingX with same OrderPreflight.TestOrder() that priceGapBingXProbePrice uses. | ✓ |
| Full read-only sweep | GetTicker + GetBalance + LoadAllContracts + GetOrderBook. Broader. | |
| Match what original bingxprobe did | Restore exactly what 21cb60b deleted. | |

**User's choice:** Preflight TestOrder + ticker.

---

## PG-FIX-01: Realized-Slip Formula Fix

| Option | Description | Selected |
|--------|-------------|----------|
| Capture true exit mid, drop override (Recommended) | Stamp pos.LongMidAtExit / ShortMidAtExit at close. Synth fills at exit_mid ± modeled/2; realized = modeled drift + actual mid movement. | ✓ |
| Randomize synth slip per leg (±jitter) | Cheap; non-zero realized; doesn't reflect microstructure. | |
| Drift entry mid by synthetic price walk | Realistic Brownian walk; complex. | |
| Keep override, add comment + regression test | Lowest risk; doesn't satisfy spec. | |

**User's choice:** Capture true exit mid, drop override.

## PG-FIX-01: Regression Test Shape

| Option | Description | Selected |
|--------|-------------|----------|
| Non-zero AND ≠ ModeledSlipBps (Recommended) | Catches both machine-zero and override regression. | ✓ |
| Non-zero only | Simpler; would still pass with override. | |
| Statistical assertion across N trades | Strong signal but flakiness risk. | |

**User's choice:** Non-zero AND ≠ ModeledSlipBps.

---

## PG-FIX-02: Reproduction Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Audit-first via DevTools + code (Recommended) | HAR capture + grep + handler audit BEFORE code change. Document findings. | ✓ |
| Guard-first, then audit | Server-side reject FIRST, audit after. Faster. | |
| Skip audit, ship guard + regression test | Lowest investigation cost. | |

**User's choice:** Audit-first via DevTools + code.

## PG-FIX-02: Chokepoint Location

| Option | Description | Selected |
|--------|-------------|----------|
| Server-side handler reject (Recommended) | handlers.go config POST rejects price_gap_paper_mode writes without operator_action: true marker. Belt-and-suspenders with frontend audit. | ✓ |
| Lock to dedicated /api/pg/paper-mode endpoint | Move mutations to dedicated endpoint; generic config POST silently ignores. | |
| Frontend-only: remove offending POST + lint rule | No server change. Trusts frontend. | |

**User's choice:** Server-side handler reject.

---

## PG-OPS-09: Tab Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Extend PriceGap.tsx in place (Recommended) | Add Configuration card at top above Discovery/Candidates/Positions/History. | ✓ |
| New tab, deprecate PriceGap.tsx | PriceGapHub.tsx as config tab; PriceGap.tsx becomes Activity. | |
| Single tab with 4 inner sections | Inner sub-tabs (Config / Discovery / Activity / History). | |

**User's choice:** Extend PriceGap.tsx in place.

## PG-OPS-09: Migration of Legacy Controls

| Option | Description | Selected |
|--------|-------------|----------|
| Full cutover — delete from Config.tsx (Recommended) | Single source of truth. Requires UAT to confirm no missed references. | ✓ |
| Migrate + leave deprecation banner | One-release transition window. | |
| Move only obviously-Strategy-4 controls | Conservative; leaves sf_enable_price_gap_gate where it is. | |

**User's choice:** Full cutover — delete from Config.tsx.

## PG-OPS-09: New Config Sections (multiSelect)

| Option | Description | Selected |
|--------|-------------|----------|
| Scanner config | Discovery toggle, intervals, universe size, AutoPromoteScore, MaxCandidates. | ✓ |
| Breaker config | Enabled, DrawdownLimitUSDT, IntervalSec. ROADMAP success #4 requires. | ✓ |
| Ramp config | Stage sizes, hard ceiling, clean-day target. | ✓ |
| Live-capital toggle + risk thresholds | High-blast-radius toggle + slippage thresholds. | ✓ |

**User's choice:** All 4 sections selected.

---

## Final Confirmation

| Option | Description | Selected |
|--------|-------------|----------|
| All defaults + ready to write CONTEXT (Recommended) | Live-capital + breaker get typed-phrase confirm-modals (Phase 15 D-12 pattern). i18n lockstep. VERSION+CHANGELOG per plan. Ready for /gsd-plan-phase. | ✓ |
| Re-discuss something | | |

**User's choice:** Proceed to write CONTEXT.

## Claude's Discretion

Internal naming for new struct fields, exact validator clamp ranges, HAR storage path, version-bump cadence (per-plan vs bundled), React component decomposition for the Configuration card, BingX restored source cleanup needs, test file organization, HTTP status code for the PG-FIX-02 reject (409 vs 422), boot-time validation (none new needed), Telegram alert on PG-FIX-02 reject (skip).

## Deferred Ideas

Move Phase 14/15 widgets into Configuration card (kept where they are); inner sub-tabs (rejected); dedicated /api/pg/paper-mode endpoint (chose handler guard); frontend ESLint rule (defer); richer BingX probe (chose preflight + ticker); Telegram on PG-FIX-02 reject; i18n pre-commit hook; separate PriceGapHub.tsx; auto-tune parameters; cross-strategy paper toggle; sf_enable_price_gap_gate location revisit; other Phase 13 deleted utilities.
