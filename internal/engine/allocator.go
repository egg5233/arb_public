package engine

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/exchange"
)

const marginEpsilon = 0.005 // margin buffer to avoid hitting L4 boundary exactly

// feeEntry caches a single GetWithdrawFee result (fee + minWithdraw).
// valid=false means the lookup failed or was not yet attempted.
type feeEntry struct {
	fee   float64
	minWd float64
	valid bool // replaces -1 sentinel
}

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

// rebalanceDeficit records that an exchange needs additional funding from
// a cross-exchange transfer. Used both internally by executeRebalanceFundingPlan
// and by the sequential rebalance path in rebalanceFunds.
type rebalanceDeficit struct {
	exchange string
	amount   float64
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
	xferFeeDeducted float64
	altPair        *models.AlternativePair // nil for primary pair, set for alternatives
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

type exchNeedsMap map[string]float64

func (e *Engine) computeExchangeDeficit(exch string, totalNeed float64, bal rebalanceBalanceInfo) float64 {
	avail := e.rebalanceAvailable(exch, bal)

	marginDef := totalNeed - avail
	if marginDef < 0 {
		marginDef = 0
	}

	actualMargin := totalNeed / e.cfg.MarginSafetyMultiplier
	if actualMargin <= 0 {
		actualMargin = totalNeed
	}
	targetRatio := e.cfg.MarginL4Threshold - marginEpsilon
	freeTarget := 1.0 - targetRatio
	var ratioDef float64
	if freeTarget > 0 && bal.futuresTotal > 0 {
		ratioDef = (freeTarget*bal.futuresTotal - avail + actualMargin) / targetRatio
		if ratioDef < 0 {
			ratioDef = 0
		}
	}

	result := marginDef
	if ratioDef > marginDef {
		result = ratioDef
	}
	e.log.Debug("allocator: deficit %s: totalNeed=%.2f avail=%.2f marginDef=%.2f ratioDef=%.2f result=%.2f", exch, totalNeed, avail, marginDef, ratioDef, result)
	return result
}

func (e *Engine) runPoolAllocator(opps []models.Opportunity, balances map[string]rebalanceBalanceInfo, remainingSlots int) (*allocatorSelection, error) {
	if e.lossLimiter != nil && e.cfg.EnableLossLimits {
		if blocked, status := e.lossLimiter.CheckLimits(); blocked {
			e.log.Warn("allocator: loss limit breached (%s), skipping", status.BreachType)
			e.api.BroadcastLossLimits(status)
			if e.cfg.EnablePerpTelegram {
				e.telegram.NotifyLossLimitBreached(status.BreachType,
					status.DailyLoss, status.DailyLimit,
					status.WeeklyLoss, status.WeeklyLimit)
			}
			return &allocatorSelection{feasible: false}, nil
		}
	}

	// Compute max transferable surplus per exchange from all other healthy exchanges.
	// This lets the allocator consider pairs where one leg is slightly underfunded
	// but could receive a transfer from a donor exchange before entry.
	transferable := make(map[string]float64, len(balances))
	for recipient := range e.exchanges {
		var totalSurplus float64
		for donor, bal := range balances {
			if donor == recipient {
				continue
			}
			if bal.marginRatio >= e.cfg.MarginL3Threshold {
				continue // skip unhealthy donors
			}
			surplus := e.rebalanceAvailable(donor, bal)
			if bal.hasPositions {
				healthCap := e.capByMarginHealth(bal)
				if healthCap == math.MaxFloat64 {
					e.log.Debug("allocator: capByHealth %s: MaxFloat64 (no positions)", donor)
				}
				if surplus > healthCap {
					surplus = healthCap
				}
			}
			if surplus > 0 {
				totalSurplus += surplus
			}
		}
		transferable[recipient] = totalSurplus
	}
	e.log.Debug("allocator: transferable: %v", transferable)

	cache := &risk.PrefetchCache{
		TransferablePerExchange: transferable,
	}
	candidates := e.buildAllocatorCandidates(opps, cache)
	if len(candidates) == 0 {
		e.log.Debug("allocator: no candidates built from %d opps", len(opps))
		return &allocatorSelection{needs: map[string]float64{}, feasible: true}, nil
	}

	capacity := make(map[string]float64, len(balances))
	for name, bal := range balances {
		capacity[name] = e.rebalanceAvailable(name, bal)
	}
	e.log.Debug("allocator: capacity map: %v", capacity)

	totalDonorSurplus := 0.0
	for donor, bal := range balances {
		if bal.marginRatio >= e.cfg.MarginL3Threshold {
			continue
		}
		surplus := e.rebalanceAvailable(donor, bal)
		if bal.hasPositions {
			healthCap := e.capByMarginHealth(bal)
			if healthCap == math.MaxFloat64 {
				e.log.Debug("allocator: capByHealth %s: MaxFloat64 (no positions)", donor)
			}
			if surplus > healthCap {
				surplus = healthCap
			}
		}
		if surplus > 0 {
			totalDonorSurplus += surplus
		}
	}
	e.log.Debug("allocator: totalDonorSurplus=%.2f", totalDonorSurplus)

	timeout := time.Duration(e.cfg.AllocatorTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Millisecond
	}
	selected := e.solveAllocator(candidates, capacity, balances, remainingSlots, timeout, totalDonorSurplus)
	if len(selected) == 0 {
		e.log.Debug("allocator: solver returned no selections from %d candidates", len(candidates))
		return &allocatorSelection{needs: map[string]float64{}, feasible: false}, nil
	}

	// Re-validate solver selections with accumulated reserved, matching :40's
	// sequential approval pattern (engine.go:1651-1734).
	{
		reserved := map[string]float64{}
		revalCache := &risk.PrefetchCache{
			TransferablePerExchange: transferable, // same transfer-aware view as buildAllocatorCandidates
		}
		validated := make([]allocatorChoice, 0, len(selected))
		for _, choice := range selected {
			opp := findOppBySymbol(opps, choice.symbol)
			if opp == nil {
				continue
			}
			approval, err := e.risk.SimulateApprovalForPair(*opp, choice.longExchange, choice.shortExchange, reserved, choice.altPair, revalCache)
			if err != nil || approval == nil || !approval.Approved || approval.Size <= 0 {
				e.log.Info("allocator: re-validation rejected %s (%s/%s, reserved=%v): %v",
					choice.symbol, choice.longExchange, choice.shortExchange, reserved,
					func() string {
						if approval != nil {
							return approval.Reason
						}
						return "nil/error"
					}())
				continue
			}
			choice.requiredMargin = approval.RequiredMargin
			if choice.requiredMargin <= 0 {
				choice.requiredMargin = e.cfg.CapitalPerLeg * e.cfg.MarginSafetyMultiplier
				e.log.Debug("allocator: re-validate %s fallback margin=%.2f", choice.symbol, choice.requiredMargin)
			}
			validated = append(validated, choice)
			reserved[choice.longExchange] += approval.RequiredMargin
			reserved[choice.shortExchange] += approval.RequiredMargin
		}
		selected = validated
	}
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

	// Post-solver simulation: verify that planned transfers + position openings
	// won't push any exchange to L3+ health level.
	if feasible && len(selected) > 0 {
		simOK, simFee := e.simulateTransferPlan(selected, balances)
		for !simOK && len(selected) > 0 {
			worstIdx := 0
			worstVal := selected[0].baseValue
			for i := 1; i < len(selected); i++ {
				if selected[i].baseValue < worstVal {
					worstVal = selected[i].baseValue
					worstIdx = i
				}
			}
			e.log.Info("allocator: simulation rejected, dropping %s (%s/%s, value=%.4f)",
				selected[worstIdx].symbol, selected[worstIdx].longExchange,
				selected[worstIdx].shortExchange, worstVal)
			selected = append(selected[:worstIdx], selected[worstIdx+1:]...)

			if len(selected) > 0 {
				simOK, simFee = e.simulateTransferPlan(selected, balances)
			}
		}
		if simFee > 0 {
			e.log.Info("allocator: simulation passed, estimated transfer fees=%.4f", simFee)
		}
		needs = make(map[string]float64, len(selected)*2)
		totalValue = 0
		for _, choice := range selected {
			needs[choice.longExchange] += choice.requiredMargin
			needs[choice.shortExchange] += choice.requiredMargin
			totalValue += choice.baseValue
		}
		feasible = len(selected) > 0
	}

	return &allocatorSelection{
		choices:        selected,
		needs:          needs,
		totalBaseValue: totalValue,
		feasible:       feasible,
	}, nil
}

func (e *Engine) buildAllocatorCandidates(opps []models.Opportunity, cache *risk.PrefetchCache) []allocatorCandidate {
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
			approval, err := e.risk.SimulateApprovalForPair(opp, longExch, shortExch, nil, alt, cache)
			if err != nil || approval == nil || !approval.Approved || approval.Size <= 0 || approval.Price <= 0 {
				if err != nil {
					e.log.Debug("allocator: appendChoice error %s %s/%s: %v", opp.Symbol, longExch, shortExch, err)
				} else if approval != nil && !approval.Approved {
					e.log.Debug("allocator: appendChoice rejected %s %s/%s: %s", opp.Symbol, longExch, shortExch, approval.Reason)
				} else if approval != nil {
					e.log.Debug("allocator: appendChoice rejected %s %s/%s: size=%.0f price=%.6f", opp.Symbol, longExch, shortExch, approval.Size, approval.Price)
				}
				return
			}
			// Run pair-keyed filters (backtest, persistence, volatility) on alternative pairs.
			// Discovery already filtered the original pair, but alternatives bypass that check.
			if alt != nil {
				altOpp := models.Opportunity{
					Symbol:        opp.Symbol,
					LongExchange:  longExch,
					ShortExchange: shortExch,
					Spread:        spread,
					IntervalHours: intervalHours,
					NextFunding:   alt.NextFunding,
					OIRank:        opp.OIRank,
				}
				if reason := e.discovery.CheckPairFilters(altOpp); reason != "" {
					e.log.Debug("allocator: appendChoice rejected %s %s/%s: %s", opp.Symbol, longExch, shortExch, reason)
					return
				}
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
				altPair:        alt,
			})
			e.log.Debug("allocator: choice added %s %s/%s: margin=%.2f baseValue=%.4f notional=%.2f", opp.Symbol, longExch, shortExch, requiredMargin, choices[len(choices)-1].baseValue, entryNotional)
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
				e.log.Debug("allocator: skip alt %s/%s for %s: spread=%.4f", alt.LongExchange, alt.ShortExchange, opp.Symbol, alt.Spread)
				continue
			}
			altCopy := alt
			appendChoice(alt.LongExchange, alt.ShortExchange, alt.Spread, alt.IntervalHours, &altCopy)
		}
		if len(choices) == 0 {
			e.log.Debug("allocator: no viable choices for %s", opp.Symbol)
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

func (e *Engine) solveAllocator(candidates []allocatorCandidate, capacity map[string]float64, balances map[string]rebalanceBalanceInfo, remainingSlots int, timeout time.Duration, xferBudget float64) []allocatorChoice {
	start := time.Now()
	bestValue := -1.0
	var bestChoices []allocatorChoice
	feeCache := map[string]feeEntry{}

	incumbent := e.greedyAllocatorSeed(candidates, cloneFloatMap(capacity), balances, feeCache, remainingSlots, xferBudget)
	bestValue = e.allocatorChoiceValue(incumbent, balances, feeCache)
	bestChoices = append(bestChoices, incumbent...)
	greedyN := len(incumbent)
	greedyVal := bestValue

	var branch func(int, map[string]float64, int, []allocatorChoice, float64, exchNeedsMap)
	branch = func(idx int, cap map[string]float64, slots int, current []allocatorChoice, currentValue float64, needs exchNeedsMap) {
		if time.Since(start) >= timeout {
			e.log.Debug("allocator: B&B timeout after %v, best=%d choices", time.Since(start), len(bestChoices))
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

		ub := allocatorUpperBound(candidates, idx, slots)
		if currentValue+ub <= bestValue {
			e.log.Debug("allocator: B&B prune idx=%d bound=%.4f<=best=%.4f", idx, currentValue+ub, bestValue)
			return
		}

		for _, choice := range candidates[idx].choices {
			needsTransfer := cap[choice.longExchange] < choice.requiredMargin || cap[choice.shortExchange] < choice.requiredMargin
			if needsTransfer {
				fsp := firstSettlementProfit(choice.spreadBpsH, choice.intervalHours, choice.entryNotional)
				xferFee := estimateChoiceTransferFee(e, choice, cap, balances, feeCache)
				if xferFee < 0 || fsp <= xferFee {
					e.log.Info("allocator: reject xfer %s %s/%s fsp=%.4f xferFee=%.4f (need fsp>fee)",
						choice.symbol, choice.longExchange, choice.shortExchange, fsp, xferFee)
					continue
				}
				// Budget check: compute total deficit if this choice is added.
				nextNeeds := make(exchNeedsMap, len(needs)+2)
				for k, v := range needs {
					nextNeeds[k] = v
				}
				nextNeeds[choice.longExchange] += choice.requiredMargin
				nextNeeds[choice.shortExchange] += choice.requiredMargin
				totalDeficit := 0.0
				for exch, totalNeed := range nextNeeds {
					if bal, ok := balances[exch]; ok {
						totalDeficit += e.computeExchangeDeficit(exch, totalNeed, bal)
					}
				}
				if totalDeficit > xferBudget {
					e.log.Debug("allocator: budget exceeded %s %s/%s deficit=%.2f budget=%.2f", choice.symbol, choice.longExchange, choice.shortExchange, totalDeficit, xferBudget)
					continue
				}
				e.log.Info("allocator: admit xfer %s %s/%s fsp=%.4f xferFee=%.4f",
					choice.symbol, choice.longExchange, choice.shortExchange, fsp, xferFee)
				adjustedChoice := choice
				adjustedChoice.baseValue -= xferFee
				adjustedChoice.xferFeeDeducted = xferFee
				nextCap := cloneFloatMap(cap)
				nextCap[adjustedChoice.longExchange] = math.Max(0, nextCap[adjustedChoice.longExchange]-adjustedChoice.requiredMargin)
				nextCap[adjustedChoice.shortExchange] = math.Max(0, nextCap[adjustedChoice.shortExchange]-adjustedChoice.requiredMargin)
				next := append(current, adjustedChoice)
				branch(idx+1, nextCap, slots-1, next, currentValue+adjustedChoice.baseValue, nextNeeds)
				continue
			}
			nextNeeds := make(exchNeedsMap, len(needs)+2)
			for k, v := range needs {
				nextNeeds[k] = v
			}
			nextNeeds[choice.longExchange] += choice.requiredMargin
			nextNeeds[choice.shortExchange] += choice.requiredMargin
			nextCap := cloneFloatMap(cap)
			nextCap[choice.longExchange] -= choice.requiredMargin
			nextCap[choice.shortExchange] -= choice.requiredMargin
			e.log.Debug("allocator: B&B selected %s %s/%s (no xfer)", choice.symbol, choice.longExchange, choice.shortExchange)
			next := append(current, choice)
			branch(idx+1, nextCap, slots-1, next, currentValue+choice.baseValue, nextNeeds)
		}

		branch(idx+1, cap, slots, current, currentValue, needs)
	}

	branch(0, cloneFloatMap(capacity), remainingSlots, nil, 0, exchNeedsMap{})
	e.log.Debug("allocator: solver done: greedy=%d/%.4f B&B=%d/%.4f improved=%v", greedyN, greedyVal, len(bestChoices), bestValue, bestValue > greedyVal)
	return bestChoices
}

func (e *Engine) greedyAllocatorSeed(candidates []allocatorCandidate, capacity map[string]float64, balances map[string]rebalanceBalanceInfo, feeCache map[string]feeEntry, remainingSlots int, xferBudget float64) []allocatorChoice {
	selected := make([]allocatorChoice, 0, remainingSlots)
	needs := exchNeedsMap{}
	for _, candidate := range candidates {
		if remainingSlots <= 0 {
			break
		}
		for _, choice := range candidate.choices {
			needsTransfer := capacity[choice.longExchange] < choice.requiredMargin || capacity[choice.shortExchange] < choice.requiredMargin
			if needsTransfer {
				fsp := firstSettlementProfit(choice.spreadBpsH, choice.intervalHours, choice.entryNotional)
				xferFee := estimateChoiceTransferFee(e, choice, capacity, balances, feeCache)
				if xferFee < 0 || fsp <= xferFee {
					e.log.Info("allocator: greedy reject xfer %s %s/%s fsp=%.4f xferFee=%.4f (need fsp>fee)",
						choice.symbol, choice.longExchange, choice.shortExchange, fsp, xferFee)
					continue
				}
				// Budget check: compute total deficit if this choice is added.
				testNeeds := make(exchNeedsMap, len(needs)+2)
				for k, v := range needs {
					testNeeds[k] = v
				}
				testNeeds[choice.longExchange] += choice.requiredMargin
				testNeeds[choice.shortExchange] += choice.requiredMargin
				totalDeficit := 0.0
				for exch, totalNeed := range testNeeds {
					if bal, ok := balances[exch]; ok {
						totalDeficit += e.computeExchangeDeficit(exch, totalNeed, bal)
					}
				}
				if totalDeficit > xferBudget {
					e.log.Debug("allocator: greedy budget exceeded %s %s/%s deficit=%.2f budget=%.2f", choice.symbol, choice.longExchange, choice.shortExchange, totalDeficit, xferBudget)
					continue
				}
				adjustedChoice := choice
				adjustedChoice.baseValue -= xferFee
				adjustedChoice.xferFeeDeducted = xferFee
				capacity[adjustedChoice.longExchange] = math.Max(0, capacity[adjustedChoice.longExchange]-adjustedChoice.requiredMargin)
				capacity[adjustedChoice.shortExchange] = math.Max(0, capacity[adjustedChoice.shortExchange]-adjustedChoice.requiredMargin)
				needs[choice.longExchange] += choice.requiredMargin
				needs[choice.shortExchange] += choice.requiredMargin
				e.log.Debug("allocator: greedy xfer selected %s %s/%s margin=%.2f fee=%.4f", adjustedChoice.symbol, adjustedChoice.longExchange, adjustedChoice.shortExchange, adjustedChoice.requiredMargin, adjustedChoice.xferFeeDeducted)
				selected = append(selected, adjustedChoice)
				remainingSlots--
				break
			}
			needs[choice.longExchange] += choice.requiredMargin
			needs[choice.shortExchange] += choice.requiredMargin
			capacity[choice.longExchange] -= choice.requiredMargin
			capacity[choice.shortExchange] -= choice.requiredMargin
			e.log.Debug("allocator: greedy selected %s %s/%s margin=%.2f capAfter: %s=%.2f %s=%.2f",
				choice.symbol, choice.longExchange, choice.shortExchange, choice.requiredMargin,
				choice.longExchange, capacity[choice.longExchange],
				choice.shortExchange, capacity[choice.shortExchange])
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

	feeCache := map[string]feeEntry{}
	for exchName, deficit := range deficits {
		remaining := deficit
		for remaining > 0 {
			donor, available, fee, ok := e.findAllocatorDonor(exchName, donorGross, balances)
			if !ok {
				return false
			}

			// Always floor deficit to donor's minWithdraw (cached).
			e.cfg.RLock()
			addrs := e.cfg.ExchangeAddresses[exchName]
			e.cfg.RUnlock()
			var chain string
			for _, c := range []string{"APT", "BEP20"} {
				if addrs[c] != "" {
					chain = c
					break
				}
			}
			if chain != "" {
				cacheKey := donor + ":" + chain
				entry, cached := feeCache[cacheKey]
				if !cached {
					_, minWd, err := e.exchanges[donor].GetWithdrawFee("USDT", chain)
					entry = feeEntry{minWd: minWd, valid: err == nil}
					feeCache[cacheKey] = entry
				}
				if entry.valid && remaining < entry.minWd {
					remaining = entry.minWd
				}
			}

			netPossible := available - fee
			if netPossible <= 0 {
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
		if donor == recipient || avail <= 0 {
			continue
		}
		if balances[donor].marginRatio >= e.cfg.MarginL3Threshold {
			e.log.Info("rebalance: donor %s skipped for %s: marginRatio %.4f >= L3 %.4f", donor, recipient, balances[donor].marginRatio, e.cfg.MarginL3Threshold)
			continue
		}
		fee, minWd, err := e.exchanges[donor].GetWithdrawFee("USDT", chain)
		if err != nil {
			continue
		}
		// FIX 3: fee-mode aware donor filter
		if minWd > 0 {
			isGross := e.exchanges[donor].WithdrawFeeInclusive()
			if isGross {
				if avail < minWd {
					e.log.Debug("rebalance: donor %s skipped: avail %.2f < minWd %.2f (gross)", donor, avail, minWd)
					continue
				}
			} else {
				if avail < minWd+fee {
					e.log.Debug("rebalance: donor %s skipped: avail %.2f < minWd+fee %.2f (net)", donor, avail, minWd+fee)
					continue
				}
			}
		}
		usable := avail - fee
		if usable <= 0 {
			e.log.Info("rebalance: donor %s skipped for %s: idle too low (%.2f, fee=%.4f)", donor, recipient, avail, fee)
			continue
		}
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

// firstSettlementProfit estimates the funding profit from one settlement interval.
func firstSettlementProfit(spreadBpsH, intervalHours, notional float64) float64 {
	return spreadBpsH * intervalHours * notional / 10000.0
}

// estimateChoiceTransferFee computes the cheapest withdraw fee needed to fund
// each leg of the choice that exceeds local capacity.
func estimateChoiceTransferFee(e *Engine, choice allocatorChoice, cap map[string]float64, balances map[string]rebalanceBalanceInfo, feeCache map[string]feeEntry) float64 {
	totalFee := 0.0
	if cap[choice.longExchange] < choice.requiredMargin {
		fee := cheapestTransferFee(e, choice.longExchange, balances, cap, feeCache)
		if fee < 0 {
			e.log.Debug("allocator: xferFee: no donor for long leg %s (%s)", choice.longExchange, choice.symbol)
			return -1 // no viable donor
		}
		totalFee += fee
	}
	if cap[choice.shortExchange] < choice.requiredMargin {
		fee := cheapestTransferFee(e, choice.shortExchange, balances, cap, feeCache)
		if fee < 0 {
			e.log.Debug("allocator: xferFee: no donor for short leg %s (%s)", choice.shortExchange, choice.symbol)
			return -1 // no viable donor
		}
		totalFee += fee
	}
	return totalFee
}

// cheapestTransferFee finds the cheapest withdraw fee from any donor to the
// recipient exchange. Returns -1 if no viable donor exists.
func cheapestTransferFee(e *Engine, recipient string, balances map[string]rebalanceBalanceInfo, cap map[string]float64, feeCache map[string]feeEntry) float64 {
	e.cfg.RLock()
	addrs := e.cfg.ExchangeAddresses[recipient]
	e.cfg.RUnlock()
	if len(addrs) == 0 {
		e.log.Debug("allocator: cheapestTransferFee: no addresses configured for %s", recipient)
		return -1
	}
	var chain string
	for _, candidate := range []string{"APT", "BEP20"} {
		if addr := addrs[candidate]; addr != "" {
			chain = candidate
			break
		}
	}
	if chain == "" {
		e.log.Debug("allocator: cheapestTransferFee: no supported chain for %s (have: %v)", recipient, addrs)
		return -1
	}

	bestFee := -1.0
	checkedCount := 0
	for donor := range balances {
		if donor == recipient {
			continue
		}
		checkedCount++
		if balances[donor].marginRatio >= e.cfg.MarginL3Threshold {
			e.log.Debug("allocator: cheapestFee skip donor %s for %s: ratio=%.4f>=L3=%.4f", donor, recipient, balances[donor].marginRatio, e.cfg.MarginL3Threshold)
			continue
		}
		cacheKey := donor + "|" + recipient + "|" + chain
		entry, ok := feeCache[cacheKey]
		if !ok {
			var err error
			var minWd float64
			entry.fee, minWd, err = e.exchanges[donor].GetWithdrawFee("USDT", chain)
			if err != nil {
				feeCache[cacheKey] = feeEntry{valid: false}
				continue
			}
			entry.minWd = minWd
			entry.valid = true
			feeCache[cacheKey] = entry
		}
		if !entry.valid {
			continue
		}
		fee := entry.fee
		minWd := entry.minWd
		// FIX 3: fee-mode aware donor filter
		if minWd > 0 {
			isGross := e.exchanges[donor].WithdrawFeeInclusive()
			if isGross {
				if cap[donor] < minWd {
					e.log.Debug("allocator: cheapestFee donor %s skipped: cap %.2f < minWd %.2f (gross)", donor, cap[donor], minWd)
					continue
				}
			} else {
				if cap[donor] < minWd+fee {
					e.log.Debug("allocator: cheapestFee donor %s skipped: cap %.2f < minWd+fee %.2f (net)", donor, cap[donor], minWd+fee)
					continue
				}
			}
		}
		// Check donor actually has surplus after its own allocator commitments.
		donorSurplus := cap[donor] // already reduced by prior choices in this solver round
		if balances[donor].hasPositions {
			healthCap := e.capByMarginHealth(balances[donor])
			if healthCap < donorSurplus {
				donorSurplus = healthCap
			}
		}
		if donorSurplus-fee <= 0 {
			continue
		}
		if bestFee < 0 || fee < bestFee {
			bestFee = fee
		}
	}
	if bestFee < 0 {
		e.log.Debug("allocator: cheapestTransferFee: no viable donor for %s (chain=%s, checked=%d)", recipient, chain, checkedCount)
	}
	return bestFee
}

func (e *Engine) allocatorChoiceValue(choices []allocatorChoice, balances map[string]rebalanceBalanceInfo, feeCache map[string]feeEntry) float64 {
	sum := 0.0
	alreadyDeducted := 0.0
	for _, choice := range choices {
		sum += choice.baseValue
		alreadyDeducted += choice.xferFeeDeducted
	}
	if len(choices) == 0 {
		return sum
	}

	needs := make(map[string]float64)
	for _, choice := range choices {
		needs[choice.longExchange] += choice.requiredMargin
		needs[choice.shortExchange] += choice.requiredMargin
	}
	transferCost := e.estimateAllocatorTransferCost(needs, balances, feeCache)
	// Subtract only the portion of transfer cost not already deducted from
	// baseValue of individual choices to avoid double-counting.
	netTransferCost := transferCost - alreadyDeducted
	if netTransferCost < 0 {
		netTransferCost = 0
	}
	return sum - netTransferCost
}

func (e *Engine) estimateAllocatorTransferCost(needs map[string]float64, balances map[string]rebalanceBalanceInfo, feeCache map[string]feeEntry) float64 {
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

		marginDeficit := need - bal.futures
		if marginDeficit < 0 {
			marginDeficit = 0
		}
		actualMargin := need / e.cfg.MarginSafetyMultiplier
		if actualMargin <= 0 {
			actualMargin = need
		}
		targetRatio := e.cfg.MarginL4Threshold - marginEpsilon
		freeTarget := 1.0 - targetRatio
		var ratioDeficit float64
		if freeTarget > 0 && bal.futuresTotal > 0 {
			ratioDeficit = (freeTarget*bal.futuresTotal - bal.futures + actualMargin) / targetRatio
			if ratioDeficit < 0 {
				ratioDeficit = 0
			}
		}
		transferAmt := marginDeficit
		if ratioDeficit > transferAmt {
			transferAmt = ratioDeficit
		}
		e.log.Debug("allocator: transferCost deficit %s: need=%.2f futures=%.2f marginDef=%.2f ratioDef=%.2f transferAmt=%.2f", name, need, bal.futures, marginDeficit, ratioDeficit, transferAmt)
		if bal.spot > 0 {
			actualTransfer := transferAmt
			if actualTransfer > bal.spot {
				actualTransfer = bal.spot
			}
			if actualTransfer >= 1.0 {
				transferAmt -= actualTransfer
			}
		}
		if transferAmt > 0 {
			crossDeficits = append(crossDeficits, deficit{exchange: name, amount: transferAmt})
		}
	}

	totalCost := 0.0
	for _, d := range crossDeficits {
		remaining := d.amount
		for remaining > 0 {
			donor, available, fee, ok := e.findAllocatorDonorWithCache(d.exchange, donorGross, balances, feeCache)
			if !ok {
				// No donor available. Use average fee from this cycle's cache,
				// filtering out -1 error sentinels. Fall back to config penalty
				// if no valid fees cached.
				var feeSum float64
				var feeCount int
				for _, ent := range feeCache {
					if ent.valid {
						feeSum += ent.fee
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
			if netPossible <= 0 {
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

func (e *Engine) findAllocatorDonorWithCache(recipient string, donorGross map[string]float64, balances map[string]rebalanceBalanceInfo, feeCache map[string]feeEntry) (string, float64, float64, bool) {
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
		if donor == recipient || avail <= 0 {
			continue
		}
		if balances[donor].marginRatio >= e.cfg.MarginL3Threshold {
			e.log.Info("rebalance: donor %s skipped for %s: marginRatio %.4f >= L3 %.4f", donor, recipient, balances[donor].marginRatio, e.cfg.MarginL3Threshold)
			continue
		}

		cacheKey := donor + "|" + recipient + "|" + chain
		entry, ok := feeCache[cacheKey]
		if !ok {
			var err error
			var minWd float64
			entry.fee, minWd, err = e.exchanges[donor].GetWithdrawFee("USDT", chain)
			if err != nil {
				feeCache[cacheKey] = feeEntry{valid: false}
				continue
			}
			entry.minWd = minWd
			entry.valid = true
			feeCache[cacheKey] = entry
		}
		if !entry.valid {
			continue
		}
		fee := entry.fee
		minWd := entry.minWd
		// FIX 3: fee-mode aware donor filter
		if minWd > 0 {
			isGross := e.exchanges[donor].WithdrawFeeInclusive()
			if isGross {
				if avail < minWd {
					continue
				}
			} else {
				if avail < minWd+fee {
					continue
				}
			}
		}

		usable := avail - fee
		if usable <= 0 {
			e.log.Info("rebalance: donor %s skipped for %s: idle too low (%.2f, fee=%.4f)", donor, recipient, avail, fee)
			continue
		}
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
	// Determine if this exchange uses a unified account (futures pool = main balance).
	// For unified accounts, rebalanceAvailable returns futures only — spot is already
	// included in the futures pool and must not be double-counted.
	isUnified := false
	type unifiedChecker interface{ IsUnified() bool }
	if uc, ok := e.exchanges[name].(unifiedChecker); ok && uc.IsUnified() {
		isUnified = true
	}

	// Compute transferable futures capacity.
	// Unified: the entire futures balance is the transferable pool.
	// Split: use maxTransferOut (from exchange API); fall back to futures with
	//        a margin-ratio safety cap if maxTransferOut is unavailable.
	var futuresAvail float64
	if isUnified {
		futuresAvail = bal.futures
	} else {
		futuresAvail = bal.maxTransferOut
		if futuresAvail <= 0 {
			futuresAvail = bal.futures
			if bal.marginRatio > 0 && bal.futuresTotal > 0 && e.cfg.MarginL4Threshold > 0 {
				safeMax := bal.futuresTotal * (1.0 - bal.marginRatio/e.cfg.MarginL4Threshold)
				if safeMax < futuresAvail {
					futuresAvail = safeMax
				}
			}
		}
	}

	// Gross surplus = transferable futures + spot (split only) - own need.
	var surplus float64
	if isUnified {
		// spot is part of the futures pool — don't add separately
		surplus = futuresAvail - localNeed
	} else {
		surplus = futuresAvail + bal.spot - localNeed
	}
	if surplus <= 0 {
		e.log.Debug("allocator: donorGross %s surplus=%.2f<=0", name, surplus)
		return 0
	}

	// Cap by margin health to avoid draining past L3/L4.
	if bal.hasPositions {
		healthCap := e.capByMarginHealth(bal)
		if healthCap < surplus {
			surplus = healthCap
		}
	}
	if surplus < 0 {
		return 0
	}

	e.log.Info("rebalance: donor %s capacity: futures=%.2f total=%.2f marginRatio=%.4f maxTransferOut=%.2f futuresAvail=%.2f surplus=%.2f unified=%v hasPos=%v",
		name, bal.futures, bal.futuresTotal, bal.marginRatio, bal.maxTransferOut, futuresAvail, surplus, isUnified, bal.hasPositions)

	return surplus
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

// simulateTransferPlan verifies that a set of allocator choices can be funded
// via cross-exchange transfers without pushing any exchange to L3+ health level.
// It clones exchange balances, simulates spot→futures moves and cross-exchange
// transfers for each deficit, then deducts actual margin for position opening
// and checks all post-state ratios stay below L3.
func (e *Engine) simulateTransferPlan(choices []allocatorChoice, balances map[string]rebalanceBalanceInfo) (feasible bool, totalFee float64) {
	// 1. Clone balances (rebalanceBalanceInfo is a value type — struct copy is safe).
	sim := make(map[string]rebalanceBalanceInfo, len(balances))
	for k, v := range balances {
		sim[k] = v
	}

	// 2. Compute aggregate margin needs per exchange from selected choices.
	needs := make(map[string]float64)
	for _, c := range choices {
		needs[c.longExchange] += c.requiredMargin
		needs[c.shortExchange] += c.requiredMargin
	}

	// 3. For each exchange with a deficit, simulate transfers.
	for exch, need := range needs {
		avail := e.rebalanceAvailable(exch, sim[exch])

		// Same-exchange spot→futures: move spot to futures so the post-trade
		// ratio check (which only uses futures balance) sees the full amount.
		// Skip unified-account exchanges where spot and futures share the same pool.
		isUnified := false
		if uc, ok := e.exchanges[exch].(interface{ IsUnified() bool }); ok && uc.IsUnified() {
			isUnified = true
		}
		if !isUnified && sim[exch].spot > 0 {
			b := sim[exch]
			move := b.spot
			e.log.Debug("allocator: simXfer spot→futures %s: %.2f", exch, move)
			b.futures += move
			b.spot = 0
			b.futuresTotal += move
			sim[exch] = b
			avail = e.rebalanceAvailable(exch, sim[exch])
		}

		// Cross-exchange transfers for remaining deficit.
		// Trigger when EITHER margin buffer is insufficient OR post-trade
		// ratio would exceed L4 after opening.
		wouldExceedL4 := false
		{
			bal := sim[exch]
			actualM := need / e.cfg.MarginSafetyMultiplier
			if actualM <= 0 {
				actualM = need
			}
			if bal.futuresTotal > 0 {
				postAvail := bal.futures - actualM
				if postAvail < 0 {
					postAvail = 0
				}
				postRatio := 1 - postAvail/bal.futuresTotal
				wouldExceedL4 = postRatio >= e.cfg.MarginL4Threshold
			}
		}
		if avail < need || wouldExceedL4 {
			marginDeficit := need - avail
			if marginDeficit < 0 {
				marginDeficit = 0
			}

			// Post-trade ratio deficit: ensure ratio stays below L4 after opening.
			// After transfer T and opening with margin M:
			//   ratio = 1 - (futures+T-M)/(total+T) < L4
			//   T >= ((1-L4)*total - futures + M) / L4
			actualMargin := need / e.cfg.MarginSafetyMultiplier
			if actualMargin <= 0 {
				actualMargin = need
			}
			bal := sim[exch]
			var ratioDeficit float64
			targetRatio := e.cfg.MarginL4Threshold - marginEpsilon
			freeTarget := 1.0 - targetRatio
			if freeTarget > 0 && bal.futuresTotal > 0 {
				ratioDeficit = (freeTarget*bal.futuresTotal - bal.futures + actualMargin) / targetRatio
			}

			deficit := marginDeficit
			if ratioDeficit > deficit {
				deficit = ratioDeficit
			}
			for deficit > 0 {
				bestDonor := ""
				bestSurplus := 0.0
				for donor, bal := range sim {
					if donor == exch {
						continue
					}
					if bal.marginRatio >= e.cfg.MarginL3Threshold {
						continue
					}
					surplus := e.rebalanceAvailable(donor, bal) - needs[donor]
					if bal.hasPositions {
						healthCap := e.capByMarginHealth(bal)
						if surplus > healthCap {
							surplus = healthCap
						}
					}
					if surplus > bestSurplus {
						bestDonor = donor
						bestSurplus = surplus
					}
				}
				if bestDonor == "" {
					e.log.Debug("allocator: simXfer no donor for %s deficit=%.2f", exch, deficit)
					return false, 0
				}

				move := math.Min(deficit, bestSurplus)

				// Floor to donor's minWithdraw (replaces hardcoded 10.0).
				e.cfg.RLock()
				addrs := e.cfg.ExchangeAddresses[exch]
				e.cfg.RUnlock()
				var simChain string
				for _, c := range []string{"APT", "BEP20"} {
					if addrs[c] != "" {
						simChain = c
						break
					}
				}
				minWd := 10.0 // fallback
				if simChain != "" {
					if _, mw, err := e.exchanges[bestDonor].GetWithdrawFee("USDT", simChain); err == nil {
						minWd = mw
					}
				}
				if move < minWd {
					if bestSurplus < minWd {
						e.log.Debug("allocator: simXfer %s→%s bestSurplus=%.2f < minWd=%.2f, infeasible", bestDonor, exch, bestSurplus, minWd)
						return false, 0
					}
					move = minWd
				}

				e.log.Debug("allocator: simXfer %s→%s move=%.2f", bestDonor, exch, move)
				d := sim[bestDonor]
				d.futures -= move
				d.futuresTotal -= move
				sim[bestDonor] = d

				r := sim[exch]
				r.futures += move
				r.futuresTotal += move
				sim[exch] = r

				deficit -= move
				totalFee += 0.1 // conservative per-transfer fee estimate
			}
		}
	}

	// 4. Simulate position opening — deduct actual margin (not the buffered amount).
	for _, c := range choices {
		margin := c.requiredMargin / e.cfg.MarginSafetyMultiplier
		if margin <= 0 {
			margin = c.requiredMargin
		}

		lb := sim[c.longExchange]
		lb.futures -= margin
		sim[c.longExchange] = lb

		sb := sim[c.shortExchange]
		sb.futures -= margin
		sim[c.shortExchange] = sb
	}

	// 5. Check only INVOLVED exchanges' post-state margin ratio stays below L4.
	// Involved = exchanges used as legs in selected choices + donor exchanges.
	// Uses the existing L4 threshold (same as the pre-existing post-trade check).
	involved := make(map[string]bool)
	for _, c := range choices {
		involved[c.longExchange] = true
		involved[c.shortExchange] = true
	}
	// Also mark donor exchanges (any exchange whose balance decreased).
	for name, bal := range sim {
		if bal.futures < balances[name].futures {
			involved[name] = true
		}
	}
	for name := range involved {
		bal := sim[name]
		if bal.futuresTotal <= 0 {
			continue
		}
		futures := bal.futures
		if futures < 0 {
			futures = 0
		}
		ratio := 1 - futures/bal.futuresTotal
		if ratio >= e.cfg.MarginL4Threshold {
			e.log.Info("simulateTransferPlan: %s would reach ratio=%.4f >= L4=%.4f, rejecting",
				name, ratio, e.cfg.MarginL4Threshold)
			return false, 0
		}
	}

	return true, totalFee
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

func (e *Engine) executeRebalanceFundingPlan(needs map[string]float64, balances map[string]rebalanceBalanceInfo, precomputedDeficits []rebalanceDeficit) {
	var crossDeficits []rebalanceDeficit

	if precomputedDeficits != nil {
		// Sequential rebalance: deficits already computed, skip spot→futures phase.
		crossDeficits = precomputedDeficits
	} else {
	for name, need := range needs {
		bal := balances[name]
		targetFreeRatio := 1 - e.cfg.MarginL4Threshold
		if targetFreeRatio <= 0 {
			targetFreeRatio = 0.20
		}

		if bal.futures >= need {
			isUnified := false
			if uc, ok := e.exchanges[name].(interface{ IsUnified() bool }); ok && uc.IsUnified() {
				isUnified = true
			}

			// Non-unified: try spot→futures ratio relief (existing logic)
			if !isUnified && bal.spot > 0 && bal.futuresTotal > 0 {
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
				continue
			}

			// Unified account or no spot: check if post-trade ratio would breach L4.
			// If so, queue a cross-exchange transfer for the ratio deficit.
			if bal.futuresTotal > 0 {
				actualMargin := need / e.cfg.MarginSafetyMultiplier
				if actualMargin <= 0 {
					actualMargin = need
				}
				projectedAvail := bal.futures - actualMargin
				if projectedAvail < 0 {
					projectedAvail = 0
				}
				projectedRatio := 1 - projectedAvail/bal.futuresTotal
				if projectedRatio >= e.cfg.MarginL4Threshold {
					// Compute ratio deficit: how much extra needed so ratio < L4
					targetRatio := e.cfg.MarginL4Threshold - marginEpsilon
					freeTarget := 1.0 - targetRatio
					ratioDeficit := (freeTarget*bal.futuresTotal - bal.futures + actualMargin) / targetRatio
					if ratioDeficit > 0 {
						e.log.Info("rebalance: %s post-trade ratio=%.4f >= L4=%.4f, queueing cross-exchange deficit=%.2f",
							name, projectedRatio, e.cfg.MarginL4Threshold, ratioDeficit)
						crossDeficits = append(crossDeficits, rebalanceDeficit{name, ratioDeficit})
					}
				}
			}
			continue
		}

		marginDeficit := need - bal.futures
		if marginDeficit < 0 {
			marginDeficit = 0
		}
		actualMargin := need / e.cfg.MarginSafetyMultiplier
		if actualMargin <= 0 {
			actualMargin = need
		}
		targetRatio := e.cfg.MarginL4Threshold - marginEpsilon
		freeTarget := 1.0 - targetRatio
		var ratioDeficit float64
		if freeTarget > 0 && bal.futuresTotal > 0 {
			ratioDeficit = (freeTarget*bal.futuresTotal - bal.futures + actualMargin) / targetRatio
			if ratioDeficit < 0 {
				ratioDeficit = 0
			}
		}
		transferAmt := marginDeficit
		if ratioDeficit > transferAmt {
			transferAmt = ratioDeficit
		}
		e.log.Debug("rebalance: deficit %s: need=%.2f futures=%.2f marginDef=%.2f ratioDef=%.2f transferAmt=%.2f", name, need, bal.futures, marginDeficit, ratioDeficit, transferAmt)
		if bal.spot > 0 {
			actualTransfer := transferAmt
			if actualTransfer > bal.spot {
				actualTransfer = bal.spot
			}
			if actualTransfer < 1.0 {
				e.log.Debug("rebalance: %s spot->futures skip (%.4f USDT below minimum)", name, actualTransfer)
				if transferAmt > 0 {
					crossDeficits = append(crossDeficits, rebalanceDeficit{name, transferAmt})
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
		if transferAmt > 0 {
			crossDeficits = append(crossDeficits, rebalanceDeficit{name, transferAmt})
		}
	}
	} // end else (precomputedDeficits == nil)

	if len(crossDeficits) == 0 {
		e.log.Info("rebalance: all exchanges funded, no cross-exchange transfers needed")
		return
	}

	surplus := map[string]float64{}
	for name := range e.exchanges {
		surplus[name] = e.allocatorDonorGrossCapacity(name, balances[name], needs[name])
	}

	pendingDeposits := map[string]float64{}
	pendingStartBal := map[string]float64{}
	for i := range crossDeficits {
		remaining := crossDeficits[i].amount
		exchName := crossDeficits[i].exchange
		e.log.Debug("rebalance: processing crossDeficit %s amount=%.2f", exchName, remaining)
		// FIX 9: removed remaining < 1.0 skip — intentional over-transfer when deficit < minWd
		for remaining > 0 {
			var bestDonor string
			var bestSurplus float64
			for name, s := range surplus {
				if name == exchName || s <= 0 {
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

			fee, minWd, feeErr := e.exchanges[bestDonor].GetWithdrawFee("USDT", chain)
			if feeErr != nil {
				e.log.Warn("rebalance: %s GetWithdrawFee failed: %v, skipping donor", bestDonor, feeErr)
				surplus[bestDonor] = 0
				continue
			}
			e.log.Info("rebalance: %s withdraw fee=%.4f minWd=%.4f USDT via %s", bestDonor, fee, minWd, chain)

			// FIX 6a: fee-mode aware cap — keep existing net-fee path, add fee-inclusive path.
			isGross := e.exchanges[bestDonor].WithdrawFeeInclusive()
			if !isGross && balances[bestDonor].hasPositions {
				// net-fee: actual debit = contribution + fee; cap so total stays within surplus
				if contribution+fee > bestSurplus {
					contribution = bestSurplus - fee
					e.log.Debug("rebalance: net-fee cap %s: contribution=%.2f fee=%.2f surplus=%.2f", bestDonor, contribution, fee, bestSurplus)
				}
			} else if isGross && balances[bestDonor].hasPositions {
				// fee-inclusive: withdrawAmtForAPI = contribution + fee, actual debit = contribution + fee
				if contribution+fee > bestSurplus {
					contribution = bestSurplus - fee
					e.log.Debug("rebalance: gross-fee cap %s: contribution=%.2f fee=%.2f surplus=%.2f", bestDonor, contribution, fee, bestSurplus)
				}
			}
			if contribution <= 0 {
				e.log.Warn("rebalance: %s contribution %.2f too low after fee %.4f deduction, skipping", bestDonor, contribution, fee)
				surplus[bestDonor] = 0
				continue
			}

			// FIX 8: minWd early bump — bump contribution up to netFloor if too small
			// fee-inclusive: withdrawAmtForAPI = contribution + fee, so need contribution >= minWd - fee
			// net-fee: withdrawAmtForAPI = contribution, so need contribution >= minWd
			if minWd > 0 {
				var netFloor float64
				if isGross {
					netFloor = minWd - fee
				} else {
					netFloor = minWd
				}
				if netFloor < 0 {
					netFloor = 0
				}
				if contribution < netFloor {
					cappedFloor := math.Min(netFloor, bestSurplus-fee)
					if cappedFloor <= 0 {
						e.log.Debug("rebalance: %s contribution %.2f below minWd net floor %.2f and no room to bump, skipping", bestDonor, contribution, netFloor)
						surplus[bestDonor] = 0
						continue
					}
					e.log.Info("rebalance: %s bumping contribution %.2f -> %.2f to meet minWd net floor %.2f", bestDonor, contribution, cappedFloor, netFloor)
					contribution = cappedFloor
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
			// Only skip futures→spot transfer for unified-account exchanges
			// (where futures and spot share the same balance pool).
			skipOuterTransfer := false
			if uc, ok := e.exchanges[bestDonor].(interface{ IsUnified() bool }); ok && uc.IsUnified() {
				skipOuterTransfer = true
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

			// Unified accounts: spot balance is 0 (same pool as futures).
			// Use GetFuturesBalance to get the real withdrawable amount.
			var donorSpotBal *exchange.Balance
			var donorBalErr error
			if uc, ok := e.exchanges[bestDonor].(interface{ IsUnified() bool }); ok && uc.IsUnified() {
				donorSpotBal, donorBalErr = e.exchanges[bestDonor].GetFuturesBalance()
			} else {
				donorSpotBal, donorBalErr = e.exchanges[bestDonor].GetSpotBalance()
			}
			if donorBalErr != nil {
				e.log.Error("rebalance: %s get spot balance failed: %v", bestDonor, donorBalErr)
				surplus[bestDonor] = 0
				continue
			}

			grossRequired := netAmount + fee
			// isGross was already declared above in FIX 6a block; reuse it here.

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
				e.log.Debug("rebalance: adjusted netAmount %s: spotAvail=%.2f gross=%.2f net=%.2f", bestDonor, donorSpotBal.Available, grossRequired, netAmount)
			}
			if netAmount <= 0 {
				e.log.Warn("rebalance: %s spot balance too low to withdraw (netAmount=%.4f, fee=%.4f)", bestDonor, netAmount, fee)
				surplus[bestDonor] = 0
				continue
			}

			withdrawAmtForAPI := netAmount
			if isGross {
				withdrawAmtForAPI = netAmount + fee
			}

			// FIX 8: final guard — skip if withdrawal amount is below exchange minimum
			if minWd > 0 && withdrawAmtForAPI < minWd {
				e.log.Info("rebalance: %s withdraw %.4f below min %.2f after late caps, skipping", bestDonor, withdrawAmtForAPI, minWd)
				surplus[bestDonor] = 0
				continue
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
				// Unified accounts: use futures balance as baseline (deposits land in unified pool).
				if uc, ok := e.exchanges[exchName].(interface{ IsUnified() bool }); ok && uc.IsUnified() {
					if fb, err := e.exchanges[exchName].GetFuturesBalance(); err == nil {
						pendingStartBal[exchName] = fb.Available
					} else {
						e.log.Warn("rebalance: %s unified GetFuturesBalance for deposit baseline failed: %v, using spot fallback", exchName, err)
						pendingStartBal[exchName] = balances[exchName].spot
					}
				} else {
					pendingStartBal[exchName] = balances[exchName].spot
				}
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
			// Unified accounts: deposits land in unified pool, poll via GetFuturesBalance.
			var spotBal *exchange.Balance
			var pollErr error
			if uc, ok := recipientExch.(interface{ IsUnified() bool }); ok && uc.IsUnified() {
				spotBal, pollErr = recipientExch.GetFuturesBalance()
			} else {
				spotBal, pollErr = recipientExch.GetSpotBalance()
			}
			if pollErr != nil {
				continue
			}
			if spotBal.Available >= startBal+totalPending*0.9 {
				arrived = true
				e.log.Info("rebalance: all deposits confirmed on %s (bal=%.2f)", recipient, spotBal.Available)
				break
			}
		}
		if !arrived {
			e.log.Warn("rebalance: deposits on %s not confirmed within 5min, skipping spot->futures", recipient)
			continue
		}
		if totalPending <= 0 {
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

func findOppBySymbol(opps []models.Opportunity, symbol string) *models.Opportunity {
	for i := range opps {
		if opps[i].Symbol == symbol {
			return &opps[i]
		}
	}
	return nil
}
