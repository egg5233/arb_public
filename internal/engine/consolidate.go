package engine

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// StartConsolidator launches a background goroutine that periodically
// reconciles local position records with actual exchange positions.
// Fixes: orphan exchange positions, stale local records, size mismatches.
func (e *Engine) StartConsolidator() {
	go e.runConsolidator()
}

func (e *Engine) runConsolidator() {
	// Run once at startup after a short delay for WS connections to establish.
	time.Sleep(10 * time.Second)

	// missCount tracks consecutive "leg missing" detections per position+side.
	// key: "posID:side" (e.g. "katusdt-123:short"). Only used for BingX legs
	// to guard against transient API glitches returning empty positions.
	missCount := map[string]int{}

	// dustIgnore tracks orphan positions that are below exchange minSize and
	// cannot be closed via API. Prevents infinite consolidation loops.
	// key: "exchange:symbol:side"
	dustIgnore := map[string]bool{}

	e.consolidatePositions(missCount, dustIgnore)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.consolidatePositions(missCount, dustIgnore)
		case <-e.stopCh:
			e.log.Info("consolidator stopped")
			return
		}
	}
}

// consolidatePositions compares local position records with exchange state
// and fixes mismatches.
func (e *Engine) consolidatePositions(missCount map[string]int, dustIgnore map[string]bool) {
	positions, err := e.db.GetActivePositions()
	if err != nil {
		e.log.Error("consolidate: failed to get positions: %v", err)
		return
	}

	// Build a map of what positions we expect on each exchange.
	// key: "exchange:symbol:side"
	expectedPositions := map[string]struct{}{}
	for _, pos := range positions {
		if pos.Status != models.StatusActive && pos.Status != models.StatusPartial {
			continue
		}
		if pos.LongSize > 0 {
			key := fmt.Sprintf("%s:%s:long", pos.LongExchange, pos.Symbol)
			expectedPositions[key] = struct{}{}
		}
		if pos.ShortSize > 0 {
			key := fmt.Sprintf("%s:%s:short", pos.ShortExchange, pos.Symbol)
			expectedPositions[key] = struct{}{}
		}
	}

	// Check each active position: verify exchange sizes match local record.
	// Build sibling size map so duplicate-symbol positions are handled correctly.
	// key: "exchange:symbol:side" → total size claimed by all local positions.
	siblingTotals := map[string]float64{}
	for _, pos := range positions {
		if pos.Status != models.StatusActive && pos.Status != models.StatusPartial {
			continue
		}
		if pos.LongSize > 0 {
			key := fmt.Sprintf("%s:%s:long", pos.LongExchange, pos.Symbol)
			siblingTotals[key] += pos.LongSize
		}
		if pos.ShortSize > 0 {
			key := fmt.Sprintf("%s:%s:short", pos.ShortExchange, pos.Symbol)
			siblingTotals[key] += pos.ShortSize
		}
	}

	// Build a set of symbols currently being entered or exited, so the
	// consolidator doesn't race with depth fill / exit goroutines.
	busySymbols := map[string]bool{} // "exchange:symbol" → true

	e.exitMu.Lock()
	for posID, active := range e.exitActive {
		if !active {
			continue
		}
		for _, pos := range positions {
			if pos.ID == posID {
				busySymbols[pos.LongExchange+":"+pos.Symbol] = true
				busySymbols[pos.ShortExchange+":"+pos.Symbol] = true
				break
			}
		}
	}
	e.exitMu.Unlock()

	e.entryMu.Lock()
	for key := range e.entryActive {
		busySymbols[key] = true
	}
	e.entryMu.Unlock()

	for _, pos := range positions {
		if pos.Status != models.StatusActive {
			continue
		}
		// Skip positions with an active exit goroutine.
		e.exitMu.Lock()
		exiting := e.exitActive[pos.ID]
		e.exitMu.Unlock()
		if exiting {
			continue
		}
		// Skip positions whose symbol is mid-entry or mid-exit on either exchange.
		if busySymbols[pos.LongExchange+":"+pos.Symbol] || busySymbols[pos.ShortExchange+":"+pos.Symbol] {
			continue
		}
		e.consolidatePosition(pos, siblingTotals, missCount)
	}

	// Reconcile stranded partial positions (Fix B)
	for _, pos := range positions {
		if pos.Status != models.StatusPartial {
			continue
		}
		if busySymbols[pos.LongExchange+":"+pos.Symbol] || busySymbols[pos.ShortExchange+":"+pos.Symbol] {
			continue
		}
		e.reconcilePartialPosition(pos)
	}

	// Build exclusion set from active spot-futures positions so the perp-perp
	// consolidator doesn't flag their futures legs as orphans.
	spotFuturesKeys := make(map[string]bool)
	if spotPositions, sfErr := e.db.GetActiveSpotPositions(); sfErr == nil {
		for _, sp := range spotPositions {
			// Dir A = futures long, Dir B = futures short
			side := "long"
			if sp.Direction == "buy_spot_short" {
				side = "short"
			}
			key := fmt.Sprintf("%s:%s:%s", sp.Exchange, sp.Symbol, side)
			spotFuturesKeys[key] = true
		}
	}

	// Check for orphan exchange positions not tracked by any local record.
	for name, exch := range e.exchanges {
		exchPositions, err := exch.GetAllPositions()
		if err != nil {
			continue
		}
		for _, ep := range exchPositions {
			size, _ := utils.ParseFloat(ep.Total)
			if size <= 0 {
				continue
			}
			key := fmt.Sprintf("%s:%s:%s", name, ep.Symbol, ep.HoldSide)
			if _, expected := expectedPositions[key]; !expected {
				// Skip if this exchange:symbol is mid-entry or mid-exit.
				if busySymbols[name+":"+ep.Symbol] {
					continue
				}
				// Skip if this is a spot-futures position's futures leg.
				if spotFuturesKeys[key] {
					continue
				}

				// Skip orphan positions already closed as dust.
				dustKey := fmt.Sprintf("%s:%s:%s", name, ep.Symbol, ep.HoldSide)
				if dustIgnore[dustKey] {
					e.log.Debug("consolidate: skipping dust-ignored orphan %s %s on %s", ep.Symbol, ep.HoldSide, name)
					continue
				}

				markPrice, _ := utils.ParseFloat(ep.MarkPrice)
				notional := size * markPrice
				e.log.Warn("consolidate: orphan position on %s: %s %s size=%.6f notional=%.2f USDT",
					name, ep.Symbol, ep.HoldSide, size, notional)

				if e.cfg.DryRun {
					e.log.Info("[DRY RUN] would close orphan %s %s on %s", ep.Symbol, ep.HoldSide, name)
					continue
				}
				var closeSide exchange.Side
				if ep.HoldSide == "long" {
					closeSide = exchange.SideSell
				} else {
					closeSide = exchange.SideBuy
				}

				// Check if dust (below minSize). Dust can't go through
				// closeFullyWithRetry because the minSize guard blocks it.
				// Place a direct market ReduceOnly order instead.
				var minSize float64
				if e.contracts != nil {
					if exContracts, ok := e.contracts[name]; ok {
						if ci, ok := exContracts[ep.Symbol]; ok {
							minSize = ci.MinSize
						}
					}
				}
				if minSize > 0 && size < minSize {
					// Bypass formatSize (step-rounding may floor to 0). Send raw
					// size so the exchange's ReduceOnly logic caps to actual position.
					sizeStr := e.formatSize(name, ep.Symbol, size)
					if sizeF, _ := strconv.ParseFloat(sizeStr, 64); sizeF <= 0 {
						sizeStr = strconv.FormatFloat(size, 'f', -1, 64)
					}
					e.log.Info("consolidate: dust close %s %s on %s size=%s (below minSize %.6f)",
						ep.Symbol, ep.HoldSide, name, sizeStr, minSize)
					oid, err := exch.PlaceOrder(exchange.PlaceOrderParams{
						Symbol:     ep.Symbol,
						Side:       closeSide,
						OrderType:  "market",
						Size:       sizeStr,
						Force:      "ioc",
						ReduceOnly: true,
					})
					if err != nil {
						e.log.Error("consolidate: dust close failed %s %s on %s: %v — will retry next cycle",
							ep.Symbol, ep.HoldSide, name, err)
					} else {
						e.log.Info("consolidate: dust close order placed %s %s on %s oid=%s",
							ep.Symbol, ep.HoldSide, name, oid)
						e.ownOrders.Store(name+":"+oid, struct{}{})
						// Only suppress future retries on successful order placement.
						// Next cycle will re-check; if position is gone, no action needed.
						dustIgnore[dustKey] = true
					}
					continue
				}

				e.log.Info("consolidate: closing orphan %s %s on %s size=%.6f (verified retry)",
					ep.Symbol, ep.HoldSide, name, size)
				rem := e.closeFullyWithRetry(exch, ep.Symbol, closeSide, size)
				if rem > 0 {
					e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", ep.Symbol, closeSide, rem, exch.Name())
				}
				exch.CancelAllOrders(ep.Symbol)
			}
		}
	}
}

