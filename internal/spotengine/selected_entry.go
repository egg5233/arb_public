package spotengine

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/utils"
)

// selected_entry.go implements the spotEntryExecutor contract declared at
// internal/engine/spot_entry_iface.go. The unified cross-strategy entry
// selector pulls candidates, scores them against preheld reservations, and
// dispatches winners via OpenSelectedEntry, which mirrors ManualOpen's
// execution primitives but commits a preheld reservation instead of calling
// Reserve during dispatch.

// ListEntryCandidates returns fresh, post-filter spot arbitrage candidates
// drawn from the latest discovery scan. Stale entries older than maxAge are
// dropped; entries with a non-empty FilterStatus never qualify.
//
// The returned slice is a defensive copy of candidate projections: callers
// may freely retain or iterate without holding SpotEngine locks.
func (e *SpotEngine) ListEntryCandidates(maxAge time.Duration) []models.SpotEntryCandidate {
	opps := e.getLatestOpps()
	if len(opps) == 0 {
		return nil
	}
	now := time.Now()
	out := make([]models.SpotEntryCandidate, 0, len(opps))
	for _, o := range opps {
		if o.FilterStatus != "" {
			continue
		}
		if maxAge > 0 && !o.Timestamp.IsZero() {
			if age := now.Sub(o.Timestamp); age > maxAge {
				continue
			}
		}
		out = append(out, models.SpotEntryCandidate{
			Symbol:     o.Symbol,
			BaseCoin:   o.BaseCoin,
			Exchange:   o.Exchange,
			Direction:  o.Direction,
			FundingAPR: o.FundingAPR,
			BorrowAPR:  o.BorrowAPR,
			FeePct:     o.FeePct,
			Timestamp:  o.Timestamp,
		})
	}
	return out
}

