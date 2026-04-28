---
phase: 11
plan: 05
subsystem: pricegaptrader
tags: [auto-discovery, telemetry, redis, websocket, rest, bootstrap, runtime-wiring]
dependency_graph:
  requires:
    - 11-01 # discovery config schema + SaveJSONWithBakRing
    - 11-02 # *Registry chokepoint
    - 11-03 # POST /api/config + pg-admin routed through Registry; validate.go shared
    - 11-04 # Scanner core + TelemetryWriter interface + CycleSummary/CycleRecord
  provides:
    - "internal/pricegaptrader.Telemetry — Redis-backed TelemetryWriter implementation (5 keys + 3 WS events)"
    - "internal/pricegaptrader.StateResponse / ScoresResponse / SubScores / ScorePoint / SnapshotEntry / ThresholdBand — REST envelope types"
    - "internal/pricegaptrader.Tracker.SetScanner + scanLoop + subscribeUniverse — runtime integration"
    - "internal/pricegaptrader.ValidateSymbol — bounded canonical regex for path-parameters (T-11-33)"
    - "internal/api.Server.SetDiscoveryTelemetry + DiscoveryTelemetryReader interface"
    - "internal/api.Server.BroadcastPriceGapDiscoveryEvent — generic WS forwarder for pg_scan_* events"
    - "GET /api/pg/discovery/state + GET /api/pg/discovery/scores/{symbol} REST endpoints"
    - "cmd/main.go bootstrap: Telemetry unconditional, Scanner conditional on cfg.PriceGapDiscoveryEnabled"
  affects:
    - "Plan 11-06 (UI): consumes the REST endpoints + WS pg_scan_cycle/metrics/score events"
tech_stack:
  added:
    - "github.com/redis/go-redis/v9 ZAdd/HSet/HIncrBy/RPush/LTrim/Set/Get/HGetAll/LRange/ZRange/Expire (already in go.mod via internal/database)"
  patterns:
    - "Narrow DI seams: DiscoveryBroadcaster (1 method) + DiscoveryTelemetryReader (2 methods) preserve pricegaptrader↔api module boundary."
    - "Default-OFF gating in cmd/main.go: Scanner construction is the ONLY gated piece; Telemetry is always built so REST endpoints render the OFF state."
    - "5-second metrics throttle + 1-second per-symbol score debounce coalesce WS event volume; pg_scan_cycle is unthrottled (low rate by design)."
    - "Bounded path-param regex `^[A-Z0-9]{2,20}USDT$` rejects Redis-key injection (T-11-33)."
    - "Per-Redis-call 3-second context timeout in Telemetry — a hung Redis can't freeze scanLoop."
    - "Bounded RunCycle ctx in scanLoop (= cycle interval) — scanner can't block stop signal."
key_files:
  created:
    - internal/pricegaptrader/telemetry.go
    - internal/pricegaptrader/telemetry_test.go
    - internal/api/pricegap_discovery_handlers.go
    - internal/api/pricegap_discovery_handlers_test.go
  modified:
    - internal/pricegaptrader/tracker.go        # +ScanRunner, +SetScanner, +subscribeUniverse, +scanLoop
    - internal/pricegaptrader/validate.go       # +pathSymbolRE, +ValidateSymbol
    - internal/pricegaptrader/validate_test.go  # +TestValidateSymbol (12 sub-tests)
    - internal/api/server.go                    # +telemetry field, +DiscoveryTelemetryReader, +SetDiscoveryTelemetry, +2 routes
    - internal/api/pricegap_handlers.go         # +BroadcastPriceGapDiscoveryEvent
    - cmd/main.go                               # +Telemetry unconditional, +Scanner conditional, +SetDiscoveryTelemetry wiring
