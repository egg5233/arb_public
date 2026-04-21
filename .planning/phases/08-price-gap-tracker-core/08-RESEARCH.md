# Phase 8: Price-Gap Tracker Core — Research

**Researched:** 2026-04-21
**Domain:** New isolated Go subsystem (`internal/pricegaptrader/`) — event detection, delta-neutral IOC execution, Redis persistence, pre-entry risk gates. Gated behind `PriceGapEnabled` (default OFF).
**Confidence:** HIGH (all findings verified by direct inspection of repo files).

---

## Summary

Phase 8 is a greenfield Go package whose shape is already heavily constrained by existing repo conventions. The planner's job is mostly **pattern-match + wire-up + fill two critical gaps**:

1. **Kline gap (HIGH risk).** The unified `Exchange` interface in `pkg/exchange/exchange.go:9-134` has **no `GetKlines` / `GetCandles` / OHLC method anywhere**. Confirmed by case-insensitive grep across `pkg/exchange/{binance,bybit,gateio,bitget,okx,bingx}` (zero matches). CONTEXT §D-04 assumes "REST poll 1m klines"; the planner MUST choose between (a) adding a `GetKlines(symbol, interval, limit)` method to the 35-method `Exchange` interface + 4 adapters, or (b) reframing D-04 to build 1m close bars in-memory by sampling existing WS BBO via `GetBBO`/`GetPriceStore` every poll-tick. Option (b) is strictly smaller blast-radius and matches D-02 "no new adapter methods unless raised". **Recommendation:** use existing WS BBO + in-memory 1m bar aggregator; treat `KlineStalenessSec` as a `time.Since(lastBBOUpdate)` check per leg.
2. **Slippage helper gap (MEDIUM risk).** CONTEXT §code_context mentions `pkg/utils/EstimateSlippage` + `RateToBpsPerHour`. Verified: **`EstimateSlippage` does not exist anywhere in the repo** (`grep -rn "EstimateSlippage" --include="*.go"` → 0 matches). `RateToBpsPerHour` exists (in `pkg/utils/math.go` per discovery usage) but is funding-specific. Realized slippage must be computed inline from leg fills.

Everything else — constructor, Start/Stop, Redis persistence, config struct extension, `cmd/*` scaffold, miniredis tests, main.go wiring, delist check — follows a single clear precedent (spotengine).

**Primary recommendation:** Mirror `internal/spotengine/` package layout 1-for-1; name the struct `Tracker` (not `Engine`) to honor D-01; build 1m bars in-memory from WS BBO instead of adding kline methods to adapters.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01** New package `internal/pricegaptrader/` (naming = "tracker", not "engine").
- **D-02** No imports of `internal/engine/` or `internal/spotengine/`. Allowed: `pkg/exchange/`, `pkg/utils/`, `internal/models/`, `internal/config/`, `internal/database/`. `internal/notify/` deferred to Phase 9.
- **D-03** Startup conditional on `PriceGapEnabled`; wired after SpotEngine in `cmd/main.go`. Graceful shutdown reverse-order. Open positions persist across restart (Redis rehydration).
- **D-04** Hybrid data path: REST 1m klines for spread calc + WS last-trade/ticker as freshness heartbeat. *(Conflicts with actual adapter surface — see Summary item 1.)*
- **D-05** Poll cadence 30s; 5 candidates × 4 exchanges × 2 per-min = ~40 req/min.
- **D-06** Spread formula: `(price_A - price_B) / mid × 10_000 bps`.
- **D-07** Event fires when `|spread| ≥ T` for ≥4 consecutive 1m bars. Bar history in-memory only; resets on restart.
- **D-08** Candidate tuples direction-pinned: `{symbol, long_exch, short_exch, threshold_bps, max_position_usdt, modeled_slippage_bps}`.
- **D-09** Simultaneous IOC market orders via existing `pkg/exchange/` `PlaceOrder` with `Force: "ioc"`.
- **D-10** Partial-fill reconciliation = unwind over-filled leg to match smaller fill (simpler than `engine.go:3024` `retrySecondLeg`). Zero-fill on one leg → close the other immediately. Circuit-breaker = 5 consecutive `PlaceOrder` failures pause tracker.
- **D-11** Exit on `|spread| ≤ T/2` OR 4h max-hold timer.
- **D-12** Exit simultaneous IOC; partial-fill remainder retried as market; positions must close fully.
- **D-13** `ExitReason` enum: `reverted | max_hold | manual | risk_gate | exec_quality`.
- **D-14** Redis DB 2, prefix `pg:`. Keys: `pg:pos:{id}`, `pg:positions:active`, `pg:history` (LIST cap 500), `pg:candidate:disabled:{symbol}`, `pg:slippage:{candidate_id}`.
- **D-15** Position ID: `pg_{symbol}_{exchA}_{exchB}_{unix_nano}`.
- **D-16** Rehydration on startup from active SET.
- **D-17** Pre-entry checks in fixed order: Gate concentration cap 50%, delist/halt/staleness, max concurrent 3, per-position notional cap, budget remaining.
- **D-18** Margin mode = cross (shared pool with perp-perp). Isolation via caps, not margin mode.
- **D-19** Exec-quality auto-disable: mean realized > 2× mean modeled over last 10 closed → `pg:candidate:disabled:{symbol} = 1`.
- **D-20** Re-enable via `cmd/pg-admin/` (Redis toggle). NOT via config.json hand-edit.
- **D-21** Modeled slippage static per-candidate from config (seeded from `edge_v2.json`). Realized computed from leg fills.
- **D-22** New `PriceGap*` config fields (11 total — listed in CONTEXT.md).
- **D-23** Config reload via existing `/api/config` (Phase 9 wires dashboard; Phase 8 only startup-load).

