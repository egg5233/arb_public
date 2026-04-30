# Phase 14: Daily Reconcile + Live Ramp Controller - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in 14-CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-30
**Phase:** 14-daily-reconcile-live-ramp-controller
**Areas discussed:** Reconcile job model, Ramp gate semantics, Anomaly + digest, Operator surfaces

---

## Reconcile Job Model

### Q1: How should the reconcile job execute?

| Option | Description | Selected |
|--------|-------------|----------|
| In-process daemon | Goroutine + time.Ticker inside pricegaptrader Tracker, fires UTC 00:30 daily. Matches existing scanner.go / promotion-controller pattern. Single binary, no extra systemd unit. pg-admin still gets a manual-run subcommand. | ✓ |
| Standalone binary + systemd timer | cmd/pg-reconcile/main.go invoked by systemd OnCalendar. Clean isolation, no shared goroutine state. Adds a deployment unit + install.sh changes. | |
| Daemon + CLI both | In-process daemon for production; pg-admin reconcile run --date=X CLI for backfills. Most flexible; both paths share the same internal Reconciler struct. | |

**User's choice:** In-process daemon (Recommended)
**Notes:** Matches Phase 12 controller wiring + scanner pattern; pg-admin CLI explicitly captured in operator-surface area below.

### Q2: How should reconcile discover positions closed on a given day?

| Option | Description | Selected |
|--------|-------------|----------|
| New dated index pg:positions:closed:{date} | SADD posID at close time inside monitor.closePair. O(1) date lookup. One Redis write per close. | ✓ |
| Filter pg:positions hash by ClosedAt field | No new write path. Linear scan; could grow unbounded; impacts rehydrate cost too. | |
| Use pg:history LIST + parse | Existing chronological list, capped at 500 — high-volume days could drop pre-cap data, breaking idempotency. | |

**User's choice:** New dated index pg:positions:closed:{date} (Recommended)
**Notes:** Bound on Redis growth + O(1) lookup wins over the no-write-path option.

### Q3: What happens when reconcile fails after the 3-retry window?

| Option | Description | Selected |
|--------|-------------|----------|
| Skip day + log + Telegram critical alert | Day stays unreconciled. Ramp treats unreconciled days as ambiguous (no clean credit, no demote). Operator backfills via pg-admin. | ✓ |
| Replay missed dates on next successful run | On boot or next-day fire, scan pg:reconcile:daily:* for gaps, back-fill. More moving parts; could mask repeated failures. | |
| Block ramp evaluation until reconciled | Ramp gate refuses to evaluate until prior-day reconcile lands. Hard-coupling. | |

**User's choice:** Skip day + log + Telegram critical alert (Recommended)
**Notes:** Conservative: errs toward not promoting. Manual backfill is explicit operator action.

### Q4: How should daily reconcile achieve byte-identical re-run output?

| Option | Description | Selected |
|--------|-------------|----------|
| Sort by (close_ts, position_id) + canonical JSON | Deterministic input ordering + json.Marshal with sorted map keys. Plain HSET overwrite. Locked by unit test. | ✓ |
| SET key value with JSON canonicalization | Single SET write per date. Simpler key shape; full overwrite trivially byte-identical. Loses subfield query without parsing. | |
| Lua script atomic compute+write | Most robust against concurrent races. Hardest to test/debug. Probably overkill — single writer. | |

**User's choice:** Sort by (close_ts, position_id) + canonical JSON (Recommended)
**Notes:** Tested directly via byte-level assertEqual on re-run.

---

## Ramp Gate Semantics

### Q1: What counts as a 'clean day' for the ramp counter?

| Option | Description | Selected |
|--------|-------------|----------|
| PnL ≥ 0 AND ≥1 closed position | Trading-day-based. No-activity days hold the counter. 7 actual trading days. Conservative. | ✓ |
| PnL ≥ 0 only (calendar) | Any UTC day with PnL ≥ 0 counts, even with zero closed positions. Ramps faster on quiet markets. | |
| PnL ≥ 0 AND no anomalies flagged | Strictest — anomaly flag blocks streak progression even if net PnL was positive. | |

**User's choice:** PnL ≥ 0 AND ≥1 closed position (Recommended)
**Notes:** Prevents trivial promotion through quiet weeks; prevents accidental demotion during operator downtime.

### Q2: How does live capital relate to the existing PriceGapPaperMode flag?

