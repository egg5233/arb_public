# Milestones

## v2.1 Candidate Operations (Shipped: 2026-04-27)

**Phases completed:** 3 phases (10, 13, 999.1), 11 plans, 6 tasks
**Releases:** v0.34.10 (PG-OPS-08), v0.34.11 (PG-VAL-03 + PG-DEBT-01), v0.35.0 (Phase 999.1)

**Key accomplishments:**

- **Phase 10 — Dashboard Candidate CRUD (PG-OPS-07):** Operators can Add/Edit/Delete `PriceGapCandidate` entries from the Price-Gap dashboard tab. POST /api/config validates server-side (regex symbol, exchange enum, range checks, duplicate-tuple) with an active-position safety guard that blocks delete OR tuple-change edits when a `pg:positions:active` row matches the removed tuple. EN + zh-TW i18n in lockstep. 17 Node-native frontend tests + 2 Go tests pin modal CRUD, PG-OPS-08 carryover invariant, i18n parity, and tracker hot-reload — zero new npm deps (lockdown honoured).
- **Phase 13 — v2.0 Deferred Closure:** PG-VAL-03 paper-mode realized slip override → ModeledSlipBps (v0.34.11, commit d144c54). PG-OPS-08 dashboard auto-POST guard via SaveJSON absolute-path fallback (v0.34.10). PG-DEBT-01 `cmd/bingxprobe/` deleted (v0.34.11; case-insensitive JSON decode bug it diagnosed already fixed in v0.34.6). All three closed via direct commits, no GSD phase directory.
- **Phase 999.1 — Bidirectional pricegap candidates (PG-DIR-01, backlog promotion):** `direction: "pinned" | "bidirectional"` field added to `PriceGapCandidate`. Pinned now sign-filters (closes latent bug — `barRing.allExceed` previously used `math.Abs` and silently fired wrong-side trades on inverse spreads — BREAKING note in CHANGELOG). Bidirectional fires on either sign with executor leg-role swap; observability fields `FiredDirection` + `CandidateLongExch` + `CandidateShortExch` persisted on positions. Dashboard modal Direction radio + 5 i18n keys (EN + zh-TW lockstep). 14 detector/executor TDD tests + 3 E2E paper-mode + 5 i18n parity tests.

### Known Gaps (deferred to v2.2)

- **PG-DISC-01** — Auto-discovery scanner (Phase 11) — never built
- **PG-DISC-03** — Discovered candidates Redis audit trail (Phase 11) — never built
- **PG-DISC-02** — Auto-promotion (Phase 12) — never built

These are the headline "auto-discovery" half of "Candidate Operations". Closed as tech debt mirroring v1.0 precedent (22/24 reqs); `/gsd-audit-milestone v2.1` flagged the gap formally and `.planning/milestones/v2.1-MILESTONE-AUDIT.md` records the rationale. v2.2 "Auto-Discovery & Promotion" is in ROADMAP.md as Planned.

---

## v2.0 Multi-Strategy Expansion (Shipped: 2026-04-25)

**Phases completed:** 2 phases, 19 plans, 35 tasks

**Key accomplishments:**

- [Rule 1 — Fix] Plan sample code used `err.Error() == "redis: nil"` for missing-key detection.
- Wave-3 core: internal/pricegaptrader package established with Tracker lifecycle + 4-bar persistence detector. PG-01 event detection fires only when four consecutive same-sign 1m bars exceed the per-candidate threshold and BBO samples are fresh. Module boundary (D-02) enforced; no imports of internal/engine or internal/spotengine.
- Wave-4 gatekeeper between detector and execution: 6 deterministic gates composed in D-17 order, returning the first failure as a typed GateDecision. This is the sole authorization check Plan 05 will call before any adapter PlaceOrder.
- Wave-5 live-trading-critical path: concurrent IOC market orders on both legs, D-10 unwind-to-match partial-fill reconciliation via MARKET (not IOC), zero-fill-on-one-leg → immediate ReduceOnly close of the other, 5-consecutive-failure circuit breaker, per-symbol Redis lock, and D-15 position-ID format. 8 tests cover every failure mode the threat model enumerates.
- Wave-6 completes Strategy-4 MVP lifecycle: per-position exit monitor (PG-03 D-11/D-12), exec-quality auto-disable rolling window (PG-RISK-03 D-19), and startup rehydration with orphan detection (PG-04). Exit fires on |spread| ≤ T×reversionFactor or now−OpenedAt ≥ maxHold; both legs closed via simultaneous IOC ReduceOnly with MARKET ReduceOnly remainder retry. Realized PnL + slippage persisted per D-21. After 10 closed trades, if mean realized > 2× mean modeled the candidate auto-disables via SetCandidateDisabled. Startup rehydrate orphans zero-leg positions and re-enrolls survivors; idempotent via atomic seq token so double-rehydrate is safe under -race.
- None.
- 1. [Rule 3 — Blocking signature mismatch] config/database constructor signatures differ from plan sketch
- One-liner:
- One-liner:
- One-liner:
- One-liner:
- One-liner:
- One-liner:
- 1. [Rule 2 — Security/Correctness] Wired `DebugLog` through API layer
- 1. [Process deviation — documented]
- PASS — Phase 9 UAT signed off 2026-04-25.

