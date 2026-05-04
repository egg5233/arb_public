package pricegaptrader

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
)

// ---- Fixtures ---------------------------------------------------------------

func newExecTestTracker(t *testing.T) (*Tracker, *stubExchange, *stubExchange, *fakeStore) {
	t.Helper()
	longEx := newStubExchange("binance")
	shortEx := newStubExchange("gate")
	store := newFakeStore()
	delist := newFakeDelistChecker()
	cfg := &config.Config{
		PriceGapEnabled:              true,
		PriceGapBudget:               5000,
		PriceGapMaxConcurrent:        3,
		PriceGapGateConcentrationPct: 0.5,
		PriceGapKlineStalenessSec:    90,
	}
	exch := map[string]exchange.Exchange{
		"binance": longEx,
		"gate":    shortEx,
	}
	tr := NewTracker(exch, store, delist, cfg)
	return tr, longEx, shortEx, store
}

func execCand() models.PriceGapCandidate {
	return models.PriceGapCandidate{
		Symbol:             "SOON",
		LongExch:           "binance",
		ShortExch:          "gate",
		ThresholdBps:       200,
		MaxPositionUSDT:    5000,
		ModeledSlippageBps: 7.5,
	}
}

func execDet() DetectionResult {
	return DetectionResult{
		SpreadBps:    250,
		MidLong:      1.00,
		MidShort:     1.025,
		StalenessSec: 1,
	}
}

// countReduceOnly counts ReduceOnly orders placed on a leg.
func countReduceOnly(ords []exchange.PlaceOrderParams) int {
	n := 0
	for _, o := range ords {
		if o.ReduceOnly {
			n++
		}
	}
	return n
}

// ---- Tests ------------------------------------------------------------------

func TestExecution_HappyPath(t *testing.T) {
	tr, longEx, shortEx, store := newExecTestTracker(t)
	longEx.queueFill(100, 1.00, nil)
	shortEx.queueFill(100, 1.025, nil)

	pos, err := tr.openPair(execCand(), 100, execDet())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if pos == nil {
		t.Fatal("expected position, got nil")
	}
	if pos.Status != models.PriceGapStatusOpen {
		t.Errorf("Status = %q, want open", pos.Status)
	}
	if pos.LongSize != 100 || pos.ShortSize != 100 {
		t.Errorf("sizes = (%v, %v), want (100, 100)", pos.LongSize, pos.ShortSize)
	}
	if pos.EntrySpreadBps != 250 {
		t.Errorf("EntrySpreadBps = %v, want 250", pos.EntrySpreadBps)
	}
	if pos.ModeledSlipBps != 7.5 {
		t.Errorf("ModeledSlipBps = %v, want 7.5", pos.ModeledSlipBps)
	}
	// No unwind orders.
	if n := countReduceOnly(longEx.placedOrders()); n != 0 {
		t.Errorf("long ReduceOnly count = %d, want 0", n)
	}
	if n := countReduceOnly(shortEx.placedOrders()); n != 0 {
		t.Errorf("short ReduceOnly count = %d, want 0", n)
	}
	if len(store.saved) != 1 {
		t.Errorf("saved position count = %d, want 1", len(store.saved))
	}
}

func TestExecution_PartialFill_UnwindMatches(t *testing.T) {
	tr, longEx, shortEx, _ := newExecTestTracker(t)
	longEx.queueFill(100, 1.00, nil)
	shortEx.queueFill(80, 1.025, nil)

	pos, err := tr.openPair(execCand(), 100, execDet())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if pos.LongSize != 80 || pos.ShortSize != 80 {
		t.Errorf("unwind-to-match sizes = (%v, %v), want (80, 80)",
			pos.LongSize, pos.ShortSize)
	}
	// Long leg should have 1 entry + 1 ReduceOnly sell for 20.
	longOrds := longEx.placedOrders()
	if len(longOrds) != 2 {
		t.Fatalf("long order count = %d, want 2", len(longOrds))
	}
	if countReduceOnly(longOrds) != 1 {
		t.Errorf("long ReduceOnly count = %d, want 1", countReduceOnly(longOrds))
	}
	// The ReduceOnly order should be a SELL closing 20 base units.
	unwind := longOrds[1]
	if unwind.Side != exchange.SideSell {
		t.Errorf("unwind side = %v, want sell", unwind.Side)
	}
	size, _ := strconv.ParseFloat(unwind.Size, 64)
	if size < 19.99 || size > 20.01 {
		t.Errorf("unwind size = %v, want ~20", size)
	}
	// Short leg untouched beyond entry.
	if len(shortEx.placedOrders()) != 1 {
		t.Errorf("short order count = %d, want 1", len(shortEx.placedOrders()))
	}
}