// consolidatePosition checks a single position's exchange legs and fixes
// the local record if they don't match. siblingTotals maps
// "exchange:symbol:side" → total size claimed by ALL local positions,
// so duplicate-symbol positions sharing the same exchange leg are handled.
// bingxMissThreshold is the number of consecutive consolidation cycles where
// a BingX leg must report size=0 before we declare it missing. Guards against
// transient BingX API responses returning empty position arrays.
const bingxMissThreshold = 3

func (e *Engine) consolidatePosition(pos *models.ArbitragePosition, siblingTotals map[string]float64, missCount map[string]int) {
	longExch, lok := e.exchanges[pos.LongExchange]
	shortExch, sok := e.exchanges[pos.ShortExchange]
	if !lok || !sok {
		return
	}

	// Get actual exchange positions for this symbol.
	longSize, longErr := getExchangePositionSize(longExch, pos.Symbol, "long")
	shortSize, shortErr := getExchangePositionSize(shortExch, pos.Symbol, "short")

	if longErr != nil || shortErr != nil {
		return // can't verify, skip
	}

	// When multiple local positions share the same (exchange, symbol, side),
	// the exchange reports the combined total. Compare against the aggregate
	// of all local records, not this single position.
	longKey := fmt.Sprintf("%s:%s:long", pos.LongExchange, pos.Symbol)
	shortKey := fmt.Sprintf("%s:%s:short", pos.ShortExchange, pos.Symbol)
	localLongTotal := siblingTotals[longKey]
	localShortTotal := siblingTotals[shortKey]

	// If there are sibling positions, compare at aggregate level and skip
	// per-position sync (we can't apportion the exchange total to individuals).
	if localLongTotal != pos.LongSize || localShortTotal != pos.ShortSize {
		// Multiple positions share this leg. Validate at aggregate level only.
		longAggDiff := abs(longSize - localLongTotal)
		shortAggDiff := abs(shortSize - localShortTotal)
		longAggPct := longAggDiff / max(localLongTotal, 1)
		shortAggPct := shortAggDiff / max(localShortTotal, 1)
		if longAggPct > 0.01 {
			e.log.Warn("consolidate: %s aggregate long size mismatch on %s: local_total=%.6f exchange=%.6f (%.1f%%)",
				pos.Symbol, pos.LongExchange, localLongTotal, longSize, longAggPct*100)
		}
		if shortAggPct > 0.01 {
			e.log.Warn("consolidate: %s aggregate short size mismatch on %s: local_total=%.6f exchange=%.6f (%.1f%%)",
				pos.Symbol, pos.ShortExchange, localShortTotal, shortSize, shortAggPct*100)
		}
		return // skip per-position sync for shared legs
	}

	// Detect exchange-flat positions: both exchange sizes are 0 but status Active.
	// This catches positions where exit reduced both legs but consolidator
	// didn't mark them closed before sizes were zeroed.
	if longSize == 0 && shortSize == 0 && pos.Status == models.StatusActive {
		// Guard: skip if position was updated very recently (within 60s).
		// Protects against recent consolidator size syncs (L372 sets UpdatedAt)
		// and recent entry/exit activity. Note: reducePosition does NOT set
		// UpdatedAt, so this guard does not cover the L4 reduce window directly.
		// That window is milliseconds in practice and not a practical concern.
		if time.Since(pos.UpdatedAt) < 60*time.Second {
			e.log.Debug("consolidate: %s exchange-flat but recently updated (%.0fs ago), skipping",
				pos.ID, time.Since(pos.UpdatedAt).Seconds())
			return
		}

		e.log.Warn("consolidate: %s exchange-flat (both sizes=0) but Active for %.0fs, attempting close",
			pos.ID, time.Since(pos.UpdatedAt).Seconds())
		// Set local sizes to 0 so markPositionClosed skips close orders (legs already flat).
		pos.LongSize = 0
		pos.ShortSize = 0
		e.markPositionClosed(pos, "exchange-flat detected by consolidator")
		return
	}

	// Tolerance: 1% or $1 notional, whichever is larger.
	longDiff := abs(longSize - pos.LongSize)
	shortDiff := abs(shortSize - pos.ShortSize)
	longPct := longDiff / max(pos.LongSize, 1)
	shortPct := shortDiff / max(pos.ShortSize, 1)

	needsUpdate := false

	// Check for missing legs (exchange position is 0 but local says it exists).
	// For BingX legs, require multiple consecutive misses before acting —
	// BingX API can transiently return empty position arrays.
	if pos.LongSize > 0 && longSize == 0 {
		missKey := pos.ID + ":long"
		if pos.LongExchange == "bingx" {
			missCount[missKey]++
			if missCount[missKey] < bingxMissThreshold {
				e.log.Warn("consolidate: %s long leg missing on bingx (local=%.6f exchange=0), miss %d/%d — waiting for confirmation",
					pos.ID, pos.LongSize, missCount[missKey], bingxMissThreshold)
				return
			}
		}
		e.log.Warn("consolidate: %s long leg missing on %s (local=%.6f exchange=0), closing position",
			pos.ID, pos.LongExchange, pos.LongSize)
		delete(missCount, missKey)
		e.markPositionClosed(pos, "long leg missing on exchange")
		return
	}
	// Long leg is present — reset miss counter.
	delete(missCount, pos.ID+":long")

	if pos.ShortSize > 0 && shortSize == 0 {
		missKey := pos.ID + ":short"
		if pos.ShortExchange == "bingx" {
			missCount[missKey]++
			if missCount[missKey] < bingxMissThreshold {
				e.log.Warn("consolidate: %s short leg missing on bingx (local=%.6f exchange=0), miss %d/%d — waiting for confirmation",
					pos.ID, pos.ShortSize, missCount[missKey], bingxMissThreshold)
				return
			}
		}
		e.log.Warn("consolidate: %s short leg missing on %s (local=%.6f exchange=0), closing position",
			pos.ID, pos.ShortExchange, pos.ShortSize)
		delete(missCount, missKey)
		e.markPositionClosed(pos, "short leg missing on exchange")
		return
	}
	// Short leg is present — reset miss counter.
	delete(missCount, pos.ID+":short")

	// Check for size mismatches > 1%.
	if longPct > 0.01 {
		e.log.Warn("consolidate: %s long size mismatch: local=%.6f exchange=%.6f (%.1f%%)",
			pos.ID, pos.LongSize, longSize, longPct*100)
		needsUpdate = true
	}
	if shortPct > 0.01 {
		e.log.Warn("consolidate: %s short size mismatch: local=%.6f exchange=%.6f (%.1f%%)",
			pos.ID, pos.ShortSize, shortSize, shortPct*100)
		needsUpdate = true
	}

	if needsUpdate {
		newLong := longSize
		newShort := shortSize
		if err := e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
			if fresh.Status != models.StatusActive {
				return false
			}
			fresh.LongSize = newLong
			fresh.ShortSize = newShort
			fresh.UpdatedAt = time.Now().UTC()
			return true
		}); err != nil {
			e.log.Error("consolidate: failed to update %s: %v", pos.ID, err)
		} else {
			e.log.Info("consolidate: synced %s sizes: long=%.6f short=%.6f", pos.ID, newLong, newShort)
			e.api.BroadcastPositionUpdate(pos)
		}

		// Balance enforcer: if long and short are imbalanced after sync,
		// trim the excess side to restore delta neutrality.
		if !e.cfg.DryRun {
			e.enforceBalance(pos, newLong, newShort)
		}
	}
}

