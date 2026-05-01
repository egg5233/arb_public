# Phase 15: Drawdown Circuit Breaker - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-01
**Phase:** 15-drawdown-circuit-breaker
**Mode:** discuss (interactive, default cadence)
**Areas discussed:** Eval cadence, Strike+limit, Recovery UX, Test+widget

---

## Eval cadence (Area A)

### A.1 — What triggers a breaker evaluation tick?

| Option | Description | Selected |
|--------|-------------|----------|
| 5-min ticker | Dedicated goroutine + time.Ticker firing every 5 min, decoupled from reconcile. Pairs naturally with the spec's ≥5 min strike spacing. | ✓ |
| On close event | Evaluate immediately after every position close. Instant reaction; couples breaker to close path; can be noisy. | |
| Reconcile only | Coupled to UTC 00:30 daily reconcile. Simplest. Too coarse for 'rolling 24h' intent. | |

**User's choice:** 5-min ticker
**Notes:** Recommended. Most aggressive cadence in the system but correct given the strike-spacing contract.

### A.2 — How is the rolling 24h realized PnL computed?

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse index | SMEMBERS pg:positions:closed:{today} + {yesterday}, GET each, filter close_ts >= now-24h, sum. Reuses Phase 14 D-02 index. | ✓ |
| New ZSET key | New pg:breaker:closed_pnl ZSET keyed by close_ts. O(log N) lookup; second close-time index to maintain. | |
| You decide | Claude picks based on volume estimate. | |

**User's choice:** Reuse index
**Notes:** Recommended. No new key. Leverages Phase 14's already-populated index.

### A.3 — How is the Bybit :04–:05:30 blackout suppression scoped?

| Option | Description | Selected |
|--------|-------------|----------|
| Skip whole tick | Entire eval tick is no-op during blackout. Matches spec wording verbatim. Simple. | ✓ |
| Skip Bybit only | Evaluator runs but Bybit-leg PnL contributions excluded. Partial-window math errors. | |

**User's choice:** Skip whole tick
**Notes:** Recommended.

### A.4 — What happens to a pending Strike-1 across a blackout-suppressed tick?

| Option | Description | Selected |
|--------|-------------|----------|
| Preserve strike | Strike-1 state survives blackout. Drawdown that crosses blackout still fires. | ✓ |
| Reset strike | Strike-1 cleared on each blackout-suppressed tick. Conservative against false positives. | |
| You decide | Claude picks based on test fixture scenarios. | |

**User's choice:** Preserve strike
**Notes:** Recommended. Blackout = "skip evaluation", not "reset state".

---

## Strike+limit (Area B)

### B.1 — Where does Strike-1 pending state live?

| Option | Description | Selected |
|--------|-------------|----------|
| Redis-persisted | New pg:breaker:state HASH with 5 fields. Survives kill -9. Mirrors Phase 14 ramp HASH precedent. | ✓ |
| In-memory only | Strike-1 in struct field; restart clears it. Conservative against stale-strike bugs but masks real drawdowns across restart. | |
| You decide | Claude picks based on restart-safety risk model. | |

**User's choice:** Redis-persisted
**Notes:** Recommended.

### B.2 — What does PriceGapDrawdownLimitUSDT measure?

| Option | Description | Selected |
|--------|-------------|----------|
| Absolute USDT | Single value (e.g., -50). Doesn't auto-scale with stage. Matches spec field name. | ✓ |
| % of stage | Pct of current ramp stage size. Auto-scales but re-shapes the spec field. | |
| Both | Absolute by default; optional pct field overrides if non-zero. | |

**User's choice:** Absolute USDT
**Notes:** Recommended. Spec field name pinned ...USDT.

### B.3 — What is the PaperModeStickyUntil field shape?

| Option | Description | Selected |
|--------|-------------|----------|
| Sentinel ts | int64 unix-ms. 0 = not sticky. MaxInt64 = sticky-until-operator-clears. Future-proof. | ✓ |
| Boolean+reason | Two keys (sticky bool + trip_reason string). Cleaner semantics, more keys. | |
| RFC3339 string | ISO8601 string. Human-readable in Redis CLI. Slightly more bytes. | |

**User's choice:** Sentinel ts
**Notes:** Recommended.

### B.4 — When PnL recovers between ticks, what happens to a pending Strike-1?

| Option | Description | Selected |
|--------|-------------|----------|
| Clear on recovery | Recovered tick clears pending_strike=0. Re-arm fresh. | ✓ |
| Hold N minutes | Strike-1 persists for fixed window regardless of intervening ticks. | |
| You decide | Claude picks; simplest is clear-on-recovery. | |

**User's choice:** Clear on recovery
**Notes:** Recommended. "Two consecutive breaches" interpreted in spirit.

---

## Recovery UX (Area C)

### C.1 — Where can the operator clear the sticky flag?

| Option | Description | Selected |
|--------|-------------|----------|
| pg-admin only | Single trigger path. Matches Phase 14 D-14 read-only-dashboard precedent. | |
| pg-admin + dash | Both surfaces, both with typed-phrase confirmation. More accessible during incident response. | ✓ |
| Dashboard only | UI-first. Easier but harder to audit. | |

