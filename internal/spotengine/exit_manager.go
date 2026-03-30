package spotengine

import (
	"fmt"
	"math"
	"time"

	"arb/internal/models"
)

// exitState tracks in-flight exits to prevent double-triggering.
// Protected by SpotEngine.exitMu.
type exitState struct {
	exiting map[string]bool // posID → true if exit in progress
}

// checkExitTriggers evaluates 5 exit triggers in priority order for a position.
// Returns the reason string and whether this is an emergency (parallel close).
// An empty reason means no trigger fired.
func (e *SpotEngine) checkExitTriggers(pos *models.SpotFuturesPosition) (reason string, isEmergency bool) {
	isDirA := pos.Direction == "borrow_sell_long"

	// ---------------------------------------------------------------
	// 1. Borrow Cost Drift (Direction A only)
	// ---------------------------------------------------------------
	if isDirA {
		maxAPR := e.cfg.SpotFuturesMaxBorrowAPR
		if maxAPR > 0 && pos.CurrentBorrowAPR > maxAPR {
			e.log.Warn("exit trigger: %s borrow APR %.2f%% > max %.2f%%",
				pos.Symbol, pos.CurrentBorrowAPR*100, maxAPR*100)
			return "borrow_cost_exceeded", false
		}

		graceMin := e.cfg.SpotFuturesBorrowGraceMin
		if graceMin <= 0 {
			graceMin = 30
		}
		if pos.NegativeYieldSince != nil {
			elapsed := time.Since(*pos.NegativeYieldSince)
			if elapsed > time.Duration(graceMin)*time.Minute {
				e.log.Warn("exit trigger: %s negative yield for %s (grace: %dm)",
					pos.Symbol, elapsed.Round(time.Second), graceMin)
				return "borrow_cost_exceeded", false
			}
		}
	}

	// ---------------------------------------------------------------
	// 2. Funding Rate Drop
	// ---------------------------------------------------------------
	currentFundingAPR := e.lookupCurrentFundingAPR(pos.Symbol, pos.Exchange, pos.Direction)
	if currentFundingAPR <= 0 {
		// Symbol not in latest scan — fall back to entry-time APR.
		currentFundingAPR = pos.FundingAPR
	}
	if currentFundingAPR > 0 {
		borrowAPR := pos.CurrentBorrowAPR
		if !isDirA {
			borrowAPR = 0 // Direction B has no borrow
		}
		minNet := e.cfg.SpotFuturesMinNetYieldAPR
		if currentFundingAPR-borrowAPR < minNet {
			e.log.Warn("exit trigger: %s net yield %.2f%% < min %.2f%% (funding=%.2f%% borrow=%.2f%%)",
				pos.Symbol, (currentFundingAPR-borrowAPR)*100, minNet*100,
				currentFundingAPR*100, borrowAPR*100)
			return "yield_below_minimum", false
		}
	}

	// ---------------------------------------------------------------
	// 3. Price Spike
	// ---------------------------------------------------------------
	priceExitPct := e.cfg.SpotFuturesPriceExitPct
	if priceExitPct <= 0 {
		priceExitPct = 20.0
	}
	priceEmergencyPct := e.cfg.SpotFuturesPriceEmergencyPct
	if priceEmergencyPct <= 0 {
		priceEmergencyPct = 30.0
	}

	if pos.FuturesEntry > 0 {
		futExch, ok := e.exchanges[pos.Exchange]
		if ok {
			ob, err := futExch.GetOrderbook(pos.Symbol, 5)
			if err == nil && len(ob.Bids) > 0 && len(ob.Asks) > 0 {
				currentPrice := (ob.Bids[0].Price + ob.Asks[0].Price) / 2
				movePct := (currentPrice - pos.FuturesEntry) / pos.FuturesEntry * 100

				// Track peak absolute move.
				absMove := math.Abs(movePct)
				if absMove > pos.PeakPriceMovePct {
					pos.PeakPriceMovePct = absMove
				}

				if isDirA {
					// Direction A (long futures, short spot): UP move is risky.
					if movePct > priceEmergencyPct {
						e.log.Error("exit trigger: %s EMERGENCY price spike +%.1f%% (entry=%.4f now=%.4f)",
							pos.Symbol, movePct, pos.FuturesEntry, currentPrice)
						return "emergency_price_spike", true
					}
					if movePct > priceExitPct {
						e.log.Warn("exit trigger: %s price spike +%.1f%% (entry=%.4f now=%.4f)",
							pos.Symbol, movePct, pos.FuturesEntry, currentPrice)
						return "price_spike_exit", false
					}
				} else {
					// Direction B (short futures, long spot): UP move is risky.
					if movePct > priceEmergencyPct {
						e.log.Error("exit trigger: %s EMERGENCY price spike +%.1f%% (entry=%.4f now=%.4f)",
							pos.Symbol, movePct, pos.FuturesEntry, currentPrice)
						return "emergency_price_spike", true
					}
					if movePct > priceExitPct {
						e.log.Warn("exit trigger: %s price spike +%.1f%% (entry=%.4f now=%.4f)",
							pos.Symbol, movePct, pos.FuturesEntry, currentPrice)
						return "price_spike_exit", false
					}
				}
			} else if err != nil {
				e.log.Warn("exit check: %s GetOrderbook failed: %v", pos.Symbol, err)
			}
		}
	}

	// ---------------------------------------------------------------
	// 4. Margin Health (Direction A only)
	// ---------------------------------------------------------------
	if isDirA {
		marginExitPct := e.cfg.SpotFuturesMarginExitPct
		if marginExitPct <= 0 {
			marginExitPct = 85.0
		}
		marginEmergencyPct := e.cfg.SpotFuturesMarginEmergencyPct
		if marginEmergencyPct <= 0 {
			marginEmergencyPct = 95.0
		}

		smExch, ok := e.spotMargin[pos.Exchange]
		if ok && pos.BorrowAmount > 0 {
			mb, err := smExch.GetMarginBalance(pos.BaseCoin)
			if err == nil {
				// utilization = borrowed value / (borrowed value + available value) * 100
				price := pos.SpotEntryPrice
				if price <= 0 {
					price = pos.FuturesEntry
				}
				if price > 0 {
					borrowedValue := pos.BorrowAmount * price
					availableValue := mb.Available * price
					totalValue := borrowedValue + availableValue
					if totalValue > 0 {
						utilPct := borrowedValue / totalValue * 100
						pos.MarginUtilizationPct = utilPct

						if utilPct > marginEmergencyPct {
							e.log.Error("exit trigger: %s EMERGENCY margin utilization %.1f%% > %.1f%%",
								pos.Symbol, utilPct, marginEmergencyPct)
							return "margin_health_exit", true
						}
						if utilPct > marginExitPct {
							e.log.Warn("exit trigger: %s margin utilization %.1f%% > %.1f%%",
								pos.Symbol, utilPct, marginExitPct)
							return "margin_health_exit", false
						}
					}
				}
			} else {
				e.log.Warn("exit check: %s GetMarginBalance(%s) failed: %v", pos.Symbol, pos.BaseCoin, err)
			}
		}
	}

	// ---------------------------------------------------------------
	// 5. No trigger
	// ---------------------------------------------------------------
	return "", false
}

