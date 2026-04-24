---
status: pending
phase: 09-price-gap-dashboard-paper-live-operations
plan: 11
source: [09-UAT.md, 09-VERIFICATION.md]
started: <!-- YYYY-MM-DDTHH:MM:SS+08:00 -->
updated: <!-- YYYY-MM-DDTHH:MM:SS+08:00 -->
operator: <!-- your name -->
dev_host: <!-- hostname or "localhost" -->
build_version: <!-- output of `cat VERSION` -->
candidates_loaded: <!-- comma-separated symbols from cfg.PriceGapCandidates -->
spread_threshold_bps: <!-- e.g. 20 -->
paper_mode: true
debug_log: true
---

# Phase 9 — UAT Re-walk Evidence

Walkthrough log for the 14 rows blocked/deferred in `09-UAT.md` (2026-04-22 outcome),
now unblocked by:

- **Plan 09-09** — `PriceGapDebugLog` flag surfaces non-fire reasons in logs, rate-limited per `(symbol, reason)` at 60s
- **Plan 09-10** — `Tracker.Start()` subscribes every candidate × 2 legs; 15s BBO liveness assertion warns on silent legs

Fill each `<!-- … -->` placeholder inline as you walk. Commit when every row is PASS
or has a documented blocking reason.

---

## Pre-Flight — Observability Smoke (Plans 09-09 + 09-10)

Before the 14-row walk, prove the observability plumbing works. All three checks
must PASS or the rest of the walkthrough will stall the same way as 2026-04-22.

| # | Check | Expected log line (example) | Observed | Timestamp | PASS / FAIL |
|---|-------|-----------------------------|----------|-----------|:-----------:|
| P1 | Subscribe fan-out on startup | `pricegap: subscribed SOONUSDT@binance` × candidates × 2 legs | <!-- --> | <!-- --> | <!-- --> |
| P2 | BBO liveness within 15s grace | `pricegap: BBO live SOONUSDT@binance ...` for every leg | <!-- --> | <!-- --> | <!-- --> |
| P3 | Non-fire reasons surface (`PriceGapDebugLog=true`) | `pricegap: no-fire SOONUSDT reason=spread_below_threshold …` | <!-- --> | <!-- --> | <!-- --> |

Notes / anomalies:
<!-- paste any surprising log lines, e.g. dropped subscriptions, BBO silence beyond 15s -->

---

## Task 1 — Full 14-Row UAT Walk (requires a detection to fire)

Threshold pushed to `PriceGapSpreadThresholdBps=20`, paper mode ON, debug log ON.
Expect a fire in 5–30 min on a liquid candidate (SOON / VIC / RAVE).

### Waiting for first fire

| Field | Value |
|-------|-------|
| Fire timestamp | <!-- e.g. 2026-04-24T14:32:07+08:00 --> |
| Candidate that fired | <!-- e.g. SOONUSDT --> |
| Entry spread (bps) | <!-- --> |
| Legs opened | <!-- e.g. binance long / gate short --> |

### 14-row status table

| # | Req | Behavior | Expected | Observed (log snippet / screenshot ref) | Timestamp | PASS / FAIL |
|---|-----|----------|----------|-----------------------------------------|-----------|:-----------:|
| 1 | PG-OPS-02 | Live Positions table populates after paper entry | Row with Mode=paper, Entry / Current / Hold / PnL columns | <!-- --> | <!-- --> | <!-- --> |
| 2 | PG-OPS-02 | Current spread updates via WebSocket | DevTools → Network → WS shows `pg_positions` frames every few s | <!-- --> | <!-- --> | <!-- --> |
| 3 | PG-OPS-02 | PAPER badge violet, LIVE badge green, row tint subtle violet | Visual match with UI-SPEC Color section | <!-- --> | <!-- --> | <!-- --> |
| 4 | PG-OPS-03 | Closed Log shows Modeled / Realized / Delta columns | Row appears on close; red when realized > modeled | <!-- --> | <!-- --> | <!-- --> |
| 5 | PG-OPS-03 | "Load 100 more" button when >100 rows in history | Button visible after sufficient history | <!-- --> | <!-- --> | <!-- --> |
| 6 | PG-OPS-04 | Paper toggle in header flips pill + persists | Violet ↔ green; survives refresh; writes `config.json` | <!-- --> | <!-- --> | <!-- --> |
| 7 | PG-OPS-04 | Flipping toggle mid-life does NOT relabel open paper rows | Existing row stays Mode=paper after flip to live | <!-- --> | <!-- --> | <!-- --> |
| 8 | PG-OPS-04 | Default boot (no `price_gap.paper_mode` key) shows PAPER pill | Fresh boot → violet pill | <!-- --> | <!-- --> | <!-- --> |
| 9 | PG-OPS-05 | Telegram entry alert with 📝 PAPER prefix | Message arrives on entry; prefix present | <!-- --> | <!-- --> | <!-- --> |
| 10 | PG-OPS-05 | Telegram exit alert with reason / PnL / realized slippage / hold | Message on close, all four fields populated | <!-- --> | <!-- --> | <!-- --> |
| 11 | PG-OPS-05 | Risk-block alert fires once; suppressed for 1h on same (gate, symbol) | Trigger a block twice in <1h → second is silent | <!-- --> | <!-- --> | <!-- --> |
| 12 | PG-OPS-05 | No regression: perp-perp `NotifyAutoEntry` still fires | Watch a perp-perp trade cycle; Telegram still alerts | <!-- --> | <!-- --> | <!-- --> |
| 13 | PG-VAL-01 | Closed Log row for paper trade shows Realized > 0 bps | Realized column non-zero on first paper close | <!-- --> | <!-- --> | <!-- --> |
| 14 | PG-VAL-02 | Rolling Metrics populates 24h / 7d / 30d bps/day per candidate; default sort 30d desc | Table renders; active candidate has values; others zero-padded; sort order correct | <!-- --> | <!-- --> | <!-- --> |

