package spotengine

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// ManualOpen executes a spot-futures arbitrage entry for the given symbol,
// exchange, and direction. It runs synchronously (blocking) and returns an
// error if any pre-check or execution step fails.
func (e *SpotEngine) ManualOpen(symbol, exchName, direction string) error {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	exchName = strings.ToLower(strings.TrimSpace(exchName))
	direction = strings.TrimSpace(direction)

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
	notional := size * midPrice
	e.log.Info("ManualOpen: %s size=%.6f (%s) notional=%.2f USDT", symbol, size, sizeStr, notional)

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
		spotEntryPrice, futuresEntryPrice, spotFilledQty, futuresFilledQty, borrowAmount, err = e.executeBorrowSellLong(smExch, futExch, symbol, baseCoin, sizeStr, size)
		futuresSide = "long"
	case "buy_spot_short":
		spotEntryPrice, futuresEntryPrice, spotFilledQty, futuresFilledQty, err = e.executeBuySpotShort(smExch, futExch, symbol, sizeStr, size)
		futuresSide = "short"
	}

	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	// Calculate entry fees (2 legs: spot + futures, taker rate).
	takerFee := spotFees[exchName]
	if takerFee == 0 {
		takerFee = 0.0005 // default 0.05%
	}
	entryFees := (spotFilledQty * spotEntryPrice * takerFee) + (futuresFilledQty * futuresEntryPrice * takerFee)

	// ---------------------------------------------------------------
	// 6. Save position
	// ---------------------------------------------------------------
	now := time.Now().UTC()
	posID := utils.GenerateID("sf-"+symbol, now.UnixMilli())

	pos := &models.SpotFuturesPosition{
		ID:               posID,
		Symbol:           symbol,
		BaseCoin:         baseCoin,
		Exchange:         exchName,
		Direction:        direction,
		Status:           models.SpotStatusActive,
		SpotSize:         spotFilledQty,
		SpotEntryPrice:   spotEntryPrice,
		FuturesSize:      futuresFilledQty,
		FuturesEntry:     futuresEntryPrice,
		FuturesSide:      futuresSide,
		BorrowAmount:     borrowAmount,
		BorrowRateHourly: opp.BorrowAPR / 8760,
		FundingAPR:       opp.FundingAPR,
		FeeAPR:           opp.FeeAPR,
		CurrentBorrowAPR: opp.BorrowAPR,
		NotionalUSDT:     notional,
		EntryFees:        entryFees,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := e.db.SaveSpotPosition(pos); err != nil {
		e.log.Error("ManualOpen: failed to save position: %v", err)
		return fmt.Errorf("position executed but failed to save: %w", err)
	}

	e.api.BroadcastSpotPositionUpdate(pos)
	e.log.Info("ManualOpen: SUCCESS — %s on %s [%s] spot=%.6f@%.6f futures=%.6f@%.6f notional=%.2f",
		symbol, exchName, direction, spotFilledQty, spotEntryPrice, futuresFilledQty, futuresEntryPrice, notional)

	return nil
}

