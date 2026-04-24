package pricegaptrader

import (
	"strings"
	"sync"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// ============================================================================
// Plan 09-10 — Gap #2 closure: Tracker.Start() subscribes each configured
// candidate on both leg exchanges, fail-soft per-leg, fail-loud on post-grace
// BBO silence.
//
// Tests:
//   Task 1 — subscribeCandidates fan-out + fail-soft
//     T1.1 FanOut:            Start() calls SubscribeSymbol exactly once per (leg, symbol)
//     T1.2 UnknownExchange:   missing exchange key -> WARN + continue; other legs still subscribed
//     T1.3 SubscribeFalse:    SubscribeSymbol returns false -> WARN + continue; other candidates ok
//     T1.4 EmptyCandidates:   zero candidates -> zero calls
//     T1.5 RestartIdempotent: Stop + new Tracker (fresh stopCh) re-subscribes cleanly
//     T1.6 SameExchBothLegs:  LongExch == ShortExch -> subscribed twice on same exchange
//
//   Task 2 — Startup BBO-liveness assertion
//     T2.1 BBOLive:           all legs return bboOK=true → no WARN after grace
//     T2.2 BBODead:           one leg bboOK=false → WARN naming (symbol, exchange)
//     T2.3 StopBeforeGrace:   Stop() during grace → goroutine exits cleanly (no leak under -race)
// ============================================================================

// logCapture is a utils.Logger writer substitute: we hook via a shared string
// buffer the Logger's underlying io.Writer path. Since utils.Logger writes to
// both stdout and a log file, we can't trivially intercept; instead we assert
// subscription behavior via the per-symbol stubExchange counter and assert
// WARN-path side-effects via the "no subscription attempt on missing leg"
// invariant, which is observable without capturing the log stream.
//
// For explicit WARN-message coverage we fall back to calling subscribeOneLeg /
// checkOneLegBBO directly against a tracker whose exchanges map is crafted to
// hit each branch.

func subTestCfg(candidates []models.PriceGapCandidate) *config.Config {
	return &config.Config{
		PriceGapEnabled:              true,
		PriceGapBudget:               5000,
		PriceGapMaxConcurrent:        3,
		PriceGapGateConcentrationPct: 0.5,
		PriceGapKlineStalenessSec:    90,
		PriceGapPollIntervalSec:      3600, // long, so tickLoop does nothing during tests
		PriceGapMaxHoldMin:           240,
		PriceGapExitReversionFactor:  0.5,
		PriceGapBarPersistence:       4,
		PriceGapCandidates:           candidates,
	}
}

func subTestCand(sym, longEx, shortEx string) models.PriceGapCandidate {
	return models.PriceGapCandidate{
		Symbol:             sym,
		LongExch:           longEx,
		ShortExch:          shortEx,
		ThresholdBps:       200,
		MaxPositionUSDT:    1000,
		ModeledSlippageBps: 7.5,
	}
}

func newSubTracker(t *testing.T, exch map[string]exchange.Exchange, cfg *config.Config) *Tracker {
	t.Helper()
	db, _ := newE2EClient(t)
	return NewTracker(exch, db, newFakeDelistChecker(), cfg)
}

// ----------------------------------------------------------------------------
// Task 1 tests
// ----------------------------------------------------------------------------

// T1.1 FanOut — 2 candidates, 3 exchanges (one shared leg); Start() fans out
// SubscribeSymbol exactly once per (exchange, symbol) tuple.
func TestSubscribeCandidates_FanOut(t *testing.T) {
	bin := newStubExchange("binance")
	byb := newStubExchange("bybit")
	gio := newStubExchange("gateio")
	exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb, "gateio": gio}

	cA := subTestCand("SOONUSDT", "binance", "bybit")
	cB := subTestCand("VICUSDT", "binance", "gateio")
	cfg := subTestCfg([]models.PriceGapCandidate{cA, cB})
	tr := newSubTracker(t, exch, cfg)

	tr.subscribeCandidates()

	// Expectations:
	//   binance: SOONUSDT=1, VICUSDT=1
	//   bybit:   SOONUSDT=1
	//   gateio:  VICUSDT=1
	if got := bin.subscribeCount("SOONUSDT"); got != 1 {
		t.Fatalf("binance SOONUSDT subscribe count = %d, want 1", got)
	}
	if got := bin.subscribeCount("VICUSDT"); got != 1 {
		t.Fatalf("binance VICUSDT subscribe count = %d, want 1", got)
	}
	if got := byb.subscribeCount("SOONUSDT"); got != 1 {
		t.Fatalf("bybit SOONUSDT subscribe count = %d, want 1", got)
	}
	if got := byb.subscribeCount("VICUSDT"); got != 0 {
		t.Fatalf("bybit VICUSDT subscribe count = %d, want 0 (not a bybit leg)", got)
	}
	if got := gio.subscribeCount("VICUSDT"); got != 1 {
		t.Fatalf("gateio VICUSDT subscribe count = %d, want 1", got)
	}
	if got := gio.subscribeCount("SOONUSDT"); got != 0 {
		t.Fatalf("gateio SOONUSDT subscribe count = %d, want 0 (not a gateio leg)", got)
	}
}

