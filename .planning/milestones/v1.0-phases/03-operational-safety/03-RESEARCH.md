# Phase 3: Operational Safety - Research

**Researched:** 2026-04-02
**Domain:** Go backend safety features (Telegram notifications, loss limits) + React dashboard integration
**Confidence:** HIGH

## Summary

Phase 3 adds two safety features to the perp-perp engine: Telegram notifications for critical events and rolling-window loss limits that halt new entries. Circuit breaker (PP-02) has been dropped per user decision D-05. The codebase already has a `TelegramNotifier` in `internal/notify/telegram.go` used by the spot-futures engine -- the perp-perp engine needs the same pattern wired in with new notification methods for SL triggers, emergency closes, and consecutive API errors. Loss limits need a new rolling-window PnL tracker using Redis sorted sets, a pre-entry gate in `executeArbitrage()`, and a dashboard banner on the Overview page.

All required patterns are already established in the codebase: the config 6-touch-point convention, the ToggleField/NumberField dashboard components, the `BroadcastAlert` WS push pattern, and the nil-receiver-safe TelegramNotifier pattern. No new libraries or external dependencies are needed.

**Primary recommendation:** Build Telegram notifications first (reuses existing `TelegramNotifier`), then loss limits (new Redis data structure + pre-entry gate). Both features follow the established `Enable{Feature}` config pattern with dashboard toggles (default OFF).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Critical events only -- SL triggered, emergency close (L4/L5), 3+ consecutive API errors on same exchange. Target: 1-3 messages per day in normal conditions.
- **D-02:** Error alert trigger: 3 consecutive API failures on the same exchange. Not rate-based, not single-failure.
- **D-03:** Rate limiting: cooldown per event type -- same event type can only fire once per 5 minutes. Prevents spam during sustained outages.
- **D-04:** Extensible design -- architecture must make it easy to add spot-futures events later. The perp-perp integration should follow the same pattern so both engines share the notifier cleanly.
- **D-05:** Circuit breaker dropped from Phase 3 scope.
- **D-06:** Rolling time windows -- 24-hour and 7-day sliding windows, not calendar day/week resets.
- **D-07:** Breach action: halt new entries only. Existing positions continue to be managed.
- **D-08:** Independent from L0-L5 risk tiers. Loss limits track realized PnL losses. Risk tiers track margin health.
- **D-09:** Dashboard visibility: banner/badge on existing Overview page showing "Daily loss: -$X / $Y limit" -- turns red when approaching threshold. No dedicated safety page needed.
- **D-10:** All new features must have configurable on/off switch (default OFF) + dashboard toggle.

### Claude's Discretion
- Specific default threshold values for loss limits (daily and weekly amounts)
- TelegramNotifier refactoring approach -- whether to add perp-perp methods to existing notifier or create a shared notification dispatcher
- Redis key design for rolling loss window tracking
- Implementation ordering of plans (Telegram first vs loss limits first)

### Deferred Ideas (OUT OF SCOPE)
- Circuit breaker (PP-02) -- dropped, API failures already prevent entries naturally
- Telegram for non-critical events -- entries, exits, rotations, rebalances, funding snapshots
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PP-01 | Telegram notifications for perp-perp engine critical events (entry, exit, errors, SL triggers) | Existing TelegramNotifier pattern in `internal/notify/telegram.go` with nil-receiver safety. Scoped per D-01: SL triggered, emergency close (L4/L5), 3+ consecutive API errors only. Engine needs notifier field + injection in `cmd/main.go`. |
| PP-02 | Circuit breaker pauses trading on an exchange when error rate or latency exceeds threshold | **DROPPED** per D-05. Not in scope for Phase 3. |
| PP-03 | Daily and weekly loss limits halt new entries when threshold breached | Redis sorted set for rolling PnL events. Pre-entry gate in `executeArbitrage()`. Dashboard banner on Overview page. Config fields for enable + daily/weekly thresholds. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| go-redis/v9 | 9.18.0 | Redis sorted sets for rolling PnL windows | Already in project, sorted sets (ZADD/ZRANGEBYSCORE) are the standard Redis pattern for time-windowed aggregation |
| Go stdlib `net/http` | 1.26 | Telegram Bot API calls | Already used by TelegramNotifier, no extra dependency needed |
| Go stdlib `sync` | 1.26 | Cooldown timers and concurrent error counters | Already used throughout engine for mutexes and atomic operations |
| React 19 | 19.2.0 | Dashboard UI for loss limit banner | Already in project |
| Tailwind CSS 4 | 4.2.1 | Styling for banner component | Already in project |

