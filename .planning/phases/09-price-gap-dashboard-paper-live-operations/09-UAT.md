# Phase 9 — User Acceptance Testing Checklist

Human operator runs through these after `make build` and local launch. Each
row maps to a requirement and an `09-UI-SPEC.md` section. Every row must
be checked for Phase 9 sign-off — any unchecked row blocks release.

> **Safety reminder:** Claude MUST NOT modify `config.json`. Toggles are
> flipped via the dashboard, or the operator edits `config.json` manually.
> Run this walkthrough on a dev/staging host, never on a live-capital
> deployment without explicit board approval.

## Requirement Coverage

| Req        | Behavior                                                                                                                                                                                    | UI-SPEC ref            | Checked |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------- | ------- |
| PG-OPS-01  | Price-Gap tab appears between spot-positions and history in nav                                                                                                                             | Nav Integration        | [ ]     |
| PG-OPS-01  | Candidates table lists every entry from `cfg.PriceGapCandidates`                                                                                                                            | Layout Contract §2     | [ ]     |
| PG-OPS-01  | Disable button on an enabled candidate opens modal with reason field; typing reason + confirm writes `pg:candidate:disabled:<symbol>` (verify via `redis-cli -n 2 GET pg:candidate:disabled:<SYM>`) | Interaction Contracts  | [ ]     |
| PG-OPS-01  | Re-enable button on a disabled candidate opens modal showing reason + age; confirm clears the Redis key                                                                                     | Interaction Contracts  | [ ]     |
| PG-OPS-01  | Disabling via `pg-admin` CLI then refreshing dashboard shows same state (single-writer parity)                                                                                              | Canonical refs         | [ ]     |
| PG-OPS-02  | Live Positions table populates after a paper entry, showing Entry / Current / Hold / PnL columns                                                                                            | Live Positions table   | [ ]     |
| PG-OPS-02  | Current spread updates via WS (watch DevTools → Network → WS frames for `pg_positions`)                                                                                                     | Interaction Contracts  | [ ]     |
| PG-OPS-02  | PAPER badge violet, LIVE badge green; row tint on paper rows is subtle violet                                                                                                               | Color                  | [ ]     |
| PG-OPS-03  | Closed Log shows Modeled, Realized, Delta columns with correct sign color (red when realized > modeled)                                                                                     | Closed Log table       | [ ]     |
| PG-OPS-03  | "Load 100 more" button appears when >100 rows in history                                                                                                                                    | Pagination             | [ ]     |
| PG-OPS-04  | Paper toggle in header flips between violet PAPER pill and green LIVE pill; POST `/api/config` persists to `config.json`                                                                    | Header toggles         | [ ]     |
| PG-OPS-04  | Flipping toggle mid-life does NOT relabel existing open paper positions                                                                                                                     | State Machine          | [ ]     |
| PG-OPS-04  | Default boot with no `price_gap.paper_mode` key in `config.json` → PAPER pill shown (default TRUE per D-12)                                                                                 | Pitfall 1              | [ ]     |
| PG-OPS-05  | Telegram entry alert arrives on paper entry with 📝 PAPER prefix                                                                                                                            | Copywriting Contract   | [ ]     |
| PG-OPS-05  | Telegram exit alert arrives with reason, PnL, realized slippage, hold duration                                                                                                              | D-20                   | [ ]     |
| PG-OPS-05  | Telegram risk-block alert arrives once; subsequent alerts for same `(gate, symbol)` within 1h are suppressed                                                                                | D-21                   | [ ]     |
| PG-OPS-05  | No regression: perp-perp entry still fires `NotifyAutoEntry` (watch a perp-perp trade cycle)                                                                                                | Plan 04 regression     | [ ]     |
| PG-VAL-01  | Closed Log row for a paper trade shows Realized > 0 bps (Pitfall 7 proof)                                                                                                                   | D-04                   | [ ]     |
| PG-VAL-02  | Rolling Metrics section populates with 24h / 7d / 30d bps/day per candidate                                                                                                                 | D-25                   | [ ]     |
| PG-VAL-02  | Default sort of the Metrics table is 30d bps/day descending                                                                                                                                 | UI-SPEC Data Persistence | [ ]   |
| i18n       | Switch locale to zh-TW; every Price-Gap surface renders Chinese copy, no English fallback                                                                                                   | Copywriting Contract   | [ ]     |
| Accessibility | Modals respond to Esc; `role="dialog"` present on both Disable and Re-enable modals (check DevTools → Elements)                                                                         | Accessibility Contract | [ ]     |

## Runbook

1. `make build` (frontend first, then Go binary). Both stages must exit 0.
2. Launch the bot locally: `./arb` (or via systemd on a dev host).
3. Open the dashboard, log in with the bearer token (stored in `localStorage.arb_token`), and navigate to the Price-Gap tab.
4. Walk every row above and check the box as each behavior confirms. File any failing row as a GitHub issue tagged `phase-9 / bug` before continuing.
5. Flip a paper→live cutover by toggling the header pill **in a controlled run** — only in dev/staging. Never flip to live on a production capital host without board approval.
6. Review the Telegram channel for the expected entry / exit / risk-block messages. Confirm 📝 PAPER prefix on paper-mode rows.
7. Let the bot run for 5+ minutes with both perp-perp and spot-futures engines active; confirm existing positions are unaffected and existing Telegram alerts still fire (perp-perp `NotifyAutoEntry`, spot-futures entry/exit, etc.).
8. Sign this file (append `**UAT completed: YYYY-MM-DD by <operator>**` at the bottom) and return to the orchestrator.

