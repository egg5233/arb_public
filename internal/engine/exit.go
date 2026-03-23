package engine

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"arb/pkg/exchange"
	"arb/internal/models"
	"arb/pkg/utils"
)

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
	e.exitMu.Unlock()

	// Set status to "exiting".
	_ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
		if fresh.Status != models.StatusActive {
			return false
		}
		fresh.Status = models.StatusExiting
		return true
	})
	e.api.BroadcastPositionUpdate(pos)

	go func() {
		defer func() {
			e.exitMu.Lock()
			delete(e.exitCancels, pos.ID)
			delete(e.exitActive, pos.ID)
			e.exitMu.Unlock()
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

	// Wait up to 2s for depth data.
	for i := 0; i < 20; i++ {
		_, lok := longExch.GetDepth(pos.Symbol)
		_, sok := shortExch.GetDepth(pos.Symbol)
		if lok && sok {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	var closedLong, closedShort float64
	var longVWAPSum, shortVWAPSum float64
	var longConsecFails, shortConsecFails int
	const maxConsecFails = 5

	timeout := time.Duration(e.cfg.ExitDepthTimeoutSec) * time.Second
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	e.log.Info("depth exit loop for %s: longSize=%.6f shortSize=%.6f timeout=%v",
		pos.ID, totalLong, totalShort, timeout)

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

		// Determine chunk size from available depth.
		remaining := math.Min(longRemaining, shortRemaining)
		bidQty := 0.0
		for _, lvl := range longDepth.Bids {
			bidQty += lvl.Quantity
		}
		askQty := 0.0
		for _, lvl := range shortDepth.Asks {
			askQty += lvl.Quantity
		}

		size := math.Min(remaining, math.Min(bidQty, askQty))

		// Round to step size.
		if e.contracts != nil {
			if exContracts, ok := e.contracts[pos.LongExchange]; ok {
				if ci, ok := exContracts[pos.Symbol]; ok && ci.StepSize > 0 {
					size = utils.RoundToStep(size, ci.StepSize)
				}
			}
		}

		bestBid := longDepth.Bids[0].Price
		if size*bestBid < e.cfg.MinChunkUSDT {
			continue
		}

		// Sell long leg first (reduce-only).
		slippage := e.cfg.SlippageBPS / 10000.0
		sellPrice := e.formatPrice(pos.LongExchange, pos.Symbol, bestBid*(1-slippage))
		sizeStr := utils.FormatSize(size, 6)

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
		bestAsk := shortDepth.Asks[0].Price
		buyPrice := e.formatPrice(pos.ShortExchange, pos.Symbol, bestAsk*(1+slippage))
		buySize := utils.FormatSize(filled, 6)

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
		if avg2 > 0 {
			shortVWAPSum += avg2 * filled2
		}

		// If short didn't fully fill, adjust long accumulator.
		// The long IOC already executed — small delta imbalance is acceptable;
		// consolidator will reconcile.
		if filled2 < filled {
			excess := filled - filled2
			closedLong -= excess
			if avg > 0 {
				longVWAPSum -= avg * excess
			}
		}

		e.log.Info("depth exit %s: tick closedLong=%.6f closedShort=%.6f",
			pos.ID, closedLong, closedShort)
	}

	// Market fallback for remainder.
	longRemaining := totalLong - closedLong
	shortRemaining := totalShort - closedShort

	if longRemaining > 0 || shortRemaining > 0 {
		e.log.Info("depth exit: market fallback for %s (longRem=%.6f shortRem=%.6f)",
			pos.ID, longRemaining, shortRemaining)
		mktLongPrice, mktShortPrice := e.executeMarketClose(pos, longExch, shortExch, longRemaining, shortRemaining)
		if longRemaining > 0 && mktLongPrice > 0 {
			longVWAPSum += mktLongPrice * longRemaining
			closedLong += longRemaining
		}
		if shortRemaining > 0 && mktShortPrice > 0 {
			shortVWAPSum += mktShortPrice * shortRemaining
			closedShort += shortRemaining
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

	// Cancel stop-loss orders before closing.
	e.cancelStopLosses(pos)

	// Calculate PnL and finalize.
	longPnL := (longClosePrice - pos.LongEntry) * totalLong
	shortPnL := (pos.ShortEntry - shortClosePrice) * totalShort
	realizedPnL := longPnL + shortPnL + pos.FundingCollected

	// Sanity check: PnL should not exceed position notional value.
	// If it does, the close price is likely wrong — fall back to 0 PnL.
	notional := math.Max(pos.LongEntry*totalLong, pos.ShortEntry*totalShort)
	if notional > 0 && math.Abs(realizedPnL) > notional*2 {
		e.log.Error("depth-exit %s: PnL %.4f exceeds 2x notional %.4f — close prices suspect (longClose=%.8f shortClose=%.8f), zeroing PnL",
			pos.ID, realizedPnL, notional, longClosePrice, shortClosePrice)
		realizedPnL = pos.FundingCollected // keep only funding as PnL
		longPnL = 0
		shortPnL = 0
	}

	if err := e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
		fresh.RealizedPnL = realizedPnL
		fresh.Status = models.StatusClosed
		fresh.UpdatedAt = time.Now().UTC()
		fresh.LongSize = 0
		fresh.ShortSize = 0
		return true
	}); err != nil {
		e.log.Error("failed to save closed position %s: %v", pos.ID, err)
	}

	// Re-read for broadcast.
	pos.RealizedPnL = realizedPnL
	pos.Status = models.StatusClosed
	pos.LongSize = 0
	pos.ShortSize = 0

	if err := e.db.AddToHistory(pos); err != nil {
		e.log.Error("failed to add to history: %v", err)
	}
	won := realizedPnL > 0
	if err := e.db.UpdateStats(realizedPnL, won); err != nil {
		e.log.Error("failed to update stats: %v", err)
	}
	e.api.BroadcastPositionUpdate(pos)

	e.log.Info("position %s depth-exit closed: pnl=%.4f (long=%.4f short=%.4f funding=%.4f)",
		pos.ID, realizedPnL, longPnL, shortPnL, pos.FundingCollected)

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

	// Reconcile PnL from exchange trade history asynchronously.
	posCopy := *pos
	go e.reconcilePnL(&posCopy)

	return nil
}

// reconcilePnL queries actual trade history from all exchanges involved in
// this position (including rotated-away exchanges) and recomputes realized PnL
// from real fill data. Runs async after position close. Updates the position
// record and stats if different. Retries up to 3 times on failure.
func (e *Engine) reconcilePnL(pos *models.ArbitragePosition) {
	const maxRetries = 3
	delays := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}

	for attempt := 0; attempt < maxRetries; attempt++ {
		time.Sleep(delays[attempt])

		ok := e.tryReconcilePnL(pos, attempt+1)
		if ok {
			return
		}
	}
	e.log.Error("reconcile %s: all %d attempts failed, keeping local PnL=%.4f",
		pos.ID, maxRetries, pos.RealizedPnL)
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

	if !needsPnLUpdate && !needsFundingUpdate {
		return true
	}

	if err := e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
		if needsPnLUpdate {
			fresh.RealizedPnL = reconciledPnL
		}
		if needsFundingUpdate {
			fresh.FundingCollected = reconciledFunding
		}
		return true
	}); err != nil {
		e.log.Error("reconcile %s: failed to update position: %v", pos.ID, err)
		return true // don't retry DB errors
	}

	if needsPnLUpdate {
		statsDiff := reconciledPnL - oldPnL
		wasWon := oldPnL > 0
		nowWon := reconciledPnL > 0
		if wasWon != nowWon {
			e.log.Warn("reconcile %s: win/loss status changed (old=%.4f new=%.4f)", pos.ID, oldPnL, reconciledPnL)
		}
		if err := e.db.UpdateStats(statsDiff, false); err != nil {
			e.log.Error("reconcile %s: failed to update stats: %v", pos.ID, err)
		}
		e.log.Info("reconcile %s: corrected PnL %.4f → %.4f (diff=%.4f)", pos.ID, oldPnL, reconciledPnL, diff)
	}
	if needsFundingUpdate {
		e.log.Info("reconcile %s: corrected FundingCollected %.4f → %.4f", pos.ID, pos.FundingCollected, reconciledFunding)
	}

	// Update the history entry so the dashboard shows corrected PnL.
	if needsPnLUpdate || needsFundingUpdate {
		updated, err := e.db.GetPosition(pos.ID)
		if err == nil && updated != nil {
			if err := e.db.UpdateHistoryEntry(updated); err != nil {
				e.log.Error("reconcile %s: failed to update history: %v", pos.ID, err)
			}
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

	// Use local recorded sizes for THIS position (exchange total may include siblings).
	longSize := pos.LongSize
	shortSize := pos.ShortSize

	var longClosePrice, shortClosePrice float64
	if emergency {
		e.log.Info("emergency market close for %s", pos.ID)
		longClosePrice, shortClosePrice = e.executeMarketClose(pos, longExch, shortExch, longSize, shortSize)
	} else {
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

	// Cancel stop-loss orders before closing.
	e.cancelStopLosses(pos)

	// Calculate realized PnL.
	// Long PnL = (close - entry) * size
	// Short PnL = (entry - close) * size
	longPnL := (longClosePrice - pos.LongEntry) * longSize
	shortPnL := (pos.ShortEntry - shortClosePrice) * shortSize
	realizedPnL := longPnL + shortPnL + pos.FundingCollected

	// Sanity check: PnL should not exceed position notional value.
	notional := math.Max(pos.LongEntry*longSize, pos.ShortEntry*shortSize)
	if notional > 0 && math.Abs(realizedPnL) > notional*2 {
		e.log.Error("closePosition %s: PnL %.4f exceeds 2x notional %.4f — close prices suspect (longClose=%.8f shortClose=%.8f), zeroing PnL",
			pos.ID, realizedPnL, notional, longClosePrice, shortClosePrice)
		realizedPnL = pos.FundingCollected
		longPnL = 0
		shortPnL = 0
	}

	pos.RealizedPnL = realizedPnL
	pos.Status = models.StatusClosed
	pos.UpdatedAt = time.Now().UTC()

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

	e.api.BroadcastPositionUpdate(pos)

	mode := "smart"
	if emergency {
		mode = "emergency"
	}
	e.log.Info("position %s closed (%s): pnl=%.4f (long=%.4f short=%.4f funding=%.4f)",
		pos.ID, mode, realizedPnL, longPnL, shortPnL, pos.FundingCollected)

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

	// Reconcile PnL from exchange trade history asynchronously.
	posCopy := *pos
	go e.reconcilePnL(&posCopy)

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
			Size:       utils.FormatSize(longSize, 6),
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
			Size:       utils.FormatSize(shortSize, 6),
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
				Size:       utils.FormatSize(longSize, 6),
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
				Size:       utils.FormatSize(shortSize, 6),
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

		// Find opportunity for same symbol.
		var opp *models.Opportunity
		for i := range opps {
			if opps[i].Symbol == pos.Symbol {
				opp = &opps[i]
				break
			}
		}
		if opp == nil {
			continue
		}

		// Classify: which leg is shared, which should rotate?
		sharedLeg, oldExch, newExch, legSide := classifyRotation(pos, opp)
		if sharedLeg == "" {
			continue // both legs same or both different
		}

		// Compute current live spread for position.
		currentSpread := e.computeLiveSpread(pos)

		// Penalize moves to higher-frequency settlement exchanges.
		settlementPenalty := e.settlementFrequencyPenalty(pos, *opp, legSide, oldExch, newExch)
		improvement := opp.Spread - currentSpread - settlementPenalty

		// Apply threshold with hysteresis for return-to-previous.
		threshold := e.cfg.RotationThresholdBPS
		if newExch == pos.LastRotatedFrom {
			threshold *= 2
		}
		if improvement < threshold {
			continue
		}

		e.log.Info("rotation triggered for %s: %s leg %s → %s (current=%.1f opp=%.1f penalty=%.1f improvement=%.1f threshold=%.1f bps/h)",
			pos.ID, legSide, oldExch, newExch, currentSpread, opp.Spread, settlementPenalty, improvement, threshold)

		if e.cfg.DryRun {
			e.log.Info("[DRY RUN] would rotate %s %s leg: %s → %s", pos.ID, legSide, oldExch, newExch)
			continue
		}

		e.rotateLeg(pos, *opp, legSide, oldExch, newExch)
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
		e.log.Warn("rotation: set leverage on %s: %v", newExchName, err)
	}
	if err := newExch.SetMarginMode(opp.Symbol, "cross"); err != nil {
		e.log.Warn("rotation: set margin mode on %s: %v", newExchName, err)
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
		e.closeFullyWithRetry(newExch, pos.Symbol, closeSide, openFilled)
		return
	}

	// Must have filled enough to be worth keeping (at least 50% of target).
	if openFilled < closeSize*0.5 {
		e.log.Warn("rotation: open fill %.6f < 50%% of target %.6f, closing it back", openFilled, closeSize)
		e.closeFullyWithRetry(newExch, pos.Symbol, closeSide, openFilled)
		return
	}

	// ---------------------------------------------------------------
	// Step 2: CLOSE old leg — only the amount that was opened.
	// This is the committed point. We have a new leg open, must close old.
	// ---------------------------------------------------------------
	closeQty := openFilled // only close what we successfully opened
	closeSizeStr := utils.FormatSize(closeQty, 6)

	oldBBO, olok := e.getBBOWithFallback(oldExch, pos.Symbol)
	if !olok {
		// Can't get price — use a very aggressive price to ensure fill.
		e.log.Warn("rotation: BBO unavailable on old exchange %s, using market close", oldExchName)
		e.closeOldLegMarket(oldExch, pos.Symbol, closeSide, closeSizeStr)
		goto updatePosition
	}

	{
		var closePrice string
		if legSide == "short" {
			closePrice = e.formatPrice(oldExchName, pos.Symbol, oldBBO.Ask*(1+slippage))
		} else {
			closePrice = e.formatPrice(oldExchName, pos.Symbol, oldBBO.Bid*(1-slippage))
		}

		e.log.Info("rotation step 2: close %s on %s @ %s size=%s", closeSide, oldExchName, closePrice, closeSizeStr)

		closeOID, err := oldExch.PlaceOrder(exchange.PlaceOrderParams{
			Symbol:     pos.Symbol,
			Side:       closeSide,
			OrderType:  "limit",
			Price:      closePrice,
			Size:       closeSizeStr,
			Force:      "ioc",
			ReduceOnly: true,
		})
		if err != nil {
			e.log.Error("rotation: close IOC failed on %s: %v — retrying with market", oldExchName, err)
			e.closeOldLegMarket(oldExch, pos.Symbol, closeSide, closeSizeStr)
			goto updatePosition
		}

		closeFilled, _ := e.confirmFill(oldExch, closeOID, pos.Symbol)
		e.log.Info("rotation: close filled %.6f/%.6f on %s", closeFilled, closeQty, oldExchName)

		// Market retry for any unfilled remainder.
		remainder := closeQty - closeFilled
		if remainder > 0.0001 {
			e.log.Info("rotation: market close remainder %.6f on %s", remainder, oldExchName)
			e.closeOldLegMarket(oldExch, pos.Symbol, closeSide, utils.FormatSize(remainder, 6))
		}
	}

updatePosition:
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
}

// updateRotationStopLoss cancels the old SL on the rotated-away exchange and
// places a new SL on the new exchange at the new entry price.
func (e *Engine) updateRotationStopLoss(pos *models.ArbitragePosition, legSide, oldExchName, newExchName string, newSize, newEntry float64) {
	leverage := float64(e.cfg.Leverage)
	if leverage <= 0 {
		leverage = 3
	}
	distance := 0.9 / leverage

	// Cancel old SL.
	if legSide == "short" && pos.ShortSLOrderID != "" {
		if exch, ok := e.exchanges[oldExchName]; ok {
			if err := exch.CancelStopLoss(pos.Symbol, pos.ShortSLOrderID); err != nil {
				e.log.Warn("rotation: cancel old short SL %s on %s: %v", pos.ShortSLOrderID, oldExchName, err)
			}
		}
	} else if legSide == "long" && pos.LongSLOrderID != "" {
		if exch, ok := e.exchanges[oldExchName]; ok {
			if err := exch.CancelStopLoss(pos.Symbol, pos.LongSLOrderID); err != nil {
				e.log.Warn("rotation: cancel old long SL %s on %s: %v", pos.LongSLOrderID, oldExchName, err)
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

	// Persist new SL order ID.
	_ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
		if legSide == "short" {
			fresh.ShortSLOrderID = oid
		} else {
			fresh.LongSLOrderID = oid
		}
		return true
	})
	if legSide == "short" {
		pos.ShortSLOrderID = oid
	} else {
		pos.LongSLOrderID = oid
	}
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
func (e *Engine) closeFullyWithRetry(exch exchange.Exchange, symbol string, side exchange.Side, totalQty float64) {
	remaining := totalQty
	deadline := time.Now().Add(30 * time.Second)

	for attempt := 0; attempt < 10 && remaining > 0; attempt++ {
		sizeStr := utils.FormatSize(remaining, 6)
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

		filled, _ := e.confirmFill(exch, oid, symbol)
		remaining -= filled
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
}
