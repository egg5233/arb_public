# Phase 8: Price-Gap Tracker Core - Context

**Gathered:** 2026-04-21
**Status:** Ready for planning

<domain>
## Phase Boundary

New isolated subsystem `internal/pricegaptrader/` that detects cross-exchange price-gap events on a **static** candidate list (no scanner, no state machine) and opens/closes **delta-neutral** 2-leg IOC positions under hard risk gates. Entire subsystem is gated behind `PriceGapEnabled` (default OFF) with byte-for-byte isolation from `internal/engine/` and `internal/spotengine/`. Dashboard tab, paper mode, Telegram alerts, and rolling per-candidate metrics are **out of scope — Phase 9**.

Requirements covered: PG-01..05, PG-RISK-01..05, PG-OPS-06.

</domain>

<decisions>
## Implementation Decisions

### Module & Isolation
- **D-01:** New package `internal/pricegaptrader/` — naming matches ROADMAP Phase 8 ("tracker", not "engine") to signal lighter connotation vs Strategy 1/2 engines.
- **D-02:** Strict module boundary — no imports of `internal/engine/` or `internal/spotengine/`; may import `pkg/exchange/`, `pkg/utils/`, `internal/models/`, `internal/config/`, `internal/database/` (for its own Redis namespace), `internal/notify/` is Phase 9 only.
- **D-03:** Startup conditional on `PriceGapEnabled`, wired into `main.go` after SpotEngine init; graceful shutdown closes WS subscriptions and stops goroutines in reverse order. Open positions remain open across restart (rehydrated from Redis).

### Data Path (Kline + Freshness)
- **D-04:** Hybrid data path — REST poll 1m klines on both legs per candidate for spread calculation; subscribe to existing WS last-trade/ticker streams on both legs as a **freshness + halt heartbeat** (`<90s` gate per PG-RISK-02). The REST call provides the canonical 1m close used for spread; WS heartbeat detects stale markets independently of REST polling lag.
- **D-05:** Poll cadence: every 30s (tracker tick). Rate-limit budget for 5 candidates × 4 exchanges × 2 (per-minute calls) = ~40 req/min, well under any per-exchange cap.
- **D-06:** Spread formula: `(price_A − price_B) / mid × 10_000 bps` (matches Phase 0 methodology, `STRATEGY_DESIGN.md §3.1`). Mid = `(price_A + price_B) / 2`.

### Event Detection
- **D-07:** Event fires when `|spread| ≥ T` for **≥4 consecutive 1m bars** (PG-01). 4-bar window is in-memory per candidate; **resets on process restart** (PG-relevant state in Redis is positions only, not bar history).
- **D-08:** Candidate config schema is **direction-pinned** per tuple — e.g. `{symbol: "SOON", long_exch: "gate", short_exch: "binance", threshold_bps: 200, max_position_usdt: 5000}`. Event fires only in the configured direction; reverse dislocations are ignored (matches Phase 0/1 measured edge). Operators who want both directions on the same pair configure two candidate entries.

### Entry Execution
- **D-09:** Simultaneous IOC market orders on both legs via `pkg/exchange/` adapters (PG-02). Use existing adapter `PlaceOrder(PlaceOrderParams{...})` with `TimeInForce: IOC`.
- **D-10:** **Partial-fill reconciliation = unwind the over-filled leg down to match the smaller fill** (MVP option 1). Simpler than `engine.go:3022 retrySecondLeg` pattern; accepts a smaller-than-intended position; guarantees post-entry delta-neutrality. If one leg fills zero, close the other immediately at market. Circuit-breaker: 5 consecutive `PlaceOrder` failures pauses the tracker (log WARN; operator intervention required).

### Exit Execution
- **D-11:** Exit conditions (either fires exit) per PG-03: `|spread| ≤ T/2` **OR** 4h max-hold timer elapses.
- **D-12:** Exit uses simultaneous IOC market orders to close both legs. On partial exit fill, retry remainder as market order — positions must close fully (mirrors existing "positions must close fully" principle from CLAUDE.local.md error-handling rules).
- **D-13:** Exit reason stamped on closed position: `reverted` | `max_hold` | `manual` | `risk_gate` | `exec_quality` (supports analytics + PG-VAL-01 in Phase 9).

