package engine

import "testing"

// Step 2 of PLAN-allocator-unified.md added per-leg margin fields to
// allocatorChoice. These helpers must:
//   (a) fall back to symmetric requiredMargin when per-leg fields are zero,
//       so legacy call sites keep working unchanged;
//   (b) return the per-leg values when both are populated;
//   (c) maxRequiredMargin reports the larger of the two per-leg values
//       (or the symmetric fallback).
func TestAllocatorChoiceMarginNeeds(t *testing.T) {
	// (a) symmetric fallback when per-leg fields zero
	c := allocatorChoice{requiredMargin: 100}
	l, s := c.marginNeeds()
	if l != 100 || s != 100 {
		t.Fatalf("symmetric fallback: want (100,100), got (%.2f,%.2f)", l, s)
	}
	if m := c.maxRequiredMargin(); m != 100 {
		t.Fatalf("symmetric maxRequiredMargin: want 100, got %.2f", m)
	}

	// (b) per-leg values when both populated — should prefer per-leg over
	// legacy requiredMargin even when they differ.
	c2 := allocatorChoice{
		requiredMargin:      150,
		longRequiredMargin:  120,
		shortRequiredMargin: 180,
	}
	l2, s2 := c2.marginNeeds()
	if l2 != 120 || s2 != 180 {
		t.Fatalf("per-leg values: want (120,180), got (%.2f,%.2f)", l2, s2)
	}
	if m := c2.maxRequiredMargin(); m != 180 {
		t.Fatalf("per-leg maxRequiredMargin: want 180 (max), got %.2f", m)
	}

	// (c) only one per-leg populated → treated as not-populated, falls back
	// to symmetric. This matches the helper's guard (`both > 0`).
	c3 := allocatorChoice{requiredMargin: 50, longRequiredMargin: 30}
	l3, s3 := c3.marginNeeds()
	if l3 != 50 || s3 != 50 {
		t.Fatalf("partial per-leg fallback: want (50,50), got (%.2f,%.2f)", l3, s3)
	}
}
