# Phase 14: Daily Reconcile + Live Ramp Controller — Research

**Researched:** 2026-04-30
**Domain:** `internal/pricegaptrader/` daily reconcile daemon + Redis-persisted live-capital ramp controller (first phase to touch live capital)
**Confidence:** HIGH (every code anchor verified by file:line; only minor drift on monitor.go assignment line — 247 not 248)

---

## User Constraints (from CONTEXT.md)

### Locked Decisions (D-01..D-16) — research these, NOT alternatives

- **D-01** In-process daemon: goroutine + `time.Ticker` inside `Tracker`, fires UTC 00:30. Shares internal `Reconciler` struct with `pg-admin reconcile run --date=X`.
- **D-02** New dated index `pg:positions:closed:{YYYY-MM-DD}` (SET), SADD'd in `monitor.closePair` after `pos.ClosedAt`. Best-effort error log.
- **D-03** 3-retry pattern (5s/15s/30s); on triple-fail: log + Telegram critical + skip the day. Unreconciled days = ambiguous (no clean credit, no demote).
- **D-04** Idempotency via deterministic ordering (sort by `(close_ts, position_id)`) + canonical JSON (sorted map keys). Locked by unit test.
- **D-05** Clean day = realized PnL ≥ 0 AND ≥1 closed Strategy 4 position. No-activity days HOLD the counter.
- **D-06** `PriceGapLiveCapital` (bool, default false). Live activation requires `paper_mode=false` AND `live_capital=true` (validated together).
- **D-07** Ramp gate is no-op when `PriceGapLiveCapital=false`. Reconcile STILL runs in paper mode (streak data accumulates).
- **D-08** `demote_count` is telemetry only. Phase 15 owns circuit-break primitive — don't double-implement.
- **D-09** `PriceGapAnomalySlippageBps` config field, default 50, validated [0, 500].
- **D-10** Missing exchange close-ts → fall back to `pos.ClosedAt`. Record `anomaly_missing_close_ts: true`. Position still aggregated.
- **D-11** Daily Telegram digest: total realized PnL, positions count, win/loss split, ramp stage + counter, anomaly position IDs.
- **D-12** Anomalies inline in digest only. No per-anomaly critical alert (Phase 15 territory).
- **D-13** Six new pg-admin subcommands: `reconcile run/show`, `ramp show/reset/force-promote/force-demote`.
- **D-14** Read-only dashboard widget in existing Pricegap-tracker tab. NO mutation UI.
- **D-15** Force-op event log: write `pg:ramp:state` HASH + append `pg:ramp:events` LIST. force-promote does NOT zero counter; force-demote DOES.
- **D-16** Reconcile fire time hardcoded UTC 00:30 (single constant, no config).

### Hard Contracts (locked by ROADMAP/REQUIREMENTS — also verified verbatim)

