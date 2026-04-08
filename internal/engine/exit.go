package engine

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// checkIntervalChanges queries both legs of every active position for their
// current funding interval. If the two legs now have different intervals,
// the position is flagged for exit via spawnExitGoroutine.
// Called before checkExitsV2 on each ExitScan cycle.
func (e *Engine) checkIntervalChanges() {
	positions, err := e.db.GetActivePositions()
	if err != nil {
		e.log.Error("interval check: failed to get active positions: %v", err)
		return
	}

	for _, pos := range positions {
		if pos.Status != models.StatusActive {
			continue
		}

		// Skip if an exit goroutine is already running.
		e.exitMu.Lock()
		running := e.exitActive[pos.ID]
		e.exitMu.Unlock()
		if running {
			continue
		}

		// Settlement-window guard: skip within ±10min of funding (rates unreliable).
		if !pos.NextFunding.IsZero() {
			untilFunding := time.Until(pos.NextFunding)
			sinceFunding := -untilFunding
			if (untilFunding > 0 && untilFunding < 10*time.Minute) ||
				(sinceFunding > 0 && sinceFunding < 10*time.Minute) {
				continue
			}
		}

		longExch, ok := e.exchanges[pos.LongExchange]
		if !ok {
			continue
		}
		shortExch, ok := e.exchanges[pos.ShortExchange]
		if !ok {
			continue
		}

		// Use GetFundingRate (returns rate + interval) instead of GetFundingInterval.
		longRate, err := longExch.GetFundingRate(pos.Symbol)
		if err != nil {
			continue
		}
		shortRate, err := shortExch.GetFundingRate(pos.Symbol)
		if err != nil {
			continue
		}

		longInterval := longRate.Interval
		shortInterval := shortRate.Interval

		// Skip if either interval is zero/unset (API failure).
		if longInterval <= 0 || shortInterval <= 0 {
			continue
		}

		diff := longInterval - shortInterval
		if diff < 0 {
			diff = -diff
		}
		if diff <= 30*time.Minute {
			continue // intervals match, nothing to do
		}

		if !e.cfg.AllowMixedIntervals {
			// Strict mode: exit on any interval mismatch.
			reason := fmt.Sprintf("interval mismatch: %s=%v %s=%v",
				pos.LongExchange, longInterval, pos.ShortExchange, shortInterval)
			e.log.Info("interval check: %s — %s, triggering exit", pos.ID, reason)
			e.spawnExitGoroutine(pos, reason)
			continue
		}

		// AllowMixedIntervals: check if spread is still positive before exiting.
		longIntervalH := longInterval.Hours()
		shortIntervalH := shortInterval.Hours()

		longBpsH := longRate.Rate * 10000 / longIntervalH
		shortBpsH := shortRate.Rate * 10000 / shortIntervalH
		currentSpread := shortBpsH - longBpsH

		if currentSpread > 0 {
			// Spread is still positive — keep the position.
			// Update NextFunding to the collecting side's settlement time.
			var collectingNextFunding time.Time
			if longBpsH < 0 && shortBpsH < 0 {
				collectingNextFunding = longRate.NextFunding // long collects
			} else if longBpsH >= 0 && shortBpsH >= 0 {
				collectingNextFunding = shortRate.NextFunding // short collects
			} else {
				// Mixed signs — use earliest
				if longRate.NextFunding.Before(shortRate.NextFunding) {
					collectingNextFunding = longRate.NextFunding
				} else {
					collectingNextFunding = shortRate.NextFunding
				}
			}
			if !collectingNextFunding.IsZero() {
				_ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
					fresh.NextFunding = collectingNextFunding
					return true
				})
			}

			e.log.Info("interval check: %s — intervals diverged (%s=%v %s=%v) but spread=%.2f bps/h still positive, keeping",
				pos.ID, pos.LongExchange, longInterval, pos.ShortExchange, shortInterval, currentSpread)
			continue
		}

		// Spread is negative or zero — exit.
		reason := fmt.Sprintf("interval mismatch + spread negative: %s=%v %s=%v spread=%.2f bps/h",
			pos.LongExchange, longInterval, pos.ShortExchange, shortInterval, currentSpread)
		e.log.Info("interval check: %s — %s, triggering exit", pos.ID, reason)
		e.spawnExitGoroutine(pos, reason)
	}
}

// checkExitsV2 evaluates all active positions for exit conditions.
// Called on every scan result (every ~10 minutes).
func (e *Engine) checkExitsV2() {
	positions, err := e.db.GetActivePositions()
	if err != nil {
		e.log.Error("exit check: failed to get active positions: %v", err)
		return
	}

	for _, pos := range positions {
		if pos.Status != models.StatusActive {
			continue
		}

		// Skip if an exit goroutine is already running for this position.
		e.exitMu.Lock()
		running := e.exitActive[pos.ID]
		e.exitMu.Unlock()
		if running {
			continue
		}

		// Safety: spread reversal triggers exit (with optional tolerance).
		// This check is BEFORE the min-hold gate — a reversed spread is a safety
		// measure that must not be blocked by the min-hold requirement.
		if e.cfg.EnableSpreadReversal {
			if reversed, reason := e.checkSpreadReversal(pos); reversed {
				tolerance := e.cfg.SpreadReversalTolerance
				if tolerance > 0 && pos.ReversalCount < tolerance {
					pos.ReversalCount++
					if err := e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
						fresh.ReversalCount = pos.ReversalCount
						return true
					}); err != nil {
						e.log.Error("failed to update reversal count for %s: %v", pos.ID, err)
					}
					e.log.Info("exit check: %s — %s (reversal %d/%d, tolerating)", pos.ID, reason, pos.ReversalCount, tolerance)
					continue
				}
				e.log.Info("exit check: %s — %s (reversal %d, exiting)", pos.ID, reason, pos.ReversalCount+1)
				e.spawnExitGoroutine(pos, reason)
				continue
			}
		}

		// Zero-spread safety: exit if both legs have equal funding rate for too long.
		// When tolerance is reached, schedule a post-settlement confirmation instead
		// of exiting immediately — rates often diverge again after settlement.
		if e.cfg.ZeroSpreadTolerance > 0 {
			if zeroSpread, zReason := e.checkZeroSpread(pos); zeroSpread {
				if pos.ZeroSpreadCount+1 >= e.cfg.ZeroSpreadTolerance {
					e.log.Info("exit check: %s — %s (zero-spread %d/%d, scheduling post-settlement confirm)", pos.ID, zReason, pos.ZeroSpreadCount+1, e.cfg.ZeroSpreadTolerance)
					e.schedulePostSettlementZeroCheck(pos)
					continue
				}
				pos.ZeroSpreadCount++
				if err := e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
					fresh.ZeroSpreadCount = pos.ZeroSpreadCount
					return true
				}); err != nil {
					e.log.Error("failed to update zero-spread count for %s: %v", pos.ID, err)
				}
				e.log.Info("exit check: %s — %s (zero-spread %d/%d, tolerating)", pos.ID, zReason, pos.ZeroSpreadCount, e.cfg.ZeroSpreadTolerance)
				continue
			} else if pos.ZeroSpreadCount > 0 {
				pos.ZeroSpreadCount = 0
				if err := e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
					fresh.ZeroSpreadCount = 0
					return true
				}); err != nil {
					e.log.Error("failed to reset zero-spread count for %s: %v", pos.ID, err)
				}
				e.log.Info("exit check: %s — spread diverged, zero-spread count reset", pos.ID)
			}
		}

		// Min-hold gate: don't exit before first funding settlement.
		// Prevents exiting before collecting any revenue (guaranteed loss).
		// Manual close and margin emergencies bypass checkExitsV2 entirely.
		if !pos.NextFunding.IsZero() && time.Now().Before(pos.NextFunding) {
			continue
		}

		// Skip spread evaluation within ±10min of funding settlement (rates unreliable).
		if !pos.NextFunding.IsZero() {
			untilFunding := time.Until(pos.NextFunding)
			sinceFunding := -untilFunding
			if (untilFunding > 0 && untilFunding < 10*time.Minute) ||
				(sinceFunding > 0 && sinceFunding < 10*time.Minute) {
				continue
			}
		}

	}
}

// spawnExitGoroutine launches a background goroutine to close a position
// using the depth-fill loop. The goroutine is cancellable via context for L4/L5 preemption.
func (e *Engine) spawnExitGoroutine(pos *models.ArbitragePosition, reason string) {
	e.exitMu.Lock()
	if e.exitActive[pos.ID] {
		e.exitMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.exitCancels[pos.ID] = cancel
	e.exitActive[pos.ID] = true
	done := make(chan struct{})
	e.exitDone[pos.ID] = done
	e.exitMu.Unlock()

	// Set status to "exiting" and store exit reason.
	_ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
		if fresh.Status != models.StatusActive {
			return false
		}
		fresh.Status = models.StatusExiting
		fresh.ExitReason = reason
		return true
	})
	e.api.BroadcastPositionUpdate(pos)

	go func() {
		defer func() {
			e.exitMu.Lock()
			delete(e.exitCancels, pos.ID)
			delete(e.exitActive, pos.ID)
			delete(e.exitDone, pos.ID)
			e.exitMu.Unlock()
			close(done)
		}()

		e.log.Info("exit goroutine started for %s: %s", pos.ID, reason)
		err := e.executeDepthExit(ctx, pos)
		if err != nil {
			if ctx.Err() != nil {
				e.log.Info("exit goroutine for %s cancelled (preempted by L4/L5)", pos.ID)
				return // L4/L5 will handle closure
			}
			e.log.Error("depth exit failed for %s: %v — falling back to smart close", pos.ID, err)
			if closeErr := e.closePosition(pos); closeErr != nil {
				e.log.Error("fallback close also failed for %s: %v", pos.ID, closeErr)
			}
		}
	}()
}