func TestExecution_ZeroFillOneLeg_MarketClosesOther(t *testing.T) {
	tr, longEx, shortEx, store := newExecTestTracker(t)
	longEx.queueFill(100, 1.00, nil) // long filled
	shortEx.queueFill(0, 0, nil)     // short zero-fill

	pos, err := tr.openPair(execCand(), 100, execDet())
	if err == nil {
		t.Fatal("expected error on zero-fill, got nil")
	}
	if pos != nil {
		t.Errorf("pos should be nil on zero-fill, got %+v", pos)
	}
	// Long leg must have entry + ReduceOnly sell for 100.
	longOrds := longEx.placedOrders()
	if len(longOrds) != 2 {
		t.Fatalf("long order count = %d, want 2", len(longOrds))
	}
	unwind := longOrds[1]
	if !unwind.ReduceOnly || unwind.Side != exchange.SideSell {
		t.Errorf("unwind = %+v, want ReduceOnly SELL", unwind)
	}
	// No position saved.
	if len(store.saved) != 0 {
		t.Errorf("saved positions = %d, want 0", len(store.saved))
	}
}

func TestExecution_BothLegsFail_NoCleanup(t *testing.T) {
	tr, longEx, shortEx, store := newExecTestTracker(t)
	longEx.queueFill(0, 0, errors.New("long down"))
	shortEx.queueFill(0, 0, errors.New("short down"))

	_, err := tr.openPair(execCand(), 100, execDet())
	if err == nil {
		t.Fatal("expected err on both-fail, got nil")
	}
	// Exactly one PlaceOrder call per leg — the failed entry attempt. No cleanup.
	if n := len(longEx.placedOrders()); n != 1 {
		t.Errorf("long orders = %d, want 1 (entry attempt only)", n)
	}
	if n := len(shortEx.placedOrders()); n != 1 {
		t.Errorf("short orders = %d, want 1 (entry attempt only)", n)
	}
	if len(store.saved) != 0 {
		t.Errorf("saved = %d, want 0", len(store.saved))
	}
}

func TestExecution_BingXShortPreflightBlocksBeforeAnyRealLeg(t *testing.T) {
	tr, longEx, _, store := newExecTestTracker(t)
	bingx := newStubExchange("bingx")
	bingx.preflightErr = errors.New("bingx API error code=109400 msg=API orders are temporarily disabled")
	tr.exchanges["bingx"] = bingx

	cand := execCand()
	cand.ShortExch = "bingx"

	_, err := tr.openPair(cand, 100, execDet())
	if err == nil {
		t.Fatal("openPair returned nil error, want BingX preflight rejection")
	}
	if !strings.Contains(err.Error(), "pricegap: bingx preflight blocked SOON sell") {
		t.Fatalf("error = %q, want BingX preflight context", err.Error())
	}
	if len(longEx.placedOrders()) != 0 {
		t.Fatalf("non-BingX long real orders = %d, want 0", len(longEx.placedOrders()))
	}
	if len(bingx.placedOrders()) != 0 {
		t.Fatalf("BingX real orders = %d, want 0", len(bingx.placedOrders()))
	}
	if len(store.saved) != 0 {
		t.Fatalf("saved positions = %d, want 0", len(store.saved))
	}
	calls := bingx.preflightOrders()
	if len(calls) != 1 {
		t.Fatalf("BingX preflight calls = %d, want 1", len(calls))
	}
	call := calls[0]
	if call.Symbol != "SOON" ||
		call.Side != exchange.SideSell ||
		call.OrderType != "limit" ||
		call.Price != "2.03975000" ||
		call.Size != "100.000000" ||
		call.Force != "ioc" {
		t.Fatalf("BingX preflight params = %+v", call)
	}
}

