package engine

import (
	"testing"

	"arb/internal/config"
	"arb/pkg/utils"
)

// newTestEngineForOverrideGuard builds a minimal Engine wired only with the
// fields keepFundedChoices / canReserveAllocatorChoice / reserveAllocatorChoice
// touch (config + logger). It does not start scanners or open any network
// resources.
func newTestEngineForOverrideGuard(t *testing.T) *Engine {
	t.Helper()
	cfg := &config.Config{}
	cfg.MarginL4Threshold = 0.80
	cfg.MarginSafetyMultiplier = 2.0
	return &Engine{
		cfg: cfg,
		log: utils.NewLogger("test"),
	}
}

// TestKeepFundedChoices_AllFunded — test #2 from plan v4: all legs have
// enough futures; every choice must survive with balance reservations
// tracked so later choices still see available capacity.
func TestKeepFundedChoices_AllFunded(t *testing.T) {
	e := newTestEngineForOverrideGuard(t)
	choices := []allocatorChoice{
		{symbol: "A", longExchange: "bitget", shortExchange: "bingx", requiredMargin: 60},
		{symbol: "B", longExchange: "okx", shortExchange: "gateio", requiredMargin: 60},
	}
	post := map[string]rebalanceBalanceInfo{
		"bitget": {futures: 200, futuresTotal: 500},
		"bingx":  {futures: 200, futuresTotal: 500},
		"okx":    {futures: 200, futuresTotal: 500},
		"gateio": {futures: 200, futuresTotal: 500},
	}
	kept := e.keepFundedChoices(choices, post)
	if len(kept) != 2 {
		t.Fatalf("want 2 kept, got %d: %+v", len(kept), kept)
	}
	if _, ok := kept["A"]; !ok {
		t.Error("choice A should be kept")
	}
	if _, ok := kept["B"]; !ok {
		t.Error("choice B should be kept")
	}
}

// TestKeepFundedChoices_UnfundedRecipient — test #3 from plan v4: one
// recipient remained at pre-exec balance (transfer failed / skipped).
// Choices requiring that recipient must be dropped.
func TestKeepFundedChoices_UnfundedRecipient(t *testing.T) {
	e := newTestEngineForOverrideGuard(t)
	choices := []allocatorChoice{
		{symbol: "SIREN", longExchange: "bitget", shortExchange: "bingx", requiredMargin: 60},
		{symbol: "PIPPIN", longExchange: "okx", shortExchange: "gateio", requiredMargin: 60},
	}
	// bitget is the unfunded recipient — not enough futures for the buffered
	// need of 60. PIPPIN's legs both have plenty.
	post := map[string]rebalanceBalanceInfo{
		"bitget": {futures: 50, futuresTotal: 180},
		"bingx":  {futures: 200, futuresTotal: 500},
		"okx":    {futures: 200, futuresTotal: 500},
		"gateio": {futures: 200, futuresTotal: 500},
	}
	kept := e.keepFundedChoices(choices, post)
	if len(kept) != 1 {
		t.Fatalf("want 1 kept, got %d: %+v", len(kept), kept)
	}
	if _, ok := kept["SIREN"]; ok {
		t.Error("SIREN should have been dropped (bitget under-funded)")
	}
	if _, ok := kept["PIPPIN"]; !ok {
		t.Error("PIPPIN should still be kept")
	}
}

// TestKeepFundedChoices_SharedRecipientPartialFunding — test #4 from plan v4:
// two choices share the same recipient. Partial funding covers one but not
// both. First-in-order wins.
func TestKeepFundedChoices_SharedRecipientPartialFunding(t *testing.T) {
	e := newTestEngineForOverrideGuard(t)
	choices := []allocatorChoice{
		{symbol: "FIRST", longExchange: "bitget", shortExchange: "bingx", requiredMargin: 60},
		{symbol: "SECOND", longExchange: "bitget", shortExchange: "okx", requiredMargin: 60},
	}
	// bitget has enough for exactly one reservation. After FIRST reserves
	// 60, bitget.futures=90 and next margin debit (60/MSM=30) pushes the
	// ratio over L4 for SECOND.
	post := map[string]rebalanceBalanceInfo{
		"bitget": {futures: 150, futuresTotal: 500}, // pre-ratio=0.70 ok; after reserving 60 → 90 → 0.82 breach
		"bingx":  {futures: 200, futuresTotal: 500},
		"okx":    {futures: 200, futuresTotal: 500},
	}
	kept := e.keepFundedChoices(choices, post)
	if len(kept) != 1 {
		t.Fatalf("want 1 kept (order-dependent), got %d: %+v", len(kept), kept)
	}
	if _, ok := kept["FIRST"]; !ok {
		t.Error("FIRST should win the shared recipient")
	}
	if _, ok := kept["SECOND"]; ok {
		t.Error("SECOND should have been dropped after FIRST reserved capacity")
	}
}

