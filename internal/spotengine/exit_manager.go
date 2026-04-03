package spotengine

import (
	"errors"
	"fmt"
	"math"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
)

// exitState tracks in-flight exits to prevent double-triggering.
// Protected by SpotEngine.exitMu.
type exitState struct {
	exiting map[string]bool // posID → true if exit in progress
}

// checkExitTriggers evaluates exit triggers in priority order for a position.
// Returns the reason string and whether this is an emergency (parallel close).
// An empty reason means no trigger fired.
//
// Priority ordering:
//
//	Phase 1: ALWAYS-ON SAFETY TRIGGERS (bypass all guards)
//	  1. Price Spike
//	  2. Margin Health
//	Phase 2: GUARD GATES (block yield-based triggers only)
//	  3. Min-hold gate
//	  4. Settlement window guard
//	  5. Exit spread gate
//	Phase 3: YIELD-BASED TRIGGERS (gated by Phase 2)
//	  6. Borrow Cost Drift (Dir A only)
//	  7. Funding Rate Drop
func (e *SpotEngine) checkExitTriggers(pos *models.SpotFuturesPosition) (reason string, isEmergency bool) {
	isDirA := pos.Direction == "borrow_sell_long"

	// ===============================================================
	// PHASE 1: ALWAYS-ON SAFETY TRIGGERS (bypass all guards)
	// ===============================================================

	// ---------------------------------------------------------------
	// 1. Price Spike
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
					// Direction A (long futures, short spot):
					//   UP move -> borrow buy-back cost risk
					//   DOWN move -> long futures liquidation risk
					if movePct > priceEmergencyPct || movePct < -priceEmergencyPct {
						e.log.Error("exit trigger: %s EMERGENCY price move %.1f%% (entry=%.4f now=%.4f)",
							pos.Symbol, movePct, pos.FuturesEntry, currentPrice)
						return "emergency_price_spike", true
					}
					if movePct > priceExitPct || movePct < -priceExitPct {
						e.log.Warn("exit trigger: %s price move %.1f%% (entry=%.4f now=%.4f)",
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
	// 2. Margin Health
	//    Direction A: spot-margin borrow utilization
	//    Direction B: futures margin ratio from GetFuturesBalance
	// ---------------------------------------------------------------
	marginExitPct := e.cfg.SpotFuturesMarginExitPct
	if marginExitPct <= 0 {
		marginExitPct = 85.0
	}
	marginEmergencyPct := e.cfg.SpotFuturesMarginEmergencyPct
	if marginEmergencyPct <= 0 {
		marginEmergencyPct = 95.0
	}

	if isDirA {
		smExch, ok := e.spotMargin[pos.Exchange]
		if ok && pos.BorrowAmount > 0 {
			// For borrow_sell_long, collateral is USDT (from selling borrowed coins),
			// not the base coin (which is 0 after selling). Check USDT balance.
			mb, err := smExch.GetMarginBalance("USDT")
			if err == nil {
				price := pos.SpotEntryPrice
				if price <= 0 {
					price = pos.FuturesEntry
				}
				if price > 0 {
					borrowedValue := pos.BorrowAmount * price
					availableValue := mb.Available
					if availableValue <= 0 && borrowedValue > 0 {
						// No USDT collateral with outstanding borrow = immediate emergency
						pos.MarginUtilizationPct = 999.0
						e.log.Error("exit trigger: %s EMERGENCY no available USDT collateral for borrow (%.4f %s borrowed, value=%.2f)",
							pos.Symbol, pos.BorrowAmount, pos.BaseCoin, borrowedValue)
						return "margin_health_exit", true
					}
					if availableValue > 0 {
						utilPct := borrowedValue / availableValue * 100
						pos.MarginUtilizationPct = utilPct

						if utilPct > marginEmergencyPct {
							e.log.Error("exit trigger: %s EMERGENCY margin utilization %.1f%% > %.1f%% (borrowed=%.2f avail=%.2f USDT)",
								pos.Symbol, utilPct, marginEmergencyPct, borrowedValue, availableValue)
							return "margin_health_exit", true
						}
						if utilPct > marginExitPct {
							e.log.Warn("exit trigger: %s margin utilization %.1f%% > %.1f%% (borrowed=%.2f avail=%.2f USDT)",
								pos.Symbol, utilPct, marginExitPct, borrowedValue, availableValue)
							return "margin_health_exit", false
						}
					}
				}
			} else {
				e.log.Warn("exit check: %s GetMarginBalance(%s) failed: %v", pos.Symbol, pos.BaseCoin, err)
			}
		}

		// Also check futures margin for Direction A (long futures can be liquidated).
		futExch, futOk := e.exchanges[pos.Exchange]
		if futOk {
			bal, err := futExch.GetFuturesBalance()
			if err == nil && bal.MarginRatio > 0 {
				futUtilPct := bal.MarginRatio * 100
				if futUtilPct > pos.MarginUtilizationPct {
					pos.MarginUtilizationPct = futUtilPct
				}

				if futUtilPct > marginEmergencyPct {
					e.log.Error("exit trigger: %s EMERGENCY futures margin ratio %.1f%% > %.1f%%",
						pos.Symbol, futUtilPct, marginEmergencyPct)
					return "margin_health_exit", true
				}
				if futUtilPct > marginExitPct {
					e.log.Warn("exit trigger: %s futures margin ratio %.1f%% > %.1f%%",
						pos.Symbol, futUtilPct, marginExitPct)
					return "margin_health_exit", false
				}
			} else if err != nil {
				e.log.Warn("exit check: %s GetFuturesBalance failed: %v", pos.Symbol, err)
			}
		}
	} else {
		// Direction B: check futures-side margin ratio.
		futExch, ok := e.exchanges[pos.Exchange]
		if ok {
			bal, err := futExch.GetFuturesBalance()
			if err == nil && bal.MarginRatio > 0 {
				utilPct := bal.MarginRatio * 100
				pos.MarginUtilizationPct = utilPct

				if utilPct > marginEmergencyPct {
					e.log.Error("exit trigger: %s EMERGENCY futures margin ratio %.1f%% > %.1f%%",
						pos.Symbol, utilPct, marginEmergencyPct)
					return "margin_health_exit", true
				}
				if utilPct > marginExitPct {
					e.log.Warn("exit trigger: %s futures margin ratio %.1f%% > %.1f%%",
						pos.Symbol, utilPct, marginExitPct)
					return "margin_health_exit", false
				}
			} else if err != nil {
				e.log.Warn("exit check: %s GetFuturesBalance failed: %v", pos.Symbol, err)
			}
		}
	}

	// ===============================================================
	// PHASE 2: GUARD GATES (block yield-based triggers only)
	// ===============================================================

	// ---------------------------------------------------------------
	// 3. Min-hold gate (per D-06)
	// ---------------------------------------------------------------
	if e.cfg.SpotFuturesEnableMinHold {
		minHold := time.Duration(e.cfg.SpotFuturesMinHoldHours) * time.Hour
		if minHold > 0 && time.Since(pos.CreatedAt) < minHold {
			e.log.Info("exit-guard: %s min-hold active (age=%s, required=%s), deferring yield exit",
				pos.Symbol, time.Since(pos.CreatedAt).Round(time.Minute), minHold)
			return "", false
		}
	}

	// ---------------------------------------------------------------
	// 4. Settlement window guard (per D-07)
	// ---------------------------------------------------------------
	if e.cfg.SpotFuturesEnableSettlementGuard {
		windowMin := e.cfg.SpotFuturesSettlementWindowMin
		if windowMin <= 0 {
			windowMin = 10
		}
		if isInSettlementWindow(windowMin) {
			e.log.Info("exit-guard: %s settlement window active (%d min), deferring yield exit",
				pos.Symbol, windowMin)
			return "", false
		}
	}

	// ---------------------------------------------------------------
	// 5. Exit spread gate (per D-11)
	// ---------------------------------------------------------------
	if e.cfg.SpotFuturesEnableExitSpreadGate {
		slippagePct := e.estimateUnwindSlippage(pos)
		maxSlippage := e.cfg.SpotFuturesExitSpreadPct
		if maxSlippage <= 0 {
			maxSlippage = 0.3
		}
		if slippagePct > maxSlippage {
			e.log.Info("exit-guard: %s spread %.2f%% > max %.2f%%, deferring yield exit",
				pos.Symbol, slippagePct, maxSlippage)
			return "", false
		}
	}

	// ===============================================================
	// PHASE 3: YIELD-BASED TRIGGERS (gated by Phase 2)
	// ===============================================================

	// ---------------------------------------------------------------
	// 6. Borrow Cost Drift (Direction A only)
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
	// 7. Funding Rate Drop
	// ---------------------------------------------------------------
	var currentFundingAPR, feePct float64
	var hasFundingData bool
	if opp, found := e.lookupCurrentOpp(pos.Symbol, pos.Exchange, pos.Direction); found {
		currentFundingAPR = opp.FundingAPR
		feePct = opp.FeePct
		hasFundingData = true
	} else {
		// Symbol not in latest scan — fall back to entry-time data.
		currentFundingAPR = pos.FundingAPR
		feePct = pos.FeePct
		hasFundingData = currentFundingAPR > 0
	}
	// Last-resort feePct: calculate from spotFees if position predates FeePct field.
	if feePct == 0 {
		takerFee := spotFees[pos.Exchange]
		if takerFee == 0 {
			takerFee = 0.0005
		}
		feePct = takerFee * 4
	}
	if hasFundingData {
		borrowAPR := pos.CurrentBorrowAPR
		if !isDirA {
			borrowAPR = 0 // Direction B has no borrow
		}
		minNet := e.cfg.SpotFuturesMinNetYieldAPR
		netYield := currentFundingAPR - borrowAPR
		if netYield < minNet {
			e.log.Warn("exit trigger: %s net yield %.2f%% < min %.2f%% (funding=%.2f%% borrow=%.2f%% fee=%.2f%% one-time)",
				pos.Symbol, netYield*100, minNet*100,
				currentFundingAPR*100, borrowAPR*100, feePct*100)
			return "yield_below_minimum", false
		}
	}

	// ---------------------------------------------------------------
	// 8. No trigger
	// ---------------------------------------------------------------
	return "", false
}

// isInSettlementWindow returns true if the current UTC time is within windowMin
// minutes of a standard 8h funding settlement (:00 at hours 0, 8, 16 UTC).
func isInSettlementWindow(windowMin int) bool {
	now := time.Now().UTC()
	return isInSettlementWindowAt(now.Hour(), now.Minute(), windowMin)
}

// isInSettlementWindowAt checks if the given hour:minute (UTC) falls within
// windowMin minutes of a standard 8h funding settlement. Extracted for testability.
func isInSettlementWindowAt(hour, minute, windowMin int) bool {
	// Standard settlement: every 8 hours at :00 (00:00, 08:00, 16:00 UTC).
	// Settlement hours: 0, 8, 16 → hour%8 == 0.
	// Previous hours: 7, 15, 23 → hour%8 == 7.
	if hour%8 == 0 && minute < windowMin {
		return true
	}
	if hour%8 == 7 && minute >= 60-windowMin {
		return true
	}
	return false
}

// estimateUnwindSlippage estimates the percentage cost of unwinding a position
// by checking the top-of-book spread from the futures orderbook.
func (e *SpotEngine) estimateUnwindSlippage(pos *models.SpotFuturesPosition) float64 {
	exch, ok := e.exchanges[pos.Exchange]
	if !ok {
		return 0
	}
	ob, err := exch.GetOrderbook(pos.Symbol, 5)
	if err != nil || len(ob.Bids) == 0 || len(ob.Asks) == 0 {
		return 0
	}
	mid := (ob.Bids[0].Price + ob.Asks[0].Price) / 2
	if mid <= 0 {
		return 0
	}
	// 2 legs: spot + futures, each pays half-spread.
	spreadPct := (ob.Asks[0].Price - ob.Bids[0].Price) / mid * 100
	// Estimated total: 2 legs * spread (simplified; real slippage depends on size).
	return spreadPct * 2
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
	// Only reset ExitRetryCount on fresh exits (not monitor retries).
	now := time.Now().UTC()
	isFreshExit := pos.Status != models.SpotStatusExiting
	err := e.lockedUpdatePosition(pos.ID, func(p *models.SpotFuturesPosition) bool {
		p.Status = models.SpotStatusExiting
		p.ExitReason = reason
		p.ExitTriggeredAt = &now
		if isFreshExit {
			p.ExitRetryCount = 0
		}
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
		// Fallback: persist any partial exit progress before returning.
		// The close methods checkpoint each leg individually, but this
		// catches any edge case where in-memory state advanced without
		// a prior checkpoint write.
		if pos.FuturesExit > 0 || pos.SpotExitFilledQty > 0 || pos.SpotExitPrice > 0 || pos.SpotExitFilled || pos.PendingSpotExitOrderID != "" {
			if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
				e.log.Error("initiateExit: fallback checkpoint failed for %s: %v", pos.ID, cpErr)
			}
		}
		var pendingErr *pendingSpotExitError
		if errors.As(err, &pendingErr) {
			e.log.Warn("initiateExit: %s awaiting spot exit confirmation on order %s: %v",
				pos.ID, pendingErr.orderID, pendingErr.err)
			return
		}
		e.log.Error("CRITICAL: ClosePosition failed for %s (%s): %v — position stuck in 'exiting', manual intervention required",
			pos.ID, pos.Symbol, err)
		return
	}

	// If repay is still pending (e.g. Bybit blackout), keep position in "exiting"
	// state and let the monitor loop retry repay on next tick.
	if pos.PendingRepay {
		if err := e.lockedUpdatePosition(pos.ID, func(p *models.SpotFuturesPosition) bool {
			p.PendingRepay = true
			p.PendingRepayRetryAt = pos.PendingRepayRetryAt
			p.FuturesExit = pos.FuturesExit
			p.SpotExitFilledQty = pos.SpotExitFilledQty
			p.SpotExitPrice = pos.SpotExitPrice
			p.SpotExitFilled = pos.SpotExitFilled || spotExitComplete(pos)
			p.PendingSpotExitOrderID = pos.PendingSpotExitOrderID
			p.ExitFees = pos.ExitFees
			return true
		}); err != nil {
			e.log.Error("initiateExit: failed to persist PendingRepay for %s: %v", pos.ID, err)
		}
		if pos.PendingRepayRetryAt != nil {
			e.log.Warn("initiateExit: %s trade legs closed but repay deferred until %s (blackout)",
				pos.ID, pos.PendingRepayRetryAt.Format(time.RFC3339))
		} else {
			e.log.Warn("initiateExit: %s trade legs closed but repay pending — will retry on next monitor tick", pos.ID)
		}
		return
	}

	// Close succeeded — run post-exit cleanup.
	e.completeExit(pos, reason)
}

// persistExitCheckpoint durably persists exit-leg progress to Redis so that
// monitor retries skip already-closed legs. Only writes fields that have
// progressed beyond what Redis already holds.
func (e *SpotEngine) persistExitCheckpoint(pos *models.SpotFuturesPosition) error {
	return e.lockedUpdatePosition(pos.ID, func(p *models.SpotFuturesPosition) bool {
		changed := false
		if pos.FuturesExit > 0 && p.FuturesExit == 0 {
			p.FuturesExit = pos.FuturesExit
			changed = true
		}
		if pos.SpotExitFilledQty > p.SpotExitFilledQty {
			p.SpotExitFilledQty = pos.SpotExitFilledQty
			changed = true
		}
		if (pos.SpotExitFilled || spotQtyComplete(pos.SpotExitFilledQty, pos.SpotSize)) && !p.SpotExitFilled {
			p.SpotExitFilled = true
			changed = true
		}
		if pos.SpotExitPrice > 0 && math.Abs(pos.SpotExitPrice-p.SpotExitPrice) > spotQtyTolerance {
			p.SpotExitPrice = pos.SpotExitPrice
			changed = true
		}
		if pos.PendingSpotExitOrderID != p.PendingSpotExitOrderID {
			p.PendingSpotExitOrderID = pos.PendingSpotExitOrderID
			changed = true
		}
		if pos.ExitFees > 0 {
			p.ExitFees = pos.ExitFees
			changed = true
		}
		if pos.PendingRepay && !p.PendingRepay {
			p.PendingRepay = true
			changed = true
		}
		if pos.PendingRepayRetryAt != nil && p.PendingRepayRetryAt == nil {
			p.PendingRepayRetryAt = pos.PendingRepayRetryAt
			changed = true
		}
		return changed
	})
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
		p.SpotExitFilledQty = pos.SpotExitFilledQty
		p.SpotExitFilled = pos.SpotExitFilled || spotExitComplete(pos)
		p.SpotExitPrice = pos.SpotExitPrice
		p.PendingSpotExitOrderID = ""
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
		// Record analytics snapshot for spot position close.
		if e.snapshotWriter != nil {
			e.snapshotWriter.RecordSpotClose(updated)
		}
	}

	// Update stats.
	if err := e.db.UpdateSpotStats(totalPnL, totalPnL >= 0); err != nil {
		e.log.Error("completeExit: failed to update stats for %s: %v", pos.ID, err)
	}
	e.releaseSpotPosition(pos.ID)
	e.borrowVelocity.Delete(pos.ID)

	// Transfer remaining USDT back to futures for separate-account exchanges.
	// Dir A used the margin account; Dir B used the spot account.
	if needsMarginTransfer(pos.Exchange) {
		if smExch, ok := e.spotMargin[pos.Exchange]; ok {
			if isDirA {
				if mb, mbErr := smExch.GetMarginBalance("USDT"); mbErr == nil && mb.Available > 1.0 {
					transferAmt := fmt.Sprintf("%.2f", mb.Available)
					if tfErr := smExch.TransferFromMargin("USDT", transferAmt); tfErr != nil {
						e.log.Warn("completeExit: TransferFromMargin(%s USDT) on %s: %v", transferAmt, pos.Exchange, tfErr)
					} else {
						e.log.Info("completeExit: transferred %s USDT from margin back to futures on %s", transferAmt, pos.Exchange)
					}
				}
			} else {
				// Dir B: USDT is in the spot account after selling.
				// Use TransferToFutures (spot → futures) from the Exchange interface.
				if exch, ok := e.exchanges[pos.Exchange]; ok {
					transferAmt := fmt.Sprintf("%.2f", pos.NotionalUSDT*1.05)
					if tfErr := exch.TransferToFutures("USDT", transferAmt); tfErr != nil {
						e.log.Warn("completeExit: TransferToFutures(%s USDT) on %s: %v", transferAmt, pos.Exchange, tfErr)
					} else {
						e.log.Info("completeExit: transferred %s USDT from spot back to futures on %s", transferAmt, pos.Exchange)
					}
				}
			}
		}
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
	if updated != nil && e.api != nil {
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

	if pos.Status == models.SpotStatusPending && pos.ExitReason == spotEntryManualRecoveryReason {
		return e.resolveManualRecovery(positionID)
	}

	if pos.Status != models.SpotStatusActive {
		return fmt.Errorf("position %s not active: status %q", positionID, pos.Status)
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
		if updated.PendingRepay {
			return fmt.Errorf("trade legs closed but margin repay pending (e.g. Bybit blackout) — will auto-retry on next monitor tick")
		}
		return fmt.Errorf("close failed — position %s stuck in status %q, manual intervention required", positionID, updated.Status)
	}

	return nil
}

func (e *SpotEngine) resolveManualRecovery(positionID string) error {
	mu := e.posLock(positionID)
	mu.Lock()
	defer mu.Unlock()

	pos, err := e.db.GetSpotPosition(positionID)
	if err != nil {
		return fmt.Errorf("position not found: %w", err)
	}
	if pos == nil {
		return fmt.Errorf("position %s not found", positionID)
	}
	if pos.Status != models.SpotStatusPending || pos.ExitReason != spotEntryManualRecoveryReason {
		return fmt.Errorf("position %s not pending manual recovery", positionID)
	}

	smExch, ok := e.spotMargin[pos.Exchange]
	if !ok {
		return fmt.Errorf("exchange %s does not support spot margin", pos.Exchange)
	}

	if pos.PendingSpotExitOrderID != "" {
		querier, ok := smExch.(exchange.SpotMarginOrderQuerier)
		if !ok {
			return fmt.Errorf("manual recovery order query unsupported for %s", pos.Exchange)
		}
		status, err := querier.GetSpotMarginOrder(pos.PendingSpotExitOrderID, pos.Symbol)
		if err != nil {
			return fmt.Errorf("manual recovery cleanup order check failed: %w", err)
		}
		if status == nil {
			return fmt.Errorf("manual recovery cleanup order %s not found", pos.PendingSpotExitOrderID)
		}
		if isActiveSpotOrderStatus(status.Status) {
			return fmt.Errorf(
				"manual recovery cleanup order %s still active: status=%s filled=%.6f",
				pos.PendingSpotExitOrderID,
				status.Status,
				status.FilledQty,
			)
		}
	}

	bal, err := smExch.GetMarginBalance(pos.BaseCoin)
	if err != nil {
		return fmt.Errorf("manual recovery balance check failed: %w", err)
	}

	liability := bal.Borrowed + bal.Interest
	if math.Abs(liability) > spotQtyTolerance || math.Abs(bal.TotalBalance) > spotQtyTolerance {
		return fmt.Errorf(
			"manual recovery still open on exchange: total=%.6f available=%.6f borrowed=%.6f interest=%.6f",
			bal.TotalBalance,
			bal.Available,
			bal.Borrowed,
			bal.Interest,
		)
	}

	now := time.Now().UTC()
	pos.Status = models.SpotStatusClosed
	pos.ExitCompletedAt = &now
	pos.PendingEntryOrderID = ""
	pos.PendingSpotExitOrderID = ""
	pos.PendingRepay = false
	pos.PendingRepayRetryAt = nil
	pos.UpdatedAt = now

	if err := e.db.SaveSpotPosition(pos); err != nil {
		return fmt.Errorf("failed to clear manual recovery position: %w", err)
	}

	e.releaseSpotPosition(pos.ID)
	e.borrowVelocity.Delete(pos.ID)
	if e.api != nil {
		e.api.BroadcastSpotPositionUpdate(pos)
	}

	e.log.Info("ManualClose: cleared manual recovery %s %s on %s after operator flatten confirmation",
		pos.Symbol, pos.ID, pos.Exchange)
	return nil
}
