# Phase 15: Drawdown Circuit Breaker — Research

**Researched:** 2026-05-01
**Domain:** Strategy 4 (pricegaptrader) safety / circuit breaker / paper-mode sticky flag
**Confidence:** HIGH (all critical claims verified against existing code in `internal/pricegaptrader/`, `internal/database/pricegap_state.go`, `internal/config/config.go`, `internal/notify/`)

## Summary

Phase 15 adds a realized-PnL drawdown breaker to Strategy 4. The breaker is the safety net behind Phase 14's live-capital ramp: Phase 14 lets capital scale up; Phase 15 stops it when realized losses cross a hard line. Implementation is fully constrained by `15-CONTEXT.md` (D-01 through D-18 locked); this research validates the integration points, confirms hard contracts against the actual codebase, and documents the algorithmic shape.

All anchors named in CONTEXT exist on disk and behave as the CONTEXT describes. The biggest concrete adjustment for the planner: paper-mode is currently read in `execution.go` directly as `t.cfg.PriceGapPaperMode` at four call sites (lines 170, 227, 269, 362) — Phase 15 must introduce a single helper (e.g. `Tracker.IsPaperModeActive()`) and migrate all four reads, otherwise the sticky flag is bypassable.

**Primary recommendation:** Build the breaker as a `BreakerController` daemon mirroring `RampController` + `Reconciler`: 5-field Redis HASH (`pg:breaker:state`), capped LIST (`pg:breaker:trips`), `time.Ticker` daemon spawned by `Tracker.Start`, with a single `IsPaperModeActive()` chokepoint replacing the four direct `cfg.PriceGapPaperMode` reads. Three new pg-admin subcommands (`recover`, `test-fire`, `show`), three new API handlers, and a Breaker subsection inside the existing Phase 14 widget on the Pricegap-tracker tab.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Evaluation Cadence + 24h Window (Area A)**
- **D-01:** 5-min dedicated ticker — new goroutine + `time.Ticker` on `Tracker`, decoupled from reconcile/scanner/promotion/ramp daemons. Spawned in `Tracker.Start`, cancelled via `context.Context`.
- **D-02:** Aggregate from existing Phase 14 index. On each tick: `SMEMBERS pg:positions:closed:{today}` + `SMEMBERS pg:positions:closed:{yesterday}`, then `GET pg:position:{id}` for each, filter `close_ts >= now - 24h`, sum `RealizedPnLUSDT`. No new key.
- **D-03:** Whole-tick blackout suppression. If wall-clock minute is in `[:04, :05:30)`, the entire tick is a no-op — no PnL fetch, no strike state mutation.
- **D-04:** Pending Strike-1 survives blackout-suppressed ticks. Blackout = "skip evaluation", not "reset state".

**Strike State + Threshold + Sticky Flag (Area B)**
- **D-05:** `pg:breaker:state` HASH (Redis-persisted), 5 fields: `pending_strike` (0|1), `strike1_ts` (unix-ms or 0), `last_eval_ts`, `last_eval_pnl_usdt`, `paper_mode_sticky_until` (unix-ms; 0 = not sticky, `math.MaxInt64` = sticky-until-operator-clears).
- **D-06:** `PriceGapDrawdownLimitUSDT` is absolute USDT (single negative value). Trips when `sum(realized_pnl_24h) < limit`. Does NOT auto-scale with ramp stage.
- **D-07:** `PaperModeStickyUntil` is int64 unix-ms with sentinel. `0` = not sticky; `math.MaxInt64` = sticky-until-operator-clears. Stored on `pg:breaker:state`. Persisted **before** any other trip side-effect (atomicity anchor).
- **D-08:** Strike-1 clears on PnL recovery — a tick that sees `realized_pnl_24h >= threshold` between Strike-1 and Strike-2 clears `pending_strike=0`.

**Recovery UX + Auto-Disable Scope (Area C)**
- **D-09:** Recovery dual-surface (pg-admin + dashboard). `pg-admin breaker recover --confirm` (typed phrase `RECOVER`); Dashboard "Recover" button (modal requiring typed `RECOVER`). Both emit identical audit events.
- **D-10:** New `paused_by_breaker` boolean on each `Candidate` registry entry — distinct from operator-set `disabled`. Entry path checks BOTH (`disabled || paused_by_breaker` → reject).
- **D-11:** Auto re-enable on recovery — clears `paused_by_breaker=false` on all candidates AND zeroes `paper_mode_sticky_until`. Operator-set `disabled=true` candidates stay disabled.
- **D-12:** Typed-phrase confirmation on every mutation surface (`RECOVER` / `TEST-FIRE`).

**Synthetic Test Fire + Observability Surface (Area D)**
- **D-13:** Test fire dual-surface (pg-admin + dashboard, symmetric with D-09).
- **D-14:** Test fire defaults to a real trip; `--dry-run` flag opts into simulation. Trip records distinguish `source: "live" | "test_fire" | "test_fire_dry_run"`.
- **D-15:** Trip side-effect ordering: (1) write `paper_mode_sticky_until=MaxInt64` first; (2) `LPUSH` trip record; (3) write `paused_by_breaker=true` on candidates; (4) Telegram critical alert; (5) WS broadcast. Steps 2–5 are best-effort.
- **D-16:** Dashboard widget extends Phase 14 widget — Breaker subsection on the Pricegap-tracker tab. Phase 16 PG-OPS-09 absorbs.
- **D-17:** Telegram alert is full-context single message, **critical bucket** (bypasses allowlist).
- **D-18:** `pg:breaker:trips` is a LIST capped at 500 entries. Each entry JSON: `{trip_ts, trip_pnl_usdt, threshold, ramp_stage, paused_candidate_count, recovery_ts (nullable), recovery_operator (nullable), source}`. `LPUSH` on trip; `LSET` on recovery to backfill.

**Hard Contracts (locked by ROADMAP/REQUIREMENTS):**
- REALIZED PnL only — no MTM, no unrealized.
- Rolling 24h window — not calendar-day, not since-last-reconcile.
- `PriceGapDrawdownLimitUSDT` — exact field name (PG-LIVE-02).
- Auto-revert via sticky `PaperModeStickyUntil` flag.
- Auto-disable any open candidate.
- Telegram critical alert + WS broadcast.
- `pg:breaker:trips` log key.
- Bybit `:04–:05:30` blackout suppression.
- Two-strike rule, ≥5 min apart.
- Recovery requires explicit operator action; sticky does NOT auto-clear on restart or page reload.
- Synthetic test fire exercises full cycle without engine restart.
- Module isolation: all new code in `internal/pricegaptrader/`. No imports of `internal/engine` or `internal/spotengine`.
- Default OFF: every new config flag (`PriceGapBreakerEnabled` default false, `PriceGapDrawdownLimitUSDT` default 0).
- `pg:*` Redis namespace.
- Commit + VERSION + CHANGELOG together.