### Persistence (Redis)
- **D-14:** Redis key namespace: `pg:` prefix under existing Redis DB 2.
  - `pg:pos:{id}` — per-position JSON (PG-04)
  - `pg:positions:active` — SET of active position IDs
  - `pg:history` — closed position log (LIST, capped ~500 entries matching spot engine pattern)
  - `pg:candidate:disabled:{symbol}` — exec-quality disable flag (D-19)
  - `pg:slippage:{candidate_id}` — rolling list of last 10 trades' realized-vs-modeled slippage (supports PG-RISK-03)
- **D-15:** Position ID: `pg_{symbol}_{exchA}_{exchB}_{unix_nano}` (follows spot engine naming convention — readable, unique per-process, no UUID dependency).
- **D-16:** Positions rehydrate on startup — monitor loop picks up active positions from Redis and resumes spread/time tracking.

### Risk Gates (pre-entry, all must pass)
- **D-17:** Deterministic pre-entry checks (PG-RISK-01..05):
  1. **Gate concentration cap (PG-RISK-01)** — sum of requested notional across positions with a Gate leg ≤ 50% of `PriceGapBudget`. Enforced on **pre-trade requested notional**, not post-fill (deterministic math).
  2. **Denylist / halt / staleness (PG-RISK-02)** — both legs must have: no delist flag (poll exchange info per-exchange; cache 5 min, matches existing discovery scanner pattern), trading status = normal, last kline < 90s old.
  3. **Max concurrent positions (PG-RISK-04)** — tracker-wide open count < 3.
  4. **Per-position notional cap (PG-RISK-05)** — per-candidate from config (`max_position_usdt` field).
  5. **Budget remaining** — sum of all open requested notionals + this candidate's requested ≤ `PriceGapBudget`.
- **D-18:** **Margin mode = cross** — positions share the exchange-level cross-margin pool with existing perp-perp engine. No new margin-mode code path. Isolation is via the **per-candidate + concentration caps + budget cap**, not via exchange margin mode.

### Exec-Quality Override (PG-RISK-03)
- **D-19:** After 10 closed trades on a candidate, if `mean(realized_slippage_bps) > 2 × mean(modeled_slippage_bps)`, auto-set `pg:candidate:disabled:{symbol}` = `1` with reason + timestamp. Subsequent entries on that candidate are blocked with exit reason `exec_quality`.
- **D-20:** **Re-enable path (pre-Phase 9)** — ship a small `cmd/pg-admin/` helper binary in Phase 8 that supports `pg-admin enable <symbol>` / `pg-admin disable <symbol>` / `pg-admin status`, operating directly on Redis. Phase 9 replaces this with a dashboard toggle. Hand-editing `config.json` is explicitly forbidden (CLAUDE.local.md rule).

### Slippage Modeling
- **D-21:** Modeled slippage per candidate = static value in config (seeded from `/tmp/phase0-pricegap/edge_v2.json` Phase 1 measurements). Realized slippage = `(actual_fill_vwap − mid_at_decision) / mid × 10_000 bps` per leg, summed for the round-trip. Stored in `pg:slippage:{candidate_id}` as JSON entries.

### Config Surface (config.json, defaults OFF/zero)
- **D-22:** New top-level fields:
  - `PriceGapEnabled` (bool, default `false`) — master switch (PG-OPS-06)
  - `PriceGapBudget` (float, default `5000` USDT for MVP)
  - `PriceGapMaxConcurrent` (int, default `3`)
  - `PriceGapGateConcentrationPct` (float, default `0.50`)
  - `PriceGapMaxHoldMin` (int, default `240` — 4h)
  - `PriceGapExitReversionFactor` (float, default `0.5` — i.e. exit at T/2)
  - `PriceGapBarPersistence` (int, default `4`)
  - `PriceGapKlineStalenessSec` (int, default `90`)
  - `PriceGapPollIntervalSec` (int, default `30`)
  - `PriceGapCandidates` — slice of `PriceGapCandidate{Symbol, LongExch, ShortExch, ThresholdBps, MaxPositionUSDT, ModeledSlippageBps}`