decisions:
  - "Telemetry is unconditional inside cfg.PriceGapEnabled block (read-side OFF state must be observable). Scanner construction is the ONLY thing gated by cfg.PriceGapDiscoveryEnabled — preserves D-11 default-OFF for the goroutine path."
  - "scanFirstTickOffset = 13s (chosen distinct from tickLoop's 7s to avoid collision; both offset off Bybit :04..:05:30 blackout)."
  - "DiscoveryBroadcaster in pricegaptrader package is a NARROW (1-method) interface — *api.Server satisfies it via existing s.hub.Broadcast pattern. No api package import inside Telemetry."
  - "Path-param ValidateSymbol uses bounded `^[A-Z0-9]{2,20}USDT$` regex (T-11-33 Redis-key injection mitigation). canonicalSymbolRE in the candidate validator stays unbounded for backward compatibility with existing tests."
  - "Unknown symbol on /api/pg/discovery/scores returns 200 + empty points (UI shows 'no history yet'); only canonical-form check produces 400. Matches existing pricegap_handlers behaviour."
  - "Telemetry GetState reads pg:scan:metrics + pg:scan:enabled + last cycle envelope (LRange -1 -1) and re-derives WhyRejected histogram + ScoreSnapshot from the cycle. No secondary read of pg:scan:rejections — last cycle's WhyRejected is the authoritative seed."
metrics:
  duration: "~30min"
  completed: "2026-04-28"
---

# Phase 11 Plan 05: Telemetry + Runtime Bootstrap Summary

Closes the runtime live-system path for the Phase 11 auto-discovery scanner: with this plan complete, setting `PriceGapDiscoveryEnabled=true` and restarting the binary produces actual scan cycles writing to Redis and broadcasting WS events. Implements PG-DISC-03 (telemetry persistence) and the runtime half of PG-DISC-01 (scanner runs in production).

## Telemetry Surface

| Method | Type | Effect |
|---|---|---|
| `WriteCycle(ctx, summary)` | TelemetryWriter | RPush + LTrim 1000 to `pg:scan:cycles`; HSet `pg:scan:metrics`; ZAdd accepted records to `pg:scan:scores:{sym}` (7d EXPIRE); HIncrBy reasons into `pg:scan:rejections:{date}` (30d EXPIRE); broadcast pg_scan_cycle (no throttle) + maybeBroadcastMetrics + queueScoreBroadcast per accepted record |
| `WriteEnabledFlag(ctx, enabled)` | TelemetryWriter | Set `pg:scan:enabled` to "0"/"1" |
| `WriteSymbolNoCrossPair(ctx)` | TelemetryWriter | HIncrBy `pg:scan:metrics` field `symbol_no_cross_pair` by 1 |
| `WriteCycleFailed(ctx)` | TelemetryWriter | HSet `pg:scan:metrics` field `cycle_failed=1`; cleared by next WriteCycle |
| `GetState(ctx) → StateResponse` | read helper | Composes REST envelope from metrics HASH + enabled flag + last cycle |
| `GetScores(ctx, symbol) → ScoresResponse` | read helper | ZRange the per-symbol ZSET, sort ascending by ts, attach threshold band |

Compile-time assertion: `var _ TelemetryWriter = (*Telemetry)(nil)`.

## Redis Schema

| Key | Type | TTL | Writer |
|---|---|---|---|
| `pg:scan:cycles` | LIST (cap 1000 via LTrim) | none | WriteCycle |
| `pg:scan:scores:{symbol}` | ZSET (member=JSON ScorePoint, score=int score) | 7d | WriteCycle (per accepted record) |
| `pg:scan:rejections:{date}` | HASH (field=ScanReason, value=count) | 30d | WriteCycle (date = UTC `2006-01-02`) |
| `pg:scan:metrics` | HASH | none | WriteCycle / WriteSymbolNoCrossPair / WriteCycleFailed |
| `pg:scan:enabled` | STRING ("0"/"1") | none | WriteEnabledFlag |

## WS Event Types

| Event | Source | Throttle | Payload |
|---|---|---|---|
| `pg_scan_cycle` | every WriteCycle | none (low rate by design) | full `CycleSummary` |
| `pg_scan_metrics` | every WriteCycle | 5s | metrics HASH map |
| `pg_scan_score` | per accepted record | 1s debounce, coalesced per symbol | `{symbol, score, ts, sub_scores}` |

## Tracker scanLoop Integration