### Claude's Discretion

- Internal struct names: `BreakerController`, `RealizedPnLAggregator`, `BreakerStateStore` (Phase 14 used `RampController`, `Reconciler`).
- Whether the 5-min ticker is configurable (`PriceGapBreakerIntervalSec`, default 300, validated [60, 3600]) — **research recommends configurable** with safety floor.
- Whether eval-tick logs `realized_pnl_24h` always or only-on-breach — **research recommends only-on-breach** for cardinality control, plus periodic `INFO` heartbeat every N ticks.
- Boot-time guard: if `PriceGapBreakerEnabled=true` and `pg:breaker:state` is missing, **initialize fresh** (`pending_strike=0`, `paper_mode_sticky_until=0`). Different from Phase 14 ramp.
- Atomicity primitive choice for D-15 — sequential best-effort writes with Step 1 single-key HSET is sufficient (no Lua needed).
- Locale strings for new buttons + widget labels — both `en.ts` and `zh-TW.ts`. Typed phrases (`RECOVER`, `TEST-FIRE`) are magic strings, do NOT translate.
- Default `PriceGapDrawdownLimitUSDT` for fresh installs — `0` (effectively armed-but-never-trips).

### Deferred Ideas (OUT OF SCOPE)

- Auto-recovery from drawdown breaker (sticky flag MUST be operator-cleared).
- MTM-based breaker (realized PnL only).
- Per-symbol or per-pair breakers (single global breaker for v2.2).
- Calendar-day breaker (rolling 24h mandated).
- Breaker thresholds that scale with ramp stage (absolute USDT chosen per D-06).
- Top-level Pricegap dashboard tab consolidation (Phase 16 PG-OPS-09).
- Realized-slippage machine-zero fix (Phase 16 PG-FIX-01).
- Per-fill Telegram alerts on breaker trip (descoped from v2.2).
- Audit log of recovery actors beyond operator name + timestamp.
- Continuous-curve breaker thresholds.
- Auto-tune of threshold from PnL history.
- Cross-strategy breaker (perp-perp + spot-futures + pricegap shared circuit) — v3.0.
- Test fire bypassing the typed-phrase requirement.
- Multi-tenant breaker state (single-process system).

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PG-LIVE-02 | Drawdown circuit breaker: rolling-24h realized PnL, `PriceGapDrawdownLimitUSDT`, sticky `PaperModeStickyUntil`, auto-disable open candidates, Telegram critical + WS broadcast, `pg:breaker:trips`, Bybit `:04–:05:30` blackout suppression, two-strike rule (≥5 min apart), operator recovery (sticky no auto-clear) | Architecture (BreakerController + 5-field HASH + capped LIST), Integration map (paper-mode helper, candidate registry pause flag, dashboard widget extension), Algorithm (two-strike state machine + blackout gate), Validation (synthetic test fire) — all sections below |

</phase_requirements>

## Project Constraints (from CLAUDE.local.md)

[VERIFIED: read-on-disk]

- **DO NOT modify `config.json`** at repo root — live runtime config with API keys and tuned trading params. Phase 15 config schema additions go through `internal/config/config.go` Go struct + `applyJSON` parsing; runtime mutations land via `POST /api/config` which writes a `.bak` backup.
- **npm lockdown** — no new npm packages. Frontend changes use existing React + Recharts only via `npm ci`.
- **Build order** — frontend `npm run build` BEFORE Go `go build` because of `go:embed`. Document in plan.
- **Module boundaries** — all new code in `internal/pricegaptrader/`. No imports of `internal/engine` or `internal/spotengine`. `pkg/exchange/` and `pkg/utils/` are standalone (no `internal/` imports).
- **Default OFF master toggle** — every new risk/strategy feature requires (1) config boolean switch (default OFF), (2) dashboard UI toggle, (3) persisted to `config.json` via `POST /api/config`. Phase 15: `PriceGapBreakerEnabled` (bool, default false).
- **i18n lockstep** — all new user-facing strings in BOTH `web/src/i18n/en.ts` and `web/src/i18n/zh-TW.ts`. Typed phrases (`RECOVER`, `TEST-FIRE`) are magic strings — do NOT translate.
- **Display timezone** — Asia/Taipei (UTC+8) for all dashboard time formatting (Telegram alert window-start/end timestamps in CONTEXT D-17).
- **VERSION + CHANGELOG together** — every commit must update both `VERSION` and `CHANGELOG.md`.
- **Live trading** — Phase 15 must NOT disturb existing perp-perp engine (`internal/engine/`) or spot-futures engine (`internal/spotengine/`). All work additive inside `internal/pricegaptrader/`.

## Standard Stack

