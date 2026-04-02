# Codebase Concerns

**Analysis Date:** 2026-04-01

## Tech Debt

**Binance REST client has no retry logic:**
- Issue: The Binance HTTP client (`pkg/exchange/binance/client.go`) performs raw `http.Do()` with zero retry/backoff logic. All other 5 exchange clients (Bybit, BingX, Bitget, Gate.io, OKX) implement `retryDo()` with exponential backoff (1s, 2s, 4s) and retryable error code maps.
- Files: `pkg/exchange/binance/client.go` (lines 185-225)
- Impact: Any transient Binance API error (rate limit 429, network hiccup, 5xx) causes immediate failure. During entry/exit depth fills, a single failed order placement can break the fill loop. Binance is the most commonly used exchange in the system.
- Fix approach: Add a `retryDo()` wrapper matching the pattern in `pkg/exchange/bitget/client.go:207-235`. Define retryable codes (e.g., -1001 DISCONNECTED, -1015 TOO_MANY_ORDERS). Estimated effort: 1-2 hours.
- Severity: **HIGH**

**Bybit REST client has no retry logic:**
- Issue: Like Binance, the Bybit client defines `retryableCodes` (`pkg/exchange/bybit/client.go:25-29`) but never uses them in the request path. `Get()`/`Post()` call raw HTTP methods directly.
- Files: `pkg/exchange/bybit/client.go` (lines 60-163)
- Impact: Same as Binance — transient errors cause immediate failure. Bybit is a primary exchange.
- Fix approach: Add `retryDo()` matching other adapters. The retryable codes are already defined.
- Severity: **HIGH**

**UpdatePositionFields is not truly atomic (TOCTOU race):**
- Issue: `UpdatePositionFields()` in `internal/database/state.go:207-219` reads a position with `GetPosition()`, applies the mutator, then calls `SavePosition()`. Between the read and write, another goroutine can modify the same position. This is a classic read-modify-write race. The comment says "atomically" but Redis GET + SET is not atomic without WATCH/MULTI or Lua scripting.
- Files: `internal/database/state.go:207-219`
- Impact: Concurrent goroutines (exit, consolidator, funding tracker, SL handler) can overwrite each other's changes. The system has multiple concurrent writers per position: `checkExitsV2`, `trackFunding`, `consolidatePositions`, `handleSLFill`, `schedulePostSettlementZeroCheck`, `handleReduce`. A status change by closePosition can be reverted by updateFundingCollected saving a stale copy.
- Fix approach: Use Redis WATCH/MULTI for optimistic locking, or a Lua script that reads + mutates + writes in a single atomic operation. Alternatively, use per-position Go mutexes (the spot engine already does this with `posMu sync.Map`).
- Severity: **HIGH**

**No `recover()` anywhere in the codebase:**
- Issue: Zero uses of `recover()` in the entire Go codebase. Any goroutine panic will crash the entire process. The engine spawns 15+ background goroutines (run, forwardAlerts, trackFunding, consumeHealthActions, consumeSLFills, runConsolidator, exit goroutines, scheduled timers, balance refresh, allocator reconcile, discovery loop, monitor loop, etc.).
- Files: All goroutine launch sites in `internal/engine/engine.go`, `internal/spotengine/engine.go`, `internal/risk/health.go`, `internal/api/server.go`
- Impact: A nil-pointer dereference or slice-bounds panic in any goroutine kills the live trading process. Positions with open margin will be unmonitored until systemd restarts. Restart takes time and loses in-memory state (BBO cache, depth data, SL index).
- Fix approach: Add `defer func() { if r := recover(); r != nil { ... } }()` at the top of every `go func()` and named goroutine entry. Log the panic + stack trace.
- Severity: **HIGH**

**Gate.io quanto multiplier round-trip bug (known P0):**
- Issue: `PlaceOrder` sends 0 contracts instead of expected 88. The test `TestQuantoMultiplierRoundTrip` fails consistently on both `dev` and `main` branches.
- Files: `pkg/exchange/gateio/adapter.go`, `pkg/exchange/gateio/adapter_test.go:142`
- Impact: Gate.io order placement may send incorrect contract sizes. If the bug only manifests for quanto-multiplied contracts, affected symbols will fail to trade.
- Fix approach: Root cause investigation in `PlaceOrder` contract sizing logic. The quanto multiplier conversion between base units and contract count is broken.
- Severity: **HIGH**

