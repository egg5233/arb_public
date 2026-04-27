# Roadmap: Arb Bot — Unified Arbitrage System

## Milestones

- ✅ **v1.0 Unified Arb Platform** — Phases 1–7 (shipped 2026-04-21). See `milestones/v1.0-ROADMAP.md`
- ✅ **v2.0 Multi-Strategy Expansion** — Phases 8–9 (shipped 2026-04-25). See `milestones/v2.0-ROADMAP.md`

## Phases

<details>
<summary>✅ v1.0 Unified Arb Platform (Phases 1–7) — SHIPPED 2026-04-21</summary>

- [x] Phase 1: Spot-Futures Exchange Expansion (3/3 plans) — SF-01, SF-02, SF-03
- [x] Phase 2: Spot-Futures Automation (3/3 plans) — SF-04, SF-05, SF-06, SF-07
- [x] Phase 3: Operational Safety (3/3 plans) — PP-01, PP-03 (PP-02 dropped per D-05)
- [x] Phase 4: Performance Analytics (4/4 plans) — PP-04, AN-01..06
- [x] Phase 5: Capital Allocation (3/3 plans) — CA-01..04
- [x] Phase 6: Spot-Futures Risk Hardening (4/4 plans) — SF-RISK-02, SF-RISK-03
- [x] Phase 7: Milestone Polish (1/1 plan) — SF-RISK-01 (verif missing; code live v0.29.0)

Audit: `tech_debt` status — all code shipped and live; verification/Nyquist docs incomplete. Debt carried forward as v2.0 backlog.

Full detail: `.planning/milestones/v1.0-ROADMAP.md`
Requirements: `.planning/milestones/v1.0-REQUIREMENTS.md`
Audit: `.planning/milestones/v1.0-MILESTONE-AUDIT.md`

</details>

<details>
<summary>✅ v2.0 Multi-Strategy Expansion (Phases 8–9) — SHIPPED 2026-04-25</summary>

- [x] Phase 8: Price-Gap Tracker Core (8/8 plans) — PG-01..05, PG-RISK-01..05, PG-OPS-06
- [x] Phase 9: Price-Gap Dashboard & Paper→Live Operations (11/11 plans, 8 core + 2 gap-closure + 1 UAT rewalk) — PG-OPS-01..05, PG-VAL-01..02

Live-fire UAT signed off 2026-04-25 against pg_SOONUSDT_gateio_bingx position with paper-mode chokepoint and Gate 0 duplicate-candidate gate validated end-to-end. Three known issues deferred to v2.1 backlog:
1. `realized_slippage_bps` machine-zero in paper mode (calc bug — formula computes delta vs modeled)
2. Dashboard auto-POST on page load flipped `paper_mode=false` once (source unknown; needs DevTools Network capture)
3. `cmd/bingxprobe/` debug utility (committed; consider promoting to `make probe-bingx` target)

Full detail: `.planning/milestones/v2.0-ROADMAP.md`
Requirements: `.planning/milestones/v2.0-REQUIREMENTS.md`

</details>

### 🚧 v2.1 Candidate Operations (Planning)

**Goal:** Operationalize Strategy 4's candidate lifecycle — Dashboard CRUD for manual control + algorithmic auto-discovery + score-threshold auto-promotion. **Paper-only for the entire milestone**; no live capital flip until v2.2.

**Constraints (locked):** paper-only, 6 existing exchanges, score-threshold auto-promotion (no review gate).

- [x] **Phase 10: Dashboard Candidate CRUD** — Add/Edit/Delete modal in Price-Gap tab; round-trip via `/api/config` + i18n EN/zh-TW lockstep (PG-OPS-07) (completed 2026-04-25)
- [ ] **Phase 11: Auto-discovery Scanner** — Periodic scan across 6 exchanges; score by spread persistence + book depth + retreat depth; write to `pg:discovered:*` Redis keys (PG-DISC-01, PG-DISC-03)
- [ ] **Phase 12: Auto-promotion** — Discovered candidates above `PriceGapAutoPromoteScore` threshold append to `cfg.PriceGapCandidates`; respect `PriceGapMaxCandidates` cap; Telegram + WS broadcast on promotion (PG-DISC-02)
- [x] **Phase 13: v2.0 Deferred Closure** — closed 2026-04-25 in v0.34.10/v0.34.11. PG-OPS-08 = `go test` writing live config via SaveJSON absolute-path fallback (fixed v0.34.10). PG-VAL-03 = paper-mode realized slip overrides to ModeledSlipBps (fixed v0.34.11). PG-DEBT-01 = `cmd/bingxprobe/` deleted (v0.34.11; case-insensitive JSON decode bug it diagnosed already fixed in v0.34.6).

Requirements: `.planning/REQUIREMENTS.md`

### Phase 10: Dashboard Candidate CRUD

**Goal:** Operator can Add/Edit/Delete `PriceGapCandidate` entries from the Price-Gap dashboard tab, with changes persisting to `config.json` via the existing `POST /api/config` round-trip and surfacing in the running tracker on the next scan tick — no manual `config.json` editing required.

**Scope (PG-OPS-07):**
- New "Add candidate" button in `web/src/pages/PriceGap.tsx`
- Shared Add/Edit modal with 6 fields: `symbol`, `long_exch`, `short_exch`, `threshold_bps`, `max_position_usdt`, `modeled_slippage_bps`
- Delete confirmation dialog with active-position safety check
- Frontend + backend validation (defense-in-depth)
- EN + zh-TW i18n keys added in lockstep under `priceGap.candidates.*`
- Reuse existing `POST /api/config` endpoint and `SaveJSON` `.bak` backup flow
- Existing per-candidate Disable/Re-enable buttons unchanged

