# Roadmap: Arb Bot — Unified Arbitrage System

## Milestones

- ✅ **v1.0 Unified Arb Platform** — Phases 1–7 (shipped 2026-04-21). See `milestones/v1.0-ROADMAP.md`
- 🚧 **v2.0 Multi-Strategy Expansion** — Strategy 4 (price-gap arb) + v1.0 tech debt (planning)

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

### 🚧 v2.0 Multi-Strategy Expansion (Planning)

Primary goal: ship Strategy 4 (cross-exchange price-gap arbitrage) as a minimal tracker (Option C — MVP-first), validate live edge for 4–8 weeks, upgrade to full Gated Discovery architecture (Option D) only if PnL and candidate growth justify it.

Phases TBD — will be populated via `/gsd-new-milestone` → `/gsd-discuss-phase`. Seed input: `/tmp/phase0-pricegap/STRATEGY_DESIGN.md`.

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
