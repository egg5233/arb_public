# Requirements: Arb Bot — v2.0 Multi-Strategy Expansion

**Defined:** 2026-04-21
**Core Value:** "I deposit USDT, select my risk preference, and the system automatically finds opportunities across multiple strategies, opens positions, collects yield, exits when profitable — with capital shifting between strategies as opportunities shift."

## v2.0 Requirements (Strategy 4 MVP + validation infrastructure)

Requirements for this milestone. Each maps to roadmap phases 8+.

### Core Functionality (Strategy 4 tracker)

- [ ] **PG-01**: Price-gap tracker detects spread dislocations on configured candidate pairs using 1m kline data with ≥4-bar minimum-duration filter
- [ ] **PG-02**: Entry opens delta-neutral 2-leg positions via existing exchange adapters (IOC market orders on both legs simultaneously)
- [ ] **PG-03**: Exit closes positions when |spread| reverts to ≤ T/2 OR 4h max-hold timeout elapses
- [x] **PG-04**: Positions persist to Redis under own namespace (e.g., `pg:pos:{id}`) and survive process restart
- [x] **PG-05**: Candidate list (symbol, exchange pair, threshold, max position size) is configurable via config.json, not hardcoded

### Risk Controls

- [ ] **PG-RISK-01**: Gate exchange concentration cap enforces ≤50% of PriceGapBudget in live positions involving a Gate leg
- [ ] **PG-RISK-02**: Hard denylist pre-entry check blocks entry if either leg has a delist flag, halt status, or kline data older than 90s
- [x] **PG-RISK-03**: Execution-quality override forces a candidate back to disabled if realized slippage exceeds 2× modeled across the last 10 trades
- [ ] **PG-RISK-04**: Max concurrent positions cap (3 in v1) prevents over-exposure
- [ ] **PG-RISK-05**: Per-position notional cap per candidate (from config) enforced at entry

### Dashboard & Operations

- [ ] **PG-OPS-01**: New Dashboard "Price-Gap" tab shows candidate list with per-candidate enable/disable toggle
- [ ] **PG-OPS-02**: Dashboard shows live price-gap positions with entry spread, current spread, hold time, current PnL
- [ ] **PG-OPS-03**: Dashboard shows closed positions log with realized-vs-modeled edge comparison
- [ ] **PG-OPS-04**: Paper mode toggle runs full event/entry logic without placing real orders (used for first ~3 live days)
- [ ] **PG-OPS-05**: Telegram notifications fire on entry, exit, and risk-gate blocks (reusing existing notifier infrastructure)
- [x] **PG-OPS-06**: Config switch `PriceGapEnabled` (default OFF) gates the entire subsystem, with round-trip dashboard API persistence

### Live Validation Infrastructure

- [ ] **PG-VAL-01**: Every trade logs realized slippage vs modeled slippage (data supports exec-quality gate PG-RISK-03 and future model tuning)
- [ ] **PG-VAL-02**: Dashboard computes rolling 7d/30d net bps/day per candidate (supports human promote/demote decisions)

**Total v2.0 requirements:** 18

## Future Requirements (v2.1+, deferred)

Tracked but not in current roadmap. Promoted to v2.1 only after v2.0 live validation.

### Full Gated Discovery (Option D upgrade)

- **PG-D-01**: Rolling-spread scanner maintains 14d statistics per (symbol, pair) across broad universe (≥$100k daily volume floor)
- **PG-D-02**: Candidate lifecycle state machine (`watch` → `eligible` → `approved` → `cooldown`/`blocked`) with auto-promotion thresholds (half-life ≤15m median, ≥0.75 events/day, ≥65% reversion, ≥20 bps/day net after 1.5× slippage haircut, 2 consecutive daily passes)
- **PG-D-03**: Auto-demotion on stats decay (hysteresis: softer thresholds) and immediate hard demotion on delist/halt
- **PG-D-04**: Correlation cluster cap (≤40% per regime like `gate-binance`, `gate-bitget`)
- **PG-D-05**: Add OKX + BingX to Strategy 4 exchange universe
- **PG-D-06**: Capital allocator integration — Strategy 4 shares budget with existing strategies under unified profile

### v1.0 Tech Debt

- **DEBT-01**: Phase 07 retrospective VERIFICATION.md authored (SF-RISK-01 — maintenance gate shipped as v0.29.0, artifact missing)
- **DEBT-02**: Nyquist Wave-0 VALIDATION.md files for v1.0 phases 01, 03, 04, 06
- **DEBT-03**: Browser confirmation checklist executed for human_needed verifs on phases 02, 03, 05, 06

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Cross-asset pairs trading (e.g., BTC/ETH ratio) | Confirmed out — Strategy 4 is same-symbol cross-exchange only |
| Strategy 4 rolling scanner in v2.0 | Deferred per MVP-first decision (design doc §8); only 5 candidates in universe, scanner is overkill |
| Auto-promotion of candidates to live | Human approval required in v2.0; auto-promotion deferred to (D) upgrade gated on live validation |
| OKX + BingX in Strategy 4 v1 | Deferred; not needed for MVP which focuses on 4-exchange (Binance/Bybit/Bitget/Gate) edge |
| Capital allocator integration v1 | Strategy 4 gets separate budget in v2.0; unification deferred to v2.1 |
| Funding cost integration | MVP uses 4h max-hold; funding cycle rarely hits. Model in (D) upgrade if half-lives lengthen |
| BingX for spot-futures | No margin/borrow support (carried from v1.0 out-of-scope) |
| Binance Portfolio Margin | Reverted v0.20.1 — too complex for limited benefit (carried from v1.0) |

## Traceability

Which phases cover which requirements. Populated by roadmapper.

| Requirement | Phase | Status |
|-------------|-------|--------|
| PG-01 | Phase 8 | Pending |
| PG-02 | Phase 8 | Pending |
| PG-03 | Phase 8 | Pending |
| PG-04 | Phase 8 | Complete |
| PG-05 | Phase 8 | Complete |
| PG-RISK-01 | Phase 8 | Pending |
| PG-RISK-02 | Phase 8 | Pending |
| PG-RISK-03 | Phase 8 | Complete |
| PG-RISK-04 | Phase 8 | Pending |
| PG-RISK-05 | Phase 8 | Pending |
| PG-OPS-01 | Phase 9 | Pending |
| PG-OPS-02 | Phase 9 | Pending |
| PG-OPS-03 | Phase 9 | Pending |
| PG-OPS-04 | Phase 9 | Pending |
| PG-OPS-05 | Phase 9 | Pending |
| PG-OPS-06 | Phase 8 | Complete |
| PG-VAL-01 | Phase 9 | Pending |
| PG-VAL-02 | Phase 9 | Pending |

**Coverage:**
- v2.0 requirements: 18 total
- Mapped to phases: 18 / 18 ✓
- Phase 8: 11 requirements (PG-01..05, PG-RISK-01..05, PG-OPS-06)
- Phase 9: 7 requirements (PG-OPS-01..05, PG-VAL-01..02)
- Unmapped: 0

---
*Requirements defined: 2026-04-21*
*Last updated: 2026-04-21 — traceability populated by roadmapper (2 phases, 100% coverage)*