- **Stages:** 100 / 500 / 1000 USDT/leg (PG-LIVE-01)
- **Promotion:** 7 clean days. **Asymmetric ratchet:** any loss day → counter=0 + demote one stage.
- **5 Redis fields on `pg:ramp:state`:** `current_stage`, `clean_day_counter`, `last_eval_ts`, `last_loss_day_ts`, `demote_count`.
- **`min(stage_size, hard_ceiling)` enforced at sizing call site** (defense in depth — gate enforces, sizer prevents over-formed requests).
- **Hard ceiling for v2.2:** 1000 USDT/leg. Anything higher is Out-of-Scope per REQUIREMENTS.
- **Reconcile aggregation key:** `(position_id, version)` (success-criterion #1 — Phase 14 must define `Version` semantics; field absent today, see Open Q1).
- **Reconcile timestamp source:** exchange close-ts; local clock fallback ONLY per D-10.
- **Reconcile output key:** `pg:reconcile:daily:{YYYY-MM-DD}`.
- **Ramp gate:** Gate 6 in `risk_gate.go preEntry` (= "gate #7" by spec count, since Gate 0 lockout + Gates 1–5 = 6 existing). New error: `ErrPriceGapRampExceeded`. Reason tag: `"ramp"`.
- **Module isolation:** all new code in `internal/pricegaptrader/`. No imports of `internal/engine/` or `internal/spotengine/` (perp-perp 3-retry pattern is **imitated**, not imported).
- **Default OFF:** every new config flag defaults OFF / safe.
- **Persistence on `kill -9`:** `pg:ramp:state` written after every state-changing operation. Locked with unit test.

### Claude's Discretion (research resolves these)

JSON schema of `pg:reconcile:daily:{date}`; `pg:ramp:events` LIST cap; boot-time catchup logic; struct names; `PriceGapPosition.Version` semantics; pre-flight live-capital validation; pg-admin output format; digest message wording; final config field names; concurrent-evaluator guard; test fixtures for invariants.

### Deferred Ideas (OUT OF SCOPE)

Phase 15 drawdown breaker (PG-LIVE-02); Phase 16 dashboard tab consolidation (PG-OPS-09); Phase 16 paper-mode bug closure (PG-FIX-01); v2.3+ stages above 1000 USDT/leg; per-fill Telegram alerts (PG-LIVE-04); calibration view (PG-DISC-05); continuous-curve ramp; cross-strategy ramp; auto-recovery from breaker; gradient promotion within stage; configurable reconcile fire time.

---

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| **PG-LIVE-01** | Conservative ramp controller, 5 explicit Redis fields, asymmetric ratchet, `min(stage,ceiling)` at sizing call site, gate #7. | Phase 12 `PromotionController` + `RedisWSPromoteSink` is the proven template (in-memory streak + Redis-persisted sink). For Phase 14 the streak counter MUST be Redis-persisted (HASH `pg:ramp:state`) per Pitfall 3 — diverges from Phase 12 D-03's in-memory streaks. Sizing call site is `tracker.go:560` (`notional := cand.MaxPositionUSDT`) — wrap as `min(min(cand.MaxPositionUSDT, stage_size), hard_ceiling)`. Gate insertion: after Gate 5 concentration in `risk_gate.go:120`, before `return GateDecision{Approved: true}` at line 135. |
| **PG-LIVE-03** | Daily reconcile UTC 00:30, exchange close-ts keyed `(position_id, version)`, idempotent, 3-retry, anomaly flagging. | Reuse 3-retry shape from `internal/engine/exit.go:1119` and `internal/engine/exit.go:3204` — both use `delays := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}` literally. Imitate, do NOT import. Daemon ticker pattern from `tracker.go:158-192` (`scanLoop`). `pg:positions:closed:{date}` SADD inside `monitor.closePair` after line **247** (NOT 248 — see anchor drift below). |

---

## Phase Summary

Phase 14 adds two coupled subsystems inside `internal/pricegaptrader/` — a daily reconcile daemon (UTC 00:30) that aggregates closed Strategy 4 positions for the prior UTC day into byte-identical-on-rerun `pg:reconcile:daily:{date}` Redis records, and a Redis-persisted live-capital ramp state machine (100→500→1000 USDT/leg, asymmetric ratchet) that gates entry sizing through a new `risk_gate.go` Gate 6. Reconcile is the truth-source for the clean-day signal that the ramp consumes; reconcile runs in paper mode too so the streak data is warm when the operator flips `PriceGapLiveCapital=true`. Every new behavior is gated by a config flag defaulting OFF — this is the **first phase to touch live capital**, so conservative-by-default is non-negotiable. Phase 12's `PromotionController` is the structural template (controller + sink + narrow interfaces, daemon goroutine spawned alongside `scanLoop`); the deviation is that **streak state must be Redis-persisted** (Pitfall 3) — Phase 12's in-memory streaks were acceptable because cold-restart cost only 6 cycles, while ramp-controller cold-restart with no Redis state could silently demote or skip a stage transition.

**Primary recommendation:** Structure Phase 14 as **5 sequential plans** — (1) config schema + `PriceGapPosition.Version` field + `pg:positions:closed:{date}` write hook in `monitor.closePair`; (2) `Reconciler` core (TDD: idempotency byte-equality, missing-ts fallback, 3-retry skip-day); (3) `RampController` core (TDD: asymmetric ratchet, hard-ceiling at sizing call site, kill-9 persistence) + `risk_gate.go` Gate 6 + sizer integration; (4) `Tracker` daemon wiring + reconcile→ramp event-bus + 6 pg-admin subcommands + `Notifier.NotifyPriceGapDailyDigest` + Telegram allowlist bucket `pricegap_digest`; (5) read-only dashboard widget in existing `web/src/pages/PriceGap.tsx` + 2 read-only API endpoints + i18n EN/zh-TW lockstep.

---

## Key Findings (per-question answers, file:line citations)

### Q1 — Perp-perp 3-retry pattern location

**Verified [VERIFIED: codebase grep].** Two literal occurrences of the exact `5s/15s/30s` delay schedule:

```go
delays := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}
```

- `internal/engine/exit.go:1119` — within an exit retry loop
- `internal/engine/exit.go:3204` — second occurrence, also exit-path retry

There is NO single helper named `retry3()` to import. PG-LIVE-03 says "reuse perp-perp 3-retry pattern" — per CONTEXT module-isolation rule, the planner must **imitate** the literal slice + `for i, d := range delays { ... time.Sleep(d) }` shape inside `internal/pricegaptrader/`, not import `internal/engine`.

**Other retry shapes nearby (do NOT confuse):**
- `internal/engine/engine.go:4569` uses `5/10/20` — different schedule (alloc-related); reject.
- `internal/engine/consolidate.go: for attempt := 0; attempt < 3; attempt++` — 50ms backoff, GetPosition retry; reject.
- `internal/engine/engine.go:1788` and `:1812` — 3-attempt SavePosition retry with linear `attempt*500ms` and `attempt*1s` delays; reject.

**Recommended pattern for `Reconciler.runForDate`:**

```go
// Source: internal/engine/exit.go:1119 (imitated, not imported — D-15 module isolation)
delays := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}
var lastErr error
for attempt, d := range delays {
    if attempt > 0 {
        select {
        case <-time.After(d):
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    if err := r.aggregateAndWrite(ctx, date); err != nil {
        lastErr = err
        r.log.Warn("reconcile %s attempt %d/3 failed: %v", date, attempt+1, err)
        continue
    }
    return nil // success
}
// triple-fail: critical Telegram + skip day per D-03
r.log.Error("reconcile %s FAILED after 3 attempts: %v", date, lastErr)
r.notifier.NotifyPriceGapReconcileFailure(date, lastErr) // new method, see Q7
return lastErr
```

### Q2 — `models.PriceGapPosition` fields

**Verified [VERIFIED: file read of `internal/models/pricegap_position.go:40-89`].**

| Field | Exists? | Notes |
|-------|---------|-------|
| `ID string` | ✓ | line 41 |
| `Symbol string` | ✓ | line 42 |
| `Status string` | ✓ | line 58, with constant `PriceGapStatusClosed = "closed"` at line 10 |
| `Mode string` | ✓ | line 61 ("paper" / "live") |
| `EntrySpreadBps float64` | ✓ | line 63 |
| `NotionalUSDT float64` | ✓ | line 65 |
| `RealizedSlipBps float64` | ✓ | line 83 (NOT `RealizedSlippageBps` — JSON tag `realized_slippage_bps` differs from Go field name) |
| `RealizedPnL float64` | ✓ | line 84 |
| `OpenedAt time.Time` | ✓ | line 87 |
| `ClosedAt time.Time` | ✓ | line 88 |
| `Version` | **✗ MISSING** | Phase 14 must add this. Open Q1 below. |
| Exchange-close-ts | **✗ MISSING** | CONTEXT D-10 says "add if missing" — confirmed missing; Phase 14 adds. |

**Decisions for Phase 14 (research recommendation):**

1. **Add `Version int \`json:"version,omitempty"\`** to `PriceGapPosition`. Semantics: 0 (omitted) on legacy/pre-Phase-14 records (decoded as zero-value `int`), 1 on new records, incremented if a closed position is amended (e.g., late fill correction). The `(position_id, version)` aggregation key is then idempotent: reconcile re-runs see the same `(id, version)` set.

2. **Add `ExchangeClosedAt time.Time \`json:"exchange_closed_at,omitempty"\`** stamped from the exchange's order/position close timestamp inside `monitor.closePair`. When unavailable, the field is zero — reconcile then falls back to `ClosedAt` per D-10 and flags `anomaly_missing_close_ts: true`.

Both additions are backward-compatible (Go `encoding/json` ignores unknown fields and zero-fills missing ones, matching the established `Mode` field precedent at line 61).

### Q3 — `risk_gate.go preEntry` Gates 0–5

**Verified [VERIFIED: file read of `internal/pricegaptrader/risk_gate.go:1-150`].** Current gate ordering (NOTE: code comment says "6 deterministic gates" but there are actually 7 — Gate 0 lockout was added later):

| Gate | Line | Purpose | Telegram allowlist tag | Reason tag |
|------|------|---------|------------------------|------------|
| Gate 0 | 58–64 | Per-candidate re-entry lockout (duplicate active pos) | none — sustained market signal | `"duplicate_candidate"` |
| Gate 1 | 67–74 | Exec-quality disabled flag | `"exec_quality"` | `"exec_quality: " + reason` |
| Gate 2 | 77–81 | Max concurrent (PG-RISK-04) | `"max_concurrent"` | `"max_concurrent"` |
| Gate 3 | 83–88 | Per-position notional cap (PG-RISK-05) | none — caller-side bug | `"per_position_cap"` |
| Gate 4 | 90–100 | Budget remaining | `"budget"` | `"budget"` |
| Gate 5 | 102–120 | Gate.io concentration cap 50% (PG-RISK-01) | `"concentration"` | `"concentration"` |
| Gate 6 (current) | 122–133 | Delist + staleness (PG-RISK-02) | `"delist"` / `"kline_stale"` | same |

**Phase 14 ramp gate insertion site (research recommendation):**

The literal phrase "Gate 6" is currently used by the delist/staleness gate. CONTEXT.md says the new ramp gate "inserts after Gate 5 (concentration)" → must become **Gate 6 ramp**, pushing delist+staleness to **Gate 7**. Renumber comments + the `TestRiskGate_OrderingInvariant` lock test (CONTEXT mentions it but I did not verify a test of that exact name — Open Q4). The function comment at line 17 must be updated from "6 deterministic gates" to "8" (Gate 0 + Gates 1–7).

```go
// New Gate 6 — insert after Gate 5 concentration block ending at line 120,
// before "Gate 6: delist..." (which becomes new Gate 7).
//
// D-07: no-op when PriceGapLiveCapital=false. D-22 (sizing call site) +
// this gate enforce min(stage_size, hard_ceiling) twice — defense in depth.
if t.cfg.PriceGapLiveCapital {
    state, err := t.rampStore.LoadRampState()
    if err != nil {
        // Fail-closed: no ramp state with live_capital=true is the most
        // dangerous misconfig possible (CONTEXT specifics §6 boot guard
        // already prevents start-up; this is a runtime defense).
        t.notifier.NotifyPriceGapRiskBlock(cand.Symbol, "ramp", "ramp state read error: " + err.Error())
        return GateDecision{Err: ErrPriceGapRampStateUnavailable, Reason: "ramp"}
    }
    stageSize := stageSizeUSDT(state.CurrentStage, t.cfg) // helper looks up Stage1/2/3 size
    cap := math.Min(stageSize, t.cfg.PriceGapHardCeilingUSDT)
    if requestedNotionalUSDT > cap {
        t.notifier.NotifyPriceGapRiskBlock(cand.Symbol, "ramp",
            fmt.Sprintf("requested=$%.0f stage=%d stage_size=$%.0f ceiling=$%.0f cap=$%.0f",
                requestedNotionalUSDT, state.CurrentStage, stageSize, t.cfg.PriceGapHardCeilingUSDT, cap))
        return GateDecision{Err: ErrPriceGapRampExceeded, Reason: "ramp"}
    }
}
```

`GateDecision` shape unchanged: `{Approved bool, Err error, Reason string}` from line 11–15.

### Q4 — `monitor.go closePair` SADD insertion site

**Verified [VERIFIED: file read of `internal/pricegaptrader/monitor.go` near close path].** **Anchor drift detected — CONTEXT says "near line 248", actual is line 247:**

```go
// Line 245–252 actual:
pos.ExitReason = reason
pos.Status = models.PriceGapStatusClosed   // line 246 (CONTEXT said 247)
pos.ClosedAt = time.Now()                   // line 247 (CONTEXT said 248) ← INSERT AFTER THIS
if err := t.db.SavePriceGapPosition(pos); err != nil {        // line 248
    t.log.Warn("pricegap: SavePriceGapPosition(closed) failed for %s: %v", pos.ID, err)  // line 249
}
_ = t.db.RemoveActivePriceGapPosition(pos.ID)
_ = t.db.AddPriceGapHistory(pos)
```

**Insertion (Phase 14 Plan 1):**

```go
pos.ClosedAt = time.Now()
// Phase 14 D-02: dated index for O(1) reconcile lookup. Best-effort (matches
// the existing pattern at lines 248–249).
date := pos.ExchangeClosedAt
if date.IsZero() {
    date = pos.ClosedAt
}
if err := t.db.AddPriceGapClosedPositionForDate(pos.ID, date); err != nil {
    t.log.Warn("pricegap: AddPriceGapClosedPositionForDate failed for %s: %v", pos.ID, err)
}
if err := t.db.SavePriceGapPosition(pos); err != nil {
    ...
}
```

The SADD itself lives in `internal/database/pricegap_state.go` as a new exported method `AddPriceGapClosedPositionForDate(posID string, date time.Time)` that constructs the key `pg:positions:closed:" + date.UTC().Format("2006-01-02")` and SADDs the ID.

### Q5 — `internal/pricegaptrader/promotion.go` (Phase 12 controller pattern)

**Verified [VERIFIED: file read of `promotion.go:1-360` and `promote_event_sink.go:1-100`].** This is the structural template Phase 14 follows:

- **Controller struct shape** (line 322–342): owns dependencies as narrow interfaces (`RegistryWriter`, `ActivePositionChecker`, `PromoteEventSink`, `PromoteNotifier`, `TelemetrySink`, `*config.Config`, `*utils.Logger`, `nowFunc`). State maps owned exclusively by the controller and touched only inside `Apply`.
- **Idempotent eval pattern**: `Apply(ctx context.Context, summary CycleSummary)` is called synchronously at the end of `Scanner.RunCycle` (D-16 in Phase 12). Single goroutine, no channel plumbing.
- **Event-sink composition** (`promote_event_sink.go:53-110`): `RedisWSPromoteSink` implements `PromoteEventSink.Emit(ctx, ev)` by RPUSH + LTRIM 1000 to `pg:promote:events` + WS broadcast. `WSBroadcaster` is a narrow interface (line 88) so this file does NOT import `internal/api` — preserves D-15 module boundary. **Phase 14 ramp event log mirrors this pattern exactly.**
- **Daemon goroutine wiring** (`tracker.go:142-147`): controller is constructed in `cmd/main.go`, attached to `Scanner` via `NewScanner(...)`, and runs synchronously inside `scanLoop`'s ticker. **Phase 14's `RampController.Eval()` runs synchronously inside the new `reconcileLoop`'s ticker (after reconcile completes for the prior UTC day) — single goroutine, no race.**

**Phase 14 mirror:**

| Phase 12 | Phase 14 (recommended) |
|----------|------------------------|
| `PromotionController` | `RampController` |
| `PromoteEvent` | `RampEvent` (action: `evaluate`/`force_promote`/`force_demote`/`reset`, prior_stage, new_stage, operator, ts, reason) |
| `PromoteEventSink` | `RampEventSink` (RPUSH + LTRIM 500 to `pg:ramp:events`, recommend cap **500** matching `pg:history` precedent — see CONTEXT discretion) |
| `RedisWSPromoteSink` | `RedisWSRampSink` |
| `PromoteNotifier` | `RampNotifier` (Telegram on force-op + on demote, NOT on every clean-day eval — too chatty) |
| `pg:promote:events` LIST cap 1000 | `pg:ramp:events` LIST cap 500 |
| in-memory streak map (D-03) | **Redis-persisted `pg:ramp:state` HASH** — Pitfall 3 mandates this. |
| `Apply(ctx, summary)` | `Eval(ctx, prevDayDate, dailyRecord)` — called by reconcile daemon after it writes a successful `pg:reconcile:daily:{date}` |

### Q6 — `internal/pricegaptrader/scanner.go` ticker pattern

**Verified [VERIFIED: file read of `tracker.go:158-192`].** The `scanLoop` is the canonical pattern:

```go
// tracker.go:158-192 verbatim shape:
func (t *Tracker) scanLoop() {
    defer t.wg.Done()
    interval := t.scanInterval
    if interval <= 0 { ... }
    firstTick := time.After(t.scanFirstTickOffset)
    var ticker *time.Ticker

    runOnce := func() {
        ctx, cancel := context.WithTimeout(context.Background(), interval)
        defer cancel()
        t.scanner.RunCycle(ctx, time.Now())
    }

    for {
        select {
        case <-t.stopCh:
            if ticker != nil { ticker.Stop() }
            return
        case <-firstTick:
            ticker = time.NewTicker(interval)
            runOnce()
        case <-chanTick(ticker):
            runOnce()
        }
    }
}
```

**Phase 14 `reconcileLoop` (Plan 4 daemon goroutine):**

```go
// Imitates tracker.go:158-192. Fires UTC 00:30 daily (D-16).
func (t *Tracker) reconcileLoop() {
    defer t.wg.Done()
    runOnce := func() {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
        defer cancel()
        date := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02") // prior UTC day
        if err := t.reconciler.RunForDate(ctx, date); err != nil {
            t.log.Error("reconcile daily failed for %s: %v", date, err)
            return // RunForDate already emits critical Telegram on triple-fail
        }
        // Reconcile success → drive ramp evaluator (D-05 clean-day signal)
        record, err := t.reconciler.LoadRecord(ctx, date)
        if err == nil {
            t.ramp.Eval(ctx, date, record)
        }
        // Daily Telegram digest (D-11)
        t.notifier.NotifyPriceGapDailyDigest(date, record, t.ramp.Snapshot())
    }
    for {
        next := nextUTCFireTime(time.Now()) // 00:30 UTC each day
        select {
        case <-t.stopCh:
            return
        case <-time.After(time.Until(next)):
            runOnce()
        }
    }
}
```

`nextUTCFireTime` is a pure function — easy to unit-test against frozen `now`.

### Q7 — `internal/pricegaptrader/notify.go` Notifier interface

**Verified [VERIFIED: file read of `notify.go:66-98`].** Current interface (line 89–98):

```go
type PriceGapNotifier interface {
    NotifyPriceGapEntry(pos *models.PriceGapPosition)
    NotifyPriceGapExit(pos *models.PriceGapPosition, reason string, pnl float64, duration time.Duration)
    NotifyPriceGapRiskBlock(symbol, gate, detail string)
}
```

`NoopNotifier` at line 100–112 implements all three as no-ops.

**Phase 14 additions (research recommendation):**

```go
// Add to PriceGapNotifier interface:
NotifyPriceGapDailyDigest(date string, record DailyReconcileRecord, ramp RampSnapshot)
NotifyPriceGapReconcileFailure(date string, err error) // critical (D-03 triple-fail path)

// Add Noop stubs to NoopNotifier matching established pattern.
```

`DailyReconcileRecord` and `RampSnapshot` are new value types declared in `pricegaptrader` (NOT in `models` — they are notification DTOs, not persisted shapes).

### Q8 — `internal/notify/telegram.go` allowlist + critical-bypass

**Verified [VERIFIED: file read of `telegram.go:339-470`].**

- **Risk-block allowlist** (line 340–349): `priceGapGateAllowlist = map[string]struct{}{...}` with current gate names `concentration / max_concurrent / kline_stale / delist / budget / exec_quality`. **Phase 14 must add `"ramp"` to this map** — the new ramp-block notification uses gate name `"ramp"` per CONTEXT integration-points. T-09-17 unknown-gate rejection means the notification is silently dropped if not added.
- **Per-event cooldown** via `checkCooldownAt(eventKey, now)` at line ~206 (5-min cooldown). Reused by `NotifyPriceGapRiskBlock` (line 1054–1066) with eventKey `"pg_risk:" + gate + ":" + symbol`.
- **Sanitization**: `sanitizeForTelegram(s, max)` at line 975–993 strips control chars + truncates to max bytes. Required for any operator-derived detail string.
- **Paper vs live tagging** (line 998–1006): `priceGapTag(pos)` returns `("LIVE", "")` or `("PAPER", "📝 PAPER ")`. Phase 14 digest uses the LIVE tag when `PriceGapLiveCapital=true`, PAPER when false (digest fires in both modes per D-07).

**For `NotifyPriceGapDailyDigest` (new):** non-critical (matches D-11 — anomalies inline, not paged). Use a fresh cooldown key like `"pg_daily_digest:" + date` (one-shot per UTC day), 5-min cooldown is irrelevant since the daemon fires once daily. CONTEXT mentions "`pricegap_digest` allowlist bucket" — but the existing `priceGapGateAllowlist` is a **gate-name** allowlist, not a notification-bucket separation. Recommendation: **the digest does NOT need allowlist treatment** because it's not driven by an arbitrary gate name; it's a hardcoded notification with a fixed event-key construction. The "bucket" terminology in CONTEXT is loose — what's actually needed is that the digest path bypasses no critical infrastructure (it's plain non-critical).

