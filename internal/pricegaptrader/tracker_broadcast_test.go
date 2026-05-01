package pricegaptrader

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
)

// ---- Spy Broadcaster + Notifier ---------------------------------------------

// spyCall records an annotated call for deterministic test assertions.
// Kind encodes the method ("pg_event:entry", "pg_positions:N", "notify_entry",
// "notify_exit:reason", "notify_risk:gate"). Args are method-specific.
type spyCall struct {
	Kind string
	Args any
}

// spyBroadcaster implements pricegaptrader.Broadcaster, recording every call.
type spyBroadcaster struct {
	mu    sync.Mutex
	calls []spyCall
}

func newSpyBroadcaster() *spyBroadcaster { return &spyBroadcaster{} }

func (s *spyBroadcaster) BroadcastPriceGapPositions(positions []*models.PriceGapPosition) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, spyCall{Kind: "pg_positions", Args: len(positions)})
}

func (s *spyBroadcaster) BroadcastPriceGapEvent(evt PriceGapEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, spyCall{Kind: "pg_event:" + evt.Type, Args: evt})
}

func (s *spyBroadcaster) BroadcastPriceGapCandidateUpdate(upd PriceGapCandidateUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, spyCall{Kind: "pg_candidate_update", Args: upd})
}

func (s *spyBroadcaster) countByKind(prefix string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, c := range s.calls {
		if len(c.Kind) >= len(prefix) && c.Kind[:len(prefix)] == prefix {
			n++
		}
	}
	return n
}

func (s *spyBroadcaster) snapshot() []spyCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]spyCall, len(s.calls))
	copy(out, s.calls)
	return out
}

// spyNotifier implements pricegaptrader.PriceGapNotifier, recording every call.
type spyNotifier struct {
	mu    sync.Mutex
	calls []spyCall
}

func newSpyNotifier() *spyNotifier { return &spyNotifier{} }

func (n *spyNotifier) NotifyPriceGapEntry(pos *models.PriceGapPosition) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, spyCall{Kind: "notify_entry", Args: pos})
}

func (n *spyNotifier) NotifyPriceGapExit(pos *models.PriceGapPosition, reason string, pnl float64, duration time.Duration) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, spyCall{Kind: "notify_exit", Args: reason})
}

func (n *spyNotifier) NotifyPriceGapRiskBlock(symbol, gate, detail string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, spyCall{Kind: "notify_risk:" + gate, Args: symbol})
}

// Phase 14 PG-LIVE-03: extend spyNotifier with reconcile-related methods
// so it continues to satisfy the widened PriceGapNotifier interface.
// Tests in this file do not exercise reconcile dispatch; the methods are
// here purely for compile-time conformance.
func (n *spyNotifier) NotifyPriceGapDailyDigest(date string, _ DailyReconcileRecord, _ models.RampState) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, spyCall{Kind: "notify_daily_digest", Args: date})
}

func (n *spyNotifier) NotifyPriceGapReconcileFailure(date string, err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	args := date
	if err != nil {
		args = date + ":" + err.Error()
	}
	n.calls = append(n.calls, spyCall{Kind: "notify_reconcile_failure", Args: args})
}

// Phase 14 Plan 14-04: ramp surfaces for compile-time PriceGapNotifier conformance.
func (n *spyNotifier) NotifyPriceGapRampDemote(prior, next int, reason string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, spyCall{Kind: "notify_ramp_demote", Args: fmt.Sprintf("%d->%d:%s", prior, next, reason)})
}

func (n *spyNotifier) NotifyPriceGapRampForceOp(action string, prior, next int, operator, reason string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, spyCall{Kind: "notify_ramp_force_op:" + action, Args: fmt.Sprintf("%d->%d:%s:%s", prior, next, operator, reason)})
}

// Phase 15 Plan 15-03 — breaker surfaces for compile-time PriceGapNotifier conformance.
func (n *spyNotifier) NotifyPriceGapBreakerTrip(record models.BreakerTripRecord) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, spyCall{Kind: "notify_breaker_trip", Args: fmt.Sprintf("pnl=%.2f", record.TripPnLUSDT)})
	return nil
}

func (n *spyNotifier) NotifyPriceGapBreakerRecovery(record models.BreakerTripRecord, operator string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, spyCall{Kind: "notify_breaker_recovery", Args: operator})
	return nil
}

func (n *spyNotifier) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.calls)
}

func (n *spyNotifier) countKind(kind string) int {
	n.mu.Lock()
	defer n.mu.Unlock()
	c := 0
	for _, call := range n.calls {
		if call.Kind == kind {
			c++
		}
	}
	return c
}

