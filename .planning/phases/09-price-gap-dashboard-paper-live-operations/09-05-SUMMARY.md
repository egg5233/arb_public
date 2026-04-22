---
phase: 09-price-gap-dashboard-paper-live-operations
plan: 05
subsystem: pricegaptrader+api
tags: [phase-9, metrics, aggregator, d-24, pg-val-02, pure-function]
requires:
  - 09-01 (GetPriceGapHistory reader, PriceGapPosition D-23 write-path for Modeled/RealizedSlipBps)
  - 09-02 (pricegap_handlers.go baseline + /state + /metrics stub routes)
  - 09-03 (paper-mode writes Mode-tagged rows to pg:history — aggregator tolerates mode-mixed input)
provides:
  - pricegaptrader.CandidateMetrics value type
  - pricegaptrader.ComputeCandidateMetrics pure aggregator (no I/O, caller-supplied clock)
  - GET /api/pricegap/metrics — real impl replacing Plan 02 stub
  - GET /api/pricegap/state — metrics field now populated from same computer
  - padWithConfigCandidates handler helper (zero-activity rows for configured candidates)
affects:
  - UI Wave 3/4 plans consuming /api/pricegap/metrics and /api/pricegap/state.metrics
tech-stack:
  added: []
  patterns:
    - "on-demand rolling-window aggregation (pattern 4)"
    - "pure function with caller-supplied clock for testability"
    - "single data source (pg:history) — D-24 simplification, no pg:slippage:* read"
    - "config-list padding: handler backfills zero-rows so UI always renders a row per configured pair"
key-files:
  created:
    - internal/pricegaptrader/metrics.go
    - internal/pricegaptrader/metrics_test.go
  modified:
    - internal/api/pricegap_handlers.go
    - internal/api/pricegap_handlers_test.go
decisions:
  - Windows are CUMULATIVE (`age <= 24h` counts in 24h AND 7d AND 30d buckets), not exclusive — matches UI-SPEC intent where "last 24h" is a subset of "last 7d".
  - Window membership uses `<=` (inclusive on the far edge) so a trade at exactly 7 days old counts in the 7d bucket. This gives predictable behavior for the boundary tests and avoids a fencepost footgun when cron-scheduled trades land on the edge.
  - Pure function emits NO row for candidates with zero trades. Padding is the HANDLER's job so library callers that want "only real data" do not have to filter.
  - Aggregator sorts desc by Bps30dPerDay with stable tiebreak on Candidate key; handler re-sorts after padding so populated rows outrank zero-rows.
  - Handler reads the full pg:history window (limit=500, the Phase 8 cap) rather than the shorter default (100) — metrics over 30 days require deeper history.
  - `computeMetricsForResponse` helper is shared by /state and /metrics so the two payloads cannot drift.
  - Test `TestHandlePriceGapMetrics_Sorted30dDesc` uses three DIFFERENT exchange pairs on the same symbol (not three symbols) because cfg-padding would otherwise inject zero-bps rows that beat the seeded low-bps trade in the desc sort. This also exercises the candidate-key disambiguation path.
metrics:
  duration: ~35m
  tasks_completed: 2
  files_changed: 4
  commits: 4
  tests_added: 15 (11 pure + 4 handler)
  completed: "2026-04-22"
---

# Phase 09 Plan 05: Rolling Metrics Aggregator Summary

**One-liner:** PG-VAL-02 on-demand rolling metrics aggregator — pure function over pg:history, padded handler wiring for /state and /metrics, zero new Redis write paths (D-24 honored).

## Commits

| Task | Name | Commit | Files |
| ---- | ---- | ------ | ----- |
| 1 (RED) | Failing tests for ComputeCandidateMetrics | `edc62d0` | metrics_test.go |
| 1 (GREEN) | ComputeCandidateMetrics pure aggregator | `66bd31f` | metrics.go |
| 2 (RED) | Failing handler tests (metrics + state wiring) | `c9fb54d` | pricegap_handlers_test.go |
| 2 (GREEN) | Wire aggregator into /state + /metrics | `75cc9c5` | pricegap_handlers.go |

## CandidateMetrics Schema

```go
type CandidateMetrics struct {
    Candidate      string  `json:"candidate"`       // "<symbol>_<long>_<short>"
    Symbol         string  `json:"symbol"`
    LongExchange   string  `json:"long_exchange"`
    ShortExchange  string  `json:"short_exchange"`
    TradesWindow   int     `json:"trades_window"`   // count within 30d (widest)
    WinPct         float64 `json:"win_pct"`         // 0..100, relative to 30d trades
    AvgRealizedBps float64 `json:"avg_realized_bps"`
    Bps24hPerDay   float64 `json:"bps_24h_per_day"`
    Bps7dPerDay    float64 `json:"bps_7d_per_day"`
    Bps30dPerDay   float64 `json:"bps_30d_per_day"`
}
```