**Config secrets stored in plaintext JSON:**
- Issue: API keys, secret keys, passphrases, Redis password, dashboard password, Telegram token, and AI API key are all stored in `config.json` as plaintext. The file is gitignored but lives on disk with standard permissions (644).
- Files: `config.json` (gitignored), `internal/config/config.go:128-201`
- Impact: Any process on the server with read access to the working directory can read all exchange API keys. A misconfigured backup or log export could leak secrets.
- Fix approach: Move secrets to environment variables (already partially supported), a secrets manager, or at minimum change `config.json` permissions to 600. Consider encrypting at rest.
- Severity: **MEDIUM**

**L3 cross-exchange transfer is not implemented:**
- Issue: The L3 health action handler explicitly logs "cross-exchange transfer is NOT possible" and returns early. Only same-exchange (futures-to-spot) transfers work. The code at `internal/engine/engine.go:796-801` documents this.
- Files: `internal/engine/engine.go:769-818`
- Impact: When margin health reaches L3 on an exchange, the system cannot automatically move funds from a healthier exchange. It falls through to L4 (position reduction) which is more disruptive.
- Fix approach: Implement withdrawal + deposit flow similar to `rebalanceFunds()`. This is complex (requires on-chain confirmation polling) but the rebalance code already does it.
- Severity: **MEDIUM**

**Spot-futures engine hardcoded fee table:**
- Issue: The spot engine uses a `spotFees` map with hardcoded taker fee rates per exchange. These rates change when exchanges run promotions or when VIP tiers change.
- Files: `internal/spotengine/execution.go:264-266` (default 0.0005 fallback), `internal/spotengine/exit_manager.go` (same pattern)
- Impact: Fee calculations in PnL may drift from actual fees. Over time, reported PnL will diverge from actual account P/L.
- Fix approach: Query fee rates from exchange APIs at startup and cache them, or derive from actual trade history.
- Severity: **LOW**

**Binance PM code reverted but still in git history:**
- Issue: Binance Portfolio Margin support was added and then fully reverted in v0.20.1. The code exists in git history but not in the codebase.
- Files: Git history only (no current files affected)
- Impact: None currently. But if someone cherry-picks from history, they will get incompatible code.
- Fix approach: Document in ARCHITECTURE.md that PM was tried and abandoned. Already noted in CLAUDE.md.
- Severity: **INFORMATIONAL**

## Known Bugs

**Gate.io TestQuantoMultiplierRoundTrip failure:**
- Symptoms: `PlaceOrder` sends 0 contracts instead of expected 88 (base units 880 / mult 10)
- Files: `pkg/exchange/gateio/adapter_test.go:142`, `pkg/exchange/gateio/adapter.go`
- Trigger: Any Gate.io symbol that uses a non-1.0 quanto multiplier
- Workaround: The bug may not affect symbols with multiplier=1.0, which are the majority of USDT-M perps

**SL fill channel can drop events:**
- Symptoms: Stop-loss or liquidation fills may be silently dropped if the `slFillCh` buffered channel (capacity 64) is full
- Files: `internal/engine/engine.go:1042-1047`
- Trigger: Rapid succession of order fill callbacks when the consumer goroutine is busy
- Workaround: The channel is consumed by a dedicated goroutine, so drops should be rare. But during mass liquidation events, 64 events could theoretically be exceeded.

## Security Considerations

**WebSocket endpoint has no authentication:**
- Risk: The `/ws` WebSocket endpoint at `internal/api/server.go:117` has no auth middleware. Anyone who can reach the dashboard port receives all real-time data: positions, balances, opportunities, alerts, logs.
- Files: `internal/api/server.go:117`, `internal/api/ws.go:106-121`
- Current mitigation: The `CheckOrigin` upgrader accepts all origins. If the dashboard is bound to a non-localhost address, any network-adjacent client can connect.
- Recommendations: Add token-based authentication to the WebSocket handshake (e.g., pass the Bearer token as a query parameter). Alternatively, restrict to localhost binding.
- Severity: **HIGH**

**Dashboard update endpoint executes shell scripts:**
- Risk: The `/api/update` endpoint at `internal/api/handlers.go:1899-1945` executes `scripts/pull-release.sh` via `exec.Command`, then runs `git fetch` and `git show`. After completion, it calls `os.Exit(1)` to trigger a systemd restart. A compromised dashboard session could trigger arbitrary restarts and potentially replace the binary.
- Files: `internal/api/handlers.go:1898-1945`
- Current mitigation: Protected by `authMiddleware` (requires valid Bearer token). Dashboard password uses `crypto/subtle.ConstantTimeCompare`.
- Recommendations: Add rate limiting to the update endpoint. Consider requiring a confirmation step.
- Severity: **MEDIUM**

