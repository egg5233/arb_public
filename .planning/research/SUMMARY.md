# v2.2 Research Summary — Auto-Discovery & Live Strategy 4

**Synthesized:** 2026-04-28
**Sources:** STACK.md, FEATURES.md, ARCHITECTURE-v2.2.md, PITFALLS.md
**Milestone goal:** Promote Strategy 4 from paper to live capital + ship deferred auto-discovery + close paper-mode bugs + clear v1.0 tech debt.
**Confidence:** HIGH

> Note: prior content here was the 2026-04-01 v1.0 summary; replaced for v2.2.

---

## Executive Summary

v2.2 promotes Strategy 4 from paper observation to live capital and ships the auto-discovery pipeline deferred twice from v2.0/v2.1 — all additive inside `internal/pricegaptrader/`, **zero new Go dependencies, zero new npm dependencies**.

The primary risk is the **three-writer race** on `cfg.PriceGapCandidates` (operator dashboard + pg-admin CLI + new auto-promotion goroutine), which requires a `CandidateRegistry` chokepoint to land BEFORE the scanner is write-permitted.

Secondary risks: ramp-counter loss across systemd restarts (binary drift monitor fires routinely), and the drawdown breaker must measure **REALIZED PnL only in rolling 24h** — not calendar-day MTM — to avoid funding-settlement false trips.

---

## Stack Additions

**Net additions: ZERO** — every v2.2 feature composes from existing dependencies.

- Go: `redis/go-redis/v9` v9.18.0, `modernc.org/sqlite` v1.48.1, `gorilla/websocket` v1.5.3, `miniredis/v2` v2.37.0 — all already in `go.mod`
- Frontend: Recharts 3.8.1 + React 19.2.x — sufficient for telemetry dashboards
- No new `pkg/exchange` interface methods needed; existing `GetSpotBBO(symbol)` is sufficient for the auto-discovery scanner (klines NOT required because the existing 4-bar persistence detector builds bars from BBO samples)

**Pre-flight smoke for every plan:** `go mod verify && go build ./... && cd web && npm ci`

---

## Feature Categorization

### Auto-Discovery Scanner (PG-DISC-01)
- **Table stakes:** Bounded universe (cap 20 symbols × 6 exchanges = 120 BBOs/cycle), ≥4-bar persistence, BBO freshness gate, depth probe, default OFF
- **Differentiators:** Score history visualization, score-vs-realized-fill calibration view
- **Anti-features:** N×N pair scan every cycle (rate-limit hostile), per-tick scoring, single-bar promotion
- **Size:** M

### Auto-Promotion (PG-DISC-02)
- **Table stakes:** Score gate, max-cap (default 12), persisted via SaveJSON, idempotent dedupe (incl. v0.35.0 `direction` field), active-position guard on demote (Phase 10 reuse), observation streak ≥6 cycles before promotion clears
- **Differentiators:** Telegram + WS broadcast on promote/demote
- **Anti-features:** Auto-promotion → immediate trade firing (preserve soak period), auto-tuning threshold from PnL (sample size too small)
- **Size:** M

### Discovery Telemetry (PG-DISC-03)
- **Table stakes:** Scanner cycle stats in Redis, dashboard tab/section, score history per candidate, why-rejected breakdown
- **Differentiators:** Promote/demote event timeline
- **Anti-features:** Real-time per-tick streaming (use existing WS hub batching)
- **Size:** S

### Strategy 4 Live Capital + Ramp Controller
- **Table stakes:** Discrete stages (100 → 500 → 1000 USDT/leg), 7-clean-days signal sourced from reconcile, `min(stage, hard_ceiling)` enforced at sizing site, Redis-persisted ramp state (5 explicit fields), idempotent daily evaluation
- **Differentiators:** Asymmetric ratchet (loss day resets clean-day counter to 0 + demotes one stage), Telegram on stage advance/revert
- **Anti-features:** Continuous-curve ramp, in-memory ramp counter (loses across systemd restarts), gating that lives outside `risk_gate.go`
- **Size:** L (highest-stakes phase)

### Drawdown Circuit Breaker
- **Table stakes:** REALIZED PnL only, rolling 24h window, suppress during Bybit `:04-:05:30` blackout, two-strike rule, human-gated recovery via sticky `PaperModeStickyUntil`
- **Differentiators:** Telegram critical alert + WS, automatic auto-disable of any open candidate
- **Anti-features:** Calendar-day boundary, MTM trigger, auto-recovery without operator confirmation
- **Size:** M