## Exit Criteria

Every row in the Requirement Coverage table must be checked. Any unchecked row blocks Phase 9 sign-off and requires a gap-closure plan before the version is shipped to the live-capital host.

## Sign-Off

<!-- Operator appends here after the walkthrough completes -->

---

## UAT Outcome (2026-04-22, Plan 09-08 Task 3)

**Result: PARTIAL — blocked by detector observability gap. Phase 9 NOT signed off.**

The operator began UAT against a live dev host with 4 candidates loaded (SOON, RAVE, HOLO, SIGN — all also in the funding-rate scanner's subscription universe) and thresholds set aggressively low (20 bps) for observability. `pg-admin status` confirmed the tracker was healthy and loaded; Redis showed no active positions; `journalctl -u arb` showed zero `pricegap: opened` or `pricegap: gate-blocked` lines in 10+ minutes of runtime. No paper trades fired, so every UAT row that depends on a live detection was unreachable.

### Root-Cause Gaps (tracked for Phase 9.1)

- **Gap #1** — silent detector non-fire in `internal/pricegaptrader/tracker.go:287-289`: `if !det.Fired { continue }` without logging `det.Reason`, making the detector a black box in production.
- **Gap #2** — no BBO subscription path for pricegap candidates: `pkg/exchange.SubscribeSymbol` has zero callers in `internal/pricegaptrader/` or `cmd/main.go`; pricegap candidates only get BBO "for free" when they overlap the funding-rate scanner's universe.

Together these explain why UAT stalls and why the operator cannot diagnose without code changes.

### Row-by-Row Status

| Row | Status | Note |
|-----|--------|------|
| PG-OPS-01 nav placement | ✅ Passed | Tab renders between spot-positions and history |
| PG-OPS-01 Candidates table lists `cfg.PriceGapCandidates` | ✅ Passed | All 4 candidates visible |
| PG-OPS-01 Disable modal + Redis write | ⏸ Not walked | Deferred — operator chose to stop UAT after observability gap surfaced |
| PG-OPS-01 Re-enable modal + Redis key clear | ⏸ Not walked | Deferred |
| PG-OPS-01 pg-admin ↔ dashboard parity | ⏸ Not walked | Deferred |
| PG-OPS-02 Live Positions table populates | 🛑 BLOCKED | Gap #1 + #2 — no detection fires |
| PG-OPS-02 WS `pg_positions` frames | 🛑 BLOCKED | No active positions to broadcast |
| PG-OPS-02 PAPER / LIVE badge colors | ⏸ Not walked | Cannot visually confirm without a row |
| PG-OPS-03 Closed Log Modeled/Realized/Delta columns | 🛑 BLOCKED | No trades to populate the log |
| PG-OPS-03 "Load 100 more" pagination | ⏸ Not walked | Empty history |
| PG-OPS-04 Header toggle flips pill + persists | ✅ Passed | POST `/api/config` writes flag, `.bak` created |
| PG-OPS-04 Mid-life flip preserves Mode | 🟨 Proven in test, not walked | `TestPaperToLiveCutover_NoOrphans` green; live walkthrough blocked |
| PG-OPS-04 Default PAPER pill on missing key | ✅ Passed | Fresh boot without `price_gap.paper_mode` shows violet |
| PG-OPS-05 Entry alert with 📝 PAPER prefix | 🛑 BLOCKED | No entry to trigger |
| PG-OPS-05 Exit alert with reason/PnL/slip/hold | 🛑 BLOCKED | No exit to trigger |
| PG-OPS-05 Risk-block cooldown | 🛑 BLOCKED | No block to trigger |
| PG-OPS-05 Perp-perp NotifyAutoEntry regression | 🟨 Proven in test, not walked | Plan 04 golden-string regression green |
| PG-VAL-01 Realized > 0 on paper row | 🟨 Proven in test, not walked | `TestPaperToLiveCutover_ClosedLogShape` asserts this |
| PG-VAL-02 Rolling Metrics 24h/7d/30d | 🛑 BLOCKED | No history to aggregate |
| PG-VAL-02 Default sort 30d desc | ⏸ Not walked | Table empty |
| i18n zh-TW coverage | ✅ Passed | Locale switch, no English fallback observed on rendered panels |
| Accessibility (Esc close, role=dialog) | ⏸ Not walked | Modals blocked by no-detection cascade |

Legend: ✅ passed / ⏸ deferred (not reached) / 🛑 blocked by observability gap / 🟨 proven in automated tests but not walked live

### Next Steps

1. Phase 9 verifier runs and is expected to return `gaps_found` referencing this outcome.
2. Phase 9.1 drafts fixes for Gap #1 + Gap #2.
3. UAT resumes after 9.1 ships, walking all 🛑 and ⏸ rows.
4. No live-capital deployment of 0.34.0 until the full walkthrough signs off.

