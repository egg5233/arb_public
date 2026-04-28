# Architecture: v2.2 Auto-Discovery & Live Strategy 4 Integration

**Milestone:** v2.2 (extending existing `internal/pricegaptrader/` module)
**Researched:** 2026-04-27
**Confidence:** HIGH (existing code paths verified, integration points enumerated)

> Note: `.planning/research/ARCHITECTURE.md` retains the v2.0 platform-level architecture research. This document is the v2.2-specific delta — additive components only.

## Executive Summary

v2.2 ships **6 new components** that all live inside (or directly adjacent to) the existing `internal/pricegaptrader/` module. **No new top-level packages** are required, and the existing module-boundary rule (no imports of `internal/engine` or `internal/spotengine`) is preserved. Integration is achieved by:

1. Adding new sub-files to `internal/pricegaptrader/` (scanner, ramp, breaker, reconcile, telemetry, promotion).
2. Reusing the existing `Tracker` lifecycle as the goroutine host (existing errgroup).
3. Reusing `*config.Config.SaveJSON()` (already mutex-protected via `sync.RWMutex` at `config.go:70`) for `PriceGapCandidates` mutations.
4. Adding new `pg:scan:*`, `pg:promote:*`, `pg:ramp:*`, `pg:reconcile:*`, `pg:breaker:*` Redis keys (continues existing `pg:` namespace).
5. Hooking the existing `risk_gate.go` for ramp budget enforcement (no new gate file — keeps Wave-4 single-authorization-point invariant).
6. Reusing the existing `internal/api` WS hub for telemetry broadcast.

## Recommended Architecture

```
                   ┌─────────────────────────────────────────────────────────┐
                   │              internal/pricegaptrader/                   │
                   │  (existing module — strict boundary preserved)          │
                   │                                                         │
   tracker.go ────►│ Tracker.Run() goroutine — central lifecycle             │
   (existing)      │   ├── detector.go           (existing — 4-bar BBO)     │
                   │   ├── risk_gate.go ◄────────┐  (existing + extended)   │
                   │   ├── execution.go          │                           │
                   │   ├── monitor.go            │                           │
                   │   ├── rehydrate.go          │                           │
                   │   │                         │                           │
   NEW v2.2 ──────►│   ├── scanner.go            │  (PG-DISC-01)            │
                   │   ├── promotion.go ─────────┼──► cfg.SaveJSON()        │
                   │   ├── ramp.go ──────────────┘  (gates risk_gate)       │
                   │   ├── breaker.go ──────────────► cfg.PriceGapPaperMode │
                   │   ├── reconcile.go             (daily PnL job)         │
                   │   └── telemetry.go             (Redis + WS publisher)  │
                   └─────────────────────────────────────────────────────────┘
                              │                            │
                              ▼                            ▼
                  ┌─────────────────────┐    ┌──────────────────────────┐
                  │  Redis (DB 2)       │    │  internal/api WS hub     │
                  │  pg:scan:*          │    │  (BroadcastEvent)        │
                  │  pg:promote:*       │    │                          │
                  │  pg:ramp:*          │    │  Frontend tabs:          │
                  │  pg:reconcile:*     │    │  - Discovery (new)       │
                  │  pg:breaker:*       │    │  - PriceGap (existing)   │
                  └─────────────────────┘    └──────────────────────────┘
```

### Component Boundaries

| Component | New / Modified | Owner Module | Responsibility | Communicates With |
|-----------|----------------|--------------|----------------|-------------------|
| `scanner.go` | **NEW** | `internal/pricegaptrader` | Periodic market scan, score, surface candidates | `pkg/exchange` (read-only ticker/depth), Redis (`pg:scan:*`), `telemetry.go` |
| `promotion.go` | **NEW** | `internal/pricegaptrader` | Score → `cfg.PriceGapCandidates` mutation; Telegram + WS | `*config.Config.SaveJSON()`, `internal/notify`, `telemetry.go` |
| `ramp.go` | **NEW** | `internal/pricegaptrader` | Stage controller (100/500/1000 USDT), clean-day counter | Redis (`pg:ramp:*`), called by `risk_gate.go` |
| `breaker.go` | **NEW** | `internal/pricegaptrader` | Daily PnL drawdown circuit; flips `PriceGapPaperMode` | `pg:history`, `*config.Config.SaveJSON()`, `internal/notify` |
| `reconcile.go` | **NEW** | `internal/pricegaptrader` | Daily PnL roll-up job (00:05 UTC) | `pg:history` → `pg:reconcile:daily:YYYY-MM-DD` |
| `telemetry.go` | **NEW** | `internal/pricegaptrader` | Centralizes Redis writes + WS publish for v2.2 events | `internal/database`, `internal/api` (WS hub) |
| `risk_gate.go` | **MODIFIED** | `internal/pricegaptrader` | +1 gate: `gateRampBudget()` (consults `ramp.go`) | `ramp.go` |
| `tracker.go` | **MODIFIED** | `internal/pricegaptrader` | Spawns scanner + reconcile + breaker goroutines under existing `errgroup` | New components |
| `cmd/arb/main.go` (or `internal/app`) | **MODIFIED** | bootstrap | Pass scanner cadence + ramp config; **no startup-order shift** | Tracker constructor |
| `internal/api/handlers.go` | **MODIFIED** | `internal/api` | New routes: `GET /api/pg/scan/history`, `GET /api/pg/ramp/status`, `GET /api/pg/reconcile/daily` | WS hub |
| `web/` | **MODIFIED** | frontend | New "Discovery" sub-tab on PriceGap page; ramp status badge; breaker trip toast | WS subscriptions |

