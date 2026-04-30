# Phase 14: Daily Reconcile + Live Ramp Controller - Context

**Gathered:** 2026-04-30
**Status:** Ready for planning

<domain>
## Phase Boundary

Strategy 4 (pricegap) gets two coupled subsystems inside `internal/pricegaptrader/`:

1. **Daily reconcile job** — fires UTC 00:30, aggregates closed Strategy 4 positions for the prior UTC day into `pg:reconcile:daily:{YYYY-MM-DD}` Redis records. Idempotent (re-run produces byte-identical output). Flags anomalies (large slippage, missing exchange close-timestamp). Emits a daily Telegram digest. Reuses perp-perp 3-retry pattern.
2. **Live ramp controller** — Redis-persisted state machine gating live-capital sizing through three discrete stages: 100 → 500 → 1000 USDT/leg. Asymmetric ratchet: 7 clean days promote one stage; any single loss day resets clean-day counter to 0 AND demotes one stage. `min(stage_size, hard_ceiling)` enforced at the sizing call site. Integrated into `risk_gate.go` as Gate 6 (= "gate #7" by spec count).

**The clean-day signal flows reconcile → ramp.** Reconcile is the truth-source; ramp is the consumer.

This is **the first phase to touch live capital.** Conservative-by-default: every new behavior gated by a config flag, default OFF.

Out of scope this phase:
- Drawdown circuit breaker (Phase 15, PG-LIVE-02)
- Dashboard tab consolidation (Phase 16, PG-OPS-09)
- Paper-mode bug closure (Phase 16, PG-FIX-01/02)
- Tech-debt sweep (Phase 17)

</domain>

<decisions>
## Implementation Decisions

### Reconcile Job Model
- **D-01:** **In-process daemon.** Reconcile runs as a goroutine + `time.Ticker` inside the existing pricegaptrader `Tracker`. Fires once daily at UTC 00:30. Matches existing `scanner.go` and Phase 12 promotion-controller patterns. No new systemd unit. `pg-admin reconcile run --date=YYYY-MM-DD` is the manual / re-run path; both paths share the same internal `Reconciler` struct.
- **D-02:** **New dated index `pg:positions:closed:{YYYY-MM-DD}` (SET of position IDs).** Written via `SADD` inside `monitor.closePair` after `pos.ClosedAt` is assigned (one-line addition near `internal/pricegaptrader/monitor.go:248`). O(1) date lookup at reconcile time. Best-effort error log on Redis failure (matches existing pattern at lines 249–250).
- **D-03:** **3-retry → skip-day on triple-fail.** Reuses perp-perp 3-retry pattern (5s/15s/30s backoff per PG-LIVE-03). On triple-failure: log, emit Telegram critical alert, skip the day. Day stays unreconciled. Ramp controller treats unreconciled days as **ambiguous** — no clean-day credit, no demote. Operator backfills via `pg-admin reconcile run --date=X`. Conservative: errs toward not promoting.
- **D-04:** **Idempotency via deterministic ordering + canonical JSON.** Reconcile sorts positions by `(close_ts, position_id)` before aggregating, serializes day records with sorted map keys (`json.Marshal` with canonical key order). Plain HSET / SET overwrite. Re-running for the same date produces byte-identical output. Locked with a unit test.

