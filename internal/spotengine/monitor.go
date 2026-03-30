package spotengine

import (
	"time"

	"arb/internal/models"
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

// monitorPosition refreshes borrow rate and updates accrued cost for one position.
func (e *SpotEngine) monitorPosition(pos *models.SpotFuturesPosition) {
	// Direction B (buy_spot_short) has no borrow costs — just broadcast health.
	if pos.Direction != "borrow_sell_long" {
		e.broadcastHealth(pos)
		return
	}

	smExch, ok := e.spotMargin[pos.Exchange]
	if !ok {
		e.log.Warn("monitor: no SpotMarginExchange for %s — skipping %s", pos.Exchange, pos.ID)
		return
	}

	// Refresh borrow rate using the shared cache from discovery.go.
	rate, err := e.getCachedBorrowRate(pos.Exchange, pos.BaseCoin, smExch)
	if err != nil {
		e.log.Warn("monitor: GetMarginInterestRate(%s/%s) failed: %v", pos.Exchange, pos.BaseCoin, err)
		e.broadcastHealth(pos)
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
	err = e.db.UpdateSpotPositionFields(pos.ID, func(p *models.SpotFuturesPosition) bool {
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

	// Re-read updated position for broadcast.
	updated, err := e.db.GetSpotPosition(pos.ID)
	if err == nil {
		e.broadcastHealth(updated)
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
