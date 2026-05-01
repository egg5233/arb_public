---
gsd_state_version: 1.0
milestone: v2.2
milestone_name: Auto-Discovery & Live Strategy 4
status: planning
stopped_at: Phase 15 context gathered
last_updated: "2026-05-01T02:54:05.775Z"
last_activity: 2026-04-30
progress:
  total_phases: 5
  completed_phases: 3
  total_plans: 15
  completed_plans: 15
  percent: 100
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-28 after v2.2 milestone start)

**Core value:** "I deposit USDT, select my risk preference, and the system automatically finds opportunities across multiple strategies, opens positions, collects yield, exits when profitable, and I can see exactly how much each position earned — with capital shifting between strategies as opportunities shift."
**Current focus:** Phase 14 — daily-reconcile-live-ramp-controller

## Current Position

Phase: 15
Plan: Not started
Status: Phase 14 closed; ready to plan Phase 15 (drawdown-circuit-breaker, PG-LIVE-02)
Last activity: 2026-04-30

**v2.2 phase structure (6 phases, 14 reqs, 100% coverage):**

1. Phase 11 — Auto-Discovery Scanner + Chokepoint + Telemetry (PG-DISC-01, PG-DISC-03, PG-DISC-04)
2. Phase 12 — Auto-Promotion (PG-DISC-02)
3. Phase 14 — Daily Reconcile + Live Ramp Controller (PG-LIVE-01, PG-LIVE-03)
4. Phase 15 — Drawdown Circuit Breaker (PG-LIVE-02)
5. Phase 16 — Paper-Mode Cleanup + Dashboard Consolidation (PG-FIX-01, PG-FIX-02, DEV-01, PG-OPS-09)
6. Phase 17 — v1.0 Tech-Debt Sweep (DEBT-V1-01, DEBT-V1-02, DEBT-V1-03)

Progress (v2.2): [          ] 0%

**Phase ordering:** Validate-first (Architecture research) chosen over money-safe-first (Pitfalls). Hard constraint (PG-DISC-04 chokepoint must land before scanner is write-permitted) satisfied by co-locating chokepoint with scanner read-only build in Phase 11. Reconcile + Ramp consecutive in Phase 14. Tech-debt LAST per Pitfall 7.

**Phase numbering:** Reused 11+12 from v2.1 deferred-numbering plan; 13 already consumed by v2.1 closure; v2.2 continues at 14–17.

## Performance Metrics

**Velocity (v1.0 baseline):**

- Total plans completed: 66
- Average duration: 14 min
- Total execution time: ~5 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 03 | 3 | 41min | 14min |
| 06 | 4 | - | - |
| 08 | 8 | - | - |
| 09 | 11 | - | - |
| 10 | 5 | - | - |
| 999.1 | 6 | - | - |
| 11 | 6 | - | - |
| 12 | 4 | - | - |
| 14 | 5 | - | - |

**Recent Trend:**

- Last 3 plans: 03-01(8min), 03-02(19min), 03-03(14min)
- Trend: stable

