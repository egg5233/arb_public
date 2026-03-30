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

	// 1d. Check no duplicate symbol already open.
	active, err := e.db.GetActiveSpotPositions()
	if err != nil {
		return fmt.Errorf("failed to check active positions: %w", err)
	}
	for _, pos := range active {
		if pos.Symbol == symbol && pos.Exchange == exchName {
			return fmt.Errorf("position for %s on %s already open", symbol, exchName)
		}
	}

	// 1e. Check capacity.
	if len(active) >= e.cfg.SpotFuturesMaxPositions {
		return fmt.Errorf("at max capacity (%d/%d)", len(active), e.cfg.SpotFuturesMaxPositions)
	}

	// 1f. Dry run check.
	if e.cfg.DryRun {
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
	capital := e.cfg.SpotFuturesCapitalPerPosition
	if capital <= 0 {
		capital = 200 // default $200
	}
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
		CurrentBorrowAPR: opp.BorrowAPR,
		NotionalUSDT:     notional,
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
	spotFilled, spotAvg = e.confirmSpotFill(futExch, spotOrderID, symbol)
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
		return 0, 0, 0, 0, 0, fmt.Errorf("futures long failed: %w", err)
	}
	e.log.Info("ManualOpen [borrow_sell_long] step 3: futures order placed: %s", futOrderID)

	// Confirm futures fill.
	futFilled, futAvg = e.confirmFuturesFill(futExch, futOrderID, symbol)
	if futFilled <= 0 {
		e.log.Error("ManualOpen [borrow_sell_long] step 3: futures order got 0 fill — rolling back spot sell")
		e.rollbackSpotOrder(smExch, symbol, exchange.SideBuy, spotFilledStr)
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
	spotFilled, spotAvg = e.confirmSpotFill(futExch, spotOrderID, symbol)
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
func (e *SpotEngine) confirmSpotFill(exch exchange.Exchange, orderID, symbol string) (filledQty, avgPrice float64) {
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
		e.log.Warn("confirmSpotFill: REST query %s failed: %v — assuming full fill of market IOC", orderID, err)
		// For market IOC, assume full fill if the order was accepted.
		// We don't have a REST endpoint for spot margin order details in all adapters.
		return 0, 0
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
