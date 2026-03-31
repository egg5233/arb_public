package spotengine

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

const spotEntryLockKey = "spot_entry"
const spotQtyTolerance = 1e-6
const spotEntryManualRecoveryReason = "manual_intervention_required"

var spotEntryLockTTL = 5 * time.Minute

// ManualOpen executes a spot-futures arbitrage entry for the given symbol,
// exchange, and direction. It runs synchronously (blocking) and returns an
// error if any pre-check or execution step fails.
func (e *SpotEngine) ManualOpen(symbol, exchName, direction string) error {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	exchName = strings.ToLower(strings.TrimSpace(exchName))
	direction = strings.TrimSpace(direction)

	lock, acquired, err := e.db.AcquireOwnedLock(spotEntryLockKey, spotEntryLockTTL)
	if err != nil {
		return fmt.Errorf("failed to acquire spot entry lock: %w", err)
	}
	if !acquired {
		return errors.New("spot entry already in progress")
	}
	defer func() {
		if err := lock.Release(); err != nil {
			e.log.Warn("ManualOpen: release entry lock: %v", err)
		}
	}()
	requireEntryLock := func(action string) error {
		if err := lock.Check(); err != nil {
			return fmt.Errorf("spot entry lock lost before %s: %w", action, err)
		}
		return nil
	}

	e.log.Info("ManualOpen: %s on %s direction=%s", symbol, exchName, direction)

	// ---------------------------------------------------------------
	// 1. Pre-checks
	// ---------------------------------------------------------------

	// 1a. Validate direction.
	if direction != "borrow_sell_long" && direction != "buy_spot_short" {
		return fmt.Errorf("invalid direction %q — must be borrow_sell_long or buy_spot_short", direction)
	}

	// 1b. Find opportunity in latest scan results.
	opps := e.getLatestOpps()
	var opp *SpotArbOpportunity
	for i := range opps {
		if opps[i].Symbol == symbol && opps[i].Exchange == exchName && opps[i].Direction == direction {
			opp = &opps[i]
			break
		}
	}
	if opp == nil {
		return fmt.Errorf("opportunity not found in latest scan for %s on %s (%s)", symbol, exchName, direction)
	}
	if opp.FilterStatus != "" {
		return fmt.Errorf("opportunity %s on %s (%s) is filtered: %s", symbol, exchName, direction, opp.FilterStatus)
	}

	// 1c. Check exchange supports SpotMarginExchange.
	smExch, ok := e.spotMargin[exchName]
	if !ok {
		return fmt.Errorf("exchange %s does not support spot margin", exchName)
	}
	futExch, ok := e.exchanges[exchName]
	if !ok {
		return fmt.Errorf("exchange %s not found", exchName)
	}

	// 1d. Check no duplicate symbol already open (any exchange).
	active, err := e.db.GetActiveSpotPositions()
	if err != nil {
		return fmt.Errorf("failed to check active positions: %w", err)
	}
	for _, pos := range active {
		if pos.Symbol == symbol {
			return fmt.Errorf("position for %s already open on %s", symbol, pos.Exchange)
		}
	}

	// 1e. Check capacity.
	if len(active) >= e.cfg.SpotFuturesMaxPositions {
		return fmt.Errorf("at max capacity (%d/%d)", len(active), e.cfg.SpotFuturesMaxPositions)
	}

	// 1f. Dry run check.
	if e.cfg.SpotFuturesDryRun {
		return fmt.Errorf("dry run mode — trade not executed")
	}
	if err := requireEntryLock("orderbook lookup"); err != nil {
		return err
	}

	// ---------------------------------------------------------------
	// 2. Get spot price via orderbook
	// ---------------------------------------------------------------
	ob, err := futExch.GetOrderbook(symbol, 5)
	if err != nil {
		return fmt.Errorf("failed to get orderbook for %s on %s: %w", symbol, exchName, err)
	}
	if len(ob.Bids) == 0 || len(ob.Asks) == 0 {
		return fmt.Errorf("empty orderbook for %s on %s", symbol, exchName)
	}
	midPrice := (ob.Bids[0].Price + ob.Asks[0].Price) / 2
	e.log.Info("ManualOpen: %s mid price = %.6f", symbol, midPrice)

	// ---------------------------------------------------------------
	// 3. Position sizing
	// ---------------------------------------------------------------
	capital := e.capitalForExchange(exchName)
	baseCoin := opp.BaseCoin
	rawSize := capital / midPrice

	// For Direction A, cap by max borrowable.
	if direction == "borrow_sell_long" {
		mb, err := smExch.GetMarginBalance(baseCoin)
		if err != nil {
			e.log.Warn("ManualOpen: GetMarginBalance(%s) failed: %v — proceeding with computed size", baseCoin, err)
		} else if mb.MaxBorrowable > 0 && rawSize > mb.MaxBorrowable {
			e.log.Info("ManualOpen: capping size from %.6f to MaxBorrowable %.6f", rawSize, mb.MaxBorrowable)
			rawSize = mb.MaxBorrowable
		}
	}

	// Round size to 6 decimal places (safe default for Phase 3a).
	size := math.Floor(rawSize*1e6) / 1e6
	if size <= 0 {
		return fmt.Errorf("computed size is 0 for %s (capital=%.2f price=%.6f)", symbol, capital, midPrice)
	}
	sizeStr := utils.FormatSize(size, 6)
	plannedNotional := size * midPrice
	e.log.Info("ManualOpen: %s size=%.6f (%s) notional=%.2f USDT", symbol, size, sizeStr, plannedNotional)

	now := time.Now().UTC()
	posID := utils.GenerateID("sf-"+symbol, now.UnixMilli())
	entryPos := &models.SpotFuturesPosition{
		ID:               posID,
		Symbol:           symbol,
		BaseCoin:         baseCoin,
		Exchange:         exchName,
		Direction:        direction,
		Status:           models.SpotStatusPending,
		SpotSize:         size,
		BorrowRateHourly: opp.BorrowAPR / 8760,
		FundingAPR:       opp.FundingAPR,
		FeeAPR:           opp.FeeAPR,
		CurrentBorrowAPR: opp.BorrowAPR,
		NotionalUSDT:     plannedNotional,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if direction == "borrow_sell_long" {
		entryPos.FuturesSide = "long"
	} else {
		entryPos.FuturesSide = "short"
	}

	if err := requireEntryLock("capital reservation"); err != nil {
		return err
	}
	reservation, err := e.reserveSpotCapital(exchName, plannedNotional)
	if err != nil {
		return fmt.Errorf("capital allocator rejected: %w", err)
	}
	if err := requireEntryLock("leverage setup"); err != nil {
		e.releaseSpotReservation(reservation)
		return err
	}

	// ---------------------------------------------------------------
	// 4. Set leverage on futures
	// ---------------------------------------------------------------
	leverage := e.cfg.SpotFuturesLeverage
	if leverage <= 0 {
		leverage = 3
	}
	leverageStr := strconv.Itoa(leverage)
	if err := futExch.SetLeverage(symbol, leverageStr, ""); err != nil {
		e.log.Warn("ManualOpen: SetLeverage(%s, %s) warning: %v", symbol, leverageStr, err)
		// Non-fatal — some exchanges return error if already set.
	}

	// ---------------------------------------------------------------
	// 5. Execute based on direction
	// ---------------------------------------------------------------
	var spotEntryPrice, futuresEntryPrice float64
	var spotFilledQty, futuresFilledQty float64
	var futuresSide string
	var borrowAmount float64

	switch direction {
	case "borrow_sell_long":
		spotEntryPrice, futuresEntryPrice, spotFilledQty, futuresFilledQty, borrowAmount, err = e.executeBorrowSellLong(smExch, futExch, symbol, baseCoin, sizeStr, size, requireEntryLock, entryPos)
		futuresSide = "long"
	case "buy_spot_short":
		spotEntryPrice, futuresEntryPrice, spotFilledQty, futuresFilledQty, err = e.executeBuySpotShort(smExch, futExch, symbol, sizeStr, size, requireEntryLock, entryPos)
		futuresSide = "short"
	}

	if err != nil {
		var pendingErr *pendingSpotEntryError
		if errors.As(err, &pendingErr) {
			if pendingErr.pendingPos != nil {
				if saveErr := e.db.SaveSpotPosition(pendingErr.pendingPos); saveErr != nil {
					e.log.Error("ManualOpen: failed to save pending spot recovery %s: %v", pendingErr.posID, saveErr)
					return fmt.Errorf("%w (manual recovery position %s could not be persisted: %v; capital reservation left uncommitted)",
						err, pendingErr.posID, saveErr)
				} else if e.api != nil {
					e.api.BroadcastSpotPositionUpdate(pendingErr.pendingPos)
				}
			}
			amount := plannedNotional
			if pendingErr.capitalAmount > 0 {
				amount = pendingErr.capitalAmount
			}
			if commitErr := e.commitSpotCapital(reservation, pendingErr.posID, amount); commitErr != nil {
				e.log.Error("ManualOpen: capital commit failed for pending entry %s: %v", pendingErr.posID, commitErr)
			}
			return err
		}
		e.releaseSpotReservation(reservation)
		return fmt.Errorf("execution failed: %w", err)
	}

	actualNotional := spotFilledQty * spotEntryPrice

	// Calculate entry fees (2 legs: spot + futures, taker rate).
	takerFee := spotFees[exchName]
	if takerFee == 0 {
		takerFee = 0.0005 // default 0.05%
	}
	entryFees := (spotFilledQty * spotEntryPrice * takerFee) + (futuresFilledQty * futuresEntryPrice * takerFee)

	// ---------------------------------------------------------------
	// 6. Save position
	// ---------------------------------------------------------------
	pos := entryPos
	pos.Status = models.SpotStatusActive
	pos.SpotSize = spotFilledQty
	pos.SpotEntryPrice = spotEntryPrice
	pos.FuturesSize = futuresFilledQty
	pos.FuturesEntry = futuresEntryPrice
	pos.FuturesSide = futuresSide
	pos.BorrowAmount = borrowAmount
	pos.NotionalUSDT = actualNotional
	pos.EntryFees = entryFees
	pos.PendingEntryOrderID = ""
	pos.UpdatedAt = time.Now().UTC()

	if err := e.db.SaveSpotPosition(pos); err != nil {
		e.releaseSpotReservation(reservation)
		e.log.Error("ManualOpen: failed to save position: %v", err)
		return fmt.Errorf("position executed but failed to save: %w", err)
	}
	if err := e.commitSpotCapital(reservation, posID, actualNotional); err != nil {
		e.log.Error("ManualOpen: capital commit failed: %v", err)
	}

	e.api.BroadcastSpotPositionUpdate(pos)
	e.log.Info("ManualOpen: SUCCESS — %s on %s [%s] spot=%.6f@%.6f futures=%.6f@%.6f notional=%.2f",
		symbol, exchName, direction, spotFilledQty, spotEntryPrice, futuresFilledQty, futuresEntryPrice, actualNotional)

	return nil
}

// executeBorrowSellLong handles Direction A: borrow coin, sell spot, long futures.
func (e *SpotEngine) executeBorrowSellLong(
	smExch exchange.SpotMarginExchange,
	futExch exchange.Exchange,
	symbol, baseCoin, sizeStr string,
	size float64,
	requireEntryLock func(string) error,
	pos *models.SpotFuturesPosition,
) (spotAvg, futAvg, spotFilled, futFilled, borrowAmt float64, err error) {

	// Step 1: Borrow
	if err := requireEntryLock("margin borrow"); err != nil {
		return 0, 0, 0, 0, 0, err
	}
	e.log.Info("ManualOpen [borrow_sell_long] step 1: MarginBorrow %s %s", baseCoin, sizeStr)
	err = smExch.MarginBorrow(exchange.MarginBorrowParams{
		Coin:   baseCoin,
		Amount: sizeStr,
	})
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("MarginBorrow failed: %w", err)
	}
	borrowAmt = size
	pos.BorrowAmount = borrowAmt
	e.log.Info("ManualOpen [borrow_sell_long] step 1: borrowed %s %s", sizeStr, baseCoin)

	// Step 2: Sell spot (margin order)
	if err := requireEntryLock("spot sell"); err != nil {
		e.rollbackBorrow(smExch, baseCoin, sizeStr)
		return 0, 0, 0, 0, 0, err
	}
	e.log.Info("ManualOpen [borrow_sell_long] step 2: PlaceSpotMarginOrder SELL %s %s", symbol, sizeStr)
	spotOrderID, err := smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
		Symbol:    symbol,
		Side:      exchange.SideSell,
		OrderType: "market",
		Size:      sizeStr,
		Force:     "ioc",
	})
	if err != nil {
		// Rollback: repay the borrow.
		e.log.Error("ManualOpen [borrow_sell_long] step 2 FAILED: %v — rolling back borrow", err)
		e.rollbackBorrow(smExch, baseCoin, sizeStr)
		return 0, 0, 0, 0, 0, fmt.Errorf("spot sell failed: %w", err)
	}
	e.log.Info("ManualOpen [borrow_sell_long] step 2: spot order placed: %s", spotOrderID)

	// Confirm spot fill.
	spotFilled, spotAvg, confirmed, confErr := e.confirmSpotFill(smExch, futExch, spotOrderID, symbol, size)
	if !confirmed {
		if cpErr := e.persistPendingEntry(pos, spotOrderID); cpErr != nil {
			return 0, 0, 0, 0, 0, e.abortAcceptedSpotEntry(smExch, futExch, pos, spotOrderID, symbol, size, cpErr)
		}
		return 0, 0, 0, 0, 0, &pendingSpotEntryError{posID: pos.ID, orderID: spotOrderID, err: confErr}
	}
	if spotFilled <= 0 {
		e.log.Error("ManualOpen [borrow_sell_long] step 2: spot order got 0 fill — rolling back borrow")
		e.rollbackBorrow(smExch, baseCoin, sizeStr)
		return 0, 0, 0, 0, 0, fmt.Errorf("spot sell got 0 fill (order %s)", spotOrderID)
	}
	e.log.Info("ManualOpen [borrow_sell_long] step 2: spot fill=%.6f avg=%.6f", spotFilled, spotAvg)

	// Step 3: Long futures
	spotFilledStr := utils.FormatSize(spotFilled, 6)
	if err := requireEntryLock("futures long"); err != nil {
		e.rollbackSpotOrder(smExch, symbol, exchange.SideBuy, spotFilledStr)
		e.rollbackBorrow(smExch, baseCoin, sizeStr)
		return 0, 0, 0, 0, 0, err
	}
	e.log.Info("ManualOpen [borrow_sell_long] step 3: PlaceOrder futures BUY %s %s", symbol, spotFilledStr)
	futOrderID, err := futExch.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:    symbol,
		Side:      exchange.SideBuy,
		OrderType: "market",
		Size:      spotFilledStr,
		Force:     "ioc",
	})
	if err != nil {
		// Rollback: reverse the spot sell by buying back.
		e.log.Error("ManualOpen [borrow_sell_long] step 3 FAILED: %v — rolling back spot sell", err)
		e.rollbackSpotOrder(smExch, symbol, exchange.SideBuy, spotFilledStr)
		e.rollbackBorrow(smExch, baseCoin, sizeStr)
		return 0, 0, 0, 0, 0, fmt.Errorf("futures long failed: %w", err)
	}
	e.log.Info("ManualOpen [borrow_sell_long] step 3: futures order placed: %s", futOrderID)

	// Confirm futures fill.
	futFilled, futAvg = e.confirmFuturesFill(futExch, futOrderID, symbol)
	if futFilled <= 0 {
		e.log.Error("ManualOpen [borrow_sell_long] step 3: futures order got 0 fill — rolling back spot sell")
		e.rollbackSpotOrder(smExch, symbol, exchange.SideBuy, spotFilledStr)
		e.rollbackBorrow(smExch, baseCoin, sizeStr)
		return 0, 0, 0, 0, 0, fmt.Errorf("futures long got 0 fill (order %s)", futOrderID)
	}
	e.log.Info("ManualOpen [borrow_sell_long] step 3: futures fill=%.6f avg=%.6f", futFilled, futAvg)

	return spotAvg, futAvg, spotFilled, futFilled, borrowAmt, nil
}

