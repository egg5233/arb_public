# Phase 12: Auto-Promotion - Context

**Gathered:** 2026-04-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Auto-promotion controller closes the loop between the Phase 11 read-only scanner and the Phase 11 chokepoint. The controller consumes scanner cycle output, tracks per-candidate streaks, and calls `Registry.Add` / `Registry.Delete` to mutate `cfg.PriceGapCandidates` automatically — with the same safety guards that operator dashboard CRUD and `pg-admin` go through. Both promote and demote events are surfaced on Telegram and the WS hub, and persisted to a bounded Redis timeline that fills the empty placeholder card Phase 11 left in the Discovery dashboard section.

What is **out of scope** for this phase: scanner score formula changes, registry chokepoint changes, dashboard Discovery section restructure (Phase 16 PG-OPS-09), live capital ramp (Phase 14), drawdown breaker (Phase 15), per-fill Telegram alerts (PG-LIVE-04 deferred to v2.3+).

</domain>

<decisions>
## Implementation Decisions

### Streak Tracking
- **D-01:** Per-candidate **counter** semantics: increment by 1 when the cycle's `CycleRecord.Score` for that `(symbol, longExch, shortExch, direction)` tuple is `≥ cfg.PriceGapAutoPromoteScore`; reset to 0 when the score is below threshold or when the candidate is missing from the cycle's accepted `Records`. Promote when counter `≥ 6`. Closest reading of "≥6 consecutive cycles" in PG-DISC-02; testable with a single int per tuple.
- **D-02:** **Strict consecutive semantics** — any non-accepted cycle (rejected for `stale_bbo`, `insufficient_persistence`, `insufficient_depth`, etc., or absent from the universe row) resets the counter. Aligns with Pitfall 1: fragile candidates should not promote.
- **D-03:** **Streak storage is in-memory** (`map[candidateKey]streakState` inside the controller). Cold restart requires 6 fresh cycles before any promote can fire — acceptable because the controller does not persist between daemon restarts and a clean baseline after restart matches operator expectations. No new Redis key for streak state.

### Auto-Demote Policy
- **D-04:** **Symmetric streak demote**: a promoted candidate is auto-demoted when its `CycleRecord.Score` is `< cfg.PriceGapAutoPromoteScore` (or absent from accepted `Records`) for 6 consecutive cycles. Mirrors the promote logic — no new threshold to tune.
- **D-05:** **Active-position guard always blocks demote**. Before calling `Registry.Delete(...)`, controller checks `pg:positions:active` SET membership for any active position whose candidate tuple matches; if matched, the demote is **skipped silently and the demote-streak counter is held** (not reset) so the demote fires on the next cycle once the position closes. No Telegram on blocked demote — operator already sees the position via the existing dashboard PG positions panel.
- **D-06:** Auto-demote is mandatory in this phase — manual-only demote is rejected because it leaves stale promoted candidates after universe shifts and contradicts the lifecycle goal in PG-DISC-02.

### Master Toggle
- **D-07:** **Single switch**: reuse `cfg.PriceGapDiscoveryEnabled` for both scanner and promotion controller. When `=true`, scanner runs AND controller runs. When `=false`, neither runs. No new config field. Phase 12 is the natural completion of what discovery was always meant to do.

### Cap-Full Behavior
- **D-08:** When the candidate count after the proposed Add would exceed `cfg.PriceGapMaxCandidates` (default 12), the promotion is **skipped silently**. Controller increments a new `cap_full_skips` HASH field on `pg:scan:metrics`. **No Telegram, no event in `pg:promote:events`**. Operator sees the skip in dashboard cycle-stats counters; no alert fatigue when the cap is sustained-full. The skipped candidate's streak counter is **held at 6** (not reset) so it auto-promotes the moment a slot frees.
- **D-09:** No auto-displacement of lower-scored promoted candidates to make room. Displacement adds a "why did my candidate disappear?" question that operators should never have to answer.

