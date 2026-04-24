// Package pricegaptrader — unit tests for the rate-limited non-fire reason
// logger introduced by Phase 9 gap-closure Gap #1 (Plan 09-09).
//
// Scope: exercise Tracker.shouldLogNoFire in isolation (synthetic clock, no
// wall-clock dependency) and drive runTick end-to-end with the flag OFF/ON
// to prove the OFF-silence contract and the ON-rate-limit contract.
package pricegaptrader

import (
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// newDebugLogTracker builds a minimal Tracker sufficient for shouldLogNoFire
// unit tests. No exchanges, no db, no broadcaster — only the flag + the map.
func newDebugLogTracker(t *testing.T, flag bool) *Tracker {
	t.Helper()
	return &Tracker{
		cfg:              &config.Config{PriceGapDebugLog: flag},
		log:              utils.NewLogger("pg-tracker-test"),
		lastNoFireLogged: make(map[string]time.Time),
	}
}

// Test 1: empty reason is defensively rejected and does not update state.
func TestShouldLogNoFire_EmptyReasonRejected(t *testing.T) {
	tr := newDebugLogTracker(t, true)
	now := time.Unix(1_700_000_000, 0)

	if tr.shouldLogNoFire("SOONUSDT", "", now) {
		t.Fatalf("expected shouldLogNoFire to reject empty reason, got true")
	}
	if len(tr.lastNoFireLogged) != 0 {
		t.Fatalf("expected empty map after empty-reason call, got %d entries", len(tr.lastNoFireLogged))
	}
}

// Test 2: first (symbol, reason) call emits — returns true and records ts.
func TestShouldLogNoFire_FirstCallEmits(t *testing.T) {
	tr := newDebugLogTracker(t, true)
	now := time.Unix(1_700_000_000, 0)

	if !tr.shouldLogNoFire("SOONUSDT", "stale_bbo", now) {
		t.Fatalf("expected first (SOONUSDT, stale_bbo) call to return true")
	}
	if _, ok := tr.lastNoFireLogged["SOONUSDT|stale_bbo"]; !ok {
		t.Fatalf("expected map to record SOONUSDT|stale_bbo entry")
	}
}

// Test 3: repeat within cooldown is suppressed.
func TestShouldLogNoFire_WithinCooldownSuppressed(t *testing.T) {
	tr := newDebugLogTracker(t, true)
	base := time.Unix(1_700_000_000, 0)

	if !tr.shouldLogNoFire("SOONUSDT", "stale_bbo", base) {
		t.Fatalf("expected first call true")
	}
	if tr.shouldLogNoFire("SOONUSDT", "stale_bbo", base.Add(30*time.Second)) {
		t.Fatalf("expected repeat at t+30s suppressed, got true")
	}
	if tr.shouldLogNoFire("SOONUSDT", "stale_bbo", base.Add(59*time.Second)) {
		t.Fatalf("expected repeat at t+59s suppressed, got true")
	}
}

// Test 4: after cooldown, next call emits (bucket refill).
func TestShouldLogNoFire_AfterCooldownEmits(t *testing.T) {
	tr := newDebugLogTracker(t, true)
	base := time.Unix(1_700_000_000, 0)

	if !tr.shouldLogNoFire("SOONUSDT", "stale_bbo", base) {
		t.Fatalf("expected first call true")
	}
	// At exactly the cooldown boundary (strict-less-than means 60s emits).
	if !tr.shouldLogNoFire("SOONUSDT", "stale_bbo", base.Add(priceGapNoFireLogCooldown)) {
		t.Fatalf("expected call at cooldown boundary to emit")
	}
	// Double cooldown — must emit.
	if !tr.shouldLogNoFire("SOONUSDT", "stale_bbo", base.Add(2*priceGapNoFireLogCooldown)) {
		t.Fatalf("expected call at 2× cooldown to emit")
	}
}

// Test 5: different reason same symbol — independent.
func TestShouldLogNoFire_DifferentReasonSameSymbol(t *testing.T) {
	tr := newDebugLogTracker(t, true)
	base := time.Unix(1_700_000_000, 0)

	if !tr.shouldLogNoFire("SOONUSDT", "stale_bbo", base) {
		t.Fatalf("expected first reason true")
	}
	if !tr.shouldLogNoFire("SOONUSDT", "insufficient_persistence", base) {
		t.Fatalf("expected second reason independent")
	}
	if !tr.shouldLogNoFire("SOONUSDT", "sample_error: bbo unavailable", base) {
		t.Fatalf("expected third reason independent")
	}
}

// Test 6: different symbol same reason — independent.
func TestShouldLogNoFire_DifferentSymbolSameReason(t *testing.T) {
	tr := newDebugLogTracker(t, true)
	base := time.Unix(1_700_000_000, 0)

	if !tr.shouldLogNoFire("SOONUSDT", "stale_bbo", base) {
		t.Fatalf("expected SOON first")
	}
	if !tr.shouldLogNoFire("RAVEUSDT", "stale_bbo", base) {
		t.Fatalf("expected RAVE independent")
	}
	if !tr.shouldLogNoFire("VICUSDT", "stale_bbo", base) {
		t.Fatalf("expected VIC independent")
	}
}

// Test 7: 100 rapid calls within cooldown → exactly one emit (flood guard).
func TestShouldLogNoFire_FloodSuppressed(t *testing.T) {
	tr := newDebugLogTracker(t, true)
	base := time.Unix(1_700_000_000, 0)

	emitted := 0
	for i := 0; i < 100; i++ {
		// 500ms between calls → 50s span < 60s cooldown.
		if tr.shouldLogNoFire("SOONUSDT", "stale_bbo", base.Add(time.Duration(i)*500*time.Millisecond)) {
			emitted++
		}
	}
	if emitted != 1 {
		t.Fatalf("expected exactly 1 emit across 100 calls inside cooldown, got %d", emitted)
	}
}

// Test 8: runTick with flag OFF must NOT touch lastNoFireLogged.
// Test 9: runTick with flag ON within cooldown emits exactly one map entry.
//
// Harness: one candidate, no BBOs set → detectOnce returns sample_error;
// no active positions in db; gates never reached because runTick short-circuits
// on !det.Fired. With flag OFF → zero map entries. With flag ON → one entry.
func TestRunTick_DebugLog_FlagOffAndOn(t *testing.T) {
	cand := e2eCandidate()

	t.Run("FlagOff_NoMapGrowth", func(t *testing.T) {
		db, _ := newE2EClient(t)
		bin := newStubExchange("binance")
		byb := newStubExchange("bybit")
		exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}

		cfg := e2eCfg([]models.PriceGapCandidate{cand})
		cfg.PriceGapDebugLog = false // explicit OFF
		tr := NewTracker(exch, db, newFakeDelistChecker(), cfg)

		for i := 0; i < 10; i++ {
			tr.runTick(time.Unix(1_700_000_000+int64(i), 0))
		}
		if got := len(tr.lastNoFireLogged); got != 0 {
			t.Fatalf("flag OFF: expected 0 map entries after 10 non-firing ticks, got %d", got)
		}
	})

	t.Run("FlagOn_OneEntryWithinCooldown", func(t *testing.T) {
		db, _ := newE2EClient(t)
		bin := newStubExchange("binance")
		byb := newStubExchange("bybit")
		exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}

		cfg := e2eCfg([]models.PriceGapCandidate{cand})
		cfg.PriceGapDebugLog = true // explicit ON
		tr := NewTracker(exch, db, newFakeDelistChecker(), cfg)

		// 10 ticks, 1s apart → all inside 60s cooldown.
		for i := 0; i < 10; i++ {
			tr.runTick(time.Unix(1_700_000_000+int64(i), 0))
		}
		if got := len(tr.lastNoFireLogged); got != 1 {
			t.Fatalf("flag ON: expected 1 map entry after 10 rapid non-firing ticks, got %d", got)
		}
	})
}