// T1.2 UnknownExchange — one candidate references an exchange missing from the
// exchanges map; subscribeCandidates must NOT panic, must subscribe the present
// leg, and must skip the missing one.
func TestSubscribeCandidates_UnknownExchange(t *testing.T) {
	bin := newStubExchange("binance")
	// No "bybit" in the map — candidate references it as ShortExch.
	exch := map[string]exchange.Exchange{"binance": bin}

	cand := subTestCand("SOONUSDT", "binance", "bybit")
	cfg := subTestCfg([]models.PriceGapCandidate{cand})
	tr := newSubTracker(t, exch, cfg)

	// Must not panic.
	tr.subscribeCandidates()

	if got := bin.subscribeCount("SOONUSDT"); got != 1 {
		t.Fatalf("binance SOONUSDT subscribe count = %d, want 1 (present leg)", got)
	}
	// No way to check bybit — it doesn't exist. The invariant is "no panic and
	// the present leg was still subscribed".
}

// T1.3 SubscribeFalse — SubscribeSymbol returns false on one leg; tracker must
// log-and-continue rather than abort; second candidate must still be processed.
func TestSubscribeCandidates_SubscribeFalse(t *testing.T) {
	bin := newStubExchange("binance")
	byb := newStubExchange("bybit")
	// Configure bybit to reject subscriptions.
	byb.subscribeFn = func(symbol string) bool { return false }
	exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}

	cA := subTestCand("SOONUSDT", "binance", "bybit")
	cB := subTestCand("VICUSDT", "binance", "bybit")
	cfg := subTestCfg([]models.PriceGapCandidate{cA, cB})
	tr := newSubTracker(t, exch, cfg)

	tr.subscribeCandidates()

	// Bybit's subscribeFn still records the call even when returning false.
	if got := byb.subscribeCount("SOONUSDT"); got != 1 {
		t.Fatalf("bybit SOONUSDT attempt count = %d, want 1", got)
	}
	if got := byb.subscribeCount("VICUSDT"); got != 1 {
		t.Fatalf("bybit VICUSDT attempt count = %d, want 1 (second cand still attempted)", got)
	}
	if got := bin.subscribeCount("SOONUSDT"); got != 1 {
		t.Fatalf("binance SOONUSDT subscribe count = %d, want 1", got)
	}
	if got := bin.subscribeCount("VICUSDT"); got != 1 {
		t.Fatalf("binance VICUSDT subscribe count = %d, want 1", got)
	}
}

// T1.4 EmptyCandidates — zero candidates, zero SubscribeSymbol calls.
func TestSubscribeCandidates_Empty(t *testing.T) {
	bin := newStubExchange("binance")
	exch := map[string]exchange.Exchange{"binance": bin}
	cfg := subTestCfg(nil)
	tr := newSubTracker(t, exch, cfg)

	tr.subscribeCandidates()

	if got := len(bin.subs); got != 0 {
		t.Fatalf("expected 0 subscribe calls on empty candidates, got %d", got)
	}
}

