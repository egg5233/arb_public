package spotengine

import (
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
)

// monitorLoop periodically checks all active spot-futures positions:
// refreshes borrow rates, accrues borrow cost, tracks negative yield,
// and broadcasts health updates via WebSocket.
func (e *SpotEngine) monitorLoop() {
	defer e.wg.Done()

	interval := time.Duration(e.cfg.SpotFuturesMonitorIntervalSec) * time.Second
	if interval < 10*time.Second {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	e.log.Info("spot-futures monitor started (interval: %s)", interval)

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.monitorTick()
		}
	}
}

// monitorTick runs one pass over all active positions.
func (e *SpotEngine) monitorTick() {
	positions, err := e.db.GetActiveSpotPositions()
	if err != nil {
		e.log.Error("monitor: failed to get active positions: %v", err)
		return
	}
	if len(positions) == 0 {
		return
	}

	for _, pos := range positions {
		if pos.Status != models.SpotStatusActive {
			continue
		}
		e.monitorPosition(pos)
	}
}

// monitorPosition refreshes borrow rate, updates accrued cost, checks exit
// triggers, and broadcasts health for one position. Both Direction A and B
// are evaluated for exit triggers; only Direction A updates borrow rates.
func (e *SpotEngine) monitorPosition(pos *models.SpotFuturesPosition) {
	isDirA := pos.Direction == "borrow_sell_long"

	// ---------------------------------------------------------------
	// Direction A: update borrow rates and accrued cost.
	// ---------------------------------------------------------------
	if isDirA {
		smExch, ok := e.spotMargin[pos.Exchange]
		if !ok {
			e.log.Warn("monitor: no SpotMarginExchange for %s — skipping borrow update for %s", pos.Exchange, pos.ID)
		} else {
			e.updateBorrowCost(pos, smExch)
		}
	}

	// Re-read position to get latest state after borrow update.
	latest, err := e.db.GetSpotPosition(pos.ID)
	if err != nil {
		e.log.Error("monitor: failed to re-read position %s: %v", pos.ID, err)
		latest = pos // fall back to original
	}

	// Broadcast health for all directions.
	e.broadcastHealth(latest)

	// ---------------------------------------------------------------
	// Check exit triggers for all directions.
	// ---------------------------------------------------------------
	reason, isEmergency := e.checkExitTriggers(latest)

	// Persist tracking metrics updated by checkExitTriggers (best-effort).
	if latest.PeakPriceMovePct > 0 || latest.MarginUtilizationPct > 0 {
		_ = e.lockedUpdatePosition(latest.ID, func(p *models.SpotFuturesPosition) bool {
			changed := false
			if latest.PeakPriceMovePct > p.PeakPriceMovePct {
				p.PeakPriceMovePct = latest.PeakPriceMovePct
				changed = true
			}
			if latest.MarginUtilizationPct != p.MarginUtilizationPct {
				p.MarginUtilizationPct = latest.MarginUtilizationPct
				changed = true
			}
			return changed
		})
	}

	if reason != "" {
		if !e.isExiting(latest.ID) {
			go e.initiateExit(latest, reason, isEmergency)
		}
	}
}

// updateBorrowCost refreshes the borrow rate and accrues cost for a Direction A position.
func (e *SpotEngine) updateBorrowCost(pos *models.SpotFuturesPosition, smExch exchange.SpotMarginExchange) {
	rate, err := e.getCachedBorrowRate(pos.Exchange, pos.BaseCoin, smExch)
	if err != nil {
		e.log.Warn("monitor: GetMarginInterestRate(%s/%s) failed: %v", pos.Exchange, pos.BaseCoin, err)
		return
	}

	now := time.Now().UTC()
	hourlyRate := rate.HourlyRate
	currentAPR := hourlyRate * 8760

	// Calculate accrued borrow cost since last check (in USDT).
	var costDelta float64
	if !pos.LastBorrowRateCheck.IsZero() && pos.BorrowAmount > 0 {
		hoursElapsed := now.Sub(pos.LastBorrowRateCheck).Hours()
		if hoursElapsed > 0 && hoursElapsed < 24 { // sanity cap: skip stale gaps > 24h
			interestInCoin := pos.BorrowAmount * hourlyRate * hoursElapsed
			// Convert to USDT using entry price (stable proxy for delta-neutral position).
			price := pos.SpotEntryPrice
			if price <= 0 {
				price = pos.FuturesEntry
			}
			if price > 0 {
				costDelta = interestInCoin * price
			}
		}
	}

	// Determine current funding APR for yield comparison.
	fundingAPR := pos.FundingAPR
	if latest := e.lookupCurrentFundingAPR(pos.Symbol, pos.Exchange, pos.Direction); latest > 0 {
		fundingAPR = latest
	}
	negativeYield := fundingAPR > 0 && currentAPR > fundingAPR

	// Update position fields atomically.
	err = e.lockedUpdatePosition(pos.ID, func(p *models.SpotFuturesPosition) bool {
		p.CurrentBorrowAPR = currentAPR
		p.LastBorrowRateCheck = now
		p.BorrowCostAccrued += costDelta

		if negativeYield {
			if p.NegativeYieldSince == nil {
				t := now
				p.NegativeYieldSince = &t
				e.log.Warn("monitor: %s negative yield started — borrowAPR=%.2f%% > fundingAPR=%.2f%%",
					p.Symbol, currentAPR*100, fundingAPR*100)
			}
		} else {
			if p.NegativeYieldSince != nil {
				e.log.Info("monitor: %s negative yield recovered — borrowAPR=%.2f%% <= fundingAPR=%.2f%%",
					p.Symbol, currentAPR*100, fundingAPR*100)
			}
			p.NegativeYieldSince = nil
		}

		return true
	})
	if err != nil {
		e.log.Error("monitor: failed to update position %s: %v", pos.ID, err)
		return
	}

	// Alert if borrow rate exceeds max threshold.
	maxAPR := e.cfg.SpotFuturesMaxBorrowAPR
	if maxAPR > 0 && currentAPR > maxAPR {
		e.log.Warn("ALERT: %s borrow APR %.2f%% exceeds max %.2f%% — exit recommended",
			pos.Symbol, currentAPR*100, maxAPR*100)
	}
}

// broadcastHealth sends a position health update via WebSocket.
func (e *SpotEngine) broadcastHealth(pos *models.SpotFuturesPosition) {
	e.api.BroadcastSpotHealth(pos)
}

// lookupCurrentFundingAPR searches the latest discovery scan for the current
// funding APR of a symbol/exchange/direction combination.
func (e *SpotEngine) lookupCurrentFundingAPR(symbol, exchName, direction string) float64 {
	opps := e.getLatestOpps()
	for _, opp := range opps {
		if opp.Symbol == symbol && opp.Exchange == exchName && opp.Direction == direction {
			return opp.FundingAPR
		}
	}
	return 0
}
