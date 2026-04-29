# Phase 12: Auto-Promotion - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-29
**Phase:** 12-auto-promotion
**Areas discussed:** Streak tracking, Auto-demote policy, Cap-full behavior, Event surfacing, Master toggle

---

## Gray Area Selection

| Option | Description | Selected |
|--------|-------------|----------|
| Streak tracking | How "≥6 consecutive cycles" is measured + persisted | ✓ |
| Auto-demote policy | When a promoted candidate gets auto-demoted | ✓ |
| Cap-full behavior | What happens when cap is hit | ✓ |
| Event surfacing | Redis schema + WS + Telegram for promote/demote events | ✓ |

**User's choice:** All four areas selected.

---

## Streak Tracking

### Q1: How is "≥6 consecutive cycles" measured?

| Option | Description | Selected |
|--------|-------------|----------|
| Counter | Per-candidate int: increment if score ≥ threshold, reset to 0 if below; promote when counter ≥ 6 | ✓ |
| Sliding window | Read last 6 entries from `pg:scan:scores:{symbol}` ZSET; all must be ≥ threshold | |
| Average of last 6 | Mean of last 6 scanner cycle scores ≥ threshold | |

**User's choice:** Counter
**Notes:** Closest to literal "consecutive" wording in PG-DISC-02; simplest to test.

### Q2: What happens when a candidate isn't in next cycle's accepted Records?

| Option | Description | Selected |
|--------|-------------|----------|
| Reset streak | Any non-accepted cycle resets the counter — strict consecutive | ✓ |
| Hold streak | Only score < threshold breaks; missing/rejected cycles ignored | |
| Tolerate 1 miss | Allow 1 missed cycle; 2 consecutive misses reset | |

**User's choice:** Reset streak
**Notes:** Pitfall-1 aligned; fragile candidates should not promote.

---

## Auto-Demote Policy

### Q3: When should a promoted candidate be auto-demoted?

| Option | Description | Selected |
|--------|-------------|----------|
| Symmetric streak | 6 consecutive cycles below `PriceGapAutoPromoteScore`, mirror promote logic | ✓ |
| Hysteresis | 6 consecutive cycles below a lower demote threshold (e.g. promote − 10) | |
| Manual-only demote | Controller never auto-demotes; operator demotes via dashboard/pg-admin | |
| Disappear-from-universe | Demote when candidate drops out of `Records` for 6 cycles regardless of why | |

**User's choice:** Symmetric streak
**Notes:** No new threshold to tune; mirror of promote logic.

---

## Master Toggle

### Q4: Should auto-promote share `PriceGapDiscoveryEnabled` or have its own switch?

| Option | Description | Selected |
|--------|-------------|----------|
| Single switch | Reuse `PriceGapDiscoveryEnabled` — scanner + promote both ON/OFF together | ✓ |
| Separate flag | New `PriceGapAutoPromoteEnabled` for read-only scanner during calibration | |

**User's choice:** Single switch
**Notes:** Phase 12 is what discovery was always meant to do; one config field is the natural surface.

---

## Cap-Full Behavior

### Q5: When `PriceGapMaxCandidates` is hit and a higher-scored candidate would otherwise promote?

| Option | Description | Selected |
|--------|-------------|----------|
| Skip silently | Increment a `cap_full_skips` metric on `pg:scan:metrics`; no Telegram, no event | ✓ |
| Skip + Telegram nudge | Skip and send one Telegram per cycle that the cap is full | |
| Auto-displace | Demote lowest-scored promoted candidate to make room (guard-permitting) | |

**User's choice:** Skip silently
**Notes:** Least surprise; operator sees the skip in dashboard counters; no alert fatigue.

---

## Event Surfacing

### Q6: Redis schema for promote/demote events

| Option | Description | Selected |
|--------|-------------|----------|
| LIST + LTrim 1000 | `pg:promote:events` LIST with RPush + LTrim 1000 — mirrors `pg:scan:cycles` | ✓ |
| HASH per-day + EXPIRE | `pg:promote:events:{YYYY-MM-DD}` HASH, 30d EXPIRE — mirrors rejections | |
| Both | LIST for live timeline + per-day HASH counter | |

**User's choice:** LIST + LTrim 1000
**Notes:** Symmetry with Phase 11 telemetry pattern; ordered, easy to render as timeline, bounded memory.

### Q7: Telegram alert shape for promote/demote events

| Option | Description | Selected |
|--------|-------------|----------|
| Per-event, unique key | One Telegram per event with cooldown key including action+symbol+exchanges+direction | ✓ |
| Per-event, no cooldown | One Telegram per event, bypass cooldown entirely | |
| Per-cycle digest | One Telegram per scanner cycle that had any events | |

**User's choice:** Per-event, unique key
**Notes:** Distinct events bypass each other's 5-min cooldown via existing `checkCooldownAt()`; same-candidate flap within 5 minutes is intentionally throttled.

---

## Final Confirmation

### Q8: Anything else to nail down before writing CONTEXT.md?

| Option | Description | Selected |
|--------|-------------|----------|
| Ready for context | The 7 decisions cover the implementation gray areas; remaining choices are Claude's discretion | ✓ |
| More questions | Surface 2-4 additional gray areas | |

**User's choice:** Ready for context

---

## Claude's Discretion

The following areas were either explicitly deferred to planning-time judgment or not surfaced as questions:
- Exact field names + JSON encoding of `pg:promote:events` entries
- Exact REST route for the events seed (single state endpoint vs separate `/promote-events`)
- Internal struct names + signatures of controller's interface dependencies
- Test fixture shapes for streak transitions
- Telegram message wording polish
- Whether `cap_full_skips` is a single counter or per-symbol

## Deferred Ideas

Captured in CONTEXT.md `<deferred>` section — see that file for the full list with rejection rationale per item.
