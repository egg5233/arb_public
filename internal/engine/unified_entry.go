package engine

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"arb/internal/models"
	"arb/internal/risk"
)

// unified_entry.go — cross-strategy entry orchestrator.
//
// Runs once per EntryScan tick (at `cfg.EntryScanMinute`) when
// `unifiedOwnerReady()` is true AND `e.spotEntry` has been installed.
// Responsibilities:
//   - gather perp and spot candidates (fresh, de-staled)
//   - consume rebalance `allocOverrides` once (shared helper)
//   - build per-symbol GroupKey candidate bundles (cross-strategy dedup)
//   - solve via selection_core.solveGroupedSearch
//   - atomically pre-hold capital for ALL winners via ReserveBatch
//   - dispatch winners (perp via executeArbitrage with preheld reservation,
//     spot via spotEntry.OpenSelectedEntry) in parallel
//   - release any un-committed reservations on dispatch error
//
// Design boundaries:
//   - Pure orchestration: no low-level trade execution or rebalance logic.
//   - Reuses existing feasibility primitives (risk.ApproveWithReservedCached
//     on the perp side; spotEntry.BuildEntryPlan on the spot side).
//   - Legacy safety nets (engine.go:2103 perp dup guard, spotengine/
//     execution.go:143 spot dup guard) remain in place.

// perpDispatchRequest bundles a perp opportunity with its risk approval
// so the dispatch stage can call executeTradeV2WithPos without re-running
// feasibility or re-reserving capital. Approval.Size / .Price / .GapBPS
// are the values produced by ApproveWithReservedCached during the
// feasibility pass and must be carried forward verbatim into dispatch.
type perpDispatchRequest struct {
	Opp      models.Opportunity
	Approval *models.RiskApproval
}

// unifiedEntryChoice is the selector's per-candidate record. Exactly one
// of Perp / Spot is non-nil — Strategy disambiguates when both are
// trivially checked. Key is the caller-controlled unique identifier used
// by selection_core (GroupKey dedup), ReserveBatch (key->reservation
// mapping) and the dispatcher map.
type unifiedEntryChoice struct {
	Key       string
	Symbol    string
	Strategy  risk.Strategy
	ValueUSDT float64
	SlotCost  int
	Perp      *perpDispatchRequest
	Spot      *models.SpotEntryPlan
}

