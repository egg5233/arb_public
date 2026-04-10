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

type transferStep struct {
	From, To string
	Amount   float64
	Fee      float64
	Chain    string
}

type dryRunResult struct {
	Feasible   bool
	TotalFee   float64
	Steps      []transferStep
	PostRatios map[string]float64
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
			if bal.marginRatio >= e.cfg.MarginL4Threshold {
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
		if bal.marginRatio >= e.cfg.MarginL4Threshold {
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
	feeCache := map[string]feeEntry{}
	selected := e.solveAllocator(candidates, capacity, balances, remainingSlots, timeout, feeCache)
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

	preCheck := e.dryRunTransferPlan(selected, balances, feeCache)
	feasible := preCheck.Feasible

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
		recheck := e.dryRunTransferPlan(selected, balances, feeCache)
		feasible = recheck.Feasible
	}

	// Post-solver simulation: verify that planned transfers + position openings
	// won't push any exchange to L4+ health level.
	if feasible && len(selected) > 0 {
		simResult := e.dryRunTransferPlan(selected, balances, feeCache)
		for !simResult.Feasible && len(selected) > 0 {
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
				simResult = e.dryRunTransferPlan(selected, balances, feeCache)
			}
		}
		if simResult.TotalFee > 0 {
			e.log.Info("allocator: simulation passed, estimated transfer fees=%.4f", simResult.TotalFee)
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

	// Step 6: No rerun — solver uses dryRunTransferPlan for all feasibility checks,
	// so reducing capacity has no effect. Both dryRunTransferPlan and executor use
	// sorted recipient iteration and deterministic donor tie-breaks for parity.
	// When pool allocator fails (feasible=false), Step 7 falls through to
	// the sequential path which handles per-opp donor rescue independently.

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

func (e *Engine) solveAllocator(candidates []allocatorCandidate, capacity map[string]float64, balances map[string]rebalanceBalanceInfo, remainingSlots int, timeout time.Duration, feeCache map[string]feeEntry) []allocatorChoice {
	start := time.Now()
	bestValue := -1.0
	var bestChoices []allocatorChoice

	incumbent := e.greedyAllocatorSeed(candidates, cloneFloatMap(capacity), balances, feeCache, remainingSlots)
	if len(incumbent) > 0 {
		result := e.dryRunTransferPlan(incumbent, balances, feeCache)
		if result.Feasible {
			bestValue = 0
			for _, c := range incumbent {
				bestValue += c.baseValue
			}
			bestValue -= result.TotalFee
			bestChoices = append(bestChoices, incumbent...)
		}
	}
	greedyN := len(incumbent)
	greedyVal := bestValue

	var branch func(int, map[string]float64, int, []allocatorChoice, float64)
	branch = func(idx int, cap map[string]float64, slots int, current []allocatorChoice, currentValue float64) {
		if time.Since(start) >= timeout {
			e.log.Debug("allocator: B&B timeout after %v, best=%d choices", time.Since(start), len(bestChoices))
			return
		}
		if idx >= len(candidates) || slots <= 0 {
			// Leaf evaluation via dryRunTransferPlan.
			result := e.dryRunTransferPlan(current, balances, feeCache)
			if !result.Feasible {
				return
			}
			score := 0.0
			for _, c := range current {
				score += c.baseValue
			}
			score -= result.TotalFee
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
			nextCap := cloneFloatMap(cap)
			nextCap[choice.longExchange] = math.Max(0, nextCap[choice.longExchange]-choice.requiredMargin)
			nextCap[choice.shortExchange] = math.Max(0, nextCap[choice.shortExchange]-choice.requiredMargin)
			e.log.Debug("allocator: B&B try %s %s/%s", choice.symbol, choice.longExchange, choice.shortExchange)
			next := append(current, choice)
			branch(idx+1, nextCap, slots-1, next, currentValue+choice.baseValue)
		}

		branch(idx+1, cap, slots, current, currentValue)
	}

	branch(0, cloneFloatMap(capacity), remainingSlots, nil, 0)
	e.log.Debug("allocator: solver done: greedy=%d/%.4f B&B=%d/%.4f improved=%v", greedyN, greedyVal, len(bestChoices), bestValue, bestValue > greedyVal)
	return bestChoices
}

func (e *Engine) greedyAllocatorSeed(candidates []allocatorCandidate, capacity map[string]float64, balances map[string]rebalanceBalanceInfo, feeCache map[string]feeEntry, remainingSlots int) []allocatorChoice {
	selected := make([]allocatorChoice, 0, remainingSlots)
	for _, candidate := range candidates {
		if remainingSlots <= 0 {
			break
		}
		for _, choice := range candidate.choices {
			// Try adding this choice and validate via dryRunTransferPlan.
			trial := append(append([]allocatorChoice(nil), selected...), choice)
			result := e.dryRunTransferPlan(trial, balances, feeCache)
			if !result.Feasible {
				e.log.Debug("allocator: greedy skip %s %s/%s: dryRun infeasible", choice.symbol, choice.longExchange, choice.shortExchange)
				continue
			}
			capacity[choice.longExchange] = math.Max(0, capacity[choice.longExchange]-choice.requiredMargin)
			capacity[choice.shortExchange] = math.Max(0, capacity[choice.shortExchange]-choice.requiredMargin)
			e.log.Debug("allocator: greedy selected %s %s/%s margin=%.2f",
				choice.symbol, choice.longExchange, choice.shortExchange, choice.requiredMargin)
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



func cloneFloatMap(src map[string]float64) map[string]float64 {
	dst := make(map[string]float64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
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

	// Cap by margin health to avoid draining past L4.
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
// of an exchange without pushing its margin ratio past the L4 threshold.
// Returns math.MaxFloat64 when the exchange has no positions or the
// calculation is not applicable (no margin data).
func (e *Engine) capByMarginHealth(bal rebalanceBalanceInfo) float64 {
	if !bal.hasPositions {
		return math.MaxFloat64
	}
	if bal.marginRatio <= 0 || bal.futuresTotal <= 0 || e.cfg.MarginL4Threshold <= 0 {
		return math.MaxFloat64
	}
	// maint = marginRatio * futuresTotal (estimated maintenance margin)
	// After removing X: newRatio = maint / (futuresTotal - X) must stay < L4
	// Solve: X < futuresTotal - maint / L4
	maint := bal.marginRatio * bal.futuresTotal
	cap := bal.futuresTotal - maint/e.cfg.MarginL4Threshold
	if cap < 0 {
		return 0
	}
	return cap
}

// dryRunTransferPlan simulates the entire transfer + position-opening plan
// and returns a detailed result with feasibility, total fees, steps, and
// post-trade margin ratios. This replaces multiple independent transfer
// evaluation paths with a single oracle that mirrors executor semantics.
func (e *Engine) dryRunTransferPlan(choices []allocatorChoice, balances map[string]rebalanceBalanceInfo, feeCache map[string]feeEntry) dryRunResult {
	// 1. Clone balances into sim map.
	sim := make(map[string]rebalanceBalanceInfo, len(balances))
	for k, v := range balances {
		sim[k] = v
	}

	// 2. Aggregate margin needs per exchange from all choices.
	needs := make(map[string]float64)
	for _, c := range choices {
		needs[c.longExchange] += c.requiredMargin
		needs[c.shortExchange] += c.requiredMargin
	}

	var steps []transferStep
	totalFee := 0.0

	// 3. For each exchange with need > 0, simulate transfers.
	// Sort recipients for deterministic iteration (Go map order is random).
	sortedRecipients := make([]string, 0, len(needs))
	for exch := range needs {
		sortedRecipients = append(sortedRecipients, exch)
	}
	sort.Strings(sortedRecipients)
	// Pass 1: Do all local spot→futures moves first (matches executor which does this
	// before building surplus map at allocator.go:1210, 1229).
	for _, exch := range sortedRecipients {
		need := needs[exch]
		isUnified := false
		if uc, ok := e.exchanges[exch].(interface{ IsUnified() bool }); ok && uc.IsUnified() {
			isUnified = true
		}
		if !isUnified && sim[exch].spot > 0 {
			if sim[exch].futures >= need {
				// Executor path: futures sufficient, check ratio relief only (allocator.go:1103-1113)
				if sim[exch].futuresTotal <= 0 {
					continue // no total = no ratio concern
				}
				projectedAvail := sim[exch].futures - need
				if projectedAvail < 0 {
					projectedAvail = 0
				}
				projectedRatio := 1 - projectedAvail/sim[exch].futuresTotal
				if projectedRatio >= e.cfg.MarginL4Threshold {
					targetFreeRatio := 1 - e.cfg.MarginL4Threshold
					if targetFreeRatio <= 0 {
						targetFreeRatio = 0.20
					}
					extra := (need - sim[exch].futuresTotal*targetFreeRatio) / targetFreeRatio
					if extra > sim[exch].spot {
						extra = sim[exch].spot
					}
					if extra >= 1.0 {
						b := sim[exch]
						b.futures += extra
						b.spot -= extra
						b.futuresTotal += extra
						sim[exch] = b
						e.log.Debug("dryRun: spot→futures %s: %.2f (ratio relief)", exch, extra)
					}
				}
			} else {
				// Executor path: futures insufficient, use max(marginDeficit, ratioDeficit)
				// to match executor at allocator.go:1171, 1188, 1193.
				marginDef := need - sim[exch].futures
				if marginDef < 0 {
					marginDef = 0
				}
				var ratioDef float64
				if sim[exch].futuresTotal > 0 {
					targetFR := 1 - e.cfg.MarginL4Threshold
					if targetFR <= 0 {
						targetFR = 0.20
					}
					ratioDef = (need - sim[exch].futuresTotal*targetFR) / targetFR
					if ratioDef < 0 {
						ratioDef = 0
					}
				}
				localMove := marginDef
				if ratioDef > localMove {
					localMove = ratioDef
				}
				if localMove > sim[exch].spot {
					localMove = sim[exch].spot
				}
				if localMove >= 1.0 {
					b := sim[exch]
					b.futures += localMove
					b.spot -= localMove
					b.futuresTotal += localMove
					sim[exch] = b
					e.log.Debug("dryRun: spot→futures %s: %.2f (deficit cover)", exch, localMove)
				}
			}
		}
	}

	// Compute donor budgets AFTER all local moves (matches executor at allocator.go:1210, 1229).
	donorBudget := make(map[string]float64, len(sim))
	for name, bal := range sim {
		donorBudget[name] = e.allocatorDonorGrossCapacity(name, bal, needs[name])
	}

	// Pass 2: Cross-exchange transfers for remaining deficits.
	triedDonors := map[string]bool{}
	for _, exch := range sortedRecipients {
		need := needs[exch]

		// Compute deficit for cross-exchange transfer.
		avail := sim[exch].futures
		marginDeficit := need - avail
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
		if freeTarget > 0 && sim[exch].futuresTotal > 0 {
			ratioDeficit = (freeTarget*sim[exch].futuresTotal - sim[exch].futures + actualMargin) / targetRatio
			if ratioDeficit < 0 {
				ratioDeficit = 0
			}
		}
		deficit := marginDeficit
		if ratioDeficit > deficit {
			deficit = ratioDeficit
		}

		// 3c. While deficit > 0, find best donor.
		for deficit > 0 {
			bestDonor := ""
			bestSurplus := 0.0
			for donor := range sim {
				if donor == exch || triedDonors[donor] {
					continue
				}
				if sim[donor].marginRatio >= e.cfg.MarginL4Threshold {
					continue
				}
				// Use pre-computed donor budget (decremented after each use)
				surplus := donorBudget[donor]
				// Deterministic tie-break: pick highest surplus, then alphabetical name
				if surplus > bestSurplus || (surplus == bestSurplus && (bestDonor == "" || donor < bestDonor)) {
					bestDonor = donor
					bestSurplus = surplus
				}
			}
			if bestDonor == "" {
				e.log.Debug("dryRun: no donor for %s deficit=%.2f", exch, deficit)
				return dryRunResult{Feasible: false}
			}

			// Determine if donor is unified.
			donorIsUnified := false
			if uc, ok := e.exchanges[bestDonor].(interface{ IsUnified() bool }); ok && uc.IsUnified() {
				donorIsUnified = true
			}

			// Fee lookup FIRST (before any sim mutation), so failed fee doesn't corrupt sim.
			e.cfg.RLock()
			addrs := e.cfg.ExchangeAddresses[exch]
			e.cfg.RUnlock()
			var chain string
			for _, c := range []string{"APT", "BEP20"} {
				if addrs[c] != "" {
					chain = c
					break
				}
			}
			if chain == "" {
				e.log.Debug("dryRun: no chain for %s", exch)
				// Mark this donor as tried-and-failed, continue to find another
				triedDonors[bestDonor] = true
				donorBudget[bestDonor] = 0 // zero globally, matching executor
				continue
			}
			cacheKey := bestDonor + "|" + exch + "|" + chain
			entry, cached := feeCache[cacheKey]
			if !cached {
				fee, minWd, err := e.exchanges[bestDonor].GetWithdrawFee("USDT", chain)
				if err != nil {
					feeCache[cacheKey] = feeEntry{valid: false}
					e.log.Debug("dryRun: fee lookup failed %s→%s: %v", bestDonor, exch, err)
					triedDonors[bestDonor] = true
					continue // skip this donor, try next
				}
				entry = feeEntry{fee: fee, minWd: minWd, valid: true}
				feeCache[cacheKey] = entry
			}
			if !entry.valid {
				triedDonors[bestDonor] = true
				donorBudget[bestDonor] = 0 // zero globally, matching executor
				continue // skip this donor, try next
			}
			fee := entry.fee
			minWd := entry.minWd

			// Donor futures→spot prep (split-account only, AFTER fee resolved).
			// Mirrors executor: resolve fee first, compute exact requiredSpot, then prep.
			// Must happen BEFORE donorAvailable check so spot reflects transferable funds.
			if !donorIsUnified {
				// Estimate required spot matching executor pattern (allocator.go:1311, 1330):
				// 1. Cap net move by bestSurplus (same as move will be capped later)
				// 2. Apply minWd floor (fee-mode aware)
				// 3. requiredSpot = cappedNet + fee
				isGrossPrep := e.exchanges[bestDonor].WithdrawFeeInclusive()
				estNet := deficit
				if estNet > bestSurplus {
					estNet = bestSurplus // cap by donor surplus, matching later move cap
				}
				if isGrossPrep {
					netFloor := minWd - fee
					if netFloor > estNet {
						estNet = netFloor
					}
				} else {
					if minWd > estNet {
						estNet = minWd
					}
				}
				// Final cap: match later budgetNet cap (donorBudget - fee)
				budgetNetPrep := donorBudget[bestDonor] - fee
				if budgetNetPrep < 0 {
					budgetNetPrep = 0
				}
				if estNet > budgetNetPrep {
					estNet = budgetNetPrep
				}
				if estNet > bestSurplus {
					estNet = bestSurplus
				}
				// If estNet below minWd after all caps, skip this donor
				if !isGrossPrep && estNet < minWd && bestSurplus < minWd {
					triedDonors[bestDonor] = true
					continue
				}
				if isGrossPrep && estNet < (minWd-fee) && bestSurplus < (minWd-fee) {
					triedDonors[bestDonor] = true
					continue
				}
				requiredSpot := estNet + fee
				shortfall := requiredSpot - sim[bestDonor].spot
				if shortfall > 0 {
					donorBal := sim[bestDonor]
					// Subtract own margin needs first, then cap (executor pattern at allocator.go:1348-1364)
					maxMove := donorBal.maxTransferOut
					if maxMove <= 0 {
						maxMove = donorBal.futures
					}
					maxMove -= needs[bestDonor]
					if donorBal.maxTransferOut > 0 && maxMove > donorBal.maxTransferOut {
						maxMove = donorBal.maxTransferOut
					}
					healthCap := e.capByMarginHealth(donorBal) - needs[bestDonor]
					if healthCap < maxMove {
						maxMove = healthCap
					}
					movedToSpot := shortfall
					if movedToSpot > maxMove {
						movedToSpot = maxMove
					}
					if movedToSpot > donorBal.futures {
						movedToSpot = donorBal.futures
					}
					if movedToSpot > 0 {
						d := sim[bestDonor]
						d.futures -= movedToSpot
						d.spot += movedToSpot
						sim[bestDonor] = d
						e.log.Debug("dryRun: donor %s futures→spot %.2f (need=%.2f)", bestDonor, movedToSpot, requiredSpot)
					}
				}
			}

			// Determine available amount from donor (after futures→spot prep).
			var donorAvailable float64
			if donorIsUnified {
				donorAvailable = sim[bestDonor].futures - needs[bestDonor] - fee
			} else {
				donorAvailable = sim[bestDonor].spot - fee
			}
			if donorAvailable <= 0 {
				e.log.Debug("dryRun: donor %s no available after prep+fee (avail=%.2f fee=%.4f)", bestDonor, donorAvailable, fee)
				triedDonors[bestDonor] = true
				donorBudget[bestDonor] = 0 // zero globally, matching executor
				continue
			}

			// Cap move by donor surplus (mirrors executor at allocator.go:1203)
			move := deficit
			if move > donorAvailable {
				move = donorAvailable
			}
			if move > bestSurplus {
				move = bestSurplus
			}
			// Cap by donor budget minus fee (executor caps contribution = bestSurplus - fee
			// and skips when non-positive, at allocator.go:1298)
			budgetNet := donorBudget[bestDonor] - fee
			if budgetNet <= 0 {
				e.log.Debug("dryRun: donor %s budget %.2f <= fee %.4f, skip", bestDonor, donorBudget[bestDonor], fee)
				triedDonors[bestDonor] = true
				donorBudget[bestDonor] = 0 // zero globally, matching executor
				continue
			}
			if move > budgetNet {
				move = budgetNet
			}

			// MinWd floor (fee-mode aware).
			isGross := e.exchanges[bestDonor].WithdrawFeeInclusive()
			var effectiveFloor float64
			if isGross {
				effectiveFloor = minWd - fee
				if effectiveFloor < 0 {
					effectiveFloor = 0
				}
			} else {
				effectiveFloor = minWd
			}
			// Cap effectiveFloor by donor budget (matching executor's min(netFloor, bestSurplus-fee))
			budgetCap := donorBudget[bestDonor] - fee
			if budgetCap < 0 {
				budgetCap = 0
			}
			if effectiveFloor > budgetCap {
				effectiveFloor = budgetCap
			}
			if effectiveFloor > 0 && move < effectiveFloor {
				if donorAvailable >= effectiveFloor {
					move = effectiveFloor
				} else {
					e.log.Debug("dryRun: donor %s avail %.2f < minWd floor %.2f, skip", bestDonor, donorAvailable, effectiveFloor)
					triedDonors[bestDonor] = true
					continue
				}
			}

			// Final minWd reject: if actual withdrawal amount would be below exchange minimum,
			// the executor skips this donor (allocator.go:1440). Match that here.
			withdrawAmt := move
			if isGross {
				withdrawAmt = move + fee
			}
			if minWd > 0 && withdrawAmt < minWd {
				e.log.Debug("dryRun: donor %s withdrawAmt %.4f < minWd %.2f after caps, skip", bestDonor, withdrawAmt, minWd)
				triedDonors[bestDonor] = true
				donorBudget[bestDonor] = 0 // zero globally, matching executor
				continue
			}

			// Apply to sim.
			if donorIsUnified {
				d := sim[bestDonor]
				d.futures -= move + fee
				d.futuresTotal -= move + fee
				sim[bestDonor] = d
			} else {
				// Split-account: withdrawal from spot only. Do NOT reduce futuresTotal —
				// split-account spot is separate from futures total (plan v12 line 86).
				d := sim[bestDonor]
				d.spot -= move + fee
				sim[bestDonor] = d
			}
			r := sim[exch]
			r.futures += move
			r.futuresTotal += move
			sim[exch] = r

			steps = append(steps, transferStep{From: bestDonor, To: exch, Amount: move, Fee: fee, Chain: chain})
			totalFee += fee
			deficit -= move
			// Decrement donor budget (matches executor's surplus[bestDonor] -= netAmount + fee)
			donorBudget[bestDonor] -= move + fee
		}
	}

	// 4. Simulate position opening: deduct actual margin per choice.
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

	// 5. Post-trade L4 check on all involved exchanges.
	involved := make(map[string]bool)
	for _, c := range choices {
		involved[c.longExchange] = true
		involved[c.shortExchange] = true
	}
	for name := range sim {
		if sim[name].futures < balances[name].futures {
			involved[name] = true
		}
	}
	postRatios := make(map[string]float64, len(involved))
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
		postRatios[name] = ratio
		if ratio >= e.cfg.MarginL4Threshold {
			e.log.Debug("dryRun: %s post-ratio=%.4f >= L4=%.4f, infeasible", name, ratio, e.cfg.MarginL4Threshold)
			return dryRunResult{Feasible: false}
		}
	}

	return dryRunResult{
		Feasible:   true,
		TotalFee:   totalFee,
		Steps:      steps,
		PostRatios: postRatios,
	}
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
	// Sort recipients for deterministic iteration (matches dryRunTransferPlan).
	sortedNeeds := make([]string, 0, len(needs))
	for name := range needs {
		sortedNeeds = append(sortedNeeds, name)
	}
	sort.Strings(sortedNeeds)
	for _, name := range sortedNeeds {
		need := needs[name]
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
				if balances[name].marginRatio >= e.cfg.MarginL4Threshold {
					e.log.Info("rebalance: skipping donor %s (marginRatio=%.4f >= L4=%.4f)", name, balances[name].marginRatio, e.cfg.MarginL4Threshold)
					continue
				}
				// Deterministic tie-break: pick highest surplus, then alphabetical name
				if s > bestSurplus || (s == bestSurplus && (bestDonor == "" || name < bestDonor)) {
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
				// the donor exchange past the L4 threshold.
				healthCap := e.capByMarginHealth(donorBal) - needs[bestDonor]
				if healthCap < maxMove {
					e.log.Info("rebalance: %s maxMove capped by healthCap: %.2f -> %.2f (marginRatio=%.4f, L4=%.4f)",
						bestDonor, maxMove, healthCap, donorBal.marginRatio, e.cfg.MarginL4Threshold)
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
					if moveAmt > freshBal.MaxTransferOut {
						e.log.Info("rebalance: %s capping moveAmt %.4f to fresh maxTransferOut %.4f", bestDonor, moveAmt, freshBal.MaxTransferOut)
						moveAmt = freshBal.MaxTransferOut
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
