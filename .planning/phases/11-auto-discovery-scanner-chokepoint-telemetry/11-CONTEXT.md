# Phase 11: Auto-Discovery Scanner + Chokepoint + Telemetry - Context

**Gathered:** 2026-04-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Three coupled deliverables inside `internal/pricegaptrader/` (default OFF, no live-capital risk):

1. **Read-only auto-discovery scanner** (`scanner.go`) — polls bounded universe (≤20 symbols × 6 exchanges), applies persistence + freshness + depth + denylist gates, computes per-candidate score, writes cycle output to `pg:scan:*` Redis keys. Cannot mutate `cfg.PriceGapCandidates`.
2. **CandidateRegistry chokepoint** (`registry.go` — PG-DISC-04) — single mutex + atomic file write + timestamped `.bak` ring serializing all writers (dashboard handler, pg-admin, future scanner promotion in Phase 12).
3. **Discovery telemetry/dashboard** — sub-section inside the existing PriceGap tab showing cycle stats, per-candidate score history, why-rejected breakdown, and an empty placeholder card for the Phase 12 promote/demote timeline.

Phase 11 does NOT include: auto-promotion path (Phase 12), live capital, ramp controller, breaker, daily reconcile, or PG-OPS-09 top-level tab consolidation (Phase 16).

</domain>

<decisions>
## Implementation Decisions

### Score Formula
- **D-01:** Score shape is **gate-then-magnitude**: hard gates (≥4-bar persistence, BBO <90s on both legs, depth probe pass, not on denylist, cross-exchange pair exists) must ALL pass before any non-zero score is emitted; magnitude reflects the tradeable edge above gates.
- **D-02:** Score is an **integer 0–100** (stored as float in ZSET for ordering, conceptually integer for thresholds and display).
- **D-03:** **All sub-score components persisted** alongside the top-line score per cycle: `{spread_bps, persistence_bars, depth_score, freshness_age_s, funding_bps_h, gates_passed[], gates_failed[]}`. Enables dashboard sub-score breakdown and future PG-DISC-05 score-vs-realized calibration.
- **D-04:** Rejected candidates are **captured with `score=0` and a why-rejected reason code** in the cycle record; every (symbol × cross-pair) scanned writes a record. Drives the why-rejected dashboard breakdown directly from cycle data.

### Universe & Denylist
- **D-05:** Scan universe is **operator-curated** via a new `PriceGapDiscoveryUniverse` config field (`[]string` of canonical `BTCUSDT`-form symbols). No auto-derivation from history. ≤20 entries enforced at config load.
- **D-06:** Exchange pairing is **all-cross of universe × supported exchanges** — for each symbol, every (long_exch, short_exch) combination across the 6 supported exchanges is considered, skipping exchanges where the symbol isn't listed.
- **D-07:** Denylist lives in a dedicated `PriceGapDiscoveryDenylist` config field (`[]string` of canonical symbols, optionally `symbol@exchange` tuples). Denylist match → `score=0`, `why_rejected=denylist`. Editable via the same registry chokepoint as candidates.
- **D-08:** Symbols with only one exchange listing in the universe are **skipped silently** with a `symbol_no_cross_pair` counter incremented in `pg:scan:metrics`. No per-symbol record written for skipped singletons (counter-only observability).