*Updated after each plan completion*
| Phase 04 P01 | 6min | 2 tasks | 6 files |
| Phase 04 P03 | 19min | 2 tasks | 10 files |
| Phase 04 P04 | 8 | 3 tasks | 16 files |
| Phase 05 P01 | 7min | 2 tasks | 5 files |
| Phase 05 P02 | 13min | 3 tasks | 7 files |
| Phase 05 P03 | 5min | 2 tasks | 6 files |
| Phase 06 P01 | 16min | 2 tasks | 15 files |
| Phase 06 P02 | 10min | 1 tasks | 7 files |
| Phase 06 P03 | 15min | 2 tasks | 6 files |
| Phase 06 P04 | 7min | 2 tasks | 8 files |
| Phase 07 P01 | 4min | 3 tasks | 8 files |
| Phase 08 P01 | 3 | 2 tasks | 4 files |
| Phase 08 P02 | 8min | 2 tasks | 2 files |
| Phase 08 P03 | 15min | 4 tasks | 5 files |
| Phase 08 P04 | 8 | 3 tasks | 3 files |
| Phase 08-price-gap-tracker-core P05 | 12min | 3 tasks | 2 files |
| Phase 08 P06 | 18min | 4 tasks | 11 files |
| Phase 08 P07 | 45m | 3 tasks | 4 files |
| Phase 08 P08 | 12m | 3 tasks | 4 files |
| Phase 09 P01 | 35min | 3 tasks | 13 files |
| Phase 09 P02 | 45min | 2 tasks | 11 files |
| Phase 09 P03 | 30m | 2 tasks | 3 files |
| Phase 09 P04 | 25min | 2 tasks | 5 files |
| Phase 09 P05 | 35m | 2 tasks | 4 files |
| Phase 09 P06 | ~40 min | 2 tasks | 10 files |
| Phase 09 P07 | ~35min | 2 tasks | 5 files |
| Phase 09 P08 | 30min | 3 tasks | 5 files |
| Phase 09 P09-09 | 25min | 3 tasks | 10 files |
| Phase 09 P09-10 | 30min | 2 tasks | 4 files |
| Phase 10 P01 | 5min | 2 tasks | 3 files |
| Phase 10 P02 | 6min | 2 tasks | 2 files |
| Phase 10 P03 | 7min | 2 tasks | 3 files |
| Phase 10 P04 | 6min | 2 tasks | 4 files |
| Phase 999.1 P01 | 45m | 3 tasks | 11 files |
| Phase 999.1 P02 | 5min | 1 tasks | 3 files |
| Phase 999.1 P04 | 10min | 1 tasks | 1 files |
| Phase 999.1-bidirectional-pricegap-candidates P05 | 25min | 2 tasks | 2 files |
| Phase 999.1 P06 | 5min | 2 tasks | 2 files |
| Phase 12 P01 | 18min | 3 tasks | 3 files |
| Phase 12 P02 | 22 | 2 tasks | 7 files |
| Phase 12 P03 | 25min | 2 tasks | 7 files |
| Phase 12 P04 | 35min | 4 tasks | 7 files |
| Phase 14 P01 | 12min | 3 tasks | 14 files |
| Phase 14 P02 | 8min | 3 tasks | 6 files |
| Phase 14 P03 | 8min | 4 tasks | 10 files |
| Phase 14 P04 | 15min | 3 tasks | 19 files |
| Phase 14 P05 | 12min | 3 tasks | 8 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [v2.2 Roadmap]: Validate-first phase ordering (Architecture research) chosen over money-safe-first (Pitfalls research). Scanner + telemetry calibrate against paper data BEFORE live capital is ramped; chokepoint co-located with scanner read-only build in Phase 11 to satisfy PG-DISC-04 hard constraint without delaying validation work.
- [v2.2 Roadmap]: Phase numbers 11+12 reused from v2.1 deferred-numbering plan; 13 already consumed by v2.1 closure; v2.2 continues at 14–17. Six phases total: 11 (scanner+chokepoint+telemetry), 12 (auto-promotion), 14 (reconcile+ramp), 15 (breaker), 16 (paper-bug+dashboard consolidation), 17 (tech-debt).
- [v2.2 Roadmap]: Reconcile (PG-LIVE-03) co-located with Ramp (PG-LIVE-01) in Phase 14 — ramp's clean-day signal depends on reconcile output; consecutive-phase compromise rejected in favor of single-phase delivery.
- [v2.2 Roadmap]: v1.0 tech-debt sweep (Phase 17) sequenced LAST per Pitfall 7. Re-exercising stale paths during retrospective is risky; live capital + chokepoint + breaker must be stable first. Surfaced regressions spawn separate Phase 999.x hot-fix mini-phases per v2.1 precedent.
- [v2.2 Roadmap]: Paper-mode bug closure (PG-FIX-01, PG-FIX-02), DEV-01, and dashboard consolidation (PG-OPS-09) bundled in Phase 16. PG-OPS-09 surfaces ALL Strategy 4 config in one tab — natural co-delivery with paper-mode fix because both touch dashboard wiring.

v2.0 + v1.0 decisions retained (truncated for brevity — see git history for full list):

