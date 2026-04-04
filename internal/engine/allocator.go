package engine

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
)

type allocatorFeeSchedule struct {
	Taker float64
}

var allocatorExchangeFees = map[string]allocatorFeeSchedule{
	"binance": {Taker: 0.05 * 0.8},
	"bybit":   {Taker: 0.055 * 0.8},
	"okx":     {Taker: 0.05 * 0.8},
	"bitget":  {Taker: 0.06 * 0.8},
	"gateio":  {Taker: 0.05 * 0.8},
	"bingx":   {Taker: 0.05 * 0.8},
}

type rebalanceBalanceInfo struct {
	futures        float64
	spot           float64
	futuresTotal   float64
	marginRatio    float64
	maxTransferOut float64
	hasPositions   bool // true if this exchange has active perp-perp legs
}

type allocatorChoice struct {
	symbol         string
	longExchange   string
	shortExchange  string
	spreadBpsH     float64
	intervalHours  float64
	requiredMargin float64
	entryNotional  float64
	baseValue      float64
}

type allocatorCandidate struct {
	symbol  string
	choices []allocatorChoice
}

type allocatorSelection struct {
	choices        []allocatorChoice
	needs          map[string]float64
	totalBaseValue float64
	feasible       bool
}

func (e *Engine) runPoolAllocator(opps []models.Opportunity, balances map[string]rebalanceBalanceInfo, remainingSlots int) (*allocatorSelection, error) {
	candidates := e.buildAllocatorCandidates(opps)
	if len(candidates) == 0 {
		return &allocatorSelection{needs: map[string]float64{}, feasible: true}, nil
	}

	capacity := make(map[string]float64, len(balances))
	for name, bal := range balances {
		capacity[name] = e.rebalanceAvailable(name, bal)
	}

	timeout := time.Duration(e.cfg.AllocatorTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Millisecond
	}
	selected := e.solveAllocator(candidates, capacity, balances, remainingSlots, timeout)
	if len(selected) == 0 {
		return &allocatorSelection{needs: map[string]float64{}, feasible: false}, nil
	}

	needs := make(map[string]float64)
	totalValue := 0.0
	for _, choice := range selected {
		needs[choice.longExchange] += choice.requiredMargin
		needs[choice.shortExchange] += choice.requiredMargin
		totalValue += choice.baseValue
	}

	feasible := e.isAllocatorFundingFeasible(needs, balances)

	// Fallback: if the best selection is infeasible, iteratively drop the
	// lowest-value choice and recheck until we find a feasible subset or
	// run out of choices.
	for !feasible && len(selected) > 0 {
		worstIdx := 0
		worstVal := selected[0].baseValue
		for i := 1; i < len(selected); i++ {
			if selected[i].baseValue < worstVal {
				worstVal = selected[i].baseValue
				worstIdx = i
			}
		}
		e.log.Info("allocator: infeasible, dropping %s (%s/%s, value=%.4f)",
			selected[worstIdx].symbol, selected[worstIdx].longExchange,
			selected[worstIdx].shortExchange, worstVal)
		selected = append(selected[:worstIdx], selected[worstIdx+1:]...)

		// Recalculate needs and total value from the reduced set.
		needs = make(map[string]float64, len(selected)*2)
		totalValue = 0
		for _, choice := range selected {
			needs[choice.longExchange] += choice.requiredMargin
			needs[choice.shortExchange] += choice.requiredMargin
			totalValue += choice.baseValue
		}
		feasible = e.isAllocatorFundingFeasible(needs, balances)
	}

	return &allocatorSelection{
		choices:        selected,
		needs:          needs,
		totalBaseValue: totalValue,
		feasible:       feasible,
	}, nil
}