// executeDepthExit runs a depth-fill exit loop similar to executeTradeV2 but reversed.
// Sells on longExch (reduce-only), buys on shortExch (reduce-only).
// Falls back to market orders after ExitDepthTimeoutSec.
func (e *Engine) executeDepthExit(ctx context.Context, pos *models.ArbitragePosition) error {
	longExch, ok := e.exchanges[pos.LongExchange]
	if !ok {
		return fmt.Errorf("long exchange %s not found", pos.LongExchange)
	}
	shortExch, ok := e.exchanges[pos.ShortExchange]
	if !ok {
		return fmt.Errorf("short exchange %s not found", pos.ShortExchange)
	}

	// Use the local recorded sizes for THIS position, not the exchange total.
	// The exchange total may include sibling positions on the same symbol.
	totalLong := pos.LongSize
	totalShort := pos.ShortSize

	// Subscribe to depth.
	longExch.SubscribeDepth(pos.Symbol)
	shortExch.SubscribeDepth(pos.Symbol)
	defer longExch.UnsubscribeDepth(pos.Symbol)
	defer shortExch.UnsubscribeDepth(pos.Symbol)

	// Wait up to 8s for depth data with unsub/resub retry (mirrors entry pattern).
	depthReady := false
	for attempt := 0; attempt < 2; attempt++ {
		for i := 0; i < 40; i++ { // 4s per attempt
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			_, lok := longExch.GetDepth(pos.Symbol)
			_, sok := shortExch.GetDepth(pos.Symbol)
			if lok && sok {
				depthReady = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if depthReady {
			break
		}
		if attempt == 0 {
			e.log.Warn("depth exit %s: no depth after 4s, re-subscribing", pos.ID)
			longExch.UnsubscribeDepth(pos.Symbol)
			shortExch.UnsubscribeDepth(pos.Symbol)
			time.Sleep(200 * time.Millisecond)
			longExch.SubscribeDepth(pos.Symbol)
			shortExch.SubscribeDepth(pos.Symbol)
		}
	}

	var closedLong, closedShort float64
	var longVWAPSum, shortVWAPSum float64

	if !depthReady {
		e.log.Warn("depth exit %s: no depth after 8s, using market close directly", pos.ID)
	}

	var longConsecFails, shortConsecFails int
	const maxConsecFails = 5

	timeout := time.Duration(e.cfg.ExitDepthTimeoutSec) * time.Second
	startTime := time.Now()
	deadline := startTime.Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var gapRejected bool // only log gap rejection on state change
	initialGapBPS := 0.0
	baseGapBPS := e.cfg.ExitMaxGapBPS

	// Snapshot the starting gap so exits that begin in a wide market do not wait
	// for the ramp to catch up before the first fill attempts.
	if longDepth, lok := longExch.GetDepth(pos.Symbol); lok {
		if shortDepth, sok := shortExch.GetDepth(pos.Symbol); sok && len(longDepth.Bids) > 0 && len(shortDepth.Asks) > 0 {
			initialGapBPS = (shortDepth.Asks[0].Price/longDepth.Bids[0].Price - 1) * 10000
			baseGapBPS = math.Max(baseGapBPS, initialGapBPS)
			if initialGapBPS > e.cfg.ExitMaxGapBPS {
				e.log.Info("depth exit %s: initial gap %.1fbps > config %.1fbps, using dynamic baseline", pos.ID, initialGapBPS, e.cfg.ExitMaxGapBPS)
			}
		}
	}

	// Look up step/min size for unfillable remainder check.
	var exitStepSize, exitMinSize float64
	if e.contracts != nil {
		for _, exchName := range []string{pos.LongExchange, pos.ShortExchange} {
			if exContracts, ok := e.contracts[exchName]; ok {
				if ci, ok := exContracts[pos.Symbol]; ok {
					if ci.StepSize > exitStepSize {
						exitStepSize = ci.StepSize
					}
					if ci.MinSize > exitMinSize {
						exitMinSize = ci.MinSize
					}
				}
			}
		}
	}

	e.log.Info("depth exit for %s: longSize=%.6f shortSize=%.6f timeout=%v cfgGap=%.1fbps initialGap=%.1fbps baseGap=%.1fbps depthReady=%v",
		pos.ID, totalLong, totalShort, timeout, e.cfg.ExitMaxGapBPS, initialGapBPS, baseGapBPS, depthReady)

	lastFillTime := time.Now()

	if !depthReady {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		e.cancelStopLosses(pos)
		time.Sleep(500 * time.Millisecond)
		goto marketFallback
	}

exitLoop:
	for {
		longRemaining := totalLong - closedLong
		shortRemaining := totalShort - closedShort
		if longRemaining <= 0 && shortRemaining <= 0 {
			break
		}
		if time.Now().After(deadline) {
			e.log.Warn("depth exit: timeout for %s", pos.ID)
			break
		}
		if time.Since(lastFillTime) > 30*time.Second {
			e.log.Warn("depth exit: no fill progress for 30s for %s, falling back to market close", pos.ID)
			break
		}
		if longConsecFails >= maxConsecFails || shortConsecFails >= maxConsecFails {
			e.log.Error("depth exit: circuit breaker for %s (longFails=%d shortFails=%d)",
				pos.ID, longConsecFails, shortConsecFails)
			break
		}

		select {
		case <-ctx.Done():
			e.log.Info("depth exit: cancelled for %s", pos.ID)
			return ctx.Err()
		case <-e.stopCh:
			break exitLoop
		case <-ticker.C:
		}

		// Read depth: sell on longExch (bids), buy on shortExch (asks).
		longDepth, lok := longExch.GetDepth(pos.Symbol)
		shortDepth, sok := shortExch.GetDepth(pos.Symbol)
		if !lok || !sok {
			continue
		}
		if time.Since(longDepth.Time) > 5*time.Second || time.Since(shortDepth.Time) > 5*time.Second {
			continue
		}
		if len(longDepth.Bids) == 0 || len(shortDepth.Asks) == 0 {
			continue
		}

		// Cross-exchange exit gap: we SELL long (into bids), BUY short (from asks).
		bestBid := longDepth.Bids[0].Price
		bestAsk := shortDepth.Asks[0].Price
		exitGapBPS := (bestAsk/bestBid - 1) * 10000

		// Gap gate: single bounded ramp from baseGapBPS to 3x over the timeout.
		elapsed := time.Since(startTime).Seconds()
		totalSec := timeout.Seconds()
		relaxFactor := 1.0 + 2.0*(elapsed/totalSec)
		if relaxFactor > 3.0 {
			relaxFactor = 3.0
		}
		effectiveMaxGap := baseGapBPS * relaxFactor
		if effectiveMaxGap > 50 {
			// Hard cap at 50bps. If we've been capped for >60% of timeout, bail to market.
			effectiveMaxGap = 50
			if elapsed > totalSec*0.6 {
				e.log.Warn("depth exit %s: gap capped at 50bps for >60%% of timeout, breaking to market", pos.ID)
				break
			}
		}

		if exitGapBPS > effectiveMaxGap {
			if !gapRejected {
				e.log.Info("depth exit %s: gap=%.1fbps > limit=%.1fbps, waiting...",
					pos.ID, exitGapBPS, effectiveMaxGap)
				gapRejected = true
			}
			continue
		}
		if gapRejected {
			e.log.Info("depth exit %s: gap=%.1fbps recovered (limit=%.1fbps)",
				pos.ID, exitGapBPS, effectiveMaxGap)
			gapRejected = false
		}

		// Aggregate depth only within the gap threshold (mirror entry pattern).
		remaining := math.Min(longRemaining, shortRemaining)

		// Check unfillable remainder.
		if exitStepSize > 0 && remaining < exitStepSize {
			e.log.Info("depth exit %s: remaining %.6f < step %.6f, done", pos.ID, remaining, exitStepSize)
			break
		}
		if exitMinSize > 0 && remaining < exitMinSize {
			e.log.Info("depth exit %s: remaining %.6f < min %.6f, done", pos.ID, remaining, exitMinSize)
			break
		}

		var bidQty, askQty float64
		for _, lvl := range longDepth.Bids {
			levelGap := (bestAsk/lvl.Price - 1) * 10000
			if levelGap > effectiveMaxGap {
				break
			}
			bidQty += lvl.Quantity
		}
		for _, lvl := range shortDepth.Asks {
			levelGap := (lvl.Price/bestBid - 1) * 10000
			if levelGap > effectiveMaxGap {
				break
			}
			askQty += lvl.Quantity
		}

		if bidQty <= 0 || askQty <= 0 {
			continue
		}

		size := math.Min(remaining, math.Min(bidQty, askQty))

		// Round to step size.
		if exitStepSize > 0 {
			size = utils.RoundToStep(size, exitStepSize)
		}

		if size*bestBid < e.cfg.MinChunkUSDT {
			continue
		}

		// Sell long leg first (reduce-only).
		slippage := e.cfg.SlippageBPS / 10000.0
		sellPrice := e.formatPrice(pos.LongExchange, pos.Symbol, bestBid*(1-slippage))
		sizeStr := e.formatSize(pos.LongExchange, pos.Symbol, size)

		oid, err := longExch.PlaceOrder(exchange.PlaceOrderParams{
			Symbol:     pos.Symbol,
			Side:       exchange.SideSell,
			OrderType:  "limit",
			Price:      sellPrice,
			Size:       sizeStr,
			Force:      "ioc",
			ReduceOnly: true,
		})
		if err != nil {
			e.log.Warn("depth exit: long sell failed: %v", err)
			longConsecFails++
			continue
		}
		longConsecFails = 0
		filled, avg := e.confirmFill(longExch, oid, pos.Symbol)
		if filled <= 0 {
			continue
		}

		closedLong += filled
		if avg > 0 {
			longVWAPSum += avg * filled
		}

		// Buy to close short for matched qty (reduce-only).
		bestAsk = shortDepth.Asks[0].Price // refresh from current depth
		buyPrice := e.formatPrice(pos.ShortExchange, pos.Symbol, bestAsk*(1+slippage))
		buySize := e.formatSize(pos.ShortExchange, pos.Symbol, filled)

		oid2, err := shortExch.PlaceOrder(exchange.PlaceOrderParams{
			Symbol:     pos.Symbol,
			Side:       exchange.SideBuy,
			OrderType:  "limit",
			Price:      buyPrice,
			Size:       buySize,
			Force:      "ioc",
			ReduceOnly: true,
		})
		if err != nil {
			e.log.Warn("depth exit: short buy failed: %v", err)
			shortConsecFails++
			continue
		}
		shortConsecFails = 0
		filled2, avg2 := e.confirmFill(shortExch, oid2, pos.Symbol)

		closedShort += filled2
		if filled2 > 0 {
			lastFillTime = time.Now()
		}
		if avg2 > 0 {
			shortVWAPSum += avg2 * filled2
		}

		// If short didn't fully fill, do NOT decrement closedLong — the long
		// was already sold on exchange. Falsifying bookkeeping causes the market
		// fallback to re-sell shares that are already gone. The consolidator
		// will reconcile any small imbalance.
		if filled2 < filled {
			excess := filled - filled2
			e.log.Warn("depth exit %s: short under-filled by %.6f (long=%.6f short=%.6f), imbalance will be reconciled by consolidator",
				pos.ID, excess, filled, filled2)
		}

		e.log.Info("depth exit %s: tick closedLong=%.6f closedShort=%.6f",
			pos.ID, closedLong, closedShort)
	}

	// Check for L4/L5 preemption before cancelling SLs — keep SL protection if preempted.
	if ctx.Err() != nil {
		e.log.Info("depth exit: cancelled after depth loop for %s — SLs preserved for emergency handler", pos.ID)
		return ctx.Err()
	}

	// Cancel stop-loss orders after depth loop, before market fallback.
	// This keeps SLs tracked during depth-fill for safety, but cancels before
	// market orders to prevent SL triggering during aggressive close.
	e.cancelStopLosses(pos)
	time.Sleep(500 * time.Millisecond)

marketFallback:
	// Check for L4/L5 preemption before market fallback.
	if ctx.Err() != nil {
		e.log.Info("depth exit: cancelled before market fallback for %s", pos.ID)
		return ctx.Err()
	}

	// Market fallback for remainder.
	longRemaining := totalLong - closedLong
	shortRemaining := totalShort - closedShort

	if longRemaining > 0 || shortRemaining > 0 {
		e.log.Info("depth exit: market fallback for %s (longRem=%.6f shortRem=%.6f)",
			pos.ID, longRemaining, shortRemaining)
		if longRemaining > 0 {
			filled, avg := e.closeFullyWithRetryPriced(ctx, longExch, pos.Symbol, exchange.SideSell, longRemaining)
			if avg > 0 {
				longVWAPSum += avg * filled
			}
			closedLong += filled
		}
		if shortRemaining > 0 {
			filled, avg := e.closeFullyWithRetryPriced(ctx, shortExch, pos.Symbol, exchange.SideBuy, shortRemaining)
			if avg > 0 {
				shortVWAPSum += avg * filled
			}
			closedShort += filled
		}
	}

	// Compute VWAP close prices.
	var longClosePrice, shortClosePrice float64
	if closedLong > 0 {
		longClosePrice = longVWAPSum / closedLong
	}
	if closedShort > 0 {
		shortClosePrice = shortVWAPSum / closedShort
	}

	// Fall back to BBO if close price is missing.
	if longClosePrice <= 0 && totalLong > 0 {
		if bbo, ok := longExch.GetBBO(pos.Symbol); ok && bbo.Bid > 0 {
			longClosePrice = (bbo.Bid + bbo.Ask) / 2
		} else {
			longClosePrice = pos.LongEntry
		}
	}
	if shortClosePrice <= 0 && totalShort > 0 {
		if bbo, ok := shortExch.GetBBO(pos.Symbol); ok && bbo.Ask > 0 {
			shortClosePrice = (bbo.Bid + bbo.Ask) / 2
		} else {
			shortClosePrice = pos.ShortEntry
		}
	}

	// Check for incomplete close (partial fill or ctx cancelled).
	longRemainder := totalLong - closedLong
	shortRemainder := totalShort - closedShort
	fullyFlat := longRemainder <= 0 && shortRemainder <= 0

	if !fullyFlat {
		// If cancelled by L4/L5, do NOT modify position state — emergency handler owns it now.
		if ctx.Err() != nil {
			e.log.Info("depth exit %s: cancelled during partial close (longRem=%.6f shortRem=%.6f) — deferring to emergency handler",
				pos.ID, longRemainder, shortRemainder)
			return ctx.Err()
		}
		e.log.Error("CRITICAL: depth-exit %s: NOT fully flat — longRemainder=%.6f shortRemainder=%.6f — manual intervention needed",
			pos.ID, longRemainder, shortRemainder)
	}

	// Calculate PnL using actual closed quantities.
	longPnL := (longClosePrice - pos.LongEntry) * closedLong
	shortPnL := (pos.ShortEntry - shortClosePrice) * closedShort
	realizedPnL := longPnL + shortPnL + pos.FundingCollected - pos.EntryFees

	// Sanity check: PnL should not exceed position notional value.
	// If it does, the close price is likely wrong — fall back to 0 PnL.
	notional := math.Max(pos.LongEntry*totalLong, pos.ShortEntry*totalShort)
	if notional > 0 && math.Abs(realizedPnL) > notional*2 {
		e.log.Error("depth-exit %s: PnL %.4f exceeds 2x notional %.4f — close prices suspect (longClose=%.8f shortClose=%.8f), zeroing PnL",
			pos.ID, realizedPnL, notional, longClosePrice, shortClosePrice)
		realizedPnL = pos.FundingCollected - pos.EntryFees // keep only funding minus entry fees as PnL
		longPnL = 0
		shortPnL = 0
	}

	if fullyFlat {
		// Cancel orphan TP/SL/algo orders BEFORE marking closed — prevents race
		// where a new entry re-uses the symbol and the async cancel wipes its orders.
		longExch.CancelAllOrders(pos.Symbol)
		shortExch.CancelAllOrders(pos.Symbol)

		if err := e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
			fresh.RealizedPnL = realizedPnL
			fresh.LongExit = longClosePrice
			fresh.ShortExit = shortClosePrice
			fresh.Status = models.StatusClosed
			fresh.UpdatedAt = time.Now().UTC()
			fresh.LongSize = 0
			fresh.ShortSize = 0
			return true
		}); err != nil {
			e.log.Error("failed to save closed position %s: %v", pos.ID, err)
		}
	} else {
		// Partial close — only update remaining sizes, revert to active for retry.
		// Do NOT write partial PnL/exit prices to avoid accumulation errors on retry.
		if err := e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
			fresh.Status = models.StatusActive
			fresh.LongSize = longRemainder
			fresh.ShortSize = shortRemainder
			fresh.UpdatedAt = time.Now().UTC()
			return true
		}); err != nil {
			e.log.Error("failed to save partial-closed position %s: %v", pos.ID, err)
		}
	}

	// Update local pos for broadcast.
	if fullyFlat {
		pos.RealizedPnL = realizedPnL
		pos.LongExit = longClosePrice
		pos.ShortExit = shortClosePrice
		pos.LongSize = 0
		pos.ShortSize = 0
		pos.Status = models.StatusClosed

		if err := e.db.AddToHistory(pos); err != nil {
			e.log.Error("failed to add to history: %v", err)
		}
		won := realizedPnL > 0
		if err := e.db.UpdateStats(realizedPnL, won); err != nil {
			e.log.Error("failed to update stats: %v", err)
		}
		if e.lossLimiter != nil {
			e.lossLimiter.RecordClosedPnL(pos.ID, realizedPnL, pos.Symbol, time.Now().UTC())
		}

		e.log.Info("position %s depth-exit closed: pnl=%.4f (long=%.4f short=%.4f funding=%.4f entryFees=%.4f)",
			pos.ID, realizedPnL, longPnL, shortPnL, pos.FundingCollected, pos.EntryFees)

		// Set symbol cooldown on loss close.
		if realizedPnL < 0 && e.cfg.LossCooldownHours > 0 {
			cooldown := time.Duration(e.cfg.LossCooldownHours * float64(time.Hour))
			e.discovery.SetSymbolCooldown(pos.Symbol, cooldown)
		}
		// Set re-entry cooldown on any close.
		if e.cfg.ReEnterCooldownHours > 0 {
			cooldown := time.Duration(e.cfg.ReEnterCooldownHours * float64(time.Hour))
			e.discovery.SetReEnterCooldown(pos.Symbol, cooldown)
		}

		// Broadcast before reconcile — reconcilePnL will broadcast corrected data if PnL changes.
		e.api.BroadcastPositionUpdate(pos)

		// Reconcile PnL from exchange trade history (immediate attempt is synchronous).
		posCopy := *pos
		e.reconcilePnL(&posCopy)
	} else {
		pos.Status = models.StatusActive
		pos.LongSize = longRemainder
		pos.ShortSize = shortRemainder
		e.log.Error("position %s depth-exit PARTIAL: longRem=%.6f shortRem=%.6f — reverted to active for retry",
			pos.ID, longRemainder, shortRemainder)

		// Clear stale SL and TP order IDs before re-attaching.
		_ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
			fresh.LongSLOrderID = ""
			fresh.ShortSLOrderID = ""
			fresh.LongTPOrderID = ""
			fresh.ShortTPOrderID = ""
			return true
		})
		pos.LongSLOrderID = ""
		pos.ShortSLOrderID = ""
		pos.LongTPOrderID = ""
		pos.ShortTPOrderID = ""

		// Re-attach stop-losses for the remaining position.
		e.attachStopLosses(pos)

		e.api.BroadcastPositionUpdate(pos)
	}

	return nil
}

