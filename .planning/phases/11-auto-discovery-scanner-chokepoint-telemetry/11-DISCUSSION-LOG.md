# Phase 11: Auto-Discovery Scanner + Chokepoint + Telemetry - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-28
**Phase:** 11-auto-discovery-scanner-chokepoint-telemetry
**Mode:** discuss (interactive)
**Areas discussed:** Score formula shape, Universe & denylist, Chokepoint API shape, Telemetry dashboard placement

---

## Score Formula Shape

### Q: What's the overall shape of the score formula?

| Option | Description | Selected |
|--------|-------------|----------|
| Gate-then-magnitude | Hard gates first (4-bar persistence, BBO <90s, depth probe, denylist) — must ALL pass or score is 0. Then magnitude reflects the tradeable edge above gates. Mirrors Pitfall 1 guidance and Phase 8's `barRing` pattern. | ✓ |
| Weighted sum of normalized features | Each component normalized to 0-1, multiplied by tunable weights, summed. | |
| Hybrid: hard gates + weighted magnitude | Gates produce binary pass/fail; within-gate magnitude is a weighted sum. | |

**User's choice:** Gate-then-magnitude

### Q: What's the score numeric range / type?

| Option | Description | Selected |
|--------|-------------|----------|
| Integer 0-100 | Easy to threshold/display/compare. Stored as float in ZSET for ordering. | ✓ |
| Float 0.0-1.0 | Standard normalized. | |
| Raw bps/h with cap | Score IS the realized edge in bps/h, capped at e.g. 50bps. | |

**User's choice:** Integer 0-100

### Q: Sub-score components stored separately?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — store all components | Persist `{spread_bps, persistence_bars, depth_score, freshness_age_s, funding_bps_h, gates_passed[]}` alongside top-line. | ✓ |
| No — just top-line + reason code | Only final score and a why-rejected enum. | |
| Hybrid: top-line + reason + spread/depth only | Compromise. | |

**User's choice:** Yes — store all components

### Q: When a candidate fails gates, capture or skip?

| Option | Description | Selected |
|--------|-------------|----------|
| Capture with score=0 + reason | Every (symbol × cross-pair) scanned writes a record. Drives why-rejected directly from cycle data. | ✓ |
| Capture rejected as aggregate counts | Less detail, smaller footprint. | |
| Skip rejected entirely | Strongest cost savings, weakest observability. | |

**User's choice:** Capture with score=0 + reason

---

## Universe & Denylist

### Q: How is the ≤20-symbol scan universe seeded?

| Option | Description | Selected |
|--------|-------------|----------|
| Operator-curated config list | New `PriceGapDiscoveryUniverse` `[]string`. Operator owns the list. | ✓ |
| Auto-derive from existing pricegap history | Scanner reads `pg:history`, picks top-N. Self-curating but feedback loop. | |
| Hybrid: operator seed + auto-add from history | | |

**User's choice:** Operator-curated config list

### Q: How are exchange pairs generated within the universe?

| Option | Description | Selected |
|--------|-------------|----------|
| All-cross of universe + supported exchanges | For each symbol, all `(long_exch, short_exch)` combos across listed exchanges. | ✓ |
| Operator-specified per symbol | Universe entries are full tuples. | |
| Mirror existing `PriceGapCandidates` exchange set | Implicit pairing. | |

**User's choice:** All-cross of universe + supported exchanges

### Q: Where does the denylist of known-noisy pairs live?

| Option | Description | Selected |
|--------|-------------|----------|
| Dedicated `PriceGapDiscoveryDenylist` config field | New `[]string`. `score=0`, `why_rejected=denylist` on match. | ✓ |
| Per-symbol flag on existing `PriceGapCandidate` | Conflates denylisting with being a candidate. | |
| Reuse existing SOON-listing detection | Auto-deny on pre-listing flag, no operator-curated list. | |

**User's choice:** Dedicated `PriceGapDiscoveryDenylist` config field

### Q: Symbols with only one exchange listing in the universe?

| Option | Description | Selected |
|--------|-------------|----------|
| Skip silently with cycle metric | Counter-only; self-correcting via metric review. | ✓ |
| `score=0` with reason `no_cross_pair` | More observable but inflates breakdown. | |
| Reject at config validation | Strict but breaks scanner on delistings. | |

**User's choice:** Skip silently with cycle metric

---

## Chokepoint API Shape

### Q: What's the surface of the new `CandidateRegistry`?

| Option | Description | Selected |
|--------|-------------|----------|
| Typed methods: `Add` / `Update` / `Delete` / `Replace` | Each method does lock → reload → mutate → atomic write → release. Idempotent dedupe inside `Add`. | ✓ |
| Generic `Mutate(ctx, fn)` callback | Single mutator method; flexible but harder to audit at call sites. | |
| Both: typed methods backed by `Mutate` | More API surface, more flexibility. | |

**User's choice:** Typed methods: `Add` / `Update` / `Delete` / `Replace`

### Q: Where does the registry live, and how does it own `*config.Config`?

| Option | Description | Selected |
|--------|-------------|----------|
| New `internal/pricegaptrader/registry.go`, holds `*config.Config` | Same module as scanner/promotion. `cfg.mu` authoritative. | ✓ |
| New `internal/config/registry.go` | Expands the config package's API surface; risks future writers bypassing it. | |
| Embed inside existing pricegaptrader Tracker | Conflates Tracker (lifecycle/state) with config-mutation chokepoint. | |

**User's choice:** New `internal/pricegaptrader/registry.go`, holds `*config.Config`

### Q: How do existing call sites (dashboard handler + pg-admin) migrate?