// executeBuySpotShort handles Direction B: buy spot, short futures.
func (e *SpotEngine) executeBuySpotShort(
	smExch exchange.SpotMarginExchange,
	futExch exchange.Exchange,
	symbol, sizeStr string,
	size float64,
	requireEntryLock func(string) error,
	pos *models.SpotFuturesPosition,
) (spotAvg, futAvg, spotFilled, futFilled float64, err error) {

	// Step 1: Buy spot (margin order)
	if err := requireEntryLock("spot buy"); err != nil {
		return 0, 0, 0, 0, err
	}
	e.log.Info("ManualOpen [buy_spot_short] step 1: PlaceSpotMarginOrder BUY %s %s", symbol, sizeStr)
	spotOrderID, err := smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
		Symbol:    symbol,
		Side:      exchange.SideBuy,
		OrderType: "market",
		Size:      sizeStr,
		Force:     "ioc",
	})
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("spot buy failed: %w", err)
	}
	e.log.Info("ManualOpen [buy_spot_short] step 1: spot order placed: %s", spotOrderID)

	// Confirm spot fill.
	spotFilled, spotAvg, confirmed, confErr := e.confirmSpotFill(smExch, futExch, spotOrderID, symbol, size)
	if !confirmed {
		if cpErr := e.persistPendingEntry(pos, spotOrderID); cpErr != nil {
			return 0, 0, 0, 0, e.abortAcceptedSpotEntry(smExch, futExch, pos, spotOrderID, symbol, size, cpErr)
		}
		return 0, 0, 0, 0, &pendingSpotEntryError{posID: pos.ID, orderID: spotOrderID, err: confErr}
	}
	if spotFilled <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("spot buy got 0 fill (order %s)", spotOrderID)
	}
	e.log.Info("ManualOpen [buy_spot_short] step 1: spot fill=%.6f avg=%.6f", spotFilled, spotAvg)

	// Step 2: Short futures
	spotFilledStr := utils.FormatSize(spotFilled, 6)
	if err := requireEntryLock("futures short"); err != nil {
		e.rollbackSpotOrder(smExch, symbol, exchange.SideSell, spotFilledStr)
		return 0, 0, 0, 0, err
	}
	e.log.Info("ManualOpen [buy_spot_short] step 2: PlaceOrder futures SELL %s %s", symbol, spotFilledStr)
	futOrderID, err := futExch.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:    symbol,
		Side:      exchange.SideSell,
		OrderType: "market",
		Size:      spotFilledStr,
		Force:     "ioc",
	})
	if err != nil {
		// Rollback: sell the spot back.
		e.log.Error("ManualOpen [buy_spot_short] step 2 FAILED: %v — rolling back spot buy", err)
		e.rollbackSpotOrder(smExch, symbol, exchange.SideSell, spotFilledStr)
		return 0, 0, 0, 0, fmt.Errorf("futures short failed: %w", err)
	}
	e.log.Info("ManualOpen [buy_spot_short] step 2: futures order placed: %s", futOrderID)

	// Confirm futures fill.
	futFilled, futAvg = e.confirmFuturesFill(futExch, futOrderID, symbol)
	if futFilled <= 0 {
		e.log.Error("ManualOpen [buy_spot_short] step 2: futures order got 0 fill — rolling back spot buy")
		e.rollbackSpotOrder(smExch, symbol, exchange.SideSell, spotFilledStr)
		return 0, 0, 0, 0, fmt.Errorf("futures short got 0 fill (order %s)", futOrderID)
	}
	e.log.Info("ManualOpen [buy_spot_short] step 2: futures fill=%.6f avg=%.6f", futFilled, futAvg)

	return spotAvg, futAvg, spotFilled, futFilled, nil
}

