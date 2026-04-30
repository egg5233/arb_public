---
phase: 12
plan: 02
subsystem: pricegaptrader, api, notify
tags: [auto-promotion, redis, websocket, telegram, rest, tdd, PG-DISC-02]
requires:
  - .planning/phases/12-auto-promotion/12-01-SUMMARY.md  # PromoteEventSink/TelemetrySink/PromoteNotifier interfaces
  - internal/pricegaptrader/promotion.go                  # Plan 01 PromoteEvent struct + interfaces
  - internal/pricegaptrader/telemetry.go                  # Plan 11-05 Redis HASH+LIST pattern reused
  - internal/api/pricegap_discovery_handlers.go           # Plan 11-05 auth + envelope pattern reused
  - internal/notify/telegram.go                           # Phase 3 cooldown primitive reused
provides:
  - RedisWSPromoteSink (concrete PromoteEventSink — Redis LIST + WS hub)
  - Telemetry.IncCapFullSkip (concrete TelemetrySink method)
  - TelegramNotifier.NotifyPromoteEvent (concrete PromoteNotifier method)
  - GET /api/pg/discovery/promote-events (REST seed for dashboard timeline)
  - WSBroadcaster narrow interface (preserves D-15 module boundary)
affects:
  - internal/pricegaptrader/  # 1 new file (promote_event_sink.go) + IncCapFullSkip added to telemetry.go
  - internal/notify/          # NotifyPromoteEvent added to telegram.go
  - internal/api/             # handler added + route registered in server.go
tech-stack:
  added: []  # zero new dependencies
  patterns:
    - "RPush + LTrim 1000 (mirrors pg:scan:cycles)"
    - "HASH HINCRBY for per-symbol counters (mirrors pg:scan:metrics)"
    - "narrow interface declared at consumer (WSBroadcaster) for module boundary"
    - "newest-first response by post-LRANGE reverse"
    - "Telegram cooldown key built from event tuple to discriminate flap vs distinct events"
key-files:
  created:
    - internal/pricegaptrader/promote_event_sink.go
    - internal/pricegaptrader/promote_event_sink_test.go
    - internal/api/pricegap_promote_events_test.go
    - internal/notify/telegram_promote_test.go
  modified:
    - internal/pricegaptrader/telemetry.go              # +IncCapFullSkip
    - internal/notify/telegram.go                       # +NotifyPromoteEvent
    - internal/api/pricegap_discovery_handlers.go       # +handlePgDiscoveryPromoteEvents + json import
    - internal/api/server.go                            # +route registration
decisions:
  - "Plan example wrote `t.db.GetRedis()`; the actual database.Client method is `Redis()` (verified internal/database/redis.go:45). Adopted Redis() throughout (Rule 3 deviation — naming drift in plan)."
  - "Telegram message format uses raw `↔` rune (UTF-8 \\xe2\\x86\\x94) embedded as a byte sequence in the format string so test assertions match the on-wire bytes regardless of editor encoding handling."
  - "WSBroadcaster narrow interface lives in pricegaptrader (not internal/api) so the package never imports internal/api — *api.Hub satisfies it via duck typing at the wiring site (cmd/main.go in Plan 12-03)."
  - "REST handler uses s.db.Redis() directly (mirroring telemetry's existing reads) rather than going through s.telemetry — keeps the handler authoritative on the LIST schema and decouples Plan 12 from any Telemetry helper API."
  - "Empty-symbol IncCapFullSkip is silently a no-op (defensive; prevents poisoning the HASH with a `cap_full_skips:` zero-suffix field if a future caller passes an empty string)."
metrics:
  duration: 22min
  completed: 2026-04-30
---

# Phase 12 Plan 02: PromoteEventSink + Telemetry HINCRBY + Telegram + REST seed — Summary

The four I/O sinks the controller needs: Redis-backed event LIST, WS push, per-symbol cap-full counter, Telegram alert, and a Bearer-authed REST seed for the dashboard timeline. All implementations satisfy the interfaces Plan 01 declared, so Plan 03 can wire concrete `*RedisWSPromoteSink`, `*Telemetry`, `*TelegramNotifier` into `NewPromotionController` without further interface changes.