- **D-23:** All values reload via existing `/api/config` POST path (dashboard hook in Phase 9; Phase 8 reads from config.json at startup). Adding fields to `internal/config/config.go` struct follows existing SpotFutures* field pattern (lines 183+).

### Claude's Discretion
- Exact goroutine topology inside `pricegaptrader/` (single-goroutine event loop vs worker pool) — planner picks based on execution profile; 5 candidates × 2 exchanges is low enough that a single tick goroutine plus per-position monitor goroutines is fine.
- Telemetry / log-line formats beyond the `ExitReason` enum (structured logging follows `pkg/utils/` conventions).
- Internal interface shape for injecting the exchange registry into the tracker (follow `models.Discoverer` / `models.StateStore` DI pattern — planner decides exact names).
- `cmd/pg-admin/` CLI UX (flag names, output format) — follow `cmd/*` patterns already in the repo.
- Kline endpoint naming per adapter (REST kline method already exists in some adapters; planner audits per-adapter).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Strategy design
- `/tmp/phase0-pricegap/STRATEGY_DESIGN.md` — Full design doc; §3.5 (Phase 1 shortlist), §8 (MVP rollout), §9 (integration), §10 (open questions — note Q3/Q4/Q5 answered in this CONTEXT)
- `/tmp/phase0-pricegap/edge_v2.json` — Phase 1 depth/slippage measurements (seed values for `ModeledSlippageBps` per candidate)
- `/tmp/phase0-pricegap/depth_snapshot.json` — Order-book snapshots used to compute Phase 1 slippage

### Project-level constraints
- `/var/solana/data/arb/CLAUDE.local.md` — Module-boundary rules (`internal/pricegaptrader/` may not import `internal/engine/` or `internal/spotengine/`), new-feature rollout pattern (config switch + dashboard toggle + `config.json` persistence), "never modify config.json" rule, delegation mode
- `/var/solana/data/arb/.planning/PROJECT.md` — Project vision, core value statement
- `/var/solana/data/arb/.planning/REQUIREMENTS.md` — PG-01..05, PG-RISK-01..05, PG-OPS-06 definitions
- `/var/solana/data/arb/.planning/ROADMAP.md` §"Phase 8" — goal + 5 success criteria

