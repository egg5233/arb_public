# Phase 9: Price-Gap Dashboard & Paper→Live Operations — Research

**Researched:** 2026-04-22
**Domain:** Dashboard integration (Go REST+WS + React/Vite), paper-mode branching in `internal/pricegaptrader/`, Telegram notifier extension, on-demand rolling-metrics aggregation, realized-vs-modeled slippage persistence
**Confidence:** HIGH (every claim verified by reading the actual file referenced; no training-data speculation)

---

## Summary

Phase 9 wraps the already-shipped Phase 8 `internal/pricegaptrader/` subsystem with operator-facing surfaces: a new React page, three new Telegram methods, a paper-mode branch at one chokepoint in `execution.go`, and an on-demand metrics aggregator that reads `pg:history` + `pg:slippage:*`. Every integration point exists in the codebase today — no new libraries, no new transports. The UI-SPEC (approved) locks all visual decisions; this research validates technical feasibility and surfaces the Phase 8 schema gaps that must be closed before Phase 9 planning.

**Primary recommendation:** Follow the Phase 8 D-series decisions verbatim — same Redis key shapes, same nil-safe notifier pattern, single chokepoint for paper mode in `openPair` / `closePair`. The one material gap is the `SlippageSample` schema: it stores `(Realized, Modeled)` scalars but lacks `ModeledBps` and per-trade `PositionID + CandidateID` joinability needed for the Closed Log "Modeled vs Realized" column. Plan must either (a) extend `SlippageSample` or (b) read `pg:history` (which already carries `ModeledSlipBps` + `RealizedSlipBps` on each `PriceGapPosition`) as the primary source for Closed Log and Rolling Metrics.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Dashboard Layout**
- **D-01** One new top-level page `Price-Gap`. Single scroll: Status bar → Candidates → Live Positions → Closed Log → Rolling Metrics. No sibling PG Positions / PG History pages.
- **D-02** Dense tables only; no cards, sparklines, or detail drawer in v1.
- **D-03** Live Positions columns: Symbol, Long-exch, Short-exch, Size, Entry spread (bps), Current spread (bps), Hold, PnL (USDT + bps), Paper/Live badge. No distance-to-exit / executional detail / risk-gate reference in v1.
- **D-04** Closed Log columns: Symbol, Long/Short, Size, Entry spread, Exit spread, Hold duration, Reason, Realized bps, Modeled bps, Delta (realized − modeled) bps, PnL USDT, Mode. Default sort: close-time desc.

**Real-Time Updates**
- **D-05** Extend WS hub with `pg_positions` (full active list), `pg_event` (entry/exit/auto-disable), `pg_candidate_update`. REST seeds, WS deltas. No SSE, no polling fallback.
- **D-06** Frontend follows existing `Positions.tsx` pattern: REST seed on mount, WS handlers patch state.

**Paper Mode (PG-OPS-04)**
- **D-07** Paper toggle in Price-Gap page header, next to master enable. Not in Config tab.
- **D-08** Paper positions distinguished by `PAPER` badge in Symbol column + soft row tint. Single table, single codepath.
- **D-09** Paper persists to same Redis keys as live (`pg:positions`, `pg:positions:active`, closed set, `pg:slippage:*`). Each position carries `mode: "paper" | "live"` stamped at entry.
- **D-10** Paper mode runs full event detection, risk-gate evaluation, entry/exit decision logic — only `PlaceOrder` is skipped. Fills synthesized at mid. Realized slippage for paper computed identically to live.
- **D-11** Paper→Live transition manual-only. No auto-promote. Existing paper positions keep `mode: "paper"` through close.
- **D-12** Config surface: `PriceGapPaperMode bool` (default `true`). Round-trips via POST `/api/config` + `.bak`. Never written by anything else.

**Candidate Disable / Re-enable**
- **D-13** Disabled rows greyed inline with `Disabled: <reason>` + timestamp + inline Re-enable button. Not hidden.
- **D-14** Re-enable gated by confirmation modal showing reason + disabled-at.
- **D-15** Dashboard manual disable with optional reason text field (defaults `"manual"`). Writes `pg:candidate:disabled:<symbol>` identically to `pg-admin disable`.
- **D-16** No disable history UI in this phase.
- **D-17** Dashboard re-enable path equivalent to `pg-admin enable <symbol>`. `pg-admin` CLI remains supported.

**Telegram (PG-OPS-05)**
- **D-18** Three methods on `*TelegramNotifier`, nil-safe:
  - `NotifyPriceGapEntry(pos *PriceGapPosition)` — no cooldown
  - `NotifyPriceGapExit(pos *PriceGapPosition, reason string, pnl float64, duration time.Duration)` — no cooldown
  - `NotifyPriceGapRiskBlock(symbol, gate, detail string)` — cooldown keyed `pg_risk:<gate>:<symbol>`, reusing `checkCooldown`
- **D-19** Entry content: symbol, long/short exchs, size, entry spread bps, modeled edge bps, paper/live tag.
- **D-20** Exit content: symbol, exit reason, PnL USDT + bps, realized slippage bps, hold duration, paper/live tag.
- **D-21** Risk-block content: symbol, gate name (`concentration`, `max_concurrent`, `kline_stale`, `delist`, `budget`, `exec_quality`), short detail.
- **D-22** Alerts fire in Paper mode too, prefixed `📝 PAPER`.

**Slippage & Metrics (PG-VAL-01/02)**
- **D-23** Confirm Phase 8 writes per-trade realized slippage at entry AND exit (round-trip). If only aggregate-ratio persists, extend `pg:slippage:*` to carry `{position_id, candidate, entry_ts, exit_ts, modeled_bps, realized_entry_bps, realized_exit_bps, realized_total_bps, pnl_usdt, mode}`.
- **D-24** Rolling metrics computed on-demand from closed set + `pg:slippage:*`. No background rollup.
- **D-25** Metrics columns: Candidate, Trades (window), Win %, Avg realized bps, 24h bps/day, 7d bps/day, 30d bps/day. Sortable. One row per candidate.

### Claude's Discretion

- i18n key names and exact English + zh-TW copy (must be in lockstep).
- Row tint / badge / button colors — already locked in UI-SPEC as `violet-400`, `violet-500/5`, `violet-500/15`, `yellow-500/5`, `yellow-600/20`.
- Empty-state copy — locked in UI-SPEC.
- Default pagination / row cap on closed log — UI-SPEC locks 100 rows default, 500 cap.
- Default sort direction per table — UI-SPEC locks each.
- Loading skeleton — UI-SPEC locks 3-row `animate-pulse` per `<tbody>`.
- Modal component — UI-SPEC locks hand-rolled `fixed inset-0` matching `Positions.tsx` L602–630.
- REST seed shape — UI-SPEC locks single aggregate `GET /api/pricegap/state` + per-section endpoints for re-seed.
- Candidate enabled/disabled toggle — UI-SPEC locks Redis-only (`pg:candidate:disabled:<symbol>`), identical to `pg-admin` semantics.

