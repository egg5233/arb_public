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

// needsMarginTransfer returns true for exchanges with separate margin accounts
// that require explicit fund transfers between futures and margin accounts.
// Binance and Bitget have separate cross-margin accounts; Bybit UTA, OKX, and
// Gate.io have unified accounts where funds are shared.
func needsMarginTransfer(exchName string) bool {
	switch exchName {
	case "binance", "bitget":
		return true
	default:
		return false
	}
}

// futuresStepSize looks up the futures contract step size for symbol on the
// given exchange. Returns 0 if contracts are unavailable (caller should skip
// rounding).
func (e *SpotEngine) futuresStepSize(futExch exchange.Exchange, symbol string) (stepSize float64, minSize float64, decimals int) {
	contracts, err := futExch.LoadAllContracts()
	if err != nil {
		return 0, 0, 6
	}
	ci, ok := contracts[symbol]
	if !ok {
		return 0, 0, 6
	}
	return ci.StepSize, ci.MinSize, ci.SizeDecimals
}

// roundToFuturesStep rounds qty down to futures step size. Returns the rounded
// quantity and its string representation. If step is 0, uses 6 decimal places.
func roundToFuturesStep(qty, stepSize float64, decimals int) (float64, string) {
	if stepSize > 0 {
		qty = utils.RoundToStep(qty, stepSize)
	}
	return qty, utils.FormatSize(qty, decimals)
}

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

	// 1d. Maintenance rate warning for manual opens (per D-04: allow bypass with warning).
	if e.cfg.SpotFuturesEnableMaintenanceGate {
		leverage := float64(e.cfg.SpotFuturesLeverage)
		if leverage <= 0 {
			leverage = 3.0
		}
		capitalPerLeg := e.cfg.SpotFuturesCapitalSeparate
		if !isSeparateAccount(exchName) {
			capitalPerLeg = e.cfg.SpotFuturesCapitalUnified
		}
		notional := capitalPerLeg * leverage
		mr := e.getMaintenanceRate(symbol, exchName, notional)
		survivable := (1.0 / leverage) - mr
		threshold := 0.90 / leverage
		if survivable < threshold {
			e.log.Warn("manual-open: %s on %s maintenance_rate=%.1f%%, survivable=%.1f%% < threshold=%.1f%% — proceeding per manual override",
				symbol, exchName, mr*100, survivable*100, threshold*100)
		}
	}

	// 1e. Check no duplicate symbol already open (any exchange).
	active, err := e.db.GetActiveSpotPositions()
	if err != nil {
		return fmt.Errorf("failed to check active positions: %w", err)
	}
	for _, pos := range active {
		if pos.Symbol == symbol {
			return fmt.Errorf("position for %s already open on %s", symbol, pos.Exchange)
		}
	}

	// 1f. Check capacity.
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

	// For separate-account exchanges (Binance, Bitget), transfer USDT to the
	// correct wallet BEFORE sizing. Dir A needs margin (for auto-borrow sell),
	// Dir B needs spot (plain buy). Without this, MaxBorrowable is near-zero
	// (Dir A) or spot has no USDT (Dir B).
	if needsMarginTransfer(exchName) {
		transferAmt := fmt.Sprintf("%.2f", capital*1.05) // 5% buffer for fees/slippage
		if direction == "borrow_sell_long" {
			// Dir A: USDT → margin account (for auto-borrow sell)
			e.log.Info("ManualOpen: transferring %s USDT to margin (%s separate margin account)", transferAmt, exchName)
			if tErr := smExch.TransferToMargin("USDT", transferAmt); tErr != nil {
				e.log.Warn("ManualOpen: TransferToMargin(%s USDT): %v (may already have funds)", transferAmt, tErr)
			}
		} else {
			// Dir B: USDT → spot account (plain spot buy, not margin)
			e.log.Info("ManualOpen: transferring %s USDT to spot (%s separate accounts)", transferAmt, exchName)
			if tErr := futExch.TransferToSpot("USDT", transferAmt); tErr != nil {
				e.log.Warn("ManualOpen: TransferToSpot(%s USDT): %v (may already have funds)", transferAmt, tErr)
			}
		}
	}

	// For Direction A, cap by max borrowable.
	// On separate-account exchanges, poll until the transferred collateral
	// is reflected in MaxBorrowable (transfer settlement can lag).
	if direction == "borrow_sell_long" {
		var maxBorrow float64
		if needsMarginTransfer(exchName) {
			minExpected := rawSize * 0.5 // expect at least half the target to be borrowable
			for poll := 0; poll < 10; poll++ {
				mb, err := smExch.GetMarginBalance(baseCoin)
				if err != nil {
					e.log.Warn("ManualOpen: GetMarginBalance(%s) poll %d failed: %v", baseCoin, poll+1, err)
					break
				}
				maxBorrow = mb.MaxBorrowable
				if maxBorrow >= minExpected {
					break
				}
				e.log.Info("ManualOpen: MaxBorrowable %.6f < expected %.6f — waiting for collateral settlement (%d/10)", maxBorrow, minExpected, poll+1)
				time.Sleep(500 * time.Millisecond)
			}
		} else {
			mb, err := smExch.GetMarginBalance(baseCoin)
			if err != nil {
				e.log.Warn("ManualOpen: GetMarginBalance(%s) failed: %v — proceeding with computed size", baseCoin, err)
			} else {
				maxBorrow = mb.MaxBorrowable
			}
		}
		if maxBorrow > 0 && rawSize > maxBorrow {
			e.log.Info("ManualOpen: capping size from %.6f to MaxBorrowable %.6f", rawSize, maxBorrow)
			rawSize = maxBorrow
		}
	}

	// Round size to futures contract step size so both legs match exactly.
	futStep, futMin, futDec := e.futuresStepSize(futExch, symbol)
	var size float64
	if futStep > 0 {
		size = utils.RoundToStep(rawSize, futStep)
	} else {
		size = math.Floor(rawSize*1e6) / 1e6
	}
	if size <= 0 || (futMin > 0 && size < futMin) {
		e.log.Warn("ManualOpen: computed size %.6f (raw=%.6f step=%.6f min=%.6f) too small for %s", size, rawSize, futStep, futMin, symbol)
		return fmt.Errorf("computed size %.6f too small for %s (step=%.6f min=%.6f)", size, symbol, futStep, futMin)
	}
	sizeStr := utils.FormatSize(size, futDec)
	plannedNotional := size * midPrice
	// Reject if borrowable amount is less than 10% of the target capital —
	// too small to be worth trading and likely to hit exchange minimums.
	minNotional := capital * 0.10
	if minNotional < 5.0 {
		minNotional = 5.0
	}
	if plannedNotional < minNotional {
		e.log.Warn("ManualOpen: notional %.2f USDT too small (min %.0f USDT) for %s — MaxBorrowable may be too low (size=%.6f price=%.6f)", plannedNotional, minNotional, symbol, size, midPrice)
		return fmt.Errorf("notional %.2f USDT too small (min %.0f USDT) for %s (size=%.6f price=%.6f)", plannedNotional, minNotional, symbol, size, midPrice)
	}
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
		FeePct:           opp.FeePct,
		CurrentBorrowAPR: opp.BorrowAPR,
		NotionalUSDT:     plannedNotional,
		HedgeIntact:      true,
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
	if err := e.persistPendingEntry(entryPos, ""); err != nil {
		e.releaseSpotReservation(reservation)
		return fmt.Errorf("failed to persist pending entry before execution: %w", err)
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

	// (TransferToMargin moved to step 3, before MaxBorrowable check)

	// ---------------------------------------------------------------
	// 5. Execute based on direction
	// ---------------------------------------------------------------
	var spotEntryPrice, futuresEntryPrice float64
	var spotFilledQty, futuresFilledQty float64
	var futuresSide string
	var borrowAmount float64

	switch direction {
	case "borrow_sell_long":
		spotEntryPrice, futuresEntryPrice, spotFilledQty, futuresFilledQty, borrowAmount, err = e.executeBorrowSellLong(smExch, futExch, symbol, baseCoin, sizeStr, size, futStep, futMin, futDec, requireEntryLock, entryPos)
		futuresSide = "long"
	case "buy_spot_short":
		spotEntryPrice, futuresEntryPrice, spotFilledQty, futuresFilledQty, err = e.executeBuySpotShort(smExch, futExch, symbol, sizeStr, size, plannedNotional, futStep, futMin, futDec, requireEntryLock, entryPos)
		futuresSide = "short"
	}

	if err != nil {
		var pendingErr *pendingSpotEntryError
		if errors.As(err, &pendingErr) {
			recoverySaveFailed := false
			if pendingErr.pendingPos != nil {
				if saveErr := e.db.SaveSpotPosition(pendingErr.pendingPos); saveErr != nil {
					e.log.Error("ManualOpen: failed to save pending spot recovery %s: %v", pendingErr.posID, saveErr)
					recoverySaveFailed = true
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
			if recoverySaveFailed {
				return fmt.Errorf("%w (manual recovery position %s could not be persisted; existing pending record retained)", err, pendingErr.posID)
			}
			e.log.Error("ManualOpen: %s on %s — pending entry error: %v", symbol, exchName, err)
			return err
		}
		e.abandonPendingEntry(entryPos, "entry_failed")
		e.releaseSpotReservation(reservation)
		e.log.Error("ManualOpen: %s on %s — execution failed: %v", symbol, exchName, err)
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
	if err := e.persistPendingCheckpoint(pos); err != nil {
		if commitErr := e.commitSpotCapital(reservation, posID, actualNotional); commitErr != nil {
			e.log.Error("ManualOpen: capital commit failed after pending checkpoint save error: %v", commitErr)
		}
		e.log.Error("ManualOpen: failed to checkpoint executed position: %v", err)
		return fmt.Errorf("position executed but failed to checkpoint pending state: %w", err)
	}
	if err := e.commitSpotCapital(reservation, posID, actualNotional); err != nil {
		e.log.Error("ManualOpen: capital commit failed: %v", err)
	}

	pos.Status = models.SpotStatusActive
	if err := e.db.SaveSpotPosition(pos); err != nil {
		e.log.Error("ManualOpen: failed to save position: %v", err)
		return fmt.Errorf("position executed and pending checkpoint saved, but failed to promote active: %w", err)
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
	futStep, futMin float64,
	futDec int,
	requireEntryLock func(string) error,
	pos *models.SpotFuturesPosition,
) (spotAvg, futAvg, spotFilled, futFilled, borrowAmt float64, err error) {

	// Step 1: Margin sell with auto-borrow (single API call handles borrow + sell).
	// This lets the exchange manage the borrow internally, avoiding separate
	// borrow API calls, integer precision issues, and risk limit rejections.
	if err := requireEntryLock("margin sell (auto-borrow)"); err != nil {
		return 0, 0, 0, 0, 0, err
	}
	e.log.Info("ManualOpen [borrow_sell_long] step 1: PlaceSpotMarginOrder SELL %s %s (auto-borrow)", symbol, sizeStr)
	spotOrderID, err := smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
		Symbol:     symbol,
		Side:       exchange.SideSell,
		OrderType:  "market",
		Size:       sizeStr,
		AutoBorrow: true,
	})
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("margin sell (auto-borrow) failed: %w", err)
	}
	e.log.Info("ManualOpen [borrow_sell_long] step 1: spot order placed: %s", spotOrderID)
	if cpErr := e.persistPendingEntry(pos, spotOrderID); cpErr != nil {
		return 0, 0, 0, 0, 0, e.abortAcceptedSpotEntry(smExch, futExch, pos, spotOrderID, symbol, size, cpErr)
	}

	// Confirm spot fill.
	spotFilled, spotAvg, confirmed, confErr := e.confirmSpotFill(smExch, futExch, spotOrderID, symbol, size)
	if !confirmed {
		return 0, 0, 0, 0, 0, &pendingSpotEntryError{posID: pos.ID, orderID: spotOrderID, err: confErr}
	}
	if spotFilled <= 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("spot sell got 0 fill (order %s)", spotOrderID)
	}
	borrowAmt = spotFilled // auto-borrow borrows exactly what was sold
	pos.BorrowAmount = borrowAmt
	e.log.Info("ManualOpen [borrow_sell_long] step 1: spot fill=%.6f avg=%.6f (borrowed=%.6f)", spotFilled, spotAvg, borrowAmt)

	// Step 2: Long futures — re-round spot fill to futures step (handles partial fills).
	futQty, futQtyStr := roundToFuturesStep(spotFilled, futStep, futDec)
	spotFilledStr := utils.FormatSize(spotFilled, 6)
	buybackQuote := utils.FormatSize(spotFilled*spotAvg*1.02, 2) // 2% slippage buffer for rollback
	if futQty <= 0 || (futMin > 0 && futQty < futMin) {
		e.log.Error("ManualOpen [borrow_sell_long] step 2: spot fill %.6f rounds to futures qty %.6f (step=%.6f min=%.6f) — too small, rolling back", spotFilled, futQty, futStep, futMin)
		e.rollbackBorrowSell(smExch, symbol, baseCoin, spotFilledStr, buybackQuote, spotFilled)
		return 0, 0, 0, 0, 0, fmt.Errorf("spot fill %.6f too small for futures (step=%.6f min=%.6f)", spotFilled, futStep, futMin)
	}
	if err := requireEntryLock("futures long"); err != nil {
		e.rollbackBorrowSell(smExch, symbol, baseCoin, spotFilledStr, buybackQuote, spotFilled)
		return 0, 0, 0, 0, 0, err
	}
	e.log.Info("ManualOpen [borrow_sell_long] step 2: PlaceOrder futures BUY %s %s (spot=%.6f step=%.6f)", symbol, futQtyStr, spotFilled, futStep)
	futOrderID, err := futExch.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:    symbol,
		Side:      exchange.SideBuy,
		OrderType: "market",
		Size:      futQtyStr,
		Force:     "ioc",
	})
	if err != nil {
		// Rollback: buy back spot + explicitly repay borrow (auto-repay unreliable on some exchanges).
		e.log.Error("ManualOpen [borrow_sell_long] step 2 FAILED: %v — rolling back spot sell", err)
		e.rollbackBorrowSell(smExch, symbol, baseCoin, spotFilledStr, buybackQuote, spotFilled)
		return 0, 0, 0, 0, 0, fmt.Errorf("futures long failed: %w", err)
	}
	e.log.Info("ManualOpen [borrow_sell_long] step 2: futures order placed: %s", futOrderID)

	// Confirm futures fill.
	futFilled, futAvg = e.confirmFuturesFill(futExch, futOrderID, symbol)
	if futFilled <= 0 {
		e.log.Error("ManualOpen [borrow_sell_long] step 2: futures order got 0 fill — rolling back spot sell")
		e.rollbackSpotOrder(smExch, symbol, exchange.SideBuy, spotFilledStr, buybackQuote, true)
		return 0, 0, 0, 0, 0, fmt.Errorf("futures long got 0 fill (order %s)", futOrderID)
	}
	e.log.Info("ManualOpen [borrow_sell_long] step 2: futures fill=%.6f avg=%.6f", futFilled, futAvg)
	pos.PendingEntryOrderID = ""
	pos.SpotSize = spotFilled
	pos.SpotEntryPrice = spotAvg
	pos.FuturesSize = futFilled
	pos.FuturesEntry = futAvg
	pos.FuturesSide = "long"
	pos.NotionalUSDT = spotFilled * spotAvg

	return spotAvg, futAvg, spotFilled, futFilled, borrowAmt, nil
}