**CORS allows all origins:**
- Risk: `Access-Control-Allow-Origin: *` is set on all API endpoints via `internal/api/server.go:163`.
- Files: `internal/api/server.go:161-173`
- Current mitigation: Session tokens are required for mutating endpoints.
- Recommendations: Restrict CORS origin to the actual dashboard URL or localhost.
- Severity: **LOW**

**Dashboard listens on HTTP (no TLS):**
- Risk: The dashboard server uses `http.ListenAndServe` with no TLS configuration. Bearer tokens, passwords, and all position data are transmitted in plaintext.
- Files: `internal/api/server.go:127-138`
- Current mitigation: If accessed only over localhost or a VPN, risk is reduced.
- Recommendations: Add TLS support or put behind a reverse proxy (nginx/caddy) with TLS termination.
- Severity: **MEDIUM**

**When no dashboard password is configured, auth bypassed for reads:**
- Risk: If `DashboardPassword` is empty, all GET endpoints skip authentication entirely. Write endpoints require auth but return a helpful error message.
- Files: `internal/api/auth.go:96-121`
- Current mitigation: The empty-password mode issues tokens to any login request (line 158-167), so writes are still gatekept by the session mechanism.
- Recommendations: Always require a password in production.
- Severity: **LOW**

## Performance Bottlenecks

**History list scan for UpdateHistoryEntry is O(N):**
- Problem: `UpdateHistoryEntry()` reads the entire history list (up to 1000 entries), iterates through all entries, deserializes each one to find a matching ID, then uses `LSet` to update by index.
- Files: `internal/database/state.go:244-269`
- Cause: Redis List does not support key-based lookups. The function scans all 1000 entries every time.
- Improvement path: Use a Redis Hash for history (keyed by position ID) with a sorted set for ordering, or accept the O(N) scan since it runs infrequently (only on PnL reconciliation).
- Severity: **LOW**

**Health monitor polls all exchanges every 30 seconds:**
- Problem: `HealthMonitor.run()` calls `checkAll()` every 30 seconds, which calls `GetFuturesBalance()` + `GetSpotBalance()` + `GetPosition()` for every active position on every exchange.
- Files: `internal/risk/health.go:116-132`, `internal/risk/health.go:146-217`
- Cause: Per-exchange health requires fresh balance and position data. 6 exchanges x (2 balance calls + N position calls) = 12+ API calls per cycle.
- Improvement path: Batch position queries, cache balance data from the 60s refresh cycle instead of re-querying.
- Severity: **LOW**

**Depth exit loop polls at 100ms with synchronous order placement:**
- Problem: The exit depth fill loop in `internal/engine/exit.go:362-607` ticks every 100ms and places synchronous IOC orders. Each order placement involves a REST API call (100-500ms). This means the effective fill rate is one order per 200-600ms.
- Files: `internal/engine/exit.go:362-607`
- Cause: Sequential long-then-short order placement per tick.
- Improvement path: Place long and short orders concurrently within each tick. The entry depth fill may have the same issue.
- Severity: **LOW**

## Fragile Areas

**Exit depth-fill loop with partial close recovery:**
- Files: `internal/engine/exit.go:298-795`
- Why fragile: The loop manages 6 interacting states: depth readiness, gap gating, consecutive fail breaker, timeout, step-size unfillable detection, and SL cancellation. If the long leg fills but the short doesn't, the system logs a warning but does NOT roll back the long fill. The imbalance is deferred to the consolidator.
- Safe modification: Always run the full test matrix after changes. Test with: depth timeout, partial fills, gap rejection, and context cancellation.
- Test coverage: No unit tests for the exit depth fill loop. Only the spot engine execution has tests (`internal/spotengine/execution_test.go`).

**Consolidator orphan detection with BingX 3-miss threshold:**
- Files: `internal/engine/consolidate.go:19-42`, `internal/engine/consolidate.go:100-170`
- Why fragile: BingX's API intermittently returns empty position lists. The consolidator uses a `missCount` map with a threshold of 3 consecutive misses before treating a position as orphaned. If the threshold is wrong, it either auto-closes real positions or fails to detect actual orphans.
- Safe modification: Add integration test with mock exchange that simulates intermittent empty responses.
- Test coverage: No tests for the consolidator at all.

