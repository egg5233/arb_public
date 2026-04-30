# Roadmap: Arb Bot — Unified Arbitrage System

## Milestones

- ✅ **v1.0 Unified Arb Platform** — Phases 1–7 (shipped 2026-04-21). See `milestones/v1.0-ROADMAP.md`
- ✅ **v2.0 Multi-Strategy Expansion** — Phases 8–9 (shipped 2026-04-25). See `milestones/v2.0-ROADMAP.md`
- ✅ **v2.1 Candidate Operations** — Phases 10, 13, 999.1 shipped 2026-04-27 (PG-DISC-01/02/03 deferred to v2.2). See `milestones/v2.1-ROADMAP.md`
- 📋 **v2.2 Auto-Discovery & Live Strategy 4** — Phases 11, 12, 14–17 (planned)

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

Audit: `tech_debt` status — all code shipped and live; verification/Nyquist docs incomplete. Debt closed in v2.2 Phase 17.

Full detail: `.planning/milestones/v1.0-ROADMAP.md`
Requirements: `.planning/milestones/v1.0-REQUIREMENTS.md`
Audit: `.planning/milestones/v1.0-MILESTONE-AUDIT.md`

</details>

<details>
<summary>✅ v2.0 Multi-Strategy Expansion (Phases 8–9) — SHIPPED 2026-04-25</summary>

- [x] Phase 8: Price-Gap Tracker Core (8/8 plans) — PG-01..05, PG-RISK-01..05, PG-OPS-06
- [x] Phase 9: Price-Gap Dashboard & Paper→Live Operations (11/11 plans) — PG-OPS-01..05, PG-VAL-01..02

Three known issues (machine-zero realized slip, dashboard auto-flip, bingxprobe Make target) closed in v2.2 Phase 16.

Full detail: `.planning/milestones/v2.0-ROADMAP.md`
Requirements: `.planning/milestones/v2.0-REQUIREMENTS.md`

</details>

<details>
<summary>✅ v2.1 Candidate Operations (Phases 10, 13, 999.1) — SHIPPED 2026-04-27</summary>

- [x] Phase 10: Dashboard Candidate CRUD (5/5 plans) — PG-OPS-07
- [x] Phase 13: v2.0 Deferred Closure — PG-VAL-03, PG-OPS-08, PG-DEBT-01 (direct commits)
- [x] Phase 999.1: Bidirectional pricegap candidates (6/6 plans) — PG-DIR-01

PG-DISC-01/02/03 explicitly deferred to v2.2 (Phases 11+12 reused per original numbering plan).

Full detail: `.planning/milestones/v2.1-ROADMAP.md`
Requirements: `.planning/milestones/v2.1-REQUIREMENTS.md`
Audit: `.planning/milestones/v2.1-MILESTONE-AUDIT.md`

</details>

### 📋 v2.2 Auto-Discovery & Live Strategy 4 (Active)

**Goal:** Promote Strategy 4 from paper observation to live capital with conservative ramp, ship the deferred auto-discovery + auto-promotion pipeline, close paper-mode bugs, and clear v1.0 tech debt completely.