// TestKeepFundedChoices_LocalReliefKept — regression test #6 from plan v4:
// the crossDeficits-empty path (incident scenario inverted). If local spot->
// futures relief succeeded, PostBalances reflects the updated state and
// overrides should be KEPT.
func TestKeepFundedChoices_LocalReliefKept(t *testing.T) {
	e := newTestEngineForOverrideGuard(t)
	choices := []allocatorChoice{
		{symbol: "LOCAL", longExchange: "bitget", shortExchange: "bingx", requiredMargin: 60},
	}
	// Pre-exec: bitget had futures=50 (under buffered need 60), but local
	// spot->futures relief added enough (simulated — executor would have
	// mutated balances map in place at allocator.go:1150-1155).
	post := map[string]rebalanceBalanceInfo{
		"bitget": {futures: 100, futuresTotal: 200}, // post-relief, ratio=0.50 ok; after margin debit 30 → 70 → 0.65 ok
		"bingx":  {futures: 200, futuresTotal: 500},
	}
	kept := e.keepFundedChoices(choices, post)
	if _, ok := kept["LOCAL"]; !ok {
		t.Error("LOCAL override should survive because post-relief balances cover the buffered need")
	}
}

// TestReserveAllocatorChoice_PerLegMath — regression test #7 from plan v4:
// reserving a choice with requiredMargin=60 must subtract 60 from BOTH legs,
// not 30. This mirrors allocator.go:231-232 / 666-667 semantics.
func TestReserveAllocatorChoice_PerLegMath(t *testing.T) {
	e := newTestEngineForOverrideGuard(t)
	work := map[string]rebalanceBalanceInfo{
		"bitget": {futures: 200, futuresTotal: 500},
		"bingx":  {futures: 200, futuresTotal: 500},
	}
	c := allocatorChoice{symbol: "X", longExchange: "bitget", shortExchange: "bingx", requiredMargin: 60}
	e.reserveAllocatorChoice(work, c)
	if got := work["bitget"].futures; got != 140 {
		t.Errorf("bitget futures: want 140 (200-60), got %v", got)
	}
	if got := work["bingx"].futures; got != 140 {
		t.Errorf("bingx futures: want 140 (200-60), got %v", got)
	}
}

// TestCanReserveAllocatorChoice_MissingExchange — defensive: choice naming an
// exchange not in the working map must return false, not panic.
func TestCanReserveAllocatorChoice_MissingExchange(t *testing.T) {
	e := newTestEngineForOverrideGuard(t)
	work := map[string]rebalanceBalanceInfo{
		"bitget": {futures: 200, futuresTotal: 500},
	}
	c := allocatorChoice{symbol: "X", longExchange: "bitget", shortExchange: "ghost", requiredMargin: 60}
	if e.canReserveAllocatorChoice(work, c) {
		t.Error("should return false when an exchange is missing from work map")
	}
}

// TestPostTradeMarginRatio — shared helper sanity check. The L4 threshold in
// newTestEngineForOverrideGuard is 0.80.
func TestPostTradeMarginRatio(t *testing.T) {
	e := newTestEngineForOverrideGuard(t)
	bal := rebalanceBalanceInfo{futures: 100, futuresTotal: 500}

	// No margin applied: ratio = 1 - 100/500 = 0.80 — breaches L4 (>= threshold).
	ratio, ok := e.postTradeMarginRatio(bal, 0)
	if ok {
		t.Errorf("ratio=%v at threshold should be !ok", ratio)
	}

	// Small margin debited: ratio climbs above 0.80 → !ok.
	_, ok = e.postTradeMarginRatio(bal, 50)
	if ok {
		t.Error("expected !ok after debiting 50 margin (ratio goes to 0.90)")
	}

	// Healthier balance: ratio 0.60 is well below threshold.
	healthy := rebalanceBalanceInfo{futures: 200, futuresTotal: 500}
	_, ok = e.postTradeMarginRatio(healthy, 0)
	if !ok {
		t.Error("expected ok for healthy 0.60 ratio")
	}

	// Zero futuresTotal: guard returns ok=true, ratio=0.
	empty := rebalanceBalanceInfo{futures: 0, futuresTotal: 0}
	ratio, ok = e.postTradeMarginRatio(empty, 0)
	if !ok || ratio != 0 {
		t.Errorf("empty bal: want (0, true), got (%v, %v)", ratio, ok)
	}
}
