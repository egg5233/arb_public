package engine

import "testing"

func TestReconcileAppliedPnLUsesFreshStoredValues(t *testing.T) {
	reconciled, diff, needsUpdate := reconcileAppliedPnL(8, -1, 3, 9)
	if reconciled != 10 {
		t.Fatalf("reconciled = %.2f, want 10.00", reconciled)
	}
	if diff != 1 {
		t.Fatalf("diff = %.2f, want 1.00 based on fresh stored RealizedPnL", diff)
	}
	if !needsUpdate {
		t.Fatal("needsUpdate = false, want true")
	}
}

func TestReconcileAppliedPnLNoStatsAdjustmentWhenFreshAlreadyMatches(t *testing.T) {
	reconciled, diff, needsUpdate := reconcileAppliedPnL(8, -1, 3, 10)
	if reconciled != 10 {
		t.Fatalf("reconciled = %.2f, want 10.00", reconciled)
	}
	if diff != 0 {
		t.Fatalf("diff = %.2f, want 0.00", diff)
	}
	if needsUpdate {
		t.Fatal("needsUpdate = true, want false")
	}
}