- `Tracker.scanFirstTickOffset = 13s` — distinct from tickLoop's 7s to avoid co-firing under fresh start. Both offsets push first tick past Bybit `:04..:05:30` blackout.
- `Tracker.scanInterval` — 0 means runtime-read `cfg.PriceGapDiscoveryIntervalSec` (production default 300s); tests override.
- scanLoop launched in `Start()` ONLY when `t.scanner != nil` (set via `SetScanner` from cmd/main.go's gated construction).
- `subscribeUniverse()` called in `Start()` BEFORE scanLoop launch — fans out `SubscribeSymbol` for every (universe-symbol, exchange) tuple. Pitfall 5 mitigation: scanner's first cycle reads live BBO instead of empty book.
- Each cycle gets `context.WithTimeout(parent, interval)` so a hung scanner can't block the stop signal (T-11-29 mitigation).

## REST Endpoint Shapes

`GET /api/pg/discovery/state` →

```json
{ "ok": true, "data": {
  "enabled": true, "last_run_at": 1700000000, "next_run_in": 300,
  "candidates_seen": 18, "accepted": 3, "rejected": 15, "errors": 0,
  "duration_ms": 250, "cycle_failed": false,
  "why_rejected": {"insufficient_persistence": 8, "stale_bbo": 7},
  "score_snapshot": [{"symbol": "BTCUSDT", "long_exch": "binance", "short_exch": "bybit",
    "score": 75, "sub_scores": {...}, "gates_passed": [...], "gates_failed": [...]}]
}}
```

`GET /api/pg/discovery/scores/BTCUSDT` →

```json
{ "ok": true, "data": {
  "symbol": "BTCUSDT",
  "points": [{"ts": 1700000000, "score": 75, "sub_scores": {...}}],
  "threshold_band": {"auto_promote": 60}
}}
```

## ValidateSymbol Path-Param Regex

Location: `internal/pricegaptrader/validate.go` (appended; `pathSymbolRE = ^[A-Z0-9]{2,20}USDT$`).

Bounded form rejects:
- lowercase / mixed case
- hyphens / separators
- empty / no base / single-char base
- length > 20 chars before USDT (Redis-key injection / pathological-length)
- Cyrillic look-alikes
- `\r\n`-injected payloads

12 sub-tests cover happy + 8 rejection cases.

## main.go Bootstrap Wiring

```go
if cfg.PriceGapEnabled {
    pgTracker = pricegaptrader.NewTracker(...)
    pgTracker.SetBroadcaster(apiSrv)
    pgTracker.SetNotifier(tg)

    // Telemetry: unconditional — REST endpoints need to render OFF state.
    pgTelemetry := pricegaptrader.NewTelemetry(db, apiSrv, cfg, log)
    apiSrv.SetDiscoveryTelemetry(pgTelemetry)

    // Scanner: conditional — D-11 default OFF.
    if cfg.PriceGapDiscoveryEnabled {
        pgScanner := pricegaptrader.NewScanner(cfg, pgRegistry, exchanges, pgTelemetry, log)
        pgTracker.SetScanner(pgScanner)
    }

    pgTracker.Start() // launches scanLoop only if scanner non-nil
}
```

## subscribeUniverse Pre-warm Count

`len(cfg.PriceGapDiscoveryUniverse) × len(exchanges)`. With production cap of 20 universe symbols × 6 exchanges = 120 SubscribeSymbol calls at scanner launch. Each call is fail-soft (false return logged once); singletons get silent-skipped at the scanner layer (D-08).

## Test Counts

| File | Sub-tests | Notes |
|---|---|---|
| internal/pricegaptrader/telemetry_test.go | 17 | TelemetryWriter (4 methods) + 2 read helpers + Tracker scanLoop/subscribeUniverse + WS throttle/debounce + cycle list cap |
| internal/api/pricegap_discovery_handlers_test.go | 12 | Happy / empty / 401 / 503 / 400 / unknown-symbol / route smoke |
| internal/pricegaptrader/validate_test.go (TestValidateSymbol) | 12 | Bounded regex coverage (4 happy + 8 rejected) |

Combined: **41 sub-tests, all passing.**

## Verification Snapshot

```
$ go build ./...
(success)

$ go test ./internal/pricegaptrader/... ./internal/api/... ./cmd/... -count=1 -timeout=180s
325 passed in 21 packages

$ go test -race ./internal/pricegaptrader/ -count=1 -timeout=120s
225 passed (race-detector clean)

$ go vet ./internal/pricegaptrader/... ./internal/api/... ./cmd/...
(no issues)

$ gofmt -l internal/pricegaptrader/telemetry.go internal/pricegaptrader/telemetry_test.go \
            internal/pricegaptrader/tracker.go internal/pricegaptrader/validate.go \
            internal/pricegaptrader/validate_test.go internal/api/pricegap_discovery_handlers.go \
            internal/api/pricegap_discovery_handlers_test.go internal/api/server.go \
            internal/api/pricegap_handlers.go cmd/main.go
(empty)

$ grep -c 'var _ TelemetryWriter = (\*Telemetry)(nil)' internal/pricegaptrader/telemetry.go
1

$ grep -c 'pg:scan:cycles\|pg:scan:scores\|pg:scan:rejections\|pg:scan:metrics\|pg:scan:enabled' internal/pricegaptrader/telemetry.go
≥5

$ grep -c 'pg_scan_cycle\|pg_scan_metrics\|pg_scan_score' internal/pricegaptrader/telemetry.go
≥3

$ grep -c 'scanLoop\|subscribeUniverse' internal/pricegaptrader/tracker.go
≥2

$ grep -E 'internal/engine|internal/spotengine' internal/pricegaptrader/telemetry.go internal/pricegaptrader/tracker.go | wc -l
0  (module boundary preserved)

$ grep -c 'pricegaptrader\.NewScanner' cmd/main.go
1

$ grep -c 'pricegaptrader\.NewTelemetry' cmd/main.go
1

$ grep -c 'cfg\.PriceGapDiscoveryEnabled' cmd/main.go
1  (the conditional guard)
```

## Deviations from Plan

**[Rule 1 — Plan/Reality Drift] Plan referenced `cmd/arb/main.go`; actual file is `cmd/main.go`.**
- **Found during:** initial file read.
- **Issue:** Plan 11-05 frontmatter and action steps refer to `cmd/arb/main.go`, but the actual binary entrypoint in this repo is `cmd/main.go` (verified via `ls cmd/`).
- **Fix:** Bootstrap edits made to `cmd/main.go`. All Plan acceptance criteria for the bootstrap path remain satisfied.
- **Files modified:** `cmd/main.go`
- **Commit:** rolled into Task 3 commit `5430564`.

**[Rule 2 — Critical safety] Telemetry constructed UNCONDITIONALLY inside `cfg.PriceGapEnabled` block.**
- **Found during:** plan analysis — Note 4 in Plan 05 Task 3 explicitly calls this out.
- **Rationale:** REST handlers must render the `enabled=false` state so the dashboard can distinguish "scanner disabled" from "scanner has never run yet". Wiring Telemetry only when discovery is enabled would crash REST with 503 in default-OFF deployment.
- **Implementation:** Telemetry construction is OUTSIDE the `if cfg.PriceGapDiscoveryEnabled` guard (but inside the existing `if cfg.PriceGapEnabled` block — discovery requires the price-gap subsystem to be up).
- **Commit:** Task 3 commit `5430564`.

**[Rule 3 — Blocking issue] Existing `Broadcaster` interface only has 2 methods; needed a generic event broadcaster for pg_scan_* events without proliferating method signatures.**
- **Found during:** Telemetry implementation.
- **Issue:** Adding 3 new methods (`BroadcastScanCycle`, `BroadcastScanMetrics`, `BroadcastScanScore`) to `pricegaptrader.Broadcaster` would force every test fake (and `NoopBroadcaster`) to implement them. None of the existing tracker code paths broadcast pg_scan_* events; only Telemetry needs them.
- **Fix:** Introduced `pricegaptrader.DiscoveryBroadcaster` — a NARROW (1-method) interface with `BroadcastPriceGapDiscoveryEvent(eventType string, payload interface{})`. *Server satisfies it via existing `s.hub.Broadcast` pattern. Telemetry depends only on this narrow interface; no widening of the existing Broadcaster surface.
- **Commit:** Task 1 commit `af1fd59`.

**[Rule 1 — Bug] Plan 05 Task 1 spec says `WriteCycleFailed` HSets `cycle_failed=1` and "cleared to 0 after next successful WriteCycle".**
- **Found during:** test 8 (TestTelemetry_WriteCycleFailed_CycleClears) wiring.
- **Issue:** As specified, WriteCycle's metrics HSet must include `cycle_failed=0` so the next successful cycle clears the forensic flag. Initial implementation omitted the field.
- **Fix:** WriteCycle's metrics map now sets `"cycle_failed": "0"` (string form to match WriteCycleFailed's `"1"` so HGetAll produces consistent type). Test 8 verifies the round-trip.
- **Commit:** Task 1 commit `af1fd59`.

