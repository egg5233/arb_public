package risk

import (
	"fmt"
	"math"

	"arb/internal/config"
	"arb/internal/database"
	"arb/pkg/exchange"
	"arb/internal/models"
	"arb/pkg/utils"
)

// Compile-time check: *Manager satisfies models.RiskChecker.
var _ models.RiskChecker = (*Manager)(nil)

// Manager performs pre-trade risk assessment before entering positions.
// It satisfies models.RiskChecker.
type Manager struct {
	exchanges map[string]exchange.Exchange
	db        *database.Client
	cfg       *config.Config
	log       *utils.Logger
}

// NewManager creates a new risk Manager.
func NewManager(exchanges map[string]exchange.Exchange, db *database.Client, cfg *config.Config) *Manager {
	return &Manager{
		exchanges: exchanges,
		db:        db,
		cfg:       cfg,
		log:       utils.NewLogger("risk"),
	}
}

// Approve runs all pre-trade risk checks and returns an Approval decision.
func (m *Manager) Approve(opp models.Opportunity) (*models.RiskApproval, error) {
	// a. Position count check
	active, err := m.db.GetActivePositions()
	if err != nil {
		return nil, fmt.Errorf("get active positions: %w", err)
	}
	if len(active) >= m.cfg.MaxPositions {
		return &models.RiskApproval{Approved: false, Reason: fmt.Sprintf("max positions reached (%d/%d)", len(active), m.cfg.MaxPositions)}, nil
	}

	longExch, ok := m.exchanges[opp.LongExchange]
	if !ok {
		return &models.RiskApproval{Approved: false, Reason: fmt.Sprintf("long exchange %s not configured", opp.LongExchange)}, nil
	}
	shortExch, ok := m.exchanges[opp.ShortExchange]
	if !ok {
		return &models.RiskApproval{Approved: false, Reason: fmt.Sprintf("short exchange %s not configured", opp.ShortExchange)}, nil
	}

	// b. Capital check — get balances from both exchanges.
	// If futures balance is insufficient and spot has funds, auto-transfer.
	longBal, err := longExch.GetFuturesBalance()
	if err != nil {
		return nil, fmt.Errorf("get balance from %s: %w", opp.LongExchange, err)
	}
	shortBal, err := shortExch.GetFuturesBalance()
	if err != nil {
		return nil, fmt.Errorf("get balance from %s: %w", opp.ShortExchange, err)
	}

	needed := m.cfg.CapitalPerLeg
	if needed > 0 {
		m.ensureFuturesBalance(opp.LongExchange, longExch, longBal, needed)
		m.ensureFuturesBalance(opp.ShortExchange, shortExch, shortBal, needed)
		// Re-fetch after potential transfer
		longBal, _ = longExch.GetFuturesBalance()
		shortBal, _ = shortExch.GetFuturesBalance()
	}

	balances := map[string]float64{
		opp.LongExchange:  longBal.Available,
		opp.ShortExchange: shortBal.Available,
	}

	// c. Leverage check
	leverage := m.cfg.Leverage
	if leverage > MaxLeverage() {
		leverage = MaxLeverage()
		m.log.Warn("configured leverage %d exceeds hard cap, clamped to %d", m.cfg.Leverage, leverage)
	}

	// Get a reference price from the long exchange orderbook
	longOB, err := longExch.GetOrderbook(opp.Symbol, 20)
	if err != nil {
		return nil, fmt.Errorf("get orderbook from %s: %w", opp.LongExchange, err)
	}
	if len(longOB.Asks) == 0 || len(longOB.Bids) == 0 {
		return &models.RiskApproval{Approved: false, Reason: "empty orderbook on long exchange"}, nil
	}
	midPrice := (longOB.Bids[0].Price + longOB.Asks[0].Price) / 2.0

	// Calculate position size
	remainingSlots := m.cfg.MaxPositions - len(active)
	size := m.CalculateSize(opp, balances)
	if size <= 0 {
		return &models.RiskApproval{Approved: false, Reason: "insufficient capital for minimum position size"}, nil
	}

	// Verify enough free margin per leg
	requiredMarginPerLeg := (size * midPrice) / float64(leverage)
	if longBal.Available < requiredMarginPerLeg {
		return &models.RiskApproval{Approved: false, Reason: fmt.Sprintf("insufficient margin on %s: need %.2f, have %.2f", opp.LongExchange, requiredMarginPerLeg, longBal.Available)}, nil
	}
	if shortBal.Available < requiredMarginPerLeg {
		return &models.RiskApproval{Approved: false, Reason: fmt.Sprintf("insufficient margin on %s: need %.2f, have %.2f", opp.ShortExchange, requiredMarginPerLeg, shortBal.Available)}, nil
	}

	// d. Orderbook depth / slippage check on both exchanges
	shortOB, err := shortExch.GetOrderbook(opp.Symbol, 20)
	if err != nil {
		return nil, fmt.Errorf("get orderbook from %s: %w", opp.ShortExchange, err)
	}

	if len(shortOB.Asks) == 0 || len(shortOB.Bids) == 0 {
		return &models.RiskApproval{Approved: false, Reason: fmt.Sprintf("empty orderbook on %s", opp.ShortExchange)}, nil
	}

	// Compute both mid prices for slippage estimation.
	shortMid := midPrice
	if len(shortOB.Bids) > 0 && len(shortOB.Asks) > 0 {
		shortMid = (shortOB.Bids[0].Price + shortOB.Asks[0].Price) / 2.0
	}

	// Estimate slippage on both legs at full size.
	longSlippage := estimateSlippageFromLevels(longOB.Asks, size, midPrice)
	shortSlippage := estimateSlippageFromLevels(shortOB.Bids, size, shortMid)

	if longSlippage > m.cfg.SlippageBPS || shortSlippage > m.cfg.SlippageBPS {
		// Slippage too high at full size — try adaptive sizing.
		// Load contract info from both exchanges; use the more restrictive step/min.
		var stepSize, minSizeContract float64
		for _, exch := range []exchange.Exchange{longExch, shortExch} {
			contracts, err := exch.LoadAllContracts()
			if err != nil {
				continue
			}
			ci, ok := contracts[opp.Symbol]
			if !ok {
				continue
			}
			if ci.StepSize > stepSize {
				stepSize = ci.StepSize
			}
			if ci.MinSize > minSizeContract {
				minSizeContract = ci.MinSize
			}
		}
		if stepSize <= 0 {
			// No contract info available — skip adaptive sizing, reject as before.
			bottleneck := opp.LongExchange
			bps := longSlippage
			if shortSlippage > longSlippage {
				bottleneck = opp.ShortExchange
				bps = shortSlippage
			}
			return &models.RiskApproval{Approved: false, Reason: fmt.Sprintf("slippage too high on %s: %.1f bps > %.1f bps limit", bottleneck, bps, m.cfg.SlippageBPS)}, nil
		}

		// Min size = max(contract minimum, $10 floor / price * leverage)
		// Round UP to ensure we don't go below the capital floor.
		minSizeFromCapital := (minCapitalFloor * float64(leverage)) / midPrice
		minSizeFromCapital = math.Ceil(minSizeFromCapital/stepSize) * stepSize
		adaptiveMinSize := math.Max(minSizeContract, minSizeFromCapital)

		adaptedSize := findMaxSizeForSlippage(
			longOB.Asks, shortOB.Bids,
			midPrice, shortMid,
			size, adaptiveMinSize,
			m.cfg.SlippageBPS, stepSize,
		)

		if adaptedSize <= 0 {
			minLong := estimateSlippageFromLevels(longOB.Asks, adaptiveMinSize, midPrice)
			minShort := estimateSlippageFromLevels(shortOB.Bids, adaptiveMinSize, shortMid)
			bottleneck := opp.LongExchange
			bottleneckBps := minLong
			if minShort > minLong {
				bottleneck = opp.ShortExchange
				bottleneckBps = minShort
			}
			return &models.RiskApproval{Approved: false, Reason: fmt.Sprintf(
				"slippage %.1f bps at minimum size %.0f ($%.2f) on %s (limit=%.1f bps)",
				bottleneckBps, adaptiveMinSize, adaptiveMinSize*midPrice/float64(leverage), bottleneck, m.cfg.SlippageBPS,
			)}, nil
		}

		originalNotional := size * midPrice / float64(leverage)
		newNotional := adaptedSize * midPrice / float64(leverage)
		m.log.Info("[adaptive] %s: size reduced %.0f → %.0f ($%.2f → $%.2f per leg, limit=%.1f bps)",
			opp.Symbol, size, adaptedSize, originalNotional, newNotional, m.cfg.SlippageBPS)
		size = adaptedSize
		requiredMarginPerLeg = (size * midPrice) / float64(leverage)
	}

	// e. Cross-exchange price gap check (VWAP across depth for actual trade size)
	longVWAP := VWAPFromLevels(longOB.Asks, size)
	shortVWAP := VWAPFromLevels(shortOB.Bids, size)
	gap := longVWAP - shortVWAP
	absGapBps := math.Abs(gap) / midPrice * 10000

	// Reject if absolute gap is extreme (prices on different level = not comparable).
	if absGapBps > m.cfg.MaxPriceGapBPS {
		return &models.RiskApproval{Approved: false, Reason: fmt.Sprintf("price gap too high: %.1f bps > %.1f bps hard cap (long=%.6f short=%.6f)", absGapBps, m.cfg.MaxPriceGapBPS, longVWAP, shortVWAP)}, nil
	}

	// For cost-based recovery check, only consider unfavorable gap (we pay more than receive).
	if gap < 0 {
		gap = 0
	}
	gapBps := gap / midPrice * 10000

	if gapBps > m.cfg.PriceGapFreeBPS {
		// opp.Spread is already bps/h — recovery is simply gap / spread_per_hour
		if opp.Spread <= 0 {
			return &models.RiskApproval{Approved: false, Reason: fmt.Sprintf("price gap %.1f bps with zero funding spread", gapBps)}, nil
		}
		recoveryHours := gapBps / opp.Spread
		intervalHours := opp.IntervalHours
		if intervalHours <= 0 {
			intervalHours = 8 // conservative default for unknown intervals
		}
		maxRecoveryHours := m.cfg.MaxGapRecoveryIntervals * intervalHours
		recoveryIntervals := recoveryHours / intervalHours
		if recoveryHours > maxRecoveryHours {
			return &models.RiskApproval{Approved: false, Reason: fmt.Sprintf("price gap %.1f bps, recovery %.1f intervals > %.1f limit (%.0fh interval)", gapBps, recoveryIntervals, m.cfg.MaxGapRecoveryIntervals, intervalHours)}, nil
		}
		m.log.Info("price gap %.1f bps for %s, recovery %.1f intervals (within %.1f limit, %.0fh interval)", gapBps, opp.Symbol, recoveryIntervals, m.cfg.MaxGapRecoveryIntervals, intervalHours)
	}

	// f. Per-exchange capital exposure cap
	// Ensure that the proposed position doesn't push any single exchange
	// beyond 60% of total capital deployed across all exchanges.
	totalCapital := longBal.Total + shortBal.Total
	if totalCapital > 0 {
		// Sum existing notional exposure per exchange from active positions.
		longExposure := requiredMarginPerLeg
		shortExposure := requiredMarginPerLeg
		for _, pos := range active {
			if pos.LongExchange == opp.LongExchange {
				longExposure += (pos.LongSize * pos.LongEntry) / float64(leverage)
			}
			if pos.ShortExchange == opp.LongExchange {
				longExposure += (pos.ShortSize * pos.ShortEntry) / float64(leverage)
			}
			if pos.LongExchange == opp.ShortExchange {
				shortExposure += (pos.LongSize * pos.LongEntry) / float64(leverage)
			}
			if pos.ShortExchange == opp.ShortExchange {
				shortExposure += (pos.ShortSize * pos.ShortEntry) / float64(leverage)
			}
		}

		maxExposure := MaxExposurePerExchange(totalCapital)
		if longExposure > maxExposure {
			return &models.RiskApproval{Approved: false, Reason: fmt.Sprintf("would exceed %.0f%% capital cap on %s: exposure=%.2f cap=%.2f", maxExposurePct*100, opp.LongExchange, longExposure, maxExposure)}, nil
		}
		if shortExposure > maxExposure {
			return &models.RiskApproval{Approved: false, Reason: fmt.Sprintf("would exceed %.0f%% capital cap on %s: exposure=%.2f cap=%.2f", maxExposurePct*100, opp.ShortExchange, shortExposure, maxExposure)}, nil
		}
	}

	// g. Exchange concentration — soft warning only
	if IsExchangeOverexposed(opp.LongExchange, active) {
		m.log.Warn("exchange %s has >60%% of active positions (long leg)", opp.LongExchange)
	}
	if IsExchangeOverexposed(opp.ShortExchange, active) {
		m.log.Warn("exchange %s has >60%% of active positions (short leg)", opp.ShortExchange)
	}

	_ = remainingSlots

	m.log.Info("approved %s: size=%.6f price=%.2f spread=%.2f bps/h gapBps=%.1f", opp.Symbol, size, midPrice, opp.Spread, gapBps)
	return &models.RiskApproval{
		Approved: true,
		Size:     size,
		Reason:   "all pre-trade checks passed",
		Price:    midPrice,
		GapBPS:   gapBps,
	}, nil
}