### Supporting
No new libraries needed. Everything builds on existing stack.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Redis sorted sets | Redis lists + manual timestamp parsing | Sorted sets give O(log N) range queries by score (timestamp); lists require full scan. Sorted sets are the correct data structure. |
| In-memory cooldown map | Redis-based cooldown | In-memory is simpler and sufficient -- cooldowns reset on restart which is acceptable behavior (conservative: allows notifications to re-fire after restart) |
| Per-event-type cooldown | Global rate limit | Per-event-type is better: SL trigger and API error can fire close together without being suppressed |

## Architecture Patterns

### Recommended Project Structure

No new directories needed. New files integrate into existing packages:

```
internal/
  notify/
    telegram.go          # ADD: new methods for perp-perp events + cooldown logic
    telegram_test.go     # ADD: tests for new methods and cooldown
  engine/
    engine.go            # MODIFY: add telegram field, inject in NewEngine
    engine.go            # MODIFY: call notifications at SL trigger, emergency close
    exit.go              # MODIFY: call notifications at position close with loss
  risk/
    loss_limit.go        # NEW: rolling window loss tracker with Redis sorted set
    loss_limit_test.go   # NEW: tests for loss limit logic
  config/
    config.go            # MODIFY: add 5 new fields (enable + 4 thresholds)
  api/
    handlers.go          # MODIFY: add loss_limits to status/config response
  database/
    state.go             # MODIFY: add sorted set CRUD for loss events
web/src/
  pages/
    Overview.tsx         # MODIFY: add loss limit banner
    Config.tsx           # MODIFY: add safety section with toggles
  i18n/
    en.ts                # MODIFY: add i18n keys
    zh-TW.ts             # MODIFY: add i18n keys
  types.ts               # MODIFY: add LossLimitStatus type
```

### Pattern 1: TelegramNotifier Extension for Perp-Perp

**What:** Add new notification methods to the existing `TelegramNotifier` for perp-perp events, with per-event-type cooldown.

**When to use:** When a critical perp-perp event occurs (SL triggered, L4/L5 emergency close, 3+ consecutive API errors).

**Design:**
```go
// internal/notify/telegram.go additions

// Cooldown tracking (in-memory, resets on restart -- acceptable)
type TelegramNotifier struct {
    botToken string
    chatID   string
    client   *http.Client
    log      *utils.Logger
    
    // Per-event-type cooldown (D-03: same event type once per 5 min)
    cooldownMu sync.Mutex
    lastSent   map[string]time.Time // event type key -> last sent time
}

// NotifySLTriggered sends alert when stop-loss fires on perp-perp position.
func (t *TelegramNotifier) NotifySLTriggered(pos *models.ArbitragePosition, leg, exchange string) {
    if t == nil { return }
    if !t.checkCooldown("sl_triggered") { return }
    // format and send via go t.send(text)
}

// NotifyEmergencyClosePerp sends alert for L4/L5 margin emergency on perp-perp.
func (t *TelegramNotifier) NotifyEmergencyClosePerp(exchange string, level string, posCount int) {
    if t == nil { return }
    if !t.checkCooldown("emergency_close_perp:" + exchange) { return }
    // format and send
}

// NotifyConsecutiveAPIErrors sends alert when 3+ consecutive API failures detected.
func (t *TelegramNotifier) NotifyConsecutiveAPIErrors(exchange string, errCount int, lastErr error) {
    if t == nil { return }
    if !t.checkCooldown("api_errors:" + exchange) { return }
    // format and send
}

// checkCooldown returns true if enough time has passed since last notification
// of this type. Thread-safe.
func (t *TelegramNotifier) checkCooldown(eventKey string) bool {
    t.cooldownMu.Lock()
    defer t.cooldownMu.Unlock()
    now := time.Now()
    if last, ok := t.lastSent[eventKey]; ok && now.Sub(last) < 5*time.Minute {
        return false
    }
    t.lastSent[eventKey] = now
    return true
}
```

**Key design decisions:**
- Nil-receiver pattern preserved (matches existing spot-futures methods)
- Cooldown is in-memory (not Redis) -- simpler, resets on restart are acceptable
- Event key includes exchange name for per-exchange cooldown on API errors
- Methods use `go t.send()` pattern (non-blocking, fire-and-forget)

