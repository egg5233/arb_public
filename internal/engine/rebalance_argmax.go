package engine

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/exchange"
)

// rebalanceFundsArgmax enumerates candidate choices (approved + capital-rescue),
// scores every subset by net PnL, and picks the globally best feasible subset.
//
// Replaces the legacy tail-prune + pool-allocator fallback with a single pass
// that can choose subsets that include capital-rejected but transferable
// opportunities — the fix for the 2026-04-20 15:45 scenario where 288 USDT of
// donor capital sat idle while a 147 USDT BingX-constrained opp was discarded
// before enumeration.
func (e *Engine) rebalanceFundsArgmax(passedOpps ...[]models.Opportunity) {
	// Loss-limiter gate: mirror the pool allocator (allocator.go:335-346) and
	// executeArbitrage so argmax does not spend donor withdrawal fees during
	// a trading halt. Without this gate the rescue can move capital to a
	// recipient that entry will then refuse to trade from, stranding funds.
	if e.lossLimiter != nil && e.cfg.EnableLossLimits {
		if blocked, status := e.lossLimiter.CheckLimits(); blocked {
			e.log.Warn("[argmax] loss limit breached (%s), skipping", status.BreachType)
			if e.api != nil {
				e.api.BroadcastLossLimits(status)
			}
			if e.cfg.EnablePerpTelegram && e.telegram != nil {
				e.telegram.NotifyLossLimitBreached(status.BreachType,
					status.DailyLoss, status.DailyLimit,
					status.WeeklyLoss, status.WeeklyLimit)
			}
			return
		}
	}

	opps := e.resolveArgmaxOpps(passedOpps...)
	if len(opps) == 0 {
		e.log.Info("[argmax] no opportunities, skipping")
		return
	}

	active, err := e.db.GetActivePositions()
	if err != nil {
		e.log.Error("[argmax] failed to get active positions: %v", err)
		return
	}

	balances, ok := e.snapshotArgmaxBalances(active)
	if !ok {
		return
	}

	remainingSlots := e.cfg.MaxPositions - len(active)
	if remainingSlots <= 0 {
		e.log.Info("[argmax] at max capacity (%d/%d), skipping", len(active), e.cfg.MaxPositions)
		return
	}

	candidates, cache, builderRejects := e.buildArgmaxCandidates(opps, active, balances)
	if len(candidates) == 0 {
		e.logArgmaxRejects(builderRejects)
		e.log.Info("[argmax] no candidates built from %d opps", len(opps))
		return
	}

	subsetCap := remainingSlots
	if e.cfg.RebalanceSubsetSizeCap > 0 && e.cfg.RebalanceSubsetSizeCap < subsetCap {
		subsetCap = e.cfg.RebalanceSubsetSizeCap
	}
	if subsetCap <= 0 {
		e.log.Info("[argmax] no remaining slots")
		return
	}

	startedAt := time.Now()
	timeout := time.Duration(e.cfg.AllocatorTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Millisecond
	}
	deadline := startedAt.Add(timeout)

	feeCache := make(map[string]feeEntry)

	// Stream-best: keep only the current best combo + a feasibility counter.
	// The slice-of-all-combos approach would grow unbounded under high
	// candidate counts (MEDIUM severity, flagged by independent review).
	var best *scoredCombo
	feasibleCount := 0
	rejectStats := map[string]int{}
	for k, v := range builderRejects {
		rejectStats[k] = v
	}
	evaluated := 0
	timedOut := false

	// Enumerate by ascending subset size so an early timeout still leaves the
	// smaller proven subsets available. Candidates are already sorted by base
	// value descending in buildArgmaxCandidates.
	for size := 1; size <= subsetCap; size++ {
		enumerateCombinations(candidates, size, func(combo []allocatorChoice) bool {
			if time.Now().After(deadline) {
				timedOut = true
				return false
			}
			evaluated++

			// Guard: flat enumeration can pick the same symbol twice (primary +
			// alternative pair share the same symbol).
			if hasDuplicateSymbols(combo) {
				rejectStats["duplicate_symbol"]++
				return true
			}

			result := e.dryRunTransferPlan(combo, balances, feeCache)
			if !result.Feasible {
				rejectStats["infeasible"]++
				return true
			}
			if e.violatesDonorFloorFromSteps(result.Steps, balances, combo, e.cfg.RebalanceDonorFloorPct) {
				rejectStats["donor_floor"]++
				return true
			}
			base := 0.0
			for _, c := range combo {
				base += c.baseValue
			}
			net := base - result.TotalFee
			if net < e.cfg.RebalanceMinNetPnLUSDT {
				rejectStats["min_net_pnl"]++
				return true
			}
			feasibleCount++
			cand := scoredCombo{
				choices:    append([]allocatorChoice(nil), combo...),
				dryRun:     result,
				baseValue:  base,
				netPnL:     net,
				symbolsKey: comboSymbolsKey(combo),
			}
			if best == nil || argmaxBetter(cand, *best) {
				candPtr := cand
				best = &candPtr
			}
			return true
		})
		if timedOut {
			break
		}
	}

	elapsed := time.Since(startedAt)
	e.logArgmaxRejects(rejectStats)

	if best == nil {
		e.log.Info("[argmax] scan choices=%d combos=%d feasible=0 timeout=%v elapsed=%s slots=%d thresholds={minNet=%.2f donorFloor=%.2f}",
			len(candidates), evaluated, timedOut, elapsed,
			subsetCap, e.cfg.RebalanceMinNetPnLUSDT, e.cfg.RebalanceDonorFloorPct)
		return
	}

	e.log.Info("[argmax] scan choices=%d combos=%d feasible=%d timeout=%v elapsed=%s slots=%d thresholds={minNet=%.2f donorFloor=%.2f}",
		len(candidates), evaluated, feasibleCount, timedOut, elapsed,
		subsetCap, e.cfg.RebalanceMinNetPnLUSDT, e.cfg.RebalanceDonorFloorPct)
	e.log.Info("[argmax] pick symbols=%v netPnL=%.4f base=%.4f fee=%.4f steps=%s postRatios=%v",
		symbolsOf(best.choices), best.netPnL, best.baseValue, best.dryRun.TotalFee,
		formatArgmaxSteps(best.dryRun.Steps), best.dryRun.PostRatios)

	// Reserved-aware sequential re-validation of the chosen combo. Argmax's
	// per-candidate approvals all used reserved=nil, so combos where multiple
	// picks share a constrained exchange can have individually-passing
	// approvals that fail at entry time once `executeArbitrage`'s sequential
	// reservation chain applies. Mirrors the legacy allocator's guard at
	// allocator.go:393-426. Drops picks that fail under accumulated reserved
	// margin, updates Size/Price/RequiredMargin from replay approvals, and
	// re-runs dryRunTransferPlan on the validated subset. Without this step
	// argmax could fund donor transfers for symbols the entry scan will
	// reject, stranding capital on recipient exchanges.
	fees := e.getEffectiveAllocatorFees()
	validated := e.reservedValidateArgmaxCombo(best.choices, opps, cache, fees)
	if len(validated) == 0 {
		e.log.Warn("[argmax] reserved re-validation dropped all %d picks, skipping", len(best.choices))
		return
	}
	if len(validated) < len(best.choices) {
		e.log.Info("[argmax] reserved re-validation: %d → %d picks (dropped %v, kept %v)",
			len(best.choices), len(validated),
			symbolsDropped(best.choices, validated),
			symbolsOf(validated))
		// Re-run feasibility + scoring on the reduced combo. The original
		// dryRun plan covered more choices; replaying ensures transfer
		// amounts, donor floors, and net-PnL all reflect the surviving set.
		result := e.dryRunTransferPlan(validated, balances, feeCache)
		if !result.Feasible {
			e.log.Warn("[argmax] reserved re-validation subset infeasible after drop, skipping")
			return
		}
		if e.violatesDonorFloorFromSteps(result.Steps, balances, validated, e.cfg.RebalanceDonorFloorPct) {
			e.log.Warn("[argmax] reserved re-validation subset violates donor floor, skipping")
			return
		}
		base := 0.0
		for _, c := range validated {
			base += c.baseValue
		}
		net := base - result.TotalFee
		if net < e.cfg.RebalanceMinNetPnLUSDT {
			e.log.Info("[argmax] reserved re-validation subset netPnL=%.4f < minNet=%.2f, skipping",
				net, e.cfg.RebalanceMinNetPnLUSDT)
			return
		}
		best.choices = validated
		best.dryRun = result
		best.baseValue = base
		best.netPnL = net
		e.log.Info("[argmax] pick (revalidated) symbols=%v netPnL=%.4f base=%.4f fee=%.4f steps=%s",
			symbolsOf(best.choices), best.netPnL, best.baseValue, best.dryRun.TotalFee,
			formatArgmaxSteps(best.dryRun.Steps))
	}

	// Final capacity recheck under capacityMu. Kept to a short critical
	// section — we deliberately do NOT hold capacityMu across the execute RPCs
	// or post-execution replay (those can take seconds to minutes and would
	// block ManualOpen + executeArbitrage).
	e.capacityMu.Lock()
	activeNow, err := e.db.GetActivePositions()
	if err != nil {
		e.capacityMu.Unlock()
		e.log.Warn("[argmax] capacity recheck failed: %v, skipping", err)
		return
	}
	if e.cfg.MaxPositions-len(activeNow) < len(best.choices) {
		e.capacityMu.Unlock()
		e.log.Warn("[argmax] capacity changed before execute (want=%d have=%d), skipping",
			len(best.choices), e.cfg.MaxPositions-len(activeNow))
		return
	}
	// Re-check the feature flag under the same lock. Operator may have toggled
	// EnableArgmaxRebalance off between dispatch and execute; without this
	// recheck the flag cannot serve as an emergency brake — an in-flight cycle
	// would still move funds and publish overrides that entry later consumes.
	if !e.cfg.EnableArgmaxRebalance {
		e.capacityMu.Unlock()
		e.log.Warn("[argmax] feature flag disabled between dispatch and execute, aborting")
		return
	}
	e.capacityMu.Unlock()

	// Execute WITHOUT any lock held.
	needs := argmaxNeedsFromChoices(best.choices)
	result := e.executeRebalanceFundingPlan(needs, balances, nil, best.dryRun.Steps)

	// keepFundedChoices may call fetchLiveRebalanceBalance (RPC), so compute
	// it without any lock held, then publish under a short locked section.
	kept := e.keepFundedChoices(best.choices, result.PostBalances, result.FundedReceivers, result.PendingDeposits)

	e.log.Info("[argmax] execute local=%v cross=%v funded=%v pending=%v unfunded=%v skip=%v kept=%d/%d",
		result.LocalTransferHappened, result.CrossTransferHappened,
		result.FundedReceivers, result.PendingDeposits, result.Unfunded,
		result.SkipReasons, len(kept), len(best.choices))

	// Publish under capacityMu, but first re-read the flag AND re-check that
	// capacity wasn't consumed by a concurrent ManualOpen during the transfer
	// window. If either guard fails, drop the kept overrides so the next entry
	// scan does not open against a stale plan.
	e.capacityMu.Lock()
	activePost, postErr := e.db.GetActivePositions()
	if postErr != nil {
		e.capacityMu.Unlock()
		e.log.Warn("[argmax] post-execute capacity recheck failed: %v, dropping overrides", postErr)
		return
	}
	if e.cfg.MaxPositions-len(activePost) < len(kept) {
		e.capacityMu.Unlock()
		e.log.Warn("[argmax] capacity consumed during execute (want=%d have=%d), dropping %d overrides",
			len(kept), e.cfg.MaxPositions-len(activePost), len(kept))
		return
	}
	// Final flag recheck immediately before the write. Must happen under
	// allocOverrideMu so a concurrent ClearAllocOverrides (invoked by the
	// config POST handler on flag true→false) cannot interleave between our
	// check and the write. Without this the race is: [argmax] see flag=true,
	// DB query runs, [handler] flag=false + ClearAllocOverrides, [argmax]
	// publish stale overrides.
	e.allocOverrideMu.Lock()
	if !e.cfg.EnableArgmaxRebalance {
		e.allocOverrideMu.Unlock()
		e.capacityMu.Unlock()
		e.log.Warn("[argmax] feature flag disabled during execute, dropping %d overrides without publishing", len(kept))
		return
	}
	e.allocOverrides = kept
	e.allocOverrideMu.Unlock()
	e.capacityMu.Unlock()
}