### Claude's Discretion

- Exact goroutine topology (single-tick vs worker-pool).
- Telemetry / log-line formats beyond `ExitReason`.
- Internal interface shape for injecting exchange registry + Redis store.
- `cmd/pg-admin/` CLI UX (flag names, output format).
- Kline endpoint naming per adapter **IF** the planner adds kline methods.

### Deferred Ideas (OUT OF SCOPE for Phase 8)

- Rolling-spread scanner, candidate state machine, auto-promotion.
- Correlation / regime-cluster cap.
- OKX + BingX inclusion.
- Shared capital allocator integration.
- Funding-cost integration during hold.
- Persistent bar-history across restart.
- Dashboard / paper mode / Telegram alerts / rolling bps metrics (all Phase 9).
- Cross-strategy HealthMonitor coupling.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PG-01 | 1m kline spread detection ≥4-bar persistence filter | §Detection state machine; Summary §1 (kline gap — recommend in-memory bars from WS BBO) |
| PG-02 | Delta-neutral 2-leg IOC entry | `pkg/exchange/types.go:67-77` `PlaceOrderParams{Force: "ioc"}`; reference-only pattern `internal/engine/engine.go:3024 retrySecondLeg` (NOT reused); simpler `executeTrade` at `internal/engine/engine.go:3167+` |
| PG-03 | Exit on `T/2` reversion OR 4h max-hold | In-memory exit monitor goroutine per active position |
| PG-04 | Positions persist to `pg:pos:{id}`, survive restart | `internal/database/spot_state.go:14-54` pattern; new `pricegap_state.go` |
| PG-05 | Candidate list config-driven | `internal/config/config.go:183+ SpotFutures*` field pattern; new `PriceGapCandidates []PriceGapCandidate` slice |
| PG-RISK-01 | Gate concentration ≤50% of `PriceGapBudget` | Pre-entry deterministic sum over active positions with `gate` leg |
| PG-RISK-02 | Delist / halt / kline-staleness <90s gate | `internal/discovery/delist.go:72 IsDelisted(symbol)` API; adapter `LoadAllContracts().DeliveryDate`; staleness = `time.Since(lastBBOUpdate)` |
| PG-RISK-03 | Exec-quality 2× overshoot auto-disable after 10 trades | `pg:slippage:{candidate_id}` LIST; realized vs modeled comparison on every close |
| PG-RISK-04 | Max concurrent positions cap | Count from `pg:positions:active` SET cardinality |
| PG-RISK-05 | Per-position notional cap per candidate | `PriceGapCandidate.MaxPositionUSDT` |
| PG-OPS-06 | `PriceGapEnabled` switch (default OFF) + config.json persistence | `internal/config/config.go` + `applyJSON` + `jsonConfig` fields (pattern at line 1134-1166 for `sf.Enabled`) |
</phase_requirements>

---

## Project Constraints (from CLAUDE.local.md)

- **NEVER modify `config.json`** — live runtime config with API keys/credentials; Phase 8 extends the Go struct but must NOT touch the on-disk file. Operators update via `/api/config` POST or hand-edit at their own risk outside Claude.
- **npm lockdown** — Phase 8 is pure Go; no frontend work. If any frontend touch sneaks in: `npm ci` only.
- **Build order** — `npm run build` (web/) before `go build` because of `go:embed`. Phase 8 doesn't touch web/, so a plain `go build` is correct per-task but the full release build order must be respected.
- **Delegation mode** — any task touching 2+ files should be delegated to Sonnet 4.6 / Opus 4.6 teammates.
- **CHANGELOG.md + VERSION** — every commit must bump both.
- **graphify routing** — `graphify-publish/AI_ROUTER.md` first for any code-navigation question, not repo-wide grep.
- **Go code navigation** — use LSP (gopls) for tracing call paths, not grep, when available.

---

## Standard Stack

### Core (already in repo — no new deps)

| Package | Version | Purpose | File Evidence |
|---------|---------|---------|---------------|
| `github.com/redis/go-redis/v9` | v9.18.0 | Redis client | `go.mod:9` |
| `github.com/alicebob/miniredis/v2` | v2.37.0 | Test-time Redis | `go.mod:6`, `internal/spotengine/autoentry_test.go:10` |
| stdlib `sync`, `context`, `time`, `encoding/json` | — | Concurrency + persistence | all engine files |

### New dependencies

**None.** Phase 8 must not add Go modules. CLAUDE.local.md npm lockdown applies to JS only, but the project's convention is "single binary, minimal deps" — any proposed addition needs user approval.

