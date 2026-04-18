package engine

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/exchange"
)

const marginEpsilon = 0.005 // margin buffer to avoid hitting L4 boundary exactly

// rebalanceTargetRatio returns the post-rebalance margin ratio target.
// Uses MarginL4Headroom (default 0.05) for meaningful headroom below L4
// so entry scans don't immediately re-hit the L4 gate after a transfer.
func (e *Engine) rebalanceTargetRatio() float64 {
	target := e.cfg.MarginL4Threshold - e.cfg.MarginL4Headroom
	if target <= 0 || target >= e.cfg.MarginL4Threshold {
		target = e.cfg.MarginL4Threshold - marginEpsilon
	}
	return target
}

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
	futures                     float64
	spot                        float64
	futuresTotal                float64
	marginRatio                 float64
	maxTransferOut              float64
	maxTransferOutAuthoritative bool
	hasPositions                bool // true if this exchange has active perp-perp legs
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

// rebalanceExecutionResult reports POST-EXECUTION state so callers can
// revalidate allocator choices against reality rather than trusting the
// feasibility assumptions the allocator made (which include theoretical
// cross-exchange transferables that may not have moved in practice).
//
// PostBalances reflects actual balances after all successful transfers and
// same-exchange relief moves. Unfunded/SkipReasons are diagnostic only.
type rebalanceExecutionResult struct {
	PostBalances map[string]rebalanceBalanceInfo
	Unfunded     map[string]float64
	SkipReasons  map[string]string
	// FundedReceivers: exchanges that received a cross-exchange deposit AND
	// successfully moved it to futures this cycle. Used as a trigger for the
	// live-balance recheck in keepFundedChoices — not a bias for entry scan.
	FundedReceivers       map[string]float64
	LocalTransferHappened bool
	CrossTransferHappened bool
}