// executeBorrowSellLong handles Direction A: borrow coin, sell spot, long futures.
func (e *SpotEngine) executeBorrowSellLong(
	smExch exchange.SpotMarginExchange,
	futExch exchange.Exchange,
	symbol, baseCoin, sizeStr string,
	size float64,
) (spotAvg, futAvg, spotFilled, futFilled, borrowAmt float64, err error) {

	// Step 1: Borrow
	e.log.Info("ManualOpen [borrow_sell_long] step 1: MarginBorrow %s %s", baseCoin, sizeStr)
	err = smExch.MarginBorrow(exchange.MarginBorrowParams{
		Coin:   baseCoin,
		Amount: sizeStr,
	})
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("MarginBorrow failed: %w", err)
	}
	borrowAmt = size
	e.log.Info("ManualOpen [borrow_sell_long] step 1: borrowed %s %s", sizeStr, baseCoin)

	// Step 2: Sell spot (margin order)
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
	spotFilled, spotAvg = e.confirmSpotFill(futExch, spotOrderID, symbol, size)
	if spotFilled <= 0 {
		e.log.Error("ManualOpen [borrow_sell_long] step 2: spot order got 0 fill — rolling back borrow")
		e.rollbackBorrow(smExch, baseCoin, sizeStr)
		return 0, 0, 0, 0, 0, fmt.Errorf("spot sell got 0 fill (order %s)", spotOrderID)
	}
	e.log.Info("ManualOpen [borrow_sell_long] step 2: spot fill=%.6f avg=%.6f", spotFilled, spotAvg)

	// Step 3: Long futures
	spotFilledStr := utils.FormatSize(spotFilled, 6)
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
) (spotAvg, futAvg, spotFilled, futFilled float64, err error) {

	// Step 1: Buy spot (margin order)
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
	spotFilled, spotAvg = e.confirmSpotFill(futExch, spotOrderID, symbol, size)
	if spotFilled <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("spot buy got 0 fill (order %s)", spotOrderID)
	}
	e.log.Info("ManualOpen [buy_spot_short] step 1: spot fill=%.6f avg=%.6f", spotFilled, spotAvg)

	// Step 2: Short futures
	spotFilledStr := utils.FormatSize(spotFilled, 6)
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

