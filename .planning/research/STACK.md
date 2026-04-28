# Technology Stack — v2.2 Auto-Discovery & Live Strategy 4

**Project:** Arb Bot v2.2
**Researched:** 2026-04-27
**Mode:** Subsequent milestone — additive only
**Overall confidence:** HIGH

> Supersedes the 2026-04-01 stack note (which scoped a different milestone). v2.2 is purely additive on top of v2.1 deps.

## TL;DR — Zero New Dependencies Required

**v2.2 can ship with the existing dependency set in `go.mod` and `web/package.json`.** All three feature clusters (auto-discovery scanner, live capital ramp + reconcile + drawdown, discovery telemetry) compose from primitives already in the codebase: `pkg/exchange.Exchange.GetSpotBBO`, `internal/pricegaptrader` BBO polling loop, `redis/go-redis/v9` v9.18.0, `modernc.org/sqlite` v1.48.1, `internal/notify` Telegram, Recharts 3.8.1.

The npm lockdown is honoured automatically — no frontend additions needed. All telemetry rendering uses Recharts components already shipped in v1.0 Phase 4. **No Context7 verification needed** because no version bumps or new packages are proposed.

## Recommended Stack (Existing → Reused)

### Core Framework — No Changes

| Technology | Version | Purpose in v2.2 | Why |
|------------|---------|-----------------|-----|
| Go | 1.26 (`go.mod`) | Scanner goroutine, ramp controller, reconcile job | Stdlib `time.Ticker`, `context`, `sync` cover all concurrency needs |
| `pkg/exchange` (internal) | live | BBO polling for scanner | `GetSpotBBO(symbol) (BBO, error)` already on all 6 adapters; no new interface methods |
| `internal/pricegaptrader` | v0.34.x → v0.35.x | Live capital ramp host module | Detector + executor + monitor + tracker pre-built; ramp is a budget wrapper around existing entry path |

### Database / State — No Changes

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `github.com/redis/go-redis/v9` | v9.18.0 | Discovery telemetry, candidate score history, ramp state | Already used; Redis DB 2 is sole state layer per project rule. ZADD/ZRANGEBYSCORE/EVAL are stable core API |
| `modernc.org/sqlite` | v1.48.1 | Daily PnL reconcile aggregation reads | Already wired (v1.0 AN-04); reconcile job *reads* analytics DB and Redis closed-trade history, *writes* a daily summary row |
| `github.com/alicebob/miniredis/v2` | v2.37.0 | Test-time Redis fake for new scanner/ramp tests | Already used by existing pricegaptrader tests |

### Notifications — No Changes

| Technology | Path | Purpose | Why |
|------------|------|---------|-----|
| `internal/notify` | live | Per-fill Telegram alerts (Strategy 4 live), promote/demote alerts, drawdown breach | `*TelegramNotifier` is nil-safe; reuse per-event-type cooldown system from v1.0 PP-01 |

### Frontend — No Changes (npm lockdown)

| Library | Version | Purpose | Why |
|---------|---------|---------|-----|
| `recharts` | 3.8.1 (locked) | Score history sparkline, ramp ladder visual, discovery cycle metrics | Same charts as v1.0 Phase 4 analytics |
| `react` / `react-dom` | 19.2.x (locked) | Discovery panel + ramp status card | `useEffect` polling + WS listener — patterns already in v2.1 dashboard CRUD modal |
| `react-is` | 19.2.4 (locked) | Recharts peer dep | Already locked |

**Forbidden:** any `npm install` / `npm update`. Lockfile is the gate. If a third-party charting widget feels needed, decompose into existing Recharts primitives.

## Integration Points

### 1. Auto-Discovery Scanner (PG-DISC-01) — pkg/exchange + Redis

```
internal/pricegaptrader/discovery/
├── scanner.go      // 30–60s ticker, fan-out goroutine per (exchange, symbol)
├── score.go        // composite score: spread_bps × persistence × volume × spread_stability
└── proposal.go     // CandidateProposal struct (read-only — does not mutate config)
```