For `NotifyPriceGapReconcileFailure` (D-03 critical): goes through whichever critical-bypass path exists for L4/L5 alerts. I did NOT find a "critical bypass" mechanism explicitly named in the file slice I read; recommend the planner verify at planning time whether `(*TelegramNotifier).Send` is sufficient or whether a `SendCritical` exists. (Open Q3.)

### Q9 — `internal/database/pricegap_state.go` namespace + save path

**Verified [VERIFIED: file read of `pricegap_state.go:1-100`].** Current namespace block (lines 14–28):

```go
const (
    keyPricegapPositions      = "pg:positions"           // HSET id -> JSON
    keyPricegapActive         = "pg:positions:active"    // SET of active ids
    keyPricegapHistory        = "pg:history"             // LIST, capped 500
    keyPricegapDisabledPrefix = "pg:candidate:disabled:"
    keyPricegapSlippagePrefix = "pg:slippage:"

    priceGapHistoryCap  = 500
    priceGapSlippageCap = 10
    priceGapLockPrefix  = "arb:locks:pg:"
)
```

**Phase 14 additions (research recommendation, Plan 1):**

```go
// Add to namespace block:
keyPricegapClosedPrefix       = "pg:positions:closed:"   // SET per-day, key = prefix + YYYY-MM-DD
keyPricegapReconcileDailyPref = "pg:reconcile:daily:"    // STRING per-day, key = prefix + YYYY-MM-DD
keyPricegapRampState          = "pg:ramp:state"          // HASH, 5 explicit fields
keyPricegapRampEvents         = "pg:ramp:events"         // LIST, capped 500

priceGapRampEventsCap = 500   // matches pg:history precedent
```

**New methods on `*Client`** (interface methods land on `models.PriceGapStore` for DI):

| Method | Purpose |
|--------|---------|
| `AddPriceGapClosedPositionForDate(posID string, date time.Time) error` | SADD into `pg:positions:closed:{YYYY-MM-DD}` |
| `GetPriceGapClosedPositionsForDate(date string) ([]string, error)` | SMEMBERS for reconcile aggregation |
| `SavePriceGapReconcileDaily(date string, payload []byte) error` | SET (overwrite — D-04 byte-identical re-runs) |
| `LoadPriceGapReconcileDaily(date string) ([]byte, bool, error)` | GET; returns (data, exists, err) |
| `SavePriceGapRampState(s RampState) error` | HSET 5 fields atomically (Pipeline) |
| `LoadPriceGapRampState() (RampState, bool, error)` | HGETALL + decode |
| `AppendPriceGapRampEvent(ev RampEvent) error` | RPUSH + LTRIM 500 |

The `*Client` does not implement `PriceGapStore` directly — Phase 8 at line 31 declares `var _ models.PriceGapStore = (*Client)(nil)` to lock the interface conformance; Phase 14 extends `PriceGapStore` with the new methods (in `internal/models/`), then `*Client` implements them.

### Q10 — `internal/config/config.go` PriceGap block

**Verified [VERIFIED: file read of config.go:1170-1248].** Existing `PriceGap*` fields are at lines 1172–1198 (struct), with the JSON DTO `jsonPriceGap` at lines 1224–1248. `validatePriceGapDiscovery` is at line 31 (NOT line 31 in the original CONTEXT pointer to "~line 31" — verified).

**Phase 14 additions (research recommendation, Plan 1):**

```go
// In Config struct, after PriceGapMaxCandidates (line 1197):
PriceGapLiveCapital         bool    // D-06 default false; live-capital activation flag
PriceGapStage1SizeUSDT      float64 // default 100; PG-LIVE-01
PriceGapStage2SizeUSDT      float64 // default 500; PG-LIVE-01
PriceGapStage3SizeUSDT      float64 // default 1000; PG-LIVE-01
PriceGapHardCeilingUSDT     float64 // default 1000; v2.2 hard ceiling
PriceGapAnomalySlippageBps  float64 // D-09 default 50; range [0, 500]
PriceGapCleanDaysToPromote  int     // default 7; PG-LIVE-01 promotion threshold

// In jsonPriceGap struct, after MaxCandidates (line 1247):
LiveCapital         *bool    `json:"live_capital,omitempty"`
Stage1SizeUSDT      *float64 `json:"stage_1_size_usdt,omitempty"`
Stage2SizeUSDT      *float64 `json:"stage_2_size_usdt,omitempty"`
Stage3SizeUSDT      *float64 `json:"stage_3_size_usdt,omitempty"`
HardCeilingUSDT     *float64 `json:"hard_ceiling_usdt,omitempty"`
AnomalySlippageBps  *float64 `json:"anomaly_slippage_bps,omitempty"`
CleanDaysToPromote  *int     `json:"clean_days_to_promote,omitempty"`
```

**New `validatePriceGapLive(c *Config) error`** modeled on `validatePriceGapDiscovery` at line 31:

```go
func validatePriceGapLive(c *Config) error {
    if c.PriceGapLiveCapital && c.PriceGapPaperMode {
        return fmt.Errorf("PriceGapLiveCapital=true requires PriceGapPaperMode=false (D-06)")
    }
    if c.PriceGapStage1SizeUSDT < 0 || c.PriceGapStage1SizeUSDT > c.PriceGapStage2SizeUSDT {
        return fmt.Errorf("PriceGapStage1SizeUSDT=%v must be ≥0 and ≤ Stage2", c.PriceGapStage1SizeUSDT)
    }
    if c.PriceGapStage2SizeUSDT > c.PriceGapStage3SizeUSDT {
        return fmt.Errorf("Stage2 must be ≤ Stage3")
    }
    if c.PriceGapStage3SizeUSDT > c.PriceGapHardCeilingUSDT {
        return fmt.Errorf("Stage3=%v exceeds HardCeiling=%v", c.PriceGapStage3SizeUSDT, c.PriceGapHardCeilingUSDT)
    }
    if c.PriceGapHardCeilingUSDT > 1000 {
        return fmt.Errorf("HardCeiling=%v exceeds v2.2 ceiling 1000", c.PriceGapHardCeilingUSDT)
    }
    if c.PriceGapAnomalySlippageBps < 0 || c.PriceGapAnomalySlippageBps > 500 {
        return fmt.Errorf("PriceGapAnomalySlippageBps=%v outside [0,500]", c.PriceGapAnomalySlippageBps)
    }
    if c.PriceGapCleanDaysToPromote < 1 || c.PriceGapCleanDaysToPromote > 30 {
        return fmt.Errorf("PriceGapCleanDaysToPromote=%d outside [1,30]", c.PriceGapCleanDaysToPromote)
    }
    return nil
}
```

Pre-flight live-capital activation rule (CONTEXT discretion item): refuse to flip `PriceGapLiveCapital=true` if `pg:reconcile:daily:*` keys do not exist for at least the last `PriceGapCleanDaysToPromote` days. Implemented in `validatePriceGapLive` only if a `*database.Client` is reachable at config-load (likely not — config loads before Redis connects). **Recommendation: enforce at runtime in `Tracker.Start()` boot guard (CONTEXT specifics §6) instead of config-load.** If `PriceGapLiveCapital=true` and `pg:ramp:state` is missing OR fewer than 7 reconcile keys exist for past 7 days, panic + critical Telegram + refuse to start.

### Q11 — `cmd/pg-admin/main.go` registrar pattern