// executeBuySpotShort handles Direction B: buy spot, short futures.
func (e *SpotEngine) executeBuySpotShort(
	smExch exchange.SpotMarginExchange,
	futExch exchange.Exchange,
	symbol, sizeStr string,
	size, notionalUSDT float64,
	futStep, futMin float64,
	futDec int,
	requireEntryLock func(string) error,
	pos *models.SpotFuturesPosition,
) (spotAvg, futAvg, spotFilled, futFilled float64, err error) {

	// Step 1: Buy spot (margin order)
	if err := requireEntryLock("spot buy"); err != nil {
		return 0, 0, 0, 0, err
	}
	// Pass both Size (base qty) and QuoteSize (USDT amount) for the market buy.
	// Adapters that support base-qty market BUY (Bybit: marketUnit=baseCoin) use Size;
	// adapters that only accept quote-qty (Gate.io, OKX, Binance, Bitget) use QuoteSize.
	// Futures leg is sized from actual spot fill via roundToFuturesStep, so alignment is safe.
	quoteSizeStr := fmt.Sprintf("%.2f", notionalUSDT)
	e.log.Info("ManualOpen [buy_spot_short] step 1: PlaceSpotMarginOrder BUY %s size=%s quote=%s", symbol, sizeStr, quoteSizeStr)
	spotOrderID, err := smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
		Symbol:    symbol,
		Side:      exchange.SideBuy,
		OrderType: "market",
		Size:      sizeStr,
		QuoteSize: quoteSizeStr,
	})
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("spot buy failed: %w", err)
	}
	e.log.Info("ManualOpen [buy_spot_short] step 1: spot order placed: %s", spotOrderID)
	if cpErr := e.persistPendingEntry(pos, spotOrderID); cpErr != nil {
		return 0, 0, 0, 0, e.abortAcceptedSpotEntry(smExch, futExch, pos, spotOrderID, symbol, size, cpErr)
	}

	// Confirm spot fill.
	spotFilled, spotAvg, confirmed, confErr := e.confirmSpotFill(smExch, futExch, spotOrderID, symbol, size)
	if !confirmed {
		return 0, 0, 0, 0, &pendingSpotEntryError{posID: pos.ID, orderID: spotOrderID, err: confErr}
	}
	if spotFilled <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("spot buy got 0 fill (order %s)", spotOrderID)
	}
	e.log.Info("ManualOpen [buy_spot_short] step 1: spot fill=%.6f avg=%.6f", spotFilled, spotAvg)

	// Query actual received amount: exchange deducts fee from received coin on BUY.
	// Track the net for SpotSize (what we can sell on exit) but use gross for futures
	// leg sizing — the fee is negligible and doesn't break delta neutrality.
	spotNetReceived := spotFilled
	if querier, ok := smExch.(exchange.SpotMarginOrderQuerier); ok {
		if feeStatus, qErr := querier.GetSpotMarginOrder(spotOrderID, symbol); qErr == nil && feeStatus != nil && feeStatus.FeeDeducted > 0 {
			spotNetReceived = spotFilled - feeStatus.FeeDeducted
			// Floor to the spot LOT_SIZE step so the exit sell passes LOT_SIZE.
			// Hardcoding 5dp (1e-5) previously caused DEGOUSDT (step=0.01) to
			// store 1258.1406, which Binance later rejected on SELL with -1013.
			qtyStep := 1e-5
			if rules, rErr := smExch.SpotOrderRules(symbol); rErr == nil && rules != nil && rules.QtyStep > 0 {
				qtyStep = rules.QtyStep
			}
			spotNetReceived = utils.RoundToStep(spotNetReceived, qtyStep)
			e.log.Info("ManualOpen [buy_spot_short] step 1: fee deducted=%.8f net received=%.6f (gross=%.6f, step=%g)", feeStatus.FeeDeducted, spotNetReceived, spotFilled, qtyStep)
		}
	}

	// Step 2: Short futures — use gross fill (not net) for futures leg sizing.
	futQty, futQtyStr := roundToFuturesStep(spotFilled, futStep, futDec)
	spotFilledStr := utils.FormatSize(spotFilled, 6)
	if futQty <= 0 || (futMin > 0 && futQty < futMin) {
		e.log.Error("ManualOpen [buy_spot_short] step 2: spot fill %.6f rounds to futures qty %.6f (step=%.6f min=%.6f) — too small, rolling back", spotFilled, futQty, futStep, futMin)
		e.rollbackSpotOrder(smExch, symbol, exchange.SideSell, spotFilledStr, "", false)
		return 0, 0, 0, 0, fmt.Errorf("spot fill %.6f too small for futures (step=%.6f min=%.6f)", spotFilled, futStep, futMin)
	}
	if err := requireEntryLock("futures short"); err != nil {
		e.rollbackSpotOrder(smExch, symbol, exchange.SideSell, spotFilledStr, "", false)
		return 0, 0, 0, 0, err
	}
	e.log.Info("ManualOpen [buy_spot_short] step 2: PlaceOrder futures SELL %s %s (spot=%.6f step=%.6f)", symbol, futQtyStr, spotFilled, futStep)
	futOrderID, err := futExch.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:    symbol,
		Side:      exchange.SideSell,
		OrderType: "market",
		Size:      futQtyStr,
		Force:     "ioc",
	})
	if err != nil {
		// Rollback: sell the spot back.
		e.log.Error("ManualOpen [buy_spot_short] step 2 FAILED: %v — rolling back spot buy", err)
		e.rollbackSpotOrder(smExch, symbol, exchange.SideSell, spotFilledStr, "", false)
		return 0, 0, 0, 0, fmt.Errorf("futures short failed: %w", err)
	}
	e.log.Info("ManualOpen [buy_spot_short] step 2: futures order placed: %s", futOrderID)

	// Confirm futures fill.
	futFilled, futAvg = e.confirmFuturesFill(futExch, futOrderID, symbol)
	if futFilled <= 0 {
		e.log.Error("ManualOpen [buy_spot_short] step 2: futures order got 0 fill — rolling back spot buy")
		e.rollbackSpotOrder(smExch, symbol, exchange.SideSell, spotFilledStr, "", false)
		return 0, 0, 0, 0, fmt.Errorf("futures short got 0 fill (order %s)", futOrderID)
	}
	e.log.Info("ManualOpen [buy_spot_short] step 2: futures fill=%.6f avg=%.6f", futFilled, futAvg)
	pos.PendingEntryOrderID = ""
	pos.SpotSize = spotNetReceived // Net after fee — what we can actually sell on exit
	pos.SpotEntryPrice = spotAvg
	pos.FuturesSize = futFilled
	pos.FuturesEntry = futAvg
	pos.FuturesSide = "short"
	pos.NotionalUSDT = spotFilled * spotAvg // Use gross for notional tracking

	return spotAvg, futAvg, spotNetReceived, futFilled, nil
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
		var reverseQuote string
		if reverseSide == exchange.SideBuy && recoveryAvg > 0 {
			reverseQuote = utils.FormatSize(filledQty*recoveryAvg*1.02, 2)
		}
		reverseOrderID, err := smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
			Symbol:    symbol,
			Side:      reverseSide,
			OrderType: "market",
			Size:      reverseQty,
			QuoteSize: reverseQuote,
			AutoRepay: pos.Direction == "borrow_sell_long",
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

	// With auto-borrow/auto-repay, the exchange handles repayment during
	// the buyback order above — no separate MarginRepay call needed.

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
	pos.PendingEntryOrderID = orderID
	return e.persistPendingCheckpoint(pos)
}