// initiateExit runs the full exit sequence for a position. It should be called
// in a goroutine for automated exits, or synchronously for manual closes.
func (e *SpotEngine) initiateExit(pos *models.SpotFuturesPosition, reason string, isEmergency bool) {
	// Mark as exiting (prevent double-trigger).
	e.exitMu.Lock()
	e.exitState.exiting[pos.ID] = true
	e.exitMu.Unlock()

	defer func() {
		e.exitMu.Lock()
		delete(e.exitState.exiting, pos.ID)
		e.exitMu.Unlock()
	}()

	// Update position status to "exiting".
	now := time.Now().UTC()
	err := e.lockedUpdatePosition(pos.ID, func(p *models.SpotFuturesPosition) bool {
		p.Status = models.SpotStatusExiting
		p.ExitReason = reason
		p.ExitTriggeredAt = &now
		return true
	})
	if err != nil {
		e.log.Error("initiateExit: failed to set exiting status for %s: %v", pos.ID, err)
		return
	}

	emergencyStr := ""
	if isEmergency {
		emergencyStr = " [EMERGENCY]"
	}
	e.log.Info("initiateExit: %s %s on %s — reason=%s%s",
		pos.Symbol, pos.ID, pos.Exchange, reason, emergencyStr)

	// Execute the close — this is synchronous and handles all trade legs.
	if err := e.ClosePosition(pos, reason, isEmergency); err != nil {
		e.log.Error("CRITICAL: ClosePosition failed for %s (%s): %v — position stuck in 'exiting', manual intervention required",
			pos.ID, pos.Symbol, err)
		return
	}

	// Close succeeded — run post-exit cleanup.
	e.completeExit(pos, reason)
}