---

## Architecture Patterns

### Recommended package structure (mirror `spotengine`)

```
internal/pricegaptrader/
├── tracker.go             # Tracker struct, NewTracker, Start, Stop (mirrors engine.go:82-156)
├── detector.go            # 1m bar aggregator + 4-bar persistence state machine (PG-01)
├── execution.go           # Entry: simultaneous IOC + unwind-to-match; Exit: spread/max-hold
├── risk_gate.go           # Pre-entry 5-check evaluator (PG-RISK-01..05)
├── slippage.go            # Realized vs modeled computation + auto-disable logic
├── monitor.go             # Per-position monitor goroutine (spread + max-hold timer)
├── rehydrate.go           # Startup-time position reload from Redis
├── tracker_test.go        # miniredis + stub exchange tests
├── detector_test.go
├── risk_gate_test.go
└── execution_test.go
```

### Pattern 1: Tracker struct + constructor
**What:** Mirror `SpotEngine` exactly.
**Source:** `internal/spotengine/engine.go:82-112`
```go
// In NEW file internal/pricegaptrader/tracker.go
type Tracker struct {
    exchanges map[string]exchange.Exchange
    db        PriceGapStore // interface, not *database.Client directly
    cfg       *config.Config
    log       *utils.Logger
    stopCh    chan struct{}
    wg        sync.WaitGroup
    exitWG    sync.WaitGroup

    // In-memory 1m bar state (resets on restart per D-07)
    barsMu sync.RWMutex
    bars   map[string]*candidateBars // key = candidate ID

    // Active position monitor registry
    monMu     sync.Mutex
    monitors  map[string]context.CancelFunc // posID -> cancel
}

func NewTracker(
    exchanges map[string]exchange.Exchange,
    db PriceGapStore,
    cfg *config.Config,
) *Tracker { ... }
```

### Pattern 2: Start/Stop with WaitGroup
**Source:** `internal/spotengine/engine.go:130-148`
```go
func (t *Tracker) Start() {
    t.log.Info("Price-gap tracker starting (candidates=%d, budget=$%.0f)",
        len(t.cfg.PriceGapCandidates), t.cfg.PriceGapBudget)
    t.wg.Add(1)
    go t.tickLoop()      // single poll goroutine
    // re-start per-position monitors via rehydrate.go
}
func (t *Tracker) Stop() {
    close(t.stopCh)
    t.wg.Wait()
    t.exitWG.Wait()
}
```

### Pattern 3: Redis persistence (copy `spot_state.go` verbatim with `pg:` prefix)
**Source:** `internal/database/spot_state.go:14-54`, `:95-131`, `:121-131`
- `keyPricegapPositions = "pg:positions"` (HSET id -> JSON)
- `keyPricegapActive = "pg:positions:active"` (SET)
- `keyPricegapHistory = "pg:history"` (LIST, LPush+LTrim to 500)
- `keyPricegapDisabledPrefix = "pg:candidate:disabled:"`
- `keyPricegapSlippagePrefix = "pg:slippage:"`
- Use `c.rdb.Pipeline()` per `spot_state.go:45-53`.
- Save-pattern: `HSET` + `SAdd`/`SRem` by status in one pipeline.

### Pattern 4: Config struct extension
**Source:** `internal/config/config.go:183-223` (struct fields), `:1134-1166` (JSON apply).
- Add 11 `PriceGap*` fields to `Config` struct (CONTEXT D-22).
- Add shadow struct `PriceGapJSON` in `jsonConfig` with pointer fields (`*bool`, `*int`, `*float64`) matching `sf.Enabled = *bool` pattern at `:1135`.
- Copy pointer→concrete mapping in `applyJSON` — line 1134 shows `if sf := jc.SpotFutures; sf != nil { if sf.Enabled != nil { c.SpotFuturesEnabled = *sf.Enabled } ... }`.
- JSON tag naming: snake_case; all defaults OFF/zero.

### Pattern 5: Interface-driven DI
**Source:** `internal/models/interfaces.go:58-85 StateStore`
Proposed new interface in `internal/models/interfaces.go` (or new file `internal/models/pricegap_interfaces.go`):
```go
type PriceGapStore interface {
    SavePriceGapPosition(p *PriceGapPosition) error
    GetPriceGapPosition(id string) (*PriceGapPosition, error)
    GetActivePriceGapPositions() ([]*PriceGapPosition, error)
    AddPriceGapHistory(p *PriceGapPosition) error

    IsCandidateDisabled(symbol string) (bool, string, error)
    SetCandidateDisabled(symbol, reason string) error
    ClearCandidateDisabled(symbol string) error

    AppendSlippageSample(candidateID string, realized, modeled float64) error
    GetSlippageWindow(candidateID string, n int) ([]SlippageSample, error)

    // Lock reuse — per-candidate entry serialization
    AcquireLock(resource string, ttl time.Duration) (bool, error)
    ReleaseLock(resource string) error
}
```
Concrete `*database.Client` satisfies it via methods added to `internal/database/pricegap_state.go` (new file).

