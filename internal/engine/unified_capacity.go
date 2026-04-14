package engine

import "arb/internal/risk"

// unified_capacity.go ??capacity snapshot used by the unified cross-strategy
// entry evaluator so B&B feasibility matches ReserveBatch's cumulative cap
// checks. The snapshot is point-in-time: one allocator Summary() call plus
// dynamic strategy-pct overrides derived from the current opportunity mix.

type unifiedCapacitySnapshot struct {
	committedByStrategy map[risk.Strategy]float64
	committedByExchange map[string]float64

	effectivePerpPct float64
	effectiveSpotPct float64
	perpPctOverride  float64
	spotPctOverride  float64

	perExchangeCap float64
	totalCap       float64
}

func (e *Engine) buildCapacitySnapshot(perpHasOpps, spotHasOpps bool) (*unifiedCapacitySnapshot, error) {
	snap := &unifiedCapacitySnapshot{
		committedByStrategy: map[risk.Strategy]float64{},
		committedByExchange: map[string]float64{},
	}
	if e == nil || e.cfg == nil {
		return snap, nil
	}

	snap.totalCap = e.cfg.MaxTotalExposureUSDT
	if snap.totalCap > 0 && e.cfg.MaxPerExchangePct > 0 {
		snap.perExchangeCap = snap.totalCap * e.cfg.MaxPerExchangePct
	}

	var (
		committed     = map[risk.Strategy]float64{}
		effectivePerp = e.cfg.MaxPerpPerpPct
		effectiveSpot = e.cfg.MaxSpotFuturesPct
	)

	if e.allocator != nil && e.allocator.Enabled() {
		summary, err := e.allocator.Summary()
		if err != nil {
			return nil, err
		}
		for k, v := range summary.ByStrategy {
			snap.committedByStrategy[k] = v
			committed[k] = v
		}
		for k, v := range summary.ByExchange {
			snap.committedByExchange[k] = v
		}
		if summary.EffectivePerpPct > 0 {
			effectivePerp = summary.EffectivePerpPct
		}
		if summary.EffectiveSpotPct > 0 {
			effectiveSpot = summary.EffectiveSpotPct
		}
		snap.perpPctOverride = e.allocator.DynamicStrategyPct(risk.StrategyPerpPerp, perpHasOpps, spotHasOpps, committed)
		snap.spotPctOverride = e.allocator.DynamicStrategyPct(risk.StrategySpotFutures, perpHasOpps, spotHasOpps, committed)
	}

	snap.effectivePerpPct = effectivePerp
	snap.effectiveSpotPct = effectiveSpot
	if snap.perpPctOverride <= 0 {
		snap.perpPctOverride = snap.effectivePerpPct
	}
	if snap.spotPctOverride <= 0 {
		snap.spotPctOverride = snap.effectiveSpotPct
	}

	return snap, nil
}

func unifiedSnapshotStrategyPct(snap *unifiedCapacitySnapshot, strategy risk.Strategy) float64 {
	if snap == nil {
		return 0
	}
	switch strategy {
	case risk.StrategySpotFutures:
		pct := snap.effectiveSpotPct
		if snap.spotPctOverride > pct {
			pct = snap.spotPctOverride
		}
		return pct
	default:
		pct := snap.effectivePerpPct
		if snap.perpPctOverride > pct {
			pct = snap.perpPctOverride
		}
		return pct
	}
}

func unifiedEvaluatorFeasible(
	snap *unifiedCapacitySnapshot,
	keyToChoice map[string]*unifiedEntryChoice,
	exposuresOf func(*unifiedEntryChoice) map[string]float64,
	keys []string,
) bool {
	if snap == nil {
		return true
	}

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
		requestTotal := 0.0
		for exchangeName, amount := range exposures {
			if amount <= 0 {
				continue
			}
			requestTotal += amount
			addByExchange[exchangeName] += amount
			addTotal += amount
		}
		addByStrategy[c.Strategy] += requestTotal
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
		pct := unifiedSnapshotStrategyPct(snap, strategy)
		if pct <= 0 || snap.totalCap <= 0 {
			continue
		}
		strategyCap := snap.totalCap * pct
		if snap.committedByStrategy[strategy]+add > strategyCap {
			return false
		}
	}

	if snap.perExchangeCap > 0 {
		for exchangeName, add := range addByExchange {
			if snap.committedByExchange[exchangeName]+add > snap.perExchangeCap {
				return false
			}
		}
	}

	return true
}
