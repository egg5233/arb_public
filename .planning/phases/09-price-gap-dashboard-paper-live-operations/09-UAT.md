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
