package engine

import (
	"math"
	"strings"

	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/exchange"
)

// rebalance.go — §4.3 / §4.5 two-pass allocator flow (Steps 3 + 4 of
// plans/PLAN-allocator-unified.md).
//
// Pass 1: rank-first sequential walk with inline capital rescue.
//   - Approved candidates: SKIP (entry scan opens them by rank) but reserve
//     per-leg margin so later iterations cannot double-commit.
//   - RejectionKindCapital with priced fields: queued as rescue candidates.
//   - All other rejections stay in `remaining` so Pass 2 alt-pair search can
//     find an alt-pair that passes the failed gate.
//
// Step 3.5: MANDATORY post-transfer approval replay. pricedCapitalRejection
// returns BEFORE slippage/gap/spread-stability/health gates, and
// dryRunTransferPlan only validates funding mechanics. Every rescue candidate
// with planned funding is replayed against projected post-transfer balances;
// drops or size changes rebuild `needs` + re-run dryRunTransferPlan.
//
// Pass 2: thin call-through to existing runPoolAllocator with balances cloned
// and Pass-1 reservations subtracted (futures AND maxTransferOut for
// split-account donors), plus remainingSlots reduced by kept count.

// prepareRebalanceContext is the common setup shared by the new two-pass flow
// and any future callers. Loads opps, active positions, snapshots balances,
// and short-circuits when no work is possible.
//
// Returns (opps, active, balances, ok). ok=false signals "skip this cycle"
// with the caller NOT emitting an error (reasons are logged here).
func (e *Engine) prepareRebalanceContext(passedOpps ...[]models.Opportunity) ([]models.Opportunity, []*models.ArbitragePosition, map[string]rebalanceBalanceInfo, bool) {
	var opps []models.Opportunity
	if len(passedOpps) > 0 && len(passedOpps[0]) > 0 {
		opps = passedOpps[0]
	} else {
		opps = e.discovery.GetOpportunities()
	}
	if len(opps) == 0 {
		e.log.Info("rebalance: no opportunities, skipping")
		return nil, nil, nil, false
	}

	active, err := e.db.GetActivePositions()
	if err != nil {
		e.log.Error("rebalance: failed to get active positions: %v", err)
		return nil, nil, nil, false
	}
	remainingSlots := e.cfg.MaxPositions - len(active)
	if remainingSlots <= 0 {
		e.log.Info("rebalance: at max capacity (%d/%d), skipping", len(active), e.cfg.MaxPositions)
		return nil, nil, nil, false
	}

	// Pre-filter: drop opps whose symbol already has an active position or is
	// blacklisted.
	activeSymbols := make(map[string]bool)
	for _, p := range active {
		if p.Status != models.StatusClosed {
			activeSymbols[p.Symbol] = true
		}
	}
	var newOpps []models.Opportunity
	for _, opp := range opps {
		if activeSymbols[opp.Symbol] {
			continue
		}
		if blocked, err := e.db.IsBlacklisted(opp.Symbol); err == nil && blocked {
			continue
		}
		newOpps = append(newOpps, opp)
	}
	if len(newOpps) == 0 {
		e.log.Info("rebalance: all %d opps already have active positions or are blacklisted, skipping", len(opps))
		return nil, nil, nil, false
	}
	if len(newOpps) < len(opps) {
		e.log.Info("rebalance: filtered %d/%d opps (active symbols/blacklist removed)", len(newOpps), len(opps))
	}
	opps = newOpps

	// Build set of exchanges that have active position legs so snapshot
	// balances know which exchanges carry live exposure.
	exchWithPositions := make(map[string]bool)
	for _, p := range active {
		if p.Status == models.StatusClosed {
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
		e.log.Info("rebalance: %s futures=%.2f spot=%.2f futuresTotal=%.2f maintRatio=%.4f usageRatio=%.4f maxTransferOut=%.2f hasPos=%v",
			name, bi.futures, bi.spot, bi.futuresTotal, bi.marginRatio, rebalanceUsageRatio(bi), bi.maxTransferOut, bi.hasPositions)
	}

	return opps, active, balances, true
}

// rebalancePass1 implements the §4.1 canonical flow: walk opps in rank order,
// classify each via SimulateApprovalForPair into (a) approved/skip+reserve,
// (b) capital-short rescue candidate, or (c) non-capital reject → Pass 2.
//
// Returns:
//   - kept: choices for which dry-run transfer + post-transfer replay passed
//     AND executeRebalanceFundingPlan actually funded the legs. These get
//     published as allocator overrides by the caller.
//   - pass2Input: opportunities NOT kept (case (c) non-capital rejects,
//     case (b) failed rescues, case (a) approved already removed).
//   - reserved: merged map of approved-case-(a) + kept-rescue reservations so
//     Pass 2 does not double-commit capital on shared exchanges.
func (e *Engine) rebalancePass1(
	opps []models.Opportunity,
	active []*models.ArbitragePosition,
	balances map[string]rebalanceBalanceInfo,
) (
	kept []allocatorChoice,
	pass2Input []models.Opportunity,
	reserved map[string]float64,
) {
	// Capacity bound: Pass 1 must not reserve or execute transfers for more
	// symbols than `MaxPositions - len(active)` can open. Without this,
	// pass1ReservedSlots can over-count and Pass 2 gets starved, or worse,
	// transfers are executed for symbols the entry scan cannot open.
	remainingSlots := e.cfg.MaxPositions - len(active)
	if remainingSlots < 0 {
		remainingSlots = 0
	}
	// pass1UsedSlots tracks case-(a) approved-sufficient + rescue candidates
	// enqueued. Each such symbol will either (a) be opened by entry Stage 1
	// or (b) have a rescue transfer executed for it — both consume one slot.
	pass1UsedSlots := 0

	reserved = map[string]float64{}
	// approvedCaseA tracks reservations from case (a) separately from rescue
	// reservations, so when rescue execution fails we can rebuild `reserved`
	// without the speculative rescue reservations.
	approvedCaseA := map[string]float64{}

	// `remaining` preserves insertion/rank order via `remainingOrder`; the map
	// gives O(1) lookups for symbol dedup and deletion.
	remainingOrder := make([]string, 0, len(opps))
	remaining := make(map[string]models.Opportunity, len(opps))
	addRemaining := func(opp models.Opportunity) {
		if _, ok := remaining[opp.Symbol]; ok {
			return
		}
		remainingOrder = append(remainingOrder, opp.Symbol)
		remaining[opp.Symbol] = opp
	}

	cache := e.buildTransferableCache(balances)
	// Initialize Orderbooks so SimulateApprovalForPair populates the map on the
	// first approval, and later replay calls can reuse cached books instead of
	// re-fetching from exchanges.
	if cache.Orderbooks == nil {
		cache.Orderbooks = make(map[string]*exchange.Orderbook)
	}

	var candidates []allocatorChoice
	// rescueSymbols tracks symbols that were picked as rescue candidates so
	// we can merge the case-(a) reservations with the surviving kept subset
	// below without double-counting.
	rescueSymbols := map[string]bool{}

	for _, opp := range opps {
		addRemaining(opp)

		// Capacity bound: once Pass 1 has consumed `remainingSlots` between
		// case-(a) approvals and rescue candidates, subsequent opps route to
		// Pass 2 (case-c behavior) — they stay in `remaining`, no reservation,
		// no approval call. Pass 2 still gets a fair shot at them with its
		// own slot accounting.
		if pass1UsedSlots >= remainingSlots {
			e.log.Debug("rebalance: pass1 slots exhausted (%d/%d), skipping %s",
				pass1UsedSlots, remainingSlots, opp.Symbol)
			continue
		}

		approval, err := e.risk.SimulateApprovalForPair(opp, opp.LongExchange, opp.ShortExchange, reserved, nil, cache)
		if err != nil {
			e.log.Debug("rebalance: pass1 simulate error %s %s/%s: %v", opp.Symbol, opp.LongExchange, opp.ShortExchange, err)
			continue
		}
		if approval == nil {
			continue
		}

		switch {
		case approval.Approved:
			// Case (a): capital sufficient — SKIP. Entry scan will open by
			// rank. Reserve per-leg margin so later Pass 1 iterations cannot
			// over-commit.
			longNeed := approval.LongMarginNeeded
			if longNeed <= 0 {
				longNeed = approval.RequiredMargin
			}
			shortNeed := approval.ShortMarginNeeded
			if shortNeed <= 0 {
				shortNeed = approval.RequiredMargin
			}
			if longNeed <= 0 && shortNeed <= 0 {
				// No sizing data — skip reservation to avoid over-blocking
				// subsequent siblings on shared exchanges.
				delete(remaining, opp.Symbol)
				continue
			}
			reserved[opp.LongExchange] += longNeed
			reserved[opp.ShortExchange] += shortNeed
			approvedCaseA[opp.LongExchange] += longNeed
			approvedCaseA[opp.ShortExchange] += shortNeed
			delete(remaining, opp.Symbol) // disjoint-set invariant
			pass1UsedSlots++             // case-(a) consumes a slot (entry scan opens)
			e.log.Debug("rebalance: pass1 case(a) approved %s %s/%s reserved long=%.2f short=%.2f",
				opp.Symbol, opp.LongExchange, opp.ShortExchange, longNeed, shortNeed)

		case approval.Kind == models.RejectionKindCapital && hasPricedApprovalFields(approval):
			// Case (b): capital-short rescue candidate. Enqueue for
			// dryRunTransferPlan + post-transfer replay. Stays in `remaining`
			// until rescue definitively succeeds (kept) — failed rescues
			// stay so Pass 2 alt-pair can retry.
			choice := rescueChoiceFromApproval(opp, approval, e.cfg.CapitalPerLeg*e.cfg.MarginSafetyMultiplier)
			candidates = append(candidates, choice)
			rescueSymbols[opp.Symbol] = true
			longNeed, shortNeed := choice.marginNeeds()
			reserved[opp.LongExchange] += longNeed
			reserved[opp.ShortExchange] += shortNeed
			pass1UsedSlots++ // rescue-candidate consumes a slot (transfer→entry opens)
			e.log.Debug("rebalance: pass1 case(b) rescue-candidate %s %s/%s margin long=%.2f short=%.2f",
				opp.Symbol, opp.LongExchange, opp.ShortExchange, longNeed, shortNeed)

		default:
			// Case (c): non-capital reject (or capital without priced fields).
			// Stays in `remaining` for Pass 2 alt-pair search.
			e.log.Debug("rebalance: pass1 case(c) reject %s kind=%s reason=%s",
				opp.Symbol, approval.Kind.String(), approval.Reason)
		}
	}

	// Early return: no rescue candidates. Only case-(a) reservations remain.
	// pass2Input = everything that wasn't approved (case (c) left in remaining).
	if len(candidates) == 0 {
		pass2Input = opportunitiesInOrder(remaining, remainingOrder)
		reserved = cloneFloatMap(approvedCaseA)
		e.log.Info("rebalance: pass1 scan opps=%d kept=0 pass2=%d",
			len(opps), len(pass2Input))
		return nil, pass2Input, reserved
	}

	// Step 3: first-pass dry-run transfer plan on rescue candidates.
	// CRITICAL: pre-subtract case-(a) reservations from balances so the
	// dry-run does not see earmarked capital as free. Without this, a
	// rescue candidate sharing an exchange with an earlier case-(a) opp
	// under-plans the transfer and strands funds.
	feeCache := map[string]feeEntry{}
	replayBaseBalances := cloneBalancesWithReservations(balances, approvedCaseA)
	dryRun := e.dryRunTransferPlan(candidates, replayBaseBalances, feeCache)

	// Step 3.5: post-transfer approval replay (MANDATORY). Repeat until fixed
	// point or sanity-iteration limit reached (3 rounds).
	survivors, changed := e.postTransferReplayFilter(candidates, dryRun, cache, balances, approvedCaseA)
	if changed {
		// Candidates resized/dropped — re-run dry-run on the new survivor
		// set. Do NOT execute the stale plan.
		dryRun = e.dryRunTransferPlan(survivors, replayBaseBalances, feeCache)
	}

	// Fixed-point iteration. postTransferReplayFilter internally iterates up
	// to 3 rounds, but an outer guard protects against pathological cases.
	for iter := 0; iter < 2 && changed; iter++ {
		survivors, changed = e.postTransferReplayFilter(survivors, dryRun, cache, balances, approvedCaseA)
		if changed {
			dryRun = e.dryRunTransferPlan(survivors, replayBaseBalances, feeCache)
		}
	}

	if len(survivors) == 0 || !dryRun.Feasible {
		// All rescue candidates failed or post-replay plan infeasible. They
		// stay in `remaining` so Pass 2 alt-pair can retry different exchange
		// combinations. Only case-(a) reservations carry forward.
		pass2Input = opportunitiesInOrder(remaining, remainingOrder)
		reserved = cloneFloatMap(approvedCaseA)
		e.log.Info("rebalance: pass1 scan opps=%d kept=0 pass2=%d",
			len(opps), len(pass2Input))
		return nil, pass2Input, reserved
	}

	// Step 4a: apply rescue profitability + donor-floor guards. If the planned
	// transfers do not produce enough net PnL or would drain a donor below
	// the floor, reject the whole rescue batch and route survivors to Pass 2.
	if !e.passesRebalanceRescueGuards(survivors, balances, dryRun) {
		pass2Input = opportunitiesInOrder(remaining, remainingOrder)
		reserved = cloneFloatMap(approvedCaseA)
		e.log.Info("rebalance: pass1 scan opps=%d kept=0 pass2=%d (rescue guards rejected)",
			len(opps), len(pass2Input))
		return nil, pass2Input, reserved
	}

	// Step 4: execute only the dry-run-planned steps.
	needs := buildNeedsFromCandidates(survivors)
	result := e.executeRebalanceFundingPlan(needs, balances, nil, dryRun.Steps)

	// keepFundedChoices replays choices against post-execution balances and
	// funded receivers, dropping any whose legs weren't actually funded.
	keptMap := e.keepFundedChoices(survivors, result.PostBalances, result.FundedReceivers, result.PendingDeposits)
	kept = make([]allocatorChoice, 0, len(keptMap))
	for _, c := range survivors {
		if k, ok := keptMap[c.symbol]; ok {
			kept = append(kept, k)
		}
	}

	// Remove kept symbols from `remaining` so pass2Input excludes successful
	// overrides but retains failed-rescue symbols for alt-pair retry.
	for _, c := range kept {
		delete(remaining, c.symbol)
	}

	pass2Input = opportunitiesInOrder(remaining, remainingOrder)

	// Rebuild `reserved` = case-(a) + kept-rescue. Failed-rescue speculative
	// reservations are dropped so Pass 2 can use that capital for alt-pair.
	reserved = mergeReservations(approvedCaseA, reservationsFromKept(kept))

	e.log.Info("rebalance: pass1 scan opps=%d kept=%d pass2=%d",
		len(opps), len(kept), len(pass2Input))
	e.log.Info("rebalance: pass1 execute local=%v cross=%v funded=%v pending=%v unfunded=%v skip=%v kept=%d/%d",
		result.LocalTransferHappened, result.CrossTransferHappened,
		result.FundedReceivers, result.PendingDeposits, result.Unfunded,
		result.SkipReasons, len(kept), len(survivors))

	_ = rescueSymbols // reserved for future diagnostics
	return kept, pass2Input, reserved
}

// postTransferReplayFilter re-runs SimulateApprovalForPair for every rescue
// candidate with planned funding on either leg. Operates on the full
// candidate list and returns only survivors that remained Approved under
// projected post-transfer balances.
//
// Iterates up to 3 rounds internally so mid-batch size changes that shift
// accumulated funding on shared exchanges converge.
//
// Returns (survivors, changed). changed=true means the caller MUST re-run
// dryRunTransferPlan before executing — the stale plan may overfund or
// underfund relative to the new survivor set.
func (e *Engine) postTransferReplayFilter(
	candidates []allocatorChoice,
	dryRun dryRunResult,
	origCache *risk.PrefetchCache,
	balances map[string]rebalanceBalanceInfo,
	approvedCaseA map[string]float64,
) ([]allocatorChoice, bool) {
	if len(candidates) == 0 {
		return candidates, false
	}

	// Bug 5 fix: start from balances that already have case-(a) reservations
	// removed, so rescue candidates sharing an exchange with an earlier
	// case-(a) symbol do not see that earmarked capital as free.
	projected := cloneBalancesWithReservations(balances, approvedCaseA)
	for _, step := range dryRun.Steps {
		// Bug 6 fix: donor debit semantics differ for unified vs split-account
		// exchanges. projectTransferStepOnBalance mirrors dryRunTransferPlan.
		src := projected[step.From]
		projected[step.From] = e.projectTransferStepOnBalance(step, src)

		dst := projected[step.To]
		dst.futures += step.Amount
		dst.futuresTotal += step.Amount
		projected[step.To] = dst
	}

	// Convert projected rebalanceBalanceInfo to *exchange.Balance for the
	// PrefetchCache the risk manager consumes.
	projBalances := make(map[string]*exchange.Balance, len(projected))
	for name, b := range projected {
		projBalances[name] = &exchange.Balance{
			Available:                   b.futures,
			Total:                       b.futuresTotal,
			MarginRatio:                 b.marginRatio,
			MaxTransferOut:              b.maxTransferOut,
			MaxTransferOutAuthoritative: b.maxTransferOutAuthoritative,
		}
	}
	replayCache := &risk.PrefetchCache{
		Balances:   projBalances,
		Orderbooks: origCache.Orderbooks,
		// TransferablePerExchange intentionally nil — projected balances
		// already represent the rescue-funded state; cache top-up logic must
		// not add on top of it.
	}

	survivors := make([]allocatorChoice, 0, len(candidates))
	// replayReserved accumulates per-exchange margin from survivors already
	// accepted earlier in this replay pass. Do NOT pass the outer `reserved`
	// map: it already contains case-(a) reservations (baked into projected
	// balances above) and speculative rescue reservations — using it would
	// double-count.
	replayReserved := map[string]float64{}
	changed := false

	for _, c := range candidates {
		opp := models.Opportunity{
			Symbol:        c.symbol,
			LongExchange:  c.longExchange,
			ShortExchange: c.shortExchange,
			Spread:        c.spreadBpsH,
			IntervalHours: c.intervalHours,
		}
		approval, err := e.risk.SimulateApprovalForPair(opp, c.longExchange, c.shortExchange, replayReserved, c.altPair, replayCache)
		if err != nil {
			e.log.Info("rebalance: pass1 replay dropped %s %s/%s: approval error: %v",
				c.symbol, c.longExchange, c.shortExchange, err)
			changed = true
			continue
		}
		if approval == nil || !approval.Approved {
			reason := "nil approval"
			kind := ""
			if approval != nil {
				reason = approval.Reason
				kind = approval.Kind.String()
			}
			e.log.Info("rebalance: pass1 replay dropped %s %s/%s: %s (kind=%s)",
				c.symbol, c.longExchange, c.shortExchange, reason, kind)
			changed = true
			continue
		}
		if approval.Size <= 0 || approval.Price <= 0 {
			e.log.Info("rebalance: pass1 replay dropped %s %s/%s: unpriced approval",
				c.symbol, c.longExchange, c.shortExchange)
			changed = true
			continue
		}

		// Approved under projected balances. Update sizing if the replay
		// resized or re-margined relative to the original capital-rejection
		// approval we built the choice from.
		updated := c
		longNeed := approval.LongMarginNeeded
		if longNeed <= 0 {
			longNeed = approval.RequiredMargin
		}
		shortNeed := approval.ShortMarginNeeded
		if shortNeed <= 0 {
			shortNeed = approval.RequiredMargin
		}
		required := approval.RequiredMargin
		if required <= 0 {
			required = math.Max(longNeed, shortNeed)
		}
		if required <= 0 {
			required = e.cfg.CapitalPerLeg * e.cfg.MarginSafetyMultiplier
		}
		entryNotional := approval.Size * approval.Price

		if updated.longRequiredMargin != longNeed ||
			updated.shortRequiredMargin != shortNeed ||
			updated.requiredMargin != required ||
			updated.entryNotional != entryNotional {
			changed = true
		}
		updated.longRequiredMargin = longNeed
		updated.shortRequiredMargin = shortNeed
		updated.requiredMargin = required
		updated.entryNotional = entryNotional
		survivors = append(survivors, updated)

		// Accumulate survivor reservation so later candidates in this replay
		// pass see their margin consumed.
		replayReserved[updated.longExchange] += longNeed
		replayReserved[updated.shortExchange] += shortNeed
	}

	if len(survivors) != len(candidates) {
		changed = true
	}
	return survivors, changed
}

// runPoolAllocatorPass2 is the §4.5 thin call-through. It clones balances,
// subtracts Pass-1 reservations (futures AND maxTransferOut), reduces
// remainingSlots by pass1KeptCount, then delegates to runPoolAllocator
// unchanged. All transfer/rescue, donor prep, local relief, min-withdraw,
// multi-donor, executor-replay logic is preserved.
//
// `runPoolAllocator` has no `reserved` parameter and resets its own
// reservations internally, so the only way to carry Pass-1 reservations into
// Pass 2 is to pre-adjust the balance map.
func (e *Engine) runPoolAllocatorPass2(
	pass2Input []models.Opportunity,
	balances map[string]rebalanceBalanceInfo,
	reserved map[string]float64,
	pass1KeptCount int,
) {
	if !e.cfg.EnablePoolAllocator {
		e.log.Info("rebalance: pass-2 skipped (pool allocator disabled)")
		return
	}
	if len(pass2Input) == 0 {
		return
	}

	active, err := e.db.GetActivePositions()
	if err != nil {
		e.log.Warn("rebalance: pass-2 failed to count active positions: %v", err)
		return
	}

	remainingSlots := e.cfg.MaxPositions - len(active) - pass1KeptCount
	if remainingSlots <= 0 {
		e.log.Info("rebalance: pass-2 no remaining slots (max=%d active=%d pass1Kept=%d)",
			e.cfg.MaxPositions, len(active), pass1KeptCount)
		return
	}

	adjusted := cloneBalancesWithReservations(balances, reserved)

	sel, err := e.runPoolAllocator(pass2Input, adjusted, remainingSlots)
	if err != nil {
		e.log.Warn("rebalance: pass-2 pool allocator failed: %v", err)
		return
	}
	if sel == nil || !sel.feasible {
		e.log.Info("rebalance: pass-2 pool allocator infeasible")
		return
	}
	e.log.Info("rebalance: pass-2 pool allocator selected %d opps (value=%.4f, choices=%s)",
		len(sel.choices), sel.totalBaseValue, e.formatAllocatorSummary(sel))

	// Execute any transfers planned by the pool allocator. For alt-pair
	// selections that already have enough capital (no transfer needed), the
	// plan steps will be empty — still publish the selections as overrides
	// so the entry scan can act on them.
	result := e.executeRebalanceFundingPlan(sel.needs, adjusted, nil, sel.plan.Steps)
	kept := e.keepFundedChoices(sel.choices, result.PostBalances, result.FundedReceivers, result.PendingDeposits)
	if len(kept) == 0 {
		e.log.Info("rebalance: pass-2 stored 0 allocator overrides after funding replay")
		return
	}

	e.allocOverrideMu.Lock()
	// Merge Pass-1 and Pass-2 overrides. rebalanceFunds already published
	// Pass-1 overrides via e.executePicks → e.allocOverrides. Add Pass-2
	// kept choices on top of them so the entry scan sees both.
	if e.allocOverrides == nil {
		e.allocOverrides = make(map[string]allocatorChoice, len(kept))
	}
	for sym, c := range kept {
		e.allocOverrides[sym] = c
	}
	e.allocOverrideMu.Unlock()

	e.log.Info("rebalance: pass-2 execute local=%v cross=%v funded=%v pending=%v kept=%d/%d",
		result.LocalTransferHappened, result.CrossTransferHappened,
		result.FundedReceivers, result.PendingDeposits, len(kept), len(sel.choices))
}

// cloneBalancesWithReservations returns a copy of `balances` with
// `reserved[exch]` subtracted from both `futures` and (when set)
// `maxTransferOut` on each exchange. Leaves `futuresTotal` unchanged so
// account equity tracking stays consistent — only available-for-new-use
// capital is reduced.
//
// For split-account exchanges the maxTransferOut reduction is critical:
// without it Pass 2 could plan a cross-exchange withdrawal that dips into
// capital Pass 1 has already reserved for its own kept choices.
func cloneBalancesWithReservations(
	balances map[string]rebalanceBalanceInfo,
	reserved map[string]float64,
) map[string]rebalanceBalanceInfo {
	adjusted := cloneRebalanceBalances(balances)
	for exch, amt := range reserved {
		if amt <= 0 {
			continue
		}
		b := adjusted[exch]
		b.futures = math.Max(0, b.futures-amt)
		if b.maxTransferOut > 0 {
			// Split-account donors: reserved capital must not be withdrawn
			// by Pass 2. maxTransferOut=0 means unknown/API-failed and is
			// left alone to avoid flipping it from "unknown" to "zero".
			b.maxTransferOut = math.Max(0, b.maxTransferOut-amt)
		}
		adjusted[exch] = b
	}
	return adjusted
}

// executePicks publishes Pass-1 kept choices as allocator overrides for the
// entry scan to consume in Stage 2, under the allocOverride mutex.
func (e *Engine) executePicks(kept []allocatorChoice) {
	if len(kept) == 0 {
		return
	}
	overrides := make(map[string]allocatorChoice, len(kept))
	for _, c := range kept {
		overrides[c.symbol] = c
	}
	e.allocOverrideMu.Lock()
	e.allocOverrides = overrides
	e.allocOverrideMu.Unlock()
	e.log.Info("rebalance: stored %d pass-1 allocator overrides for entry scan", len(kept))
}

// projectBalancesAfterFunding returns a copy of `balances` with per-leg
// margin of each kept choice subtracted from futures (and maxTransferOut
// when set). Equivalent to cloneBalancesWithReservations applied to a
// reserved-map built from kept choices.
func projectBalancesAfterFunding(
	balances map[string]rebalanceBalanceInfo,
	kept []allocatorChoice,
) map[string]rebalanceBalanceInfo {
	if len(kept) == 0 {
		return balances
	}
	res := reservationsFromKept(kept)
	return cloneBalancesWithReservations(balances, res)
}

// ----- small helpers --------------------------------------------------------

// hasPricedApprovalFields returns true when a capital-rejection approval
// carries enough pricing data for dryRunTransferPlan sizing and margin
// accounting during Pass 1 rescue candidate construction.
func hasPricedApprovalFields(a *models.RiskApproval) bool {
	if a == nil {
		return false
	}
	if a.Size <= 0 || a.Price <= 0 {
		return false
	}
	required := a.RequiredMargin
	if required <= 0 {
		required = math.Max(a.LongMarginNeeded, a.ShortMarginNeeded)
	}
	return required > 0
}

// rescueChoiceFromApproval builds an allocatorChoice from a capital-rejection
// approval. `fallbackMargin` is used only when the approval carries no
// RequiredMargin/per-leg fields (should not happen for priced rejections but
// kept as a defensive default).
func rescueChoiceFromApproval(opp models.Opportunity, a *models.RiskApproval, fallbackMargin float64) allocatorChoice {
	longNeed := a.LongMarginNeeded
	if longNeed <= 0 {
		longNeed = a.RequiredMargin
	}
	shortNeed := a.ShortMarginNeeded
	if shortNeed <= 0 {
		shortNeed = a.RequiredMargin
	}
	required := a.RequiredMargin
	if required <= 0 {
		required = math.Max(longNeed, shortNeed)
	}
	if required <= 0 && fallbackMargin > 0 {
		required = fallbackMargin
		if longNeed <= 0 {
			longNeed = fallbackMargin
		}
		if shortNeed <= 0 {
			shortNeed = fallbackMargin
		}
	}
	entryNotional := a.Size * a.Price
	return allocatorChoice{
		symbol:              opp.Symbol,
		longExchange:        opp.LongExchange,
		shortExchange:       opp.ShortExchange,
		spreadBpsH:          opp.Spread,
		intervalHours:       opp.IntervalHours,
		requiredMargin:      required,
		longRequiredMargin:  longNeed,
		shortRequiredMargin: shortNeed,
		entryNotional:       entryNotional,
		baseValue:           entryNotional, // crude fallback; unused for non-subset flow
		altPair:             nil,
	}
}

// opportunitiesInOrder returns the opportunity values from `remaining` in
// the order recorded by `order`. Symbols removed from `remaining` are
// skipped. Preserves the original rank ordering so Pass 2 sees opps in the
// same order Pass 1 did.
func opportunitiesInOrder(remaining map[string]models.Opportunity, order []string) []models.Opportunity {
	out := make([]models.Opportunity, 0, len(remaining))
	for _, sym := range order {
		if opp, ok := remaining[sym]; ok {
			out = append(out, opp)
		}
	}
	return out
}

// buildNeedsFromCandidates aggregates per-leg margin needs per exchange.
// Each candidate contributes its long-leg need to the long exchange and its
// short-leg need to the short exchange (independent, not a split).
func buildNeedsFromCandidates(candidates []allocatorChoice) map[string]float64 {
	out := map[string]float64{}
	for _, c := range candidates {
		longNeed, shortNeed := c.marginNeeds()
		out[c.longExchange] += longNeed
		out[c.shortExchange] += shortNeed
	}
	return out
}

// mergeReservations combines two reservation maps without mutating either.
// Used to merge case-(a) reservations with kept-rescue reservations after
// Pass 1 completes.
func mergeReservations(a, b map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(a)+len(b))
	for k, v := range a {
		out[k] += v
	}
	for k, v := range b {
		out[k] += v
	}
	return out
}