- **Input:** iterate the configured symbol universe (denylist-aware per `project_strategy4_pricegap` memory). Call `exch.GetSpotBBO(symbol)` per (exchange, symbol) tuple. Universe should be hard-capped (e.g. 20 symbols × 6 exchanges = 120 BBO pulls per cycle) to bound rate-limit cost.
- **Cadence:** 30s ticker default (config: `PriceGapDiscoveryIntervalSec`). Avoid Bybit `:04–:05:30` blackout window — gate the ticker through the same blackout helper used by existing scanners.
- **Concurrency:** `sync.WaitGroup` + bounded worker pool (max 6 goroutines = one per exchange) — same pattern as `internal/discovery/scanner.go`.
- **No new exchange methods.** Scanner samples BBOs for cross-exchange comparison; klines are NOT required (the 4-bar persistence detector in `internal/pricegaptrader/detector.go` already runs against BBO-derived bars).

### 2. Auto-Promotion (PG-DISC-02) — config.json write path

- Reuse `cfg.SaveJSON()` absolute-path fallback (shipped v0.34.10, PG-OPS-08). Append to `cfg.PriceGapCandidates` slice; existing `.bak` flow handles atomicity.
- **Cap enforcement:** `PriceGapMaxCandidates` int (default 12). Drop lowest-score *auto* candidate when at cap; never evict pinned operator-added candidates — discriminate via new `Source: "operator" | "auto"` field on `PriceGapCandidate`.
- **Hot reload:** existing tracker hot-reload path (v2.1 Phase 10) already re-reads `cfg.PriceGapCandidates` — no new wiring.
- **Active-position guard:** mirror PG-OPS-07 logic — block auto-demotion of any candidate with a matching `pg:positions:active` row.

### 3. Discovery Telemetry (PG-DISC-03) — Redis namespacing

New keys (mirror existing `pg:` prefix convention):

| Key | Type | TTL | Purpose |
|-----|------|-----|---------|
| `pg:disc:cycle:{ts}` | Hash | 24h | Per-cycle metrics (`duration_ms`, `candidates_seen`, `above_threshold`, `errors`) |
| `pg:disc:score:{exch_long}:{exch_short}:{symbol}` | Sorted Set (score=ts ms, member=score_bps) | 7d via ZREMRANGEBYSCORE | Score history for sparkline |
| `pg:disc:promotions` | List, capped 200 via LTRIM | none | Promote/demote audit trail with reason codes |
| `pg:disc:active_proposals` | Hash | 5m | Current cycle's candidate proposals (for dashboard pull) |

`redis/go-redis/v9` v9.18.0 covers all of these natively (verified against existing usage in `internal/database/`). Pattern for trim + insert in one round-trip already exists as a Lua `EVAL` script in `internal/database/locks.go` — copy that pattern.

### 4. Live Capital Ramp Controller — internal/pricegaptrader/ramp/

```
internal/pricegaptrader/ramp/
├── controller.go    // 100→500→1000 USDT/leg ladder; reads cfg.PriceGapRampTier
├── reconcile.go     // Daily PnL reconcile job (00:05 UTC, single goroutine)
└── circuit.go       // Drawdown threshold breach → atomically set cfg.PriceGapPaperMode=true
```

- **Ramp state:** `pg:ramp:state` Redis hash (`tier`, `clean_days`, `last_reset_ts`, `last_advance_ts`).
- **Tier ladder:** 100 USDT/leg (initial) → 500 after 7 consecutive clean days (no drawdown breach, no exec-quality auto-disable trigger) → 1000 hard ceiling for v2.2. Tier downgrades on circuit breaker fire.
- **Reconcile job:** `time.NewTicker(24*time.Hour)` aligned to 00:05 UTC; reads `pg:positions:closed:*` from Redis and the SQLite analytics tables to compute realized PnL diff vs prior day. Writes `pg:ramp:reconcile:{date}` summary hash.
- **Drawdown breaker:** if rolling 24h realized PnL on Strategy 4 < `-PriceGapDrawdownLimitUSDT`, atomically `cfg.PaperMode=true` via `cfg.SaveJSON` and Telegram alert. Fail-safe direction is paper (existing default), so any partial failure leaves the system safer.

## Dependencies Considered and Rejected