### Pattern 2: Consecutive API Error Counter

**What:** Track consecutive API failures per exchange, trigger notification at threshold.

**When to use:** When exchange PlaceOrder/GetPosition calls fail. The counter resets on success.

**Design:**
```go
// In engine.go or a new file in engine package
type apiErrorTracker struct {
    mu       sync.Mutex
    counts   map[string]int // exchange -> consecutive failure count
    telegram *notify.TelegramNotifier
}

func (t *apiErrorTracker) recordError(exchange string, err error) {
    t.mu.Lock()
    t.counts[exchange]++
    count := t.counts[exchange]
    t.mu.Unlock()
    
    if count == 3 { // D-02: trigger at exactly 3
        t.telegram.NotifyConsecutiveAPIErrors(exchange, count, err)
    }
}

func (t *apiErrorTracker) recordSuccess(exchange string) {
    t.mu.Lock()
    t.counts[exchange] = 0
    t.mu.Unlock()
}
```

**Integration points:** The error counter hooks into the existing PlaceOrder error paths in `executeArbitrage()` and depth fill loops. Success resets the counter.

### Pattern 3: Rolling Window Loss Limits with Redis Sorted Sets

**What:** Track realized PnL events in a Redis sorted set keyed by timestamp, query rolling windows for limit checks.

**When to use:** Before every new entry attempt, and on dashboard status poll.

**Design:**
```go
// Redis key: arb:loss_events (sorted set, score = unix timestamp)
// Member: JSON {pnl: -12.34, pos_id: "abc123", closed_at: "2026-04-02T10:00:00Z"}

// internal/risk/loss_limit.go
type LossLimitChecker struct {
    db           *database.Client
    cfg          *config.Config
    log          *utils.Logger
    telegram     *notify.TelegramNotifier
}

// RecordClosedPnL adds a PnL event to the rolling window.
func (l *LossLimitChecker) RecordClosedPnL(posID string, pnl float64, closedAt time.Time) error {
    // ZADD arb:loss_events <unix_timestamp> <json_member>
    // ZREMRANGEBYSCORE arb:loss_events -inf <7_days_ago> (trim old data)
}

// CheckLimits returns (blocked bool, status LossLimitStatus).
func (l *LossLimitChecker) CheckLimits() (bool, LossLimitStatus) {
    now := time.Now()
    // ZRANGEBYSCORE arb:loss_events <24h_ago> +inf -> sum for daily
    // ZRANGEBYSCORE arb:loss_events <7d_ago> +inf  -> sum for weekly
    // Compare against thresholds
}

type LossLimitStatus struct {
    Enabled       bool    `json:"enabled"`
    DailyLoss     float64 `json:"daily_loss"`
    DailyLimit    float64 `json:"daily_limit"`
    WeeklyLoss    float64 `json:"weekly_loss"`
    WeeklyLimit   float64 `json:"weekly_limit"`
    Breached      bool    `json:"breached"`
    BreachType    string  `json:"breach_type"` // "daily", "weekly", or ""
}
```

**Redis sorted set operations:**
- `ZADD arb:loss_events <timestamp> <json>` -- record event
- `ZRANGEBYSCORE arb:loss_events <window_start> +inf` -- query window
- `ZREMRANGEBYSCORE arb:loss_events -inf <7_days_ago>` -- prune old data (run on each write)

**Why sorted sets:** Score = Unix timestamp gives O(log N) range queries. No scanning of all events. Clean rolling window semantics. Automatic ordering. The project already uses go-redis v9 which supports all ZSET operations.

### Pattern 4: Pre-Entry Gate Integration

**What:** Check loss limits before allowing new entries in `executeArbitrage()`.

**Integration point:**
```go
// In engine.go executeArbitrage(), after acquiring capacity lock,
// before processing opportunities:
if e.lossLimiter != nil && e.cfg.EnableLossLimits {
    blocked, status := e.lossLimiter.CheckLimits()
    if blocked {
        e.log.Warn("loss limit breached (%s): daily=%.2f/%.2f weekly=%.2f/%.2f -- halting entries",
            status.BreachType, status.DailyLoss, status.DailyLimit,
            status.WeeklyLoss, status.WeeklyLimit)
        e.capacityMu.Unlock()
        return
    }
}
```