// reconcilePnL queries actual trade history from all exchanges involved in
// this position (including rotated-away exchanges) and recomputes realized PnL
// from real fill data. Runs async after position close. Updates the position
// record and stats if different. Retries up to 3 times on failure.
func (e *Engine) reconcilePnL(pos *models.ArbitragePosition) {
	e.pnlReconcileMu.Lock()

	// Immediate attempt: wait 2s for exchange to finalize, then try once.
	time.Sleep(2 * time.Second)
	if e.tryReconcilePnL(pos, 1) {
		e.pnlReconcileMu.Unlock()
		return
	}
	e.pnlReconcileMu.Unlock()

	// Async retries if immediate attempt failed.
	go func() {
		delays := []time.Duration{5 * time.Second, 15 * time.Second}
		for i, d := range delays {
			time.Sleep(d)
			e.pnlReconcileMu.Lock()
			ok := e.tryReconcilePnL(pos, i+2)
			e.pnlReconcileMu.Unlock()
			if ok {
				return
			}
		}
		e.log.Error("reconcile %s: all attempts failed, keeping PnL=%.4f", pos.ID, pos.RealizedPnL)
	}()
}

// tryReconcilePnL performs a single reconciliation attempt using exchange-native
// position close PnL APIs. Returns true on success, false if data was incomplete.
func (e *Engine) tryReconcilePnL(pos *models.ArbitragePosition, attempt int) bool {
	since := pos.CreatedAt.Add(-1 * time.Minute)

	// Only query the current two legs. Rotated-away leg PnL is already captured
	// in pos.RotationPnL at rotation time — no need to re-query.
	longExch, lok := e.exchanges[pos.LongExchange]
	shortExch, sok := e.exchanges[pos.ShortExchange]
	if !lok || !sok {
		e.log.Warn("reconcile %s [attempt %d]: exchange not found (long=%s short=%s)",
			pos.ID, attempt, pos.LongExchange, pos.ShortExchange)
		return true // don't retry missing exchanges
	}

	// Query exchange-native close PnL for each leg.
	longPnLs, lErr := longExch.GetClosePnL(pos.Symbol, since)
	if lErr != nil {
		e.log.Warn("reconcile %s [attempt %d]: %s GetClosePnL failed: %v",
			pos.ID, attempt, pos.LongExchange, lErr)
		return false // retry — API may succeed on next attempt
	}

	shortPnLs, sErr := shortExch.GetClosePnL(pos.Symbol, since)
	if sErr != nil {
		e.log.Warn("reconcile %s [attempt %d]: %s GetClosePnL failed: %v",
			pos.ID, attempt, pos.ShortExchange, sErr)
		return false
	}

	// Aggregate all matching records per side (handles partial closes).
	// For Binance (no side info), use aggregated records directly.
	longAgg, longOK := aggregateClosePnLBySide(longPnLs, "long")
	shortAgg, shortOK := aggregateClosePnLBySide(shortPnLs, "short")

	if !longOK || !shortOK {
		e.log.Warn("reconcile %s [attempt %d]: missing close PnL record (long=%v short=%v, longRecords=%d shortRecords=%d)",
			pos.ID, attempt, longOK, shortOK, len(longPnLs), len(shortPnLs))
		return false // retry — exchange may not have finalized the position yet
	}

	// When multiple local positions share the same exchange+symbol+side,
	// the exchange only has one merged position. Split the exchange-reported
	// PnL proportionally by each position's size.
	longAgg = e.splitSharedPnL(longAgg, pos, "long")
	shortAgg = e.splitSharedPnL(shortAgg, pos, "short")

	// Calculate reconciled PnL from exchange-reported figures.
	reconciledPnL := longAgg.NetPnL + shortAgg.NetPnL + pos.RotationPnL
	reconciledFunding := longAgg.Funding + shortAgg.Funding
	totalFees := longAgg.Fees + shortAgg.Fees
	oldPnL := pos.RealizedPnL
	diff := reconciledPnL - oldPnL

	e.log.Info("reconcile %s [attempt %d]: exchange PnL=%.4f (long=%.4f short=%.4f rotation=%.4f fees=%.4f funding=%.4f) local=%.4f diff=%.4f",
		pos.ID, attempt, reconciledPnL, longAgg.NetPnL, shortAgg.NetPnL, pos.RotationPnL, totalFees, reconciledFunding, oldPnL, diff)

	// Sanity check: diff should not exceed position notional.
	longNotional := pos.LongEntry * pos.LongSize
	shortNotional := pos.ShortEntry * pos.ShortSize
	notional := math.Max(longNotional, shortNotional)
	if notional <= 0 {
		// Sizes zeroed (depth-exit path) — estimate from exchange-reported close sizes.
		notional = math.Max(longAgg.CloseSize*longAgg.EntryPrice, shortAgg.CloseSize*shortAgg.EntryPrice)
	}
	if notional > 0 && math.Abs(diff) > notional {
		e.log.Warn("reconcile %s [attempt %d]: diff %.4f exceeds notional %.4f — likely incomplete data, retrying",
			pos.ID, attempt, diff, notional)
		return false
	}

	// Only update if meaningful difference (>$0.01).
	needsPnLUpdate := math.Abs(diff) >= 0.01
	needsFundingUpdate := math.Abs(reconciledFunding-pos.FundingCollected) >= 0.01

	// Overwrite exit prices only if exchange returned a non-zero value.
	reconciledLongExit := longAgg.ExitPrice
	reconciledShortExit := shortAgg.ExitPrice
	needsExitUpdate := (reconciledLongExit > 0 && reconciledLongExit != pos.LongExit) ||
		(reconciledShortExit > 0 && reconciledShortExit != pos.ShortExit)

	// Check if per-leg breakdown fields need update.
	// Per-field comparison: any field differs from its agg counterpart triggers update.
	needsBreakdownUpdate := pos.LongTotalFees != longAgg.Fees ||
		pos.ShortTotalFees != shortAgg.Fees ||
		pos.LongFunding != longAgg.Funding ||
		pos.ShortFunding != shortAgg.Funding ||
		pos.LongClosePnL != longAgg.PricePnL ||
		pos.ShortClosePnL != shortAgg.PricePnL

	if !needsPnLUpdate && !needsFundingUpdate && !needsExitUpdate && !needsBreakdownUpdate {
		// Even when numbers didn't change, mark reconciliation as done so
		// analytics can trust this position's PnL figures.
		if !pos.HasReconciled {
			_ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
				fresh.HasReconciled = true
				fresh.PartialReconcile = false
				return true
			})
			// Update history entry too — use UpdateHistoryEntry (NOT AddToHistory which is LPush append)
			pos.HasReconciled = true
			pos.PartialReconcile = false
			if err := e.db.UpdateHistoryEntry(pos); err != nil {
				e.log.Warn("reconcile %s: failed to update history HasReconciled: %v", pos.ID, err)
			}
		}
		return true
	}

	if err := e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
		if needsPnLUpdate {
			fresh.RealizedPnL = reconciledPnL
		}
		if needsFundingUpdate {
			fresh.FundingCollected = reconciledFunding
		}
		// Per-leg breakdown + decomposition: populate whenever PnL, funding, or breakdown needs update.
		if needsBreakdownUpdate || needsPnLUpdate || needsFundingUpdate {
			fresh.ExitFees = totalFees
			// BasisGainLoss = current-leg price movement only (no fees, no funding).
			// RotationPnL is NOT subtracted: it's NetPnL (includes rotation fees+funding),
			// not PricePnL, and is tracked separately in its own field.
			fresh.BasisGainLoss = longAgg.PricePnL + shortAgg.PricePnL
			fresh.LongTotalFees = longAgg.Fees
			fresh.ShortTotalFees = shortAgg.Fees
			fresh.LongFunding = longAgg.Funding
			fresh.ShortFunding = shortAgg.Funding
			fresh.LongClosePnL = longAgg.PricePnL
			fresh.ShortClosePnL = shortAgg.PricePnL
		}
		fresh.HasReconciled = true
		fresh.PartialReconcile = false
		if reconciledLongExit > 0 {
			fresh.LongExit = reconciledLongExit
		}
		if reconciledShortExit > 0 {
			fresh.ShortExit = reconciledShortExit
		}
		return true
	}); err != nil {
		e.log.Error("reconcile %s: failed to update position: %v", pos.ID, err)
		return true // don't retry DB errors
	}

	if needsPnLUpdate {
		// Adjust total_pnl only — do NOT call UpdateStats which increments trade/win/loss counts.
		statsDiff := reconciledPnL - oldPnL
		if err := e.db.AdjustPnL(statsDiff); err != nil {
			e.log.Error("reconcile %s: failed to adjust stats PnL: %v", pos.ID, err)
		} else {
			e.log.Info("reconcile %s: corrected PnL %.4f → %.4f (stats adjusted by %.4f)",
				pos.ID, oldPnL, reconciledPnL, statsDiff)
		}

		// Correct win/loss counts if partial close PnL sign changed after reconciliation.
		if pos.PartialReconcile {
			oldWon := pos.RealizedPnL > 0
			newWon := reconciledPnL > 0
			if oldWon != newWon {
				if err := e.db.AdjustWinLoss(oldWon, newWon); err != nil {
					e.log.Error("reconcile %s: AdjustWinLoss failed: %v", pos.ID, err)
				}
			}
		}
	}
	if needsFundingUpdate {
		e.log.Info("reconcile %s: corrected FundingCollected %.4f → %.4f", pos.ID, pos.FundingCollected, reconciledFunding)
	}

	// Update the history entry so the dashboard shows corrected PnL / per-leg breakdown.
	if needsPnLUpdate || needsFundingUpdate || needsExitUpdate || needsBreakdownUpdate {
		updated, err := e.db.GetPosition(pos.ID)
		if err == nil && updated != nil {
			if err := e.db.UpdateHistoryEntry(updated); err != nil {
				e.log.Error("reconcile %s: failed to update history: %v", pos.ID, err)
			}
		}
	}

	// Broadcast corrected PnL to frontend.
	if updated, err := e.db.GetPosition(pos.ID); err == nil && updated != nil {
		e.api.BroadcastPositionUpdate(updated)

		// Record analytics snapshot for position close.
		if e.snapshotWriter != nil {
			e.snapshotWriter.RecordPerpClose(updated)
		}
	}

	return true
}