// markPositionClosed closes a position that has a missing leg on the exchange.
// If the remaining leg still exists, it closes it with verified retry.
// Only marks closed if confirmed flat.
func (e *Engine) markPositionClosed(pos *models.ArbitragePosition, reason string) {
	// Try to close BOTH legs — including the one reported as "missing".
	// The "missing" leg may still exist on the exchange (transient API glitch),
	// so we re-query and attempt to close it too. Uses verified retry (up to 10 attempts).
	if pos.LongSize > 0 {
		if longExch, ok := e.exchanges[pos.LongExchange]; ok {
			actualSize, err := getExchangePositionSize(longExch, pos.Symbol, "long")
			if err == nil && actualSize > 0 {
				e.log.Info("consolidate: closing long leg on %s: %.6f (verified retry)", pos.LongExchange, actualSize)
				rem := e.closeFullyWithRetry(longExch, pos.Symbol, exchange.SideSell, actualSize)
				if rem > 0 {
					e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", pos.Symbol, exchange.SideSell, rem, longExch.Name())
				}
			} else {
				// Leg not found on re-query — try closing with local size as fallback.
				e.log.Info("consolidate: long leg not found on %s re-query, attempting close with local size %.6f",
					pos.LongExchange, pos.LongSize)
				rem := e.closeFullyWithRetry(longExch, pos.Symbol, exchange.SideSell, pos.LongSize)
				if rem > 0 {
					e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", pos.Symbol, exchange.SideSell, rem, longExch.Name())
				}
			}
		}
	}
	if pos.ShortSize > 0 {
		if shortExch, ok := e.exchanges[pos.ShortExchange]; ok {
			actualSize, err := getExchangePositionSize(shortExch, pos.Symbol, "short")
			if err == nil && actualSize > 0 {
				e.log.Info("consolidate: closing short leg on %s: %.6f (verified retry)", pos.ShortExchange, actualSize)
				rem := e.closeFullyWithRetry(shortExch, pos.Symbol, exchange.SideBuy, actualSize)
				if rem > 0 {
					e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", pos.Symbol, exchange.SideBuy, rem, shortExch.Name())
				}
			} else {
				// Leg not found on re-query — try closing with local size as fallback.
				e.log.Info("consolidate: short leg not found on %s re-query, attempting close with local size %.6f",
					pos.ShortExchange, pos.ShortSize)
				rem := e.closeFullyWithRetry(shortExch, pos.Symbol, exchange.SideBuy, pos.ShortSize)
				if rem > 0 {
					e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", pos.Symbol, exchange.SideBuy, rem, shortExch.Name())
				}
			}
		}
	}

	// Verify both legs are flat before marking closed.
	// If verification fails (API error), treat as NOT confirmed flat.
	var longRemaining, shortRemaining float64
	verifyOK := true
	if longExch, ok := e.exchanges[pos.LongExchange]; ok {
		rem, err := getExchangePositionSize(longExch, pos.Symbol, "long")
		if err != nil {
			e.log.Error("consolidate: cannot verify long leg for %s: %v, keeping active", pos.ID, err)
			verifyOK = false
		}
		longRemaining = rem
	}
	if shortExch, ok := e.exchanges[pos.ShortExchange]; ok {
		rem, err := getExchangePositionSize(shortExch, pos.Symbol, "short")
		if err != nil {
			e.log.Error("consolidate: cannot verify short leg for %s: %v, keeping active", pos.ID, err)
			verifyOK = false
		}
		shortRemaining = rem
	}
	if !verifyOK || longRemaining > 0 || shortRemaining > 0 {
		e.log.Error("consolidate: CRITICAL — %s not confirmed flat (long=%.6f short=%.6f verified=%v), keeping active",
			pos.ID, longRemaining, shortRemaining, verifyOK)
		return
	}

	// Query close PnL from both exchanges and aggregate per-leg breakdown.
	var longPnLs, shortPnLs []exchange.ClosePnL
	if longExch, ok := e.exchanges[pos.LongExchange]; ok {
		pnls, err := longExch.GetClosePnL(pos.Symbol, pos.CreatedAt)
		if err == nil {
			longPnLs = pnls
		}
	}
	if shortExch, ok := e.exchanges[pos.ShortExchange]; ok {
		pnls, err := shortExch.GetClosePnL(pos.Symbol, pos.CreatedAt)
		if err == nil {
			shortPnLs = pnls
		}
	}

	const maxConsolidateRetries = 6 // 6 × 5 min = 30 min

	longAgg, longOK := aggregateClosePnLBySide(longPnLs, "long")
	shortAgg, shortOK := aggregateClosePnLBySide(shortPnLs, "short")

	partialClose := false
	if !longOK || !shortOK {
		e.consolidateRetries[pos.ID]++
		retries := e.consolidateRetries[pos.ID]

		if retries <= maxConsolidateRetries {
			e.log.Warn("consolidate: %s missing PnL (long=%v short=%v) attempt %d/%d, keeping existing PnL=%.4f funding=%.4f",
				pos.ID, longOK, shortOK, retries, maxConsolidateRetries, pos.RealizedPnL, pos.FundingCollected)
			return
		}

		// Max retries exceeded — use partial data, fall through to full close path.
		e.log.Warn("consolidate: %s max retries (%d) exceeded, force-closing with partial PnL",
			pos.ID, maxConsolidateRetries)
		delete(e.consolidateRetries, pos.ID)

		if !longOK {
			longAgg = exchange.ClosePnL{}
			partialClose = true
		}
		if !shortOK {
			shortAgg = exchange.ClosePnL{}
			partialClose = true
		}
		// Fall through to splitSharedPnL → reconciledPnL → full close bookkeeping
	} else {
		// Both sides OK — clear any retry counter.
		delete(e.consolidateRetries, pos.ID)
	}

	longAgg = e.splitSharedPnL(longAgg, pos, "long")
	shortAgg = e.splitSharedPnL(shortAgg, pos, "short")

	reconciledPnL := longAgg.NetPnL + shortAgg.NetPnL + pos.RotationPnL
	reconciledFunding := longAgg.Funding + shortAgg.Funding
	totalFees := longAgg.Fees + shortAgg.Fees

	e.log.Info("consolidate: %s reconciled: long=%.4f short=%.4f rotation=%.4f total=%.4f funding=%.4f fees=%.4f",
		pos.ID, longAgg.NetPnL, shortAgg.NetPnL, pos.RotationPnL, reconciledPnL, reconciledFunding, totalFees)

	pos.RealizedPnL = reconciledPnL
	pos.FundingCollected = reconciledFunding
	pos.ExitFees = totalFees
	// BasisGainLoss = current-leg price movement only (no fees, no funding).
	// RotationPnL is NOT subtracted: it's NetPnL (includes rotation fees+funding),
	// not PricePnL, and is tracked separately in its own field.
	pos.BasisGainLoss = longAgg.PricePnL + shortAgg.PricePnL
	pos.LongTotalFees = longAgg.Fees
	pos.ShortTotalFees = shortAgg.Fees
	pos.LongFunding = longAgg.Funding
	pos.ShortFunding = shortAgg.Funding
	pos.LongClosePnL = longAgg.PricePnL
	pos.ShortClosePnL = shortAgg.PricePnL
	pos.LongExit = longAgg.ExitPrice
	pos.ShortExit = shortAgg.ExitPrice
	pos.LongSize = 0
	pos.ShortSize = 0
	pos.Status = models.StatusClosed
	if partialClose {
		pos.HasReconciled = false
		pos.PartialReconcile = true
		e.log.Warn("consolidate: %s PARTIAL close (long=%v short=%v), queuing async reconcile", pos.ID, longOK, shortOK)
	} else {
		pos.HasReconciled = true
	}
	pos.UpdatedAt = time.Now().UTC()

	// Cancel orphan TP/SL/algo orders BEFORE SavePosition — prevents race
	// where a new entry re-uses the symbol and the async cancel wipes its orders.
	if le, ok := e.exchanges[pos.LongExchange]; ok {
		le.CancelAllOrders(pos.Symbol)
	}
	if se, ok := e.exchanges[pos.ShortExchange]; ok {
		se.CancelAllOrders(pos.Symbol)
	}

	pos.ExitReason = reason
	if err := e.db.SavePosition(pos); err != nil {
		e.log.Error("consolidate: failed to close %s: %v", pos.ID, err)
	}
	e.log.Info("[reconcile-debug] AddToHistory %s: LongTotalFees=%.6f ShortTotalFees=%.6f LongFunding=%.6f ShortFunding=%.6f LongClosePnL=%.6f ShortClosePnL=%.6f HasReconciled=%v",
		pos.ID, pos.LongTotalFees, pos.ShortTotalFees, pos.LongFunding, pos.ShortFunding, pos.LongClosePnL, pos.ShortClosePnL, pos.HasReconciled)
	if err := e.db.AddToHistory(pos); err != nil {
		e.log.Error("consolidate: failed to add %s to history: %v", pos.ID, err)
	}
	won := reconciledPnL > 0
	if err := e.db.UpdateStats(reconciledPnL, won); err != nil {
		e.log.Error("consolidate: failed to update stats for %s: %v", pos.ID, err)
	}
	e.releasePerpPosition(pos.ID)
	e.api.BroadcastPositionUpdate(pos)
	e.log.Info("consolidate: closed %s (%s) pnl=%.4f (long=%.4f short=%.4f funding=%.4f rotation=%.4f)",
		pos.ID, reason, reconciledPnL, longAgg.NetPnL, shortAgg.NetPnL, reconciledFunding, pos.RotationPnL)

	if partialClose {
		posCopy := *pos
		go e.reconcilePnL(&posCopy)
	}
}