func (e *Engine) buildAllocatorCandidates(opps []models.Opportunity) []allocatorCandidate {
	fees := e.getEffectiveAllocatorFees()
	candidates := make([]allocatorCandidate, 0, len(opps))
	for _, opp := range opps {
		seen := map[string]bool{}
		choices := make([]allocatorChoice, 0, 1+len(opp.Alternatives))
		appendChoice := func(longExch, shortExch string, spread, intervalHours float64, alt *models.AlternativePair) {
			key := longExch + ">" + shortExch
			if seen[key] {
				return
			}
			seen[key] = true
			approval, err := e.risk.SimulateApprovalForPair(opp, longExch, shortExch, nil, alt)
			if err != nil || approval == nil || !approval.Approved || approval.Size <= 0 || approval.Price <= 0 {
				return
			}
			entryNotional := approval.Size * approval.Price
			requiredMargin := approval.RequiredMargin
			if requiredMargin <= 0 {
				requiredMargin = e.cfg.CapitalPerLeg * e.cfg.MarginSafetyMultiplier
			}
			choices = append(choices, allocatorChoice{
				symbol:         opp.Symbol,
				longExchange:   longExch,
				shortExchange:  shortExch,
				spreadBpsH:     spread,
				intervalHours:  intervalHours,
				requiredMargin: requiredMargin,
				entryNotional:  entryNotional,
				baseValue:      computeAllocatorBaseValue(spread, entryNotional, longExch, shortExch, e.cfg.MinHoldTime.Hours(), fees),
			})
		}

		appendChoice(opp.LongExchange, opp.ShortExchange, opp.Spread, opp.IntervalHours, nil)
		for _, alt := range opp.Alternatives {
			// Verify both exchanges in the alternative pair are available.
			if _, ok := e.exchanges[alt.LongExchange]; !ok {
				e.log.Warn("allocator: skipping alternative %s %s>%s — long exchange not available", opp.Symbol, alt.LongExchange, alt.ShortExchange)
				continue
			}
			if _, ok := e.exchanges[alt.ShortExchange]; !ok {
				e.log.Warn("allocator: skipping alternative %s %s>%s — short exchange not available", opp.Symbol, alt.LongExchange, alt.ShortExchange)
				continue
			}
			if alt.Spread <= 0 {
				continue
			}
			// Lightweight verification proxy: skip alternatives whose spread
			// deviates more than 50% from the primary pair's spread.  Real
			// VerifyRates is pair-specific and too heavy to run here; this
			// guards against unreliable Loris data for the alt pair.
			if opp.Spread > 0 && math.Abs(alt.Spread-opp.Spread)/opp.Spread > 0.5 {
				e.log.Info("allocator: skip alt %s/%s for %s — spread %.2f deviates >50%% from primary %.2f",
					alt.LongExchange, alt.ShortExchange, opp.Symbol, alt.Spread, opp.Spread)
				continue
			}
			altCopy := alt
			appendChoice(alt.LongExchange, alt.ShortExchange, alt.Spread, alt.IntervalHours, &altCopy)
		}
		if len(choices) == 0 {
			continue
		}
		sort.Slice(choices, func(i, j int) bool {
			if choices[i].baseValue == choices[j].baseValue {
				return choices[i].requiredMargin < choices[j].requiredMargin
			}
			return choices[i].baseValue > choices[j].baseValue
		})
		candidates = append(candidates, allocatorCandidate{
			symbol:  opp.Symbol,
			choices: choices,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].choices[0].baseValue > candidates[j].choices[0].baseValue
	})
	return candidates
}

func (e *Engine) solveAllocator(candidates []allocatorCandidate, capacity map[string]float64, balances map[string]rebalanceBalanceInfo, remainingSlots int, timeout time.Duration) []allocatorChoice {
	start := time.Now()
	bestValue := -1.0
	var bestChoices []allocatorChoice
	feeCache := map[string]float64{}

	incumbent := e.greedyAllocatorSeed(candidates, cloneFloatMap(capacity), remainingSlots)
	bestValue = e.allocatorChoiceValue(incumbent, balances, feeCache)
	bestChoices = append(bestChoices, incumbent...)

	var branch func(int, map[string]float64, int, []allocatorChoice, float64)
	branch = func(idx int, cap map[string]float64, slots int, current []allocatorChoice, currentValue float64) {
		if time.Since(start) >= timeout {
			return
		}
		if idx >= len(candidates) || slots <= 0 {
			score := e.allocatorChoiceValue(current, balances, feeCache)
			if score > bestValue {
				bestValue = score
				bestChoices = append([]allocatorChoice(nil), current...)
			}
			return
		}

		if currentValue+allocatorUpperBound(candidates, idx, slots) <= bestValue {
			return
		}

		for _, choice := range candidates[idx].choices {
			if cap[choice.longExchange] < choice.requiredMargin || cap[choice.shortExchange] < choice.requiredMargin {
				continue
			}
			nextCap := cloneFloatMap(cap)
			nextCap[choice.longExchange] -= choice.requiredMargin
			nextCap[choice.shortExchange] -= choice.requiredMargin
			next := append(current, choice)
			branch(idx+1, nextCap, slots-1, next, currentValue+choice.baseValue)
		}

		branch(idx+1, cap, slots, current, currentValue)
	}

	branch(0, cloneFloatMap(capacity), remainingSlots, nil, 0)
	return bestChoices
}

func (e *Engine) greedyAllocatorSeed(candidates []allocatorCandidate, capacity map[string]float64, remainingSlots int) []allocatorChoice {
	selected := make([]allocatorChoice, 0, remainingSlots)
	for _, candidate := range candidates {
		if remainingSlots <= 0 {
			break
		}
		for _, choice := range candidate.choices {
			if capacity[choice.longExchange] < choice.requiredMargin || capacity[choice.shortExchange] < choice.requiredMargin {
				continue
			}
			capacity[choice.longExchange] -= choice.requiredMargin
			capacity[choice.shortExchange] -= choice.requiredMargin
			selected = append(selected, choice)
			remainingSlots--
			break
		}
	}
	return selected
}

