package engine

import (
	"fmt"
	"time"

	"arb/internal/analytics"
	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/utils"
)

func (e *Engine) createPendingPosition(opp models.Opportunity) *models.ArbitragePosition {
	now := time.Now().UTC()
	return &models.ArbitragePosition{
		ID:            utils.GenerateID(opp.Symbol, now.UnixMilli()),
		Symbol:        opp.Symbol,
		LongExchange:  opp.LongExchange,
		ShortExchange: opp.ShortExchange,
		Status:        models.StatusPending,
		EntrySpread:   opp.Spread,
		AllExchanges:  []string{opp.LongExchange, opp.ShortExchange},
		NextFunding:   e.computeNextFunding(opp.Symbol, opp.LongExchange, opp.ShortExchange),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func (e *Engine) reservePerpCapital(opp models.Opportunity, approval *models.RiskApproval) (*risk.CapitalReservation, error) {
	if e.allocator == nil || !e.allocator.Enabled() {
		return nil, nil
	}
	return e.allocator.Reserve(risk.StrategyPerpPerp, map[string]float64{
		opp.LongExchange:  approval.RequiredMargin,
		opp.ShortExchange: approval.RequiredMargin,
	})
}

func (e *Engine) commitPerpCapital(res *risk.CapitalReservation, posID string) error {
	if e.allocator == nil || !e.allocator.Enabled() || res == nil {
		return nil
	}
	pos, err := e.db.GetPosition(posID)
	if err != nil {
		return fmt.Errorf("load position for capital commit: %w", err)
	}
	leverage := float64(e.cfg.Leverage)
	if leverage <= 0 {
		leverage = 1
	}
	return e.allocator.Commit(res, posID, map[string]float64{
		pos.LongExchange:  (pos.LongSize * pos.LongEntry) / leverage,
		pos.ShortExchange: (pos.ShortSize * pos.ShortEntry) / leverage,
	})
}

// effectiveCapitalPerLeg returns the USDT per leg, using allocator's derived value
// when unified capital is enabled, or falling back to cfg.CapitalPerLeg.
func (e *Engine) effectiveCapitalPerLeg() float64 {
	if e.allocator != nil && e.allocator.Enabled() {
		if ecl := e.allocator.EffectiveCapitalPerLeg(); ecl > 0 {
			return ecl
		}
	}
	return e.cfg.CapitalPerLeg
}

// updateAllocation recomputes effective strategy allocation percentages using
// trailing APR from Phase 4 analytics. Called once per EntryScan cycle.
// This activates CA-03 (performance weighting) and feeds CA-04 (dynamic shifting).
func (e *Engine) updateAllocation() {
	if e.allocator == nil || !e.cfg.EnableUnifiedCapital {
		return
	}

	// Load closed positions for trailing APR computation.
	perps, err := e.db.GetHistory(200)
	if err != nil {
		e.log.Warn("updateAllocation: failed to load perp history: %v", err)
		return
	}
	spots, err := e.db.GetSpotHistory(200)
	if err != nil {
		e.log.Warn("updateAllocation: failed to load spot history: %v", err)
		return
	}

	// Compute per-strategy summary (APR, trade count).
	summaries := analytics.ComputeStrategySummary(perps, spots)

	var perpAPR, spotAPR float64
	var perpTrades, spotTrades int
	for _, s := range summaries {
		switch s.Strategy {
		case "perp":
			perpAPR = s.APR
			perpTrades = s.TradeCount
		case "spot":
			spotAPR = s.APR
			spotTrades = s.TradeCount
		}
	}

	// Check minimum trade threshold: need >= 3 trades per strategy for meaningful APR.
	minTrades := 3
	if perpTrades < minTrades || spotTrades < minTrades {
		// Insufficient data -- use base profile split, no performance tilt.
		e.allocator.SetEffectiveAllocation(
			e.cfg.MaxPerpPerpPct,
			e.cfg.MaxSpotFuturesPct,
		)
		e.log.Info("updateAllocation: insufficient trades (perp=%d, spot=%d, min=%d), using base split %.0f/%.0f",
			perpTrades, spotTrades, minTrades,
			e.cfg.MaxPerpPerpPct*100, e.cfg.MaxSpotFuturesPct*100)
		return
	}

	// Compute performance-weighted allocation (CA-03).
	perpPct, spotPct := risk.ComputeEffectiveAllocation(
		perpAPR, spotAPR,
		e.cfg.MaxPerpPerpPct, e.cfg.MaxSpotFuturesPct,
		e.cfg.AllocationFloorPct, e.cfg.AllocationCeilingPct,
	)

	// Cache in allocator for use by strategyPct and DynamicStrategyPct.
	e.allocator.SetEffectiveAllocation(perpPct, spotPct)
	e.log.Info("updateAllocation: perpAPR=%.2f%% spotAPR=%.2f%% -> effective split %.1f/%.1f",
		perpAPR, spotAPR, perpPct*100, spotPct*100)
}

// dynamicStrategyPct returns the effective strategy percentage cap for this scan cycle,
// accounting for dynamic shifting when the other strategy has no opportunities (CA-04).
func (e *Engine) dynamicStrategyPct(strategy risk.Strategy, perpHasOpps, spotHasOpps bool) float64 {
	if e.allocator == nil || !e.cfg.EnableUnifiedCapital {
		return 0
	}
	summary, err := e.allocator.Summary()
	if err != nil {
		e.log.Warn("dynamicStrategyPct: summary failed: %v", err)
		return 0
	}
	committed := map[risk.Strategy]float64{
		risk.StrategyPerpPerp:    summary.ByStrategy[risk.StrategyPerpPerp],
		risk.StrategySpotFutures: summary.ByStrategy[risk.StrategySpotFutures],
	}
	return e.allocator.DynamicStrategyPct(strategy, perpHasOpps, spotHasOpps, committed)
}

func (e *Engine) releasePerpReservation(res *risk.CapitalReservation) {
	if e.allocator == nil || !e.allocator.Enabled() || res == nil {
		return
	}
	if err := e.allocator.ReleaseReservation(res.ID); err != nil {
		e.log.Error("allocator release reservation %s: %v", res.ID, err)
	}
}

func (e *Engine) releasePerpPosition(posID string) {
	if e.allocator == nil || !e.allocator.Enabled() || posID == "" {
		return
	}
	if err := e.allocator.ReleasePosition(posID); err != nil {
		e.log.Error("allocator release position %s: %v", posID, err)
	}
}