**Module-boundary check:** All NEW files live in `internal/pricegaptrader/`. None import `internal/engine` or `internal/spotengine`. `cfg.SaveJSON()` and `internal/notify` are already imported by the module today (per `tracker.go` + `notify.go`).

## Answers to Specific Questions

### 1. Where does the auto-discovery scanner live?

**Recommendation: `internal/pricegaptrader/scanner.go`** (new file inside existing module).

**Rationale:**
- `internal/discovery/` is owned by perp-perp engine; coupling it to pricegap would re-introduce the boundary that v2.0 D-02 intentionally drew.
- A separate `internal/pgdiscovery/` adds a third top-level module for a single goroutine that needs `Tracker`-internal types (`Candidate`, `ModeledSlipBps`, `pg:` Redis namespace). Net cost > benefit.
- Inside `internal/pricegaptrader/` the scanner can share types, errors, and `metrics.go` registries with detector/execution without exporting them.

**Pattern:** scanner is a goroutine launched by `Tracker.Run()` via the existing `errgroup`, gated by `cfg.PriceGapAutoDiscoveryEnabled` (default OFF). Cadence configurable (default 5 min). It runs **outside** the fixed-minute scan schedule used by perp-perp because pricegap doesn't share funding-window alignment — Bybit `:04–:05:30` blackout is honored by reading `cfg.BybitBlackoutEnabled` and skipping that window.

### 2. Concurrent CRUD safety for `cfg.PriceGapCandidates`

**Existing protections to reuse:**
- `*config.Config` has `sync.RWMutex mu` at `config.go:70` — already protects field reads/writes.
- `SaveJSON()` (`config.go:1572`) is the canonical persist path; v2.1 PG-OPS-08 added an absolute-path fallback that survives partial writes (`.bak` backup, `keepNonZero` tripwire at `:1519`).
- v2.1 Phase 10 dashboard CRUD already POSTs through `/api/config` which calls `SaveJSON()` under the same lock.

**Auto-promotion strategy (no new lock):**
1. `promotion.go` acquires `cfg.mu.Lock()` (or uses existing `cfg.UpdateField` helper if present).
2. Re-reads `cfg.PriceGapCandidates` under lock, applies dedupe (symbol+longExch+shortExch+direction tuple, including v0.35.0 bidirectional field), applies `PriceGapMaxCandidates` cap.
3. Within same critical section, calls `cfg.SaveJSON()`.
4. Releases lock, **then** publishes WS + Telegram (network I/O outside lock).

**Race scenario handled:** operator edits via dashboard while scanner promotes — last-write-wins is acceptable because both paths fully serialize through `cfg.mu` + `SaveJSON()`. Promotion is idempotent (dedupe by tuple), so a race that re-promotes a just-deleted candidate is recoverable. No data corruption.

**Active-position guard:** v2.1 Phase 10 added a guard that blocks delete/tuple-change when `pg:positions:active` matches. Auto-promotion **adds** entries — never modifies/removes — so the guard is irrelevant for the promotion path. Auto-demotion (score drop) MUST honor the same guard before removing.

### 3. Live ramp controller integration

