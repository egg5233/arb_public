# Phase 15: Drawdown Circuit Breaker - Context

**Gathered:** 2026-05-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Strategy 4 (pricegap) gets a drawdown circuit breaker that monitors realized PnL on a rolling 24h window and auto-flips the engine to paper mode on breach via the new `PaperModeStickyUntil` sticky flag. Two-strike rule (≥5 min apart, breach + confirmation tick) prevents single-snapshot trips. Bybit `:04–:05:30` blackout suppresses evaluation. Recovery requires explicit operator action — the sticky flag does NOT auto-clear. Synthetic test fire validates the full breaker → paper-flip → operator-recovery cycle without engine restart.

This is the safety net behind Phase 14's live-capital ramp. Phase 14 lets capital scale up; Phase 15 stops it when realized losses cross a hard line.

Out of scope this phase:
- Auto-recovery from drawdown breaker (REQUIREMENTS Out-of-Scope — sticky must be operator-cleared)
- MTM tracking (spec: realized PnL only)
- Calendar-day windowing (spec: rolling 24h)
- Per-symbol breakers (single global breaker for v2.2)
- Top-level Pricegap dashboard tab (Phase 16, PG-OPS-09)
- Realized-slippage machine-zero fix (Phase 16, PG-FIX-01)

</domain>

<decisions>
## Implementation Decisions

### Evaluation Cadence + 24h Window (Area A)
- **D-01:** **5-min dedicated ticker.** New goroutine + `time.Ticker` on `Tracker`, decoupled from reconcile/scanner/promotion/ramp daemons. Pairs naturally with the spec's ≥5 min strike spacing (one tick = one strike opportunity). Lifecycle: spawned in `Tracker.Start`, cancelled via `context.Context` on graceful shutdown.
- **D-02:** **Aggregate from existing Phase 14 index.** On each tick: `SMEMBERS pg:positions:closed:{today}` + `SMEMBERS pg:positions:closed:{yesterday}` (UTC dates), then `GET pg:position:{id}` for each member, filter `close_ts >= now - 24h`, sum `RealizedPnLUSDT`. No new key. Cheap at v2.2 volumes (≤low double-digit closes/day). Reuses Phase 14 D-02 index as a read-only consumer.
- **D-03:** **Whole-tick blackout suppression.** If wall-clock minute-of-hour is in `[:04, :05:30)`, the entire eval tick is a no-op — no PnL fetch, no strike state mutation. Matches PG-LIVE-02 wording "Suppress evaluation" verbatim. Single timestamp check at top of tick.
- **D-04:** **Pending Strike-1 survives blackout-suppressed ticks.** Blackout = "skip evaluation", not "reset state". A drawdown that crosses blackout still fires Strike-2 on the next eligible tick if breach + ≥5 min elapsed since Strike-1.

### Strike State + Threshold + Sticky Flag (Area B)
- **D-05:** **`pg:breaker:state` HASH (Redis-persisted).** Five fields: `pending_strike` (0|1), `strike1_ts` (unix-ms or 0), `last_eval_ts`, `last_eval_pnl_usdt`, `paper_mode_sticky_until` (unix-ms; 0 = not sticky, `math.MaxInt64` = sticky-until-operator-clears). Five-field shape mirrors Phase 14 ramp HASH precedent. Strike survives `kill -9`. Locked with a unit test that kills + restarts mid-strike.
- **D-06:** **`PriceGapDrawdownLimitUSDT` is absolute USDT.** Single negative value (e.g., `-50`). Trips when `sum(realized_pnl_24h) < limit`. Does NOT auto-scale with ramp stage — operator manually re-tunes when ramping. Field name pinned by PG-LIVE-02 spec.
- **D-07:** **`PaperModeStickyUntil` is int64 unix-ms with sentinel.** `0` = not sticky (engine respects normal `paper_mode` config). `math.MaxInt64` = sticky-until-operator-clears (engine forces paper mode regardless of config). Future-proof: a real future timestamp would mean timed-sticky if ever spec'd. Stored on `pg:breaker:state`. Persisted **before** any other trip side-effect (atomicity anchor — see D-15).
- **D-08:** **Strike-1 clears on PnL recovery.** A tick that sees `realized_pnl_24h >= threshold` between Strike-1 and Strike-2 clears `pending_strike=0`. Re-arms fresh next time. "Two consecutive breaches" interpreted in spirit: both ticks must be in breach.