### Ramp Gate Semantics
- **D-05:** **Clean day = realized PnL ≥ 0 AND ≥1 closed Strategy 4 position that day.** No-activity days **hold** the counter — they neither increment it (no streak credit) nor reset it (no demote). 7 clean days means 7 actual trading days, not 7 calendar days. Rationale: prevents trivial promotion through quiet weeks; prevents accidental demotion during operator-initiated downtime.
- **D-06:** **New `PriceGapLiveCapital` config flag (bool, default false).** Independent of `PriceGapPaperMode`. Live-capital activation requires **both** `paper_mode=false` AND `live_capital=true` — two switches to risk capital, validated together at config-load. Single flag would couple "is paper" with "is gated by ramp" and remove the ability to dry-run live-ramp logic.
- **D-07:** **Ramp gate is no-op when `PriceGapLiveCapital=false`.** Paper-mode sizing continues to use existing per-candidate `MaxPositionUSDT`. **However, reconcile still runs in paper mode** — the clean-day signal accumulates regardless of mode, so the moment the operator flips `live_capital=true`, the streak data is already present.
- **D-08:** **`demote_count` is telemetry only.** Monotonic counter, exposed in dashboard + `pg-admin ramp show`. Drives no automatic action. Phase 15 (drawdown breaker, PG-LIVE-02) owns the actual circuit-break primitive — don't double-implement here. Manual operator review if `demote_count` grows quickly.

### Anomaly Detection + Telegram Digest
- **D-09:** **New `PriceGapAnomalySlippageBps` config field, default 50, validated [0, 500].** Reconcile flags positions where `abs(realized_slippage_bps) > threshold`. NOTE: depends on **PG-FIX-01** (Phase 16) fixing the realized-slippage machine-zero bug for paper-mode emission — for live capital this signal works directly because real fills generate non-zero slippage natively.
- **D-10:** **Missing exchange close-timestamp → fall back to `pos.ClosedAt` (local clock).** Use the local timestamp set in `monitor.go:248`. Record `anomaly_missing_close_ts: true` in the day record + digest entry. Day still reconciles. Position is **not** excluded from PnL aggregation — exclusion would silently drop totals if all positions happened to miss exchange ts.
- **D-11:** **Daily Telegram digest = roll-up + anomaly highlights.** Single message containing: total realized PnL (USDT), positions closed count, win/loss split, current ramp stage + clean-day counter, and a list of anomaly-flagged position IDs. Compact, readable on phone. Per-position detail available via dashboard / `pg-admin reconcile show --date=X`.
- **D-12:** **Anomalies inline in digest only.** No separate per-anomaly critical alert at reconcile time. Phase 15 (drawdown breaker) handles fire-now critical cases. Reconcile anomalies are next-day operator review territory, not pages.

### Operator Surfaces
- **D-13:** **pg-admin gets a full surface.** New subcommands:
  - `reconcile run --date=YYYY-MM-DD` — manual fire / re-run for any date
  - `reconcile show --date=YYYY-MM-DD` — display stored daily record
  - `ramp show` — current state (stage, counter, last loss day, demote_count)
  - `ramp reset` — back to stage 1, counter=0, append event
  - `ramp force-promote` — one-stage operator override (counter NOT zeroed)
  - `ramp force-demote` — one-stage operator override (counter zeroed, matches asymmetric ratchet)
- **D-14:** **Read-only dashboard widget in existing Pricegap-tracker tab.** Shows ramp stage, clean-day counter, last loss day, last reconcile date + summary, anomaly list. **No mutation UI** — force-promote / reset / etc. live in pg-admin only this phase. Phase 16 (PG-OPS-09) will absorb this widget when it consolidates the new top-level Pricegap tab.
- **D-15:** **Force-op event log.** All force-promote / force-demote / reset operations:
  1. Write the new ramp state to `pg:ramp:state` (HASH with the 5 explicit fields).
  2. Append a versioned entry to `pg:ramp:events` LIST with `{action, prior_stage, new_stage, operator, timestamp, reason}`.
  3. `force-promote` does **not** zero the clean-day counter (operator override is intentional).
  4. `force-demote` **does** zero the clean-day counter (matches the asymmetric ratchet — any demote zeroes the streak).
  5. `reset` zeroes counter and sets stage = 1.
- **D-16:** **Reconcile fire time hardcoded UTC 00:30.** Single constant. PG-LIVE-03 says "30+ min after UTC 00:00" — 00:30 is the minimum and avoids both Bybit blackout (`:04–:05:30`) and funding settlement windows. No config field. Re-tune by code change + version bump if ever needed.