// T1.5 RestartIdempotent — Start() then Stop() then build a fresh tracker
// (same exchanges) and Start() again; the second Start() re-issues subscriptions.
// Full Start→Stop→Start on one Tracker isn't safe because stopCh is single-use;
// what we assert is the idempotent adapter-layer contract: calling
// subscribeCandidates twice on the same exchanges doubles the count without
// panicking or leaking state (the adapter, not the tracker, owns dedup).
func TestSubscribeCandidates_RestartIdempotent(t *testing.T) {
	bin := newStubExchange("binance")
	byb := newStubExchange("bybit")
	exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}

	cand := subTestCand("SOONUSDT", "binance", "bybit")
	cfg := subTestCfg([]models.PriceGapCandidate{cand})
	tr := newSubTracker(t, exch, cfg)

	tr.subscribeCandidates()
	tr.subscribeCandidates()

	// 2 calls each — stubExchange doesn't dedupe; the real adapter does.
	if got := bin.subscribeCount("SOONUSDT"); got != 2 {
		t.Fatalf("binance SOONUSDT count after 2 Starts = %d, want 2", got)
	}
	if got := byb.subscribeCount("SOONUSDT"); got != 2 {
		t.Fatalf("bybit SOONUSDT count after 2 Starts = %d, want 2", got)
	}
}

// T1.6 SameExchBothLegs — theoretical edge case; SubscribeSymbol is idempotent
// at adapter layer so the tracker making 2 calls is safe.
func TestSubscribeCandidates_SameExchBothLegs(t *testing.T) {
	bin := newStubExchange("binance")
	exch := map[string]exchange.Exchange{"binance": bin}

	cand := subTestCand("SOONUSDT", "binance", "binance")
	cfg := subTestCfg([]models.PriceGapCandidate{cand})
	tr := newSubTracker(t, exch, cfg)

	tr.subscribeCandidates()

	if got := bin.subscribeCount("SOONUSDT"); got != 2 {
		t.Fatalf("binance SOONUSDT count = %d, want 2 (both legs same exchange)", got)
	}
}

// Log-capture helper — utils.Logger writes via its file+stdout sink. For branch
// coverage of WARN lines, we attach a subscriber via the logger's broadcast
// interface.
type captureSubscriber struct {
	mu   sync.Mutex
	logs []string
}

func (c *captureSubscriber) Write(line string) {
	c.mu.Lock()
	c.logs = append(c.logs, line)
	c.mu.Unlock()
}

func (c *captureSubscriber) contains(substr string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, l := range c.logs {
		if strings.Contains(l, substr) {
			return true
		}
	}
	return false
}

// Attach a capture subscriber to the global utils logger broadcaster. The
// Tracker's *utils.Logger emits through the same global broadcaster.
func attachLogCapture(t *testing.T, _ *Tracker) *captureSubscriber {
	t.Helper()
	cap := &captureSubscriber{}
	ch := utils.Subscribe()
	done := make(chan struct{})
	go func() {
		for {
			select {
			case entry, ok := <-ch:
				if !ok {
					return
				}
				cap.Write("[" + entry.Level + "] [" + entry.Module + "] " + entry.Message)
			case <-done:
				return
			}
		}
	}()
	t.Cleanup(func() {
		close(done)
		utils.Unsubscribe(ch)
	})
	return cap
}

// TestSubscribeCandidates_WarnOnMissing — asserts the WARN log line itself.
func TestSubscribeCandidates_WarnOnMissing(t *testing.T) {
	bin := newStubExchange("binance")
	exch := map[string]exchange.Exchange{"binance": bin}

	cand := subTestCand("VICUSDT", "binance", "bybit") // bybit absent
	cfg := subTestCfg([]models.PriceGapCandidate{cand})
	tr := newSubTracker(t, exch, cfg)
	cap := attachLogCapture(t, tr)

	tr.subscribeCandidates()

	// Give the subscribe goroutine a tick to drain the capture channel.
	time.Sleep(20 * time.Millisecond)

	if !cap.contains("pricegap: subscribe") {
		t.Fatalf("expected 'pricegap: subscribe' WARN; logs: %v", cap.logs)
	}
	if !cap.contains("bybit") {
		t.Fatalf("expected missing-exchange WARN to name 'bybit'; logs: %v", cap.logs)
	}
	if !cap.contains("VICUSDT") {
		t.Fatalf("expected missing-exchange WARN to name 'VICUSDT'; logs: %v", cap.logs)
	}
}

