# Phase 3: Operational Safety - Context

**Gathered:** 2026-04-02
**Status:** Ready for planning

<domain>
## Phase Boundary

Safety nets for the perp-perp engine: Telegram notifications for critical events, daily/weekly loss limits that halt new entries. Protects the live trading system and keeps the operator informed without requiring constant monitoring.

</domain>

<decisions>
## Implementation Decisions

### Telegram Notifications (PP-01)
- **D-01:** Critical events only — SL triggered, emergency close (L4/L5), 3+ consecutive API errors on same exchange. Target: 1-3 messages per day in normal conditions.
- **D-02:** Error alert trigger: 3 consecutive API failures on the same exchange. Not rate-based, not single-failure.
- **D-03:** Rate limiting: cooldown per event type — same event type can only fire once per 5 minutes. Prevents spam during sustained outages.
- **D-04:** Extensible design — architecture must make it easy to add spot-futures events later (auto-entry, auto-exit, emergency close are already in TelegramNotifier but only wired to spotengine). The perp-perp integration should follow the same pattern so both engines share the notifier cleanly.

### Circuit Breaker (PP-02) — DROPPED
- **D-05:** Circuit breaker dropped from Phase 3 scope. Rationale: if an exchange API is failing, PlaceOrder already fails naturally — the circuit breaker would formalize what already happens. The 3-consecutive-error Telegram alert provides sufficient operator awareness. Deferred to backlog if flaky-but-not-dead API problems occur in practice.

### Loss Limits (PP-03)
- **D-06:** Rolling time windows — 24-hour and 7-day sliding windows, not calendar day/week resets.
- **D-07:** Breach action: halt new entries only. Existing positions continue to be managed (exits, rebalances, funding collection all continue). Operator receives Telegram alert with current loss numbers.
- **D-08:** Independent from L0-L5 risk tiers. Loss limits track realized PnL losses. Risk tiers track margin health. No interaction or escalation between the two systems.
- **D-09:** Dashboard visibility: banner/badge on existing Overview page showing "Daily loss: -$X / $Y limit" — turns red when approaching threshold. No dedicated safety page needed.

### General
- **D-10:** All new features must have configurable on/off switch (default OFF) + dashboard toggle, per project convention (`feedback_risk_configurable_switch.md`).

### Claude's Discretion
- Specific default threshold values for loss limits (daily and weekly amounts)
- TelegramNotifier refactoring approach — whether to add perp-perp methods to existing notifier or create a shared notification dispatcher
- Redis key design for rolling loss window tracking
- Implementation ordering of plans (Telegram first vs loss limits first)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Perp-Perp Engine
- `internal/engine/engine.go` — Main orchestration loop, trade execution, SL detection, funding tracker
- `internal/engine/exit.go` — Exit strategy, depth-fill, rotation, PnL reconciliation
- `internal/risk/health.go` — L0-L5 margin health tiers (loss limits must be independent)
- `internal/risk/monitor.go` — Active monitoring, 4-dimension checks

### Existing Notification System
- `internal/notify/telegram.go` — TelegramNotifier with send(), NotifyAutoEntry/Exit/EmergencyClose (spot-futures only)
- `internal/notify/telegram_test.go` — Existing test patterns

### Config & Dashboard
- `internal/config/config.go` — Config struct, JSON tags, defaults, apply, toJSON, fromEnv patterns (6 touch-points per field)
- `internal/api/handlers.go` — REST endpoint patterns, writeJSON helper
- `web/src/pages/Overview.tsx` — Where loss limit banner would be added
- `web/src/i18n/en.ts` / `web/src/i18n/zh-TW.ts` — Both locales must stay in sync

### Project Conventions
- `feedback_risk_configurable_switch.md` — Config on/off switch + dashboard toggle required for all risk features

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `TelegramNotifier` in `internal/notify/telegram.go` — Already has `send()`, `formatDuration()`, `formatExitReason()`. Nil-receiver safe. Only needs new methods for perp-perp events.
- Config already has `TELEGRAM_BOT_TOKEN` / `TELEGRAM_CHAT_ID` fields — no new config for Telegram connectivity.
- `internal/database/state.go` — Redis state patterns for position/history CRUD, reusable for loss tracking.

### Established Patterns
- Config field convention: 6 touch-points per field (struct, JSON, defaults, apply, toJSON, fromEnv)
- Dashboard toggle convention: ToggleSwitch + NumberField with opacity-50 conditional dimming (established in Phase 2)
- Risk feature pattern: `Enable{Feature}` bool + threshold values, all in Config struct

### Integration Points
- Perp-perp engine (`internal/engine/engine.go`) needs TelegramNotifier injected — currently no notifier field
- `cmd/main.go` wiring — TelegramNotifier is created there, currently only passed to SpotEngine
- Loss limit check would integrate at entry point in `executeArbitrage()` — pre-trade gate similar to risk manager checks
- Overview.tsx for loss limit banner display

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches.

</specifics>

<deferred>
## Deferred Ideas

- **Circuit breaker (PP-02)** — Per-exchange API health monitoring with auto-pause. Dropped because API failures already prevent entries naturally. Revisit if intermittent/flaky API issues become a real problem in production.
- **Telegram for non-critical events** — Entries, exits, rotations, rebalances, funding snapshots. Current scope is critical-only. Can be added later as configurable notification levels.

</deferred>

---

*Phase: 03-operational-safety*
*Context gathered: 2026-04-02*