// reservedValidateArgmaxCombo replays SimulateApprovalForPair for each pick
// in the combo with an accumulating reserved-margin map so picks whose
// individually-approved Size no longer fits once earlier picks reserve
// balance on shared exchanges are dropped. Picks that still approve have
// their Size/Price/RequiredMargin updated from the replay approval (which
// may resize downward under pressure). The combo order controls priority:
// earlier entries (already sorted by argmax baseValue desc) keep their
// reservation, later entries must fit in what remains.
func (e *Engine) reservedValidateArgmaxCombo(
	combo []allocatorChoice,
	opps []models.Opportunity,
	cache *risk.PrefetchCache,
	fees map[string]allocatorFeeSchedule,
) []allocatorChoice {
	reserved := map[string]float64{}
	validated := make([]allocatorChoice, 0, len(combo))

	for _, c := range combo {
		opp := findOppBySymbol(opps, c.symbol)
		if opp == nil {
			// Missing origin opp (e.g., symbol was filtered after build).
			// Drop rather than operate on stale data.
			e.log.Info("[argmax] reserved-replay %s %s>%s dropped: source opp not found",
				c.symbol, c.longExchange, c.shortExchange)
			continue
		}

		approval, err := e.risk.SimulateApprovalForPair(*opp, c.longExchange, c.shortExchange, reserved, c.altPair, cache)
		if err != nil {
			e.log.Info("[argmax] reserved-replay %s %s>%s dropped: approval error: %v",
				c.symbol, c.longExchange, c.shortExchange, err)
			continue
		}
		if approval == nil {
			e.log.Info("[argmax] reserved-replay %s %s>%s dropped: nil approval", c.symbol, c.longExchange, c.shortExchange)
			continue
		}
		if !approval.Approved {
			e.log.Info("[argmax] reserved-replay %s %s>%s dropped: %s (kind=%s)",
				c.symbol, c.longExchange, c.shortExchange, approval.Reason, approval.Kind.String())
			continue
		}
		if approval.Size <= 0 || approval.Price <= 0 {
			e.log.Info("[argmax] reserved-replay %s %s>%s dropped: unpriced approval",
				c.symbol, c.longExchange, c.shortExchange)
			continue
		}

		// Accept — update the choice with the replay's sizing and accumulate
		// reserved margin for subsequent picks. Use LongMarginNeeded /
		// ShortMarginNeeded (buffered) so downstream approvals see the full
		// buffer-adjusted reservation, matching executeArbitrage's behavior.
		updated := c
		entryNotional := approval.Size * approval.Price
		required := approval.RequiredMargin
		if required <= 0 {
			required = e.cfg.CapitalPerLeg * e.cfg.MarginSafetyMultiplier
		}
		updated.requiredMargin = required
		updated.entryNotional = entryNotional
		updated.baseValue = computeAllocatorBaseValue(updated.spreadBpsH, entryNotional, updated.longExchange, updated.shortExchange, e.cfg.MinHoldTime.Hours(), fees)
		validated = append(validated, updated)

		longMargin := approval.LongMarginNeeded
		if longMargin <= 0 {
			longMargin = approval.RequiredMargin
		}
		shortMargin := approval.ShortMarginNeeded
		if shortMargin <= 0 {
			shortMargin = approval.RequiredMargin
		}
		reserved[c.longExchange] += longMargin
		reserved[c.shortExchange] += shortMargin
	}

	return validated
}