## Window-Boundary Semantics

- Age of a trade = `now.Sub(pos.ClosedAt)` where `now` is passed by the caller.
- Window membership is `age <= window` (inclusive).
- Buckets are CUMULATIVE — a 1-hour-old trade contributes to 24h, 7d, and 30d.
- Trades with `age > 30d`, `age < 0`, `NotionalUSDT == 0`, or `ClosedAt.IsZero()` are skipped (T-09-24 divide-by-zero guard).
- `BpsNPerDay = sum(trade_bps in window) / N_days` where `trade_bps = RealizedPnL / NotionalUSDT * 10_000`.
- `WinPct = wins / TradesWindow * 100` where a win is `RealizedPnL > 0`. Denominator is 30d total, so WinPct is the 30-day win rate.
- Output sorted desc by `Bps30dPerDay`; stable tiebreak on `Candidate` key.

## Padding Behavior (Handler Side)

`padWithConfigCandidates(computed, cfg.PriceGapCandidates)`:

1. Start with the aggregator output (only candidates with ≥1 trade).
2. Build the set of present keys.
3. For every configured candidate whose key `Symbol_LongExch_ShortExch` is missing, append a zero-valued `CandidateMetrics` row.
4. Re-sort the combined slice desc by `Bps30dPerDay` (zero-rows drift to the bottom).

Rationale: UI-SPEC Rolling Metrics table always renders a row per configured pair so operators see "this pair hasn't traded yet" explicitly, not a gap.

Shared helper `(*Server).computeMetricsForResponse()` ensures `/state.metrics` and `/metrics` return the same payload — guarantees the two endpoints cannot drift.

## D-24 Confirmation — pg:slippage:* Not Read

- `grep -q "pg:slippage" internal/pricegaptrader/metrics.go` → no match.
- `grep -q "pg:slippage" internal/api/pricegap_handlers.go` → no match.
- The aggregator reads only fields on `*models.PriceGapPosition`: `Symbol`, `LongExchange`, `ShortExchange`, `ClosedAt`, `NotionalUSDT`, `RealizedPnL`. `ModeledSlipBps` / `RealizedSlipBps` are documented as future-column candidates in the package doc but not consumed by the current rows.
- Test `TestComputeCandidateMetrics_HistoryOnlyDataSource` builds fixtures where every row carries non-zero `ModeledSlipBps` + `RealizedSlipBps` + `RealizedPnL` + `NotionalUSDT` + `ClosedAt` and asserts `Bps7dPerDay` is non-zero and equals `sum/7` — proves pg:history alone suffices.

## Security / Threat Mitigations

| Threat ID | Mitigation                                                 | Test                                                 |
|-----------|------------------------------------------------------------|------------------------------------------------------|
| T-09-22   | History read capped at Phase 8 D-14 limit (500 rows)       | Handler calls `GetPriceGapHistory(0, priceGapHistoryLimitMax)` |
| T-09-23   | No PII/secret leakage — metrics are bps-normalized only    | Bearer-gated via `cors+authMiddleware` chain (Plan 02 pattern) |
| T-09-24   | Divide-by-zero on zero notional                            | `TestComputeCandidateMetrics_NoNotionalNoPanic`      |

## Tests Added (15 total)