// ---------------------------------------------------------------------------
// Fill confirmation
// ---------------------------------------------------------------------------

// confirmFuturesFill checks WS then REST to get fill quantity and average price
// for a futures IOC order. Mirrors the perp-perp engine's confirmFill pattern.
func (e *SpotEngine) confirmFuturesFill(exch exchange.Exchange, orderID, symbol string) (filledQty, avgPrice float64) {
	deadline := time.Now().Add(5 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		if upd, ok := exch.GetOrderUpdate(orderID); ok {
			if upd.Status == "filled" || upd.Status == "cancelled" {
				return upd.FilledVolume, upd.AvgPrice
			}
		}
		if time.Now().After(deadline) {
			break
		}
		<-ticker.C
	}

	// Timeout — try WS one more time.
	if upd, ok := exch.GetOrderUpdate(orderID); ok {
		if upd.Status == "filled" || upd.Status == "cancelled" {
			e.log.Info("confirmFuturesFill: WS terminal %s: status=%s filled=%.6f avg=%.8f",
				orderID, upd.Status, upd.FilledVolume, upd.AvgPrice)
			return upd.FilledVolume, upd.AvgPrice
		}
		e.log.Warn("confirmFuturesFill: timeout %s: WS status=%s filled=%.6f (non-terminal)",
			orderID, upd.Status, upd.FilledVolume)
	} else {
		e.log.Warn("confirmFuturesFill: timeout %s: no WS update", orderID)
	}

	// Cancel any resting order and query REST.
	if err := exch.CancelOrder(symbol, orderID); err != nil {
		e.log.Warn("confirmFuturesFill: cancel %s: %v", orderID, err)
	}
	time.Sleep(200 * time.Millisecond)

	restFilled, restErr := exch.GetOrderFilledQty(orderID, symbol)
	if restErr != nil {
		e.log.Warn("confirmFuturesFill: REST query %s failed: %v", orderID, restErr)
		return 0, 0
	}
	e.log.Info("confirmFuturesFill: REST %s filled=%.6f", orderID, restFilled)

	// Try to get avg price from WS store after REST fallback.
	if upd, ok := exch.GetOrderUpdate(orderID); ok && upd.AvgPrice > 0 {
		return restFilled, upd.AvgPrice
	}
	return restFilled, 0
}

type pendingSpotExitError struct {
	orderID string
	err     error
}

func (e *pendingSpotExitError) Error() string {
	if e.err == nil {
		return fmt.Sprintf("spot exit order %s awaiting confirmation", e.orderID)
	}
	return fmt.Sprintf("spot exit order %s awaiting confirmation: %v", e.orderID, e.err)
}

func (*pendingSpotExitError) nonRetryable() bool { return true }

type pendingSpotEntryError struct {
	posID         string
	orderID       string
	state         string
	err           error
	pendingPos    *models.SpotFuturesPosition
	capitalAmount float64
}

func (e *pendingSpotEntryError) Error() string {
	state := e.state
	if state == "" {
		state = "spot entry pending confirmation"
	}
	if e.err == nil {
		return fmt.Sprintf("%s on order %s", state, e.orderID)
	}
	return fmt.Sprintf("%s on order %s: %v", state, e.orderID, e.err)
}