// ---- Fixtures ---------------------------------------------------------------

func newHookTracker(t *testing.T, paper bool) (*Tracker, *stubExchange, *stubExchange, *fakeStore, *spyBroadcaster, *spyNotifier) {
	t.Helper()
	longEx := newStubExchange("binance")
	shortEx := newStubExchange("gate")
	store := newFakeStore()
	cfg := &config.Config{
		PriceGapEnabled:              true,
		PriceGapBudget:               5000,
		PriceGapMaxConcurrent:        3,
		PriceGapGateConcentrationPct: 0.5,
		PriceGapKlineStalenessSec:    90,
		PriceGapPollIntervalSec:      30,
		PriceGapMaxHoldMin:           240,
		PriceGapExitReversionFactor:  0.5,
		PriceGapPaperMode:            paper,
	}
	exch := map[string]exchange.Exchange{"binance": longEx, "gate": shortEx}
	tr := NewTracker(exch, store, newFakeDelistChecker(), cfg)
	bc := newSpyBroadcaster()
	nf := newSpyNotifier()
	tr.SetBroadcaster(bc)
	tr.SetNotifier(nf)
	return tr, longEx, shortEx, store, bc, nf
}

func hookCand() models.PriceGapCandidate {
	return models.PriceGapCandidate{
		Symbol:             "SOON",
		LongExch:           "binance",
		ShortExch:          "gate",
		ThresholdBps:       200,
		MaxPositionUSDT:    5000,
		ModeledSlippageBps: 7.5,
	}
}

func hookDet() DetectionResult {
	return DetectionResult{
		Fired:        true,
		SpreadBps:    250,
		MidLong:      1.00,
		MidShort:     1.025,
		StalenessSec: 1,
	}
}

// ---- Task 1 tests -----------------------------------------------------------

// TestHook_OpenPair_FiresEntryEvent: successful openPair fires pg_event:entry.
func TestHook_OpenPair_FiresEntryEvent(t *testing.T) {
	tr, longEx, shortEx, _, bc, _ := newHookTracker(t, false)
	longEx.queueFill(100, 1.00, nil)
	shortEx.queueFill(100, 1.025, nil)

	pos, err := tr.openPair(hookCand(), 100, hookDet())
	if err != nil || pos == nil {
		t.Fatalf("openPair failed: err=%v pos=%v", err, pos)
	}
	if n := bc.countByKind("pg_event:entry"); n != 1 {
		t.Fatalf("pg_event:entry count = %d, want 1", n)
	}
	// And pg_positions fires (unthrottled because first call).
	if n := bc.countByKind("pg_positions"); n != 1 {
		t.Fatalf("pg_positions count = %d, want 1", n)
	}
}

// TestHook_OpenPair_FiresTelegramEntry: successful openPair calls NotifyPriceGapEntry.
func TestHook_OpenPair_FiresTelegramEntry(t *testing.T) {
	tr, longEx, shortEx, _, _, nf := newHookTracker(t, false)
	longEx.queueFill(100, 1.00, nil)
	shortEx.queueFill(100, 1.025, nil)

	_, err := tr.openPair(hookCand(), 100, hookDet())
	if err != nil {
		t.Fatalf("openPair: %v", err)
	}
	if n := nf.countKind("notify_entry"); n != 1 {
		t.Fatalf("notify_entry count = %d, want 1", n)
	}
}

// TestHook_ClosePair_FiresExitEvent: closePair fires pg_event:exit and notify_exit.
func TestHook_ClosePair_FiresExitEvent(t *testing.T) {
	tr, longEx, shortEx, _, bc, nf := newHookTracker(t, false)
	longEx.queueFill(100, 1.00, nil)
	shortEx.queueFill(100, 1.025, nil)
	pos, err := tr.openPair(hookCand(), 100, hookDet())
	if err != nil {
		t.Fatalf("openPair: %v", err)
	}
	// Throttle reset + queue close fills.
	tr.positionsBroadcastMu.Lock()
	tr.lastPositionsBroadcast = time.Time{}
	tr.positionsBroadcastMu.Unlock()
	longEx.queueFill(100, 1.00, nil)
	shortEx.queueFill(100, 1.025, nil)

	tr.closePair(pos, models.ExitReasonReverted)

	if n := bc.countByKind("pg_event:exit"); n != 1 {
		t.Fatalf("pg_event:exit count = %d, want 1", n)
	}
	if n := nf.countKind("notify_exit"); n != 1 {
		t.Fatalf("notify_exit count = %d, want 1", n)
	}
}