## What changed

- **`internal/pricegaptrader/promote_event_sink.go`** (new, 87 lines): `RedisWSPromoteSink` struct + `NewRedisWSPromoteSink(db, hub)` constructor + `Emit(ctx, ev)` method. Marshals `PromoteEvent` to JSON, RPushes onto `pg:promote:events` LIST, LTrims to last 1000 (D-10 bound), and broadcasts `pg_promote_event` over the WS hub. `WSBroadcaster` narrow interface declared inline (`Broadcast(eventType, payload)`) so the package does NOT import internal/api (D-15 boundary).
- **`internal/pricegaptrader/telemetry.go`** (+24 lines): `IncCapFullSkip(ctx, symbol)` method appended after `WriteCycleFailed`. HASH HINCRBY on `pg:scan:metrics` field `cap_full_skips:{symbol}`, per-symbol so operators see WHICH symbol is sustained-cap-full. Empty symbol is a defensive no-op. Reuses the existing `boundCtx(ctx)` 3s timeout helper.
- **`internal/notify/telegram.go`** (+34 lines): `NotifyPromoteEvent(action, symbol, longExch, shortExch, direction, score, streak)` method. Cooldown key per D-13: `"pg_promote:" + action + ":" + symbol + ":" + longExch + ":" + shortExch + ":" + direction`. Format per D-14: `"[PG {action}] {symbol} {long}↔{short} ({direction}) score=N streak=M"`. Nil-receiver safe per project convention.
- **`internal/api/pricegap_discovery_handlers.go`** (+38 lines + `encoding/json` import): `handlePgDiscoveryPromoteEvents` reads `pg:promote:events` LIST via `s.db.Redis().LRange`, unmarshals each entry as `pricegaptrader.PromoteEvent`, reverses to newest-first, and returns `{ ok: true, data: []PromoteEvent }`. T-12-07 mitigation: malformed entries silently skipped. Auth enforced by the existing `authMiddleware` wrapper at the route registration site (T-12-08).
- **`internal/api/server.go`** (+4 lines): `mux.HandleFunc("GET /api/pg/discovery/promote-events", s.cors(s.authMiddleware(s.handlePgDiscoveryPromoteEvents)))` adjacent to the existing `state` and `scores/{symbol}` routes.

## Tests added

| File | Tests | Coverage |
|------|-------|----------|
| `internal/pricegaptrader/promote_event_sink_test.go` | 4 | Redis JSON round-trip, LTrim 1000 (1500-event stress, oldest 500 dropped), WS broadcast capture (eventType + payload deep equal), per-symbol HASH increment + empty-symbol defense |
| `internal/notify/telegram_promote_test.go` | 5 | Exact format string with `↔` rune; demote prefix; distinct-key fan-out within cooldown; same-key suppression within 5-min cooldown; nil-receiver safety |
| `internal/api/pricegap_promote_events_test.go` | 4 | Newest-first ordering; empty-LIST returns `{ok:true,data:[]}`; missing-auth → 401; malformed-JSON entries skipped, valid ones returned |

Total: **13 new unit tests, 13/13 PASS** in their first or second compile.

## How decisions D-08..D-14 are honored

| Decision | Implementation site |
|----------|--------------------|
| D-08 cap-full HOLD telemetry | `Telemetry.IncCapFullSkip` HINCRBYs `cap_full_skips:{symbol}` on `pg:scan:metrics` |
| D-10 PromoteEvent JSON keys + LIST bound 1000 | `RedisWSPromoteSink.Emit` uses Plan 01 struct's locked tags + `LTrim(... -1000, -1)` |
| D-11 sink fans to Redis + WS | `Emit` does both before returning |
| D-12 REST endpoint newest-first | `handlePgDiscoveryPromoteEvents` reverses post-LRANGE |
| D-13 Telegram per-event cooldown key | `NotifyPromoteEvent` builds 6-field key tuple |
| D-14 Telegram format string | `"[PG %s] %s %s\xe2\x86\x94%s (%s) score=%d streak=%d"` |
| D-15 module boundary | `WSBroadcaster` narrow interface in pricegaptrader; zero `internal/api` imports there |