func TestExecution_BingXLongPreflightRunsBeforeLiveOrders(t *testing.T) {
	tr, _, shortEx, _ := newExecTestTracker(t)
	bingx := newStubExchange("bingx")
	bingx.contracts["SOON"] = exchange.ContractInfo{PriceStep: 0.0001, PriceDecimals: 4, SizeDecimals: 2}
	tr.exchanges["bingx"] = bingx

	cand := execCand()
	cand.LongExch = "bingx"
	bingx.queueFill(100, 1.00, nil)
	shortEx.queueFill(100, 1.025, nil)

	pos, err := tr.openPair(cand, 100, execDet())
	if err != nil {
		t.Fatalf("openPair returned error: %v", err)
	}
	if pos == nil {
		t.Fatal("expected position, got nil")
	}
	calls := bingx.preflightOrders()
	if len(calls) != 1 {
		t.Fatalf("BingX preflight calls = %d, want 1", len(calls))
	}
	call := calls[0]
	if call.Symbol != "SOON" ||
		call.Side != exchange.SideBuy ||
		call.OrderType != "limit" ||
		call.Price != "0.0100" ||
		call.Size != "100.00" ||
		call.Force != "ioc" {
		t.Fatalf("BingX preflight params = %+v", call)
	}
	if len(bingx.placedOrders()) != 1 {
		t.Fatalf("BingX real orders = %d, want 1 after successful preflight", len(bingx.placedOrders()))
	}
	if len(shortEx.placedOrders()) != 1 {
		t.Fatalf("short real orders = %d, want 1 after successful preflight", len(shortEx.placedOrders()))
	}
}

func TestExecution_BingXInverseWireLongPreflightBlocksBeforeLiveOrders(t *testing.T) {
	tr, longEx, _, store := newExecTestTracker(t)
	bingx := newStubExchange("bingx")
	bingx.preflightErr = errors.New("bingx API error code=109400 msg=API orders are temporarily disabled")
	tr.exchanges["bingx"] = bingx

	cand := bidiCand()
	cand.ShortExch = "bingx"

	_, err := tr.openPair(cand, 100, bidiInverseDet())
	if err == nil {
		t.Fatal("openPair returned nil error, want inverse BingX preflight rejection")
	}
	if !strings.Contains(err.Error(), "pricegap: bingx preflight blocked SOON buy") {
		t.Fatalf("error = %q, want inverse BingX buy preflight context", err.Error())
	}
	calls := bingx.preflightOrders()
	if len(calls) != 1 {
		t.Fatalf("BingX preflight calls = %d, want 1", len(calls))
	}
	if calls[0].Side != exchange.SideBuy {
		t.Fatalf("BingX inverse preflight side = %s, want buy", calls[0].Side)
	}
	if len(longEx.placedOrders()) != 0 || len(bingx.placedOrders()) != 0 {
		t.Fatalf("live orders before failed inverse preflight: long=%d bingx=%d",
			len(longEx.placedOrders()), len(bingx.placedOrders()))
	}
	if len(store.saved) != 0 {
		t.Fatalf("saved positions = %d, want 0", len(store.saved))
	}
}