// TestHook_RiskGateDeny_FiresTelegramBlock: each of 5 Telegram-eligible gates
// (concentration, max_concurrent, kline_stale, delist, budget) denying an
// entry invokes NotifyPriceGapRiskBlock with the right gate name.
func TestHook_RiskGateDeny_FiresTelegramBlock(t *testing.T) {
	tests := []struct {
		name     string
		gate     string
		setup    func(tr *Tracker, store *fakeStore) ([]*models.PriceGapPosition, models.PriceGapCandidate, DetectionResult)
	}{
		{
			name: "exec_quality",
			gate: "exec_quality",
			setup: func(tr *Tracker, store *fakeStore) ([]*models.PriceGapPosition, models.PriceGapCandidate, DetectionResult) {
				store.disabled["SOON"] = "auto-disabled"
				return nil, hookCand(), hookDet()
			},
		},
		{
			name: "max_concurrent",
			gate: "max_concurrent",
			setup: func(tr *Tracker, store *fakeStore) ([]*models.PriceGapPosition, models.PriceGapCandidate, DetectionResult) {
				active := []*models.PriceGapPosition{
					{ID: "a", NotionalUSDT: 100},
					{ID: "b", NotionalUSDT: 100},
					{ID: "c", NotionalUSDT: 100},
				}
				return active, hookCand(), hookDet()
			},
		},
		{
			name: "budget",
			gate: "budget",
			setup: func(tr *Tracker, store *fakeStore) ([]*models.PriceGapPosition, models.PriceGapCandidate, DetectionResult) {
				active := []*models.PriceGapPosition{{ID: "a", NotionalUSDT: 4900}}
				return active, hookCand(), hookDet()
			},
		},
		{
			name: "concentration",
			gate: "concentration",
			setup: func(tr *Tracker, store *fakeStore) ([]*models.PriceGapPosition, models.PriceGapCandidate, DetectionResult) {
				active := []*models.PriceGapPosition{
					{ID: "x", LongExchange: "gate", ShortExchange: "binance", NotionalUSDT: 2400},
				}
				return active, hookCand(), hookDet()
			},
		},
		{
			name: "kline_stale",
			gate: "kline_stale",
			setup: func(tr *Tracker, store *fakeStore) ([]*models.PriceGapPosition, models.PriceGapCandidate, DetectionResult) {
				det := hookDet()
				det.StalenessSec = 120 // > 90s config
				return nil, hookCand(), det
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr, _, _, store, _, nf := newHookTracker(t, false)
			active, cand, det := tc.setup(tr, store)
			res := tr.preEntry(cand, 1000, det, active)
			if res.Approved {
				t.Fatalf("expected denial for gate %q", tc.gate)
			}
			want := "notify_risk:" + tc.gate
			if n := nf.countKind(want); n != 1 {
				t.Fatalf("%s count = %d, want 1 (got calls=%v)", want, n, nf.calls)
			}
		})
	}
}

// TestHook_RiskGateDeny_Delist_FiresTelegramBlock: the delist gate uses the
// shared fakeDelistChecker which the helper does not expose on the returned
// tuple; cover it as a separate test.
func TestHook_RiskGateDeny_Delist_FiresTelegramBlock(t *testing.T) {
	longEx := newStubExchange("binance")
	shortEx := newStubExchange("gate")
	store := newFakeStore()
	delist := newFakeDelistChecker()
	delist.delisted["SOON"] = true
	cfg := &config.Config{
		PriceGapEnabled:              true,
		PriceGapBudget:               5000,
		PriceGapMaxConcurrent:        3,
		PriceGapGateConcentrationPct: 0.5,
		PriceGapKlineStalenessSec:    90,
	}
	exch := map[string]exchange.Exchange{"binance": longEx, "gate": shortEx}
	tr := NewTracker(exch, store, delist, cfg)
	nf := newSpyNotifier()
	tr.SetNotifier(nf)

	res := tr.preEntry(hookCand(), 1000, hookDet(), nil)
	if res.Approved {
		t.Fatalf("delist must block")
	}
	if n := nf.countKind("notify_risk:delist"); n != 1 {
		t.Fatalf("notify_risk:delist count = %d, want 1", n)
	}
}