---

## v1.0 Unified Arb Platform (Shipped: 2026-04-21)

**Phases completed:** 7 phases, 21 plans, 38 tasks

**Key accomplishments:**

- Spot vs Margin Routing (critical)
- Gate.io 28/28 and OKX 27/28 livetest pass; cross-margin spot orders handle borrow/repay implicitly in OKX Futures mode via tdMode=cross + ccy=USDT
- Native Loris-based scanner replacing CoinGlass Chrome scraper, with net yield calculation (fundingAPR - borrowAPR - feeAPR) and 9 new config fields for Phase 2 exit/entry guards
- Exit guards (min-hold, settlement window, spread gate) and entry basis gating protecting spot-futures automation pipeline, with emergency-first trigger priority ordering
- Extended spot config API with 9 new fields, added dashboard toggles with conditional dimming, i18n in both locales, source indicator on Opportunities page, and auto-entry pipeline unit test confirming SF-05 wiring
- 4 Telegram notification methods with per-event-type cooldown wired into perp-perp engine for SL triggers, L4/L5 emergency closes, and consecutive API errors
- Rolling-window loss limit system using Redis sorted sets with 24h/7d PnL tracking, pre-entry gate in executeArbitrage, and WebSocket broadcast to dashboard
- Full 6-touch-point config for 5 safety fields, Dashboard Safety tab with loss limit/Telegram toggles, Overview loss limit banner with 3-state color, and version bump to 0.26.0
- SQLite time-series store with WAL mode and aggregator functions for APR, win rate, per-exchange metrics, and strategy comparison -- all backed by 10 passing tests
- Recharts 3.8.1 and react-is 19.2.4 added to frontend via audited lockfile update with zero axios contamination
- EnableAnalytics 6-touch config toggle, dual API endpoints for PnL history and summaries, SnapshotWriter with buffered non-blocking channel, PnL decomposition (ExitFees/BasisGainLoss) in reconcilePnL, BBO slippage capture at entry
- Analytics dashboard with Recharts charts (cumulative PnL, strategy comparison, exchange metrics), History PnL drill-down, config toggle, sidebar nav, and full i18n support
- Risk profile presets, unified capital config (7 fields), performance-weighted allocation algorithm, derived capital-per-leg, and dynamic strategy shifting with 22 new tests
- Unified capital allocation wired into perp-perp engine, spot-futures engine, risk manager sizing, scan-cycle APR computation, and REST API with profile handling
- Allocation tab in Config (profile selector, unified capital toggle, bounds) and Overview card (pool total, strategy splits, exchange exposure) with full i18n and version 0.27.0
- Per-contract maintenance margin rates from 5 exchange adapters with tiered tier-matching, TTL cache, conservative fallback, and 6-touch-point config
- Maintenance rate-aware survivable-drop check as risk gate check 6, preventing GUAUSDT-like overleveraged entries at 3x/30% maintenance (3.3% survivable < 30% threshold)
- Maintenance-rate-aware liquidation distance trigger with graduated emergency/exit/warn response, and health monitor extended to include spot-futures positions in L4/L5 actions with cross-engine dispatch to SpotEngine
- MaintenanceRate field flows from exchange adapters through native/CoinGlass discovery to dashboard opportunities table with color-coded risk levels (red/amber/gray), display only per D-15
- Wired 3 maintenance gate config fields (toggle, default rate, cache TTL) through dashboard API and Config.tsx sf-general tab with server-side validation and round-trip test

---
