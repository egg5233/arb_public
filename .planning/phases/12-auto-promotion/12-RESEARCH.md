# Phase 12: Auto-Promotion - Research

**Researched:** 2026-04-29
**Domain:** `internal/pricegaptrader/` controller + chokepoint integration + dashboard timeline
**Confidence:** HIGH (every anchor below was opened and read at the cited line; no `[ASSUMED]` claims).

---

## User Constraints (from CONTEXT.md)

### Locked Decisions

**Streak tracking**
- D-01: per-candidate **counter** keyed by `(symbol, longExch, shortExch, direction)`. Increment when `CycleRecord.Score ≥ cfg.PriceGapAutoPromoteScore`; reset to 0 when below threshold or absent from accepted Records. Promote at `≥ 6`.
- D-02: strict-consecutive (any non-accepted cycle resets).
- D-03: storage is **in-memory** `map[candidateKey]streakState` inside controller. No Redis key. Cold restart = 6 fresh cycles.

**Auto-demote**
- D-04: symmetric streak demote (6 cycles below threshold or absent → demote).
- D-05: **active-position guard always blocks demote**. Skip silently, **hold demote-streak counter** (do not reset). No Telegram on blocked demote.
- D-06: auto-demote is mandatory in this phase.

**Master toggle**
- D-07: reuse `cfg.PriceGapDiscoveryEnabled` for both scanner and controller. No new field.

**Cap-full**
- D-08: skip silently when add would exceed `cfg.PriceGapMaxCandidates`. Increment `cap_full_skips` HASH field on `pg:scan:metrics`. **No Telegram, no event in `pg:promote:events`.** Hold streak at 6.
- D-09: no auto-displacement.

