# Phase 11: Auto-Discovery Scanner + Chokepoint + Telemetry — Research

**Researched:** 2026-04-28
**Domain:** Read-only market scanner + write chokepoint + telemetry inside `internal/pricegaptrader/` (Strategy 4 / cross-exchange price-gap arb)
**Confidence:** HIGH (every code anchor verified in-tree; CONTEXT.md decisions locked; v2.2 architecture/pitfalls research already exists)

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| **PG-DISC-01** | Auto-discovery scanner polls bounded universe (≤20 symbols × 6 exchanges) on configurable interval, applies ≥4-bar persistence + BBO freshness + depth probe, computes per-candidate score, writes to `pg:scan:*`. Default OFF via `PriceGapDiscoveryEnabled`. Read-only until PG-DISC-04. | §"Scanner architecture", §"Universe selection", §"Gates & score" |
| **PG-DISC-03** | Discovery telemetry in dashboard: cycle stats, score history, why-rejected breakdown, promote/demote timeline placeholder. Reads `pg:scan:*` via WS hub batching + REST seed. | §"Dashboard contract", §"Redis schema" |
| **PG-DISC-04** | `CandidateRegistry` chokepoint serializes ALL `cfg.PriceGapCandidates` writers (dashboard handler, pg-admin, future scanner) through one mutex + atomic file write + timestamped `.bak` ring. Required prerequisite before scanner is write-permitted. | §"CandidateRegistry chokepoint" |
</phase_requirements>

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Score Formula**
- **D-01:** Score shape is **gate-then-magnitude**: hard gates (≥4-bar persistence, BBO <90s on both legs, depth probe pass, not on denylist, cross-exchange pair exists) must ALL pass before any non-zero score is emitted; magnitude reflects the tradeable edge above gates.
- **D-02:** Score is an **integer 0–100** (stored as float in ZSET for ordering, conceptually integer for thresholds/display).
- **D-03:** **All sub-score components persisted** alongside the top-line score per cycle: `{spread_bps, persistence_bars, depth_score, freshness_age_s, funding_bps_h, gates_passed[], gates_failed[]}`. Enables sub-score breakdown UI and future PG-DISC-05 calibration.
- **D-04:** Rejected candidates are **captured with `score=0` + why-rejected reason code** in the cycle record; every (symbol × cross-pair) scanned writes a record.

**Universe & Denylist**
- **D-05:** Universe is **operator-curated** via new `PriceGapDiscoveryUniverse` config field (`[]string` of canonical `BTCUSDT` symbols). ≤20 entries enforced at config load.
- **D-06:** Exchange pairing is **all-cross of universe × supported exchanges** — every (long_exch, short_exch) combo per symbol, skipping exchanges where the symbol isn't listed.
- **D-07:** Denylist lives in dedicated `PriceGapDiscoveryDenylist` config field (`[]string`, supports `symbol@exchange` tuples). Match → `score=0`, `why_rejected=denylist`. Editable via the same registry chokepoint.
- **D-08:** Symbols with only one exchange listing are **skipped silently** with a `symbol_no_cross_pair` counter incremented in `pg:scan:metrics`.

**CandidateRegistry Chokepoint**
- **D-09:** Typed methods: `Add(ctx, candidate)`, `Update(ctx, idx, candidate)`, `Delete(ctx, idx)`, `Replace(ctx, []candidate)`. Each does lock → reload from disk → mutate → atomic write → release. Idempotent dedupe by `(symbol, longExch, shortExch, direction)` tuple inside `Add`.
- **D-10:** Registry lives in **new `internal/pricegaptrader/registry.go`**. Holds `*config.Config` reference. `cfg.mu` (`config.go:70`) remains authoritative lock. Module boundary preserved.
- **D-11:** **Hard-cut migration**: dashboard `POST /api/config` candidate path AND `cmd/pg-admin` BOTH call registry methods directly. No legacy `cfg.PriceGapCandidates = …` path remains. Concurrent-writer integration test required.
- **D-12:** **Timestamped `.bak` ring**: atomic write to `config.json.tmp` → `fsync` → `os.Rename` to `config.json` → write `config.json.bak.{unix_ts}` → prune to newest 5.
- **D-13:** Scanner read-only enforced at **compile time** via `RegistryReader` interface (only `Get`/`List`). Scanner constructor accepts `RegistryReader`, never `*Registry`.

**Discovery Telemetry & Dashboard**
- **D-14:** Discovery section is a **sub-section inside the existing PriceGap dashboard tab** in Phase 11. Phase 16 (PG-OPS-09) will move it to the new top-level Price-Gap tab.
- **D-15:** Phase 11 Discovery section content: (1) Cycle stats card; (2) Per-candidate score history chart (Recharts line, 7d window from `pg:scan:scores:{symbol}` ZSET, threshold band overlay); (3) Why-rejected breakdown table/pie; (4) Empty placeholder card for Phase 12 promote/demote timeline.
- **D-16:** Discovery section is **always visible** with "Scanner OFF" banner when `PriceGapDiscoveryEnabled=false`.
- **D-17:** Update freshness: **REST seed + WS throttled push**. WS subscriptions on `pg:scan:cycle` (per-cycle), `pg:scan:metrics` (5s throttle), `pg:scan:score` (debounced).

### Claude's Discretion