### Recovery UX + Auto-Disable Scope (Area C)
- **D-09:** **Recovery dual-surface (pg-admin + dashboard).** Two paths:
  1. `pg-admin breaker recover --confirm` — prompts for typed phrase `RECOVER` before executing.
  2. Dashboard "Recover" button on Pricegap-tracker tab — opens modal requiring typed `RECOVER` text input before submitting `POST /api/pg/breaker/recover`.
  Both paths require operator authentication and emit identical audit events.
  This deviates from Phase 14 D-14 (read-only dashboard) — Phase 15 dashboard widget IS write-capable, but only for recovery + test-fire. Phase 16 PG-OPS-09 absorbs the widget into the new top-level Pricegap tab.
- **D-10:** **New `paused_by_breaker` boolean on each `Candidate` registry entry.** Distinct from existing operator-set `disabled` field. Entry path checks BOTH (`disabled || paused_by_breaker` → reject). Trip writes `paused_by_breaker=true` on every candidate that has an open or recently-active position. Reversible without touching operator-set `disabled` state.
- **D-11:** **Auto re-enable on recovery.** Recovery action clears `paused_by_breaker=false` on every candidate AND zeroes `paper_mode_sticky_until`. Operator-set `disabled=true` candidates stay disabled. One-step recovery; no per-candidate manual review required.
- **D-12:** **Typed-phrase confirmation on every mutation surface.** Both pg-admin AND dashboard Recover button AND test-fire surfaces require the operator to type the literal phrase `RECOVER` (recover) or `TEST-FIRE` (test fire) before the mutation executes. Prevents shell-history one-shot recovery, mis-clicks on touchscreen / shared sessions, and trivial replay attacks via API.

### Synthetic Test Fire + Observability Surface (Area D)
- **D-13:** **Test fire dual-surface (symmetric with D-09).** Two paths:
  1. `pg-admin breaker test-fire [--dry-run] --confirm` — typed phrase `TEST-FIRE`.
  2. Dashboard "Test fire" button on Pricegap-tracker tab — modal with same typed-phrase requirement.