### Deferred Ideas (OUT OF SCOPE)

- Per-candidate sparkline charts.
- Promote/demote "Suggested action" column (threshold-based recommendation).
- Disable history UI (last N toggles per candidate).
- Auto paper→live transition after N clean days.
- Force-close-all-paper button.
- Detail drawer per candidate / position.
- Separate PG Positions / PG History pages.
- SSE / EventSource transport.
- Redis Streams + background rollup for metrics.
- Exec-quality auto-disable Telegram alert.
- Paper↔live toggle-flip Telegram alert.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PG-OPS-01 | Dashboard Price-Gap tab with per-candidate enable/disable toggle | `App.tsx` Page union + renderPage switch pattern (L21, L246); inline `pg:candidate:disabled:<symbol>` Redis ops identical to `cmd/pg-admin` |
| PG-OPS-02 | Live positions: entry/current spread, hold, PnL | `PriceGapPosition` model carries all required fields (`EntrySpreadBps`, `LongFillPrice/ShortFillPrice`, `OpenedAt`, `RealizedPnL`); `GetActivePriceGapPositions` returns the active set; WS hub `Broadcast("pg_positions", ...)` delivers them incrementally |
| PG-OPS-03 | Closed positions log with realized-vs-modeled edge | `PriceGapPosition.RealizedSlipBps` populated on close (monitor.go L212); `ModeledSlipBps` stamped at entry (execution.go L142); `AddPriceGapHistory` writes to `pg:history` (LIST cap 500 per Phase 8 D-14) |
| PG-OPS-04 | Paper mode: full logic, no order placement, modeled fills | Single chokepoint: `openPair` calls `placeLeg` (execution.go L154) and `closePair` calls `placeCloseLegIOC` (monitor.go L249). Wrap both with `if t.cfg.PriceGapPaperMode { synthesize fill }`. Existing circuit breaker, lock, persistence, monitor startup all stay identical |
| PG-OPS-05 | Telegram on entry / exit / risk-gate block, reuse notifier | `*TelegramNotifier` nil-safe pattern established (telegram.go L75..167); `checkCooldown(eventKey)` (L196–213) keyed cooldowns already support the `pg_risk:<gate>:<symbol>` convention |
| PG-VAL-01 | Per-trade realized-vs-modeled slippage logging | `SlippageSample{PositionID, Realized, Modeled, Timestamp}` exists; `AppendSlippageSample` + `GetSlippageWindow` implemented. **Gap: sample lacks `CandidateID` foreign key and `ExitTS / EntryTS` breakdown — planner must decide whether to extend sample schema or read `pg:history` directly for the closed-log Modeled/Realized columns.** See "Phase 8 Dependencies" below. |
| PG-VAL-02 | Rolling 7d/30d bps/day per candidate | On-demand aggregator: read `pg:history` (already keyed with `ClosedAt`, `RealizedPnL`, `NotionalUSDT`, `Symbol+LongExch+ShortExch`), compute window, divide bps PnL by window days. No new write path. |
</phase_requirements>

## Project Constraints (from CLAUDE.md / CLAUDE.local.md)

| Directive | Phase 9 impact |
|-----------|---------------|
| **npm lockdown** — no `npm install`, `npm update`, `npx`; only `npm ci` from `web/package-lock.json` | No new React/Tailwind libraries. Hand-rolled components only. UI-SPEC confirms this. |
| **Never modify `config.json`** directly | Only `POST /api/config` writes it, with `.bak` backup. The PaperMode toggle must use this authorised path. |
| **Build order** — `web/` build FIRST, then Go | Plans that change frontend MUST run `npm run build` before `go build`; otherwise `go:embed` serves stale JS. Verification step in every plan. |
| **i18n lockstep** — `en.ts` AND `zh-TW.ts` updated together | Every new `pricegap.*` key added to both files in the same commit. `TranslationKey` is derived from `en.ts` so a missing zh-TW entry compiles but renders the English fallback per `index.ts` L20. |
| **Dashboard API envelope** `{ ok, data?, error? }` via `writeJSON` | All new `/api/pricegap/*` handlers use the same helper. |
| **Bearer token auth** via `localStorage[arb_token]` | New REST endpoints go through `s.cors(s.authMiddleware(...))` chain in `server.go`. |
| **Module boundary** — `internal/pricegaptrader/` must NOT import `internal/engine/` or `internal/spotengine/` | Any new metrics-aggregator or paper-mode code lives inside `internal/pricegaptrader/` or in `internal/api/` (handler layer), never crossing into the other two engines. |
| **Nil-safe `*TelegramNotifier`** — methods safe on nil receiver | All three new `NotifyPriceGap*` methods begin with `if t == nil { return }`, matching existing pattern (telegram.go L145–147, L169–171). |
| **New-feature rollout pattern** — config bool (default OFF) + dashboard toggle + `config.json` persistence | `PriceGapPaperMode` defaults to **TRUE** (per D-12 "safe until validated") — this is an explicit exception to the usual default-OFF rule and is justified in CONTEXT.md. |
| **Versioning** — `VERSION` + `CHANGELOG.md` updated every commit | Every Phase 9 plan bumps both. |
| **Live trading risk** — no change to perp-perp or spot-futures paths | New Telegram methods added alongside existing ones. Notifier instance reused, no new configuration keys for perp or spot-futures notification gating. |

## Standard Stack

Phase 9 adds **zero** new dependencies. Every component is already in `go.mod` or `web/package.json`. Verification: `head -n 30 go.mod` confirms Go 1.26+, `web/package.json` confirms React 18 + Vite + Tailwind.

### Core (already installed — reused verbatim)