[VERIFIED: codebase grep] — Phase 15 reuses the existing Strategy 4 stack. **Zero new external dependencies.**

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `time.Ticker` / `context.Context` | (Go 1.26+) | 5-min eval daemon goroutine | Direct precedent: `Reconciler` uses identical pattern in `internal/pricegaptrader/reconciler.go` |
| Go stdlib `sync.RWMutex` | (Go 1.26+) | Serialize Redis HASH read/write on `pg:breaker:state` | Direct precedent: `RampController` + `CandidateRegistry` |
| `internal/database` Redis client (`go-redis`) | (`go.mod`) | `HSet`/`HGetAll`/`LPush`/`LTrim`/`LSet`/`SMembers` | Existing `*database.Client` already exposes all needed primitives |
| Go stdlib `encoding/json` | (Go 1.26+) | Trip-record JSON marshalling | Same shape as Phase 14 `RampEvent` JSON in `pg:ramp:events` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/notify.TelegramNotifier` | (existing) | Critical-bucket alert dispatch | `NotifyPriceGapBreakerTrip` and `NotifyPriceGapBreakerRecovery` (new methods) |
| `*api.Hub` (WS) | (existing) | Broadcast `pg.breaker.trip` / `pg.breaker.recover` | Same wiring as Phase 12 `RedisWSPromoteSink` (Server.Hub() accessor — Plan 12-03 D-15 boundary precedent) |
| React + Recharts (frontend) | (`web/package.json` locked) | Breaker subsection in Pricegap-tracker tab | Extends Phase 14 RampReconcileSection — no new components needed beyond a Modal + status badge |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| 5-min `time.Ticker` (D-01) | Piggyback on scanner/reconcile cycle | **Rejected by D-01.** Decoupling buys clean lifecycle and predictable strike spacing. Verified: scanner runs at `PriceGapDiscoveryIntervalSec` default 300; ramp Eval is daily; reconcile fires UTC 00:30. Sharing any of them couples the breaker to unrelated cadence. |
| Lua atomic Multi-step trip (D-15) | Single Lua script for all 5 steps | Sequential is fine — Step 1 is a single-key `HSet` (atomic). Lua adds dependency complexity without buying anything because steps 2–5 are best-effort by design. |
| New top-level Redis namespace | Use `pg:breaker:*` (chosen) | `pg:*` is the established Strategy 4 namespace. `pg:breaker:state` and `pg:breaker:trips` slot in cleanly. |

**Installation:** No new dependencies. The phase compiles against the existing `go.mod` and `web/package-lock.json`.

**Version verification:** N/A — this is internal Go code; no third-party packages introduced.

## Architecture Patterns

### Recommended Project Structure
```
internal/pricegaptrader/
├── breaker_controller.go         # NEW — BreakerController struct + Run(ctx) daemon + Trip/Recover/Show
├── breaker_controller_test.go    # NEW — TDD: two-strike, blackout, sticky-flag, kill -9 mid-strike
├── breaker_aggregator.go         # NEW — RealizedPnLAggregator (rolling 24h sum from pg:positions:closed:{date})
├── breaker_aggregator_test.go    # NEW
├── breaker_test_fire.go          # NEW — synthetic trip path (CONTEXT D-14)
├── notify.go                     # EDIT — add NotifyPriceGapBreakerTrip + NotifyPriceGapBreakerRecovery to Notifier interface; NoopNotifier stubs
├── registry.go                   # EDIT — add `paused_by_breaker bool` to Candidate; entry path reads `disabled || paused_by_breaker`
├── tracker.go                    # EDIT — spawn BreakerController.Run goroutine in Start; add IsPaperModeActive(ctx) helper
├── execution.go                  # EDIT — replace 4 direct `t.cfg.PriceGapPaperMode` reads (lines 170, 227, 269, 362) with `t.IsPaperModeActive(ctx)`

internal/database/
└── pricegap_state.go             # EDIT — add LoadBreakerState/SaveBreakerState/AppendBreakerTrip/UpdateBreakerTripRecovery; add 2 namespace constants

internal/notify/
└── pricegap_breaker.go           # NEW — TelegramNotifier.NotifyPriceGapBreakerTrip + Recovery (CRITICAL bucket)

internal/config/
└── config.go                     # EDIT — add PriceGapBreakerEnabled, PriceGapDrawdownLimitUSDT, PriceGapBreakerIntervalSec; extend validatePriceGapLive

internal/api/
└── pricegap_breaker_handlers.go  # NEW — POST /api/pg/breaker/recover + test-fire; GET /api/pg/breaker/state

cmd/pg-admin/
└── breaker.go                    # NEW — `breaker recover|test-fire|show` subcommands

web/src/
├── components/PriceGap/BreakerSubsection.tsx  # NEW — append to existing RampReconcileSection in Pricegap-tracker tab
├── i18n/en.ts                    # EDIT — add breaker keys
├── i18n/zh-TW.ts                 # EDIT — add same keys (lockstep)
```

### Pattern 1: Daemon goroutine + time.Ticker [VERIFIED: existing code]
**What:** Spawn breaker as goroutine inside `Tracker.Start`; eval loop wakes on `ticker.C` and on `ctx.Done()` exits.
**When to use:** Every periodic Strategy 4 daemon (reconcile, ramp, scanner) follows this shape.
**Example pattern (mirrors `Reconciler` in `internal/pricegaptrader/reconciler.go`):**
```go
// internal/pricegaptrader/breaker_controller.go (sketch)
func (bc *BreakerController) Run(ctx context.Context) {
    interval := bc.cfg.PriceGapBreakerIntervalSec
    if interval <= 0 { interval = 300 }
    ticker := time.NewTicker(time.Duration(interval) * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case t := <-ticker.C:
            if err := bc.evalTick(ctx, t); err != nil {
                bc.log.Warn("breaker eval tick: %v", err)
            }
        }
    }
}
```

### Pattern 2: 5-field Redis HASH (`pg:breaker:state`) [VERIFIED: Phase 14 precedent]
**Source:** `internal/database/pricegap_state.go` lines 407–485 (`SavePriceGapRampState` / `LoadPriceGapRampState`) — exact precedent.
**Shape:**
```
HSET pg:breaker:state
  pending_strike            "0" | "1"
  strike1_ts                "<unix_ms>"        # 0 if no pending strike
  last_eval_ts              "<unix_ms>"
  last_eval_pnl_usdt        "<decimal>"
  paper_mode_sticky_until   "0" | "<unix_ms>" | "9223372036854775807"   # MaxInt64