func TestExecution_CircuitBreakerOpensAfterFiveFailures(t *testing.T) {
	tr, longEx, shortEx, _ := newExecTestTracker(t)
	// Queue 3 rounds of both-legs-fail → 6 failure bumps total; breaker trips at 5.
	for i := 0; i < 3; i++ {
		longEx.queueFill(0, 0, errors.New("long down"))
		shortEx.queueFill(0, 0, errors.New("short down"))
	}
	for i := 0; i < 3; i++ {
		_, _ = tr.openPair(execCand(), 100, execDet())
	}
	if !tr.isCircuitOpen() {
		t.Fatal("circuit breaker should be open after 6 failures")
	}
	// Subsequent attempt should short-circuit with ErrPriceGapCircuitBreaker
	// and place NO new orders.
	longBefore := len(longEx.placedOrders())
	shortBefore := len(shortEx.placedOrders())
	_, err := tr.openPair(execCand(), 100, execDet())
	if !errors.Is(err, ErrPriceGapCircuitBreaker) {
		t.Errorf("err = %v, want ErrPriceGapCircuitBreaker", err)
	}
	if len(longEx.placedOrders()) != longBefore {
		t.Error("long orders changed after circuit trip")
	}
	if len(shortEx.placedOrders()) != shortBefore {
		t.Error("short orders changed after circuit trip")
	}
}

func TestExecution_LockHeld_SecondCallBlocks(t *testing.T) {
	tr, longEx, shortEx, store := newExecTestTracker(t)
	store.forceLockBusy = true

	_, err := tr.openPair(execCand(), 100, execDet())
	if err == nil {
		t.Fatal("expected lock-busy error, got nil")
	}
	// No orders placed because lock acquisition happens before PlaceOrder.
	if n := len(longEx.placedOrders()); n != 0 {
		t.Errorf("long orders = %d, want 0", n)
	}
	if n := len(shortEx.placedOrders()); n != 0 {
		t.Errorf("short orders = %d, want 0", n)
	}
}

func TestExecution_ResetFailuresOnSuccess(t *testing.T) {
	tr, longEx, shortEx, _ := newExecTestTracker(t)
	// Round 1: both legs fail → failures = 2
	longEx.queueFill(0, 0, errors.New("long down"))
	shortEx.queueFill(0, 0, errors.New("short down"))
	_, _ = tr.openPair(execCand(), 100, execDet())
	tr.failMu.Lock()
	if tr.failures < 2 {
		tr.failMu.Unlock()
		t.Fatalf("after 1st round failures = %d, want >= 2", tr.failures)
	}
	tr.failMu.Unlock()

	// Round 2: both legs succeed → counter resets to 0.
	longEx.queueFill(100, 1.00, nil)
	shortEx.queueFill(100, 1.025, nil)
	_, err := tr.openPair(execCand(), 100, execDet())
	if err != nil {
		t.Fatalf("round 2 unexpected err: %v", err)
	}
	tr.failMu.Lock()
	got := tr.failures
	tr.failMu.Unlock()
	if got != 0 {
		t.Errorf("failures after success = %d, want 0", got)
	}
}

// ---- PG-DIR-01: bidirectional executor leg-role swap tests -----------------

// bidiCand returns a bidirectional candidate (same tuple as execCand).
func bidiCand() models.PriceGapCandidate {
	c := execCand()
	c.Direction = models.PriceGapDirectionBidirectional
	return c
}

// bidiInverseDet returns a DetectionResult with NEGATIVE spread (configured
// short_exch is the cheaper / "wire-side long" leg).
func bidiInverseDet() DetectionResult {
	return DetectionResult{
		SpreadBps:    -250,
		MidLong:      1.025, // configured-long-exch mid (the EXPENSIVE side)
		MidShort:     1.00,  // configured-short-exch mid (the CHEAP side)
		StalenessSec: 1,
	}
}