| Component | Location | Purpose | Why Standard |
|-----------|----------|---------|--------------|
| WebSocket hub (`hub.Broadcast(topic, payload)`) | `internal/api/server.go` L62, L216–258 | Push pg_positions / pg_event / pg_candidate_update deltas | Same one-liner pattern used by `BroadcastPositionUpdate`, `BroadcastOpportunities`, `BroadcastStats`, `BroadcastAlert`, `BroadcastLossLimits`, `BroadcastRejection` |
| `writeJSON(w, status, Response{})` envelope | `internal/api/handlers.go` | Standard `{ ok, data?, error? }` response | Every existing handler uses it; planner does not re-design |
| `*TelegramNotifier` with `checkCooldown(key)` | `internal/notify/telegram.go` L196–213 | Nil-safe notifier, per-key cooldown (default 1h) | Existing `NotifySLTriggered`, `NotifyEmergencyClosePerp`, `NotifyConsecutiveAPIErrors` all follow this pattern |
| Redis (DB 2) via `internal/database/pricegap_state.go` | `SavePriceGapPosition`, `GetActivePriceGapPositions`, `AddPriceGapHistory`, `IsCandidateDisabled`, `SetCandidateDisabled`, `ClearCandidateDisabled`, `AppendSlippageSample`, `GetSlippageWindow` | All CRUD for dashboard REST + WS | Phase 8 shipped these; `pg-admin` CLI and new dashboard handlers share them verbatim |
| `PriceGapPosition` model (`internal/models/pricegap_position.go`) | `ID, Symbol, LongExchange, ShortExchange, Status, EntrySpreadBps, ThresholdBps, NotionalUSDT, LongFillPrice, ShortFillPrice, LongSize, ShortSize, LongMidAtDecision, ShortMidAtDecision, ModeledSlipBps, RealizedSlipBps, RealizedPnL, ExitReason, OpenedAt, ClosedAt` | Payload for all pg_* WS topics + REST responses | Existing JSON-tagged struct, directly serialisable |
| `TranslationKey` type + `useLocale()` hook | `web/src/i18n/index.ts` | Compile-time-safe i18n | Adding keys to `en.ts` updates the type automatically |
| `formatAge`, `formatDateTime`, `pnlColor` helpers | `web/src/pages/Positions.tsx` | Re-used by new page | UI-SPEC explicitly locks reuse |
| Modal pattern `fixed inset-0 bg-black/50` | `Positions.tsx` L602–630 | Disable/Re-enable confirmation modals | Hand-rolled, no new library |

### Supporting

| Component | Purpose | When to Use |
|-----------|---------|-------------|
| `AcquirePriceGapLock` / `ReleasePriceGapLock` (Redis SET NX) | Per-symbol distributed lock | Paper mode still acquires the lock for determinism — don't bypass |
| `sampleLegs` + `computeSpreadBps` (tracker.go helpers) | Current-spread computation for Live Positions "Current spread" column | Call from `/api/pricegap/state` handler at request time to provide fresh BBO-derived spread per open position |
| `exchange.Exchange.GetBBO` | BBO used for paper-mode synthetic fill price | Mid = (bid+ask)/2 — already how `det.MidLong / det.MidShort` is populated in detector.go |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| On-demand metrics aggregation from `pg:history` (D-24) | Pre-aggregated sorted set written on close | Faster query but new write path + rebuild tool; data volume (~hundreds of trades per candidate) does not justify complexity. User chose on-demand. |
| WS topics for pg_* | REST polling every 5s | Polling is inconsistent with every other live-data page. WS hub one-liner already exists. |
| Extending `SlippageSample` to carry all closed-log fields | Reading `pg:history` directly for closed-log rows | `pg:history` already carries `ModeledSlipBps` + `RealizedSlipBps` + `RealizedPnL` on each position — zero schema change needed. Slippage sample can stay lean as exec-quality-gate input. **Recommendation: read `pg:history` for closed log, use `pg:slippage:*` for the auto-disable rolling window only.** |

**Installation:** *None required.* Phase 9 ships on existing dependencies.

**Version verification:** not applicable — no new packages.

## Architecture Patterns

### Recommended Module Layout

```
internal/pricegaptrader/
├── execution.go          # MODIFY: add paper-mode branch in placeLeg + placeCloseLegIOC
├── monitor.go            # MODIFY: same paper-mode branch + pos.Mode stamped at entry
├── notify.go             # NEW: hook points invoked on entry/exit/risk-block; lives inside pricegaptrader to keep module-boundary rule intact
├── metrics.go            # NEW: on-demand rolling-window aggregator (pure function; reads models)
└── ...existing files

internal/notify/
└── telegram.go           # MODIFY: add NotifyPriceGapEntry / Exit / RiskBlock

internal/api/
├── pricegap_handlers.go  # NEW: GET /api/pricegap/state, POST /api/pricegap/candidate/{sym}/disable|enable, sub-section re-seed endpoints
├── server.go             # MODIFY: register /api/pricegap/* routes, add BroadcastPriceGap{Positions,Event,CandidateUpdate}
└── config_handlers.go    # MODIFY: extend jsonPriceGap with paper_mode flag (already present as struct per config.go L304)

internal/config/
└── config.go             # MODIFY: add PriceGapPaperMode bool + default TRUE (D-12 exception to default-OFF rule)

web/src/
├── App.tsx               # MODIFY: Page union adds 'price-gap'; nav registration between 'spot-positions' and 'history'
├── pages/PriceGap.tsx    # NEW: single file; split to PriceGap/{Header,Candidates,LivePositions,ClosedLog,Metrics}.tsx only if >600 lines
└── i18n/{en,zh-TW}.ts    # MODIFY: add all pricegap.* keys in lockstep
```

### Pattern 1: WS Broadcast Extension

**What:** One-line `s.hub.Broadcast("topic", payload)` method on `*Server`.
**When to use:** Every new real-time push to dashboard.

```go
// Source: verified against internal/api/server.go L216–258

// BroadcastPriceGapPositions pushes the full active list to all WS clients.
func (s *Server) BroadcastPriceGapPositions(positions []*models.PriceGapPosition) {
    s.hub.Broadcast("pg_positions", positions)
}

// BroadcastPriceGapEvent pushes a single entry/exit/auto-disable event.
// Event shape: { type: "entry"|"exit"|"auto_disable", position?: PriceGapPosition, symbol?: string, reason?: string }
func (s *Server) BroadcastPriceGapEvent(evt PriceGapEvent) {
    s.hub.Broadcast("pg_event", evt)
}

// BroadcastPriceGapCandidateUpdate pushes a single candidate's state change.
func (s *Server) BroadcastPriceGapCandidateUpdate(update PriceGapCandidateUpdate) {
    s.hub.Broadcast("pg_candidate_update", update)
}
```

Tracker publishes by holding a `Broadcaster` interface (avoid importing `internal/api` into `internal/pricegaptrader`):

```go
// Inside internal/pricegaptrader, new notify.go
type Broadcaster interface {
    BroadcastPriceGapPositions(positions []*models.PriceGapPosition)
    BroadcastPriceGapEvent(evt PriceGapEvent)
    BroadcastPriceGapCandidateUpdate(update PriceGapCandidateUpdate)
}
// Tracker holds this via DI; cmd/main.go wires &server.
```

### Pattern 2: Paper-Mode Chokepoint

**What:** Single conditional inside `placeLeg` and `placeCloseLegIOC` — no duplicated decision/monitor/exit code.
**When to use:** Every path that calls `ex.PlaceOrder`.

