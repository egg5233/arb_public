package engine

import (
	"sync"
	"sync/atomic"
	"testing"

	"arb/internal/models"
	"arb/pkg/utils"
)

// consume_overrides_test.go — exercises consumeOverridesAndEnrichOpps,
// the single shared helper that consumes e.allocOverrides under
// allocOverrideMu exactly once and dispatches to one of three branches
// (tier-2 patch, RescanSymbols fallback, or no-op). Tests cover each
// branch plus the concurrency guarantee per
// plans/PLAN-unified-capital-allocator.md Section 11.

// stubRescanner implements symbolRescanner for tests. It records the
// SymbolPair slice it received and returns a caller-supplied result.
type stubRescanner struct {
	mu      sync.Mutex
	calls   int
	lastIn  []models.SymbolPair
	resultF func([]models.SymbolPair) []models.Opportunity
}

func (s *stubRescanner) RescanSymbols(pairs []models.SymbolPair) []models.Opportunity {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.lastIn = append([]models.SymbolPair(nil), pairs...)
	if s.resultF != nil {
		return s.resultF(pairs)
	}
	return nil
}

// newTestEngineForConsumeOverrides constructs a minimal Engine with
// only the fields consumeOverridesAndEnrichOpps + its helpers touch:
// the override map + mutex, the rescanner override hook, and a logger.
// No DB, no allocator, no discovery scanner is attached.
func newTestEngineForConsumeOverrides(t *testing.T) *Engine {
	t.Helper()
	return &Engine{
		log: utils.NewLogger("test"),
	}
}

// TestConsumeOverridesAndEnrichOpps_NonEmptyOppsAppliesTier2Patch —
// With non-empty opps AND non-empty overrides whose primary pair matches
// an opp, the helper must return the tier-2 patch result with
// tier="tier-2-override-patch".
func TestConsumeOverridesAndEnrichOpps_NonEmptyOppsAppliesTier2Patch(t *testing.T) {
	e := newTestEngineForConsumeOverrides(t)
	e.allocOverrides = map[string]allocatorChoice{
		"BTCUSDT": {
			symbol:        "BTCUSDT",
			longExchange:  "binance",
			shortExchange: "bybit",
		},
	}
	perpOpps := []models.Opportunity{
		{
			Symbol:        "BTCUSDT",
			LongExchange:  "binance",
			ShortExchange: "bybit",
			Spread:        5.0,
		},
		{
			Symbol:        "ETHUSDT",
			LongExchange:  "gateio",
			ShortExchange: "okx",
			Spread:        4.0,
		},
	}

	got, tier := e.consumeOverridesAndEnrichOpps(perpOpps)

	if tier != "tier-2-override-patch" {
		t.Fatalf("tier: want tier-2-override-patch, got %q", tier)
	}
	if len(got) != 1 {
		t.Fatalf("expected only BTCUSDT to survive tier-2 filter, got %d opps: %+v", len(got), got)
	}
	if got[0].Symbol != "BTCUSDT" {
		t.Fatalf("expected surviving opp to be BTCUSDT, got %q", got[0].Symbol)
	}
	if e.allocOverrides != nil {
		t.Fatalf("allocOverrides must be cleared after consume; got %+v", e.allocOverrides)
	}
}

