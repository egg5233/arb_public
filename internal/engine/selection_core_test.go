package engine

import (
	"sort"
	"testing"
	"time"
)

// selection_core_test.go — unit tests for the generic grouped B&B
// search (selection_core.go). Tests cover slot-cap, mutual exclusion,
// feasibility rejection, and timeout fallback per
// plans/PLAN-unified-capital-allocator.md Section 11.

func sortedKeys(keys []string) []string {
	out := append([]string(nil), keys...)
	sort.Strings(out)
	return out
}

// TestSolveGroupedSearch_PicksBestSubsetUnderSlotCap verifies that the
// solver picks the highest-value subset when slot capacity is the only
// binding constraint.
//
// Setup: 3 disjoint-group candidates with values 100, 80, 50 under a
// slot cap of 2. Expected picks: values 100 + 80 = 180, dropping the
// lowest 50.
func TestSolveGroupedSearch_PicksBestSubsetUnderSlotCap(t *testing.T) {
	groups := map[string][]searchChoice{
		"A": {{Key: "a1", GroupKey: "A", ValueUSDT: 100, SlotCost: 1}},
		"B": {{Key: "b1", GroupKey: "B", ValueUSDT: 80, SlotCost: 1}},
		"C": {{Key: "c1", GroupKey: "C", ValueUSDT: 50, SlotCost: 1}},
	}
	evaluate := func(keys []string) (float64, bool) {
		sum := 0.0
		for _, k := range keys {
			switch k {
			case "a1":
				sum += 100
			case "b1":
				sum += 80
			case "c1":
				sum += 50
			}
		}
		return sum, true
	}
	keys := solveGroupedSearch(groups, 2, 100*time.Millisecond, evaluate)
	got := sortedKeys(keys)
	want := []string{"a1", "b1"}
	if len(got) != len(want) {
		t.Fatalf("want %d keys, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("want %v, got %v", want, got)
		}
	}
}

// TestSolveGroupedSearch_OneChoicePerGroup verifies mutual exclusion:
// multiple high-value candidates in the same group must produce at
// most one winner from that group.
//
// Setup: group "A" contains 3 alternatives (value 100, 90, 80); group
// "B" has one choice (value 70). Slot cap 3. Even with room for 3,
// result must be exactly two keys — one from A, one from B.
func TestSolveGroupedSearch_OneChoicePerGroup(t *testing.T) {
	groups := map[string][]searchChoice{
		"A": {
			{Key: "a1", GroupKey: "A", ValueUSDT: 100, SlotCost: 1},
			{Key: "a2", GroupKey: "A", ValueUSDT: 90, SlotCost: 1},
			{Key: "a3", GroupKey: "A", ValueUSDT: 80, SlotCost: 1},
		},
		"B": {{Key: "b1", GroupKey: "B", ValueUSDT: 70, SlotCost: 1}},
	}
	values := map[string]float64{"a1": 100, "a2": 90, "a3": 80, "b1": 70}
	evaluate := func(keys []string) (float64, bool) {
		sum := 0.0
		for _, k := range keys {
			sum += values[k]
		}
		return sum, true
	}
	keys := solveGroupedSearch(groups, 3, 100*time.Millisecond, evaluate)

	// Count wins per group.
	wins := map[string]int{}
	for _, k := range keys {
		wins[k[:1]]++
	}
	if wins["a"] > 1 {
		t.Fatalf("group A must have <=1 winner, got %d: %v", wins["a"], keys)
	}
	if wins["b"] > 1 {
		t.Fatalf("group B must have <=1 winner, got %d: %v", wins["b"], keys)
	}
	// Best choice from A is a1 (value 100), plus b1: total 170.
	got := sortedKeys(keys)
	want := []string{"a1", "b1"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("want %v (best a + b), got %v", want, got)
	}
}

// TestSolveGroupedSearch_EvaluateFeasibilityRejection verifies that a
// candidate set for which evaluate returns feasible=false is excluded.
//
// Setup: two groups. evaluate declares any set containing "a_bad"
// infeasible. The solver must pick "a_good" instead.
func TestSolveGroupedSearch_EvaluateFeasibilityRejection(t *testing.T) {
	groups := map[string][]searchChoice{
		"A": {
			{Key: "a_bad", GroupKey: "A", ValueUSDT: 200, SlotCost: 1},
			{Key: "a_good", GroupKey: "A", ValueUSDT: 100, SlotCost: 1},
		},
		"B": {{Key: "b1", GroupKey: "B", ValueUSDT: 50, SlotCost: 1}},
	}
	values := map[string]float64{"a_bad": 200, "a_good": 100, "b1": 50}
	evaluate := func(keys []string) (float64, bool) {
		for _, k := range keys {
			if k == "a_bad" {
				return 0, false
			}
		}
		sum := 0.0
		for _, k := range keys {
			sum += values[k]
		}
		return sum, true
	}
	keys := solveGroupedSearch(groups, 2, 100*time.Millisecond, evaluate)
	got := sortedKeys(keys)
	want := []string{"a_good", "b1"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("want %v (a_bad must be rejected), got %v", want, got)
	}
}

// TestSolveGroupedSearch_TimeoutFallsBackToGreedy verifies that when
// the B&B phase cannot finish within the timeout budget, the solver
// returns the greedy-seed result (or the best-so-far found by B&B
// before the timer fired).
//
// Setup: construct many groups so B&B would take measurable time, and
// use a near-zero timeout to force greedy-only. The greedy seed still
// runs to completion since it is outside the timed B&B loop.
func TestSolveGroupedSearch_TimeoutFallsBackToGreedy(t *testing.T) {
	const groupCount = 200
	groups := make(map[string][]searchChoice, groupCount)
	values := make(map[string]float64, groupCount*2)
	for i := 0; i < groupCount; i++ {
		g := "G" + itoaTest(i)
		k1 := g + "-a"
		k2 := g + "-b"
		v1 := 100.0 - float64(i)*0.1 // descending so greedy picks best
		v2 := v1 - 0.5
		values[k1] = v1
		values[k2] = v2
		groups[g] = []searchChoice{
			{Key: k1, GroupKey: g, ValueUSDT: v1, SlotCost: 1},
			{Key: k2, GroupKey: g, ValueUSDT: v2, SlotCost: 1},
		}
	}
	evaluate := func(keys []string) (float64, bool) {
		// Simulate a small amount of work so B&B actually takes time.
		sum := 0.0
		for _, k := range keys {
			sum += values[k]
		}
		return sum, true
	}
	// 1 nanosecond timeout — B&B cannot complete; expect greedy result.
	keys := solveGroupedSearch(groups, 10, 1*time.Nanosecond, evaluate)
	if len(keys) == 0 {
		t.Fatalf("expected greedy to produce some result despite B&B timeout, got 0 keys")
	}
	// Greedy picks best choice per group, up to slot cap. With 10 slots
	// and groups sorted by best-value desc, greedy picks the top 10
	// groups' best choices.
	if len(keys) != 10 {
		t.Fatalf("greedy should fill 10 slots, got %d", len(keys))
	}
	// Ensure every returned key is a top choice ("-a") of some group.
	for _, k := range keys {
		if len(k) < 2 || k[len(k)-2:] != "-a" {
			t.Fatalf("greedy should pick best (-a) per group, got %q", k)
		}
	}
}

// itoaTest is a tiny inline helper to avoid importing strconv only for
// this file. Works for small nonneg ints used in the timeout test.
func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	var b [6]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
