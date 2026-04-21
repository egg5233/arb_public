# Phase 2: Spot-Futures Automation - Context

**Gathered:** 2026-04-02
**Status:** Ready for planning

<domain>
## Phase Boundary

The spot-futures engine autonomously discovers, opens, and exits positions without manual intervention. This phase takes the existing manual-trigger system (Phase 1) and wires up the full automation pipeline: native discovery, auto-entry, auto-exit with edge case handling, and basis/spread gating.

</domain>

<decisions>
## Implementation Decisions

### Discovery Pipeline (SF-04)
- **D-01:** Build a native discovery scanner using Loris API (funding rates) + exchange borrow rate APIs (already available via `getCachedBorrowRate`). This replaces CoinGlass Chrome scraper as the primary source.
- **D-02:** Keep CoinGlass scraper as an optional fallback data source, not the primary. The native scanner is more reliable (no headless Chrome dependency, no page structure breakage risk).
- **D-03:** Net yield calculation: `funding APR - borrow APR - trading fees`. Rank opportunities by net yield descending across all 5 exchanges.

### Auto-Open Pipeline (SF-05)
- **D-04:** Reuse existing `attemptAutoEntries()` → `checkRiskGate()` → `ManualOpen()` path. The code is already built and wired — Phase 2 enables and hardens it.
- **D-05:** One entry per scan cycle (existing sequential design). Prevents over-committing before Redis reflects the new position.

### Auto-Exit Edge Cases (SF-06)
- **D-06:** Add configurable min-hold gate (don't exit before first funding settlement). Reasonable default, dashboard toggle.
- **D-07:** Add configurable settlement-window guard (skip exit evaluation during funding settlement periods when rates are temporarily unreliable). Reasonable default, dashboard toggle.
- **D-08:** Existing edge case handling stays: blackout windows (Bybit :04-:05:30), pending repay retry, stuck exit escalation to emergency after 5 retries.

### Basis/Spread Gating (SF-07)
- **D-09:** All gating thresholds are configurable with reasonable defaults and dashboard toggles. This follows the project convention (`feedback_risk_configurable_switch.md`).
- **D-10:** Entry basis/spread check: reject when spot-futures basis is too wide. Percentage-based threshold (configurable).
- **D-11:** Exit spread gate: threshold-gated close so exits don't fire at unfavorable spread.

### Go-Live Strategy
- **D-12:** Simple flip approach. Deploy → enable dry-run → observe logs → disable dry-run → live. Uses existing config knobs: `auto_enabled`, `dry_run`, `max_positions`. No new code needed for the transition itself.
- **D-13:** Graduated ramp via existing `MaxPositions=1` → increase after first successful trade cycle.

### Claude's Discretion
- Specific threshold default values for basis/spread gating, min-hold duration, settlement window timing
- Native scanner polling interval and data merging strategy with CoinGlass fallback
- Implementation ordering of plans (discovery first vs exit safeguards first)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Spot Engine Core
- `internal/spotengine/engine.go` — `discoveryLoop()` (line 130), `attemptAutoEntries()` called from here, `launchExit()` (line 121)
- `internal/spotengine/discovery.go` — `runDiscoveryScan()` (line 69), `getCachedBorrowRate()` (line 234), CoinGlass Redis reader
- `internal/spotengine/autoentry.go` — `attemptAutoEntries()` (line 10), the auto-entry hook
- `internal/spotengine/exit_manager.go` — `checkExitTriggers()` (line 22), `initiateExit()` (line 260), `completeExit()` (line 392)
- `internal/spotengine/monitor.go` — `monitorLoop()` (line 15), `monitorPosition()` (line 99)
- `internal/spotengine/risk_gate.go` — `checkRiskGate()` (line 18), 5 gate checks
- `internal/spotengine/rate_velocity.go` — `RateVelocityDetector`, borrow spike detection
- `internal/spotengine/capital.go` — Capital reservation/release helpers
- `internal/spotengine/execution.go` — `ManualOpen()` (line 62), used by both dashboard and auto-entry

### Data Sources
- `internal/scraper/spotarb.go` — CoinGlass Chrome scraper, writes to Redis `coinGlassSpotArb`
- Loris API (`https://api.loris.tools/funding`) — funding rate source used by perp-perp discovery

### Perp-Perp Reference Patterns
- `internal/discovery/scanner.go` — Perp-perp discovery loop pattern (clock-aligned scans, typed scan cycles)
- `internal/engine/exit.go` — `checkExitsV2()` (line 140), min-hold gate, settlement window guard patterns to adapt

### Config
- `internal/config/config.go` — All `SpotFutures*` fields (lines 176-201), especially `SpotFuturesAutoEnabled`, `SpotFuturesDryRun`, `SpotFuturesPersistenceScans`

### Dashboard
- `internal/api/spot_handlers.go` — Spot-futures API endpoints, `handleSpotAutoConfig` (line 261)
- `web/src/pages/Config.tsx` — Spot-futures config tabs (sf-general, sf-sizing, sf-discovery, sf-exit)

### Phase 1 Context
- `.planning/phases/01-spot-futures-exchange-expansion/01-CONTEXT.md` — Prior decisions, per-exchange complexity notes

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **Full auto-entry pipeline**: `attemptAutoEntries()` → `checkRiskGate()` → `ManualOpen()` — already built, just needs enabling
- **Full auto-exit pipeline**: `monitorPosition()` → `checkExitTriggers()` → `launchExit()` → `initiateExit()` → `completeExit()` — fully automated
- **Borrow rate fetching**: `getCachedBorrowRate()` (5-min cache) and `getFreshBorrowRate()` (live) per exchange
- **Persistence counters**: Redis-backed `IncrSpotPersistence()` / `GetSpotPersistence()` for consecutive-scan gating
- **Cooldown system**: Redis TTL keys for post-loss reentry blackout
- **Rate velocity detector**: Ring buffer borrow rate spike detection (disabled by default)
- **Loris API client**: Already used by perp-perp discovery for funding rates

### Established Patterns
- Config fields: `Enable{Feature}` bool (default OFF) + JSON tag + dashboard toggle
- Discovery: polling loop with configurable interval, results pushed to dashboard via WebSocket
- Exit triggers: priority-ordered evaluation, emergency flag escalation
- Redis state: position CRUD, distributed locks, TTL-based cooldowns

### Integration Points
- `discoveryLoop()` in `engine.go` — where native scanner replaces/augments CoinGlass reader
- `runDiscoveryScan()` in `discovery.go` — where data source switching happens
- `checkExitTriggers()` in `exit_manager.go` — where min-hold and settlement guards insert
- `handleSpotAutoConfig` in `spot_handlers.go` — where new config fields get exposed to dashboard
- Config.tsx spot-futures tabs — where new dashboard toggles go

</code_context>

<specifics>
## Specific Ideas

- The native scanner should calculate the same `SpotArbOpportunity` struct that CoinGlass produces, so downstream code (filtering, ranking, display) needs zero changes
- Borrow rate APIs are already per-exchange — the native scanner just needs to iterate all symbols with margin support on each exchange
- For the CoinGlass fallback: if native scanner has data, use it; if stale/empty, fall back to CoinGlass Redis key (same staleness check that exists today)

</specifics>

<deferred>
## Deferred Ideas

- **Cross-exchange spot-futures** (borrow on X, trade on Y) — v2 requirement (XSF-01, XSF-02, XSF-03), fundamentally different architecture
- **Automated dry-run validation** (track "would-have" entries and compare profitability) — nice-to-have, not needed for Phase 2

</deferred>

---

*Phase: 02-spot-futures-automation*
*Context gathered: 2026-04-02*