// TestOpenPair_PinnedForward — pinned candidate, positive spread → BUY on
// configured LongExch, SELL on configured ShortExch; FiredDirection="forward";
// CandidateLongExch == LongExchange == configured.
func TestOpenPair_PinnedForward(t *testing.T) {
	tr, longEx, shortEx, _ := newExecTestTracker(t)
	cand := execCand() // Direction="" → treated as pinned
	longEx.queueFill(100, 1.00, nil)
	shortEx.queueFill(100, 1.025, nil)

	pos, err := tr.openPair(cand, 100, execDet())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if pos.FiredDirection != models.PriceGapFiredForward {
		t.Errorf("FiredDirection = %q, want %q", pos.FiredDirection, models.PriceGapFiredForward)
	}
	if pos.LongExchange != "binance" || pos.ShortExchange != "gate" {
		t.Errorf("wire-side roles = (%q, %q), want (binance, gate)", pos.LongExchange, pos.ShortExchange)
	}
	if pos.CandidateLongExch != "binance" || pos.CandidateShortExch != "gate" {
		t.Errorf("configured tuple = (%q, %q), want (binance, gate)", pos.CandidateLongExch, pos.CandidateShortExch)
	}
	// BUY went to long_exch (binance), SELL to short_exch (gate).
	longOrds := longEx.placedOrders()
	shortOrds := shortEx.placedOrders()
	if len(longOrds) == 0 || longOrds[0].Side != exchange.SideBuy {
		t.Errorf("expected BUY on binance, got %+v", longOrds)
	}
	if len(shortOrds) == 0 || shortOrds[0].Side != exchange.SideSell {
		t.Errorf("expected SELL on gate, got %+v", shortOrds)
	}
}

// TestOpenPair_BidirectionalForward — bidirectional + positive spread behaves
// identically to pinned forward (no swap).
func TestOpenPair_BidirectionalForward(t *testing.T) {
	tr, longEx, shortEx, _ := newExecTestTracker(t)
	longEx.queueFill(100, 1.00, nil)
	shortEx.queueFill(100, 1.025, nil)

	pos, err := tr.openPair(bidiCand(), 100, execDet())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if pos.FiredDirection != models.PriceGapFiredForward {
		t.Errorf("FiredDirection = %q, want forward", pos.FiredDirection)
	}
	if pos.LongExchange != "binance" || pos.ShortExchange != "gate" {
		t.Errorf("wire-side roles = (%q, %q), want (binance, gate)", pos.LongExchange, pos.ShortExchange)
	}
}

// TestOpenPair_BidirectionalInverse — bidirectional + negative spread →
// SWAP wire-side roles. BUY goes to configured short_exch (now the
// wire-side long); SELL goes to configured long_exch. CandidateLongExch
// preserves the CONFIGURED tuple for Phase 10 D-11 guard matching.
func TestOpenPair_BidirectionalInverse(t *testing.T) {
	tr, longEx, shortEx, _ := newExecTestTracker(t)
	// "longEx" var is configured-long (binance); "shortEx" is configured-short (gate).
	// In an inverse fire, BUY goes to gate (configured short, wire long) and
	// SELL goes to binance (configured long, wire short). Queue fills accordingly.
	shortEx.queueFill(100, 1.00, nil) // wire-side long fill
	longEx.queueFill(100, 1.025, nil) // wire-side short fill

	pos, err := tr.openPair(bidiCand(), 100, bidiInverseDet())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if pos.FiredDirection != models.PriceGapFiredInverse {
		t.Errorf("FiredDirection = %q, want %q", pos.FiredDirection, models.PriceGapFiredInverse)
	}
	// Wire-side roles SWAPPED.
	if pos.LongExchange != "gate" || pos.ShortExchange != "binance" {
		t.Errorf("wire-side roles = (%q, %q), want (gate, binance) [swapped]", pos.LongExchange, pos.ShortExchange)
	}
	// Configured tuple PRESERVED.
	if pos.CandidateLongExch != "binance" || pos.CandidateShortExch != "gate" {
		t.Errorf("configured tuple = (%q, %q), want (binance, gate) [unchanged]",
			pos.CandidateLongExch, pos.CandidateShortExch)
	}
	// BUY went to gate (configured short, wire long), SELL to binance.
	gateOrds := shortEx.placedOrders()
	binanceOrds := longEx.placedOrders()
	if len(gateOrds) == 0 || gateOrds[0].Side != exchange.SideBuy {
		t.Errorf("expected BUY on gate (wire-side long), got %+v", gateOrds)
	}
	if len(binanceOrds) == 0 || binanceOrds[0].Side != exchange.SideSell {
		t.Errorf("expected SELL on binance (wire-side short), got %+v", binanceOrds)
	}
}