// runUnifiedEntrySelection is the top-level EntryScan handler when the
// unified cross-strategy selector owns EntryScan dispatch.
//
// Flow (per plan Section 3):
//  1. Inner readiness defense gate (cfg + allocator). Warn+return if
//     somehow reached with allocator disabled (covers config reload
//     races between ownership gate evaluation and dispatch).
//  2. Log one observability line at tick start.
//  3. Gather perp opps from discovery.
//  4. consumeOverridesAndEnrichOpps(perpOpps) — shared helper consumes
//     `allocOverrides` exactly once (covers both tier-2 patch and v0.32.8
//     RescanSymbols branches).
//  5. Gather spot candidates via spotEntry.ListEntryCandidates with
//     freshness window 2 * SpotDiscoveryInterval.
//  6. Build perpDispatchRequest[] (feasibility-filtered via risk.Approve)
//     and spotPlans (built via spotEntry.BuildEntryPlan).
//  7. Load unifiedOccupancy snapshot and groupChoicesBySymbol. Symbols
//     already occupied across either store are excluded.
//  8. solveGroupedSearch with evaluate callback that enforces:
//       - combined slot cap (GlobalSlotsRemaining)
//       - spot sub-cap (SpotSlotsRemaining)
//       - cumulative risk.checkCapsWithOverride (via ReserveBatch at
//         prehold time; evaluate itself uses a cheap approximation)
//  9. ReserveBatch for all winners — atomic all-or-nothing prehold.
//  10. Dispatch perp winners via runUnifiedPerpDispatch (which commits
//      the preheld reservation on success, releases on failure).
//  11. Dispatch spot winners via spotEntry.OpenSelectedEntry.
//  12. ReleaseBatch for any reservation whose dispatch did not commit.
func (e *Engine) runUnifiedEntrySelection() error {
	// Step 1 — inner readiness defense gate.
	if e == nil || e.cfg == nil {
		return errors.New("unified entry: engine or cfg nil")
	}
	if !e.cfg.EnableCapitalAllocator || e.allocator == nil || !e.allocator.Enabled() {
		e.log.Warn("unified entry: skipped — capital allocator not ready (cfg=%v, alloc=%v, enabled=%v)",
			e.cfg.EnableCapitalAllocator,
			e.allocator != nil,
			e.allocator != nil && e.allocator.Enabled())
		return nil
	}
	if e.spotEntry == nil {
		// Defense in depth — the outer gate already checks this but
		// callers can in principle invoke runUnifiedEntrySelection
		// directly (tests, future paths).
		e.log.Warn("unified entry: skipped — spotEntry executor not installed")
		return nil
	}

	// Step 2 — observability log.
	nowMinute := time.Now().UTC().Minute()
	e.log.Info("[engine] unified entry: tick at minute=%d (cfg.EntryScanMinute=%d) unified_entry=on capital_allocator=on",
		nowMinute, e.cfg.EntryScanMinute)

	// Step 3 — gather perp opps.
	originalPerpOpps := e.discovery.GetOpportunities()

	// Step 4 — consume overrides once and apply the stale-override tier-3
	// fallback. The helper preserves the original scan so a stale override
	// does not silently drop the perp side (legacy parity with the
	// EntryScan tier-3 branch at engine.go:1338-1343).
	perpOpps, overrideTier := e.selectUnifiedPerpOpps(originalPerpOpps)
	e.log.Info("unified entry: override tier=%s, perpOpps=%d", overrideTier, len(perpOpps))

	// Step 5 — gather spot candidates with same freshness window legacy
	// uses for perp (engine.go dynamic shift cache).
	scanInterval := time.Duration(e.cfg.SpotFuturesScanIntervalMin) * time.Minute
	if scanInterval < time.Minute {
		scanInterval = 10 * time.Minute
	}
	spotCandidates := e.spotEntry.ListEntryCandidates(2 * scanInterval)
	e.log.Info("unified entry: perpOpps=%d spotCandidates=%d", len(perpOpps), len(spotCandidates))

	// Step 6a — load occupancy snapshot so we can exclude already-held
	// symbols while building candidates (avoids wasted feasibility calls).
	occ, err := e.loadUnifiedOccupancy()
	if err != nil {
		return fmt.Errorf("load occupancy: %w", err)
	}
	if occ.GlobalSlotsRemaining <= 0 {
		e.log.Info("unified entry: no slots remaining (activePerp=%d activeSpot=%d)",
			occ.ActivePerp, occ.ActiveSpot)
		return nil
	}

	// Step 6b — build perp dispatch requests. For each candidate perp
	// opp we run the cheap ApproveWithReservedCached feasibility check;
	// dropped opps don't enter the group map. reserved= is empty here
	// because the unified selector uses ReserveBatch to serialize
	// capital — no per-candidate reserved-accounting during feasibility.
	var perpReqs []*perpDispatchRequest
	{
		reserved := map[string]float64{}
		for _, opp := range perpOpps {
			if _, seen := occ.ActiveSymbols[opp.Symbol]; seen {
				continue // legacy dup guard + cross-strategy dedup
			}
			approval, err := e.risk.ApproveWithReservedCached(opp, reserved, nil)
			if err != nil || approval == nil || !approval.Approved || approval.Size <= 0 {
				continue
			}
			req := &perpDispatchRequest{Opp: opp, Approval: approval}
			perpReqs = append(perpReqs, req)
		}
	}
	e.log.Info("unified entry: perp approved=%d (from %d opps)", len(perpReqs), len(perpOpps))

	// Step 6c — build spot plans.
	var spotPlans []*models.SpotEntryPlan
	for _, c := range spotCandidates {
		if _, seen := occ.ActiveSymbols[c.Symbol]; seen {
			continue
		}
		plan, err := e.spotEntry.BuildEntryPlan(c)
		if err != nil || plan == nil {
			continue
		}
		if plan.PlannedNotionalUSDT <= 0 || plan.PlannedBaseSize <= 0 {
			continue
		}
		spotPlans = append(spotPlans, plan)
	}
	e.log.Info("unified entry: spot plans=%d (from %d candidates)", len(spotPlans), len(spotCandidates))

	if len(perpReqs) == 0 && len(spotPlans) == 0 {
		e.log.Info("unified entry: no viable candidates, nothing to select")
		return nil
	}

	// Step 7 — group by symbol and build searchChoice records.
	groups, keyToChoice := e.groupChoicesBySymbol(perpReqs, spotPlans, occ)
	if len(groups) == 0 {
		e.log.Info("unified entry: no groups after dedup+feasibility")
		return nil
	}

	// Step 8 — solve. Build a capacity snapshot once so the evaluator's
	// per-exchange / per-strategy / total feasibility checks stay cheap
	// (no per-evaluate Redis round-trip). The snapshot mirrors the caps
	// allocator.checkCapsWithOverride enforces at ReserveBatch time, so
	// sets admitted by the evaluator are also admitted by ReserveBatch.
	snap, err := e.buildCapacitySnapshot()
	if err != nil {
		e.log.Warn("unified entry: buildCapacitySnapshot failed: %v", err)
		// Fall through with a permissive snapshot; legacy evaluator
		// would have run without this gate anyway.
		snap = &unifiedCapacitySnapshot{}
	}

	timeout := time.Duration(e.cfg.AllocatorTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Millisecond
	}
	evaluate := e.makeUnifiedEvaluator(keyToChoice, occ, snap)
	winnerKeys := solveGroupedSearch(groups, occ.GlobalSlotsRemaining, timeout, evaluate)
	if len(winnerKeys) == 0 {
		e.log.Info("unified entry: solver returned no winners")
		return nil
	}

	winners := make([]*unifiedEntryChoice, 0, len(winnerKeys))
	for _, k := range winnerKeys {
		c, ok := keyToChoice[k]
		if !ok || c == nil {
			continue
		}
		winners = append(winners, c)
	}
	e.log.Info("unified entry: selected %d winners", len(winners))

	// Step 9 — ReserveBatch all winners atomically.
	items := make([]risk.BatchReservationItem, 0, len(winners))
	for _, w := range winners {
		exposures, cap := e.exposuresForChoice(w)
		if len(exposures) == 0 {
			continue
		}
		items = append(items, risk.BatchReservationItem{
			Key:         w.Key,
			Strategy:    w.Strategy,
			Exposures:   exposures,
			CapOverride: cap,
		})
	}
	batch, err := e.allocator.ReserveBatch(items)
	if err != nil {
		e.log.Warn("unified entry: ReserveBatch failed: %v", err)
		return nil
	}
	if batch == nil || len(batch.Items) == 0 {
		// Allocator disabled or items emptied — nothing to commit.
		e.log.Info("unified entry: ReserveBatch produced no items (allocator disabled or empty exposures)")
		return nil
	}

	// Step 10/11 — dispatch winners in parallel. Each dispatcher is
	// responsible for committing the preheld reservation on success;
	// on error we leave the reservation in `batch` so the final
	// ReleaseBatch cleans it up.
	var wg sync.WaitGroup
	var mu sync.Mutex
	committed := make(map[string]struct{}, len(winners))
	for _, w := range winners {
		res, ok := batch.Items[w.Key]
		if !ok || res == nil {
			// No reservation for this winner (empty exposures, allocator
			// disabled etc.) — skip dispatch; nothing to release either.
			continue
		}
		wg.Add(1)
		go func(choice *unifiedEntryChoice, res *risk.CapitalReservation) {
			defer wg.Done()
			var dispatchErr error
			switch choice.Strategy {
			case risk.StrategyPerpPerp:
				dispatchErr = e.dispatchUnifiedPerp(choice, res)
			case risk.StrategySpotFutures:
				dispatchErr = e.dispatchUnifiedSpot(choice, res)
			default:
				dispatchErr = fmt.Errorf("unknown strategy %q", choice.Strategy)
			}
			if dispatchErr != nil {
				e.log.Error("unified entry: dispatch %s (%s) failed: %v",
					choice.Key, choice.Symbol, dispatchErr)
				return
			}
			mu.Lock()
			committed[choice.Key] = struct{}{}
			mu.Unlock()
		}(w, res)
	}
	wg.Wait()

	// Step 12 — release any un-committed reservations. ReleaseBatch
	// inspects Redis under WATCH and skips already-committed keys, so
	// calling it with the full batch is always safe.
	if err := e.allocator.ReleaseBatch(batch); err != nil {
		e.log.Warn("unified entry: ReleaseBatch cleanup failed: %v", err)
	}
	e.log.Info("unified entry: dispatch complete — committed=%d released=%d",
		len(committed), len(batch.Items)-len(committed))
	return nil
}

