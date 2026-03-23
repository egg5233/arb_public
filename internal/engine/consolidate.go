package engine

import (
	"fmt"
	"time"

	"arb/pkg/exchange"
	"arb/internal/models"
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
	e.consolidatePositions()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.consolidatePositions()
		case <-e.stopCh:
			e.log.Info("consolidator stopped")
			return
		}
	}
}

// consolidatePositions compares local position records with exchange state
// and fixes mismatches.
func (e *Engine) consolidatePositions() {
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
		e.consolidatePosition(pos, siblingTotals)
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

				markPrice, _ := utils.ParseFloat(ep.MarkPrice)
				notional := size * markPrice
				e.log.Warn("consolidate: orphan position on %s: %s %s size=%.6f notional=%.2f USDT",
					name, ep.Symbol, ep.HoldSide, size, notional)

				// Auto-close orphan positions via reduce-only market IOC.
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
				sizeStr := e.formatSize(name, ep.Symbol, size)
				e.log.Info("consolidate: closing orphan %s %s on %s size=%s",
					ep.Symbol, ep.HoldSide, name, sizeStr)
				e.closeLeg(exch, ep.Symbol, closeSide, sizeStr)
			}
		}
	}
}

// consolidatePosition checks a single position's exchange legs and fixes
// the local record if they don't match. siblingTotals maps
// "exchange:symbol:side" → total size claimed by ALL local positions,
// so duplicate-symbol positions sharing the same exchange leg are handled.
func (e *Engine) consolidatePosition(pos *models.ArbitragePosition, siblingTotals map[string]float64) {
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

	// Tolerance: 1% or $1 notional, whichever is larger.
	longDiff := abs(longSize - pos.LongSize)
	shortDiff := abs(shortSize - pos.ShortSize)
	longPct := longDiff / max(pos.LongSize, 1)
	shortPct := shortDiff / max(pos.ShortSize, 1)

	needsUpdate := false

	// Check for missing legs (exchange position is 0 but local says it exists).
	if pos.LongSize > 0 && longSize == 0 {
		e.log.Warn("consolidate: %s long leg missing on %s (local=%.6f exchange=0), closing position",
			pos.ID, pos.LongExchange, pos.LongSize)
		e.markPositionClosed(pos, "long leg missing on exchange")
		return
	}
	if pos.ShortSize > 0 && shortSize == 0 {
		e.log.Warn("consolidate: %s short leg missing on %s (local=%.6f exchange=0), closing position",
			pos.ID, pos.ShortExchange, pos.ShortSize)
		e.markPositionClosed(pos, "short leg missing on exchange")
		return
	}

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
	}
}

// markPositionClosed closes a position that has a missing leg on the exchange.
// If the remaining leg still exists, it closes it too.
func (e *Engine) markPositionClosed(pos *models.ArbitragePosition, reason string) {
	// Try to close the remaining leg if it exists.
	if pos.LongSize > 0 {
		if longExch, ok := e.exchanges[pos.LongExchange]; ok {
			actualSize, err := getExchangePositionSize(longExch, pos.Symbol, "long")
			if err == nil && actualSize > 0 {
				e.log.Info("consolidate: closing remaining long leg on %s: %.6f", pos.LongExchange, actualSize)
				e.closeLeg(longExch, pos.Symbol, exchange.SideSell, e.formatSize(pos.LongExchange, pos.Symbol, actualSize))
			}
		}
	}
	if pos.ShortSize > 0 {
		if shortExch, ok := e.exchanges[pos.ShortExchange]; ok {
			actualSize, err := getExchangePositionSize(shortExch, pos.Symbol, "short")
			if err == nil && actualSize > 0 {
				e.log.Info("consolidate: closing remaining short leg on %s: %.6f", pos.ShortExchange, actualSize)
				e.closeLeg(shortExch, pos.Symbol, exchange.SideBuy, e.formatSize(pos.ShortExchange, pos.Symbol, actualSize))
			}
		}
	}

	pos.Status = models.StatusClosed
	pos.UpdatedAt = time.Now().UTC()
	if err := e.db.SavePosition(pos); err != nil {
		e.log.Error("consolidate: failed to close %s: %v", pos.ID, err)
	}
	if err := e.db.AddToHistory(pos); err != nil {
		e.log.Error("consolidate: failed to add %s to history: %v", pos.ID, err)
	}
	e.api.BroadcastPositionUpdate(pos)
	e.log.Info("consolidate: closed %s (%s)", pos.ID, reason)
}