### Codebase references (patterns to mirror)
- `internal/spotengine/engine.go` — package layout for an isolated parallel engine; `NewSpotEngine(...)` constructor pattern (line 82)
- `internal/engine/engine.go:3167+` — existing simultaneous-IOC entry logic (reference only — we implement a simpler variant in pricegaptrader)
- `internal/engine/engine.go:3022 retrySecondLeg` — existing partial-fill recovery (reference only — we use simpler unwind-to-match in Phase 8)
- `internal/database/spot_state.go` — Redis persistence pattern (HSET + active SET + history LIST + TTL prefix keys) to mirror for `pg:*` namespace
- `internal/database/state.go:22-32` — existing `arb:*` key constants (our namespace is `pg:` distinct)
- `internal/risk/health.go` — HealthMonitor L0–L5 tiers + HealthAction channel (not extended in Phase 8; cross-margin means existing HealthMonitor already covers tracker exposure at exchange level)
- `internal/config/config.go:183+` — `SpotFutures*` field pattern to follow for new `PriceGap*` fields
- `pkg/exchange/types.go` — `PlaceOrderParams`, IOC enum, `ErrRepayBlackout`, error types
- `pkg/exchange/exchange.go` — 35-method `Exchange` interface (verify existence of per-adapter REST kline method during planning)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pkg/exchange/` 35-method interface — IOC market orders, ticker, kline REST (audit per-adapter), symbol info for delist/halt checks
- `pkg/utils/` — `RoundToStep`, `EstimateSlippage`, `RateToBpsPerHour`, structured logger
- `internal/database/Client` — add new methods for `pg:*` keys alongside existing `arb:*` + `arb:spot_*` keys; reuse Redis pipeline pattern from `spot_state.go`
- `internal/notify/notifier.go` — Telegram notifier is safe on nil receiver; Phase 9 wires this in. Phase 8 does not send Telegram.
- `internal/database/locks.go` — per-symbol Redis distributed locks (`arb:locks:<resource>`); pricegaptrader can reuse for entry/exit serialization using a `pg:` sub-prefix (e.g. `arb:locks:pg:<symbol>`)
- Exchange adapter symbol mapping (Gate.io `BTC_USDT`, OKX `BTC-USDT-SWAP`) — internal format stays `BTCUSDT`; existing adapters handle conversion

### Established Patterns
- **Interface-driven DI** (`models.Discoverer`, `models.StateStore`, `models.RiskChecker`) — define a `models.PriceGapStore` interface so tests can inject a fake Redis
- **Feature rollout trio** — config switch default OFF + dashboard toggle + config.json persistence (dashboard in Phase 9; Phase 8 lands switch + persistence)
- **Redis namespacing** — all keys prefixed by subsystem (`arb:`, `arb:spot_`, now `pg:`)
- **Graceful shutdown** — reverse startup order; close WS, stop goroutines, flush Redis
- **Error sentinel pattern** — `pkg/exchange/types.go` `ErrRepayBlackout` style; consider `ErrPriceGapDisabled`, `ErrPriceGapConcentrationCap` for typed gate denials

### Integration Points
- `main.go` — new conditional block after spotengine, guarded on `cfg.PriceGapEnabled`; pass `exchanges`, `db`, `cfg` into `pricegaptrader.NewTracker(...)`
- `internal/config/config.go` — extend Config struct with `PriceGap*` fields (line 183+ style); extend JSON load / defaults
- `internal/database/` — new file `pricegap_state.go` with `pg:*` key constants + CRUD methods
- `cmd/pg-admin/` — new CLI binary (trivial: reads Redis, toggles disable flags); matches existing `cmd/*` pattern
- HTTP API (Phase 9 adds routes) — Phase 8 may stub `/api/pricegap/health` returning `{enabled, budget, open_positions}` for smoke-test during live rollout; planner decides if this ships in Phase 8 or waits

</code_context>

<specifics>
## Specific Ideas

- Seed the candidate list with the 5 shortlist rows from `STRATEGY_DESIGN.md §3.5` (SOON, SKYAI×3, DRIFT) — each gets its `ModeledSlippageBps` from `edge_v2.json`.
- SOON binance-gate is ~80% of expected portfolio edge — first correctness bar is "SOON trades correctly end-to-end".
- The existing perp-perp engine's simultaneous-IOC-with-slippage-ceiling pattern (`engine.go:3167+`) is intentionally NOT reused — price-gap tracker uses simpler IOC-market-both-legs + unwind-to-match. This is a deliberate "tracker not engine" choice per design doc §8.
- Keep module dependency surface minimal — if something is only needed for Phase 9 (dashboard, telegram, paper mode), do not stub it in Phase 8.

</specifics>

<deferred>
## Deferred Ideas

- **Rolling-spread scanner** (PG-D-01) — not needed for 5-candidate MVP
- **Candidate state machine** (`watch`/`eligible`/`approved`/`cooldown`/`blocked`) (PG-D-02) — replaced in MVP by simple enable/disable flag
- **Auto-promotion** — human-controlled via `pg-admin` / Phase 9 dashboard
- **Correlation / regime-cluster cap** (PG-D-04) — deferred; single Gate-concentration cap suffices for 5 candidates
- **OKX + BingX inclusion** (PG-D-05) — deferred to v2.1
- **Shared capital allocator integration** (PG-D-06) — separate budget in v2.0
- **Funding-cost integration during hold** — 4h max-hold avoids funding cycles in common case; revisit only if candidate half-lives lengthen
- **Persistent bar-history across restart** — rejected for MVP (in-memory reset is safer)
- **Dashboard / paper mode / Telegram alerts / rolling bps metrics** — explicitly Phase 9 (PG-OPS-01..05, PG-VAL-01..02)
- **Cross-strategy HealthMonitor coupling** — cross-margin means existing L0–L5 monitor already observes tracker exposure at exchange level; no new health tiers

</deferred>

---

*Phase: 08-price-gap-tracker-core*
*Context gathered: 2026-04-21*