### Hard Contracts (Carried Forward — Locked by ROADMAP/REQUIREMENTS)
These are NOT gray areas; they are pinned by the spec. Listed here so downstream agents see them in one place:
- **Stages:** 100 / 500 / 1000 USDT/leg (PG-LIVE-01)
- **Promotion threshold:** 7 clean days (PG-LIVE-01)
- **Asymmetric ratchet:** any loss day → counter=0 + demote one stage (PG-LIVE-01, success-criterion #4)
- **5 explicit Redis fields on `pg:ramp:state` HASH:** `current_stage`, `clean_day_counter`, `last_eval_ts`, `last_loss_day_ts`, `demote_count` (PG-LIVE-01)
- **`min(stage_size, hard_ceiling)` enforced at the sizing call site** — not only at stage transitions. A config typo of `stage_3_size_usdt: 9999` must still size at 1000 (PG-LIVE-01, success-criterion #5)
- **Hard ceiling for v2.2:** 1000 USDT/leg (REQUIREMENTS Constraints, Out-of-Scope)
- **Reconcile aggregation key:** `(position_id, version)` (success-criterion #1)
- **Reconcile timestamp source:** exchange close-timestamp (NOT local clock) for aggregation. Local clock only as the missing-ts fallback per D-10.
- **Reconcile output key:** `pg:reconcile:daily:{YYYY-MM-DD}`
- **Ramp gate:** Gate 6 in `risk_gate.go preEntry` (= "gate #7" by spec count, since Gate 0 lockout + Gates 1–5 = 6 existing). New error: `ErrPriceGapRampExceeded`. Reason tag: `"ramp"`.
- **Module isolation:** all new code in `internal/pricegaptrader/`. No imports of `internal/engine/` or `internal/spotengine/`.
- **Default OFF:** every new config flag (`PriceGapLiveCapital`, `PriceGapAnomalySlippageBps` if it gates anything) defaults OFF / safe value.
- **Persistence on `kill -9`:** `pg:ramp:state` written after every state-changing operation (eval, force-op). Restart resumes correct stage + clean-day counter. Locked with a unit test.

### Claude's Discretion
- Exact JSON schema of the `pg:reconcile:daily:{date}` value (fields, ordering — must support D-04 byte-identical re-runs).
- `pg:ramp:events` LIST cap (suggest 500 to match `pg:history` precedent, but Claude decides).
- Boot-time catchup logic (if process starts after UTC 00:30 today and today's reconcile hasn't fired, run-immediately-on-boot vs wait-until-tomorrow).
- Internal struct/interface names for `Reconciler`, `RampController`, the daemon orchestrator. Phase 12 used `PromotionController` + `RedisWSPromoteSink` — analogous shape welcome.
- `PriceGapPosition.Version` field semantics: how it interacts with the `(position_id, version)` aggregation key. Researcher should verify the field exists and document its meaning.
- Pre-flight validation rule preventing live-capital activation without minimum reconcile history (e.g., refuse to flip `live_capital=true` if no `pg:reconcile:daily:*` keys exist for the past 7 days). Design judgment.
- pg-admin output formatting (TSV vs JSON vs human-readable), consistent with Phase 12 precedent.
- Telegram digest message wording, so long as the D-11 required fields are present.
- Ramp config field names beyond the locked set: `PriceGapStage1SizeUSDT` / `PriceGapStage2SizeUSDT` / `PriceGapStage3SizeUSDT` / `PriceGapHardCeilingUSDT` / `PriceGapCleanDaysToPromote` are reasonable; Claude picks final shape.
- Concurrent-evaluator guard for the ramp daemon (single-process is fine; lock if multi-process becomes a concern).
- Test fixtures for asymmetric-ratchet invariants, missing-day catchup, idempotency byte-equality.

### Folded Todos
None — no todos matched Phase 14 scope at session start.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Roadmap & Requirements
- `.planning/ROADMAP.md` §"Phase 14: Daily Reconcile + Live Ramp Controller" — phase goal + 6 success criteria
- `.planning/REQUIREMENTS.md` §"Strategy 4 Live Capital" §**PG-LIVE-01** — ramp controller contract verbatim (5 Redis fields, stages, asymmetric ratchet, hard ceiling, gate #7, idempotent eval)
- `.planning/REQUIREMENTS.md` §"Strategy 4 Live Capital" §**PG-LIVE-03** — daily reconcile contract verbatim (UTC 00:00 + 30 min, idempotent, anomaly flagging, 3-retry, exchange close-ts)
- `.planning/REQUIREMENTS.md` §"Out of Scope" — boundaries (no live capital >1000 USDT/leg, no continuous-curve ramp, no auto-recovery)
- `.planning/REQUIREMENTS.md` §"Constraints (locked at milestone start)" — module boundary, namespace, default-OFF rule
- `.planning/REQUIREMENTS.md` §"Success Criteria" — milestone-level checkpoints (#1, #5 specifically reference Phase 14 deliverables)
- `.planning/PROJECT.md` §"Current Milestone: v2.2 Auto-Discovery & Live Strategy 4" — milestone goal + key context (live-capital risk framing)

### Phase 12 (Direct Dependency — auto-promoted candidates ride this ramp)
- `.planning/phases/12-*-CONTEXT.md` §"Implementation Decisions" — controller wiring, master-toggle pattern, telemetry hooks (analogous shape for ramp + reconcile)
- `.planning/phases/12-*-DISCUSSION-LOG.md` — promote-event-sink pattern (reuse for `pg:ramp:events`)

### Phase 11 (precedent for module wiring + risk_gate composition)
- `.planning/phases/11-auto-discovery-scanner-chokepoint-telemetry/11-CONTEXT.md` §"Integration Points" — telemetry / dashboard / risk_gate wiring conventions
- `.planning/phases/11-auto-discovery-scanner-chokepoint-telemetry/11-CONTEXT.md` §"Established Patterns" — interface-driven DI (D-02), default-OFF master switch (D-11)

### v2.2 Research
- `.planning/research/ARCHITECTURE-v2.2.md` — live-ramp + reconcile architecture (consult before sizing decisions)
- `.planning/research/PITFALLS.md` — Pitfall 7 precedent (regression safety: any latent bugs surfaced during reconcile dev get spawned as separate Phase 999.x hot-fix, not silently merged)
- `.planning/research/SUMMARY.md` — milestone goal alignment (live capital is the v2.2 headline feature)

### Existing Code Anchors (read before modifying)
- `internal/pricegaptrader/risk_gate.go` — current `preEntry` Gates 0–5; new ramp gate inserts as Gate 6 after concentration. Returns `ErrPriceGapRampExceeded` with Reason `"ramp"`.
- `internal/pricegaptrader/monitor.go` lines 147–252 (`closePair` + close path) — site of the new `SADD pg:positions:closed:{date}` write after `pos.ClosedAt` is set.
- `internal/pricegaptrader/notify.go` lines 66–83 (`Notifier` interface + `NoopNotifier`) — add `NotifyPriceGapDailyDigest` method here, then implement in `internal/notify/telegram.go` with `pricegap_digest` allowlist bucket (non-critical).
- `internal/pricegaptrader/promotion.go` (Phase 12 controller) — pattern reference for ramp controller: Redis-persisted state, idempotent eval, event-sink log.
- `internal/pricegaptrader/scanner.go` — goroutine + `time.Ticker` pattern reference for the reconcile daemon.
- `internal/database/pricegap_state.go` lines 18–32 (key namespace) — extend with `pg:positions:closed:{date}`, `pg:ramp:state`, `pg:ramp:events`, `pg:reconcile:daily:{date}`.
- `internal/database/pricegap_state.go` lines 41–65 (`SavePriceGapPosition`) — close-path Redis pipeline; the new `SADD pg:positions:closed:{date}` belongs in the same pipeline (or in `monitor.closePair`, design choice).
- `internal/config/config.go` lines 341–366 (`PriceGap*` config block) — site of new fields: `PriceGapLiveCapital`, `PriceGapStage1SizeUSDT`, `PriceGapStage2SizeUSDT`, `PriceGapStage3SizeUSDT`, `PriceGapHardCeilingUSDT`, `PriceGapAnomalySlippageBps`, `PriceGapCleanDaysToPromote` (final names Claude's discretion).
- `internal/config/config.go` `validatePriceGapDiscovery` (~line 31) — model for adding `validatePriceGapLive` validator (range checks, paper-vs-live consistency).
- `cmd/pg-admin/main.go` — subcommand registration pattern (Phase 8 + 12 added subcommands cleanly without breaking existing ones).
- `internal/models/pricegap.go` — `PriceGapPosition` struct (verify `Version`, `ClosedAt`, `RealizedSlippageBps` fields exist; add exchange-close-ts field if missing).
- `internal/api/` (find pricegap handler file via glob) — add `/api/pg/ramp` and `/api/pg/reconcile/{date}` read-only endpoints.
- `web/src/` (locate Pricegap-tracker tab component) — add Ramp + Reconcile read-only widget.

### Cross-Project History
- `git log --oneline internal/pricegaptrader/` — history of pricegap module evolution (Phase 8 → 9 → 10 → 999.1 → 11 → 12 → 14).
- v1.0 perp-perp engine — source of the 3-retry pattern referenced by PG-LIVE-03. Find via `grep -rn 'retry.*3\|MaxRetries\|backoffSchedule' internal/engine/`.
- v2.0 RETROSPECTIVE.md Pitfall 7 — regression-safety precedent: any latent bug surfaced during Phase 14 dev → spawn 999.x hot-fix mini-phase, not silent merge.
- v0.36.0 (Phase 12 ship) — most recent precedent for adding a controller into the pricegap module without disturbing live trading.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`Tracker` struct** (`internal/pricegaptrader/tracker.go`) — host for new `Reconciler` + `RampController` fields, daemon goroutines spawned alongside scanner + promotion controller.
- **`Notifier` interface** (`internal/pricegaptrader/notify.go`) — already exposes `NotifyPriceGapEntry/Exit/RiskBlock`. Add `NotifyPriceGapDailyDigest` here; `NoopNotifier` gets the no-op stub. Implement in `internal/notify/telegram.go` with `pricegap_digest` allowlist bucket.
- **`models.PriceGapPosition`** — already has `ClosedAt`, `RealizedSlippageBps`, `EntrySpreadBps`, `Status` (`PriceGapStatusClosed`). Verify `Version` field; add exchange-close-ts field if missing.
- **`pg-admin` subcommand registration** — Phase 12 demonstrated clean addition without breaking Phase 8 commands. Reuse the same registrar pattern for `reconcile` and `ramp` subcommands.
- **`pg:history` LIST cap pattern (500)** — reuse for `pg:ramp:events` LIST cap.
- **3-retry pattern** in perp-perp engine — referenced explicitly by PG-LIVE-03; researcher should locate exact location and signature.
- **Telegram allowlist + critical-bypass pattern** (Phase 9, `internal/notify/telegram.go`) — `pricegap_digest` is non-critical; reconcile triple-fail alert IS critical.
- **Risk-gate composition** (`risk_gate.go preEntry`, Gates 0–5) — new ramp gate slots in as Gate 6 with the same telemetry shape (`GateDecision{Err, Reason}`, optional `NotifyPriceGapRiskBlock` for the operator-actionable case).
- **Interface-driven DI** (D-02) — define `RampStateStore` interface, inject `*database.Client` in production, fakes in tests. Same pattern as `DelistChecker`, `ActivePositionChecker` from Phase 11/12.

### Established Patterns
- **Default-OFF master toggle.** `PriceGapDiscoveryEnabled` is the precedent (Phase 11 D-11). `PriceGapLiveCapital` follows: false at fresh install, validates against `PriceGapPaperMode` at config-load.
- **Strict `pg:*` Redis namespace.** All new keys: `pg:positions:closed:{date}`, `pg:ramp:state`, `pg:ramp:events`, `pg:reconcile:daily:{date}`. Documented in `internal/database/pricegap_state.go` namespace block.
- **Config validated at load.** `validatePriceGapDiscovery` is the shape — `validatePriceGapLive` should mirror it: range-check stage sizes, enforce `stage_1 ≤ stage_2 ≤ stage_3 ≤ hard_ceiling`, reject `live_capital=true && paper_mode=true`.
- **Module isolation.** All Phase 14 code in `internal/pricegaptrader/` (controller, reconciler, daemon). No imports of `internal/engine` or `internal/spotengine`. The reconcile retry-pattern reference from PG-LIVE-03 means *imitate the pattern*, not import the package.
- **Commit + version + CHANGELOG together.** Every commit updates `VERSION` + `CHANGELOG.md` (project rule from CLAUDE.local.md).
- **Best-effort Redis writes with logged failure.** `monitor.closePair` uses `_ = t.db.SavePriceGapPosition(pos)` then logs. The new `SADD pg:positions:closed:{date}` follows the same pattern — failure logs but doesn't block close.
- **Phase 16 absorbs.** Phase 14 widget lives in the existing Pricegap-tracker tab and migrates to the new top-level Pricegap tab when PG-OPS-09 ships. Don't pre-build the Phase 16 layout.

### Integration Points
- **`monitor.closePair`** (`internal/pricegaptrader/monitor.go:147`) — add `SADD pg:positions:closed:{YYYY-MM-DD}` after the `pos.ClosedAt` assignment at line 248. Use the date derived from `pos.ClosedAt` (or exchange close-ts when present, falling back per D-10). Best-effort error log.
- **`risk_gate.go preEntry`** — insert Gate 6 after Gate 5 (concentration). Pure-math gate: `if requestedNotionalUSDT > min(stageSize, hardCeiling) { return GateDecision{Err: ErrPriceGapRampExceeded, Reason: "ramp"} }`. Telegram-allowlist `"ramp"` for `NotifyPriceGapRiskBlock`. Skips when `PriceGapLiveCapital=false`.
- **Sizing call site (D-22)** — wrap notional sizing in the same `min(stage_size, hard_ceiling)` math at the source-of-truth call site (probably `tracker.go` entry path or `execution.go`). The gate enforces; the sizer prevents over-sized requests from being formed at all. Two layers, defense in depth.
- **`Tracker` startup** — spawn reconcile daemon + ramp controller goroutines alongside existing scanner / promotion controller, with `context.Context` cancellation for graceful shutdown.
- **API handlers** (`internal/api/` — find pricegap handler file) — add `/api/pg/ramp` (GET, returns ramp state JSON) and `/api/pg/reconcile/{date}` (GET, returns daily record). Both read-only this phase.
- **Frontend Pricegap-tracker tab** (`web/src/` — locate component) — add a "Ramp + Reconcile" panel: ramp stage display, clean-day counter, last loss day, last reconcile summary, anomaly list. Read-only, no mutation UI. i18n keys in en + zh-TW lockstep.
- **`Notifier` impl** (`internal/notify/telegram.go`) — implement `NotifyPriceGapDailyDigest` with new `pricegap_digest` allowlist bucket (non-critical). Reconcile triple-fail uses existing critical alert path.
- **`pg-admin`** — register `reconcile` and `ramp` subcommands. Both share internal types (`Reconciler`, `RampController`) with the daemon — the CLI path and daemon path are two front-ends to the same logic.
- **Config load** — `validatePriceGapLive` invoked from existing config validation chain.

</code_context>

<specifics>
## Specific Ideas

- **"First phase to touch live capital."** The ramp gate is the safety net, but the bigger principle is: every Strategy 4 entry path going forward must consult the ramp gate. PRINCIPLE: sizing must funnel through a single chokepoint that calls `min(stage_size, hard_ceiling)`. Operator can grep one function name to verify all live-capital sizing is gated.
- **Asymmetric ratchet is asymmetric for a reason.** Live capital is dangerous; promotion is slow, demotion is fast. Don't accidentally introduce symmetric semantics in helper code (e.g., a "transition" function that takes a +1 / -1 step parameter). Add a `TestAsymmetricRatchetInvariant` locking the contract: forward step requires 7 cumulative clean days, backward step takes one loss day.
- **Keep `Reconciler` and `RampController` in separate files/types within `pricegaptrader`.** Each unit-tested in isolation. Phase 12 split `PromotionController` from `RedisWSPromoteSink` — same separation-of-concerns shape applies here.
- **Dashboard widget shows "Live capital: ON / OFF" prominently.** Operator must never be confused about whether they are at risk. Color-coded badge (red when live, gray when paper).
- **Idempotency lock test:** literally re-run reconcile twice for the same date and `assert.Equal` on the byte-level Redis value. Cheap to write, catches the entire class of "non-deterministic JSON" bugs.
- **Boot-time guard:** on Tracker startup, if `PriceGapLiveCapital=true` but `pg:ramp:state` is missing, **refuse to start** (panic + Telegram critical alert). Live capital with no ramp state is the most dangerous misconfig possible.

</specifics>

<deferred>
## Deferred Ideas

- **Drawdown circuit breaker** (PG-LIVE-02) — Phase 15.
- **Top-level Pricegap dashboard tab** (PG-OPS-09) — Phase 16. Phase 14 widget lives in existing tab, migrates later.
- **Realized-slippage machine-zero fix** (PG-FIX-01) — Phase 16. Affects D-09 anomaly detection in paper mode but not live-capital mode.
- **Per-fill Telegram alerts with dedicated `pricegap_fill` bucket** (PG-LIVE-04) — descoped from v2.2 per REQUIREMENTS.
- **Score-vs-realized-fill calibration view** (PG-DISC-05) — v2.3+ per REQUIREMENTS.
- **Ramp stages above 1000 USDT/leg** — Out-of-Scope per REQUIREMENTS Out-of-Scope ("hard ceiling for v2.2"). Revisit in v2.3 after live observation.
- **Continuous-curve ramp** — Out-of-Scope per REQUIREMENTS. Discrete stages preferred for cause/effect clarity.
- **Per-symbol or per-pair ramp tier** — single global ramp for v2.2 simplicity.
- **Multi-leg sizing (different size for long vs short leg)** — both legs share `stage_size` in v2.2.
- **Auto-tune of stage sizes from PnL** — Out-of-Scope, manual calibration only.
- **Cross-strategy ramp** (perp-perp + spot-futures + pricegap sharing capital pool) — v3.0 unification milestone.
- **Auto-recovery from drawdown breaker** — Out-of-Scope. Sticky paper flag does not auto-clear (PG-LIVE-02).
- **Gradient promotion within a stage** (e.g., 100 → 200 → 300 → 400 → 500) — Out-of-Scope (continuous curve).
- **`PriceGapReconcileMinuteUTC` configurable** — D-16 chose hardcoded; revisit only if real-world need surfaces.

### Reviewed Todos (not folded)
None — no pending todos matched Phase 14 scope.

</deferred>

---

*Phase: 14-daily-reconcile-live-ramp-controller*
*Context gathered: 2026-04-30*
