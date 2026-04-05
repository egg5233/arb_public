---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: executing
stopped_at: Completed 06-01-PLAN.md
last_updated: "2026-04-05T14:14:23.333Z"
last_activity: 2026-04-05
progress:
  total_phases: 6
  completed_phases: 5
  total_plans: 20
  completed_plans: 17
  percent: 85
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-01)

**Core value:** "I deposit USDT, select my risk preference, and the system automatically finds opportunities across both strategies, opens positions, collects funding, exits when profitable, and I can see exactly how much each position earned."
**Current focus:** Phase 06 — spot-futures-risk-hardening

## Current Position

Phase: 06 (spot-futures-risk-hardening) — EXECUTING
Plan: 2 of 4
Status: Ready to execute
Last activity: 2026-04-05

Progress: [██████████] 100%

## Performance Metrics

**Velocity:**

- Total plans completed: 16
- Average duration: 14 min
- Total execution time: 0.7 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 03 | 3 | 41min | 14min |

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

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

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

### Pending Todos

None yet.

### Blockers/Concerns

- Each remaining exchange (Binance, Gate.io, Bitget, OKX) will have unique margin API quirks -- budget for 3-5 adapter bugs per exchange (v0.22.44-49 precedent)
- npm lockfile update process needed before Phase 4 frontend work (charting libraries)

## Session Continuity

Last session: 2026-04-05T14:14:23.329Z
Stopped at: Completed 06-01-PLAN.md
Resume file: None