// reservationsFromKept builds a per-exchange reservation map from the kept
// choice list, matching allocator.go:231-232 semantics (requiredMargin
// applies to each leg's own exchange independently).
func reservationsFromKept(kept []allocatorChoice) map[string]float64 {
	out := map[string]float64{}
	for _, c := range kept {
		longNeed, shortNeed := c.marginNeeds()
		out[c.longExchange] += longNeed
		out[c.shortExchange] += shortNeed
	}
	return out
}

// symbolOrderString is a diagnostic helper for logging the rank order of
// pass2Input. Not currently called but kept handy for debug tracing.
func symbolOrderString(opps []models.Opportunity) string {
	parts := make([]string, len(opps))
	for i, o := range opps {
		parts[i] = o.Symbol
	}
	return strings.Join(parts, ",")
}

// passesRebalanceRescueGuards applies profitability + donor-floor checks on a
// Pass 1 rescue batch BEFORE executing transfers. Rejects batches whose net
// PnL (spread-based base value minus total transfer fees) is below
// RebalanceMinNetPnLUSDT, or that would drain any donor below the
// RebalanceDonorFloorPct fraction of its initial gross capacity.
func (e *Engine) passesRebalanceRescueGuards(
	choices []allocatorChoice,
	balances map[string]rebalanceBalanceInfo,
	dryRun dryRunResult,
) bool {
	base := e.rebalanceChoicesBaseValue(choices)
	net := base - dryRun.TotalFee
	if e.cfg.RebalanceMinNetPnLUSDT > 0 && net < e.cfg.RebalanceMinNetPnLUSDT {
		e.log.Info("rebalance: pass1 rescue rejected by min net pnl: net=%.4f min=%.4f base=%.4f fee=%.4f",
			net, e.cfg.RebalanceMinNetPnLUSDT, base, dryRun.TotalFee)
		return false
	}
	if e.violatesDonorFloorFromSteps(dryRun.Steps, balances, choices, e.cfg.RebalanceDonorFloorPct) {
		e.log.Info("rebalance: pass1 rescue rejected by donor floor: floor=%.4f",
			e.cfg.RebalanceDonorFloorPct)
		return false
	}
	return true
}