// confirmSpotFill confirms a spot margin order fill. Spot margin orders may
// not appear in the futures WS private stream, so we wait briefly then fall
// back to REST via GetOrderFilledQty. We also try GetOrderUpdate in case
// the exchange routes spot fills through the same WS.
func (e *SpotEngine) confirmSpotFill(exch exchange.Exchange, orderID, symbol string, expectedQty float64) (filledQty, avgPrice float64) {
	// Wait a moment for the order to settle.
	time.Sleep(2 * time.Second)

	// Try WS first (some exchanges route spot order updates here).
	if upd, ok := exch.GetOrderUpdate(orderID); ok {
		if upd.FilledVolume > 0 {
			e.log.Info("confirmSpotFill: WS %s: filled=%.6f avg=%.8f", orderID, upd.FilledVolume, upd.AvgPrice)
			return upd.FilledVolume, upd.AvgPrice
		}
	}

	// REST fallback — GetOrderFilledQty works for futures orders; for spot
	// margin orders it may or may not work depending on the exchange adapter.
	// This is best-effort for Phase 3a.
	restFilled, err := exch.GetOrderFilledQty(orderID, symbol)
	if err != nil {
		e.log.Warn("confirmSpotFill: REST query %s failed: %v — assuming full fill of market IOC (qty=%.6f)", orderID, err, expectedQty)
		// For market IOC, assume full fill if the order was accepted.
		// We don't have a REST endpoint for spot margin order details in all adapters.
		return expectedQty, 0
	}
	e.log.Info("confirmSpotFill: REST %s filled=%.6f", orderID, restFilled)

	// Try WS for avg price.
	if upd, ok := exch.GetOrderUpdate(orderID); ok && upd.AvgPrice > 0 {
		return restFilled, upd.AvgPrice
	}
	return restFilled, 0
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
	if pos.SpotExitPrice > 0 {
		e.log.Info("ClosePosition [Dir A] step 2: spot already closed (exit=%.6f), skipping", pos.SpotExitPrice)
	} else {
		e.log.Info("ClosePosition [Dir A] step 2: buy back spot BUY %s %s", pos.Symbol, spotSizeStr)
		var spotFilled, spotAvg float64
		err := e.retryLeg("dirA-spot-buyback", 3, 2*time.Second, func() error {
			orderID, placeErr := smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
				Symbol:    pos.Symbol,
				Side:      exchange.SideBuy,
				OrderType: "market",
				Size:      spotSizeStr,
				Force:     "ioc",
			})
			if placeErr != nil {
				return fmt.Errorf("spot buyback failed: %w", placeErr)
			}
			filled, avg := e.confirmSpotFill(futExch, orderID, pos.Symbol, pos.SpotSize)
			if filled <= 0 {
				return fmt.Errorf("spot buyback got 0 fill (order %s)", orderID)
			}
			spotFilled, spotAvg = filled, avg
			return nil
		})
		if err != nil {
			return fmt.Errorf("spot buyback failed (futures already closed): %w", err)
		}
		if spotAvg <= 0 && spotFilled > 0 {
			if ob, obErr := futExch.GetOrderbook(pos.Symbol, 5); obErr == nil && len(ob.Bids) > 0 && len(ob.Asks) > 0 {
				spotAvg = (ob.Bids[0].Price + ob.Asks[0].Price) / 2
				e.log.Warn("ClosePosition [Dir A]: spot avg price 0, using orderbook mid %.6f", spotAvg)
			}
		}
		if spotAvg > 0 {
			pos.SpotExitPrice = spotAvg
		}
		e.log.Info("ClosePosition [Dir A] step 2: spot buyback fill=%.6f avg=%.6f", spotFilled, spotAvg)
		// Checkpoint: persist SpotExitPrice immediately after confirmed fill.
		if spotAvg > 0 {
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
	if pos.SpotExitPrice > 0 {
		e.log.Info("ClosePosition [Dir B] step 2: spot already closed (exit=%.6f), skipping", pos.SpotExitPrice)
	} else {
		e.log.Info("ClosePosition [Dir B] step 2: sell spot SELL %s %s", pos.Symbol, spotSizeStr)
		var spotFilled, spotAvg float64
		err := e.retryLeg("dirB-spot-sell", 3, 2*time.Second, func() error {
			orderID, placeErr := smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
				Symbol:    pos.Symbol,
				Side:      exchange.SideSell,
				OrderType: "market",
				Size:      spotSizeStr,
				Force:     "ioc",
			})
			if placeErr != nil {
				return fmt.Errorf("spot sell failed: %w", placeErr)
			}
			filled, avg := e.confirmSpotFill(futExch, orderID, pos.Symbol, pos.SpotSize)
			if filled <= 0 {
				return fmt.Errorf("spot sell got 0 fill (order %s)", orderID)
			}
			spotFilled, spotAvg = filled, avg
			return nil
		})
		if err != nil {
			return fmt.Errorf("spot sell failed (futures already closed): %w", err)
		}
		if spotAvg <= 0 && spotFilled > 0 {
			if ob, obErr := futExch.GetOrderbook(pos.Symbol, 5); obErr == nil && len(ob.Bids) > 0 && len(ob.Asks) > 0 {
				spotAvg = (ob.Bids[0].Price + ob.Asks[0].Price) / 2
				e.log.Warn("ClosePosition [Dir B]: spot avg price 0, using orderbook mid %.6f", spotAvg)
			}
		}
		if spotAvg > 0 {
			pos.SpotExitPrice = spotAvg
		}
		e.log.Info("ClosePosition [Dir B] step 2: spot sold fill=%.6f avg=%.6f", spotFilled, spotAvg)
		// Checkpoint: persist SpotExitPrice immediately after confirmed fill.
		if spotAvg > 0 {
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
		orderID, err := smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
			Symbol:    pos.Symbol,
			Side:      spotSide,
			OrderType: "market",
			Size:      spotSizeStr,
			Force:     "ioc",
		})
		if err != nil {
			ch <- result{leg: "spot", err: fmt.Errorf("spot close: %w", err)}
			return
		}
		_, avg := e.confirmSpotFill(futExch, orderID, pos.Symbol, pos.SpotSize)
		ch <- result{leg: "spot", avg: avg}
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
		}
	}

	// Checkpoint: persist whatever legs succeeded plus fees and repay state.
	if pos.FuturesExit > 0 || pos.SpotExitPrice > 0 {
		if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
			e.log.Error("EMERGENCY: failed to checkpoint exit progress: %v", cpErr)
		}
	}

	if futErr != nil {
		return futErr
	}
	return spotErr
}
