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

- [ ] **Phase 10: Dashboard Candidate CRUD** — Add/Edit/Delete modal in Price-Gap tab; round-trip via `/api/config` + i18n EN/zh-TW lockstep (PG-OPS-07)
- [ ] **Phase 11: Auto-discovery Scanner** — Periodic scan across 6 exchanges; score by spread persistence + book depth + retreat depth; write to `pg:discovered:*` Redis keys (PG-DISC-01, PG-DISC-03)
- [ ] **Phase 12: Auto-promotion** — Discovered candidates above `PriceGapAutoPromoteScore` threshold append to `cfg.PriceGapCandidates`; respect `PriceGapMaxCandidates` cap; Telegram + WS broadcast on promotion (PG-DISC-02)
- [ ] **Phase 13: v2.0 Deferred Closure** — Fix `realized_slippage_bps` calc, audit dashboard auto-POST mystery, decide `cmd/bingxprobe/` fate (PG-VAL-03, PG-OPS-08, PG-DEBT-01)

Requirements: `.planning/REQUIREMENTS.md`

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
| 10. Dashboard Candidate CRUD | v2.1 | 0/? | Not started | — |
| 11. Auto-discovery Scanner | v2.1 | 0/? | Not started | — |
| 12. Auto-promotion | v2.1 | 0/? | Not started | — |
| 13. v2.0 Deferred Closure | v2.1 | 0/? | Not started | — |