func allocatorUpperBound(candidates []allocatorCandidate, idx, slots int) float64 {
	sum := 0.0
	for ; idx < len(candidates) && slots > 0; idx++ {
		if len(candidates[idx].choices) == 0 {
			continue
		}
		sum += math.Max(0, candidates[idx].choices[0].baseValue)
		slots--
	}
	return sum
}

func (e *Engine) isAllocatorFundingFeasible(needs map[string]float64, balances map[string]rebalanceBalanceInfo) bool {
	if len(needs) == 0 {
		return true
	}
	donorGross := make(map[string]float64, len(balances))
	deficits := make(map[string]float64)

	for name, bal := range balances {
		localAvail := e.rebalanceAvailable(name, bal)
		need := needs[name]
		if need > localAvail {
			deficits[name] = need - localAvail
		}
		donorGross[name] = e.allocatorDonorGrossCapacity(name, bal, need)
	}
	if len(deficits) == 0 {
		return true
	}

	for exchName, deficit := range deficits {
		remaining := deficit
		for remaining > 10 {
			donor, available, fee, ok := e.findAllocatorDonor(exchName, donorGross, balances)
			if !ok {
				return false
			}
			netPossible := available - fee
			if netPossible <= 10 {
				donorGross[donor] = 0
				continue
			}
			move := math.Min(remaining, netPossible)
			remaining -= move
			donorGross[donor] -= move + fee
		}
	}
	return true
}

func (e *Engine) findAllocatorDonor(recipient string, donorGross map[string]float64, balances map[string]rebalanceBalanceInfo) (string, float64, float64, bool) {
	e.cfg.RLock()
	addrs := e.cfg.ExchangeAddresses[recipient]
	e.cfg.RUnlock()
	if len(addrs) == 0 {
		return "", 0, 0, false
	}
	var chain string
	for _, candidate := range []string{"APT", "BEP20"} {
		if addr := addrs[candidate]; addr != "" {
			chain = candidate
			break
		}
	}
	if chain == "" {
		return "", 0, 0, false
	}

	bestDonor := ""
	bestAvail := 0.0
	bestFee := 0.0
	for donor, avail := range donorGross {
		if donor == recipient || avail <= 10 {
			if donor != recipient {
				e.log.Info("rebalance: donor %s skipped for %s: idle too low (%.2f)", donor, recipient, avail)
			}
			continue
		}
		if balances[donor].marginRatio >= e.cfg.MarginL3Threshold {
			e.log.Info("rebalance: donor %s skipped for %s: marginRatio %.4f >= L3 %.4f", donor, recipient, balances[donor].marginRatio, e.cfg.MarginL3Threshold)
			continue
		}
		fee, err := e.exchanges[donor].GetWithdrawFee("USDT", chain)
		if err != nil {
			continue
		}
		usable := avail - fee
		if usable > bestAvail {
			bestDonor = donor
			bestAvail = avail
			bestFee = fee
		}
	}
	if bestDonor == "" {
		return "", 0, 0, false
	}
	e.log.Info("rebalance: donor %s selected for %s: idle=%.2f fee=%.4f", bestDonor, recipient, bestAvail, bestFee)
	return bestDonor, bestAvail, bestFee, true
}

func (e *Engine) rebalanceAvailable(name string, bal rebalanceBalanceInfo) float64 {
	total := bal.futures + bal.spot
	type unifiedChecker interface{ IsUnified() bool }
	if name == "okx" || name == "bybit" {
		return bal.futures
	}
	if uc, ok := e.exchanges[name].(unifiedChecker); ok && uc.IsUnified() {
		return bal.futures
	}
	return total
}

// getEffectiveAllocatorFees returns allocator fee schedule with dynamic overrides
// from the Scanner's trading fee data. Falls back to hardcoded defaults.
func (e *Engine) getEffectiveAllocatorFees() map[string]allocatorFeeSchedule {
	if e.discovery == nil {
		return allocatorExchangeFees
	}
	scannerFees := e.discovery.GetExchangeFees()
	if len(scannerFees) == 0 {
		return allocatorExchangeFees
	}
	result := make(map[string]allocatorFeeSchedule, len(allocatorExchangeFees))
	for name, fee := range allocatorExchangeFees {
		result[name] = fee
	}
	// Override with scanner's dynamic fees (already in percentage format).
	for name, fee := range scannerFees {
		result[name] = allocatorFeeSchedule{Taker: fee.Taker} // fee is discovery.FeeSchedule
	}
	return result
}

func computeAllocatorBaseValue(spread, entryNotional float64, longExch, shortExch string, holdHours float64, fees map[string]allocatorFeeSchedule) float64 {
	feesA := fees[longExch]
	feesB := fees[shortExch]
	grossFundingValue := spread * holdHours * entryNotional / 10000.0
	tradingFees := 2.0 * (feesA.Taker + feesB.Taker) / 100.0 * entryNotional
	return grossFundingValue - tradingFees
}