func (e *SpotEngine) persistPendingCheckpoint(pos *models.SpotFuturesPosition) error {
	pos.Status = models.SpotStatusPending
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
			// With auto-borrow, the exchange only borrows on actual fill.
			// Check if there's actually outstanding debt before attempting repay.
			if pos.Direction == "borrow_sell_long" {
				if mb, mErr := smExch.GetMarginBalance(pos.BaseCoin); mErr == nil && (mb.Borrowed+mb.Interest) > 0 {
					e.rollbackBorrow(smExch, pos.BaseCoin, utils.FormatSize(mb.Borrowed+mb.Interest, 6))
				}
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

	if pos.PendingEntryOrderID == "" && pos.SpotEntryPrice <= 0 && pos.FuturesSize <= 0 {
		e.log.Warn("pending entry %s: awaiting durable spot-order checkpoint before recovery", pos.ID)
		return
	}

	if pos.SpotSize <= 0 {
		e.log.Warn("pending entry %s: no confirmed spot size yet", pos.ID)
		return
	}

	futuresSide := "short"
	switch pos.Direction {
	case "borrow_sell_long":
		futuresSide = "long"
	case "buy_spot_short":
		futuresSide = "short"
	}
	if existingSize, existingAvg, ok, err := pendingEntryFuturesPosition(futExch, pos.Symbol, futuresSide); err != nil {
		e.log.Warn("pending entry %s: existing futures hedge check failed: %v", pos.ID, err)
	} else if ok {
		pos.FuturesSize = existingSize
		pos.FuturesEntry = existingAvg
		pos.FuturesSide = futuresSide
		pos.Status = models.SpotStatusActive
		takerFee := spotFees[pos.Exchange]
		if takerFee == 0 {
			takerFee = 0.0005
		}
		pos.EntryFees = (pos.SpotSize * pos.SpotEntryPrice * takerFee) + (existingSize * existingAvg * takerFee)
		pos.UpdatedAt = time.Now().UTC()
		if err := e.db.SaveSpotPosition(pos); err != nil {
			e.log.Error("pending entry %s: failed to promote recovered futures hedge: %v", pos.ID, err)
			return
		}
		if e.api != nil {
			e.api.BroadcastSpotPositionUpdate(pos)
		}
		e.log.Info("pending entry %s recovered existing %s futures hedge on %s", pos.ID, futuresSide, pos.Exchange)
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

func pendingEntryFuturesPosition(exch exchange.Exchange, symbol, side string) (filledQty, avgPrice float64, ok bool, err error) {
	positions, err := exch.GetPosition(symbol)
	if err != nil {
		return 0, 0, false, err
	}
	for _, p := range positions {
		if p.HoldSide != side {
			continue
		}
		qty, qtyErr := utils.ParseFloat(p.Total)
		if qtyErr != nil || math.Abs(qty) <= spotQtyTolerance {
			continue
		}
		avg, _ := utils.ParseFloat(p.AverageOpenPrice)
		return qty, avg, true, nil
	}
	return 0, 0, false, nil
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

// rollbackSpotOrder attempts to reverse a spot order by placing an opposite market order.
// quoteSizeStr is the USDT amount for market BUY orders (required by some exchanges like Bitget).
func (e *SpotEngine) rollbackSpotOrder(smExch exchange.SpotMarginExchange, symbol string, side exchange.Side, sizeStr, quoteSizeStr string, autoRepay bool) {
	e.log.Info("ROLLBACK: reversing spot — %s %s %s (quote=%s autoRepay=%v)", side, symbol, sizeStr, quoteSizeStr, autoRepay)
	oid, err := smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
		Symbol:    symbol,
		Side:      side,
		OrderType: "market",
		Size:      sizeStr,
		QuoteSize: quoteSizeStr,
		AutoRepay: autoRepay,
	})
	if err != nil {
		e.log.Error("ROLLBACK: spot reverse order FAILED: %v — manual intervention needed", err)
	} else {
		e.log.Info("ROLLBACK: spot reverse order placed: %s", oid)
	}
}

// rollbackBorrowSell reverses a Dir A (borrow-sell-long) spot order: buys back
// the asset and explicitly repays the margin borrow. AutoRepay is unreliable on
// some exchanges (e.g., Bybit UTA does not auto-repay on buy).
func (e *SpotEngine) rollbackBorrowSell(smExch exchange.SpotMarginExchange, symbol, baseCoin, sizeStr, quoteSizeStr string, borrowAmt float64) {
	// Step 1: buy back spot.
	e.rollbackSpotOrder(smExch, symbol, exchange.SideBuy, sizeStr, quoteSizeStr, true)

	// Step 2: explicitly repay borrow (auto-repay unreliable).
	time.Sleep(2 * time.Second) // let settlement propagate
	repayStr := utils.FormatSize(borrowAmt, 8)
	e.log.Info("ROLLBACK: explicit MarginRepay(%s %s) after buyback", repayStr, baseCoin)
	if err := smExch.MarginRepay(exchange.MarginRepayParams{Coin: baseCoin, Amount: repayStr}); err != nil {
		e.log.Error("ROLLBACK: MarginRepay(%s %s) FAILED: %v — check manually", repayStr, baseCoin, err)
	} else {
		e.log.Info("ROLLBACK: MarginRepay(%s %s) succeeded — borrow cleared", repayStr, baseCoin)
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

// isSpotResidualDust reports whether remaining spot qty is below the exchange's
// minimum tradeable size after flooring to QtyStep. Returns (isDust, effectiveQty).
// On dust, callers mark the spot leg closed instead of retrying a doomed order.
func isSpotResidualDust(rules *exchange.SpotOrderRules, remaining, priceHint float64) (bool, float64) {
	effectiveQty := math.Floor(remaining/rules.QtyStep) * rules.QtyStep
	if effectiveQty < rules.MinBaseQty {
		return true, effectiveQty
	}
	if rules.MinNotional > 0 && effectiveQty*priceHint < rules.MinNotional {
		return true, effectiveQty
	}
	return false, effectiveQty
}

// isAlreadyFlatError reports whether err indicates the futures position is already
// closed at the exchange — used for idempotent close detection on monitor retries.
func isAlreadyFlatError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "empty position") ||
		strings.Contains(msg, "position not exist") ||
		strings.Contains(msg, "no position") ||
		strings.Contains(msg, "position is zero") ||
		strings.Contains(msg, "reduce only order")
}

// verifyFuturesFlat double-checks with GetPosition whether the futures leg is
// flat. Returns true only when the exchange confirms zero open contracts.
func verifyFuturesFlat(futExch exchange.Exchange, symbol string) bool {
	positions, err := futExch.GetPosition(symbol)
	if err != nil {
		return false
	}
	if len(positions) == 0 {
		return true
	}
	for _, p := range positions {
		size, parseErr := strconv.ParseFloat(p.Total, 64)
		if parseErr != nil {
			return false
		}
		if math.Abs(size) > 1e-8 {
			return false
		}
	}
	return true
}

// ClosePosition closes an active spot-futures position by unwinding both legs.
// For Direction A (borrow_sell_long): close futures long → buy back spot → repay borrow.
// For Direction B (buy_spot_short): close futures short → sell spot.
// In emergency mode, both legs are closed in parallel with market IOC orders
// and a 5-second hard timeout.
// The method updates pos.SpotExitPrice and pos.FuturesExit in place on success.
func (e *SpotEngine) ClosePosition(pos *models.SpotFuturesPosition, reason string, isEmergency bool) error {
	e.log.Info("ClosePosition: %s on %s reason=%s emergency=%v", pos.Symbol, pos.Exchange, reason, isEmergency)
	pos.SyncHedgeState()
	if pos.HedgeBroken {
		e.log.Error("ClosePosition: refusing to close %s on %s because hedge is broken", pos.Symbol, pos.Exchange)
		e.telegram.NotifySpotCloseBlocked(pos, reason)
		return fmt.Errorf("hedge broken for %s on %s", pos.Symbol, pos.Exchange)
	}

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
				if isAlreadyFlatError(placeErr) && verifyFuturesFlat(futExch, pos.Symbol) {
					e.log.Info("ClosePosition [Dir A]: futures already flat (idempotent): %v", placeErr)
					if pos.FuturesExit == 0 {
						if ob, obErr := futExch.GetOrderbook(pos.Symbol, 5); obErr == nil && len(ob.Bids) > 0 && len(ob.Asks) > 0 {
							pos.FuturesExit = (ob.Bids[0].Price + ob.Asks[0].Price) / 2
						}
					}
					futFilled, futAvg = pos.FuturesSize, pos.FuturesExit
					return nil
				}
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

	// Step 2: Buy back spot (to return borrowed coin).
	// Exchanges with flash-repay (Bitget) skip the buy order entirely — the exchange
	// converts USDT collateral to repay the borrow directly.
	if spotExitComplete(pos) {
		e.log.Info("ClosePosition [Dir A] step 2: spot already closed (exit=%.6f), skipping", pos.SpotExitPrice)
	} else if flasher, ok := smExch.(exchange.FlashRepayer); ok {
		// Flash-repay path: no buy order, exchange handles conversion.
		e.log.Info("ClosePosition [Dir A] step 2: flash-repay %s on %s (skipping spot buyback)", pos.BaseCoin, pos.Exchange)
		repayID, err := flasher.FlashRepay(pos.BaseCoin)
		if err != nil {
			return fmt.Errorf("flash-repay failed: %w", err)
		}
		e.log.Info("ClosePosition [Dir A] step 2: flash-repay submitted repayId=%s", repayID)
		// Wait for settlement, then verify borrow is cleared.
		time.Sleep(3 * time.Second)
		if mb, mbErr := smExch.GetMarginBalance(pos.BaseCoin); mbErr == nil && (mb.Borrowed+mb.Interest) > 0 {
			e.log.Warn("ClosePosition [Dir A] step 2: flash-repay borrow not fully settled (remaining=%.8f), retrying", mb.Borrowed+mb.Interest)
			time.Sleep(3 * time.Second)
		}
		// Use futures exit price as the spot exit price for PnL tracking.
		pos.SpotExitFilledQty = pos.SpotSize
		pos.SpotExitFilled = true
		pos.SpotExitPrice = pos.FuturesExit
		if pos.SpotExitPrice <= 0 {
			if ob, obErr := futExch.GetOrderbook(pos.Symbol, 5); obErr == nil && len(ob.Bids) > 0 && len(ob.Asks) > 0 {
				pos.SpotExitPrice = (ob.Bids[0].Price + ob.Asks[0].Price) / 2
			}
		}
		e.log.Info("ClosePosition [Dir A] step 2: flash-repay complete, exit price=%.6f", pos.SpotExitPrice)
		if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
			e.log.Error("ClosePosition [Dir A]: failed to checkpoint flash-repay: %v", cpErr)
		}
	} else {
		// Standard path: buy back spot with auto-repay.
		e.log.Info("ClosePosition [Dir A] step 2: buy back spot BUY %s %s", pos.Symbol, utils.FormatSize(remainingSpotExitQty(pos), 6))
		// Dust short-circuit: if the residue is below exchange minimum, verify no
		// outstanding borrow before marking the spot leg closed (Dir A has borrow liability).
		if rules, rulesErr := smExch.SpotOrderRules(pos.Symbol); rulesErr == nil && rules != nil {
			remaining := remainingSpotExitQty(pos)
			refPrice := pos.FuturesExit
			if refPrice <= 0 {
				refPrice = pos.SpotEntryPrice
			}
			if isDust, eff := isSpotResidualDust(rules, remaining, refPrice); isDust {
				bal, balErr := smExch.GetMarginBalance(pos.BaseCoin)
				borrowed := 0.0
				if bal != nil {
					borrowed = bal.Borrowed
				}
				if balErr == nil && borrowed <= rules.MinBaseQty*0.001 {
					e.log.Warn("ClosePosition [Dir A]: spot residue %.6f (eff %.6f) is dust, borrow=%.8f cleared — marking closed",
						remaining, eff, borrowed)
					pos.SpotExitFilledQty = pos.SpotSize
					pos.SpotExitFilled = true
					pos.ExitReason += fmt.Sprintf(" (spot dust residue ignored: %.6f %s)", remaining, pos.BaseCoin)
					return nil
				}
				e.log.Warn("ClosePosition [Dir A]: spot residue %.6f is dust but borrow=%.8f remains — continuing", remaining, borrowed)
			}
		}
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
				refPrice := pos.FuturesExit
				if refPrice <= 0 {
					refPrice = pos.SpotEntryPrice
				}
				quoteEst := utils.FormatSize(remaining*refPrice*1.01, 2)
				var placeErr error
				orderID, placeErr = smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
					Symbol:    pos.Symbol,
					Side:      exchange.SideBuy,
					OrderType: "market",
					Size:      remainingStr,
					QuoteSize: quoteEst,
					AutoRepay: true,
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
		if pos.SpotExitFilledQty > 0 || pos.SpotExitPrice > 0 {
			if cpErr := e.persistExitCheckpoint(pos); cpErr != nil {
				e.log.Error("ClosePosition [Dir A]: failed to checkpoint spot exit: %v", cpErr)
			}
		}
	}

	// Step 3: Repay residual borrow.
	// Wait for exchange settlement — some exchanges (Bybit UTA) auto-settle
	// borrows after buyback but need a few seconds to process.
	time.Sleep(3 * time.Second)
	if mb, mbErr := smExch.GetMarginBalance(pos.BaseCoin); mbErr == nil && (mb.Borrowed+mb.Interest) > 0 {
		repayAmount := utils.FormatSize(mb.Borrowed+mb.Interest, 6)
		e.log.Info("ClosePosition [Dir A] step 3: repay residual borrow %s %s", repayAmount, pos.BaseCoin)
		if err := smExch.MarginRepay(exchange.MarginRepayParams{
			Coin:   pos.BaseCoin,
			Amount: repayAmount,
		}); err != nil {
			// Re-check balance: borrow may have auto-settled, or we may hold enough
			// to cover it (Bybit UTA reconciles implicit isLeverage=1 borrows internally).
			if mb2, mb2Err := smExch.GetMarginBalance(pos.BaseCoin); mb2Err == nil {
				if mb2.Borrowed <= 0 && mb2.Interest <= 0 {
					e.log.Info("ClosePosition [Dir A] step 3: borrow cleared by auto-settlement (was %s, now 0)", repayAmount)
				} else if mb2.TotalBalance >= mb2.Borrowed+mb2.Interest {
					e.log.Info("ClosePosition [Dir A] step 3: repay endpoint failed but balance covers liability (balance=%.8f >= borrowed=%.8f) — exchange will auto-settle",
						mb2.TotalBalance, mb2.Borrowed+mb2.Interest)
				} else {
					e.log.Error("ClosePosition [Dir A] repay FAILED: %v — will retry on next monitor tick for %s %s",
						err, repayAmount, pos.BaseCoin)
					pos.PendingRepay = true
					var blackout *exchange.ErrRepayBlackout
					if errors.As(err, &blackout) {
						pos.PendingRepayRetryAt = &blackout.RetryAfter
					}
				}
			} else {
				e.log.Error("ClosePosition [Dir A] repay FAILED: %v — will retry on next monitor tick for %s %s",
					err, repayAmount, pos.BaseCoin)
				pos.PendingRepay = true
			}
		}
	} else if mbErr == nil {
		e.log.Info("ClosePosition [Dir A] step 3: borrow already settled (borrowed=0) — no repay needed")
	} else if mbErr != nil {
		e.log.Warn("ClosePosition [Dir A] step 3: GetMarginBalance failed (%v), falling back to full repay", mbErr)
		repayAmount := utils.FormatSize(pos.BorrowAmount, 6)
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
	}

	// Step 3b: Sweep any micro-dust with repaid_all.
	// The exact-amount repay above can leave floating-point dust. A zero-amount
	// repay triggers repaid_all on exchanges that support it (Gate.io unified).
	time.Sleep(1 * time.Second)
	if mb, mbErr := smExch.GetMarginBalance(pos.BaseCoin); mbErr == nil && (mb.Borrowed+mb.Interest) > 0 {
		dust := mb.Borrowed + mb.Interest
		e.log.Info("ClosePosition [Dir A] step 3b: dust sweep %s (remaining=%.8f)", pos.BaseCoin, dust)
		if err := smExch.MarginRepay(exchange.MarginRepayParams{
			Coin:   pos.BaseCoin,
			Amount: "0",
		}); err != nil {
			e.log.Warn("ClosePosition [Dir A] step 3b: dust sweep failed: %v (%.8f %s remains)", err, dust, pos.BaseCoin)
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
				if isAlreadyFlatError(placeErr) && verifyFuturesFlat(futExch, pos.Symbol) {
					e.log.Info("ClosePosition [Dir B]: futures already flat (idempotent): %v", placeErr)
					if pos.FuturesExit == 0 {
						if ob, obErr := futExch.GetOrderbook(pos.Symbol, 5); obErr == nil && len(ob.Bids) > 0 && len(ob.Asks) > 0 {
							pos.FuturesExit = (ob.Bids[0].Price + ob.Asks[0].Price) / 2
						}
					}
					futFilled, futAvg = pos.FuturesSize, pos.FuturesExit
					return nil
				}
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
		// Fetch rules once — used for dust short-circuit and for flooring the
		// sell qty to LOT_SIZE step (avoids Binance -1013 when SpotSize from a
		// quote-qty market buy is not aligned to stepSize, e.g. DEGOUSDT 1258.1406
		// vs step=0.01).
		var spotRules *exchange.SpotOrderRules
		if rules, rulesErr := smExch.SpotOrderRules(pos.Symbol); rulesErr == nil && rules != nil {
			spotRules = rules
			remaining := remainingSpotExitQty(pos)
			priceHint := pos.FuturesExit
			if priceHint <= 0 {
				priceHint = pos.SpotEntryPrice
			}
			if isDust, eff := isSpotResidualDust(rules, remaining, priceHint); isDust {
				e.log.Warn("ClosePosition [Dir B]: spot residue %.6f (eff %.6f) below minBase=%.6f/minNotional=%.2f — marking closed",
					remaining, eff, rules.MinBaseQty, rules.MinNotional)
				pos.SpotExitFilledQty = pos.SpotSize
				pos.SpotExitFilled = true
				pos.ExitReason += fmt.Sprintf(" (spot dust residue ignored: %.6f %s)", remaining, pos.BaseCoin)
				return nil
			}
		}
		err := e.retryLeg("dirB-spot-sell", 3, 2*time.Second, func() error {
			orderID := pos.PendingSpotExitOrderID
			if orderID == "" {
				remaining := remainingSpotExitQty(pos)
				if remaining <= 0 {
					pos.SpotExitFilledQty = pos.SpotSize
					pos.SpotExitFilled = true
					return nil
				}
				sellQty := remaining
				if spotRules != nil && spotRules.QtyStep > 0 {
					sellQty = utils.RoundToStep(remaining, spotRules.QtyStep)
					if sellQty <= 0 {
						// Floored to zero — residue below step is dust, mark closed.
						e.log.Warn("ClosePosition [Dir B]: remaining %.8f floors to 0 at step=%g — marking closed", remaining, spotRules.QtyStep)
						pos.SpotExitFilledQty = pos.SpotSize
						pos.SpotExitFilled = true
						pos.ExitReason += fmt.Sprintf(" (spot dust residue ignored: %.8f %s)", remaining, pos.BaseCoin)
						return nil
					}
				}
				remainingStr := utils.FormatSize(sellQty, 6)
				var placeErr error
				orderID, placeErr = smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
					Symbol:    pos.Symbol,
					Side:      exchange.SideSell,
					OrderType: "market",
					Size:      remainingStr,
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
			if isAlreadyFlatError(err) && verifyFuturesFlat(futExch, pos.Symbol) {
				e.log.Info("EMERGENCY: futures already flat (idempotent): %v", err)
				avg := pos.FuturesExit
				if avg == 0 {
					if ob, obErr := futExch.GetOrderbook(pos.Symbol, 5); obErr == nil && len(ob.Bids) > 0 && len(ob.Asks) > 0 {
						avg = (ob.Bids[0].Price + ob.Asks[0].Price) / 2
					}
				}
				ch <- result{leg: "futures", avg: avg}
				return
			}
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
		// Dust short-circuit: skip untradeable residue without placing a doomed order.
		if rules, rulesErr := smExch.SpotOrderRules(pos.Symbol); rulesErr == nil && rules != nil {
			if isDust, eff := isSpotResidualDust(rules, remaining, pos.SpotEntryPrice); isDust {
				if pos.Direction == "borrow_sell_long" {
					bal, balErr := smExch.GetMarginBalance(pos.BaseCoin)
					borrowed := 0.0
					if bal != nil {
						borrowed = bal.Borrowed
					}
					if balErr == nil && borrowed <= rules.MinBaseQty*0.001 {
						e.log.Warn("EMERGENCY: spot residue %.6f (eff %.6f) is dust, borrow=%.8f cleared — marking closed", remaining, eff, borrowed)
						pos.SpotExitFilledQty = pos.SpotSize
						pos.SpotExitFilled = true
						ch <- result{leg: "spot", avg: pos.SpotExitPrice}
						return
					}
					e.log.Warn("EMERGENCY: spot residue %.6f is dust but borrow=%.8f remains — attempting order", remaining, borrowed)
				} else {
					e.log.Warn("EMERGENCY: spot residue %.6f (eff %.6f) is dust — marking closed", remaining, eff)
					pos.SpotExitFilledQty = pos.SpotSize
					pos.SpotExitFilled = true
					ch <- result{leg: "spot", avg: pos.SpotExitPrice}
					return
				}
			}
		}
		remainingStr := utils.FormatSize(remaining, 6)
		var quoteEst string
		if spotSide == exchange.SideBuy {
			quoteEst = utils.FormatSize(remaining*pos.SpotEntryPrice*1.02, 2)
		}
		orderID, err := smExch.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
			Symbol:    pos.Symbol,
			Side:      spotSide,
			OrderType: "market",
			Size:      remainingStr,
			QuoteSize: quoteEst,
			AutoRepay: pos.Direction == "borrow_sell_long",
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

	// Repay residual borrow if Direction A (auto-repay should have handled most/all).
	if pos.Direction == "borrow_sell_long" {
		if mb, mbErr := smExch.GetMarginBalance(pos.BaseCoin); mbErr == nil && (mb.Borrowed+mb.Interest) > 0 {
			residual := utils.FormatSize(mb.Borrowed+mb.Interest, 6)
			e.log.Info("EMERGENCY: repaying residual borrow %s %s", residual, pos.BaseCoin)
			if err := smExch.MarginRepay(exchange.MarginRepayParams{
				Coin:   pos.BaseCoin,
				Amount: residual,
			}); err != nil {
				e.log.Error("EMERGENCY: repay FAILED: %v — will retry on next monitor tick", err)
				pos.PendingRepay = true
				var blackout *exchange.ErrRepayBlackout
				if errors.As(err, &blackout) {
					pos.PendingRepayRetryAt = &blackout.RetryAfter
				}
			}
		} else if mbErr != nil {
			e.log.Warn("EMERGENCY: GetMarginBalance failed (%v), falling back to full repay", mbErr)
			repayAmount := utils.FormatSize(pos.BorrowAmount, 6)
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