## Plan 03 contract compliance

- `NewPromotionController` from Plan 01 takes:
  - `RegistryWriter` ← `*Registry` (registry.go)
  - `ActivePositionChecker` ← `*DBActivePositionChecker` (Plan 03 will declare)
  - `PromoteEventSink` ← `*RedisWSPromoteSink` (this plan, satisfied)
  - `PromoteNotifier` ← `*notify.TelegramNotifier` (this plan via NotifyPromoteEvent)
  - `TelemetrySink` ← `*Telemetry` (this plan via IncCapFullSkip)
- All 5 dependencies have concrete impls and the interfaces compile-time-assert in tests (`var _ PromoteEventSink = (*RedisWSPromoteSink)(nil)`, `var _ TelemetrySink = (*Telemetry)(nil)`).
- Compile-time assertion for `PromoteNotifier ← *TelegramNotifier` is naturally enforced by Plan 03 wiring; tests here verify the method exists with the correct signature.

## Verification (last run)

```
go build ./...                                                                          # PASS
go vet  ./...                                                                           # PASS
go test ./internal/pricegaptrader/ -run "TestRedisWSPromoteSink|TestTelemetry_IncCapFullSkip" -count=1  # 4 PASS
go test ./internal/notify/        -run "TestNotifyPromoteEvent" -count=1                # 5 PASS
go test ./internal/api/           -run "TestHandlePgDiscoveryPromoteEvents" -count=1    # 4 PASS
go test ./internal/pricegaptrader/... ./internal/api/... ./internal/notify/...          # 364 PASS, 0 FAIL
grep '"arb/internal/api"' internal/pricegaptrader/*.go                                  # zero matches (boundary preserved)
```

## Commits

- `8916226` feat(12-02): add Telemetry.IncCapFullSkip + RedisWSPromoteSink with Redis+WS emission
- `5d2d3d5` feat(12-02): add /api/pg/discovery/promote-events REST seed + Telegram NotifyPromoteEvent (D-12, D-13, D-14)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `database.Client` method name drift in plan example**
- **Found during:** Task 1 first compile attempt.
- **Issue:** Plan example wrote `t.db.GetRedis().HIncrBy(...)`. Actual method on `*database.Client` is `Redis()` (one method, no `Get` prefix; see `internal/database/redis.go:45`). Using the plan literal would have caused `tel.db.GetRedis undefined`.
- **Fix:** Used `t.db.Redis()` consistently in `IncCapFullSkip`, `RedisWSPromoteSink.Emit`, and the REST handler. Mirrors the existing usage on the very same `Telemetry` struct (`telemetry.go:183`, `t.db.Redis()`).
- **Files:** `internal/pricegaptrader/telemetry.go`, `internal/pricegaptrader/promote_event_sink.go`, `internal/api/pricegap_discovery_handlers.go`.
- **Commits:** `8916226`, `5d2d3d5` (incorporated before commit; not a follow-up).

**2. [Rule 3 - Blocking] miniredis HGet/HKeys signatures changed since plan example**
- **Found during:** Task 1 first test run.
- **Issue:** Plan example wrote `mr.HGet(key, field)` returning `(value, error)`. The vendored `miniredis/v2` returns just `value string` for `HGet` and `(keys, err)` for `HKeys`.
- **Fix:** Adjusted the test to drop the err return on `HGet` and accept `(keys, err)` on `HKeys`.
- **Files:** `internal/pricegaptrader/promote_event_sink_test.go`.
- **Commit:** `8916226` (test re-run was the second iteration; both fixes landed together).

**3. [Rule 2 - Critical] Defensive empty-symbol guard on IncCapFullSkip**
- **Found during:** Task 1 design.
- **Issue:** A future caller passing `""` would write a HASH field literally named `cap_full_skips:` (empty suffix), polluting the metrics namespace and confusing operators.
- **Fix:** Early-return on empty symbol. Test asserts the poisoned field never appears.
- **Commit:** `8916226`.