// selectUnifiedPerpOpps consumes rebalance allocator overrides (via the
// shared consumeOverridesAndEnrichOpps helper) and applies the stale-
// override tier-3 fallback: when overrides existed but produced no
// usable opps (patch stripped everything, or RescanSymbols returned
// empty) AND the caller passed a non-empty original opp list, we fall
// back to the original list as a rank-based tier so the perp side of
// the unified dispatch is not silently dropped.
//
// This mirrors the legacy EntryScan tier-3 fallback at engine.go:1338-
// 1343; without it, a stale override (e.g. allocator picked a pair whose
// current-scan verification dropped it) would suppress perp entirely
// and attemptAutoEntries has already been suppressed upstream when the
// unified flag is on — both paths dead.
//
// Returns the perp opp slice to feed into the selector plus a human-
// readable tier tag for observability. Tier values:
//   - "none":                     no overrides present, pass-through
//   - "tier-2-override-patch":    tier-2 patched some opps successfully
//   - "tier-2-override-fallback": RescanSymbols found salvageable opps
//   - "tier-3-rank-based":        overrides stale, falling back to input
func (e *Engine) selectUnifiedPerpOpps(originalPerpOpps []models.Opportunity) ([]models.Opportunity, string) {
	perpOpps, overrideTier := e.consumeOverridesAndEnrichOpps(originalPerpOpps)

	if len(perpOpps) == 0 && len(originalPerpOpps) > 0 &&
		(overrideTier == "tier-2-override-patch" || overrideTier == "tier-2-override-fallback-empty") {
		e.log.Warn("unified entry: overrides all stale (tier=%s), falling back to tier-3-rank-based with %d opps",
			overrideTier, len(originalPerpOpps))
		return originalPerpOpps, "tier-3-rank-based"
	}
	return perpOpps, overrideTier
}