### CandidateRegistry Chokepoint (PG-DISC-04)
- **D-09:** Registry exposes **typed methods**: `Add(ctx, candidate)`, `Update(ctx, idx, candidate)`, `Delete(ctx, idx)`, `Replace(ctx, []candidate)`. Each method does lock → reload from disk → mutate → atomic write → release. Idempotent dedupe by `(symbol, longExch, shortExch, direction)` tuple inside `Add` (mirrors v0.35.0 bidirectional field).
- **D-10:** Registry lives in **new `internal/pricegaptrader/registry.go`**. Holds a `*config.Config` reference. `cfg.mu` remains the authoritative lock; registry methods take that lock, reload from disk, mutate, call `cfg.SaveJSON()`, release. Module boundary preserved (no import of `internal/engine` or `internal/spotengine`).
- **D-11:** **Hard-cut migration** in this phase: dashboard `POST /api/config` candidate-mutation path AND `cmd/pg-admin` BOTH call registry methods directly. No legacy `cfg.PriceGapCandidates = append(...)` path remains anywhere. Integration test asserts concurrent dashboard+pg-admin writes produce zero lost mutations (roadmap success criterion #3).
- **D-12:** **Timestamped `.bak` ring** replaces single `.bak`: atomic write to `config.json.tmp` → `fsync` → `os.Rename` to `config.json` → write `config.json.bak.{unix_ts}` → prune ring to newest 5. Mirrors Pitfall 2's "ring of N `.bak.{ts}`" guidance.
- **D-13:** Scanner read-only enforced at **compile time** via `RegistryReader` interface (only `Get`/`List` methods). Scanner constructor accepts `RegistryReader`, never `*Registry`. Phase 12 swaps to full `*Registry` for the auto-promotion goroutine. Test asserts no mutation paths exist from scanner package.

### Discovery Telemetry & Dashboard
- **D-14:** Discovery section is a **sub-section inside the existing PriceGap dashboard tab** in Phase 11. Phase 16 (PG-OPS-09) will move it to the new Price-Gap top-level tab as part of the broader consolidation; placement migrates with PG-OPS-09, not before.
- **D-15:** Phase 11 Discovery section content:
  1. **Cycle stats card** (`last_run_at`, `candidates_seen`, `accepted`, `rejected`, `errors`, `next_run_in`).
  2. **Per-candidate score history chart** (Recharts line, 7d window from `pg:scan:scores:{symbol}` ZSET, threshold band overlay reads `PriceGapAutoPromoteScore` for context even though Phase 11 doesn't act on it).
  3. **Why-rejected breakdown** (table or pie aggregated over last N cycles by reason code).
  4. **Empty placeholder card** for Phase 12 promote/demote timeline — small empty-state card with "Promote/demote events will appear here once Phase 12 ships" text. Honors roadmap success criterion #2 verbatim while avoiding speculative timeline UI.
- **D-16:** Discovery section is **always visible**, with a "Scanner OFF" banner shown when `PriceGapDiscoveryEnabled=false`. Empty cycle stats render in disabled state. Operators can find the controls without first toggling config.
- **D-17:** Update freshness: **REST seed + WS throttled push**. Initial mount calls `GET /api/pg/discovery/state` for current cycle stats + score history snapshot; WS subscriptions on `pg:scan:cycle` (per-cycle), `pg:scan:metrics` (5s throttle), `pg:scan:score` (debounced) drive incremental updates. Reuses existing dashboard pattern (REST seed + WS hub batching).

### Claude's Discretion
- Magnitude weight specifics (spread vs depth vs funding contribution within the 0–100 magnitude post-gates).
- Gate evaluation order (short-circuit ordering for performance).
- Exact REST route paths for `/api/pg/discovery/*`.
- Specific Recharts component composition / styling.
- Concurrent-writer test harness shape (success criterion #3 integration test).
- Bootup behavior when scanner starts mid-cycle (skip / force-immediate / wait-for-next-tick).
- Scanner per-cycle error budget (consecutive ticker-read failures before circuit-break-into-cycle-metrics).
- Initial seed values for `PriceGapDiscoveryUniverse` and `PriceGapDiscoveryDenylist` (defaults to empty slices; operator populates post-deploy).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents (researcher, planner, executor, verifier) MUST read these before planning or implementing.**

### Roadmap & Requirements
- `.planning/ROADMAP.md` §"Phase 11: Auto-Discovery Scanner + Chokepoint + Telemetry" — phase goal + 5 success criteria
- `.planning/REQUIREMENTS.md` §"Auto-Discovery & Promotion" — PG-DISC-01, PG-DISC-03, PG-DISC-04 acceptance text
- `.planning/REQUIREMENTS.md` §"Constraints (locked at milestone start)" — module boundary, npm lockdown, default-OFF rule, hard ceiling
- `.planning/PROJECT.md` §"Current Milestone: v2.2 Auto-Discovery & Live Strategy 4" — milestone scope and Strategy 4 narrow-edge context

### v2.2 Research
- `.planning/research/SUMMARY.md` §"Auto-Discovery Scanner (PG-DISC-01)" / §"Discovery Telemetry (PG-DISC-03)" — feature categorization
- `.planning/research/ARCHITECTURE-v2.2.md` §"1. Where does the auto-discovery scanner live?" — module location rationale
- `.planning/research/ARCHITECTURE-v2.2.md` §"2. Concurrent CRUD safety for cfg.PriceGapCandidates" — chokepoint integration with `cfg.mu` / `SaveJSON` / `.bak`
- `.planning/research/ARCHITECTURE-v2.2.md` §"4. Telemetry data flow" — Redis key schema table (`pg:scan:*`, `pg:promote:*`)
- `.planning/research/ARCHITECTURE-v2.2.md` §"Concurrency Invariants to Preserve" + §"Anti-Patterns to Avoid"
- `.planning/research/PITFALLS.md` §"Pitfall 1: Auto-Discovery False-Positive Pump" — gate requirements (4-bar persistence, BBO <90s, depth, denylist, ÷8 funding normalization)
- `.planning/research/PITFALLS.md` §"Pitfall 2: Three-Writer Race on cfg.PriceGapCandidates" — chokepoint requirements (registry, atomic write, `.bak` ring, dedupe, audit trail)
- `.planning/research/FEATURES.md` §"Score decomposition" — sub-score visibility differentiator

### Existing Code Anchors (read before modifying)
- `internal/pricegaptrader/tracker.go` — `Tracker.Run()` errgroup, where the new scanner goroutine launches; existing tracker pattern to mirror
- `internal/pricegaptrader/risk_gate.go` — PG-RISK gates pattern for BBO freshness <90s (mirror target for scanner freshness gate)
- `internal/pricegaptrader/detector.go` + `detector_test.go` — `barRing.allExceed` pattern + 4-bar persistence invariant the scanner score must mirror
- `internal/pricegaptrader/metrics.go` — existing Redis writer pattern + WS hub envelope; `telemetry.go` (if added) follows same pattern
- `internal/config/config.go:70` — `sync.RWMutex mu` (the lock the registry takes)
- `internal/config/config.go:1572` — `SaveJSON()` canonical persist path
- `internal/config/config.go:1519` — `keepNonZero` tripwire + absolute-path fallback (v2.1 PG-OPS-08)
- `internal/api/handlers.go` — `POST /api/config` handler (one of the writers being migrated to the chokepoint)
- `cmd/pg-admin/main.go` — pg-admin CLI command surface (second writer being migrated to the chokepoint)
- `web/` — existing PriceGap tab structure and existing Recharts components for the score-history chart pattern

### Cross-Project History
- `.planning/RETROSPECTIVE.md` — "1-bar close-to-close spread crossings inflate edge estimates ~10×" (motivates 4-bar persistence gate)
- v2.1 Phase 999.1 commits — bidirectional `direction` field added to `PriceGapCandidate` (dedupe tuple includes this)
- v2.1 Phase 10 — `pg:positions:active` active-position guard pattern (used in Phase 12 demote, not Phase 11 itself, but the guard surface is referenced from `RegistryReader.List` consumers)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/pricegaptrader/tracker.go` `Tracker.Run()` errgroup — scanner goroutine slots in here; no new top-level startup component needed (preserves graceful-shutdown reverse-order logic).
- `internal/pricegaptrader/risk_gate.go` BBO freshness logic — scanner reuses or mirrors the same `<90s` threshold helper.
- `internal/pricegaptrader/detector.go` `barRing.allExceed` pattern — scanner persistence gate uses the same mechanism (4-bar same-sign cross).
- `internal/pricegaptrader/metrics.go` Redis writer + WS publish pattern — `telemetry.go` (if added as a separate file) follows the same envelope.
- `*config.Config` `sync.RWMutex mu` (`config.go:70`) — registry takes this lock; no new mutex required.
- `cfg.SaveJSON()` (`config.go:1572`) — registry's atomic-write path; already includes single `.bak` write today (D-12 extends to a timestamped ring).
- Existing Recharts components in `web/` — score-history chart reuses the chart wrapper pattern from the analytics page.
- WS hub `BroadcastEvent` envelope `{type, ts, payload}` — telemetry pushes use this directly.

### Established Patterns
- Module boundary: `internal/pricegaptrader/` does NOT import `internal/engine` or `internal/spotengine`. All new files in this phase live inside this boundary.
- Default-OFF feature gating: new features require config bool (default false) + dashboard toggle + persisted via `SaveJSON`.
- Single config save path: `cfg.SaveJSON()` is the only writer to `config.json`. Registry preserves this — no `os.WriteFile` shortcuts.
- REST seed + WS push: dashboard initial-mount via REST, incremental updates via WS subscriptions.
- Internal symbol form `BTCUSDT` (uppercase, no separators); per-exchange mapping is inside each adapter — scanner enforces canonical form once at ingest.
- Loris funding rates are 8h-equivalent — divide by 8 before any bps/h comparison in score formula.
- Bybit `:04–:05:30` blackout — scanner skips ticker reads during this window when `cfg.BybitBlackoutEnabled`.

### Integration Points
- `cmd/arb/main.go` (or `internal/app`) bootstrap — passes scanner cadence + universe + denylist config into the Tracker constructor. No startup-order shift.
- `internal/api/handlers.go` `POST /api/config` handler — call site #1 migrating to registry methods (currently mutates `cfg.PriceGapCandidates` directly via `SaveJSON`).
- `cmd/pg-admin/main.go` — call site #2 migrating to registry methods. Whether pg-admin links the tracker in-process or RPCs to the running daemon is open (TBD in research / planning; currently pg-admin imports config directly).
- `web/src/.../PriceGap.tsx` (or equivalent) — host of the new Discovery sub-section. Layout placement migrates to Phase 16's PG-OPS-09 top-level tab.
- `internal/api/` adds new GET routes: `GET /api/pg/discovery/state`, `GET /api/pg/discovery/scores/{symbol}` (or a single state endpoint with embedded score history — exact split is Claude's discretion).

</code_context>

<specifics>
## Specific Ideas

- "Gate-then-magnitude" score shape is the explicit antidote to Pitfall 1's false-positive pump scenario; this is non-negotiable.
- Phase 11 must merge the chokepoint BEFORE Phase 12 starts — Phase 12 cannot ship without the registry already in place.
- Compile-time read-only enforcement via `RegistryReader` is preferred over runtime checks because it eliminates a class of bugs entirely (scanner package literally cannot mutate).
- The empty placeholder card for the Phase 12 timeline is a deliberate scaffold, not throwaway UI — Phase 12 swaps the empty-state for a populated component without restructuring the section.
- Migration to the registry is hard-cut for both writers in the same phase to avoid carrying a two-writer race window into the paper-mode period.

</specifics>

<deferred>
## Deferred Ideas

- **PG-OPS-09 top-level Price-Gap tab** — Phase 16 scope; Phase 11 places the Discovery section under the existing PriceGap tab and migrates to the new top-level tab when PG-OPS-09 lands.
- **Auto-promotion path (PG-DISC-02)** — Phase 12 scope; Phase 11 ships the registry but the scanner cannot call mutator methods (compile-time blocked via `RegistryReader`).
- **Score-vs-realized-fill calibration view (PG-DISC-05)** — deferred to v2.3 per `REQUIREMENTS.md`. Phase 11 sub-score persistence enables this future view.
- **ML-based scoring** — anti-feature per `FEATURES.md`; not considered.
- **Per-tick scanner scoring** — anti-feature per `FEATURES.md` and `REQUIREMENTS.md` "Out of Scope"; cycle-driven only.
- **Auto-tune `PriceGapAutoPromoteScore` from PnL** — out of scope per `REQUIREMENTS.md` (sample size too small).
- **Universe auto-derivation from `pg:history`** — explicitly rejected in D-05; operator-curated only.
- **Continuous-curve ramp / promotion** — anti-feature per `REQUIREMENTS.md`; discrete only.

</deferred>

---

*Phase: 11-auto-discovery-scanner-chokepoint-telemetry*
*Context gathered: 2026-04-28*