func (e *Engine) allocatorChoiceValue(choices []allocatorChoice, balances map[string]rebalanceBalanceInfo, feeCache map[string]float64) float64 {
	sum := 0.0
	for _, choice := range choices {
		sum += choice.baseValue
	}
	if len(choices) == 0 {
		return sum
	}

	needs := make(map[string]float64)
	for _, choice := range choices {
		needs[choice.longExchange] += choice.requiredMargin
		needs[choice.shortExchange] += choice.requiredMargin
	}
	return sum - e.estimateAllocatorTransferCost(needs, balances, feeCache)
}

func (e *Engine) estimateAllocatorTransferCost(needs map[string]float64, balances map[string]rebalanceBalanceInfo, feeCache map[string]float64) float64 {
	if len(needs) == 0 {
		return 0
	}

	donorGross := make(map[string]float64, len(balances))
	type deficit struct {
		exchange string
		amount   float64
	}
	var crossDeficits []deficit

	targetFreeRatio := 1.0 - e.cfg.MarginL4Threshold
	if targetFreeRatio <= 0 {
		targetFreeRatio = 0.05
	}

	for name, bal := range balances {
		need := needs[name]
		donorGross[name] = e.allocatorDonorGrossCapacity(name, bal, need)

		if bal.futures >= need {
			continue
		}

		shortfall := need - bal.futures
		l4Extra := need/targetFreeRatio - bal.futuresTotal - shortfall
		if l4Extra < 0 {
			l4Extra = 0
		}
		if l4Extra > shortfall {
			l4Extra = shortfall
		}
		transferAmt := shortfall + l4Extra
		if bal.spot > 0 {
			actualTransfer := transferAmt
			if actualTransfer > bal.spot {
				actualTransfer = bal.spot
			}
			if actualTransfer >= 1.0 {
				transferAmt -= actualTransfer
			}
		}
		if transferAmt > 10 {
			crossDeficits = append(crossDeficits, deficit{exchange: name, amount: transferAmt})
		}
	}

	totalCost := 0.0
	for _, d := range crossDeficits {
		remaining := d.amount
		for remaining > 10 {
			donor, available, fee, ok := e.findAllocatorDonorWithCache(d.exchange, donorGross, balances, feeCache)
			if !ok {
				// No donor available. Use average fee from this cycle's cache,
				// filtering out -1 error sentinels. Fall back to config penalty
				// if no valid fees cached.
				var feeSum float64
				var feeCount int
				for _, f := range feeCache {
					if f >= 0 {
						feeSum += f
						feeCount++
					}
				}
				if feeCount > 0 {
					totalCost += feeSum / float64(feeCount)
				} else {
					const transferPenaltyFallback = 0.1 // USDT, fallback when no fee data available
				totalCost += transferPenaltyFallback
				}
				break
			}
			netPossible := available - fee
			if netPossible <= 10 {
				donorGross[donor] = 0
				continue
			}
			move := math.Min(remaining, netPossible)
			totalCost += fee // actual withdraw fee is the transfer cost
			remaining -= move
			donorGross[donor] -= move + fee
		}
	}

	return totalCost
}

