# Requirements — v2.2 Auto-Discovery & Live Strategy 4

**Defined:** 2026-04-28
**Core Value:** "I deposit USDT, select my risk preference, and the system automatically finds opportunities across multiple strategies, opens positions, collects yield, exits when profitable, and I can see exactly how much each position earned — with capital shifting between strategies as opportunities shift."

---

## Constraints (locked at milestone start)

- Live trading risk: changes must not break perp-perp, spot-futures, or pricegap-paper engines
- Module boundary: all new code lives inside `internal/pricegaptrader/` — no imports of `internal/engine` or `internal/spotengine`
- Redis namespace: all new keys under existing `pg:*` prefix
- npm lockdown still in force — no new frontend dependencies; Recharts/React only via `npm ci`
- Strategy 4 live capital hard ceiling for v2.2: 1000 USDT/leg
- All new behavior gated by config flags, default OFF
- Phase numbering continues from v2.1 (next phase = 14; phase 11+12 numbers reserved per original deferred-numbering plan)

---

## v2.2 Requirements

### Auto-Discovery & Promotion

- [ ] **PG-DISC-01**: Auto-discovery scanner polls a bounded universe (≤20 symbols × 6 exchanges) on configurable interval, applies ≥4-bar persistence detector + BBO freshness gate + depth probe, computes a per-candidate score, and writes scanner cycle output to `pg:scan:*` Redis keys. Default OFF via `PriceGapDiscoveryEnabled`. Scanner is read-only until PG-DISC-04 (CandidateRegistry chokepoint) lands.
- [x] **PG-DISC-02**: Auto-promotion controller appends candidates with score ≥ `PriceGapAutoPromoteScore` (new config field) to `cfg.PriceGapCandidates` via the chokepoint, capped by `PriceGapMaxCandidates` (new config field, default 12), persisted via SaveJSON with `.bak` rotation. Idempotent dedupe (incl. v0.35.0 `direction` field). Observation streak ≥6 cycles required before promotion clears. Auto-demote honors `pg:positions:active` guard (Phase 10 reuse). Promote/demote events emit Telegram critical alert + WS broadcast.
- [ ] **PG-DISC-03**: Discovery telemetry surfaces scanner cycle stats, score history per candidate, why-rejected breakdown, and promote/demote event timeline in the dashboard (new section under PG-OPS-09 tab) using existing Recharts components. Read path uses `pg:scan:*` and `pg:promote:*` Redis keys via existing WS hub batching.
- [ ] **PG-DISC-04**: `CandidateRegistry` chokepoint serializes all writers (operator dashboard CRUD from Phase 10, pg-admin CLI, scanner auto-promotion, scanner auto-demote) through one mutex + atomic file write + timestamped `.bak` ring. Required prerequisite before scanner is write-permitted (PG-DISC-02 cannot land until PG-DISC-04 ships).

### Strategy 4 Live Capital

- [x] **PG-LIVE-01**: Conservative ramp controller gates Strategy 4 live capital through discrete stages — 100 USDT/leg → 500 USDT/leg after 7 clean days → hard ceiling 1000 USDT/leg for v2.2. Ramp state Redis-persisted with 5 explicit fields (current stage, clean-day counter, last evaluation timestamp, last-loss-day timestamp, demote count). Asymmetric ratchet: any loss day resets the clean-day counter to 0 and demotes one stage. `min(stage_size, hard_ceiling)` enforced at the sizing call site, not only at stage transitions. Idempotent daily evaluation. Gate integrated into `risk_gate.go` as gate #7.
- [x] **PG-LIVE-02**: Drawdown circuit breaker monitors Strategy 4 daily REALIZED PnL on a rolling 24h window (NOT calendar-day, NOT MTM). When realized PnL drops below `PriceGapDrawdownLimitUSDT` (new config field), breaker fires: auto-revert live → paper via sticky `PaperModeStickyUntil` flag, auto-disable any open candidate, Telegram critical alert + WS broadcast, log to `pg:breaker:trips`. Suppress evaluation during Bybit `:04-:05:30` blackout. Two-strike rule (require breach + confirmation tick before firing). Recovery requires explicit operator action — sticky flag does not auto-clear.
- [x] **PG-LIVE-03**: Daily PnL reconcile job runs 30+ minutes after UTC 00:00 (deliberately offset from Bybit blackout + funding settlement windows). Aggregates closed Strategy 4 positions keyed by `(position_id, version)` using exchange close-timestamp (not local clock), reuses perp-perp 3-retry pattern. Output to `pg:reconcile:daily:{date}` Redis keys. Provides clean-day signal to PG-LIVE-01. Anomaly flagging on large slippage / missing close timestamps.

### Operations

- [ ] **PG-OPS-09**: New top-level "Price-Gap" dashboard tab consolidates ALL Strategy 4 configuration in one place — paper-mode toggle, ramp tier display, breaker threshold input, scanner config (interval, universe size, score threshold), `PriceGapMaxCandidates`, `PriceGapAutoPromoteScore`, plus the existing candidate CRUD UI from Phase 10 and bidirectional mode from Phase 999.1. Sits alongside Exchanges / Perp-Perp / Spot-Futures. Existing Strategy 4 config controls in other tabs migrate or proxy to the new tab.

### Paper-Mode Bug Closure