```
All fields stored as decimal strings (mirrors RampState convention). Pipeline `HSet` for atomic write (matches Phase 14 D-09).

### Pattern 3: Capped LIST trip log (`pg:breaker:trips`) [VERIFIED: Phase 14 precedent]
**Source:** `internal/database/pricegap_state.go` lines 487–502 (`AppendPriceGapRampEvent`) — same shape (`LPUSH` + `LTRIM` to 500). On recovery, `LSET` on the most-recent entry to backfill `recovery_ts` + `recovery_operator`.

### Pattern 4: Single paper-mode chokepoint helper [DESIGN DECISION — required]
[VERIFIED: existing reads in execution.go: lines 170, 227, 269, 362]

**The principal correctness property of this phase.** Today, paper-mode is read directly as `t.cfg.PriceGapPaperMode` at four call sites in `internal/pricegaptrader/execution.go`:
- L170 — entry path order placement chokepoint
- L227 — paper-mode synth-fill chokepoint (Plan 09-03 Pattern 2)
- L269 — BingX-specific paper guard
- L362 — close-leg paper-mode chokepoint (D-12 immutable mode)

Phase 15 adds a single helper:
```go
// Tracker method (tracker.go, NEW)
func (t *Tracker) IsPaperModeActive(ctx context.Context) (bool, error) {
    if t.cfg.PriceGapPaperMode { return true, nil }
    state, exists, err := t.breakerStore.LoadBreakerState()
    if err != nil { return true, err } // fail-safe to paper on Redis error
    if exists && state.PaperModeStickyUntil != 0 &&
       time.Now().UnixMilli() < state.PaperModeStickyUntil {
        return true, nil
    }
    return false, nil
}
```

ALL 4 reads in `execution.go` migrate to `t.IsPaperModeActive(ctx)`. The grep `grep -n "cfg.PriceGapPaperMode" internal/pricegaptrader/execution.go` after the change MUST return zero matches — this becomes a static check (mirrors `scanner_static_test.go`).

The fail-safe-to-paper rule on Redis error is deliberate: a Redis outage during a real trip must not silently allow live trading; preferring false positives (slow trade) over false negatives (live during outage) matches the asymmetric two-strike rule.

### Anti-Patterns to Avoid
- **Bypassing the chokepoint** — adding any new `cfg.PriceGapPaperMode` read outside `IsPaperModeActive`. Static test must catch this.
- **Reordering D-15 trip side-effects** — sticky flag MUST be persisted before steps 2–5. Comment block + unit test that fails steps 2–5 deliberately.
- **Auto-clearing sticky on Tracker restart** — boot path must NEVER zero the sticky field. Boot guard: if `PriceGapBreakerEnabled=true` and `pg:breaker:state` missing, write a fresh state with `pending_strike=0` AND `paper_mode_sticky_until=0`. If state exists, load as-is (sticky survives restart by definition).
- **Dropping Strike-1 on blackout tick** — blackout suppresses evaluation, NOT state. The pending-strike field carries through.
- **Mutex-protecting the Redis HASH read in the hot path** — paper-mode helper is read every entry/close; cache the loaded state with short TTL (~30s) or read-through with metric to detect Redis-call cardinality blow-up.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Daemon lifecycle | Custom goroutine + bare `for { time.Sleep }` loop | `time.Ticker` + `ctx.Done()` (existing Reconciler/Promotion pattern) | Reconciler has shutdown semantics already debugged (Phase 14 P02). |
| Redis HASH atomic write | Multiple `HSet` calls without pipeline | `pipe.HSet(ctx, key, map[string]any{...})` (mirrors `SavePriceGapRampState`) | The existing pattern is verified across Phase 14 unit tests including a `kill -9` test. |
| Capped trip log | Manual `LRANGE` + filter | `LPush` + `LTrim` to 500 (mirrors `pg:ramp:events`, `pg:history`) | Cap is enforced atomically with `LTrim` in the same `TxPipelined`. |
| Telegram critical-bucket dispatch | New filter logic | `TelegramNotifier` already has critical-bypass for `NotifyPriceGapReconcileFailure` (`internal/notify/pricegap_reconcile.go` L83) — copy that shape | Phase 14's reconcile-failure path is the live precedent for fire-now alerts. |
| WS broadcast wiring | New WS hub injection into `pricegaptrader` | Use the Server.Hub() public accessor (Plan 12-03 D-17) — wire concrete `*api.Hub` into a narrow `BreakerWSBroadcaster` interface | Module isolation rule preserved; same shape Phase 12 used for promote events. |
| Typed-phrase confirmation | Custom challenge-response | Read JSON body field `confirmation_phrase`, compare to literal `"RECOVER"` / `"TEST-FIRE"` | Magic-string literal in API + CLI; no localization, no hashing. Trivial implementation, deliberate replay-resistance pattern. |
| Symbol normalization in PnL aggregation | Re-implement `BTCUSDT` canonicalization | Already done — `pg:position:{id}` keys are by ID; Symbol on the position is canonical from entry path | Phase 14 closed-position SADD writes the canonical position. |

**Key insight:** Phase 14 already built every primitive Phase 15 needs. The breaker is functionally a copy-paste of Reconciler's daemon shape + RampController's HASH persistence + a thin two-strike state machine + a paper-mode chokepoint refactor. **Risk is in the refactor (paper-mode helper migration) and in the test-fire integration test, not in the breaker mechanics.**

## Common Pitfalls

### Pitfall 1: Bypassed paper-mode read
**What goes wrong:** Future code adds `if t.cfg.PriceGapPaperMode` somewhere new and bypasses the sticky flag.
**Why it happens:** Existing call sites in `execution.go` (L170, L227, L269, L362) and tests in `paper_mode_test.go`, `paper_to_live_test.go`, `paper_close_leg_test.go` already use the direct read. Engineers will copy that pattern.
**How to avoid:** Add a `breaker_static_test.go` regex-grep test that fails the build if any `internal/pricegaptrader/*.go` file (excluding `tracker.go` where `IsPaperModeActive` is defined and the test files for the helper itself) contains `cfg.PriceGapPaperMode` outside test files. Same shape as `scanner_static_test.go`.
**Warning signs:** Code review finds a new branch checking paper-mode directly; static test green but the helper isn't called.

### Pitfall 2: Strike state lost on `kill -9` mid-strike
**What goes wrong:** Strike-1 fires, process killed, restart, Strike-2 condition hit but pending state lost — never trips.
**Why it happens:** State held only in memory.
**How to avoid:** Persist `pg:breaker:state` HASH IMMEDIATELY when Strike-1 fires (before returning from `evalTick`). Unit test (mirrors Phase 14 RampController kill-9 test): set state, simulate restart by recreating BreakerController, verify `pending_strike=1` carries.
**Warning signs:** `evalTick` ends without a Save call.

### Pitfall 3: Race between trip side-effects and concurrent entry
**What goes wrong:** Trip writes `paper_mode_sticky_until=MaxInt64` at step 1; before step 3 (`paused_by_breaker=true`), an in-flight entry tick checks the registry, sees not paused, attempts to open a position. The entry path reads `IsPaperModeActive(ctx)` which now returns true → entry is paper-mode-stamped, not blocked. Good outcome (sticky flag is the safety property). But the Telegram alert at step 4 does not yet know about that entry.
**Why it happens:** Steps 1–5 are not atomic.
**How to avoid:** D-15 step ordering is correct: sticky-flag-first ensures any concurrent entry becomes paper-mode-stamped, not live. The CandidateRegistry pause flag is a defense-in-depth second layer (D-10 entry path: `disabled || paused_by_breaker`). Document that the sticky-flag is the **load-bearing** safety property; pause flag is a redundant guard for operator clarity.
**Warning signs:** Code review proposes reordering D-15 steps. Reject.

### Pitfall 4: Blackout window check confusion (UTC vs local)
**What goes wrong:** `inBybitBlackout(now)` in `internal/pricegaptrader/scanner.go:539` uses `now.Minute()` directly (no timezone conversion). Bybit's blackout is wall-clock minute-of-hour, identical in any timezone.
**How to avoid:** REUSE `inBybitBlackout(now time.Time) bool` from `scanner.go` directly — make it a package-level exported function or just call it. Do NOT write a parallel implementation. Verified shape: returns true if `now.Minute() == 4`, or `now.Minute() == 5 && now.Second() < 30`.
**Warning signs:** New blackout helper in `breaker_controller.go`. Reject.

### Pitfall 5: 24h aggregation reads stale closed-position SET
**What goes wrong:** A position closed >24h ago is still in `pg:positions:closed:{yesterday}` but should not contribute to the rolling 24h window.
**Why it happens:** D-02 says "filter `close_ts >= now - 24h`" — easy to forget.
**How to avoid:** Aggregator implementation:
1. Compute `today = now.UTC().Format("2006-01-02")` and `yesterday = (now-24h).UTC().Format("2006-01-02")`.
2. `SMEMBERS` both keys, union.
3. For each ID: `LoadPriceGapPosition(id)` → check `pos.ExchangeClosedAt` (preferred per `reconciler.go:255`) or `pos.ClosedAt` fallback.
4. **Only include if** `closeTs >= now.Add(-24h)`.
5. Sum `pos.RealizedPnL` (field name `RealizedPnL` per `internal/models/pricegap_position.go:84`).
**Warning signs:** Aggregator sums all members of both SETs without timestamp filter.

### Pitfall 6: Missing daily SET key (yesterday)
**What goes wrong:** First-day operation — `pg:positions:closed:{yesterday}` doesn't exist; SMEMBERS returns nil; aggregator may panic on nil.
**Why it happens:** Phase 14 P01 SADDs are best-effort and only land when positions actually close.
**How to avoid:** Use `redis.Nil` handling — treat missing SET as empty list. Aggregator must be resilient to: both keys missing, only one missing, both empty. Phase 14 closed-position SET has no TTL (per Phase 14 `[Phase 14 P01]` decision), but a *new* day with zero closes still won't have the key.
**Warning signs:** No test for "fresh database, breaker enabled, first eval tick".

### Pitfall 7: Test fire defaults to dry-run (D-14 trap)
**What goes wrong:** Operator runs `pg-admin breaker test-fire --confirm` thinking it's safe, but D-14 says default is **real trip**.
**Why it happens:** Standard CLI conventions usually have explicit `--apply` or `--execute` for dangerous ops.
**How to avoid:** D-14 is the locked decision (real trip is the default). Mitigation: typed-phrase requirement (`TEST-FIRE`) + post-execution Telegram + dashboard tripped state make accidents recoverable. Document loudly in `pg-admin breaker test-fire --help`. Reinforce in Telegram alert with `source: test_fire`.
**Warning signs:** Help text not warning the operator about default behavior.

### Pitfall 8: Redis-down silent paper-mode (defense-in-depth)
**What goes wrong:** Redis dies; `LoadBreakerState()` returns error; engine has no breaker state; in absence of safe handling, code reads `cfg.PriceGapPaperMode` and proceeds to live trading.
**How to avoid:** `IsPaperModeActive` returns `(true, err)` on Redis error — fail-safe to paper. Log + alert (Telegram critical) when Redis errors persist > 1 min. Operator can disable `PriceGapBreakerEnabled` to fall back to legacy paper/live config-only behavior, but that's an explicit decision.
**Warning signs:** Helper returns the cfg flag on Redis error.

### Pitfall 9: i18n drift (en.ts vs zh-TW.ts)
**What goes wrong:** New string added to `en.ts` but not `zh-TW.ts`; type-check fails.
**How to avoid:** `TranslationKey` type is derived from the EN locale file (per CLAUDE.local.md i18n section); compile-time safety prevents missing keys. The check enforces lockstep but does NOT enforce non-default values — verify `zh-TW.ts` has actual translated strings, not English copies.
**Warning signs:** zh-TW values are English text.

## Code Examples

### Two-strike eval tick (algorithm core) [DESIGN]
```go
// internal/pricegaptrader/breaker_controller.go (sketch — exact field names per CONTEXT D-05)
func (bc *BreakerController) evalTick(ctx context.Context, now time.Time) error {
    if !bc.cfg.PriceGapBreakerEnabled { return nil }

    // D-03: whole-tick blackout suppression — reuse existing helper from scanner.go
    if inBybitBlackout(now) { return nil }

    state, _, err := bc.store.LoadBreakerState()
    if err != nil { return fmt.Errorf("load: %w", err) }

    // If sticky already non-zero, breaker has tripped — nothing to evaluate.
    if state.PaperModeStickyUntil != 0 { return nil }

    pnl, err := bc.aggregator.Realized24h(ctx, now)
    if err != nil { return fmt.Errorf("aggregate: %w", err) }

    state.LastEvalTs = now.UnixMilli()
    state.LastEvalPnLUSDT = pnl

    threshold := bc.cfg.PriceGapDrawdownLimitUSDT // negative number
    inBreach := pnl < threshold

    if !inBreach {
        // D-08: PnL recovered — clear pending strike if any.
        if state.PendingStrike == 1 {
            state.PendingStrike = 0
            state.Strike1Ts = 0
            bc.log.Info("breaker: PnL recovered (24h=%.2f >= threshold=%.2f), strike cleared", pnl, threshold)
        }
        return bc.store.SaveBreakerState(state)
    }

    // In breach.
    if state.PendingStrike == 0 {
        // First strike.
        state.PendingStrike = 1
        state.Strike1Ts = now.UnixMilli()
        bc.log.Warn("breaker: STRIKE 1 (24h=%.2f < threshold=%.2f), awaiting confirmation tick ≥5min away", pnl, threshold)
        return bc.store.SaveBreakerState(state)
    }

    // Pending strike. Spec requires "≥5 min apart" — the 5-min ticker enforces this naturally,
    // but enforce defensively in case of clock jitter or interval misconfig.
    elapsed := now.UnixMilli() - state.Strike1Ts
    if elapsed < 5*60*1000 {
        // Defensive: should not happen with default 300s interval.
        return bc.store.SaveBreakerState(state)
    }

    // Strike 2 — TRIP (D-15 ordering).
    return bc.trip(ctx, state, pnl, "live")
}
```

### Trip function (D-15 ordering) [DESIGN]
```go
// D-15 ordering — sticky flag FIRST is the load-bearing safety property.
func (bc *BreakerController) trip(ctx context.Context, state BreakerState, pnl24h float64, source string) error {
    rampStage := 0
    if bc.ramp != nil { rampStage = bc.ramp.Snapshot().CurrentStage }

    // STEP 1: Sticky flag persisted FIRST. Any partial failure beyond this leaves engine in safest state.
    state.PendingStrike = 0
    state.Strike1Ts = 0
    state.PaperModeStickyUntil = math.MaxInt64
    if err := bc.store.SaveBreakerState(state); err != nil {
        // Step 1 failure is critical — return without proceeding to steps 2-5.
        return fmt.Errorf("trip: save sticky flag: %w", err)
    }

    // Steps 2-5 are best-effort.

    // STEP 2: LPush trip record.
    record := BreakerTripRecord{
        TripTs: time.Now().UnixMilli(),
        TripPnLUSDT: pnl24h,
        Threshold: bc.cfg.PriceGapDrawdownLimitUSDT,
        RampStage: rampStage,
        Source: source,
    }
    pausedCount, err := bc.pauseAllOpenCandidates(ctx) // STEP 3
    if err != nil { bc.log.Warn("trip: pause candidates: %v", err) }
    record.PausedCandidateCount = pausedCount

    if err := bc.store.AppendBreakerTrip(record); err != nil {
        bc.log.Warn("trip: append trip log: %v", err)
    }

    // STEP 4: Telegram critical alert.
    bc.notifier.NotifyPriceGapBreakerTrip(record)

    // STEP 5: WS broadcast.
    if bc.wsBroadcaster != nil {
        bc.wsBroadcaster.Broadcast("pg.breaker.trip", record)
    }
    return nil
}
```

### Aggregator: rolling 24h realized PnL [DESIGN]
```go
// internal/pricegaptrader/breaker_aggregator.go (sketch)
func (a *RealizedPnLAggregator) Realized24h(ctx context.Context, now time.Time) (float64, error) {
    today := now.UTC().Format("2006-01-02")
    yesterday := now.Add(-24*time.Hour).UTC().Format("2006-01-02")
    cutoff := now.Add(-24*time.Hour)

    idsToday, err := a.store.GetPriceGapClosedPositionsForDate(today)
    if err != nil { return 0, fmt.Errorf("smembers today: %w", err) }
    idsYesterday, err := a.store.GetPriceGapClosedPositionsForDate(yesterday)
    if err != nil { return 0, fmt.Errorf("smembers yesterday: %w", err) }

    seen := make(map[string]struct{}, len(idsToday)+len(idsYesterday))
    sum := 0.0
    for _, id := range append(idsToday, idsYesterday...) {
        if _, dup := seen[id]; dup { continue }
        seen[id] = struct{}{}
        pos, ok, err := a.store.LoadPriceGapPosition(id)
        if err != nil || !ok { continue }
        closeTs := pos.ExchangeClosedAt
        if closeTs.IsZero() { closeTs = pos.ClosedAt }
        if closeTs.Before(cutoff) { continue } // Pitfall 5
        sum += pos.RealizedPnL
    }
    return sum, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Pre-Phase-14: paper-mode toggle was the only safety net for Strategy 4 | Phase 14 added daily reconcile + ramp controller | 2026-04-30 (v0.37.0) | Live capital is now staged 100→500→1000 USDT/leg with clean-day ratchet — the breaker arms a separate axis (drawdown), perpendicular to the ramp |
| Direct `cfg.PriceGapPaperMode` reads scattered across `execution.go` | Single chokepoint helper `Tracker.IsPaperModeActive(ctx)` | Phase 15 (this phase) | Sticky flag becomes uncircumventable; future paper-mode logic centralizes |
| No drawdown defense beyond ramp asymmetric ratchet | Two-strike rolling-24h breaker with sticky flag | Phase 15 | Asymmetric protection: ramp punishes single bad day; breaker punishes sustained 24h drawdown |

**Deprecated/outdated:**
- None — Phase 15 is purely additive.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (current project standard, verified by 21 existing `_test.go` files in `internal/pricegaptrader/`) |
| Config file | None — `go test ./...` runs everything; project precedent uses `t.Run` subtests |
| Quick run command | `go test ./internal/pricegaptrader/... -run Breaker -count=1` |
| Full suite command | `go test ./internal/pricegaptrader/... ./internal/database/... ./internal/api/... ./internal/notify/... ./cmd/pg-admin/... -count=1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PG-LIVE-02 | Two-strike: single in-breach tick does NOT trip | unit | `go test ./internal/pricegaptrader -run TestBreaker_SingleStrike_NoTrip -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | Two-strike: two in-breach ticks ≥5 min apart trips | unit | `go test ./internal/pricegaptrader -run TestBreaker_TwoStrike_Trips -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | PnL recovery between strikes clears pending (D-08) | unit | `go test ./internal/pricegaptrader -run TestBreaker_RecoveryClearsPendingStrike -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | Bybit blackout suppresses entire eval tick (D-03) | unit | `go test ./internal/pricegaptrader -run TestBreaker_BlackoutSuppression -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | Pending strike survives blackout-suppressed tick (D-04) | unit | `go test ./internal/pricegaptrader -run TestBreaker_PendingSurvivesBlackout -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | Sticky flag survives `kill -9` mid-strike (D-05) | unit | `go test ./internal/pricegaptrader -run TestBreaker_StateSurvivesRestart -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | Trip side-effect ordering — Step 1 succeeds even if 2-5 fail (D-15) | unit | `go test ./internal/pricegaptrader -run TestBreaker_TripOrdering_StickyFirstWhenStepsFail -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | Sticky flag forces paper-mode in `IsPaperModeActive` | unit | `go test ./internal/pricegaptrader -run TestTracker_IsPaperModeActive_Sticky -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | All `cfg.PriceGapPaperMode` reads in `execution.go` migrated | static | `go test ./internal/pricegaptrader -run TestStaticCheck_NoDirectPaperModeRead -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | Aggregator filters `close_ts >= now-24h` (Pitfall 5) | unit | `go test ./internal/pricegaptrader -run TestAggregator_24hFilter -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | Aggregator handles missing yesterday SET (Pitfall 6) | unit | `go test ./internal/pricegaptrader -run TestAggregator_MissingYesterdaySet -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | Recovery clears `paused_by_breaker` AND zeroes sticky (D-11) | integration | `go test ./internal/pricegaptrader -run TestBreaker_RecoveryReenablesCandidatesAndClearsSticky -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | `paused_by_breaker` blocks entry path (D-10) | integration | `go test ./internal/pricegaptrader -run TestRiskGate_PausedByBreakerBlocksEntry -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | API handlers reject without typed phrase (D-12) | integration | `go test ./internal/api -run TestPgBreakerHandlers_TypedPhraseRequired -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | `pg:breaker:trips` LPUSH + LTRIM 500 cap | integration | `go test ./internal/database -run TestPriceGapState_BreakerTripsCap -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | Telegram NotifyPriceGapBreakerTrip uses critical bucket | unit | `go test ./internal/notify -run TestNotifyPriceGapBreakerTrip_CriticalBucket -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | Synthetic test-fire: full cycle without engine restart (success-criterion #5) | E2E | `go test ./internal/pricegaptrader -run TestBreaker_SyntheticTestFire_FullCycle -count=1` (recorded narrative in 15-HUMAN-UAT.md) | ❌ Wave 0 |
| PG-LIVE-02 | Default OFF (`PriceGapBreakerEnabled=false`) — no daemon work | unit | `go test ./internal/pricegaptrader -run TestBreaker_DisabledByDefault -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | Boot guard: missing `pg:breaker:state` initializes fresh | unit | `go test ./internal/pricegaptrader -run TestBreaker_FreshBootInit -count=1` | ❌ Wave 0 |
| PG-LIVE-02 | Validation: `PriceGapDrawdownLimitUSDT > 0` rejected; `PriceGapBreakerIntervalSec < 60` rejected | unit | `go test ./internal/config -run TestValidatePriceGapLive_BreakerFields -count=1` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/pricegaptrader -run Breaker -count=1` (≤5s expected — pure logic, mocks for Redis)
- **Per wave merge:** `go test ./internal/pricegaptrader/... ./internal/database/... ./internal/api/... ./internal/notify/... ./internal/config/... ./cmd/pg-admin/... -count=1` (~30s)
- **Phase gate:** Full suite green + recorded `15-HUMAN-UAT.md` for synthetic test-fire (success-criterion #5)

### What Can/Cannot Be Unit-Tested
- **Unit testable (pure logic):** Two-strike state machine, blackout detection (function reuse from scanner.go), 24h filter math, D-08 recovery clear, D-15 ordering, sticky-flag persistence, aggregator dedup.
- **Integration testable (against miniredis):** Full Redis HASH save/load, capped LIST behavior, multi-key writes (state + trip log + candidate pause).
- **Mock at boundary (NOT testable end-to-end in CI):** Real Telegram send (mock at `Notifier` interface — verified by `internal/notify/telegram_regression_test.go` precedent), real WS push (mock `BreakerWSBroadcaster` interface), real exchange order placement (paper-mode chokepoint already prevents this).
- **HUMAN-UAT only:** Real dashboard interaction — operator types `RECOVER` in modal, sees Tripped→Armed transition. Phase 14 P05 has exact precedent (`14-HUMAN-UAT.md`).

### Wave 0 Gaps
- [ ] `internal/pricegaptrader/breaker_controller_test.go` — unit tests for two-strike state machine + blackout + Strike-1 clear + restart-survival
- [ ] `internal/pricegaptrader/breaker_aggregator_test.go` — 24h filter, missing-key resilience, dedup
- [ ] `internal/pricegaptrader/breaker_static_test.go` — regex check that `cfg.PriceGapPaperMode` does not appear outside `tracker.go` and test files
- [ ] `internal/pricegaptrader/breaker_test_fire_test.go` — synthetic full-cycle integration test (success-criterion #5)
- [ ] `internal/database/pricegap_breaker_state_test.go` — HASH/LIST persistence + cap test
- [ ] `internal/api/pricegap_breaker_handlers_test.go` — typed-phrase validation, recover/test-fire/state handlers
- [ ] `internal/notify/pricegap_breaker_test.go` — critical bucket dispatch verification
- [ ] `internal/config/config_test.go` — extend with breaker-fields validation tests (limit ≤ 0, interval ≥ 60)
- [ ] `cmd/pg-admin/breaker_test.go` — typed-phrase prompt, three subcommands smoke
- [ ] `internal/pricegaptrader/registry_concurrent_test.go` — extend with `paused_by_breaker` write-path concurrency
- [ ] `15-HUMAN-UAT.md` — operator-recorded full-cycle synthetic-fire dashboard exercise

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.26+ | All Phase 15 code | ✓ | (per `go.mod`) | — |
| Redis (DB 2) | `pg:breaker:state`, `pg:breaker:trips`, existing `pg:positions:closed:{date}` | ✓ | (existing arb deployment) | — |
| Node v22.13.0 + nvm | Frontend build | ✓ | (per CLAUDE.local.md) | — |
| Telegram bot tokens | Critical alert | ✓ | (existing config) | Notifier.NotifyPriceGapBreakerTrip is no-op in `NoopNotifier`; tests use mock |

**Missing dependencies:** None. All required infrastructure already in production for Phase 14.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `pos.RealizedPnL` is the canonical realized-PnL field on `*models.PriceGapPosition` (verified at `internal/models/pricegap_position.go:84`) and is fully populated by the time the position is added to `pg:positions:closed:{date}` | Aggregator design, Pattern 4 | If field is populated AFTER the SADD, aggregator could read 0 for very recent closes. Phase 14 P01 set the SADD AFTER `pos.ClosedAt` line 248 — but `pos.RealizedPnL` is set even earlier in `monitor.go:207`, so the ordering is safe. Already validated by Phase 14 reconciler reading the same field. |
| A2 | `inBybitBlackout(now time.Time) bool` in `internal/pricegaptrader/scanner.go:539` is currently package-private; making it exported (or extracting it to a shared `blackout.go`) is acceptable | Pitfall 4, Pattern reuse | Refactor needed if not currently exposed. Mitigation: simple rename or copy a one-liner if review prefers no shared file. |
| A3 | The four direct `cfg.PriceGapPaperMode` reads in `execution.go` (L170, L227, L269, L362) are the ONLY non-test reads in the `internal/pricegaptrader/` module | Pattern 4, Pitfall 1 | Verified by grep: production reads are only at those 4 sites; the rest are in `_test.go` files (which legitimately set the flag for test fixtures). The static test will fail-loud if a future PR adds another. |
| A4 | `Tracker` already has access to a `*database.Client` it can extend with `BreakerStateStore` interface satisfaction, similar to how Phase 14 wired RampController via setters | Architecture | Verified: Phase 14 uses `SetReconciler/SetRamp/SetSizer` setters on `Tracker` in `cmd/main.go`. The same pattern handles `SetBreakerController`. |
| A5 | The dashboard widget extension uses existing Phase 14 `RampReconcileSection.tsx` location and adds a sibling `BreakerSubsection.tsx` — no new top-level tab needed (D-16 says Phase 16 absorbs) | Architecture, structure | Confirmed by D-16 + Phase 14 P05 widget design. Modal pattern for typed-phrase confirmation is standard React. |
| A6 | `*api.Hub` exposes a `Broadcast(eventType string, payload any)` method (or equivalent) suitable for `pg.breaker.trip` and `pg.breaker.recover` events | Pattern reuse | Verified by Plan 12-03 D-17 — Server.Hub() public accessor was added in Phase 12 to wire Phase 12 promote-event broadcaster. Same shape applies. |
| A7 | TelegramNotifier's "critical bucket" pattern (used by `NotifyPriceGapReconcileFailure` per `internal/notify/pricegap_reconcile.go:83`) bypasses any allowlist/rate-limit filter and dispatches immediately | Notify design | The CONTEXT and Phase 14 D-11 both reference this pattern. Need to read `telegram.go` allowlist logic during planning to confirm exact bypass path. |

## Open Questions

1. **Should `IsPaperModeActive` be cached in-memory with short TTL?**
   - What we know: Helper is called every entry attempt and every close attempt. Without caching, every entry hits Redis once for sticky check.
   - What's unclear: Cardinality at v2.2 volumes (≤5 active candidates, ~2-5 closes/day) is trivially low — caching may be premature optimization.
   - Recommendation: **No caching in v2.2.** Re-read on every call. Add metric for `IsPaperModeActive` call rate; revisit only if Redis cardinality becomes operational pain.

2. **Should the boot guard write-fresh-state run on EVERY tracker start, or only on first observed missing key?**
   - What we know: D-Discretion says "if `PriceGapBreakerEnabled=true` and `pg:breaker:state` is missing, **initialize fresh**".
   - What's unclear: If state exists with sticky=MaxInt64 (post-trip, pre-recovery), startup must NOT clear sticky.
   - Recommendation: Boot guard reads state; if `exists=false`, write fresh `{0,0,0,0,0}`. If `exists=true`, leave intact. Locked by unit test `TestBreaker_BootGuard_PreservesExistingTrip`.

3. **Where does the Tracker get the current `RampStage` for the trip record?**
   - What we know: D-15 step 2 trip record includes `ramp_stage`. Phase 14 exposes `RampController.Snapshot() RampState` returning `CurrentStage`.
   - What's unclear: Tracker holds a `*RampController` reference (set via `SetRamp` per Phase 14 P03), so it's directly accessible.
   - Recommendation: BreakerController takes a `RampSnapshotter` narrow interface (single `Snapshot() RampState` method) — same pattern as Phase 14 Plan 14-03 forward-declared `RampSnapshotter`.

4. **What does the Test Fire dry-run (`--dry-run`) actually log?**
   - What we know: D-14 says "logs would-trip + computes 24h PnL but skips all mutations".
   - What's unclear: Does dry-run write to `pg:breaker:trips` with `source: "test_fire_dry_run"` (per D-18 `source` enum) or NOT write to the trip log at all?
   - Recommendation: D-14 says "skips all mutations" — therefore **dry-run does NOT write to `pg:breaker:trips`**. The `source: "test_fire_dry_run"` value in the D-18 enum is for dashboard display of in-flight test-fire previews, not persisted trip records. Validate with discuss-phase if ambiguous; default to no-write for safety.

## Pitfalls

(Sections above; see Common Pitfalls 1-9.)

## Sources

### Primary (HIGH confidence)
- `.planning/phases/15-drawdown-circuit-breaker/15-CONTEXT.md` — All locked decisions D-01 through D-18 + canonical refs + code anchors
- `.planning/phases/14-daily-reconcile-live-ramp-controller/14-CONTEXT.md` — Phase 14 hard contracts and ramp-controller precedent
- `.planning/REQUIREMENTS.md` §"Strategy 4 Live Capital" §PG-LIVE-02 — verbatim drawdown breaker contract
- `.planning/ROADMAP.md` §"Phase 15" — 5 success criteria
- `internal/pricegaptrader/scanner.go:539-545` — `inBybitBlackout(now time.Time) bool` (verified function signature + minute/second logic)
- `internal/pricegaptrader/reconciler.go:82-117` + `ramp_controller.go:51-95` — Daemon goroutine + 5-field HASH precedent
- `internal/database/pricegap_state.go:38-42, 407-502` — Redis namespace constants, `SavePriceGapRampState`, `AppendPriceGapRampEvent`
- `internal/pricegaptrader/notify.go:65-108` — Notifier interface + NoopNotifier shape
- `internal/notify/pricegap_reconcile.go:80-100` — Critical-bucket TelegramNotifier precedent
- `internal/pricegaptrader/execution.go:170, 227, 269, 362` — Four direct `cfg.PriceGapPaperMode` reads
- `internal/config/config.go:74-76, 380-419, 470-476, 905, 1521-1619` — `validatePriceGapLive`, `PriceGap*` config block, applyJSON
- `internal/models/pricegap_position.go:84` — `RealizedPnL float64` field name
- `internal/models/pricegap_ramp.go:24-29` — RampState 5-field struct (precedent shape)

### Secondary (MEDIUM confidence)
- Phase 14 SUMMARY files (per directory listing, present at `14-01-SUMMARY.md`...`14-05-SUMMARY.md`) — confirmed reconcile + ramp shipped on v0.37.0 per accumulated context entries in STATE.md
- `internal/pricegaptrader/scanner_test.go:407-440` — `TestScanner_BybitBlackout` test exists and exercises the blackout window — confirms the function works correctly

### Tertiary (LOW confidence)
- (none — every load-bearing claim verified by grep)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new deps; all primitives verified in existing code
- Architecture: HIGH — Phase 14 ramp_controller + reconciler are direct precedents
- Pitfalls: HIGH — derived from explicit codebase reads (paper-mode read sites, blackout helper, RealizedPnL field, RampState shape)
- Algorithm: HIGH — two-strike state machine fully specified by CONTEXT D-05 + D-08
- Test mapping: HIGH — Phase 14 P02/P03 test patterns are direct templates

**Research date:** 2026-05-01
**Valid until:** 2026-05-31 (30 days; codebase shape stable per Phase 14 ship)