// enforceBalance trims the excess side when long and short sizes are
// imbalanced beyond 5%, restoring delta neutrality.
// After trimming: confirms fill, updates local record, re-places stop losses.
func (e *Engine) enforceBalance(pos *models.ArbitragePosition, longSize, shortSize float64) {
	if longSize <= 0 || shortSize <= 0 {
		return
	}

	diff := abs(longSize - shortSize)
	minSide := min(longSize, shortSize)
	if diff/minSide < 0.05 {
		return // within 5% tolerance
	}

	var trimExch exchange.Exchange
	var trimSide exchange.Side
	var trimExchName string
	var excess float64

	if longSize > shortSize {
		excess = longSize - shortSize
		trimExchName = pos.LongExchange
		trimSide = exchange.SideSell
		exch, ok := e.exchanges[trimExchName]
		if !ok {
			return
		}
		trimExch = exch
	} else {
		excess = shortSize - longSize
		trimExchName = pos.ShortExchange
		trimSide = exchange.SideBuy
		exch, ok := e.exchanges[trimExchName]
		if !ok {
			return
		}
		trimExch = exch
	}

	sizeStr := e.formatSize(trimExchName, pos.Symbol, excess)
	e.log.Warn("consolidate: %s imbalanced long=%.6f short=%.6f, trimming %.6f on %s (%s)",
		pos.ID, longSize, shortSize, excess, trimExchName, trimSide)

	// Place reduce-only market order and confirm fill.
	orderID, err := trimExch.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:     pos.Symbol,
		Side:       trimSide,
		OrderType:  "market",
		Size:       sizeStr,
		Force:      "ioc",
		ReduceOnly: true,
	})
	if err != nil {
		e.log.Error("consolidate: trim order failed for %s on %s: %v", pos.ID, trimExchName, err)
		return
	}
	e.log.Info("consolidate: trim order %s placed on %s for %s", orderID, trimExchName, pos.ID)

	// Confirm fill (reuse engine's confirmFill with timeout).
	filled, _ := e.confirmFill(trimExch, orderID, pos.Symbol)
	if filled <= 0 {
		e.log.Error("consolidate: trim order %s on %s did not fill, skipping record update", orderID, trimExchName)
		return
	}
	e.log.Info("consolidate: trim filled %.6f/%.6f on %s for %s", filled, excess, trimExchName, pos.ID)

	// Cancel old stop losses before updating sizes.
	e.cancelStopLosses(pos)

	// Use actual filled amount to compute new balanced size.
	// If partial fill, remaining imbalance = excess - filled.
	actualNewLong := longSize
	actualNewShort := shortSize
	if longSize > shortSize {
		actualNewLong = longSize - filled // trimmed the long side
	} else {
		actualNewShort = shortSize - filled // trimmed the short side
	}

	if err := e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
		if fresh.Status != models.StatusActive {
			return false
		}
		fresh.LongSize = actualNewLong
		fresh.ShortSize = actualNewShort
		fresh.LongSLOrderID = ""
		fresh.ShortSLOrderID = ""
		fresh.LongTPOrderID = ""
		fresh.ShortTPOrderID = ""
		fresh.UpdatedAt = time.Now().UTC()
		return true
	}); err != nil {
		e.log.Error("consolidate: failed to update %s after trim: %v", pos.ID, err)
		return
	}
	e.log.Info("consolidate: %s balanced to long=%.6f short=%.6f (trimmed %.6f of %.6f excess)",
		pos.ID, actualNewLong, actualNewShort, filled, excess)

	// Re-read position and place new stop losses with correct sizes.
	updated, err := e.db.GetPosition(pos.ID)
	if err != nil {
		e.log.Error("consolidate: failed to re-read %s for SL placement: %v", pos.ID, err)
		return
	}
	e.attachStopLosses(updated)
	e.api.BroadcastPositionUpdate(updated)
}