// rebalanceChoicesBaseValue sums expected base-value PnL (spread × notional
// scaled to hold duration, minus trade fees) across all rescue choices. This
// is the same metric the pool allocator uses for subset scoring.
func (e *Engine) rebalanceChoicesBaseValue(choices []allocatorChoice) float64 {
	fees := e.getEffectiveAllocatorFees()
	holdHours := e.cfg.MinHoldTime.Hours()
	total := 0.0
	for _, c := range choices {
		total += computeAllocatorBaseValue(c.spreadBpsH, c.entryNotional, c.longExchange, c.shortExchange, holdHours, fees)
	}
	return total
}

// violatesDonorFloorFromSteps returns true if any donor in the planned
// transfer steps would end up with less than floorPct of its initial gross
// capacity. floorPct ≤ 0 disables the floor.
func (e *Engine) violatesDonorFloorFromSteps(
	steps []transferStep,
	balances map[string]rebalanceBalanceInfo,
	choices []allocatorChoice,
	floorPct float64,
) bool {
	if floorPct <= 0 {
		return false
	}
	spent := map[string]float64{}
	for _, step := range steps {
		spent[step.From] += step.Amount + step.Fee
	}
	needs := buildNeedsFromCandidates(choices)
	for donor, used := range spent {
		initial := e.allocatorDonorGrossCapacity(donor, balances[donor], needs[donor])
		if initial > 0 && (initial-used)/initial < floorPct {
			return true
		}
	}
	return false
}

