package risk

import (
	"fmt"
	"math"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// Compile-time check: *Manager satisfies models.RiskChecker.
var _ models.RiskChecker = (*Manager)(nil)

// PrefetchCache holds pre-fetched exchange data (balances, orderbooks) to avoid
// redundant API calls when approving multiple opportunities in a batch.
type PrefetchCache struct {
	Balances               map[string]*exchange.Balance   // exchange name -> futures balance
	Orderbooks             map[string]*exchange.Orderbook // "exchange:symbol" -> orderbook
	TransferablePerExchange map[string]float64             // exchange name -> max USDT receivable from other exchanges
	ActivePositions         []*models.ArbitragePosition    // pre-fetched active positions (avoid repeated DB calls in batch)
}

// Manager performs pre-trade risk assessment before entering positions.
// It satisfies models.RiskChecker.
type Manager struct {
	exchanges       map[string]exchange.Exchange
	db              *database.Client
	cfg             *config.Config
	log             *utils.Logger
	allocator       *CapitalAllocator
	spreadStability *SpreadStabilityChecker
	scorer          *ExchangeScorer
}

// NewManager creates a new risk Manager.
func NewManager(exchanges map[string]exchange.Exchange, db *database.Client, cfg *config.Config, allocator *CapitalAllocator) *Manager {
	return &Manager{
		exchanges:       exchanges,
		db:              db,
		cfg:             cfg,
		log:             utils.NewLogger("risk"),
		allocator:       allocator,
		spreadStability: NewSpreadStabilityChecker(db, cfg),
	}
}

// effectiveCapitalPerLeg returns the USDT per leg using the allocator's derived
// value when unified capital is enabled, falling back to cfg.CapitalPerLeg.
// When strategy is non-empty, the per-strategy dynamic cap is applied.
func (m *Manager) effectiveCapitalPerLeg(strategy ...string) float64 {
	if m.allocator != nil {
		if ecl := m.allocator.EffectiveCapitalPerLeg(strategy...); ecl > 0 {
			return ecl
		}
	}
	return m.cfg.CapitalPerLeg
}

func (m *Manager) SetSpreadHistoryProvider(provider func(models.Opportunity, int, time.Duration) []database.SpreadHistoryPoint) {
	if m == nil || m.spreadStability == nil {
		return
	}
	m.spreadStability.SetHistoryProvider(provider)
}

func (m *Manager) SetExchangeScorer(scorer *ExchangeScorer) {
	if m == nil {
		return
	}
	m.scorer = scorer
}

// Approve runs all pre-trade risk checks and returns an Approval decision.
func (m *Manager) Approve(opp models.Opportunity) (*models.RiskApproval, error) {
	return m.approveInternal(opp, nil, false, nil)
}

// ApproveWithReserved is like Approve but subtracts already-reserved margin
// from available balances before checking sufficiency.
func (m *Manager) ApproveWithReserved(opp models.Opportunity, reserved map[string]float64) (*models.RiskApproval, error) {
	return m.approveInternal(opp, reserved, false, nil)
}

// ApproveWithReservedCached is like ApproveWithReserved but uses pre-fetched
// balance and orderbook data to avoid redundant API calls in batch approval loops.
// If a needed value is missing from the cache, it falls back to a live API call.
func (m *Manager) ApproveWithReservedCached(opp models.Opportunity, reserved map[string]float64, cache *PrefetchCache) (*models.RiskApproval, error) {
	return m.approveInternal(opp, reserved, false, cache)
}

// SimulateApproval runs the full approval checks without side effects.
// It skips ensureFuturesBalance (no spot→futures transfer) and lock acquisition.
// Used by V2 rebalance to predict which opps would pass real approval.
func (m *Manager) SimulateApproval(opp models.Opportunity, reserved map[string]float64) (*models.RiskApproval, error) {
	return m.approveInternal(opp, reserved, true, nil)
}

// SimulateApprovalForPair is a side-effect-free approval simulation for a
// specific long/short pair derived from an existing opportunity.
// alt, when non-nil, overrides the rate/spread/cost fields so that downstream
// checks (gap recovery, spread stability) use the alternative pair's values
// instead of the primary pair's.
func (m *Manager) SimulateApprovalForPair(opp models.Opportunity, longExch, shortExch string, reserved map[string]float64, alt *models.AlternativePair, cache *PrefetchCache) (*models.RiskApproval, error) {
	pairOpp := opp
	pairOpp.LongExchange = longExch
	pairOpp.ShortExchange = shortExch
	if alt != nil {
		pairOpp.LongRate = alt.LongRate
		pairOpp.ShortRate = alt.ShortRate
		pairOpp.Spread = alt.Spread
		pairOpp.CostRatio = alt.CostRatio
		pairOpp.Score = alt.Score
		pairOpp.IntervalHours = alt.IntervalHours
	}
	return m.approveInternal(pairOpp, reserved, true, cache)
}

// ApproveRotation performs margin, health, and exposure checks for a rotation.
// It skips position-count and spread checks (rotation doesn't add positions
// and has its own threshold logic).
func (m *Manager) ApproveRotation(exchName string, symbol string, size float64, refPrice float64) (bool, string) {
	exch, ok := m.exchanges[exchName]
	if !ok {
		return false, fmt.Sprintf("exchange %s not configured", exchName)
	}

	bal, err := exch.GetFuturesBalance()
	if err != nil {
		return false, fmt.Sprintf("get balance on %s: %v", exchName, err)
	}

	// Leverage clamp (same as approveInternal)
	leverage := m.cfg.Leverage
	if leverage > MaxLeverage() {
		leverage = MaxLeverage()
	}

	requiredMargin := (size * refPrice) / float64(leverage)
	safetyMultiplier := m.cfg.MarginSafetyMultiplier
	buffered := requiredMargin * safetyMultiplier

	if bal.Available < buffered {
		return false, fmt.Sprintf("insufficient margin on %s: need %.2f (%.0f%% buffer), have %.2f",
			exchName, buffered, (safetyMultiplier-1)*100, bal.Available)
	}

	// Projected margin ratio (with bal.Total > 0 guard)
	if bal.Total > 0 {
		projectedAvail := bal.Available - requiredMargin
		if projectedAvail < 0 {
			projectedAvail = 0
		}
		projectedRatio := 1 - projectedAvail/bal.Total
		if projectedRatio >= m.cfg.MarginL4Threshold {
			return false, fmt.Sprintf("post-rotation margin ratio %.2f on %s would reach L4 (%.2f)",
				projectedRatio, exchName, m.cfg.MarginL4Threshold)
		}
	}

	// Exchange health scoring
	if m.cfg.EnableExchangeHealthScoring && m.scorer != nil {
		snapshot := m.scorer.Snapshot(exchName)
		if snapshot.Score < m.scorer.minScore() {
			return false, fmt.Sprintf("exchange health too low on %s: %.2f < %.2f",
				exchName, snapshot.Score, m.scorer.minScore())
		}
	}

	// Per-exchange exposure cap (rotation is open-first, so target temporarily holds both)
	if m.allocator == nil || !m.allocator.Enabled() {
		active, err := m.db.GetActivePositions()
		if err != nil {
			return false, fmt.Sprintf("get active positions for exposure check: %v", err)
		}
		{
			totalCapital := bal.Total
			// Sum existing exposure on target exchange from all active positions
			exposure := requiredMargin // the new rotation leg
			for _, pos := range active {
				if pos.LongExchange == exchName {
					exposure += (pos.LongSize * pos.LongEntry) / float64(leverage)
				}
				if pos.ShortExchange == exchName {
					exposure += (pos.ShortSize * pos.ShortEntry) / float64(leverage)
				}
			}
			if totalCapital > 0 {
				maxExposure := MaxExposurePerExchange(totalCapital)
				if exposure > maxExposure {
					return false, fmt.Sprintf("rotation would exceed exposure cap on %s: %.2f > %.2f",
						exchName, exposure, maxExposure)
				}
			}
		}
	}

	return true, ""
}

// notionalRejectionKind returns RejectionKindConfig when the per-leg capital
// ceiling is so low that no transfer can raise sizing above the $10 notional floor,
// and RejectionKindCapital otherwise (transfer top-up may cure the rejection).
func notionalRejectionKind(effectiveCap float64, leverage int) models.RejectionKind {
	if effectiveCap > 0 && effectiveCap*float64(leverage) < 10 {
		return models.RejectionKindConfig
	}
	return models.RejectionKindCapital
}

// approveInternal is the shared implementation for Approve and ApproveWithReserved.
// When cache is non-nil, pre-fetched balances and orderbooks are used instead of
// live API calls. Cache misses fall back to live calls transparently.
func (m *Manager) approveInternal(opp models.Opportunity, reserved map[string]float64, dryRun bool, cache *PrefetchCache) (*models.RiskApproval, error) {
	// a. Position count check — use cached active positions if available to avoid
	// repeated Redis calls when approving a batch of opportunities.
	var active []*models.ArbitragePosition
	if cache != nil && cache.ActivePositions != nil {
		active = cache.ActivePositions
	} else {
		var err error
		active, err = m.db.GetActivePositions()
		if err != nil {
			return nil, fmt.Errorf("get active positions: %w", err)
		}
		// Store back into cache for subsequent calls in the same batch.
		if cache != nil {
			cache.ActivePositions = active
		}
	}
	if len(active) >= m.cfg.MaxPositions {
		reason := fmt.Sprintf("max positions reached (%d/%d)", len(active), m.cfg.MaxPositions)
		m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
		return &models.RiskApproval{Approved: false, Kind: models.RejectionKindConfig, Reason: reason}, nil
	}

	longExch, ok := m.exchanges[opp.LongExchange]
	if !ok {
		reason := fmt.Sprintf("long exchange %s not configured", opp.LongExchange)
		m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
		return &models.RiskApproval{Approved: false, Kind: models.RejectionKindConfig, Reason: reason}, nil
	}
	shortExch, ok := m.exchanges[opp.ShortExchange]
	if !ok {
		reason := fmt.Sprintf("short exchange %s not configured", opp.ShortExchange)
		m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
		return &models.RiskApproval{Approved: false, Kind: models.RejectionKindConfig, Reason: reason}, nil
	}

	// b. Capital check — get balances from both exchanges (use cache if available).
	// If futures balance is insufficient and spot has funds, auto-transfer.
	var longBal, shortBal *exchange.Balance
	if cache != nil && cache.Balances != nil {
		longBal = cache.Balances[opp.LongExchange]
		shortBal = cache.Balances[opp.ShortExchange]
	}
	if longBal == nil {
		var err2 error
		longBal, err2 = longExch.GetFuturesBalance()
		if err2 != nil {
			return nil, fmt.Errorf("get balance from %s: %w", opp.LongExchange, err2)
		}
	}
	if shortBal == nil {
		var err2 error
		shortBal, err2 = shortExch.GetFuturesBalance()
		if err2 != nil {
			return nil, fmt.Errorf("get balance from %s: %w", opp.ShortExchange, err2)
		}
	}

	// Single-snapshot effective capital — reused at sizing and notional-floor
	// classification to eliminate concurrent dynamicCaps mutation window.
	// Clamped leverage is computed below (after the balance fetch) as the existing
	// `leverage` local at the leverage-check block, and threaded into sizing there.
	effectiveCap := m.effectiveCapitalPerLeg(string(StrategyPerpPerp))

	needed := effectiveCap
	if !dryRun {
		const sweepTarget = 1e9
		if needed > 0 {
			// Fixed-capital mode still needs the full split-account sweep; otherwise
			// post-trade L4 uses an artificially low futures total and can reject a
			// trade that already has idle spot available.
			m.ensureFuturesBalance(opp.LongExchange, longExch, longBal, sweepTarget)
			m.ensureFuturesBalance(opp.ShortExchange, shortExch, shortBal, sweepTarget)
		} else {
			m.ensureFuturesBalance(opp.LongExchange, longExch, longBal, sweepTarget)
			m.ensureFuturesBalance(opp.ShortExchange, shortExch, shortBal, sweepTarget)
		}
		// Re-fetch after potential transfer; keep old balance on error.
		// Always re-fetch live here since ensureFuturesBalance has side effects.
		if lb, err := longExch.GetFuturesBalance(); err == nil {
			longBal = lb
			// Update cache so subsequent approvals in the batch see the fresh balance.
			if cache != nil && cache.Balances != nil {
				cache.Balances[opp.LongExchange] = lb
			}
		}
		if sb, err := shortExch.GetFuturesBalance(); err == nil {
			shortBal = sb
			if cache != nil && cache.Balances != nil {
				cache.Balances[opp.ShortExchange] = sb
			}
		}
	}

	// In simulate mode, account for spot balance and cross-exchange transferable
	// funds. We clone the balance structs first to avoid cumulative mutations
	// when the same exchange appears in multiple candidate evaluations.
	if dryRun && needed > 0 {
		longClone := *longBal
		shortClone := *shortBal
		longBal = &longClone
		shortBal = &shortClone
		for _, pair := range []struct {
			name string
			exch exchange.Exchange
			bal  *exchange.Balance
		}{
			{opp.LongExchange, longExch, longBal},
			{opp.ShortExchange, shortExch, shortBal},
		} {
			bufferedNeed := needed * m.cfg.MarginSafetyMultiplier
			// For split-account exchanges, unconditionally add spot balance so
			// dryRun sees the same avail as rebalanceAvailable (futures+spot).
			// Same-exchange long+short is structurally impossible in perp-perp,
			// so per-leg spot augmentation cannot double-count.
			isUnified := false
			if uc, ok := pair.exch.(interface{ IsUnified() bool }); ok && uc.IsUnified() {
				isUnified = true
			}
			if !isUnified {
				if spotBal, err := pair.exch.GetSpotBalance(); err == nil && spotBal.Available > 0 {
					pair.bal.Available += spotBal.Available
					pair.bal.Total += spotBal.Available
					m.log.Debug("approval dryRun %s: added spot %.2f to %s (avail now=%.2f)", opp.Symbol, spotBal.Available, pair.name, pair.bal.Available)
				}
			}
			// Add cross-exchange transferable surplus (allocator planning only).
			// Trigger when EITHER margin buffer is insufficient OR post-trade
			// ratio would exceed L4. Only inflate by the deficit needed.
			if cache != nil && cache.TransferablePerExchange != nil {
				// Check if post-trade ratio would exceed L4 even with sufficient margin buffer.
				margin := needed
				wouldExceedL4 := false
				if pair.bal.Total > 0 {
					postAvail := pair.bal.Available - margin
					if postAvail < 0 {
						postAvail = 0
					}
					postRatio := 1 - postAvail/pair.bal.Total
					wouldExceedL4 = postRatio >= m.cfg.MarginL4Threshold
				}

				if pair.bal.Available < bufferedNeed || wouldExceedL4 {
					if topUp, ok := cache.TransferablePerExchange[pair.name]; ok && topUp > 0 {
						// Compute margin buffer deficit.
						deficit := bufferedNeed - pair.bal.Available
						if deficit < 0 {
							deficit = 0
						}

						// Also compute the post-trade ratio deficit: how much extra
						// is needed so that post-trade ratio stays below L4.
						targetRatio := m.cfg.MarginL4Threshold - 0.005 // marginEpsilon: avoid hitting L4 boundary
						freeTarget := 1.0 - targetRatio
						if freeTarget > 0 && pair.bal.Total > 0 {
							ratioDeficit := (freeTarget*pair.bal.Total - pair.bal.Available + margin) / targetRatio
							if ratioDeficit > deficit {
								deficit = ratioDeficit
							}
						}

						actualTopUp := deficit
						if actualTopUp > topUp {
							actualTopUp = topUp
						}
						if actualTopUp > 0 {
							pair.bal.Available += actualTopUp
							pair.bal.Total += actualTopUp
						}
					}
				}
			}
		}
	}

	// Subtract already-reserved margin from prior approvals in the same batch.
	effectiveLongAvail := longBal.Available
	effectiveShortAvail := shortBal.Available
	if reserved != nil {
		effectiveLongAvail -= reserved[opp.LongExchange]
		effectiveShortAvail -= reserved[opp.ShortExchange]
		if effectiveLongAvail < 0 {
			effectiveLongAvail = 0
		}
		if effectiveShortAvail < 0 {
			effectiveShortAvail = 0
		}
	}

	balances := map[string]float64{
		opp.LongExchange:  effectiveLongAvail,
		opp.ShortExchange: effectiveShortAvail,
	}

	// c. Leverage check
	leverage := m.cfg.Leverage
	if leverage > MaxLeverage() {
		leverage = MaxLeverage()
		m.log.Warn("configured leverage %d exceeds hard cap, clamped to %d", m.cfg.Leverage, leverage)
	}

	// Get a reference price from the long exchange orderbook (use cache if available).
	var longOB *exchange.Orderbook
	if cache != nil && cache.Orderbooks != nil {
		if cached := cache.Orderbooks[opp.LongExchange+":"+opp.Symbol]; cached != nil {
			if cached.Time.IsZero() || time.Since(cached.Time) < 5*time.Second {
				longOB = cached
			}
			// else: stale, fall through to live fetch
		}
	}
	if longOB == nil {
		var err2 error
		longOB, err2 = longExch.GetOrderbook(opp.Symbol, 20)
		if err2 != nil {
			return nil, fmt.Errorf("get orderbook from %s: %w", opp.LongExchange, err2)
		}
	}
	if len(longOB.Asks) == 0 || len(longOB.Bids) == 0 {
		m.log.Debug("approval rejected %s: empty orderbook on %s", opp.Symbol, opp.LongExchange)
		return &models.RiskApproval{Approved: false, Kind: models.RejectionKindMarket, Reason: "empty orderbook on long exchange"}, nil
	}
	longMid := (longOB.Bids[0].Price + longOB.Asks[0].Price) / 2.0
	midPrice := longMid // used as reference for sizing (preserves existing behavior)

	// Calculate position size (pass midPrice to avoid redundant orderbook fetch).
	size := m.calculateSizeWithPrice(opp, balances, midPrice, active, effectiveCap, leverage)
	if size <= 0 {
		m.log.Debug("approval rejected %s: insufficient capital for minimum position size (long=%s=%.2f short=%s=%.2f)", opp.Symbol, opp.LongExchange, effectiveLongAvail, opp.ShortExchange, effectiveShortAvail)
		return &models.RiskApproval{Approved: false, Kind: models.RejectionKindCapital, Reason: "insufficient capital for minimum position size"}, nil
	}

	// $10 per-leg notional floor (4.1): reject if either leg's notional < $10
	longNotional := size * longMid
	if longNotional < 10.0 {
		reason := fmt.Sprintf("long-leg notional $%.2f < $10 minimum on %s", longNotional, opp.LongExchange)
		m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
		return &models.RiskApproval{Approved: false, Kind: notionalRejectionKind(effectiveCap, leverage), Reason: reason}, nil
	}

	// Verify enough free margin per leg (with configurable safety buffer).
	// Use per-leg mid prices so each exchange's margin is computed from its own orderbook.
	safetyMultiplier := m.cfg.MarginSafetyMultiplier
	safetyPct := (safetyMultiplier - 1) * 100
	longMarginPerLeg := (size * longMid) / float64(leverage)
	longMarginWithBuffer := longMarginPerLeg * safetyMultiplier
	if effectiveLongAvail < longMarginWithBuffer {
		reason := fmt.Sprintf("insufficient margin buffer on %s: need %.2f (including %.0f%% safety buffer), have %.2f", opp.LongExchange, longMarginWithBuffer, safetyPct, effectiveLongAvail)
		m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
		return &models.RiskApproval{Approved: false, Kind: models.RejectionKindCapital, Reason: reason}, nil
	}

	// Short-leg margin check is deferred until after shortOB is fetched (see below).

	// Post-trade margin ratio projection (long leg — short leg checked after shortOB fetch)
	if longBal.Total > 0 {
		projectedAvail := effectiveLongAvail - longMarginPerLeg
		if projectedAvail < 0 {
			projectedAvail = 0
		}
		projectedRatio := 1 - projectedAvail/longBal.Total
		if projectedRatio >= m.cfg.MarginL4Threshold {
			reason := fmt.Sprintf("post-trade margin ratio would reach %.2f on %s (L4 threshold: %.2f)", projectedRatio, opp.LongExchange, m.cfg.MarginL4Threshold)
			m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
			return &models.RiskApproval{Approved: false, Kind: models.RejectionKindCapital, Reason: reason}, nil
		}
	}

	// d. Orderbook depth / slippage check on both exchanges (use cache if available).
	var shortOB *exchange.Orderbook
	if cache != nil && cache.Orderbooks != nil {
		if cached := cache.Orderbooks[opp.ShortExchange+":"+opp.Symbol]; cached != nil {
			if cached.Time.IsZero() || time.Since(cached.Time) < 5*time.Second {
				shortOB = cached
			}
			// else: stale, fall through to live fetch
		}
	}
	if shortOB == nil {
		var err2 error
		shortOB, err2 = shortExch.GetOrderbook(opp.Symbol, 20)
		if err2 != nil {
			return nil, fmt.Errorf("get orderbook from %s: %w", opp.ShortExchange, err2)
		}
	}

	if len(shortOB.Asks) == 0 || len(shortOB.Bids) == 0 {
		reason := fmt.Sprintf("empty orderbook on %s", opp.ShortExchange)
		m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
		return &models.RiskApproval{Approved: false, Kind: models.RejectionKindMarket, Reason: reason}, nil
	}

	// Compute short mid price from short exchange orderbook.
	shortMid := longMid
	if len(shortOB.Bids) > 0 && len(shortOB.Asks) > 0 {
		shortMid = (shortOB.Bids[0].Price + shortOB.Asks[0].Price) / 2.0
	}
	// $10 per-leg notional floor (4.1): check short leg
	shortNotional := size * shortMid
	if shortNotional < 10.0 {
		reason := fmt.Sprintf("short-leg notional $%.2f < $10 minimum on %s", shortNotional, opp.ShortExchange)
		m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
		return &models.RiskApproval{Approved: false, Kind: notionalRejectionKind(effectiveCap, leverage), Reason: reason}, nil
	}

	// Short-leg margin check (deferred from above — needed shortMid)
	shortMarginPerLeg := (size * shortMid) / float64(leverage)
	shortMarginWithBuffer := shortMarginPerLeg * safetyMultiplier
	if effectiveShortAvail < shortMarginWithBuffer {
		reason := fmt.Sprintf("insufficient margin buffer on %s: need %.2f (including %.0f%% safety buffer), have %.2f", opp.ShortExchange, shortMarginWithBuffer, safetyPct, effectiveShortAvail)
		m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
		return &models.RiskApproval{Approved: false, Kind: models.RejectionKindCapital, Reason: reason}, nil
	}

	// Post-trade margin ratio projection (short leg)
	if shortBal.Total > 0 {
		projectedAvail := effectiveShortAvail - shortMarginPerLeg
		if projectedAvail < 0 {
			projectedAvail = 0
		}
		projectedRatio := 1 - projectedAvail/shortBal.Total
		if projectedRatio >= m.cfg.MarginL4Threshold {
			reason := fmt.Sprintf("post-trade margin ratio would reach %.2f on %s (L4 threshold: %.2f)", projectedRatio, opp.ShortExchange, m.cfg.MarginL4Threshold)
			m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
			return &models.RiskApproval{Approved: false, Kind: models.RejectionKindCapital, Reason: reason}, nil
		}
	}

	// Estimate slippage on both legs at full size.
	longSlippage := estimateSlippageFromLevels(longOB.Asks, size, longMid)
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
			reason := fmt.Sprintf("slippage too high on %s: %.1f bps > %.1f bps limit", bottleneck, bps, m.cfg.SlippageBPS)
			m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
			return &models.RiskApproval{Approved: false, Kind: models.RejectionKindSpread, Reason: reason}, nil
		}

		// Min size = contract minimum from exchange
		adaptiveMinSize := minSizeContract

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
			reason := fmt.Sprintf("slippage %.1f bps at minimum size %.0f ($%.2f) on %s (limit=%.1f bps)",
				bottleneckBps, adaptiveMinSize, adaptiveMinSize*midPrice/float64(leverage), bottleneck, m.cfg.SlippageBPS)
			m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
			return &models.RiskApproval{Approved: false, Kind: models.RejectionKindSpread, Reason: reason}, nil
		}

		originalNotional := size * midPrice / float64(leverage)
		newNotional := adaptedSize * midPrice / float64(leverage)
		m.log.Info("[adaptive] %s: size reduced %.0f → %.0f ($%.2f → $%.2f per leg, limit=%.1f bps)",
			opp.Symbol, size, adaptedSize, originalNotional, newNotional, m.cfg.SlippageBPS)
		size = adaptedSize
		// Recompute per-leg margins after adaptive sizing
		longMarginPerLeg = (size * longMid) / float64(leverage)
		longMarginWithBuffer = longMarginPerLeg * safetyMultiplier
		shortMarginPerLeg = (size * shortMid) / float64(leverage)
		shortMarginWithBuffer = shortMarginPerLeg * safetyMultiplier
	}

	// e. Cross-exchange price gap check (VWAP across depth for actual trade size)
	longVWAP := VWAPFromLevels(longOB.Asks, size)
	shortVWAP := VWAPFromLevels(shortOB.Bids, size)
	gap := longVWAP - shortVWAP
	absGapBps := math.Abs(gap) / midPrice * 10000

	// Reject if absolute gap is extreme (prices on different level = not comparable).
	if absGapBps > m.cfg.MaxPriceGapBPS {
		reason := fmt.Sprintf("price gap too high: %.1f bps > %.1f bps hard cap (long=%.6f short=%.6f)", absGapBps, m.cfg.MaxPriceGapBPS, longVWAP, shortVWAP)
		m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
		return &models.RiskApproval{Approved: false, Kind: models.RejectionKindSpread, Reason: reason}, nil
	}

	// For cost-based recovery check, only consider unfavorable gap (we pay more than receive).
	if gap < 0 {
		gap = 0
	}
	gapBps := gap / midPrice * 10000

	if gapBps > m.cfg.PriceGapFreeBPS {
		// opp.Spread is already bps/h — recovery is simply gap / spread_per_hour
		if opp.Spread <= 0 {
			reason := fmt.Sprintf("price gap %.1f bps with zero funding spread", gapBps)
			m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
			return &models.RiskApproval{Approved: false, Kind: models.RejectionKindSpread, Reason: reason}, nil
		}
		recoveryHours := gapBps / opp.Spread
		intervalHours := opp.IntervalHours
		if intervalHours <= 0 {
			intervalHours = 8 // conservative default for unknown intervals
		}
		maxRecoveryHours := m.cfg.MaxGapRecoveryIntervals * intervalHours
		recoveryIntervals := recoveryHours / intervalHours
		if recoveryHours > maxRecoveryHours {
			reason := fmt.Sprintf("price gap %.1f bps, recovery %.1f intervals > %.1f limit (%.0fh interval)", gapBps, recoveryIntervals, m.cfg.MaxGapRecoveryIntervals, intervalHours)
			m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
			return &models.RiskApproval{Approved: false, Kind: models.RejectionKindSpread, Reason: reason}, nil
		}
		m.log.Info("price gap %.1f bps for %s, recovery %.1f intervals (within %.1f limit, %.0fh interval)", gapBps, opp.Symbol, recoveryIntervals, m.cfg.MaxGapRecoveryIntervals, intervalHours)
	}

	// f. Spread stability hard gate
	if reason, err := m.spreadStability.Check(opp, reserved != nil); err != nil {
		return nil, fmt.Errorf("spread stability check: %w", err)
	} else if reason != "" {
		m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
		return &models.RiskApproval{Approved: false, Kind: models.RejectionKindSpread, Reason: reason}, nil
	}

	if m.cfg.EnableExchangeHealthScoring && m.scorer != nil {
		longSnapshot := m.scorer.Snapshot(opp.LongExchange)
		if longSnapshot.Score < m.scorer.minScore() {
			reason := fmt.Sprintf("exchange health too low on %s: %.2f < %.2f", opp.LongExchange, longSnapshot.Score, m.scorer.minScore())
			m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
			return &models.RiskApproval{Approved: false, Kind: models.RejectionKindHealth, Reason: reason}, nil
		}
		shortSnapshot := m.scorer.Snapshot(opp.ShortExchange)
		if shortSnapshot.Score < m.scorer.minScore() {
			reason := fmt.Sprintf("exchange health too low on %s: %.2f < %.2f", opp.ShortExchange, shortSnapshot.Score, m.scorer.minScore())
			m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
			return &models.RiskApproval{Approved: false, Kind: models.RejectionKindHealth, Reason: reason}, nil
		}
	}

	// g. Per-exchange capital exposure cap.
	// When the cross-strategy allocator is enabled, entry budgeting is enforced
	// centrally at reservation time, so the legacy pair-local cap is skipped.
	if m.allocator == nil || !m.allocator.Enabled() {
		// Ensure that the proposed position doesn't push any single exchange
		// beyond 60% of total capital deployed across the candidate pair.
		totalCapital := longBal.Total + shortBal.Total
		if totalCapital > 0 {
			// Sum existing notional exposure per exchange from active positions.
			longExposure := longMarginPerLeg
			shortExposure := shortMarginPerLeg
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
				reason := fmt.Sprintf("would exceed %.0f%% capital cap on %s: exposure=%.2f cap=%.2f", maxExposurePct*100, opp.LongExchange, longExposure, maxExposure)
				m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
				return &models.RiskApproval{Approved: false, Kind: models.RejectionKindCapacity, Reason: reason}, nil
			}
			if shortExposure > maxExposure {
				reason := fmt.Sprintf("would exceed %.0f%% capital cap on %s: exposure=%.2f cap=%.2f", maxExposurePct*100, opp.ShortExchange, shortExposure, maxExposure)
				m.log.Debug("approval rejected %s: %s", opp.Symbol, reason)
				return &models.RiskApproval{Approved: false, Kind: models.RejectionKindCapacity, Reason: reason}, nil
			}
		}
	}

	// h. Exchange concentration — soft warning only
	if IsExchangeOverexposed(opp.LongExchange, active) {
		m.log.Warn("exchange %s has >60%% of active positions (long leg)", opp.LongExchange)
	}
	if IsExchangeOverexposed(opp.ShortExchange, active) {
		m.log.Warn("exchange %s has >60%% of active positions (short leg)", opp.ShortExchange)
	}

	requiredWithBuffer := math.Max(longMarginWithBuffer, shortMarginWithBuffer)
	m.log.Info("approved %s: size=%.6f longMid=%.2f shortMid=%.2f spread=%.2f bps/h gapBps=%.1f", opp.Symbol, size, longMid, shortMid, opp.Spread, gapBps)
	return &models.RiskApproval{
		Approved:          true,
		Size:              size,
		Reason:            "all pre-trade checks passed",
		Price:             midPrice,
		GapBPS:            gapBps,
		RequiredMargin:    requiredWithBuffer,
		LongMarginNeeded:  longMarginWithBuffer,
		ShortMarginNeeded: shortMarginWithBuffer,
	}, nil
}

