// scanner_test.go — Plan 04 unit tests for the auto-discovery scanner.
//
// Coverage map (PLAN 11-04 Test 1 .. Test 15):
//   Test 1  TestScanner_DefaultOff           — PriceGapDiscoveryEnabled=false → only enabled-flag write
//   Test 2  TestScanner_UniverseWalk         — universe × cross-product cardinality
//   Test 3  TestScanner_PersistenceGate      — 1-bar fail → 4-bar pass
//   Test 4  TestScanner_BBOFreshnessGate     — stale BBO resets ring + reason=stale_bbo
//   Test 5  TestScanner_DepthGate            — clearable < floor → reason=insufficient_depth
//   Test 6  TestScanner_DenylistSymbol       — symbol-form denylist match
//   Test 7  TestScanner_DenylistTuple        — symbol@exchange tuple match
//   Test 8  TestScanner_BybitBlackout        — :04..:05:30 wall-clock window
//   Test 9  TestScanner_SingletonSilentSkip  — only one listing → WriteSymbolNoCrossPair
//   Test 10 TestScanner_GateThenMagnitude    — all-pass yields non-zero score + sub-scores
//   Test 11 TestScanner_OneBarPumpRejected   — Pitfall 1 antidote
//   Test 12 TestScanner_CanonicalSymbolJoin  — RESEARCH A1 normalization
//   Test 13 (compile-time read-only)         — covered by scanner_static_test.go
//   Test 14 TestScanner_CycleErrorBudget     — 5 consecutive errors → cycle aborted
//   Test 15 TestScanner_SubScoreRecord       — D-03 sub-score persistence
package pricegaptrader

import (
	"context"
	"sync"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// ---------------------------------------------------------------------------
// Scanner-specific fakes (kept local to scanner_test.go so the existing
// stubExchange contract elsewhere isn't perturbed).
// ---------------------------------------------------------------------------

// scanStub wraps the package-wide stubExchange to add a populated GetDepth
// hook. The ambient stubExchange.GetDepth always returns nil/false; the
// scanner needs depth probes for the depth gate, so we shadow that method.
type scanStub struct {
	*stubExchange
	depthMu sync.Mutex
	depths  map[string]*exchange.Orderbook
}

func newScanStub(name string) *scanStub {
	return &scanStub{
		stubExchange: newStubExchange(name),
		depths:       map[string]*exchange.Orderbook{},
	}
}

func (s *scanStub) setDepth(symbol string, bids, asks []exchange.PriceLevel) {
	s.depthMu.Lock()
	defer s.depthMu.Unlock()
	s.depths[symbol] = &exchange.Orderbook{Symbol: symbol, Bids: bids, Asks: asks, Time: time.Now()}
}

// GetDepth shadows stubExchange.GetDepth to surface the test-injected book.
func (s *scanStub) GetDepth(symbol string) (*exchange.Orderbook, bool) {
	s.depthMu.Lock()
	defer s.depthMu.Unlock()
	if ob, ok := s.depths[symbol]; ok {
		return ob, true
	}
	return nil, false
}

// fakeRegistryReader is the read-only registry used by scanner. The scanner
// only consults List() for the "already promoted" set.
type fakeRegistryReader struct {
	items []models.PriceGapCandidate
}

func (f *fakeRegistryReader) Get(idx int) (models.PriceGapCandidate, bool) {
	if idx < 0 || idx >= len(f.items) {
		return models.PriceGapCandidate{}, false
	}
	return f.items[idx], true
}
func (f *fakeRegistryReader) List() []models.PriceGapCandidate {
	out := make([]models.PriceGapCandidate, len(f.items))
	copy(out, f.items)
	return out
}

// fakeTele captures every TelemetryWriter call so tests can assert exact
// counts and the contents of CycleSummary.
type fakeTele struct {
	mu              sync.Mutex
	enabledFlags    []bool
	cycles          []CycleSummary
	noCrossPair     int
	cycleFailed     int
	writeCycleErr   error
	writeFlagErr    error
	writeNoPairErr  error
	writeFailErr    error
}

func (f *fakeTele) WriteCycle(_ context.Context, s CycleSummary) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cycles = append(f.cycles, s)
	return f.writeCycleErr
}
func (f *fakeTele) WriteEnabledFlag(_ context.Context, on bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enabledFlags = append(f.enabledFlags, on)
	return f.writeFlagErr
}
func (f *fakeTele) WriteSymbolNoCrossPair(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.noCrossPair++
	return f.writeNoPairErr
}
func (f *fakeTele) WriteCycleFailed(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cycleFailed++
	return f.writeFailErr
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newScannerCfg() *config.Config {
	return &config.Config{
		PriceGapDiscoveryEnabled:      true,
		PriceGapDiscoveryUniverse:     []string{"BTCUSDT"},
		PriceGapDiscoveryDenylist:     nil,
		PriceGapDiscoveryIntervalSec:  300,
		PriceGapDiscoveryThresholdBps: 100,
		PriceGapDiscoveryMinDepthUSDT: 1000,
		PriceGapKlineStalenessSec:     90,
		PriceGapMaxCandidates:         12,
		PriceGapAutoPromoteScore:      60,
	}
}

// fillBookAtSpread synthesizes a one-level orderbook with `qty` units at
// `mid`, so depthClearable at threshold-bps spread = mid * qty. Used to
// drive the depth gate above/below the floor.
func fillBookAtSpread(mid, qty float64) ([]exchange.PriceLevel, []exchange.PriceLevel) {
	bid := []exchange.PriceLevel{{Price: mid, Quantity: qty}}
	ask := []exchange.PriceLevel{{Price: mid, Quantity: qty}}
	return bid, ask
}

// nonBlackout is :40 — the regular EntryScan minute, well outside the bybit
// blackout window :04-:05:30.
func nonBlackout(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 12, 40, 0, 0, time.UTC)
}

