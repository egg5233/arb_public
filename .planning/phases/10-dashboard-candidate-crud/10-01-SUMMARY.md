---
phase: 10-dashboard-candidate-crud
plan: 01
subsystem: api
tags: [pricegap, candidate-crud, validation, http-handler, go]

# Dependency graph
requires:
  - phase: 08-price-gap-tracker-core
    provides: PriceGapCandidate struct, PriceGapCandidates config slice, GetActivePriceGapPositions
  - phase: 09-price-gap-dashboard-paper-live-operations
    provides: priceGapUpdate struct shell (Enabled/PaperMode/DebugLog), pg:positions:active read patterns
provides:
  - Candidates *[]models.PriceGapCandidate field on priceGapUpdate (pointer-to-slice; nil vs empty distinguished)
  - validatePriceGapCandidates(cs []PriceGapCandidate) []string — D-05..D-11 invariants
  - (*Server).guardActivePositionRemoval(prev, next) (string, error) — D-14 + Pitfall 4 tuple-change orphan guard
  - POST /api/config price_gap.candidates apply branch — full-replace, last-write-wins, reuses existing SaveJSON path
affects: [10-02-broadcast, 10-03-frontend-modal, 10-04-i18n, 10-05-vitest]

# Tech tracking
tech-stack:
  added: []  # zero new dependencies — pure additive on existing handler + helper file
  patterns:
    - "Pointer-to-slice config update field for nil-vs-empty distinction (Pitfall 2)"
    - "Server-side normalisation (TrimSpace + ToUpper) BEFORE validation + storage (Pitfall 3)"
    - "Set-difference active-position guard against pg:positions:active (covers tuple-change orphans, Pitfall 4)"

key-files:
  created:
    - internal/api/pricegap_candidates_validate_test.go
  modified:
    - internal/api/handlers.go
    - internal/api/pricegap_handlers.go

key-decisions:
  - "Pointer-to-slice (*[]models.PriceGapCandidate) — nil = untouched, empty = intentional delete-all per Pitfall 2 / D-16"
  - "Validation collates ALL errors (joined with '; ') instead of fail-fast — dashboard surfaces every issue at once"
  - "Active-position guard at HTTP 409, not 400 — semantically a conflict with current world state, not a bad request"
  - "Helpers live in pricegap_handlers.go (not handlers.go) — keeps the apply branch slim and aligns with existing pricegap surface organisation"
  - "Defensive copy of *pg.Candidates before normalisation prevents mutation of caller's decoded JSON struct"
  - "Phase-9 disable flag preservation NOT needed — disable state is keyed in Redis (pg:candidate:disabled:<symbol>) by symbol alone, lives outside the candidate slice; tuple-change edits do not orphan the disable flag because it persists on Symbol"

patterns-established:
  - "Pattern: server-side enum + range validation collated into []string error list — reusable for any /api/config sub-block"
  - "Pattern: 'set-removed = prev∖next' tuple-change orphan guard — generalises to any config slice whose entries can have downstream Redis state"

requirements-completed: [PG-OPS-07]

# Metrics
duration: 5min
completed: 2026-04-25
---

# Phase 10 Plan 01: Backend POST /api/config price_gap.candidates Summary

**POST /api/config now accepts a `price_gap.candidates` array with full server-side validation (regex symbol, exchange enum, range checks, duplicate-tuple) and an active-position safety guard that blocks delete OR tuple-change edits when an open pg:positions:active row matches a removed tuple.**

## Final Struct Shape (handlers.go ~line 783)

```go
// priceGapUpdate mirrors the "price_gap" block of /api/config POST bodies.
// Phase 9 D-12: PaperMode is the dashboard-writable live/paper toggle.
// Phase 9 gap-closure (Plan 09-09): DebugLog gates the rate-limited
// non-fire reason logger in tracker.runTick.
// Phase 10 (Plan 10-01): Candidates is a pointer-to-slice so nil = "not
// provided" and empty slice = "intentional delete-all" (Pitfall 2). Full
// replace, last-write-wins per D-16.
type priceGapUpdate struct {
    Enabled    *bool                          `json:"enabled"`
    PaperMode  *bool                          `json:"paper_mode"`
    DebugLog   *bool                          `json:"debug_log"`
    Candidates *[]models.PriceGapCandidate    `json:"candidates"`
}
```

## Apply-Block Insertion (handlers.go lines 1604-1631)