```go
// Source: pattern derived from internal/pricegaptrader/execution.go L154–175

func (t *Tracker) placeLeg(
    ex exchange.Exchange, symbol string, side exchange.Side,
    sizeBase float64, decimals int, force string, fillPrice float64,
) fillResult {
    params := exchange.PlaceOrderParams{
        Symbol: symbol, Side: side, OrderType: "market",
        Size: strconv.FormatFloat(roundStep(sizeBase, decimals), 'f', decimals, 64),
        Force: force,
    }
    // D-10 paper-mode branch: synthesize fill, skip exchange call.
    if t.cfg.PriceGapPaperMode {
        return fillResult{
            orderID: fmt.Sprintf("paper_%s_%d", symbol, time.Now().UnixNano()),
            filled:  roundStep(sizeBase, decimals),
            price:   fillPrice, // mid-at-decision = modeled fill per D-10
        }
    }
    orderID, err := ex.PlaceOrder(params)
    // ...rest unchanged
}
```

Same pattern applied to `placeCloseLegIOC` and `closeLegMarket`.

### Pattern 3: Nil-safe Notifier Methods

```go
// Source: verified against internal/notify/telegram.go L144–172

func (t *TelegramNotifier) NotifyPriceGapEntry(pos *models.PriceGapPosition) {
    if t == nil { return }
    prefix := ""
    if pos.Mode == "paper" {
        prefix = "📝 PAPER "
    }
    text := fmt.Sprintf("%sPRICE-GAP ENTRY\nSymbol: %s\nLong: %s  Short: %s\nSize: %.4f  Notional: $%.2f\nEntry: %.1f bps  Modeled: %.1f bps",
        prefix, pos.Symbol, pos.LongExchange, pos.ShortExchange,
        pos.LongSize, pos.NotionalUSDT, pos.EntrySpreadBps, pos.ModeledSlipBps)
    t.send(text)
}

func (t *TelegramNotifier) NotifyPriceGapRiskBlock(symbol, gate, detail string) {
    if t == nil { return }
    if !t.checkCooldown("pg_risk:" + gate + ":" + symbol) { return }
    t.send(fmt.Sprintf("PRICE-GAP RISK BLOCK\nSymbol: %s  Gate: %s\n%s", symbol, gate, detail))
}
```

### Pattern 4: On-Demand Metrics Aggregation

```go
// Source: pattern adapted from analytics_handlers.go (rolling aggregates)

type CandidateMetrics struct {
    Candidate      string  `json:"candidate"`        // "<symbol>_<long>_<short>"
    Symbol         string  `json:"symbol"`
    TradesWindow   int     `json:"trades_window"`    // count over max(window)
    WinPct         float64 `json:"win_pct"`
    AvgRealizedBps float64 `json:"avg_realized_bps"`
    Bps24hPerDay   float64 `json:"bps_24h_per_day"`
    Bps7dPerDay    float64 `json:"bps_7d_per_day"`
    Bps30dPerDay   float64 `json:"bps_30d_per_day"`
}

// Pure function; inputs = []PriceGapPosition (from pg:history, already sorted newest first).
// Groups by (symbol, long, short), filters by ClosedAt > now - window, sums (RealizedPnL / NotionalUSDT * 10000), divides by window days.
```

### Anti-Patterns to Avoid

- **Dual write of candidate disable state** — do NOT add a config.json bool for per-candidate enable/disable alongside the Redis flag. Two sources of truth drift within 24 hours. UI-SPEC locks Redis-only.
- **Re-implementing the modal component** — UI-SPEC locks the inline `fixed inset-0 bg-black/50` pattern from `Positions.tsx` L602–630. Do not introduce `@radix-ui` or any dialog library (npm lockdown anyway).
- **Polling `/api/pricegap/positions` on a 5s timer as a fallback** — UI-SPEC D-05 rules this out. WS is the only live update transport. An error banner (`pricegap.warn.wsDisconnected`) is enough.
- **Putting paper-mode toggle in Config tab** — D-07 is explicit: header only. Operators need to see the mode during the 3-day validation.
- **Splitting paper and live into separate tables** — D-08 requires single-table unified view with row-level badge.
- **Writing Phase 9 code paths that depend on `internal/engine` or `internal/spotengine`** — violates the module boundary rule from Phase 8. The metrics aggregator and Broadcaster live inside `internal/pricegaptrader/` (or in `internal/api/` handler layer at the outermost edge).
- **Stamping `pos.Mode` outside `openPair`** — `mode` must be set at entry once and immutable afterward (D-11). The string lives on `PriceGapPosition` and survives through Redis round-trip.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| WS client reconnection | Custom reconnect loop | Existing WS pattern in `Positions.tsx` | Handles dev/prod HTTPS switching, auth token refresh |
| Modal component | New library / `<dialog>` polyfill | `Positions.tsx` L602 inline modal | UI-SPEC locked; no new npm packages allowed |
| Rolling-metrics store | Redis Streams or sorted set writes on close | On-demand read of `pg:history` | Data volume trivial; D-24 locks on-demand; cuts one plan's worth of write-path complexity |
| Per-trade modeled-vs-realized join | New FK in SlippageSample | `pg:history` already carries both | `PriceGapPosition.ModeledSlipBps` (execution.go L142) + `RealizedSlipBps` (monitor.go L212) are both persisted |
| Telegram cooldown | New `time.Time` map per method | `checkCooldown("pg_risk:<gate>:<sym>")` | Existing infra in telegram.go L196 |
| i18n type-safety | Add runtime assertion | `TranslationKey` derived from `en.ts` | Type system already enforces; missing zh-TW keys fall through to English per `index.ts` L20 |

**Key insight:** Phase 8 already persisted everything the Closed Log needs. The primary risk in Phase 9 is *duplicating* persistence paths rather than *missing* them. Read-only aggregation over `pg:history` is the lightest-weight path to PG-OPS-03, PG-VAL-01, PG-VAL-02.

## Runtime State Inventory

> N/A — greenfield operator surface. No rename/refactor/migration involved. Phase 9 adds new Redis keys (if any — see `mode` field stamping below) without renaming existing keys. The only schema migration risk is the `PriceGapPosition.Mode string` field addition: existing rehydrated positions with missing `mode` must default to `"live"`.

**Nothing found in category:** Stored data, live service config, OS-registered state, secrets/env vars, build artifacts — verified by grep. The only in-place schema extension is `PriceGapPosition.Mode` (default `"live"` when absent).

## Common Pitfalls

### Pitfall 1: `PriceGapPaperMode` default