### Pattern 6: `cmd/pg-admin/main.go` scaffold
**Source:** `cmd/balance/main.go:1-60`
- Top of file: `package main`, imports `arb/internal/config`, `arb/internal/database`.
- `cfg := config.Load()` → `db, _ := database.NewClient(cfg)` → read/write `pg:candidate:disabled:*`.
- Sub-commands via `os.Args[1]` switch: `enable <symbol>`, `disable <symbol>`, `status`, `positions`.
- Output via `fmt.Printf` tabular layout (see `balance/main.go:19-22`).

### Pattern 7: Graceful shutdown in `cmd/main.go`
**Source:** `cmd/main.go:439-492`
- Startup order (line 437-456): Scanner → Risk → Health → API → Engine → Scraper → SpotEngine.
- Insert Price-Gap tracker **after SpotEngine** (CONTEXT D-03):
```go
var tracker *pricegaptrader.Tracker
if cfg.PriceGapEnabled {
    tracker = pricegaptrader.NewTracker(exchanges, db, cfg)
    tracker.Start()
    log.Info("Price-gap tracker started")
}
```
- Shutdown reverse order (line 467-492): `tracker.Stop()` before `spotEng.Stop()` if started.

### Anti-patterns to avoid
- **Don't import `internal/engine/` or `internal/spotengine/`** (D-02 boundary). Share only via `pkg/exchange/`, `pkg/utils/`, `internal/models/`, `internal/config/`, `internal/database/`.
- **Don't reuse `retrySecondLeg`** (`internal/engine/engine.go:3024`). D-10 mandates simpler unwind-to-match.
- **Don't add kline methods to adapters silently.** Either raise the decision to the user OR build 1m bars in-memory from WS BBO (recommended).
- **Don't write to `config.json`** — CLAUDE.local.md forbids it. Config changes are via `/api/config` (Phase 9) or user hand-edit.

---

## Per-Adapter Data-Source Audit

This is the critical open item from CONTEXT §D-04.

| Exchange | Kline REST method | WS ticker / last-trade | BBO API | GetOrderbook | LoadAllContracts (delist) |
|----------|---------------------|-------------------------|---------|--------------|---------------------------|
| Binance | **MISSING** | `SubscribeSymbol()` + `GetBBO(symbol)` + `GetPriceStore()` (`pkg/exchange/exchange.go:65-68`) | ✓ | ✓ | ✓ (`DeliveryDate` on `ContractInfo`, `pkg/exchange/types.go:107+`) |
| Bybit | **MISSING** | ✓ (same interface) | ✓ | ✓ | ✓ (`deliveryTime`) |
| Gate.io | **MISSING** | ✓ (same interface) | ✓ | ✓ | ✓ |
| Bitget | **MISSING** | ✓ (same interface) | ✓ | ✓ | ✓ |

**Verification:** `grep -rln -iE "kline\|candle\|ohlcv" pkg/exchange/` returned **zero files**. The `Exchange` interface at `pkg/exchange/exchange.go:9-134` has no kline method. Training knowledge of adapter internals is **not** sufficient — confirmed empirically.

**Planner must choose:**
- **Option A (recommended):** Reframe D-04. Build 1m bars in-memory from `GetBBO(symbol)` mid-price, sampled at D-05's 30s tick. 2 samples per minute per leg = 1m bar close = last sample at :00. Freshness check = `time.Since(lastBBOUpdate) < 90s` using `GetPriceStore()` timestamps. Advantages: zero adapter changes, satisfies D-02, matches Phase 0 methodology closely enough for `T=200` bps events (signal >> measurement error). Cost: bar history resets on restart (CONTEXT D-07 already accepts this).
- **Option B:** Add `GetKlines(symbol, interval string, limit int) ([]Kline, error)` to the 35-method `Exchange` interface + 4 adapter implementations. Bigger scope; requires user approval to expand interface; risks breaking the 6 existing exchange adapters' test contracts. **Explicitly out-of-bounds per CLAUDE.local.md concern "Exchange adapter reuse only — no new adapter methods; if a method is missing, raise before adding."**

The planner MUST surface this decision as Plan-0 before execution starts.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Redis locks | Custom SET NX Lua | `database.Client.AcquireLock(resource, ttl)` (`internal/database/locks.go`) | Already has Lua + token-owned lease + renewal |
| Position ID | UUID library | `fmt.Sprintf("pg_%s_%s_%s_%d", sym, ex1, ex2, time.Now().UnixNano())` (D-15) | No new deps; matches spotengine naming |
| Redis client | New connection pool | Injected `*database.Client` (shared) | Single Redis pool per process |
| Structured logger | log/slog | `utils.NewLogger("pg-tracker")` (`pkg/utils/logging.go`) | Matches stdout format other engines use |
| JSON serialization | Custom codec | `encoding/json` + struct tags (see `models/spot_position.go`) | stdlib |
| Symbol → exchange-format mapping | Inline string munging | Adapters already convert `BTCUSDT` → `BTC_USDT` / `BTC-USDT-SWAP` internally | CLAUDE.local.md "adapter rules" |
| Delist check | New REST poller | `scanner.IsDelisted(symbol)` (`internal/discovery/delist.go:72`) | Global 6h poll already running; reuse read-only |
| Contract status / DeliveryDate | Per-tracker poller | `exchange.LoadAllContracts()[symbol].DeliveryDate` (`pkg/exchange/types.go:107+` + CLAUDE.md note on `deliveryDate`) | Already populated at startup + refresh |