| Option | Description | Selected |
|--------|-------------|----------|
| Hard-cut both in this phase | Dashboard handler AND `cmd/pg-admin` both call registry directly. No legacy path. Integration test required. | ✓ |
| Wrap legacy path with adapter | Two paths exist temporarily, weakening the chokepoint guarantee. | |
| Migrate dashboard now, defer pg-admin to Phase 12 | Two-writer race window during paper-mode. | |

**User's choice:** Hard-cut both in this phase

### Q: Backup ring layout when the registry persists to `config.json`?

| Option | Description | Selected |
|--------|-------------|----------|
| Timestamped ring `.bak.{ts}` keep last N | Atomic write to `.tmp` → fsync → rename → prune to newest 5. Pitfall 2 explicitly calls out single `.bak` as data-loss risk. | ✓ |
| Append-only audit log + single `.bak` | Pitfall 2's data-loss concern shifts to Redis TTL handling. | |
| Both: timestamped ring AND Redis audit | Belt-and-suspenders. | |

**User's choice:** Timestamped ring `.bak.{ts}` keep last N

### Q: How is the scanner read-only enforced (Phase 11) until Phase 12 ships?

| Option | Description | Selected |
|--------|-------------|----------|
| Scanner takes a `*RegistryReader` (no write methods) | Compile-time enforcement. Phase 12 swaps to full `*Registry`. | ✓ |
| Runtime gate inside Registry methods | Scanner can still call mutators; weaker than compile-time. | |
| Comment + lint rule + test only | No structural enforcement. | |

**User's choice:** Scanner takes a `*RegistryReader` (no write methods)

---

## Telemetry Dashboard Placement

### Q: Where does the Phase 11 Discovery section live in the dashboard?

| Option | Description | Selected |
|--------|-------------|----------|
| Sub-section inside existing PriceGap tab | Phase 16 (PG-OPS-09) moves it to the new top-level tab. Minimal frontend churn. | ✓ |
| New top-level "Discovery" tab now, fold into Price-Gap in Phase 16 | Throwaway navigation. | |
| Add Phase 16's new Price-Gap top-level tab now (out-of-order PG-OPS-09) | Crosses scope boundary; Phase 16 also touches paper-mode toggle, ramp tier, breaker threshold. | |

**User's choice:** Sub-section inside existing PriceGap tab

### Q: What does the Discovery section show in Phase 11? *(multiSelect)*

| Option | Description | Selected |
|--------|-------------|----------|
| Cycle stats card | `last_run_at`, `candidates_seen`, `accepted`, `rejected`, `errors`. Reads `pg:scan:metrics`, WS at 5s throttle. | ✓ |
| Per-candidate score history chart | Recharts line chart over 7d ZSET window with threshold band overlay. | ✓ |
| Why-rejected breakdown table/pie | Aggregated reasons over last N cycles. Drives operator action (denylist edits, universe pruning). | ✓ |
| Empty promote/demote timeline | Placeholder timeline component; Phase 12 fills. (Initially excluded; reconsidered in follow-up below.) | (later confirmed) |

**User's choice:** First three items; the empty timeline was reconsidered in a follow-up question.

### Q (follow-up): How should we handle the empty promote/demote timeline that ROADMAP success criterion #2 calls for?

| Option | Description | Selected |
|--------|-------------|----------|
| Drop it — update success criteria, defer timeline UI to Phase 12 | Cleaner; means a roadmap-criterion edit. | |
| Keep the placeholder — small empty-state card | Tiny card honors criterion verbatim, ~30 lines of frontend. | ✓ |
| Build the timeline component now, leave data path empty | Most code now, smoothest Phase 12 handoff. | |

**User's choice:** Keep the placeholder — small empty-state card

### Q: Should the Discovery section be visible when `PriceGapDiscoveryEnabled=false`?

| Option | Description | Selected |
|--------|-------------|----------|
| Visible but with "Scanner OFF" banner | Always renders; banner with link to enable. Empty cycle stats render in disabled state. | ✓ |
| Hidden until scanner enabled | Conditional render; hard to find. | |
| Visible with banner + score-history archive on | Last-known cycle data remains visible even with scanner OFF. | |

**User's choice:** Visible but with "Scanner OFF" banner

### Q: How fresh should the Discovery section update?

| Option | Description | Selected |
|--------|-------------|----------|
| REST seed + WS throttled push | Initial mount via REST, incremental via WS subscriptions on `pg:scan:cycle` / `pg:scan:metrics` / `pg:scan:score`. Matches existing dashboard pattern. | ✓ |
| Polling every 30s | Lower implementation cost but stale during cycle windows. | |
| WS only, no REST seed | Empty until first push arrives. Worst UX on first load. | |

**User's choice:** REST seed + WS throttled push

---

## Claude's Discretion

Areas left to planning / execution:

- Magnitude weight specifics within the 0–100 score (gate-then-magnitude shape locked, weights flexible).
- Gate evaluation order (short-circuit ordering for performance).
- Exact REST route paths under `/api/pg/discovery/*`.
- Recharts component composition / styling for the score-history chart.
- Concurrent-writer integration test fixture shape (success criterion #3).
- Bootup behavior when scanner starts mid-cycle.
- Per-cycle scanner error budget (consecutive ticker-read failures before circuit-break-into-cycle-metrics).
- Initial seed values for `PriceGapDiscoveryUniverse` and `PriceGapDiscoveryDenylist` (defaults to empty).

## Deferred Ideas Surfaced During Discussion

- PG-OPS-09 top-level Price-Gap tab → Phase 16.
- Auto-promotion path (PG-DISC-02) → Phase 12.
- Score-vs-realized-fill calibration view (PG-DISC-05) → v2.3.
- Universe auto-derivation from `pg:history` → out of scope (D-05 rejects).