**What goes wrong:** If default follows the project rollout rule (default OFF), operators starting Phase 9 without explicit flag flip run LIVE trades immediately.
**Why it happens:** The rollout convention says "default OFF". D-12 overrides this specifically — paper is safe, live is not.
**How to avoid:** `PriceGapPaperMode bool` default **TRUE** in `config.Defaults()`, with a prominent comment citing D-12. Flag the divergence in the plan.
**Warning signs:** Fresh `config.json` without `price_gap.paper_mode` ending up in live mode on first run.

### Pitfall 2: Mode flip mid-life

**What goes wrong:** Operator flips Paper→Live while a paper position is open; WS handlers retag live, fills become a mix.
**Why it happens:** Mode read from global config instead of per-position.
**How to avoid:** Stamp `pos.Mode` at entry in `openPair` (execution.go L126–144). Monitor, close, notifier, and dashboard all read `pos.Mode`, never `cfg.PriceGapPaperMode` directly. D-11 explicit.
**Warning signs:** Closed Log row with `mode="live"` but realized fill price equals `LongMidAtDecision` (suggests synthesized fill was mistagged).

### Pitfall 3: WS broadcast storm on rehydrate

**What goes wrong:** Startup rehydrates N positions; each `startMonitor` triggers one broadcast per position, flooding the hub.
**Why it happens:** Naive implementation fires `BroadcastPriceGapEvent` from `rehydrate()`.
**How to avoid:** `rehydrate()` does NOT broadcast. Dashboard clients pick up the full state via the next `pg_positions` full-snapshot push (scheduled every N seconds or on first state change after rehydrate completes).
**Warning signs:** Dashboard connection drops during tracker startup.

### Pitfall 4: SlippageSample ↔ PriceGapPosition join missing

**What goes wrong:** Closed Log wants to show Modeled bps + Realized bps per row; dev writes new code to read both `pg:history` and `pg:slippage:<candID>` and join them — then one row is in history but not yet in slippage (race).
**Why it happens:** Two sources of truth for the same datum.
**How to avoid:** `PriceGapPosition` already carries BOTH `ModeledSlipBps` and `RealizedSlipBps`. Closed Log reads ONLY `pg:history`. `pg:slippage:*` remains the exec-quality-gate input exclusively (scoped to `recordSlippageAndMaybeDisable`, slippage.go L32–77).
**Warning signs:** New `GetSlippageSampleByPositionID` API method appearing in the plan.

### Pitfall 5: i18n drift

**What goes wrong:** New `pricegap.*` key added to `en.ts` only; `zh-TW.ts` renders the English literal because of the fallback in `index.ts` L20.
**Why it happens:** `TranslationKey` is derived from `en.ts`, so the compiler accepts a zh-TW file missing keys without error.
**How to avoid:** Each plan commit adds keys to both files. Add a diff-time check in the plan verification: `diff <(grep "^\s*'pricegap\." web/src/i18n/en.ts) <(grep "^\s*'pricegap\." web/src/i18n/zh-TW.ts) | wc -l` should be 0.
**Warning signs:** English string showing in the Chinese UI.

### Pitfall 6: Candidate toggle drift between CLI and dashboard

**What goes wrong:** `pg-admin disable` sets `pg:candidate:disabled:<sym> = "manual"`; dashboard writes `{reason:"manual", disabled_at:1234567890}` JSON.
**Why it happens:** Two writers disagree on value shape.
**How to avoid:** `SetCandidateDisabled(symbol, reason string)` is the ONLY writer. Dashboard handler wraps the exact call with the text-field reason (or `"manual"` default). Value shape stays plain string per Phase 8 Redis semantics. Reason + disabled-at composite: store disabled-at via `SetEx` TTL or a sibling key `pg:candidate:disabled_at:<sym>` — **plan must pick one**. Simpler: the `reason` string can be `"manual @ 2026-04-22T12:34:56Z"` (CLI emits this too).
**Warning signs:** Dashboard JSON parse errors when reading a CLI-disabled candidate.

### Pitfall 7: Paper-mode realized-slippage falls to zero

**What goes wrong:** Paper fill price = mid, so realized_bps = 0. Closed Log shows every paper trade at zero slippage. Operator loses trust in the instrumentation.
**Why it happens:** `closePair` computes realized slippage from actual fill vs mid (monitor.go L202–212). If paper synthesizes fill at mid, the delta is always 0.
**How to avoid:** In paper mode, synthesize fills at `mid ± modeled_slip_bps / 2` on each side. This exercises the slippage pipeline with a non-zero realized number. Alternatively, document explicitly that paper-mode realized bps is always 0 and the Closed Log shows only Modeled + Delta. **Planner must pick.** Recommendation: synthesize at `mid ± modeled/2` — gives the operator realistic Closed Log rows before live money lands.
**Warning signs:** Every paper row in Closed Log has `realized_bps=0` and `delta_bps = -modeled`.

### Pitfall 8: Rebuild forgotten

**What goes wrong:** Plan edits `PriceGap.tsx`, commits, runs `go build`. Binary serves the old embedded SPA.
**Why it happens:** Frontend must build first (`npm run build` → `web/dist/`) then `go build` picks up `go:embed`.
**How to avoid:** Every plan that touches `web/` ends with explicit `make build` (or `npm run build && go build`). Verification step runs `curl /api/version` or visually inspects the new page.
**Warning signs:** `web/dist/` timestamps older than source file timestamps after a deploy.

## Code Examples

### Example 1: Paper-mode synthetic fill (pattern for PG-OPS-04)

```go
// Source: derived pattern from internal/pricegaptrader/execution.go L154
// D-10: paper mode runs full logic, suppresses real PlaceOrder, synthesizes fill.

func (t *Tracker) placeLeg(ex exchange.Exchange, symbol string, side exchange.Side,
    sizeBase float64, decimals int, force string, fillPrice float64,
) fillResult {
    sizeStr := strconv.FormatFloat(roundStep(sizeBase, decimals), 'f', decimals, 64)

    if t.cfg.PriceGapPaperMode {
        // Synthesize adverse-by-half-modeled fill to exercise slippage pipeline (see Pitfall 7).
        adverse := 0.0
        if t.paperModeledEdgeBps > 0 {
            adverse = fillPrice * (t.paperModeledEdgeBps / 2.0) / 10_000.0
        }
        synthesizedPrice := fillPrice
        if side == exchange.SideBuy {
            synthesizedPrice = fillPrice + adverse // buy slightly above mid
        } else {
            synthesizedPrice = fillPrice - adverse // sell slightly below mid
        }
        return fillResult{
            orderID: "paper_" + symbol + "_" + strconv.FormatInt(time.Now().UnixNano(), 10),
            filled:  roundStep(sizeBase, decimals),
            price:   synthesizedPrice,
        }
    }

    orderID, err := ex.PlaceOrder(exchange.PlaceOrderParams{
        Symbol: symbol, Side: side, OrderType: "market", Size: sizeStr, Force: force,
    })
    // ...existing path unchanged
}
```

