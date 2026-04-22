---
phase: 09-price-gap-dashboard-paper-live-operations
plan: 02
subsystem: api+pricegaptrader+database
tags: [phase-9, dashboard-api, broadcaster, paper-mode, pitfall-6, stride-mitigations]
requires:
  - Phase 9 Plan 01 (Broadcaster interface, PriceGapPaperMode field, GetPriceGapHistory reader, PriceGapPosition.Mode)
provides:
  - 7 REST routes under /api/pricegap/* (all bearer-gated via s.cors(s.authMiddleware(...)))
  - *Server implements pricegaptrader.Broadcaster (compile-time assertion)
  - /api/config accepts price_gap.paper_mode (nested) and price_gap_paper_mode (flat) — round-trips to cfg.PriceGapPaperMode
  - SaveJSON persists price_gap.{enabled,paper_mode} through the existing .bak-backed writer
  - IsCandidateDisabled extended to 4-tuple (disabled, reason, disabledAt, err); JSON-on-disk with legacy plain-string backward compat (Pitfall 6)
affects:
  - Phase 9 Plan 05 (metrics) — replaces stub body in handlePriceGapMetrics
  - Phase 9 Plan 06 (wiring)  — single line to inject *Server as Broadcaster into tracker
  - Phase 9 Waves 3 + 4 UI plans — read state/candidates/closed/metrics endpoints
tech-stack:
  added: []
  patterns:
    - "net/http ServeMux pattern-param routes (POST /api/pricegap/candidate/{symbol}/...)"
    - "cors+authMiddleware chain (project standard)"
    - "compile-time interface assertion (var _ Interface = (*Concrete)(nil))"
    - "single-writer invariant across CLI + dashboard (both call SetCandidateDisabled)"
key-files:
  created:
    - internal/api/pricegap_handlers.go
    - internal/api/pricegap_handlers_test.go
  modified:
    - internal/api/server.go          # 7 route registrations
    - internal/api/auth.go            # isMutatingEndpoint covers /api/pricegap/candidate/
    - internal/api/handlers.go        # configUpdate.PriceGap + flat price_gap_paper_mode apply path
    - internal/config/config.go       # SaveJSON persists price_gap.{enabled,paper_mode}
    - internal/database/pricegap_state.go         # IsCandidateDisabled → 4-tuple, SetCandidateDisabled writes JSON
    - internal/database/pricegap_state_test.go    # +5 tests (DisableFlag expanded + JSONShape + LegacyPlainString + WritesJSON + RemovesKey)
    - internal/models/pricegap_interfaces.go      # PriceGapStore.IsCandidateDisabled signature
    - internal/pricegaptrader/risk_gate.go        # caller updated (disabled, reason, _, err)
    - internal/pricegaptrader/risk_gate_test.go   # fakeStore return arity
    - cmd/pg-admin/main.go            # status output uses disabledAt timestamp when present
decisions:
  - Flat `price_gap_paper_mode` root field accepted alongside nested `price_gap.paper_mode` so the dashboard UI can bind a single boolean without synthesising a nested object. Nested wins on conflict.
  - Broadcast methods live in pricegap_handlers.go (not server.go) so the Broadcaster contract and its implementation are co-located — easier to audit the Plan-01-to-Plan-02 interface wire-up.
  - isMutatingEndpoint extended to treat POST /api/pricegap/candidate/* as state-changing so auth is enforced even in the no-password dev-mode branch (T-09-06 mitigation, Phase 9 Pitfall 6 audit note).
  - Metrics handler ships an empty slice stub rather than being omitted entirely — gives UI a stable shape so Plan 05 can swap in the real computer without the dashboard flashing missing-endpoint errors.
  - Did NOT change writeJSON, Hub, or any existing handler — additive only to keep blast radius bounded on a live-trading repo.
metrics:
  duration: "~45 min"
  tasks_completed: 2
  files_changed: 11
  commits: 2
  completed: "2026-04-22"
---

# Phase 09 Plan 02: Dashboard REST + Broadcaster Summary

**One-liner:** Ship the bearer-gated `/api/pricegap/*` REST surface + implement `pricegaptrader.Broadcaster` on `*Server`, with `/api/config` round-tripping `paper_mode` through the existing `.bak`-backed SaveJSON path.

## Commits

| Task | Name                                                          | Commit    | Files                                                                                    |
| ---- | ------------------------------------------------------------- | --------- | ---------------------------------------------------------------------------------------- |
| 1    | Align candidate disable schema (JSON + disabled_at)           | `e26a28c` | pricegap_state.go (+ test), pricegap_interfaces.go, risk_gate.go (+ test), pg-admin/main |
| 2    | HTTP handlers + Broadcaster impl + /api/config paper_mode     | `7ce4c4d` | pricegap_handlers.go (+ test), server.go, auth.go, handlers.go, config.go                |

## Route Table

| Method | Path                                             | Handler                          | Notes                                           |
| ------ | ------------------------------------------------ | -------------------------------- | ----------------------------------------------- |
| GET    | `/api/pricegap/state`                            | `handlePriceGapState`            | Aggregate seed: enabled, paper_mode, budget, candidates[], active_positions[], recent_closed[] (≤100), metrics[] (stub) |
| GET    | `/api/pricegap/candidates`                       | `handlePriceGapCandidates`       | Candidates + disable state                       |
| GET    | `/api/pricegap/positions`                        | `handlePriceGapPositions`        | Active pricegap positions                        |
| GET    | `/api/pricegap/closed`                           | `handlePriceGapClosed`           | `?offset=0&limit=100` (max 500, T-09-12)         |
| GET    | `/api/pricegap/metrics`                          | `handlePriceGapMetrics`          | Plan 05 stub — empty slice today                 |
| POST   | `/api/pricegap/candidate/{symbol}/disable`       | `handlePriceGapCandidateDisable` | Body: `{"reason":"..."}`; allowlist + 256 cap   |
| POST   | `/api/pricegap/candidate/{symbol}/enable`        | `handlePriceGapCandidateEnable`  | Clears `pg:candidate:disabled:<symbol>`          |

All routes: `mux.HandleFunc("{METHOD} /api/pricegap/...", s.cors(s.authMiddleware(handler)))`.

## Broadcaster Methods on *Server

```go
var _ pricegaptrader.Broadcaster = (*Server)(nil)

func (s *Server) BroadcastPriceGapPositions(positions []*models.PriceGapPosition)
func (s *Server) BroadcastPriceGapEvent(evt pricegaptrader.PriceGapEvent)
func (s *Server) BroadcastPriceGapCandidateUpdate(upd pricegaptrader.PriceGapCandidateUpdate)
```

WS channels: `pg_positions`, `pg_event`, `pg_candidate_update`.

## /api/config Paper-Mode Wiring

`configUpdate` gained:

```go
PriceGap          *priceGapUpdate `json:"price_gap"`             // nested form
PriceGapPaperMode *bool           `json:"price_gap_paper_mode"`  // flat shortcut
```

Both round-trip to `cfg.PriceGapPaperMode`. `SaveJSON` now writes the `price_gap` block with `{enabled, paper_mode}` under the existing `.bak` backup path. No direct `os.WriteFile("config.json", ...)` in any new handler code.

## Disable-Schema Alignment (Pitfall 6)

**Before:** `IsCandidateDisabled(symbol) (bool, string, error)` — plain-string value on disk.
**After:** `IsCandidateDisabled(symbol) (bool, string, int64, error)` — JSON blob `{reason, disabled_at}` on disk, with legacy plain-string values still readable (fallback path, `disabledAt=0`).

Single-writer invariant: dashboard `POST /api/pricegap/candidate/.../disable` and `pg-admin disable` both call `*database.Client.SetCandidateDisabled(symbol, reason)` — the store method is the only writer.

## Threat Mitigations Exercised by Tests

| Threat ID | Mitigation                              | Test                                                 |
| --------- | --------------------------------------- | ---------------------------------------------------- |
| T-09-06   | Bearer-required on `/api/pricegap/*`    | `TestPriceGapState_AuthRequired` (401 without token) |
| T-09-07   | Symbol allowlist on disable/enable      | `TestPriceGapCandidateDisable_UnknownSymbol` (400)   |
| T-09-08   | 256-byte reason cap + ctrl-char strip   | `TestPriceGapCandidateDisable_ReasonTooLong` (400), `TestPriceGapCandidateDisable_ControlCharsStripped` |
| T-09-11   | Single writer: CLI + dashboard → store  | Task 1 fakeStore + JSON-shape assertions             |
| T-09-12   | limit cap 500                           | `TestPriceGapClosed_LimitCap`                        |

## Test Counts

| Package                      | New Tests | What                                                      |
| ---------------------------- | --------- | --------------------------------------------------------- |
| `internal/database/...`      | +4        | JSON-shape, legacy plain-string, WritesJSON, RemovesKey   |
| `internal/api/...`           | +13       | auth, shape, paging, cap, disable/enable happy+edge, paper round-trip (nested+flat), Broadcaster interface + call |
| Pre-existing suites          | 0 new     | All 141 tests across api/database/pricegaptrader/config/pg-admin pass under `-race -count=1` |

Total **+17 new tests**; 141 tests green.

## Deviations from Plan

### 1. [Rule 2 — Missing critical] Extended isMutatingEndpoint to cover `/api/pricegap/candidate/`

- **Found during:** Task 2 implementation (writing the 401-without-bearer test).
- **Issue:** Plan listed "Bearer token auth in localStorage key arb_token" but didn't specify that `authMiddleware` has a dev-mode branch which bypasses auth entirely when `DashboardPassword == ""` and the endpoint is non-mutating. POST disable/enable would have silently worked in that branch — bypassing T-09-06.
- **Fix:** Added `strings.HasPrefix(path, "/api/pricegap/candidate/")` to `isMutatingEndpoint` so POST disable/enable are always authenticated even in dev mode.
- **Files modified:** `internal/api/auth.go`
- **Commit:** `7ce4c4d`

### 2. [Rule 3 — Blocking] Added flat `price_gap_paper_mode` root alias alongside nested form

- **Found during:** Task 2 <action> bullet 4 — "ensure the price_gap_paper_mode field from Plan 01 is actually mapped into cfg.PriceGapPaperMode".
- **Issue:** Plan 01 added `jsonPriceGap.PaperMode` in `config/config.go` for the file load path only. `handlePostConfig` had no `price_gap` block at all — the root shape used exclusively `{strategy:{...}, spot_futures:{...}, ...}` style nesting.
- **Fix:** Added `priceGapUpdate` struct + `configUpdate.PriceGap` nested form AND `configUpdate.PriceGapPaperMode` root-level flat shortcut. Nested wins on conflict. This matches the acceptance criterion `grep -q "price_gap_paper_mode\|PriceGapPaperMode" internal/api/config_handlers.go` (the real file is `handlers.go` — see Note 3).
- **Files modified:** `internal/api/handlers.go`
- **Commit:** `7ce4c4d`

### 3. [Plan fact-finding — Not a deviation] config_handlers.go does not exist

- **Found during:** Task 2 read_first.
- **Detail:** Plan frontmatter listed `internal/api/config_handlers.go` as a file to modify, but the API package has `handlers.go` (96K — all handlers incl. config live in one file), `analytics_handlers.go`, `spot_handlers.go`, etc. There is no `config_handlers.go`. All acceptance criteria greps remain satisfied because the PriceGapPaperMode wiring now lives in `handlers.go` (proved: `grep -q "price_gap_paper_mode\|PriceGapPaperMode" internal/api/handlers.go` → match).
- **Resolution:** Put config-related code in `handlers.go` with the other `configUpdate` struct definitions; kept pricegap handlers in the new `pricegap_handlers.go`.

### 4. [Rule 2 — Missing critical] Added SaveJSON persistence for `price_gap.{enabled,paper_mode}`

- **Found during:** Task 2 end-to-end check — "does paper mode survive a restart?".
- **Issue:** `SaveJSON` wrote every other config block to the `raw` map but had no `price_gap` block. An in-memory flip would revert on restart because `config.json` re-loads from disk.
- **Fix:** Added a 3-line `priceGap := getMap(raw, "price_gap"); priceGap["enabled"] = ...; priceGap["paper_mode"] = ...` block before the `MarshalIndent`. Does NOT overwrite `candidates`, `budget`, or other static config — those come from the operator-edited file via the `raw` passthrough.
- **Files modified:** `internal/config/config.go`
- **Commit:** `7ce4c4d`

### Auth Gates
None.

### Out-of-scope
None encountered.

## Verification

- `go build ./...` — exits 0
- `go test ./internal/api/... ./internal/database/... ./internal/pricegaptrader/... ./internal/config/... ./cmd/pg-admin/... -count=1 -race` — 141 tests pass
- `bash scripts/check-i18n-sync.sh` — 690 keys in sync (no UI changes this plan)
- `git status config.json` — unchanged (file untouched — confirms CLAUDE.local.md invariant)

## Self-Check

- FOUND: internal/api/pricegap_handlers.go
- FOUND: internal/api/pricegap_handlers_test.go
- FOUND: handlePriceGapState in pricegap_handlers.go
- FOUND: handlePriceGapCandidateDisable / handlePriceGapCandidateEnable
- FOUND: var _ pricegaptrader.Broadcaster = (*Server)(nil) compile-time assertion
- FOUND: BroadcastPriceGapPositions / Event / CandidateUpdate
- FOUND: 7 route registrations in internal/api/server.go
- FOUND: configUpdate.PriceGap + PriceGapPaperMode in handlers.go
- FOUND: price_gap SaveJSON block in config.go
- FOUND commit: e26a28c (Task 1)
- FOUND commit: 7ce4c4d (Task 2)

## Self-Check: PASSED