// symbolsDropped returns the symbols present in `before` but not in `after`.
// Used for the reserved-replay log line so operators can see which picks
// the sequential re-validation eliminated.
func symbolsDropped(before, after []allocatorChoice) []string {
	kept := make(map[string]bool, len(after))
	for _, c := range after {
		kept[c.symbol] = true
	}
	out := make([]string, 0)
	for _, c := range before {
		if !kept[c.symbol] {
			out = append(out, c.symbol)
		}
	}
	return out
}

// ClearAllocOverrides wipes any cached allocator overrides. Registered with
// the API server so the config POST handler can drop stale argmax overrides
// immediately when the operator toggles EnableArgmaxRebalance off — otherwise
// the next entry scan would still consume an argmax-sourced plan even though
// the feature flag is now disabled.
func (e *Engine) ClearAllocOverrides() {
	// Count and clear under the mutex, log after unlocking so the I/O
	// path does not hold the lock.
	e.allocOverrideMu.Lock()
	n := len(e.allocOverrides)
	if n > 0 {
		e.allocOverrides = nil
	}
	e.allocOverrideMu.Unlock()
	if n > 0 {
		e.log.Info("rebalance: cleared %d allocator overrides (argmax flag disabled)", n)
	}
}

// resolveArgmaxOpps returns the passed opportunities when provided, otherwise
// pulls the latest from discovery. Kept separate so rebalanceFundsArgmax stays
// readable.
func (e *Engine) resolveArgmaxOpps(passedOpps ...[]models.Opportunity) []models.Opportunity {
	if len(passedOpps) > 0 && len(passedOpps[0]) > 0 {
		return passedOpps[0]
	}
	return e.discovery.GetOpportunities()
}