// TestHook_AutoDisable_FiresEventAndCandidateUpdate: slippage auto-disable
// fires pg_event:auto_disable + pg_candidate_update + notify_risk:exec_quality.
func TestHook_AutoDisable_FiresEventAndCandidateUpdate(t *testing.T) {
	tr, _, _, store, bc, nf := newHookTracker(t, false)

	// Seed 10 slippage samples with realized > 2x modeled to trip the window.
	candID := "SOON_binance_gate"
	for i := 0; i < 10; i++ {
		_ = store.AppendSlippageSample(candID, models.SlippageSample{
			PositionID: "p",
			Realized:   50, // 5x modeled
			Modeled:    10,
			Timestamp:  time.Now(),
		})
	}

	// recordSlippageAndMaybeDisable reads samples, appends one more (sample 11),
	// then disables since window-over-10 mean still >2x modeled.
	pos := &models.PriceGapPosition{
		ID: "pos-1", Symbol: "SOON",
		LongExchange: "binance", ShortExchange: "gate",
		RealizedSlipBps: 50, ModeledSlipBps: 10,
	}
	tr.recordSlippageAndMaybeDisable(pos)

	if n := bc.countByKind("pg_event:auto_disable"); n != 1 {
		t.Fatalf("pg_event:auto_disable count = %d, want 1", n)
	}
	if n := bc.countByKind("pg_candidate_update"); n != 1 {
		t.Fatalf("pg_candidate_update count = %d, want 1", n)
	}
	if n := nf.countKind("notify_risk:exec_quality"); n != 1 {
		t.Fatalf("notify_risk:exec_quality count = %d, want 1", n)
	}
	// And the pg_candidate_update must flag Disabled=true with a non-empty reason.
	found := false
	for _, c := range bc.snapshot() {
		if c.Kind == "pg_candidate_update" {
			upd, ok := c.Args.(PriceGapCandidateUpdate)
			if !ok {
				t.Fatalf("candidate_update Args type = %T", c.Args)
			}
			if !upd.Disabled {
				t.Fatalf("candidate_update.Disabled = false, want true")
			}
			if upd.Symbol != "SOON" {
				t.Fatalf("candidate_update.Symbol = %q, want SOON", upd.Symbol)
			}
			if upd.Reason == "" {
				t.Fatalf("candidate_update.Reason = empty, want non-empty")
			}
			if upd.DisabledAt == 0 {
				t.Fatalf("candidate_update.DisabledAt = 0, want unix timestamp")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("did not find pg_candidate_update event")
	}
}

// TestRehydratePathSilent: spy Broadcaster+Notifier record zero calls after
// rehydrate on a tracker holding 3 active positions (Pitfall 3 / T-09-25).
func TestRehydratePathSilent(t *testing.T) {
	store := newFakeStore()
	store.active = []*models.PriceGapPosition{
		{ID: "r1", Symbol: "AAA", LongExchange: "binance", ShortExchange: "gate", LongSize: 100, ShortSize: 100, OpenedAt: time.Now().Add(-time.Hour), ThresholdBps: 200},
		{ID: "r2", Symbol: "BBB", LongExchange: "binance", ShortExchange: "gate", LongSize: 100, ShortSize: 100, OpenedAt: time.Now().Add(-time.Hour), ThresholdBps: 200},
		{ID: "r3", Symbol: "CCC", LongExchange: "binance", ShortExchange: "gate", LongSize: 100, ShortSize: 100, OpenedAt: time.Now().Add(-time.Hour), ThresholdBps: 200},
	}
	longEx := newStubExchange("binance")
	shortEx := newStubExchange("gate")
	// Make all legs look nonzero on the exchange so rehydrate re-enrolls them.
	for _, sym := range []string{"AAA", "BBB", "CCC"} {
		longEx.positions[sym] = []exchange.Position{{Symbol: sym, Total: "100"}}
		shortEx.positions[sym] = []exchange.Position{{Symbol: sym, Total: "100"}}
	}
	cfg := &config.Config{
		PriceGapEnabled:         true,
		PriceGapPollIntervalSec: 30,
	}
	tr := NewTracker(
		map[string]exchange.Exchange{"binance": longEx, "gate": shortEx},
		store,
		newFakeDelistChecker(),
		cfg,
	)
	bc := newSpyBroadcaster()
	nf := newSpyNotifier()
	tr.SetBroadcaster(bc)
	tr.SetNotifier(nf)

	tr.rehydrate()

	if n := len(bc.snapshot()); n != 0 {
		t.Fatalf("rehydrate MUST NOT broadcast; got %d calls: %v", n, bc.snapshot())
	}
	if n := nf.count(); n != 0 {
		t.Fatalf("rehydrate MUST NOT notify; got %d calls", n)
	}

	// Clean up monitors spawned by rehydrate.
	tr.monMu.Lock()
	for _, c := range tr.monitors {
		c()
	}
	tr.monMu.Unlock()
}

// TestHook_PaperMode_AlsoFires: paper-mode entry/exit hooks fire identically
// to live (D-22 parity). Paper mode is set via config; placeLeg synthesizes
// fills so no PlaceOrder is required — makes this independent of stub scripts.
func TestHook_PaperMode_AlsoFires(t *testing.T) {
	tr, longEx, shortEx, _, bc, nf := newHookTracker(t, true)

	pos, err := tr.openPair(hookCand(), 100, hookDet())
	if err != nil || pos == nil {
		t.Fatalf("paper openPair failed: err=%v", err)
	}
	if pos.Mode != models.PriceGapModePaper {
		t.Fatalf("Mode = %q, want paper", pos.Mode)
	}
	if n := bc.countByKind("pg_event:entry"); n != 1 {
		t.Fatalf("paper pg_event:entry count = %d, want 1", n)
	}
	if n := nf.countKind("notify_entry"); n != 1 {
		t.Fatalf("paper notify_entry count = %d, want 1", n)
	}

	// Close: reset throttle so pg_positions fires again for the exit.
	tr.positionsBroadcastMu.Lock()
	tr.lastPositionsBroadcast = time.Time{}
	tr.positionsBroadcastMu.Unlock()
	tr.closePair(pos, models.ExitReasonMaxHold)

	if n := bc.countByKind("pg_event:exit"); n != 1 {
		t.Fatalf("paper pg_event:exit count = %d, want 1", n)
	}
	if n := nf.countKind("notify_exit"); n != 1 {
		t.Fatalf("paper notify_exit count = %d, want 1", n)
	}

	// Paper placed no real PlaceOrder — stubs recorded zero orders.
	if n := len(longEx.placedOrders()); n != 0 {
		t.Fatalf("paper mode long orders = %d, want 0", n)
	}
	if n := len(shortEx.placedOrders()); n != 0 {
		t.Fatalf("paper mode short orders = %d, want 0", n)
	}
}

// TestHook_NilNotifierSafe: SetNotifier(nil) reverts to NoopNotifier so a hook
// invocation does not panic.
func TestHook_NilNotifierSafe(t *testing.T) {
	tr, longEx, shortEx, _, _, _ := newHookTracker(t, false)
	tr.SetNotifier(nil) // should revert to NoopNotifier
	longEx.queueFill(100, 1.00, nil)
	shortEx.queueFill(100, 1.025, nil)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil-notifier hook panicked: %v", r)
		}
	}()
	_, err := tr.openPair(hookCand(), 100, hookDet())
	if err != nil {
		t.Fatalf("openPair: %v", err)
	}
}