### Pattern 5: Dashboard Banner on Overview

**What:** A banner at the top of the Overview page showing loss limit status.

**Design:**
```tsx
// In Overview.tsx, above the stat cards grid
{lossLimits && lossLimits.enabled && (
  <div className={`rounded-lg p-3 border ${
    lossLimits.breached 
      ? 'bg-red-900/30 border-red-700 text-red-300'
      : dailyPct > 0.8 
        ? 'bg-yellow-900/30 border-yellow-700 text-yellow-300'
        : 'bg-gray-900 border-gray-800 text-gray-300'
  }`}>
    <div className="flex items-center justify-between text-sm">
      <span>{t('overview.lossLimits')}</span>
      <div className="flex gap-4 font-mono">
        <span>24h: ${Math.abs(lossLimits.daily_loss).toFixed(2)} / ${lossLimits.daily_limit.toFixed(2)}</span>
        <span>7d: ${Math.abs(lossLimits.weekly_loss).toFixed(2)} / ${lossLimits.weekly_limit.toFixed(2)}</span>
      </div>
    </div>
    {lossLimits.breached && (
      <div className="text-xs mt-1 font-semibold">{t('overview.lossLimitBreached')}</div>
    )}
  </div>
)}
```

**Data delivery:** Loss limit status pushed via WebSocket `loss_limits` message type, and also available via REST `GET /api/status` response.

### Anti-Patterns to Avoid

- **Don't add notifications inside tight loops:** The depth fill loop runs many iterations per second. Only notify on final outcomes (SL confirmed, position fully closed), not intermediate states.
- **Don't use Redis for cooldown timers:** Overkill for a simple in-memory map. Cooldown state doesn't need persistence -- it's fine if cooldowns reset on restart.
- **Don't create a separate safety dashboard page:** Per D-09, use a banner on the existing Overview page. Keep it simple.
- **Don't interact with the L0-L5 risk tier system:** Per D-08, loss limits are completely independent. Don't check margin health in the loss limiter or modify risk tiers based on loss limits.
- **Don't record winning trades in the loss events sorted set:** Only losses are relevant for the limit check. Recording all trades wastes memory and complicates the sum calculation.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Rolling time window | Manual timestamp parsing on Redis lists | Redis sorted sets (ZADD/ZRANGEBYSCORE) | O(log N) range queries vs O(N) full scan; built-in score-based ordering |
| Rate limiting notifications | Custom timer goroutines | sync.Mutex + map[string]time.Time | Simple, thread-safe, no goroutine overhead |
| Telegram Bot API | Full HTTP client with retries | Existing `send()` method in TelegramNotifier | Already handles HTTP POST, error logging, response checking |

**Key insight:** The codebase already has all the building blocks. TelegramNotifier exists with the right patterns. Redis client supports sorted sets. Config system has the 6-touch-point convention. The work is wiring and integration, not building new infrastructure.

## Common Pitfalls

### Pitfall 1: Blocking the Engine Loop with Telegram API Calls
**What goes wrong:** Telegram API takes 1-5 seconds. If called synchronously in the SL handler or emergency close path, it blocks time-critical close operations.
**Why it happens:** Forgetting the `go t.send(text)` goroutine pattern.
**How to avoid:** All notification methods MUST use `go t.send(text)`, never synchronous send. This is already the pattern in spot-futures notifications.
**Warning signs:** Position close latency spikes correlated with Telegram network issues.

### Pitfall 2: Cooldown Key Granularity
**What goes wrong:** Too-coarse cooldown keys (e.g., "api_error" for all exchanges) suppress alerts for exchange B when exchange A already triggered. Too-fine keys (e.g., per-symbol per-exchange) generate too many messages.
**Why it happens:** Not thinking through the event deduplication model.
**How to avoid:** Use `eventType:exchange` as the cooldown key. Per D-03, same event type + exchange combination can only fire once per 5 minutes.
**Warning signs:** Either getting spammed or missing alerts during multi-exchange issues.

### Pitfall 3: Loss Limit Window Drift
**What goes wrong:** Using `time.Now()` at query time but storing events with different clock sources leads to events falling in/out of the window inconsistently.
**Why it happens:** Mixing server time, Redis time, and exchange-reported close time.
**How to avoid:** Always use `time.Now().UTC()` at the moment of PnL recording. The sorted set score is the local server timestamp, not the exchange timestamp.
**Warning signs:** Loss limit status fluctuating between "breached" and "not breached" without new trades.