| Option | Description | Selected |
|--------|-------------|----------|
| New PriceGapLiveCapital flag, default OFF | Independent boolean. live=true requires paper_mode=false. Validated together. Two switches to risk capital. | ✓ |
| Reuse PriceGapPaperMode=false as the live signal | Single flag, simpler. Couples 'is paper' with 'is gated by ramp' — no dry-run live ramp. | |
| PriceGapLiveCapital + stage-1-only test mode | Live flag plus a dry-run mode with zero notional. More config surface. | |

**User's choice:** New PriceGapLiveCapital flag, default OFF (Recommended)
**Notes:** Two-flag explicitness chosen over single-flag simplicity for live-capital safety.

### Q3: Does the ramp gate fire in paper mode, or only in live capital mode?

| Option | Description | Selected |
|--------|-------------|----------|
| Only when PriceGapLiveCapital=true; paper bypasses | Ramp gate is a no-op in paper. Reconcile still runs in paper, so streak data is ready when operator flips live. | ✓ |
| Always evaluate, even in paper mode | Paper sizing clamped to ramp stage. Catches sizing bugs earlier; paper PnL no longer reflects unfettered candidate sizing. | |
| Paper bypasses, but log would-have-blocked decisions | Same as recommended + shadow telemetry. Useful for first weeks of live observation. | |

**User's choice:** Only when PriceGapLiveCapital=true; paper bypasses (Recommended)
**Notes:** Reconcile-runs-in-paper detail keeps the streak data warm.

### Q4: What is demote_count used for?

| Option | Description | Selected |
|--------|-------------|----------|
| Telemetry + ops visibility only | Monotonic counter, exposed in dashboard + pg-admin. Drives no automatic action. Phase 15 owns circuit-break. | ✓ |
| Cap on max demotes per rolling window | Freeze ramp at stage 1 after N demotes within M days. Risks overlap with Phase 15. | |
| Trigger Telegram alert above threshold | Telemetry + alert when count crosses threshold. No enforcement. | |

**User's choice:** Telemetry + ops visibility only (Recommended)
**Notes:** Avoids double-implementing circuit-break with Phase 15 (PG-LIVE-02).

---

## Anomaly + Digest

### Q1: How should the slippage anomaly threshold be set?

| Option | Description | Selected |
|--------|-------------|----------|
| Configurable PriceGapAnomalySlippageBps, default 50 | Operator-tunable. Validated [0, 500]. Same realized_slippage_bps signal PG-FIX-01 fixes. | ✓ |
| Hardcoded 50 bps (revisit later) | No config field. Re-tune via code change + version bump. | |
| Dynamic relative to entry edge | Self-scaling to candidate quality. Harder to reason about. | |

**User's choice:** Configurable PriceGapAnomalySlippageBps, default 50 (Recommended)
**Notes:** Has explicit dependency on PG-FIX-01 (Phase 16) for paper-mode slippage emission.

### Q2: What does reconcile do when a closed position is missing its exchange close-timestamp?

| Option | Description | Selected |
|--------|-------------|----------|
| Flag + fall back to ClosedAt (local clock) | Use pos.ClosedAt; record anomaly_missing_close_ts in day record + digest. Day still reconciles. Position still counts toward PnL. | ✓ |
| Flag only — exclude missing-ts position from PnL | Excluded + flagged. Strict; risks silently dropping totals if all positions miss ts. | |
| Block reconcile (skip whole day) | Strongest data integrity but blocks the clean-day signal. | |

**User's choice:** Flag + fall back to ClosedAt (local clock) (Recommended)
**Notes:** Keeps the clean-day signal flowing; anomaly flag handles operator review.

### Q3: What should the daily Telegram digest contain?

| Option | Description | Selected |
|--------|-------------|----------|
| Roll-up + anomaly highlights | Total realized PnL, positions count, win/loss split, ramp stage + counter, flagged position IDs. Compact, single message. | ✓ |
| Per-position table + summary line | Full per-position breakdown. Verbose; ugly above ~10 positions/day. | |
| Summary only — no anomaly inline | Just totals + ramp state. Anomalies via separate critical channel. | |

**User's choice:** Roll-up + anomaly highlights (Recommended)
**Notes:** Per-position detail still available via dashboard / pg-admin reconcile show.

### Q4: How should anomalies be routed to Telegram?

| Option | Description | Selected |
|--------|-------------|----------|
| Inline in daily digest only | Anomalies appear in digest list. No separate per-anomaly alert. | ✓ |
| Inline in digest + critical alert per anomaly | Each anomaly emits its own alert AND appears in digest. Higher noise. | |
| Critical alert only — omit from digest | Two-channel split. | |