- **D-14:** **Test fire defaults to a real trip; `--dry-run` flag opts into simulation.** Default behavior performs a real trip (sticky flag set, paper mode forced, candidates paused, Telegram alert, `pg:breaker:trips` log) — operator must run real recovery path. Validates ROADMAP success-criterion #5 ("exercises full breaker → paper-flip → operator recovery cycle without engine restart"). `--dry-run` logs "would-trip" + computes 24h PnL but skips all mutations — for quick CI smoke checks of the eval path. Trip records distinguish `source: "live" | "test_fire" | "test_fire_dry_run"`.
- **D-15:** **Trip side-effect ordering (atomicity anchor).** On trip, in this order:
  1. Write `paper_mode_sticky_until=MaxInt64` on `pg:breaker:state` first (the engine's authoritative paper-mode source).
  2. `LPUSH` trip record onto `pg:breaker:trips`.
  3. Write `paused_by_breaker=true` on candidates via CandidateRegistry chokepoint.
  4. Send Telegram critical alert.
  5. WS broadcast trip event.
  Ordering ensures any partial failure leaves the engine in paper mode (the safest state). Steps 2–5 are best-effort with logged failures (per Phase 14 best-effort precedent). Step 1 is the load-bearing safety property — covered by a unit test that fails steps 2–5 deliberately and verifies sticky=MaxInt64 still persisted.
- **D-16:** **Dashboard widget = extend Phase 14 widget.** Add a "Breaker" subsection to the existing Phase 14 Ramp+Reconcile widget on the Pricegap-tracker tab. Shows: armed/tripped state, current rolling 24h realized PnL (USDT), threshold, last trip ts, last trip PnL, paused-candidate count, Recover button (visible only when tripped), Test-fire button (always visible, disabled while tripped). Phase 16 PG-OPS-09 absorbs the entire widget into the new top-level Pricegap tab — don't pre-build Phase 16 layout.
- **D-17:** **Telegram alert is full-context single message.** Fields: realized 24h PnL (USDT), configured threshold, window start/end timestamps (Asia/Taipei display per project convention), ramp stage at trip, paused-candidate count, recovery instruction line (e.g., `"Recover: pg-admin breaker recover --confirm"`). Compact, phone-readable. Mirrors Phase 14 D-11 digest shape. **Critical bucket** (bypasses allowlist filtering).
- **D-18:** **`pg:breaker:trips` is a LIST capped at 500 entries.** Each entry is JSON: `{trip_ts, trip_pnl_usdt, threshold, ramp_stage, paused_candidate_count, recovery_ts (nullable), recovery_operator (nullable), source: "live"|"test_fire"|"test_fire_dry_run"}`. Cap mirrors Phase 14 `pg:ramp:events` and `pg:history`. `LPUSH` on trip; `LSET` on recovery to backfill `recovery_ts` + `recovery_operator`.

### Hard Contracts (Carried Forward — Locked by ROADMAP/REQUIREMENTS)
These are NOT gray areas; pinned by spec. Listed here for downstream agents:
- **REALIZED PnL only** — no MTM, no unrealized (PG-LIVE-02 verbatim).
- **Rolling 24h window** — not calendar-day, not since-last-reconcile (PG-LIVE-02).
- **`PriceGapDrawdownLimitUSDT`** — exact field name (PG-LIVE-02).
- **Auto-revert live → paper via sticky flag `PaperModeStickyUntil`** (PG-LIVE-02).
- **Auto-disable any open candidate** (PG-LIVE-02).
- **Telegram critical alert + WS broadcast** (PG-LIVE-02).
- **`pg:breaker:trips` log key** (PG-LIVE-02 verbatim).
- **Bybit `:04–:05:30` blackout suppression** (PG-LIVE-02).
- **Two-strike rule, ≥5 min apart** (PG-LIVE-02 + ROADMAP §"Phase 15" success-criterion #2).
- **Recovery requires explicit operator action; sticky does NOT auto-clear on restart or page reload** (PG-LIVE-02 + ROADMAP success-criterion #4).
- **Synthetic test fire exercises full cycle without engine restart** (ROADMAP success-criterion #5).
- **Module isolation:** all new code in `internal/pricegaptrader/`. No imports of `internal/engine` or `internal/spotengine` (REQUIREMENTS Constraints).
- **Default OFF:** every new config flag (`PriceGapBreakerEnabled` default false, `PriceGapDrawdownLimitUSDT` default 0). Validated at config-load.
- **`pg:*` Redis namespace.**
- **Commit + VERSION + CHANGELOG together** (project rule from CLAUDE.local.md).

### Claude's Discretion
- Internal struct/interface names: `BreakerController`, `RealizedPnLAggregator`, `BreakerStateStore` (Phase 14 used `RampController`, `Reconciler` — analogous shape welcome).
- Whether the 5-min ticker is configurable (`PriceGapBreakerIntervalSec`, default 300, validated [60, 3600]) or hardcoded — spec says ≥5 min; configurable allows tuning. Lean configurable with safety floor.
- Whether eval-tick logs `realized_pnl_24h` always or only-on-breach (cardinality vs observability trade-off).
- Boot-time guard: if `PriceGapBreakerEnabled=true` and `pg:breaker:state` is missing, **initialize fresh** (`pending_strike=0`, `paper_mode_sticky_until=0`). Different from Phase 14 ramp (which refuses to start without state) — breaker can safely cold-start, since a fresh breaker is permissive (armed + ready), not a load-bearing capital decision.
- Atomicity primitive choice for D-15 — single Lua script vs sequential best-effort writes. Sequential is probably correct (Step 1 is single-key HSET); Claude verifies.
- Telemetry exposure (Prometheus-style metrics if they exist; otherwise dashboard-only).
- Test fixture organization for the synthetic test-fire integration test (15-HUMAN-UAT.md per project pattern records the recorded full-cycle exercise).
- Locale strings for the new Recover/Test-fire buttons + breaker widget labels — must add to BOTH `web/src/i18n/en.ts` AND `web/src/i18n/zh-TW.ts` (project convention). Typed phrases (`RECOVER`, `TEST-FIRE`) are magic strings and do NOT translate.
- Default value of `PriceGapDrawdownLimitUSDT` for fresh installs — `0` (breaker effectively armed-but-never-trips) is the conservative default. Operator sets a real value before flipping `PriceGapBreakerEnabled=true`.

### Folded Todos
None — no pending todos matched Phase 15 scope at session start.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Roadmap & Requirements
- `.planning/ROADMAP.md` §"Phase 15: Drawdown Circuit Breaker" — phase goal + 5 success criteria
- `.planning/REQUIREMENTS.md` §"Strategy 4 Live Capital" §**PG-LIVE-02** — drawdown breaker contract verbatim (rolling 24h realized, two-strike, sticky flag, blackout, operator recovery)
- `.planning/REQUIREMENTS.md` §"Out of Scope" — boundaries (no auto-recovery, no MTM, no continuous-curve, no per-symbol breaker)
- `.planning/REQUIREMENTS.md` §"Constraints (locked at milestone start)" — module boundary, namespace, default-OFF rule
- `.planning/REQUIREMENTS.md` §"Success Criteria" — milestone success-criterion #4 ("Drawdown breaker has been exercised") references this phase

### Phase 14 (Direct Dependency — sticky flag, ramp, reconcile, closed-position index)
- `.planning/phases/14-daily-reconcile-live-ramp-controller/14-CONTEXT.md` — ALL decisions Phase 15 carries forward (module isolation D-precedent, Redis namespace, pg-admin pattern, dashboard widget pattern, Notifier extension, ramp gate composition)
- `.planning/phases/14-daily-reconcile-live-ramp-controller/14-CONTEXT.md` §"Hard Contracts" — `PriceGapLiveCapital` flag, Redis HASH 5-field precedent, sizing chokepoint
- `.planning/phases/14-daily-reconcile-live-ramp-controller/14-DISCUSSION-LOG.md` — promote-event-sink + ramp-event-log shape; reuse for `pg:breaker:trips`

### Phase 11 + 12 (precedent for module wiring + risk_gate composition + chokepoints)
- `.planning/phases/11-auto-discovery-scanner-chokepoint-telemetry/11-CONTEXT.md` §"Integration Points" — telemetry / dashboard / risk_gate wiring conventions
- `.planning/phases/12-auto-promotion/12-CONTEXT.md` — controller wiring pattern (`PromotionController` is the precedent for this phase's `BreakerController`)

### v2.2 Research
- `.planning/research/ARCHITECTURE-v2.2.md` — drawdown breaker architecture (consult before sizing decisions)
- `.planning/research/PITFALLS.md` — Pitfall 7 precedent: latent bugs surfaced during dev → spawn separate Phase 999.x hot-fix mini-phase, not silent merge

### Existing Code Anchors (read before modifying)
- `internal/pricegaptrader/risk_gate.go` — current `preEntry` Gates 0–6 (Phase 14 added Gate 6 ramp). Phase 15 likely does NOT add a new gate — the breaker uses the sticky flag to short-circuit the engine's live-mode at the `paper_mode` check, not the entry gate. **Researcher must verify** the exact paper-mode read site and confirm sticky-flag short-circuit is the right integration point.
- `internal/pricegaptrader/monitor.go:248` (`closePair`) — site of the `pg:positions:closed:{date}` SADD added in Phase 14 (D-02). Phase 15 reads this index — read-only, no modification.
- `internal/pricegaptrader/notify.go` — `Notifier` interface; add `NotifyPriceGapBreakerTrip(ctx, TripPayload) error` method here, then implement in `internal/notify/telegram.go` with **critical bucket** (NOT `pricegap_digest`; this is fire-now alert).
- `internal/pricegaptrader/promotion.go` (Phase 12 controller) — pattern reference for breaker controller: Redis-persisted state, idempotent eval, event-sink log.
- `internal/pricegaptrader/ramp_controller.go` (Phase 14) — closest precedent for the breaker's state machine + 5-field HASH + boot-time recovery (NOTE: breaker boot-guard differs — see Claude's Discretion).
- `internal/pricegaptrader/reconciler.go` (Phase 14) — daemon goroutine + `time.Ticker` pattern for the breaker's 5-min eval loop.
- `internal/pricegaptrader/registry.go` — CandidateRegistry; add `paused_by_breaker bool` to the candidate struct + audit how the entry path reads it.
- `internal/pricegaptrader/registry_concurrent_test.go` — concurrent-write tests; the `paused_by_breaker` write path must be exercised here.
- `internal/database/pricegap_state.go` — Redis key namespace block; add `pg:breaker:state` HASH and `pg:breaker:trips` LIST.
- `internal/config/config.go` (`PriceGap*` block, ~lines 341–366 in Phase 14) — site of new fields: `PriceGapBreakerEnabled` (bool, default false), `PriceGapDrawdownLimitUSDT` (float, default 0, validated [-1e6, 0]), `PriceGapBreakerIntervalSec` (int, default 300, validated [60, 3600]).
- `internal/config/config.go` `validatePriceGapLive` (Phase 14) — extend to validate breaker fields (limit must be ≤ 0, interval must be ≥ 60s).
- `cmd/pg-admin/main.go` — subcommand registration. Phase 15 adds: `breaker recover --confirm` (typed-phrase prompt), `breaker test-fire [--dry-run] --confirm` (typed-phrase prompt), `breaker show` (read state).
- `internal/api/` (find pricegap handler file via glob, e.g., `pricegap_handler.go`) — add `POST /api/pg/breaker/recover` and `POST /api/pg/breaker/test-fire` (auth + typed-phrase verification + optional `dry_run` body field). `GET /api/pg/breaker/state` for read.
- `web/src/` — locate Pricegap-tracker tab component (Phase 14 widget). Extend with Breaker subsection. i18n keys MUST be added to BOTH `web/src/i18n/en.ts` and `web/src/i18n/zh-TW.ts` lockstep.
- `internal/notify/telegram.go` — implement `NotifyPriceGapBreakerTrip` with **critical** bucket (bypasses allowlist filter, follows existing critical-bypass pattern from Phase 9).

### Cross-Project History
- `git log --oneline internal/pricegaptrader/` — module evolution (Phase 8 → 9 → 10 → 999.1 → 11 → 12 → 14 → 15).
- v0.37.0 (Phase 14 ship) — most recent precedent for adding a controller into pricegap module without disturbing live trading.
- v1.0 perp-perp engine — `TradingPaused` / `EmergencyClose` mechanisms (precedent for circuit-breaker UX patterns; **reference only**, do not import — module isolation rule).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`Tracker` struct** (`internal/pricegaptrader/tracker.go`) — host for new `BreakerController` field, daemon goroutine spawned alongside scanner / promotion / reconcile / ramp daemons.
- **`Notifier` interface** (`internal/pricegaptrader/notify.go`) — already exposes `NotifyPriceGapEntry/Exit/RiskBlock/DailyDigest`. Add `NotifyPriceGapBreakerTrip` here; `NoopNotifier` gets the no-op stub.
- **`pg:positions:closed:{YYYY-MM-DD}` SET index** (Phase 14 D-02) — the breaker's primary input. `monitor.closePair` populates; Phase 15 read-only.
- **`pg:reconcile:daily:{date}` HASH** (Phase 14) — informational only for breaker (NOT the eval source — breaker computes its own rolling 24h from raw closed-position data, not the reconcile's calendar-day total).
- **`pg-admin` subcommand registration pattern** (Phase 8/12/14) — clean addition of `breaker` subcommand without breaking existing commands.
- **5-field Redis HASH precedent** (`pg:ramp:state` from Phase 14) — same shape for `pg:breaker:state`.
- **500-entry LIST cap precedent** (`pg:history`, `pg:ramp:events` from Phase 14) — reuse for `pg:breaker:trips`.
- **Telegram critical-bucket pattern** (`internal/notify/telegram.go`) — bypasses allowlist filtering for fire-now alerts; reuse for breaker trip.
- **`Reconciler` daemon `time.Ticker` pattern** (Phase 14) — direct precedent for breaker's 5-min eval loop.
- **Interface-driven DI** (Phase 11/12/14 D-02) — define `BreakerStateStore` interface, inject `*database.Client` in production, fakes in tests.
- **CandidateRegistry chokepoint** (Phase 11 PG-DISC-04) — single-writer serialization for the new `paused_by_breaker` field write.

### Established Patterns
- **Default-OFF master toggle.** `PriceGapDiscoveryEnabled` (Phase 11) → `PriceGapLiveCapital` (Phase 14) → `PriceGapBreakerEnabled` (Phase 15). Same pattern, validated at config-load.
- **Strict `pg:*` Redis namespace.** All new keys: `pg:breaker:state`, `pg:breaker:trips`. Documented in `internal/database/pricegap_state.go` namespace block.
- **Config validated at load.** `validatePriceGapLive` (Phase 14) extended with breaker validators (limit ≤ 0, interval ≥ 60s).
- **Module isolation.** All Phase 15 code in `internal/pricegaptrader/`. No imports of `internal/engine` or `internal/spotengine`. `TradingPaused`/`EmergencyClose` patterns are *reference only*, not imports.
- **Best-effort Redis writes with logged failure.** Trip side-effects steps 2–5 (D-15) follow this pattern.
- **Phase 16 absorbs.** Phase 15 widget lives in the existing Pricegap-tracker tab and migrates to the new top-level Pricegap tab when PG-OPS-09 ships. Don't pre-build Phase 16 layout.
- **Atomicity-by-ordering.** When a multi-step mutation can fail mid-way, order steps such that partial failure leaves the system in the safest state. D-15 applies this: sticky flag first → other steps best-effort.
- **i18n lockstep.** All new UI strings must be in both `en.ts` and `zh-TW.ts`. CI / type-check enforces.
- **Commit + VERSION + CHANGELOG together** (project rule).

### Integration Points
- **`Tracker` startup** — spawn `BreakerController.Run(ctx)` goroutine alongside existing scanner / promotion / reconcile / ramp daemons, with `context.Context` cancellation for graceful shutdown.
- **Engine paper-mode check (LOCATE during research)** — the engine consults `paper_mode` somewhere on the entry path. Phase 15 makes this consultation honor `pg:breaker:state.paper_mode_sticky_until` — when sticky is non-zero AND `now < sticky_until` (with `MaxInt64` = always sticky), force `paper_mode=true` regardless of config. Researcher must locate exact site (likely `tracker.go` entry path or a paper-mode helper). Recommended: route ALL paper-mode reads through a single `tracker.IsPaperModeActive(ctx) (bool, error)` helper so future code can't bypass the sticky flag.
- **CandidateRegistry mutation** — extend `registry.go` Candidate struct with `paused_by_breaker bool`; entry path reads `candidate.disabled || candidate.paused_by_breaker`; trip writes `paused_by_breaker=true` for every open-position candidate via the chokepoint; recovery clears all `paused_by_breaker=false`.
- **API handlers** — add `POST /api/pg/breaker/recover` (auth + typed-phrase body), `POST /api/pg/breaker/test-fire` (auth + typed-phrase + optional `dry_run` body), `GET /api/pg/breaker/state` (read state JSON). All under existing pricegap handler file.
- **Frontend Pricegap-tracker tab** — extend Phase 14 widget with Breaker subsection. Recover button + Test-fire button each open a confirm-modal that requires the operator to type `RECOVER` / `TEST-FIRE` (the typed phrase itself is a magic string and does NOT translate; surrounding modal labels DO translate via i18n keys).
- **`Notifier` impl** (`internal/notify/telegram.go`) — implement `NotifyPriceGapBreakerTrip` with **critical** bucket (bypasses allowlist).
- **`pg-admin breaker` subcommand** — three subcommands: `recover`, `test-fire`, `show`. Each shares internal types with the daemon (the CLI path and daemon path are two front-ends to the same `BreakerController` logic).
- **WS broadcast** — emit `pg.breaker.trip` event with full trip payload on trip; `pg.breaker.recover` on recovery. Frontend widget subscribes to update in real time.
- **Config load** — `validatePriceGapLive` invoked from existing config validation chain; extend with breaker validators.
- **`internal/database/pricegap_state.go`** — add helpers: `LoadBreakerState`, `SaveBreakerState`, `AppendBreakerTrip`, `UpdateBreakerTripRecovery`. Five-field HASH + capped LIST.

</code_context>

<specifics>
## Specific Ideas

- **"Sticky flag is the engine's authoritative paper-mode source while non-zero."** The breaker's safety property is: any code path that reads `paper_mode` MUST also read `pg:breaker:state.paper_mode_sticky_until` and force paper mode when sticky is active. PRINCIPLE: route all paper-mode reads through a single helper (e.g., `tracker.IsPaperModeActive(ctx) (bool, error)`) so future code doesn't accidentally bypass the sticky flag. Operator can grep one function name to verify all reads are gated.
- **Two-strike rule is asymmetric: easy to clear, hard to fire.** Single-tick recovery clears Strike-1 (D-08); two consecutive in-breach ticks with ≥5 min gap fire the trip. The asymmetry is correct: false negatives (slow trip) are recoverable; false positives (fast trip from a noisy snapshot) cause real ops pain.
- **Test fire validates the FULL cycle, not just the trigger.** Success-criterion #5 requires the synthetic test to exercise breaker → paper-flip → operator recovery without engine restart. The integration test (15-HUMAN-UAT.md per project pattern) records: (a) operator runs `pg-admin breaker test-fire --confirm`; (b) Telegram alert received; (c) dashboard widget shows "Tripped"; (d) operator runs `pg-admin breaker recover --confirm`; (e) widget back to "Armed"; (f) candidates re-enabled; (g) engine continues without restart.
- **Boot-time guard is permissive (unlike Phase 14 ramp).** Phase 14 specific said "if `PriceGapLiveCapital=true` but `pg:ramp:state` missing → refuse to start". For the breaker, fresh state (`pending_strike=0`, `paper_mode_sticky_until=0`) is a safe cold-start — the breaker is a watchdog, not a state-keeper of capital. Initialize fresh on first boot.
- **Trip side-effect ordering is a load-bearing safety property (D-15).** Anyone modifying the trip path must preserve the rule: sticky flag MUST be persisted before any other side-effect. Add a comment block at the top of the `trip()` function locking this in. A unit test that fails the sequence on purpose (mock Redis to fail step 2) and verifies sticky=`MaxInt64` anyway prevents regression.
- **Dashboard widget shows "Live Capital: ON / OFF + Breaker: Armed / Tripped" prominently.** Operator must never be confused about whether they are at risk. Color-coded badges (red when Tripped, green when Armed-Live, gray when Paper).
- **Typed-phrase is `RECOVER` / `TEST-FIRE` literally, regardless of locale.** The phrase is a magic string for safety, not a translatable label. i18n keys translate the surrounding modal text ("Type RECOVER to confirm:") but not the typed phrase itself.

</specifics>

<deferred>
## Deferred Ideas

- **Auto-recovery from drawdown breaker** — REQUIREMENTS Out-of-Scope. Sticky flag MUST be operator-cleared.
- **MTM-based breaker** — spec is realized PnL only; v3.0+ if ever.
- **Per-symbol or per-pair breakers** — single global breaker for v2.2 simplicity.
- **Calendar-day breaker** — spec mandates rolling 24h.
- **Breaker thresholds that scale with ramp stage** — Phase 15 chose absolute USDT (D-06). Operator manually re-tunes when ramping. Revisit in v2.3 if operationally painful.
- **Top-level Pricegap dashboard tab consolidation** — Phase 16 PG-OPS-09. Phase 15 widget extends the Phase 14 widget location; Phase 16 absorbs.
- **Realized-slippage machine-zero fix** — Phase 16 PG-FIX-01. Affects breaker accuracy in paper mode but live capital is the priority surface.
- **Per-fill Telegram alerts on breaker trip** — descoped from v2.2 per REQUIREMENTS Out-of-Scope. Only the trip event itself is alerted, not individual contributing fills.
- **Audit log of recovery actors beyond operator name in `pg:breaker:trips`** — current shape (operator, timestamp) is sufficient; richer audit (IP, session) is v3.0 territory.
- **Continuous-curve breaker thresholds** — discrete absolute USDT only.
- **Auto-tune of threshold from PnL history** — REQUIREMENTS Out-of-Scope ("manual calibration with operator review").
- **Cross-strategy breaker (perp-perp + spot-futures + pricegap shared circuit)** — v3.0 unification milestone.
- **Test fire bypassing the typed-phrase requirement** — explicitly rejected; would make it too easy to mis-fire in live ops.
- **Multi-tenant breaker state (`pg:breaker:state` per operator)** — single-process system per CLAUDE.local.md; no need.

### Reviewed Todos (not folded)
None — no pending todos matched Phase 15 scope at session start.

</deferred>

---

*Phase: 15-drawdown-circuit-breaker*
*Context gathered: 2026-05-01*
