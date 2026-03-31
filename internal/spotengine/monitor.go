package spotengine

import (
	"errors"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
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
		if pos.Status == models.SpotStatusPending {
			if pos.ExitReason == spotEntryManualRecoveryReason {
				continue
			}
			e.reconcilePendingEntry(pos)
			continue
		}
		if pos.Status == models.SpotStatusExiting && pos.PendingRepay {
			// Skip retry if deferred until a specific time (e.g. Bybit blackout).
			if pos.PendingRepayRetryAt != nil && time.Now().UTC().Before(*pos.PendingRepayRetryAt) {
				continue
			}
			e.retryPendingRepay(pos)
			continue
		}
		if pos.Status == models.SpotStatusExiting && !pos.PendingRepay {
			if pos.ExitTriggeredAt != nil && time.Since(*pos.ExitTriggeredAt) > 2*time.Minute {
				if e.isExiting(pos.ID) {
					// Exit goroutine still running — don't double-trigger.
					continue
				}
				_ = e.lockedUpdatePosition(pos.ID, func(p *models.SpotFuturesPosition) bool {
					p.ExitRetryCount++
					return true
				})
				isEmergency := pos.ExitRetryCount+1 >= 5
				e.log.Warn("monitor: retrying stuck exit for %s (retry #%d, emergency=%v)",
					pos.ID, pos.ExitRetryCount+1, isEmergency)
				go e.initiateExit(pos, pos.ExitReason, isEmergency)
			}
			continue
		}
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

	// Persist live economics snapshot for API/dashboard consumption.
	e.updateLiveEconomics(latest, isDirA)

	// Re-read again to include live economics for broadcast.
	if updated, err := e.db.GetSpotPosition(pos.ID); err != nil {
		e.log.Error("monitor: failed to re-read position %s after economics update: %v", pos.ID, err)
		// latest retains its pre-updateLiveEconomics value — stale but safe
	} else {
		latest = updated
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
	rate, err := e.getFreshBorrowRate(pos.Exchange, pos.BaseCoin, smExch)
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
	if opp, found := e.lookupCurrentOpp(pos.Symbol, pos.Exchange, pos.Direction); found {
		fundingAPR = opp.FundingAPR
	}
	negativeYield := currentAPR > fundingAPR

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

// updateLiveEconomics computes and persists the current funding, fee, and net
// yield snapshot so the API and dashboard reflect the same economics the exit
// engine uses.
func (e *SpotEngine) updateLiveEconomics(pos *models.SpotFuturesPosition, isDirA bool) {
	now := time.Now()
	var currentFundingAPR, feeAPR float64
	source := "entry_fallback"

	if opp, found := e.lookupCurrentOpp(pos.Symbol, pos.Exchange, pos.Direction); found {
		currentFundingAPR = opp.FundingAPR
		feeAPR = opp.FeeAPR
		source = "live_scan"
	} else {
		currentFundingAPR = pos.FundingAPR
		feeAPR = pos.FeeAPR
	}

	// Last-resort feeAPR for legacy positions predating FeeAPR field.
	if feeAPR == 0 {
		takerFee := spotFees[pos.Exchange]
		if takerFee == 0 {
			takerFee = 0.0005
		}
		feeAPR = takerFee * 4 * (365.0 / assumedHoldDays)
	}

	borrowAPR := pos.CurrentBorrowAPR
	if !isDirA {
		borrowAPR = 0
	}
	netYield := currentFundingAPR - borrowAPR - feeAPR

	_ = e.lockedUpdatePosition(pos.ID, func(p *models.SpotFuturesPosition) bool {
		p.CurrentFundingAPR = currentFundingAPR
		p.CurrentFeeAPR = feeAPR
		p.CurrentNetYieldAPR = netYield
		p.YieldDataSource = source
		p.YieldSnapshotAt = &now
		return true
	})
}

// retryPendingRepay retries margin repay for a position whose trade legs are
// closed but repay failed (e.g. Bybit blackout, partial buyback, or accrued
// interest shortfall). It queries the actual liability via GetMarginBalance
// and buys the missing spot if the account is short before retrying repay.
func (e *SpotEngine) retryPendingRepay(pos *models.SpotFuturesPosition) {
	smExch, ok := e.spotMargin[pos.Exchange]
	if !ok {
		e.log.Error("retryPendingRepay: no SpotMarginExchange for %s — cannot retry repay for %s", pos.Exchange, pos.ID)
		return
	}

	// Query actual liability (principal + accrued interest) and available balance.
	bal, balErr := smExch.GetMarginBalance(pos.BaseCoin)
	repayAmount := utils.FormatSize(pos.BorrowAmount, 6) // fallback
	if balErr == nil && bal.Borrowed > 0 {
		liability := bal.Borrowed + bal.Interest
		repayAmount = utils.FormatSize(liability, 6)

		// If we don't hold enough coin, buy the shortfall.
		deficit := liability - bal.Available
		if deficit > 0 {
			// Add 0.5% buffer for slippage on the market buy.
			deficitWithBuffer := deficit * 1.005
			deficitStr := utils.FormatSize(deficitWithBuffer, 6)
			e.log.Info("retryPendingRepay: account short %.6f %s (available=%.6f, liability=%.6f) — buying deficit %s",
				deficit, pos.BaseCoin, bal.Available, liability, deficitStr)

			orderID, buyErr := smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
				Symbol:    pos.Symbol,
				Side:      exchange.SideBuy,
				OrderType: "market",
				Size:      deficitStr,
				Force:     "ioc",
			})
			if buyErr != nil {
				e.log.Warn("retryPendingRepay: deficit buy failed for %s: %v — will retry next tick", pos.ID, buyErr)
				return
			}

			// Confirm fill via the shared WS cache plus native spot order query.
			if futExch, fOk := e.exchanges[pos.Exchange]; fOk {
				filled, _, confirmed, confErr := e.confirmSpotFill(smExch, futExch, orderID, pos.Symbol, deficitWithBuffer)
				if !confirmed {
					e.log.Warn("retryPendingRepay: deficit buy %s unconfirmed for %s: %v — will retry next tick", orderID, pos.ID, confErr)
					return
				}
				e.log.Info("retryPendingRepay: deficit buy filled %.6f %s for %s", filled, pos.BaseCoin, pos.ID)
			}
		}
	} else if balErr != nil {
		e.log.Warn("retryPendingRepay: GetMarginBalance failed for %s: %v — using original BorrowAmount", pos.BaseCoin, balErr)
	}

	e.log.Info("retryPendingRepay: attempting repay %s %s for %s", repayAmount, pos.BaseCoin, pos.ID)

	if err := smExch.MarginRepay(exchange.MarginRepayParams{
		Coin:   pos.BaseCoin,
		Amount: repayAmount,
	}); err != nil {
		var blackout *exchange.ErrRepayBlackout
		if errors.As(err, &blackout) {
			e.log.Info("retryPendingRepay: Bybit blackout for %s — deferring retry until %s",
				pos.ID, blackout.RetryAfter.Format(time.RFC3339))
			_ = e.lockedUpdatePosition(pos.ID, func(p *models.SpotFuturesPosition) bool {
				p.PendingRepayRetryAt = &blackout.RetryAfter
				return true
			})
			return
		}
		e.log.Warn("retryPendingRepay: repay still failing for %s: %v — will retry next tick", pos.ID, err)
		return
	}

	e.log.Info("retryPendingRepay: repay succeeded for %s — completing exit", pos.ID)

	// Clear PendingRepay flag and retry-after.
	if err := e.lockedUpdatePosition(pos.ID, func(p *models.SpotFuturesPosition) bool {
		p.PendingRepay = false
		p.PendingRepayRetryAt = nil
		return true
	}); err != nil {
		e.log.Error("retryPendingRepay: failed to clear PendingRepay for %s: %v", pos.ID, err)
	}

	// Re-read position to get latest state for completeExit.
	updated, err := e.db.GetSpotPosition(pos.ID)
	if err != nil {
		e.log.Error("retryPendingRepay: failed to re-read position %s: %v", pos.ID, err)
		return
	}
	e.completeExit(updated, updated.ExitReason)
}

// lookupCurrentOpp searches the latest discovery scan for the full opportunity
// matching a symbol/exchange/direction combination.
func (e *SpotEngine) lookupCurrentOpp(symbol, exchName, direction string) (SpotArbOpportunity, bool) {
	opps := e.getLatestOpps()
	for _, opp := range opps {
		if opp.Symbol == symbol && opp.Exchange == exchName && opp.Direction == direction {
			return opp, true
		}
	}
	return SpotArbOpportunity{}, false
}