// runCycles invokes RunCycle `n` times advancing the clock by 1 minute each
// step. Tests that exercise the persistence gate depend on this — barRing.push
// dedups within the same minute, so each tick MUST land on a new unix-minute.
func runCycles(s *Scanner, ctx context.Context, n int, start time.Time) {
	for i := 0; i < n; i++ {
		s.RunCycle(ctx, start.Add(time.Duration(i)*time.Minute))
	}
}

// scenario builds a scanner with two stubs (binance long, bybit short) each
// publishing the supplied BTCUSDT BBO + depth book.
func scenarioTwoLeg(longBid, longAsk, shortBid, shortAsk, depthQty float64) (
	*Scanner, *fakeTele, *fakeRegistryReader, *scanStub, *scanStub,
) {
	cfg := newScannerCfg()
	long := newScanStub("binance")
	short := newScanStub("bybit")
	long.setBBO("BTCUSDT", longBid, longAsk, time.Now())
	short.setBBO("BTCUSDT", shortBid, shortAsk, time.Now())
	bL, aL := fillBookAtSpread((longBid+longAsk)/2, depthQty)
	bS, aS := fillBookAtSpread((shortBid+shortAsk)/2, depthQty)
	long.setDepth("BTCUSDT", bL, aL)
	short.setDepth("BTCUSDT", bS, aS)
	exchanges := map[string]exchange.Exchange{"binance": long, "bybit": short}
	tele := &fakeTele{}
	reg := &fakeRegistryReader{}
	log := utils.NewLogger("scanner-test")
	sc := NewScanner(cfg, reg, exchanges, tele, log)
	return sc, tele, reg, long, short
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// Test 1 — D-11 default OFF: with PriceGapDiscoveryEnabled=false, RunCycle
// invokes only WriteEnabledFlag(false) and emits NO per-symbol records.
func TestScanner_DefaultOff(t *testing.T) {
	cfg := newScannerCfg()
	cfg.PriceGapDiscoveryEnabled = false
	tele := &fakeTele{}
	long := newScanStub("binance")
	short := newScanStub("bybit")
	exchanges := map[string]exchange.Exchange{"binance": long, "bybit": short}
	reg := &fakeRegistryReader{}
	log := utils.NewLogger("scanner-test")
	sc := NewScanner(cfg, reg, exchanges, tele, log)

	sc.RunCycle(context.Background(), nonBlackout(time.Now()))

	if len(tele.enabledFlags) != 1 || tele.enabledFlags[0] != false {
		t.Fatalf("expected exactly one WriteEnabledFlag(false); got %+v", tele.enabledFlags)
	}
	if len(tele.cycles) != 0 {
		t.Fatalf("expected zero WriteCycle calls when disabled; got %d", len(tele.cycles))
	}
	if tele.noCrossPair != 0 {
		t.Fatalf("expected zero WriteSymbolNoCrossPair when disabled; got %d", tele.noCrossPair)
	}
	if tele.cycleFailed != 0 {
		t.Fatalf("expected zero WriteCycleFailed when disabled; got %d", tele.cycleFailed)
	}
}

// Test 2 — universe walk: 2 symbols × 3 supported exchanges → 2 * (3*2) = 12
// cross-pair records (P(3,2)=6 ordered pairs per symbol).
func TestScanner_UniverseWalk(t *testing.T) {
	cfg := newScannerCfg()
	cfg.PriceGapDiscoveryUniverse = []string{"BTCUSDT", "ETHUSDT"}
	cfg.PriceGapDiscoveryThresholdBps = 1000000 // ensure no accept; we only count records
	tele := &fakeTele{}
	a := newScanStub("a")
	b := newScanStub("b")
	c := newScanStub("c")
	for _, ex := range []*scanStub{a, b, c} {
		ex.setBBO("BTCUSDT", 100, 100.1, time.Now())
		ex.setBBO("ETHUSDT", 50, 50.05, time.Now())
		bL, aL := fillBookAtSpread(100.05, 100)
		ex.setDepth("BTCUSDT", bL, aL)
		bE, aE := fillBookAtSpread(50.025, 100)
		ex.setDepth("ETHUSDT", bE, aE)
	}
	exchanges := map[string]exchange.Exchange{"a": a, "b": b, "c": c}
	reg := &fakeRegistryReader{}
	sc := NewScanner(cfg, reg, exchanges, tele, utils.NewLogger("t"))

	sc.RunCycle(context.Background(), nonBlackout(time.Now()))

	if got, want := len(tele.cycles), 1; got != want {
		t.Fatalf("WriteCycle calls=%d want %d", got, want)
	}
	got := len(tele.cycles[0].Records)
	want := 2 * (3 * 2) // 2 symbols × P(3,2)=6 ordered pairs
	if got != want {
		t.Fatalf("records=%d want %d", got, want)
	}
}

// Test 3 — ≥4-bar persistence: a single bar with crossing spread scores 0
// with reason=insufficient_persistence; after 4 bars same-sign the gate passes.
func TestScanner_PersistenceGate(t *testing.T) {
	sc, tele, _, _, _ := scenarioTwoLeg(100, 100.1, 102, 102.1, 100)
	start := nonBlackout(time.Now())

	// First cycle — only 1 bar populated; persistence gate fails.
	sc.RunCycle(context.Background(), start)
	if got := len(tele.cycles); got != 1 {
		t.Fatalf("cycle count=%d want 1", got)
	}
	rec := findRec(tele.cycles[0].Records, "BTCUSDT", "binance", "bybit")
	if rec == nil {
		t.Fatalf("no record for binance→bybit on first cycle")
	}
	if rec.WhyRejected != ReasonInsufficientPersistence {
		t.Fatalf("rec.WhyRejected=%q want %q", rec.WhyRejected, ReasonInsufficientPersistence)
	}
	if rec.Score != 0 {
		t.Fatalf("rec.Score=%d want 0", rec.Score)
	}

	// Run 3 more cycles spaced 1 minute apart so the bar ring fills with
	// 4 distinct unix-minute samples — same-sign, all exceed threshold.
	runCycles(sc, context.Background(), 3, start.Add(time.Minute))

	last := tele.cycles[len(tele.cycles)-1]
	rec = findRec(last.Records, "BTCUSDT", "binance", "bybit")
	if rec == nil {
		t.Fatalf("no final record")
	}
	if rec.Score == 0 {
		t.Fatalf("expected non-zero score after 4 bars; got 0 (rejected=%q)", rec.WhyRejected)
	}
}

// Test 4 — BBO freshness: simulate a stale leg by advancing time beyond
// PriceGapKlineStalenessSec without ticking; the next cycle resets the ring
// and emits reason=stale_bbo for that pair.
func TestScanner_BBOFreshnessGate(t *testing.T) {
	sc, tele, _, _, _ := scenarioTwoLeg(100, 100.1, 102, 102.1, 100)
	start := nonBlackout(time.Now())

	// Build a 4-bar same-sign chain so the persistence gate is on track to pass.
	runCycles(sc, context.Background(), 4, start)

	// Skip 5 minutes (>90s threshold) — the next sample is stale relative to
	// last record. Persistence ring should reset → reason=stale_bbo.
	sc.RunCycle(context.Background(), start.Add(10*time.Minute))
	last := tele.cycles[len(tele.cycles)-1]
	rec := findRec(last.Records, "BTCUSDT", "binance", "bybit")
	if rec == nil {
		t.Fatalf("no record after stale gap")
	}
	if rec.WhyRejected != ReasonStaleBBO {
		t.Fatalf("WhyRejected=%q want %q", rec.WhyRejected, ReasonStaleBBO)
	}
	if rec.Score != 0 {
		t.Fatalf("Score=%d want 0", rec.Score)
	}
}

// Test 5 — depth probe: clearable USDT at threshold spread is below the
// floor → reason=insufficient_depth. Above floor (in scenarioTwoLeg with
// qty=100) the depth gate passes and persistence is the limiting gate.
func TestScanner_DepthGate(t *testing.T) {
	// Tiny depth book — clearable USDT will be ~mid * 0.001 < $1000 floor.
	sc, tele, _, longEx, shortEx := scenarioTwoLeg(100, 100.1, 102, 102.1, 0.001)
	_ = longEx
	_ = shortEx
	start := nonBlackout(time.Now())

	runCycles(sc, context.Background(), 4, start)

	last := tele.cycles[len(tele.cycles)-1]
	rec := findRec(last.Records, "BTCUSDT", "binance", "bybit")
	if rec == nil {
		t.Fatalf("no record")
	}
	if rec.WhyRejected != ReasonInsufficientDepth {
		t.Fatalf("WhyRejected=%q want %q", rec.WhyRejected, ReasonInsufficientDepth)
	}
}

// Test 6 — denylist by symbol: a denylist entry of "BTCUSDT" rejects every
// cross-pair for that symbol with reason=denylist.
func TestScanner_DenylistSymbol(t *testing.T) {
	sc, tele, _, _, _ := scenarioTwoLeg(100, 100.1, 102, 102.1, 100)
	sc.cfg.PriceGapDiscoveryDenylist = []string{"BTCUSDT"}

	sc.RunCycle(context.Background(), nonBlackout(time.Now()))

	last := tele.cycles[len(tele.cycles)-1]
	if len(last.Records) == 0 {
		t.Fatalf("no records emitted")
	}
	for _, r := range last.Records {
		if r.WhyRejected != ReasonDenylist {
			t.Errorf("rec %s/%s→%s WhyRejected=%q want denylist",
				r.Symbol, r.LongExch, r.ShortExch, r.WhyRejected)
		}
	}
}

// Test 7 — denylist tuple: only the (long=any, short=bybit) and
// (long=bybit, short=any) pairs are rejected for that tuple; a third
// exchange's pair should still pass denylist gate (and fail later at
// persistence on first cycle, which is fine).
func TestScanner_DenylistTuple(t *testing.T) {
	cfg := newScannerCfg()
	cfg.PriceGapDiscoveryDenylist = []string{"BTCUSDT@bybit"}
	a := newScanStub("a")
	b := newScanStub("bybit")
	c := newScanStub("c")
	for _, ex := range []*scanStub{a, b, c} {
		ex.setBBO("BTCUSDT", 100, 100.1, time.Now())
		bL, aL := fillBookAtSpread(100.05, 100)
		ex.setDepth("BTCUSDT", bL, aL)
	}
	exchanges := map[string]exchange.Exchange{"a": a, "bybit": b, "c": c}
	reg := &fakeRegistryReader{}
	tele := &fakeTele{}
	sc := NewScanner(cfg, reg, exchanges, tele, utils.NewLogger("t"))

	// Use a non-blackout minute (:40) by injecting nowFunc explicitly so the
	// blackout gate can't preempt the denylist gate ordering verification.
	sc.nowFunc = func() time.Time { return nonBlackout(time.Now()) }
	sc.RunCycle(context.Background(), nonBlackout(time.Now()))

	last := tele.cycles[len(tele.cycles)-1]
	denylistHits := 0
	nonHits := 0
	for _, r := range last.Records {
		touchesBybit := r.LongExch == "bybit" || r.ShortExch == "bybit"
		if touchesBybit {
			if r.WhyRejected != ReasonDenylist {
				t.Errorf("bybit-tuple %s→%s WhyRejected=%q want denylist", r.LongExch, r.ShortExch, r.WhyRejected)
			}
			denylistHits++
		} else {
			if r.WhyRejected == ReasonDenylist {
				t.Errorf("non-bybit %s→%s should NOT trigger denylist", r.LongExch, r.ShortExch)
			}
			nonHits++
		}
	}
	if denylistHits == 0 || nonHits == 0 {
		t.Fatalf("expected mixed denylist + non-denylist records; got hits=%d non=%d", denylistHits, nonHits)
	}
}

// Test 8 — Bybit blackout: during :04..:05:30 the bybit-leg sample is
// short-circuited and the cross-pair scores 0 with reason=bybit_blackout.
// Other exchanges in the cross product still scan normally.
func TestScanner_BybitBlackout(t *testing.T) {
	cfg := newScannerCfg()
	a := newScanStub("a")
	b := newScanStub("bybit")
	c := newScanStub("c")
	for _, ex := range []*scanStub{a, b, c} {
		ex.setBBO("BTCUSDT", 100, 100.1, time.Now())
		bL, aL := fillBookAtSpread(100.05, 100)
		ex.setDepth("BTCUSDT", bL, aL)
	}
	exchanges := map[string]exchange.Exchange{"a": a, "bybit": b, "c": c}
	tele := &fakeTele{}
	sc := NewScanner(cfg, &fakeRegistryReader{}, exchanges, tele, utils.NewLogger("t"))

	// 12:04:30 UTC — inside the :04..:05:30 window.
	now := time.Date(2026, 4, 28, 12, 4, 30, 0, time.UTC)
	sc.RunCycle(context.Background(), now)

	last := tele.cycles[len(tele.cycles)-1]
	for _, r := range last.Records {
		touchesBybit := r.LongExch == "bybit" || r.ShortExch == "bybit"
		if touchesBybit && r.WhyRejected != ReasonBybitBlackout {
			t.Errorf("bybit-leg %s→%s WhyRejected=%q want bybit_blackout", r.LongExch, r.ShortExch, r.WhyRejected)
		}
		if !touchesBybit && r.WhyRejected == ReasonBybitBlackout {
			t.Errorf("non-bybit %s→%s should not be blackout-rejected", r.LongExch, r.ShortExch)
		}
	}
}

// Test 9 — singleton silent skip (D-08): a symbol listed on only one
// exchange emits no per-symbol record and one WriteSymbolNoCrossPair call.
func TestScanner_SingletonSilentSkip(t *testing.T) {
	cfg := newScannerCfg()
	cfg.PriceGapDiscoveryUniverse = []string{"SOMETHINGUSDT"}
	a := newScanStub("a")
	b := newScanStub("b")
	a.setBBO("SOMETHINGUSDT", 1, 1.001, time.Now())
	bL, aL := fillBookAtSpread(1.0005, 1000)
	a.setDepth("SOMETHINGUSDT", bL, aL)
	exchanges := map[string]exchange.Exchange{"a": a, "b": b}
	tele := &fakeTele{}
	sc := NewScanner(cfg, &fakeRegistryReader{}, exchanges, tele, utils.NewLogger("t"))

	sc.RunCycle(context.Background(), nonBlackout(time.Now()))

	if tele.noCrossPair != 1 {
		t.Fatalf("noCrossPair count=%d want 1", tele.noCrossPair)
	}
	if len(tele.cycles[0].Records) != 0 {
		t.Fatalf("expected zero per-symbol records; got %d", len(tele.cycles[0].Records))
	}
}

// Test 10 — gate-then-magnitude: 4-bar persistence + freshness OK + depth
// above floor → score in [1,100] AND sub-scores populated.
func TestScanner_GateThenMagnitude(t *testing.T) {
	// Long mid 100, short mid 102 → spread = (100-102)/101*10000 ≈ -198 bps.
	// Bidirectional fires on either sign; default candidate direction is
	// pinned, but the scanner's persistence gate uses Bidirectional so a
	// negative chain still counts.
	sc, tele, _, _, _ := scenarioTwoLeg(100, 100.1, 102, 102.1, 100)
	sc.cfg.PriceGapDiscoveryThresholdBps = 100
	start := nonBlackout(time.Now())

	runCycles(sc, context.Background(), 4, start)

	last := tele.cycles[len(tele.cycles)-1]
	rec := findRec(last.Records, "BTCUSDT", "binance", "bybit")
	if rec == nil {
		t.Fatalf("no record")
	}
	if rec.WhyRejected != ReasonAccepted {
		t.Fatalf("WhyRejected=%q want accepted; (rec=%+v)", rec.WhyRejected, rec)
	}
	if rec.Score < 1 || rec.Score > 100 {
		t.Fatalf("Score=%d outside [1,100]", rec.Score)
	}
	if rec.PersistenceBars < 4 {
		t.Fatalf("PersistenceBars=%d want >=4", rec.PersistenceBars)
	}
	if rec.SpreadBps == 0 {
		t.Fatalf("SpreadBps=0 — should be ~|198|")
	}
	if rec.DepthScore <= 0 {
		t.Fatalf("DepthScore=%v want >0", rec.DepthScore)
	}
	if len(rec.GatesPassed) == 0 {
		t.Fatalf("GatesPassed empty")
	}
}

// Test 11 — Pitfall 1: a 1-bar pump on a thin pair is REJECTED.
// Same fixture as Test 3 but only one cycle — score must be 0 with
// reason=insufficient_persistence.
func TestScanner_OneBarPumpRejected(t *testing.T) {
	sc, tele, _, _, _ := scenarioTwoLeg(100, 100.1, 105, 105.1, 100) // 500bps pump
	sc.RunCycle(context.Background(), nonBlackout(time.Now()))

	last := tele.cycles[len(tele.cycles)-1]
	rec := findRec(last.Records, "BTCUSDT", "binance", "bybit")
	if rec == nil {
		t.Fatalf("no record")
	}
	if rec.Score != 0 || rec.WhyRejected != ReasonInsufficientPersistence {
		t.Fatalf("Score=%d WhyRejected=%q want 0 + insufficient_persistence",
			rec.Score, rec.WhyRejected)
	}
}

// Test 12 — canonical symbol join: BBOs published under canonical "BTCUSDT"
// are joined across multiple exchanges with no normalization loss.
func TestScanner_CanonicalSymbolJoin(t *testing.T) {
	sc, tele, _, _, _ := scenarioTwoLeg(100, 100.1, 102, 102.1, 100)
	sc.RunCycle(context.Background(), nonBlackout(time.Now()))

	last := tele.cycles[len(tele.cycles)-1]
	if len(last.Records) == 0 {
		t.Fatalf("no records — canonical join failed")
	}
	for _, r := range last.Records {
		if r.Symbol != "BTCUSDT" {
			t.Errorf("record symbol=%q want BTCUSDT (canonical)", r.Symbol)
		}
	}
}

// Test 14 — cycle error budget: 5 consecutive sample errors abort the
// remainder of that cycle and increment cycleFailed.
func TestScanner_CycleErrorBudget(t *testing.T) {
	cfg := newScannerCfg()
	// 6 exchanges so we can guarantee >5 cross-pairs to trip the budget.
	stubs := map[string]*scanStub{}
	for _, n := range []string{"a", "b", "c", "d", "e", "f"} {
		s := newScanStub(n)
		// NOTE: deliberately do NOT set BBO — every GetBBO returns ok=false,
		// so listingExchanges() will mark all as not-listing, triggering
		// the singleton skip path. To exercise the error budget we set BBO
		// but NOT depth on most legs so depth-probe fails are sample errors
		// (a fall-through of the GetDepth=false path).
		s.setBBO("BTCUSDT", 100, 100.1, time.Now())
		// no setDepth — GetDepth returns false
		stubs[n] = s
	}
	exchanges := map[string]exchange.Exchange{}
	for k, v := range stubs {
		exchanges[k] = v
	}
	tele := &fakeTele{}
	sc := NewScanner(cfg, &fakeRegistryReader{}, exchanges, tele, utils.NewLogger("t"))
	sc.RunCycle(context.Background(), nonBlackout(time.Now()))

	if tele.cycleFailed == 0 {
		t.Fatalf("expected at least one WriteCycleFailed; got 0 (records=%d)",
			len(tele.cycles[0].Records))
	}
}

// Test 15 — sub-score record persistence (D-03): an accepted candidate's
// CycleRecord populates the 7 documented sub-score fields.
func TestScanner_SubScoreRecord(t *testing.T) {
	sc, tele, _, _, _ := scenarioTwoLeg(100, 100.1, 102, 102.1, 100)
	start := nonBlackout(time.Now())
	runCycles(sc, context.Background(), 4, start)

	last := tele.cycles[len(tele.cycles)-1]
	rec := findRec(last.Records, "BTCUSDT", "binance", "bybit")
	if rec == nil {
		t.Fatalf("no record")
	}

	// All 7 keys present (per PLAN behavior block):
	//   spread_bps, persistence_bars, depth_score, freshness_age_s,
	//   funding_bps_h, gates_passed, gates_failed
	if rec.SpreadBps == 0 {
		t.Errorf("SpreadBps unset")
	}
	if rec.PersistenceBars == 0 {
		t.Errorf("PersistenceBars unset")
	}
	if rec.DepthScore == 0 {
		t.Errorf("DepthScore unset")
	}
	// FreshnessAgeSec may legitimately be 0 on the very first sample, but
	// after 4 ticks at 1m apart it is ≥60s.
	if rec.FreshnessAgeSec < 60 {
		t.Errorf("FreshnessAgeSec=%d want >=60", rec.FreshnessAgeSec)
	}
	// FundingBpsPerHr may be 0 (no funding cache wired in test) — still a
	// "populated" field; we just check the field exists by reading it.
	_ = rec.FundingBpsPerHr
	if rec.GatesPassed == nil {
		t.Errorf("GatesPassed nil")
	}
	if rec.GatesFailed == nil {
		t.Errorf("GatesFailed nil")
	}
}

// findRec is a small helper for table assertions.
func findRec(recs []CycleRecord, sym, longEx, shortEx string) *CycleRecord {
	for i := range recs {
		r := &recs[i]
		if r.Symbol == sym && r.LongExch == longEx && r.ShortExch == shortEx {
			return r
		}
	}
	return nil
}
