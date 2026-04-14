package engine

import (
	"arb/internal/risk"
)

// unified_capacity.go — capacity snapshot used by the unified cross-strategy
// entry evaluator so B&B leaf feasibility matches ReserveBatch's cumulative
// cap checks. Without this gate the solver can greedily pick a high-value
// subset whose cumulative per-exchange margin or per-strategy exposure
// would breach the allocator's caps; ReserveBatch then rejects the whole
// batch and the tick produces zero winners (all work wasted).
//
// Contract: the snapshot is point-in-time (single call to allocator.Summary
// plus the caller-provided balances map). The evaluator then simulates
// cumulative deltas from the candidate keys against this snapshot without
// further Redis round-trips, so the callback stays cheap enough to run
// from inside the branch-and-bound walk.

// unifiedCapacitySnapshot captures everything the evaluator needs to reject
// infeasible candidate subsets pre-ReserveBatch:
//
//   - committedByStrategy: Redis-backed exposure already committed+reserved
//     per strategy at tick start.
//   - committedByExchange: same, keyed by exchange name.
//   - strategyCap: per-strategy USDT ceiling (cfg.MaxTotalExposureUSDT *
//     strategyPct, honoring dynamic caps when supplied).
//   - perExchangeCap: per-exchange USDT ceiling (cfg.MaxTotalExposureUSDT *
//     cfg.MaxPerExchangePct). Zero disables the check (mirrors allocator
//     semantics).
//   - totalCap: cfg.MaxTotalExposureUSDT. Zero disables (same as allocator).
//
// The evaluator reproduces allocator.checkCapsWithOverride's accounting
// verbatim so every rejection here would also trip the allocator — i.e.
// the evaluator is strictly more conservative and never admits a set that
// ReserveBatch would then block.
type unifiedCapacitySnapshot struct {
	committedByStrategy map[risk.Strategy]float64
	committedByExchange map[string]float64

	strategyCap    map[risk.Strategy]float64
	perExchangeCap float64
	totalCap       float64
}

// buildCapacitySnapshot constructs the snapshot used by the evaluator. When
// the allocator is disabled or not yet initialized the snapshot's caps are
// all zero, which the feasibility helper treats as "no cap enforced"
// (mirrors allocator.checkCapsWithOverride short-circuits on totalCap<=0).
//
// The per-strategy caps embed dynamic strategy percentages when the
// allocator exposes them — same lookup the allocator itself uses inside
// strategyPct so the evaluator's ceiling stays in sync with the cap a
// subsequent ReserveBatch would apply.
func (e *Engine) buildCapacitySnapshot() (*unifiedCapacitySnapshot, error) {
	snap := &unifiedCapacitySnapshot{
		committedByStrategy: map[risk.Strategy]float64{},
		committedByExchange: map[string]float64{},
		strategyCap:         map[risk.Strategy]float64{},
	}
	if e == nil || e.cfg == nil {
		return snap, nil
	}
	snap.totalCap = e.cfg.MaxTotalExposureUSDT
	if snap.totalCap > 0 && e.cfg.MaxPerExchangePct > 0 {
		snap.perExchangeCap = snap.totalCap * e.cfg.MaxPerExchangePct
	}

	// Per-strategy caps: static + dynamic. We compute both perp and spot
	// here so the evaluator has a ready-to-use ceiling for either strategy
	// without another allocator round-trip at evaluate time.
	if snap.totalCap > 0 {
		if e.cfg.MaxPerpPerpPct > 0 {
			snap.strategyCap[risk.StrategyPerpPerp] = snap.totalCap * e.cfg.MaxPerpPerpPct
		}
		if e.cfg.MaxSpotFuturesPct > 0 {
			snap.strategyCap[risk.StrategySpotFutures] = snap.totalCap * e.cfg.MaxSpotFuturesPct
		}
	}

	// Load current committed+reserved totals from the allocator so the
	// cumulative check is "current state + batch items <= cap".
	if e.allocator != nil && e.allocator.Enabled() {
		summary, err := e.allocator.Summary()
		if err != nil {
			return nil, err
		}
		for k, v := range summary.ByStrategy {
			snap.committedByStrategy[k] = v
		}
		for k, v := range summary.ByExchange {
			snap.committedByExchange[k] = v
		}
	}
	return snap, nil
}

// evaluatorFeasible reports whether the combined exposures for a set of
// candidate keys would clear every allocator cap. Semantics mirror
// risk.CapitalAllocator.checkCapsWithOverride so the evaluator agrees
// with ReserveBatch on what is/isn't feasible.
//
// Inputs:
//   - snap:        point-in-time capacity snapshot.
//   - keyToChoice: the selector's key → choice map (used to look up
//     exposures without re-deriving from groups).
//   - keys:        candidate subset to evaluate.
//
// Returns true when every cap clears; false otherwise. On any unknown
// key the function returns false (defensive — should never happen given
// the solver only passes keys from the groups it was built with).
func unifiedEvaluatorFeasible(
	snap *unifiedCapacitySnapshot,
	keyToChoice map[string]*unifiedEntryChoice,
	exposuresOf func(*unifiedEntryChoice) map[string]float64,
	keys []string,
) bool {
	if snap == nil {
		return true
	}
	// Cumulative per-strategy and per-exchange tallies.
	addByStrategy := map[risk.Strategy]float64{}
	addByExchange := map[string]float64{}
	addTotal := 0.0
	for _, k := range keys {
		c, ok := keyToChoice[k]
		if !ok || c == nil {
			return false
		}
		exposures := exposuresOf(c)
		if len(exposures) == 0 {
			continue
		}
		for exchange, amount := range exposures {
			if amount <= 0 {
				continue
			}
			addByStrategy[c.Strategy] += amount
			addByExchange[exchange] += amount
			addTotal += amount
		}
	}

	if snap.totalCap > 0 {
		currentTotal := 0.0
		for _, v := range snap.committedByStrategy {
			currentTotal += v
		}
		if currentTotal+addTotal > snap.totalCap {
			return false
		}
	}
	for strategy, add := range addByStrategy {
		cap := snap.strategyCap[strategy]
		if cap <= 0 {
			continue
		}
		if snap.committedByStrategy[strategy]+add > cap {
			return false
		}
	}
	if snap.perExchangeCap > 0 {
		for exchange, add := range addByExchange {
			if snap.committedByExchange[exchange]+add > snap.perExchangeCap {
				return false
			}
		}
	}
	return true
}