**4. [Plan-explicit choice] REST handler reads Redis directly rather than going through Telemetry helper**
- **Why:** The plan ACTION block sketched the handler as `s.db.GetRedis().LRange(...)` rather than a `s.telemetry.GetPromoteEvents(...)` accessor. Adopted as written. Reasoning preserved for future readers: keeps the LIST schema authoritative in one place (the handler) and avoids growing the `DiscoveryTelemetryReader` interface for a one-LIST read.
- **Files:** `internal/api/pricegap_discovery_handlers.go`.
- **Not a real deviation** — flagged here so Plan 03 + future maintainers know this was a deliberate plan-internal split, not an accidental skip of an existing helper.

### Asks (none)

No checkpoints, no auth gates, no architectural changes.

## Threat Flags

None — all new surface areas are explicitly registered in Plan 02 threat-model (T-12-07..T-12-12). T-12-07 mitigation (skip malformed) is implemented and asserted by `TestHandlePgDiscoveryPromoteEvents_SkipsMalformedEntries`. T-12-08 mitigation (Bearer auth) is implemented and asserted by `TestHandlePgDiscoveryPromoteEvents_RequiresAuth`. T-12-10 mitigation (LTrim 1000) is implemented and asserted by `TestRedisWSPromoteSink_EmitTrimsTo1000`. T-12-09 (Telegram flap suppression) is implemented and asserted by `TestNotifyPromoteEvent_SameKeySuppressedWithinCooldown`.

## Self-Check

- [x] `internal/pricegaptrader/promote_event_sink.go` exists (87 lines)
- [x] `internal/pricegaptrader/promote_event_sink_test.go` exists
- [x] `internal/api/pricegap_promote_events_test.go` exists
- [x] `internal/notify/telegram_promote_test.go` exists
- [x] `internal/pricegaptrader/promote_event_sink.go` contains `type RedisWSPromoteSink struct`
- [x] `internal/pricegaptrader/promote_event_sink.go` contains `keyPromoteEvents = "pg:promote:events"`
- [x] `internal/pricegaptrader/promote_event_sink.go` contains `wsEventPromoteEvent = "pg_promote_event"`
- [x] `internal/pricegaptrader/promote_event_sink.go` contains `LTrim(ctx, keyPromoteEvents, -promoteEventsMaxLen, -1)`
- [x] `internal/pricegaptrader/promote_event_sink.go` contains `type WSBroadcaster interface`
- [x] `internal/pricegaptrader/telemetry.go` contains `func (t *Telemetry) IncCapFullSkip(`
- [x] `internal/pricegaptrader/telemetry.go` contains `HIncrBy(c, keyMetrics, "cap_full_skips:"+symbol, 1)` (per-symbol prefix)
- [x] `internal/notify/telegram.go` contains `func (t *TelegramNotifier) NotifyPromoteEvent(`
- [x] `internal/notify/telegram.go` contains `"pg_promote:" + action + ":"` (cooldown key prefix)
- [x] `internal/notify/telegram.go` contains the `[PG %s] ... score=%d streak=%d` format string
- [x] `internal/notify/telegram.go` contains `if t == nil` nil-safe guard inside NotifyPromoteEvent
- [x] `internal/api/pricegap_discovery_handlers.go` contains `handlePgDiscoveryPromoteEvents`
- [x] `internal/api/pricegap_discovery_handlers.go` contains `LRange(r.Context(), "pg:promote:events"`
- [x] `internal/api/pricegap_discovery_handlers.go` contains the reverse-to-newest-first loop
- [x] `internal/api/server.go` contains `"GET /api/pg/discovery/promote-events"`
- [x] commit `8916226` present in `git log`
- [x] commit `5d2d3d5` present in `git log`
- [x] `go build ./...` succeeds
- [x] `go vet ./...` succeeds
- [x] `go test ./internal/pricegaptrader/... ./internal/api/... ./internal/notify/...` 364 pass
- [x] `grep -r "internal/api" internal/pricegaptrader/promote_event_sink.go` zero matches (module boundary)
- [x] no worktree branches dangling — `git worktree list` shows only main checkout

## Self-Check: PASSED