// completeExit performs post-exit cleanup: PnL calculation, status update,
// history recording, stats update, cooldown, and broadcast.
func (e *SpotEngine) completeExit(pos *models.SpotFuturesPosition, reason string) {
	isDirA := pos.Direction == "borrow_sell_long"

	// Calculate PnL.
	var spotPnL, futuresPnL float64

	if isDirA {
		// Direction A: sold spot at entry, buying back at exit.
		spotPnL = (pos.SpotEntryPrice - pos.SpotExitPrice) * pos.SpotSize
	} else {
		// Direction B: bought spot at entry, selling at exit.
		spotPnL = (pos.SpotExitPrice - pos.SpotEntryPrice) * pos.SpotSize
	}

	if pos.FuturesSide == "long" {
		futuresPnL = (pos.FuturesExit - pos.FuturesEntry) * pos.FuturesSize
	} else {
		// short
		futuresPnL = (pos.FuturesEntry - pos.FuturesExit) * pos.FuturesSize
	}

	totalPnL := spotPnL + futuresPnL - pos.BorrowCostAccrued - pos.EntryFees - pos.ExitFees

	// Update position to closed — persist exit prices from ClosePosition and
	// tracking metrics from checkExitTriggers.
	now := time.Now().UTC()
	err := e.lockedUpdatePosition(pos.ID, func(p *models.SpotFuturesPosition) bool {
		p.Status = models.SpotStatusClosed
		p.RealizedPnL = totalPnL
		p.ExitCompletedAt = &now
		p.FuturesExit = pos.FuturesExit
		p.SpotExitPrice = pos.SpotExitPrice
		p.PeakPriceMovePct = pos.PeakPriceMovePct
		p.MarginUtilizationPct = pos.MarginUtilizationPct
		return true
	})
	if err != nil {
		e.log.Error("completeExit: failed to update position %s to closed: %v", pos.ID, err)
		return
	}

	// Add to history.
	// Re-read position for history record.
	updated, err := e.db.GetSpotPosition(pos.ID)
	if err != nil {
		e.log.Error("completeExit: failed to re-read position %s for history: %v", pos.ID, err)
	} else {
		if err := e.db.AddToSpotHistory(updated); err != nil {
			e.log.Error("completeExit: failed to add %s to history: %v", pos.ID, err)
		}
	}

	// Update stats.
	if err := e.db.UpdateSpotStats(totalPnL, totalPnL >= 0); err != nil {
		e.log.Error("completeExit: failed to update stats for %s: %v", pos.ID, err)
	}

	// Set cooldown on loss.
	if totalPnL < 0 {
		cooldownHours := e.cfg.SpotFuturesLossCooldownHours
		if cooldownHours <= 0 {
			cooldownHours = 4
		}
		if err := e.db.SetSpotCooldown(pos.Symbol, cooldownHours); err != nil {
			e.log.Error("completeExit: failed to set cooldown for %s: %v", pos.Symbol, err)
		} else {
			e.log.Info("completeExit: %s cooldown set for %dh (loss: %.2f USDT)", pos.Symbol, cooldownHours, totalPnL)
		}
	}

	// Broadcast final update.
	if updated != nil {
		e.api.BroadcastSpotPositionUpdate(updated)
	}

	// Log summary.
	pnlSign := "+"
	if totalPnL < 0 {
		pnlSign = ""
	}
	e.log.Info("EXIT COMPLETE: %s %s on %s — reason=%s pnl=%s%.4f USDT (spot=%.4f futures=%.4f borrow=-%.4f fees=-%.4f)",
		pos.Symbol, pos.ID, pos.Exchange, reason,
		pnlSign, totalPnL, spotPnL, futuresPnL, pos.BorrowCostAccrued, pos.EntryFees+pos.ExitFees)

	// Telegram alert.
	if e.telegram != nil && reason != "manual_close" {
		duration := time.Duration(0)
		if pos.ExitCompletedAt != nil {
			duration = pos.ExitCompletedAt.Sub(pos.CreatedAt)
		} else {
			duration = now.Sub(pos.CreatedAt)
		}
		isEmergency := reason == "emergency_price_spike" || (reason == "margin_health_exit" && pos.MarginUtilizationPct > e.cfg.SpotFuturesMarginEmergencyPct)
		if isEmergency {
			e.telegram.NotifyEmergencyClose(pos, reason, totalPnL)
		} else {
			e.telegram.NotifyAutoExit(pos, reason, totalPnL, duration)
		}
	}
}

// ManualClose handles a user-initiated position close from the dashboard.
// It runs synchronously and returns an error if the close fails.
func (e *SpotEngine) ManualClose(positionID string) error {
	pos, err := e.db.GetSpotPosition(positionID)
	if err != nil {
		return fmt.Errorf("position not found: %w", err)
	}
	if pos == nil {
		return fmt.Errorf("position %s not found", positionID)
	}

	if pos.Status != models.SpotStatusActive {
		return fmt.Errorf("position %s status is %q, expected %q", positionID, pos.Status, models.SpotStatusActive)
	}

	if e.isExiting(positionID) {
		return fmt.Errorf("position %s is already being exited", positionID)
	}

	e.log.Info("ManualClose: %s %s on %s", pos.Symbol, positionID, pos.Exchange)

	// Run synchronously (not in goroutine) since this is a manual action.
	e.initiateExit(pos, "manual_close", false)

	// Check if it succeeded by verifying the position reached closed status.
	updated, err := e.db.GetSpotPosition(positionID)
	if err != nil {
		return fmt.Errorf("failed to verify close result: %w", err)
	}
	if updated.Status != models.SpotStatusClosed {
		return fmt.Errorf("close failed — position %s stuck in status %q, manual intervention required", positionID, updated.Status)
	}

	return nil
}
