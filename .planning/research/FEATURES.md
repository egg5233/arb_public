# Feature Research — v2.2 Auto-Discovery & Live Strategy 4

**Domain:** Crypto cross-exchange price-gap arbitrage (Strategy 4) — promotion from paper to live
**Researched:** 2026-04-27
**Confidence:** HIGH (grounded in existing internal/pricegaptrader code + 3+ days paper observation + Codex design review 2026-04-21)

This research scopes the FIVE new feature areas in v2.2 against existing infrastructure. Complexity sizing assumes existing modules (internal/pricegaptrader, internal/notify, pg-admin, dashboard CRUD, paper-mode chokepoint) are reused — not rebuilt.

---

## 1. Auto-Discovery Scanner (PG-DISC-01)

### Table Stakes

| Feature | Why Expected | Complexity | Dependencies / Notes |
|---------|--------------|------------|----------------------|
| Periodic scan loop with config-driven cadence (default ~5 min) | Existing tracker is event-driven on candidates only; scanner needs its own goroutine separate from PG tracker tick | **M** | New goroutine in `internal/pricegaptrader/discovery.go`; reuse Scheduler pattern from engine. Default OFF via `EnablePriceGapDiscovery` switch. |
| 24h rolling volume filter per (symbol, exchange) | Standard liquidity filter — without it, scanner suggests untradeable shitcoins | **S** | Reuses existing exchange adapter `GetTicker24h` / volume fields; no new adapter surface. |
| Spread persistence sampling (median/mean abs spread over rolling window) | The whole edge is spread persistence; without measurement, scoring is a guess | **M** | Already have 1m kline ingestion in PG detector — extend to record per-candidate spread series in `pg:disco:samples:<symbol>:<lExch>:<sExch>` (Redis). |
| Min depth probe at top-of-book | Phase 1 of Strategy 4 scoping killed most T=100 candidates on real slippage; depth must be measured | **M** | Reuse `EstimateSlippage` in pkg/utils + adapter `GetOrderBook`. |
| Hard denylist exclusion | Codex review 2026-04-21 flagged this as first-class control; required to suppress known-bad pairs (delisted, deliveryDate close, manipulation history) | **S** | Already partially implemented in tracker via delist veto — promote to config field `PriceGapDiscoveryDenylist []string`. |
| Output: structured `PriceGapDiscoveryCandidate` with score + reasoning fields | Operators need to understand *why* something scored high before promotion (Phase 8 risk gates lessons) | **S** | New struct in `pg/types.go`; persist to Redis `pg:disco:results:<cycle_ts>`. |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Score decomposition (volume / spread / depth / persistence sub-scores) | Operator can see whether a candidate is high-volume-low-edge or vice versa — informs human override | **S** | Don't collapse sub-scores into one number too early. |
| Scanner cycle telemetry (pairs scanned, pairs filtered, time-per-cycle, top-N runners-up) | Trust-building — paper-mode era showed operators want to see "the scanner thought about 800 pairs and 12 passed gates" | **M** | Persist cycle stats in `pg:disco:cycles:<ts>`; consumed by PG-DISC-03 dashboard. |
| Reasoning string per candidate ("rejected: 24h vol $42k below $250k floor") | Debugging discovery decisions without log archaeology | **S** | Single string field on candidate output. |

### Anti-Features

| Feature | Why Tempting | Why Problematic | Alternative |
|---------|--------------|-----------------|-------------|
| Scan ALL N×N exchange pairs every cycle | "completeness" | 6 exchanges × ~500 symbols × N² combinations = thousands of API calls; rate-limit hostile | Pre-filter to symbols traded on ≥2 exchanges, then iterate pairs only for those. |
| ML-based scoring (regression on historical PnL) | "smart" scoring | Strategy 4 has ~5-pair edge universe — too few samples to fit; overfits noise | Linear weighted sum of normalized sub-scores; weights configurable. |
| Per-tick spread monitoring during scan | "highest fidelity" | Defeats the purpose — Strategy 4 fires on 4-bar persistence, not single ticks; scan is supposed to find candidates, not pre-trade them | Stick to 1m bar ingestion frequency; let tracker handle real-time once promoted. |
| Auto-add to candidates list at scan time | Conflates discovery with promotion | Operators lose review window; promotion must be a separate gated step | See PG-DISC-02 below. |