// projectTransferStepOnBalance mirrors dryRunTransferPlan's actual debit
// semantics so postTransferReplayFilter's projected balances match reality:
//   - Unified-account donor (Bybit UTA, Gate.io cross): external withdrawal
//     debits the unified futures pool, so subtract step.Amount+step.Fee
//     from src.futures.
//   - Split-account donor (Binance/OKX/Bitget/BingX): external withdrawal
//     comes from the SPOT pool (after a prep futures→spot move inside
//     dryRunTransferPlan). transferStep carries only the external withdrawal
//     leg, so debit src.spot and leave src.futures alone.
//
// Without this split, postTransferReplayFilter would shrink a split-account
// donor's futures even though dryRunTransferPlan left it untouched — any
// other rescue candidate trading on that donor's futures side would be
// falsely resized or dropped.
func (e *Engine) projectTransferStepOnBalance(step transferStep, src rebalanceBalanceInfo) rebalanceBalanceInfo {
	donorIsUnified := false
	if exch, ok := e.exchanges[step.From]; ok {
		if uc, ok := exch.(interface{ IsUnified() bool }); ok && uc.IsUnified() {
			donorIsUnified = true
		}
	}

	if donorIsUnified {
		src.futures = math.Max(0, src.futures-step.Amount-step.Fee)
		src.futuresTotal = math.Max(0, src.futuresTotal-step.Amount-step.Fee)
		return src
	}

	// Split-account donor: debit spot only. maxTransferOut is also reduced
	// since withdrawals consume it.
	src.spot = math.Max(0, src.spot-step.Amount-step.Fee)
	if src.maxTransferOut > 0 {
		src.maxTransferOut = math.Max(0, src.maxTransferOut-step.Amount-step.Fee)
	}
	return src
}