### Example 2: Telegram notifier extension (pattern for PG-OPS-05)

```go
// Source: mirrors internal/notify/telegram.go L75–143 (NotifyAutoEntry, NotifyAutoExit, NotifyEmergencyClose).

func (t *TelegramNotifier) NotifyPriceGapEntry(pos *models.PriceGapPosition) {
    if t == nil { return }
    tag := "LIVE"
    prefix := ""
    if pos.Mode == "paper" {
        tag = "PAPER"
        prefix = "📝 PAPER "
    }
    text := fmt.Sprintf(
        "%sPRICE-GAP ENTRY [%s]\nSymbol: %s\nLong: %s  Short: %s\nSize: %.4f  Notional: $%.2f\nEntry spread: %.1f bps  Modeled: %.1f bps",
        prefix, tag, pos.Symbol, pos.LongExchange, pos.ShortExchange,
        pos.LongSize, pos.NotionalUSDT, pos.EntrySpreadBps, pos.ModeledSlipBps,
    )
    t.send(text)
}

func (t *TelegramNotifier) NotifyPriceGapExit(pos *models.PriceGapPosition, reason string, pnl float64, duration time.Duration) {
    if t == nil { return }
    tag := "LIVE"
    prefix := ""
    if pos.Mode == "paper" { tag = "PAPER"; prefix = "📝 PAPER " }
    pnlBps := 0.0
    if pos.NotionalUSDT > 0 {
        pnlBps = pnl / pos.NotionalUSDT * 10_000.0
    }
    text := fmt.Sprintf(
        "%sPRICE-GAP EXIT [%s]\nSymbol: %s  Reason: %s\nPnL: $%.2f (%.1f bps)\nRealized slippage: %.1f bps\nHold: %s",
        prefix, tag, pos.Symbol, formatExitReason(reason),
        pnl, pnlBps, pos.RealizedSlipBps, formatDuration(duration),
    )
    t.send(text)
}

func (t *TelegramNotifier) NotifyPriceGapRiskBlock(symbol, gate, detail string) {
    if t == nil { return }
    if !t.checkCooldown("pg_risk:" + gate + ":" + symbol) { return }
    t.send(fmt.Sprintf("PRICE-GAP RISK BLOCK\nSymbol: %s  Gate: %s\n%s", symbol, gate, detail))
}
```

### Example 3: REST seed aggregate handler (pattern for PG-OPS-01..03)