- Magnitude weight specifics (spread vs depth vs funding contribution within 0–100 magnitude post-gates).
- Gate evaluation order (short-circuit ordering for performance).
- Exact REST route paths for `/api/pg/discovery/*`.
- Specific Recharts component composition / styling.
- Concurrent-writer test harness shape (success criterion #3 integration test).
- Bootup behavior when scanner starts mid-cycle (skip / force-immediate / wait-for-next-tick).
- Scanner per-cycle error budget (consecutive ticker-read failures before circuit-break-into-cycle-metrics).
- Initial seed values for `PriceGapDiscoveryUniverse` and `PriceGapDiscoveryDenylist` (defaults to empty slices).

### Deferred Ideas (OUT OF SCOPE)

- **PG-OPS-09 top-level Price-Gap tab** → Phase 16.
- **Auto-promotion path (PG-DISC-02)** → Phase 12. Phase 11 ships the registry but scanner cannot call mutators (compile-time blocked).
- **Score-vs-realized-fill calibration view (PG-DISC-05)** → v2.3.
- **ML-based scoring** — anti-feature.
- **Per-tick scanner scoring** — anti-feature; cycle-driven only.
- **Auto-tune `PriceGapAutoPromoteScore`** — sample size too small.
- **Universe auto-derivation from `pg:history`** — explicitly rejected in D-05.
- **Continuous-curve ramp / promotion** — discrete only.
</user_constraints>

---

## Project Constraints (from CLAUDE.local.md)

| Directive | Implication for Phase 11 |
|-----------|--------------------------|
| `npm install` / `npm update` / `npx` / `pnpm install` FORBIDDEN; only `npm ci` | Discovery UI must reuse existing Recharts, no new deps |
| `config.json` is the live runtime config — DO NOT touch directly; use `cfg.SaveJSON()` only | Registry calls `cfg.SaveJSON()`; no `os.WriteFile` shortcuts |
| Frontend builds BEFORE Go binary (`go:embed`); `make build` order is non-negotiable | Plan must include `make build-frontend && go build` step |
| Module boundary: `internal/pricegaptrader/` MUST NOT import `internal/engine` or `internal/spotengine` | Registry + scanner + telemetry stay inside pricegaptrader; chokepoint takes `*config.Config` directly |
| New feature rollout: config bool (default OFF) + dashboard toggle + persisted via `SaveJSON` | `PriceGapDiscoveryEnabled` ships default `false` |
| `gofmt` + stdlib idioms; interface-based DI for cross-package boundaries | `RegistryReader` interface (Get/List); scanner constructor takes the interface |
| i18n: en + zh-TW MUST stay in lockstep; `TranslationKey` is derived from EN | Every new key (Discovery section labels, "Scanner OFF" banner, why-rejected reasons) added to BOTH locale files |
| Internal symbol form `BTCUSDT` (uppercase, no separators); per-exchange map inside adapters | Scanner enforces canonical form once at universe ingest; cross-exchange BBO join uses canonical key |
| Loris funding rates are 8h-equivalent — always ÷8 for bps/h | If funding contributes to magnitude, divide by 8 first |
| Bybit `:04–:05:30` blackout | Scanner skips Bybit ticker reads in this window when `cfg.BybitBlackoutEnabled` |
| Versioning: every commit bumps `VERSION` + updates `CHANGELOG.md` | Wave-N planning includes the version bump as a final task |

---

## Summary

Phase 11 ships three coupled deliverables inside `internal/pricegaptrader/`:

1. **Read-only scanner** (`scanner.go`): periodic goroutine launched by `Tracker.Start()` (mirroring the existing `tickLoop` + `assertBBOLiveness` pattern). On each cycle, walks `cfg.PriceGapDiscoveryUniverse` × all-cross of `cfg.SupportedExchanges`, samples BBO via existing `Exchange.GetBBO`, runs gates, computes 0–100 magnitude, writes per-cycle records to `pg:scan:*`. Default OFF via `PriceGapDiscoveryEnabled`.

2. **CandidateRegistry chokepoint** (`registry.go` — PG-DISC-04): `*Registry` holds `*config.Config`, exposes `Add/Update/Delete/Replace`. Each method takes `cfg.mu`, reloads from disk, mutates, calls `cfg.SaveJSON()` (extended to write `config.json.bak.{unix_ts}` and prune ring to 5), releases. Compile-time read-only via `RegistryReader` interface for the scanner.

3. **Discovery telemetry** (`telemetry.go` — PG-DISC-03): centralizes Redis writes + WS broadcasts for scanner output. Dashboard sub-section under existing PriceGap tab shows cycle stats card, score-history Recharts chart, why-rejected breakdown, and an empty placeholder card for Phase 12.

**Primary recommendation:** Decompose into 5 plans matching CONTEXT decisions: (1) Config schema + RegistryReader interface; (2) `CandidateRegistry` + `.bak` ring + atomic save; (3) Hard-cut migration of dashboard handler + pg-admin; (4) Scanner goroutine + gates + score + telemetry writes; (5) Discovery dashboard section + i18n + WS subscriptions. Plan 2 is the hard prerequisite for Plan 4 (scanner cannot land before chokepoint is the only writer).

---

## Standard Stack (verified in repo)

### Core (no new dependencies — npm lockdown + Go zero-new-deps target)

| Library / API | Version | Purpose | Source |
|---------------|---------|---------|--------|
| Go stdlib `sync.RWMutex` | Go 1.26 | `cfg.mu` (already at `config.go:70`) — registry reuses; no new lock | [VERIFIED: config.go:70] |
| `time.Ticker` | Go stdlib | Scanner cadence loop (mirrors `tickLoop` at `tracker.go:355`) | [VERIFIED: tracker.go:355–397] |
| `pkg/exchange.Exchange.GetBBO(symbol)` | in-tree | BBO source for scanner; returns `(BBO, ok)` | [VERIFIED: detector.go:139–146] |
| `pkg/exchange.Exchange.SubscribeSymbol(symbol)` | in-tree | Pre-warm BBO stream for new universe symbols | [VERIFIED: tracker.go:240–263] |
| `internal/database.Client` (Redis DB 2) | in-tree | Telemetry writes to `pg:scan:*` | [VERIFIED: existing `pg:` namespace usage] |
| Recharts (frontend) | locked in `web/package-lock.json` | Score-history line chart (existing `web/src/components/PnLChart.tsx` pattern) | [VERIFIED: web/src/components/PnLChart.tsx exists] |

### Supporting

| Library | Purpose | When to Use |
|---------|---------|-------------|
| `golang.org/x/sync/errgroup` | NOT used by Tracker today (uses `sync.WaitGroup` + `stopCh` channel pattern; see `tracker.go:46–48`). Phase 11 follows the same `WaitGroup + stopCh` discipline. | Add scanner via `t.wg.Add(1); go t.scanLoop()` mirroring `assertBBOLiveness` |
| `os.Rename` + `*os.File.Sync()` | Atomic config.json write + ring rotation | Inside `cfg.SaveJSON()` extension for `.bak.{ts}` ring |
| `models.PriceGapCandidate` | Existing struct with `Direction` field (v0.35.0) | Registry's `Add` dedupe tuple is `(Symbol, LongExch, ShortExch, Direction)` |

### Alternatives Considered (rejected by CONTEXT.md)

| Instead of | Could Use | Reason rejected |
|------------|-----------|-----------------|
| `internal/pricegaptrader/registry.go` | `internal/config/registry.go` | Expands config package surface; risks future writers bypassing it (CONTEXT D-10) |
| `RegistryReader` compile-time | Runtime gate inside Registry methods | Scanner could still call mutators; weaker (CONTEXT D-13) |
| Single timestamped backup | Append-only audit log | Pitfall 2 specifies ring of N `.bak.{ts}` (CONTEXT D-12) |
| Generic `Mutate(ctx, fn)` | Typed methods | Harder to audit at call sites (CONTEXT D-09) |

---

## Architecture Patterns

### Recommended Project Structure

```
internal/pricegaptrader/
├── tracker.go         # EXISTING — Tracker.Start() launches scanner goroutine alongside tickLoop
├── detector.go        # EXISTING — barRing.allExceedDirected pattern; scanner mirrors persistence gate
├── risk_gate.go       # EXISTING — BBO freshness gate pattern; scanner reuses staleness threshold
├── metrics.go         # EXISTING — pure aggregator pattern; telemetry follows same envelope
├── registry.go        # NEW (Plan 2) — CandidateRegistry + RegistryReader interface
├── scanner.go         # NEW (Plan 4) — read-only scanner goroutine, cycle output to pg:scan:*
└── telemetry.go       # NEW (Plan 5) — centralizes Redis writes + WS broadcasts for scanner

internal/api/
└── pricegap_handlers.go  # MODIFIED — POST /api/config candidate path migrates to registry
                          # NEW: GET /api/pg/discovery/state, GET /api/pg/discovery/scores/{symbol}

cmd/pg-admin/
└── main.go            # MODIFIED — add commands that mutate candidates, route through registry

web/src/
├── pages/PriceGap.tsx (or equivalent)   # MODIFIED — add Discovery sub-section
├── components/Discovery*.tsx            # NEW — cycle stats card, score history chart,
│                                        #       why-rejected breakdown, placeholder timeline
├── hooks/usePgDiscovery.ts              # NEW — REST seed + WS subscription wiring
└── i18n/en.ts + i18n/zh-TW.ts           # MODIFIED — new keys (lockstep mandatory)
```

### Pattern 1: Scanner Goroutine (mirrors `tickLoop`)

**What:** Launch from `Tracker.Start()` via `t.wg.Add(1); go t.scanLoop()`. Scan loop is gated by `cfg.PriceGapDiscoveryEnabled`; when disabled, the goroutine still runs but every iteration is a no-op (write-of-zero `pg:scan:metrics` so dashboard "Scanner OFF" banner has data to render).

**When to use:** Scanner is conceptually independent of detection (which is per-tick over `cfg.PriceGapCandidates`). Scanner walks `cfg.PriceGapDiscoveryUniverse` cross-product instead.

**Example pattern (verified against `tracker.go:355–397`):**

```go
// Source: internal/pricegaptrader/tracker.go (existing tickLoop pattern)
func (t *Tracker) scanLoop() {
    defer t.wg.Done()
    interval := time.Duration(t.cfg.PriceGapDiscoveryIntervalSec) * time.Second
    if interval <= 0 { interval = 5 * time.Minute }
    firstTick := time.After(13 * time.Second)  // offset off Bybit blackout boundary
    var ticker *time.Ticker
    for {
        select {
        case <-t.stopCh:
            if ticker != nil { ticker.Stop() }
            return
        case <-firstTick:
            ticker = time.NewTicker(interval)
            t.runScanCycle(time.Now())
        case now := <-chanTick(ticker):
            t.runScanCycle(now)
        }
    }
}
```

### Pattern 2: CandidateRegistry Chokepoint (mirrors `cfg.mu` + `SaveJSON` discipline)

**What:** All `cfg.PriceGapCandidates` mutations route through `*Registry`. Registry uses the existing `cfg.mu` (no new lock) — but as a **single chokepoint type** so writers can't bypass it.

**When to use:** Every site currently doing `s.cfg.PriceGapCandidates = next` becomes `s.registry.Replace(ctx, next)`.

**Pattern:**

```go
// internal/pricegaptrader/registry.go (NEW)
type Registry struct {
    cfg *config.Config // takes cfg.mu inside methods
    log *utils.Logger
}

type RegistryReader interface {
    Get(idx int) (models.PriceGapCandidate, bool)
    List() []models.PriceGapCandidate
}

func (r *Registry) Add(ctx context.Context, c models.PriceGapCandidate) error {
    r.cfg.Lock()
    defer r.cfg.Unlock()
    // 1. Reload from disk to defeat stale-in-memory writers (Pitfall 2 belt-and-suspenders)
    // 2. Dedupe by (Symbol, LongExch, ShortExch, Direction) — idempotent (D-09)
    // 3. Enforce PriceGapMaxCandidates cap
    // 4. Append in-memory
    // 5. Call r.cfg.SaveJSON() — atomic .tmp → fsync → rename + .bak.{ts} ring
    return nil
}
```

### Pattern 3: REST Seed + WS Push (existing dashboard convention)

**What:** Discovery section initial mount calls `GET /api/pg/discovery/state` for current cycle stats + score history snapshot. WS subscribes to `pg:scan:cycle`, `pg:scan:metrics`, `pg:scan:score` channels. Existing pattern reused from PnL/positions panels.

**Verified:** `tracker.go:209–225` shows the throttled WS broadcast envelope used today; `pricegap_handlers.go:186` shows the existing GET handler pattern.

### Anti-Patterns to Avoid

- **Don't** import `internal/engine` or `internal/spotengine` from `internal/pricegaptrader` — module boundary [VERIFIED: tracker.go:1–22 docstring].
- **Don't** add a second config save path (e.g., direct `os.WriteFile`) — bypasses `keepNonZero` tripwire at `config.go:1519` and the new `.bak` ring [VERIFIED: config.go:1519–1544].
- **Don't** allow scanner to take `*Registry`; only `RegistryReader` (CONTEXT D-13).
- **Don't** schedule scanner inside the perp-perp `:10/:20/:30/:35/:40/:45/:50` dispatcher — coupling unrelated lifecycles (Architecture research §1).
- **Don't** call `cfg.SaveJSON()` while holding `cfg.mu` if `SaveJSON` itself takes the lock — verify and use the existing pattern (handlers.go uses `s.cfg.Unlock()` before responding but holds the lock through the candidate mutation; the registry should follow the same shape).

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 4-bar persistence ring | New ring buffer | `barRing` from `detector.go:21–109` | Already covers same-sign + magnitude + dedup-within-minute; PG-DIR-01 closure |
| BBO freshness check | New staleness gate | Same `cfg.PriceGapKlineStalenessSec` + per-candidate wall-clock interval pattern from `detector.go:194–201` | Pkg/exchange.BBO carries no timestamp — wall-clock interval is the proven approach |
| Cross-exchange symbol normalization | `strings.ToUpper`/`ReplaceAll` in scanner | Existing canonical form `BTCUSDT` enforced once at universe ingest; per-exchange mapping inside adapters | Pitfall 1 warning sign #3 |
| Atomic config.json write | Custom temp-file logic | Extend `cfg.SaveJSON()` (`config.go:1572`) to add `.bak.{ts}` ring | Already handles `keepNonZero` tripwire, raw-map preservation, exchange-secret protection |
| WS broadcast envelope | New event type | Existing `Broadcaster.BroadcastPriceGapPositions` pattern + new typed methods (`BroadcastPgScanCycle`, `BroadcastPgScanMetrics`, `BroadcastPgScanScore`) | Tracker already injects via `SetBroadcaster` (`tracker.go:140–146`) — new methods slot in identically |
| Ticker reads with rate limits | Custom rate-limiter | `Exchange.GetBBO` reads in-process WS cache (no rate-limit cost); `SubscribeSymbol` is the only wire-side cost | BBO is push-cached, not pull-fetched |
| Distributed lock for scanner | New `pg:lock:scan` | None needed — scanner is read-only; only the chokepoint needs serialization, and `cfg.mu` already provides it | Architecture research §"Concurrency Invariants" |

**Key insight:** Phase 11's value is composing existing primitives correctly. The only genuinely new logic is (a) the score-magnitude formula above gates, (b) the universe cross-product walker, and (c) the `.bak.{ts}` ring rotation inside `SaveJSON`.

---

## Specific Technical Approach (per focus area)

### 1. Scanner Architecture

- **Goroutine host:** `Tracker.Start()` — same lifecycle owner as existing `tickLoop`. Add `t.wg.Add(1); go t.scanLoop()` after the existing `assertBBOLiveness` launch.
- **Cadence:** `cfg.PriceGapDiscoveryIntervalSec` (default 300s = 5 min). Same `firstTick` offset trick as `tickLoop` (constant ≠ 7s to avoid scanner + detector colliding on the same `:00` boundary; suggest 13s).
- **Coexistence:** Scanner runs OUTSIDE the perp-perp `:10/:20/:30/...` schedule (which is funding-window-aligned and irrelevant to pricegap). Coexists with `tickLoop` (per-30s detection over `cfg.PriceGapCandidates`) — they share the BBO cache via `Exchange.GetBBO` but have disjoint candidate sets.
- **Bybit blackout:** Scanner reads `cfg.BybitBlackoutEnabled` (existing field — verified in `pricegaptrader/tracker.go` startup-offset comment) and skips Bybit ticker reads in `:04–:05:30`. Skip = score=0 with `why_rejected=bybit_blackout` for that exchange-leg only; other exchanges in the cross-product still scan.
- **Disabled state:** When `PriceGapDiscoveryEnabled=false`, scanner still ticks but writes only an "OFF" stamp to `pg:scan:metrics` (so the dashboard "Scanner OFF" banner can render `last_run_at = none`).
- **Bootup behavior (Claude's discretion):** RECOMMEND **wait-for-next-tick**. The 13s `firstTick` offset already handles startup blackout avoidance; forcing an immediate cycle creates a thundering-herd problem when the binary drift monitor restarts the process during a Bybit blackout edge.
- **Per-cycle error budget (Claude's discretion):** RECOMMEND **5 consecutive ticker-read failures across exchanges** (mirrors v1.0 PlaceOrder breaker `D-10` at 5) → emit `cycle_failed` to `pg:scan:metrics` and skip remaining work for that cycle. Resets on next successful ticker read.

### 2. Universe Selection & Cross-Exchange BBO Collection

- **Universe field:** `PriceGapDiscoveryUniverse []string` in `*config.Config`, paired with a `[]string` JSON shape in `jsonPriceGap`. Validate at config load: ≤20 entries, each matches `^[A-Z0-9]{2,20}USDT$` (reuse `validatePriceGapCandidates` regex per Pitfall 1 §"Auto-promotion accepts unsanitized symbol").
- **Pairing (D-06):** For each `symbol` in universe, generate every ordered `(long_exch, short_exch)` pair from the supported-exchange set where the symbol is listed on BOTH. Cardinality: 6×5 = 30 ordered pairs per symbol max. Skip exchange-leg if `Exchange.GetBBO(symbol)` returns `ok=false` (not subscribed/listed).
- **Symbol no-cross-pair (D-08):** If only one exchange in the universe has the symbol, increment `pg:scan:metrics` counter `symbol_no_cross_pair` and emit nothing else. No per-symbol record.
- **BBO collection:** Reuse `sampleLegs(t.exchanges, candidate)` pattern from `detector.go:126–152` directly — synthesize a transient `models.PriceGapCandidate` shape per scanned pair (just for the BBO read; no persistence). Returns `(midLong, midShort, bboOK, err)`.
- **Symbol normalization:** Universe entries are stored canonical (`BTCUSDT`). Scanner normalizes once at universe-ingest validation. Per-exchange mapping (Gate `BTC_USDT`, OKX `BTC-USDT-SWAP`) happens inside each adapter — scanner never sees those forms [VERIFIED: tracker.go:21 docstring; CLAUDE.local.md "internal symbol form"].

### 3. Persistence / Freshness / Depth Gates

| Gate | Mechanism | Source pattern |
|------|-----------|----------------|
| **≥4-bar persistence** | Per-(symbol, long_exch, short_exch) `barRing` (new map keyed by canonical pair tuple, parallel to `Tracker.bars`). On each cycle, push 1m close; check `allExceedDirected(T, "bidirectional")` since scanner is direction-agnostic at discovery time. T defaults to `cfg.PriceGapDiscoveryThresholdBps` (new field, default = `MaxPriceGapBPS / 2` ≈ 100bps). | `barRing.allExceedDirected` at `detector.go:54–78` |
| **BBO freshness <90s** | Per-(symbol, exchange) wall-clock interval since last successful sample; if gap ≥ `cfg.PriceGapKlineStalenessSec` (existing field, default 90), reset ring. Same logic as `detectOnce` `cb.lastSampleAt` check. | `detector.go:194–201` |
| **Depth probe** | NEW logic. Scanner calls `Exchange.GetOrderBook(symbol, depth)` (verify in pkg/exchange — likely exists per existing depth-fill executor) and computes notional-clearable at threshold spread. If clearable < `cfg.PriceGapDiscoveryMinDepthUSDT` (new field, default 1000), gate fails. **Open question:** confirm `GetOrderBook` is in the 35-method `Exchange` interface or only in some adapters — if not interface-level, scanner uses BBO size as a proxy (less accurate but cheaper). |
| **Denylist** | `(symbol, exchange)` tuple OR canonical `symbol` match against `cfg.PriceGapDiscoveryDenylist`. Match → `score=0, why_rejected=denylist`. | New |
| **Cross-exchange pair exists** | Both `Exchange.GetBBO` returns `ok=true` AND symbol is in both adapters' subscription set. | `detector.go:139–146` |

### 4. Scoring (CONTEXT D-01 to D-04)

**Shape:** gate-then-magnitude. ALL gates pass → magnitude in 0–100. ANY gate fails → score=0 + reason code.

**Magnitude formula (Claude's discretion — recommend):**
```
magnitude = clamp(
    0.5 * spread_excess_bps_norm   // (|spread| - T) / T capped at 1.0
  + 0.3 * depth_score              // clearable_usdt / target_usdt capped at 1.0
  + 0.2 * funding_alignment        // funding favors trade direction (Loris ÷8 first)
, 0, 1) * 100
```
Funding contribution is bounded but not gating — the sign helps; absent funding data ⇒ component=0. **All sub-components persisted** per CONTEXT D-03 so PG-DISC-05 can recalibrate weights without reshipping the schema.

**Why-rejected reason codes (CONTEXT D-04):**
- `insufficient_persistence` (mirror `detector.go:226`)
- `stale_bbo` (mirror `detector.go:220`)
- `insufficient_depth`
- `denylist`
- `bybit_blackout`
- `symbol_not_listed_long` / `symbol_not_listed_short`
- `sample_error: <detail>`

### 5. CandidateRegistry Chokepoint (PG-DISC-04)

**Existing direct mutations to migrate (verified by grep):**

| File:line | Mutation | Migrates to |
|-----------|----------|-------------|
| `internal/api/handlers.go:1700` | `s.cfg.PriceGapCandidates = next` (POST /api/config) | `s.registry.Replace(ctx, next)` |
| `internal/config/config.go:1439` | `c.PriceGapCandidates = pg.Candidates` (config load) | **Stays** — load path is single-threaded boot, no chokepoint needed |
| `cmd/pg-admin/main.go` | (no mutation today; pg-admin only writes Redis disable flags) | NEW commands (e.g., `pg-admin candidates list/add/delete`) call registry directly via in-process import — pg-admin links `internal/pricegaptrader/registry.go` |

**pg-admin in-process vs RPC (Claude's discretion):** RECOMMEND **in-process import**. pg-admin already imports `internal/config` and `internal/database`; adding `internal/pricegaptrader/registry` is symmetric. The dashboard daemon is NOT running when pg-admin runs (operator stops the bot first per Pitfall 2 §"recovery"). Concurrent dashboard+pg-admin writes happen only when the operator runs pg-admin while the bot is live — that's exactly the case the chokepoint protects against (both processes lock on `cfg.mu` via the same registry struct, but they're two separate processes ⇒ the chokepoint is `config.json` itself via reload-from-disk + atomic write). The integration test for success-criterion #3 must exercise this two-process model, not just two goroutines in one process.

**Atomic write + ring (CONTEXT D-12):** Extend `cfg.SaveJSON` (or add a sibling helper called by SaveJSON):
```
1. write config.json.tmp
2. f.Sync() (fsync)
3. os.Rename(config.json.tmp, config.json)
4. write config.json.bak.{unix_ts} (copy of post-rename content)
5. list config.json.bak.* sorted by ts desc
6. delete all but newest 5
```
Existing single-`.bak` behavior (in `SaveJSON`) is replaced by step 4–6. The `keepNonZero` tripwire (`config.go:1519`) and exchange-secret preservation logic stay intact.

**Compile-time read-only (CONTEXT D-13):**
```go
type RegistryReader interface {
    Get(idx int) (models.PriceGapCandidate, bool)
    List() []models.PriceGapCandidate
}
```
`*Registry` implements `RegistryReader`. Scanner constructor: `NewScanner(reader RegistryReader, ...)`. Phase 12 swaps to `*Registry`. Test asserts no scanner-package symbol references `Add`/`Update`/`Delete`/`Replace`.

### 6. Redis Schema (extends Architecture research §4)

| Key | Type | Writer | TTL | Cardinality | WS Channel |
|-----|------|--------|-----|-------------|------------|
| `pg:scan:cycles` | LIST (rpush + ltrim 1000) | scanner | none (capped) | 1 list, ~1000 entries | `pg:scan:cycle` |
| `pg:scan:scores:{symbol}` | ZSET (score, ts) | scanner | 7d (`EXPIRE` 604800) | one per universe symbol | `pg:scan:score` (debounced) |
| `pg:scan:rejections:{date}` | HASH (reason → count) | scanner | 30d | one per UTC day | (folded into pg:scan:cycle) |
| `pg:scan:metrics` | HASH (last_run_at, candidates_seen, accepted, rejected, errors, next_run_in, symbol_no_cross_pair) | scanner | none | 1 | `pg:scan:metrics` (5s throttle) |
| `pg:scan:enabled` | STRING ("0"/"1") | telemetry | none | 1 | (read on REST seed only) |
| `pg:registry:audit` | LIST (rpush + ltrim 200) | registry | none (capped) | 1 list — captures `{ts, source, op, before_count, after_count}` | (none — read on demand) |

**Atomic patterns:** `MULTI/EXEC` for `pg:scan:cycles` rpush + `pg:scan:metrics` HSET in one cycle. SET-NX-LUA distributed locks NOT needed (scanner is read-only; chokepoint is process-local + file-system).

### 7. Dashboard Discovery Section (PG-DISC-03)

**Placement (CONTEXT D-14):** Sub-section inside the existing PriceGap tab (`web/src/pages/PriceGap.tsx` or equivalent — verified by `web/src/i18n/en.ts:13` `'nav.pricegap': 'Price-Gap'`).

**Data contract (UI-SPEC.md will detail visuals):**

| Endpoint | Response shape | Purpose |
|----------|---------------|---------|
| `GET /api/pg/discovery/state` | `{ enabled: bool, last_run_at: int, candidates_seen: int, accepted: int, rejected: int, errors: int, next_run_in: int, why_rejected: { reason: count }, score_snapshot: [ { symbol, score, sub_scores, gates_passed, gates_failed } ] }` | Initial REST seed for cycle stats + why-rejected + current scores |
| `GET /api/pg/discovery/scores/{symbol}` | `{ symbol, points: [ {ts, score, sub_scores} ], threshold_band: {auto_promote: int} }` | 7d ZSET dump for score-history chart |
| WS `pg:scan:cycle` | full latest cycle envelope | per-cycle push (no throttle) |
| WS `pg:scan:metrics` | metrics HASH | 5s throttle |
| WS `pg:scan:score` | `{symbol, score, sub_scores, ts}` | debounced |

**i18n keys (en + zh-TW lockstep mandatory):**
- `pricegap.discovery.title`
- `pricegap.discovery.scannerOff`
- `pricegap.discovery.lastRun`, `pricegap.discovery.candidatesSeen`, `.accepted`, `.rejected`, `.errors`, `.nextRunIn`
- `pricegap.discovery.scoreHistory.title`, `.thresholdBand`
- `pricegap.discovery.whyRejected.title`, `.reason.{insufficient_persistence|stale_bbo|insufficient_depth|denylist|bybit_blackout|symbol_not_listed_long|symbol_not_listed_short|sample_error}`
- `pricegap.discovery.timeline.placeholder` (Phase 12 fills this)

### 8. Score-history chart (Recharts)

Reuse the `web/src/components/PnLChart.tsx` wrapper pattern [VERIFIED: file exists]. Component shape:
- `<ResponsiveContainer><LineChart><Line dataKey="score" /><ReferenceArea y1={threshold} y2={100} /></LineChart></ResponsiveContainer>`
- Threshold band overlay reads `cfg.PriceGapAutoPromoteScore` (a Phase 12 field, but Phase 11 displays it for context — CONTEXT D-15 #2). Phase 11 introduces the field schema; Phase 12 adds the action.

---

## Common Pitfalls

### Pitfall 1: Scanner false-positive pump (Pitfall 1 from PITFALLS.md)
**What goes wrong:** Single-bar spread cross on a thin SOON listing scores high.
**Why:** Raw spread magnitude without persistence/freshness/depth gates.
**How to avoid:** ALL gates pass before non-zero score (D-01); ≥4-bar same-sign via `barRing.allExceedDirected`; BBO <90s on both legs; depth probe ≥ floor; symbol normalization at ingest; Loris funding ÷8.
**Warning signs:** Score formula referencing `spread` without `persistence`/`bars_above`/`consecutive`; missing denylist consult; `strings.ToUpper`/`ReplaceAll` on symbols outside adapter boundaries.

### Pitfall 2: Three-writer race on cfg.PriceGapCandidates
**What goes wrong:** Dashboard, pg-admin, scanner-promotion all `append` and call `SaveJSON` concurrently → silent mutation loss.
**Why:** v2.1 added second writer (Phase 10 dashboard CRUD); scanner promotion (Phase 12) would be the third without the chokepoint.
**How to avoid:** Phase 11 lands the registry NOW. Hard-cut migration: dashboard handler + pg-admin both call `registry.Replace`/`Add`/`Delete`. Scanner constructor takes `RegistryReader` — compile-time blocked from mutating. `.bak.{ts}` ring of 5.
**Warning signs:** `cfg.PriceGapCandidates = …` outside `registry.go`; `SaveJSON` called from more than one helper; missing tuple-equality check before append.

### Pitfall 3: i18n EN/zh-TW drift
**What goes wrong:** New `pricegap.discovery.*` keys land in EN but not zh-TW → `TranslationKey` type errors at build time.
**How to avoid:** Add ALL new keys to both files in the same commit; verify `npm run typecheck` passes; CLAUDE.local.md warns this is a recurring failure mode (Phase 999.1 burned us here).

### Pitfall 4: `keepNonZero` tripwire blocks `.bak` ring writes
**What goes wrong:** Test fixture passes `&Config{}` to a registry that calls `SaveJSON`, which sees in-memory zeroes and the tripwire warns + preserves disk values — but ring rotation still writes a backup of zeroed-out test state.
**How to avoid:** Tests MUST set `CONFIG_FILE` to a sandbox path (per `config.go:1583–1593` post-incident comment). Registry tests that exercise `SaveJSON` round-trip use a tmpdir + `os.Setenv("CONFIG_FILE", …)`.

### Pitfall 5: Scanner BBO subscription not pre-warmed
**What goes wrong:** Universe symbol exists on Gate but `SubscribeSymbol` was never called; `GetBBO` returns `ok=false` forever; scanner perpetually reports `sample_error` instead of `stale_bbo`.
**How to avoid:** On `Tracker.Start()`, after the existing `subscribeCandidates()` call, add `subscribeUniverse()` that fans out `SubscribeSymbol` for every (symbol, exchange) in the universe-cross-product. Mirrors `tracker.go:239–263` exactly. Add same fail-loud `assertBBOLiveness` extension after the 15s grace window.

### Pitfall 6: Scanner cycle stalls on slow exchange
**What goes wrong:** One adapter's `GetBBO` hangs (WS reconnecting); scanner holds the cycle open indefinitely.
**How to avoid:** `GetBBO` returns from in-process cache (no I/O); slow case is when the cache is empty. Per-(symbol, exchange) read has a hard timeout via `context.WithTimeout(2*time.Second)` if calling depth probe (`GetOrderBook` does I/O). Cycle as a whole has a `5s × len(universe)` wall-clock budget — exceeded → emit `cycle_timeout` and move on.

---

## Code Examples (verified patterns from in-tree sources)

### Existing barRing usage — scanner mirrors this for per-(symbol, pair) state

```go
// Source: internal/pricegaptrader/detector.go:54–78
func (b *barRing) allExceedDirected(T float64, direction string) bool {
    var firstSign float64
    for i, ok := range b.valid {
        if !ok { return false }
        s := b.closes[i]
        if math.Abs(s) < T { return false }
        if i == 0 { firstSign = s } else if s*firstSign < 0 { return false }
    }
    if direction != models.PriceGapDirectionBidirectional && firstSign < 0 {
        return false
    }
    return true
}
```

### Existing SaveJSON safety (extends with `.bak.{ts}` ring)

```go
// Source: internal/config/config.go:1572–1610
func (c *Config) SaveJSON() error {
    return c.SaveJSONWithExchangeSecretOverrides(nil)
}
// SAFETY: explicit CONFIG_FILE env var OR config.json in cwd; no absolute fallback.
// keepNonZero tripwire at line 1519 protects critical numeric fields from zero overwrites.
```

### Existing dashboard candidate migration target

```go
// Source: internal/api/handlers.go:1665–1701
// CURRENT (will migrate to s.registry.Replace(ctx, next)):
if pg.Candidates != nil {
    next := make([]models.PriceGapCandidate, len(*pg.Candidates))
    copy(next, *pg.Candidates)
    // ... normalize, validate, guardActivePositionRemoval ...
    s.cfg.PriceGapCandidates = next   // <-- this becomes registry.Replace(ctx, next)
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Direct `cfg.PriceGapCandidates = …` mutation in handler | `*Registry.Replace(ctx, next)` chokepoint | Phase 11 (this) | Scanner promotion (Phase 12) cannot bypass; concurrent writers serialize |
| Single `config.json.bak` overwritten on every save | Timestamped ring of 5 (`config.json.bak.{unix_ts}`) | Phase 11 (this) | Lost-mutation recovery via diff against ring |
| Scanner inside perp-perp `:10/:20/:30/...` schedule | Scanner in pricegaptrader Tracker errgroup with own ticker | Phase 11 (this) | Pricegap lifecycle independent; Bybit blackout only skips scanner Bybit reads |

**Deprecated/outdated:**
- (none — all baseline patterns from Phase 8/9/10/999.1 remain authoritative)

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `pkg/exchange.Exchange` interface includes `GetOrderBook(symbol, depth)` for the depth-probe gate | §3 Persistence/Freshness/Depth Gates | If only some adapters expose this, scanner falls back to BBO-size proxy (less accurate); planner must verify against `pkg/exchange/exchange.go` interface during Wave 0. [ASSUMED — depth-fill executor uses depth, but interface-level method needs confirmation] |
| A2 | `cfg.SaveJSON()` does NOT take `cfg.mu` internally — it expects the caller to hold the lock | §"CandidateRegistry chokepoint" | If SaveJSON does take the lock, `Registry` methods must release before calling and re-acquire (or restructure). Planner must verify by reading `config.go:1572` body. [ASSUMED — verified `cfg.mu` is held in handlers.go around line 1700 before SaveJSON, so caller-holds-lock is the pattern, but explicit confirmation needed] |
| A3 | The dashboard daemon and pg-admin do NOT run concurrently in production (operator stops the bot before running pg-admin) | §5 CandidateRegistry chokepoint | If they DO run concurrently, the chokepoint is `config.json` reload-from-disk + atomic-write + ring (which is the design); the in-process mutex is sufficient even cross-process because of the atomic rename. So this assumption is belt-and-suspenders, not load-bearing. [ASSUMED — pg-admin design doc says "run while bot offline"; not enforced] |
| A4 | `cfg.BybitBlackoutEnabled` exists today | §1 Scanner architecture | If only `cfg.PriceGapBlackoutEnabled` or similar variant, planner must adjust. [ASSUMED — referenced in Architecture research and tracker.go startup-offset comment, but exact field name to be verified] |
| A5 | `web/src/pages/PriceGap.tsx` is the host file for the existing PriceGap tab | §"Recommended Project Structure" | If file path differs, planner adjusts but Discovery section still goes inside the existing PriceGap page. [ASSUMED — i18n keys reference `pricegap.*` namespace; `web/src/pages/` exists per ls] |
| A6 | Recharts `<LineChart>` + `<ReferenceArea>` are already imported elsewhere in the dashboard (no new npm install needed) | §8 Score-history chart | If only `<LineChart>` is used today and `<ReferenceArea>` requires a different import, npm lockdown still permits using already-locked Recharts subcomponents. Risk: zero (Recharts is bundled). [ASSUMED — PnLChart.tsx exists; full Recharts surface available] |

**Three of these (A1, A2, A4) are HIGH-priority verifications during Wave 0** — A1 because depth probe gate design hinges on it; A2 because incorrect lock ordering deadlocks the registry; A4 because Bybit blackout skip is a Pitfall 4 false-trip vector.

---

## Open Questions

1. **`Exchange.GetOrderBook` interface coverage** (A1)
   - What we know: existing depth-fill executor uses orderbook depth.
   - What's unclear: whether ALL 6 adapters implement it at the interface level, or only those used by depth-fill.
   - Recommendation: Wave 0 task — grep `pkg/exchange/exchange.go` for the interface; if not interface-level, downgrade depth probe to BBO-size proxy and document.

2. **Scanner-vs-detector candidate overlap**
   - What we know: scanner walks universe; detector walks `cfg.PriceGapCandidates`.
   - What's unclear: should scanner SKIP symbols already in `PriceGapCandidates` (already-promoted) or score them anyway for telemetry parity?
   - Recommendation: score them anyway (so dashboard shows score for promoted candidates too — useful for Phase 12 demote logic). Mark with `already_promoted: true` flag in the cycle record.

3. **pg-admin in-process vs RPC**
   - What we know: today's pg-admin imports `internal/config` + `internal/database` directly.
   - What's unclear: should pg-admin's new candidate-mutation commands stay in-process (write `config.json` directly while bot is stopped) or RPC to the running daemon?
   - Recommendation: in-process (operator stops bot first per existing convention). The atomic-write + ring + reload-from-disk pattern in registry methods makes this safe even if the operator forgets to stop the bot — last-write-wins, but `.bak.{ts}` ring preserves the loser.

4. **Score formula weight calibration**
   - What we know: gate-then-magnitude shape is locked; weights are Claude's discretion.
   - What's unclear: are the recommended 0.5/0.3/0.2 weights right, or do they need empirical calibration against the 3 days of paper-mode observation data already collected?
   - Recommendation: ship the recommended weights as defaults; persist all sub-components so PG-DISC-05 (v2.3) can recalibrate empirically without schema change.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.26+ | Build | ✓ (go.mod) | 1.26.x | — (install.sh provisions 1.22.12 — known drift) |
| Redis DB 2 | Telemetry writes | ✓ (live) | per ops | — |
| Recharts | Frontend chart | ✓ (web/package-lock.json) | bundled | — |
| Node v22.13.0 | `npm ci` + `make build-frontend` | ✓ via nvm | 22.13.0 | — |
| `npm` (lockdown: only `npm ci`) | Frontend build | ✓ | — | — |
| `make` | Build orchestration | ✓ | — | — |
| `pkg/exchange.Exchange.GetOrderBook` | Depth probe gate | ⚠ unverified at interface level | — | Use BBO-size proxy if not interface-level (Wave 0 confirms) |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** GetOrderBook (depth probe → BBO-size proxy if needed).

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework (Go) | `go test` (stdlib) — versions per `go.mod` |
| Framework (web) | Vitest (existing — verify in `web/package.json`) |
| Config file | `go.mod`; `web/vitest.config.*` (if present) |
| Quick run command (Go) | `go test ./internal/pricegaptrader/... -run <TestName> -count=1` |
| Quick run command (web) | `cd web && npx vitest run --testNamePattern <name>` |
| Full suite command | `make test` (or `go test ./... && cd web && npx vitest run`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PG-DISC-01 | Scanner runs at configured interval, ≤20 universe enforced, ≥4-bar persistence + BBO freshness + depth gates applied, score 0–100, cycle output to `pg:scan:*` | unit + integration | `go test ./internal/pricegaptrader/ -run TestScanner_ -count=1` | ❌ Wave 0 |
| PG-DISC-01 | Default OFF: scanner does not emit non-OFF metrics with `PriceGapDiscoveryEnabled=false` | unit | `go test ./internal/pricegaptrader/ -run TestScanner_DefaultOff -count=1` | ❌ Wave 0 |
| PG-DISC-01 | Symbol normalization: cross-exchange BBO join produces non-empty for known-good pair (`BTCUSDT` on Gate × Bitget) | integration | `go test ./internal/pricegaptrader/ -run TestScanner_CanonicalSymbolJoin -count=1` | ❌ Wave 0 |
| PG-DISC-01 | Scanner is read-only: `*Registry` cannot be obtained from scanner package; only `RegistryReader` symbols imported | static analysis | `go vet ./internal/pricegaptrader/scanner.go` + `! grep -E 'registry\.(Add\|Update\|Delete\|Replace)' internal/pricegaptrader/scanner.go` | ❌ Wave 0 |
| PG-DISC-01 | 1-bar spread pump REJECTED (Pitfall 1 fixture) | unit | `go test ./internal/pricegaptrader/ -run TestScanner_OneBarPumpRejected -count=1` | ❌ Wave 0 |
| PG-DISC-01 | Bybit blackout skip: scanner reads during `:04–:05:30` produce `bybit_blackout` reason, not score | unit | `go test ./internal/pricegaptrader/ -run TestScanner_BybitBlackout -count=1` | ❌ Wave 0 |
| PG-DISC-04 | Concurrent dashboard + pg-admin writes through chokepoint produce ZERO lost mutations | integration (multi-process) | `go test ./internal/pricegaptrader/ -run TestRegistry_ConcurrentWriters -count=1` | ❌ Wave 0 |
| PG-DISC-04 | Dedupe by `(symbol, longExch, shortExch, direction)` rejects duplicate `Add` | unit | `go test ./internal/pricegaptrader/ -run TestRegistry_Dedupe -count=1` | ❌ Wave 0 |
| PG-DISC-04 | `.bak.{ts}` ring keeps newest 5 across N saves | unit | `go test ./internal/pricegaptrader/ -run TestRegistry_BakRingPrune -count=1` | ❌ Wave 0 |
| PG-DISC-04 | Atomic write: kill -9 mid-write leaves either old or new config.json, never partial | integration | `go test ./internal/pricegaptrader/ -run TestRegistry_AtomicWrite -count=1` | ❌ Wave 0 |
| PG-DISC-04 | Hard-cut: zero direct `cfg.PriceGapCandidates =` mutations remain in handlers.go and pg-admin/main.go | static + grep | `! grep -E 'cfg\.PriceGapCandidates\s*=|s\.cfg\.PriceGapCandidates\s*=' internal/api/handlers.go cmd/pg-admin/main.go` | ❌ Wave 0 |
| PG-DISC-04 | Compile-time read-only: scanner does not import `*Registry`, only `RegistryReader` | static | `! grep -E 'pricegaptrader\.\*Registry|registry\.\(Add\|Update\|Delete\|Replace\)' internal/pricegaptrader/scanner*.go` | ❌ Wave 0 |
| PG-DISC-03 | `GET /api/pg/discovery/state` returns the documented envelope | integration (API) | `go test ./internal/api/ -run TestPgDiscoveryStateHandler -count=1` | ❌ Wave 0 |
| PG-DISC-03 | WS pushes scoped to `pg:scan:cycle`/`metrics`/`score` channels | integration | `go test ./internal/api/ -run TestPgDiscoveryWS -count=1` | ❌ Wave 0 |
| PG-DISC-03 | `PriceGapDiscoveryEnabled=false` renders "Scanner OFF" banner; cycle stats render in disabled state | manual-only (browser confirm) | `human_needed` template | ❌ Wave 0 |
| PG-DISC-03 | i18n EN/zh-TW lockstep for new `pricegap.discovery.*` keys | static | `node web/scripts/check-i18n-lockstep.js` (if missing, write Wave-0 script) | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/pricegaptrader/... -count=1` (~5–10s)
- **Per wave merge:** `go test ./internal/pricegaptrader/... ./internal/api/... -count=1 && cd web && npx vitest run`
- **Phase gate:** Full suite green before `/gsd-verify-work`; manual browser confirm for `Scanner OFF` state

### Wave 0 Gaps

- [ ] `internal/pricegaptrader/scanner_test.go` — unit tests for gates, score, default-OFF, canonical symbol join, 1-bar pump rejection, Bybit blackout
- [ ] `internal/pricegaptrader/registry_test.go` — unit tests for Add/Update/Delete/Replace, dedupe, `.bak` ring, atomic write
- [ ] `internal/pricegaptrader/registry_concurrent_test.go` — multi-process integration test (success criterion #3): two `os.Exec` workers race to mutate via registry; assert all mutations land
- [ ] `internal/api/pg_discovery_handlers_test.go` — REST `/api/pg/discovery/state` + scores/{symbol}; WS subscriptions
- [ ] `internal/pricegaptrader/scanner_static_test.go` (or build-tag tagged) — static check that scanner doesn't import write methods
- [ ] `web/src/components/Discovery*.test.tsx` — Vitest snapshot for cycle stats card, score history chart, why-rejected breakdown, placeholder card
- [ ] `web/scripts/check-i18n-lockstep.js` — script comparing en.ts and zh-TW.ts key sets; CI fail on drift
- [ ] Test fixtures: synthetic BBO data with 1-bar pump, 4-bar valid persistence, stale BBO, depth shortage, denylist match, Bybit blackout window

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Existing Bearer-token (localStorage `arb_token`) — Discovery routes inherit auth middleware |
| V3 Session Management | yes | Existing — no new sessions |
| V4 Access Control | yes | Discovery section is read-only from frontend; mutations only via `/api/config` (chokepoint) |
| V5 Input Validation | yes | Universe/denylist entries validated against `^[A-Z0-9]{2,20}USDT$` regex (reuse `validatePriceGapCandidates` precedent) |
| V6 Cryptography | no (no new crypto) | — |
| V7 Errors / Logging | yes | `pg:registry:audit` LIST captures every chokepoint write with source attribution |

### Known Threat Patterns for {Go scanner + Redis + React dashboard}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Operator typo in universe ⇒ malformed exchange call | Tampering | Server-side regex validator at config load AND inside `Registry.Add` (defense in depth) |
| Operator sets `PriceGapAutoPromoteScore = 0` (Phase 12) ⇒ everything promotes | Tampering | Server-side floor (≥50) — Phase 12 concern but field is added in Phase 11 |
| Telegram alert leaks raw exchange API error string from scanner failure | Information disclosure | Reuse existing notifier sanitizer; scanner failures emit redacted reason codes only (no raw err strings to Telegram) |
| Three-writer race silently drops a candidate edit | Tampering / Repudiation | Chokepoint serialization + `pg:registry:audit` audit trail with `{ts, source, op, before_count, after_count}` |
| `config.json.bak.{ts}` left world-readable contains exchange secrets | Information disclosure | Backup file inherits `config.json` permissions (umask 022 ⇒ 0644); operator owns systemd `WorkingDirectory` perms; no change to existing posture |
| pg-admin operator runs CLI while bot is live ⇒ split-brain mutations | Tampering | `Registry.Replace` reload-from-disk + atomic write makes split-brain recoverable; audit trail captures both writers |

---

## Sources

### Primary (HIGH confidence — verified in-tree)
- `/var/solana/data/arb/.planning/phases/11-auto-discovery-scanner-chokepoint-telemetry/11-CONTEXT.md` — locked decisions D-01 through D-17
- `/var/solana/data/arb/.planning/REQUIREMENTS.md` — PG-DISC-01, PG-DISC-03, PG-DISC-04 acceptance text + v2.2 constraints
- `/var/solana/data/arb/.planning/ROADMAP.md` §"Phase 11" — 5 success criteria, dependency notes
- `/var/solana/data/arb/.planning/research/ARCHITECTURE-v2.2.md` — module location rationale, telemetry data flow, concurrency invariants, anti-patterns
- `/var/solana/data/arb/.planning/research/PITFALLS.md` — Pitfalls 1, 2, 7 (gate requirements, chokepoint requirements, retrospective regressions)
- `/var/solana/data/arb/internal/pricegaptrader/tracker.go` — `Tracker.Run`/`Start` errgroup; subscription pattern; WS broadcast pattern; tickLoop offset
- `/var/solana/data/arb/internal/pricegaptrader/detector.go` — `barRing.allExceedDirected` 4-bar persistence; `sampleLegs` cross-exchange BBO; staleness detection
- `/var/solana/data/arb/internal/pricegaptrader/risk_gate.go` — gate decision struct; deterministic ordering; reason codes
- `/var/solana/data/arb/internal/pricegaptrader/metrics.go` — pure-aggregator pattern (telemetry follows same envelope)
- `/var/solana/data/arb/internal/config/config.go` — `sync.RWMutex mu` (line 70); `PriceGapCandidates` field (line 326); `keepNonZero` tripwire (line 1519); `SaveJSON` (line 1572); `CONFIG_FILE` env requirement (line 1583)
- `/var/solana/data/arb/internal/api/handlers.go` lines 1655–1701 — current direct `cfg.PriceGapCandidates = next` mutation site (migration target)
- `/var/solana/data/arb/cmd/pg-admin/main.go` — current pg-admin command surface
- `/var/solana/data/arb/web/src/i18n/en.ts` lines 13, 785–810 — existing pricegap i18n namespace; lockstep target
- `/var/solana/data/arb/CHANGELOG.md` 0.35.0 entry — bidirectional `Direction` field affects dedupe tuple
- `/var/solana/data/arb/CLAUDE.local.md` — npm lockdown, config.json off-limits, build order, scan schedule, internal symbol form, Loris ÷8

### Secondary (MEDIUM confidence)
- `.planning/STATE.md` — phase ordering rationale, v2.2 milestone status
- `.planning/research/SUMMARY.md`, `STACK.md`, `FEATURES.md` — feature categorization

### Tertiary (LOW confidence — flagged in Assumptions Log)
- `Exchange.GetOrderBook` interface coverage — needs Wave 0 verification (A1)
- `cfg.SaveJSON` lock semantics — needs Wave 0 confirmation (A2)
- `cfg.BybitBlackoutEnabled` exact field name — needs Wave 0 confirmation (A4)

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every library/API verified in-tree, npm/Go zero-new-deps target enforced
- Architecture: HIGH — existing `Tracker`/`barRing`/`SaveJSON`/`broadcaster` patterns map 1:1; CONTEXT decisions specify component boundaries
- Pitfalls: HIGH — three Pitfalls research documents are already authoritative; mitigations directly cited
- Validation: MEDIUM — test framework in repo is `go test` (stdlib) + Vitest; no missing infrastructure but Wave 0 must author the new test files
- Security: MEDIUM — auth surface inherited; new audit trail required (`pg:registry:audit`)

**Research date:** 2026-04-28
**Valid until:** 2026-05-28 (30 days — stable codebase, Phase 11 is additive)