// Without a durable pending-entry checkpoint, immediately unwind the accepted
// spot leg so restart recovery never depends on a record that was not saved.
func (e *SpotEngine) abortAcceptedSpotEntry(
	smExch exchange.SpotMarginExchange,
	futExch exchange.Exchange,
	pos *models.SpotFuturesPosition,
	orderID, symbol string,
	expectedQty float64,
	persistErr error,
) error {
	filledQty, filledAvg, confirmed, confErr := e.confirmSpotFill(smExch, futExch, orderID, symbol, expectedQty)
	recoveryAvg := filledAvg
	if recoveryAvg <= 0 && pos.SpotSize > 0 && pos.NotionalUSDT > 0 {
		recoveryAvg = pos.NotionalUSDT / pos.SpotSize
	}
	if !confirmed {
		return newManualRecoveryPendingEntryError(
			pos,
			orderID,
			"",
			fmt.Errorf("spot entry order %s was accepted but pending entry save failed: %w (cleanup could not reconcile order state: %v)",
				orderID, persistErr, confErr),
			expectedQty,
			recoveryAvg,
		)
	}

	if filledQty > 0 {
		reverseSide := exchange.SideSell
		action := "sell spot"
		if pos.Direction == "borrow_sell_long" {
			reverseSide = exchange.SideBuy
			action = "buy back spot"
		}
		reverseQty := utils.FormatSize(filledQty, 6)
		reverseOrderID, err := smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
			Symbol:    symbol,
			Side:      reverseSide,
			OrderType: "market",
			Size:      reverseQty,
			Force:     "ioc",
		})
		if err != nil {
			return newManualRecoveryPendingEntryError(
				pos,
				orderID,
				"",
				fmt.Errorf("spot entry order %s was accepted but pending entry save failed: %w (cleanup could not %s %.6f: %v)",
					orderID, persistErr, action, filledQty, err),
				filledQty,
				recoveryAvg,
			)
		}
		cleanupFilled, _, cleanupConfirmed, cleanupErr := e.confirmSpotFill(smExch, futExch, reverseOrderID, symbol, filledQty)
		if !cleanupConfirmed {
			return newManualRecoveryPendingEntryError(
				pos,
				orderID,
				reverseOrderID,
				fmt.Errorf("spot entry order %s was accepted but pending entry save failed: %w (cleanup order %s could not be confirmed flat after %s %.6f: %v; manual intervention required)",
					orderID, persistErr, reverseOrderID, action, filledQty, cleanupErr),
				filledQty,
				recoveryAvg,
			)
		}
		if !spotQtyComplete(cleanupFilled, filledQty) {
			return newManualRecoveryPendingEntryError(
				pos,
				orderID,
				reverseOrderID,
				fmt.Errorf("spot entry order %s was accepted but pending entry save failed: %w (cleanup order %s only reconciled %.6f of %.6f after %s; manual intervention required)",
					orderID, persistErr, reverseOrderID, cleanupFilled, filledQty, action),
				spotRemainingQty(cleanupFilled, filledQty),
				recoveryAvg,
			)
		}
		e.log.Warn("ManualOpen: pending entry save failed after accepted spot order %s; cleanup %s order %s confirmed %.6f",
			orderID, action, reverseOrderID, cleanupFilled)
	}

	if pos.Direction == "borrow_sell_long" && pos.BorrowAmount > 0 {
		repayAmount := utils.FormatSize(pos.BorrowAmount, 6)
		if err := smExch.MarginRepay(exchange.MarginRepayParams{
			Coin:   pos.BaseCoin,
			Amount: repayAmount,
		}); err != nil {
			return fmt.Errorf("spot entry order %s was accepted but pending entry save failed: %w (cleanup repay %s %s failed: %v)",
				orderID, persistErr, repayAmount, pos.BaseCoin, err)
		}
	}

	if filledQty > 0 {
		return fmt.Errorf("spot entry order %s was accepted but pending entry save failed: %w (cleaned up %.6f spot fill before aborting entry)",
			orderID, persistErr, filledQty)
	}
	return fmt.Errorf("spot entry order %s was accepted but pending entry save failed: %w (order reconciled unfilled and entry was aborted)",
		orderID, persistErr)
}

func newManualRecoveryPendingEntryError(
	pos *models.SpotFuturesPosition,
	orderID string,
	cleanupOrderID string,
	cause error,
	spotQty, spotAvg float64,
) error {
	if pos == nil {
		return cause
	}

	recoveryPos := *pos
	recoveryPos.Status = models.SpotStatusPending
	recoveryPos.PendingEntryOrderID = ""
	recoveryPos.PendingSpotExitOrderID = cleanupOrderID
	recoveryPos.ExitReason = spotEntryManualRecoveryReason
	recoveryPos.FuturesSize = 0
	recoveryPos.FuturesEntry = 0
	recoveryPos.NotionalUSDT = 0
	recoveryPos.UpdatedAt = time.Now().UTC()

	if spotQty > 0 {
		recoveryPos.SpotSize = spotQty
		if spotAvg > 0 {
			recoveryPos.SpotEntryPrice = spotAvg
			recoveryPos.NotionalUSDT = spotQty * spotAvg
		}
		if recoveryPos.Direction == "borrow_sell_long" {
			recoveryPos.BorrowAmount = spotQty
		}
	} else {
		recoveryPos.SpotSize = 0
		recoveryPos.SpotEntryPrice = 0
		if recoveryPos.Direction == "borrow_sell_long" {
			recoveryPos.BorrowAmount = 0
		}
	}

	return &pendingSpotEntryError{
		posID:         recoveryPos.ID,
		orderID:       orderID,
		state:         "manual recovery required for accepted spot entry",
		err:           cause,
		pendingPos:    &recoveryPos,
		capitalAmount: recoveryPos.NotionalUSDT,
	}
}

func spotQtyComplete(filled, target float64) bool {
	if target <= 0 {
		return filled > spotQtyTolerance
	}
	return filled+spotQtyTolerance >= target
}

func spotRemainingQty(filled, target float64) float64 {
	remaining := target - filled
	if remaining < spotQtyTolerance {
		return 0
	}
	return remaining
}

func isActiveSpotOrderStatus(status string) bool {
	switch strings.ToLower(status) {
	case "live", "new", "open", "partial_fill", "partially_filled":
		return true
	default:
		return false
	}
}

func spotExitComplete(pos *models.SpotFuturesPosition) bool {
	return spotQtyComplete(pos.SpotExitFilledQty, pos.SpotSize)
}

func remainingSpotExitQty(pos *models.SpotFuturesPosition) float64 {
	return spotRemainingQty(pos.SpotExitFilledQty, pos.SpotSize)
}

func applySpotExitFill(pos *models.SpotFuturesPosition, filledQty, avgPrice float64) {
	if filledQty <= 0 {
		return
	}

	prevQty := pos.SpotExitFilledQty
	totalQty := prevQty + filledQty
	if spotQtyComplete(totalQty, pos.SpotSize) {
		totalQty = pos.SpotSize
	}

	if avgPrice > 0 {
		if prevQty > 0 && pos.SpotExitPrice > 0 && totalQty > 0 {
			notional := (pos.SpotExitPrice * prevQty) + (avgPrice * filledQty)
			pos.SpotExitPrice = notional / totalQty
		} else {
			pos.SpotExitPrice = avgPrice
		}
	}

	pos.SpotExitFilledQty = totalQty
	pos.SpotExitFilled = spotExitComplete(pos)
}

func (e *SpotEngine) persistPendingEntry(pos *models.SpotFuturesPosition, orderID string) error {
	pos.Status = models.SpotStatusPending
	pos.PendingEntryOrderID = orderID
	pos.UpdatedAt = time.Now().UTC()
	if err := e.db.SaveSpotPosition(pos); err != nil {
		return err
	}
	if e.api != nil {
		e.api.BroadcastSpotPositionUpdate(pos)
	}
	return nil
}

func (e *SpotEngine) abandonPendingEntry(pos *models.SpotFuturesPosition, reason string) {
	pos.Status = models.SpotStatusClosed
	pos.PendingEntryOrderID = ""
	pos.ExitReason = reason
	pos.UpdatedAt = time.Now().UTC()
	if err := e.db.SaveSpotPosition(pos); err != nil {
		e.log.Error("pending entry cleanup: failed to save %s: %v", pos.ID, err)
		return
	}
	e.releaseSpotPosition(pos.ID)
	if e.api != nil {
		e.api.BroadcastSpotPositionUpdate(pos)
	}
}