- [v2.0 Roadmap]: Two phases — backend tracker core (Phase 8) then dashboard + paper→live operations (Phase 9)
- [v2.0 Roadmap]: All 5 risk gates (PG-RISK-01..05) bundled into Phase 8 because they are pre-entry invariants
- [v2.0 Roadmap]: v1.0 tech debt (DEBT-01..03) explicitly deferred — closed in v2.2 Phase 17
- [v2.0 Scoping]: 5 known-good candidates shortlist at T=200; real round-trip cost 55–90 bps (2× Phase 0 model)
- [Phase 09]: Paper mode ships as a single chokepoint at ex.PlaceOrder; pos.Mode stamped once at entry, monitor reads pos.Mode only (Pitfall 2 immutability)
- [Phase 999.1]: PG-DIR-01: Direction is a behavior property excluded from PriceGapCandidate.ID() — preserves Phase 10 D-11/D-13/D-14 identity invariants
- [Phase 999.1]: PG-DIR-01: Pinned mode receives a positive-sign filter (latent Phase-8 bug closure); CHANGELOG warns operators about behavior change
- [Phase 12]: Plan 12-02: WSBroadcaster narrow interface in pricegaptrader preserves D-15 module boundary; *api.Hub satisfies via duck typing at wiring site (Plan 12-03)
- [Phase 12]: Plan 03: scanner registry param widened to *Registry (D-17 swap); chokepoint discipline preserved by relaxed scanner_static_test.go regex forbidding only raw cfg.PriceGapCandidates assignment
- [Phase 12]: Plan 03: added Server.Hub() public accessor in internal/api so cmd/main.go can pass the WS hub to RedisWSPromoteSink without leaking internal/api into pricegaptrader (D-15 boundary preserved)
- [Phase 12]: Plan 04: PromoteTimeline replaces Phase 11 placeholder; usePgDiscovery hook does composite-key dedupe + 1000-cap at the hook layer (not component) so all consumers inherit bounded shape; PG-DISC-02 closed (success criterion #5 satisfied: events appear in dashboard timeline). Phase 12 backend+frontend feature-complete on v0.36.0 binary; default OFF behind PriceGapDiscoveryEnabled.
- [Phase 14]: [Phase 14 P01]: validatePriceGapDiscovery (Phase 11) intentionally remains unwired in Load() because wiring it broke pre-existing tests with MaxCandidates=100; only validatePriceGapLive runs at config-load (D-06 paper/live + v2.2 hard ceiling)
- [Phase 14]: [Phase 14 P01]: monitor.closePair SADD lands AFTER pos.ClosedAt (line 248) and BEFORE SavePriceGapPosition so anomalous timing is captured even when persistence fails — matches existing best-effort pattern
- [Phase 14]: [Phase 14 P01]: Per-day SET (pg:positions:closed:{YYYY-MM-DD}) has no TTL — operator backfill must work for any past date; RampState HASH uses HSet pipeline for atomic 5-field write
- [Phase 14]: [Phase 14 P02]: 3-retry pattern (5s/15s/30s) imitated locally in pricegaptrader/reconciler.go to preserve D-15 module isolation; sleeps applied BEFORE attempts 1+2 not before 0 or after 2, so triple-fail records sleeps [15s,30s] verified by TestReconcile_TripleFail_SkipsDay
- [Phase 14]: [Phase 14 P02]: ReconcileStore + ReconcileNotifier narrow interfaces declared in pricegaptrader/reconciler.go (NOT in models/pricegap_interfaces.go) — keeps reconcile-specific concerns local and avoids widening the cross-module models surface; *database.Client satisfies ReconcileStore implicitly via the existing PriceGapStore method set
- [Phase 14]: [Phase 14 P02]: TelegramNotifier digest+failure stub methods land in a separate file internal/notify/pricegap_reconcile.go (not telegram.go) — preserves the existing module-graph layout where telegram.go does not import arb/internal/pricegaptrader; Plan 14-04 replaces stub bodies with real Telegram dispatch
- [Phase 14]: Plan 14-03: Forward-declared RampSnapshotter narrow interface on Tracker (single Snapshot() method) lets Task 1 sizer wiring compile before Task 3 creates *RampController; production wires concrete *RampController via duck typing in Plan 14-04
- [Phase 14]: Plan 14-03: stageSizeForCfg helper duplicated in risk_gate.go (mirrors Sizer.stageSize) — kept in sync via doc comment; rejected exporting Sizer.StageSize to avoid coupling risk_gate.go to *Sizer when only the cap math is needed; D-22 defense in depth requires the two layers be independent
- [Phase 14]: Plan 14-03: ForcePromote preserves CleanDayCounter (D-15 #3 — operator override is intentional); ForceDemote zeroes counter + increments DemoteCount (D-15 #4 matches asymmetric ratchet); Reset clears all state including DemoteCount; all three fire NotifyPriceGapRampForceOp via narrow RampNotifier interface
- [Phase 14]: Plan 14-04: Tracker setters (SetReconciler/SetRamp/SetSizer) instead of NewTracker constructor param explosion — preserves 11+ existing tests; cmd/main.go wires via setters before Start
- [Phase 14]: Plan 14-04: Notify dispatch tests moved to internal/notify (cycle fix) — pricegaptrader notify_test.go cannot import notify because notify imports pricegaptrader via pricegap_assert.go; SetTelegramAPIBase test hook added in test_hook.go for sibling-package retargeting
- [Phase 14]: Plan 14-04: Added *database.Client.LoadPriceGapPosition (pos, exists, error) adapter — original GetPriceGapPosition conflates not-found with Redis errors; T-14-11 skipped-position counting requires the distinction
- [Phase 14]: Plan 14-04: Boot guard treats CurrentStage<1 as unified failure signal (covers nil-ramp + corrupt-Redis + legacy-empty-state). Tracker.Start panics + dispatches BOOT_GUARD critical Telegram BEFORE spawning daemon goroutines
- [Phase 14]: Plan 14-05: Narrow interfaces on *Server (RampSnapshotter + ReconcileRecordLoader) keep internal/api free of pricegaptrader concrete imports — production wires *RampController + *Reconciler via SetPgRamp/SetPgReconciler setters in cmd/main.go (D-15 boundary preserved, identical to Plan 12-03 Server.Hub() pattern)
- [Phase 14]: Plan 14-05: Server-authoritative live_capital flag — handler reads cfg.PriceGapLiveCapital directly (NEVER from client config cache); operator badge color drives money-at-risk perception, must reflect server truth on every refresh (T-14-18 mitigation)
- [Phase 14]: Plan 14-05: D-14 read-only contract enforced via explicit comment header in RampReconcileSection.tsx + grep verification (0 POST/PUT/PATCH/DELETE matches in widget); mutation UI deferred to Phase 16 PG-OPS-09 absorb when new top-level Pricegap tab is built

### Pending Todos

- `/gsd-plan-phase 11` — decompose Phase 11 (Auto-Discovery Scanner + Chokepoint + Telemetry) into executable plans

### Blockers/Concerns

- `internal/pricegaptrader/` module-boundary rule still applies to all v2.2 work — no imports of `internal/engine/` or `internal/spotengine/`.
- Redis namespace `pg:*` extends to `pg:scan:*`, `pg:promote:*`, `pg:ramp:*`, `pg:reconcile:*`, `pg:breaker:*` — must not collide with existing pg keys.
- All v2.2 features default OFF (`PriceGapDiscoveryEnabled`, `PriceGapAutoPromote`, `PriceGapLiveCapital` etc).
- npm lockdown still in force — Phase 16 dashboard consolidation uses existing Recharts/React only (`npm ci` only).
- Live trading risk: every Phase 11–17 change lands behind config flag. Perp-perp + spot-futures + Strategy 4 paper engines must remain undisturbed during ramp + breaker work (Phases 14–15).
- CandidateRegistry chokepoint MUST land before scanner is write-permitted (Phase 11 plan-phase decomposition must order this correctly within the phase).

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260408-ugr | deliveryDate-based delist detection | 2026-04-08 | 262ac8f | [260408-ugr-deliverydate-based-delist-detection](./quick/260408-ugr-deliverydate-based-delist-detection/) |
| 260415-34e | Cross-engine interference audit (21 findings: 7 HIGH / 9 MEDIUM / 5 LOW) | 2026-04-15 | d2518dc | [260415-34e-find-all-cross-engine-interference-that-](./quick/260415-34e-find-all-cross-engine-interference-that-/) |

## Session Continuity

Last session: 2026-05-01T02:54:05.770Z
Stopped at: Phase 15 context gathered
Resume file: .planning/phases/15-drawdown-circuit-breaker/15-CONTEXT.md
Next command: `/gsd-plan-phase 15` to decompose Phase 15 (Drawdown Circuit Breaker, PG-LIVE-02) into executable plans
