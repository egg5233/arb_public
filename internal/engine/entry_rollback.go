package engine

import (
	"context"
	"fmt"
	"math"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
)

func rollbackBBOPrice(exch exchange.Exchange, symbol string, side exchange.Side, fallback float64) float64 {
	if fallback > 0 {
		return fallback
	}
	bbo, ok := exch.GetBBO(symbol)
	if !ok {
		return 0
	}
	switch side {
	case exchange.SideBuy:
		if bbo.Ask > 0 {
			return bbo.Ask
		}
	case exchange.SideSell:
		if bbo.Bid > 0 {
			return bbo.Bid
		}
	}
	if bbo.Bid > 0 && bbo.Ask > 0 {
		return (bbo.Bid + bbo.Ask) / 2
	}
	return 0
}

func annotateFailedEntryRollback(pos *models.ArbitragePosition, reason, stage string, longQty, shortQty, longEntry, shortEntry, longExit, shortExit float64) {
	pos.LongSize = longQty
	pos.ShortSize = shortQty
	pos.LongCloseSize = longQty
	pos.ShortCloseSize = shortQty
	if longEntry > 0 {
		pos.LongEntry = longEntry
	}
	if shortEntry > 0 {
		pos.ShortEntry = shortEntry
	}
	if longExit > 0 {
		pos.LongExit = longExit
	}
	if shortExit > 0 {
		pos.ShortExit = shortExit
	}
	pos.EntryNotional = math.Max(pos.LongEntry*longQty, pos.ShortEntry*shortQty)

	longPnL := 0.0
	if pos.LongEntry > 0 && pos.LongExit > 0 && longQty > 0 {
		longPnL = (pos.LongExit - pos.LongEntry) * longQty
	}
	shortPnL := 0.0
	if pos.ShortEntry > 0 && pos.ShortExit > 0 && shortQty > 0 {
		shortPnL = (pos.ShortEntry - pos.ShortExit) * shortQty
	}
	pos.BasisGainLoss = longPnL + shortPnL
	pos.RealizedPnL = pos.BasisGainLoss + pos.FundingCollected - pos.EntryFees
	pos.Status = models.StatusClosed
	pos.FailureReason = reason
	pos.FailureStage = stage
	pos.ExitReason = "entry_reverted: " + reason
	pos.UpdatedAt = time.Now().UTC()
}

func (e *Engine) saveEntryPartialForTopUp(pos *models.ArbitragePosition, reason string, longExch, shortExch exchange.Exchange, longQty, shortQty, longEntry, shortEntry float64) error {
	if longQty > 0 {
		longEntry = rollbackBBOPrice(longExch, pos.Symbol, exchange.SideBuy, longEntry)
	}
	if shortQty > 0 {
		shortEntry = rollbackBBOPrice(shortExch, pos.Symbol, exchange.SideSell, shortEntry)
	}
	pos.LongSize = longQty
	pos.ShortSize = shortQty
	pos.LongCloseSize = longQty
	pos.ShortCloseSize = shortQty
	if longEntry > 0 {
		pos.LongEntry = longEntry
	}
	if shortEntry > 0 {
		pos.ShortEntry = shortEntry
	}
	pos.EntryNotional = math.Max(pos.LongEntry*longQty, pos.ShortEntry*shortQty)
	pos.Status = models.StatusPartial
	pos.FailureReason = reason
	pos.FailureStage = "entry_topup_pending"
	pos.ExitReason = ""
	pos.UpdatedAt = time.Now().UTC()

	if err := e.db.SavePosition(pos); err != nil {
		return fmt.Errorf("save partial top-up state %s: %w", pos.ID, err)
	}
	if e.api != nil {
		e.api.BroadcastPositionUpdate(pos)
	}
	if e.telegram != nil {
		e.telegram.Send("WARNING %s partial entry pending top-up: long=%.6f short=%.6f reason=%s",
			pos.Symbol, longQty, shortQty, reason)
	}
	return nil
}

func (e *Engine) closeAndRecordFailedEntryRollback(pos *models.ArbitragePosition, reason, stage string, longExch, shortExch exchange.Exchange, longQty, shortQty, longEntry, shortEntry float64) error {
	longEntry = rollbackBBOPrice(longExch, pos.Symbol, exchange.SideBuy, longEntry)
	shortEntry = rollbackBBOPrice(shortExch, pos.Symbol, exchange.SideSell, shortEntry)

	longExit := 0.0
	if longQty > 0 {
		closed, avg := e.closeFullyWithRetryPriced(context.Background(), longExch, pos.Symbol, exchange.SideSell, longQty)
		longExit = rollbackBBOPrice(longExch, pos.Symbol, exchange.SideSell, avg)
		if rem := longQty - closed; rem > 0 && !e.isDust(longExch.Name(), pos.Symbol, rem) {
			e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s after failed-entry rollback",
				pos.Symbol, exchange.SideSell, rem, longExch.Name())
		}
	}

	shortExit := 0.0
	if shortQty > 0 {
		closed, avg := e.closeFullyWithRetryPriced(context.Background(), shortExch, pos.Symbol, exchange.SideBuy, shortQty)
		shortExit = rollbackBBOPrice(shortExch, pos.Symbol, exchange.SideBuy, avg)
		if rem := shortQty - closed; rem > 0 && !e.isDust(shortExch.Name(), pos.Symbol, rem) {
			e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s after failed-entry rollback",
				pos.Symbol, exchange.SideBuy, rem, shortExch.Name())
		}
	}

	annotateFailedEntryRollback(pos, reason, stage, longQty, shortQty, longEntry, shortEntry, longExit, shortExit)
	e.log.Info("failed-entry rollback recorded for %s: long=%.6f@%.8f->%.8f short=%.6f@%.8f->%.8f pnl=%.4f reason=%s",
		pos.ID, pos.LongSize, pos.LongEntry, pos.LongExit, pos.ShortSize, pos.ShortEntry, pos.ShortExit, pos.RealizedPnL, reason)

	e.log.Info("[reconcile-debug] AddToHistory %s: LongTotalFees=%.6f ShortTotalFees=%.6f LongFunding=%.6f ShortFunding=%.6f LongClosePnL=%.6f ShortClosePnL=%.6f HasReconciled=%v",
		pos.ID, pos.LongTotalFees, pos.ShortTotalFees, pos.LongFunding, pos.ShortFunding, pos.LongClosePnL, pos.ShortClosePnL, pos.HasReconciled)
	if err := e.db.AddToHistory(pos); err != nil {
		e.log.Error("failed-entry rollback: AddToHistory failed for %s: %v", pos.ID, err)
	}
	if err := e.db.SavePosition(pos); err != nil {
		return fmt.Errorf("failed-entry rollback save %s: %w", pos.ID, err)
	}
	if e.api != nil {
		e.api.BroadcastPositionUpdate(pos)
	}
	return nil
}