### i18n coverage (piggyback check)

| Req | Check | Observed | PASS / FAIL |
|-----|-------|----------|:-----------:|
| i18n | Switch locale to `zh-TW` → every Price-Gap surface renders Chinese, no English fallback | <!-- --> | <!-- --> |

Task 1 resume signal: type **"approved"** in the orchestrator if rows 1–14 all PASS;
otherwise describe failing rows with log snippets so we can plan further gap-closure.

---

## Task 2 — PG-OPS-01 Disable / Re-enable Parity

Verifies single-writer parity between the dashboard modals (Plan 09-07 +
API handlers Plan 09-02) and the `pg-admin` CLI (Phase 8 Plan 08-08). Both must
agree on `pg:candidate:disabled:<SYM>` Redis state.

Candidate used for the parity test: <!-- e.g. RAVEUSDT -->

| # | Step | Expected Redis state / UI state | Actual (paste `redis-cli` output verbatim) | PASS / FAIL |
|---|------|--------------------------------|--------------------------------------------|:-----------:|
| 1 | Dashboard → Price-Gap tab, pick enabled candidate, click **Disable**, reason = "UAT parity test", confirm | Modal closes, row flips to Disabled state with reason | <!-- --> | <!-- --> |
| 2 | `redis-cli -n 2 GET pg:candidate:disabled:<SYM>` | JSON string containing `"UAT parity test"` + ISO8601 `disabled_at` | <!-- paste JSON --> | <!-- --> |
| 3 | Click **Re-enable** on same row, confirm | Modal closes, row flips to Enabled | <!-- --> | <!-- --> |
| 4 | `redis-cli -n 2 GET pg:candidate:disabled:<SYM>` | `(nil)` | <!-- --> | <!-- --> |
| 5 | Shell: `./pg-admin disable <SYM> --reason "CLI test"` | CLI reports success; exit 0 | <!-- paste CLI output --> | <!-- --> |
| 6 | Refresh dashboard | Row shows Disabled with reason "CLI test" | <!-- --> | <!-- --> |
| 7 | Shell: `./pg-admin enable <SYM>` | CLI reports success; exit 0 | <!-- paste CLI output --> | <!-- --> |
| 8 | Refresh dashboard | Row shows Enabled | <!-- --> | <!-- --> |

Task 2 resume signal: type **"approved"** if all 8 steps produce the expected
transitions; otherwise describe the divergence.

---

## Task 3 — Accessibility (Esc close + `role="dialog"`)

Both modals reuse the `Positions.tsx` modal shell per UI-SPEC D-14. Verify with
DevTools open.

| # | Check | Expected | Observed | PASS / FAIL |
|---|-------|----------|----------|:-----------:|
| 1 | Price-Gap tab → click **Disable** on a candidate | Modal renders | <!-- --> | <!-- --> |
| 2 | DevTools → Elements: locate modal root | `role="dialog"` attribute present | <!-- --> | <!-- --> |
| 3 | Press **Esc** | Modal closes without submitting; row remains Enabled | <!-- --> | <!-- --> |
| 4 | Click **Re-enable** on a disabled candidate; DevTools check | `role="dialog"` present, Esc closes without submit | <!-- --> | <!-- --> |
| 5 | Reopen Disable modal → click **Cancel** button | Modal closes, no submit (row stays Enabled) | <!-- --> | <!-- --> |

Screenshot / DevTools reference (optional): <!-- path or one-line confirmation -->

Task 3 resume signal: type **"approved"** if `role="dialog"` is present on both
modals and Esc closes both without submitting; otherwise describe what was
missing.

---

## Overall Sign-Off

All three tasks PASS → Phase 9 ready for `/gsd-verify-work` re-run which should
return `verified` instead of `gaps_found`.

**UAT re-walk completed:** <!-- YYYY-MM-DD by <operator> -->

**Blocking issues to file as new gaps (if any):**

<!-- bullet list of failing rows + proposed gap-closure plan number, or "none" -->