**Phase ordering rationale (Architecture / validate-first chosen):**
The roadmap follows Architecture-research ordering: scanner + telemetry calibrate against paper data BEFORE live capital is ramped. Hard constraint from Pitfall 2 (CandidateRegistry chokepoint must land before scanner is write-permitted) is satisfied by **co-locating the chokepoint with the scanner read-only build in Phase 11** — chokepoint ships inside the same phase but as a discrete plan that completes before promotion (Phase 12) goes live. This gives:
- Three days of paper-mode observation data already exists for scanner score calibration
- Zero live-capital risk during scanner build (Phase 11)
- Auto-promotion (Phase 12) cannot ship without chokepoint already merged
- Reconcile (PG-LIVE-03) and Ramp (PG-LIVE-01) are co-located in Phase 14 (ramp's clean-day signal depends on reconcile output)
- Breaker (PG-LIVE-02) follows in Phase 15 with reconcile + ramp shape already known
- Paper-mode bugs + Dashboard consolidation + DEV-01 grouped in Phase 16 (parallelizable cleanup)
- Tech-debt sweep (Phase 17) sequenced LAST per Pitfall 7

Phase numbers reuse 11 + 12 from the v2.1 deferred-numbering plan; 13 was consumed by v2.1 closure, so v2.2 continues at 14 → 17.

**Phases:**

- [x] **Phase 11: Auto-Discovery Scanner + Chokepoint + Telemetry** — Read-only scanner with score, BBO freshness, depth probe, denylist; CandidateRegistry chokepoint serializing all writers; telemetry surfaced to dashboard. Default OFF. (completed 2026-04-28)
- [x] **Phase 12: Auto-Promotion** — Score-gated auto-promotion through chokepoint with cap, dedupe, observation streak, active-position guard, Telegram + WS broadcast. (completed 2026-04-30)
- [x] **Phase 14: Daily Reconcile + Live Ramp Controller** — Daily PnL reconcile keyed by close-timestamp + ramp controller with Redis-persisted clean-day counter, asymmetric ratchet, hard-ceiling sizing guard. (completed 2026-04-30)
- [ ] **Phase 15: Drawdown Circuit Breaker** — Realized-PnL rolling-24h breaker with two-strike rule, Bybit-blackout suppression, sticky paper-mode flag, human-gated recovery.
- [ ] **Phase 16: Paper-Mode Cleanup + Dashboard Consolidation** — Fix realized-slippage zero, fix dashboard auto-POST flip, promote bingxprobe to Make target, consolidate all Strategy 4 config into new Price-Gap dashboard tab.
- [ ] **Phase 17: v1.0 Tech-Debt Sweep** — Phase 07 retrospective VERIFICATION/VALIDATION, Nyquist Wave-0 for v1.0 phases 01/03/04/06, browser confirmations for v1.0 phases 02/03/05/06.

## Phase Details

### Phase 11: Auto-Discovery Scanner + Chokepoint + Telemetry
**Goal**: A read-only scanner surfaces ranked Strategy 4 candidates with score + reasoning to the dashboard, while a single CandidateRegistry chokepoint serializes all future writers (dashboard, pg-admin, scanner) — providing the calibration window and write-protection foundation that Phase 12 promotion depends on.
**Depends on**: Nothing (additive inside `internal/pricegaptrader/`, default OFF)
**Requirements**: PG-DISC-01, PG-DISC-03, PG-DISC-04
**Success Criteria** (what must be TRUE):
  1. With `PriceGapDiscoveryEnabled=true`, the scanner runs at the configured interval, polls the bounded universe (≤20 symbols × 6 exchanges), applies ≥4-bar persistence + BBO freshness + depth probe gates, and writes scored candidates to `pg:scan:*` Redis keys.
  2. The dashboard's Discovery section displays scanner cycle stats, per-candidate score history, why-rejected breakdown, and an empty (Phase 12 will fill it) promote/demote timeline.
  3. The CandidateRegistry chokepoint compiles in and replaces all existing direct mutations of `cfg.PriceGapCandidates` from the dashboard handler and pg-admin CLI; concurrent writes from those two paths produce zero lost mutations in the integration test.
  4. The scanner is read-only — it can compute scores but cannot mutate `cfg.PriceGapCandidates`. Default-OFF behaviour verified: scanner does not run with `PriceGapDiscoveryEnabled=false`.
  5. Symbol normalization (canonical `BTCUSDT` form) verified via cross-exchange BBO join test for a known-good pair.
**Plans**: 6 plans
- [x] 11-01-PLAN.md — Config schema (8 fields) + RegistryReader interface + Plan 01 unit tests
- [x] 11-02-PLAN.md — *Registry chokepoint (Add/Update/Delete/Replace) + .bak.{ts} ring + concurrent-writer integration test
- [x] 11-03-PLAN.md — Hard-cut migration: handlers.go + pg-admin onto Registry; shared validate.go
- [x] 11-04-PLAN.md — Scanner core (gates + 0-100 magnitude scoring + read-only static check)
- [x] 11-05-PLAN.md — Telemetry (pg:scan:* + WS) + REST endpoints + cmd/arb bootstrap wiring
- [x] 11-06-PLAN.md — Discovery dashboard UI (6 components + hook + i18n lockstep + visual + live-BBO checkpoints)
**UI hint**: yes

### Phase 12: Auto-Promotion
**Goal**: The scanner can promote candidates above the score threshold to `cfg.PriceGapCandidates` automatically through the chokepoint, with all safety guards from manual CRUD honoured, observation streak enforced, and operators notified out-of-band.
**Depends on**: Phase 11 (chokepoint + scanner output must exist)
**Requirements**: PG-DISC-02
**Success Criteria** (what must be TRUE):
  1. With `PriceGapDiscoveryEnabled=true` and a candidate scoring ≥ `PriceGapAutoPromoteScore` for ≥6 consecutive cycles, the controller appends it to `cfg.PriceGapCandidates` via the chokepoint and persists to `config.json` with timestamped `.bak` rotation.
  2. The cap (`PriceGapMaxCandidates`, default 12) is enforced — the 13th candidate cannot be appended.
  3. Idempotent dedupe rejects duplicate `(symbol, longExch, shortExch, direction)` tuples (including the v0.35.0 `direction` field).
  4. Auto-demote honors the `pg:positions:active` guard — a candidate with an active position cannot be auto-demoted.
  5. Promote and demote events fire Telegram critical alert AND WS broadcast; events appear in the dashboard timeline added in Phase 11.
**Plans**: 4 plans
- [x] 12-01-PLAN.md - PromotionController core (TDD: streak, cap-full, dedupe, demote, active-position guard)
- [x] 12-02-PLAN.md - I/O surfaces: RedisWSPromoteSink, IncCapFullSkip, REST seed, Telegram NotifyPromoteEvent
- [x] 12-03-PLAN.md - Wire controller into Scanner.RunCycle + ActivePositionChecker + bootstrap + v0.36.0
- [x] 12-04-PLAN.md - Frontend swap: PromoteTimeline component + i18n + human-verify checkpoint
**UI hint**: yes

### Phase 14: Daily Reconcile + Live Ramp Controller
**Goal**: Strategy 4 has a daily PnL reconcile job producing per-day realized-PnL output AND a Redis-persisted ramp controller that gates live-capital sizing through 100 → 500 → 1000 USDT/leg stages with asymmetric ratchet — the first phase to touch live capital.
**Depends on**: Phase 12 (auto-promoted candidates ride the live ramp)
**Requirements**: PG-LIVE-01, PG-LIVE-03
**Success Criteria** (what must be TRUE):
  1. Daily reconcile fires 30+ minutes after UTC 00:00, aggregates closed Strategy 4 positions keyed by `(position_id, version)` using exchange close-timestamp, and writes `pg:reconcile:daily:{date}` Redis keys idempotently (re-run produces byte-identical output).
  2. Reconcile flags anomalies (large slippage, missing close timestamp) and emits a daily Telegram digest.
  3. Ramp state Redis-persisted with 5 explicit fields; `kill -9` mid-stage followed by restart resumes correct stage + clean-day counter.
  4. Asymmetric ratchet works: a single losing day resets clean-day counter to 0 AND demotes one stage if currently above stage 1.
  5. Hard ceiling enforced as `min(stage_size, hard_ceiling)` at the sizing call site — a config typo of `stage_3_size_usdt: 9999` still sizes at 1000.
  6. Ramp gate integrated into `risk_gate.go` as gate #7, returning `GateRampExceeded` for over-budget proposals.
**Plans**: 5 plans
- [x] 14-01-PLAN.md — Foundation (config schema + Position model + Redis namespace + close-path SADD + Wave-0 stubs)
- [x] 14-02-PLAN.md — Reconciler core (3-retry imitation + idempotency byte-equality + anomaly flagging)
- [x] 14-03-PLAN.md — RampController + Gate 6 + Sizer (asymmetric ratchet, Redis persistence, defense-in-depth caps)
- [x] 14-04-PLAN.md — Daemon + pg-admin + Telegram (UTC 00:30 fire, boot guard, 6 subcommands, 4 telegram methods, VERSION+CHANGELOG)
- [x] 14-05-PLAN.md — API + Frontend widget (read-only /api/pg/ramp + /api/pg/reconcile/{date}, dashboard panel, EN+zh-TW lockstep, human-verify)
**UI hint**: yes

### Phase 15: Drawdown Circuit Breaker
**Goal**: Strategy 4 has a drawdown circuit breaker that flips the engine to paper mode on realized-PnL drawdown breach, with no false trips during funding-settlement windows or single-snapshot artifacts, and recovery requires explicit operator action.
**Depends on**: Phase 14 (reconcile output + paper-mode toggle path)
**Requirements**: PG-LIVE-02
**Success Criteria** (what must be TRUE):
  1. Breaker monitors REALIZED PnL only on a rolling 24h window (not calendar-day, not MTM); funding-settlement window in Bybit's `:04-:05:30` blackout does NOT trigger evaluation.
  2. Two-strike rule enforced: a single snapshot below `PriceGapDrawdownLimitUSDT` does NOT trip; two consecutive evaluations ≥5 min apart are required.
  3. On trip, the breaker auto-flips `paper_mode=true`, sets `PaperModeStickyUntil`, auto-disables any open candidate, fires Telegram critical alert + WS broadcast, and logs to `pg:breaker:trips`.
  4. Recovery requires explicit operator action via dashboard — sticky flag does NOT auto-clear on restart or page reload.
  5. Synthetic test fire exercises full breaker → paper-flip → operator recovery cycle without engine restart.
**Plans**: TBD
**UI hint**: yes

### Phase 16: Paper-Mode Cleanup + Dashboard Consolidation
**Goal**: Paper-mode metrics report non-zero realized slippage, the dashboard cannot regress paper_mode to false on page load, the BingX debug utility is reproducible via Make, and all Strategy 4 configuration lives in one consolidated Price-Gap dashboard tab.
**Depends on**: Phase 15 (PaperModeStickyUntil semantics finalized)
**Requirements**: PG-FIX-01, PG-FIX-02, DEV-01, PG-OPS-09
**Success Criteria** (what must be TRUE):
  1. With `paper_mode=true`, the synth-fill formula produces non-zero `realized_slippage_bps` for closed paper positions (regression test verifies a non-zero value across modeled-vs-realized delta).
  2. Loading the dashboard cannot flip `paper_mode=false` — DevTools Network capture during page load shows no offending POST, and the Phase 9 chokepoint pattern (`pos.Mode` immutable after entry) remains intact.
  3. `make probe-bingx` runs end-to-end and prints a successful BingX probe response.
  4. The new top-level "Price-Gap" dashboard tab consolidates paper toggle, ramp display, breaker threshold, scanner config, `PriceGapMaxCandidates`, `PriceGapAutoPromoteScore`, candidate CRUD, and bidirectional mode — and legacy controls in other tabs migrate or proxy to the new tab.
  5. EN + zh-TW i18n keys for the new tab are in lockstep (no missing translations).
**Plans**: TBD
**UI hint**: yes

### Phase 17: v1.0 Tech-Debt Sweep
**Goal**: v1.0 tech-debt classification is fully cleared — Phase 07 retrospective documentation exists, Nyquist Wave-0 validations pass for stale phases, browser confirmations are recorded, and any latent regressions surfaced during the sweep are spawned as separate hot-fix mini-phases (Phase 999.x precedent).
**Depends on**: Phases 11–16 (live trading must be stable on chokepoint + breaker before re-exercising stale paths)
**Requirements**: DEBT-V1-01, DEBT-V1-02, DEBT-V1-03
**Success Criteria** (what must be TRUE):
  1. v1.0 Phase 07 VERIFICATION.md + VALIDATION.md committed; SF-RISK-01 maintenance gate dashboard wiring documented retrospectively against the live v0.29.0 code.
  2. Nyquist Wave-0 validation tests pass for v1.0 Phases 01, 03, 04, 06 (live-fire tooling run, no stub VALIDATION.md).
  3. Browser confirmations recorded for v1.0 Phases 02, 03, 05, 06 with HAR/Network panel captures of each smoke pass.
  4. Any regressions surfaced (per Pitfall 7 precedent) are opened as separate Phase 999.x hot-fix phases with their own version bumps — they do NOT silently merge into the retrospective phase.
  5. v1.0 milestone audit re-run shows `tech_debt` status cleared; coverage table reflects 24/24 reqs satisfied.
**Plans**: TBD

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
| 10. Dashboard Candidate CRUD | v2.1 | 5/5 | Complete | 2026-04-25 |
| 13. v2.0 Deferred Closure | v2.1 | n/a | Complete (direct commits) | 2026-04-25 |
| 999.1. Bidirectional pricegap candidates | v2.1 | 6/6 | Complete | 2026-04-27 |
| 11. Auto-Discovery Scanner + Chokepoint + Telemetry | v2.2 | 6/6 | Complete    | 2026-04-28 |
| 12. Auto-Promotion | v2.2 | 4/4 | Complete    | 2026-04-30 |
| 14. Daily Reconcile + Live Ramp Controller | v2.2 | 5/5 | Complete   | 2026-04-30 |
| 15. Drawdown Circuit Breaker | v2.2 | 0/? | Not started | — |
| 16. Paper-Mode Cleanup + Dashboard Consolidation | v2.2 | 0/? | Not started | — |
| 17. v1.0 Tech-Debt Sweep | v2.2 | 0/? | Not started | — |

## Backlog

(empty)