// aggregateClosePnLBySide sums all ClosePnL records matching the given side.
// Handles partial closes that produce multiple records. For Binance (empty side),
// falls back to using all records if there's exactly one aggregated record.
func aggregateClosePnLBySide(records []exchange.ClosePnL, side string) (exchange.ClosePnL, bool) {
	var agg exchange.ClosePnL
	var found bool
	var matchCount int

	for _, r := range records {
		if r.Side == side {
			agg.PricePnL += r.PricePnL
			agg.Fees += r.Fees
			agg.CloseSize += r.CloseSize
			// Take funding and net PnL only from the first match to avoid
			// double-counting when exchanges attach the same symbol-wide
			// funding total to every record (e.g. Bybit).
			if matchCount == 0 {
				agg.Funding = r.Funding
				agg.NetPnL = r.NetPnL
			} else {
				// For subsequent records, add only the non-funding portion.
				agg.NetPnL += r.NetPnL - r.Funding
			}
			// Use the last record's prices and time.
			agg.EntryPrice = r.EntryPrice
			agg.ExitPrice = r.ExitPrice
			agg.CloseTime = r.CloseTime
			agg.Side = r.Side
			found = true
			matchCount++
		}
	}

	// Fallback: Binance returns aggregated record with empty side.
	if !found && len(records) == 1 && records[0].Side == "" {
		return records[0], true
	}

	return agg, found
}

// splitSharedPnL checks if multiple local positions shared the same exchange+symbol
// on the given side (e.g. two positions both long on gateio WAXPUSDT). If so, the
// exchange only had one merged position, and the PnL must be split proportionally.
// Since closed positions have sizes zeroed, we use entry prices as a proxy for
// notional weight. When entry prices are similar, this effectively divides evenly.
func (e *Engine) splitSharedPnL(agg exchange.ClosePnL, pos *models.ArbitragePosition, side string) exchange.ClosePnL {
	exchName := pos.LongExchange
	myEntry := pos.LongEntry
	if side == "short" {
		exchName = pos.ShortExchange
		myEntry = pos.ShortEntry
	}

	// Find siblings: other recently-closed positions with the same symbol+exchange+side.
	history, err := e.db.GetHistory(50)
	if err != nil {
		return agg
	}

	closeWindow := 5 * time.Minute
	siblingCount := 0
	for _, h := range history {
		if h.Symbol != pos.Symbol || h.ID == pos.ID {
			continue
		}
		sibExch := h.LongExchange
		if side == "short" {
			sibExch = h.ShortExchange
		}
		if sibExch != exchName {
			continue
		}
		if math.Abs(h.UpdatedAt.Sub(pos.UpdatedAt).Seconds()) > closeWindow.Seconds() {
			continue
		}
		siblingCount++
	}

	if siblingCount == 0 {
		return agg // no siblings — use full PnL
	}

	// Split evenly among this position + siblings.
	// Entry prices for same symbol are very close, so even split is a good approximation.
	total := siblingCount + 1
	ratio := 1.0 / float64(total)

	e.log.Info("reconcile %s: splitting %s PnL on %s evenly (%d positions, ratio=%.4f, pnl=%.4f→%.4f)",
		pos.ID, side, exchName, total, ratio, agg.NetPnL, agg.NetPnL*ratio)

	_ = myEntry // available for future weighted split if sizes are preserved
	agg.PricePnL *= ratio
	agg.Fees *= ratio
	agg.Funding *= ratio
	agg.NetPnL *= ratio
	agg.CloseSize *= ratio
	return agg
}

// checkSpreadReversal checks if the funding spread has inverted beyond threshold.
// Called for all exit modes as a safety measure.
func (e *Engine) checkSpreadReversal(pos *models.ArbitragePosition) (bool, string) {
	// Skip spread reversal check within 10 min before or after funding settlement.
	// Pre-settlement, rates shift as the period ends; post-settlement, exchange
	// APIs return predicted next-period rates (near zero). Both would falsely
	// trigger reversal vs the entry spread.
	if !pos.NextFunding.IsZero() {
		untilFunding := time.Until(pos.NextFunding)
		sinceFunding := -untilFunding
		if untilFunding > 0 && untilFunding < 10*time.Minute {
			return false, "" // too close before settlement
		}
		if sinceFunding > 0 && sinceFunding < 10*time.Minute {
			return false, "" // too close after settlement
		}
	}

	longExch, ok := e.exchanges[pos.LongExchange]
	if !ok {
		return false, ""
	}
	shortExch, ok := e.exchanges[pos.ShortExchange]
	if !ok {
		return false, ""
	}

	longRate, err := longExch.GetFundingRate(pos.Symbol)
	if err != nil {
		return false, ""
	}
	shortRate, err := shortExch.GetFundingRate(pos.Symbol)
	if err != nil {
		return false, ""
	}

	// Normalize both rates to bps/hour to match EntrySpread units.
	// Exchanges have different funding intervals (e.g. Bybit 1h, Bitget 8h),
	// so raw rates are not directly comparable.
	longIntervalH := longRate.Interval.Hours()
	if longIntervalH <= 0 {
		longIntervalH = 8
	}
	shortIntervalH := shortRate.Interval.Hours()
	if shortIntervalH <= 0 {
		shortIntervalH = 8
	}
	longBpsH := longRate.Rate * 10000 / longIntervalH
	shortBpsH := shortRate.Rate * 10000 / shortIntervalH
	currentSpreadBpsH := shortBpsH - longBpsH

	if pos.EntrySpread > 0 && currentSpreadBpsH < 0 {
		return true, fmt.Sprintf("spread reversal: entry=%.4f bps/h current=%.4f bps/h", pos.EntrySpread, currentSpreadBpsH)
	}

	return false, ""
}

// checkZeroSpread checks if both legs have equal funding rates (spread ≈ 0).
// This detects situations where the arbitrage opportunity has disappeared.
func (e *Engine) checkZeroSpread(pos *models.ArbitragePosition) (bool, string) {
	// Skip within ±10 min of funding settlement (rates unreliable).
	if !pos.NextFunding.IsZero() {
		untilFunding := time.Until(pos.NextFunding)
		sinceFunding := -untilFunding
		if untilFunding > 0 && untilFunding < 10*time.Minute {
			return false, ""
		}
		if sinceFunding > 0 && sinceFunding < 10*time.Minute {
			return false, ""
		}
	}

	longExch, ok := e.exchanges[pos.LongExchange]
	if !ok {
		return false, ""
	}
	shortExch, ok := e.exchanges[pos.ShortExchange]
	if !ok {
		return false, ""
	}

	longRate, err := longExch.GetFundingRate(pos.Symbol)
	if err != nil {
		return false, ""
	}
	shortRate, err := shortExch.GetFundingRate(pos.Symbol)
	if err != nil {
		return false, ""
	}

	longIntervalH := longRate.Interval.Hours()
	if longIntervalH <= 0 {
		longIntervalH = 8
	}
	shortIntervalH := shortRate.Interval.Hours()
	if shortIntervalH <= 0 {
		shortIntervalH = 8
	}
	longBpsH := longRate.Rate * 10000 / longIntervalH
	shortBpsH := shortRate.Rate * 10000 / shortIntervalH
	currentSpreadBpsH := shortBpsH - longBpsH

	// Consider rates "equal" if spread is within ±0.01 bps/h (floating point tolerance)
	const epsilon = 0.01
	if currentSpreadBpsH > -epsilon && currentSpreadBpsH < epsilon {
		return true, fmt.Sprintf("zero spread: long=%.4f bps/h short=%.4f bps/h spread=%.4f bps/h", longBpsH, shortBpsH, currentSpreadBpsH)
	}

	return false, ""
}

// schedulePostSettlementZeroCheck sets a timer to fire 2 minutes after the next
// funding settlement. If the spread is still zero at that point, it triggers exit.
// This avoids closing a position prematurely when rates often diverge after settlement.
func (e *Engine) schedulePostSettlementZeroCheck(pos *models.ArbitragePosition) {
	if pos.NextFunding.IsZero() {
		e.log.Warn("post-settlement zero-check: %s — NextFunding is zero, cannot schedule (will retry next exit cycle)", pos.ID)
		return
	}

	// Dedup: reuse the pre-settle map to prevent multiple timers per position.
	e.preSettleMu.Lock()
	key := "zero:" + pos.ID
	if e.preSettleActive[key] {
		e.preSettleMu.Unlock()
		return
	}
	e.preSettleActive[key] = true
	e.preSettleMu.Unlock()

	// Check 2 minutes after settlement — rates should have updated by then.
	checkTime := pos.NextFunding.Add(2 * time.Minute)
	delay := time.Until(checkTime)
	if delay <= 0 {
		e.preSettleMu.Lock()
		delete(e.preSettleActive, key)
		e.preSettleMu.Unlock()
		return
	}

	e.log.Info("post-settlement zero-check scheduled for %s at %s (in %s)", pos.ID, checkTime.UTC().Format("15:04:05"), delay.Round(time.Second))

	go func() {
		defer func() {
			e.preSettleMu.Lock()
			delete(e.preSettleActive, key)
			e.preSettleMu.Unlock()
		}()

		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			fresh, err := e.db.GetPosition(pos.ID)
			if err != nil || fresh.Status != models.StatusActive {
				return
			}

			e.exitMu.Lock()
			running := e.exitActive[fresh.ID]
			e.exitMu.Unlock()
			if running {
				return
			}

			// Re-check spread after settlement.
			longExch, ok := e.exchanges[fresh.LongExchange]
			if !ok {
				return
			}
			shortExch, ok := e.exchanges[fresh.ShortExchange]
			if !ok {
				return
			}
			longRate, err := longExch.GetFundingRate(fresh.Symbol)
			if err != nil {
				e.log.Warn("post-settlement zero-check %s: failed to get long rate: %v", fresh.ID, err)
				return
			}
			shortRate, err := shortExch.GetFundingRate(fresh.Symbol)
			if err != nil {
				e.log.Warn("post-settlement zero-check %s: failed to get short rate: %v", fresh.ID, err)
				return
			}

			longIntervalH := longRate.Interval.Hours()
			if longIntervalH <= 0 {
				longIntervalH = 8
			}
			shortIntervalH := shortRate.Interval.Hours()
			if shortIntervalH <= 0 {
				shortIntervalH = 8
			}
			longBpsH := longRate.Rate * 10000 / longIntervalH
			shortBpsH := shortRate.Rate * 10000 / shortIntervalH
			currentSpreadBpsH := shortBpsH - longBpsH

			const epsilon = 0.01
			if currentSpreadBpsH > epsilon || currentSpreadBpsH < -epsilon {
				// Spread is no longer zero (diverged or reversed) — reset counter and let it live.
				// Reversed spreads are handled separately by checkSpreadReversal.
				e.log.Info("post-settlement zero-check: %s — spread diverged to %.4f bps/h, resetting zero-spread count", fresh.ID, currentSpreadBpsH)
				_ = e.db.UpdatePositionFields(fresh.ID, func(f *models.ArbitragePosition) bool {
					f.ZeroSpreadCount = 0
					return true
				})
				return
			}

			reason := fmt.Sprintf("zero spread confirmed post-settlement: long=%.4f bps/h short=%.4f bps/h spread=%.4f bps/h", longBpsH, shortBpsH, currentSpreadBpsH)
			e.log.Info("post-settlement zero-check: %s — %s, exiting", fresh.ID, reason)
			e.spawnExitGoroutine(fresh, reason)

		case <-e.stopCh:
			return
		}
	}()
}