// ----------------------------------------------------------------------------
// Task 2 tests — BBO liveness assertion
// ----------------------------------------------------------------------------

// T2.1 BBOLive — all legs have valid BBO; no WARN after grace window.
func TestAssertBBOLiveness_AllLive(t *testing.T) {
	bin := newStubExchange("binance")
	byb := newStubExchange("bybit")
	exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}

	bin.setBBO("SOONUSDT", 0.999, 1.001, time.Now())
	byb.setBBO("SOONUSDT", 0.998, 1.000, time.Now())

	cand := subTestCand("SOONUSDT", "binance", "bybit")
	cfg := subTestCfg([]models.PriceGapCandidate{cand})
	tr := newSubTracker(t, exch, cfg)
	cap := attachLogCapture(t, tr)

	// Run the per-leg check directly (grace elapsed).
	tr.checkOneLegBBO(cand.LongExch, cand.Symbol, "long")
	tr.checkOneLegBBO(cand.ShortExch, cand.Symbol, "short")
	time.Sleep(20 * time.Millisecond)

	if cap.contains("BBO NOT LIVE") {
		t.Fatalf("unexpected BBO NOT LIVE WARN; logs: %v", cap.logs)
	}
}

// T2.2 BBODead — one leg returns ok=false on GetBBO; WARN names the (exchange, symbol).
func TestAssertBBOLiveness_DeadLeg(t *testing.T) {
	bin := newStubExchange("binance")
	byb := newStubExchange("bybit")
	exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}

	// Only bin has a BBO; byb is silent.
	bin.setBBO("SOONUSDT", 0.999, 1.001, time.Now())

	cand := subTestCand("SOONUSDT", "binance", "bybit")
	cfg := subTestCfg([]models.PriceGapCandidate{cand})
	tr := newSubTracker(t, exch, cfg)
	cap := attachLogCapture(t, tr)

	tr.checkOneLegBBO(cand.LongExch, cand.Symbol, "long")
	tr.checkOneLegBBO(cand.ShortExch, cand.Symbol, "short")
	time.Sleep(20 * time.Millisecond)

	if !cap.contains("BBO NOT LIVE") {
		t.Fatalf("expected BBO NOT LIVE WARN for bybit leg; logs: %v", cap.logs)
	}
	if !cap.contains("bybit") {
		t.Fatalf("expected WARN to name 'bybit'; logs: %v", cap.logs)
	}
	if !cap.contains("SOONUSDT") {
		t.Fatalf("expected WARN to name 'SOONUSDT'; logs: %v", cap.logs)
	}
}

// T2.3 StopBeforeGrace — Stop() mid-grace; goroutine exits cleanly.
// We run the full assertBBOLiveness goroutine and then Stop(); wg.Wait in Stop
// must return promptly (<1s) rather than blocking for the full grace period.
func TestAssertBBOLiveness_StopBeforeGrace(t *testing.T) {
	bin := newStubExchange("binance")
	byb := newStubExchange("bybit")
	exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}

	cand := subTestCand("SOONUSDT", "binance", "bybit")
	cfg := subTestCfg([]models.PriceGapCandidate{cand})
	tr := newSubTracker(t, exch, cfg)

	// Launch the assertion goroutine directly. It must honor stopCh.
	tr.wg.Add(1)
	go tr.assertBBOLiveness()

	// Close stopCh well before the 15s grace — the goroutine must return.
	close(tr.stopCh)

	done := make(chan struct{})
	go func() {
		tr.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// ok — goroutine exited promptly.
	case <-time.After(2 * time.Second):
		t.Fatalf("assertBBOLiveness did not exit within 2s of Stop()")
	}
}
