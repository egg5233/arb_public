package engine

import (
	"sort"
	"time"
)

// selection_core.go — generic grouped branch-and-bound search.
//
// This file hosts the pure combinatorial core used by the rebalance
// allocator (internal/engine/allocator.go) and the future cross-strategy
// entry selector (unified_entry.go, introduced later in the plan).
//
// Design (per plans/PLAN-unified-capital-allocator.md Section 4):
//   - searchChoice is the abstract unit fed to the solver (a stable key,
//     a mutual-exclusion group, its USDT value, and its slot cost).
//   - solveGroupedSearch picks the subset of choices that maximizes the
//     sum of ValueUSDT subject to:
//       * cumulative SlotCost <= maxSlots
//       * at most one choice per GroupKey
//       * evaluate(keys) returning feasible=true at the leaf
//   - Pure search: no allocator, transfer, reservation, or other side
//     effects. All domain-specific feasibility lives in the evaluate
//     callback supplied by the caller.
//
// Both the rebalance-specific greedy seed (formerly
// greedyAllocatorSeed) and the branch-and-bound walk (formerly the
// branch closure inside solveAllocator) live here, parameterized over
// the caller's evaluate function so the exact same traversal pattern
// serves rebalance today and cross-strategy entry later.

// searchChoice is the abstract unit fed to solveGroupedSearch.
type searchChoice struct {
	Key       string  // stable unique id (used in evaluate + result)
	GroupKey  string  // symbol — mutual exclusion group
	ValueUSDT float64 // objective; higher is better
	SlotCost  int     // positions consumed (typically 1 per entry)
}

// searchGroup is the internal grouped-and-sorted form of the input map.
// GroupKey order is the order groups are walked by the B&B (stable by
// best baseValue, matching the existing allocator convention where
// buildAllocatorCandidates pre-sorts by choices[0].baseValue desc).
type searchGroup struct {
	Key     string
	Choices []searchChoice
}

// sortedSearchGroups converts the input map into a deterministically
// ordered slice: groups sorted by best (index 0) choice ValueUSDT desc,
// and each group's choices sorted by ValueUSDT desc with SlotCost asc
// as tie-breaker (mirrors the pre-refactor ordering in
// buildAllocatorCandidates/solveAllocator).
func sortedSearchGroups(groups map[string][]searchChoice) []searchGroup {
	out := make([]searchGroup, 0, len(groups))
	for key, choices := range groups {
		if len(choices) == 0 {
			continue
		}
		sorted := append([]searchChoice(nil), choices...)
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].ValueUSDT == sorted[j].ValueUSDT {
				return sorted[i].SlotCost < sorted[j].SlotCost
			}
			return sorted[i].ValueUSDT > sorted[j].ValueUSDT
		})
		out = append(out, searchGroup{Key: key, Choices: sorted})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Choices[0].ValueUSDT > out[j].Choices[0].ValueUSDT
	})
	return out
}

// searchUpperBound returns an optimistic ceiling on remaining value:
// take the best choice per remaining group while slot-cost permits.
// Mirrors allocatorUpperBound from pre-refactor.
func searchUpperBound(groupsSorted []searchGroup, idx, slotsRemaining int) float64 {
	sum := 0.0
	for ; idx < len(groupsSorted) && slotsRemaining > 0; idx++ {
		best := groupsSorted[idx].Choices[0]
		if best.SlotCost > slotsRemaining || best.ValueUSDT <= 0 {
			continue
		}
		sum += best.ValueUSDT
		slotsRemaining -= best.SlotCost
	}
	return sum
}

// greedySeed walks groups in order and picks the first feasible choice
// per group (feasibility decided by evaluate). Matches the live
// greedyAllocatorSeed traversal pattern: for each group, try choices
// best-first; on the first dryRun-feasible pick, commit and move on.
//
// Returns the selected keys and their cumulative evaluate() score. A
// second invocation of evaluate() over the final key set gives the
// authoritative score, but callers can use the returned value as an
// initial bound for B&B pruning.
func greedySeed(
	groupsSorted []searchGroup,
	maxSlots int,
	evaluate func(keys []string) (float64, bool),
) (keys []string, score float64, ok bool) {
	remaining := maxSlots
	selected := make([]string, 0, len(groupsSorted))
	for _, g := range groupsSorted {
		if remaining <= 0 {
			break
		}
		for _, c := range g.Choices {
			if c.SlotCost > remaining {
				continue
			}
			trial := append(append([]string(nil), selected...), c.Key)
			if _, feasible := evaluate(trial); !feasible {
				continue
			}
			selected = append(selected, c.Key)
			remaining -= c.SlotCost
			break
		}
	}
	if len(selected) == 0 {
		return nil, 0, false
	}
	s, feasible := evaluate(selected)
	if !feasible {
		return nil, 0, false
	}
	return selected, s, true
}

// solveGroupedSearch returns the subset of keys maximizing total
// ValueUSDT subject to maxSlots and at most one choice per GroupKey.
//
// The evaluate callback is called many times during branch exploration
// and must be side-effect free. It receives the current candidate set
// of keys and must return (score, feasible). score is the USDT value
// of the set under whatever accounting the caller uses (e.g., rebalance
// uses sum(baseValue) - dryRun fees).
//
// Algorithm:
//  1. Sort groups + choices deterministically.
//  2. Run greedySeed to produce an incumbent score as initial bound.
//  3. Run depth-first B&B with searchUpperBound pruning.
//  4. If elapsed >= timeout before B&B finishes, return the best
//     feasible set seen so far (falls back to greedy when nothing
//     better has been discovered).
//
// The returned slice is a subset of input choice Keys; caller
// reconstructs domain objects via its own key -> object map.
func solveGroupedSearch(
	groups map[string][]searchChoice,
	maxSlots int,
	timeout time.Duration,
	evaluate func(keys []string) (scoreUSDT float64, feasible bool),
) []string {
	if maxSlots <= 0 || len(groups) == 0 || evaluate == nil {
		return nil
	}
	groupsSorted := sortedSearchGroups(groups)
	if len(groupsSorted) == 0 {
		return nil
	}

	start := time.Now()

	bestScore := -1.0
	var bestKeys []string

	if seedKeys, seedScore, ok := greedySeed(groupsSorted, maxSlots, evaluate); ok {
		bestScore = seedScore
		bestKeys = append([]string(nil), seedKeys...)
	}

	var branch func(idx, slotsRemaining int, current []string, currentValue float64)
	branch = func(idx, slotsRemaining int, current []string, currentValue float64) {
		if time.Since(start) >= timeout {
			return
		}
		if idx >= len(groupsSorted) || slotsRemaining <= 0 {
			score, feasible := evaluate(current)
			if !feasible {
				return
			}
			if score > bestScore {
				bestScore = score
				bestKeys = append([]string(nil), current...)
			}
			return
		}

		ub := searchUpperBound(groupsSorted, idx, slotsRemaining)
		if currentValue+ub <= bestScore {
			return
		}

		// Try each choice within the group (mutual exclusion).
		for _, choice := range groupsSorted[idx].Choices {
			if choice.SlotCost > slotsRemaining {
				continue
			}
			next := append(current, choice.Key)
			branch(idx+1, slotsRemaining-choice.SlotCost, next, currentValue+choice.ValueUSDT)
		}
		// Skip this group entirely.
		branch(idx+1, slotsRemaining, current, currentValue)
	}

	branch(0, maxSlots, nil, 0)
	return bestKeys
}