func (e *SpotEngine) reconcilePendingEntry(pos *models.SpotFuturesPosition) {
	lock, acquired, err := e.db.AcquireOwnedLock(spotEntryLockKey, spotEntryLockTTL)
	if err != nil {
		e.log.Error("pending entry %s: acquire lock: %v", pos.ID, err)
		return
	}
	if !acquired {
		return
	}
	defer func() {
		if err := lock.Release(); err != nil {
			e.log.Warn("pending entry %s: release lock: %v", pos.ID, err)
		}
	}()

	smExch, ok := e.spotMargin[pos.Exchange]
	if !ok {
		e.log.Error("pending entry %s: no SpotMarginExchange for %s", pos.ID, pos.Exchange)
		return
	}
	futExch, ok := e.exchanges[pos.Exchange]
	if !ok {
		e.log.Error("pending entry %s: no futures exchange for %s", pos.ID, pos.Exchange)
		return
	}

	if pos.PendingEntryOrderID != "" {
		filled, avg, confirmed, confErr := e.confirmSpotFill(smExch, futExch, pos.PendingEntryOrderID, pos.Symbol, pos.SpotSize)
		if !confirmed {
			e.log.Warn("pending entry %s: spot order %s still unconfirmed: %v", pos.ID, pos.PendingEntryOrderID, confErr)
			return
		}

		pos.PendingEntryOrderID = ""
		if filled <= 0 {
			if pos.Direction == "borrow_sell_long" && pos.BorrowAmount > 0 {
				e.rollbackBorrow(smExch, pos.BaseCoin, utils.FormatSize(pos.BorrowAmount, 6))
			}
			e.abandonPendingEntry(pos, "entry_unfilled")
			return
		}

		pos.SpotSize = filled
		pos.SpotEntryPrice = avg
		pos.NotionalUSDT = filled * avg
		pos.UpdatedAt = time.Now().UTC()
		if err := e.db.SaveSpotPosition(pos); err != nil {
			e.log.Error("pending entry %s: failed to checkpoint confirmed spot fill: %v", pos.ID, err)
			return
		}
		e.updateSpotPositionCapital(pos.ID, pos.Exchange, pos.NotionalUSDT)
		if e.api != nil {
			e.api.BroadcastSpotPositionUpdate(pos)
		}
	}

	if pos.FuturesSize > 0 {
		pos.Status = models.SpotStatusActive
		pos.UpdatedAt = time.Now().UTC()
		if err := e.db.SaveSpotPosition(pos); err != nil {
			e.log.Error("pending entry %s: failed to promote already-hedged position: %v", pos.ID, err)
			return
		}
		if e.api != nil {
			e.api.BroadcastSpotPositionUpdate(pos)
		}
		return
	}

	if pos.SpotSize <= 0 {
		e.log.Warn("pending entry %s: no confirmed spot size yet", pos.ID)
		return
	}

	spotFilledStr := utils.FormatSize(pos.SpotSize, 6)
	var side exchange.Side
	switch pos.Direction {
	case "borrow_sell_long":
		side = exchange.SideBuy
	case "buy_spot_short":
		side = exchange.SideSell
	default:
		e.log.Error("pending entry %s: unknown direction %q", pos.ID, pos.Direction)
		return
	}

	orderID, err := futExch.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:    pos.Symbol,
		Side:      side,
		OrderType: "market",
		Size:      spotFilledStr,
		Force:     "ioc",
	})
	if err != nil {
		e.log.Warn("pending entry %s: futures hedge placement failed: %v", pos.ID, err)
		return
	}

	futFilled, futAvg := e.confirmFuturesFill(futExch, orderID, pos.Symbol)
	if futFilled <= 0 {
		e.log.Warn("pending entry %s: futures hedge order %s got 0 fill", pos.ID, orderID)
		return
	}

	pos.FuturesSize = futFilled
	pos.FuturesEntry = futAvg
	pos.Status = models.SpotStatusActive
	takerFee := spotFees[pos.Exchange]
	if takerFee == 0 {
		takerFee = 0.0005
	}
	pos.EntryFees = (pos.SpotSize * pos.SpotEntryPrice * takerFee) + (futFilled * futAvg * takerFee)
	pos.UpdatedAt = time.Now().UTC()
	if err := e.db.SaveSpotPosition(pos); err != nil {
		e.log.Error("pending entry %s: failed to promote active position: %v", pos.ID, err)
		return
	}
	if e.api != nil {
		e.api.BroadcastSpotPositionUpdate(pos)
	}
	e.log.Info("pending entry %s recovered: %s on %s is now active", pos.ID, pos.Symbol, pos.Exchange)
}

// confirmSpotFill confirms a spot margin order fill. Spot margin orders may
// not appear in the futures WS private stream, so we wait briefly, try the
// shared WS cache, then fall back to the exchange's native spot margin order
// query. The third return value distinguishes "unconfirmed" from a confirmed
// zero fill.
func (e *SpotEngine) confirmSpotFill(smExch exchange.SpotMarginExchange, exch exchange.Exchange, orderID, symbol string, expectedQty float64) (filledQty, avgPrice float64, confirmed bool, err error) {
	// Wait a moment for the order to settle.
	time.Sleep(2 * time.Second)

	// Try WS first (some exchanges route spot order updates here).
	if upd, ok := exch.GetOrderUpdate(orderID); ok {
		status := strings.ToLower(upd.Status)
		switch {
		case status == "filled" && spotQtyComplete(upd.FilledVolume, expectedQty):
			e.log.Info("confirmSpotFill: WS %s: status=%s filled=%.6f avg=%.8f", orderID, upd.Status, upd.FilledVolume, upd.AvgPrice)
			return upd.FilledVolume, upd.AvgPrice, true, nil
		case status == "filled":
			e.log.Warn("confirmSpotFill: WS %s reported filled=%.6f < expected=%.6f; waiting for native reconciliation",
				orderID, upd.FilledVolume, expectedQty)
		case isActiveSpotOrderStatus(status):
			err := fmt.Errorf("spot order %s still active with status %s (filled=%.6f)", orderID, upd.Status, upd.FilledVolume)
			e.log.Info("confirmSpotFill: %v", err)
		case status == "cancelled" || status == "closed" || status == "rejected" || status == "deactivated":
			e.log.Info("confirmSpotFill: WS terminal %s: status=%s filled=%.6f avg=%.8f",
				orderID, upd.Status, upd.FilledVolume, upd.AvgPrice)
			return upd.FilledVolume, upd.AvgPrice, true, nil
		}
	}

	querier, ok := smExch.(exchange.SpotMarginOrderQuerier)
	if !ok {
		err := fmt.Errorf("spot margin order query unsupported for %s", symbol)
		e.log.Warn("confirmSpotFill: %s", err)
		return 0, 0, false, err
	}

	status, err := querier.GetSpotMarginOrder(orderID, symbol)
	if err != nil {
		e.log.Warn("confirmSpotFill: native query %s failed: %v", orderID, err)
		return 0, 0, false, err
	}
	if status == nil {
		err := fmt.Errorf("spot margin order %s not found", orderID)
		e.log.Warn("confirmSpotFill: %v", err)
		return 0, 0, false, err
	}

	e.log.Info("confirmSpotFill: native %s status=%s filled=%.6f avg=%.8f",
		orderID, status.Status, status.FilledQty, status.AvgPrice)

	if isActiveSpotOrderStatus(status.Status) {
		err := fmt.Errorf("spot order %s still active with status %s (filled=%.6f)", orderID, status.Status, status.FilledQty)
		return 0, 0, false, err
	}

	avg := status.AvgPrice
	if avg <= 0 {
		if upd, ok := exch.GetOrderUpdate(orderID); ok && upd.AvgPrice > 0 {
			avg = upd.AvgPrice
		}
	}
	return status.FilledQty, avg, true, nil
}

// ---------------------------------------------------------------------------
// Rollback helpers
// ---------------------------------------------------------------------------

// rollbackBorrow attempts to repay a borrowed amount.
func (e *SpotEngine) rollbackBorrow(smExch exchange.SpotMarginExchange, coin, amount string) {
	e.log.Info("ROLLBACK: repaying borrow %s %s", amount, coin)
	if err := smExch.MarginRepay(exchange.MarginRepayParams{
		Coin:   coin,
		Amount: amount,
	}); err != nil {
		e.log.Error("ROLLBACK: MarginRepay(%s %s) FAILED: %v — manual intervention needed", amount, coin, err)
	} else {
		e.log.Info("ROLLBACK: MarginRepay(%s %s) succeeded", amount, coin)
	}
}

// rollbackSpotOrder attempts to reverse a spot order by placing an opposite market IOC.
func (e *SpotEngine) rollbackSpotOrder(smExch exchange.SpotMarginExchange, symbol string, side exchange.Side, sizeStr string) {
	e.log.Info("ROLLBACK: reversing spot — %s %s %s", side, symbol, sizeStr)
	oid, err := smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
		Symbol:    symbol,
		Side:      side,
		OrderType: "market",
		Size:      sizeStr,
		Force:     "ioc",
	})
	if err != nil {
		e.log.Error("ROLLBACK: spot reverse order FAILED: %v — manual intervention needed", err)
	} else {
		e.log.Info("ROLLBACK: spot reverse order placed: %s", oid)
	}
}