func cloneFloatMap(src map[string]float64) map[string]float64 {
	dst := make(map[string]float64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (e *Engine) findAllocatorDonorWithCache(recipient string, donorGross map[string]float64, balances map[string]rebalanceBalanceInfo, feeCache map[string]float64) (string, float64, float64, bool) {
	e.cfg.RLock()
	addrs := e.cfg.ExchangeAddresses[recipient]
	e.cfg.RUnlock()
	if len(addrs) == 0 {
		return "", 0, 0, false
	}
	var chain string
	for _, candidate := range []string{"APT", "BEP20"} {
		if addr := addrs[candidate]; addr != "" {
			chain = candidate
			break
		}
	}
	if chain == "" {
		return "", 0, 0, false
	}

	bestDonor := ""
	bestAvail := 0.0
	bestFee := 0.0
	for donor, avail := range donorGross {
		if donor == recipient || avail <= 10 {
			if donor != recipient {
				e.log.Info("rebalance: donor %s skipped for %s: idle too low (%.2f)", donor, recipient, avail)
			}
			continue
		}
		if balances[donor].marginRatio >= e.cfg.MarginL3Threshold {
			e.log.Info("rebalance: donor %s skipped for %s: marginRatio %.4f >= L3 %.4f", donor, recipient, balances[donor].marginRatio, e.cfg.MarginL3Threshold)
			continue
		}

		cacheKey := donor + "|" + recipient + "|" + chain
		fee, ok := feeCache[cacheKey]
		if !ok {
			var err error
			fee, err = e.exchanges[donor].GetWithdrawFee("USDT", chain)
			if err != nil {
				feeCache[cacheKey] = -1
				continue
			}
			feeCache[cacheKey] = fee
		}
		if fee < 0 {
			continue
		}

		usable := avail - fee
		if usable > bestAvail {
			bestDonor = donor
			bestAvail = avail
			bestFee = fee
		}
	}
	if bestDonor == "" {
		return "", 0, 0, false
	}
	e.log.Info("rebalance: donor %s selected for %s: idle=%.2f fee=%.4f", bestDonor, recipient, bestAvail, bestFee)
	return bestDonor, bestAvail, bestFee, true
}

func (e *Engine) allocatorDonorGrossCapacity(name string, bal rebalanceBalanceInfo, localNeed float64) float64 {
	totalSurplus := e.rebalanceAvailable(name, bal) - localNeed
	if totalSurplus <= 0 {
		return 0
	}

	if e.allocatorWithdrawalUsesMainBalance(name) {
		if bal.hasPositions {
			healthCap := e.capByMarginHealth(bal)
			if healthCap < totalSurplus {
				totalSurplus = healthCap
			}
		}
		if totalSurplus < 0 {
			return 0
		}
		return totalSurplus
	}

	movable := bal.maxTransferOut
	if movable <= 0 {
		movable = bal.futures
		if bal.marginRatio > 0 && bal.futuresTotal > 0 && e.cfg.MarginL4Threshold > 0 {
			safeMax := bal.futuresTotal * (1.0 - bal.marginRatio/e.cfg.MarginL4Threshold)
			if safeMax < movable {
				movable = safeMax
			}
		}
	}

	// For exchanges with active positions, cap transfer to keep margin ratio
	// below L3 threshold. Without this, maxTransferOut from the exchange API
	// may allow draining enough margin to trigger L4/L5 emergency actions.
	healthCap := e.capByMarginHealth(bal)
	if healthCap < movable {
		movable = healthCap
	}

	movable -= localNeed
	if movable < 0 {
		movable = 0
	}

	grossCapacity := bal.spot + movable
	if grossCapacity > totalSurplus {
		grossCapacity = totalSurplus
	}
	if grossCapacity < 0 {
		return 0
	}

	e.log.Info("rebalance: donor %s capacity: futures=%.2f total=%.2f marginRatio=%.4f maxTransferOut=%.2f healthCap=%.2f movable=%.2f gross=%.2f hasPos=%v",
		name, bal.futures, bal.futuresTotal, bal.marginRatio, bal.maxTransferOut, healthCap, movable, grossCapacity, bal.hasPositions)

	return grossCapacity
}

func (e *Engine) allocatorWithdrawalUsesMainBalance(name string) bool {
	if name == "binance" || name == "gateio" {
		return true
	}
	type unifiedChecker interface{ IsUnified() bool }
	if uc, ok := e.exchanges[name].(unifiedChecker); ok && uc.IsUnified() {
		return true
	}
	return false
}

// capByMarginHealth returns the maximum amount that can be transferred out
// of an exchange without pushing its margin ratio past the L3 threshold.
// Returns math.MaxFloat64 when the exchange has no positions or the
// calculation is not applicable (no margin data).
func (e *Engine) capByMarginHealth(bal rebalanceBalanceInfo) float64 {
	if !bal.hasPositions {
		return math.MaxFloat64
	}
	if bal.marginRatio <= 0 || bal.futuresTotal <= 0 || e.cfg.MarginL3Threshold <= 0 {
		return math.MaxFloat64
	}
	// maint = marginRatio * futuresTotal (estimated maintenance margin)
	// After removing X: newRatio = maint / (futuresTotal - X) must stay < L3
	// Solve: X < futuresTotal - maint / L3
	maint := bal.marginRatio * bal.futuresTotal
	cap := bal.futuresTotal - maint/e.cfg.MarginL3Threshold
	if cap < 0 {
		return 0
	}
	return cap
}

func (e *Engine) formatAllocatorSummary(sel *allocatorSelection) string {
	if sel == nil {
		return "nil"
	}
	names := make([]string, 0, len(sel.choices))
	for _, choice := range sel.choices {
		names = append(names, fmt.Sprintf("%s:%s/%s", choice.symbol, choice.longExchange, choice.shortExchange))
	}
	return strings.Join(names, ", ")
}

func (e *Engine) executeRebalanceFundingPlan(needs map[string]float64, balances map[string]rebalanceBalanceInfo) {
	type deficit struct {
		exchange string
		amount   float64
	}

	var crossDeficits []deficit
	for name, need := range needs {
		bal := balances[name]
		targetFreeRatio := 1 - e.cfg.MarginL4Threshold
		if targetFreeRatio <= 0 {
			targetFreeRatio = 0.20
		}

		if bal.futures >= need {
			if bal.spot > 0 && bal.futuresTotal > 0 {
				projectedAvail := bal.futures - need
				if projectedAvail < 0 {
					projectedAvail = 0
				}
				projectedRatio := 1 - projectedAvail/bal.futuresTotal
				if projectedRatio >= e.cfg.MarginL4Threshold {
					extra := (need - bal.futuresTotal*targetFreeRatio) / targetFreeRatio
					if extra > bal.spot {
						extra = bal.spot
					}
					if extra >= 1.0 {
						amtStr := fmt.Sprintf("%.4f", extra)
						postRatio := 1 - (bal.futures+extra-need)/(bal.futuresTotal+extra)
						e.log.Info("rebalance: %s spot->futures %s USDT (margin ratio relief, projected=%.2f post=%.2f L4=%.2f)", name, amtStr, projectedRatio, postRatio, e.cfg.MarginL4Threshold)
						if !e.cfg.DryRun {
							if err := e.exchanges[name].TransferToFutures("USDT", amtStr); err != nil {
								e.log.Error("rebalance: %s spot->futures failed: %v", name, err)
							} else {
								bi := balances[name]
								bi.futures += extra
								bi.spot -= extra
								balances[name] = bi
							}
						}
					}
				}
			}
			continue
		}

		shortfall := need - bal.futures
		l4Extra := need/targetFreeRatio - bal.futuresTotal - shortfall
		if l4Extra < 0 {
			l4Extra = 0
		}
		if l4Extra > shortfall {
			l4Extra = shortfall
		}
		transferAmt := shortfall + l4Extra
		if bal.spot > 0 {
			actualTransfer := transferAmt
			if actualTransfer > bal.spot {
				actualTransfer = bal.spot
			}
			if actualTransfer < 1.0 {
				e.log.Debug("rebalance: %s spot->futures skip (%.4f USDT below minimum)", name, actualTransfer)
				if transferAmt > 10 {
					crossDeficits = append(crossDeficits, deficit{name, transferAmt})
				}
				continue
			}
			postTotal := bal.futuresTotal + actualTransfer
			postRatio := 1 - (bal.futures+actualTransfer-need)/postTotal
			amtStr := fmt.Sprintf("%.4f", actualTransfer)
			e.log.Info("rebalance: %s spot->futures %s USDT (same-exchange, instant, post-ratio=%.2f L4=%.2f)", name, amtStr, postRatio, e.cfg.MarginL4Threshold)
			if !e.cfg.DryRun {
				if err := e.exchanges[name].TransferToFutures("USDT", amtStr); err != nil {
					e.log.Error("rebalance: %s spot->futures failed: %v", name, err)
				} else {
					transferAmt -= actualTransfer
					bi := balances[name]
					bi.futures += actualTransfer
					bi.spot -= actualTransfer
					balances[name] = bi
				}
			}
		}
		if transferAmt > 10 {
			crossDeficits = append(crossDeficits, deficit{name, transferAmt})
		}
	}

	if len(crossDeficits) == 0 {
		e.log.Info("rebalance: all exchanges funded, no cross-exchange transfers needed")
		return
	}

	surplus := map[string]float64{}
	for name := range e.exchanges {
		surplus[name] = e.rebalanceAvailable(name, balances[name]) - needs[name]
		// Cap by health if exchange has positions
		if balances[name].hasPositions {
			healthCap := e.capByMarginHealth(balances[name])
			if surplus[name] > healthCap {
				surplus[name] = healthCap
			}
		}
	}

	pendingDeposits := map[string]float64{}
	pendingStartBal := map[string]float64{}
	for i := range crossDeficits {
		remaining := crossDeficits[i].amount
		exchName := crossDeficits[i].exchange
		for remaining > 10 {
			var bestDonor string
			var bestSurplus float64
			for name, s := range surplus {
				if name == exchName || s <= 10 {
					continue
				}
				if balances[name].marginRatio >= e.cfg.MarginL3Threshold {
					e.log.Info("rebalance: skipping donor %s (marginRatio=%.4f >= L3=%.4f)", name, balances[name].marginRatio, e.cfg.MarginL3Threshold)
					continue
				}
				if s > bestSurplus {
					bestDonor = name
					bestSurplus = s
				}
			}
			if bestDonor == "" {
				e.log.Warn("rebalance: no donor found for %s remaining deficit=%.2f", exchName, remaining)
				break
			}

			contribution := remaining
			if contribution > bestSurplus {
				contribution = bestSurplus
			}

			e.cfg.RLock()
			origAddrs := e.cfg.ExchangeAddresses[exchName]
			recipientAddrs := make(map[string]string, len(origAddrs))
			for k, v := range origAddrs {
				recipientAddrs[k] = v
			}
			e.cfg.RUnlock()
			if len(recipientAddrs) == 0 {
				e.log.Warn("rebalance: no deposit addresses for %s", exchName)
				break
			}

			var chain, destAddr string
			for _, c := range []string{"APT", "BEP20"} {
				if addr, ok := recipientAddrs[c]; ok && addr != "" {
					chain = c
					destAddr = addr
					break
				}
			}
			if chain == "" {
				e.log.Warn("rebalance: no shared chain between %s and %s", bestDonor, exchName)
				surplus[bestDonor] = 0
				continue
			}

			fee, feeErr := e.exchanges[bestDonor].GetWithdrawFee("USDT", chain)
			if feeErr != nil {
				e.log.Warn("rebalance: %s GetWithdrawFee failed: %v, skipping donor", bestDonor, feeErr)
				surplus[bestDonor] = 0
				continue
			}
			e.log.Info("rebalance: %s withdraw fee=%.4f USDT via %s", bestDonor, fee, chain)

			// For net-fee exchanges (OKX, Bitget, Bybit), actual debit = contribution + fee.
			// Subtract actual fee from contribution so total debit stays within health cap.
			if !e.exchanges[bestDonor].WithdrawFeeInclusive() && balances[bestDonor].hasPositions {
				contribution -= fee
				if contribution < 10 {
					e.log.Warn("rebalance: %s contribution %.2f too low after fee %.4f deduction, skipping", bestDonor, contribution, fee)
					surplus[bestDonor] = 0
					continue
				}
			}

			netAmount := contribution
			requiredSpot := netAmount + fee
			e.log.Info("rebalance: cross-exchange %s->%s net=%.2f fee=%.4f required=%.2f USDT via %s", bestDonor, exchName, netAmount, fee, requiredSpot, chain)

			if e.cfg.DryRun {
				e.log.Info("[DRY RUN] would transfer %.2f USDT (net) from %s to %s via %s (fee=%.4f)", netAmount, bestDonor, exchName, chain, fee)
				remaining -= netAmount
				surplus[bestDonor] -= requiredSpot
				continue
			}

			var movedToSpot float64
			skipOuterTransfer := bestDonor == "binance" || bestDonor == "gateio"
			if !skipOuterTransfer {
				if uc, ok := e.exchanges[bestDonor].(interface{ IsUnified() bool }); ok && uc.IsUnified() {
					skipOuterTransfer = true
				}
			}
			donorBal := balances[bestDonor]
			if !skipOuterTransfer && donorBal.spot < requiredSpot {
				moveAmt := requiredSpot - donorBal.spot
				maxMove := donorBal.maxTransferOut
				if maxMove <= 0 {
					maxMove = donorBal.futures - needs[bestDonor]
					if donorBal.marginRatio > 0 && donorBal.futuresTotal > 0 && e.cfg.MarginL4Threshold > 0 {
						safeMax := donorBal.futuresTotal * (1.0 - donorBal.marginRatio/e.cfg.MarginL4Threshold)
						safeMax -= needs[bestDonor]
						if safeMax < maxMove {
							maxMove = safeMax
						}
					}
				} else {
					maxMove -= needs[bestDonor]
				}
				// Cap by margin health to prevent transfers that would push
				// the donor exchange past the L3 threshold.
				healthCap := e.capByMarginHealth(donorBal) - needs[bestDonor]
				if healthCap < maxMove {
					e.log.Info("rebalance: %s maxMove capped by healthCap: %.2f -> %.2f (marginRatio=%.4f, L3=%.4f)",
						bestDonor, maxMove, healthCap, donorBal.marginRatio, e.cfg.MarginL3Threshold)
					maxMove = healthCap
				}
				if moveAmt > maxMove && maxMove > 0 {
					moveAmt = maxMove
				}
				if moveAmt <= 0 {
					e.log.Warn("rebalance: %s has no excess futures to move to spot", bestDonor)
					surplus[bestDonor] = 0
					continue
				}
				if freshBal, err := e.exchanges[bestDonor].GetFuturesBalance(); err == nil && freshBal.MaxTransferOut > 0 {
					cap := freshBal.MaxTransferOut * 0.99
					if moveAmt > cap {
						e.log.Info("rebalance: %s capping moveAmt %.4f to fresh maxTransferOut %.4f (99%%)", bestDonor, moveAmt, cap)
						moveAmt = cap
					}
				}
				if moveAmt <= 0 {
					e.log.Warn("rebalance: %s fresh maxTransferOut too low", bestDonor)
					surplus[bestDonor] = 0
					continue
				}
				moveStr := fmt.Sprintf("%.4f", moveAmt)
				e.log.Info("rebalance: %s futures->spot %s USDT", bestDonor, moveStr)
				if err := e.exchanges[bestDonor].TransferToSpot("USDT", moveStr); err != nil {
					e.log.Error("rebalance: %s futures->spot failed: %v", bestDonor, err)
					surplus[bestDonor] = 0
					continue
				}
				e.recordTransfer(bestDonor, bestDonor+" spot", "USDT", "internal", moveStr, "0", "", "completed", "rebalance-prep")
				movedToSpot = moveAmt
			}

			donorSpotBal, err := e.exchanges[bestDonor].GetSpotBalance()
			if err != nil {
				e.log.Error("rebalance: %s get spot balance failed: %v", bestDonor, err)
				surplus[bestDonor] = 0
				continue
			}

			grossRequired := netAmount + fee
			isGross := e.exchanges[bestDonor].WithdrawFeeInclusive()

			// For net-fee exchanges, actual debit = netAmount + fee.
			// Cap so total debit doesn't exceed the margin health limit.
			if !isGross && donorBal.hasPositions {
				healthCap := e.capByMarginHealth(donorBal)
				maxNet := healthCap - fee
				if maxNet < netAmount {
					e.log.Info("rebalance: %s net capped by health: %.2f -> %.2f (healthCap=%.2f, fee=%.4f)",
						bestDonor, netAmount, maxNet, healthCap, fee)
					netAmount = maxNet
					grossRequired = netAmount + fee
				}
			}

			if donorSpotBal.Available < grossRequired {
				netAmount = donorSpotBal.Available - fee
			}
			if netAmount < 10 {
				e.log.Warn("rebalance: %s spot balance too low to withdraw (available=%.2f, fee=%.4f)", bestDonor, donorSpotBal.Available, fee)
				surplus[bestDonor] = 0
				continue
			}

			withdrawAmtForAPI := netAmount
			if isGross {
				withdrawAmtForAPI = netAmount + fee
			}
			amtStr := fmt.Sprintf("%.4f", withdrawAmtForAPI)
			result, err := e.exchanges[bestDonor].Withdraw(exchange.WithdrawParams{
				Coin:    "USDT",
				Chain:   chain,
				Address: destAddr,
				Amount:  amtStr,
			})
			if err != nil {
				e.log.Error("rebalance: withdraw from %s failed: %v", bestDonor, err)
				if movedToSpot > 0 {
					rollbackStr := fmt.Sprintf("%.4f", movedToSpot)
					e.log.Info("rebalance: rollback %s spot->futures %s USDT", bestDonor, rollbackStr)
					if bestDonor == "bingx" {
						time.Sleep(2 * time.Second)
					}
					if rbErr := e.exchanges[bestDonor].TransferToFutures("USDT", rollbackStr); rbErr != nil {
						e.log.Error("rebalance: rollback failed: %v", rbErr)
					}
				}
				surplus[bestDonor] = 0
				continue
			}

			recipientReceives := netAmount
			if isGross {
				recipientReceives = withdrawAmtForAPI - fee
			}
			e.log.Info("rebalance: withdraw from %s txid=%s (apiAmt=%.2f, recipient=%.2f, fee=%.4f, gross=%v) -> %s", bestDonor, result.TxID, withdrawAmtForAPI, recipientReceives, fee, isGross, exchName)
			e.recordTransfer(bestDonor, exchName, "USDT", chain, amtStr, result.Fee, result.TxID, "completed", "rebalance")

			if _, exists := pendingStartBal[exchName]; !exists {
				pendingStartBal[exchName] = balances[exchName].spot
			}
			pendingDeposits[exchName] += recipientReceives
			surplus[bestDonor] -= netAmount + fee
			bi := balances[bestDonor]
			bi.futures -= movedToSpot
			bi.spot -= (netAmount + fee - movedToSpot)
			if bi.spot < 0 {
				bi.spot = 0
			}
			balances[bestDonor] = bi
			remaining -= recipientReceives
		}
	}

	for recipient, totalPending := range pendingDeposits {
		if totalPending <= 0 {
			continue
		}
		recipientExch := e.exchanges[recipient]
		startBal := pendingStartBal[recipient]
		e.log.Info("rebalance: waiting for %.2f USDT total deposits on %s (startBal=%.2f)...", totalPending, recipient, startBal)
		arrived := false
		pollDeadline := time.Now().Add(5 * time.Minute)
		for time.Now().Before(pollDeadline) {
			time.Sleep(5 * time.Second)
			spotBal, err := recipientExch.GetSpotBalance()
			if err != nil {
				continue
			}
			if spotBal.Available >= startBal+totalPending*0.9 {
				arrived = true
				e.log.Info("rebalance: all deposits confirmed on %s (spot=%.2f)", recipient, spotBal.Available)
				break
			}
		}
		if !arrived {
			e.log.Warn("rebalance: deposits on %s not confirmed within 5min, skipping spot->futures", recipient)
			continue
		}
		if totalPending < 1.0 {
			continue
		}
		transferStr := fmt.Sprintf("%.4f", totalPending)
		if err := recipientExch.TransferToFutures("USDT", transferStr); err != nil {
			e.log.Error("rebalance: %s spot->futures failed: %v", recipient, err)
		} else {
			e.log.Info("rebalance: %s spot->futures %s USDT (rebalance deposit)", recipient, transferStr)
			e.recordTransfer(recipient+" spot", recipient, "USDT", "internal", transferStr, "0", "", "completed", "rebalance-recv")
		}
	}

	e.log.Info("rebalance: complete")
}
