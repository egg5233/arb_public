---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: Multi-Strategy Expansion
status: executing
stopped_at: Completed 08-06-PLAN.md
last_updated: "2026-04-21T09:48:49.706Z"
last_activity: 2026-04-21
progress:
  total_phases: 2
  completed_phases: 0
  total_plans: 8
  completed_plans: 6
  percent: 75
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-21 after v1.0 shipped)

**Core value:** "I deposit USDT, select my risk preference, and the system automatically finds opportunities across multiple strategies, opens positions, collects yield, exits when profitable, and I can see exactly how much each position earned — with capital shifting between strategies as opportunities shift."
**Current focus:** Phase 08 — price-gap-tracker-core

## Current Position

Phase: 08 (price-gap-tracker-core) — EXECUTING
Plan: 7 of 8
Status: Ready to execute
Last activity: 2026-04-21

v1.0 shipped: 7 phases, 21 plans, 381 commits over 30 days (2026-03-23 → 2026-04-21). Audit: tech_debt — documentation/verification gaps carried forward as v2.0 backlog (DEBT-01..03, deferred).

**v2.0 phase structure:**

- Phase 8: Price-Gap Tracker Core (backend) — PG-01..05, PG-RISK-01..05, PG-OPS-06 (11 reqs)
- Phase 9: Price-Gap Dashboard & Paper→Live Operations (frontend + validation) — PG-OPS-01..05, PG-VAL-01..02 (7 reqs)

Progress (v2.0): [          ] 0%

## Performance Metrics

**Velocity (v1.0 baseline):**

- Total plans completed: 21
- Average duration: 14 min
- Total execution time: ~5 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 03 | 3 | 41min | 14min |
| 06 | 4 | - | - |

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

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [v2.0 Roadmap]: Two phases — backend tracker core (Phase 8) then dashboard + paper→live operations (Phase 9); split at Go/React boundary is the natural seam and lets Phase 8 be verified headlessly via logs + Redis before UI work
- [v2.0 Roadmap]: `PG-OPS-06` (config switch) stays in Phase 8 — the switch must exist before the tracker can run; dashboard UI round-trip for the toggle lives in Phase 9 via the general `PG-OPS-01` tab wiring
- [v2.0 Roadmap]: Paper mode (`PG-OPS-04`) lives in Phase 9 — it is a dashboard toggle that gates real order placement inside the tracker; the tracker in Phase 8 honors the flag but the UX is Phase 9
- [v2.0 Roadmap]: All 5 risk gates (PG-RISK-01..05) bundled into Phase 8 because they are pre-entry invariants; no trade should ever execute without them, so they ship with the entry path
- [v2.0 Roadmap]: v1.0 tech debt (DEBT-01..03) explicitly deferred from v2.0 roadmap per PROJECT.md priority — not worth blocking Strategy 4 on retrospective docs
- [v2.0 Scoping]: Phase 0/1/round-2 complete in `/tmp/phase0-pricegap/`; 5 known-good candidates shortlist at T=200; real round-trip cost 55–90 bps (2× Phase 0 model) — this informs PG-RISK-03's 2× slippage trigger
- [v2.0 Scoping]: Initial live budget $5k, $1-3k per-leg caps — PG-RISK-05 enforces per-candidate notional from config

v1.0 decisions below (retained for reference):

