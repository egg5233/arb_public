# Pitfalls Research — v2.2 Auto-Discovery & Live Strategy 4

**Domain:** Live multi-strategy arbitrage bot — adding auto-discovery + live capital ramp + drawdown breaker + per-fill alerts to existing 3-engine system
**Researched:** 2026-04-27
**Confidence:** HIGH (grounded in this repo's history: GUAUSDT incident, config wipe, Phase 999.1 `math.Abs` bug, .planning/RETROSPECTIVE.md, v2.0/v2.1 audits)

## Critical Pitfalls

### Pitfall 1: Auto-Discovery False-Positive Pump (transient cross fires)

**What goes wrong:**
Scanner sees a momentary 1-bar spread cross on a thinly-traded SOON-listed pair, scores it high on "spread magnitude", auto-promotes it to `cfg.PriceGapCandidates`, and the executor opens real-money legs the next scan tick. Edge was timestamp-misalignment noise, not a tradable dislocation.

**Why it happens:**
RETROSPECTIVE.md key lesson: "1-bar close-to-close spread crossings inflate edge estimates ~10× due to timestamp-alignment noise." Scoring code that uses raw spread magnitude or single-bar threshold is the same anti-pattern Strategy 4 already burned itself on. The scanner is also new code with no chokepoint history.

**How to avoid:**
- Score MUST require ≥4 consecutive 1m bars same-sign-cross (mirror the Phase 8 `barRing.allExceed` invariant). Single-bar crosses are scored zero.
- Score MUST require BBO freshness <90s on BOTH legs (mirror PG-RISK).
- Score MUST factor book depth at threshold and retreat depth — not just headline spread.
- Symbol normalization: scanner internal symbol is `BTCUSDT` (uppercase, no separators); Gate `BTC_USDT`, OKX `BTC-USDT-SWAP` map happens *inside* each adapter. Compare canonical form before joining cross-exchange BBOs.
- Hard denylist of known-noisy pairs (operator-curated, persisted to config).
- Discovered candidate must show paper-mode positive net bps over a configurable observation window before its score clears `PriceGapAutoPromoteScore`.
- Loris funding rates are 8h-equivalent — scoring must always ÷8 to bps/h before comparison.

**Warning signs (Claude can grep for):**
- Scoring fn referencing `spread` or `gap_bps` without `persistence` / `bars_above` / `consecutive`.
- BBO timestamps used in scoring without an `age_seconds` or freshness comparison.
- `strings.ToUpper` or `strings.ReplaceAll` on symbols anywhere outside adapter boundaries (canonical form should be enforced once, at scanner ingest).
- Score that monotonically rises with raw `|spread|` and has no cap.
- Missing `denylist` / `excluded_symbols` reference in scanner config.
- Test fixtures lacking a "1-bar pump rejected" case.
- Funding rate used raw without `/8` divisor.

**Phase to address:** v2.2 Phase 14 (Auto-Discovery Scanner — PG-DISC-01).

---

### Pitfall 2: Three-Writer Race on cfg.PriceGapCandidates → Config Corruption

**What goes wrong:**
Operator clicks "Edit candidate" in dashboard at the same instant the auto-promotion goroutine appends a discovered candidate AND a `cmd/pg-admin disable` invocation rewrites the slice. Two writers read the same `*config.Config`, both call `SaveJSON` with their stale view, the later write wins and silently drops the other's mutation. `.bak` only preserves one prior state — the dropped change is gone forever.

**Why it happens:**
`config.json` is sole source of truth (per 2026-04-05 fix). v2.1 added Phase 10 dashboard CRUD as the second writer. Auto-promotion (PG-DISC-02) introduces a third writer with NO operator click — it fires on score crossing. Existing `SaveJSON` does plain read-modify-write with single `.bak`; nothing serializes across writers, no version/etag check.

**How to avoid:**
- Single mutation chokepoint for `cfg.PriceGapCandidates`: a `CandidateRegistry` with internal `sync.Mutex` (or `sync.RWMutex`) that ALL writers go through (dashboard handler, pg-admin RPC, auto-promotion goroutine).
- Read-modify-write inside the chokepoint: lock → reload from disk → apply mutation → atomic write → release. `config.json` becomes the durable serialization point even if writers are reading stale in-memory cfg.
- Atomic write pattern: write to `config.json.tmp` → fsync → `os.Rename` → keep ring of N `.bak.{ts}` not just single `.bak`.
- Dedup before append: scanner promotion MUST check `(symbol, long_exch, short_exch, direction)` tuple against existing entries; reject duplicates. Same key as Phase 10 active-position guard — reuse it.
- Promotion writes also persist to `pg:discovered:promoted:{ts}` Redis audit trail with prior candidate-list snapshot, so an overwrite is recoverable.

**Warning signs (Claude can grep for):**
- `cfg.PriceGapCandidates = append(cfg.PriceGapCandidates, …)` outside the new chokepoint.
- `SaveJSON(...)` called from more than one package (count grep results — should be exactly one helper).
- Auto-promotion goroutine that does NOT acquire the same mutex as `POST /api/config` handler.
- `.bak` rotation that overwrites a single file without timestamp suffix.
- Missing tuple-equality check before promotion append.
- pg-admin commands writing config without going through the same registry.

**Phase to address:** v2.2 Phase 15 (Auto-Promotion — PG-DISC-02). Must land BEFORE the scanner is allowed to write.

---

### Pitfall 3: Live Capital Ramp — Stale Clean-Days Counter Across Restarts

**What goes wrong:**
Ramp controller is at stage 1 (100 USDT/leg, day 5 of 7 clean). Process restarts (binary drift monitor, crash, deploy). On boot, the counter resets to 0, OR worse — controller reads "7 clean days elapsed since first-ever ramp start" from a stale Redis key and immediately jumps to stage 2 (500 USDT/leg) even though there was a losing day yesterday that should have reset the counter.

**Why it happens:**
Live ramp is brand new code; no rehydration path tested against the binary drift restart pattern. Three classic ramp-controller bugs apply: (a) "clean day" defined as wall-clock day not "trading day with ≥1 closed position"; (b) counter advances on `now - start_ts >= 7d` not "7 sequential clean days"; (c) state lives only in process memory and resets on restart.

**How to avoid:**
- Persist ramp state to Redis with explicit fields: `stage`, `current_stage_started_at`, `consecutive_clean_days`, `last_evaluated_day_utc`, `last_loss_day_utc`. NEVER derive from wall clock alone.
- Define "clean day" precisely: strict net-PnL ≥ 0 AND no L4/L5 health events AND ≥1 closed position. Document and test all three terms.
- Idempotent daily evaluation: a tick that fires twice on the same UTC day must not double-advance (use `last_evaluated_day_utc` guard).
- ANY losing day ⇒ reset counter to 0 AND demote stage by one if currently above stage 1. Demotion is harder than promotion (asymmetric ratchet).
- Hard ceiling 1000 USDT/leg enforced as `min(stage_size, hard_ceiling)` at sizing time, NOT only at stage transition — so config typo can't bypass.
- Rehydrate-then-validate on boot: load Redis state, recompute today's clean status from PnL log, refuse to advance if recomputed counter disagrees.
- Off-by-one defenses: counter starts at 0 (not 1) on stage entry; promotion fires at `>=7` (not `>7`); test both boundaries.

**Warning signs (Claude can grep for):**
- Ramp counter as in-memory `int` field with no `redis.HSet` companion.
- Day comparison using `time.Now().Sub(start) >= 7*24*time.Hour` (wall-clock, not clean-day count).
- Promotion condition `>` vs `>=` inconsistent with demotion check.
- Sizing path that reads `stage_budget_per_leg` directly without `min(_, hard_ceiling)`.
- Missing test for "restart mid-stage" / "restart after losing day" / "two evaluations same UTC day".
- Loss day handling that decrements counter by 1 instead of resetting to 0.

**Phase to address:** v2.2 Phase 14 (Strategy 4 Live Capital — ramp controller is the central new component).

---

### Pitfall 4: Drawdown Circuit Breaker — Funding-Settlement False Trip

**What goes wrong:**
Strategy 4 holds a delta-neutral position across a funding settlement window. Booked PnL on one leg dips negative briefly because the perp side took a funding payment that hasn't been credited to the spot/other-perp side yet (settlement is asynchronous and exchange-specific). Daily-loss breaker reads the gap, exceeds threshold, force-closes all positions and auto-reverts to paper mode. The "loss" was a settlement timing artifact and would have closed by the next reconcile pass.

**Why it happens:**
"Daily" is poorly defined. Bybit blackout :04–:05:30 means PnL snapshots taken inside that window are stale. Different exchanges have different settlement clocks (UTC 00:00 vs aligned 8h windows). Breaker code measures unrealized PnL from per-leg `marginBalance` deltas without normalizing for asynchronous funding accrual.

**How to avoid:**
- Daily breaker measures REALIZED PnL only (closed positions in rolling 24h window) — NOT mark-to-market of open positions. MTM is a separate `unrealized_drawdown_pct` gauge with a higher, position-level threshold.
- Define "day" as rolling 24h ending at `now()`, NOT calendar-day in any local timezone. Document the timezone explicitly. Asia/Taipei display ≠ accounting boundary.
- Skip evaluation during exchange-specific blackouts (Bybit :04–:05:30). Buffer breaker evaluation outside any settlement window.
- Use BingX-style normalization rule: `netProfit` already includes funding — DO NOT double-count (per `project_bingx_pnl_fix.md`).
- Two-strike rule: breaker requires breach on TWO consecutive evaluations ≥5 min apart before tripping. Eliminates single-snapshot artifacts.
- Recovery is human-gated: tripped breaker auto-flips to paper, but flipping BACK to live requires explicit operator confirmation in dashboard (NOT a timer).
- Reuse v1.0's loss-limit infrastructure (rolling 24h/7d sorted-set in Redis) — do NOT build a parallel system.

**Warning signs (Claude can grep for):**
- Breaker reading `unrealizedPnl` or `markToMarket` for the gating decision.
- `time.Now().Truncate(24*time.Hour)` or `Local()` calls in breaker code.
- Single-snapshot trip without a confirmation tick.
- Auto-recovery path: any code that flips `paper_mode=false` after a tripped breaker without a `/api/config` operator POST.
- Funding payments queried separately from PnL on adapters that already include them in netProfit (BingX).
- Breaker that doesn't suppress evaluation during exchange blackout windows.

**Phase to address:** v2.2 Phase 14 (drawdown circuit breaker is bundled with live capital).

---

### Pitfall 5: Daily PnL Reconcile — Cross-Day Positions Double-Counted or Lost

**What goes wrong:**
Reconcile job runs at UTC 00:00. Position opened 23:55, still open at 00:05. Job assigns it to "yesterday" (entry day) but 00:05 mark-to-market is read AFTER the day boundary, so unrealized fluctuations cross days. When the position closes at 02:00 the next day, realized PnL is booked to "today" — yesterday's report shows a phantom MTM gain that is then realized into today, double-counted.

**Why it happens:**
Reconcile is new code. Fee allocation is exchange-specific — some charge maker/taker on entry, some accrue funding hourly, some charge withdrawal fees with random clock skew vs the bot's own clock. Going from "first live PnL job" to "production-grade" without ledger discipline is the trap.

**How to avoid:**
- Reconcile is per-CLOSED-position, NOT per-day mark-to-market. A position contributes to "the day it CLOSED" with full lifecycle PnL (fees, funding, basis, slippage decomposition — reuse v1.0 AN-01 schema).
- Open positions across the day boundary appear in an "open at boundary" list, NOT counted toward day PnL.
- Reconcile is idempotent: keyed by `(position_id, version)`. Re-running reconcile for the same day must produce byte-identical output.
- 3-retry pattern at 5s/15s/30s for adapter PnL fetch (reuse existing perp-perp pattern; do NOT roll your own).
- Cross-check: reconciled day PnL must equal sum of `pg:positions:closed:*` realized PnL fields for that close-day. Discrepancy ⇒ alert, do NOT silently drop.
- Exchange clock skew: reconcile uses position close timestamps from EXCHANGE response, not local `time.Now()`. SQLite stores both for diffing.
- Run reconcile ≥30 minutes after UTC 00:00 to allow async settlements (Bitget, Gate.io) to land.

**Warning signs (Claude can grep for):**
- Reconcile loop iterating "yesterday's date range" by start time of position rather than close time.
- Mark-to-market values written to the daily PnL ledger.
- Missing `(position_id, version)` idempotency key.
- `time.Now()` used as the close timestamp instead of exchange response field.
- No cross-check between reconciled total and sum of closed-position records.
- Reconcile schedule running at exactly 00:00:00 UTC (settlement race).

**Phase to address:** v2.2 Phase 14 (daily PnL reconcile is part of live capital).

---

### Pitfall 6: Telegram Alert Flooding on Per-Fill Notification

**What goes wrong:**
Live ramp opens a 100 USDT/leg position; depth-fill executor places 5 slices per leg = 10 fills, each fires a `NotifyPriceGapEntry`. Then exit triggers in 4 minutes, another 10 fills. Operator's Telegram is hit with 20 messages in <5 minutes. During a critical L4/L5 health alert, the cooldown logic suppresses the SAFETY alert because it shares a notifier with the spammy fill alerts. Operator misses the actual emergency.

**Why it happens:**
Existing notifier has per-event-type cooldown (v1.0 PP-01). Adding "per fill" without a new event-type bucket means it shares cooldown with already-throttled events. Or worse, per-fill alerts use a NEW event type with no cooldown at all.

**How to avoid:**
- Per-fill alerts go in a SEPARATE event bucket with their own rate limit (e.g., `pricegap_fill`). Critical buckets (`l4_emergency`, `l5_critical`, `loss_limit_hit`, `breaker_tripped`) MUST never share a bucket with high-frequency events.
- Batch fills by position: emit ONE entry-complete and ONE exit-complete summary, NOT one per slice. Slice-level detail goes to dashboard WS, not Telegram.
- Token-bucket per bucket: e.g., 5 messages/minute for `pricegap_fill`, drop excess to "summary" message ("3 additional fills suppressed — see dashboard").
- Critical alerts (`l4`/`l5`/`breaker`) bypass ALL rate limits and use Telegram parse_mode for visual distinction.
- Notifier is `nil`-safe (already a project pattern) — but ALSO must not block engine on Telegram API timeout. Use buffered channel + worker, drop+count on full.
- Test: synthetic burst of 50 fill events in 10s must not cause dropped critical alert injected mid-burst.

**Warning signs (Claude can grep for):**
- Per-fill `NotifyXxx` call inside the per-slice loop in executor.
- New `Notify*` method without an event-type constant added to the cooldown table.
- Direct synchronous Telegram HTTP call from engine goroutine (must be channel/queue).
- Cooldown bucket shared between `pricegap_fill` and `l4_emergency`.
- Missing test that interleaves fill burst with critical alert.

**Phase to address:** v2.2 Phase 14 (Telegram per-fill is bundled with live capital — must NOT regress v1.0 PP-01).

---

### Pitfall 7: Tech-Debt Closure Surfaces Latent Regressions

**What goes wrong:**
Operator opens browser to do retrospective Phase 02 confirmation (spot-futures Dir A/B). Modal opens but submit returns 500; turns out an unrelated v2.1 refactor renamed a JSON field 30 days ago and Dir B has been silently broken since. Or: Nyquist Wave-0 for Phase 06 (maintenance gate) shows the dashboard toggle sets a key that no longer exists in `cfg`. Live trading has been bypassing the gate for 30 days.

**Why it happens:**
Code paths that haven't been clicked in 30 days drift. The retrospective is the FIRST exercise of these paths since v1.0 shipped. Phase 999.1 closure of v2.1 already exposed this pattern — it found `barRing.allExceed` had been silently firing wrong-side trades because it used `math.Abs`. Tech debt closure is dangerous specifically because it re-exercises stale paths.

**How to avoid:**
- BEFORE the retrospective, run a smoke pass: every dashboard tab, every config toggle, every modal — fail fast on 500/404/handler missing. Capture HAR/Network panel.
- Tech-debt phase plans must include "regression triage" budget: if browser confirmation surfaces a real bug, do NOT silently fix-and-merge into the retrospective phase. Open a separate hot-fix phase, document the regression, ship under its own version bump.
- Frontend builds: `npm ci` only (axios lockdown). `make build` (frontend before Go binary) is non-negotiable — a stale `dist/` will hide the regression entirely.
- Run `git log --since="30 days ago" --diff-filter=M -- internal/api/ web/src/` before starting browser confirms; flag any modified handler/page that lacks a corresponding live-fire test in 30 days.
- VERIFICATION.md retrospective is `human_needed` template — but VALIDATION.md (Nyquist Wave-0) requires actual tooling run; do not stub it with "code reviewed by hand".

**Warning signs (Claude can grep for):**
- VERIFICATION.md committed without a corresponding `git log` of the actual confirmation session.
- VALIDATION.md without a Nyquist tool invocation log.
- Dashboard toggle wired to a config field that has no usage in `internal/` (dead toggle).
- i18n key in EN locale missing from zh-TW (Phase 999.1 already proved this is a recurring lockstep failure).
- Handler 500 caught by curl but not by Vitest (frontend tests don't exercise HTTP).

**Phase to address:** v2.2 dedicated tech-debt phase (Phase 16+) — must run AFTER auto-discovery + live ramp are on chokepoint, so the retrospective itself doesn't risk live trading.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|---|---|---|---|
| Skip ramp state persistence to Redis ("we restart rarely") | -1 day of work | Counter resets ⇒ unauthorized stage jump on restart ⇒ real-money overexposure | Never (real money in flight) |
| Reuse single `.bak` file for config writes | Zero work | Concurrent writers ⇒ silent mutation loss | Only if mutation chokepoint is single-writer |
| Auto-promote without paper-mode observation window | Faster path to live promotion | False-positive promotion executes real trades | Never on live capital; OK in paper-only milestones |
| Per-fill alerts using existing `pp_critical` cooldown bucket | One-line code | Floods bucket ⇒ critical alerts dropped | Never |
| VERIFICATION.md stub with "human visually confirmed" | Closes audit checkbox | Latent regressions stay buried until next audit | Never (this is exactly the v1.0 → v2.0 → v2.1 carry pattern that brought us here) |
| Daily reconcile on calendar-day boundaries in Asia/Taipei | Matches dashboard display | Cross-day double-counting + DST traps | Never on accounting; OK on dashboard view layer only |
| Dropping the `direction: pinned` sign-filter in scoring | Simplifies score formula | Recreates the `math.Abs` bug Phase 999.1 just closed | Never |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|---|---|---|
| `cfg.PriceGapCandidates` write | Direct `append` from any goroutine | Single `CandidateRegistry` chokepoint with mutex + atomic file write |
| Bybit funding settlement | PnL snapshot during :04–:05:30 blackout | Suppress reconcile/breaker evaluation in this window |
| Gate.io 1m kline history | Scanner asks for >10k bars | Cap at 10k; document pagination |
| OKX symbol format | Scanner uses `BTC-USDT-SWAP` in cross-exchange comparison | Canonicalize to `BTCUSDT` at ingest, map at adapter boundary |
| BingX no-margin | Auto-discovery proposes BingX as spot leg | Adapter capability check; reject candidate if either leg lacks required interface |
| Telegram API | Synchronous send from engine goroutine | Buffered channel + worker, drop-and-count on overflow |
| Redis `pg:discovered:*` | TTL-less keys accumulate | Set TTL on every write; periodic janitor goroutine |
| systemd binary drift restart | Ramp controller assumes process uptime | Persist ramp state; `INVOCATION_ID` is not a session anchor |
| `config.json` `.bak` | Single file overwritten on every save | Timestamped ring (last N saves) |
| Funding rate ÷8 normalization | Scanner uses Loris 8h-equivalent rate raw | Always ÷8 to bps/h (project_loris_normalization.md) |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|---|---|---|---|
| Scanner polls all 6 exchanges every tick on every candidate-universe pair | Rate-limit 429s, kline staleness | Tiered cadence: hot list every tick, cold list every Nth tick | >50 pairs × 6 exchanges |
| Per-fill Telegram blocking engine | Entry latency spikes during volatility | Async notifier worker | First high-volatility burst |
| Discovered-candidate Redis writes without TTL | Redis DB 2 memory unbounded | TTL on `pg:discovered:*` | Days of operation |
| Auto-promotion runs in scanner goroutine | Scanner stalls on `SaveJSON` fsync | Promotion is enqueued; separate worker | Concurrent operator dashboard write |
| Reconcile job iterates all closed positions | SQLite scan grows linearly | Index on `closed_at` UTC day; query bounded range | After ~30 days of live data |
| Drawdown breaker recomputes 24h sum every tick | CPU spike on long histories | Rolling sorted-set window (reuse v1.0 loss-limit infra) | After ~7 days |

## Security Mistakes

| Mistake | Risk | Prevention |
|---|---|---|
| Auto-promotion accepts unsanitized `symbol` field from scanner | Config injection ⇒ adapter calls with crafted symbol | Reuse Phase 10 server-side regex validator (`^[A-Z0-9]{2,20}USDT$`) |
| Telegram alert echoes raw exchange API error string | Leaks API key fragments / order IDs publicly | Redact via existing notifier sanitizer; test with synthetic key string |
| Reconcile writes per-position PnL to SQLite without auth check | Dashboard exposes positions across operators | Already auth-gated (Bearer token) — verify scanner endpoints inherit |
| pg-admin CLI bypasses auth on local socket | Local privilege escalation | Hard-gate on `INVOCATION_ID` + filesystem perms on `config.json` |
| `PriceGapAutoPromoteScore` configurable via dashboard with no minimum floor | Operator typo `0` ⇒ everything auto-promotes | Server-side minimum (e.g., ≥50) |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---|---|---|
| Auto-promoted candidate appears in list with no visual indicator | Operator can't tell "I added this" vs "scanner added this" | Badge: `auto` vs `manual` on row, with promoted timestamp |
| Drawdown breaker trips silently | Operator sees positions closing, doesn't know why | Banner + Telegram critical alert + dashboard event log entry, ALL three |
| Live ramp stage shown only as integer "Stage 2" | Operator doesn't know what 2 means | Display `Stage 2 — 500 USDT/leg, 4 of 7 clean days, next: 1000 USDT/leg` |
| Paper mode toggle on same row as Live capital toggle | One-click slip ⇒ live trading | Separate sections; require typed confirmation for live flip |
| Per-fill Telegram with no position context | 20 alerts, can't tell which position | Position ID + total slices in alert: `Fill 3/5 for pg_SOON_gate_bingx` |

## "Looks Done But Isn't" Checklist

- [ ] **Auto-discovery scanner:** Often missing canonical symbol normalization across exchanges — verify cross-exchange BBO join produces non-empty result for known-good pair (`BTCUSDT` on Gate × Bitget).
- [ ] **Auto-promotion:** Often missing tuple-dedup against existing `cfg.PriceGapCandidates` — verify promoting a manually-added candidate is rejected.
- [ ] **Auto-promotion:** Often missing `.bak` rotation — verify N saves leaves N timestamped backups.
- [ ] **Live ramp:** Often missing restart rehydration — verify `kill -9` mid-stage and reboot resumes correct stage + counter.
- [ ] **Live ramp:** Often missing `min(stage, hard_ceiling)` guard at sizing call site — verify config typo of `stage_3_size_usdt: 9999` still sizes at 1000.
- [ ] **Drawdown breaker:** Often missing two-strike rule — verify a single funding-window snapshot below threshold does NOT trip.
- [ ] **Drawdown breaker:** Often missing human-gated recovery — verify a tripped breaker stays in paper mode after restart.
- [ ] **Daily reconcile:** Often missing idempotency — verify running it twice produces identical output.
- [ ] **Daily reconcile:** Often missing position-close timestamp from EXCHANGE response — verify cross-day position is booked to its close-day, not its open-day.
- [ ] **Telegram per-fill:** Often missing rate-limit + critical-bucket isolation — verify burst of 50 fills does not delay a synthetic L5 alert by >2s.
- [ ] **Telegram per-fill:** Often missing async dispatch — verify Telegram API timeout (set MITM 30s delay) does not block engine tick.
- [ ] **Tech-debt browser confirms:** Often skipped after stale-path bugs — verify `git log` for changed handler files, run smoke pass before retrospective.
- [ ] **i18n EN/zh-TW lockstep:** Verify every new key exists in BOTH locale files (Phase 999.1 already burned us here).
- [ ] **All v2.2 features:** Default OFF in `config.json` per project rule — verify `EnablePriceGapAutoDiscovery=false`, `EnablePriceGapAutoPromote=false`, `EnablePriceGapLiveCapital=false`.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---|---|---|
| False-positive auto-promotion executed real trades | MEDIUM | 1. Flip `EnablePriceGapAutoPromote=false`. 2. Demote candidate via dashboard. 3. Close any open position via dashboard. 4. Add symbol to denylist. 5. Audit scanner score formula. |
| Config corrupted by 3-writer race | HIGH | 1. Stop bot (systemd). 2. Restore `config.json.bak.{ts}` chosen via diff against Redis `pg:discovered:promoted:*` audit. 3. Audit `pg:positions:active` against restored candidate list. 4. Restart. |
| Ramp counter advanced incorrectly post-restart | MEDIUM | 1. Stop bot. 2. Manually reset Redis ramp keys to last known good stage. 3. Recompute clean-days from PnL log. 4. Restart with verbose ramp logs. |
| Drawdown breaker false-tripped | LOW | 1. Inspect breaker event log + position PnL at trip time. 2. If artifact: human-confirm flip back to live via dashboard. 3. Tighten 2-strike spacing or threshold if pattern repeats. |
| Reconcile double-counted across day boundary | MEDIUM | 1. Truncate erroneous day rows. 2. Re-run reconcile from `pg:positions:closed:*` ground truth. 3. Idempotency key fix. |
| Telegram drowned operator missed critical alert | HIGH (depends on incident) | 1. Verify L4/L5 fired in event log. 2. Move per-fill bucket to summary mode. 3. Add monitoring on bucket overflow counter. |
| Tech-debt retrospective surfaced live regression | HIGH | 1. Stop tech-debt phase. 2. Open hot-fix phase. 3. Fix + version bump + Telegram-announce. 4. Re-run smoke pass. 5. Resume retrospective. |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---|---|---|
| 1. Auto-discovery false positives | v2.2 Phase 14 (PG-DISC-01) | Synthetic 1-bar pump fixture rejected by scoring; paper-mode observation window enforced |
| 2. 3-writer config race | v2.2 Phase 15 (PG-DISC-02), gates Phase 14 going live | Concurrent dashboard + scanner + pg-admin write test produces no lost mutations; tuple-dedup test green |
| 3. Live ramp counter | v2.2 Phase 14 (live capital) | `kill -9` during stage advance test; loss-day-resets-counter test; min(_, ceiling) sizing test |
| 4. Drawdown breaker false trip | v2.2 Phase 14 (live capital) | Funding-window snapshot does not trip; 2-strike rule enforced; human-gated recovery test |
| 5. Daily reconcile cross-day | v2.2 Phase 14 (live capital) | Cross-day position booked to close-day; idempotency test; clock-skew test against exchange timestamps |
| 6. Telegram alert flood | v2.2 Phase 14 (live capital, per-fill alerts) | 50-fill burst + injected L5 alert: critical alert delivered <2s |
| 7. Tech-debt retrospective regressions | v2.2 Phase 16+ (sequenced AFTER live + scanner are stable) | Smoke pass HAR captured; `git log` review documented; any regression spawns hot-fix phase |

## Sources

- `.planning/RETROSPECTIVE.md` — v1.0 lessons; especially "1-bar crossings inflate edge ~10×" and "GSD encoded code-done not verified-done"
- `.planning/MILESTONES.md` — v2.0 + v2.1 shipped notes; Phase 999.1 `math.Abs` bug
- `.planning/milestones/v2.0-ROADMAP.md` — Phase 8 known issues: realized_slip zero, paper_mode auto-flip
- `.planning/milestones/v2.1-ROADMAP.md` — Phase 10 active-position guard pattern; tuple-dedup precedent
- `CLAUDE.local.md` — config.json sole source of truth; Bybit blackout; npm lockdown; build order
- Memory: `project_guausdt_close_bug.md` — dust partial fill recovery
- Memory: `project_config_wipe_fix.md` — why config.json single-write discipline matters
- Memory: `project_bingx_pnl_fix.md` — netProfit includes funding; reconcile must not double-count
- Memory: `project_loris_normalization.md` — ÷8 normalization for funding rates in scanner scoring
- Memory: `feedback_scan_minute_regression.md` — regressions in re-exercised paths

---
*Pitfalls research for: v2.2 Auto-Discovery & Live Strategy 4*
*Researched: 2026-04-27*