// BuildEntryPlan sizes a SpotEntryCandidate into a feasibility-checked
// SpotEntryPlan. The plan carries the exact USDT amount that will be
// reserved, so the selector's scoring and batch reservation see identical
// numbers to what execution commits.
//
// Feasibility rules (matches live ManualOpen at execution.go:183-265):
//   - Dir A on UNIFIED accounts (Bybit UTA / OKX / Gate.io): cap rawSize by
//     MaxBorrowableBase ONLY when MaxBorrowableBase > 0. When MaxBorrowable is
//     unknown, do NOT cap pre-transfer — live execution polls post-transfer.
//   - Dir A on SEPARATE accounts (Binance / Bitget): do NOT cap pre-transfer.
//     Live execution transfers USDT to margin FIRST, then polls MaxBorrowable.
//     Plan.MaxBorrowableBase=0 (not authoritative).
//   - Dir B: no borrow, MaxBorrowableBase=0.
//   - BingX: returns an error — no spot margin adapter is available.
//   - Rejection: PlannedBaseSize <= 0, PlannedBaseSize < futMin (when futMin>0),
//     or PlannedNotionalUSDT < max(CapitalBudgetUSDT*0.10, 5.0).
func (e *SpotEngine) BuildEntryPlan(c models.SpotEntryCandidate) (*models.SpotEntryPlan, error) {
	symbol := strings.ToUpper(strings.TrimSpace(c.Symbol))
	exchName := strings.ToLower(strings.TrimSpace(c.Exchange))
	direction := strings.TrimSpace(c.Direction)

	if direction != "borrow_sell_long" && direction != "buy_spot_short" {
		return nil, fmt.Errorf("BuildEntryPlan: invalid direction %q", direction)
	}

	smExch, smOK := e.spotMargin[exchName]
	futExch, futOK := e.exchanges[exchName]
	if !futOK {
		return nil, fmt.Errorf("BuildEntryPlan: exchange %s not available", exchName)
	}
	if !smOK {
		// BingX and any other venue without a SpotMarginExchange adapter is
		// ineligible for spot-futures entry regardless of direction.
		return nil, fmt.Errorf("BuildEntryPlan: exchange %s does not support spot margin", exchName)
	}

	// Capital budget for this exchange — respects unified vs separate capital
	// config and allocator overrides (capitalForExchange unchanged).
	capital := e.capitalForExchange(exchName)
	if capital <= 0 {
		return nil, fmt.Errorf("BuildEntryPlan: zero capital budget for %s", exchName)
	}

	// Mid price from futures orderbook BBO — identical source to ManualOpen
	// execution.go:170-178 so planning and execution reconcile to the same
	// snapshot.
	ob, err := futExch.GetOrderbook(symbol, 5)
	if err != nil {
		return nil, fmt.Errorf("BuildEntryPlan: orderbook for %s on %s: %w", symbol, exchName, err)
	}
	if ob == nil || len(ob.Bids) == 0 || len(ob.Asks) == 0 {
		return nil, fmt.Errorf("BuildEntryPlan: empty orderbook for %s on %s", symbol, exchName)
	}
	midPrice := (ob.Bids[0].Price + ob.Asks[0].Price) / 2
	if midPrice <= 0 {
		return nil, fmt.Errorf("BuildEntryPlan: non-positive mid price %.6f for %s", midPrice, symbol)
	}

	rawSize := capital / midPrice

	// Direction A on UNIFIED accounts: size-aware MaxBorrowable cap.
	// Separate accounts skip this — MaxBorrowable is not authoritative until
	// the live transfer-to-margin step in ManualOpen completes.
	var maxBorrowable float64
	if direction == "borrow_sell_long" && !needsMarginTransfer(exchName) {
		mb, mbErr := smExch.GetMarginBalance(c.BaseCoin)
		if mbErr != nil {
			// Unknown — do not cap; execution will cap post-poll if needed.
			e.log.Warn("BuildEntryPlan: GetMarginBalance(%s) on %s failed: %v — proceeding without cap", c.BaseCoin, exchName, mbErr)
		} else if mb != nil {
			maxBorrowable = mb.MaxBorrowable
			if maxBorrowable > 0 && rawSize > maxBorrowable {
				rawSize = maxBorrowable
			}
		}
	}

	// Round size down to futures contract step so both legs match exactly at
	// execution time. When contract info is unavailable, fall back to 6dp
	// flooring (same as ManualOpen). Step decimals are only needed for the
	// string-formatted order payload at execution; planning doesn't care.
	futStep, futMin, _ := e.futuresStepSize(futExch, symbol)
	var plannedBase float64
	if futStep > 0 {
		plannedBase = utils.RoundToStep(rawSize, futStep)
	} else {
		plannedBase = math.Floor(rawSize*1e6) / 1e6
	}
	if plannedBase <= 0 {
		return nil, fmt.Errorf("BuildEntryPlan: computed size 0 for %s (raw=%.6f step=%.6f)", symbol, rawSize, futStep)
	}
	if futMin > 0 && plannedBase < futMin {
		return nil, fmt.Errorf("BuildEntryPlan: size %.6f below futures min %.6f for %s", plannedBase, futMin, symbol)
	}

	plannedNotional := plannedBase * midPrice

	// Budget floor — reject trades smaller than 10% of the per-exchange
	// capital budget (or $5, whichever is larger). Matches ManualOpen.
	budgetFloor := capital * 0.10
	if budgetFloor < 5.0 {
		budgetFloor = 5.0
	}
	if plannedNotional < budgetFloor {
		return nil, fmt.Errorf("BuildEntryPlan: notional %.2f USDT below floor %.2f for %s (size=%.6f price=%.6f)",
			plannedNotional, budgetFloor, symbol, plannedBase, midPrice)
	}

	leverage := float64(e.cfg.SpotFuturesLeverage)
	if leverage <= 0 {
		leverage = 3.0
	}
	futuresMargin := plannedNotional / leverage

	// Transfer flag semantics (matches live ManualOpen behavior):
	//   Dir A on separate  → transfer USDT to margin
	//   Dir B on separate  → transfer USDT to spot
	//   Unified            → no transfer (shared pool)
	var requiresTransfer bool
	var transferTarget string
	if needsMarginTransfer(exchName) {
		requiresTransfer = true
		if direction == "borrow_sell_long" {
			transferTarget = "margin"
		} else {
			transferTarget = "spot"
		}
	}

	// Normalize candidate identifiers so selector keys are stable.
	normalized := c
	normalized.Symbol = symbol
	normalized.Exchange = exchName
	normalized.Direction = direction

	plan := &models.SpotEntryPlan{
		Candidate:                normalized,
		CapitalBudgetUSDT:        capital,
		MidPrice:                 midPrice,
		PlannedBaseSize:          plannedBase,
		PlannedNotionalUSDT:      plannedNotional,
		FuturesMarginUSDT:        futuresMargin,
		MaxBorrowableBase:        0,
		RequiresInternalTransfer: requiresTransfer,
		TransferTarget:           transferTarget,
	}
	// Only Dir A on unified accounts carries an authoritative MaxBorrowable
	// on the plan. Dir A separate and Dir B always carry 0.
	if direction == "borrow_sell_long" && !needsMarginTransfer(exchName) {
		plan.MaxBorrowableBase = maxBorrowable
	}

	return plan, nil
}