---

## Runtime State Inventory

Phase 8 is greenfield (new code + new Redis namespace), not a rename. Confirming every category:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | **None** — `pg:*` is a new Redis namespace. No collision with `arb:*` or `arb:spot_*` (verified against `internal/database/state.go:22-33` constants and `spot_state.go:14-24`). | None |
| Live service config | **None** — tracker runs in-process under existing systemd unit | None |
| OS-registered state | **None** | None |
| Secrets/env vars | **None** — no new credentials (uses existing exchange API keys) | None |
| Build artifacts | **New** — `cmd/pg-admin/` binary will produce a new Go build target; `go build ./cmd/pg-admin` after Phase 8 ships | Document in Makefile |

---

## Common Pitfalls

### Pitfall 1: Silent adapter-method assumption (HIGH)
**What goes wrong:** Plan assumes `exchange.GetKlines()` exists (per CONTEXT D-04 wording) and task executor hits a compile error on first task.
**Root cause:** The CONTEXT was written assuming adapter capability; adapter actually exposes only WS BBO + REST orderbook.
**Prevention:** Plan-0 must be "Audit + Decide data-source approach" with user confirmation before any `pricegaptrader/` code lands.
**Warning sign:** Any task referring to `GetKlines` / `GetCandles` without explicit "adds method to Exchange interface" callout.

### Pitfall 2: Starving the tracker during Bybit blackout
**What goes wrong:** 30s poll tick hits Bybit at :04:30 during blackout window (per CLAUDE.md Scan Schedule — Bybit :04–:05:30).
**Prevention:** Either align tick clock to `time.Now().Truncate(30*time.Second)` offset by +7s (so ticks fall on :07, :37 — outside blackout band), or add explicit skip for Bybit-leg calls during the blackout window. Document in detector.go.

### Pitfall 3: Ghost-position rehydration desync
**What goes wrong:** Process restarts mid-exit. `pg:positions:active` still contains position that was already market-closed on-exchange. Rehydrate re-starts monitor loop; first exit attempt fails because position doesn't exist on the exchange.
**Prevention:** On rehydration, call `GetPosition(symbol)` on both legs BEFORE enrolling in monitor. If either leg returns zero size, mark as closed in Redis immediately (reason: `recovered_orphan`). Mirror the `consolidator` logic in `internal/engine/engine.go` (3-miss threshold for BingX per CLAUDE.md).

### Pitfall 4: Partial-fill unwind creates its own partial fill
**What goes wrong:** D-10 says "unwind over-filled leg to match smaller fill". If the unwind IOC itself partially fills, we're asymmetric again.
**Prevention:** Unwind path must be market (not IOC) per D-12 "positions must close fully" convention. Retry as market order on any IOC shortfall.

### Pitfall 5: `pg:*` key collision with a future subsystem
**What goes wrong:** Unlikely today but `pg:` is two letters; long-term a "Postgres" or "Payment Gateway" subsystem could collide.
**Prevention:** Document the namespace in `internal/database/pricegap_state.go` header comment and in ARCHITECTURE.md (new section — planner must add).

### Pitfall 6: `config.json` hand-edit drift
**What goes wrong:** Operator hand-edits `config.json` to enable tracker + add candidates; a later `/api/config` POST wipes it (per CLAUDE.local.md lockdown).
**Prevention:** Ship the `pg-admin` binary with `pg-admin candidates list` / `pg-admin enable-flag` helpers so operators don't need to hand-edit for steady-state operations. Flag defaults ship in `config.go` `applyJSON` defaults.

### Pitfall 7: Exec-quality disable without undisable path in Phase 8
**What goes wrong:** After 10 trades, a candidate auto-disables. Operator has no Phase 8 UI. Candidate stays disabled forever.
**Prevention:** `pg-admin enable <symbol>` (D-20) MUST ship in Phase 8, not Phase 9. Add as a distinct task.

---

## Code Examples

### Example 1: PriceGapPosition model (add to `internal/models/pricegap_position.go`)
```go
package models

import "time"

type PriceGapPosition struct {
    ID              string    `json:"id"`
    Symbol          string    `json:"symbol"`
    LongExchange    string    `json:"long_exchange"`
    ShortExchange   string    `json:"short_exchange"`
    Status          string    `json:"status"` // "pending" | "open" | "exiting" | "closed"
    EntrySpreadBps  float64   `json:"entry_spread_bps"`
    ThresholdBps    float64   `json:"threshold_bps"`
    NotionalUSDT    float64   `json:"notional_usdt"`
    LongFillPrice   float64   `json:"long_fill_price"`
    ShortFillPrice  float64   `json:"short_fill_price"`
    LongSize        float64   `json:"long_size"`
    ShortSize       float64   `json:"short_size"`
    ModeledSlipBps  float64   `json:"modeled_slippage_bps"`
    RealizedSlipBps float64   `json:"realized_slippage_bps"`
    RealizedPnL     float64   `json:"realized_pnl"`
    ExitReason      string    `json:"exit_reason"` // reverted|max_hold|manual|risk_gate|exec_quality
    OpenedAt        time.Time `json:"opened_at"`
    ClosedAt        time.Time `json:"closed_at,omitempty"`
}
```