### Daily PnL Reconcile
- **Table stakes:** Per-CLOSED-position keyed by `(position_id, version)`, exchange close-timestamp (not local clock), reuse perp-perp 3-retry pattern, run 30+ min after UTC 00:00
- **Differentiators:** Daily summary digest, anomaly flagging (large slippage, missing close)
- **Anti-features:** Including open positions in daily PnL, local-clock window, single-attempt fetch
- **Size:** S–M

### Telegram Per-Fill Alerts
- **Table stakes:** Dedicated `pricegap_fill` bucket isolated from L4/L5/breaker critical alerts, ONE entry-complete + ONE exit-complete per position (not per slice), async worker, critical bypass path retained
- **Differentiators:** Per-fill rate-limit, batched digest if rate exceeded
- **Anti-features:** Fire from paper executions (drowns signal), share rate limit with critical alerts
- **Size:** S

### Paper-Mode Bug Closure
- **Items:** `realized_slippage_bps` zero-fix (Phase 9 Pitfall 7 formula bug), paper_mode auto-flip diagnose+fix (DevTools capture + handler audit), promote `cmd/bingxprobe/` → `make probe-bingx` target
- **Constraint:** Don't regress the Phase 9 chokepoint pattern (paper mode = single chokepoint at `ex.PlaceOrder`, `pos.Mode` immutable after entry)
- **Size:** S

### v1.0 Tech-Debt Sweep
- **Items:** Phase 07 VERIFICATION.md + VALIDATION.md (SF-RISK-01); Nyquist Wave-0 for phases 01, 03, 04, 06; browser confirmations for 02, 03, 05, 06
- **Constraint:** Surfaced regressions during retrospective review treated as separate hot-fix mini-phases (Phase 999.1 precedent)
- **Size:** L (volume), low complexity per item

---

## Architecture Integration

**Module boundary preserved:** all 6 new components live as new files inside `internal/pricegaptrader/`:
- `scanner.go` (PG-DISC-01)
- `promotion.go` (PG-DISC-02)
- `telemetry.go` (PG-DISC-03)
- `ramp.go` (live ramp controller)
- `breaker.go` (drawdown circuit breaker)
- `reconcile.go` (daily PnL reconcile)

**No new top-level packages.** Boundary rule (no `internal/engine` / `internal/spotengine` imports) preserved.