```go
// Source: mirrors internal/api/handlers.go envelope pattern (writeJSON).

func (s *Server) handlePriceGapState(w http.ResponseWriter, r *http.Request) {
    active, err := s.db.GetActivePriceGapPositions()
    if err != nil {
        writeJSON(w, 500, Response{Error: err.Error()}); return
    }
    history, err := s.db.GetPriceGapHistory(500) // Phase 8 D-14 cap
    if err != nil {
        writeJSON(w, 500, Response{Error: err.Error()}); return
    }
    candidates := make([]CandidateView, 0, len(s.cfg.PriceGapCandidates))
    for _, c := range s.cfg.PriceGapCandidates {
        disabled, reason, _ := s.db.IsCandidateDisabled(c.Symbol)
        candidates = append(candidates, CandidateView{
            Candidate: c, Disabled: disabled, DisabledReason: reason,
        })
    }
    metrics := pricegaptrader.ComputeCandidateMetrics(history, time.Now())
    writeJSON(w, 200, Response{OK: true, Data: map[string]interface{}{
        "enabled":          s.cfg.PriceGapEnabled,
        "paper_mode":       s.cfg.PriceGapPaperMode,
        "budget":           s.cfg.PriceGapBudget,
        "candidates":       candidates,
        "active_positions": active,
        "recent_closed":    history,
        "metrics":          metrics,
    }})
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Redis HSET for config | `config.json` sole source of truth | 2026-04-05 | Only `POST /api/config` writes config; PaperMode follows this exclusively |
| Two-source candidate disable (config + Redis) | Redis-only (`pg:candidate:disabled:<sym>`) | Phase 8 D-19/D-20 | `pg-admin` CLI and dashboard both wrap `SetCandidateDisabled` |
| Polling dashboard data | REST seed + WS delta | Pre-v1.0 | Every live page uses this; Phase 9 extends it |

**Deprecated/outdated:** N/A for Phase 9 — no legacy paths to migrate off.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `PriceGapPosition.Mode` field does NOT yet exist on the struct (verified via model grep showing current fields) | Phase 8 Dependencies | Plan must add the field via schema extension with default `"live"` on rehydrate of pre-Phase-9 positions. **Check Phase 8 SUMMARY / git log before planning.** |
| A2 | `GetPriceGapHistory(n int)` exists or can be added trivially alongside `AddPriceGapHistory` | Pattern 4, Example 3 | Verified `AddPriceGapHistory` exists in `pricegap_state.go` L127; reading it back needs a new `LRANGE 0 n-1` method if missing. **Planner verifies.** |
| A3 | `paperModeledEdgeBps` on Tracker is a new derived field | Pitfall 7 | If planner chooses to synthesize fills at mid exactly (realized=0 always), this field is unnecessary. Either path is defensible; document the choice. |
| A4 | Existing `Positions.tsx` WS reconnection already handles token refresh | Don't Hand-Roll | **Verify**: grep for `new WebSocket` returned no matches in the quick scan — the WS hook may be in a shared helper file. Planner must locate and confirm pattern. |
| A5 | No new `pricegap_state.go` Redis keys needed beyond existing ones | Runtime State Inventory | If planner decides to add disabled-at as a sibling key, this adds one key to the namespace — low risk, but document in plan. |

## Open Questions

1. **Should paper-mode synthesized fills include modeled-slippage adverse selection?**
   - What we know: D-10 says "fills synthesized at mid (or the gap-derived quote)". Mid ≡ realized_bps = 0 always.
   - What's unclear: Whether operator wants exec-quality gate to be exercisable in paper mode (requires non-zero realized).
   - Recommendation: Synthesize at `mid ± modeled/2` to validate the full pipeline. Document the choice in plan.

2. **Disabled-at timestamp persistence**
   - What we know: CLI stores only `reason string` via `SetCandidateDisabled(symbol, reason)`; no explicit timestamp.
   - What's unclear: Whether reason-string format can carry the timestamp, or a sibling key is needed.
   - Recommendation: Format reason as `"<human_reason> @ <RFC3339 UTC>"` — dashboard parses it back. No schema change. Alternative: `pg:candidate:disabled:<sym>` JSON `{reason, disabled_at}` + backfill path for CLI-written entries.

3. **Current-spread refresh cadence**
   - What we know: Live Positions "Current spread" column needs fresh BBO-derived spread.
   - What's unclear: Is it computed per-request inside `handlePriceGapState`, or piggybacked on the monitor's existing poll?
   - Recommendation: Handler calls `sampleLegs` per active position at request time; cache 5s in-memory to avoid per-click BBO fetch. WS delta picks up monitor's own `checkAndMaybeExit` samples.

4. **`pg_event` payload schema**
   - What we know: D-05 lists the topic; payload shape undefined.
   - What's unclear: One discriminated-union type vs. three topics.
   - Recommendation: Single struct `PriceGapEvent{Type: "entry"|"exit"|"auto_disable", Position *PriceGapPosition, Symbol string, Reason string}` — frontend switches on `type`. Keeps WS topic count small (3 topics total, honouring D-05).

5. **Closed Log pagination persistence**
   - What we know: UI-SPEC locks 100-row default, 500 cap.
   - What's unclear: Does `pg:history` LRANGE support offset correctly for "Load 100 more"?
   - Recommendation: `GetPriceGapHistory(offset, limit int)` — verify method signature in plan; trivially wraps `LRANGE start stop`.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.26+ | Backend | ✓ | per `go.mod` | — |
| Node v22.13.0 via nvm | Frontend build | ✓ | per project memory | — |
| Redis (DB 2) | pg:* keys | ✓ | Live | — |
| Telegram bot token | `NotifyPriceGap*` | Optional | — | Nil-safe notifier skips send — no-op if unconfigured |
| npm `node_modules` in web/ | Vite build | ✓ — locked | per `package-lock.json` | **Blocking:** `npm ci` only (no `npm install`) per CLAUDE.local.md |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** Telegram (notifier nil-safe; in dev the bot token is typically absent — this is expected).

## Phase 8 Dependencies (must be nailed down before Phase 9 planning)

| Item | Required By Phase 9 | Current Status |
|------|---------------------|----------------|
| `PriceGapPosition.Mode string` field | D-08, D-09, D-11 — every paper/live distinction | **Likely missing** — not seen in model grep. Phase 8 CONTEXT D-23 mentions `mode` but Phase 8 model file currently lacks it. **Planner verifies via `grep "Mode" internal/models/pricegap_position.go` and adds if absent.** |
| `GetPriceGapHistory(n int)` or `(offset, limit)` | D-04 Closed Log + D-25 metrics | **Verify existence** — only `AddPriceGapHistory` was found in grep; reader method may or may not exist |
| `sampleLegs` + `computeSpreadBps` re-exported from `tracker.go` | Live Positions "Current spread" column | Currently package-private — planner either re-exports or moves to a shared helper file |
| `BroadcastPriceGap*` methods on `*Server` | D-05 WS topics | Do not exist — Phase 9 adds them |
| `PriceGapPaperMode` config field | D-12 | Not in current `config.Config` grep (10 fields listed, no paper_mode). Phase 9 adds it + default `true` + JSON round-trip |
| `NotifyPriceGap{Entry,Exit,RiskBlock}` on `*TelegramNotifier` | D-18..D-22 | Do not exist — Phase 9 adds them |
| Broadcaster DI into `Tracker` | Pattern 1 | Not wired — Phase 9 adds the interface + cmd/main.go injection |
| `pg-admin` CLI parity | D-15, D-17 | CLI exists (`cmd/pg-admin/main.go`); dashboard handlers must wrap identical Redis ops |
| Phase 8 SUMMARY — paper-mode hook locations | Every paper-mode plan | Verify Phase 8 left `execution.go` / `monitor.go` with clean single chokepoints (appears so per code reading) |

**Planner action:** First plan must be "Phase 8 delta verification + `Mode` field + `PaperMode` config + `GetPriceGapHistory` reader" before any UI-facing work starts. Treat as a bridge plan: roughly 3 tasks, all backend, gated on `go build` + existing test suite green.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `testify`-style assertions (verified by existing `pricegap_state_test.go`, `execution_test.go`, `monitor_test.go`, `tracker_test.go`, `slippage_test.go`, `risk_gate_test.go`) |
| Config file | `go.mod` at repo root; no separate test config |
| Quick run command | `go test ./internal/pricegaptrader/... ./internal/notify/... ./internal/api/... ./internal/config/... -count=1 -race -short` |
| Full suite command | `go test ./... -count=1 -race` (then `cd web && npm ci && npm run build` to verify frontend compiles) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PG-OPS-01 | Candidate disable via dashboard writes identical Redis shape to `pg-admin disable` | unit | `go test ./internal/api -run TestPriceGapCandidateDisable -race` | ❌ Wave 0 |
| PG-OPS-01 | Re-enable clears the flag | unit | `go test ./internal/api -run TestPriceGapCandidateEnable -race` | ❌ Wave 0 |
| PG-OPS-02 | `/api/pricegap/state` returns active positions with required fields populated | unit | `go test ./internal/api -run TestPriceGapState_Active -race` | ❌ Wave 0 |
| PG-OPS-02 | `BroadcastPriceGapPositions` fires after entry | unit | `go test ./internal/pricegaptrader -run TestEntry_BroadcastsPgPositions -race` | ❌ Wave 0 |
| PG-OPS-03 | Closed log rows include `ModeledSlipBps`, `RealizedSlipBps`, delta | unit | `go test ./internal/api -run TestPriceGapState_ClosedLog_ModeledVsRealized -race` | ❌ Wave 0 |
| PG-OPS-04 | `PriceGapPaperMode=true` suppresses `PlaceOrder` and synthesizes fill | unit | `go test ./internal/pricegaptrader -run TestPaperMode_NoPlaceOrder -race` | ❌ Wave 0 |
| PG-OPS-04 | `pos.Mode` stamped at entry and immutable | unit | `go test ./internal/pricegaptrader -run TestPaperMode_StampsModeImmutable -race` | ❌ Wave 0 |
| PG-OPS-04 | Paper-mode toggle round-trip persists to `config.json` | unit | `go test ./internal/api -run TestConfig_PaperMode_RoundTrip -race` | ❌ Wave 0 |
| PG-OPS-05 | `NotifyPriceGapEntry` emits `📝 PAPER` prefix when `pos.Mode=="paper"` | unit | `go test ./internal/notify -run TestNotifyPriceGapEntry_PaperPrefix -race` | ❌ Wave 0 |
| PG-OPS-05 | `NotifyPriceGapRiskBlock` respects `checkCooldown` per gate+symbol | unit | `go test ./internal/notify -run TestNotifyPriceGapRiskBlock_Cooldown -race` | ❌ Wave 0 |
| PG-OPS-05 | Nil-safe notifier receivers | unit | `go test ./internal/notify -run TestNotifyPriceGap_Nil -race` | ❌ Wave 0 |
| PG-VAL-01 | Per-trade slippage fields populated on close (already partially covered by Phase 8 monitor_test) | unit | `go test ./internal/pricegaptrader -run TestClosePair_WritesSlippageFields -race` | Partial — extend existing |
| PG-VAL-02 | `ComputeCandidateMetrics` produces correct 24h/7d/30d bps/day from synthetic history | unit | `go test ./internal/pricegaptrader -run TestComputeCandidateMetrics_Windows -race` | ❌ Wave 0 |
| PG-VAL-02 | Metrics handler returns per-candidate rows sorted by 30d desc | unit | `go test ./internal/api -run TestPriceGapState_MetricsSort -race` | ❌ Wave 0 |
| UI / i18n lockstep | Every `pricegap.*` key in en.ts exists in zh-TW.ts | script gate | `diff <(grep -oE "'pricegap\.[a-zA-Z.]+" web/src/i18n/en.ts | sort -u) <(grep -oE "'pricegap\.[a-zA-Z.]+" web/src/i18n/zh-TW.ts | sort -u)` must be empty | ❌ Wave 0 |
| Frontend build | Page renders without type errors | build gate | `cd web && npm ci && npm run build` | ✓ |
| Backend build | All new code compiles | build gate | `go build ./...` | ✓ |
| Manual | Dashboard renders Price-Gap page, toggles work, WS updates flow | manual-only | browser smoke test (Phase 9 HUMAN-UAT.md) | — |

### Sampling Rate

- **Per task commit:** `go test ./internal/pricegaptrader/... ./internal/notify/... ./internal/api/... -count=1 -race -short`
- **Per wave merge:** `go test ./... -count=1 -race && cd web && npm ci && npm run build`
- **Phase gate:** Full suite green + manual browser smoke + Telegram test message received + paper-mode round-trip verified before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/api/pricegap_handlers_test.go` — covers PG-OPS-01, PG-OPS-02, PG-OPS-03, PG-VAL-02 handler-level tests
- [ ] `internal/pricegaptrader/paper_mode_test.go` (or extend `execution_test.go`) — covers PG-OPS-04
- [ ] `internal/pricegaptrader/metrics_test.go` — covers PG-VAL-02 aggregator pure-function tests
- [ ] `internal/notify/telegram_pricegap_test.go` (or extend `telegram_test.go`) — covers PG-OPS-05
- [ ] i18n sync check — new gate in Makefile or plan verification: `make i18n-check` target that diffs keys
- [ ] Frontend smoke — `web/src/pages/PriceGap.test.tsx` not currently used in codebase (no Vitest setup observed); if Phase 9 introduces it, verify with planner. **Recommendation: skip frontend unit tests; cover via manual UAT + TypeScript compile-time check.**

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Bearer token via `authMiddleware` — existing pattern, no change |
| V3 Session Management | yes | `localStorage[arb_token]` — existing, single source |
| V4 Access Control | yes | All `/api/pricegap/*` routes go through `cors` + `authMiddleware` — no anonymous access |
| V5 Input Validation | yes | Disable-reason text: length cap (e.g. 256 chars), strip control characters. Symbol param validated against `cfg.PriceGapCandidates` allowlist — reject unknown symbols |
| V6 Cryptography | no | No new crypto paths (no secrets, no signing beyond existing exchange adapters) |