### Pitfall 4: Double-Counting PnL After Reconciliation
**What goes wrong:** Recording the PnL event when position closes, then reconciliation changes the PnL value, but the sorted set still has the old value.
**Why it happens:** `reconcilePnL` runs async after position close and may update `RealizedPnL`.
**How to avoid:** Record the loss event at close time with the initial PnL estimate. The reconciliation diff is typically small (< $1) and not worth the complexity of updating sorted set members. Alternatively, use pos.ID as the sorted set member and upsert on reconciliation.
**Warning signs:** Cumulative loss in the tracker doesn't match the stats hash `total_pnl`.

### Pitfall 5: Config Hot-Reload Not Wiring Loss Limiter
**What goes wrong:** Changing loss limit thresholds in the dashboard doesn't take effect until restart.
**Why it happens:** The loss limiter reads config once at construction time instead of checking `cfg.DailyLossLimit` on each call.
**How to avoid:** Read threshold values from `cfg` at check time, not construction time. The `Config` struct is already protected by `sync.RWMutex` for concurrent access. The loss limiter holds a pointer to the shared `*config.Config`.
**Warning signs:** Changing thresholds in dashboard has no effect.

### Pitfall 6: Not Updating Both i18n Files
**What goes wrong:** Adding English keys to `en.ts` but forgetting `zh-TW.ts`, or vice versa. This causes TypeScript compilation errors because `TranslationKey` is derived from the English keys.
**Why it happens:** Easy to forget in a dual-locale system.
**How to avoid:** Always edit both files in the same commit. The TypeScript compiler will catch missing keys in `zh-TW.ts` if keys are added to `en.ts`.
**Warning signs:** `npm run build` fails with missing key errors.

## Code Examples

Verified patterns from existing codebase:

### Existing TelegramNotifier Nil-Receiver Pattern
```go
// Source: internal/notify/telegram.go:58-77
func (t *TelegramNotifier) NotifyAutoEntry(pos *models.SpotFuturesPosition, expectedYieldAPR float64) {
    if t == nil {
        return
    }
    // ... format message ...
    go t.send(text)
}
```

### Existing Config 6-Touch-Point Pattern
For adding a new config field (e.g., `EnableLossLimits`):
1. **Struct field:** `EnableLossLimits bool` in Config struct
2. **JSON struct:** `EnableLossLimits *bool \`json:"enable_loss_limits"\`` in the nested JSON struct
3. **Default value:** `EnableLossLimits: false` in `Load()`
4. **applyJSON:** `if jc.Safety != nil && jc.Safety.EnableLossLimits != nil { c.EnableLossLimits = *jc.Safety.EnableLossLimits }`
5. **toJSON:** `safety["enable_loss_limits"] = c.EnableLossLimits` in the save function
6. **fromEnv:** `if v := os.Getenv("ENABLE_LOSS_LIMITS"); v != "" { c.EnableLossLimits = v == "1" || v == "true" }`

### Existing Redis Key Pattern
```go
// Source: internal/database/state.go:18-37
const (
    keyConfig              = "arb:config"
    keyPositions           = "arb:positions"
    keyHistory             = "arb:history"
    keyStats               = "arb:stats"
    // NEW: keyLossEvents  = "arb:loss_events"
)
```

### Existing Dashboard Config Response Pattern
```go
// Source: internal/api/handlers.go:305-313
type configResponse struct {
    DryRun      bool                              `json:"dry_run"`
    Strategy    configStrategyResponse            `json:"strategy"`
    // ...
    SpotFutures *configSpotFuturesResponse        `json:"spot_futures,omitempty"`
    // NEW: Safety *configSafetyResponse `json:"safety,omitempty"`
}
```

### Existing WebSocket Push Pattern
```go
// Source: internal/api/server.go:222-225
func (s *Server) BroadcastAlert(alert interface{}) {
    s.hub.Broadcast("alert", alert)
}
// NEW: BroadcastLossLimits follows this exact pattern
```