- [Roadmap]: Spot-futures expansion before operational safety -- user Priority 1
- [Roadmap]: PP-04 grouped with analytics (Phase 4) not safety (Phase 3) -- it is a dashboard/data feature
- [Roadmap]: Phase 3 has no dependency on Phases 1-2 -- can be pulled forward if needed
- [Phase 01]: OKX cross-margin borrows via tdMode=cross + ccy=USDT (Futures mode implicit) instead of autoLoan API
- [Phase 03-01]: Cooldown logic uses injectable time parameter for deterministic testing
- [Phase 03-01]: Notifier does not gate on config -- callers check EnablePerpTelegram before calling
- [Phase 03-02]: Fail-open on Redis query errors -- loss event query failure does not block entries
- [Phase 03-03]: Safety tab uses emerald color to distinguish from amber-colored Global Risk tab
- [Phase 03-03]: VERSION bumped to 0.26.0 to cover all Phase 3 operational safety work
- [Phase 04]: Pure-Go SQLite via modernc.org/sqlite -- no CGO dependency, single binary preserved
- [Phase 04]: Perp-perp 50/50 PnL split across exchanges for fair attribution in exchange metrics
- [Phase 04]: Analytics routes always registered (return 503 when disabled) to avoid frontend 404s
- [Phase 04]: SnapshotWriter uses non-blocking buffered channel (100) — analytics never blocks trading
- [Phase 04]: BasisGainLoss formula: reconciledPnL - reconciledFunding - rotationPnL + totalFees
- [Phase 04]: Analytics tab uses same pattern as Safety tab (no sub-tabs) in Config strategy toggle bar
- [Phase 05]: New allocation JSON section in config -- groups all Phase 5 fields together
- [Phase 05]: ComputeEffectiveAllocation is a pure function (not method) for easy unit testing
- [Phase 05]: SizeMultiplier applied only in EffectiveCapitalPerLeg derivation, not during profile application
- [Phase 05]: Use GetHistory(200)/GetSpotHistory(200) for trailing APR instead of nonexistent GetClosedPositions
- [Phase 05]: Server gets allocator via SetCapitalAllocator setter, matching existing DI pattern
- [Phase 05]: Minimum 3 trades per strategy before performance-weighted allocation tilt
- [Phase 05]: Direct fetch in Overview useEffect for allocation data instead of threading through App props
- [Phase 05]: Violet color scheme for allocation tab to distinguish from risk (amber) and safety (emerald)
- [Phase 06]: GetMaintenanceRate kept as optional interface (maintenanceRateProvider), not on Exchange interface -- BingX excluded
- [Phase 06]: OKX/Bitget MaintenanceRate=0 in LoadAllContracts (fetched on demand); Gate.io populates inline
- [Phase 06]: Lazy cache initialization in getMaintenanceRate() avoids SpotEngine constructor changes
- [Phase 06]: Used isSeparateAccount() for capital-per-leg selection in maintenance gate -- consistent with existing codebase pattern
- [Phase 06]: Cross-engine dispatch via callback function type (not interface) for health monitor spot-futures actions
- [Phase 06]: Fail-open on GetActiveSpotPositions Redis error in health monitor -- perp-perp continues
- [Phase 06]: L4 selects largest spot position by NotionalUSDT; L5 includes ALL spot positions
- [Phase 06]: lookupMaintenanceRateForDisplay() reuses existing getMaintenanceRate() with planned notional from config
- [Phase 06]: Maintenance rate column after Net APR / before Gap; color thresholds 10%/5% for visual risk assessment
- [Phase 07]: Maintenance gate toggle placed in sf-general tab (top-level engine feature, not exit-specific)
- [Phase 07]: Server-side validation matches config.go applyJSON: MaintenanceDefault 0 < val < 1.0, CacheTTL >= 1
- [Phase 08]: Plan 08-01: ExitReason is string enum (not int) for readable Redis JSON; colocated domain types split across two files (position data vs behavioural contract)
- [Phase 08]: Token-based pg lock API (AcquirePriceGapLock returns token) with compare-and-delete release via reused releaseLockScript
- [Phase 08]: priceGapLockPrefix=arb:locks:pg: — sub-prefix under existing arb:locks: root, distinct from perp-perp arb:locks:<sym> (T-08-08)
- [Phase 08]: Plan 08-03: DelistChecker interface added to internal/models (IsDelisted bool) to preserve D-02 boundary — *discovery.Scanner satisfies it
- [Phase 08]: Plan 08-03: pkg/exchange.BBO has no UpdatedAt — wall-clock gap between successive samples used as staleness gate instead of BBO timestamp
- [Phase 08]: Plan 04: preEntry composes 6 gates in fixed D-17 order; TestRiskGate_OrderingInvariant locks blame-attribution
- [Phase 08]: Plan 04: Gate-concentration cap (PG-RISK-01) only evaluates when the current candidate has a Gate leg; pre-existing gate positions can't retroactively block non-gate requests
- [Phase 08]: Plan 04: Fail-open on Redis disable-flag read error (Phase 03-02 precedent); WARN log + proceed to other 5 gates
- [Phase 08]: Idempotent startMonitor via atomic seq token (not reflect-based closure-pointer identity) — flaky under -race
- [Phase 08]: Optional vwapReader interface for exit PnL — production adapters skip, tests opt-in via stubExchange.GetOrderVwap
- [Phase 08]: Strict > 2x exec-quality rule (not >=) + divide-by-zero guard on mean(modeled)
- [Phase 08]: Conservative orphan policy: any err OR zero-total leg -> ExitReasonOrphan (prefer safety over re-enrolling ghost positions)

### Pending Todos

None yet. Next action: `/gsd-plan-phase 8` to decompose Phase 8 into executable plans.

### Blockers/Concerns

- `internal/pricegaptrader/` is a NEW module — strict boundary, must not import `internal/engine/` or `internal/spotengine/`. Plan-phase must enforce this at design time.
- Redis namespace `pg:*` must not collide with existing perp-perp or spot-futures keys.
- Startup wiring: tracker goroutine must respect `PriceGapEnabled` flag and shut down cleanly on SIGTERM like existing engines.
- Exchange adapter reuse only — no new adapter methods; if a method is missing, raise before adding.
- npm lockdown still in force — Phase 9 dashboard work uses existing Recharts/React stack only (`npm ci` only, no new deps).
- Live trading risk: every Phase 8 change lands behind `PriceGapEnabled=false`. No code path affecting perp-perp or spot-futures may be touched.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260408-ugr | deliveryDate-based delist detection | 2026-04-08 | 262ac8f | [260408-ugr-deliverydate-based-delist-detection](./quick/260408-ugr-deliverydate-based-delist-detection/) |
| 260415-34e | Cross-engine interference audit (21 findings: 7 HIGH / 9 MEDIUM / 5 LOW) | 2026-04-15 | d2518dc | [260415-34e-find-all-cross-engine-interference-that-](./quick/260415-34e-find-all-cross-engine-interference-that-/) |

## Session Continuity

Last session: 2026-04-21T09:48:40.096Z
Stopped at: Completed 08-06-PLAN.md
Resume file: None
Next command: `/gsd-plan-phase 8`