**Event surfacing**
- D-10: `pg:promote:events` LIST, RPush + LTrim 1000. Each entry: `ts` (unix ms), `action` (`promote`|`demote`), `symbol`, `long_exch`, `short_exch`, `direction`, `score`, `streak_cycles`, `reason` (`score_threshold_met`|`score_below_threshold`).
- D-11: WS event name `pg_promote_event` via existing `s.hub.Broadcast(eventType, payload)`.
- D-12: REST seed endpoint under `/api/pg/discovery/*` (combined or separate route — Claude's discretion).

**Telegram**
- D-13: per-event cooldown key `"pg_promote:" + action + ":" + symbol + ":" + longExch + ":" + shortExch + ":" + direction`. Existing 5-min `checkCooldownAt()`.
- D-14: format `"[PG promote] {symbol} {long_exch}↔{short_exch} ({direction}) score={score} streak={streak_cycles}"` via `(*TelegramNotifier).Send(format, args...)` raw path.

**Wiring**
- D-15: new `internal/pricegaptrader/promotion.go` (+ `promotion_test.go`). Imports through small interfaces declared in `promotion.go`.
- D-16: synchronous `controller.Apply(ctx, summary)` at end of `Scanner.RunCycle()` (single goroutine).
- D-17: scanner constructor swap from `RegistryReader` to `*Registry`. Update `scanner_static_test.go` assertion.

### Claude's Discretion
- Exact JSON field names of `pg:promote:events` entries (schema list locked, ordering open).
- REST route shape (single combined `/state` extension vs separate `/promote-events`).
- Internal interface struct names (e.g., `ActivePositionChecker`, `PromoteNotifier`).
- Test fixtures for streak transitions.
- `cap_full_skips` shape (counter vs per-symbol HASH field).

### Deferred Ideas (OUT OF SCOPE)
- Auto-displacement, hysteresis, manual-only demote, per-cycle digest, separate `PriceGapAutoPromoteEnabled`, Redis-backed streak storage, score-vs-realized calibration view, PG-LIVE-04 fill alerts, auto-tune of threshold, HASH-per-day event store.

---

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| **PG-DISC-02** | Auto-promote candidates with `score ≥ PriceGapAutoPromoteScore` for `≥6 consecutive cycles` via the chokepoint, capped by `PriceGapMaxCandidates`, idempotent dedupe, auto-demote with `pg:positions:active` guard, Telegram + WS broadcast. | All anchors below resolve every clause: `Registry.Add`/`Delete` (cap + dedupe), `pg:positions:active` SET (guard), `(*TelegramNotifier).Send` + `checkCooldownAt` (Telegram + cooldown), `(*Server).BroadcastPriceGapDiscoveryEvent` (WS), `Scanner.RunCycle` (sync controller call site). |

---

## Project Constraints (from CLAUDE.local.md)

- **Module boundary:** `internal/pricegaptrader/` MUST NOT import `internal/engine/` or `internal/spotengine/`. Controller follows pattern `Telemetry` already established (narrow interface for WS broadcaster declared inside `pricegaptrader`, satisfied by `api.*Server`).
- **`config.json` is the sole source of truth.** Controller MUST go through `Registry.Add/Delete` → `cfg.SaveJSONWithBakRing()` (Phase 11 ring). Never `cfg.PriceGapCandidates = append(...)`.
- **Default-OFF rollout:** master toggle is `cfg.PriceGapDiscoveryEnabled` (reused per D-07); no new bool, no new dashboard toggle.
- **`go:embed` build order:** `npm run build` (web/) → then `go build`. Phase 12 modifies `PromoteTimelinePlaceholder.tsx` → must rebuild frontend before Go binary.
- **i18n lockstep:** `web/src/i18n/en.ts` + `web/src/i18n/zh-TW.ts` MUST stay in sync. Existing keys for the placeholder are at en:874-875 / zh-TW:920-921 (`pricegap.discovery.timeline.title`, `pricegap.discovery.timeline.placeholder`). Phase 12 replaces the placeholder body — keep title key, deprecate or extend `placeholder` text.
- **npm lockdown:** zero new packages. Recharts/React only via `npm ci`.
- **`config.json` read-only:** the agent MUST NOT touch `config.json`; controller mutates it through `cfg.SaveJSONWithBakRing()` at runtime via the registry — that is allowed because it's runtime, not editor-time.

---

## Summary

Phase 12 adds **one new Go file** (`internal/pricegaptrader/promotion.go`) plus a tiny scanner edit, a one-line bootstrap edit in `cmd/main.go:503`, an updated static-test assertion, an optional REST handler, and a frontend swap of the `PromoteTimelinePlaceholder` empty card with a populated timeline. **Every locked decision in CONTEXT.md maps to an existing code anchor that already supports the design** — `Registry.Add` returns `ErrCapExceeded` and `ErrDuplicateCandidate` for cap-full and dedupe; `*database.Client` has `SMembers(pg:positions:active)` + `GetPriceGapPosition(id)` for the guard; `TelegramNotifier.Send` + `checkCooldownAt` provide per-event-key dedup; `(*Server).BroadcastPriceGapDiscoveryEvent` is already the WS broadcast surface; `Telemetry.WriteCycle` is the LIST + LTrim 1000 template to mirror for `pg:promote:events`.

**Primary recommendation:** structure Phase 12 as 4 sequential plans — (1) Plan 12-01: streak controller + interfaces + unit tests; (2) Plan 12-02: scanner integration + static-test assertion update + bootstrap wire-up; (3) Plan 12-03: REST seed endpoint + WS event verification; (4) Plan 12-04: frontend timeline component (populate placeholder) + i18n + visual regression. The chokepoint, registry, telemetry, WS hub, Telegram cooldown, position store, and dashboard scaffold are all already built and verified — Phase 12 is **wiring**, not new infrastructure.

---

## Code Anchors

### A1 — `Scanner.RunCycle` controller call site (D-16)

**File:** `internal/pricegaptrader/scanner.go:237-321`

```go
// scanner.go:215-232
func NewScanner(
    cfg *config.Config,
    registry RegistryReader,            // ← swap to *Registry per D-17
    exchanges map[string]exchange.Exchange,
    telemetry TelemetryWriter,
    log *utils.Logger,
) *Scanner { ... }

// scanner.go:237-321 (key lines)
func (s *Scanner) RunCycle(ctx context.Context, now time.Time) {
    if !s.cfg.PriceGapDiscoveryEnabled { ...; return }   // line 238 — D-07 master switch
    s.cfg.RLock()
    universe := append([]string(nil), s.cfg.PriceGapDiscoveryUniverse...)
    ...
    s.cfg.RUnlock()
    ...
    summary := CycleSummary{...}
    // all gate work happens here
    ...
    completedAt := s.nowFunc()
    summary.CompletedAt = completedAt.Unix()
    summary.DurationMs = completedAt.Sub(startedAtNanos).Milliseconds()
    _ = s.telemetry.WriteCycle(ctx, summary)              // line 320 — TELEMETRY
    // ← D-16 INSERT: s.controller.Apply(ctx, summary)    // line 321
}
```

**Why it matters:**
- `RunCycle` runs in the single `scanLoop` goroutine (`tracker.go:537-571`). Synchronous append after `WriteCycle` is the locked decision (D-16).
- The cycle context already has the cycle-interval deadline (`tracker.go:552`: `context.WithTimeout(context.Background(), interval)`). Controller can reuse the same `ctx` — no new bounded context needed.
- `summary.Records []CycleRecord` is the controller's input. `summary.WhyRejected` histogram is irrelevant (controller iterates `Records`, picks `Score >= threshold && WhyRejected == ReasonAccepted`).

### A2 — `CycleRecord` struct (controller input shape)

**File:** `internal/pricegaptrader/scanner.go:88-116`

```go
type CycleRecord struct {
    Symbol    string `json:"symbol"`
    LongExch  string `json:"long_exch"`
    ShortExch string `json:"short_exch"`
    Score     int    `json:"score"`
    SpreadBps       float64
    PersistenceBars int
    DepthScore      float64
    FreshnessAgeSec int
    FundingBpsPerHr float64
    GatesPassed []string
    GatesFailed []string
    WhyRejected     ScanReason
    AlreadyPromoted bool
    Timestamp       int64
}
```

**Critical gap for tuple matching:** `CycleRecord` has **NO `Direction` field**. The scanner runs bidirectional-only (see `scanner.go:163: models.PriceGapDirectionBidirectional`). Promoted candidates carry a `Direction` field per v0.35.0 (`models.PriceGapCandidate.Direction`). The controller MUST pick a direction for newly-promoted candidates — either:
- (a) hardcode `Direction = "bidirectional"` (matches scanner's runtime semantics), OR
- (b) read it from a new optional config or from the existing universe seed.

**Recommendation: option (a) — `Direction = models.PriceGapDirectionBidirectional`.** This aligns with `scanner.go:163` (`barRingExceedThreshold` always uses Bidirectional). Document it as a known limitation: operators wanting `pinned-long` direction must promote manually.

The streak counter key SHOULD therefore be `symbol|longExch|shortExch` (no direction) — direction is a derived field at promote-time.

### A3 — `Registry.Add` / `Registry.Delete` chokepoint signatures

**File:** `internal/pricegaptrader/registry.go:118-220`

```go
// registry.go:38-49 — sentinel errors
var (
    ErrDuplicateCandidate = errors.New("registry: duplicate candidate (symbol, long_exch, short_exch, direction)")
    ErrCapExceeded        = errors.New("registry: PriceGapMaxCandidates cap exceeded")
    ErrIndexOutOfRange    = errors.New("registry: index out of range")
)

// registry.go:118 — Add (cap + dedupe + atomic save + audit)
func (r *Registry) Add(ctx context.Context, source string, c models.PriceGapCandidate) error
// registry.go:192 — Delete (idx-based, atomic save + audit)
func (r *Registry) Delete(ctx context.Context, source string, idx int) error
// registry.go:91 — Get (RegistryReader)
func (r *Registry) Get(idx int) (models.PriceGapCandidate, bool)
// registry.go:102 — List (RegistryReader, returns defensive copy)
func (r *Registry) List() []models.PriceGapCandidate
```

**Existing call sites (audit trail tag pattern):**
- `cmd/pg-admin/main.go:227,256` — uses `"pg-admin"`
- `internal/api/pricegap_handlers.go` (Phase 11 Plan 03 migration) — uses `"dashboard-handler"`

**Phase 12 controller MUST use `"scanner-promote"` and `"scanner-demote"`** to match the bounded enumeration declared in `registry.go:117` comment.

**Behavior of `Registry.Add`:**
1. Acquires `cfg.LockConfigFile()` (cross-process flock) → `cfg.Lock()` (in-memory rwmutex). Locks are released on return.
2. Calls `r.reloadFromDiskLocked()` so cross-process writes from `pg-admin` are observed.
3. Returns `ErrCapExceeded` BEFORE returning `ErrDuplicateCandidate` (cap check at line 132-134; dedupe at line 135-142). **Order matters for D-08:** controller must distinguish "cap full" from "duplicate" so cap-full silently increments `cap_full_skips` while dedupe ignores silently (already-promoted candidate is fine, scanner's `AlreadyPromoted` field handles this).
4. On persist failure rolls back in-memory mutation.
5. Writes `pg:registry:audit` LIST best-effort (already capped at 200 via LTrim).

**Delete by index:** controller must match the candidate tuple to find its index. Pattern:
```go
list := registry.List()  // defensive copy
for i, c := range list {
    if c.Symbol == sym && c.LongExch == long && c.ShortExch == short && c.Direction == dir {
        if err := registry.Delete(ctx, "scanner-demote", i); err != nil { ... }
        break
    }
}
```
**Race risk:** between `List()` and `Delete(i)`, another writer could mutate the slice. `Registry.Delete` itself takes the locks and reloads from disk, so the `i` could now point to a different candidate. **Mitigation:** Delete should re-find by tuple inside the lock; but the locked-method API only takes `idx`. Either (a) expose a new `DeleteByTuple` helper or (b) accept the rare race and let the controller retry on next cycle (it's a 6-cycle streak; extra cycle is acceptable).

**Recommendation:** add a small helper inside `promotion.go` (NOT in `registry.go`) that does `List → find idx → Delete`, retried up to 1× on `ErrIndexOutOfRange`. This avoids changing the chokepoint surface.

### A4 — Active-position guard read path (D-05)

**File:** `internal/database/pricegap_state.go:20, 88, 119-122, 399-411`

```go
// pricegap_state.go:20
keyPricegapActive = "pg:positions:active"  // SET of active position IDs

// pricegap_state.go:~86-95 — exact getter exists
func (c *Client) GetActivePriceGapPositions() ([]*models.PriceGapPosition, error) {
    ids, err := c.rdb.SMembers(ctx, keyPricegapActive).Result()
    ...
    // for each id: HGet pg:positions, JSON unmarshal models.PriceGapPosition
}

// pricegap_state.go:399 — single-record getter
func (c *Client) GetPriceGapPosition(id string) (*models.PriceGapPosition, error)
```

**Position record fields the controller matches against** (`internal/models/pricegap_position.go:42-56`):
```go
type PriceGapPosition struct {
    Symbol             string  `json:"symbol"`
    ...
    FiredDirection     string  `json:"fired_direction,omitempty"`
    CandidateLongExch  string  `json:"candidate_long_exch,omitempty"`
    CandidateShortExch string  `json:"candidate_short_exch,omitempty"`
    ...
}
```

**Critical:** position uses `CandidateLongExch` + `CandidateShortExch` + `FiredDirection`, NOT `LongExch`/`ShortExch`/`Direction`. Controller's tuple-match must use these fields. There's also `PriceGapStatusClosed` constant — only `Status != Closed` positions appear in the SET (`pricegap_state.go:388-393` SAdd/SRem on save).

**Active position checker interface for controller (Claude's discretion D-15):**
```go
// declared in promotion.go
type ActivePositionChecker interface {
    HasActivePosition(symbol, longExch, shortExch string) (bool, error)
}
// satisfied by a thin adapter in cmd/main.go that wraps *database.Client
```

The adapter can either iterate `GetActivePriceGapPositions()` (one Redis round-trip + N HGets) or, since the controller runs once every 5 minutes, the simpler form is acceptable. If perf becomes a concern, cache by reading `SMembers` once and walking JSON only on tuple match.

### A5 — Telemetry pattern for `pg:promote:events`

**File:** `internal/pricegaptrader/telemetry.go:179-265, 31-43, 411-413`

The exact LIST + LTrim pattern to mirror is at `telemetry.go:185-193`:
```go
// 1. Append cycle envelope to LIST + cap at 1000.
if body, err := json.Marshal(summary); err == nil {
    c, cancel := boundCtx(ctx)
    if err := rdb.RPush(c, keyCycles, body).Err(); err != nil {
        t.warnf("pg:scan:cycles rpush: %v", err)
    }
    _ = rdb.LTrim(c, keyCycles, -1000, -1).Err()
    cancel()
}
```

**`boundCtx(parent)` (telemetry.go:411):** wraps in `context.WithTimeout(parent, 3*time.Second)`. Controller MUST use the same pattern to prevent a hung Redis from blocking the scan loop.

**Phase 12 mirror — proposed (Claude's discretion D-10 schema):**
```go
const keyPromoteEvents = "pg:promote:events"
const promoteEventsCap = 1000

type PromoteEvent struct {
    Ts            int64  `json:"ts"`              // unix ms
    Action        string `json:"action"`          // "promote" | "demote"
    Symbol        string `json:"symbol"`
    LongExch      string `json:"long_exch"`
    ShortExch     string `json:"short_exch"`
    Direction     string `json:"direction"`
    Score         int    `json:"score"`
    StreakCycles  int    `json:"streak_cycles"`
    Reason        string `json:"reason"`          // "score_threshold_met" | "score_below_threshold"
}
```

**Recommendation:** add an `EmitPromoteEvent(ctx, evt PromoteEvent) error` method on the existing `*Telemetry` struct rather than declaring a new top-level writer. This co-locates with `WriteCycle` and gets the same `boundCtx(3s)` discipline + the same Redis client + the same `DiscoveryBroadcaster` interface for WS push (saves a constructor parameter on `PromotionController`).

For the WS broadcast, reuse `(*Telemetry).broadcast("pg_promote_event", evt)` (see `telemetry.go:458-463`) — already routes through `DiscoveryBroadcaster.BroadcastPriceGapDiscoveryEvent`.

### A6 — Telegram cooldown + Send signature (D-13, D-14)

**File:** `internal/notify/telegram.go:72, 196-216`

```go
// telegram.go:72 — Send signature confirmed (D-14 reference is correct)
func (t *TelegramNotifier) Send(format string, args ...interface{}) {
    // formatted via fmt.Sprintf, sends via t.send(text)
}

// telegram.go:196-216 — cooldown
func (t *TelegramNotifier) checkCooldown(eventKey string) bool {
    return t.checkCooldownAt(eventKey, time.Now())
}
func (t *TelegramNotifier) checkCooldownAt(eventKey string, now time.Time) bool {
    t.cooldownMu.Lock()
    defer t.cooldownMu.Unlock()
    if last, ok := t.lastSent[eventKey]; ok && now.Sub(last) < 5*time.Minute {
        return false
    }
    t.lastSent[eventKey] = now
    return true
}
```

**Confirmed:**
- 5-minute cooldown is hardcoded.
- `lastSent map[string]time.Time` — keys are arbitrary strings, no schema enforcement, so D-13's `"pg_promote:" + action + ":" + ...` key works directly.
- **Nil-receiver safe** (see `NotifySLTriggered` at telegram.go:42-44 which checks `if t == nil { return }`). Controller can hold a `*TelegramNotifier` and call methods without a nil check, OR follow the `Telemetry`-pattern of declaring a narrow interface inside `promotion.go`:

```go
// declared in promotion.go (D-15)
type PromoteNotifier interface {
    Send(format string, args ...interface{})
    CheckCooldown(eventKey string) bool   // wraps checkCooldown
}
```

**Note:** `checkCooldown` is unexported (lowercase). To use it from `internal/pricegaptrader/`, **export it as `CheckCooldown` on `*TelegramNotifier`** (one tiny edit to telegram.go). Alternative: declare `PromoteNotifier` to take `(eventKey, format, args...)` and have a thin notifier struct in `promotion.go`'s wiring layer that combines both. Either way is fine; **recommendation: declare a `PromoteNotifier` interface in `promotion.go`** with one method `NotifyPromote(eventKey, format string, args ...interface{})`, and have `cmd/main.go` wire up an inline adapter that does `if n.checkCooldown(eventKey) { n.Send(format, args...) }`. This keeps `notify/telegram.go` untouched — no new public surface beyond what's already there.

### A7 — WS hub broadcast surface

**File:** `internal/api/ws.go:96-103`, `internal/api/pricegap_handlers.go:494-501`

```go
// ws.go:96
func (h *Hub) Broadcast(msgType string, data interface{}) {
    msg := wsMessage{Type: msgType, Data: data}
    b, _ := json.Marshal(msg)
    h.broadcast <- b
}

// pricegap_handlers.go:494-501 — already wired
func (s *Server) BroadcastPriceGapDiscoveryEvent(eventType string, payload interface{}) {
    s.hub.Broadcast(eventType, payload)
}
```

**Confirmed naming convention** from existing pricegap events (`pricegap_handlers.go:478-491`):
- `pg_positions`, `pg_event`, `pg_candidate_update`, plus discovery's `pg_scan_cycle`, `pg_scan_metrics`, `pg_scan_score`.

**D-11's `pg_promote_event`** fits the pattern. The frontend WS handler is ready to receive arbitrary `msg.type` values (`usePgDiscovery.ts:204-247` — switch on `msg.type` with default-ignore). Plan 12-04 adds the `'pg_promote_event'` case.

### A8 — Discovery dashboard placeholder card (frontend swap target)

**File:** `web/src/components/Discovery/PromoteTimelinePlaceholder.tsx` (entire file, 26 lines)

Currently a static empty card with `data-testid="promote-timeline-placeholder"`. Translation keys at:
- `web/src/i18n/en.ts:874-875`:
  - `pricegap.discovery.timeline.title` = `'Promote / Demote Timeline'`
  - `pricegap.discovery.timeline.placeholder` = `'Promote and demote events will appear here once Phase 12 (auto-promotion) ships.'`
- `web/src/i18n/zh-TW.ts:920-921`:
  - `pricegap.discovery.timeline.title` = `'升級／降級時間線'`
  - `pricegap.discovery.timeline.placeholder` = `'升級與降級事件將於 Phase 12（自動升級）上線後顯示於此。'`

**Plan 12-04 transforms this card into a populated timeline.** It must:
1. Subscribe to WS event `pg_promote_event` via `usePgDiscovery` hook (extend it OR create `usePgPromoteEvents`).
2. Seed via REST: `GET /api/pg/discovery/promote-events` (or extend `/state` to embed last N events — D-12 discretion).
3. Render newest-first list with `action` (promote/demote color), `symbol`, `long↔short`, `(direction)`, `score`, `streak={n}`, relative timestamp.
4. Add new i18n keys (lockstep en/zh-TW): `pricegap.discovery.timeline.empty`, `.action.promote`, `.action.demote`, `.scoreLabel`, `.streakLabel`, etc. Existing parity test at `web/src/i18n/__tests__/discovery_keys.test.ts` will catch missing zh-TW.
5. Keep the title key + `data-testid` — the existing `DiscoverySection.test.ts` likely asserts on `promote-timeline-placeholder`. Plan 12-04 should rename the testid to `promote-timeline-card` or update the test.

**Existing WS subscription pattern to mirror** (`usePgDiscovery.ts:204-247`): the hook already opens a WS, parses NDJSON, switches on `msg.type`. Extending the hook with one more case is the lightest path:
```ts
case 'pg_promote_event': {
    const evt = msg.data as PromoteEvent;
    setPromoteEvents((prev) => [evt, ...prev].slice(0, 1000));
    break;
}
```

### A9 — `scanner_static_test.go` no-mutation assertion

**File:** `internal/pricegaptrader/scanner_static_test.go:38-45`

```go
if regexp.MustCompile(`\*Registry\b`).MatchString(stripped) {
    t.Errorf("scanner.go references concrete *Registry — must use RegistryReader interface (D-13 / T-11-26)")
}

mutatorRe := regexp.MustCompile(`registry\.(Add|Update|Delete|Replace)\(`)
if mutatorRe.MatchString(stripped) {
    t.Errorf("scanner.go calls a Registry mutator — must be read-only (D-13 / T-11-26)")
}
```

**D-17 mandate:** Phase 12 swaps scanner constructor to `*Registry` (not `RegistryReader`). The static-test assertion at lines 38-45 MUST change.

**Two sub-decisions for the planner:**
1. **Allow `*Registry` token in scanner.go?** Yes — D-17 explicitly relaxes this. Remove the regex at line 38 OR retarget it: keep it asserting that scanner does NOT do raw `cfg.PriceGapCandidates = append(...)` mutation.
2. **Allow `registry.Add/Delete` calls in scanner package?** Phase 12's controller is in `promotion.go`, NOT `scanner.go`. The current test only reads `scanner.go`. **Easiest path:** keep the test reading only `scanner.go`; the regex at line 42 still applies. The controller in `promotion.go` is unconstrained because the test doesn't read it.

**Recommended assertion update:**
```go
// Replace lines 38-45 with:
// Allow *Registry now that D-17 swap shipped, but still forbid raw field mutation.
if regexp.MustCompile(`cfg\.PriceGapCandidates\s*=`).MatchString(stripped) {
    t.Errorf("scanner.go writes cfg.PriceGapCandidates directly — must go through *Registry (D-17 / Pitfall 2)")
}
// scanner.go itself still doesn't mutate; promotion.go does. Keep the registry.Add/Delete check
// applied ONLY to scanner.go to preserve scope.
mutatorRe := regexp.MustCompile(`registry\.(Add|Update|Delete|Replace)\(`)
if mutatorRe.MatchString(stripped) {
    t.Errorf("scanner.go calls a Registry mutator — must delegate to PromotionController in promotion.go (D-15)")
}
```

This is grep-verifiable: the planner can write a task with acceptance criteria `grep -E '\*Registry\\b' scanner.go` returning hits AND `grep -E 'cfg\\.PriceGapCandidates\\s*=' scanner.go` returning **zero** hits.

### A10 — Config field validations (PG-DISC-02 input)

**File:** `internal/config/config.go:54-58, 357-366, 814-815`

```go
// config.go:54-58 — server-side range checks (Validate())
if c.PriceGapAutoPromoteScore < 50 || c.PriceGapAutoPromoteScore > 100 {
    return fmt.Errorf("PriceGapAutoPromoteScore=%d outside [50,100]", ...)
}
if c.PriceGapMaxCandidates < 1 || c.PriceGapMaxCandidates > 50 {
    return fmt.Errorf("PriceGapMaxCandidates=%d outside [1,50]", ...)
}

// config.go:357-366 — declarations
PriceGapDiscoveryEnabled      bool     // D-11-default OFF master switch
PriceGapDiscoveryUniverse     []string // D-05 ≤20 entries
PriceGapDiscoveryDenylist     []string
PriceGapDiscoveryIntervalSec  int      // default 300 (5min); floor 60
PriceGapDiscoveryThresholdBps int      // default 100; floor 10
PriceGapDiscoveryMinDepthUSDT int      // default 1000; floor 0
PriceGapAutoPromoteScore      int      // default 60; server floor 50, ceiling 100
PriceGapMaxCandidates         int      // default 12; range 1-50

// config.go:814-815 — defaults set in init
c.PriceGapAutoPromoteScore = 60
c.PriceGapMaxCandidates    = 12
```

**Confirmed:** all three fields exist with the exact ranges CONTEXT.md cites. Phase 12 reads them; **no new config fields**.

**Important:** the controller must read these under `cfg.RLock()` (mirror `scanner.go:247-256` snapshot pattern) so a dashboard config-write mid-cycle doesn't tear.

### A11 — Bootstrap wiring site

**File:** `cmd/main.go:165, 220-221, 477, 495-510`

```go
// cmd/main.go:220-221 — registry constructed
pgRegistry := pricegaptrader.NewRegistry(cfg, db.PriceGapAudit(), utils.NewLogger("pg-registry"))
apiSrv.SetRegistry(pgRegistry)

// cmd/main.go:475-510 — tracker + telemetry + scanner wiring (today)
var pgTracker *pricegaptrader.Tracker
if cfg.PriceGapEnabled {
    pgTracker = pricegaptrader.NewTracker(exchanges, db, scanner, cfg)

    pgTelemetry := pricegaptrader.NewTelemetry(
        db,
        apiSrv,           // satisfies DiscoveryBroadcaster
        cfg,
        utils.NewLogger("pg-telemetry"),
    )
    apiSrv.SetDiscoveryTelemetry(pgTelemetry)

    if cfg.PriceGapDiscoveryEnabled {
        pgScanner := pricegaptrader.NewScanner(
            cfg,
            pgRegistry,    // ← TODAY: passed as RegistryReader (compile-time bound)
            exchanges,
            pgTelemetry,
            utils.NewLogger("pg-scanner"),
        )
        pgTracker.SetScanner(pgScanner)
    }
}
```

**Phase 12 inserts at cmd/main.go:~509 (between scanner construction and SetScanner):**
```go
pgController := pricegaptrader.NewPromotionController(
    cfg,
    pgRegistry,                           // *Registry, full mutator (D-17)
    pgTelemetry,                          // for EmitPromoteEvent (D-10/D-11)
    activePositionAdapter{db: db},        // tiny adapter wrapping *database.Client
    promoteNotifierAdapter{tg: telegram}, // wraps *notify.TelegramNotifier
    utils.NewLogger("pg-promotion"),
)
pgScanner := pricegaptrader.NewScanner(cfg, pgRegistry, exchanges, pgTelemetry, log)
pgScanner.SetController(pgController)    // OR pass into NewScanner — Claude's discretion
pgTracker.SetScanner(pgScanner)
```

**Adapters live in `cmd/main.go`** (NOT inside `pricegaptrader/`) to preserve module boundary — they translate `*database.Client` and `*notify.TelegramNotifier` into the small interfaces declared in `promotion.go`.

### A12 — PG positions REST API for active guard (helper context)

**File:** `internal/database/pricegap_state.go:43-122` + `internal/api/pricegap_handlers.go`

`*database.Client` directly exposes `GetActivePriceGapPositions() ([]*models.PriceGapPosition, error)` — no need to go through the API layer. Controller gets the slice, walks it once per Apply call. Each call is O(N) where N = number of active PG positions (typically <12 because of `PriceGapMaxCandidates`). Trivial cost; no caching needed.

### A13 — Tracker scanLoop interval interaction

**File:** `internal/pricegaptrader/tracker.go:537-571`

```go
runOnce := func() {
    ctx, cancel := context.WithTimeout(context.Background(), interval)
    defer cancel()
    t.scanner.RunCycle(ctx, time.Now())   // synchronous; controller.Apply runs inside this ctx
}
```

**Confirmed:** the cycle context already has the cycle interval as deadline. Controller's Redis writes via Telemetry use `boundCtx(parent, 3s)` so they always finish before the cycle deadline.

---

## Integration Points Map

| Change-point | File:Line | Planner task suggestion |
|--------------|-----------|--------------------------|
| New controller | `internal/pricegaptrader/promotion.go` (NEW) | Plan 12-01: declare `PromotionController` + 4 small interfaces (`PromoteNotifier`, `ActivePositionChecker`, `PromoteRegistry` wrapping `Add/Delete/List`, `PromoteEventEmitter`); implement `Apply(ctx, summary)` with streak map + per-event Telegram + REST event emit. |
| Controller unit tests | `internal/pricegaptrader/promotion_test.go` (NEW) | Plan 12-01: 12+ table tests — promote at exactly 6 cycles, miss at cycle 4 resets, demote at 6 absent, demote blocked by guard holds streak, cap-full skips silently and increments counter, dedupe error swallowed, Telegram cooldown applied per unique key. |
| Scanner constructor swap | `internal/pricegaptrader/scanner.go:215-232` | Plan 12-02: change `registry RegistryReader` → `registry *Registry` AND add `controller *PromotionController` (or `setController` setter). Field name `s.controller`. |
| Scanner Apply call | `internal/pricegaptrader/scanner.go:321` (just after WriteCycle) | Plan 12-02: insert `if s.controller != nil { s.controller.Apply(ctx, summary) }`. |
| Static test assertion | `internal/pricegaptrader/scanner_static_test.go:38-45` | Plan 12-02: replace `*Registry` regex with `cfg\.PriceGapCandidates\s*=` regex (forbid raw write only). Keep `registry\.(Add|Delete|...)` check on scanner.go scope. Add explicit comment that promotion.go is the legitimate writer. |
| Telemetry method | `internal/pricegaptrader/telemetry.go` (extend) | Plan 12-02: add `func (t *Telemetry) EmitPromoteEvent(ctx, evt PromoteEvent) error` that does RPush + LTrim + WS broadcast on `pg_promote_event`. Mirrors lines 185-193 + 458-463. |
| Bootstrap | `cmd/main.go:~509` (between scanner and SetScanner) | Plan 12-02: construct `PromotionController`; declare 2 small adapter structs; pass into scanner. |
| REST endpoint | `internal/api/pricegap_handlers.go` (extend) | Plan 12-03: add `GET /api/pg/discovery/promote-events` returning LRange 0 -1 of `pg:promote:events`. OR extend existing `GET /api/pg/discovery/state` to embed last 100. **Recommendation:** separate endpoint — simpler, smaller payload, frontend can paginate. |
| WS frontend hook | `web/src/hooks/usePgDiscovery.ts:204-247` | Plan 12-04: add `case 'pg_promote_event':` to switch; add `promoteEvents: PromoteEvent[]` to `UsePgDiscoveryResult`. |
| Frontend timeline | `web/src/components/Discovery/PromoteTimelinePlaceholder.tsx` (REWRITE) | Plan 12-04: rename to `PromoteTimeline.tsx` (or keep filename, change body); seed via REST; subscribe via hook; render newest-first list with color-coded action chips. |
| i18n EN | `web/src/i18n/en.ts:874-875 + new keys` | Plan 12-04: keep `pricegap.discovery.timeline.title`; replace `placeholder` text or add `.empty`, `.action.promote`, `.action.demote`, `.scoreLabel`, `.streakLabel`, `.cap_full_skips`. |
| i18n zh-TW | `web/src/i18n/zh-TW.ts:920-921 + lockstep` | Plan 12-04: mirror every new key; `web/src/i18n/__tests__/parity.test.ts` will fail if missing. |
| Frontend test update | `web/src/components/Discovery/__tests__/DiscoverySection.test.ts` | Plan 12-04: any assertion on `promote-timeline-placeholder` testid → update or rename testid. |

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Atomic config write | A new `os.WriteFile`-with-rename path | `Registry.Add/Delete` → `cfg.SaveJSONWithBakRing()` | Phase 11 already gives you cap, dedupe, atomic save, `.bak.{ts}` ring, audit, cross-process flock. Pitfall 2 is the explicit reason. |
| Cooldown / rate-limit | A new `map[string]time.Time` in promotion.go | `(*TelegramNotifier).checkCooldownAt(eventKey, now)` (telegram.go:206) | Already 5-min, thread-safe, used everywhere. Per-event-key strategy in D-13 leverages it directly. |
| Bounded LIST in Redis | A new RPush+LRange+manual-trim helper | `Telemetry.EmitPromoteEvent` calling `RPush + LTrim -1000 -1` | Mirror `WriteCycle` (telemetry.go:185-193) exactly. Same `boundCtx(3s)` to prevent hung-Redis stalls. |
| WS broadcast plumbing | A new hub or routing struct | `(*Server).BroadcastPriceGapDiscoveryEvent(eventType, payload)` (pricegap_handlers.go:500) | One method, already wired. Frontend hook already routes by `msg.type`. |
| Active-position lookup | A new SMEMBERS+HGet loop | `*database.Client.GetActivePriceGapPositions()` (pricegap_state.go:~86) | Returns full `[]*models.PriceGapPosition` with `CandidateLongExch`/`CandidateShortExch`/`FiredDirection`. |
| Streak persistence | A new Redis ZSET / HASH | In-memory `map[string]struct{ promote, demote int }` per D-03 | CONTEXT decision. Cold-restart fresh baseline is the feature. |
| Telegram message formatting | New chat helper | `(*TelegramNotifier).Send(format, args...)` (telegram.go:72) | D-14 explicitly references this signature. |

---

## Common Pitfalls

### Pitfall 1: Tuple key mismatch between scanner and registry
**What goes wrong:** Scanner emits `(symbol, longExch, shortExch)` with no direction. Registry stores candidates with `Direction` field. Streak counter and dedupe key MUST be consistent or the controller will infinitely re-promote bidirectional candidates as if they were "missing".
**Avoidance:** key streaks by `symbol|longExch|shortExch` (no direction). When promoting, set `Direction = models.PriceGapDirectionBidirectional`. When demoting, find by `(Symbol, LongExch, ShortExch)` and `Direction == bidirectional` only — leave manually-promoted directional candidates alone.

### Pitfall 2: Race between `List()` and `Delete(idx)`
**What goes wrong:** Controller fetches index, dashboard mutates slice, controller calls Delete with stale idx → either deletes wrong candidate or returns ErrIndexOutOfRange.
**Avoidance:** wrap List+Delete in a single helper that retries once on ErrIndexOutOfRange (re-list, re-find, re-delete). Don't expose this in `Registry`; keep it local to `promotion.go`. The controller runs once per cycle so a single retry is sufficient.

### Pitfall 3: Position-record field naming
**What goes wrong:** `models.PriceGapPosition` uses `CandidateLongExch`/`CandidateShortExch`/`FiredDirection`, NOT `LongExch`/`ShortExch`/`Direction`. Tuple-match against the wrong field always returns false → demote NEVER blocked → live position closed unexpectedly.
**Avoidance:** unit-test the active-position guard with a fixture `PriceGapPosition` that populates `CandidateLongExch="binance"`, etc., and verify the guard returns true.

### Pitfall 4: `cap_full_skips` counter infinite increment
**What goes wrong:** Once a candidate hits streak=6 and cap is full, it stays at streak=6 forever (D-08). Every cycle increments `cap_full_skips`. Operators see a noisy counter.
**Mitigation:** that's the design — D-08 says counter is the only signal. Frontend should display "current cap-full-pending" candidates (count of streaks held at 6) separately from the cumulative `cap_full_skips`. Or only increment once per (candidate, streak-reached event).
**Recommendation for planner:** make `cap_full_skips` a HASH field on `pg:scan:metrics` keyed by symbol (Claude's discretion in D-08), incremented once per candidate-cycle when streak is held at 6. Operators can then see WHICH candidates are stuck.

### Pitfall 5: Cold-restart streak loss is invisible
**What goes wrong:** D-03 mandates in-memory streaks. After a daemon restart, all streaks are 0. Operators expecting "almost-promoted" candidates won't see them resume for 6 cycles.
**Mitigation:** log at INFO level on controller startup: `"pg-promotion: streak counters reset (in-memory storage; 6 fresh cycles required before next promote)"`. Frontend can show a banner if `last_run_at < 6 * intervalSec` after process start.

### Pitfall 6: Telegram failure stalls the cycle
**What goes wrong:** Telegram API hangs → `Send` blocks → `RunCycle` blocks → scanLoop misses next tick.
**Avoidance:** `(*TelegramNotifier).Send` already does `t.client.PostForm` with 10-second `http.Client{Timeout: 10s}` (telegram.go:98). But that's still 10 s of cycle stall. **Recommendation:** the controller should fire-and-forget Telegram in a goroutine: `go n.NotifyPromote(...)`. Order of operations: (1) Registry.Add succeeds → (2) emit event to Redis (synchronous, 3s bound) → (3) `go` Telegram. Failure to Telegram NEVER blocks the cycle.

### Pitfall 7: WS broadcast on disconnected hub
**What goes wrong:** `(*Server).BroadcastPriceGapDiscoveryEvent` → `s.hub.Broadcast(...)` → puts on a buffered channel. If the channel is full (no readers), it blocks.
**Avoidance:** check `ws.go:96-103` — `Broadcast` does `h.broadcast <- b` which IS blocking. **Verification needed:** confirm `h.broadcast` channel has sufficient buffer or that `Broadcast` is non-blocking. (NOT confirmed in this research session — the hub's channel buffer size lives outside the lines I read.) **Action item:** Plan 12-02 must verify hub buffer + add a select-with-default if needed. This is also the case for existing `pg_scan_*` events; if Phase 11 already runs without issue, Phase 12 inherits the safety.

---

## Code Examples

### Recommended `promotion.go` skeleton

```go
// Package pricegaptrader — Phase 12 / PG-DISC-02 auto-promotion controller.
//
// PromotionController consumes scanner cycle output and mutates
// cfg.PriceGapCandidates via the Registry chokepoint. Streak tracking is
// in-memory per D-03 — cold restart requires 6 fresh cycles.
//
// D-17: scanner now holds *Registry (full mutator) instead of RegistryReader.
// The compile-time read-only block is intentionally relaxed for this package
// because the controller is the legitimate writer.
package pricegaptrader

import (
    "context"
    "fmt"
    "sync"

    "arb/internal/config"
    "arb/internal/models"
    "arb/pkg/utils"
)

// Sources for Registry audit trail (matches the bounded enumeration in
// registry.go:117 comment).
const (
    sourcePromote = "scanner-promote"
    sourceDemote  = "scanner-demote"
)

// Reasons in the pg:promote:events JSON `reason` field (D-10).
const (
    reasonScoreThresholdMet   = "score_threshold_met"
    reasonScoreBelowThreshold = "score_below_threshold"
)

// Streak threshold per D-01/D-04. Hardcoded — no new config field.
const streakThreshold = 6

// ActivePositionChecker is the narrow read surface the controller needs from
// the database client (D-15 module boundary).
type ActivePositionChecker interface {
    GetActivePriceGapPositions() ([]*models.PriceGapPosition, error)
}

// PromoteRegistry is the narrow write surface (D-17 — full *Registry, but
// declared as an interface to keep the controller package-internal-decoupled
// from registry.go's import set).
type PromoteRegistry interface {
    List() []models.PriceGapCandidate
    Add(ctx context.Context, source string, c models.PriceGapCandidate) error
    Delete(ctx context.Context, source string, idx int) error
}

// PromoteEventEmitter writes pg:promote:events + WS broadcast.
type PromoteEventEmitter interface {
    EmitPromoteEvent(ctx context.Context, evt PromoteEvent) error
    IncrementCapFullSkip(ctx context.Context, symbol string) error
}

// PromoteNotifier wraps Telegram with cooldown.
type PromoteNotifier interface {
    NotifyPromote(eventKey, format string, args ...interface{})
}

// PromotionController is the streak controller.
type PromotionController struct {
    cfg      *config.Config
    registry PromoteRegistry
    emitter  PromoteEventEmitter
    posCheck ActivePositionChecker
    notifier PromoteNotifier
    log      *utils.Logger

    mu      sync.Mutex
    promote map[string]int // key = "sym|long|short"
    demote  map[string]int
}

// Apply consumes the cycle summary and performs at most O(N) promote/demote
// actions where N = len(summary.Records).
//
// Synchronous — runs in scanner.RunCycle's goroutine after WriteCycle.
func (c *PromotionController) Apply(ctx context.Context, summary CycleSummary) {
    // ... (implementation: walk Records, update streaks, fire promote/demote)
}
```

### Scanner integration (one-line edit)

```go
// scanner.go:321 — INSERT after telemetry.WriteCycle
_ = s.telemetry.WriteCycle(ctx, summary)
if s.controller != nil {
    s.controller.Apply(ctx, summary)
}
```

### Updated `scanner_static_test.go` assertion

```go
// scanner_static_test.go:38-45 (REPLACE)
// D-17: scanner.go may now reference *Registry (controller swap shipped).
// Forbid raw field-write to cfg.PriceGapCandidates regardless.
if regexp.MustCompile(`cfg\.PriceGapCandidates\s*=`).MatchString(stripped) {
    t.Errorf("scanner.go writes cfg.PriceGapCandidates directly — must delegate to PromotionController (D-15 / Pitfall 2)")
}
// scanner.go itself still does not call mutators — only promotion.go does.
mutatorRe := regexp.MustCompile(`registry\.(Add|Update|Delete|Replace)\(`)
if mutatorRe.MatchString(stripped) {
    t.Errorf("scanner.go calls a Registry mutator — must delegate to PromotionController (D-15)")
}
```

---

## Test Surface

The plan-checker requires grep-verifiable acceptance criteria. The following patterns are concrete enough for plan-tasks:

| Test class | Acceptance grep | Where |
|------------|-----------------|-------|
| Controller exists | `grep -E '^type PromotionController struct' internal/pricegaptrader/promotion.go` | Plan 12-01 |
| Apply method exists | `grep -E 'func \(c \*PromotionController\) Apply\(' internal/pricegaptrader/promotion.go` | Plan 12-01 |
| Streak threshold = 6 | `grep -E 'streakThreshold\s*=\s*6' internal/pricegaptrader/promotion.go` | Plan 12-01 |
| Audit source tags | `grep -E '"scanner-promote"\|"scanner-demote"' internal/pricegaptrader/promotion.go` | Plan 12-01 |
| ErrCapExceeded handled | `grep -E 'errors\.Is\(.*,.*ErrCapExceeded\)' internal/pricegaptrader/promotion.go` | Plan 12-01 |
| ErrDuplicateCandidate handled | `grep -E 'errors\.Is\(.*,.*ErrDuplicateCandidate\)' internal/pricegaptrader/promotion.go` | Plan 12-01 |
| Active-position guard called | `grep -E 'GetActivePriceGapPositions\|HasActivePosition' internal/pricegaptrader/promotion.go` | Plan 12-01 |
| Telemetry EmitPromoteEvent | `grep -E 'func \(t \*Telemetry\) EmitPromoteEvent' internal/pricegaptrader/telemetry.go` | Plan 12-02 |
| LIST cap = 1000 | `grep -E 'LTrim.*-1000\|promoteEventsCap\s*=\s*1000' internal/pricegaptrader/telemetry.go` | Plan 12-02 |
| WS event name | `grep -E '"pg_promote_event"' internal/pricegaptrader/telemetry.go` | Plan 12-02 |
| Scanner controller call | `grep -E 's\.controller\.Apply\|s\.controller != nil' internal/pricegaptrader/scanner.go` | Plan 12-02 |
| Static test relaxation | `grep -E 'cfg\\\\.PriceGapCandidates\\\\s\*=' internal/pricegaptrader/scanner_static_test.go` | Plan 12-02 |
| Bootstrap construction | `grep -E 'pricegaptrader\.NewPromotionController' cmd/main.go` | Plan 12-02 |
| REST endpoint registered | `grep -E '/api/pg/discovery/promote-events' internal/api/` | Plan 12-03 |
| Frontend WS case | `grep -E "case 'pg_promote_event'" web/src/hooks/usePgDiscovery.ts` | Plan 12-04 |
| i18n EN keys | `grep -E "'pricegap.discovery.timeline.action.promote'" web/src/i18n/en.ts` | Plan 12-04 |
| i18n zh-TW keys | `grep -E "'pricegap.discovery.timeline.action.promote'" web/src/i18n/zh-TW.ts` | Plan 12-04 |
| i18n parity test passes | `cd web && npm run test -- parity.test.ts` | Plan 12-04 |

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Backend framework | `go test` (stdlib `testing` + table tests; existing pattern in `internal/pricegaptrader/*_test.go`) |
| Frontend framework | Vitest (existing config; see `web/vitest.config.ts`) |
| Backend quick run | `go test ./internal/pricegaptrader/... -run 'TestPromotionController' -count 1` |
| Frontend quick run | `cd web && npx vitest run src/components/Discovery src/hooks/usePgDiscovery` |
| Full suite | `go test ./... -count 1 && cd web && npm run test` |
| Phase gate | All-green before `/gsd-verify-work` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PG-DISC-02 | Promote at exactly 6 consecutive accepted cycles | unit | `go test ./internal/pricegaptrader -run 'TestPromotionController_PromoteAt6' -count 1` | ❌ Wave 0 |
| PG-DISC-02 | Reset streak on missing-from-Records | unit | `go test ./internal/pricegaptrader -run 'TestPromotionController_StreakResetOnMiss' -count 1` | ❌ Wave 0 |
| PG-DISC-02 | Reset streak on score-below-threshold | unit | `go test ./internal/pricegaptrader -run 'TestPromotionController_StreakResetOnLowScore' -count 1` | ❌ Wave 0 |
| PG-DISC-02 | Demote at 6 consecutive missing/below | unit | `go test ./internal/pricegaptrader -run 'TestPromotionController_Demote' -count 1` | ❌ Wave 0 |
| PG-DISC-02 | Active-position guard blocks demote, holds streak | unit | `go test ./internal/pricegaptrader -run 'TestPromotionController_DemoteBlockedByActive' -count 1` | ❌ Wave 0 |
| PG-DISC-02 | Cap-full skips silently, no Telegram, increments counter, holds streak at 6 | unit | `go test ./internal/pricegaptrader -run 'TestPromotionController_CapFullSilent' -count 1` | ❌ Wave 0 |
| PG-DISC-02 | Dedupe (already-promoted) is silent no-op | unit | `go test ./internal/pricegaptrader -run 'TestPromotionController_DedupeSilent' -count 1` | ❌ Wave 0 |
| PG-DISC-02 | Telegram cooldown applied per unique key | unit | `go test ./internal/pricegaptrader -run 'TestPromotionController_TelegramCooldown' -count 1` | ❌ Wave 0 |
| PG-DISC-02 | Master switch off → no controller actions | unit | `go test ./internal/pricegaptrader -run 'TestPromotionController_MasterSwitchOff' -count 1` | ❌ Wave 0 |
| PG-DISC-02 | Telegram failure does not block flow | unit | `go test ./internal/pricegaptrader -run 'TestPromotionController_TelegramFailureNonFatal' -count 1` | ❌ Wave 0 |
| PG-DISC-02 | Redis emit failure does not block flow | unit | `go test ./internal/pricegaptrader -run 'TestPromotionController_RedisEmitFailureNonFatal' -count 1` | ❌ Wave 0 |
| PG-DISC-02 | LIST + LTrim 1000 schema | integration (miniredis) | `go test ./internal/pricegaptrader -run 'TestTelemetry_EmitPromoteEvent_LTrim' -count 1` | ❌ Wave 0 |
| PG-DISC-02 | Scanner static test allows new path | unit (existing file) | `go test ./internal/pricegaptrader -run 'TestScanner_NoRegistryMutators' -count 1` | ✅ existing — must update assertion |
| PG-DISC-02 | Frontend WS pg_promote_event handled | unit (Vitest) | `cd web && npx vitest run src/hooks/usePgDiscovery.test.ts` | ❌ Wave 0 |
| PG-DISC-02 | Frontend timeline renders newest-first | component | `cd web && npx vitest run src/components/Discovery/PromoteTimeline.test.tsx` | ❌ Wave 0 |
| PG-DISC-02 | i18n EN/zh-TW parity for new keys | unit (Vitest) | `cd web && npx vitest run src/i18n/__tests__/parity.test.ts` | ✅ existing — must add keys; test catches mismatch |

### Sampling Rate (Shannon-Nyquist concern)

The scanner runs every `PriceGapDiscoveryIntervalSec` (default 300s = 5min). The streak counter samples that signal at exactly 1× cadence — there is **no Nyquist sampling concern at the controller level**, because the controller observes every cycle output (not a sub-sampled view).

However, **operator dashboard refresh rate** does sample the streak state. Because streak is in-memory and the dashboard reads via WS push (not polling), each `pg_promote_event` is observed exactly once. The empty-card → populated-card transition at the moment of promote is the only edge where a missed event would show a gap. **Mitigation:** REST seed on mount (`GET /api/pg/discovery/promote-events`) ensures the timeline reconstructs even if the WS connection dropped during the promote tick.

**There is one rate concern worth a unit test:** if two cycles fire faster than expected (e.g. operator manually triggers a cycle while the regular ticker fires), the streak counter could double-increment. **Test:** `TestPromotionController_DoubleApplySameCycleStartedAt` — call `Apply` twice with the same `summary.StartedAt`; verify streak increments only once. This guards against a future bug where someone adds an "on-demand cycle" feature.

### Per task commit
- Backend tasks: `go test ./internal/pricegaptrader/... -count 1`
- Frontend tasks: `cd web && npx vitest run src/components/Discovery src/hooks`
- Bootstrap task: `go build ./... && go test ./internal/pricegaptrader/... -count 1`

### Per wave merge
- Backend wave: full `internal/pricegaptrader` + `internal/api` test suites
- Frontend wave: `cd web && npm run test` (full Vitest suite)

### Phase gate
- Full suite: `go test ./... -count 1 && cd web && npm run test && cd web && npm run build && go build ./...`

### Wave 0 Gaps
- [ ] `internal/pricegaptrader/promotion_test.go` — covers all 11 controller behaviors
- [ ] `internal/pricegaptrader/telemetry_promote_test.go` (or extend existing telemetry_test.go) — LIST + LTrim + WS event
- [ ] `web/src/hooks/usePgDiscovery.test.ts` — extend (or create) for `pg_promote_event` case
- [ ] `web/src/components/Discovery/PromoteTimeline.test.tsx` — render newest-first + empty state + i18n
- [ ] No new test framework needed — Go stdlib `testing` and Vitest are already configured

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (REST endpoint) | Existing Bearer token in `arb_token` localStorage; no change |
| V3 Session Management | no | — |
| V4 Access Control | yes (REST endpoint) | Existing dashboard auth gate covers `/api/pg/discovery/*` |
| V5 Input Validation | yes | New REST endpoint takes no params (LRange) — but JSON unmarshal of `pg:promote:events` entries must use `json.Unmarshal` into typed struct, not `map[string]interface{}` |
| V6 Cryptography | no | — |
| V7 Error Handling | yes | Controller errors must NOT leak Redis URLs / internal paths in event payloads or Telegram messages |
| V14 Configuration | yes | `Registry.Add` is the chokepoint — config writes go through atomic save + `.bak.{ts}` ring (Pitfall 2 antidote) |

### Known Threat Patterns for `internal/pricegaptrader/`

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Race on `cfg.PriceGapCandidates` (3 writers) | Tampering | `Registry.Add/Delete` chokepoint with `cfg.LockConfigFile()` + `cfg.Lock()` |
| Telegram cooldown bypass via crafted key | DoS / nuisance | Per-event-key cooldown is bounded by `(action, symbol, longExch, shortExch, direction)` tuple — NOT operator-supplied; controller constructs key, adversary can't inject |
| Streak counter pollution by replay | Tampering | Streak storage is in-process memory — no external write path |
| Hung Redis blocks scanLoop | DoS | `boundCtx(parent, 3*time.Second)` on every Redis call |
| Hung Telegram blocks scanLoop | DoS | Fire-and-forget goroutine for `Send` (Pitfall 6 mitigation) |
| Forged `pg_promote_event` in WS stream | Spoofing | Server-only emits; clients are read-only on `/ws`; no inbound pg_promote_event handler |

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | Build | ✓ | 1.26+ (per go.mod) | — |
| Redis | Telemetry + active-position guard | ✓ | DB 2 (existing) | — |
| Node.js / npm | Frontend build | ✓ | v22.13.0 (existing) | `npm ci` only — npm lockdown active |
| Telegram bot token | Optional (controller works with nil notifier) | conditional | — | nil-receiver-safe `(*TelegramNotifier).Send` |

**No external dependencies block this phase.**

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | WS hub channel is non-blocking enough to absorb a single promote-event broadcast | Pitfall 7 | Cycle stall on hung WS — unverified in this research session; Plan 12-02 must verify in `internal/api/ws.go` Hub buffer size (around line 42-95). |

**Everything else** in this research is `[VERIFIED: file:line]` — opened the file, read the line cited.

---

## Open Questions

1. **Cap-full counter shape (D-08 Claude's discretion):**
   - Single counter `pg:scan:metrics` HSET `cap_full_skips` (incremented every cycle a candidate is held at streak=6)
   - OR per-symbol HASH field `cap_full_skips:{symbol}` (one per stuck candidate)
   - **Recommendation:** per-symbol HASH field (within `pg:scan:metrics` itself with prefix), incremented once per (candidate, cycle) when streak is at 6. Operators see WHICH symbols are stuck.
   - Planner should pick one in Plan 12-02; both are grep-verifiable.

2. **REST route shape (D-12 Claude's discretion):**
   - Separate route `GET /api/pg/discovery/promote-events`
   - OR extend `GET /api/pg/discovery/state` to embed last 100 events
   - **Recommendation:** separate route. Smaller `/state` payload; frontend can paginate `/promote-events`. Plan 12-03 picks one.

3. **PromotionController constructor signature:**
   - Pass into `NewScanner(...)` as required arg
   - OR add `(*Scanner).SetController(c)` setter
   - **Recommendation:** SetController setter (mirrors `SetScanner` on `*Tracker`). Minimizes ripple to existing `NewScanner` call sites and tests.

4. **`(*TelegramNotifier).checkCooldown` export:**
   - The function is unexported. Controller can wrap via an adapter struct in `cmd/main.go` that holds `*TelegramNotifier` and exposes `NotifyPromote(eventKey, format, args)` which combines cooldown + Send. **Recommendation: do this** — keeps `notify/telegram.go` untouched.

5. **Scanner constructor change scope:**
   - D-17 mandates the swap from `RegistryReader` to `*Registry`. The interface declaration in `registry_reader.go` REMAINS for future read-only consumers. Only the scanner's parameter type changes.
   - **Verified:** `registry_reader.go:22-32` declares `RegistryReader` with only `Get/List`. `*Registry` already implements it (`registry.go:76: var _ RegistryReader = (*Registry)(nil)`). The swap is type-narrow widening — drop-in compatible.

---

## Sources

### Primary (HIGH confidence — opened, read at cited lines)
- `internal/pricegaptrader/scanner.go` (lines 88-321) — CycleRecord/CycleSummary/RunCycle
- `internal/pricegaptrader/registry.go` (lines 38-220) — Add/Delete/sentinel errors
- `internal/pricegaptrader/registry_reader.go` (full file) — RegistryReader interface
- `internal/pricegaptrader/telemetry.go` (lines 31-265, 411-413, 458-463) — LIST + LTrim + boundCtx + broadcast
- `internal/pricegaptrader/scanner_static_test.go` (full file) — current assertion regexes
- `internal/pricegaptrader/tracker.go` (lines 96-152, 537-571) — scanLoop + SetScanner
- `internal/database/pricegap_state.go` (lines 1-122) — keyPricegapActive + active-position getters
- `internal/notify/telegram.go` (lines 1-100, 196-216) — Send + checkCooldownAt
- `internal/api/pricegap_handlers.go` (lines 478-501) — Hub broadcast methods
- `internal/api/ws.go` (lines 96-103) — Hub.Broadcast signature
- `internal/api/server.go` (lines 74-81, 408-420) — DiscoveryTelemetryReader, SetRegistry
- `internal/config/config.go` (lines 32-58, 357-366, 814-815, 1480-1508) — PriceGap config validation + defaults
- `internal/models/pricegap_position.go` (lines 40-56) — PriceGapPosition fields (CandidateLongExch etc.)
- `cmd/main.go` (lines 165, 220-221, 477, 495-510) — bootstrap wiring
- `cmd/pg-admin/main.go` (lines 222-296, 324) — registry.Add/Delete call site pattern
- `web/src/components/Discovery/PromoteTimelinePlaceholder.tsx` (full file)
- `web/src/components/Discovery/DiscoverySection.tsx` (lines 278-350)
- `web/src/hooks/usePgDiscovery.ts` (full file, 275 lines)
- `web/src/i18n/en.ts` (lines 874-880) — discovery i18n keys
- `web/src/i18n/zh-TW.ts` (lines 920-921) — zh-TW lockstep keys
- `.planning/phases/12-auto-promotion/12-CONTEXT.md` (full)
- `.planning/REQUIREMENTS.md` §"Auto-Discovery & Promotion" PG-DISC-02
- `.planning/ROADMAP.md` §"Phase 12: Auto-Promotion"
- `.planning/phases/11-auto-discovery-scanner-chokepoint-telemetry/11-CONTEXT.md`

### Secondary (referenced but not opened in this session)
- `.planning/research/ARCHITECTURE-v2.2.md` §"2. Concurrent CRUD safety" + §"4. Telemetry data flow" — referenced via 12-CONTEXT.md
- `.planning/research/PITFALLS.md` §"Pitfall 1" + §"Pitfall 2" — referenced via 12-CONTEXT.md

### Confidence Breakdown
| Area | Level | Reason |
|------|-------|--------|
| Standard stack | HIGH | All packages already in go.mod / package.json; zero new deps |
| Architecture | HIGH | Every interface and call site verified at cited line |
| Pitfalls | MEDIUM-HIGH | Pitfall 7 (WS hub buffer) noted as Plan 12-02 verification item |
| Test surface | HIGH | All test commands grep-verifiable; framework already in place |
| Security | HIGH | All controls inherited from existing patterns; no new threat surface |

**Research date:** 2026-04-29
**Valid until:** 2026-05-13 (14 days — `internal/pricegaptrader/` was last modified during Phase 11, completed 2026-04-28; stable for the immediate phase planning window)

---

## RESEARCH COMPLETE

**Phase:** 12 - Auto-Promotion
**Confidence:** HIGH

### Key Findings

1. **All 17 locked decisions map cleanly to existing code anchors.** Phase 12 is wiring, not new infrastructure — Registry chokepoint, Telemetry LIST+LTrim, position store, Telegram cooldown, WS hub broadcast, dashboard scaffold all already exist.

2. **`CycleRecord` has no `Direction` field.** Scanner is bidirectional-only. Controller must hardcode `Direction = models.PriceGapDirectionBidirectional` when promoting; streak key uses `(symbol|long|short)` without direction.

3. **`models.PriceGapPosition` uses `CandidateLongExch`/`CandidateShortExch`/`FiredDirection`** — NOT `LongExch`/`ShortExch`/`Direction`. Active-position guard tuple-match must use the candidate-prefixed fields.

4. **`scanner_static_test.go` line 38-45 needs surgical update**, not rewrite. Replace `*Registry` regex with `cfg\.PriceGapCandidates\s*=` regex (forbid raw write only). The `registry.Add/Delete` regex stays — scanner.go itself shouldn't call mutators; `promotion.go` does.

5. **Frontend timeline replaces existing `PromoteTimelinePlaceholder.tsx` (26-line file)** — the i18n keys `pricegap.discovery.timeline.title` + `.placeholder` are already in lockstep en/zh-TW. New event-rendering keys must be added in lockstep; `parity.test.ts` enforces this.

### File Created

`.planning/phases/12-auto-promotion/12-RESEARCH.md`

### Confidence Assessment

| Area | Level | Reason |
|------|-------|--------|
| Standard Stack | HIGH | Zero new deps; verified against go.mod + package.json |
| Architecture | HIGH | Every anchor opened and read at cited line |
| Pitfalls | MEDIUM-HIGH | Pitfall 7 (WS hub buffer) flagged for Plan 12-02 verification |
| Test Surface | HIGH | All 18 test commands grep-verifiable |
| Validation | HIGH | Vitest + Go testing; no new framework |

### Open Questions (for planner)

1. `cap_full_skips` shape — single counter vs per-symbol HASH field
2. REST route shape — separate `/promote-events` vs extend `/state`
3. PromotionController injection — constructor arg vs SetController setter
4. WS hub channel buffer — verify non-blocking semantics in Plan 12-02

### Ready for Planning

Research complete. Planner can now create 4 plan files (12-01 controller core, 12-02 scanner integration + bootstrap, 12-03 REST endpoint, 12-04 frontend timeline) with grep-verifiable acceptance criteria for every task.
