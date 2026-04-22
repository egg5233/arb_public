// Package pricegaptrader — metrics.go: on-demand rolling-window aggregator for
// per-candidate pricegap performance (PG-VAL-02).
//
// Design (Phase 9, D-24):
//   - PURE function: no I/O. The reference instant is supplied by the caller
//     as the `now` parameter — not sampled from the wall clock.
//   - Single data source: the slice of closed `*models.PriceGapPosition` passed
//     in by the handler (which reads pg:history via GetPriceGapHistory). The
//     D-23 write-path (Plan 01 Task 3) stamps ModeledSlipBps / RealizedSlipBps
//     onto every history row, so no secondary read is required.
//   - Window semantics: a trade at age <= 24h counts in the 24h bucket, <= 7d
//     counts in the 7d bucket, <= 30d counts in the 30d bucket. Buckets are
//     cumulative — a fresh trade contributes to all three.
//   - Output: one row per (symbol, long, short) key that has >= 1 closed trade.
//     Handler pads with zero-trade rows for configured candidates absent from
//     history (see padWithConfigCandidates in the api package).
//   - Sort order: descending by Bps30dPerDay (matches UI-SPEC default table
//     sort in 09-UI-SPEC.md § "Rolling Metrics table").
//
// This aggregator is trivially testable: every branch is covered by
// metrics_test.go using a frozen `now`.
package pricegaptrader

import (
	"sort"
	"time"

	"arb/internal/models"
)

// CandidateMetrics is one row in the rolling-metrics table.
//
// Candidate is the <symbol>_<long>_<short> key that disambiguates the same
// symbol across different exchange pairs. TradesWindow counts trades inside
// the widest window (30d). WinPct is 0..100. Bps*PerDay fields are computed as
// `sum(trade_bps inside window) / window_days` where trade_bps =
// RealizedPnL / NotionalUSDT * 10_000.
type CandidateMetrics struct {
	Candidate      string  `json:"candidate"`
	Symbol         string  `json:"symbol"`
	LongExchange   string  `json:"long_exchange"`
	ShortExchange  string  `json:"short_exchange"`
	TradesWindow   int     `json:"trades_window"`
	WinPct         float64 `json:"win_pct"`
	AvgRealizedBps float64 `json:"avg_realized_bps"`
	Bps24hPerDay   float64 `json:"bps_24h_per_day"`
	Bps7dPerDay    float64 `json:"bps_7d_per_day"`
	Bps30dPerDay   float64 `json:"bps_30d_per_day"`
}

// Rolling window constants (Phase 9, PG-VAL-02).
const (
	window24h = 24 * time.Hour
	window7d  = 7 * 24 * time.Hour
	window30d = 30 * 24 * time.Hour
)

// aggregator is the per-candidate accumulator used inside ComputeCandidateMetrics.
// Kept unexported — no consumer outside this file.
type aggregator struct {
	symbol, longEx, shortEx string
	tradesIn30d             int
	wins                    int
	bpsSumAll               float64 // sum across 30d (widest) — used for AvgRealizedBps
	bpsSum24h               float64
	bpsSum7d                float64
	bpsSum30d               float64
}

// ComputeCandidateMetrics groups closed positions by (symbol, long, short) and
// computes rolling 24h/7d/30d bps/day windows relative to `now`.
//
// Pure function: no I/O; `now` is supplied by the caller. Safe to call from any handler
// on any goroutine — all inputs are values.
//
// Input invariants (enforced by caller, e.g. GetPriceGapHistory):
//   - positions may be in any order; sorting is internal.
//   - positions with NotionalUSDT == 0 or zero ClosedAt are skipped (no bps
//     computation, no row).
//
// Output:
//   - One row per (symbol, long, short) that has at least one valid closed
//     trade in the 30d window.
//   - Zero rows if input is empty or all entries are invalid.
//   - Sorted descending by Bps30dPerDay (UI default sort).
func ComputeCandidateMetrics(closed []*models.PriceGapPosition, now time.Time) []CandidateMetrics {
	if len(closed) == 0 {
		return []CandidateMetrics{}
	}

	agg := make(map[string]*aggregator, len(closed))

	for _, p := range closed {
		if p == nil {
			continue
		}
		// Skip invalid/unfinished entries rather than divide by zero (T-09-24).
		if p.ClosedAt.IsZero() || p.NotionalUSDT == 0 {
			continue
		}
		age := now.Sub(p.ClosedAt)
		// Drop trades outside the widest window — they cannot contribute.
		if age < 0 || age > window30d {
			continue
		}

		key := p.Symbol + "_" + p.LongExchange + "_" + p.ShortExchange
		a, ok := agg[key]
		if !ok {
			a = &aggregator{
				symbol:  p.Symbol,
				longEx:  p.LongExchange,
				shortEx: p.ShortExchange,
			}
			agg[key] = a
		}

		tradeBps := p.RealizedPnL / p.NotionalUSDT * 10_000.0

		// 30d is the widest window: anything reaching this point contributes.
		a.tradesIn30d++
		a.bpsSumAll += tradeBps
		a.bpsSum30d += tradeBps
		if p.RealizedPnL > 0 {
			a.wins++
		}
		if age <= window7d {
			a.bpsSum7d += tradeBps
		}
		if age <= window24h {
			a.bpsSum24h += tradeBps
		}
	}

	out := make([]CandidateMetrics, 0, len(agg))
	for key, a := range agg {
		row := CandidateMetrics{
			Candidate:     key,
			Symbol:        a.symbol,
			LongExchange:  a.longEx,
			ShortExchange: a.shortEx,
			TradesWindow:  a.tradesIn30d,
			Bps24hPerDay:  a.bpsSum24h / 1.0,
			Bps7dPerDay:   a.bpsSum7d / 7.0,
			Bps30dPerDay:  a.bpsSum30d / 30.0,
		}
		if a.tradesIn30d > 0 {
			row.AvgRealizedBps = a.bpsSumAll / float64(a.tradesIn30d)
			row.WinPct = float64(a.wins) / float64(a.tradesIn30d) * 100.0
		}
		out = append(out, row)
	}

	// Sort by Bps30dPerDay desc; tie-break on Candidate key for determinism.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bps30dPerDay != out[j].Bps30dPerDay {
			return out[i].Bps30dPerDay > out[j].Bps30dPerDay
		}
		return out[i].Candidate < out[j].Candidate
	})

	return out
}