// TestOpenPair_LockKeyUnchangedByDirection — the per-symbol Redis lock key
// uses the CONFIGURED tuple ({Symbol}:{cand.LongExch}:{cand.ShortExch}) so a
// single candidate cannot fire forward and inverse concurrently. Asserted by
// observing that an inverse fire still acquires the SAME lock resource
// string as a forward fire.
func TestOpenPair_LockKeyUnchangedByDirection(t *testing.T) {
	tr, longEx, shortEx, store := newExecTestTracker(t)
	// First call: forward fire, succeeds.
	longEx.queueFill(100, 1.00, nil)
	shortEx.queueFill(100, 1.025, nil)
	if _, err := tr.openPair(bidiCand(), 100, execDet()); err != nil {
		t.Fatalf("forward fire: %v", err)
	}
	forwardLock := ""
	if len(store.lockHistory) > 0 {
		forwardLock = store.lockHistory[0]
	}

	// Second call: inverse fire, would-be lock res must match forward's.
	shortEx.queueFill(100, 1.00, nil)
	longEx.queueFill(100, 1.025, nil)
	if _, err := tr.openPair(bidiCand(), 100, bidiInverseDet()); err != nil {
		t.Fatalf("inverse fire: %v", err)
	}
	inverseLock := ""
	if len(store.lockHistory) >= 2 {
		inverseLock = store.lockHistory[1]
	}
	if forwardLock == "" || forwardLock != inverseLock {
		t.Fatalf("lock keys differ across directions: forward=%q inverse=%q (must use configured tuple)",
			forwardLock, inverseLock)
	}
}

// TestOpenPair_PaperModeInverseSwap — bidirectional + paper-mode + negative
// spread → paper synth fills are computed on the SWAPPED legs (paper-mode
// chokepoint preserved per Phase 9 D-12). Asserts pos.LongFillPrice reflects
// a fill at wire-side-long mid (configured short_exch's mid = 1.00 here).
func TestOpenPair_PaperModeInverseSwap(t *testing.T) {
	tr, _, _, _ := newExecTestTracker(t)
	tr.cfg.PriceGapPaperMode = true
	cand := bidiCand()
	cand.ModeledSlippageBps = 0 // simplify — synth fills exactly at mid

	pos, err := tr.openPair(cand, 100, bidiInverseDet())
	if err != nil {
		t.Fatalf("paper inverse: %v", err)
	}
	if pos.FiredDirection != models.PriceGapFiredInverse {
		t.Errorf("FiredDirection = %q, want inverse", pos.FiredDirection)
	}
	// In an inverse fire, wire-side LongMid is the configured-short mid (1.00).
	if pos.LongMidAtDecision != 1.00 {
		t.Errorf("LongMidAtDecision = %v, want 1.00 (configured short mid post-swap)", pos.LongMidAtDecision)
	}
	if pos.ShortMidAtDecision != 1.025 {
		t.Errorf("ShortMidAtDecision = %v, want 1.025 (configured long mid post-swap)", pos.ShortMidAtDecision)
	}
	// With ModeledSlippageBps=0, paper synth = mid exactly.
	if pos.LongFillPrice != 1.00 {
		t.Errorf("LongFillPrice = %v, want 1.00 (paper synth at wire-side-long mid)", pos.LongFillPrice)
	}
	if pos.ShortFillPrice != 1.025 {
		t.Errorf("ShortFillPrice = %v, want 1.025 (paper synth at wire-side-short mid)", pos.ShortFillPrice)
	}
}

func TestExecution_PositionID_Format(t *testing.T) {
	tr, longEx, shortEx, _ := newExecTestTracker(t)
	longEx.queueFill(100, 1.00, nil)
	shortEx.queueFill(100, 1.025, nil)

	pos, err := tr.openPair(execCand(), 100, execDet())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	re := regexp.MustCompile(`^pg_SOON_binance_gate_\d+$`)
	if !re.MatchString(pos.ID) {
		t.Errorf("pos.ID = %q, want match %s", pos.ID, re)
	}
}