### `internal/pricegaptrader/metrics_test.go` (11)
- `TestComputeCandidateMetrics_Empty` — nil and empty inputs both return non-nil empty slice.
- `TestComputeCandidateMetrics_SingleCandidate` — 5 trades, WinPct=80, AvgRealizedBps=9.6.
- `TestComputeCandidateMetrics_WindowBoundary_24h` — -1h/-12h/-25h → 24h bucket has 2, 7d has 3.
- `TestComputeCandidateMetrics_WindowBoundary_7d` — -6d23h in, -7d1h out.
- `TestComputeCandidateMetrics_WindowBoundary_30d` — -29d in, -31d out; TradesWindow=1.
- `TestComputeCandidateMetrics_BpsPerDayFormula` — 140 bps / 7 = 20.
- `TestComputeCandidateMetrics_MultipleCandidates` — 3 symbols → 3 rows, desc sorted.
- `TestComputeCandidateMetrics_CandidateKey` — same symbol, different exchange pairs → 2 distinct rows.
- `TestComputeCandidateMetrics_NoNotionalNoPanic` — NotionalUSDT=0 skipped cleanly.
- `TestComputeCandidateMetrics_ZeroTradesRow` — pure fn emits NO row when a candidate has zero trades (handler's job to pad).
- `TestComputeCandidateMetrics_HistoryOnlyDataSource` — proves D-24 (pg:history alone is sufficient).

### `internal/api/pricegap_handlers_test.go` (4)
- `TestHandlePriceGapMetrics_Empty` — empty history padded with 2 zero-rows from cfg.PriceGapCandidates.
- `TestHandlePriceGapMetrics_Sorted30dDesc` — three seeded pairs, strict desc order.
- `TestHandlePriceGapMetrics_PaddedForConfigCandidates` — BTC real + ETH/SOL padded, zero TradesWindow on padded rows.
- `TestHandlePriceGapState_IncludesMetrics` — /state.metrics populated, BTC row has non-zero Bps24hPerDay.

## Deviations from Plan

### 1. [Rule 3 — Blocking] Comments referencing `time.Now()` / `pg:slippage:*` rewritten to satisfy acceptance greps

- **Found during:** Task 1 acceptance-grep pass.
- **Issue:** Acceptance criteria used `! grep -q "time.Now()"` and `! grep -q "pg:slippage"` — the greps match strings in comments as well as code. Initial doc comments referenced "no time.Now()" and "no pg:slippage:* read" to communicate intent.
- **Fix:** Rephrased the same intent without using the sentinel strings — "no wall clock", "`now` is a parameter", "no secondary store read", etc. Semantically identical; the greps now pass.
- **Files modified:** `internal/pricegaptrader/metrics.go`, `internal/api/pricegap_handlers.go`.
- **Commits:** `66bd31f`, `75cc9c5`.

### 2. [Adjustment — Not a deviation] Test for desc sort uses exchange-pair disambiguation

- **Detail:** Plan behavior wording suggested three symbols with distinct Bps30d. Using three symbols while the test server has `BTCUSDT` + `SOONUSDT` in cfg.PriceGapCandidates would force padding rows to also be emitted, and (depending on values) could reorder the top. Switching the test to three different exchange pairs on `BTCUSDT` removes the padding-induced noise AND exercises the candidate-key disambiguation requirement in the same test.
- **Result:** Same assertion intent (strict desc order by Bps30dPerDay); cleaner coverage.

### 3. [Plan wording — Not a deviation] `time` import stayed in metrics.go

- **Detail:** The function signature takes `time.Time`, so the `time` package is imported. Acceptance criteria only forbid `time.Now()`, not the `time` package itself.

### Out-of-scope
None encountered.

### Auth Gates
None.

## Verification

- `go build ./...` — exits 0
- `go vet ./...` — no issues
- `go test ./internal/pricegaptrader/... ./internal/api/... -count=1 -race` — 127 tests pass
- `grep -q "func ComputeCandidateMetrics" internal/pricegaptrader/metrics.go` → match
- `grep -q "CandidateMetrics struct" internal/pricegaptrader/metrics.go` → match
- `! grep -q "time.Now()" internal/pricegaptrader/metrics.go` → OK
- `! grep -q "redis\." internal/pricegaptrader/metrics.go` → OK
- `! grep -q "pg:slippage" internal/pricegaptrader/metrics.go` → OK
- `! grep -qE "internal/(api|engine|spotengine|database)" internal/pricegaptrader/metrics.go` → OK (module boundary preserved)
- `grep -q "pricegaptrader.ComputeCandidateMetrics" internal/api/pricegap_handlers.go` → match
- `grep -q "padWithConfigCandidates" internal/api/pricegap_handlers.go` → match
- `grep -q "GetPriceGapHistory" internal/api/pricegap_handlers.go` → match
- `! grep -q "pg:slippage" internal/api/pricegap_handlers.go` → OK
- `git status config.json` → unchanged (CLAUDE.local.md invariant respected)

## Self-Check: PASSED

- FOUND: internal/pricegaptrader/metrics.go
- FOUND: internal/pricegaptrader/metrics_test.go
- FOUND: internal/api/pricegap_handlers.go (modified — ComputeCandidateMetrics call + padWithConfigCandidates)
- FOUND: internal/api/pricegap_handlers_test.go (4 new tests)
- FOUND commit: edc62d0 (Task 1 RED)
- FOUND commit: 66bd31f (Task 1 GREEN)
- FOUND commit: c9fb54d (Task 2 RED)
- FOUND commit: 75cc9c5 (Task 2 GREEN)