### Example 2: 4-bar persistence state machine (detector.go)
```go
// barRing holds last N 1m closes per candidate; no external lib needed.
type barRing struct {
    closes [4]float64   // spread_bps for last 4 bars
    valid  [4]bool
    idx    int          // next slot
    // last completed minute; used to skip tick within same minute
    lastMin int64
}

func (b *barRing) push(minute int64, spreadBps float64) bool {
    if minute == b.lastMin { return false } // dedupe within same minute
    b.closes[b.idx] = spreadBps
    b.valid[b.idx] = true
    b.idx = (b.idx + 1) % 4
    b.lastMin = minute
    return true
}

func (b *barRing) allExceed(T float64) bool {
    sign := 0.0
    for i, ok := range b.valid {
        if !ok { return false }
        s := b.closes[i]
        if math.Abs(s) < T { return false }
        if i == 0 { sign = s } else if s*sign < 0 { return false } // same-sign persistence
    }
    return true
}
```

### Example 3: Pre-entry gate composition (risk_gate.go)
```go
// GateResult mirrors models.RiskApproval shape — Approved + Reason.
func (t *Tracker) preEntry(cand PriceGapCandidate, notional float64) GateResult {
    if dis, _, _ := t.db.IsCandidateDisabled(cand.Symbol); dis {
        return GateResult{Approved: false, Reason: "exec_quality_disabled"}
    }
    if t.openCount() >= t.cfg.PriceGapMaxConcurrent {
        return GateResult{Approved: false, Reason: "max_concurrent"}
    }
    if notional > cand.MaxPositionUSDT {
        return GateResult{Approved: false, Reason: "per_candidate_cap"}
    }
    if !t.budgetOK(notional) {
        return GateResult{Approved: false, Reason: "budget"}
    }
    if !t.gateConcentrationOK(cand, notional) { // 50% cap
        return GateResult{Approved: false, Reason: "gate_concentration"}
    }
    if err := t.freshnessAndDelistCheck(cand); err != nil {
        return GateResult{Approved: false, Reason: err.Error()}
    }
    return GateResult{Approved: true}
}
```

