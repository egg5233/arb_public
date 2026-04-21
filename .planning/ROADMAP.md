# Roadmap: Arb Bot — Unified Arbitrage System

## Milestones

- ✅ **v1.0 Unified Arb Platform** — Phases 1–7 (shipped 2026-04-21). See `milestones/v1.0-ROADMAP.md`
- 🚧 **v2.0 Multi-Strategy Expansion** — Strategy 4 (price-gap arb) MVP + live validation (planning)

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

Primary goal: ship Strategy 4 (cross-exchange price-gap arbitrage) as a minimal live tracker (Option C — MVP-first), validate live edge for 4–8 weeks, and defer full Gated Discovery (Option D) until v2.1 gated on live validation.

**Phase Numbering:**
- Integer phases continue from v1.0 → v2.0 starts at **Phase 8**
- Decimal phases (e.g., 8.1) reserved for urgent post-plan insertions

Seed input: `/tmp/phase0-pricegap/STRATEGY_DESIGN.md` (§3.5 candidate shortlist, §8 MVP rollout)

- [ ] **Phase 8: Price-Gap Tracker Core** — New `internal/pricegaptrader/` module: event detection, delta-neutral IOC entry/exit, Redis persistence, config switch, core risk gates (PG-01..05, PG-RISK-01..05, PG-OPS-06)
- [ ] **Phase 9: Price-Gap Dashboard & Paper→Live Operations** — Dashboard tab, Telegram alerts, paper mode toggle, slippage instrumentation, rolling candidate metrics (PG-OPS-01..05, PG-VAL-01..02)

## Phase Details

### Phase 8: Price-Gap Tracker Core
**Goal**: A new isolated tracker subsystem detects cross-exchange price-gap events on a static candidate list and opens/closes delta-neutral positions under hard risk gates — gated behind a dashboard-off default switch
**Depends on**: Nothing (v1.0 complete; new module strictly isolated from `internal/engine/` and `internal/spotengine/`)
**Requirements**: PG-01, PG-02, PG-03, PG-04, PG-05, PG-RISK-01, PG-RISK-02, PG-RISK-03, PG-RISK-04, PG-RISK-05, PG-OPS-06
**Success Criteria** (what must be TRUE):
  1. Operator can enable `PriceGapEnabled` in `config.json` (or via dashboard API) and the tracker starts reading the static candidate list from config; with the flag OFF (default), no tracker code paths run and existing perp-perp / spot-futures engines are byte-for-byte unaffected
  2. When a spread on a configured candidate crosses its threshold `T` and persists ≥4 consecutive 1m bars with fresh kline data (<90s), the tracker places simultaneous IOC market orders on both legs via existing exchange adapters and records the position to Redis under `pg:pos:{id}`
  3. Open positions auto-close when |spread| reverts to ≤ T/2 OR a 4h max-hold timer elapses; closed positions persist with realized PnL and exit reason; all positions survive a process restart via Redis rehydration
  4. Pre-entry risk gates deterministically block entries that would violate: Gate concentration >50% of `PriceGapBudget`, max concurrent positions (3), per-candidate notional cap, any delist/halt flag on either leg, or stale kline data (>90s)
  5. After 10 closed trades on a candidate, if mean realized slippage exceeds 2× modeled slippage, the tracker auto-disables that candidate and refuses further entries until a human re-enables it
**Plans**: 8 plans
  - [x] 08-01-PLAN.md — Config surface + models + PriceGapStore interface (PG-05, PG-OPS-06)
  - [x] 08-02-PLAN.md — Redis persistence under pg:* namespace (PG-04, PG-RISK-03)
  - [x] 08-03-PLAN.md — Tracker skeleton + detector (BBO-sampled 1m bars, 4-bar persistence) (PG-01)
  - [x] 08-04-PLAN.md — 5-check pre-entry risk gate + typed errors (PG-RISK-01, -02, -04, -05)
  - [x] 08-05-PLAN.md — Simultaneous IOC entry + unwind-to-match + circuit breaker (PG-02)
  - [x] 08-06-PLAN.md — Per-position monitor + exit + exec-quality auto-disable + rehydrate (PG-03, PG-04, PG-RISK-03)
  - [ ] 08-07-PLAN.md — tickLoop wiring + cmd/main.go conditional startup/shutdown (PG-02, PG-03, PG-OPS-06)
  - [ ] 08-08-PLAN.md — cmd/pg-admin CLI + CHANGELOG + VERSION bump (PG-RISK-03)

### Phase 9: Price-Gap Dashboard & Paper→Live Operations
**Goal**: Operators can observe, control, and validate the price-gap tracker from the dashboard — including a paper-mode dry run, live position/PnL views, Telegram alerts, and per-candidate rolling performance metrics
**Depends on**: Phase 8 (tracker module must exist and be wired into startup)
**Requirements**: PG-OPS-01, PG-OPS-02, PG-OPS-03, PG-OPS-04, PG-OPS-05, PG-VAL-01, PG-VAL-02
**Success Criteria** (what must be TRUE):
  1. A new "Price-Gap" dashboard tab (8th tab) renders the configured candidate list with per-candidate enable/disable toggles that round-trip through `/api/config` and persist to `config.json`; both `en` and `zh-TW` locale files include all new keys
  2. Operator can flip a paper-mode toggle (persisted to config) that runs the full detection and entry logic but suppresses real order placement; paper-mode entries and exits appear in the dashboard tagged as paper, with modeled fills so edge can be measured without capital at risk
  3. Live price-gap positions panel shows entry spread, current spread, hold time, and current PnL in real time via WebSocket; closed positions log shows realized-vs-modeled edge comparison for each trade
  4. Telegram notifications fire via the existing notifier on: price-gap entry, price-gap exit, and risk-gate blocks — cooldowns and toggles respect existing notifier config; no change to perp-perp or spot-futures notification paths
  5. Dashboard shows per-candidate rolling 7d and 30d net bps/day computed from logged realized-slippage data, giving the operator the evidence needed to promote/demote candidates manually
**Plans**: TBD
**UI hint**: yes

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
| 8. Price-Gap Tracker Core | v2.0 | 0/TBD | Not started | — |
| 9. Price-Gap Dashboard & Paper→Live Operations | v2.0 | 0/TBD | Not started | — |