// exposuresForChoice returns the (exposures, capOverride) pair for a
// single unifiedEntryChoice, using the same units the live single-
// reservation APIs take:
//   - perp:  per-leg futures margin by exchange
//   - spot:  PlannedNotionalUSDT keyed by the spot exchange
//
// Returns empty exposures for an unknown strategy — the caller drops
// zero-exposure items (matching ReserveWithCap's no-op behavior).
func (e *Engine) exposuresForChoice(c *unifiedEntryChoice) (map[string]float64, float64) {
	if c == nil {
		return nil, 0
	}
	switch c.Strategy {
	case risk.StrategyPerpPerp:
		if c.Perp == nil || c.Perp.Approval == nil {
			return nil, 0
		}
		long := c.Perp.Approval.LongMarginNeeded
		if long <= 0 {
			long = c.Perp.Approval.RequiredMargin
		}
		short := c.Perp.Approval.ShortMarginNeeded
		if short <= 0 {
			short = c.Perp.Approval.RequiredMargin
		}
		if long <= 0 && short <= 0 {
			return nil, 0
		}
		return map[string]float64{
			c.Perp.Opp.LongExchange:  long,
			c.Perp.Opp.ShortExchange: short,
		}, 0
	case risk.StrategySpotFutures:
		if c.Spot == nil {
			return nil, 0
		}
		if c.Spot.PlannedNotionalUSDT <= 0 {
			return nil, 0
		}
		return map[string]float64{
			c.Spot.Candidate.Exchange: c.Spot.PlannedNotionalUSDT,
		}, 0
	}
	return nil, 0
}