### Existing ToggleField Dashboard Pattern
```tsx
// Source: web/src/pages/Config.tsx:230-245
const ToggleField: FC<{
  label: string;
  desc?: string;
  value: unknown;
  onChange: (v: boolean) => void;
}> = ({ label, desc, value, onChange }) => {
  const checked = Boolean(value);
  return (
    <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
      {/* ... */}
      <ToggleSwitch on={checked} onChange={onChange} />
    </div>
  );
};
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Calendar-day loss limits | Rolling 24h/7d windows | Industry standard | No "reset at midnight" gaming; consistent protection |
| Per-message Telegram rate limits | Per-event-type cooldown | D-03 decision | Different event types don't suppress each other |
| Separate notification service | Methods on existing TelegramNotifier | D-04 decision | Simpler, both engines share one notifier instance |

## Config Fields Design

### New Config Fields (5 total)

| Field | Type | JSON Key | Default | Purpose |
|-------|------|----------|---------|---------|
| `EnableLossLimits` | bool | `enable_loss_limits` | false | Master on/off switch (D-10) |
| `DailyLossLimitUSDT` | float64 | `daily_loss_limit_usdt` | 100.0 | 24-hour rolling loss threshold |
| `WeeklyLossLimitUSDT` | float64 | `weekly_loss_limit_usdt` | 300.0 | 7-day rolling loss threshold |
| `EnablePerpTelegram` | bool | `enable_perp_telegram` | false | On/off for perp-perp Telegram notifications (D-10) |
| `TelegramCooldownSec` | int | `telegram_cooldown_sec` | 300 | Per-event-type cooldown (D-03, default 5 min) |

**Recommended defaults rationale (Claude's discretion):**
- Daily $100 / Weekly $300: Conservative starting point. With 1-3 concurrent positions of $200-500 per leg, a $100 daily loss limit would trigger after 2-3 bad exits. This is a reasonable safety net for a single-user system. Operator tunes based on portfolio size.
- These are configurable via dashboard -- user adjusts to their risk tolerance.

### JSON Config Section

Recommend a new top-level `safety` section in the JSON config to keep safety-related fields grouped:

```json
{
  "safety": {
    "enable_loss_limits": false,
    "daily_loss_limit_usdt": 100,
    "weekly_loss_limit_usdt": 300,
    "enable_perp_telegram": false,
    "telegram_cooldown_sec": 300
  }
}
```

## Redis Key Design

### Loss Events Sorted Set

**Key:** `arb:loss_events`
**Type:** Sorted set
**Score:** Unix timestamp (float64, seconds with fractional)
**Member:** JSON string: `{"pos_id":"abc123","pnl":-12.34,"symbol":"BTCUSDT"}`

**Operations:**
```
ZADD arb:loss_events <timestamp> <json_member>          # record event
ZRANGEBYSCORE arb:loss_events <24h_ago> +inf            # query 24h window
ZRANGEBYSCORE arb:loss_events <7d_ago> +inf             # query 7d window  
ZREMRANGEBYSCORE arb:loss_events -inf <8d_ago>          # prune (keep 8 days)
```

**Pruning strategy:** On each write, trim events older than 8 days (1 day buffer beyond the 7-day window). This keeps the sorted set small and bounded.

**Important:** Only store losing trades (negative PnL). Winning trades are irrelevant for loss limit calculation. This keeps the dataset small and the sum calculation simple: sum of all members in window = total loss.

Wait -- actually, per the rolling window, we need to sum ALL PnL (wins and losses) to get net realized loss. If a trader loses $50 then wins $30, the net daily loss is $20, not $50. Storing only losses would overstate the risk.

**Revised decision:** Store ALL closed position PnL events (wins and losses) in the sorted set. The rolling window query sums everything to get net PnL. If net PnL is below the negative threshold, entries are halted.

## Engine Integration Points

### 1. TelegramNotifier Injection into Engine

Currently `Engine` struct has no `telegram` field. Add it:

```go
// engine.go Engine struct
telegram *notify.TelegramNotifier
```

Modify `NewEngine` to accept it (or add a setter like `SetTelegram()`).

In `cmd/main.go`, create ONE TelegramNotifier instance and pass it to both engines:
```go
tg := notify.NewTelegram(cfg.TelegramBotToken, cfg.TelegramChatID)
// Pass to perp engine
engine.SetTelegram(tg)
// Spot engine already creates its own in NewSpotEngine -- should be refactored to share
```

### 2. SL Trigger Notification Point

In `engine.go:triggerEmergencyClose()` at line 1305, after the `BroadcastAlert` call:
```go
e.telegram.NotifySLTriggered(pos, leg, exchName)
```

### 3. Emergency Close Notification Point

In `engine.go:handleEmergencyClose()` at line 1414, after logging:
```go
e.telegram.NotifyEmergencyClosePerp(action.Exchange, "L5", len(action.Positions))
```

Also in `handleReduce()` for L4:
```go
e.telegram.NotifyEmergencyClosePerp(action.Exchange, "L4", len(action.Positions))
```

### 4. API Error Counter Points

In `executeArbitrage()` where PlaceOrder errors are handled (depth fill loop), and in `closePositionWithMode()` on exchange API errors. Reset on successful PlaceOrder calls.

### 5. Loss Limit Recording Point

In `exit.go` after `db.UpdateStats()` calls (two locations: around line 748 and line 1569):
```go
if e.lossLimiter != nil {
    e.lossLimiter.RecordClosedPnL(pos.ID, realizedPnL, time.Now().UTC())
}
```

### 6. Loss Limit Check Point

In `executeArbitrage()` at line 1758, immediately after acquiring `capacityMu`:
```go
if e.lossLimiter != nil && e.cfg.EnableLossLimits {
    blocked, status := e.lossLimiter.CheckLimits()
    if blocked {
        e.log.Warn("loss limit breached: %s", status.BreachType)
        e.api.BroadcastLossLimits(status)
        e.telegram.NotifyLossLimitBreached(status)
        e.capacityMu.Unlock()
        return
    }
}
```

## Dashboard Integration

### Config Page: New Safety Section

Add a new tab to the existing strategy tabs structure. Recommend adding "safety" as a new strategy in the tab system:

```tsx
type Strategy = 'exchanges' | 'perp' | 'spot' | 'risk' | 'safety';
```

Or simpler: add safety fields to the existing Risk tab since they're conceptually related to risk management.

**Recommendation:** Add a new "Safety" sub-tab under the Risk strategy section, keeping it visually grouped with other risk controls.

### Overview Page: Loss Limit Banner

Position: Between the header and the stat cards grid.
Data source: WebSocket `loss_limits` message type, pushed whenever status changes.
Visual states:
- Green/gray: Enabled, losses within limits
- Yellow: Enabled, approaching limit (>80% of threshold)
- Red: Enabled, limit breached -- entries halted
- Hidden: Feature disabled

### New i18n Keys Needed

```
overview.lossLimits: 'Loss Limits'
overview.lossLimitBreached: 'ENTRIES HALTED - Loss limit breached'
overview.daily: '24h'
overview.weekly: '7d'
cfg.safety.title: 'Safety'
cfg.safety.enableLossLimits: 'Enable Loss Limits'
cfg.safety.enableLossLimitsDesc: 'Halt new entries when rolling loss exceeds threshold'
cfg.safety.dailyLimit: 'Daily Loss Limit (USDT)'
cfg.safety.dailyLimitDesc: 'Maximum net realized loss in rolling 24-hour window'
cfg.safety.weeklyLimit: 'Weekly Loss Limit (USDT)'
cfg.safety.weeklyLimitDesc: 'Maximum net realized loss in rolling 7-day window'
cfg.safety.enablePerpTelegram: 'Perp-Perp Telegram Alerts'
cfg.safety.enablePerpTelegramDesc: 'Send Telegram notifications for SL triggers, emergency closes, and API errors'
cfg.safety.telegramCooldown: 'Alert Cooldown (seconds)'
cfg.safety.telegramCooldownDesc: 'Minimum seconds between same-type notifications'
```

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` package |
| Config file | None (standard Go test conventions) |
| Quick run command | `go test ./internal/notify/ ./internal/risk/ -count=1 -v` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PP-01 | TelegramNotifier new methods are nil-receiver safe | unit | `go test ./internal/notify/ -run TestNilSafeNotify -v` | Exists (needs expansion) |
| PP-01 | Cooldown logic blocks duplicate notifications within 5 min | unit | `go test ./internal/notify/ -run TestCooldown -v` | Wave 0 |
| PP-01 | Cooldown allows different event types independently | unit | `go test ./internal/notify/ -run TestCooldownIndependent -v` | Wave 0 |
| PP-01 | API error counter triggers at exactly 3 consecutive failures | unit | `go test ./internal/engine/ -run TestAPIErrorCounter -v` | Wave 0 |
| PP-01 | API error counter resets on success | unit | `go test ./internal/engine/ -run TestAPIErrorCounterReset -v` | Wave 0 |
| PP-03 | Loss limiter records PnL and queries rolling 24h window | unit | `go test ./internal/risk/ -run TestLossLimit24h -v` | Wave 0 |
| PP-03 | Loss limiter queries rolling 7d window | unit | `go test ./internal/risk/ -run TestLossLimit7d -v` | Wave 0 |
| PP-03 | Loss limiter blocks when daily threshold exceeded | unit | `go test ./internal/risk/ -run TestLossLimitBlocks -v` | Wave 0 |
| PP-03 | Loss limiter allows entries when within limits | unit | `go test ./internal/risk/ -run TestLossLimitAllows -v` | Wave 0 |
| PP-03 | Loss limiter prunes events older than 8 days | unit | `go test ./internal/risk/ -run TestLossLimitPrune -v` | Wave 0 |
| PP-03 | Config fields follow 6-touch-point pattern | manual-only | Visual inspection of config.go | N/A |