// reducePosition partially closes both legs of a position by the given fraction,
// maintaining delta-neutrality. Used by the health monitor for L4 position reduction.
func (e *Engine) reducePosition(pos *models.ArbitragePosition, fraction float64) error {
	longExch, ok := e.exchanges[pos.LongExchange]
	if !ok {
		return fmt.Errorf("long exchange %s not found", pos.LongExchange)
	}
	shortExch, ok := e.exchanges[pos.ShortExchange]
	if !ok {
		return fmt.Errorf("short exchange %s not found", pos.ShortExchange)
	}

	// Use local recorded sizes for THIS position (exchange total may include siblings).
	reduceLong := pos.LongSize * fraction
	reduceShort := pos.ShortSize * fraction

	if reduceLong <= 0 && reduceShort <= 0 {
		return fmt.Errorf("nothing to reduce (long=%.6f short=%.6f fraction=%.2f)", pos.LongSize, pos.ShortSize, fraction)
	}

	deadline := time.Now().Add(60 * time.Second)

	e.log.Info("reducing position %s by %.0f%%: long=%.6f short=%.6f",
		pos.ID, fraction*100, reduceLong, reduceShort)

	// Reduce long leg (sell to close partial).
	var longReducePrice, shortReducePrice float64
	if reduceLong > 0 {
		oid, err := longExch.PlaceOrder(exchange.PlaceOrderParams{
			Symbol:     pos.Symbol,
			Side:       exchange.SideSell,
			OrderType:  "market",
			Size:       utils.FormatSize(reduceLong, 6),
			Force:      "ioc",
			ReduceOnly: true,
		})
		if err != nil {
			e.log.Error("reduce long leg failed: %v", err)
		} else {
			filled, _ := e.waitForFill(longExch, oid, pos.Symbol, deadline)
			if upd, ok := longExch.GetOrderUpdate(oid); ok && upd.AvgPrice > 0 {
				longReducePrice = upd.AvgPrice
			}
			pos.LongSize -= filled
			if pos.LongSize < 0 {
				pos.LongSize = 0
			}
		}
	}

	// Reduce short leg (buy to close partial).
	if reduceShort > 0 {
		oid, err := shortExch.PlaceOrder(exchange.PlaceOrderParams{
			Symbol:     pos.Symbol,
			Side:       exchange.SideBuy,
			OrderType:  "market",
			Size:       utils.FormatSize(reduceShort, 6),
			Force:      "ioc",
			ReduceOnly: true,
		})
		if err != nil {
			e.log.Error("reduce short leg failed: %v", err)
		} else {
			filled, _ := e.waitForFill(shortExch, oid, pos.Symbol, deadline)
			if upd, ok := shortExch.GetOrderUpdate(oid); ok && upd.AvgPrice > 0 {
				shortReducePrice = upd.AvgPrice
			}
			pos.ShortSize -= filled
			if pos.ShortSize < 0 {
				pos.ShortSize = 0
			}
		}
	}

	// Re-read from Redis and apply size changes atomically to avoid
	// overwriting status changes made by concurrent goroutines.
	newLongSize := pos.LongSize
	newShortSize := pos.ShortSize
	if err := e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
		if fresh.Status == models.StatusClosed {
			return false // already closed, skip
		}
		fresh.LongSize = newLongSize
		fresh.ShortSize = newShortSize
		return true
	}); err != nil {
		e.log.Error("failed to save reduced position: %v", err)
	}
	e.api.BroadcastPositionUpdate(pos)

	// If both legs are fully reduced, close the position.
	if pos.LongSize <= 0 && pos.ShortSize <= 0 {
		e.log.Info("position %s fully reduced, closing", pos.ID)
		// Store exit prices before delegating to closePositionEmergency.
		if longReducePrice > 0 {
			pos.LongExit = longReducePrice
		}
		if shortReducePrice > 0 {
			pos.ShortExit = shortReducePrice
		}
		return e.closePositionEmergency(pos)
	}

	e.log.Info("position %s reduced: long=%.6f short=%.6f", pos.ID, pos.LongSize, pos.ShortSize)
	return nil
}

// closePosition uses simultaneous IOC limit orders for non-emergency closes.
func (e *Engine) closePosition(pos *models.ArbitragePosition) error {
	return e.closePositionWithMode(pos, false)
}

// closePositionEmergency uses market orders for L4/L5 emergency closes.
func (e *Engine) closePositionEmergency(pos *models.ArbitragePosition) error {
	return e.closePositionWithMode(pos, true)
}

// closePositionWithMode is the shared close implementation.
// emergency=true uses immediate market orders; emergency=false uses simultaneous
// IOC limit orders with market fallback for any unfilled remainder.
func (e *Engine) closePositionWithMode(pos *models.ArbitragePosition, emergency bool) error {
	pos.Status = models.StatusClosing
	pos.UpdatedAt = time.Now().UTC()
	if err := e.db.SavePosition(pos); err != nil {
		e.log.Error("failed to save closing status for %s: %v", pos.ID, err)
	}
	e.api.BroadcastPositionUpdate(pos)

	longExch, ok := e.exchanges[pos.LongExchange]
	if !ok {
		return fmt.Errorf("long exchange %s not found", pos.LongExchange)
	}
	shortExch, ok := e.exchanges[pos.ShortExchange]
	if !ok {
		return fmt.Errorf("short exchange %s not found", pos.ShortExchange)
	}

	// Cancel stop-loss orders BEFORE closing legs to prevent SL triggers
	// racing with the close orders.
	e.cancelStopLosses(pos)

	// Use local recorded sizes for THIS position (exchange total may include siblings).
	longSize := pos.LongSize
	shortSize := pos.ShortSize

	var longClosePrice, shortClosePrice float64
	if emergency {
		// Emergency: fire both legs concurrently for fastest close.
		e.log.Info("emergency market close for %s (concurrent)", pos.ID)
		var wg sync.WaitGroup
		var muClose sync.Mutex
		if longSize > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				lp, _ := e.executeMarketClose(pos, longExch, shortExch, longSize, 0)
				muClose.Lock()
				longClosePrice = lp
				muClose.Unlock()
			}()
		}
		if shortSize > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, sp := e.executeMarketClose(pos, longExch, shortExch, 0, shortSize)
				muClose.Lock()
				shortClosePrice = sp
				muClose.Unlock()
			}()
		}
		wg.Wait()
	} else {
		// Smart close already fires IOC orders concurrently internally.
		e.log.Info("IOC close for %s", pos.ID)
		longClosePrice, shortClosePrice = e.executeSmartClose(pos, longExch, shortExch, longSize, shortSize)
	}

	// Fall back to BBO midpoint if close price is missing for a leg that was closed.
	// This happens when GetOrderUpdate (WebSocket) doesn't return data in time.
	if longClosePrice <= 0 && longSize > 0 {
		if bbo, ok := longExch.GetBBO(pos.Symbol); ok && bbo.Bid > 0 {
			longClosePrice = (bbo.Bid + bbo.Ask) / 2
			e.log.Warn("close price missing for long leg, using BBO mid %.8f", longClosePrice)
		} else {
			longClosePrice = pos.LongEntry
			e.log.Warn("close price missing for long leg, using entry price %.8f", longClosePrice)
		}
	}
	if shortClosePrice <= 0 && shortSize > 0 {
		if bbo, ok := shortExch.GetBBO(pos.Symbol); ok && bbo.Ask > 0 {
			shortClosePrice = (bbo.Bid + bbo.Ask) / 2
			e.log.Warn("close price missing for short leg, using BBO mid %.8f", shortClosePrice)
		} else {
			shortClosePrice = pos.ShortEntry
			e.log.Warn("close price missing for short leg, using entry price %.8f", shortClosePrice)
		}
	}

	// Verify both legs are actually flat on exchange before marking closed.
	// waitForFill returns on any non-zero fill, which may be partial.
	time.Sleep(500 * time.Millisecond) // brief delay for exchange state to settle
	actualLong, longVerifyErr := getExchangePositionSize(longExch, pos.Symbol, "long")
	actualShort, shortVerifyErr := getExchangePositionSize(shortExch, pos.Symbol, "short")

	// If verification failed (API error), treat as NOT confirmed flat — safer to keep active.
	notFlat := false
	if longVerifyErr != nil || shortVerifyErr != nil {
		e.log.Error("CRITICAL: closePositionWithMode %s — verification failed (longErr=%v shortErr=%v), assuming NOT flat",
			pos.ID, longVerifyErr, shortVerifyErr)
		notFlat = true
		// Use original sizes as conservative estimate.
		actualLong = longSize
		actualShort = shortSize
	} else if actualLong > 0 || actualShort > 0 {
		e.log.Error("CRITICAL: closePositionWithMode %s — legs NOT flat after close (long=%.6f short=%.6f)",
			pos.ID, actualLong, actualShort)
		notFlat = true
	}

	if notFlat {
		if err := e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
			fresh.Status = models.StatusActive
			fresh.LongSize = actualLong
			fresh.ShortSize = actualShort
			fresh.UpdatedAt = time.Now().UTC()
			return true
		}); err != nil {
			e.log.Error("failed to revert position %s to active: %v", pos.ID, err)
		}
		pos.Status = models.StatusActive
		pos.LongSize = actualLong
		pos.ShortSize = actualShort
		// Clear stale SL and TP IDs before reattaching (old SLs/TPs were already cancelled).
		_ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
			fresh.LongSLOrderID = ""
			fresh.ShortSLOrderID = ""
			fresh.LongTPOrderID = ""
			fresh.ShortTPOrderID = ""
			return true
		})
		pos.LongSLOrderID = ""
		pos.ShortSLOrderID = ""
		pos.LongTPOrderID = ""
		pos.ShortTPOrderID = ""
		// Reattach SL protection for the remaining position.
		e.attachStopLosses(pos)
		e.api.BroadcastPositionUpdate(pos)
		return fmt.Errorf("position %s not confirmed flat after close", pos.ID)
	}

	// Calculate realized PnL.
	// Long PnL = (close - entry) * size
	// Short PnL = (entry - close) * size
	longPnL := (longClosePrice - pos.LongEntry) * longSize
	shortPnL := (pos.ShortEntry - shortClosePrice) * shortSize
	realizedPnL := longPnL + shortPnL + pos.FundingCollected - pos.EntryFees

	// Sanity check: PnL should not exceed position notional value.
	notional := math.Max(pos.LongEntry*longSize, pos.ShortEntry*shortSize)
	if notional > 0 && math.Abs(realizedPnL) > notional*2 {
		e.log.Error("closePosition %s: PnL %.4f exceeds 2x notional %.4f — close prices suspect (longClose=%.8f shortClose=%.8f), zeroing PnL",
			pos.ID, realizedPnL, notional, longClosePrice, shortClosePrice)
		realizedPnL = pos.FundingCollected - pos.EntryFees
		longPnL = 0
		shortPnL = 0
	}

	pos.RealizedPnL = realizedPnL
	// Store exit prices — preserve pre-set values (e.g. from reducePosition).
	if longClosePrice > 0 {
		pos.LongExit = longClosePrice
	}
	if shortClosePrice > 0 {
		pos.ShortExit = shortClosePrice
	}
	pos.Status = models.StatusClosed
	pos.UpdatedAt = time.Now().UTC()

	// Cancel orphan TP/SL/algo orders BEFORE SavePosition — prevents race
	// where a new entry re-uses the symbol and the async cancel wipes its orders.
	longExch.CancelAllOrders(pos.Symbol)
	shortExch.CancelAllOrders(pos.Symbol)

	if err := e.db.SavePosition(pos); err != nil {
		e.log.Error("failed to save closed position: %v", err)
	}

	if err := e.db.AddToHistory(pos); err != nil {
		e.log.Error("failed to add position to history: %v", err)
	}

	won := realizedPnL > 0
	if err := e.db.UpdateStats(realizedPnL, won); err != nil {
		e.log.Error("failed to update stats: %v", err)
	}
	if e.lossLimiter != nil {
		e.lossLimiter.RecordClosedPnL(pos.ID, realizedPnL, pos.Symbol, time.Now().UTC())
	}
	e.releasePerpPosition(pos.ID)

	e.api.BroadcastPositionUpdate(pos)

	mode := "smart"
	if emergency {
		mode = "emergency"
	}
	e.log.Info("position %s closed (%s): pnl=%.4f (long=%.4f short=%.4f funding=%.4f entryFees=%.4f)",
		pos.ID, mode, realizedPnL, longPnL, shortPnL, pos.FundingCollected, pos.EntryFees)

	// Set symbol cooldown on loss close.
	if realizedPnL < 0 && e.cfg.LossCooldownHours > 0 {
		cooldown := time.Duration(e.cfg.LossCooldownHours * float64(time.Hour))
		e.discovery.SetSymbolCooldown(pos.Symbol, cooldown)
	}
	// Set re-entry cooldown on any close.
	if e.cfg.ReEnterCooldownHours > 0 {
		cooldown := time.Duration(e.cfg.ReEnterCooldownHours * float64(time.Hour))
		e.discovery.SetReEnterCooldown(pos.Symbol, cooldown)
	}

	// Reconcile PnL from exchange trade history (immediate attempt is synchronous).
	posCopy := *pos
	e.reconcilePnL(&posCopy)

	return nil
}

// executeMarketClose fires IOC market orders to close both legs immediately.
func (e *Engine) executeMarketClose(pos *models.ArbitragePosition, longExch, shortExch exchange.Exchange, longSize, shortSize float64) (longClosePrice, shortClosePrice float64) {
	deadline := time.Now().Add(60 * time.Second)

	// Close long leg (sell to close).
	if longSize > 0 {
		oid, err := longExch.PlaceOrder(exchange.PlaceOrderParams{
			Symbol:     pos.Symbol,
			Side:       exchange.SideSell,
			OrderType:  "market",
			Size:       e.formatSize(pos.LongExchange, pos.Symbol, longSize),
			Force:      "ioc",
			ReduceOnly: true,
		})
		if err != nil {
			e.log.Error("close long leg failed: %v", err)
		} else {
			e.waitForFill(longExch, oid, pos.Symbol, deadline)
			if upd, ok := longExch.GetOrderUpdate(oid); ok && upd.AvgPrice > 0 {
				longClosePrice = upd.AvgPrice
			}
		}
	}

	// Close short leg (buy to close).
	if shortSize > 0 {
		oid, err := shortExch.PlaceOrder(exchange.PlaceOrderParams{
			Symbol:     pos.Symbol,
			Side:       exchange.SideBuy,
			OrderType:  "market",
			Size:       e.formatSize(pos.ShortExchange, pos.Symbol, shortSize),
			Force:      "ioc",
			ReduceOnly: true,
		})
		if err != nil {
			e.log.Error("close short leg failed: %v", err)
		} else {
			e.waitForFill(shortExch, oid, pos.Symbol, deadline)
			if upd, ok := shortExch.GetOrderUpdate(oid); ok && upd.AvgPrice > 0 {
				shortClosePrice = upd.AvgPrice
			}
		}
	}

	return
}