// CalculateSize determines the position size in base asset for a given opportunity.
func (m *Manager) CalculateSize(opp models.Opportunity, balances map[string]float64) float64 {
	balA := balances[opp.LongExchange]
	balB := balances[opp.ShortExchange]
	availableCapital := math.Min(balA, balB)

	m.log.Info("[sizing] %s: %s=%.4f %s=%.4f availCap=%.4f capPerLeg=%.2f lev=%d",
		opp.Symbol, opp.LongExchange, balA, opp.ShortExchange, balB,
		availableCapital, m.cfg.CapitalPerLeg, m.cfg.Leverage)

	if availableCapital <= 0 {
		m.log.Info("[sizing] %s: rejected — zero available capital", opp.Symbol)
		return 0
	}

	leverage := m.cfg.Leverage
	if leverage > MaxLeverage() {
		leverage = MaxLeverage()
	}

	// Count remaining position slots
	active, err := m.db.GetActivePositions()
	if err != nil {
		m.log.Error("failed to get active positions for sizing: %v", err)
		return 0
	}
	remainingSlots := m.cfg.MaxPositions - len(active)
	if remainingSlots <= 0 {
		return 0
	}

	// Determine position value per leg.
	var maxPositionValue float64
	if m.cfg.CapitalPerLeg > 0 {
		// Fixed capital per leg — use configured amount × leverage.
		maxPositionValue = m.cfg.CapitalPerLeg * float64(leverage)
		// Clamp to available capital so we never exceed what the exchange allows.
		maxFromBalance := availableCapital * float64(leverage)
		if maxPositionValue > maxFromBalance {
			maxPositionValue = maxFromBalance
		}
	} else {
		// Auto: Capital / (num_positions * leverage * 2_exchanges).
		// availableCapital is already min(balA, balB) — i.e. per-exchange.
		// We further halve to keep a safety buffer (never deploy >50% of the
		// limiting exchange's capital per slot) so price moves don't immediately
		// threaten margin.
		maxPositionValue = (availableCapital * float64(leverage)) / (float64(remainingSlots) * 2.0)
	}

	// Get a reference price — use the long exchange orderbook
	longExch, ok := m.exchanges[opp.LongExchange]
	if !ok {
		return 0
	}
	ob, err := longExch.GetOrderbook(opp.Symbol, 5)
	if err != nil || len(ob.Bids) == 0 || len(ob.Asks) == 0 {
		return 0
	}
	currentPrice := (ob.Bids[0].Price + ob.Asks[0].Price) / 2.0
	if currentPrice <= 0 {
		return 0
	}

	sizeInBase := maxPositionValue / currentPrice

	m.log.Info("[sizing] %s: maxPosVal=%.4f price=%.6f rawSize=%.6f",
		opp.Symbol, maxPositionValue, currentPrice, sizeInBase)

	// Round down to contract step size
	contracts, err := longExch.LoadAllContracts()
	if err == nil {
		if info, found := contracts[opp.Symbol]; found && info.StepSize > 0 {
			beforeRound := sizeInBase
			sizeInBase = utils.RoundToStep(sizeInBase, info.StepSize)
			m.log.Info("[sizing] %s: stepSize=%.6f minSize=%.6f before=%.6f after=%.6f",
				opp.Symbol, info.StepSize, info.MinSize, beforeRound, sizeInBase)
			// Enforce minimum size
			if sizeInBase < info.MinSize {
				m.log.Info("[sizing] %s: rejected — size %.6f < minSize %.6f", opp.Symbol, sizeInBase, info.MinSize)
				return 0
			}
		}
	}

	// Enforce minimum capital per leg
	minCapital := MinCapitalPerLeg(leverage)
	marginPerLeg := sizeInBase * currentPrice / float64(leverage)
	if marginPerLeg < minCapital {
		m.log.Info("[sizing] %s: rejected — marginPerLeg=%.4f < minCapital=%.4f", opp.Symbol, marginPerLeg, minCapital)
		return 0
	}

	return sizeInBase
}