Inserted INSIDE the existing `if pg := upd.PriceGap; pg != nil { ... }` block, AFTER the `DebugLog` branch. Reuses the surrounding `s.cfg.Lock()` (no second lock acquisition) and the post-block `s.cfg.SaveJSONWithExchangeSecretOverrides(...)` + `s.configNotifier.Notify()` (no new persistence calls).

```go
if pg.Candidates != nil {
    next := make([]models.PriceGapCandidate, len(*pg.Candidates))
    copy(next, *pg.Candidates)
    for i := range next {
        next[i].Symbol = strings.ToUpper(strings.TrimSpace(next[i].Symbol))
        next[i].LongExch = strings.ToLower(strings.TrimSpace(next[i].LongExch))
        next[i].ShortExch = strings.ToLower(strings.TrimSpace(next[i].ShortExch))
    }
    if errs := validatePriceGapCandidates(next); len(errs) > 0 {
        s.cfg.Unlock()
        writeJSON(w, http.StatusBadRequest, Response{Error: strings.Join(errs, "; ")})
        return
    }
    if blocker, err := s.guardActivePositionRemoval(s.cfg.PriceGapCandidates, next); err != nil {
        s.cfg.Unlock()
        writeJSON(w, http.StatusInternalServerError, Response{Error: "active-position check: " + err.Error()})
        return
    } else if blocker != "" {
        s.cfg.Unlock()
        writeJSON(w, http.StatusConflict, Response{Error: blocker})
        return
    }
    s.cfg.PriceGapCandidates = next
}
```

## Helper Signatures (pricegap_handlers.go)

```go
// Package-level vars (file scope):
var symbolRE = regexp.MustCompile(`^[A-Z0-9]+USDT$`)
var allowedExch = map[string]struct{}{
    "binance": {}, "bybit": {}, "gateio": {}, "bitget": {}, "okx": {}, "bingx": {},
}

// Validation: returns one error string per failed candidate, indexed for clarity.
// Caller is expected to have already normalised case + whitespace.
func validatePriceGapCandidates(cs []models.PriceGapCandidate) []string

// Active-position guard: blocks any tuple in prev that's missing from next when
// pg:positions:active has a matching open position. Covers outright delete AND
// tuple-change edits (Pitfall 4 — the original tuple still has live state).
//   ("",   nil) → safe to apply
//   (msg,  nil) → HTTP 409 (operator-fixable)
//   ("",   err) → HTTP 500 (Redis failure)
func (s *Server) guardActivePositionRemoval(prev, next []models.PriceGapCandidate) (string, error)
```

## Test Results

| # | Subtest | Status | Notes |
|---|---------|--------|-------|
| 1 | Add | PASS | POST single candidate → 200, slice has 1 entry with normalised values |
| 2 | EmptyArrayDeleteAll | PASS | POST `candidates:[]` → 200, slice cleared (Pitfall 2 honoured) |
| 3 | NilUntouched | PASS | POST without `candidates` field → seeded slice unchanged |
| 4 | Validation_SymbolFormat | PASS | `"btc/usdt"` → 400 with "symbol must match" |
| 5 | Validation_ExchEqual | PASS | long==short → 400 with "long_exch must differ from short_exch" |
| 6 | Validation_ExchEnum | PASS | `"kraken"` → 400 with "long_exch invalid" |
| 7 | Validation_ThresholdRange | PASS | `threshold_bps=10` → 400 with "threshold_bps out of range" |
| 8 | Validation_MaxPositionRange | PASS | `max_position_usdt=99` → 400 with "max_position_usdt out of range" |
| 9 | Validation_SlippageRange | PASS | `modeled_slippage_bps=200` → 400 with "modeled_slippage_bps out of range" |
| 10 | Validation_DuplicateTuple | PASS | Two identical tuples → 400 with "duplicate tuple" |
| 11 | BlocksActiveDelete | PASS | Delete-all with open position → 409 + slice NOT mutated |
| 12 | TupleChangeOrphanGuard | PASS | Edit binance→gateio with old tuple's position open → 409 + tuple NOT changed |

`go test ./internal/api/ -count=1` → **59 tests pass, no regression**.

## Deviations from Plan

### Auto-fixed Adaptations