**Spot-futures execution with pending entry recovery:**
- Files: `internal/spotengine/execution.go:100-300`
- Why fragile: The entry flow has a 2-step execution (spot + futures) with a `pendingSpotEntryError` recovery mechanism. If the first leg succeeds but the second fails, a pending position is persisted for manual recovery. The recovery path involves capital commitment for a partially-open position.
- Safe modification: The execution test file is thorough (1504 lines). Follow existing test patterns.
- Test coverage: Good — `internal/spotengine/execution_test.go` covers happy path, partial failures, and pending recovery.

**Position status state machine:**
- Files: `internal/models/` (position statuses), `internal/engine/exit.go`, `internal/engine/engine.go`, `internal/engine/consolidate.go`
- Why fragile: Positions transition through: pending -> partial -> active -> exiting -> closing -> closed. Multiple goroutines can attempt transitions concurrently (exit goroutine, SL handler, health monitor, consolidator, manual close). The `UpdatePositionFields` mutator pattern provides some protection but is not truly atomic (see TOCTOU issue above).
- Safe modification: Always check `fresh.Status` in the mutator before applying changes. Never save a stale position object directly.
- Test coverage: Minimal — only 83 lines of engine tests.

## Scaling Limits

**Redis single-instance persistence:**
- Current capacity: Single Redis instance on DB 2, in-memory. All position state, history, balances, spread history, cooldowns, and locks stored in one Redis.
- Limit: Redis single-instance limits (memory, single-threaded command execution). If Redis goes down, all state is lost unless RDB/AOF persistence is configured.
- Scaling path: Enable Redis persistence (RDB+AOF). Consider Redis Sentinel for HA.

**History list capped at 1000 entries:**
- Current capacity: `historyMaxLen = 1000` in `internal/database/state.go:35`
- Limit: Oldest entries are silently trimmed. With 1-2 trades/day, this covers ~1.5-2 years. With active trading, much less.
- Scaling path: Archive to a persistent database (PostgreSQL, SQLite) before trim.

**Spread history capped at 256 samples per opportunity key:**
- Current capacity: `spreadHistoryMaxLen = 256` in `internal/database/state.go:36`
- Limit: With 10-minute scans, this is ~42 hours of data per opportunity pair. Older data is lost.
- Scaling path: Sufficient for current use. Increase if longer lookbacks are needed.

## Dependencies at Risk

**npm axios compromise (documented in CLAUDE.md):**
- Risk: The npm axios package has been compromised with a malicious dependency. All frontend deps are locked in `web/package-lock.json`.
- Impact: Running `npm install` or `npm update` in the `web/` directory could install malicious code.
- Migration plan: Never run `npm install/update`. Use only `npm ci`. This is already enforced by CLAUDE.md instructions.
- Severity: **HIGH**

**gorilla/websocket maintenance status:**
- Risk: The `gorilla/websocket` library is used by all 6 exchange adapters and the dashboard. The Gorilla project was archived in 2022 but has since been re-maintained.
- Impact: Low immediate risk, but security patches may lag.
- Migration plan: Consider migrating to `nhooyr.io/websocket` or `github.com/coder/websocket` if Gorilla falls behind on security fixes.
- Severity: **LOW**

## Missing Critical Features

**No automated backup/export of Redis state:**
- Problem: All position tracking, trade history, PnL stats, and configuration live in Redis with no automated backup.
- Blocks: Disaster recovery. If the Redis instance is lost, all historical data and active position records are gone.

**No circuit breaker for exchange-wide failures:**
- Problem: If an exchange API becomes completely unavailable (e.g., maintenance), the system continues attempting API calls that fail. There is no exchange-level circuit breaker that pauses trading on a degraded exchange.
- Blocks: Graceful degradation during exchange outages.

**No alerting/notification for CRITICAL log events:**
- Problem: CRITICAL log events (e.g., "NOT fully flat", "manual intervention needed") are only logged and broadcast to the dashboard WebSocket. There is no Telegram notification or other push alert for these events. Telegram notifications exist in the spot engine but not in the perp engine.
- Blocks: Operator awareness of emergencies when not watching the dashboard.

**No rate limiting on dashboard API endpoints:**
- Problem: No request rate limiting on any API endpoint. A runaway frontend bug or external scanner could overwhelm the API server.
- Blocks: Nothing currently, but increases attack surface.