// executeSmartClose fires simultaneous IOC limit orders to close both legs,
// then retries any unfilled remainder with market orders. No abort path —
// positions must close fully.
func (e *Engine) executeSmartClose(pos *models.ArbitragePosition, longExch, shortExch exchange.Exchange, longSize, shortSize float64) (longClosePrice, shortClosePrice float64) {
	// BBO snapshot → compute IOC limit prices with slippage buffer.
	longBBO, lok := longExch.GetBBO(pos.Symbol)
	shortBBO, sok := shortExch.GetBBO(pos.Symbol)
	if !lok || !sok {
		e.log.Warn("IOC close: BBO unavailable (long/%s=%v short/%s=%v), falling back to market",
			pos.LongExchange, lok, pos.ShortExchange, sok)
		return e.executeMarketClose(pos, longExch, shortExch, longSize, shortSize)
	}

	slippage := e.cfg.SlippageBPS / 10000.0
	longFloor := longBBO.Bid * (1 - slippage)     // sell long: floor price
	shortCeiling := shortBBO.Ask * (1 + slippage) // buy short: ceiling price

	e.log.Info("IOC close for %s: long sell floor=%.6f (bid=%.6f) | short buy ceiling=%.6f (ask=%.6f)",
		pos.Symbol, longFloor, longBBO.Bid, shortCeiling, shortBBO.Ask)

	// Fire both IOC limit orders concurrently.
	type orderResult struct {
		orderID string
		err     error
	}
	longCh := make(chan orderResult, 1)
	shortCh := make(chan orderResult, 1)

	if longSize > 0 {
		go func() {
			oid, err := longExch.PlaceOrder(exchange.PlaceOrderParams{
				Symbol:     pos.Symbol,
				Side:       exchange.SideSell,
				OrderType:  "limit",
				Price:      e.formatPrice(pos.LongExchange, pos.Symbol, longFloor),
				Size:       e.formatSize(pos.LongExchange, pos.Symbol, longSize),
				Force:      "ioc",
				ReduceOnly: true,
			})
			longCh <- orderResult{oid, err}
		}()
	} else {
		longCh <- orderResult{}
	}

	if shortSize > 0 {
		go func() {
			oid, err := shortExch.PlaceOrder(exchange.PlaceOrderParams{
				Symbol:     pos.Symbol,
				Side:       exchange.SideBuy,
				OrderType:  "limit",
				Price:      e.formatPrice(pos.ShortExchange, pos.Symbol, shortCeiling),
				Size:       e.formatSize(pos.ShortExchange, pos.Symbol, shortSize),
				Force:      "ioc",
				ReduceOnly: true,
			})
			shortCh <- orderResult{oid, err}
		}()
	} else {
		shortCh <- orderResult{}
	}

	longRes := <-longCh
	shortRes := <-shortCh

	if longRes.err != nil {
		e.log.Error("IOC close: long order failed: %v", longRes.err)
	}
	if shortRes.err != nil {
		e.log.Error("IOC close: short order failed: %v", shortRes.err)
	}

	// Confirm fills (WS + REST, 5s timeout).
	var longFilled, longAvg, shortFilled, shortAvg float64
	if longRes.orderID != "" {
		longFilled, longAvg = e.confirmFill(longExch, longRes.orderID, pos.Symbol)
	}
	if shortRes.orderID != "" {
		shortFilled, shortAvg = e.confirmFill(shortExch, shortRes.orderID, pos.Symbol)
	}

	e.log.Info("IOC close fills for %s: long=%.6f@%.6f short=%.6f@%.6f",
		pos.Symbol, longFilled, longAvg, shortFilled, shortAvg)

	// VWAP accumulators.
	var longFillValue, longFillVol, shortFillValue, shortFillVol float64
	if longFilled > 0 {
		avgP := longAvg
		if avgP <= 0 {
			avgP = longBBO.Bid // fallback
		}
		longFillValue = avgP * longFilled
		longFillVol = longFilled
	}
	if shortFilled > 0 {
		avgP := shortAvg
		if avgP <= 0 {
			avgP = shortBBO.Ask // fallback
		}
		shortFillValue = avgP * shortFilled
		shortFillVol = shortFilled
	}

	// Market retry for any unfilled remainder — positions must close fully.
	longRemaining := longSize - longFillVol
	shortRemaining := shortSize - shortFillVol
	if longRemaining > 0 || shortRemaining > 0 {
		e.log.Info("IOC close: market retry for remainder (long=%.6f short=%.6f)", longRemaining, shortRemaining)
		mktLongPrice, mktShortPrice := e.executeMarketClose(pos, longExch, shortExch, longRemaining, shortRemaining)
		if longRemaining > 0 && mktLongPrice > 0 {
			longFillValue += mktLongPrice * longRemaining
			longFillVol += longRemaining
		}
		if shortRemaining > 0 && mktShortPrice > 0 {
			shortFillValue += mktShortPrice * shortRemaining
			shortFillVol += shortRemaining
		}
	}

	// Compute VWAP close prices.
	if longFillVol > 0 {
		longClosePrice = longFillValue / longFillVol
	}
	if shortFillVol > 0 {
		shortClosePrice = shortFillValue / shortFillVol
	}
	return
}

// ---------------------------------------------------------------------------
// Leg Rotation
// ---------------------------------------------------------------------------

// checkRotations compares active positions against discovery opportunities
// and rotates the inferior leg when a significantly better exchange is found.
func (e *Engine) checkRotations() {
	positions, err := e.db.GetActivePositions()
	if err != nil {
		return
	}
	opps := e.discovery.GetOpportunities()
	if len(opps) == 0 {
		return
	}

	cooldown := time.Duration(e.cfg.RotationCooldownMin) * time.Minute

	for _, pos := range positions {
		if pos.Status != models.StatusActive {
			continue
		}
		// Cooldown check.
		if !pos.LastRotatedAt.IsZero() && time.Since(pos.LastRotatedAt) < cooldown {
			continue
		}

		// Skip rotation near funding settlement — rates are volatile and unreliable
		// within 10 minutes before or after a funding snapshot.
		if !pos.NextFunding.IsZero() {
			untilFunding := time.Until(pos.NextFunding)
			sinceFunding := -untilFunding // positive if past NextFunding
			if untilFunding > 0 && untilFunding < 10*time.Minute {
				continue // too close before settlement
			}
			if sinceFunding > 0 && sinceFunding < 10*time.Minute {
				continue // too close after settlement
			}
		}

		// Scan ALL opportunities for the same symbol, pick the best one.
		currentSpread := e.computeLiveSpread(pos)

		var bestOpp *models.Opportunity
		var bestSharedLeg, bestOldExch, bestNewExch, bestLegSide string
		var bestImprovement float64

		for i := range opps {
			if opps[i].Symbol != pos.Symbol {
				continue
			}

			// Classify: which leg is shared, which should rotate?
			sharedLeg, oldExch, newExch, legSide := classifyRotation(pos, &opps[i])
			if sharedLeg == "" {
				continue // both legs same or both different
			}

			// Penalize moves to higher-frequency settlement exchanges.
			settlementPenalty := e.settlementFrequencyPenalty(pos, opps[i], legSide, oldExch, newExch)
			improvement := opps[i].Spread - currentSpread - settlementPenalty

			// Apply threshold with hysteresis for return-to-previous.
			threshold := e.cfg.RotationThresholdBPS
			if newExch == pos.LastRotatedFrom {
				threshold *= 2
			}
			if improvement < threshold {
				continue
			}

			if improvement > bestImprovement {
				bestImprovement = improvement
				bestOpp = &opps[i]
				bestSharedLeg = sharedLeg
				bestOldExch = oldExch
				bestNewExch = newExch
				bestLegSide = legSide
			}
		}

		if bestOpp == nil {
			continue
		}

		_ = bestSharedLeg // used for classification, logged via legSide

		e.log.Info("rotation triggered for %s: %s leg %s → %s (current=%.1f opp=%.1f improvement=%.1f bps/h)",
			pos.ID, bestLegSide, bestOldExch, bestNewExch, currentSpread, bestOpp.Spread, bestImprovement)

		if e.cfg.DryRun {
			e.log.Info("[DRY RUN] would rotate %s %s leg: %s → %s", pos.ID, bestLegSide, bestOldExch, bestNewExch)
			continue
		}

		e.rotateLeg(pos, *bestOpp, bestLegSide, bestOldExch, bestNewExch)
	}
}

// classifyRotation determines which leg is shared and which should rotate.
// Returns empty strings if both legs are the same or both differ.
func classifyRotation(pos *models.ArbitragePosition, opp *models.Opportunity) (sharedLeg, oldExch, newExch, legSide string) {
	sameLong := pos.LongExchange == opp.LongExchange
	sameShort := pos.ShortExchange == opp.ShortExchange

	if sameLong && !sameShort {
		return "long", pos.ShortExchange, opp.ShortExchange, "short"
	}
	if !sameLong && sameShort {
		return "short", pos.LongExchange, opp.LongExchange, "long"
	}
	return "", "", "", "" // both same or both different
}

// computeLiveSpread fetches current funding rates and returns live spread in bps/h.
func (e *Engine) computeLiveSpread(pos *models.ArbitragePosition) float64 {
	longExch, ok := e.exchanges[pos.LongExchange]
	if !ok {
		return 0
	}
	shortExch, ok := e.exchanges[pos.ShortExchange]
	if !ok {
		return 0
	}

	longRate, err := longExch.GetFundingRate(pos.Symbol)
	if err != nil {
		return 0
	}
	shortRate, err := shortExch.GetFundingRate(pos.Symbol)
	if err != nil {
		return 0
	}

	longIntervalH := longRate.Interval.Hours()
	if longIntervalH <= 0 {
		longIntervalH = 8
	}
	shortIntervalH := shortRate.Interval.Hours()
	if shortIntervalH <= 0 {
		shortIntervalH = 8
	}

	longBpsH := longRate.Rate * 10000 / longIntervalH
	shortBpsH := shortRate.Rate * 10000 / shortIntervalH
	return shortBpsH - longBpsH
}

// settlementFrequencyPenalty computes the bps/h cost of moving a leg to a
// higher-frequency settlement exchange. Returns 0 if new interval >= old.
func (e *Engine) settlementFrequencyPenalty(
	pos *models.ArbitragePosition, opp models.Opportunity,
	legSide, oldExchName, newExchName string,
) float64 {
	oldExch, ok1 := e.exchanges[oldExchName]
	newExch, ok2 := e.exchanges[newExchName]
	if !ok1 || !ok2 {
		return 0
	}

	oldFR, err := oldExch.GetFundingRate(pos.Symbol)
	if err != nil {
		return 0
	}
	newFR, err := newExch.GetFundingRate(pos.Symbol)
	if err != nil {
		return 0
	}

	oldIntervalH := oldFR.Interval.Hours()
	newIntervalH := newFR.Interval.Hours()
	if oldIntervalH <= 0 || newIntervalH <= 0 || newIntervalH >= oldIntervalH {
		return 0 // same or less frequent, no penalty
	}

	// How many hours until old exchange's next settlement?
	hoursUntilOld := time.Until(oldFR.NextFunding).Hours()
	if hoursUntilOld <= 0 {
		hoursUntilOld = oldIntervalH // fallback: assume full interval
	}

	// Extra settlements the new exchange will have before old's next.
	newSettlements := int(hoursUntilOld / newIntervalH)
	oldSettlements := 1 // old would have settled once
	extraSettlements := newSettlements - oldSettlements
	if extraSettlements <= 0 {
		return 0
	}

	// Determine if this leg is paying on the new exchange.
	// Short pays when rate < 0, long pays when rate > 0.
	var payingRate float64
	if legSide == "short" && newFR.Rate < 0 {
		payingRate = -newFR.Rate // short pays abs(rate) when rate is negative
	} else if legSide == "long" && newFR.Rate > 0 {
		payingRate = newFR.Rate // long pays rate when rate is positive
	} else {
		return 0 // collecting on extra settlements — no penalty
	}

	newRateBpsPerSettlement := payingRate * 10000
	totalExtraCostBps := float64(extraSettlements) * newRateBpsPerSettlement

	// Amortize over hours until old's settlement to express as bps/h.
	penalty := totalExtraCostBps / hoursUntilOld

	e.log.Info("rotation penalty %s: %s→%s interval %.0gh→%.0gh, %d extra settlements, penalty=%.1f bps/h",
		pos.Symbol, oldExchName, newExchName, oldIntervalH, newIntervalH, extraSettlements, penalty)

	return penalty
}