**Recommendation: extend existing `risk_gate.go`** as a new gate (gate #7 in the deterministic D-17 order), not a new file.

**Rationale:** `risk_gate.go` already returns `GateDecision` with typed reasons; adding `GateRampExceeded` keeps the single-authorization-point invariant from v2.0 Wave-4. A separate pre-entry hook in `tracker.go` would create two authorization paths.

**Mechanism:**
- `ramp.go` exposes `CurrentBudget(ctx) (perLegUSD float64, err error)` reading `pg:ramp:state` (stage, clean-day counter, last-loss-day).
- `risk_gate.go` adds: if proposed leg notional > `CurrentBudget()`, return `GateDecision{Allow:false, Reason:GateRampExceeded}`.
- Clean-day advancement: `reconcile.go` increments `pg:ramp:clean_days` if daily PnL ≥ 0; resets to 0 on losing day. At `clean_days >= 7` and `stage==100`, advance to 500. Hard ceiling 1000 for v2.2.

### 4. Telemetry data flow

| Redis Key | Type | Writer | TTL | WS Channel |
|-----------|------|--------|-----|------------|
| `pg:scan:cycles` | LIST (rpush + ltrim 1000) | `scanner.go` | none (capped) | `pg:scan:cycle` |
| `pg:scan:scores:{symbol}` | ZSET (score, ts) | `scanner.go` | 7d | `pg:scan:score` (debounced) |
| `pg:scan:metrics` | HASH (last_run_at, candidates_seen, errors) | `scanner.go` | none | `pg:scan:metrics` (5s throttle) |
| `pg:promote:log` | LIST (rpush + ltrim 200) | `promotion.go` | none | `pg:promote:event` |
| `pg:ramp:state` | HASH (stage, clean_days, last_loss_day, advanced_at) | `ramp.go`, `reconcile.go` | none | `pg:ramp:state` |
| `pg:reconcile:daily:{YYYY-MM-DD}` | HASH (gross, fees, slippage, net, count) | `reconcile.go` | 90d | `pg:reconcile:daily` |
| `pg:breaker:trips` | LIST (rpush) | `breaker.go` | 90d | `pg:breaker:trip` |

**WS broadcast pattern:** reuse the existing `BroadcastEvent` hub used by tracker. Each WS message envelope: `{type, ts, payload}`. The frontend `Discovery` tab subscribes to `pg:scan:*` channels; the existing PriceGap tab subscribes to `pg:promote:*` + `pg:ramp:*` + `pg:breaker:*`.

### 5. Daily reconcile scheduling

**Recommendation: dedicated goroutine in `tracker.go`**, not integrated with the perp-perp fixed-minute schedule.

**Rationale:** the perp-perp schedule (`:10/:20/:30/...`) is funding-window-aligned and runs every minute. Reconcile only needs to fire once per day. Coupling adds noise to the scan dispatcher and crosses a module boundary.

**Pattern:**
```go
// Inside Tracker.Run():
group.Go(func() error { return reconcileDailyLoop(ctx, t.cfg, t.rdb, t.notify) })
```
Loop sleeps until next 00:05 UTC, runs reconcile, sleeps 24h. On startup, runs immediately if `pg:reconcile:daily:{yesterday}` is missing (catch-up). Idempotent via key existence check.

The `breaker.go` runs as a **lighter-weight** goroutine on a 5-minute tick (responsive to intra-day drawdown), reading the rolling `pg:history` window without waiting for daily reconcile.

### 6. Build order & dependencies

```
PG-DISC-01 Scanner ──┬──► PG-DISC-03 Telemetry (consumes scan output)
                     │
                     └──► PG-DISC-02 Auto-promotion (consumes scores)
                                 │
                                 └──► Strategy 4 Live Ramp (gates promoted candidates)
                                            │
                                            ├──► Drawdown Circuit Breaker
                                            └──► Daily PnL Reconcile (advances clean-day counter)
```

**Recommended phase ordering:**
1. **Phase 14 — PG-DISC-01 Scanner + PG-DISC-03 Telemetry** (built together; scanner is useless without telemetry to surface output). Default OFF. No live-trading risk.
2. **Phase 15 — PG-DISC-02 Auto-Promotion**. Adds first config-mutation path. Lock test required. Default OFF.
3. **Phase 16 — Daily Reconcile Job**. Pure read-only on `pg:history`. Prerequisite for ramp clean-day counter.
4. **Phase 17 — Live Capital Ramp Controller**. Depends on reconcile (clean-day source). Modifies `risk_gate.go`. **First v2.2 phase that touches live capital path.**
5. **Phase 18 — Drawdown Circuit Breaker**. Depends on reconcile data shape; flips paper mode. Build last so the rest is observable when it fires.
6. **Phase 19 — Paper-mode bug closure + tech-debt sweep**. Independent; can run in parallel with any of the above.

## Concurrency Invariants to Preserve

1. **Tracker single-owner rule** — All v2.2 goroutines launch from `Tracker.Run()` via the existing `errgroup`. Shutdown propagates via `ctx.Done()` (already wired).
2. **Config mutex** — All `cfg.PriceGapCandidates` and `cfg.PriceGapPaperMode` mutations go through `cfg.mu.Lock()` + `SaveJSON()`. No goroutine bypasses this path.
3. **Per-symbol Redis lock** — Existing `pg:lock:{symbol}` (SET NX + Lua) protects execution. Scanner is read-only and does NOT need this lock; promotion does NOT need it (mutates config, not positions).
4. **Active-position guard** — Auto-demotion (Phase 15) MUST check `pg:positions:active` set before removing a candidate, mirroring v2.1 Phase 10 dashboard guard.
5. **Startup ordering preserved** — `Scanner → RiskMon → HealthMon → API → Engine → SpotEngine → PriceGapTracker` unchanged. v2.2 components launch **inside** the existing PriceGapTracker boot, so the global order doesn't shift.
6. **Bybit blackout** — Scanner skips ticker reads on Bybit during `:04–:05:30` (read `cfg.BybitBlackoutEnabled`). Reconcile is daily and doesn't hit Bybit live.
7. **Live-engine isolation** — v2.2 must NOT add any import path from `internal/pricegaptrader` to `internal/engine` or `internal/spotengine`. The breaker's "flip to paper" is a config mutation only — engines read config, they aren't called directly.
8. **No new top-level startup component** — Avoids changing graceful-shutdown reverse-order logic.

## Anti-Patterns to Avoid

- **Don't** create `internal/pgdiscovery` as a sibling package — it forces export of types currently package-private to pricegaptrader.
- **Don't** schedule scanner in the perp-perp fixed-minute dispatcher — couples two unrelated lifecycles.
- **Don't** add a second config save path (e.g., direct `os.WriteFile`) — bypasses the v2.1 PG-OPS-08 absolute-path fallback + `.bak` discipline.
- **Don't** put ramp gating in `tracker.go` pre-execution — splits the authorization point that Wave-4 deliberately consolidated into `risk_gate.go`.
- **Don't** auto-flip `paper_mode=true` without Telegram alert + `pg:breaker:trips` audit row — operator must see why it tripped.
- **Don't** remove candidates while `pg:positions:active` row exists — mirrors v2.1 dashboard guard.
- **Don't** call `cfg.SaveJSON()` while holding `cfg.mu` if `SaveJSON` itself takes the lock — verify with current implementation; if it does, mutate fields, release lock, then SaveJSON (re-acquire internally).

## Scalability Considerations

| Concern | At v2.2 scale (~12 candidates, 1 server) | Future |
|---------|-----------------------------------------|--------|
| Scanner cycle time | 5 min × ~50 symbols × 6 exchanges ≈ ~300 ticker reads/cycle (well within rate limits) | Move to streaming WS depth if symbol set grows |
| `pg:scan:cycles` LIST | 1000-cap × 5 min cadence ≈ 3.5d retention | Migrate to time-series (Redis Streams or SQLite) |
| Promotion thrash | Capped by `PriceGapMaxCandidates` (default 12) | Add hysteresis (min hold-time before re-promote) |
| WS bandwidth | Throttle scan-metrics to 5s, debounce score updates | Per-tab subscription filters |

## Sources

- `.planning/PROJECT.md` (v2.2 milestone definition)
- `.planning/MILESTONES.md` (v2.0 + v2.1 history, deferred PG-DISC-01/02/03, Wave-4 gate design)
- `internal/pricegaptrader/` directory listing (30 files — detector/execution/monitor/risk_gate/rehydrate/notify/metrics/tracker/slippage)
- `internal/config/config.go:70` (`sync.RWMutex`), `:1519` (`keepNonZero` tripwire), `:1572` (`SaveJSON`), `:1879` (raw map preservation)
- `internal/pricegaptrader/*_test.go` showing existing pg: namespace (`pg:positions`, `pg:positions:active`, `pg:history`, `pg:lock:{symbol}`)
- v2.1 Phase 10 active-position guard pattern (per MILESTONES.md)
- v2.0 Wave-4 single-authorization-point design (per MILESTONES.md)
- v2.1 Phase 999.1 bidirectional candidate field (per MILESTONES.md — affects dedupe tuple)