// snapshotArgmaxBalances queries all exchange balances and tags exchanges that
// already hold a live leg. Mirrors the snapshot portion of rebalanceFundsLegacy
// (engine.go:606-636) so legacy and argmax paths see the same view of the world.
func (e *Engine) snapshotArgmaxBalances(active []*models.ArbitragePosition) (map[string]rebalanceBalanceInfo, bool) {
	exchWithPositions := make(map[string]bool)
	for _, p := range active {
		if p == nil || p.Status == models.StatusClosed {
			continue
		}
		exchWithPositions[p.LongExchange] = true
		exchWithPositions[p.ShortExchange] = true
	}

	balances := map[string]rebalanceBalanceInfo{}
	for name, exch := range e.exchanges {
		var bi rebalanceBalanceInfo
		if futBal, err := exch.GetFuturesBalance(); err == nil {
			bi.futures = futBal.Available
			bi.futuresTotal = futBal.Total
			bi.marginRatio = futBal.MarginRatio
			bi.maxTransferOut = futBal.MaxTransferOut
			bi.maxTransferOutAuthoritative = futBal.MaxTransferOutAuthoritative
		}
		if spotBal, err := exch.GetSpotBalance(); err == nil {
			bi.spot = spotBal.Available
		}
		bi.hasPositions = exchWithPositions[name]
		balances[name] = bi
		e.log.Info("[argmax] %s futures=%.2f spot=%.2f futuresTotal=%.2f maintRatio=%.4f usageRatio=%.4f maxTransferOut=%.2f hasPos=%v",
			name, bi.futures, bi.spot, bi.futuresTotal, bi.marginRatio, rebalanceUsageRatio(bi), bi.maxTransferOut, bi.hasPositions)
	}
	return balances, true
}