- [x] **PG-FIX-01**: Fix `realized_slippage_bps` machine-zero in paper mode. Phase 9 synth-fill formula computes delta vs modeled (Pitfall 7 documented in v2.0 retrospective); fix produces non-zero realized slippage so paper-mode metrics exercise the full Phase 8 pipeline.
- [x] **PG-FIX-02**: Diagnose + fix dashboard auto-POST that flipped `paper_mode=false` on page load (one-time observation during v2.0 UAT). Capture via DevTools Network panel, audit POST handler chain, enforce sticky paper flag respect across all writers. Must NOT regress the Phase 9 chokepoint pattern (`pos.Mode` stamped at entry, immutable).

### Developer Experience

- [x] **DEV-01**: Promote `cmd/bingxprobe/` debug utility into a `make probe-bingx` Makefile target so it's reproducible and discoverable for ops teams.

### v1.0 Tech Debt

- [ ] **DEBT-V1-01**: Write retrospective VERIFICATION.md + VALIDATION.md for v1.0 Phase 07 (SF-RISK-01 maintenance gate dashboard wiring). Code has been live since v0.29.0; this closes the unsatisfied verification requirement that classified v1.0 as `tech_debt`.
- [ ] **DEBT-V1-02**: Generate Nyquist Wave-0 validation tests for v1.0 Phases 01 (spot-futures expansion), 03 (operational safety), 04 (analytics), and 06 (spot-futures risk hardening). Code is live; validation docs are the gap.
- [ ] **DEBT-V1-03**: Run human_needed UI browser confirmations for v1.0 Phases 02 (spot-futures automation), 03 (operational safety dashboard), 05 (capital allocation), and 06 (risk hardening). Cleans up deferred verification entries; surfaced regressions become hot-fix mini-phases (Phase 999.1 precedent).

---

## v2.3+ Requirements (deferred)

### Operational Notifications (deferred per user, 2026-04-28)

- **PG-LIVE-04**: Telegram per-fill alerts with dedicated `pricegap_fill` bucket isolated from L4/L5/breaker critical alerts, per-position aggregation (one entry-complete + one exit-complete), async worker, critical bypass retained. Surfaced in research; descoped from v2.2.

### Discovery Calibration

- **PG-DISC-05**: Score-vs-realized-fill calibration view in dashboard — empirical comparison of scanner score versus actual fill quality for promoted candidates. Differentiator from FEATURES research.

---

## Out of Scope

| Feature | Reason |
|---------|--------|
| Strategy 4 live capital beyond 1000 USDT/leg | Hard ceiling for v2.2 conservative ramp; revisit in v2.3 after live observation |
| OKX + BingX in Strategy 4 universe | Already deferred per PROJECT.md; continues for v2.2 |
| Cross-exchange spot-futures | Different architecture; v3.0 territory |
| Auto-tune `PriceGapAutoPromoteScore` from PnL | Sample size too small; manual calibration with operator review |
| Per-tick scanner scoring | Anti-feature; rate-limit hostile + creates noise |
| Continuous-curve ramp | Anti-feature; obscures cause/effect; discrete stages preferred |
| Auto-recovery from drawdown breaker | Sticky paper flag does not auto-clear; operator action required |
| Telegram fill alerts in paper mode | Drowns critical alert signal; paper events stay silent |
| New Go module dependencies | Zero-new-deps target per Stack research; revisit only if a blocker |
| New npm packages | npm lockdown in force; only `npm ci` permitted |

---

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| PG-DISC-01 | Phase 11 | Pending |
| PG-DISC-02 | Phase 12 | Complete |
| PG-DISC-03 | Phase 11 | Pending |
| PG-DISC-04 | Phase 11 | Pending |
| PG-LIVE-01 | Phase 14 | Complete |
| PG-LIVE-02 | Phase 15 | Complete |
| PG-LIVE-03 | Phase 14 | Complete |
| PG-OPS-09 | Phase 16 | Pending |
| PG-FIX-01 | Phase 16 | Complete |
| PG-FIX-02 | Phase 16 | Complete |
| DEV-01 | Phase 16 | Complete |
| DEBT-V1-01 | Phase 17 | Pending |
| DEBT-V1-02 | Phase 17 | Pending |
| DEBT-V1-03 | Phase 17 | Pending |

**Coverage:**
- v2.2 requirements: 14 total
- Mapped to phases: 14 (100%)
- Unmapped: 0 ✓

---

## Success Criteria

v2.2 ships when:

1. Strategy 4 has executed at least one live entry+exit cycle at 100 USDT/leg with full reconciliation
2. Auto-discovery scanner has run for ≥7 days with telemetry showing scoring + rejection breakdown
3. CandidateRegistry chokepoint serializes 3 writers (dashboard, pg-admin, scanner) without observed race
4. Drawdown breaker has been exercised (synthetic test fire) and recovery path validated
5. Daily reconcile produces a complete daily summary for Strategy 4 positions
6. Price-Gap dashboard tab consolidates all Strategy 4 config; legacy controls in other tabs migrate or proxy
7. Phase 07 VERIFICATION.md + VALIDATION.md committed; Nyquist Wave-0 tests pass for phases 01/03/04/06; browser confirms recorded for phases 02/03/05/06
8. Paper-mode bugs (PG-FIX-01, PG-FIX-02) closed with regression tests
9. `make probe-bingx` works end-to-end
10. v1.0 tech-debt classification cleared from milestone audit

---

*Requirements defined: 2026-04-28*
*Last updated: 2026-04-28 — Roadmap step assigned phases (Phase 11/12/14/15/16/17)*