// ensureFuturesBalance checks if the futures balance is below the needed amount
// and transfers from spot if possible. Only effective on split-account exchanges
// (Binance, Bitget, Gate.io); no-op on unified accounts (OKX, Bybit).
func (m *Manager) ensureFuturesBalance(exchName string, exc exchange.Exchange, futBal *exchange.Balance, needed float64) {
	if futBal.Available >= needed {
		return
	}
	deficit := needed - futBal.Available
	if deficit < 0.01 {
		return // negligible deficit, skip transfer
	}

	spotBal, err := exc.GetSpotBalance()
	if err != nil {
		m.log.Warn("ensureFuturesBalance %s: failed to get spot balance: %v", exchName, err)
		return
	}
	if spotBal.Available <= 0 {
		return
	}

	transferAmt := deficit
	if transferAmt > spotBal.Available {
		transferAmt = spotBal.Available
	}

	amtStr := fmt.Sprintf("%.4f", transferAmt)
	m.log.Info("auto-transfer %s USDT spot→futures on %s (futures=%.2f, needed=%.2f)", amtStr, exchName, futBal.Available, needed)
	if err := exc.TransferToFutures("USDT", amtStr); err != nil {
		m.log.Error("auto-transfer %s failed: %v", exchName, err)
	}
}

// estimateSlippageFromLevels converts exchange.PriceLevel to the format expected
// by utils.EstimateSlippage and returns slippage in bps.
// VWAPFromLevels computes the volume-weighted average price to fill `size`
// units across the given orderbook levels. Falls back to the best level price
// if depth is insufficient (slippage check already catches that case).
func VWAPFromLevels(levels []exchange.PriceLevel, size float64) float64 {
	if len(levels) == 0 {
		return 0
	}
	remaining := size
	weighted := 0.0
	filled := 0.0
	for _, l := range levels {
		fill := math.Min(remaining, l.Quantity)
		weighted += fill * l.Price
		filled += fill
		remaining -= fill
		if remaining <= 0 {
			break
		}
	}
	if filled <= 0 {
		return levels[0].Price
	}
	return weighted / filled
}