// TestConsumeOverridesAndEnrichOpps_EmptyOppsCallsRescanSymbols —
// With empty opps AND non-empty overrides the helper must call
// rescanner.RescanSymbols with a SymbolPair list derived from the
// overrides and return the rescan's output with the correct tier tag.
func TestConsumeOverridesAndEnrichOpps_EmptyOppsCallsRescanSymbols(t *testing.T) {
	e := newTestEngineForConsumeOverrides(t)
	e.allocOverrides = map[string]allocatorChoice{
		"BTCUSDT": {
			symbol:        "BTCUSDT",
			longExchange:  "binance",
			shortExchange: "bybit",
		},
		"ETHUSDT": {
			symbol:        "ETHUSDT",
			longExchange:  "gateio",
			shortExchange: "okx",
		},
	}
	stub := &stubRescanner{
		resultF: func(pairs []models.SymbolPair) []models.Opportunity {
			out := make([]models.Opportunity, 0, len(pairs))
			for _, p := range pairs {
				out = append(out, models.Opportunity{
					Symbol:        p.Symbol,
					LongExchange:  p.LongExchange,
					ShortExchange: p.ShortExchange,
				})
			}
			return out
		},
	}
	e.rescannerOverride = stub

	got, tier := e.consumeOverridesAndEnrichOpps(nil)

	if tier != "tier-2-override-fallback" {
		t.Fatalf("tier: want tier-2-override-fallback, got %q", tier)
	}
	if stub.calls != 1 {
		t.Fatalf("RescanSymbols call count: want 1, got %d", stub.calls)
	}
	if len(stub.lastIn) != 2 {
		t.Fatalf("RescanSymbols should receive 2 SymbolPair entries, got %d", len(stub.lastIn))
	}
	// Each override must appear as a SymbolPair with matching exchanges.
	foundBTC := false
	foundETH := false
	for _, p := range stub.lastIn {
		if p.Symbol == "BTCUSDT" && p.LongExchange == "binance" && p.ShortExchange == "bybit" {
			foundBTC = true
		}
		if p.Symbol == "ETHUSDT" && p.LongExchange == "gateio" && p.ShortExchange == "okx" {
			foundETH = true
		}
	}
	if !foundBTC || !foundETH {
		t.Fatalf("SymbolPair list missing expected entries: %+v", stub.lastIn)
	}
	if len(got) != 2 {
		t.Fatalf("fallbackOpps should have 2 entries, got %d", len(got))
	}
	if e.allocOverrides != nil {
		t.Fatalf("allocOverrides must be cleared after fallback consume; got %+v", e.allocOverrides)
	}
}

// TestConsumeOverridesAndEnrichOpps_EmptyOverridesReturnsOppsUnchanged —
// With an empty overrides map the helper must short-circuit: return the
// same opps slice unchanged with tier="none" and never consult the
// rescanner.
func TestConsumeOverridesAndEnrichOpps_EmptyOverridesReturnsOppsUnchanged(t *testing.T) {
	e := newTestEngineForConsumeOverrides(t)
	e.allocOverrides = nil
	stub := &stubRescanner{}
	e.rescannerOverride = stub

	perpOpps := []models.Opportunity{
		{Symbol: "BTCUSDT", LongExchange: "binance", ShortExchange: "bybit"},
	}

	got, tier := e.consumeOverridesAndEnrichOpps(perpOpps)

	if tier != "none" {
		t.Fatalf("tier: want none, got %q", tier)
	}
	if len(got) != 1 || got[0].Symbol != "BTCUSDT" {
		t.Fatalf("expected perpOpps returned unchanged, got %+v", got)
	}
	if stub.calls != 0 {
		t.Fatalf("rescanner should not be called when overrides empty; got %d calls", stub.calls)
	}
}

// TestConsumeOverridesAndEnrichOpps_AcquiresAllocOverrideMuOnceAndClears
// — concurrent callers must observe consume-once semantics: exactly one
// caller sees the overrides, every subsequent caller sees an empty map.
//
// Setup: pre-populate overrides, launch N concurrent consumers. Count
// how many callers observed non-"none" behavior — must be exactly 1.
// Also assert the map is nil afterwards.
func TestConsumeOverridesAndEnrichOpps_AcquiresAllocOverrideMuOnceAndClears(t *testing.T) {
	e := newTestEngineForConsumeOverrides(t)
	e.allocOverrides = map[string]allocatorChoice{
		"BTCUSDT": {symbol: "BTCUSDT", longExchange: "binance", shortExchange: "bybit"},
	}
	stub := &stubRescanner{
		resultF: func(pairs []models.SymbolPair) []models.Opportunity {
			return []models.Opportunity{{Symbol: "BTCUSDT"}}
		},
	}
	e.rescannerOverride = stub

	const N = 20
	var nonNoneTiers int32
	var rescanCalls int32
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			_, tier := e.consumeOverridesAndEnrichOpps(nil)
			if tier != "none" {
				atomic.AddInt32(&nonNoneTiers, 1)
			}
		}()
	}
	wg.Wait()
	rescanCalls = int32(stub.calls)

	if got := atomic.LoadInt32(&nonNoneTiers); got != 1 {
		t.Fatalf("consume-once violated: expected exactly 1 non-none tier from %d concurrent callers, got %d", N, got)
	}
	if rescanCalls != 1 {
		t.Fatalf("RescanSymbols should be called once under consume-once; got %d", rescanCalls)
	}
	if e.allocOverrides != nil {
		t.Fatalf("allocOverrides must be nil after consume; got %+v", e.allocOverrides)
	}
}