### Sampling Rate
- **Per task commit:** `go test ./internal/notify/ ./internal/risk/ -count=1 -v`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/notify/telegram_test.go` -- add cooldown tests (TestCooldown, TestCooldownIndependent)
- [ ] `internal/risk/loss_limit_test.go` -- new file for loss limit unit tests (uses miniredis)
- [ ] `internal/engine/engine_test.go` -- add API error counter tests

Note: Loss limit tests should use `github.com/alicebob/miniredis/v2` (already in go.mod) for in-memory Redis sorted set testing.

## Open Questions

1. **Shared vs Separate TelegramNotifier Instance**
   - What we know: SpotEngine creates its own TelegramNotifier in `NewSpotEngine()` (line 88). The perp engine needs one too.
   - What's unclear: Should we refactor SpotEngine to accept a shared instance, or create a second one? Both work since TelegramNotifier is stateless for send operations (only cooldown state differs).
   - Recommendation: Share one instance. Pass the same `*notify.TelegramNotifier` to both engines. The cooldown map naturally handles both engines' events. Refactor SpotEngine's constructor to accept the notifier as a parameter instead of creating internally.

2. **Reconciliation PnL Adjustment**
   - What we know: `reconcilePnL` may adjust PnL after position close (typically small diff).
   - What's unclear: Should the sorted set be updated on reconciliation?
   - Recommendation: Don't update. The diff is typically < $1. The complexity of updating sorted set members (ZREM + ZADD with new value) isn't worth it for sub-dollar accuracy on a $100+ threshold.

## Sources

### Primary (HIGH confidence)
- `internal/notify/telegram.go` -- Existing TelegramNotifier implementation, nil-receiver pattern, send() method
- `internal/engine/engine.go` -- Engine struct, triggerEmergencyClose, handleEmergencyClose, executeArbitrage integration points
- `internal/engine/exit.go` -- Position close flow, PnL recording, UpdateStats calls
- `internal/config/config.go` -- Config struct pattern, 6-touch-point convention, Load() defaults
- `internal/database/state.go` -- Redis key constants, CRUD patterns
- `internal/api/handlers.go` -- REST response patterns, configResponse struct
- `web/src/pages/Config.tsx` -- ToggleSwitch, ToggleField, NumberField components
- `web/src/pages/Overview.tsx` -- Page layout, stat cards grid
- `web/src/hooks/useWebSocket.ts` -- WS message type handling
- `web/src/types.ts` -- TypeScript type definitions
- `go.mod` -- go-redis v9.18.0, miniredis v2.37.0

### Secondary (MEDIUM confidence)
- Redis sorted set operations (ZADD, ZRANGEBYSCORE, ZREMRANGEBYSCORE) -- standard Redis commands, well-documented in go-redis v9

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new dependencies, all existing libraries
- Architecture: HIGH -- follows established codebase patterns exactly
- Pitfalls: HIGH -- identified from direct code analysis of integration points
- Config design: HIGH -- follows existing 6-touch-point pattern with verified examples
- Redis design: HIGH -- sorted sets are the standard pattern for time-windowed aggregation
- Dashboard: HIGH -- reuses existing ToggleField/ToggleSwitch components

**Research date:** 2026-04-02
**Valid until:** 2026-05-02 (stable -- no external dependency changes expected)