**User's choice:** Inline in daily digest only (Recommended)
**Notes:** Phase 15 handles fire-now critical cases. Reconcile anomalies are next-day operator review.

---

## Operator Surfaces

### Q1: What pg-admin subcommands should Phase 14 add?

| Option | Description | Selected |
|--------|-------------|----------|
| Full set: reconcile run/show, ramp show/reset/promote/demote | Symmetric with Phase 12 controller surface. | ✓ |
| Read-only: reconcile show + ramp show | No mutation through pg-admin. Smaller blast radius. | |
| Reconcile-only — defer ramp CLI to Phase 15/16 | Tighter scope. Ramp surface paired with breaker tooling later. | |

**User's choice:** Full set: reconcile run/show, ramp show/reset/promote/demote (Recommended)
**Notes:** Operator needs full mutation surface for live-capital management; pg-admin is the safe out-of-band channel.

### Q2: What dashboard surface should Phase 14 build?

| Option | Description | Selected |
|--------|-------------|----------|
| Read-only widget in existing Pricegap-tracker tab | Ramp stage, counter, last loss day, last reconcile summary. No mutation UI. Phase 16 absorbs. | ✓ |
| Full surface now — build directly into new Pricegap tab | Pre-build Phase 16 layout. Expands Phase 14 scope. | |
| API-only, no UI | /api/pg/ramp + /api/pg/reconcile/{date} only. Defer all UI to Phase 16. | |

**User's choice:** Read-only widget in existing Pricegap-tracker tab (Recommended)
**Notes:** Read-only matches the conservative-first-touch-of-live-capital framing; mutation via pg-admin only.

### Q3: How are operator manual stage overrides recorded?

| Option | Description | Selected |
|--------|-------------|----------|
| pg-admin writes ramp state + appends pg:ramp:events log | Versioned event entry per force op. force-promote does NOT zero counter; force-demote DOES zero counter. | ✓ |
| Direct state write only — no event log | Simplest; loses audit trail. | |
| Block force-promote, allow force-demote + reset only | Most conservative — no manual upgrade path. | |

**User's choice:** pg-admin writes ramp state + appends pg:ramp:events log (Recommended)
**Notes:** Audit trail preserved; asymmetric ratchet semantics applied to force-demote.

### Q4: Should the reconcile fire time be configurable?

| Option | Description | Selected |
|--------|-------------|----------|
| Hardcoded UTC 00:30 | Single constant. Avoids Bybit blackout + funding settlement. No config field. | ✓ |
| Configurable PriceGapReconcileMinuteUTC (default 30) | Operator-tunable. More surface. | |
| Fire on first scan tick after UTC 00:30 | Reuses existing tick loop. Coarser timing (up to 30s late). | |

**User's choice:** Hardcoded UTC 00:30 (Recommended)
**Notes:** PG-LIVE-03 specifies 30+ min minimum; 00:30 is the safe minimum.

---

## Ready Check

### Q1: We've captured 16 decisions across all 4 areas. Any gray areas you want to expand, or ready for context?

| Option | Description | Selected |
|--------|-------------|----------|
| Ready for context | Write CONTEXT.md now. Hard contracts carry forward from ROADMAP/REQUIREMENTS. Ready for /gsd-plan-phase 14. | ✓ |
| Explore more gray areas | More gray areas (pg:reconcile schema, pg:ramp:events retention, position version semantics, boot-time catchup, PG-FIX-01 dependency). | |
| Revisit one of the 4 areas | Refine an existing area's answers. | |

**User's choice:** Ready for context (Recommended)

---

## Claude's Discretion

The following items were captured in CONTEXT.md `<decisions>` §"Claude's Discretion" — research / planning will resolve them:
- Exact JSON schema of `pg:reconcile:daily:{date}` value
- `pg:ramp:events` LIST cap
- Boot-time catchup logic
- Internal struct/interface names (`Reconciler`, `RampController`, daemon orchestrator)
- `PriceGapPosition.Version` semantics
- Pre-flight validation rule for live-capital activation
- pg-admin output formatting
- Telegram digest message wording
- Final config field names beyond locked set
- Concurrent-evaluator guard
- Test fixture details

## Deferred Ideas

Captured in CONTEXT.md `<deferred>` — out-of-scope items noted for future phases (Phase 15 breaker, Phase 16 dashboard tab, Phase 16 PG-FIX-01, v2.3+ stages above 1000, PG-LIVE-04 per-fill alerts, PG-DISC-05 calibration view, continuous ramp, cross-strategy ramp, auto-recovery, gradient promotion, configurable reconcile fire time).