// CalculateSize determines the position size in base asset for a given opportunity.
func (m *Manager) CalculateSize(opp models.Opportunity, balances map[string]float64) float64 {
	balA := balances[opp.LongExchange]
	balB := balances[opp.ShortExchange]
	availableCapital := math.Min(balA, balB)

	ecl := m.effectiveCapitalPerLeg(string(StrategyPerpPerp))
	m.log.Info("[sizing] %s: %s=%.4f %s=%.4f availCap=%.4f capPerLeg=%.2f lev=%d",
		opp.Symbol, opp.LongExchange, balA, opp.ShortExchange, balB,
		availableCapital, ecl, m.cfg.Leverage)

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
	if ecl > 0 {
		// Fixed capital per leg — use configured (or derived) amount × leverage.
		maxPositionValue = ecl * float64(leverage)
		// Clamp to available capital (minus safety buffer) so margin check won't reject.
		maxFromBalance := availableCapital * float64(leverage) / m.cfg.MarginSafetyMultiplier
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

	// Round down to contract step size — use the stricter (larger) of both
	// exchanges' StepSize and MinSize so the result is valid on both sides.
	var stepSize, minSizeContract float64
	contracts, err := longExch.LoadAllContracts()
	if err == nil {
		if info, found := contracts[opp.Symbol]; found && info.StepSize > 0 {
			stepSize = info.StepSize
			minSizeContract = info.MinSize
		}
	}
	shortExch, sok := m.exchanges[opp.ShortExchange]
	if sok {
		shortContracts, serr := shortExch.LoadAllContracts()
		if serr == nil {
			if sinfo, found := shortContracts[opp.Symbol]; found && sinfo.StepSize > 0 {
				if sinfo.StepSize > stepSize {
					stepSize = sinfo.StepSize
				}
				if sinfo.MinSize > minSizeContract {
					minSizeContract = sinfo.MinSize
				}
			}
		}
	}
	if stepSize > 0 {
		beforeRound := sizeInBase
		sizeInBase = utils.RoundToStep(sizeInBase, stepSize)
		m.log.Info("[sizing] %s: stepSize=%.6f minSize=%.6f before=%.6f after=%.6f",
			opp.Symbol, stepSize, minSizeContract, beforeRound, sizeInBase)
		// Enforce minimum size (stricter of both exchanges)
		if sizeInBase < minSizeContract {
			m.log.Info("[sizing] %s: rejected — size %.6f < minSize %.6f", opp.Symbol, sizeInBase, minSizeContract)
			return 0
		}
	}

	return sizeInBase
}

// calculateSizeWithPrice is like CalculateSize but uses a pre-computed reference
// price instead of fetching a separate orderbook. This avoids a redundant API
// call when the caller (approveInternal) already has an orderbook.
// When cachedActive is non-nil, it is used instead of querying the DB again.
func (m *Manager) calculateSizeWithPrice(opp models.Opportunity, balances map[string]float64, refPrice float64, cachedActive []*models.ArbitragePosition, effectiveCap float64, leverage int) float64 {
	balA := balances[opp.LongExchange]
	balB := balances[opp.ShortExchange]
	availableCapital := math.Min(balA, balB)

	ecl2 := effectiveCap
	m.log.Info("[sizing] %s: %s=%.4f %s=%.4f availCap=%.4f capPerLeg=%.2f lev=%d",
		opp.Symbol, opp.LongExchange, balA, opp.ShortExchange, balB,
		availableCapital, ecl2, m.cfg.Leverage)

	if availableCapital <= 0 {
		m.log.Info("[sizing] %s: rejected — zero available capital", opp.Symbol)
		return 0
	}

	active := cachedActive
	if active == nil {
		var err error
		active, err = m.db.GetActivePositions()
		if err != nil {
			m.log.Error("failed to get active positions for sizing: %v", err)
			return 0
		}
	}
	remainingSlots := m.cfg.MaxPositions - len(active)
	if remainingSlots <= 0 {
		return 0
	}

	var maxPositionValue float64
	if ecl2 > 0 {
		maxPositionValue = ecl2 * float64(leverage)
		maxFromBalance := availableCapital * float64(leverage) / m.cfg.MarginSafetyMultiplier
		if maxPositionValue > maxFromBalance {
			maxPositionValue = maxFromBalance
		}
	} else {
		maxPositionValue = (availableCapital * float64(leverage)) / (float64(remainingSlots) * 2.0)
	}
	m.log.Debug("[sizing] %s: ecl=%.2f maxPosVal=%.2f", opp.Symbol, ecl2, maxPositionValue)

	currentPrice := refPrice
	if currentPrice <= 0 {
		return 0
	}

	sizeInBase := maxPositionValue / currentPrice

	m.log.Info("[sizing] %s: maxPosVal=%.4f price=%.6f rawSize=%.6f",
		opp.Symbol, maxPositionValue, currentPrice, sizeInBase)

	// Round down to contract step size — use the stricter (larger) of both
	// exchanges' StepSize and MinSize so the result is valid on both sides.
	var stepSize, minSizeContract float64
	longExch, ok := m.exchanges[opp.LongExchange]
	if ok {
		contracts, cerr := longExch.LoadAllContracts()
		if cerr == nil {
			if info, found := contracts[opp.Symbol]; found && info.StepSize > 0 {
				stepSize = info.StepSize
				minSizeContract = info.MinSize
			}
		}
	}
	shortExch, sok := m.exchanges[opp.ShortExchange]
	if sok {
		shortContracts, serr := shortExch.LoadAllContracts()
		if serr == nil {
			if sinfo, found := shortContracts[opp.Symbol]; found && sinfo.StepSize > 0 {
				if sinfo.StepSize > stepSize {
					stepSize = sinfo.StepSize
				}
				if sinfo.MinSize > minSizeContract {
					minSizeContract = sinfo.MinSize
				}
			}
		}
	}
	if stepSize > 0 {
		beforeRound := sizeInBase
		sizeInBase = utils.RoundToStep(sizeInBase, stepSize)
		m.log.Info("[sizing] %s: stepSize=%.6f minSize=%.6f before=%.6f after=%.6f",
			opp.Symbol, stepSize, minSizeContract, beforeRound, sizeInBase)
		if sizeInBase < minSizeContract {
			m.log.Info("[sizing] %s: rejected — size %.6f < minSize %.6f", opp.Symbol, sizeInBase, minSizeContract)
			return 0
		}
	}

	return sizeInBase
}

// ensureFuturesBalance checks if the futures balance is below the needed amount
// and transfers from spot/funding if possible. Effective on all 6 exchanges:
// Binance (spot→futures), Bybit (FUND→UNIFIED), Gate.io (spot→futures or no-op
// if unified), Bitget (spot→usdt_futures), OKX (funding→trading), BingX (FUND→PFUTURES).
// GetSpotBalance returns the source bucket (spot/FUND/funding), not unified total.
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
	// Floor to 4 decimal places to prevent rounding above spotAvailable.
	transferAmt = math.Floor(transferAmt*10000) / 10000
	if transferAmt <= 0 {
		return // nothing to transfer, avoid API error
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