## Test Coverage Gaps

**Perp-perp engine has minimal test coverage:**
- What's not tested: `internal/engine/engine.go` (2883 lines) has only 83 lines of tests covering 2 utility functions. The core execution loop, exit logic, consolidator, SL detection, and all depth-fill code have zero unit tests.
- Files: `internal/engine/engine.go`, `internal/engine/exit.go` (2518 lines), `internal/engine/consolidate.go` (524 lines)
- Risk: Any refactoring or bug fix to the core engine has no regression safety net. The engine handles real money on 6 exchanges.
- Priority: **HIGH**

**No tests for exchange adapter core methods:**
- What's not tested: `PlaceOrder`, `CancelOrder`, `GetFuturesBalance`, `SetLeverage`, `GetFundingRate`, `Withdraw`, and most adapter methods have no unit tests. Only Gate.io, OKX, and Bitget have adapter tests, and those cover limited functionality.
- Files: All `pkg/exchange/*/adapter.go` files (993-1342 lines each)
- Risk: Exchange API response format changes break the adapters silently. Only discovered during live trading.
- Priority: **HIGH**

**No integration tests:**
- What's not tested: End-to-end flows (discovery -> risk -> entry -> monitor -> exit) have no integration tests. The `cmd/livetest/` tool exists for manual exchange testing but is not automated.
- Files: `cmd/livetest/main.go` (manual only)
- Risk: Component interactions are only tested in production.
- Priority: **MEDIUM**

**Database layer has minimal tests:**
- What's not tested: Only `internal/database/locks_test.go` exists. No tests for position CRUD, history, stats, spread history, or balance operations.
- Files: `internal/database/state.go` (589 lines), `internal/database/spot_state.go`
- Risk: Data corruption bugs (e.g., the TOCTOU race in UpdatePositionFields) are not caught by tests.
- Priority: **MEDIUM**

**Config loading has tests but no zero-value guard tests:**
- What's not tested: The `applyJSON` function has extensive `> 0` guards (e.g., `*d.MinHoldTimeHours > 0`) that prevent zero values from overwriting defaults. This was a P0 bug fixed in v0.22.47. The existing `config_test.go` does not specifically test that zero values in JSON do not clobber defaults.
- Files: `internal/config/config.go:575-920`, `internal/config/config_test.go`
- Risk: A regression in zero-value guards could silently reset production config (already happened once).
- Priority: **MEDIUM**

## Operational Risks

**Binary drift monitor auto-restarts with 5s grace:**
- Issue: When the on-disk binary changes, the drift monitor schedules `os.Exit(1)` after 5 seconds. This triggers a systemd restart but gives in-flight depth-fill exits very little time to complete.
- Files: `internal/api/drift_monitor.go:161-185`
- Impact: A `make build` while a position exit is in progress could interrupt the fill loop, leaving a partially-closed position. The consolidator will detect it on restart, but there is a window of unprotected exposure.
- Mitigation: The graceful shutdown path calls `eng.Stop()` which closes `stopCh`, and exit goroutines check for context cancellation. But the 5s window is tight.

**handleUpdate calls os.Exit(1) after 2 seconds:**
- Issue: The dashboard update handler runs `os.Exit(1)` in a goroutine after 2 seconds. This bypasses the graceful shutdown sequence entirely (no `eng.Stop()`, no exchange `Close()`, no Redis `Close()`).
- Files: `internal/api/handlers.go:1941-1944`
- Impact: In-flight operations are abruptly terminated. WebSocket connections are dropped without clean close. Redis state may be inconsistent if a SavePosition was in progress.
- Fix approach: Send SIGTERM to self (like the drift monitor does) instead of `os.Exit(1)` to trigger the graceful shutdown path.

**Log rotation not configured in the application:**
- Issue: The application writes logs to stdout and log files in various `logs/` subdirectories. No built-in log rotation.
- Files: `pkg/utils/logging.go` (342 lines), `internal/*/logs/` directories
- Impact: Log files grow unbounded over time. On a long-running production system, disk can fill up.
- Mitigation: Rely on systemd journal + logrotate, or add file rotation to the logger.

**No healthcheck endpoint:**
- Issue: No `/health` or `/ready` endpoint for load balancers or monitoring systems to check if the service is alive and functional.
- Files: `internal/api/server.go` (no health endpoint registered)
- Impact: External monitoring cannot distinguish between "service is up but degraded" and "service is down".

---

*Concerns audit: 2026-04-01*
