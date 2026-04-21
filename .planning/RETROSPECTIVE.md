# Project Retrospective

Living document appended per milestone.

## Milestone: v1.0 — Unified Arb Platform

**Shipped**: 2026-04-21
**Phases**: 7 | **Plans**: 21 | **Commits**: 381 | **Timeline**: 30 days (2026-03-23 → 2026-04-21)

### What Was Built

- **Spot-futures engine** live on 5 exchanges (Binance/Bybit/Gate.io/Bitget/OKX) with auto-borrow, auto-discover, auto-open, auto-exit, basis gating, and maintenance-rate-aware survivable-drop pre-entry check
- **Operational safety**: Telegram alerts, rolling-window loss limits (24h/7d), dashboard Safety tab
- **Performance analytics**: SQLite time-series, Recharts-based Analytics dashboard, per-position PnL decomposition, strategy/exchange metrics
- **Capital allocation**: unified pool, risk profiles (conservative/balanced/aggressive), performance-weighted dynamic allocation, cross-strategy rebalancing
- **Risk hardening**: liquidation-distance runtime trigger, health monitor extension for spot positions, dashboard risk color-coding

### What Worked

- **GSD per-phase atomic commits** protected live trading from regressions throughout — no production incidents during v1.0 delivery
- **Interface-based DI** (`models.Discoverer`, `models.StateStore`, optional `SpotMarginExchange`) let Phase 6 risk hardening land without cross-engine coupling
- **Config-switch-default-OFF pattern** for every new feature let code ship progressively without forcing it into production paths
- **Delegation to Codex + sub-agents** for parallel exchange audits (6-agent team pattern) caught adapter bugs before livetest
- **Config.json single-source-of-truth fix** (2026-04-05, removing Redis HSET persistence) unblocked multiple phases that were fighting config wipe

### What Was Inefficient

- **Verification artifact drift**: 4 phases (02, 03, 05, 06) ended with `human_needed` VERIFICATION.md status because browser/visual tests were deferred and never recorded. Phase 07 shipped with no VERIFICATION.md or VALIDATION.md at all. Code worked; artifacts didn't catch up.
- **REQUIREMENTS.md checkbox drift**: SF-01..SF-07 traceability table said "Complete" but checkboxes stayed unchecked. Audit had to reconcile across 3 sources.
- **Phase-07 scope creep**: started as "milestone polish" but evolved into active feature work (maintenance gate dashboard wiring = v0.29.0). Not wrong, just that the naming misled the subsequent audit.
- **Nyquist Wave-0 validation gap**: only 2 of 7 phases authored VALIDATION.md. Non-blocking for v1.0 but leaves tech-debt footprint.

### Patterns Established

- **6-touch-point config pattern**: struct → response → builder → update → persistence → UI → i18n. Used consistently across phases 3–7 for new config fields.
- **Emergency-first trigger ordering**: spot-futures automation put emergency-close ahead of entry/exit in tick loop — prevents a problematic position from missing mitigation during a scan tick.
- **Health monitor cross-engine dispatch via callback**: avoided adding SpotEngine to the `models` package interface surface (Phase 6 decision).
- **Color coding per dashboard tab**: emerald=safety, amber=risk, violet=allocation — prevents users from confusing which tab they're editing.

### Key Lessons

- **GSD plans encoded "code done" but not "verified done".** Next milestone should make browser/visual confirmation a first-class plan step, not a deferred tail action. Otherwise audits will keep returning `human_needed`.
- **Retrospective VERIFICATION.md is cheap; skipping it isn't.** The one-unsatisfied-req issue on Phase 07 is purely documentation. Authoring it during the phase would have avoided the audit carry-forward.
- **Signal-vs-noise in backtests**: the Strategy 4 scoping work (outside v1.0 scope but same session) showed that 1-bar close-to-close spread crossings inflate edge estimates ~10× due to timestamp-alignment noise. Min-duration persistence filter (≥N bars) is a must for any spread-reversion analysis.
- **Edge universes narrow under real costs.** Strategy 4's Phase 1 (depth-measured slippage) killed most T=100 candidates. Future strategy scoping should measure real slippage *before* picking threshold levels.

### Cost Observations

- GSD framework executed 7 phases in 30 days with mostly inline Claude Code (sonnet/opus mix per phase complexity)
- Rolling hardening post-milestone (v0.31.0 → v0.32.34) shipped 12+ version bumps without GSD ceremony — evidence that small fixes don't need the full pipeline
- Sub-agent spawning for parallel exchange audits (6-agent pattern) was the highest-leverage use — caught bugs a serial reviewer would have missed

---

## Cross-Milestone Trends

*Populate as milestones accumulate.*

| Milestone | Phases | Plans | Duration | Audit status | Notable |
|---|---:|---:|---|---|---|
| v1.0 | 7 | 21 | 30d | tech_debt | First milestone under GSD; doc drift emerged |