// ---------------------------------------------------------------------------
// Position close (exit execution)
// ---------------------------------------------------------------------------

// retryLeg retries a close leg function up to maxRetries times with the given
// delay between attempts. Returns nil on the first successful attempt, or the
// last error after exhaustion.
func (e *SpotEngine) retryLeg(name string, maxRetries int, delay time.Duration, fn func() error) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := fn(); err != nil {
			lastErr = err
			if nonRetryable, ok := err.(interface{ nonRetryable() bool }); ok && nonRetryable.nonRetryable() {
				return err
			}
			e.log.Warn("retryLeg %s attempt %d/%d failed: %v", name, i+1, maxRetries, err)
			if i < maxRetries-1 {
				time.Sleep(delay)
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("%s failed after %d attempts: %w", name, maxRetries, lastErr)
}

// ClosePosition closes an active spot-futures position by unwinding both legs.
// For Direction A (borrow_sell_long): close futures long → buy back spot → repay borrow.
// For Direction B (buy_spot_short): close futures short → sell spot.
// In emergency mode, both legs are closed in parallel with market IOC orders
// and a 5-second hard timeout.
// The method updates pos.SpotExitPrice and pos.FuturesExit in place on success.
func (e *SpotEngine) ClosePosition(pos *models.SpotFuturesPosition, reason string, isEmergency bool) error {
	e.log.Info("ClosePosition: %s on %s reason=%s emergency=%v", pos.Symbol, pos.Exchange, reason, isEmergency)

	futExch, ok := e.exchanges[pos.Exchange]
	if !ok {
		return fmt.Errorf("exchange %s not found", pos.Exchange)
	}
	smExch, ok := e.spotMargin[pos.Exchange]
	if !ok {
		return fmt.Errorf("exchange %s does not support spot margin", pos.Exchange)
	}

	sizeStr := utils.FormatSize(pos.FuturesSize, 6)
	spotSizeStr := utils.FormatSize(pos.SpotSize, 6)

	if isEmergency {
		return e.emergencyClose(pos, futExch, smExch, sizeStr, spotSizeStr)
	}

	switch pos.Direction {
	case "borrow_sell_long":
		return e.closeDirectionA(pos, futExch, smExch, sizeStr, spotSizeStr)
	case "buy_spot_short":
		return e.closeDirectionB(pos, futExch, smExch, sizeStr, spotSizeStr)
	default:
		return fmt.Errorf("unknown direction %q", pos.Direction)
	}
}

// closeDirectionA closes a borrow-sell-long position:
// 1. Close futures long (sell)
// 2. Buy back spot
// 3. Repay borrow
func (e *SpotEngine) closeDirectionA(
	pos *models.SpotFuturesPosition,
	futExch exchange.Exchange,
	smExch exchange.SpotMarginExchange,
	futSizeStr, spotSizeStr string,
) error {
	// Step 1: Close futures long (sell to close) — with retry
	if pos.FuturesExit > 0 {
		e.log.Info("ClosePosition [Dir A] step 1: futures already closed (exit=%.6f), skipping", pos.FuturesExit)
	} else {
		e.log.Info("ClosePosition [Dir A] step 1: close futures SELL %s %s", pos.Symbol, futSizeStr)
		var futFilled, futAvg float64
		err := e.retryLeg("dirA-futures-close", 3, 2*time.Second, func() error {
			orderID, placeErr := futExch.PlaceOrder(exchange.PlaceOrderParams{
				Symbol:     pos.Symbol,
				Side:       exchange.SideSell,
				OrderType:  "market",
				Size:       futSizeStr,
				Force:      "ioc",
				ReduceOnly: true,
			})
			if placeErr != nil {
				return fmt.Errorf("close futures long failed: %w", placeErr)
			}
			filled, avg := e.confirmFuturesFill(futExch, orderID, pos.Symbol)
			if filled <= 0 {
				return fmt.Errorf("futures close got 0 fill (order %s)", orderID)
			}
			futFilled, futAvg = filled, avg
			return nil
		})
		if err != nil {
			// Exhausted retries — escalate to emergency market close for this leg
			e.log.Error("ClosePosition [Dir A]: futures retry exhausted, escalating to emergency: %v", err)
			return e.emergencyClose(pos, futExch, smExch, futSizeStr, spotSizeStr)
		}
		if futAvg <= 0 && futFilled > 0 {
			if ob, obErr := futExch.GetOrderbook(pos.Symbol, 5); obErr == nil && len(ob.Bids) > 0 && len(ob.Asks) > 0 {
				futAvg = (ob.Bids[0].Price + ob.Asks[0].Price) / 2
				e.log.Warn("ClosePosition [Dir A]: futures avg price 0, using orderbook mid %.6f", futAvg)
			}
		}
		pos.FuturesExit = futAvg
		e.log.Info("ClosePosition [Dir A] step 1: futures closed fill=%.6f avg=%.6f", futFilled, futAvg)
		// Checkpoint: persist FuturesExit immediately after confirmed fill.
		if futAvg > 0 {
			if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
				e.log.Error("ClosePosition [Dir A]: failed to checkpoint futures exit: %v", cpErr)
			}
		}
	}

	// Step 2: Buy back spot (to return borrowed coin) — with retry
	if spotExitComplete(pos) {
		e.log.Info("ClosePosition [Dir A] step 2: spot already closed (exit=%.6f), skipping", pos.SpotExitPrice)
	} else {
		e.log.Info("ClosePosition [Dir A] step 2: buy back spot BUY %s %s", pos.Symbol, utils.FormatSize(remainingSpotExitQty(pos), 6))
		err := e.retryLeg("dirA-spot-buyback", 3, 2*time.Second, func() error {
			orderID := pos.PendingSpotExitOrderID
			if orderID == "" {
				remaining := remainingSpotExitQty(pos)
				if remaining <= 0 {
					pos.SpotExitFilledQty = pos.SpotSize
					pos.SpotExitFilled = true
					return nil
				}
				remainingStr := utils.FormatSize(remaining, 6)
				var placeErr error
				orderID, placeErr = smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
					Symbol:    pos.Symbol,
					Side:      exchange.SideBuy,
					OrderType: "market",
					Size:      remainingStr,
					Force:     "ioc",
				})
				if placeErr != nil {
					return fmt.Errorf("spot buyback failed: %w", placeErr)
				}
				pos.PendingSpotExitOrderID = orderID
				if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
					e.log.Error("ClosePosition [Dir A]: failed to checkpoint pending spot exit order %s: %v", orderID, cpErr)
				}
			} else {
				e.log.Warn("ClosePosition [Dir A]: reconciling existing spot exit order %s", orderID)
			}

			filled, avg, confirmed, confErr := e.confirmSpotFill(smExch, futExch, orderID, pos.Symbol, remainingSpotExitQty(pos))
			if !confirmed {
				return &pendingSpotExitError{orderID: orderID, err: confErr}
			}
			pos.PendingSpotExitOrderID = ""
			if filled <= 0 {
				if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
					e.log.Error("ClosePosition [Dir A]: failed to clear pending spot exit order %s: %v", orderID, cpErr)
				}
				return fmt.Errorf("spot buyback got 0 fill (order %s)", orderID)
			}
			applySpotExitFill(pos, filled, avg)
			if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
				e.log.Error("ClosePosition [Dir A]: failed to checkpoint spot exit fill: %v", cpErr)
			}
			if !spotExitComplete(pos) {
				return fmt.Errorf("spot buyback partially filled %.6f/%.6f (order %s)",
					pos.SpotExitFilledQty, pos.SpotSize, orderID)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("spot buyback failed (futures already closed): %w", err)
		}
		if pos.SpotExitPrice <= 0 && pos.SpotExitFilledQty > 0 {
			if ob, obErr := futExch.GetOrderbook(pos.Symbol, 5); obErr == nil && len(ob.Bids) > 0 && len(ob.Asks) > 0 {
				pos.SpotExitPrice = (ob.Bids[0].Price + ob.Asks[0].Price) / 2
				e.log.Warn("ClosePosition [Dir A]: spot avg price 0, using orderbook mid %.6f", pos.SpotExitPrice)
			}
		}
		e.log.Info("ClosePosition [Dir A] step 2: spot buyback fill=%.6f avg=%.6f", pos.SpotExitFilledQty, pos.SpotExitPrice)
		// Checkpoint: persist confirmed spot leg state immediately after fill.
		if pos.SpotExitFilledQty > 0 || pos.SpotExitPrice > 0 {
			if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
				e.log.Error("ClosePosition [Dir A]: failed to checkpoint spot exit: %v", cpErr)
			}
		}
	}

	// Step 3: Repay borrow
	repayAmount := utils.FormatSize(pos.BorrowAmount, 6)
	e.log.Info("ClosePosition [Dir A] step 3: repay borrow %s %s", repayAmount, pos.BaseCoin)
	if err := smExch.MarginRepay(exchange.MarginRepayParams{
		Coin:   pos.BaseCoin,
		Amount: repayAmount,
	}); err != nil {
		e.log.Error("ClosePosition [Dir A] repay FAILED: %v — will retry on next monitor tick for %s %s",
			err, repayAmount, pos.BaseCoin)
		pos.PendingRepay = true
		var blackout *exchange.ErrRepayBlackout
		if errors.As(err, &blackout) {
			pos.PendingRepayRetryAt = &blackout.RetryAfter
		}
	}

	// Record exit fees using position exit prices.
	takerFeeA := spotFees[pos.Exchange]
	if takerFeeA == 0 {
		takerFeeA = 0.0005
	}
	exitFeesA := 0.0
	if pos.FuturesExit > 0 {
		exitFeesA += pos.FuturesSize * pos.FuturesExit * takerFeeA
	}
	if pos.SpotExitPrice > 0 {
		exitFeesA += pos.SpotSize * pos.SpotExitPrice * takerFeeA
	}
	pos.ExitFees = exitFeesA

	return nil
}