**Out of scope:**
- Auto-discovery (Phase 11), auto-promotion (Phase 12)
- Live capital deployment (v2.2)
- pg-admin CLI parity for add/edit/delete
- WebSocket broadcast for multi-operator sync (v3.0)

**Canonical refs:** see `.planning/phases/10-dashboard-candidate-crud/10-CONTEXT.md` `<canonical_refs>` section.

**Requirements addressed:** PG-OPS-07

**Plans:** 5/5 plans complete

Plans:
- [x] 10-01-PLAN.md — Backend: extend priceGapUpdate struct + validation + active-position guard + Go unit tests
- [x] 10-02-PLAN.md — i18n: add 23 pricegap.candidates.* keys to en.ts and zh-TW.ts in lockstep
- [x] 10-03-PLAN.md — Frontend: Add/Edit modal + Delete confirm dialog + form state in PriceGap.tsx
- [x] 10-04-PLAN.md — Tests: Vitest for modal/PG-OPS-08/i18n parity + Go test pinning tracker hot-reload
- [x] 10-05-PLAN.md — Build + manual UAT (16-step checkpoint with operator sign-off)

## Progress

| Phase | Milestone | Plans | Status | Completed |
|---|---|---|---|---|
| 1. Spot-Futures Exchange Expansion | v1.0 | 3/3 | Complete | 2026-04-02 |
| 2. Spot-Futures Automation | v1.0 | 3/3 | Complete | 2026-04-02 |
| 3. Operational Safety | v1.0 | 3/3 | Complete | 2026-04-03 |
| 4. Performance Analytics | v1.0 | 4/4 | Complete | 2026-04-04 |
| 5. Capital Allocation | v1.0 | 3/3 | Complete | 2026-04-05 |
| 6. Spot-Futures Risk Hardening | v1.0 | 4/4 | Complete | 2026-04-05 |
| 7. Milestone Polish | v1.0 | 1/1 | Complete | 2026-04-06 |
| 8. Price-Gap Tracker Core | v2.0 | 8/8 | Complete | 2026-04-22 |
| 9. Price-Gap Dashboard & Paper→Live Operations | v2.0 | 11/11 | Complete | 2026-04-25 |
| 10. Dashboard Candidate CRUD | v2.1 | 5/5 | Complete    | 2026-04-25 |
| 11. Auto-discovery Scanner | v2.1 | 0/? | Not started | — |
| 12. Auto-promotion | v2.1 | 0/? | Not started | — |
| 13. v2.0 Deferred Closure | v2.1 | 0/? | Not started | — |
| 999.1. Bidirectional pricegap candidates | backlog | 6/6 | Complete   | 2026-04-27 |

## Backlog

### Phase 999.1: Bidirectional pricegap candidates (PLANNED)

**Goal:** Add `direction: "bidirectional" | "pinned"` field to `PriceGapCandidate`. `pinned` = configured-direction-only fire (now WITH positive-sign filter — closes latent bug where `barRing.allExceed` used `math.Abs` and silently fired bidirectional in detection). `bidirectional` = detector evaluates `|spread| ≥ T` and fires whichever sign crosses, swapping leg roles on the wire at fire time. Saves 50% config when symmetric pairs have edge in both directions; opt-in per candidate so noise-prone pairs stay pinned.

**Origin:** Operator question 2026-04-27 during Phase 10 testing. Phase 8 D-08 currently mandates two entries for both directions; this phase makes that optional. Planned 2026-04-25 after research + inline locked decisions (A1 sign-filter, A2/A5 verification, default=pinned, FiredDirection+CandidateLongExch+CandidateShortExch observability).

**Touches:** `internal/models/pricegap_interfaces.go` + `pricegap_position.go` (struct fields), `internal/pricegaptrader/{detector,execution,monitor,risk_gate}.go` (sign branch + role swap + role-blindness), `internal/api/pricegap_handlers.go` (validator), `web/src/pages/PriceGap.tsx` (modal radio), `web/src/i18n/{en,zh-TW}.ts` (lockstep keys).

**Risks managed:** noise-side trades (opt-in only), per-exchange leg-role economics (operator controls per-candidate), observability (FiredDirection + configured tuple persisted), Phase 10 active-position guard tuple matching (CandidateLongExch/ShortExch fields).

**Requirements:** [PG-DIR-01]
**Plans:** 6/6 plans complete

Plans:
- [x] 999.1-01-PLAN.md — Backend struct fields + detector sign branch + executor leg-role swap + risk-gate role-blindness verification (Wave 1)
- [x] 999.1-02-PLAN.md — Validator extension for `direction` field with enum check (Wave 1)
- [x] 999.1-03-PLAN.md — i18n EN + zh-TW lockstep keys for `pricegap.candidates.modal.direction.*` (Wave 1)
- [x] 999.1-04-PLAN.md — Frontend modal Direction radio toggle + form-state + client-side validation (Wave 2; depends on 01/02/03)
- [x] 999.1-05-PLAN.md — End-to-end paper-mode test + backward-compat regression + i18n parity test (Wave 2; depends on 01/02/03/04)
- [x] 999.1-06-PLAN.md — Full build + VERSION/CHANGELOG bump + brief operator UAT (Wave 3; autonomous: false; depends on 01-05)