// rotateLeg opens a new position on the better exchange first, then closes the
// old leg. Sequential execution (open-first) ensures:
//   - If open fails → abort cleanly, original position untouched
//   - If close fails → retry with market, position is never left without a leg
//
// Sets LastRotatedAt on ALL outcomes (success or failure) to enforce cooldown.
func (e *Engine) rotateLeg(pos *models.ArbitragePosition, opp models.Opportunity, legSide, oldExchName, newExchName string) {
	// Always set cooldown timestamp, even on failure, to prevent rapid retries.
	defer func() {
		_ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
			fresh.LastRotatedAt = time.Now().UTC()
			return true
		})
	}()

	oldExch, ok := e.exchanges[oldExchName]
	if !ok {
		e.log.Error("rotation: old exchange %s not found", oldExchName)
		return
	}
	newExch, ok := e.exchanges[newExchName]
	if !ok {
		e.log.Error("rotation: new exchange %s not found", newExchName)
		return
	}

	// Determine sizes and sides.
	var closeSize float64
	var closeSide, openSide exchange.Side
	if legSide == "short" {
		closeSize = pos.ShortSize
		closeSide = exchange.SideBuy // buy to close short
		openSide = exchange.SideSell // sell to open short
	} else {
		closeSize = pos.LongSize
		closeSide = exchange.SideSell // sell to close long
		openSide = exchange.SideBuy   // buy to open long
	}

	if closeSize <= 0 {
		e.log.Warn("rotation: nothing to rotate (size=0) for %s", pos.ID)
		return
	}

	// Set leverage and margin mode on new exchange.
	leverage := strconv.Itoa(e.cfg.Leverage)
	if err := newExch.SetLeverage(opp.Symbol, leverage, legSide); err != nil {
		if isAlreadySetError(err) {
			e.log.Debug("rotation: set leverage on %s: %v (already set)", newExchName, err)
		} else {
			e.log.Error("rotation: set leverage on %s failed: %v — aborting rotation", newExchName, err)
			return
		}
	}
	if err := newExch.SetMarginMode(opp.Symbol, "cross"); err != nil {
		if isAlreadySetError(err) {
			e.log.Debug("rotation: set margin mode on %s: %v (already set)", newExchName, err)
		} else {
			e.log.Error("rotation: set margin mode on %s failed: %v — aborting rotation", newExchName, err)
			return
		}
	}

	// Ensure both exchanges are subscribed to the symbol's price stream.
	oldExch.SubscribeSymbol(pos.Symbol)
	newExch.SubscribeSymbol(pos.Symbol)

	// BBO snapshots — fall back to orderbook REST if WS BBO not yet available.
	newBBO, nlok := e.getBBOWithFallback(newExch, pos.Symbol)
	if !nlok {
		e.log.Error("rotation: BBO unavailable on new exchange %s", newExchName)
		return
	}

	// Ensure new exchange has enough margin (auto-transfer from spot if needed).
	refPrice := (newBBO.Bid + newBBO.Ask) / 2
	requiredMargin := (closeSize * refPrice) / float64(e.cfg.Leverage)
	newBal, err := newExch.GetFuturesBalance()
	if err != nil {
		e.log.Error("rotation: failed to get balance on %s: %v", newExchName, err)
		return
	}
	if newBal.Available < requiredMargin {
		deficit := requiredMargin - newBal.Available
		spotBal, err := newExch.GetSpotBalance()
		if err == nil && spotBal.Available > 0 {
			transferAmt := deficit
			if transferAmt > spotBal.Available {
				transferAmt = spotBal.Available
			}
			amtStr := fmt.Sprintf("%.4f", transferAmt)
			e.log.Info("rotation: auto-transfer %s USDT spot→futures on %s (futures=%.2f, needed=%.2f)",
				amtStr, newExchName, newBal.Available, requiredMargin)
			if err := newExch.TransferToFutures("USDT", amtStr); err != nil {
				e.log.Error("rotation: auto-transfer on %s failed: %v", newExchName, err)
			} else {
				e.recordTransfer(newExchName+" spot", newExchName, "USDT", "internal", amtStr, "0", "", "completed", "rotation")
			}
			newBal, _ = newExch.GetFuturesBalance()
		}
		if newBal.Available < requiredMargin {
			e.log.Warn("rotation: insufficient margin on %s (have=%.2f need=%.2f), skipping",
				newExchName, newBal.Available, requiredMargin)
			return
		}
	}

	slippage := e.cfg.SlippageBPS / 10000.0
	sizeStr := utils.FormatSize(closeSize, 6)

	// Pre-check: position notional must meet minimum to avoid partial-fill loops.
	const minNotionalUSDT = 10.0
	if closeSize*refPrice < minNotionalUSDT {
		e.log.Warn("rotation: position notional too small (%.2f USDT < %.0f), skipping",
			closeSize*refPrice, minNotionalUSDT)
		return
	}

	// Compute open price on new exchange.
	var openPrice string
	if legSide == "short" {
		openPrice = e.formatPrice(newExchName, pos.Symbol, newBBO.Bid*(1-slippage))
	} else {
		openPrice = e.formatPrice(newExchName, pos.Symbol, newBBO.Ask*(1+slippage))
	}

	// ---------------------------------------------------------------
	// Step 1: OPEN new leg first. If this fails, abort — nothing changed.
	// ---------------------------------------------------------------
	e.log.Info("rotation step 1: open %s on %s @ %s size=%s", openSide, newExchName, openPrice, sizeStr)

	openOID, err := newExch.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:    pos.Symbol,
		Side:      openSide,
		OrderType: "limit",
		Price:     openPrice,
		Size:      sizeStr,
		Force:     "ioc",
	})
	if err != nil {
		e.log.Error("rotation: open on %s failed: %v — aborting, position unchanged", newExchName, err)
		return
	}

	openFilled, openAvg := e.confirmFill(newExch, openOID, pos.Symbol)
	e.log.Info("rotation: open filled %.6f@%.6f on %s", openFilled, openAvg, newExchName)

	if openFilled <= 0 {
		e.log.Warn("rotation: open got zero fill on %s — aborting, position unchanged", newExchName)
		return
	}

	// Check minimum notional on the opened amount.
	if openAvg <= 0 {
		if legSide == "short" {
			openAvg = newBBO.Bid
		} else {
			openAvg = newBBO.Ask
		}
	}
	if openFilled*refPrice < minNotionalUSDT {
		e.log.Warn("rotation: open fill too small (%.2f USDT), closing it back", openFilled*refPrice)
		rem := e.closeFullyWithRetry(newExch, pos.Symbol, closeSide, openFilled)
		if rem > 0 {
			e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", pos.Symbol, closeSide, rem, newExch.Name())
		}
		return
	}

	// Must have filled enough to be worth keeping (at least 50% of target).
	if openFilled < closeSize*0.5 {
		e.log.Warn("rotation: open fill %.6f < 50%% of target %.6f, closing it back", openFilled, closeSize)
		rem := e.closeFullyWithRetry(newExch, pos.Symbol, closeSide, openFilled)
		if rem > 0 {
			e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", pos.Symbol, closeSide, rem, newExch.Name())
		}
		return
	}

	// ---------------------------------------------------------------
	// Step 2: CLOSE old leg — only the amount that was opened.
	// This is the committed point. We have a new leg open, must close old.
	// ---------------------------------------------------------------
	closeQty := openFilled // only close what we successfully opened
	closeSizeStr := utils.FormatSize(closeQty, 6)

	// Close old leg with verified retry (up to 10 attempts).
	// Only proceed with position update if old leg is confirmed closed.
	e.log.Info("rotation step 2: close %s on %s size=%s (verified retry)", closeSide, oldExchName, closeSizeStr)
	rem := e.closeFullyWithRetry(oldExch, pos.Symbol, closeSide, closeQty)
	if rem > 0 {
		e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", pos.Symbol, closeSide, rem, oldExch.Name())
	}

	// Cancel orphan TP/SL/algo orders on old exchange after rotation close
	go oldExch.CancelAllOrders(pos.Symbol)

	// Verify old leg is actually flat before updating position record.
	time.Sleep(500 * time.Millisecond)
	oldSide := "short"
	if legSide == "long" {
		oldSide = "long"
	}
	remainingOnExch, verifyErr := getExchangePositionSize(oldExch, pos.Symbol, oldSide)
	if verifyErr != nil {
		// Verification failed — cannot confirm flat. Abort rotation, close new leg back.
		e.log.Error("CRITICAL: rotation close for %s — verification failed on %s (err=%v), aborting. Closing new leg back.",
			pos.ID, oldExchName, verifyErr)
		rem := e.closeFullyWithRetry(newExch, pos.Symbol, closeSide, openFilled)
		if rem > 0 {
			e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", pos.Symbol, closeSide, rem, newExch.Name())
		}
		return
	}
	if remainingOnExch > 0 {
		e.log.Error("CRITICAL: rotation close for %s — old leg NOT flat on %s (remaining=%.6f), aborting. Closing new leg back.",
			pos.ID, oldExchName, remainingOnExch)
		// Write remaining old-leg size back to position before aborting.
		_ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
			if legSide == "short" {
				fresh.ShortSize = remainingOnExch
			} else {
				fresh.LongSize = remainingOnExch
			}
			return true
		})
		rem := e.closeFullyWithRetry(newExch, pos.Symbol, closeSide, openFilled)
		if rem > 0 {
			e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", pos.Symbol, closeSide, rem, newExch.Name())
		}
		return
	}
	// Update position record atomically.
	if err := e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
		if fresh.Status != models.StatusActive {
			return false
		}
		if legSide == "short" {
			fresh.ShortExchange = newExchName
			fresh.ShortSize = openFilled
			fresh.ShortEntry = openAvg
		} else {
			fresh.LongExchange = newExchName
			fresh.LongSize = openFilled
			fresh.LongEntry = openAvg
		}
		fresh.LastRotatedFrom = oldExchName
		fresh.LastRotatedAt = time.Now().UTC()
		fresh.RotationCount++
		fresh.EntrySpread = opp.Spread
		fresh.RotationHistory = append(fresh.RotationHistory, models.RotationRecord{
			From:      oldExchName,
			To:        newExchName,
			LegSide:   legSide,
			Timestamp: time.Now().UTC(),
		})
		// Update NextFunding from the new exchange so it doesn't stay stale.
		if newFR, frErr := newExch.GetFundingRate(pos.Symbol); frErr == nil && !newFR.NextFunding.IsZero() {
			fresh.NextFunding = newFR.NextFunding
		}
		// Track new exchange for PnL reconciliation.
		found := false
		for _, ex := range fresh.AllExchanges {
			if ex == newExchName {
				found = true
				break
			}
		}
		if !found {
			fresh.AllExchanges = append(fresh.AllExchanges, newExchName)
		}
		return true
	}); err != nil {
		e.log.Error("rotation: failed to update position %s: %v", pos.ID, err)
		return
	}

	// Update stop-loss for the rotated leg: cancel old, place new.
	e.updateRotationStopLoss(pos, legSide, oldExchName, newExchName, openFilled, openAvg)

	e.api.BroadcastPositionUpdate(pos)
	e.log.Info("rotation complete for %s: %s leg %s → %s (size=%.6f entry=%.6f spread=%.1f bps/h)",
		pos.ID, legSide, oldExchName, newExchName, openFilled, openAvg, opp.Spread)

	// Reconcile rotation PnL from exchange data asynchronously.
	rotationTime := time.Now().UTC()
	go e.reconcileRotationPnL(pos.ID, oldExch, oldExchName, pos.Symbol, legSide, rotationTime)
}

// reconcileRotationPnL queries the old exchange's GetClosePnL to get the
// authoritative PnL from the rotated-away leg and stores it in RotationPnL.
// Uses a narrow time window around the rotation to avoid picking up stale records.
// If the position is already closed, recomputes RealizedPnL and updates history.
func (e *Engine) reconcileRotationPnL(posID string, oldExch exchange.Exchange, oldExchName, symbol, legSide string, rotationTime time.Time) {
	// Query window: 2 minutes before rotation to capture the close record.
	since := rotationTime.Add(-2 * time.Minute)

	// Retry up to 3 times with increasing delays — exchange may not finalize immediately.
	delays := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}
	for attempt, delay := range delays {
		time.Sleep(delay)

		e.pnlReconcileMu.Lock()

		records, err := oldExch.GetClosePnL(symbol, since)
		if err != nil {
			e.log.Warn("rotation PnL reconcile %s [attempt %d]: %s GetClosePnL failed: %v",
				posID, attempt+1, oldExchName, err)
			e.pnlReconcileMu.Unlock()
			continue
		}

		// Filter records to only those closed near the rotation time (±5 min).
		// Binance returns Side="" (income API has no side info) — accept if only 1 record.
		var filtered []exchange.ClosePnL
		for _, r := range records {
			sideMatch := r.Side == legSide || r.Side == ""
			timeMatch := abs(r.CloseTime.Sub(rotationTime).Seconds()) < 300
			if sideMatch && timeMatch {
				filtered = append(filtered, r)
			}
		}
		if len(filtered) == 0 {
			for i, r := range records {
				e.log.Warn("rotation PnL reconcile %s [attempt %d]: rejected record %d: side=%q closeTime=%s netPnL=%.4f",
					posID, attempt+1, i, r.Side, r.CloseTime.Format("15:04:05"), r.NetPnL)
			}
			e.log.Warn("rotation PnL reconcile %s [attempt %d]: no %s close record near rotation time on %s (total records=%d)",
				posID, attempt+1, legSide, oldExchName, len(records))
			e.pnlReconcileMu.Unlock()
			continue
		}

		// Sum filtered records (handles partial fills that produce multiple records).
		var rotPnL float64
		for _, r := range filtered {
			rotPnL += r.NetPnL
		}

		e.log.Info("rotation PnL reconcile %s [attempt %d]: %s on %s netPnL=%.4f (records=%d)",
			posID, attempt+1, legSide, oldExchName, rotPnL, len(filtered))

		if err := e.db.UpdatePositionFields(posID, func(fresh *models.ArbitragePosition) bool {
			fresh.RotationPnL += rotPnL // accumulate across multiple rotations
			fresh.UpdatedAt = time.Now().UTC()
			// Backfill PnL on the matching rotation history record (by From exchange + LegSide).
			for i := len(fresh.RotationHistory) - 1; i >= 0; i-- {
				r := &fresh.RotationHistory[i]
				if r.From == oldExchName && r.LegSide == legSide && r.PnL == nil {
					r.PnL = &rotPnL
					break
				}
			}
			return true
		}); err != nil {
			e.log.Error("rotation PnL reconcile %s: failed to update: %v", posID, err)
			e.pnlReconcileMu.Unlock()
			return
		}
		e.log.Info("rotation PnL reconcile %s: RotationPnL accumulated by %.4f", posID, rotPnL)

		// If position is already closed, re-run full reconciliation instead of
		// manually adjusting RealizedPnL. This ensures RotationPnL (just updated
		// above) is included correctly via the formula:
		// reconciledPnL = longAgg.NetPnL + shortAgg.NetPnL + pos.RotationPnL
		pos, err := e.db.GetPosition(posID)
		if err != nil {
			e.pnlReconcileMu.Unlock()
			return
		}
		if pos.Status == models.StatusClosed {
			fresh, err := e.db.GetPosition(posID)
			if err == nil && fresh != nil {
				e.tryReconcilePnL(fresh, 1)
			}
		}
		e.pnlReconcileMu.Unlock()
		return
	}
	e.log.Error("rotation PnL reconcile %s: all attempts failed for %s on %s", posID, legSide, oldExchName)
}