// groupChoicesBySymbol turns perp and spot candidate lists into a
// map[symbol][]searchChoice suitable for solveGroupedSearch, plus a
// reverse map from choice Key -> unifiedEntryChoice so the caller can
// reconstruct winners after solving.
//
// Dedup rules:
//   - occ.ActiveSymbols excludes symbols already held in either strategy
//     (plan Section 9).
//   - One GroupKey per symbol means at most one candidate per symbol
//     survives the B&B — the solver's mutual-exclusion rule already
//     enforces this; grouping just hands it the right bucket structure.
//
// Scoring:
//   - perp: uses ApproveWithReservedCached's size * price as entry
//     notional and routes through computeAllocatorBaseValue for parity
//     with the rebalance allocator.
//   - spot: uses scoreSpotEntry.NetValueUSDT.
//
// Non-positive scores are dropped — the solver would ignore them anyway
// but dropping early keeps the group map small.
func (e *Engine) groupChoicesBySymbol(
	perp []*perpDispatchRequest,
	spot []*models.SpotEntryPlan,
	occ *unifiedOccupancy,
) (map[string][]searchChoice, map[string]*unifiedEntryChoice) {
	groups := map[string][]searchChoice{}
	keyToChoice := map[string]*unifiedEntryChoice{}

	holdHours := 0.0
	if e.cfg != nil {
		holdHours = e.cfg.MinHoldTime.Hours()
	}

	// Use the allocator's fee schedule lookup via the engine so perp
	// candidates are ranked net-of-fees. With an empty/nil map the solver
	// would systematically overvalue perp vs spot because spot's scoring
	// (scoreSpotEntry) already nets fees — keep parity by charging perp
	// trading fees at the same granularity the rebalance allocator uses.
	// When discovery is unavailable (tests), getEffectiveAllocatorFees
	// returns the hardcoded allocatorExchangeFees defaults.
	perpFees := e.getEffectiveAllocatorFees()

	// Perp: one choice per approved opp (distinct long/short pair).
	for idx, req := range perp {
		if req == nil || req.Approval == nil {
			continue
		}
		if occ != nil {
			if _, seen := occ.ActiveSymbols[req.Opp.Symbol]; seen {
				continue
			}
		}
		entryNotional := req.Approval.Size * req.Approval.Price
		if entryNotional <= 0 {
			continue
		}
		value := computeAllocatorBaseValue(
			req.Opp.Spread,
			entryNotional,
			req.Opp.LongExchange,
			req.Opp.ShortExchange,
			holdHours,
			perpFees,
		)
		if value <= 0 {
			continue
		}
		key := fmt.Sprintf("perp:%s:%s:%s:%d",
			req.Opp.Symbol, req.Opp.LongExchange, req.Opp.ShortExchange, idx)
		choice := &unifiedEntryChoice{
			Key:       key,
			Symbol:    req.Opp.Symbol,
			Strategy:  risk.StrategyPerpPerp,
			ValueUSDT: value,
			SlotCost:  1,
			Perp:      req,
		}
		keyToChoice[key] = choice
		groups[req.Opp.Symbol] = append(groups[req.Opp.Symbol], searchChoice{
			Key:       key,
			GroupKey:  req.Opp.Symbol,
			ValueUSDT: value,
			SlotCost:  1,
		})
	}

	// Spot: one choice per built plan.
	for idx, plan := range spot {
		if plan == nil {
			continue
		}
		if occ != nil {
			if _, seen := occ.ActiveSymbols[plan.Candidate.Symbol]; seen {
				continue
			}
		}
		breakdown := scoreSpotEntry(plan, e.cfg)
		if breakdown.NetValueUSDT <= 0 {
			continue
		}
		key := fmt.Sprintf("spot:%s:%s:%s:%d",
			plan.Candidate.Symbol, plan.Candidate.Exchange, plan.Candidate.Direction, idx)
		choice := &unifiedEntryChoice{
			Key:       key,
			Symbol:    plan.Candidate.Symbol,
			Strategy:  risk.StrategySpotFutures,
			ValueUSDT: breakdown.NetValueUSDT,
			SlotCost:  1,
			Spot:      plan,
		}
		keyToChoice[key] = choice
		groups[plan.Candidate.Symbol] = append(groups[plan.Candidate.Symbol], searchChoice{
			Key:       key,
			GroupKey:  plan.Candidate.Symbol,
			ValueUSDT: breakdown.NetValueUSDT,
			SlotCost:  1,
		})
	}

	return groups, keyToChoice
}

// makeUnifiedEvaluator returns the evaluate callback solveGroupedSearch
// calls many times during B&B. The callback:
//   - rejects sets exceeding the combined MaxPositions slot cap
//     (implicit: solveGroupedSearch enforces maxSlots itself)
//   - rejects sets whose spot subset exceeds SpotSlotsRemaining (the
//     spot-only sub-cap)
//   - rejects sets whose cumulative exposures would breach the allocator's
//     total / per-strategy / per-exchange caps. This is the same cumulative
//     accounting ReserveBatch applies at prehold time — gating inside
//     evaluate lets the B&B prefer a slightly lower-value subset that
//     actually survives ReserveBatch, instead of being forced to the whole
//     batch via the all-or-nothing atomicity contract.
//   - computes the cumulative ValueUSDT sum as the objective
//
// The callback is side-effect free — solveGroupedSearch calls it from
// both greedy seeding and B&B leaves.
func (e *Engine) makeUnifiedEvaluator(
	keyToChoice map[string]*unifiedEntryChoice,
	occ *unifiedOccupancy,
	snap *unifiedCapacitySnapshot,
) func(keys []string) (float64, bool) {
	spotCap := 0
	if occ != nil {
		spotCap = occ.SpotSlotsRemaining
	}
	exposuresOf := func(c *unifiedEntryChoice) map[string]float64 {
		exposures, _ := e.exposuresForChoice(c)
		return exposures
	}
	return func(keys []string) (float64, bool) {
		var score float64
		spotUsed := 0
		for _, k := range keys {
			c, ok := keyToChoice[k]
			if !ok || c == nil {
				return 0, false
			}
			if c.Strategy == risk.StrategySpotFutures {
				spotUsed++
				if spotUsed > spotCap {
					return 0, false
				}
			}
			score += c.ValueUSDT
		}
		// Cumulative allocator-cap feasibility. Skipped (permissive) when
		// the snapshot is nil, which keeps older test paths and the
		// allocator-disabled branch working unchanged.
		if snap != nil {
			if !unifiedEvaluatorFeasible(snap, keyToChoice, exposuresOf, keys) {
				return 0, false
			}
		}
		return score, true
	}
}