// reconcilePartialPosition checks a StatusPartial position against the exchange
// and either promotes it to StatusActive (both legs present), closes one-sided
// exposure, or marks it closed (both legs flat).
func (e *Engine) reconcilePartialPosition(pos *models.ArbitragePosition) {
	longExch, lok := e.exchanges[pos.LongExchange]
	shortExch, sok := e.exchanges[pos.ShortExchange]
	if !lok || !sok {
		return
	}

	longActual, longErr := getExchangePositionSize(longExch, pos.Symbol, "long")
	shortActual, shortErr := getExchangePositionSize(shortExch, pos.Symbol, "short")
	if longErr != nil || shortErr != nil {
		e.log.Warn("reconcilePartial %s: query failed (long=%v short=%v)", pos.ID, longErr, shortErr)
		return
	}

	if longActual == 0 && shortActual == 0 {
		e.log.Info("reconcilePartial %s: both sides zero, closing as failed", pos.ID)
		e.markPartialClosed(pos, "partial_zero_on_reconcile", "entry_failed: no fills on reconcile")
		return
	}

	if longActual == 0 || shortActual == 0 {
		e.log.Warn("reconcilePartial %s: one-sided (long=%.6f short=%.6f)", pos.ID, longActual, shortActual)
		if longActual > 0 {
			rem := e.closeFullyWithRetry(longExch, pos.Symbol, exchange.SideSell, longActual)
			if rem > 0 {
				e.log.Error("ORPHAN: %s long %.6f on %s after partial close", pos.Symbol, rem, pos.LongExchange)
				return
			}
		}
		if shortActual > 0 {
			rem := e.closeFullyWithRetry(shortExch, pos.Symbol, exchange.SideBuy, shortActual)
			if rem > 0 {
				e.log.Error("ORPHAN: %s short %.6f on %s after partial close", pos.Symbol, rem, pos.ShortExchange)
				return
			}
		}
		e.markPartialClosed(pos, "partial_one_sided", "entry_failed: one-sided partial")
		return
	}

	minFill := math.Min(longActual, shortActual)
	trimOK := true
	postLong := longActual
	postShort := shortActual
	if longActual > minFill {
		excess := longActual - minFill
		rem := e.closeFullyWithRetry(longExch, pos.Symbol, exchange.SideSell, excess)
		postLong = minFill + rem // actual size left on exchange
		if rem > 0 {
			e.log.Error("reconcilePartial %s: long trim incomplete, %.6f remaining", pos.ID, rem)
			trimOK = false
		}
	}
	if shortActual > minFill {
		excess := shortActual - minFill
		rem := e.closeFullyWithRetry(shortExch, pos.Symbol, exchange.SideBuy, excess)
		postShort = minFill + rem // actual size left on exchange
		if rem > 0 {
			e.log.Error("reconcilePartial %s: short trim incomplete, %.6f remaining", pos.ID, rem)
			trimOK = false
		}
	}

	if !trimOK {
		// Save actual post-trim sizes, not stale pre-trim values
		pos.LongSize = postLong
		pos.ShortSize = postShort
		pos.UpdatedAt = time.Now().UTC()
		_ = e.db.SavePosition(pos)
		return
	}

	pos.LongSize = minFill
	pos.ShortSize = minFill
	pos.Status = models.StatusActive
	pos.FailureReason = ""
	pos.FailureStage = ""

	if pos.LongEntry <= 0 {
		if bbo, ok := longExch.GetBBO(pos.Symbol); ok && bbo.Ask > 0 {
			pos.LongEntry = bbo.Ask
		}
	}
	if pos.ShortEntry <= 0 {
		if bbo, ok := shortExch.GetBBO(pos.Symbol); ok && bbo.Bid > 0 {
			pos.ShortEntry = bbo.Bid
		}
	}
	pos.EntryNotional = math.Max(pos.LongEntry*pos.LongSize, pos.ShortEntry*pos.ShortSize)
	pos.UpdatedAt = time.Now().UTC()
	_ = e.db.SavePosition(pos)
	e.api.BroadcastPositionUpdate(pos)

	if pos.LongEntry > 0 && pos.ShortEntry > 0 {
		e.attachStopLosses(pos)
	} else {
		e.log.Warn("reconcilePartial %s: promoted but no entry prices, SL skipped", pos.ID)
	}
	e.log.Info("reconcilePartial %s: promoted to active (size=%.6f)", pos.ID, minFill)
}

// markPartialClosed handles close bookkeeping for a failed partial position.
// Uses same pattern as markPositionClosed but without PnL reconciliation.
func (e *Engine) markPartialClosed(pos *models.ArbitragePosition, reason, exitReason string) {
	pos.LongSize = 0
	pos.ShortSize = 0
	pos.Status = models.StatusClosed
	pos.FailureReason = reason
	pos.ExitReason = exitReason
	pos.UpdatedAt = time.Now().UTC()
	_ = e.db.SavePosition(pos)
	e.log.Info("[reconcile-debug] AddToHistory %s: LongTotalFees=%.6f ShortTotalFees=%.6f LongFunding=%.6f ShortFunding=%.6f LongClosePnL=%.6f ShortClosePnL=%.6f HasReconciled=%v",
		pos.ID, pos.LongTotalFees, pos.ShortTotalFees, pos.LongFunding, pos.ShortFunding, pos.LongClosePnL, pos.ShortClosePnL, pos.HasReconciled)
	_ = e.db.AddToHistory(pos)
	e.releasePerpPosition(pos.ID)
	e.api.BroadcastPositionUpdate(pos)
}