---

## 2. Auto-Promotion Controller (PG-DISC-02)

### Table Stakes

| Feature | Why Expected | Complexity | Dependencies / Notes |
|---------|--------------|------------|----------------------|
| Score threshold gate (`PriceGapAutoPromoteScore`) | Single configurable cutoff is the floor for any auto-promotion system | **S** | New config field; default OFF (threshold=0 means disabled). |
| Hard cap on candidate list size (`PriceGapMaxCandidates`, default 12) | Without this, scanner can grow list unboundedly → blow concentration limits | **S** | Validation on append; reject if `len(cfg.PriceGapCandidates) >= max`. |
| Atomic persistence to config.json via SaveJSON (with .bak) | config.json is sole source of truth (2026-04-05 decision); broken write = blown trading | **S** | Reuses existing SaveJSON path from Phase 10 dashboard CRUD. |
| Observation period before promotion (candidate must score ≥ threshold for N consecutive cycles) | Single-cycle promotion = noise-driven thrash; Strategy 4 retro lesson: 1-bar crossings inflate edge ~10× | **M** | Track per-candidate "passing streak" in `pg:disco:streak:<key>`; require ≥ `PriceGapPromoteStreakCycles` (default 6 = 30min @ 5min cadence). |
| WS broadcast on promote/demote | Operator awareness; consistent with Phase 10 CRUD WS pattern | **S** | Reuse existing dashboard WS event `pg_candidate_changed`. |
| Telegram notification on promote/demote | Operator out-of-band visibility; same priority as live fills | **S** | Reuse `internal/notify` priority API. |
| Idempotent promotion (no duplicate (symbol, longExch, shortExch) tuples) | Phase 10 already validates this server-side; auto-promote must reuse the same path | **S** | Reuse existing `validatePriceGapCandidate` + duplicate-tuple check. |
| Active-position safety guard on auto-DEMOTE | Phase 10 manual-delete already blocks this; auto-demote must honor same invariant | **S** | Reuse existing `pg:positions:active` tuple-match check. |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Auto-DEMOTE on sustained underperformance | Closes the loop — candidates that score below threshold for M cycles get auto-removed | **M** | MUST honor active-position guard above. |
| Promotion "apprenticeship" — promoted candidates start in `paper_mode=true` regardless of global setting | Lets a discovered candidate be observed live-spread before risking capital | **L** | Per-candidate `paper_mode` override flag — schema change to `PriceGapCandidate`. May be deferred to v2.3. |
| Cooldown after demote (don't re-promote same tuple for X hours) | Stops oscillation when a candidate hovers near threshold | **S** | Single Redis key with TTL: `pg:disco:cooldown:<key>`. |

### Anti-Features

| Feature | Why Tempting | Why Problematic | Alternative |
|---------|--------------|-----------------|-------------|
| Promote → immediately fire trade if event detected | "no operator latency" | Removes the soak period a discovered candidate needs; couples discovery to execution | Promote only — let tracker tick decide on next scheduled detection cycle. |
| Auto-tune `PriceGapAutoPromoteScore` based on live PnL | "self-improving system" | Feedback loop on tiny sample size; user explicitly chose MVP-first over D-architecture | Operator tunes threshold via dashboard. |
| Auto-promote AND auto-bump per-candidate position size | Combines two risk decisions into one | Two independent decisions with different risk profiles; combine → blow up faster | Promotion uses default sizing; sizing ramp is the separate live-capital controller. |

---

## 3. Live-Capital Ramp Controller

### Table Stakes

| Feature | Why Expected | Complexity | Dependencies / Notes |
|---------|--------------|------------|----------------------|
| Stage-based ramp with explicit stages (100 → 500 → 1000 USDT/leg) | Project requirement explicit; no inference needed | **M** | New `internal/pricegaptrader/ramp.go`; stage table in config (`PriceGapRampStages`). |
| Time-gated stage advance (e.g., 7 clean days at stage N before stage N+1) | "Conservative ramp" requires soak time, not just trade count | **S** | Track `RampStageEnteredAt` in Redis `pg:ramp:state`; advance when `now - entered >= duration AND clean`. |
| Performance-gated advance ("clean" = no auto-disable, no exec-quality breach, daily PnL ≥ 0 on rolling N days) | Time alone doesn't capture quality; existing exec-quality auto-disable signal already tracks this | **M** | Reuse existing `ExecQualityRollingWindow` + auto-disable signal (Wave-6 from v2.0). |
| Hard ceiling enforcement at stage 3 (1000 USDT/leg cap for v2.2) | Project requirement; must NOT auto-advance past ceiling regardless of performance | **S** | Constant in code, not config; defensive — config drift can't break ceiling. |
| Dashboard visibility of current stage + days remaining | Operator must know "where am I in the ramp" without log diving | **S** | New WS event `pg_ramp_state`; small Overview widget. |
| Stage advance/regression Telegram alert | Operator awareness of capital exposure changes; same channel as fills | **S** | Reuse `internal/notify`. |
| Drawdown circuit breaker — auto-revert live → paper if daily loss > threshold | Project requirement; must integrate with paper-mode chokepoint at exchange.PlaceOrder | **M** | Read daily PnL from PG-specific reconcile job (item 4); flip global `PaperMode=true` AND set sticky flag so dashboard auto-flip bug (Phase 13 fix) can't re-enable. |
| Ramp state persistence + rehydration on restart | Operator sees apparent capital regression with no event if ramp resets — obscures real state | **S** | Persist in Redis `pg:ramp:state`; rehydrate on startup (consistent with Wave-6 rehydration pattern). |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Per-candidate ramp (each candidate ramps independently from time of promotion) | Promoted candidates shouldn't inherit veteran candidates' capital tier | **L** | Schema change to `PriceGapCandidate` for per-tuple `RampStage`. May be deferred — v2.2 ships global ramp first. |
| Manual stage override via pg-admin CLI | Operator emergency control without config edit | **S** | Add `pg-admin ramp set <stage>` subcommand. |
| Ramp regression on circuit-breaker trip (drop one stage on revert, not full reset) | "punishment without starting over" — preserves soak history | **S** | After paper revert, when re-engaged live, drop one stage rather than restart at 100. |

### Anti-Features

| Feature | Why Tempting | Why Problematic | Alternative |
|---------|--------------|-----------------|-------------|
| Continuous-curve ramp (size = f(days_clean) smoothly) | "no abrupt jumps" | Hides when capital exposure changed; harder to attribute PnL deltas to size deltas | Discrete stages — operator and audit can pinpoint exactly when size changed. |
| Auto-advance past 1000 USDT/leg if performance is great | "scale into a winner" | Project explicitly caps v2.2 at 1000; advancing requires v2.3 review | Enforce hard ceiling; surface "ceiling reached" status. |
| Ramp tied to global capital allocator (CA-03 weighted allocation) | "unifies capital systems" | CA-03 is for perp-perp + spot-futures; Strategy 4 is observation-stage; coupling early invites blow-up via dynamic shifts | Keep Strategy 4 budget on its own controller until v2.3 unification review. |
| Per-tick rebalancing of ramp stage based on intraday PnL | "responsive to market" | Same anti-pattern as continuous-curve; obscures cause/effect in audit; ramp is a multi-day decision | Daily reconcile-driven advance/regression only. |

---

## 4. Daily PnL Reconcile Job

### Table Stakes

| Feature | Why Expected | Complexity | Dependencies / Notes |
|---------|--------------|------------|----------------------|
| Fixed-time daily run (e.g., 00:05 UTC, after exchange settlement windows) | Standard for PnL jobs; must avoid Bybit :04–:05:30 blackout | **S** | Time scheduler with timezone awareness; reuse `time.Ticker` pattern from engine. |
| Reads `pg:positions:closed` for the prior 24h window | Phase 8 already persists realized PnL on close (Wave-6 D-21); reuse | **S** | Existing Redis schema; no new writes from this job. |
| Per-candidate aggregation: trade count, gross PnL, fee total, slip total, win rate | Standard reconcile dimensions; operator needs to know which candidates earn vs leak | **S** | Pure aggregation; no new exchange API surface. |
| Per-day total: net PnL, capital deployed, ROI%, max drawdown intraday | Required for ramp circuit breaker (item 3) and weekly review | **M** | Drawdown requires tick-level PnL series; can compute from existing AnalyticsSnapshot if `EnableAnalytics=true`. |
| Persist results to `pg:reconcile:daily:<YYYY-MM-DD>` Redis key | Audit trail; required for ramp + dashboard history | **S** | TTL = 90 days. |
| Telegram daily summary (one message, not per-line) | Standard end-of-day operator brief | **S** | Single batched alert. |
| Idempotent — re-running for the same day produces identical result | Operator may need to re-run after fix; non-idempotent jobs corrupt history | **S** | Use Redis `SETNX` lock on the day key. |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Cross-check vs exchange settled balances | Catches accounting drift between internal PnL and exchange truth | **L** | Per-exchange balance fetch + diff; deferred to v2.3 likely. |
| Anomaly flags (single-trade > 3σ from mean, gross slip > model 5×, etc.) | Surfaces issues for human review without dashboard digging | **S** | Computed during aggregation. |
| Modeled-vs-realized slip backtest comparison | Closes the loop between PG-VAL-03 (modeled slip from v0.34.11) and live | **S** | Already have `ModeledSlipBps`; just diff. |

### Anti-Features

| Feature | Why Tempting | Why Problematic | Alternative |
|---------|--------------|-----------------|-------------|
| Hourly reconcile | "more granular" | Most positions span hours; intraday reconcile double-counts open positions | Daily reconcile + dashboard live PnL handles intraday. |
| Reconcile triggers fund transfers | "auto-rebalance based on PnL" | Couples accounting to capital movement; failures in either become entangled | Reconcile is read-only; rebalancing is a separate (existing) job. |
| Per-trade reconcile email/Telegram | "full audit trail" | Floods operator; defeats the purpose of a daily summary | Daily batched summary + on-demand drill-down via dashboard. |

---

## 5. Telegram Alerts for Live Fills

### Table Stakes

| Feature | Why Expected | Complexity | Dependencies / Notes |
|---------|--------------|------------|----------------------|
| Per-fill alert: entry (both legs filled) and exit (both legs filled) | Project requirement explicit; v1.0 already does this for perp-perp via `internal/notify` | **S** | Reuse existing `internal/notify` API; new event types `pg_entry_fill`, `pg_exit_fill`. |
| Severity levels (info / warn / critical) | Standard Telegram operational pattern; v1.0 PP-01 established this | **S** | Already supported by `internal/notify`. |
| Severity routing — fills are info, ramp/circuit-breaker are warn, machine-zero/paper-flip are critical | Without this, operator alarm fatigue | **S** | Per-event-type config in `internal/notify`. |
| Per-event-type cooldown (suppress repeats within N seconds) | v1.0 already does this for SL/L4/L5; same need here | **S** | Reuse existing cooldown mechanism. |
| Live-only filter — paper fills do NOT alert | Paper trades produce >10× the volume of live; would drown signal | **S** | Branch on `PaperMode` flag at notify call site. |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Rich message content (symbol, both legs, gross spread, modeled vs realized slip, position ID) | Single-glance triage from phone | **S** | Format string in notify call site. |
| Daily summary message at reconcile time (one combined message) | Closes the day cleanly | **S** | Already covered in reconcile section. |
| Markdown deep-link to dashboard position detail | One-tap drill-down | **M** | Requires public dashboard URL config + auth shape (currently bearer in localStorage). May be deferred. |

### Anti-Features

| Feature | Why Tempting | Why Problematic | Alternative |
|---------|--------------|-----------------|-------------|
| Alert on every order placement (not just fills) | "see everything" | IOC orders fire-and-forget; alerts on placement before fill confirmation = noise + duplicates | Alert on fill confirmation only (per project spec). |
| Alert on every spread tick crossing threshold | "early warning" | 4-bar persistence detector already gates this; pre-detection alerts are speculation | Alert only on detector → gate-passed → entry decision. |
| Per-leg alerts (separate message for long leg fill vs short leg fill) | "full granularity" | Doubles message count; legs typically fill within seconds | Single combined message at "both legs filled OR partial-fill reconciled". |
| Alert routing to multiple chats based on severity | "right person, right alert" | Single-operator system; complexity not justified at v2.2 scale | One chat ID; severity is in the message. |
| Aggregating fills into batched messages | "less noise" | Live trading volume at 100–1000 USDT/leg is low (~few fills/day at MVP); batching loses real-time signal | Per-fill direct alert; batch only for the daily summary. |

---

## Feature Dependencies

```
PG-DISC-01 (Scanner)
    └──produces──> PriceGapDiscoveryCandidate + score
                       └──consumed by──> PG-DISC-02 (Auto-Promote)
                                              └──appends to──> cfg.PriceGapCandidates
                                                                   └──consumed by──> Existing PG Tracker (Wave-3..6)

PG-DISC-03 (Telemetry) ──reads from──> PG-DISC-01 cycle stats + PG-DISC-02 promote/demote events

Live-Capital Ramp ──reads from──> Daily PnL Reconcile (clean-day signal)
                  ──reads from──> Existing exec-quality auto-disable (Wave-6)
                  ──writes to──> internal/pricegaptrader sizing path
                  ──can flip──> Global PaperMode (chokepoint at exchange.PlaceOrder)

Daily PnL Reconcile ──reads from──> pg:positions:closed (existing Wave-6 persistence)
                    ──reads from──> AnalyticsSnapshot 1m series (for intraday drawdown)
                    ──writes to──> pg:reconcile:daily:<date>
                    ──triggers──> Telegram daily summary

Telegram Per-Fill ──reads from──> Existing pricegaptrader entry/exit completion paths
                  ──guarded by──> PaperMode flag (live-only)
                  ──reuses──> internal/notify priority API (v1.0)
```

### Key Dependency Notes

- **Auto-promote MUST honor Phase 10 active-position safety guard:** Auto-DEMOTE that would orphan a live position must be blocked, exactly as Phase 10 manual-delete is blocked. Reuse the existing `pg:positions:active` tuple-match check.
- **Ramp circuit breaker → PaperMode flip MUST set sticky flag:** Phase 13's PG-OPS-08 fix solved the dashboard auto-flip bug, but a circuit breaker triggering paper revert needs to NOT be re-flipped to live by the next page load. Add `PaperModeStickyUntil` timestamp.
- **Discovery scanner depth probe shares rate-limit budget with engine:** Don't add a separate rate limiter; reuse adapter rate-limit middleware. Cap scan cadence so peak concurrency stays under engine headroom.
- **Daily reconcile depends on AnalyticsSnapshot for intraday drawdown:** If `EnableAnalytics=false`, the ramp circuit breaker has no intraday signal — must fall back to end-of-day-only check OR document the requirement.
- **Telegram per-fill must NOT fire from paper executions:** The existing chokepoint at exchange.PlaceOrder branches on PaperMode; the notify call site must be downstream of (and aware of) that branch.
- **Auto-promotion writes config.json — config.json safety is paramount:** SaveJSON pattern + .bak backup + active-position safety guard. Any failure mode that could write a corrupted candidate list must be tested explicitly (Wave-3 had `redis: nil` regression — same level of paranoia required here).

---

## MVP Definition

### Launch With (v2.2 — Priority 1)

- [ ] PG-DISC-01 minimum: scan loop + volume/spread/depth filters + denylist + reasoning string
- [ ] PG-DISC-02 minimum: score threshold + max-cap + observation streak + active-position guard on demote + WS + Telegram
- [ ] PG-DISC-03 minimum: cycle telemetry persistence + dashboard read-only view of last cycle
- [ ] Live-capital ramp: 3 stages, time + clean-day gates, hard ceiling, drawdown circuit breaker (paper revert with sticky flag)
- [ ] Daily PnL reconcile: per-day aggregation + Telegram summary + Redis persistence + idempotent
- [ ] Telegram per-fill: entry + exit, both legs, severity routing, cooldown, live-only filter

### Add After Validation (v2.3+)

- [ ] Discovery sub-score visibility breakdown in dashboard (vol/spread/depth/persistence)
- [ ] Auto-demote on underperformance + cooldown
- [ ] Per-candidate paper-mode "apprenticeship"
- [ ] Per-candidate ramp (vs global)
- [ ] Reconcile cross-check vs exchange settled balances
- [ ] Modeled-vs-realized slip backtest comparison surfaced in dashboard

### Future Consideration (v3+)

- [ ] ML-scored discovery (only after >100 closed live trades for sample size)
- [ ] Strategy 4 → unified capital allocator integration (CA-03)
- [ ] Cross-strategy capital flow on Strategy 4 outperformance

---

## Complexity Summary

| Feature Area | Total Complexity | Net New Code | Reuses Existing |
|---|---|---|---|
| PG-DISC-01 Scanner | **M** (~3 plans) | discovery.go, sample storage, scoring | Adapter `GetTicker24h` + `GetOrderBook`, `EstimateSlippage`, denylist pattern |
| PG-DISC-02 Auto-Promote | **M** (~2 plans) | promotion controller, streak tracking | SaveJSON, validatePriceGapCandidate, Phase 10 safety guard, internal/notify, WS broadcast |
| PG-DISC-03 Telemetry | **S** (~1 plan) | dashboard view + cycle storage | WS pattern, Recharts |
| Live-Capital Ramp | **M-L** (~3 plans) | ramp.go, circuit breaker, sticky paper flag | exec-quality auto-disable, pg-admin CLI, internal/notify, paper-mode chokepoint |
| Daily PnL Reconcile | **M** (~2 plans) | reconcile job + scheduler | pg:positions:closed, AnalyticsSnapshot, internal/notify, ModeledSlipBps |
| Telegram Per-Fill | **S** (~1 plan) | notify call sites at fill points | internal/notify priority API, cooldown, severity, PaperMode flag |

**Total estimated:** ~12 plans across 5–6 phases. Feasible in v2.2 alongside paper-mode bug closure (Priority 2) and v1.0 tech-debt sweep (Priority 2).

---

## Sources

- `.planning/PROJECT.md` (v2.2 milestone scope, key decisions, constraints) — HIGH
- `.planning/MILESTONES.md` (v2.0/v2.1 actual deliverables, deferred items numbering) — HIGH
- `.planning/RETROSPECTIVE.md` (v1.0 lessons: doc drift, sample-size lessons, edge-narrowing under real costs) — HIGH
- Existing internal/pricegaptrader code structure (Wave-3..6 from v2.0; Phase 10 + 999.1 from v2.1) — HIGH (referenced via project doc)
- Codex design review 2026-04-21 (Gate concentration, correlation crowding, hard denylist, exec-quality override as first-class) — HIGH
- Industry standard for ramp controllers + reconcile jobs in production trading systems — MEDIUM (general practice, not exchange-specific)