// dispatchUnifiedPerp dispatches a perp winner using a preheld
// reservation. On success the reservation is committed via the
// allocator.Commit path; on failure the reservation is left intact
// so the batch-level ReleaseBatch can reclaim it.
//
// The dispatch path mirrors executeArbitrage's Phase 2 goroutine body
// so partial-entry accounting, reconcile fallback and position save
// semantics stay identical.
func (e *Engine) dispatchUnifiedPerp(choice *unifiedEntryChoice, res *risk.CapitalReservation) error {
	if choice == nil || choice.Perp == nil || choice.Perp.Approval == nil {
		return errors.New("perp dispatch: nil choice/approval")
	}
	approval := choice.Perp.Approval
	opp := choice.Perp.Opp

	// Acquire per-symbol execute lock — mirrors executeArbitrage's per
	// candidate lock acquisition. 5-minute TTL matches the existing path.
	lockResource := fmt.Sprintf("execute:%s", opp.Symbol)
	symLock, acquired, err := e.db.AcquireOwnedLock(lockResource, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	if !acquired {
		return fmt.Errorf("execute lock busy for %s", opp.Symbol)
	}
	defer symLock.Release()

	if e.cfg.DryRun {
		e.log.Info("[DRY RUN] unified perp: would execute %s size=%.6f price=%.5f",
			opp.Symbol, approval.Size, approval.Price)
		return nil
	}

	// Persist pending position BEFORE dispatch so consolidator + dashboard see it.
	pendingPos := e.createPendingPosition(opp)
	if err := e.db.SavePosition(pendingPos); err != nil {
		return fmt.Errorf("save pending: %w", err)
	}
	if e.api != nil {
		e.api.BroadcastPositionUpdate(pendingPos)
	}

	err = e.executeTradeV2WithPos(opp, pendingPos, approval.Size, approval.Price, approval.GapBPS)
	if errors.Is(err, errPartialEntry) {
		if cErr := e.commitExistingReservation(res, pendingPos.ID, 0); cErr != nil {
			e.log.Error("unified perp: capital commit for partial %s: %v", opp.Symbol, cErr)
			if e.allocator != nil && e.allocator.Enabled() {
				_ = e.allocator.Reconcile()
			}
		}
		return nil
	}
	if err != nil {
		// Release the reservation inline so the batch's eventual
		// ReleaseBatch doesn't have to touch an already-released key
		// (ReleaseBatch skips missing keys safely either way).
		e.releasePerpReservation(res)
		e.cleanupFailedPosition(opp.Symbol, err.Error())
		return fmt.Errorf("trade execution: %w", err)
	}
	if cErr := e.commitExistingReservation(res, pendingPos.ID, 0); cErr != nil {
		e.log.Error("unified perp: capital commit failed for %s: %v", opp.Symbol, cErr)
		if e.allocator != nil && e.allocator.Enabled() {
			_ = e.allocator.Reconcile()
		}
	}
	return nil
}

// dispatchUnifiedSpot forwards a spot winner to the spot engine via
// OpenSelectedEntry, passing the preheld reservation so the spot
// engine's capital path commits against it instead of issuing a new
// reservation.
func (e *Engine) dispatchUnifiedSpot(choice *unifiedEntryChoice, res *risk.CapitalReservation) error {
	if choice == nil || choice.Spot == nil {
		return errors.New("spot dispatch: nil choice/plan")
	}
	if e.spotEntry == nil {
		return errors.New("spot dispatch: executor not installed")
	}
	return e.spotEntry.OpenSelectedEntry(choice.Spot, 0, res)
}