**User's choice:** pg-admin + dash
**Notes:** Deviates from Phase 14 D-14 — Phase 15 widget IS write-capable (recover + test-fire only). Audit risk mitigated by typed-phrase confirmation on every surface (D-12).

### C.2 — What does "auto-disable any open candidate" do?

| Option | Description | Selected |
|--------|-------------|----------|
| Paused flag | New per-candidate paused_by_breaker bool, distinct from existing disabled. Reversible cleanly on recovery. | ✓ |
| Set disabled | Mutate existing disabled field. Risk: collides with operator-set disables. | |
| Remove | Delete candidate entries. Clean slate but operator loses calibration. | |

**User's choice:** Paused flag
**Notes:** Recommended. Separation of concerns from operator-set state.

### C.3 — On recovery, what happens to candidates disabled by the breaker?

| Option | Description | Selected |
|--------|-------------|----------|
| Auto re-enable | Recovery clears paused_by_breaker on all candidates. One-step recovery. Operator-disabled stays disabled. | ✓ |
| Manual review | Recovery clears sticky only; each candidate re-enabled by hand. Forces triage. | |
| You decide | Claude picks based on test scenarios. | |

**User's choice:** Auto re-enable
**Notes:** Recommended. Operator-set disabled is preserved.

### C.4 — How strong is the recovery confirmation requirement?

| Option | Description | Selected |
|--------|-------------|----------|
| Typed phrase | Both pg-admin and dashboard require typing 'RECOVER' before executing. | ✓ |
| Flag only | --confirm flag suffices; no typed phrase. Less friction. | |
| No confirm | Single command, no flag. Too easy to mis-fire. | |

**User's choice:** Typed phrase
**Notes:** Recommended. Same applied to test-fire (D-12, D-13).

---

## Test+widget (Area D)

### D.1 — How is a synthetic test fire triggered?

| Option | Description | Selected |
|--------|-------------|----------|
| pg-admin only | Single trigger path. Matches pg-admin precedent. | |
| pg-admin + dash | Both surfaces, symmetric with C.1. Dashboard "Test fire" with typed-phrase modal. | ✓ |
| API only | POST /api/pg/breaker/test-fire only. No CLI. | |

**User's choice:** pg-admin + dash
**Notes:** Symmetric with recovery surface (C.1).

### D.2 — What does test fire actually do?

| Option | Description | Selected |
|--------|-------------|----------|
| Real default | Real trip by default; --dry-run flag opts into simulation. Validates spec success-criterion #5. | ✓ |
| Real only | Always real; no dry-run option. | |
| Dry-run default | Default = simulate; --live flag opts into real trip. Risks silent passes. | |

**User's choice:** Real default
**Notes:** Recommended. Trip records distinguish source: live | test_fire | test_fire_dry_run (D-18).

### D.3 — Where does the breaker state surface in the dashboard?

| Option | Description | Selected |
|--------|-------------|----------|
| Extend Phase14 | Add Breaker subsection to existing Phase 14 widget. Phase 16 PG-OPS-09 absorbs entire widget into new top-level Pricegap tab. | ✓ |
| New widget | Separate breaker-only widget on same tab. More layout churn for Phase 16. | |
| Wait for P16 | No dashboard surface this phase. Conflicts with C.1 dashboard recovery button. | |

**User's choice:** Extend Phase14
**Notes:** Recommended.

### D.4 — What does the Telegram critical alert contain?

| Option | Description | Selected |
|--------|-------------|----------|
| Full context | Compact phone-readable: PnL, threshold, window ts, ramp stage, paused count, recovery instruction. Mirrors Phase 14 D-11 digest shape. | ✓ |
| Minimal | Just trip event + ts + recovery command. Less actionable from phone. | |
| Full + positions | Full context plus per-position list. Risks message-length limits. | |

**User's choice:** Full context
**Notes:** Recommended.

---

## Claude's Discretion

The following items were left to Claude during planning/implementation (non-blocking):

- Internal struct/interface naming (`BreakerController`, `RealizedPnLAggregator`, `BreakerStateStore`)
- Whether the 5-min interval is configurable or hardcoded (lean configurable with safety floor)
- Eval-tick logging cardinality (always vs only-on-breach)
- Boot-time guard policy (permissive cold-start vs Phase 14 ramp's strict)
- Atomicity primitive for D-15 (Lua script vs sequential best-effort)
- Telemetry exposure shape
- Test fixture organization for the synthetic test-fire integration test
- i18n locale strings for the new Recover/Test-fire buttons
- Default value of `PriceGapDrawdownLimitUSDT` for fresh installs (default 0 = armed-but-never-trips)

## Deferred Ideas

Captured in CONTEXT.md `<deferred>` section. Highlights:

- Auto-recovery (REQUIREMENTS Out-of-Scope)
- MTM-based breaker (v3.0+)
- Per-symbol breakers (v2.2 single global)
- Threshold scaling with ramp stage (revisit v2.3)
- Top-level Pricegap tab consolidation (Phase 16 PG-OPS-09 — Phase 15 widget extends Phase 14, Phase 16 absorbs)
- Cross-strategy breaker (v3.0 unification milestone)