func estimateSlippageFromLevels(levels []exchange.PriceLevel, size, midPrice float64) float64 {
	if len(levels) == 0 || midPrice <= 0 {
		return math.MaxFloat64
	}
	converted := make([]struct{ Price, Qty float64 }, len(levels))
	for i, l := range levels {
		converted[i] = struct{ Price, Qty float64 }{Price: l.Price, Qty: l.Quantity}
	}
	return utils.EstimateSlippage(converted, size, midPrice)
}

// findMaxSizeForSlippage binary-searches for the largest position size that
// keeps slippage within the limit on both exchanges. Returns 0 if even minSize
// exceeds the limit.
func findMaxSizeForSlippage(
	askLevels, bidLevels []exchange.PriceLevel,
	longMid, shortMid float64,
	maxSize, minSize float64,
	slippageLimit float64,
	stepSize float64,
) float64 {
	// Fast path: original size fits
	longSlip := estimateSlippageFromLevels(askLevels, maxSize, longMid)
	shortSlip := estimateSlippageFromLevels(bidLevels, maxSize, shortMid)
	if longSlip <= slippageLimit && shortSlip <= slippageLimit {
		return maxSize
	}

	// Floor check: even minimum is too much
	longSlip = estimateSlippageFromLevels(askLevels, minSize, longMid)
	shortSlip = estimateSlippageFromLevels(bidLevels, minSize, shortMid)
	if longSlip > slippageLimit || shortSlip > slippageLimit {
		return 0
	}

	// Binary search: lo always passes, hi always fails or untested
	lo, hi := minSize, maxSize
	for i := 0; i < 20 && hi-lo > stepSize; i++ {
		mid := utils.RoundToStep((lo+hi)/2, stepSize)
		if mid <= lo {
			mid = lo + stepSize
		}
		if mid >= hi {
			break
		}

		worst := math.Max(
			estimateSlippageFromLevels(askLevels, mid, longMid),
			estimateSlippageFromLevels(bidLevels, mid, shortMid),
		)
		if worst <= slippageLimit {
			lo = mid
		} else {
			hi = mid
		}
	}
	return utils.RoundToStep(lo, stepSize)
}