// ---- Task 2 tests -----------------------------------------------------------

// TestHeartbeat_FiresOnEntry: entry triggers a pg_positions broadcast.
func TestHeartbeat_FiresOnEntry(t *testing.T) {
	tr, longEx, shortEx, _, bc, _ := newHookTracker(t, false)
	longEx.queueFill(100, 1.00, nil)
	shortEx.queueFill(100, 1.025, nil)

	_, err := tr.openPair(hookCand(), 100, hookDet())
	if err != nil {
		t.Fatalf("openPair: %v", err)
	}
	if n := bc.countByKind("pg_positions"); n != 1 {
		t.Fatalf("pg_positions count = %d, want 1 after entry", n)
	}
}

// TestHeartbeat_Throttled2s: second entry within 2s does NOT fire a second
// pg_positions (but pg_event:entry still fires unthrottled).
func TestHeartbeat_Throttled2s(t *testing.T) {
	tr, _, _, _, bc, _ := newHookTracker(t, true) // paper so stub fills aren't exhausted
	_, err := tr.openPair(hookCand(), 100, hookDet())
	if err != nil {
		t.Fatalf("first openPair: %v", err)
	}
	// Second entry inside throttle window.
	cand2 := hookCand()
	cand2.Symbol = "MOON" // different symbol to avoid lock collision
	_, err = tr.openPair(cand2, 100, hookDet())
	if err != nil {
		t.Fatalf("second openPair: %v", err)
	}

	if n := bc.countByKind("pg_event:entry"); n != 2 {
		t.Fatalf("pg_event:entry count = %d, want 2 (unthrottled)", n)
	}
	if n := bc.countByKind("pg_positions"); n != 1 {
		t.Fatalf("pg_positions count = %d, want 1 (throttled within 2s)", n)
	}

	// Force the throttle window to elapse and manually invoke: fires again.
	tr.positionsBroadcastMu.Lock()
	tr.lastPositionsBroadcast = time.Time{}
	tr.positionsBroadcastMu.Unlock()
	tr.maybeBroadcastPositions()
	if n := bc.countByKind("pg_positions"); n != 2 {
		t.Fatalf("pg_positions count = %d, want 2 after throttle reset", n)
	}

	// Use the unused suppress var to silence lint.
	_ = errors.New("unused placeholder")
}