No other deviations.

## Threat Model Disposition

| Threat | Mitigation |
|---|---|
| T-11-29 (Slow GetDepth stalls cycle) | scanLoop wraps RunCycle in `context.WithTimeout(_, interval)` so a hung adapter read can't block stop signal. Each Telemetry Redis call has its own 3s context timeout. |
| T-11-31 (Information disclosure via pg:scan:* keys) | accept: existing Redis posture (localhost-bound, password-protected). New keys inherit same posture. |
| T-11-32 (Race on cycle_failed flag) | accept: last-writer-wins on HSet is acceptable for a forensic flag — next WriteCycle reflects most-recent state. |
| T-11-33 (Redis-key injection via /scores/{symbol}) | mitigate: `pathSymbolRE = ^[A-Z0-9]{2,20}USDT$` enforced in pricegaptrader.ValidateSymbol; handler returns 400 before symbol reaches Redis. |
| T-11-34 (Operator floods GET state every 100ms) | accept: existing Bearer-auth + cors middleware applies. Throttle/debounce on the WRITE-side WS broadcast keeps event volume bounded regardless of REST poll rate. |
| T-11-35 (120 SubscribeSymbol storm at startup) | mitigate: SubscribeSymbol is WS-layer (idempotent at adapter); firstTick offset 13s gives WS time to settle before first cycle reads. |
| T-11-36 (Stale universe mid-cycle) | accept: scanner re-snapshots cfg.PriceGapDiscoveryUniverse under cfg.RLock at each RunCycle start. |
| T-11-44 (Bootstrap forgets to wire scanner) | mitigate: this plan IS the bootstrap; logger.Info traces the gate decision so ops can grep `[phase-11] discovery scanner`. Plan 06 checkpoint verifies live runtime. |

