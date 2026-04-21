# Phase 8: Price-Gap Tracker Core - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-21
**Phase:** 08-price-gap-tracker-core
**Mode:** discuss (interactive, user selected "all" areas)
**Areas discussed:** Kline data path, Entry leg reconciliation, Candidate schema & direction, Margin mode & budget, Exec-quality disable persistence, Detection state on restart

---

## A. Kline data path

| Option | Description | Selected |
|--------|-------------|----------|
| A1 | REST poll every 30s both legs per candidate | |
| A2 | REST poll every 10s (fresher, more API pressure) | |
| A3 | WebSocket 1m-kline subscriptions on both legs | |
| A4 | Hybrid: REST poll for spread + WS last-trade/ticker as freshness/halt heartbeat | ✓ |

**User's choice:** A4 — Hybrid
**Notes:** Uses existing per-adapter WS ticker subscriptions (already wrapped) as independent freshness + halt heartbeat, while REST polls provide the canonical 1m kline close for spread math. Poll cadence retained from A1 recommendation: 30s (see D-05).

---

## B. Entry leg reconciliation (partial fill)

| Option | Description | Selected |
|--------|-------------|----------|
| B1 | Unwind over-filled leg down to match smaller fill (simplest, delta-neutral, smaller-than-intended) | ✓ |
| B2 | Top-up under-filled leg with second IOC + market fallback (reuses `engine.go:3022 retrySecondLeg`) | |
| B3 | Abort — market-close both legs immediately | |

**User's choice:** B1 — Unwind-to-match
**Notes:** Deliberate "tracker not engine" choice per design doc §8. Simpler than the existing engine's retry logic; accepts smaller positions rather than adding complexity for the MVP.

---

## C. Candidate schema & direction

| Option | Description | Selected |
|--------|-------------|----------|
| C1 | Pinned direction per candidate (e.g. long gate, short binance — matches measured Phase 0/1 edge) | ✓ |
| C2 | Bi-directional (|spread| ≥ T fires either side) | |
| C3 | Explicit two tuples when both directions valid | |

**User's choice:** C1 — Pinned direction
**Notes:** Phase 0/1 showed the Gate-lag signal is directional; bi-directional exposes to reverse dislocations that weren't validated. Operators who want both directions just add two candidate rows.

---

## D. Margin mode & budget

### D1. Margin mode

| Option | Description | Selected |
|--------|-------------|----------|
| D1-Cross | Share cross-margin pool with existing perp-perp | ✓ |
| D1-Isolated | Isolated margin per price-gap position | |

**User's choice:** D1-Cross
**Notes:** No new margin-mode code path. Isolation is via per-candidate + Gate concentration + budget caps, not via exchange margin mode. Existing L0–L5 HealthMonitor already observes tracker exposure.

### D2. Budget enforcement

| Option | Description | Selected |
|--------|-------------|----------|
| D2-PreTrade | Sum of pre-trade requested notional across open positions | ✓ |
| D2-Realized | Realized post-fill notional |  |

**User's choice:** D2-PreTrade
**Notes:** Deterministic math at the gate. PG-RISK-01 / PG-RISK-05 both become simple addition; partial fills do not retroactively free budget until the position closes.

---

## E. Exec-quality disable persistence (PG-RISK-03)

| Option | Description | Selected |
|--------|-------------|----------|
| E1 | Redis-backed disable flag + `cmd/pg-admin` helper binary for re-enable | ✓ |
| E2 | Hand-edit `config.json` | |
| E3 | Redis + log WARN + Telegram (Telegram is Phase 9) | |

**User's choice:** E1 — Redis + `cmd/pg-admin`
**Notes:** CLAUDE.local.md forbids hand-editing `config.json`. Phase 9 replaces the helper binary with a dashboard toggle — Phase 8 ships the helper so operators can re-enable candidates before Phase 9 lands.

---

## F. Detection state on restart

| Option | Description | Selected |
|--------|-------------|----------|
| F1 | Reset 4-bar window on restart (in-memory, wait 4 fresh bars) | ✓ |
| F2 | Persist bar window to Redis | |

**User's choice:** F1 — Reset
**Notes:** Safer against stale signals after crash. Open positions still rehydrate from Redis; only the *detection state* resets. Minimal MVP behavior.

---

## Claude's Discretion

Captured in CONTEXT.md `<decisions>` "Claude's Discretion" subsection:
- Goroutine topology inside tracker
- Telemetry / log-line formats
- Interface shape for DI
- `cmd/pg-admin/` UX
- Per-adapter REST kline endpoint naming audit

## Deferred Ideas

Captured in CONTEXT.md `<deferred>` section — scanner, state machine, correlation clusters, OKX/BingX inclusion, capital allocator integration, funding-cost handling, persistent bar history, dashboard/paper/Telegram (Phase 9).