// closeDirectionB closes a buy-spot-short position:
// 1. Close futures short (buy to close)
// 2. Sell spot
func (e *SpotEngine) closeDirectionB(
	pos *models.SpotFuturesPosition,
	futExch exchange.Exchange,
	smExch exchange.SpotMarginExchange,
	futSizeStr, spotSizeStr string,
) error {
	// Step 1: Close futures short (buy to close) — with retry
	if pos.FuturesExit > 0 {
		e.log.Info("ClosePosition [Dir B] step 1: futures already closed (exit=%.6f), skipping", pos.FuturesExit)
	} else {
		e.log.Info("ClosePosition [Dir B] step 1: close futures BUY %s %s", pos.Symbol, futSizeStr)
		var futFilled, futAvg float64
		err := e.retryLeg("dirB-futures-close", 3, 2*time.Second, func() error {
			orderID, placeErr := futExch.PlaceOrder(exchange.PlaceOrderParams{
				Symbol:     pos.Symbol,
				Side:       exchange.SideBuy,
				OrderType:  "market",
				Size:       futSizeStr,
				Force:      "ioc",
				ReduceOnly: true,
			})
			if placeErr != nil {
				return fmt.Errorf("close futures short failed: %w", placeErr)
			}
			filled, avg := e.confirmFuturesFill(futExch, orderID, pos.Symbol)
			if filled <= 0 {
				return fmt.Errorf("futures close got 0 fill (order %s)", orderID)
			}
			futFilled, futAvg = filled, avg
			return nil
		})
		if err != nil {
			// Exhausted retries — escalate to emergency market close for this leg
			e.log.Error("ClosePosition [Dir B]: futures retry exhausted, escalating to emergency: %v", err)
			return e.emergencyClose(pos, futExch, smExch, futSizeStr, spotSizeStr)
		}
		if futAvg <= 0 && futFilled > 0 {
			if ob, obErr := futExch.GetOrderbook(pos.Symbol, 5); obErr == nil && len(ob.Bids) > 0 && len(ob.Asks) > 0 {
				futAvg = (ob.Bids[0].Price + ob.Asks[0].Price) / 2
				e.log.Warn("ClosePosition [Dir B]: futures avg price 0, using orderbook mid %.6f", futAvg)
			}
		}
		pos.FuturesExit = futAvg
		e.log.Info("ClosePosition [Dir B] step 1: futures closed fill=%.6f avg=%.6f", futFilled, futAvg)
		// Checkpoint: persist FuturesExit immediately after confirmed fill.
		if futAvg > 0 {
			if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
				e.log.Error("ClosePosition [Dir B]: failed to checkpoint futures exit: %v", cpErr)
			}
		}
	}

	// Step 2: Sell spot — with retry
	if spotExitComplete(pos) {
		e.log.Info("ClosePosition [Dir B] step 2: spot already closed (exit=%.6f), skipping", pos.SpotExitPrice)
	} else {
		e.log.Info("ClosePosition [Dir B] step 2: sell spot SELL %s %s", pos.Symbol, utils.FormatSize(remainingSpotExitQty(pos), 6))
		err := e.retryLeg("dirB-spot-sell", 3, 2*time.Second, func() error {
			orderID := pos.PendingSpotExitOrderID
			if orderID == "" {
				remaining := remainingSpotExitQty(pos)
				if remaining <= 0 {
					pos.SpotExitFilledQty = pos.SpotSize
					pos.SpotExitFilled = true
					return nil
				}
				remainingStr := utils.FormatSize(remaining, 6)
				var placeErr error
				orderID, placeErr = smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
					Symbol:    pos.Symbol,
					Side:      exchange.SideSell,
					OrderType: "market",
					Size:      remainingStr,
					Force:     "ioc",
				})
				if placeErr != nil {
					return fmt.Errorf("spot sell failed: %w", placeErr)
				}
				pos.PendingSpotExitOrderID = orderID
				if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
					e.log.Error("ClosePosition [Dir B]: failed to checkpoint pending spot exit order %s: %v", orderID, cpErr)
				}
			} else {
				e.log.Warn("ClosePosition [Dir B]: reconciling existing spot exit order %s", orderID)
			}

			filled, avg, confirmed, confErr := e.confirmSpotFill(smExch, futExch, orderID, pos.Symbol, remainingSpotExitQty(pos))
			if !confirmed {
				return &pendingSpotExitError{orderID: orderID, err: confErr}
			}
			pos.PendingSpotExitOrderID = ""
			if filled <= 0 {
				if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
					e.log.Error("ClosePosition [Dir B]: failed to clear pending spot exit order %s: %v", orderID, cpErr)
				}
				return fmt.Errorf("spot sell got 0 fill (order %s)", orderID)
			}
			applySpotExitFill(pos, filled, avg)
			if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
				e.log.Error("ClosePosition [Dir B]: failed to checkpoint spot exit fill: %v", cpErr)
			}
			if !spotExitComplete(pos) {
				return fmt.Errorf("spot sell partially filled %.6f/%.6f (order %s)",
					pos.SpotExitFilledQty, pos.SpotSize, orderID)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("spot sell failed (futures already closed): %w", err)
		}
		if pos.SpotExitPrice <= 0 && pos.SpotExitFilledQty > 0 {
			if ob, obErr := futExch.GetOrderbook(pos.Symbol, 5); obErr == nil && len(ob.Bids) > 0 && len(ob.Asks) > 0 {
				pos.SpotExitPrice = (ob.Bids[0].Price + ob.Asks[0].Price) / 2
				e.log.Warn("ClosePosition [Dir B]: spot avg price 0, using orderbook mid %.6f", pos.SpotExitPrice)
			}
		}
		e.log.Info("ClosePosition [Dir B] step 2: spot sold fill=%.6f avg=%.6f", pos.SpotExitFilledQty, pos.SpotExitPrice)
		// Checkpoint: persist confirmed spot leg state immediately after fill.
		if pos.SpotExitFilledQty > 0 || pos.SpotExitPrice > 0 {
			if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
				e.log.Error("ClosePosition [Dir B]: failed to checkpoint spot exit: %v", cpErr)
			}
		}
	}

	// Record exit fees using position exit prices.
	takerFeeB := spotFees[pos.Exchange]
	if takerFeeB == 0 {
		takerFeeB = 0.0005
	}
	exitFeesB := 0.0
	if pos.FuturesExit > 0 {
		exitFeesB += pos.FuturesSize * pos.FuturesExit * takerFeeB
	}
	if pos.SpotExitPrice > 0 {
		exitFeesB += pos.SpotSize * pos.SpotExitPrice * takerFeeB
	}
	pos.ExitFees = exitFeesB

	return nil
}