// updateRotationStopLoss cancels the old SL on the rotated-away exchange and
// places a new SL on the new exchange at the new entry price.
func (e *Engine) updateRotationStopLoss(pos *models.ArbitragePosition, legSide, oldExchName, newExchName string, newSize, newEntry float64) {
	leverage := float64(e.cfg.Leverage)
	if leverage <= 0 {
		leverage = 3
	}
	distance := 0.9 / leverage

	// Cancel old SL and unregister from slIndex to prevent stale triggers.
	if legSide == "short" && pos.ShortSLOrderID != "" {
		// Unregister old SL from slIndex.
		e.slIndexMu.Lock()
		delete(e.slIndex, oldExchName+":"+pos.ShortSLOrderID)
		e.slIndexMu.Unlock()
		if exch, ok := e.exchanges[oldExchName]; ok {
			if err := exch.CancelStopLoss(pos.Symbol, pos.ShortSLOrderID); err != nil {
				e.log.Warn("rotation: cancel old short SL %s on %s: %v", pos.ShortSLOrderID, oldExchName, err)
			}
		}
	} else if legSide == "long" && pos.LongSLOrderID != "" {
		// Unregister old SL from slIndex.
		e.slIndexMu.Lock()
		delete(e.slIndex, oldExchName+":"+pos.LongSLOrderID)
		e.slIndexMu.Unlock()
		if exch, ok := e.exchanges[oldExchName]; ok {
			if err := exch.CancelStopLoss(pos.Symbol, pos.LongSLOrderID); err != nil {
				e.log.Warn("rotation: cancel old long SL %s on %s: %v", pos.LongSLOrderID, oldExchName, err)
			}
		}
	}

	// Cancel old TP on the rotated leg's old exchange.
	if legSide == "short" && pos.ShortTPOrderID != "" {
		if exch, ok := e.exchanges[oldExchName]; ok {
			if err := exch.CancelTakeProfit(pos.Symbol, pos.ShortTPOrderID); err != nil {
				e.log.Warn("rotation: cancel old short TP %s on %s: %v", pos.ShortTPOrderID, oldExchName, err)
			}
		}
	} else if legSide == "long" && pos.LongTPOrderID != "" {
		if exch, ok := e.exchanges[oldExchName]; ok {
			if err := exch.CancelTakeProfit(pos.Symbol, pos.LongTPOrderID); err != nil {
				e.log.Warn("rotation: cancel old long TP %s on %s: %v", pos.LongTPOrderID, oldExchName, err)
			}
		}
	}

	// Place new SL on the new exchange.
	newExch, ok := e.exchanges[newExchName]
	if !ok || newEntry <= 0 {
		return
	}

	var side exchange.Side
	var triggerPrice float64
	if legSide == "short" {
		side = exchange.SideBuy
		triggerPrice = newEntry * (1 + distance)
	} else {
		side = exchange.SideSell
		triggerPrice = newEntry * (1 - distance)
	}

	tp := e.formatPrice(newExchName, pos.Symbol, triggerPrice)
	oid, err := newExch.PlaceStopLoss(exchange.StopLossParams{
		Symbol:       pos.Symbol,
		Side:         side,
		Size:         e.formatSize(newExchName, pos.Symbol, newSize),
		TriggerPrice: tp,
	})
	if err != nil {
		e.log.Error("rotation: SL placement failed on %s %s (%s): %v", newExchName, pos.Symbol, legSide, err)
		return
	}

	e.log.Info("rotation: SL placed on %s %s: %s trigger=%s (entry=%.4f, %.1f%% distance)",
		newExchName, pos.Symbol, side, tp, newEntry, distance*100)

	// Place new TP on the rotated leg.
	// TP on each leg triggers at the opposite leg's SL level.
	var tpOID string
	var tpTriggerPrice float64
	if legSide == "short" && pos.LongEntry > 0 {
		// Short TP triggers when price drops to long's SL level → buy to close.
		tpTriggerPrice = pos.LongEntry * (1 - distance)
	} else if legSide == "long" && pos.ShortEntry > 0 {
		// Long TP triggers when price rises to short's SL level → sell to close.
		tpTriggerPrice = pos.ShortEntry * (1 + distance)
	}
	if tpTriggerPrice > 0 {
		tpTP := e.formatPrice(newExchName, pos.Symbol, tpTriggerPrice)
		var tpErr error
		tpOID, tpErr = newExch.PlaceTakeProfit(exchange.TakeProfitParams{
			Symbol:       pos.Symbol,
			Side:         side,
			Size:         e.formatSize(newExchName, pos.Symbol, newSize),
			TriggerPrice: tpTP,
		})
		if tpErr != nil {
			e.log.Error("rotation: TP placement failed on %s %s (%s): %v", newExchName, pos.Symbol, legSide, tpErr)
		} else {
			e.log.Info("rotation: TP placed on %s %s: %s trigger=%s (opposite leg SL level)",
				newExchName, pos.Symbol, side, tpTP)
		}
	}

	// Persist new SL and TP order IDs.
	_ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
		if legSide == "short" {
			fresh.ShortSLOrderID = oid
			fresh.ShortTPOrderID = tpOID
		} else {
			fresh.LongSLOrderID = oid
			fresh.LongTPOrderID = tpOID
		}
		return true
	})
	if legSide == "short" {
		pos.ShortSLOrderID = oid
		pos.ShortTPOrderID = tpOID
	} else {
		pos.LongSLOrderID = oid
		pos.LongTPOrderID = tpOID
	}

	// Register new SL in slIndex for instant fill detection (mirrors attachStopLosses).
	e.slIndexMu.Lock()
	key := newExchName + ":" + oid
	e.slIndex[key] = slEntry{PosID: pos.ID, Leg: legSide}
	e.slIndexMu.Unlock()
}

// getBBOWithFallback tries WS BBO then falls back to orderbook REST.
func (e *Engine) getBBOWithFallback(exch exchange.Exchange, symbol string) (exchange.BBO, bool) {
	if bbo, ok := exch.GetBBO(symbol); ok {
		return bbo, true
	}
	if ob, err := exch.GetOrderbook(symbol, 5); err == nil && len(ob.Bids) > 0 && len(ob.Asks) > 0 {
		return exchange.BBO{Bid: ob.Bids[0].Price, Ask: ob.Asks[0].Price}, true
	}
	return exchange.BBO{}, false
}

// closeOldLegMarket places a reduce-only market IOC to close the old leg.
// Used as fallback when limit IOC doesn't fully fill during rotation.
func (e *Engine) closeOldLegMarket(exch exchange.Exchange, symbol string, side exchange.Side, size string) {
	oid, err := exch.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:     symbol,
		Side:       side,
		OrderType:  "market",
		Size:       size,
		Force:      "ioc",
		ReduceOnly: true,
	})
	if err != nil {
		e.log.Error("rotation: market close failed on %s %s: %v", exch.Name(), symbol, err)
		return
	}
	e.log.Info("rotation: market close on %s %s order=%s", exch.Name(), symbol, oid)
}

// closeFullyWithRetry places repeated reduce-only market IOC orders until
// the full quantity is closed or max retries exhausted. Single closeLeg calls
// can partially fill on exchanges with low liquidity, leaving orphan positions.
func (e *Engine) closeFullyWithRetry(exch exchange.Exchange, symbol string, side exchange.Side, totalQty float64) float64 {
	filled, _ := e.closeFullyWithRetryPriced(context.Background(), exch, symbol, side, totalQty)
	rem := totalQty - filled
	if rem < 0 {
		rem = 0
	}
	// Treat sub-minSize remainder as dust (same logic as inner function)
	if rem > 0 {
		var minSize float64
		if e.contracts != nil {
			if exContracts, ok := e.contracts[exch.Name()]; ok {
				if ci, ok := exContracts[symbol]; ok {
					minSize = ci.MinSize
				}
			}
		}
		if minSize > 0 && rem < minSize {
			rem = 0
		}
		// Also check if formatSize would round to zero
		if rem > 0 {
			if sizeF, _ := strconv.ParseFloat(e.formatSize(exch.Name(), symbol, rem), 64); sizeF <= 0 {
				rem = 0
			}
		}
	}
	return rem
}

// closeFullyWithRetryPriced retries market IOC up to 10 times. Returns total filled and VWAP avg price.
// Respects ctx for L4/L5 preemption.
func (e *Engine) closeFullyWithRetryPriced(ctx context.Context, exch exchange.Exchange, symbol string, side exchange.Side, totalQty float64) (totalFilled float64, avgPrice float64) {
	remaining := totalQty
	deadline := time.Now().Add(30 * time.Second)
	var vwapSum float64

	// Look up min order size to detect dust remainders that can't be closed.
	var minSize float64
	if e.contracts != nil {
		if exContracts, ok := e.contracts[exch.Name()]; ok {
			if ci, ok := exContracts[symbol]; ok {
				minSize = ci.MinSize
			}
		}
	}

	for attempt := 0; attempt < 10 && remaining > 0; attempt++ {
		if ctx.Err() != nil {
			e.log.Info("closeFullyWithRetry %s %s: cancelled by context", exch.Name(), symbol)
			break
		}
		// Skip if remaining is dust below exchange minimum order size.
		if minSize > 0 && remaining < minSize {
			e.log.Info("closeFullyWithRetry %s %s: remaining %.6f below minSize %.6f — treating as dust", exch.Name(), symbol, remaining, minSize)
			remaining = 0
			break
		}
		sizeStr := e.formatSize(exch.Name(), symbol, remaining)
		// Guard against floating point: RoundToStep can floor to 0 when
		// remaining is barely at a step boundary (e.g., 0.000999 → 0.000).
		if sizeF, _ := strconv.ParseFloat(sizeStr, 64); sizeF <= 0 {
			e.log.Info("closeFullyWithRetry %s %s: formatted size %q rounds to zero (remaining=%.8f) — treating as dust", exch.Name(), symbol, sizeStr, remaining)
			remaining = 0
			break
		}
		oid, err := exch.PlaceOrder(exchange.PlaceOrderParams{
			Symbol:     symbol,
			Side:       side,
			OrderType:  "market",
			Size:       sizeStr,
			Force:      "ioc",
			ReduceOnly: true,
		})
		if err != nil {
			e.log.Error("closeFullyWithRetry %s %s attempt %d: %v", exch.Name(), symbol, attempt+1, err)
			break
		}
		e.ownOrders.Store(exch.Name()+":"+oid, struct{}{})

		filled, avg := e.confirmFill(exch, oid, symbol)
		remaining -= filled
		totalFilled += filled
		if avg > 0 {
			vwapSum += avg * filled
		}
		e.log.Info("closeFullyWithRetry %s %s: filled=%.6f remaining=%.6f", exch.Name(), symbol, filled, remaining)

		if remaining <= 0 {
			break
		}
		if time.Now().After(deadline) {
			e.log.Error("closeFullyWithRetry %s %s: deadline exceeded, remaining=%.6f", exch.Name(), symbol, remaining)
			break
		}
		time.Sleep(1 * time.Second)
	}

	if remaining > 0 {
		e.log.Error("CRITICAL: closeFullyWithRetry %s %s failed to close %.6f of %.6f — manual intervention needed",
			exch.Name(), symbol, remaining, totalQty)
	}

	if totalFilled > 0 {
		avgPrice = vwapSum / totalFilled
	}
	return
}