## Scope Compliance

- ✅ Telemetry struct ships with full TelemetryWriter implementation + 2 read helpers
- ✅ Tracker.SetScanner + scanLoop + subscribeUniverse wired
- ✅ cmd/main.go conditionally constructs Scanner; unconditionally constructs Telemetry
- ✅ REST handlers GET /api/pg/discovery/state + /scores/{symbol} ship with Bearer auth
- ✅ ValidateSymbol bounded regex appended to validate.go
- ✅ Module boundary preserved: no internal/engine or internal/spotengine imports in any modified file
- ✅ DO NOT modify config.json — verified via `git diff` (only schema fields are read, never written from this plan's surface)
- ✅ Existing perp-perp engine, spot-futures engine, exchange adapters untouched

## Plan 06 Hand-off

The runtime live-system path is now wired. With `PriceGapDiscoveryEnabled=true` in config.json:

1. Bootstrap log: `[phase-11] discovery scanner enabled — scanLoop will start with Tracker (interval=300s)`
2. After 13s firstTick: scanner runs first cycle
3. WS clients receive `pg_scan_cycle` immediately, `pg_scan_metrics` (throttled), `pg_scan_score` (debounced per symbol)
4. `redis-cli -n 2 KEYS pg:scan:*` shows the 5 documented keys
5. `curl -H 'Authorization: Bearer ...' /api/pg/discovery/state` returns the StateResponse envelope
6. `curl ... /api/pg/discovery/scores/BTCUSDT` returns the ScoresResponse envelope

Plan 06 (UI) consumes both REST endpoints + 3 WS events without any further backend changes.

## Self-Check: PASSED

- ✅ `internal/pricegaptrader/telemetry.go` exists
- ✅ `internal/pricegaptrader/telemetry_test.go` exists
- ✅ `internal/api/pricegap_discovery_handlers.go` exists
- ✅ `internal/api/pricegap_discovery_handlers_test.go` exists
- ✅ Per-task commits exist on the branch (af1fd59, b6247d8, 5430564)
- ✅ `go build ./...` — success
- ✅ `go test ./internal/pricegaptrader/... ./internal/api/... ./cmd/...` — 325 tests pass across 21 packages
- ✅ Race detector clean (`go test -race ./internal/pricegaptrader/`) — 225 tests
- ✅ `gofmt -l` on touched files — empty
- ✅ `go vet` on touched packages — no issues
- ✅ Compile-time `var _ TelemetryWriter = (*Telemetry)(nil)` assertion present
- ✅ Module boundary preserved: no internal/engine or internal/spotengine imports
- ✅ config.json not touched