// buildArgmaxCandidates walks the primary pair plus verified Alternatives in
// their provided direction. It does NOT synthesize long/short flips (that
// would invert funding direction). Capital-rescue candidates (approved=false,
// Kind=RejectionKindCapital) are admitted only when Size, Price, and
// RequiredMargin are all non-zero — otherwise dryRun and PnL scoring cannot be
// computed and the candidate is dropped as capital_unpriced.
func (e *Engine) buildArgmaxCandidates(opps []models.Opportunity, active []*models.ArbitragePosition, balances map[string]rebalanceBalanceInfo) ([]allocatorChoice, *risk.PrefetchCache, map[string]int) {
	rejectStats := map[string]int{}
	opps = e.filterArgmaxOpps(opps, active, rejectStats)
	if len(opps) == 0 {
		return nil, nil, rejectStats
	}

	cache := e.buildTransferableCache(balances)
	// Initialize Orderbooks so SimulateApprovalForPair populates the map on the
	// first approval, and the replay for capital-rescue candidates can reuse
	// the cached books instead of re-fetching from exchanges.
	if cache.Orderbooks == nil {
		cache.Orderbooks = make(map[string]*exchange.Orderbook)
	}
	fees := e.getEffectiveAllocatorFees()

	// Build the spot-futures occupancy map once so each attempt (primary +
	// alternatives) can be tested. filterArgmaxOpps skips opps whose primary
	// pair is SF-occupied, but alternative pairs have different exchange
	// combinations and must be checked individually — otherwise argmax can
	// plan a transfer to an exchange the perp-perp entry gate would skip.
	spotFuturesKeys, _ := e.buildSpotFuturesMaps()
	spotFuturesOccupied := make(map[string]bool, len(spotFuturesKeys))
	for key := range spotFuturesKeys {
		parts := strings.SplitN(key, ":", 3)
		if len(parts) != 3 {
			continue
		}
		spotFuturesOccupied[parts[0]+":"+parts[1]] = true
	}

	type attempt struct {
		longExch      string
		shortExch     string
		spread        float64
		intervalHours float64
		alt           *models.AlternativePair
	}

	out := make([]allocatorChoice, 0, len(opps))

	for _, opp := range opps {
		attempts := make([]attempt, 0, 1+len(opp.Alternatives))
		attempts = append(attempts, attempt{opp.LongExchange, opp.ShortExchange, opp.Spread, opp.IntervalHours, nil})
		for i := range opp.Alternatives {
			a := &opp.Alternatives[i]
			if !a.Verified {
				rejectStats["alt_unverified"]++
				continue
			}
			attempts = append(attempts, attempt{a.LongExchange, a.ShortExchange, a.Spread, a.IntervalHours, a})
		}

		for _, at := range attempts {
			// Per-attempt spot-futures occupancy check — primary pair was
			// already filtered in filterArgmaxOpps, but alternatives use
			// different exchange combinations and must be tested here.
			attemptOpp := models.Opportunity{
				Symbol:        opp.Symbol,
				LongExchange:  at.longExch,
				ShortExchange: at.shortExch,
			}
			if blockedExch, blocked := ppCrossEngineBlocked(attemptOpp, spotFuturesOccupied); blocked {
				e.log.Info("[argmax] filter: %s %s>%s skipped — SF active on %s (cross-engine block)",
					opp.Symbol, at.longExch, at.shortExch, blockedExch)
				rejectStats["spot_futures_occupied"]++
				continue
			}
			if at.alt != nil {
				if _, okL := e.exchanges[at.longExch]; !okL {
					rejectStats["exchange_missing"]++
					continue
				}
				if _, okS := e.exchanges[at.shortExch]; !okS {
					rejectStats["exchange_missing"]++
					continue
				}
				if at.spread <= 0 {
					rejectStats["nonpositive_spread"]++
					continue
				}
				altOpp := models.Opportunity{
					Symbol:        opp.Symbol,
					LongExchange:  at.longExch,
					ShortExchange: at.shortExch,
					Spread:        at.spread,
					IntervalHours: at.intervalHours,
					NextFunding:   at.alt.NextFunding,
					OIRank:        opp.OIRank,
				}
				if reason := e.discovery.CheckPairFilters(altOpp); reason != "" {
					rejectStats["pair_filter"]++
					continue
				}
			}

			approval, err := e.risk.SimulateApprovalForPair(opp, at.longExch, at.shortExch, nil, at.alt, cache)
			if err != nil {
				rejectStats["approval_error"]++
				continue
			}
			if approval == nil {
				rejectStats["approval_nil"]++
				continue
			}

			if approval.Approved {
				if approval.Size <= 0 || approval.Price <= 0 {
					rejectStats["approved_unpriced"]++
					continue
				}
				entryNotional := approval.Size * approval.Price
				required := approval.RequiredMargin
				if required <= 0 {
					required = e.cfg.CapitalPerLeg * e.cfg.MarginSafetyMultiplier
				}
				out = append(out, allocatorChoice{
					symbol:         opp.Symbol,
					longExchange:   at.longExch,
					shortExchange:  at.shortExch,
					spreadBpsH:     at.spread,
					intervalHours:  at.intervalHours,
					requiredMargin: required,
					entryNotional:  entryNotional,
					baseValue:      computeAllocatorBaseValue(at.spread, entryNotional, at.longExch, at.shortExch, e.cfg.MinHoldTime.Hours(), fees),
					altPair:        at.alt,
				})
				continue
			}

			if approval.Kind != models.RejectionKindCapital {
				rejectStats[approval.Kind.String()]++
				continue
			}

			// Capital-rescue admission: full pricing data must be present so
			// dryRunTransferPlan can size transfers and scoring has a notional.
			required := approval.RequiredMargin
			if required <= 0 {
				fallback := e.effectiveCapitalPerLeg() * e.cfg.MarginSafetyMultiplier
				if fallback > 0 {
					required = fallback
				}
			}
			if required <= 0 || approval.Size <= 0 || approval.Price <= 0 {
				rejectStats["capital_unpriced"]++
				continue
			}

			// Replay approval with projected post-transfer balances so we
			// (a) use the actual trade size after funding (real entry resizes
			// upward at the higher balance, which would underestimate margin
			// if we used the rejected pre-transfer size), and (b) exercise
			// all non-capital gates (slippage / price gap / spread stability /
			// exchange health) that Capital rejection short-circuited on the
			// first pass. If the projected approval still rejects, the rescue
			// would only pay donor fees without producing a trade — drop.
			replayApproval := e.replayArgmaxApprovalAfterFunding(opp, at.longExch, at.shortExch, at.alt, balances, cache, approval)
			if replayApproval == nil {
				rejectStats["rescue_post_transfer_rejected"]++
				continue
			}
			// Use the replay's post-transfer sizing for scoring and reservation.
			required = replayApproval.RequiredMargin
			if required <= 0 {
				required = e.effectiveCapitalPerLeg() * e.cfg.MarginSafetyMultiplier
			}
			if required <= 0 || replayApproval.Size <= 0 || replayApproval.Price <= 0 {
				rejectStats["rescue_replay_unpriced"]++
				continue
			}

			entryNotional := replayApproval.Size * replayApproval.Price
			out = append(out, allocatorChoice{
				symbol:         opp.Symbol,
				longExchange:   at.longExch,
				shortExchange:  at.shortExch,
				spreadBpsH:     at.spread,
				intervalHours:  at.intervalHours,
				requiredMargin: required,
				entryNotional:  entryNotional,
				baseValue:      computeAllocatorBaseValue(at.spread, entryNotional, at.longExch, at.shortExch, e.cfg.MinHoldTime.Hours(), fees),
				altPair:        at.alt,
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].baseValue != out[j].baseValue {
			return out[i].baseValue > out[j].baseValue
		}
		// Deterministic tie-break on symbol then exchange pair to make
		// enumeration order stable across runs (map iteration in the caller
		// upstream is non-deterministic otherwise).
		if out[i].symbol != out[j].symbol {
			return out[i].symbol < out[j].symbol
		}
		if out[i].longExchange != out[j].longExchange {
			return out[i].longExchange < out[j].longExchange
		}
		return out[i].shortExchange < out[j].shortExchange
	})
	return out, cache, rejectStats
}

// filterArgmaxOpps drops opportunities whose symbol is already occupied by a
// live leg, is blacklisted in Redis, or whose exchange+symbol is occupied by
// the spot-futures engine. Matches the pre-filter logic used by
// rebalanceFundsLegacy at engine.go:580-604 AND the cross-engine block at
// engine.go:2470 / 2621 so argmax does not plan transfers for pairs
// executeArbitrage would deterministically skip.
func (e *Engine) filterArgmaxOpps(opps []models.Opportunity, active []*models.ArbitragePosition, rejectStats map[string]int) []models.Opportunity {
	activeSymbols := make(map[string]bool)
	for _, p := range active {
		if p != nil && p.Status != models.StatusClosed {
			activeSymbols[p.Symbol] = true
		}
	}

	// Build the spot-futures occupancy map the same way executeArbitrage
	// does. keys are "exchange:symbol"; matches ppCrossEngineBlocked shape.
	spotFuturesKeys, _ := e.buildSpotFuturesMaps()
	spotFuturesOccupied := make(map[string]bool, len(spotFuturesKeys))
	for key := range spotFuturesKeys {
		parts := strings.SplitN(key, ":", 3)
		if len(parts) != 3 {
			continue
		}
		spotFuturesOccupied[parts[0]+":"+parts[1]] = true
	}

	out := make([]models.Opportunity, 0, len(opps))
	for _, opp := range opps {
		if activeSymbols[opp.Symbol] {
			rejectStats["symbol_active"]++
			continue
		}
		if blocked, err := e.db.IsBlacklisted(opp.Symbol); err == nil && blocked {
			rejectStats["blacklisted"]++
			continue
		}
		if blockedExch, blocked := ppCrossEngineBlocked(opp, spotFuturesOccupied); blocked {
			e.log.Info("[argmax] filter: %s skipped — SF active on %s (cross-engine block)", opp.Symbol, blockedExch)
			rejectStats["spot_futures_occupied"]++
			continue
		}
		out = append(out, opp)
	}
	return out
}

// argmaxBetter compares two scored combos under a deterministic ordering:
// higher netPnL wins; on tie, fewer transfer steps wins (less execution
// surface); on tie, lexicographically smaller symbolsKey wins (stable seed
// for otherwise-identical combos so map-iteration order / candidate-sort
// instability cannot flip the selection).
func argmaxBetter(cand, best scoredCombo) bool {
	if cand.netPnL != best.netPnL {
		return cand.netPnL > best.netPnL
	}
	if len(cand.dryRun.Steps) != len(best.dryRun.Steps) {
		return len(cand.dryRun.Steps) < len(best.dryRun.Steps)
	}
	return cand.symbolsKey < best.symbolsKey
}

// scoredCombo is the shared shape used by the argmax stream-best loop and the
// tie-break comparator so the comparator is not a closure capturing state.
type scoredCombo struct {
	choices    []allocatorChoice
	dryRun     dryRunResult
	baseValue  float64
	netPnL     float64
	symbolsKey string
}

// comboSymbolsKey renders a stable lexicographic key for a combo by joining
// symbol|long|short triples sorted. Used solely as a tie-break seed.
func comboSymbolsKey(choices []allocatorChoice) string {
	parts := make([]string, len(choices))
	for i, c := range choices {
		parts[i] = c.symbol + "|" + c.longExchange + "|" + c.shortExchange
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// hasDuplicateSymbols reports whether any two choices share a symbol. Required
// because flat-subset enumeration can pick both the primary pair and an
// alternative pair for the same symbol.
func hasDuplicateSymbols(choices []allocatorChoice) bool {
	seen := make(map[string]bool, len(choices))
	for _, c := range choices {
		if seen[c.symbol] {
			return true
		}
		seen[c.symbol] = true
	}
	return false
}

// violatesDonorFloorFromSteps returns true when any donor's post-transfer
// gross capacity drops below floorPct of its initial capacity. Uses the same
// allocatorDonorGrossCapacity helper the allocator relies on so the floor is
// measured against the same denominator.
func (e *Engine) violatesDonorFloorFromSteps(steps []transferStep, balances map[string]rebalanceBalanceInfo,
	combo []allocatorChoice, floorPct float64) bool {
	if floorPct <= 0 || len(steps) == 0 {
		return false
	}
	spent := map[string]float64{}
	for _, s := range steps {
		spent[s.From] += s.Amount + s.Fee
	}
	needs := argmaxNeedsFromChoices(combo)
	for donor, used := range spent {
		bal, ok := balances[donor]
		if !ok {
			continue
		}
		initial := e.allocatorDonorGrossCapacity(donor, bal, needs[donor])
		if initial <= 0 {
			continue
		}
		if (initial-used)/initial < floorPct {
			return true
		}
	}
	return false
}

// argmaxNeedsFromChoices computes per-exchange required margin by summing
// requiredMargin across both legs of every choice in the subset. Matches
// allocator.go reservation semantics where requiredMargin applies to BOTH
// legs independently (not a split).
func argmaxNeedsFromChoices(choices []allocatorChoice) map[string]float64 {
	out := map[string]float64{}
	for _, c := range choices {
		out[c.longExchange] += c.requiredMargin
		out[c.shortExchange] += c.requiredMargin
	}
	return out
}

// symbolsOf extracts the symbol list from a set of choices, in order. Used
// for the [argmax] pick log line.
func symbolsOf(choices []allocatorChoice) []string {
	out := make([]string, len(choices))
	for i, c := range choices {
		out[i] = c.symbol
	}
	return out
}

// formatArgmaxSteps renders transfer steps as a compact human-readable list
// for telemetry. Example: "[bitget->bingx:147.00 (fee 0.50 chain APT)]".
func formatArgmaxSteps(steps []transferStep) string {
	if len(steps) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, s := range steps {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%s->%s:%.2f", s.From, s.To, s.Amount)
		if s.Fee > 0 || s.Chain != "" {
			fmt.Fprintf(&b, " (fee %.2f", s.Fee)
			if s.Chain != "" {
				fmt.Fprintf(&b, " chain %s", s.Chain)
			}
			b.WriteByte(')')
		}
	}
	b.WriteByte(']')
	return b.String()
}

// logArgmaxRejects emits a single "reject" log line per reason-count pair.
// Skipped entirely when there is nothing to report so a clean enumeration
// doesn't spam the log.
func (e *Engine) logArgmaxRejects(stats map[string]int) {
	if len(stats) == 0 {
		return
	}
	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		e.log.Info("[argmax] reject reason=%s count=%d", k, stats[k])
	}
}

// enumerateCombinations visits every k-subset of items in lexicographic index
// order, invoking visit(combo) for each. Returning false from visit stops the
// iteration (used for deadline breaks). items are assumed not to mutate
// during iteration — the caller gets a reused backing slice and must copy if
// it needs to retain the combo.
func enumerateCombinations(items []allocatorChoice, k int, visit func([]allocatorChoice) bool) {
	n := len(items)
	if k <= 0 || k > n {
		return
	}
	idx := make([]int, k)
	for i := range idx {
		idx[i] = i
	}
	combo := make([]allocatorChoice, k)
	for {
		for i := 0; i < k; i++ {
			combo[i] = items[idx[i]]
		}
		if !visit(combo) {
			return
		}
		// Advance to the next lex index tuple.
		pos := k - 1
		for pos >= 0 && idx[pos] == n-k+pos {
			pos--
		}
		if pos < 0 {
			return
		}
		idx[pos]++
		for i := pos + 1; i < k; i++ {
			idx[i] = idx[i-1] + 1
		}
	}
}

// replayArgmaxApprovalAfterFunding re-runs SimulateApprovalForPair with a cache
// whose Balances reflect the capital-rescue transfer already applied to both
// legs. Returns the approved RiskApproval when the candidate would pass under
// projected post-transfer balances, or nil when it would still be rejected for
// any reason (capital, market, spread, slippage, exchange health, etc.).
//
// This is the safety net for two production risks in argmax capital-rescue:
//  1. Size resize upward: the rejected approval sizes from the low pre-transfer
//     balance, but real entry at execute time resizes based on the funded
//     balance, which may need more margin than the transfer provided.
//  2. Non-capital gates: Capital rejection short-circuits approveInternal
//     before slippage / price-gap / spread-stability / exchange-health checks
//     run, so a rescued candidate could still be rejected by those gates
//     after funds arrive — wasting donor withdrawal fees on a non-trade.
//
// By projecting the transfer and replaying, we both verify those gates and
// obtain the correct post-transfer Size/Price for argmax scoring.
func (e *Engine) replayArgmaxApprovalAfterFunding(
	opp models.Opportunity,
	longExch, shortExch string,
	alt *models.AlternativePair,
	balances map[string]rebalanceBalanceInfo,
	origCache *risk.PrefetchCache,
	origApproval *models.RiskApproval,
) *models.RiskApproval {
	longBal := balances[longExch]
	shortBal := balances[shortExch]

	longBump := origApproval.LongMarginNeeded
	if longBump <= 0 {
		longBump = origApproval.RequiredMargin
	}
	shortBump := origApproval.ShortMarginNeeded
	if shortBump <= 0 {
		shortBump = origApproval.RequiredMargin
	}

	// Clone projected balances so the replay cache does not mutate the caller's
	// view. Use per-leg margin needed (or RequiredMargin fallback) as the bump.
	projBalances := map[string]*exchange.Balance{
		longExch: {
			Available: longBal.futures + longBump,
			Total:     longBal.futuresTotal + longBump,
		},
		shortExch: {
			Available: shortBal.futures + shortBump,
			Total:     shortBal.futuresTotal + shortBump,
		},
	}

	// Reuse Orderbooks from the first-pass cache so we do not re-fetch.
	// TransferablePerExchange is intentionally nil on the replay cache — the
	// projected balance already represents the rescue-funded state, so the
	// cache top-up logic must not add on top of it.
	replayCache := &risk.PrefetchCache{
		Balances:   projBalances,
		Orderbooks: origCache.Orderbooks,
	}

	replay, err := e.risk.SimulateApprovalForPair(opp, longExch, shortExch, nil, alt, replayCache)
	if err != nil || replay == nil || !replay.Approved {
		return nil
	}
	return replay
}