// cloneRebalanceBalances returns a shallow copy of the map. rebalanceBalanceInfo
// is a value type so per-key writes on the copy are independent.
func cloneRebalanceBalances(src map[string]rebalanceBalanceInfo) map[string]rebalanceBalanceInfo {
	dst := make(map[string]rebalanceBalanceInfo, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// postTradeMarginRatio computes the post-trade margin ratio for an exchange
// leg assuming `margin` is debited from its futures balance. Returns ok=false
// only when the ratio breaches the configured L4 threshold. This is the
// shared helper used by both dryRunTransferPlan and replayProjectedRatioOK so
// both paths agree on what "feasible" means.
func (e *Engine) postTradeMarginRatio(bal rebalanceBalanceInfo, margin float64) (ratio float64, ok bool) {
	if bal.futuresTotal <= 0 {
		return 0, true
	}
	futures := bal.futures - margin
	if futures < 0 {
		futures = 0
	}
	ratio = 1 - futures/bal.futuresTotal
	return ratio, ratio < e.cfg.MarginL4Threshold
}

// replayProjectedRatioOK is a thin wrapper over postTradeMarginRatio used by
// keepFundedChoices during the post-execution replay.
func (e *Engine) replayProjectedRatioOK(bal rebalanceBalanceInfo, margin float64) bool {
	_, ok := e.postTradeMarginRatio(bal, margin)
	return ok
}

// canReserveAllocatorChoice reports whether both legs of the choice still
// have enough futures capacity in the working balance map. Mirrors the
// allocator's own needs accounting where requiredMargin is added to EACH
// exchange independently (allocator.go:231-232 / 666-667 — requiredMargin
// is max(long,short) buffered, not a total).
func (e *Engine) canReserveAllocatorChoice(work map[string]rebalanceBalanceInfo, c allocatorChoice) bool {
	long, okL := work[c.longExchange]
	short, okS := work[c.shortExchange]
	if !okL || !okS {
		return false
	}
	long = e.replayUsableAllocatorBalance(c.longExchange, long)
	short = e.replayUsableAllocatorBalance(c.shortExchange, short)
	if long.futures < c.requiredMargin || short.futures < c.requiredMargin {
		return false
	}
	margin := c.requiredMargin
	if e.cfg.MarginSafetyMultiplier > 0 {
		margin = c.requiredMargin / e.cfg.MarginSafetyMultiplier
	}
	return e.replayProjectedRatioOK(long, margin) && e.replayProjectedRatioOK(short, margin)
}

func (e *Engine) replayUsableAllocatorBalance(name string, bal rebalanceBalanceInfo) rebalanceBalanceInfo {
	if uc, ok := e.exchanges[name].(interface{ IsUnified() bool }); ok && uc.IsUnified() {
		return bal
	}
	if bal.spot <= 0 {
		return bal
	}
	bal.futures += bal.spot
	bal.futuresTotal += bal.spot
	bal.spot = 0
	return bal
}

// reserveAllocatorChoice subtracts requiredMargin from BOTH legs (not /2),
// matching allocator.go:231-232 / 666-667 semantics.
func (e *Engine) reserveAllocatorChoice(work map[string]rebalanceBalanceInfo, c allocatorChoice) {
	long := work[c.longExchange]
	short := work[c.shortExchange]
	long.futures -= c.requiredMargin
	short.futures -= c.requiredMargin
	work[c.longExchange] = long
	work[c.shortExchange] = short
}

// keepFundedChoices replays allocator choices in order against the executor's
// post-execution balances, reserving capacity per kept choice. Choices that
// cannot be satisfied after real transfers are dropped so stale overrides do
// not leak into the next entry scan.
func (e *Engine) keepFundedChoices(choices []allocatorChoice, post map[string]rebalanceBalanceInfo, funded map[string]float64) map[string]allocatorChoice {
	work := cloneRebalanceBalances(post)
	kept := make(map[string]allocatorChoice, len(choices))

	for _, c := range choices {
		if e.canReserveAllocatorChoice(work, c) {
			e.reserveAllocatorChoice(work, c)
			kept[c.symbol] = c
			continue
		}

		_, longFunded := funded[c.longExchange]
		_, shortFunded := funded[c.shortExchange]
		if !longFunded && !shortFunded {
			continue
		}

		// Start from the CURRENT reserved working view so any prior kept choices
		// remain deducted. Refresh only the two legs involved in this choice.
		liveWork := cloneRebalanceBalances(work)

		longBal, okLong := e.fetchLiveRebalanceBalance(c.longExchange)
		if !okLong {
			continue
		}
		shortBal, okShort := e.fetchLiveRebalanceBalance(c.shortExchange)
		if !okShort {
			continue
		}

		liveWork[c.longExchange] = longBal
		liveWork[c.shortExchange] = shortBal

		if !e.canReserveAllocatorChoice(liveWork, c) {
			continue
		}

		// Reserve against LIVE refreshed balances, then promote that reserved
		// live view to be the new working state for subsequent choices.
		e.reserveAllocatorChoice(liveWork, c)
		work = liveWork
		kept[c.symbol] = c

		e.log.Info(
			"allocator: override %s retained via live-balance recheck (funded receiver: long=%v short=%v)",
			c.symbol, longFunded, shortFunded,
		)
	}

	return kept
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
		// Respect the exchange's authoritative withdrawable cap when
		// available. maxTransferOut reflects risk-delay freezes and
		// position collateral that raw futures balance misses
		// (e.g. Bybit's /v5/account/withdrawal availableWithdrawal).
		// maxTransferOut=0 means "unknown/API failed" — fall through
		// to raw futures in that case.
		if bal.maxTransferOut > 0 && bal.maxTransferOut < futuresAvail {
			futuresAvail = bal.maxTransferOut
		}
	} else {
		futuresAvail = bal.maxTransferOut
		if futuresAvail <= 0 {
			if bal.maxTransferOutAuthoritative {
				// Exchange authoritatively reports zero withdrawable. Don't
				// override with an estimate; skip this donor.
				e.log.Debug("allocator: donorGross %s split-account authoritative maxTransferOut=0, treating as unavailable", name)
				return 0
			}
			// Field not populated — fall back to raw futures with L4-safety estimate.
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

	e.log.Debug("rebalance: donor %s capacity: futures=%.2f total=%.2f marginRatio=%.4f maxTransferOut=%.2f futuresAvail=%.2f surplus=%.2f unified=%v hasPos=%v",
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
		targetRatio := e.rebalanceTargetRatio()
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
						if donorBal.maxTransferOutAuthoritative {
							// Exchange authoritatively reports zero transferable collateral.
							// Do not synthesize a donor budget from raw futures.
							triedDonors[bestDonor] = true
							continue
						}
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
		ratio, ok := e.postTradeMarginRatio(bal, 0) // margin already applied to sim.futures above
		postRatios[name] = ratio
		if !ok {
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

func (e *Engine) executeRebalanceFundingPlan(needs map[string]float64, balances map[string]rebalanceBalanceInfo, precomputedDeficits []rebalanceDeficit) rebalanceExecutionResult {
	// PostBalances starts as a clone of caller balances. The executor mutates
	// `balances` in place throughout (same-exchange relief, eager donor
	// decrements, etc). We mirror those mutations into PostBalances only at
	// confirmed-success points, and on batched-withdraw failure we also
	// restore the donor entry on `balances` so subsequent aliasing reflects
	// reality. keepFundedChoices reads PostBalances via this alias at end.
	result := rebalanceExecutionResult{
		PostBalances: balances,
		Unfunded:     map[string]float64{},
		SkipReasons:  map[string]string{},
	}
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
	e.log.Debug("rebalance: sortedNeeds=%v needs=%v MSM=%.2f L4=%.4f", sortedNeeds, needs, e.cfg.MarginSafetyMultiplier, e.cfg.MarginL4Threshold)
	for _, name := range sortedNeeds {
		need := needs[name]
		bal := balances[name]
		targetFreeRatio := 1 - e.cfg.MarginL4Threshold
		if targetFreeRatio <= 0 {
			targetFreeRatio = 0.20
		}

		e.log.Debug("rebalance: checking %s: need=%.2f futures=%.4f spot=%.8f total=%.4f isUnified=%v", name, need, bal.futures, bal.spot, bal.futuresTotal, func() bool { uc, ok := e.exchanges[name].(interface{ IsUnified() bool }); return ok && uc.IsUnified() }())
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
								bi.futuresTotal += extra
								balances[name] = bi
								bal = balances[name] // refresh local copy for L4 check below
								result.LocalTransferHappened = true
							}
						}
					}
				}
				// After spot relief attempt, fall through to L4 check below.
				// Previously this was an unconditional 'continue' which skipped
				// the L4 cross-exchange deficit check when spot > 0 but too small
				// to actually fix the ratio. Bug: OKX with spot=0.001 entered
				// this branch, skipped L4 check, and no crossDeficit was queued.
				e.log.Debug("rebalance: %s spot-relief done (spot=%.8f), falling through to L4 check", name, bal.spot)
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
				e.log.Debug("rebalance: %s L4-check: need=%.2f actualMargin=%.2f futures=%.2f total=%.2f projAvail=%.2f projRatio=%.4f L4=%.4f",
					name, need, actualMargin, bal.futures, bal.futuresTotal, projectedAvail, projectedRatio, e.cfg.MarginL4Threshold)
				if projectedRatio >= e.cfg.MarginL4Threshold {
					// Compute ratio deficit: how much extra needed so ratio < L4
					targetRatio := e.rebalanceTargetRatio()
					freeTarget := 1.0 - targetRatio
					ratioDeficit := (freeTarget*bal.futuresTotal - bal.futures + actualMargin) / targetRatio
					if ratioDeficit > 0 {
						e.log.Info("rebalance: %s post-trade ratio=%.4f >= L4=%.4f, queueing cross-exchange deficit=%.2f",
							name, projectedRatio, e.cfg.MarginL4Threshold, ratioDeficit)
						crossDeficits = append(crossDeficits, rebalanceDeficit{name, ratioDeficit})
					}
				}
			}
			e.log.Debug("rebalance: %s sufficient-futures path done, continue", name)
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
		targetRatio := e.rebalanceTargetRatio()
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
					bi.futuresTotal += actualTransfer // same-exchange relief: futures pool grows, keep PostBalances consistent with the other relief branch at 1150-1155
					balances[name] = bi
					result.LocalTransferHappened = true
				}
			}
		}
		if transferAmt > 0 {
			crossDeficits = append(crossDeficits, rebalanceDeficit{name, transferAmt})
		}
	}
	} // end else (precomputedDeficits == nil)

	if len(crossDeficits) == 0 {
		e.log.Info("rebalance: no cross-exchange transfers needed (crossDeficits empty after local fund relief)")
		return result
	}

	surplus := map[string]float64{}
	for name := range e.exchanges {
		surplus[name] = e.allocatorDonorGrossCapacity(name, balances[name], needs[name])
	}

	pendingDeposits := map[string]float64{}
	pendingStartBal := map[string]float64{}

	// Batch withdrawals: accumulate per donor→recipient, execute after loop.
	type batchedWd struct {
		donor, recipient string
		chain, destAddr  string
		netTotal         float64
		fee              float64
		isGross          bool
		isUnifiedDonor   bool
		movedToSpot      float64
		// debitFutures / debitSpot / debitFuturesTotal track the EXACT
		// cumulative in-memory decrements applied to the donor balance
		// during accumulation, so we can roll them back if the batched
		// Withdraw fails and reconcile merged-fee over-debit on success.
		debitFutures      float64
		debitSpot         float64
		debitFuturesTotal float64
	}
	batchedWds := map[string]*batchedWd{}
	for i := range crossDeficits {
		remaining := crossDeficits[i].amount
		exchName := crossDeficits[i].exchange
		origDeficit := remaining
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
				result.SkipReasons[exchName] = fmt.Sprintf("no donor found (deficit=%.2f)", remaining)
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
				result.SkipReasons[exchName] = "no deposit addresses configured"
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
					scheduled := origDeficit - remaining
					if scheduled >= origDeficit*0.9 {
						e.log.Info("rebalance: residual %.2f on %s below %s minWd floor %.2f after %.2f/%.2f already scheduled; not overfunding",
							remaining, exchName, bestDonor, netFloor, scheduled, origDeficit)
						result.Unfunded[exchName] = remaining
						if _, ok := result.SkipReasons[exchName]; !ok {
							result.SkipReasons[exchName] = fmt.Sprintf("residual %.2f below %s minWd floor %.2f", remaining, bestDonor, netFloor)
						}
						break
					}
					cappedFloor := math.Min(netFloor, bestSurplus-fee)
					if cappedFloor <= 0 {
						surplus[bestDonor] = 0
						continue
					}
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
					if donorBal.maxTransferOutAuthoritative {
						e.log.Info("rebalance: %s donor skipped — authoritative maxTransferOut=0", bestDonor)
						surplus[bestDonor] = 0
						continue
					}
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
				// Post-format zero guard: moveAmt can be a tiny positive (e.g. 0.00003)
				// that passes float checks but rounds to "0.0000" via %.4f. Exchanges
				// reject zero-amount transfers (bitget code=40020).
				if parsed, _ := strconv.ParseFloat(moveStr, 64); parsed <= 0 {
					e.log.Warn("rebalance: %s moveAmt %.6f rounds to zero string, skipping", bestDonor, moveAmt)
					surplus[bestDonor] = 0
					continue
				}
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

			// For unified donors, also respect MaxTransferOut (the exchange's
			// authoritative withdrawable cap). For split donors the earlier
			// :1557-1560 fresh re-cap already applied; here it is a no-op
			// because spot balance adapters don't set MaxTransferOut.
			effectiveAvail := donorSpotBal.Available
			if donorSpotBal.MaxTransferOut > 0 && donorSpotBal.MaxTransferOut < effectiveAvail {
				effectiveAvail = donorSpotBal.MaxTransferOut
			}
			if effectiveAvail < grossRequired {
				netAmount = effectiveAvail - fee
				e.log.Debug("rebalance: adjusted netAmount %s: effAvail=%.2f (spotAvail=%.2f maxTransferOut=%.2f) gross=%.2f net=%.2f",
					bestDonor, effectiveAvail, donorSpotBal.Available, donorSpotBal.MaxTransferOut, grossRequired, netAmount)
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
			// Compute this iteration's in-memory decrement so we can
			// accumulate it into the batch for possible rollback.
			// Account-type aware: unified donors have no separate spot pool —
			// the withdraw drains the unified pool directly. Split donors
			// perform a futures->spot prep (movedToSpot) then withdraw from
			// spot, so the futures pool shrinks by the prep amount and the
			// futuresTotal shrinks by the same amount.
			var iterFuturesDebit, iterSpotDebit, iterFuturesTotalDebit float64
			if skipOuterTransfer {
				// Unified donor: withdraw drains the unified futures pool.
				iterFuturesDebit = netAmount + fee
				iterFuturesTotalDebit = netAmount + fee
			} else {
				// Split donor: prep moves futures->spot, then withdraw drains spot.
				iterFuturesDebit = movedToSpot
				iterSpotDebit = netAmount + fee - movedToSpot
				iterFuturesTotalDebit = movedToSpot
			}

			// Accumulate into batch instead of immediate withdraw.
			wdKey := bestDonor + "->" + exchName
			if bw, ok := batchedWds[wdKey]; ok {
				bw.netTotal += netAmount
				bw.movedToSpot += movedToSpot
				bw.debitFutures += iterFuturesDebit
				bw.debitSpot += iterSpotDebit
				bw.debitFuturesTotal += iterFuturesTotalDebit
			} else {
				batchedWds[wdKey] = &batchedWd{
					donor: bestDonor, recipient: exchName,
					chain: chain, destAddr: destAddr,
					netTotal: netAmount, fee: fee,
					isGross: isGross, isUnifiedDonor: skipOuterTransfer,
					movedToSpot:       movedToSpot,
					debitFutures:      iterFuturesDebit,
					debitSpot:         iterSpotDebit,
					debitFuturesTotal: iterFuturesTotalDebit,
				}
			}

			// Keep surplus/balance updates for correct donor selection in subsequent iterations.
			surplus[bestDonor] -= netAmount + fee
			bi := balances[bestDonor]
			bi.futures -= iterFuturesDebit
			bi.spot -= iterSpotDebit
			bi.futuresTotal -= iterFuturesTotalDebit
			if bi.spot < 0 {
				bi.spot = 0
			}
			if bi.futuresTotal < 0 {
				bi.futuresTotal = 0
			}
			balances[bestDonor] = bi
			remaining -= netAmount
		}
		if remaining > 0 {
			// Inner loop broke (no donor / no addresses / etc) or ran out of
			// viable donor capacity; track the shortfall for caller diagnostics.
			result.Unfunded[exchName] = remaining
			if _, ok := result.SkipReasons[exchName]; !ok {
				result.SkipReasons[exchName] = fmt.Sprintf("partial fill only: %.2f of %.2f funded", origDeficit-remaining, origDeficit)
			}
		}
	}

	// Execute batched withdrawals in deterministic order.
	batchKeys := make([]string, 0, len(batchedWds))
	for k := range batchedWds {
		batchKeys = append(batchKeys, k)
	}
	sort.Strings(batchKeys)
	for _, bk := range batchKeys {
		bw := batchedWds[bk]
		withdrawAmtForAPI := bw.netTotal
		if bw.isGross {
			withdrawAmtForAPI = bw.netTotal + bw.fee
		}
		amtStr := fmt.Sprintf("%.4f", withdrawAmtForAPI)
		e.log.Info("rebalance: batched withdraw %s->%s net=%.2f fee=%.4f amount=%.2f via %s",
			bw.donor, bw.recipient, bw.netTotal, bw.fee, withdrawAmtForAPI, bw.chain)

		wdResult, err := e.exchanges[bw.donor].Withdraw(exchange.WithdrawParams{
			Coin:    "USDT",
			Chain:   bw.chain,
			Address: bw.destAddr,
			Amount:  amtStr,
		})
		if err != nil {
			e.log.Error("rebalance: batched withdraw from %s failed: %v", bw.donor, err)
			rollbackOK := true
			if bw.movedToSpot > 0 {
				rollbackStr := fmt.Sprintf("%.4f", bw.movedToSpot)
				e.log.Info("rebalance: rollback %s spot->futures %s USDT", bw.donor, rollbackStr)
				if bw.donor == "bingx" {
					time.Sleep(2 * time.Second)
				}
				if rbErr := e.exchanges[bw.donor].TransferToFutures("USDT", rollbackStr); rbErr != nil {
					e.log.Error("rebalance: rollback failed: %v", rbErr)
					rollbackOK = false
				}
			}
			if rollbackOK {
				// Exchange-side rollback succeeded (or was not needed). Restore
				// in-memory donor balance to the pre-decrement state so
				// PostBalances (aliased to balances) reflects reality.
				bi := balances[bw.donor]
				bi.futures += bw.debitFutures
				bi.spot += bw.debitSpot
				bi.futuresTotal += bw.debitFuturesTotal
				balances[bw.donor] = bi
			} else {
				// Rollback failed — funds are stuck in the donor's spot pool
				// (split donor) or pool state is unknown. Refresh from the
				// exchange; if that also fails, zero out the donor so
				// keepFundedChoices cannot reserve against stale in-memory
				// values.
				refreshed := false
				if fb, ferr := e.exchanges[bw.donor].GetFuturesBalance(); ferr == nil {
					bi := balances[bw.donor]
					bi.futures = fb.Available
					if fb.Total > 0 {
						bi.futuresTotal = fb.Total
					}
					if sb, serr := e.exchanges[bw.donor].GetSpotBalance(); serr == nil {
						bi.spot = sb.Available
						refreshed = true
					} else {
						e.log.Warn("rebalance: %s spot balance refresh failed after rollback failure: %v", bw.donor, serr)
					}
					balances[bw.donor] = bi
				} else {
					e.log.Warn("rebalance: %s futures balance refresh failed after rollback failure: %v", bw.donor, ferr)
				}
				if !refreshed {
					// Pessimistic: zero the donor so keepFundedChoices cannot
					// pick it to satisfy any choice. This is safer than
					// trusting either pre- or post-decrement in-memory state
					// when on-exchange reality is unknown.
					bi := rebalanceBalanceInfo{futuresTotal: balances[bw.donor].futuresTotal}
					balances[bw.donor] = bi
				}
			}
			// Track as unfunded for diagnostics regardless of rollback state.
			result.Unfunded[bw.recipient] = bw.netTotal
			result.SkipReasons[bw.recipient] = fmt.Sprintf("withdraw from %s failed: %v (rollbackOK=%v)", bw.donor, err, rollbackOK)
			continue
		}

		recipientReceives := bw.netTotal
		if bw.isGross {
			recipientReceives = withdrawAmtForAPI - bw.fee
		}
		e.log.Info("rebalance: withdraw from %s txid=%s (apiAmt=%.2f, recipient=%.2f, fee=%.4f, gross=%v) -> %s",
			bw.donor, wdResult.TxID, withdrawAmtForAPI, recipientReceives, bw.fee, bw.isGross, bw.recipient)
		e.recordTransfer(bw.donor, bw.recipient, "USDT", bw.chain, amtStr, wdResult.Fee, wdResult.TxID, "completed", "rebalance")

		// Reconcile donor PostBalances against the ACTUAL merged-batch debit.
		// Per-iteration tracking subtracted fee each iteration, but the merged
		// withdraw only charges one bw.fee. Restore any excess so keepFundedChoices
		// sees the true donor balance. Account-type aware — unified donors drain
		// the futures pool while split donors drain spot after a prep move.
		var actualFuturesDebit, actualSpotDebit, actualFuturesTotalDebit float64
		if bw.isUnifiedDonor {
			actualFuturesDebit = bw.netTotal + bw.fee
			actualFuturesTotalDebit = bw.netTotal + bw.fee
		} else {
			actualFuturesDebit = bw.movedToSpot
			actualSpotDebit = bw.netTotal + bw.fee - bw.movedToSpot
			actualFuturesTotalDebit = bw.movedToSpot
		}
		excessFutures := bw.debitFutures - actualFuturesDebit
		excessSpot := bw.debitSpot - actualSpotDebit
		excessFuturesTotal := bw.debitFuturesTotal - actualFuturesTotalDebit
		if excessFutures != 0 || excessSpot != 0 || excessFuturesTotal != 0 {
			bi := balances[bw.donor]
			bi.futures += excessFutures
			bi.spot += excessSpot
			bi.futuresTotal += excessFuturesTotal
			balances[bw.donor] = bi
		}

		if _, exists := pendingStartBal[bw.recipient]; !exists {
			if uc, ok := e.exchanges[bw.recipient].(interface{ IsUnified() bool }); ok && uc.IsUnified() {
				if fb, err := e.exchanges[bw.recipient].GetFuturesBalance(); err == nil {
					pendingStartBal[bw.recipient] = fb.Available
				} else {
					e.log.Warn("rebalance: %s unified GetFuturesBalance for deposit baseline failed: %v, using futures snapshot fallback", bw.recipient, err)
					pendingStartBal[bw.recipient] = balances[bw.recipient].futures
				}
			} else {
				pendingStartBal[bw.recipient] = balances[bw.recipient].spot
			}
		}
		pendingDeposits[bw.recipient] += recipientReceives
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
			arrivedAmt := spotBal.Available - startBal
			if arrivedAmt < totalPending*0.9 {
				continue
			}
			e.log.Info("rebalance: deposit threshold met (90%%) on %s: %.2f arrived of %.2f target", recipient, arrivedAmt, totalPending)

			moveAmt := math.Min(totalPending, arrivedAmt)
			moveAmt = math.Floor(moveAmt*10000) / 10000
			if moveAmt <= 0 {
				continue
			}

			// Unified receivers (bybit UTA, gateio unified): deposit already
			// sits in the unified pool — no spot→futures API call needed.
			// Credit the PostBalances directly. Mirrors v0.32.21 Section A.
			if uc, ok := recipientExch.(interface{ IsUnified() bool }); ok && uc.IsUnified() {
				bi := balances[recipient]
				bi.futures += moveAmt
				bi.futuresTotal += moveAmt
				balances[recipient] = bi
				if result.FundedReceivers == nil {
					result.FundedReceivers = make(map[string]float64)
				}
				result.FundedReceivers[recipient] += moveAmt
				result.CrossTransferHappened = true
				arrived = true
				break
			}

			// Split-account receivers: actual spot→futures API call.
			transferStr := fmt.Sprintf("%.4f", moveAmt)
			if err := recipientExch.TransferToFutures("USDT", transferStr); err != nil {
				e.log.Warn("rebalance: %s spot->futures %s USDT failed before %s: %v", recipient, transferStr, pollDeadline.Format(time.RFC3339), err)
				continue
			}

			e.log.Info("rebalance: %s spot->futures %s USDT (rebalance deposit)", recipient, transferStr)
			e.recordTransfer(recipient+" spot", recipient, "USDT", "internal", transferStr, "0", "", "completed", "rebalance-recv")
			bi := balances[recipient]
			bi.futures += moveAmt
			bi.futuresTotal += moveAmt
			bi.spot = math.Max(0, spotBal.Available-moveAmt)
			balances[recipient] = bi
			if result.FundedReceivers == nil {
				result.FundedReceivers = make(map[string]float64)
			}
			result.FundedReceivers[recipient] += moveAmt
			result.CrossTransferHappened = true
			arrived = true
			break
		}
		if !arrived {
			creditedAmt := 0.0

			// Deadline hit: mirror the same balance source used by the poll loop,
			// then preserve any late-arriving funds for override replay.
			var liveBal *exchange.Balance
			var balErr error
			if uc, ok := recipientExch.(interface{ IsUnified() bool }); ok && uc.IsUnified() {
				liveBal, balErr = recipientExch.GetFuturesBalance()
			} else {
				liveBal, balErr = recipientExch.GetSpotBalance()
			}
			if balErr == nil {
				arrivedAmt := liveBal.Available - startBal
				if arrivedAmt > 1.0 {
					creditedAmt = math.Min(totalPending, arrivedAmt)

					bi := balances[recipient]
					if uc, ok := recipientExch.(interface{ IsUnified() bool }); ok && uc.IsUnified() {
						bi.futures += creditedAmt
						bi.futuresTotal += creditedAmt
					} else {
						bi.spot += creditedAmt
					}
					balances[recipient] = bi

					if result.FundedReceivers == nil {
						result.FundedReceivers = make(map[string]float64)
					}
					result.FundedReceivers[recipient] += creditedAmt
					result.CrossTransferHappened = true
					e.log.Info("rebalance: deadline hit on %s, credited %.2f late-arriving balance for override retention", recipient, creditedAmt)
				}
			}

			shortfall := totalPending - creditedAmt
			if shortfall > 0.0001 {
				result.Unfunded[recipient] = shortfall
				if creditedAmt > 0 {
					result.SkipReasons[recipient] = fmt.Sprintf("deposit timeout after partial arrival %.2f/%.2f", creditedAmt, totalPending)
				} else {
					result.SkipReasons[recipient] = "deposit timeout"
				}
			}

			e.log.Warn("rebalance: deposits on %s not confirmed within 5min, skipping spot->futures", recipient)
		}
	}

	e.log.Info("rebalance: complete (unfunded=%d skipReasons=%d)", len(result.Unfunded), len(result.SkipReasons))
	return result
}

func findOppBySymbol(opps []models.Opportunity, symbol string) *models.Opportunity {
	for i := range opps {
		if opps[i].Symbol == symbol {
			return &opps[i]
		}
	}
	return nil
}