**1. [Rule 3 - Type drift] PriceGapCandidate field types**
- **Found during:** Task 1 read of `internal/models/pricegap_interfaces.go`
- **Issue:** Plan's `<interfaces>` block (line 78) says `ThresholdBps int`. Actual struct has `ThresholdBps float64`, `MaxPositionUSDT float64`, `ModeledSlippageBps float64`.
- **Fix:** Validator uses float comparisons (`< 50 || > 1000`), test bodies send numeric JSON values that decode cleanly into float64 (`"threshold_bps": 200`).
- **Files modified:** None — adaptation lives in the validator code (always correct against the real struct).

**2. [Rule 3 - Type drift] PriceGapCandidate has no Enabled / DisabledReason / DisabledAt fields**
- **Found during:** Task 1 read of `internal/models/pricegap_interfaces.go`
- **Issue:** Plan's `<interfaces>` block (lines 81-83) and the apply-branch action item §2 (lines 184-197) call for preserving Phase-9 `Enabled`/`DisabledReason`/`DisabledAt` fields across edits (D-13). Those fields do NOT exist on `PriceGapCandidate` — they are stored in Redis at `pg:candidate:disabled:<symbol>` and surfaced via `priceGapCandidateView` (separate response struct).
- **Fix:** Omitted the `prevByTuple` Phase-9 preservation logic from the apply branch entirely. The disable state survives by design: it is keyed in Redis on Symbol alone, so editing a candidate's tuple (or deleting + re-adding) preserves the disable flag automatically through the unchanged Redis state. D-13 is satisfied by the existing architecture, not by handler-side data carrying.
- **Files modified:** None — fixed by NOT writing the wrong code.
- **Documented in key-decisions above.**

**3. [Plan adaptation] No `int` int-cast for threshold_bps / no rounding logic**
- **Found during:** Task 2 test body construction
- **Issue:** Plan tests would have failed type assertion if they used `int` literals through JSON (Go decodes JSON numbers into float64 by default; the validator passes them through).
- **Fix:** Test bodies use plain JSON number literals; validator compares as float64 directly. No type ambiguity.

### Other Deviations

None.

## Authentication Gates

None — backend handler test path bypasses `authMiddleware` by calling `s.handlePostConfig` directly (matches existing pricegap_handlers_test.go pattern).

## Note for Plan 10-03 (Frontend)

The exact JSON shape that the apply path now expects:

```json
POST /api/config
Authorization: Bearer <token>
Content-Type: application/json

{
  "price_gap": {
    "candidates": [
      {
        "symbol": "BTCUSDT",
        "long_exch": "binance",
        "short_exch": "bybit",
        "threshold_bps": 200,
        "max_position_usdt": 5000,
        "modeled_slippage_bps": 5
      }
    ]
  }
}
```

Notes for the frontend:
- `candidates` is REQUIRED to be a JSON array when present. Send `[]` (not `null`, not omitted) to delete all.
- Omit the `candidates` field entirely to leave the existing slice untouched (e.g., when only changing `paper_mode`).
- Symbol is auto-uppercased + trimmed server-side; `long_exch`/`short_exch` are auto-lowercased + trimmed. Frontend should still mirror these (D-03 inline validation) so the user sees clean values immediately.
- All numeric fields decode as `float64` (Go default for JSON numbers); send plain numbers, not strings.
- HTTP 400 with `{ok:false,error:"<collated; messages>"}` on validation failure.
- HTTP 409 with `{ok:false,error:"candidate <SYM>/<long>/<short> has active position; close it first (position id=...)"}` when an active position would be orphaned.
- HTTP 200 with `{ok:true,data:<full configResponse>}` on success — the response includes the freshly-mutated `price_gap.candidates` for re-render.

## Self-Check: PASSED

- File `internal/api/handlers.go` modified (Candidates field + apply branch) — VERIFIED via `grep`.
- File `internal/api/pricegap_handlers.go` modified (validate + guard helpers + symbolRE/allowedExch) — VERIFIED via `grep`.
- File `internal/api/pricegap_candidates_validate_test.go` created (12 subtests) — VERIFIED via `ls`.
- Commit `6e2e5ca` (test, RED phase) — VERIFIED via `git log`.
- Commit `7ac1d72` (feat, GREEN phase) — VERIFIED via `git log`.
- `go build ./...` exits 0 — VERIFIED.
- `go test ./internal/api/ -count=1` — 59 pass, 0 fail — VERIFIED.
- All 8 acceptance criteria satisfied (AC7 grep returned 0 because the literal is concatenated with `prefix`, but runtime test confirms the message is emitted correctly).