### Known Threat Patterns for Go+React+Redis dashboard

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Reason-string injection into Telegram body | Tampering | Strip/escape in `NotifyPriceGap*` before `send`; cap length |
| Unauthorized paper→live flip | Elevation | Paper toggle goes through `/api/config` which is already auth-gated; no separate endpoint |
| Race between CLI disable and dashboard re-enable | Tampering / Repudiation | Redis single-writer per key; last-write-wins is acceptable since both go through `SetCandidateDisabled`/`ClearCandidateDisabled` atomically |
| WS message smuggling | Tampering | Hub broadcasts typed payloads only; client is read-only. No client-to-server commands on `/ws`. |
| XSS via candidate reason in dashboard | Tampering | React escapes by default; do not use `dangerouslySetInnerHTML` for reason text |
| Telegram cooldown bypass via gate-string crafting | Spoofing | Gate names restricted to a fixed enum set (D-21); reject others at handler |

## Sources

### Primary (HIGH confidence — all verified by direct file read)

- `.planning/phases/09-price-gap-dashboard-paper-live-operations/09-CONTEXT.md` — complete decision set (D-01..D-25)
- `.planning/phases/09-price-gap-dashboard-paper-live-operations/09-UI-SPEC.md` — approved UI contract
- `internal/pricegaptrader/execution.go` L1–239 — openPair, placeLeg, circuit breaker
- `internal/pricegaptrader/monitor.go` L1–281 — monitorPosition, closePair, realized slippage math
- `internal/pricegaptrader/slippage.go` L1–77 — exec-quality rolling-window auto-disable
- `internal/notify/telegram.go` L45–332 — nil-safe pattern, checkCooldown, formatDuration/formatExitReason
- `internal/api/server.go` L59–258 — WS hub, BroadcastXxx signatures, route registration
- `internal/models/pricegap_position.go` L1–58 — PriceGapPosition + SlippageSample schemas, ExitReason enum
- `internal/models/pricegap_interfaces.go` L1–46 — PriceGapStore interface contract
- `internal/config/config.go` L268–314 — PriceGap config fields + `jsonPriceGap` nested struct
- `cmd/pg-admin/main.go` L1–149 — CLI Redis semantics (must be preserved verbatim by dashboard)
- `web/src/App.tsx` L21 — Page union, L196 renderPage switch
- `web/src/i18n/index.ts` L1–43 — TranslationKey derivation, Locale context, fallback behaviour
- `.planning/REQUIREMENTS.md` — PG-OPS-01..05, PG-VAL-01..02 wording
- `.planning/ROADMAP.md` — Phase 9 success criteria
- `.planning/config.json` — `nyquist_validation: true`, `ui_phase: true`

### Secondary (MEDIUM confidence)

- Phase 8 plans/summaries in `.planning/phases/08-price-gap-tracker-core/` — scanned structure; individual plan content NOT re-read in this research (deferred to plan phase)
- STATE.md accumulated decisions for Phase 8 — constraints carried forward

### Tertiary (LOW confidence)

- None. Every claim in this research was verified against a concrete file read or grep hit.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every library/pattern exists in the codebase and was read directly
- Architecture: HIGH — integration points match Phase 8 conventions verbatim
- Pitfalls: HIGH — each one is rooted in a specific file + line cited above

**Research date:** 2026-04-22
**Valid until:** 2026-05-22 (30 days — stable codebase; refresh if Phase 8 code drifts between now and plan-phase execution)