### Promote/Demote Event Surfacing
- **D-10:** **Redis schema** — single `pg:promote:events` LIST, RPush + LTrim 1000. Mirrors the `pg:scan:cycles` pattern from Phase 11 `internal/pricegaptrader/telemetry.go`. Each entry is a JSON-encoded event with: `ts` (unix ms), `action` (`"promote"` | `"demote"`), `symbol`, `long_exch`, `short_exch`, `direction`, `score` (the score that triggered the action — last cycle's score), `streak_cycles` (how many cycles the streak ran), and `reason` (`"score_threshold_met"` | `"score_below_threshold"`).
- **D-11:** **WS event** — broadcast on the existing `s.hub.Broadcast(eventType, payload)` pattern (`internal/api/pricegap_handlers.go:501`). Event name: `pg_promote_event`. Payload identical to the JSON pushed onto `pg:promote:events`. Dashboard subscribes and renders the Phase 11 placeholder card as a populated timeline (newest first).
- **D-12:** **REST seed endpoint**: `GET /api/pg/discovery/promote-events` returns the LIST contents (LRANGE 0 -1) for initial mount; the existing Discovery state endpoint added in Phase 11 may absorb this — exact route shape is Claude's discretion at planning time, as long as REST seed + WS push pattern is honored.

### Telegram Alerts
- **D-13:** **Per-event Telegram with unique cooldown key**: one Telegram per promote and per demote. Cooldown key = `"pg_promote:" + action + ":" + symbol + ":" + longExch + ":" + shortExch + ":" + direction`. Distinct events get distinct keys, so the existing 5-min `checkCooldownAt()` in `internal/notify/telegram.go:206` does not suppress legitimate sequential events for different candidates. A flap on the same candidate within 5 minutes (promote→demote→promote) is intentionally throttled because it indicates an unstable situation operators should be alerted to via the dashboard event timeline, not duplicate Telegram messages.
- **D-14:** Telegram message format: `"[PG promote] {symbol} {long_exch}↔{short_exch} ({direction}) score={score} streak={streak_cycles}"` (and matching `[PG demote]`). Use the existing `(*TelegramNotifier).Send(format, args...)` raw path — no new `NotifyPromote` method; the existing `priceGapGateAllowlist` and structured methods are for trade-side alerts.

### Controller Wiring
- **D-15:** Controller lives in **new `internal/pricegaptrader/promotion.go`** (with `promotion_test.go`). Module boundary preserved: imports `internal/pricegaptrader/registry.go`, `internal/database` for the active-position guard read, `internal/notify` interface for Telegram, `internal/api` hub for WS — through small interfaces declared in `promotion.go`, not concrete types.
- **D-16:** **Synchronous controller call at end of `Scanner.RunCycle()`**: scanner finishes computing `CycleSummary`, then calls `controller.Apply(ctx, summary)` before returning. Single goroutine, no channel plumbing, no race with another scanner cycle starting. Controller does NOT spawn its own goroutine.
- **D-17:** **Scanner constructor swap**: scanner takes `*Registry` (full mutator type) instead of `RegistryReader` starting in Phase 12. The `RegistryReader` interface stays in the codebase — it remains the type used by any future read-only consumers (e.g., Phase 16 dashboard render path) — but the scanner no longer needs the compile-time block now that the controller is the legitimate writer in the same package. Phase 11's `scanner_static_test.go` no-mutation assertion is updated to verify scanner-the-package only mutates through `Registry.Add`/`Delete`, not direct `cfg.PriceGapCandidates` field access.

### Direction Policy
- **D-18:** **Direction is pinned to `"bidirectional"`** for every `Registry.Add` / `Registry.Delete` call the controller makes. Today's scanner is bidirectional-only (`internal/pricegaptrader/scanner.go:162` uses `models.PriceGapDirectionBidirectional`); `CycleRecord` does **not** gain a `Direction` field — Phase 11's `CycleRecord` contract is preserved. The 4-tuple `candidateKey` (D-01) remains `(symbol, longExch, shortExch, "bidirectional")` for streak counters, the Telegram cooldown key (D-13), `pg:promote:events` payload (D-10), and the `pg_promote_event` WS broadcast (D-11). Concrete consequences for the planning agent: 12-01 must NOT read `rec.Direction` or `c.Direction` (those reads were the audit's contract drift) — pass the string literal `"bidirectional"` at Registry.Add call sites; 12-02 keeps `direction` in the cooldown key + Telegram message format but with a constant value; 12-04 may render the direction badge as static metadata rather than per-row variability. If a future phase introduces `long_only` / `short_only` universes, that's a v2.3+ phase that adds a successor decision and a CycleRecord schema bump — not a Claude's-discretion item here. Settled 2026-04-29 in a gap session after the Wave 2/3 audit identified Direction drift across 12-01..12-04.

### Claude's Discretion
- Exact field names + JSON encoding of `pg:promote:events` entries (the schema list above is the contract; nullable/optional fields and field ordering are Claude's call).
- Exact REST route under `/api/pg/discovery/*` for the events seed (single combined state endpoint vs separate `/promote-events` route).
- Internal struct names and signatures of the controller's interface dependencies (e.g., `type ActivePositionChecker interface { IsActiveForCandidate(...) bool }`).
- Test fixtures for streak transitions (counter shape, miss-then-resume, demote-blocked-by-guard).
- Telegram message wording polish (so long as the four required fields — action, symbol, exchanges, direction, score, streak — are present).
- Whether `cap_full_skips` is a single counter or per-symbol (`cap_full_skip:{symbol}` or HASH field) — operator-visibility tradeoff.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Roadmap & Requirements
- `.planning/ROADMAP.md` §"Phase 12: Auto-Promotion" — phase goal + 5 success criteria
- `.planning/milestones/v2.2-REQUIREMENTS.md` §"Auto-Discovery & Promotion" §PG-DISC-02 — controller contract verbatim
- `.planning/milestones/v2.2-REQUIREMENTS.md` §"Out of Scope" — boundaries (no auto-tune of threshold, no continuous-curve ramp, no per-tick scoring)

### Phase 11 (Direct Dependency)
- `.planning/phases/11-auto-discovery-scanner-chokepoint-telemetry/11-CONTEXT.md` §"CandidateRegistry Chokepoint (PG-DISC-04)" D-09..D-13 — registry contract + RegistryReader rationale
- `.planning/phases/11-auto-discovery-scanner-chokepoint-telemetry/11-CONTEXT.md` §"Discovery Telemetry & Dashboard" D-15 — empty placeholder card description that Phase 12 fills
- `.planning/phases/11-auto-discovery-scanner-chokepoint-telemetry/11-CONTEXT.md` §"Deferred Ideas" — confirms Phase 12 scope inheritance

### v2.2 Research
- `.planning/research/ARCHITECTURE-v2.2.md` §"2. Concurrent CRUD safety for cfg.PriceGapCandidates" — chokepoint integration with `cfg.mu` / `SaveJSON` / `.bak` (controller call site is the third writer)
- `.planning/research/ARCHITECTURE-v2.2.md` §"4. Telemetry data flow" — `pg:scan:*` and `pg:promote:*` schema rationale
- `.planning/research/ARCHITECTURE-v2.2.md` §"Concurrency Invariants to Preserve" — synchronous-call rationale (D-16)
- `.planning/research/PITFALLS.md` §"Pitfall 1: Auto-Discovery False-Positive Pump" — strict-consecutive rationale (D-02) + active-position guard rationale (D-05)
- `.planning/research/PITFALLS.md` §"Pitfall 2: Three-Writer Race on cfg.PriceGapCandidates" — mandates the controller go through `Registry.Add/Delete`, never `cfg.PriceGapCandidates = append(...)` (D-15)

### Existing Code Anchors (read before modifying)
- `internal/pricegaptrader/registry.go` — `Registry.Add(ctx, source, c)` line 117, `Delete(ctx, source, idx)` line 191. Source string `"scanner-promote"` / `"scanner-demote"` is the audit trail tag.
- `internal/pricegaptrader/registry_reader.go` — `RegistryReader` interface still exists; scanner constructor swap (D-17) takes `*Registry` instead but the interface remains for read-only consumers.
- `internal/pricegaptrader/scanner.go` — `Scanner.RunCycle(ctx, now)` line 236; `CycleSummary` line 120 (controller input); `CycleRecord` line 89 with `Score` int 0–100 + `Symbol`/`LongExch`/`ShortExch` fields.
- `internal/pricegaptrader/telemetry.go` — `pg:scan:cycles` LIST + LTrim 1000 (line 9 comment) is the pattern Phase 12 mirrors for `pg:promote:events`. Also `pg:scan:metrics` HASH (line 11 comment) — `cap_full_skips` lives here.
- `internal/database/pricegap_state.go:20` — `keyPricegapActive = "pg:positions:active"` SET. Active-position guard reads SMEMBERS from this key.
- `internal/notify/telegram.go:206` — `(*TelegramNotifier).checkCooldownAt(eventKey, now)` 5-min gate that the unique-key strategy in D-13 leverages.
- `internal/api/pricegap_handlers.go:501` — `s.hub.Broadcast(eventType, payload)` pattern Phase 12 uses for `pg_promote_event`.
- `internal/api/ws.go:42` — `Hub` type definition (no constructor changes needed).
- `internal/config/config.go` — `PriceGapAutoPromoteScore` validated `[50,100]` (line 54), `PriceGapMaxCandidates` validated `[1,50]` default 12 (line 57), `PriceGapDiscoveryEnabled` line 357. All three exist; Phase 12 reads them, does not add new fields.

### Cross-Project History
- `git log --oneline internal/pricegaptrader/` — Phase 8 (Plans 01–08), Phase 9 (Plans 01–11), Phase 10 (Plans 01–05), Phase 999.1 (`direction` field), Phase 11 (Plans 01–06) — full history of how the package evolved into the form Phase 12 builds on.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`Registry.Add` / `Delete` chokepoint** (Phase 11 Plan 02): the controller never has to think about `cfg.mu`, atomic write, `.bak` ring, or dedupe — they are inside the registry methods. Controller just calls `registry.Add(ctx, "scanner-promote", c)`.
- **`pg:positions:active` SET** (Phase 8): existing Redis SET; `SMEMBERS` returns active position IDs. Match by reading the position via existing `database` getters and comparing the candidate tuple. Reused without change.
- **`Telemetry.WriteCycle` LIST + LTrim pattern** (Phase 11 Plan 05): direct template for `pg:promote:events` writer.
- **`(*TelegramNotifier).Send` + `checkCooldownAt`** (Phase 3, evolved Phase 6): per-event-key cooldown is exactly the deduplication primitive D-13 needs; no new notifier code required, just a unique key construction.
- **`s.hub.Broadcast(eventType, payload)`** (Phase 9): existing WS broadcast hub. New `pg_promote_event` event name slots in.
- **`scanner_static_test.go`** (Phase 11 Plan 04): existing static test asserting scanner has no `cfg.PriceGapCandidates` mutations — Phase 12 updates the assertion list to permit the new `Registry.Add/Delete` path while still rejecting raw field mutation.

### Established Patterns
- **Phase 11 D-09 chokepoint discipline**: every writer uses Registry methods, never raw `cfg.PriceGapCandidates = append(...)`. Phase 12 controller is the third writer (after dashboard and pg-admin) and follows the same rule.
- **Phase 11 D-13 compile-time enforcement**: Phase 12 D-17 explicitly relaxes this for the scanner constructor inside the same package — but the relaxation is bounded: only the scanner package sees `*Registry`; the read-only `RegistryReader` interface remains for external consumers. Document this in the new `promotion.go` to prevent confusion in future phases.
- **Default-OFF master switch** (project-wide convention from CLAUDE.local.md "New feature rollout pattern"): satisfied by reusing `PriceGapDiscoveryEnabled` (D-07).
- **Synchronous in-cycle work + async I/O downstream** (Phase 11 Plan 04): the scanner already runs gates + scoring synchronously inside `RunCycle`; Phase 12 controller call slots in at the end of the same goroutine.

### Integration Points
- `internal/pricegaptrader/scanner.go` — at the end of `Scanner.RunCycle()`, after `telemetry.WriteCycle(...)` succeeds, call `controller.Apply(ctx, summary)`. Constructor signature (Plan 04 / 05) takes `Telemetry` already; add `*PromotionController` (or merge into a single sink interface — Claude's discretion at planning).
- `internal/pricegaptrader/promotion.go` — new file. Holds streak counters, references to `*Registry`, the active-position guard read interface, the `PriceGapNotifier` interface (or `Send`-style notifier interface), and the WS broadcast interface.
- `cmd/arb/main.go` (or `internal/app` bootstrap) — wire the new `PromotionController` and pass it into the scanner constructor. No startup-order shift needed.
- `internal/api/pricegap_handlers.go` — add the optional REST seed endpoint for `pg:promote:events` (route name is Claude's discretion per D-12).
- `web/src/.../PriceGap.tsx` discovery section — replace the empty Phase 11 placeholder card with a populated timeline component subscribing to `pg_promote_event` WS events. EN + zh-TW i18n keys must stay in lockstep per CLAUDE.local.md i18n rule.
- `internal/pricegaptrader/scanner_static_test.go` — update the static-mutation assertion to permit `Registry.Add` / `Delete` calls from inside the package while still rejecting raw `cfg.PriceGapCandidates` mutation.

</code_context>

<specifics>
## Specific Ideas

- **Streak storage is intentionally in-memory** (D-03): a fresh 6-cycle baseline after restart is a feature, not a bug — it forces the operator to confirm a candidate is still strong before the controller writes config. No Redis key for streak state.
- **The cap-full counter `cap_full_skips`** (D-08) is the only signal that a sustained "would-promote-but-can't" condition exists; operators check the dashboard cycle stats card to see if the cap is too tight, then either raise `PriceGapMaxCandidates` or manually free a slot.
- **The scanner-package compile-time block relaxation** (D-17) is the one structural change Phase 12 makes to Phase 11's discipline. Document this clearly in `promotion.go` so future phases don't read `RegistryReader` and conclude scanner is still read-only.
- **Per-event Telegram cooldown key** (D-13) is the single most important auditability decision: it lets distinct candidate events through within 5 minutes (multiple discoveries in a single cycle) while still suppressing same-candidate flap.
- **Demote-streak hold when guard blocks** (D-05) means a candidate marked for demote stays in the demote queue across cycles until the active position closes — no event fires until the actual demote succeeds, so the operator sees one clean demote event per lifecycle, not a stuck "wanted to demote" message.

</specifics>

<deferred>
## Deferred Ideas

- **Mixed-direction universes (per-symbol `long_only` / `short_only`)** — pinned to `"bidirectional"` in D-18 because today's scanner is bidirectional-only. Revisit when scanner gains directional-mode-per-symbol; that successor phase adds a `CycleRecord.Direction` field and replaces D-18.
- **Auto-displace lowest-scored promoted candidate when cap full** — explicitly rejected (D-09); creates "why did my candidate disappear?" confusion. Revisit only if cap-full is sustained at scale.
- **Hysteresis demote threshold** (lower demote threshold than promote) — rejected in D-04 in favor of symmetric streak; revisit if real-world flap proves to be a problem.
- **Manual-only demote mode** — rejected in D-06; would leave stale promoted candidates after universe shifts.
- **Per-cycle Telegram digest** — rejected in D-13; loses event-level granularity for what PG-DISC-02 calls "critical" alerts.
- **Separate `PriceGapAutoPromoteEnabled` toggle** — rejected in D-07 in favor of single switch; revisit only if the calibration phase needs scanner read-only without write risk.
- **Redis-backed streak storage for warm restart** — rejected in D-03; in-memory is simpler and the 6-cycle restart cost is acceptable. Revisit only if daemon restarts become frequent during live operation.
- **Score-vs-realized-fill calibration view (PG-DISC-05)** — already deferred in Phase 11 to v2.3+; Phase 12's `pg:promote:events` may feed that view but no new code in this phase.
- **PG-LIVE-04 Telegram per-fill alerts** — explicitly deferred to v2.3+ per `REQUIREMENTS.md` v2.3+ section; not Phase 12 scope.
- **Auto-tune `PriceGapAutoPromoteScore` from PnL** — anti-feature per `REQUIREMENTS.md` Out of Scope; manual tuning only.
- **HASH per-day event store with EXPIRE** — rejected in D-10 in favor of LIST + LTrim 1000; if 30d aggregate stats are needed later, a separate aggregator can read from the LIST.

</deferred>

---

*Phase: 12-auto-promotion*
*Context gathered: 2026-04-29*