### Example 4: Simultaneous IOC (simpler than engine.go:3167)
```go
// Fire both legs concurrently; collect results; if asymmetric, unwind.
func (t *Tracker) openPair(cand PriceGapCandidate, size float64, longPx, shortPx float64) (*models.PriceGapPosition, error) {
    type fillRes struct{ filled, vwap float64; err error }
    longCh, shortCh := make(chan fillRes, 1), make(chan fillRes, 1)

    go func() {
        id, err := t.exchanges[cand.LongExch].PlaceOrder(exchange.PlaceOrderParams{
            Symbol: cand.Symbol, Side: exchange.SideBuy, OrderType: "market",
            Size: utils.FormatSize(size, t.decimals(cand.LongExch, cand.Symbol)),
            Force: "ioc",
        })
        // ... fetch fill via GetOrderFilledQty
        longCh <- fillRes{ /*...*/ err: err }
    }()
    go func() { /* mirror for short */ }()

    lr, sr := <-longCh, <-shortCh
    if lr.err != nil && sr.err != nil { return nil, fmt.Errorf("both legs failed") }
    // D-10: unwind to min(filled)
    match := math.Min(lr.filled, sr.filled)
    if match == 0 {
        // zero one leg -> market-close the other
        t.closeLeg(&lr, &sr)
        return nil, fmt.Errorf("zero fill on one leg")
    }
    if lr.filled > match { t.unwindMarket(cand.LongExch, cand.Symbol, exchange.SideSell, lr.filled-match) }
    if sr.filled > match { t.unwindMarket(cand.ShortExch, cand.Symbol, exchange.SideBuy, sr.filled-match) }
    return t.persistOpen(cand, match, lr.vwap, sr.vwap), nil
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Engine imports concrete `*discovery.Scanner` | Engine depends on `models.Discoverer` interface | v1.0 milestone | Tracker MUST follow: depend on `PriceGapStore` interface |
| Redis HSET for config | `config.json` sole source of truth | 2026-04-05 | Tracker config fields persist via `applyJSON` only; NO Redis HSET |
| Kline fetch via adapter | *(never existed)* | — | Must be built in-memory from WS BBO |

**Deprecated / not available:**
- `pkg/utils.EstimateSlippage` — referenced in CONTEXT but **does not exist**; realized slippage must be computed inline.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `GetBBO` timestamps on `GetPriceStore` are fresh enough to use as freshness proxy (`<90s` gate) | Summary, Pitfall 2 | Low — BBO updates every WS tick; if stale >90s, leg is dead anyway |
| A2 | In-memory 1m bar sampled at 30s tick is faithful enough to Phase 0 methodology for `T=200` events | Summary Option A | Medium — consumer of research (planner) should surface this to user for Plan-0 sign-off |
| A3 | `pg-admin` can safely mutate Redis without process coordination (tracker + admin both connected) | D-20, Pitfall 7 | Low — Redis ops are atomic; tracker reads disable flag on each entry attempt |
| A4 | Go 1.26 install.sh drift (per CLAUDE.local.md "Known drift") does not block Phase 8 because the live machine already runs 1.26+ | Tech Stack | Low if executed on live server; surfaces only on fresh deploy |

---

## Open Questions

1. **Kline source (BLOCKING)** — Option A (WS BBO sampler) vs Option B (add `GetKlines` to interface). Recommendation: Option A. **Must be Plan-0.**
2. **`pg-admin` scope** — minimum: `enable`/`disable`/`status`. Include `positions list` and `close <id>` for live-ops? Recommendation: ship all four (trivial extension of the CLI scaffold).
3. **Stub `/api/pricegap/health` endpoint** — CONTEXT §code_context marks as "planner decides". Recommendation: ship a 10-line handler returning `{enabled, budget, open_count}` — it's tiny, unblocks live-rollout smoke testing, and does NOT pre-empt Phase 9's full tab wiring.
4. **Symbol-specific `SizeDecimals`** — the tracker needs per-exchange size formatting (`utils.FormatSize(size, decimals)`) for each candidate's leg. Does `LoadAllContracts` already populate this on startup for all needed symbols? Need planner to verify `ContractInfo.SizeDecimals` is reliable for low-cap perps on Gate.io (quanto conversion per CLAUDE.local.md adapter rules).
5. **Modeled slippage seed table** — `edge_v2.json` has `SOONUSDT bin-gate @ $5k: cost 47.9 bps`, etc. Planner should seed each candidate's `ModeledSlippageBps` in config defaults using the `$5000` row (matches CONTEXT PG-RISK-05 "initial $5k budget"). SOON=47.9, SKYAI bin-gate=72.3, SKYAI bin-bitget=66.5, SKYAI bitget-gate=87.8, DRIFT=unknown (use 60.0 conservatively).

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build | ✓ | 1.26+ (go.mod:3) | — |
| Redis (DB 2) | persistence | ✓ | existing arb bot dependency | — |
| `github.com/redis/go-redis/v9` | persistence | ✓ | v9.18.0 (go.mod:9) | — |
| `github.com/alicebob/miniredis/v2` | tests | ✓ | v2.37.0 (go.mod:6) | — |
| Exchange API credentials | live trading | ✓ | existing config.json | paper-mode deferred to Phase 9 |
| Bybit non-blackout window | 30s poll | ✓ | :05:30-:04 daily | Skip Bybit-leg calls in blackout |

**No missing dependencies.** Phase 8 is pure Go; builds + tests run with existing repo setup.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` stdlib + `github.com/alicebob/miniredis/v2 v2.37.0` |
| Config file | none (idiomatic Go) |
| Quick run command | `go test ./internal/pricegaptrader/... -count=1 -short` |
| Full suite command | `go test ./internal/pricegaptrader/... -race -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| PG-01 | 4-bar persistence detector | unit | `go test ./internal/pricegaptrader -run TestDetector_FourBar -count=1` | ❌ Wave 0 — `detector_test.go` |
| PG-02 | Simultaneous IOC entry (happy + zero-fill + partial-fill) | unit+stub-exchange | `go test ./internal/pricegaptrader -run TestExecution -count=1` | ❌ Wave 0 — `execution_test.go` |
| PG-03 | Exit on T/2 + max-hold | unit | `go test ./internal/pricegaptrader -run TestMonitor_Exit -count=1` | ❌ Wave 0 — `monitor_test.go` |
| PG-04 | Redis round-trip + rehydration | integration (miniredis) | `go test ./internal/pricegaptrader -run TestRehydrate -count=1` | ❌ Wave 0 — `rehydrate_test.go` |
| PG-05 | Candidate config load + default-off | unit | `go test ./internal/config -run TestApplyJSON_PriceGap -count=1` | ❌ Wave 0 — extend `config_test.go` |
| PG-RISK-01 | Gate concentration cap math | unit (pure func) | `go test ./internal/pricegaptrader -run TestRiskGate_GateConcentration` | ❌ Wave 0 — `risk_gate_test.go` |
| PG-RISK-02 | Delist/halt/staleness block | unit+stub | `go test ./internal/pricegaptrader -run TestRiskGate_Freshness` | ❌ Wave 0 |
| PG-RISK-03 | Exec-quality 2× rolling threshold | unit | `go test ./internal/pricegaptrader -run TestSlippage_AutoDisable` | ❌ Wave 0 — `slippage_test.go` |
| PG-RISK-04 | Max-concurrent cap | unit | `go test ./internal/pricegaptrader -run TestRiskGate_MaxConcurrent` | ❌ Wave 0 |
| PG-RISK-05 | Per-position notional cap | unit | `go test ./internal/pricegaptrader -run TestRiskGate_PerPositionCap` | ❌ Wave 0 |
| PG-OPS-06 | `PriceGapEnabled` default-off + no side-effects when off | unit + manual | `go test ./internal/config -run TestDefaults_PriceGap` + manual startup log grep | ❌ Wave 0 (unit) + manual |
| End-to-end | SOON trade on small notional | **manual live** | N/A — operator observes log + Redis KEYS `pg:*` | — human step |

### Sampling Rate (Nyquist)

- **Per task commit:** `go test ./internal/pricegaptrader -count=1 -short` (<30s)
- **Per wave merge:** `go test ./internal/pricegaptrader ./internal/config ./internal/database -race -count=1`
- **Phase gate:** Full repo `go test ./... -race -count=1` green + `go vet ./...` + manual "enable tracker on $100 notional SOON for 24h paper-equivalent via logs-only" before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `internal/pricegaptrader/detector_test.go` — covers PG-01 (4-bar state machine)
- [ ] `internal/pricegaptrader/execution_test.go` — covers PG-02 (simultaneous IOC + unwind-to-match + zero-fill)
- [ ] `internal/pricegaptrader/monitor_test.go` — covers PG-03 (reversion + max-hold)
- [ ] `internal/pricegaptrader/rehydrate_test.go` — covers PG-04 (Redis round-trip with miniredis)
- [ ] `internal/pricegaptrader/risk_gate_test.go` — covers PG-RISK-01..05 (all five gates)
- [ ] `internal/pricegaptrader/slippage_test.go` — covers PG-RISK-03 (rolling 10-trade window, 2× threshold)
- [ ] `internal/pricegaptrader/testutil_test.go` — shared fixtures: `stubExchange` (mirror `spotengine/exit_triggers_test.go:16 priceStubExchange`), miniredis helper
- [ ] Extend `internal/config/config_test.go` with `TestApplyJSON_PriceGap` + `TestDefaults_PriceGap` for PG-05, PG-OPS-06.
- [ ] Integration test harness: spin up miniredis + 2 stubExchanges + Tracker, drive a full event → entry → monitor → exit cycle.
- [ ] Framework install: **none needed** — `go test` + miniredis already in `go.mod`.

---

## Sources

### Primary (HIGH confidence — direct repo inspection)

- `pkg/exchange/exchange.go:9-134` — unified 35-method `Exchange` interface, NO kline method
- `pkg/exchange/types.go:67-77` — `PlaceOrderParams` (`Force: "ioc"`)
- `pkg/exchange/types.go:107+` — `ContractInfo` with `DeliveryDate`
- `internal/spotengine/engine.go:82-112, 130-148` — Tracker mirror pattern
- `internal/database/spot_state.go:14-54, 95-131` — Redis persistence pattern
- `internal/database/state.go:22-33` — existing `arb:*` key namespace (confirms `pg:*` distinct)
- `internal/database/locks.go:1-60` — per-symbol Redis lock pattern
- `internal/models/interfaces.go:58-85` — `StateStore` DI pattern for new `PriceGapStore`
- `internal/config/config.go:183-223, 1134-1166` — `SpotFutures*` struct + `applyJSON` pattern
- `internal/discovery/delist.go:52, 72` — `StartDelistMonitor()` + `IsDelisted(symbol)` API
- `internal/engine/engine.go:3024 retrySecondLeg, 3167+ executeTrade` — reference-only IOC logic
- `cmd/balance/main.go:1-60` — sample `cmd/*` CLI scaffold
- `cmd/main.go:437-492` — startup/shutdown order
- `internal/spotengine/autoentry_test.go:10, 17`, `risk_gate_test.go:194`, `exit_triggers_test.go:16` — miniredis + stubExchange test harness
- `/tmp/phase0-pricegap/edge_v2.json` — seed ModeledSlippageBps per candidate
- `/var/solana/data/arb/CLAUDE.local.md` — module boundaries, config.json lockdown, build order, feature rollout trio
- `/var/solana/data/arb/.planning/phases/08-price-gap-tracker-core/08-CONTEXT.md` — all 23 locked decisions
- `/var/solana/data/arb/go.mod:1-10` — Go 1.26, redis v9.18.0, miniredis v2.37.0

### Secondary

- CLAUDE.local.md "Scan Schedule" — Bybit :04-:05:30 blackout
- CLAUDE.local.md "Known drift" — Go 1.26 vs install.sh 1.22.12

### Tertiary

- None. Every critical claim in this research was verified against a repo file with line number.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all deps already in go.mod
- Architecture: HIGH — mirror spotengine 1-to-1
- Pitfalls: HIGH (except P1 which is ONLY HIGH because confirmed via grep across all adapter dirs)
- Data-source audit: HIGH — confirmed empirically that no adapter has kline

**Research date:** 2026-04-21
**Valid until:** 2026-05-21 (30 days — adapter interface is stable; live trading is active so no refactors expected)

---

*Research file: `/var/solana/data/arb/.planning/phases/08-price-gap-tracker-core/08-RESEARCH.md`*
