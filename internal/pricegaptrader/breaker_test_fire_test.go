package pricegaptrader

import "testing"

// Wave 0 stubs for the synthetic test-fire path. Plan 15-04 implements.

// TestSyntheticFireFullCycle — success-criterion #5: synthetic test fire
// exercises full trip cycle (sticky set, candidates paused, alerts) without
// engine restart and without real PnL drawdown.
func TestSyntheticFireFullCycle(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-04 implements synthetic test fire")
}

// TestSyntheticFireDryRun_NoMutations — D-14 + Open Question #4: dry-run
// computes would-trip + 24h PnL but skips all Redis/Telegram/WS mutations.
func TestSyntheticFireDryRun_NoMutations(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-04 implements --dry-run mode")
}

// TestBreaker_RecoveryReenablesCandidatesAndClearsSticky — D-11: operator
// recovery clears paused_by_breaker on all candidates AND zeroes
// PaperModeStickyUntil. Operator-set disabled=true candidates stay disabled.
func TestBreaker_RecoveryReenablesCandidatesAndClearsSticky(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-04 implements recovery flow")
}