// OpenSelectedEntry dispatches a sized SpotEntryPlan produced by the unified
// selector. It mirrors ManualOpen's execution primitives (transfer, borrow
// poll, spot margin order, futures hedge, checkpointing) but sources the
// reservation from a preheld risk.CapitalAllocator.ReserveBatch item rather
// than reserving again — the selector already accounted for this exposure.
//
// capOverride mirrors risk.ReserveWithCap semantics: zero means no override.
// It is forwarded only when preheld is nil and the function must fall back to
// reserving capital itself (defensive path; selector is expected to preheld).
func (e *SpotEngine) OpenSelectedEntry(plan *models.SpotEntryPlan, capOverride float64, preheld *risk.CapitalReservation) error {
	if plan == nil {
		return errors.New("OpenSelectedEntry: nil plan")
	}
	if plan.PlannedBaseSize <= 0 || plan.PlannedNotionalUSDT <= 0 {
		return fmt.Errorf("OpenSelectedEntry: plan not sized (base=%.6f notional=%.2f)",
			plan.PlannedBaseSize, plan.PlannedNotionalUSDT)
	}

	symbol := plan.Candidate.Symbol
	exchName := plan.Candidate.Exchange
	direction := plan.Candidate.Direction
	baseCoin := plan.Candidate.BaseCoin

	lock, acquired, err := e.db.AcquireOwnedLock(spotEntryLockKey, spotEntryLockTTL)
	if err != nil {
		return fmt.Errorf("OpenSelectedEntry: acquire entry lock: %w", err)
	}
	if !acquired {
		return errors.New("OpenSelectedEntry: spot entry already in progress")
	}
	defer func() {
		if err := lock.Release(); err != nil {
			e.log.Warn("OpenSelectedEntry: release entry lock: %v", err)
		}
	}()
	requireEntryLock := func(action string) error {
		if err := lock.Check(); err != nil {
			return fmt.Errorf("spot entry lock lost before %s: %w", action, err)
		}
		return nil
	}

	e.log.Info("OpenSelectedEntry: %s on %s direction=%s base=%.6f notional=%.2f",
		symbol, exchName, direction, plan.PlannedBaseSize, plan.PlannedNotionalUSDT)

	if direction != "borrow_sell_long" && direction != "buy_spot_short" {
		return fmt.Errorf("OpenSelectedEntry: invalid direction %q", direction)
	}

	smExch, ok := e.spotMargin[exchName]
	if !ok {
		return fmt.Errorf("OpenSelectedEntry: exchange %s does not support spot margin", exchName)
	}
	futExch, ok := e.exchanges[exchName]
	if !ok {
		return fmt.Errorf("OpenSelectedEntry: exchange %s not found", exchName)
	}

	// Duplicate-symbol / capacity guards — same as ManualOpen. Selector runs
	// its own cross-strategy exclusion but these remain as defense in depth.
	active, err := e.db.GetActiveSpotPositions()
	if err != nil {
		return fmt.Errorf("OpenSelectedEntry: load active positions: %w", err)
	}
	for _, pos := range active {
		if pos.Symbol == symbol {
			return fmt.Errorf("OpenSelectedEntry: %s already open on %s", symbol, pos.Exchange)
		}
	}
	if len(active) >= e.cfg.SpotFuturesMaxPositions {
		return fmt.Errorf("OpenSelectedEntry: at max capacity (%d/%d)",
			len(active), e.cfg.SpotFuturesMaxPositions)
	}
	if e.cfg.SpotFuturesDryRun {
		return errors.New("OpenSelectedEntry: dry run mode — trade not executed")
	}
	if err := requireEntryLock("pre-execution checks"); err != nil {
		return err
	}

	// ----------------------------------------------------------------
	// Reservation handling — prefer preheld; fall back to self-reserve.
	// ----------------------------------------------------------------
	reservation := preheld
	reservedSelf := false
	if reservation == nil {
		reservation, err = e.reserveSpotCapital(exchName, plan.PlannedNotionalUSDT, capOverride)
		if err != nil {
			return fmt.Errorf("OpenSelectedEntry: capital allocator rejected: %w", err)
		}
		reservedSelf = true
	}
	// Helper closures that either commit/release the preheld reservation or
	// the self-reserved one with identical error plumbing.
	commit := func(posID string, amount float64) error {
		if preheld != nil {
			return e.commitSpotFromPreheld(reservation, posID, amount)
		}
		return e.commitSpotCapital(reservation, posID, amount)
	}
	release := func() {
		// Only release when we own the reservation. Preheld reservations are
		// released by the selector when dispatch fails; we must not double-release.
		if reservedSelf {
			e.releaseSpotReservation(reservation)
		}
	}

	// Transfer USDT to the correct wallet on separate-account venues.
	if plan.RequiresInternalTransfer {
		transferAmt := fmt.Sprintf("%.2f", plan.CapitalBudgetUSDT*1.05)
		switch plan.TransferTarget {
		case "margin":
			e.log.Info("OpenSelectedEntry: transferring %s USDT to margin (%s)", transferAmt, exchName)
			if tErr := smExch.TransferToMargin("USDT", transferAmt); tErr != nil {
				e.log.Warn("OpenSelectedEntry: TransferToMargin(%s USDT): %v (may already have funds)", transferAmt, tErr)
			}
		case "spot":
			e.log.Info("OpenSelectedEntry: transferring %s USDT to spot (%s)", transferAmt, exchName)
			if tErr := futExch.TransferToSpot("USDT", transferAmt); tErr != nil {
				e.log.Warn("OpenSelectedEntry: TransferToSpot(%s USDT): %v (may already have funds)", transferAmt, tErr)
			}
		}
	}

	// Dir A on separate accounts: poll MaxBorrowable after transfer settles.
	// This mirrors ManualOpen exactly — the plan carries 0 for MaxBorrowable
	// on separate accounts specifically so the authoritative poll happens here.
	rawSize := plan.PlannedBaseSize
	if direction == "borrow_sell_long" && needsMarginTransfer(exchName) {
		minExpected := rawSize * 0.5
		var maxBorrow float64
		for poll := 0; poll < 10; poll++ {
			mb, err := smExch.GetMarginBalance(baseCoin)
			if err != nil {
				e.log.Warn("OpenSelectedEntry: GetMarginBalance(%s) poll %d failed: %v", baseCoin, poll+1, err)
				break
			}
			maxBorrow = mb.MaxBorrowable
			if maxBorrow >= minExpected {
				break
			}
			e.log.Info("OpenSelectedEntry: MaxBorrowable %.6f < expected %.6f — waiting (%d/10)",
				maxBorrow, minExpected, poll+1)
			time.Sleep(500 * time.Millisecond)
		}
		if maxBorrow > 0 && rawSize > maxBorrow {
			e.log.Info("OpenSelectedEntry: post-transfer capping from %.6f to MaxBorrowable %.6f", rawSize, maxBorrow)
			rawSize = maxBorrow
		}
	}

	futStep, futMin, futDec := e.futuresStepSize(futExch, symbol)
	var size float64
	if futStep > 0 {
		size = utils.RoundToStep(rawSize, futStep)
	} else {
		size = math.Floor(rawSize*1e6) / 1e6
	}
	if size <= 0 || (futMin > 0 && size < futMin) {
		release()
		return fmt.Errorf("OpenSelectedEntry: post-cap size %.6f too small for %s (raw=%.6f step=%.6f min=%.6f)",
			size, symbol, rawSize, futStep, futMin)
	}
	sizeStr := utils.FormatSize(size, futDec)
	plannedNotional := size * plan.MidPrice

	budgetFloor := plan.CapitalBudgetUSDT * 0.10
	if budgetFloor < 5.0 {
		budgetFloor = 5.0
	}
	if plannedNotional < budgetFloor {
		release()
		return fmt.Errorf("OpenSelectedEntry: notional %.2f USDT below floor %.2f for %s",
			plannedNotional, budgetFloor, symbol)
	}

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
		BorrowRateHourly: plan.Candidate.BorrowAPR / 8760,
		FundingAPR:       plan.Candidate.FundingAPR,
		FeePct:           plan.Candidate.FeePct,
		CurrentBorrowAPR: plan.Candidate.BorrowAPR,
		NotionalUSDT:     plannedNotional,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if direction == "borrow_sell_long" {
		entryPos.FuturesSide = "long"
	} else {
		entryPos.FuturesSide = "short"
	}

	if err := requireEntryLock("persist pending entry"); err != nil {
		release()
		return err
	}
	if err := e.persistPendingEntry(entryPos, ""); err != nil {
		release()
		return fmt.Errorf("OpenSelectedEntry: persist pending entry: %w", err)
	}

	// ----------------------------------------------------------------
	// Set leverage (best-effort) then dispatch to direction executor.
	// ----------------------------------------------------------------
	leverage := e.cfg.SpotFuturesLeverage
	if leverage <= 0 {
		leverage = 3
	}
	leverageStr := strconv.Itoa(leverage)
	if err := futExch.SetLeverage(symbol, leverageStr, ""); err != nil {
		e.log.Warn("OpenSelectedEntry: SetLeverage(%s, %s): %v", symbol, leverageStr, err)
	}

	var spotEntryPrice, futuresEntryPrice float64
	var spotFilledQty, futuresFilledQty float64
	var futuresSide string
	var borrowAmount float64

	switch direction {
	case "borrow_sell_long":
		spotEntryPrice, futuresEntryPrice, spotFilledQty, futuresFilledQty, borrowAmount, err = e.executeBorrowSellLong(
			smExch, futExch, symbol, baseCoin, sizeStr, size, futStep, futMin, futDec, requireEntryLock, entryPos)
		futuresSide = "long"
	case "buy_spot_short":
		spotEntryPrice, futuresEntryPrice, spotFilledQty, futuresFilledQty, err = e.executeBuySpotShort(
			smExch, futExch, symbol, sizeStr, size, plannedNotional, futStep, futMin, futDec, requireEntryLock, entryPos)
		futuresSide = "short"
	}

	if err != nil {
		var pendingErr *pendingSpotEntryError
		if errors.As(err, &pendingErr) {
			recoverySaveFailed := false
			if pendingErr.pendingPos != nil {
				if saveErr := e.db.SaveSpotPosition(pendingErr.pendingPos); saveErr != nil {
					e.log.Error("OpenSelectedEntry: save pending recovery %s: %v", pendingErr.posID, saveErr)
					recoverySaveFailed = true
				} else if e.api != nil {
					e.api.BroadcastSpotPositionUpdate(pendingErr.pendingPos)
				}
			}
			amount := plannedNotional
			if pendingErr.capitalAmount > 0 {
				amount = pendingErr.capitalAmount
			}
			if commitErr := commit(pendingErr.posID, amount); commitErr != nil {
				e.log.Error("OpenSelectedEntry: capital commit failed for pending entry %s: %v", pendingErr.posID, commitErr)
			}
			if recoverySaveFailed {
				return fmt.Errorf("%w (manual recovery position %s could not be persisted)", err, pendingErr.posID)
			}
			e.log.Error("OpenSelectedEntry: %s on %s — pending entry error: %v", symbol, exchName, err)
			return err
		}
		e.abandonPendingEntry(entryPos, "entry_failed")
		release()
		e.log.Error("OpenSelectedEntry: %s on %s — execution failed: %v", symbol, exchName, err)
		return fmt.Errorf("execution failed: %w", err)
	}

	actualNotional := spotFilledQty * spotEntryPrice

	takerFee := spotFees[exchName]
	if takerFee == 0 {
		takerFee = 0.0005
	}
	entryFees := (spotFilledQty * spotEntryPrice * takerFee) + (futuresFilledQty * futuresEntryPrice * takerFee)

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
		if commitErr := commit(posID, actualNotional); commitErr != nil {
			e.log.Error("OpenSelectedEntry: capital commit failed after pending checkpoint save error: %v", commitErr)
		}
		e.log.Error("OpenSelectedEntry: failed to checkpoint executed position: %v", err)
		return fmt.Errorf("position executed but failed to checkpoint pending state: %w", err)
	}
	if err := commit(posID, actualNotional); err != nil {
		e.log.Error("OpenSelectedEntry: capital commit failed: %v", err)
	}

	pos.Status = models.SpotStatusActive
	if err := e.db.SaveSpotPosition(pos); err != nil {
		e.log.Error("OpenSelectedEntry: save final position: %v", err)
		return fmt.Errorf("position executed and checkpointed but failed to promote active: %w", err)
	}

	if e.api != nil {
		e.api.BroadcastSpotPositionUpdate(pos)
	}
	e.log.Info("OpenSelectedEntry: SUCCESS — %s on %s [%s] spot=%.6f@%.6f futures=%.6f@%.6f notional=%.2f",
		symbol, exchName, direction, spotFilledQty, spotEntryPrice, futuresFilledQty, futuresEntryPrice, actualNotional)

	return nil
}