**Verified [VERIFIED: file read of `pg-admin/main.go` lines 1-1686].** Current dispatch is a switch in `Run(args []string, deps Dependencies) int` at line 1346. Each subcommand has a `cmdXxx` function. Output format precedent:

- **TSV-ish via `text/tabwriter`** for `cmdPositionsList` (line 1660 — table format).
- **JSON-indent** for `cmdCandidatesList` (line 1442 — `json.NewEncoder(stdout); SetIndent("", "  ")`).
- **Human-readable Fprintf** for `cmdStatus` (line 1591 — labeled key/value).

**Phase 14 6 new subcommands (Plan 4):**

| Subcommand | Output format | Rationale |
|------------|---------------|-----------|
| `pg-admin reconcile run --date=YYYY-MM-DD` | Human + structured exit code | Operator runs, gets pass/fail summary; exit code: 0=success, 1=skipped (already exists), 2=triple-fail. |
| `pg-admin reconcile show --date=YYYY-MM-DD` | JSON-indent | Mirror `candidates list` — programmatic consumption + readable. |
| `pg-admin ramp show` | Human-readable Fprintf table | Mirror `cmdStatus` — operator scans visually. |
| `pg-admin ramp reset` | Human + exit code | One-liner confirmation + event log append. |
| `pg-admin ramp force-promote` | Human + exit code | One-liner confirmation. force-promote does NOT zero counter (D-15 #3). |
| `pg-admin ramp force-demote` | Human + exit code | force-demote DOES zero counter (D-15 #4). |

`Dependencies` struct gets two new fields: `Reconciler *pricegaptrader.Reconciler` and `Ramp *pricegaptrader.RampController`. Production `main()` at line 1568 wires them after `pricegaptrader.NewRegistry`. The CLI path and daemon path share the same internal types — Operator running `reconcile run --date=2026-04-29` calls into the same `Reconciler.RunForDate(ctx, "2026-04-29")` method the daemon calls.

### Q12 — API handler for pricegap routes

**Verified [VERIFIED: file read of `internal/api/server.go:182-184` and `pricegap_discovery_handlers.go`].** Existing pricegap handlers:

```go
// server.go:182-184
mux.HandleFunc("GET /api/pricegap/state", s.cors(s.authMiddleware(s.handlePriceGapState)))
mux.HandleFunc("GET /api/pricegap/candidates", s.cors(s.authMiddleware(s.handlePriceGapCandidates)))
mux.HandleFunc("GET /api/pricegap/positions", s.cors(s.authMiddleware(s.handlePriceGapPositions)))

// Plus Phase 11 discovery handlers (file: pricegap_discovery_handlers.go):
// GET /api/pg/discovery/state
// GET /api/pg/discovery/scores/{symbol}
```

**Note:** Two route prefix conventions exist in the codebase — `/api/pricegap/*` (Phase 8/9) and `/api/pg/*` (Phase 11). CONTEXT specifies `/api/pg/ramp` and `/api/pg/reconcile/{date}` — use the **Phase 11 `/api/pg/*` prefix** (matches discovery handlers, more compact).

**Phase 14 additions (Plan 5, both read-only):**

```go
// New file: internal/api/pricegap_ramp_handlers.go
mux.HandleFunc("GET /api/pg/ramp", s.cors(s.authMiddleware(s.handlePgRampState)))
mux.HandleFunc("GET /api/pg/reconcile/{date}", s.cors(s.authMiddleware(s.handlePgReconcileDay)))
```

`handlePgRampState` returns `{ok, data: RampState{stage, counter, last_eval_ts, last_loss_day_ts, demote_count}}` envelope. `handlePgReconcileDay` reads `{date}` from path, validates `YYYY-MM-DD` regex, returns the full daily record JSON via `LoadPriceGapReconcileDaily`. 404 with `{ok:false, error:"reconcile not found for date"}` when key absent.

### Q13 — Frontend Pricegap-tracker tab

**Verified [VERIFIED: file glob and grep — exists at `web/src/pages/PriceGap.tsx`, ~1500 lines].** The page already has:

- Header with paper/live mode badge, budget stat, debug-log toggle (lines 700–770).
- `<DiscoverySection />` at line 768 (Phase 11) — sub-section pattern Phase 14 mirrors.
- Candidates table (lines 800–894).
- Live Positions table (lines 901–980).
- Closed Log table (lines 986–1085).
- Rolling Metrics table (lines 1102–1172).

**Phase 14 widget insertion (Plan 5):**

```tsx
// Add after <DiscoverySection /> at line 772, before the Candidates section.
// New component: web/src/components/Ramp/RampReconcileSection.tsx
<RampReconcileSection />
```

The component:
- Mounts: fetches `/api/pg/ramp` + `/api/pg/reconcile/{yesterday}` once.
- Live updates: subscribes to a new WS event `pg_ramp_event` (planner adds; reuses existing WS hub pattern from `s.hub.Broadcast(...)`).
- Renders read-only:
  - **Big live-capital indicator** — red badge "LIVE CAPITAL: ON" / gray badge "LIVE CAPITAL: OFF" (CONTEXT specifics §4).
  - Ramp stage (1/2/3 with USDT/leg labels).
  - Clean-day counter (e.g., "5 / 7" with progress bar).
  - Last loss day (date or "never").
  - Demote count (with caveat tooltip: "Operator review only — auto circuit-break is Phase 15").
  - Last reconcile date + summary (PnL total, position count, anomaly count).
  - Anomaly list (collapsed by default, expand to see flagged position IDs).
- **No mutation UI** — D-14 is explicit. Force-promote / reset / force-demote stay in `pg-admin`.

**i18n keys** added to BOTH `web/src/i18n/en.ts` AND `web/src/i18n/zh-TW.ts` lockstep (project rule from CLAUDE.local.md). Existing `pricegap.*` namespace; new keys: `pricegap.ramp.title`, `pricegap.ramp.stage`, `pricegap.ramp.cleanDayCounter`, `pricegap.ramp.lastLossDay`, `pricegap.ramp.demoteCount`, `pricegap.ramp.liveCapitalOn`, `pricegap.ramp.liveCapitalOff`, `pricegap.reconcile.title`, `pricegap.reconcile.totalPnl`, `pricegap.reconcile.positionsClosed`, `pricegap.reconcile.winLossSplit`, `pricegap.reconcile.anomalies`, `pricegap.reconcile.noAnomalies`. Phase 12 (`PromoteTimeline`) added i18n in lockstep — same approach.

### Q14 — Boot-time catchup logic (Claude's discretion)

**Recommendation: hybrid — run-immediately-on-boot ONLY if today's reconcile hasn't fired AND we're past 00:30 UTC.**

Rationale: a clean cold-restart at, say, 14:00 UTC means we've missed today's 00:30 fire. If we wait until tomorrow 00:30, the ramp evaluator gets 24+ hours stale data; if a losing day was yesterday, the demote action is delayed by a day — risky. But if we restart at 00:25 UTC (5 min before fire time), running immediately would double-fire: once on boot, once at 00:30. So:

```go
// In reconcileLoop, before entering the for-select:
date := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
_, exists, err := t.reconciler.LoadRecord(ctx, date)
if err == nil && !exists && time.Now().UTC().Hour() >= 1 {
    // We're past 00:30 UTC and yesterday's reconcile hasn't fired.
    // Run immediately, then settle into normal cadence.
    runOnce()
}
```

This matches Phase 12's "no boot-time catchup" approach for promotion (D-03 in-memory streak resets) — but ramp is Redis-persisted, so catchup IS safe and IS desirable. **Trade-off:** the 1-hour grace window (00:30 → 01:00) is the dead zone where boot-time catchup might race the scheduled fire; acceptable because byte-equality (D-04) means a double-fire on the same date produces identical output.

### Q15 — Idempotency contract for `pg:reconcile:daily:{date}` (Claude's discretion)

**Recommended JSON schema:**

```json
{
  "schema_version": 1,
  "date": "2026-04-29",
  "computed_at": "2026-04-30T00:30:14Z",
  "totals": {
    "realized_pnl_usdt": 12.34,
    "positions_closed": 3,
    "wins": 2,
    "losses": 1,
    "net_clean": true
  },
  "positions": [
    {
      "id": "SOONUSDT_binance_bybit_1234",
      "version": 1,
      "exchange_closed_at": "2026-04-29T22:14:55Z",
      "fallback_local_close_at": false,
      "realized_pnl_usdt": 5.67,
      "realized_slippage_bps": 8.2,
      "anomaly_high_slippage": false,
      "anomaly_missing_close_ts": false
    },
    ...
  ],
  "anomalies": {
    "high_slippage_count": 0,
    "missing_close_ts_count": 0,
    "flagged_ids": []
  }
}
```

**Serialization library:** Go stdlib `encoding/json` is NOT deterministic for maps by default. **Use a custom marshaller that emits sorted map keys** OR define every field as a typed struct (Go reflects struct fields in declaration order — deterministic). **Recommend the typed-struct route**: declare `DailyReconcileRecord`, `DailyReconcileTotals`, `DailyReconcilePosition`, `DailyReconcileAnomalies` structs with explicit fields. `json.Marshal` then produces byte-identical output for byte-identical input. Sort `positions` by `(exchange_closed_at, id)` ascending before marshalling. Sort `flagged_ids` ascending. **Locked by `TestReconcileIdempotency` unit test** that runs `aggregateAndWrite` twice for the same fixture and asserts byte equality of the two `pg:reconcile:daily:{date}` STRING values.

### Q16 — Validation Architecture (Nyquist Dimension 8)

See dedicated section below.

### Q17 — Pitfalls / regression-safety surfaced in current code

**Verified [VERIFIED: PITFALLS.md Pitfall 1, 2, 3, 7].** No suspicious code surfaced during research; all the pitfalls documented are *expected* by the Phase 14 design:

- **Pitfall 1** (auto-discovery false-positive pump): not Phase 14 — already mitigated by Phase 11 scanner persistence + freshness gates.
- **Pitfall 2** (three-writer race on `cfg.PriceGapCandidates`): not Phase 14 — already mitigated by Phase 11 chokepoint. Phase 14 adds a fourth writer (no — Phase 14 does NOT mutate `PriceGapCandidates`; it only reads them). **No new race surface.**
- **Pitfall 3** (stale clean-days counter across restarts): **THIS IS THE ONE** Phase 14 directly addresses. PITFALLS.md prescription matches CONTEXT D-15 / hard-contracts exactly: 5 explicit Redis fields, "clean day" precisely defined, idempotent eval (last_evaluated_day guard), `min(stage_size, hard_ceiling)` at sizing, asymmetric ratchet. **Phase 14 plan must include all 6 PITFALLS.md "How to avoid" items as test cases** — they map 1:1 to the success criteria.
- **Pitfall 7** (tech-debt closure surfacing latent regressions): Phase 14 is NOT a tech-debt phase, but the **regression-spawn-not-silent-fix** rule applies. Recommendation: if Phase 14 implementation surfaces a v0.36.x bug (e.g., a dashboard config-write that flips paper_mode in some race), open a Phase **999.2** hot-fix mini-phase rather than silently fixing inside Phase 14. Documented as Pitfall in §"Pitfalls / Regression Risks" below.

### Q18 — Cross-phase consistency with Phase 12

**Verified [VERIFIED: Phase 12 CONTEXT.md §"Implementation Decisions" lines 1-200].** Phase 12 patterns Phase 14 mirrors:

- **Default-OFF master switch** (D-07 Phase 12) → `PriceGapLiveCapital` (D-06 Phase 14) + paper-mode independence (D-06 Phase 14).
- **Synchronous in-cycle work** (D-16 Phase 12, controller called in `RunCycle`) → Phase 14 RampController.Eval called synchronously by reconcile daemon after successful reconcile write.
- **Narrow interface DI for module boundary** (D-15 Phase 12, declares `PromoteEventSink` etc. inside `promotion.go`) → Phase 14 declares `RampStateStore`, `RampEventSink`, `RampNotifier`, `ReconcileStore` inside `ramp.go` / `reconciler.go`. No imports of `internal/api`, `internal/database` directly — wiring at `cmd/main.go` adapter layer.
- **Naming**: Phase 12 used `PromotionController` + `RedisWSPromoteSink` → Phase 14 uses `RampController` + `RedisWSRampSink` and `Reconciler` + `RedisReconcileStore`. Analogous shape.
- **Telegram cooldown via per-event-key** (D-13 Phase 12) → Phase 14 reuses the same `(*TelegramNotifier).checkCooldown(eventKey)` primitive at telegram.go:206. No new cooldown infrastructure needed.

**Divergence (deliberate):**
- Phase 12 streak storage was **in-memory** (D-03) — cold restart resets, acceptable for promotion. Phase 14 streak storage is **Redis-persisted** — Pitfall 3 mandates this for live capital. Different signal, different cost.
- Phase 12 had a single sync.Mutex-free streak map (one writer = scanner goroutine). Phase 14 has TWO writers to `pg:ramp:state`: the daemon eval path AND the pg-admin force-op path. **Recommendation: lock with Redis SET NX TTL (existing `priceGapLockPrefix` precedent) wrapping any state-changing operation.** Single-binary deployment makes this overkill today, but the lock primitive is cheap and prevents future foot-guns.

---

## Implementation Patterns to Reuse

### Pattern 1: Phase 12 Controller + Sink + Narrow Interfaces (template)

```
// Source: internal/pricegaptrader/promotion.go + promote_event_sink.go
RampController         (state machine + eval logic)        ← analog of PromotionController
  ↓ depends on
RampStateStore         (load/save pg:ramp:state HASH)      ← new interface
RampEventSink          (emit RampEvent → Redis + WS)       ← analog of PromoteEventSink
RampNotifier           (Telegram on demote / force-op)     ← analog of PromoteNotifier
ReconcileStore         (load record for clean-day signal)  ← new interface
nowFunc                (injectable clock)                  ← same as Phase 12

Concrete implementations live in separate _sink.go file:
  RedisWSRampSink (in pricegaptrader/) — RPUSH pg:ramp:events + WS broadcast
  *database.Client adapter (in cmd/main.go) — implements RampStateStore + ReconcileStore
  TelegramNotifier adapter (in cmd/main.go) — implements RampNotifier
```

### Pattern 2: 3-retry skip-on-fail (perp-perp imitation)

```go
// Imitate internal/engine/exit.go:1119 (do NOT import — D-15 module boundary).
delays := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}
for attempt, d := range delays {
    if attempt > 0 { time.Sleep(d) }
    if err := op(); err == nil { return nil }
}
// triple-fail action per D-03
```

### Pattern 3: Daemon goroutine + ticker (scanLoop imitation)

```go
// Imitate internal/pricegaptrader/tracker.go:158-192 (scanLoop).
func (t *Tracker) reconcileLoop() {
    defer t.wg.Done()
    for {
        next := nextUTCFireTime(time.Now())
        select {
        case <-t.stopCh: return
        case <-time.After(time.Until(next)): runOnce()
        }
    }
}
```

### Pattern 4: Telegram allowlist + sanitize (Phase 9 imitation)

```go
// Source: internal/notify/telegram.go:340 (priceGapGateAllowlist).
// Add "ramp" to the map. Reuse checkCooldown + sanitizeForTelegram.
```

### Pattern 5: Idempotency via typed-struct JSON (no map keys)

Define every persisted shape as a typed struct. `json.Marshal` then emits fields in declaration order — byte-identical for byte-identical input. Pre-sort any slices (positions by `(close_ts, id)`, anomaly IDs ascending).

### Pattern 6: Best-effort Redis with logged failure (monitor.closePair imitation)

```go
// Source: monitor.go:248-249 — pattern is _ = save() + log on err.
if err := t.db.NewMethod(...); err != nil {
    t.log.Warn("pricegap: NewMethod failed: %v", err)
}
```

---

## Code Anchor Verification (line-by-line confirmation)

| CONTEXT anchor | Stated location | Actual location | Drift? |
|----------------|-----------------|-----------------|--------|
| `risk_gate.go preEntry` Gates 0–5 | Gates 0–5 + Gate 6 (delist) | Gate 0 + Gates 1–6 (line 58–133); current Gate 6 = delist/staleness | **Confirmed.** Note: comment says "6 deterministic gates" but there are actually 7 (0+6) — Phase 14 must update comment to "8" (0+7 after ramp insertion). |
| `monitor.go` `closePair` line 147–252 | line 248 = `pos.ClosedAt = time.Now()` | line **247** = `pos.ClosedAt = time.Now()`; line 248 = `if err := t.db.SavePriceGapPosition(pos)` | **Drift: 1 line off.** Insertion site is "after line 247", not "after line 248". |
| `monitor.go` lines 249–250 best-effort log pattern | Best-effort error log | line 248–250: `if err := t.db.SavePriceGapPosition(pos); err != nil { t.log.Warn(...) }` | **Confirmed pattern.** |
| `notify.go` lines 66–83 Notifier interface | `Notifier` at lines 66–83 | `PriceGapNotifier` (different name) at lines 89–98; `NoopNotifier` at lines 100–112 | **Drift: name + line numbers.** Interface is named `PriceGapNotifier`, not `Notifier`. Line range is 89–98 not 66–83. |
| `promotion.go` (Phase 12 controller) | Pattern reference | `PromotionController` at promotion.go:322-342; `PromoteEventSink` at line 261; `RedisWSPromoteSink` at promote_event_sink.go:53 | **Confirmed.** |
| `scanner.go` ticker pattern | Goroutine + ticker | Scanner has `RunCycle`, but ticker pattern is in **`tracker.go:158-192` `scanLoop`**, NOT scanner.go itself | **Drift: file location.** The actual ticker daemon is in `tracker.go scanLoop`, scanner is purely synchronous (`RunCycle(ctx, now)` is called by the loop). Phase 14 reconcileLoop imitates `tracker.go scanLoop`, not scanner.go. |
| `pricegap_state.go` namespace block lines 18–32 | Namespace block | Lines **14–28** (5 keys + 3 caps) | **Drift: 4 lines off.** Block starts at 14, ends at 28. |
| `pricegap_state.go` save path lines 41–65 | Close path Redis pipeline | `SavePriceGapPosition` at lines **40–60** (pipeline at 51–58) | **Drift: 1 line off.** |
| `config.go` PriceGap block lines 341–366 | Config struct + JSON DTO | Struct lines **1170–1198**; JSON DTO lines **1224–1248** | **Major drift: line numbers way off.** CONTEXT line numbers are stale; actual block is in the 1100s, not the 300s. Functionally still valid — block exists, fields present. Plan must reference current line numbers, not CONTEXT's. |
| `config.go validatePriceGapDiscovery` ~line 31 | Validator | Verified at line **31** | **Confirmed.** |
| `cmd/pg-admin/main.go` registrar pattern | Subcommand registration | `Run(args, deps) int` at line 1335; switch dispatch at line 1346 | **Confirmed.** |
| `internal/models/pricegap.go` PriceGapPosition | Position struct | Actual file is `internal/models/pricegap_position.go` — struct at lines 40–89 | **Drift: filename.** CONTEXT says `pricegap.go`; actual is `pricegap_position.go`. |
| API pricegap handler file | "find via glob" | `internal/api/pricegap_handlers.go` (existing) + `pricegap_discovery_handlers.go` (Phase 11) | **Confirmed via glob.** New file `pricegap_ramp_handlers.go` recommended for Phase 14 routes. |
| Frontend Pricegap-tracker tab | "locate component" | `web/src/pages/PriceGap.tsx` (~1500 lines) | **Confirmed via glob.** |
| Telegram allowlist (Phase 9) | Critical/non-critical pattern | `priceGapGateAllowlist` at telegram.go:**342** (gate-name allowlist, not bucket separation) | **Naming clarification:** CONTEXT calls it a "bucket" but the existing mechanism is a gate-name allowlist for `NotifyPriceGapRiskBlock`. The new digest method does NOT need the allowlist — it has fixed event-key construction. |

**Summary of drift:** All anchors functionally exist; only line numbers and filenames have drifted (research data is 1–4 line ranges off in most cases, plus filename `pricegap.go` → `pricegap_position.go`, plus `Notifier` → `PriceGapNotifier` name discrepancy, plus `scanner.go ticker` → actually `tracker.go scanLoop`). Plans must use current line numbers from RESEARCH.md, not CONTEXT.md's stale numbers.

---

## Recommendations for Plan Boundaries

Suggested split into **5 plans** (mirrors Phase 12's 4-plan structure with one extra plan for the ramp+sizer integration). Sequential dependencies — each plan builds on prior:

### Plan 14-01: Foundation — Config schema + Position model + close-path SADD hook

**Scope:**
- Add 7 fields to `Config` struct + `jsonPriceGap` DTO + apply/toJSON/fromEnv plumbing.
- New `validatePriceGapLive(c *Config) error` modeled on `validatePriceGapDiscovery`.
- Add `PriceGapPosition.Version int` + `PriceGapPosition.ExchangeClosedAt time.Time` fields.
- Extend `models.PriceGapStore` interface with `AddPriceGapClosedPositionForDate` + 6 new methods.
- Add namespace constants to `pricegap_state.go` + implement on `*Client`.
- Insert SADD hook in `monitor.closePair` after line 247.
- Unit tests: validator round-trip, JSON DTO defaults preserved, close-path SADD writes to correct date key.

**Files touched:** `internal/config/config.go`, `internal/models/pricegap_position.go`, `internal/models/interfaces.go` (PriceGapStore), `internal/database/pricegap_state.go`, `internal/pricegaptrader/monitor.go` (1 line + 4-line block).

**Why first:** Every subsequent plan depends on the config fields, position model fields, and Redis namespace.

### Plan 14-02: Reconciler core + idempotency

**Scope:**
- New `internal/pricegaptrader/reconciler.go` declaring `Reconciler` struct + `ReconcileStore` interface + `DailyReconcileRecord` typed-struct schema.
- `Reconciler.RunForDate(ctx, date)` with 3-retry imitation of `internal/engine/exit.go:1119`.
- Aggregation logic: SMEMBERS `pg:positions:closed:{date}` → HGET `pg:positions` per ID → sort by `(exchange_closed_at, id)` → compute totals + flag anomalies → marshal typed struct → SET `pg:reconcile:daily:{date}`.
- Missing exchange close-ts fallback per D-10.
- Anomaly detection: `abs(realized_slippage_bps) > cfg.PriceGapAnomalySlippageBps`; missing close ts.
- Triple-fail path: log + critical Telegram (`NotifyPriceGapReconcileFailure`) + skip day.
- Unit tests: TDD-style — idempotency byte equality test (P0), missing-ts fallback, slippage anomaly, position-version aggregation, 3-retry success on 2nd attempt, 3-retry triple-fail emits critical.

**Files touched:** new `reconciler.go` + `reconciler_test.go`; new `errors.go` adds `ErrPriceGapReconcileFailed`; extend `Notifier` interface with `NotifyPriceGapReconcileFailure` + `NoopNotifier` stub.

### Plan 14-03: RampController core + risk_gate integration + sizer

**Scope:**
- New `internal/pricegaptrader/ramp.go` declaring `RampController` struct + `RampStateStore` interface + `RampState` typed-struct (5 fields) + `RampEvent` typed-struct.
- `RampController.Eval(ctx, date, dailyRecord)` — idempotent (guard via `last_eval_ts`), asymmetric ratchet logic.
- `RampController.ForcePromote(operator, reason)` / `ForceDemote(operator, reason)` / `Reset(operator, reason)` per D-15.
- New errors: `ErrPriceGapRampExceeded`, `ErrPriceGapRampStateUnavailable`.
- New `RampEventSink` interface; new `RedisWSRampSink` concrete impl (mirrors `RedisWSPromoteSink`) — RPUSH + LTRIM 500 to `pg:ramp:events` + WS broadcast.
- Insert Gate 6 ramp gate in `risk_gate.go preEntry` (renumbers delist to Gate 7); update gate ordering comment + invariant test.
- Add `"ramp"` to `priceGapGateAllowlist` in `telegram.go:342`.
- Sizer integration at `tracker.go:560` — wrap `cand.MaxPositionUSDT` with `min(min(cand.MaxPositionUSDT, stageSize), hardCeiling)` when `PriceGapLiveCapital=true`.
- Unit tests: TDD — asymmetric ratchet invariant (any loss day → counter=0 + demote), kill-9 persistence (state survives via Redis), idempotent eval (same-day double-fire no double-advance), hard-ceiling enforcement (config typo of `stage_3_size_usdt: 9999` still sizes at 1000), boot guard refuses start when `live_capital=true` AND `pg:ramp:state` missing.

**Files touched:** new `ramp.go` + `ramp_test.go` + `ramp_event_sink.go`; modify `risk_gate.go` (Gate 6 insertion); modify `telegram.go` (allowlist); modify `tracker.go` (sizer wrap).

### Plan 14-04: Daemon wiring + pg-admin subcommands + Telegram digest

**Scope:**
- New `Tracker.reconcileLoop` daemon goroutine (mirrors `scanLoop` pattern). Boot-time catchup logic per Q14.
- Extend `Tracker` struct with `reconciler *Reconciler` + `ramp *RampController`. Spawn daemon in `Tracker.Start` after existing scanLoop.
- Boot guard in `Tracker.Start`: if `PriceGapLiveCapital=true` AND `pg:ramp:state` missing OR fewer than 7 reconcile keys → panic + critical Telegram.
- 6 new pg-admin subcommands: `reconcile run/show`, `ramp show/reset/force-promote/force-demote`. Wire into existing `Run` switch dispatch at line 1346.
- New `Notifier.NotifyPriceGapDailyDigest` + `NoopNotifier` stub. Implementation in `internal/notify/telegram.go` constructs digest text per D-11 (total PnL, positions count, win/loss split, ramp stage + counter, anomaly IDs).
- `cmd/main.go` bootstrap: construct `Reconciler`, `RampController`, `RedisWSRampSink`, attach to `Tracker`. Adapter layer for `RampStateStore` (wraps `*database.Client`) and `RampNotifier` (wraps `*notify.TelegramNotifier`).
- Integration tests: end-to-end reconcile→ramp eval→digest emission with fake Redis + fake notifier.

**Files touched:** `internal/pricegaptrader/tracker.go` (daemon goroutine + boot guard); `internal/pricegaptrader/notify.go` (interface + Noop stubs); `internal/notify/telegram.go` (NotifyPriceGapDailyDigest + NotifyPriceGapReconcileFailure impls); `cmd/pg-admin/main.go` (6 subcommands); `cmd/main.go` (bootstrap wiring).

### Plan 14-05: API endpoints + dashboard widget + i18n lockstep

**Scope:**
- New `internal/api/pricegap_ramp_handlers.go` — `handlePgRampState` + `handlePgReconcileDay`. Register routes in `server.go:184` block. Bearer auth via existing `authMiddleware`. Response envelope `{ok, data}` matches existing `Response` struct.
- New WS event `pg_ramp_event` broadcast from `RedisWSRampSink.Emit`. Mirrors Phase 12 `pg_promote_event` shape.
- New React component `web/src/components/Ramp/RampReconcileSection.tsx`. Mounts in `PriceGap.tsx` after `<DiscoverySection />` at line 772. Read-only (no mutation UI per D-14). Live-capital ON/OFF badge, ramp stage display, clean-day counter progress bar, last loss day, demote count (with tooltip), last reconcile summary (PnL, position count, anomaly count), anomaly list collapsed-by-default.
- i18n keys added to BOTH `en.ts` AND `zh-TW.ts` lockstep — `pricegap.ramp.*` and `pricegap.reconcile.*` namespaces.
- Visual regression: human verification step (browser confirm, screenshot).
- Smoke test: API endpoints return 200 with realistic shapes; WS event delivers within 1s of force-promote.

**Files touched:** new `internal/api/pricegap_ramp_handlers.go`; modify `internal/api/server.go` (route registration); new `web/src/components/Ramp/RampReconcileSection.tsx`; modify `web/src/pages/PriceGap.tsx` (insert section); modify both `web/src/i18n/{en,zh-TW}.ts`.

---

## Validation Architecture (Nyquist Dimension 8)

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (Go 1.26+); Vitest for frontend (existing `web/` setup) |
| Config file | `go.mod` + per-package `*_test.go`; frontend `web/vitest.config.ts` (existing) |
| Quick run command | `go test ./internal/pricegaptrader/... -run TestRamp -count=1 -timeout=30s` |
| Full suite command | `go test ./... -count=1 -timeout=5m` then `cd web && npm test` |
| Phase gate | Full suite green before `/gsd-verify-work`; human browser confirm for Plan 14-05 |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PG-LIVE-03 | Reconcile idempotency byte-equality | unit | `go test ./internal/pricegaptrader/ -run TestReconcileIdempotency -v` | ❌ Wave 0 |
| PG-LIVE-03 | 3-retry success on 2nd attempt | unit | `go test ./internal/pricegaptrader/ -run TestReconcile_RetrySuccessOnSecondAttempt -v` | ❌ Wave 0 |
| PG-LIVE-03 | 3-retry triple-fail emits critical Telegram + skips day | unit | `go test ./internal/pricegaptrader/ -run TestReconcile_TripleFailEmitsCritical -v` | ❌ Wave 0 |
| PG-LIVE-03 | Missing exchange-close-ts → fallback to ClosedAt + flag | unit | `go test ./internal/pricegaptrader/ -run TestReconcile_MissingTsFallback -v` | ❌ Wave 0 |
| PG-LIVE-03 | Slippage anomaly flag at threshold + above + below | unit | `go test ./internal/pricegaptrader/ -run TestReconcile_SlippageAnomalyBoundaries -v` | ❌ Wave 0 |
| PG-LIVE-03 | (position_id, version) aggregation key — duplicate id+version dedup | unit | `go test ./internal/pricegaptrader/ -run TestReconcile_VersionDedup -v` | ❌ Wave 0 |
| PG-LIVE-01 | Asymmetric ratchet invariant — loss day → counter=0 + demote | unit | `go test ./internal/pricegaptrader/ -run TestRamp_AsymmetricRatchetInvariant -v` | ❌ Wave 0 |
| PG-LIVE-01 | Promotion at exactly 7 clean days (boundary `>=7` not `>7`) | unit | `go test ./internal/pricegaptrader/ -run TestRamp_PromoteAtSeven -v` | ❌ Wave 0 |
| PG-LIVE-01 | No-activity day HOLDS counter (D-05) | unit | `go test ./internal/pricegaptrader/ -run TestRamp_NoActivityHolds -v` | ❌ Wave 0 |
| PG-LIVE-01 | Idempotent eval — same-day double-fire no double-advance | unit | `go test ./internal/pricegaptrader/ -run TestRamp_IdempotentEval -v` | ❌ Wave 0 |
| PG-LIVE-01 | kill -9 mid-stage → restart resumes correct stage + counter | integration | `go test ./internal/pricegaptrader/ -run TestRamp_KillNineRecovery -v` | ❌ Wave 0 |
| PG-LIVE-01 | Hard-ceiling enforcement: `stage_3_size_usdt: 9999` sizes at 1000 | unit | `go test ./internal/pricegaptrader/ -run TestRamp_HardCeilingClamp -v` | ❌ Wave 0 |
| PG-LIVE-01 | Gate 6 ramp returns ErrPriceGapRampExceeded for over-budget | unit | `go test ./internal/pricegaptrader/ -run TestRiskGate_RampExceeded -v` | ❌ Wave 0 |
| PG-LIVE-01 | Gate 6 ramp no-op when PriceGapLiveCapital=false | unit | `go test ./internal/pricegaptrader/ -run TestRiskGate_RampNoOpInPaper -v` | ❌ Wave 0 |
| PG-LIVE-01 | Boot guard refuses start when live_capital=true + ramp state missing | unit | `go test ./internal/pricegaptrader/ -run TestTracker_BootGuardRefusesStart -v` | ❌ Wave 0 |
| PG-LIVE-01 | force-promote does NOT zero counter (D-15 #3) | unit | `go test ./internal/pricegaptrader/ -run TestRamp_ForcePromoteHoldsCounter -v` | ❌ Wave 0 |
| PG-LIVE-01 | force-demote DOES zero counter (D-15 #4) | unit | `go test ./internal/pricegaptrader/ -run TestRamp_ForceDemoteZerosCounter -v` | ❌ Wave 0 |
| PG-LIVE-01 | reset zeroes counter + sets stage = 1 | unit | `go test ./internal/pricegaptrader/ -run TestRamp_ResetSemantics -v` | ❌ Wave 0 |
| D-04 | Re-run reconcile twice → byte-identical Redis value | unit | `go test ./internal/pricegaptrader/ -run TestReconcile_Idempotency -v` | ❌ Wave 0 (same as PG-LIVE-03 #1) |
| D-11 | Telegram digest contains PnL, count, win/loss, ramp stage, anomaly IDs | unit | `go test ./internal/pricegaptrader/ -run TestNotifyDailyDigest_RequiredFields -v` | ❌ Wave 0 |
| D-13 | pg-admin reconcile show prints stored daily record JSON | unit | `go test ./cmd/pg-admin/ -run TestCmdReconcileShow -v` | ❌ Wave 0 |
| D-13 | pg-admin ramp force-promote appends pg:ramp:events | unit | `go test ./cmd/pg-admin/ -run TestCmdRampForcePromote -v` | ❌ Wave 0 |
| D-14 | API GET /api/pg/ramp returns RampState envelope | unit | `go test ./internal/api/ -run TestHandlePgRampState -v` | ❌ Wave 0 |
| D-14 | API GET /api/pg/reconcile/{date} returns daily record | unit | `go test ./internal/api/ -run TestHandlePgReconcileDay -v` | ❌ Wave 0 |
| D-14 | Frontend RampReconcileSection renders read-only — no mutation buttons | unit | `cd web && npx vitest run components/Ramp/RampReconcileSection.test.tsx` | ❌ Wave 0 |
| project rule | i18n EN + zh-TW lockstep — no missing keys | unit | existing `web/src/i18n/lockstep_test.ts` (or add if absent) | partial — verify |

### Sampling Rate
- **Per task commit:** `go test ./internal/pricegaptrader/ -run "TestRamp|TestReconcile|TestRiskGate" -count=1 -timeout=30s` (~10s)
- **Per wave merge:** `go test ./internal/pricegaptrader/... ./internal/api/... ./cmd/pg-admin/... -count=1` plus `cd web && npm test` (~3 min)
- **Phase gate:** Full suite green + human browser confirm of `RampReconcileSection` in PriceGap.tsx tab + `pg-admin ramp show` after a synthetic reconcile run.

### Wave 0 Gaps
- [ ] `internal/pricegaptrader/reconciler_test.go` — covers PG-LIVE-03
- [ ] `internal/pricegaptrader/ramp_test.go` — covers PG-LIVE-01
- [ ] `internal/pricegaptrader/risk_gate_test.go` — extend with TestRiskGate_RampExceeded + TestRiskGate_RampNoOpInPaper
- [ ] `internal/pricegaptrader/tracker_test.go` — extend with TestTracker_BootGuardRefusesStart + TestTracker_ReconcileLoopCatchup
- [ ] `cmd/pg-admin/main_test.go` — extend with TestCmdReconcile* + TestCmdRamp*
- [ ] `internal/api/pricegap_ramp_handlers_test.go` — new file, covers GET endpoints
- [ ] `web/src/components/Ramp/RampReconcileSection.test.tsx` — new file (Vitest)
- [ ] No framework install needed — Go stdlib + existing Vitest both present.

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Existing Bearer-token middleware (`s.authMiddleware`) on new GET endpoints |
| V3 Session Management | yes | Existing localStorage `arb_token` flow — Phase 14 reuses, adds nothing |
| V4 Access Control | yes | pg-admin force-promote/force-demote/reset are local-process-only (no remote auth) — operator must have shell access; matches existing `pg-admin candidates add` precedent |
| V5 Input Validation | yes | New config fields range-validated in `validatePriceGapLive`; date path-param `/api/pg/reconcile/{date}` must regex-match `^\d{4}-\d{2}-\d{2}$` to prevent path traversal in Redis key construction |
| V6 Cryptography | no | No new crypto |

### Known Threat Patterns for Go + Redis + React

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via `{date}` URL parameter | Tampering | Regex validate `^\d{4}-\d{2}-\d{2}$` before constructing Redis key — same pattern as existing pricegap handlers |
| Telegram message injection | Tampering | Reuse existing `sanitizeForTelegram(s, max)` at telegram.go:975 for any operator-supplied reason string in force-op events |
| Cooldown bypass via crafted gate name | Tampering (T-09-17 in code) | Already mitigated — `priceGapGateAllowlist` rejects unknown gates; Phase 14 adds `"ramp"` to the map (one-line addition) |
| Race between dashboard config-write and ramp eval | Tampering | `pg:ramp:state` writes wrap with optional Redis lock (existing `priceGapLockPrefix` precedent at pricegap_state.go:24); single-binary deployment makes contention rare but lock is cheap insurance |
| Dashboard mutation of ramp state through unauthorized path | Elevation | Phase 14 dashboard widget is **read-only** (D-14); no POST endpoints added; ALL mutation requires shell access to pg-admin |

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.26+ | All Go code | ✓ | 1.26 (per go.mod) | — |
| Redis | `pg:ramp:state`, `pg:reconcile:daily:*`, `pg:positions:closed:*`, `pg:ramp:events` | ✓ | DB 2 (existing) | — |
| Telegram bot | Daily digest + critical reconcile failure | ✓ (existing) | — | NoopNotifier — digest silently skipped, but reconcile + ramp continue |
| systemd | Auto-restart on crash | ✓ (existing) | — | — |
| Node 22.13.0 + Vite | Frontend rebuild | ✓ (existing per nvm) | — | — |
| WebSocket hub | `pg_ramp_event` live push | ✓ (existing `s.hub`) | — | REST seed of `/api/pg/ramp` is sufficient (degrade gracefully) |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** None — Phase 14 is purely additive on existing infrastructure.

---

## Project Constraints (from CLAUDE.md / CLAUDE.local.md)

These directives MUST NOT be violated by any Phase 14 plan or code:

1. **Never run `npm install` / `npm update` / `npx` / `pnpm install`** — npm axios compromise. Phase 14 frontend uses ONLY existing dependencies; if `npm ci` is needed, OK.
2. **Never modify `config.json`** — live runtime config with API keys. Phase 14 adds config FIELDS to `internal/config/config.go`; user updates `config.json` themselves on deploy.
3. **Build order: frontend → Go binary** — Phase 14 has frontend changes (RampReconcileSection); MUST run `npm run build` in `web/` BEFORE `go build`.
4. **Update VERSION + CHANGELOG.md every commit** — project rule.
5. **Module isolation:** `internal/pricegaptrader/` MUST NOT import `internal/engine/` or `internal/spotengine/` — perp-perp 3-retry pattern is **imitated** literally, not imported.
6. **Sonnet4.6 / Opus4.6 only when delegating** — applies to teammate spawning, not to implementation choice.
7. **i18n EN + zh-TW lockstep** — Phase 14 adds keys to BOTH locales in the same commit.
8. **Loris rates ÷8** — N/A for Phase 14 (no funding rate consumption).
9. **Live-trading safety** — Phase 14 IS the live-capital phase; conservative-by-default; every flag default OFF.
10. **graphify routing first, no repo-wide grep** — Phase 14 plans should reference graphify-publish/AI_ROUTER.md, not start with `grep -r`.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `(*TelegramNotifier).checkCooldown` is exported (lowercase) so a new method `NotifyPriceGapDailyDigest` in the same file can call it directly. | Q8 | LOW — if not exported, the new method just inlines the cooldown logic; minor refactor. |
| A2 | The `WSBroadcaster` narrow interface declared in `promote_event_sink.go:88` can be reused by `RedisWSRampSink` (same shape). | Q5 | LOW — the interface is package-private to `pricegaptrader`; just duplicate the declaration in `ramp_event_sink.go` if needed. |
| A3 | Adding `Version int` and `ExchangeClosedAt time.Time` to `PriceGapPosition` is JSON-backward-compatible (zero-value on legacy records). | Q2 | LOW — Go stdlib `encoding/json` zero-fills missing fields; established `Mode` field precedent (line 61) confirms. Risk: if any test fixture uses `omitempty` semantics that conflict with zero-time, fix at planning time. |
| A4 | The reconcile→ramp coupling can run in a single goroutine (reconcile completes, then `ramp.Eval` is called synchronously) without race against pg-admin force-op. | Q5, Q18 | LOW — pg-admin is a separate process with its own Redis lock; Tracker daemon is single-goroutine; no shared mutable state between them except `pg:ramp:state` which is overwritten atomically via Pipeline HSET. |
| A5 | Boot-time catchup (Q14 hybrid recommendation) won't double-fire on a 00:25→00:35 restart because byte-equality (D-04) means double-write produces identical output. | Q14 | LOW — by design. Worst case is two RPUSH events to `pg:ramp:events` in quick succession; cap=500 absorbs it. |
| A6 | `internal/notify/telegram.go` has a critical-bypass mechanism (used by L4/L5 alerts) that `NotifyPriceGapReconcileFailure` can route through. | Q8 | MEDIUM — I did NOT verify this. If no critical bypass exists, the triple-fail alert may be cooldown-throttled (5min). Plan must verify at planning time and either reuse or add a `SendCritical` path. **See Open Q3.** |
| A7 | `tracker.go:560` is the canonical sizing call site for live entries, and wrapping `cand.MaxPositionUSDT` there is sufficient. | Q3 | LOW — verified by grep showing `notional := cand.MaxPositionUSDT` is the per-candidate sizing assignment. If a second sizing path emerges later, the gate at risk_gate.go is the second line of defense. |
| A8 | Phase 12's `TestRiskGate_OrderingInvariant` test (mentioned in CONTEXT line 17) actually exists. | Q3 | LOW — if absent, Phase 14 plan should add it (one-line ordering invariant matters for blame attribution). |

---

## Open Questions

### Open Q1 — `PriceGapPosition.Version` semantics

**What we know:** Field is missing today (verified). PG-LIVE-03 success-criterion #1 says "aggregates closed Strategy 4 positions keyed by `(position_id, version)`".
**What's unclear:** When does `version` increment? At every save? Only on amendment (e.g., late fill correction)? CONTEXT.md line 81 says researcher should verify the field exists and document its meaning — field doesn't exist yet, so semantics are entirely a Phase 14 design choice.
**Recommendation:** `version` defaults to 1 on entry (in `monitor.openPair`); incremented on any post-close amendment of the position record (rare — currently only late-fill PnL reconciliation paths amend). Reconcile uses `(id, version)` as the dedup key so re-running for a date that had a position amended produces a different aggregation but still byte-identically per the new version. Alternative: skip `version` entirely and use `(id)` alone — simpler, matches the actual usage pattern (positions don't get amended in current code). **Phase 14 planner must pick before implementation.** I lean toward "add `Version int` defaulting 1 on entry, never incremented in current code, future-proofing for amendments" — adds one line to `monitor.openPair`, no behavior change today.

### Open Q2 — Pre-flight live-capital activation rule

**What we know:** CONTEXT discretion item: "refuse to flip live_capital=true if no `pg:reconcile:daily:*` keys exist for past 7 days".
**What's unclear:** Where to enforce? Config-load (likely no Redis access) vs runtime (Tracker.Start boot guard).
**Recommendation:** Tracker.Start boot guard. Config validation only checks paper-vs-live consistency + range checks. Boot guard checks Redis state and refuses to start with a critical Telegram if `live_capital=true` but reconcile history insufficient. Operator runs `pg-admin reconcile run --date=YYYY-MM-DD` for each missing day before flipping the flag.

### Open Q3 — Telegram critical-bypass for reconcile triple-fail

**What we know:** D-03 says "log + emit Telegram critical alert + skip the day". Existing `NotifyPriceGapRiskBlock` at telegram.go:1054 has 5-min cooldown.
**What's unclear:** Is there a "critical bypass" that skips cooldown? L4/L5 alerts (perp-perp engine) may use a different path I didn't read.
**Recommendation:** Plan 14-02 includes a 1-task "verify critical path" that reads telegram.go:1-300 and either (a) reuses an existing `SendCritical` method or (b) adds one (single new method, no cooldown). Until verified, the plan should reserve scope for the latter.

### Open Q4 — `TestRiskGate_OrderingInvariant` existence

**What we know:** Code comment at risk_gate.go:29 says "TestRiskGate_OrderingInvariant locks D-17."
**What's unclear:** Did not verify the test actually exists in `risk_gate_test.go`.
**Recommendation:** Plan 14-03 first task — confirm the test exists; if so, extend its expected gate-order list to include "ramp" between "concentration" and "delist". If absent, add it.

### Open Q5 — Concurrent-evaluator guard

**What we know:** CONTEXT discretion item, "single-process is fine; lock if multi-process becomes a concern".
**What's unclear:** Today's deployment is single-binary single-server (per CLAUDE.local.md). But pg-admin is a SEPARATE process — when an operator runs `pg-admin ramp force-demote` while the daemon is mid-eval, both write to `pg:ramp:state`.
**Recommendation:** Add a thin Redis lock (existing `priceGapLockPrefix` SET NX TTL pattern at pricegap_state.go:24) wrapping any `pg:ramp:state` write. Cheap, defensive, future-proofs multi-instance deploy.

### Open Q6 — `pg:ramp:events` LIST cap

**What we know:** CONTEXT discretion item, suggests 500 to match `pg:history`.
**Unclear:** None.
**Recommendation:** Cap at 500. Matches `pg:history` precedent at pricegap_state.go:21. Force-ops are rare; daily eval emits one event per day; 500 = ~1.5 years of daily evals + force-ops.

---

## Pitfalls / Regression Risks

### Pitfall A — In-memory streak counter (Phase 12 mistake to NOT repeat)

Phase 12 used in-memory streak storage (D-03) — acceptable because cold restart cost only 6 cycles. **Phase 14 must NOT do this for ramp state.** PITFALLS.md Pitfall 3 explicitly mandates Redis persistence. Warning sign: any in-memory `int` field on `RampController` for the clean-day counter without a corresponding `redis.HSet` companion. The 5 explicit Redis fields in PG-LIVE-01 are the contract — keep them in Redis, mirror them on the controller as a read cache only.

### Pitfall B — Off-by-one on promotion threshold

Promotion fires at `clean_day_counter >= 7`, NOT `> 7`. Demotion fires on `today_was_loss_day` regardless of counter value. Mismatched comparison operators are a classic ramp bug. Lock with explicit boundary tests at counter=6, 7, 8.

### Pitfall C — Wall-clock day vs trading day

PITFALLS.md Pitfall 3 warning sign: "Day comparison using `time.Now().Sub(start) >= 7*24*time.Hour`". Phase 14 must NEVER use wall-clock duration; always count clean-day counter increments. `last_loss_day_ts` is informational only (for dashboard display); not used in ramp logic.

### Pitfall D — Sizing path that bypasses the gate

PITFALLS.md Pitfall 3 warning sign: "Sizing path that reads `stage_budget_per_leg` directly without `min(_, hard_ceiling)`". Phase 14 enforces `min(stage_size, hard_ceiling)` at TWO sites (defense in depth):
1. Sizer at `tracker.go:560` — prevents over-formed requests.
2. Gate 6 in `risk_gate.go preEntry` — rejects over-sized requests at entry.
Both must use the SAME helper function `clampStageSize(stage int, cfg *config.Config) float64` to avoid drift.

### Pitfall E — Reconcile triple-fail silences ramp

D-03: triple-fail = skip day = no clean-day credit, no demote. **The dashboard widget MUST surface "last reconcile date" prominently** so an operator can see if reconcile has been silently skipping for days (e.g., Redis intermittency). Without visibility, the ramp could stagnate without the operator realizing why.

### Pitfall F — Phase 9 paper-mode realized_slippage_bps machine-zero (PG-FIX-01)

CONTEXT D-09 NOTE: "depends on PG-FIX-01 (Phase 16) fixing the realized-slippage machine-zero bug for paper-mode emission". **Phase 14 anomaly detection works correctly in LIVE mode but fires zero anomalies in paper mode** until PG-FIX-01 ships. This is intentional per the dependency note — but the plan must document this clearly so a tester running in paper mode does NOT misread "zero anomaly count" as "anomaly detection broken". Add a unit test that asserts the live-mode path flags anomalies AND a paper-mode path does NOT (until PG-FIX-01) — locks the known-state.

### Pitfall G — Pitfall 7 regression-spawn discipline

Per CONTEXT canonical-refs: "any latent bugs surfaced during reconcile dev get spawned as separate Phase 999.x hot-fix, not silently merged". If Phase 14 implementation work surfaces, e.g., a v0.36.x dashboard race or a config-load bug, **open Phase 999.2 hot-fix mini-phase** with its own version bump. Do NOT silently fix inside any Phase 14 plan — it muddles the verification trail.

### Pitfall H — Renumber-Gate-comment drift

Phase 14 inserts ramp as Gate 6, renumbering delist+staleness to Gate 7. The comment at risk_gate.go:17 currently says "6 deterministic gates". Must update to "8" (Gate 0 + Gates 1–7). The TestRiskGate_OrderingInvariant test (Open Q4) must also be updated. Easy to forget; will fail static-comment validation if any exists.

---

## Sources

### Primary (HIGH confidence — VERIFIED via file read)
- `/var/solana/data/arb/internal/pricegaptrader/risk_gate.go:1-150` — Gate 0–6 ordering, `GateDecision` shape, telemetry conventions
- `/var/solana/data/arb/internal/pricegaptrader/promotion.go:1-360` — `PromotionController` struct, narrow-interface DI, idempotent eval pattern
- `/var/solana/data/arb/internal/pricegaptrader/promote_event_sink.go:1-110` — `RedisWSPromoteSink`, `WSBroadcaster` narrow interface, RPUSH+LTRIM 1000 pattern
- `/var/solana/data/arb/internal/pricegaptrader/tracker.go:140-220` — `Start` order, `scanLoop` daemon goroutine pattern (the actual ticker, NOT in scanner.go)
- `/var/solana/data/arb/internal/pricegaptrader/notify.go:60-115` — `PriceGapNotifier` interface (NOT `Notifier`), `NoopNotifier` shape
- `/var/solana/data/arb/internal/pricegaptrader/monitor.go:240-260` — close path with `pos.ClosedAt = time.Now()` at line **247** (CONTEXT said 248 — drift 1)
- `/var/solana/data/arb/internal/models/pricegap_position.go:40-89` — full struct, no `Version` field today
- `/var/solana/data/arb/internal/database/pricegap_state.go:1-100` — namespace block at lines 14–28, save path at lines 40–60
- `/var/solana/data/arb/internal/config/config.go:31-60, 1170-1248` — `validatePriceGapDiscovery` at line 31 + PriceGap config block at 1170–1248
- `/var/solana/data/arb/cmd/pg-admin/main.go:1-1686` — `Run(args, deps) int` switch dispatch + 6 subcommand precedent
- `/var/solana/data/arb/internal/notify/telegram.go:339-470, 1037-1066` — `priceGapGateAllowlist` at line 342, sanitize + cooldown patterns
- `/var/solana/data/arb/internal/api/server.go:121-185` — route registration patterns; `/api/pg/*` (Phase 11) and `/api/pricegap/*` (Phase 8/9) coexist
- `/var/solana/data/arb/internal/api/pricegap_discovery_handlers.go:1-150` — Phase 11 handler shape (template)
- `/var/solana/data/arb/web/src/pages/PriceGap.tsx:1-1500` — existing tab structure, `<DiscoverySection />` insertion point at line 768–772
- `/var/solana/data/arb/web/src/i18n/en.ts:771-820` — existing `pricegap.*` namespace
- `/var/solana/data/arb/.planning/REQUIREMENTS.md` — PG-LIVE-01 / PG-LIVE-03 verbatim contracts
- `/var/solana/data/arb/.planning/ROADMAP.md` — Phase 14 §122–135 — 6 success criteria
- `/var/solana/data/arb/.planning/research/PITFALLS.md` — Pitfall 1, 2, 3, 7 verbatim
- `/var/solana/data/arb/.planning/phases/12-auto-promotion/12-CONTEXT.md` — Phase 12 D-03 (in-memory streak), D-15 (interface DI), D-16 (synchronous in-cycle)
- `/var/solana/data/arb/internal/engine/exit.go:1119, 3204` — perp-perp 3-retry literal `[]time.Duration{5*time.Second, 15*time.Second, 30*time.Second}` (the pattern PG-LIVE-03 references)

### Secondary (MEDIUM confidence — partial verification)
- `internal/engine/engine.go:1788, 1812, 4569` — alternate retry shapes (rejected as not-the-pattern)
- `internal/pricegaptrader/scanner.go:80-130` — Scanner struct includes `*PromotionController` (Phase 12 wiring) — confirms the controller-attached-to-scanner pattern
- `internal/pricegaptrader/tracker.go:60-150` — Tracker fields including `notifier`, `broadcaster`, `db`, `cfg` — confirms the existing DI shape Phase 14 extends

### Tertiary (LOW confidence — inferred from code patterns, NOT verified)
- A6 — Telegram critical-bypass for L4/L5 — not directly verified in this research session. Plan 14-02 must verify before implementing reconcile triple-fail alert.
- Open Q4 — `TestRiskGate_OrderingInvariant` test existence — referenced in code comment but not file-grepped this session.

---

## Metadata

**Confidence breakdown:**
- Code anchors: HIGH — every CONTEXT anchor verified by file read; minor line-number drift documented.
- Implementation patterns (Phase 12 template, perp-perp 3-retry, scanner ticker): HIGH — verified by direct file read.
- Architectural design (Reconciler/RampController shapes): HIGH — direct mirror of Phase 12 PromotionController + RedisWSPromoteSink, validated against PITFALLS.md Pitfall 3 prescription.
- Validation architecture: HIGH — every PG-LIVE-01 / PG-LIVE-03 success criterion has a named test.
- Telegram critical-bypass mechanism: MEDIUM — not directly verified; flagged Open Q3.

**Research date:** 2026-04-30
**Valid until:** 2026-05-30 (30 days; pricegaptrader package is fast-evolving — re-verify line numbers if delayed)

---

## RESEARCH COMPLETE