// emergencyClose closes both legs in PARALLEL with a 5-second hard timeout.
// Used for emergency exits (price spike >30%, margin >95%).
func (e *SpotEngine) emergencyClose(
	pos *models.SpotFuturesPosition,
	futExch exchange.Exchange,
	smExch exchange.SpotMarginExchange,
	futSizeStr, spotSizeStr string,
) error {
	e.log.Warn("EMERGENCY CLOSE: %s on %s — closing both legs in parallel", pos.Symbol, pos.Exchange)

	type result struct {
		leg string
		avg float64
		err error
	}
	ch := make(chan result, 2)

	// Determine futures close side
	var futSide exchange.Side
	if pos.FuturesSide == "long" {
		futSide = exchange.SideSell
	} else {
		futSide = exchange.SideBuy
	}

	// Determine spot close side
	var spotSide exchange.Side
	if pos.Direction == "borrow_sell_long" {
		spotSide = exchange.SideBuy // buy back borrowed coin
	} else {
		spotSide = exchange.SideSell // sell held spot
	}

	// Close futures leg in parallel
	go func() {
		orderID, err := futExch.PlaceOrder(exchange.PlaceOrderParams{
			Symbol:     pos.Symbol,
			Side:       futSide,
			OrderType:  "market",
			Size:       futSizeStr,
			Force:      "ioc",
			ReduceOnly: true,
		})
		if err != nil {
			ch <- result{leg: "futures", err: fmt.Errorf("futures close: %w", err)}
			return
		}
		_, avg := e.confirmFuturesFill(futExch, orderID, pos.Symbol)
		ch <- result{leg: "futures", avg: avg}
	}()

	// Close spot leg in parallel
	go func() {
		remaining := remainingSpotExitQty(pos)
		if remaining <= 0 {
			pos.SpotExitFilledQty = pos.SpotSize
			pos.SpotExitFilled = true
			ch <- result{leg: "spot", avg: pos.SpotExitPrice}
			return
		}
		orderID, err := smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
			Symbol:    pos.Symbol,
			Side:      spotSide,
			OrderType: "market",
			Size:      utils.FormatSize(remaining, 6),
			Force:     "ioc",
		})
		if err != nil {
			ch <- result{leg: "spot", err: fmt.Errorf("spot close: %w", err)}
			return
		}
		pos.PendingSpotExitOrderID = orderID
		if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
			e.log.Error("EMERGENCY: failed to checkpoint pending spot exit order %s: %v", orderID, cpErr)
		}
		filled, avg, confirmed, confErr := e.confirmSpotFill(smExch, futExch, orderID, pos.Symbol, remaining)
		if !confirmed {
			ch <- result{leg: "spot", err: &pendingSpotExitError{orderID: orderID, err: confErr}}
			return
		}
		pos.PendingSpotExitOrderID = ""
		if filled <= 0 {
			if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
				e.log.Error("EMERGENCY: failed to clear pending spot exit order %s: %v", orderID, cpErr)
			}
			ch <- result{leg: "spot", err: fmt.Errorf("spot close got 0 fill (order %s)", orderID)}
			return
		}
		applySpotExitFill(pos, filled, avg)
		if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
			e.log.Error("EMERGENCY: failed to checkpoint spot exit fill for %s: %v", orderID, cpErr)
		}
		if !spotExitComplete(pos) {
			ch <- result{leg: "spot", err: fmt.Errorf("spot close partially filled %.6f/%.6f (order %s)",
				pos.SpotExitFilledQty, pos.SpotSize, orderID)}
			return
		}
		ch <- result{leg: "spot", avg: pos.SpotExitPrice}
	}()

	// Collect results with 5s timeout
	timeout := time.After(5 * time.Second)
	var futErr, spotErr error
	var futLegClosed, spotLegClosed bool
	collected := 0
	for collected < 2 {
		select {
		case r := <-ch:
			collected++
			if r.err != nil {
				if r.leg == "futures" {
					futErr = r.err
				} else {
					spotErr = r.err
				}
				e.log.Error("EMERGENCY: %s leg failed: %v", r.leg, r.err)
			} else {
				if r.leg == "futures" {
					pos.FuturesExit = r.avg
					futLegClosed = true
				} else {
					pos.SpotExitPrice = r.avg
					spotLegClosed = true
				}
				e.log.Info("EMERGENCY: %s leg closed avg=%.6f", r.leg, r.avg)
			}
		case <-timeout:
			e.log.Error("EMERGENCY CLOSE TIMEOUT: %s — %d/2 legs completed", pos.Symbol, collected)
			if futErr != nil || spotErr != nil {
				return fmt.Errorf("emergency close timed out with errors: futures=%v spot=%v", futErr, spotErr)
			}
			return fmt.Errorf("emergency close timed out after 5s (%d/2 legs done)", collected)
		}
	}

	// Fallback: if a leg closed but avg price is 0, derive mid-price from orderbook
	if futLegClosed && pos.FuturesExit <= 0 {
		if ob, err := futExch.GetOrderbook(pos.Symbol, 5); err == nil && len(ob.Bids) > 0 && len(ob.Asks) > 0 {
			pos.FuturesExit = (ob.Bids[0].Price + ob.Asks[0].Price) / 2
			e.log.Warn("EMERGENCY: futures avg price 0, using orderbook mid %.6f", pos.FuturesExit)
		}
	}
	if spotLegClosed && pos.SpotExitPrice <= 0 {
		if ob, err := futExch.GetOrderbook(pos.Symbol, 5); err == nil && len(ob.Bids) > 0 && len(ob.Asks) > 0 {
			pos.SpotExitPrice = (ob.Bids[0].Price + ob.Asks[0].Price) / 2
			e.log.Warn("EMERGENCY: spot avg price 0, using orderbook mid %.6f", pos.SpotExitPrice)
		}
	}

	// Calculate exit fees from collected avg prices.
	takerFeeEmerg := spotFees[pos.Exchange]
	if takerFeeEmerg == 0 {
		takerFeeEmerg = 0.0005
	}
	pos.ExitFees = 0
	if pos.FuturesExit > 0 {
		pos.ExitFees += pos.FuturesSize * pos.FuturesExit * takerFeeEmerg
	}
	if pos.SpotExitPrice > 0 {
		pos.ExitFees += pos.SpotSize * pos.SpotExitPrice * takerFeeEmerg
	}

	// Repay borrow if Direction A
	if pos.Direction == "borrow_sell_long" && pos.BorrowAmount > 0 {
		repayAmount := utils.FormatSize(pos.BorrowAmount, 6)
		e.log.Info("EMERGENCY: repaying borrow %s %s", repayAmount, pos.BaseCoin)
		if err := smExch.MarginRepay(exchange.MarginRepayParams{
			Coin:   pos.BaseCoin,
			Amount: repayAmount,
		}); err != nil {
			e.log.Error("EMERGENCY: repay FAILED: %v — will retry on next monitor tick", err)
			pos.PendingRepay = true
			var blackout *exchange.ErrRepayBlackout
			if errors.As(err, &blackout) {
				pos.PendingRepayRetryAt = &blackout.RetryAfter
			}
		}
	}

	// Checkpoint: persist whatever legs succeeded plus fees and repay state.
	if pos.FuturesExit > 0 || pos.SpotExitFilledQty > 0 || pos.SpotExitPrice > 0 || pos.SpotExitFilled || pos.PendingSpotExitOrderID != "" {
		if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
			e.log.Error("EMERGENCY: failed to checkpoint exit progress: %v", cpErr)
		}
	}

	if futErr != nil {
		return futErr
	}
	return spotErr
}