| Considered | Rejected Because |
|------------|------------------|
| `prometheus/client_golang` for telemetry | Redis sorted sets + dashboard already cover the observability surface. No metrics scraper exists in this project. Adding Prometheus = new ops surface for a single-server deployment. |
| `robfig/cron` for daily reconcile | `time.NewTicker(24*time.Hour)` aligned to midnight UTC + `context.Context` cancellation is sufficient for one job. Existing `internal/engine/scheduler.go` shows the pattern. |
| `gorilla/mux` or chi router | `internal/api/handlers.go` uses stdlib `net/http.ServeMux` with bearer-token middleware. No reason to refactor. |
| New WebSocket library | `gorilla/websocket` v1.5.3 already handles WS push. Telemetry events ride existing `/ws` channel. |
| Time-series DB (InfluxDB / VictoriaMetrics) | Redis sorted sets with 7d retention cover the score-history sparkline. SQLite handles long-horizon PnL. |
| Adding kline/candle methods to `Exchange` interface | Detector already builds 1m bars from BBO sampling. Adding kline methods = 6-exchange adapter implementation work for zero new capability. |
| New frontend chart library | npm lockdown. Recharts 3.8.1 `LineChart` + `AreaChart` already cover sparkline + ladder visuals. |

## What NOT to Add

- **No new CEX adapters.** Out of scope per PROJECT.md; the 6 existing adapters are sufficient.
- **No npm packages.** Lockdown is absolute.
- **No new exchange interface methods.** `GetSpotBBO` is sufficient for the scanner; adding kline methods means 6-adapter changes for zero v2.2 benefit.
- **No Redis schema migration.** All new keys live under fresh `pg:disc:*` and `pg:ramp:*` namespaces; no read/write of existing keys is changed.
- **No config.json schema break.** New fields (`PriceGapDiscoveryIntervalSec`, `PriceGapAutoPromoteScore`, `PriceGapMaxCandidates`, `PriceGapRampTier`, `PriceGapDrawdownLimitUSDT`, `Source` on each candidate) are additive with safe zero-value defaults.

## Installation

**No installation steps for v2.2.** All packages already in `go.sum`. Pre-flight before any phase plan starts:

```bash
go mod verify        # All checksums match
go build ./...       # Compiles cleanly
cd web && npm ci     # Lockfile-only install (no resolution)
```

If `go mod verify` fails, that is a pre-existing environment issue, not a v2.2 dependency change.

## Sources

- `/var/solana/data/arb/go.mod` — current Go deps (HIGH — read directly)
- `/var/solana/data/arb/web/package.json` — locked frontend deps (HIGH — read directly)
- `/var/solana/data/arb/.planning/PROJECT.md` — v2.2 scope (HIGH)
- `/var/solana/data/arb/.planning/MILESTONES.md` — v2.1 deferrals into v2.2 (HIGH)
- `/var/solana/data/arb/pkg/exchange/types.go` L395-396 — `GetSpotBBO(symbol) (BBO, error)` confirmed (HIGH)
- `/var/solana/data/arb/internal/pricegaptrader/` — existing detector/executor/monitor/tracker/notify modules confirmed via directory listing (HIGH)
- `CLAUDE.local.md` — npm lockdown, config.json sole source of truth, Redis DB 2 invariants (HIGH)
- Memory: `project_strategy4_pricegap.md`, `project_phase8_pricegap_tracker.md` — narrow edge universe, denylist, exec-quality override (HIGH)

## Confidence Assessment

| Claim | Confidence | Basis |
|-------|------------|-------|
| Zero new Go deps required | HIGH | All needed primitives present in go.mod; verified against pricegaptrader directory listing |
| Zero new npm deps required | HIGH | Recharts + React already cover all UI; npm lockdown blocks alternatives anyway |
| `GetSpotBBO` sufficient for scanner | HIGH | Direct grep of `pkg/exchange/types.go` confirmed method on Exchange interface |
| Redis sorted sets sufficient for score history | HIGH | go-redis v9.18.0 ZADD/ZRANGEBYSCORE/EVAL are stable core API; existing usage in `internal/database/locks.go` |
| Daily reconcile via `time.Ticker` | HIGH | Existing scheduler in internal/engine uses identical pattern |
| No Context7 lookup needed | HIGH | All recommendations are reuses of already-validated dependencies; no version-bump speculation |