**Concurrency invariants preserved:**
- Tracker single-owner errgroup
- `cfg.mu.Lock()` + `SaveJSON()` for all candidate mutations (same path as Phase 10 dashboard CRUD)
- Per-symbol Redis lock retained
- Active-position guard reused for auto-demote
- Startup order unchanged (Scanner → RiskMon → HealthMon → API → Engine → SpotEngine → PriceGapTracker conditional)
- Bybit `:04-:05:30` blackout still respected
- Live ramp gates via `risk_gate.go` extension (gate #7), NOT a new authorization path

**Redis namespacing (all under existing `pg:`):**
- `pg:disc:*` (discovery scanner state)
- `pg:promote:*` (promotion events)
- `pg:scan:*` (scan cycle metrics)
- `pg:ramp:*` (ramp controller state)
- `pg:reconcile:daily:{date}` (daily reconcile output)
- `pg:breaker:trips` (breaker event log)

**Config additions (zero-value-safe defaults):**
- `PriceGapDiscoveryIntervalSec`
- `PriceGapAutoPromoteScore` (threshold, calibrate against paper data)
- `PriceGapMaxCandidates` (default 12)
- `PriceGapRampTier` (current stage)
- `PriceGapDrawdownLimitUSDT`
- `Source` discriminator on each `PriceGapCandidate` (manual / scanner / cli)
- `PaperModeStickyUntil` (sticky flag for breaker recovery gate)

---

## Watch Out For (Top Pitfalls)

1. **Three-writer race on `cfg.PriceGapCandidates`** — operator dashboard, pg-admin CLI, and new auto-promotion goroutine all mutate the same slice. Without a `CandidateRegistry` chokepoint, `.bak` rotation and tuple-dedupe become race-prone. **Fix:** chokepoint phase MUST land before scanner is write-permitted.

2. **Ramp counter loss across restarts** — binary drift monitor + systemd `Restart=on-failure` fire routinely. Ramp clean-day counter MUST be Redis-persisted with explicit fields, not in-memory.

3. **Drawdown breaker false trips** — calendar-day boundary causes 00:00 UTC noise; MTM trigger fires on funding-rate distortion + basis revert. Use REALIZED PnL only + rolling 24h window. Suppress during Bybit blackout.

4. **Auto-demote silently orphans live positions** — auto-demoter must honor `pg:positions:active` guard like Phase 10 manual delete. Otherwise an "unhealthy" candidate gets removed while a live position is still open.

5. **Telegram alert flooding** — high-frequency fills + critical L4/L5 alerts share one bucket. Separate `pricegap_fill` from critical, async worker, per-position aggregation (one entry-complete + one exit-complete).

6. **Paper-mode auto-flip regression** — fixing the `paper_mode=false` flip mustn't break the Phase 9 chokepoint pattern (`pos.Mode` stamped at entry, never re-read).

7. **Tech-debt closure surfacing latent bugs during live ramp** — Nyquist Wave-0 + browser confirms re-exercise stale paths. Sequence AFTER live capital is stable. Treat surfaced regressions as separate hot-fix phases.

---

## OPEN QUESTION for Roadmapper: Phase Ordering Disagreement

Architecture and Pitfalls disagree. This synthesis surfaces both; roadmap step resolves.

| Ordering | Sequence | Advantage | Risk |
|----------|----------|-----------|------|
| **Pitfalls (money-safe first)** | Live capital + ramp + breaker + reconcile (P14) → Chokepoint (P15) → Scanner + Telemetry (P16) → Auto-Promote (P17) | Live capital protected by chokepoint before any automated write; existing static candidates ride live ramp without scanner risk | Discovery scoring unvalidated when live ramp starts; calibration data not gathered before live capital |
| **Architecture (validate first)** | Scanner + Telemetry (P14) → Auto-Promote (P15) → Reconcile (P16) → Live Ramp (P17) → Breaker (P18) → Tech-debt (P19) | Scanner scoring calibrated against paper data before any live capital risk; zero live risk in P14 | If P15 chokepoint slips/regresses, auto-promotion could arrive before race protection lands |

**Hard constraints from research (apply regardless of ordering):**
- Reconcile + Ramp must be in the same phase (ramp's clean-day signal depends on reconcile output)
- CandidateRegistry chokepoint must land BEFORE scanner is write-permitted
- v1.0 tech-debt sweep is LAST (per Pitfall 7)
- Paper-mode bug closure can run parallel/independent
- 7-day soak between conservative ramp stages is non-negotiable

---

## Confidence

Overall: **HIGH**

| Area | Confidence | Reason |
|------|------------|--------|
| Stack (zero new deps) | HIGH | All recommendations are reuses; no speculation |
| Module boundary preservation | HIGH | All 6 components fit inside `internal/pricegaptrader/` |
| Redis schema additions | HIGH | Pattern matches existing `internal/database/locks.go` |
| Frontend lockdown compliance | HIGH | Recharts already covers all proposed visuals |
| Phase ordering | MEDIUM | Two valid orderings; roadmap step decides |

**Items deferred to plan-phase (not blocking roadmap):**
- Exact `PriceGapAutoPromoteScore` threshold — calibrate against 3+ days of paper-mode `pg:history` data
- Whether `BroadcastEvent` in `internal/api` is exported or needs wiring through tracker
- `PaperModeStickyUntil` exact field definition + dashboard enforcement
- Scanner universe: OKX + BingX exclusion must be explicit (deferred per PROJECT.md)
- Daily reconcile timing relative to Bybit `:04-:05:30` blackout (00:30 UTC proposed; cross-check with funding settlement windows)
- `EnableAnalytics` requirement for intraday drawdown signal — fallback if OFF must be documented

---

## Summary for Requirements Step

**6 phases recommended.** Phase numbering continues from v2.1 (next = 14). Phases 11+12 numbers reserved per original deferred-numbering plan; deferred numbers may be reused by roadmapper if cleaner.

**Track shape:**
- 3 phases for new feature pipeline (scanner / promotion / live capital + ramp + reconcile + breaker, possibly grouped per ordering choice)
- 1 phase for paper-mode bug closure (parallelizable)
- 1 phase for v1.0 tech-debt sweep (sequenced last)
- 1 phase for chokepoint serialization (precedes scanner write-permit)

Roadmapper resolves the open ordering question using the table above + the milestone-level priority signal from PROJECT.md (Strategy 4 live = Priority 1 alongside auto-discovery).
