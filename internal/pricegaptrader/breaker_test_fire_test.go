package pricegaptrader

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"

	"arb/internal/models"
	"arb/pkg/utils"
)

// ---- Extended fakes for Plan 15-04 (TestFire + Recover) -----------------

// fakeBreakerStoreFull adds AppendBreakerTrip + UpdateBreakerTripRecovery +
// LoadBreakerTripAt to fakeBreakerStore so the Recover path can exercise the
// trip-log LSet backfill.
type fakeBreakerStoreFull struct {
	*fakeBreakerStore
	updateCalls  []updateRecoveryCall
	tripsAtIdx   map[int64]models.BreakerTripRecord
	updateErr    error
	loadAtErr    error
}

type updateRecoveryCall struct {
	index      int64
	recoveryTs int64
	operator   string
}

func (f *fakeBreakerStoreFull) UpdateBreakerTripRecovery(index int64, recoveryTs int64, operator string) error {
	f.fakeBreakerStore.mu.Lock()
	defer f.fakeBreakerStore.mu.Unlock()
	f.updateCalls = append(f.updateCalls, updateRecoveryCall{index, recoveryTs, operator})
	if f.updateErr != nil {
		return f.updateErr
	}
	if rec, ok := f.tripsAtIdx[index]; ok {
		rec.RecoveryTs = &recoveryTs
		rec.RecoveryOperator = &operator
		f.tripsAtIdx[index] = rec
	}
	return nil
}

func (f *fakeBreakerStoreFull) LoadBreakerTripAt(index int64) (models.BreakerTripRecord, bool, error) {
	f.fakeBreakerStore.mu.Lock()
	defer f.fakeBreakerStore.mu.Unlock()
	if f.loadAtErr != nil {
		return models.BreakerTripRecord{}, false, f.loadAtErr
	}
	if rec, ok := f.tripsAtIdx[index]; ok {
		return rec, true, nil
	}
	return models.BreakerTripRecord{}, false, nil
}

// fakeBreakerNotifierFull adds NotifyPriceGapBreakerRecovery.
type fakeBreakerNotifierFull struct {
	*fakeBreakerNotifier
	recoveryCalls []recoveryCall
	recoveryMu    sync.Mutex
	recoveryErr   error
}

type recoveryCall struct {
	record   models.BreakerTripRecord
	operator string
}

func (f *fakeBreakerNotifierFull) NotifyPriceGapBreakerRecovery(record models.BreakerTripRecord, operator string) error {
	f.recoveryMu.Lock()
	defer f.recoveryMu.Unlock()
	f.recoveryCalls = append(f.recoveryCalls, recoveryCall{record, operator})
	return f.recoveryErr
}

// fakeRegistryClearer extends fakePauser with ClearAllPausedByBreaker so
// recovery can call it.
type fakeRegistryClearer struct {
	*fakePauser
	clearCalls int
	clearCount int
	clearErr   error
}

func (f *fakeRegistryClearer) ClearAllPausedByBreaker() (int, error) {
	f.pauser_mu().Lock()
	defer f.pauser_mu().Unlock()
	f.clearCalls++
	if f.clearErr != nil {
		return 0, f.clearErr
	}
	return f.clearCount, nil
}

// pauser_mu exposes fakePauser.mu via a helper since we embed *fakePauser.
func (f *fakeRegistryClearer) pauser_mu() *sync.Mutex {
	return &f.fakePauser.mu
}

func newFullStore() *fakeBreakerStoreFull {
	return &fakeBreakerStoreFull{
		fakeBreakerStore: &fakeBreakerStore{},
		tripsAtIdx:       map[int64]models.BreakerTripRecord{},
	}
}

func newFullNotifier() *fakeBreakerNotifierFull {
	return &fakeBreakerNotifierFull{
		fakeBreakerNotifier: &fakeBreakerNotifier{},
	}
}

// ---- TestFire tests ------------------------------------------------------

// TestSyntheticFireFullCycle — success-criterion #5: trip → paper-flip →
// operator recovery, all without engine restart. Locks the in-process state
// mutation contract.
func TestSyntheticFireFullCycle(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	store := newFullStore()
	store.fakeBreakerStore.exists = false
	agg := &fakeAggregator{pnl: -120}
	notifier := newFullNotifier()
	ws := &fakeBreakerWS{}
	pauser := &fakeRegistryClearer{fakePauser: &fakePauser{count: 2}}

	bc := NewBreakerController(cfg, store, agg, notifier, utils.NewLogger("test"))
	bc.SetWSBroadcaster(ws)
	bc.SetRegistry(pauser)
	bc.SetPositions(&fakePosLister{positions: []*models.PriceGapPosition{
		{ID: "p1", Symbol: "BTCUSDT", LongExchange: "binance", ShortExchange: "bybit"},
		{ID: "p2", Symbol: "ETHUSDT", LongExchange: "binance", ShortExchange: "bybit"},
	}})

	// Step 1: synthetic test-fire (NOT dry-run) trips the breaker.
	rec, err := bc.TestFire(context.Background(), false)
	if err != nil {
		t.Fatalf("TestFire err=%v", err)
	}
	if rec.Source != "test_fire" {
		t.Fatalf("rec.Source=%q want test_fire", rec.Source)
	}
	if store.fakeBreakerStore.state.PaperModeStickyUntil != math.MaxInt64 {
		t.Fatalf("after TestFire: sticky=%d want MaxInt64",
			store.fakeBreakerStore.state.PaperModeStickyUntil)
	}
	if len(store.fakeBreakerStore.appended) != 1 {
		t.Fatalf("AppendBreakerTrip calls=%d want 1", len(store.fakeBreakerStore.appended))
	}
	if store.fakeBreakerStore.appended[0].Source != "test_fire" {
		t.Fatalf("appended.Source=%q want test_fire", store.fakeBreakerStore.appended[0].Source)
	}
	if len(notifier.fakeBreakerNotifier.calls) != 1 {
		t.Fatalf("notifier trip calls=%d want 1", len(notifier.fakeBreakerNotifier.calls))
	}
	if len(ws.calls) != 1 {
		t.Fatalf("ws calls=%d want 1", len(ws.calls))
	}
	if pauser.fakePauser.calls != 1 {
		t.Fatalf("pauser calls=%d want 1", pauser.fakePauser.calls)
	}

	// Seed trip log at index 0 so recovery can backfill it.
	store.tripsAtIdx[0] = store.fakeBreakerStore.appended[0]

	// Step 2: operator recovery clears sticky + un-pauses candidates.
	if err := bc.Recover(context.Background(), "alice"); err != nil {
		t.Fatalf("Recover err=%v", err)
	}
	if store.fakeBreakerStore.state.PaperModeStickyUntil != 0 {
		t.Fatalf("after Recover: sticky=%d want 0",
			store.fakeBreakerStore.state.PaperModeStickyUntil)
	}
	if pauser.clearCalls != 1 {
		t.Fatalf("ClearAllPausedByBreaker calls=%d want 1", pauser.clearCalls)
	}
	if len(notifier.recoveryCalls) != 1 {
		t.Fatalf("notifier recovery calls=%d want 1", len(notifier.recoveryCalls))
	}
	if notifier.recoveryCalls[0].operator != "alice" {
		t.Fatalf("recovery operator=%q want alice", notifier.recoveryCalls[0].operator)
	}
	if len(store.updateCalls) != 1 {
		t.Fatalf("UpdateBreakerTripRecovery calls=%d want 1", len(store.updateCalls))
	}
	if store.updateCalls[0].operator != "alice" {
		t.Fatalf("update.operator=%q want alice", store.updateCalls[0].operator)
	}
	if len(ws.calls) != 2 {
		t.Fatalf("ws calls=%d want 2 (trip + recover)", len(ws.calls))
	}
	if ws.calls[1].event != "pg.breaker.recover" {
		t.Fatalf("ws[1] event=%q want pg.breaker.recover", ws.calls[1].event)
	}
}

// TestSyntheticFireDryRun_NoMutations — D-14 + Open Question #4: dry-run
// computes PnL but does NOT mutate state, append trip, dispatch Telegram, or
// broadcast WS. Aggregator IS called.
func TestSyntheticFireDryRun_NoMutations(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	store := newFullStore()
	store.fakeBreakerStore.exists = true
	agg := &fakeAggregator{pnl: -120}
	notifier := newFullNotifier()
	ws := &fakeBreakerWS{}
	pauser := &fakeRegistryClearer{fakePauser: &fakePauser{count: 5}}

	bc := NewBreakerController(cfg, store, agg, notifier, utils.NewLogger("test"))
	bc.SetWSBroadcaster(ws)
	bc.SetRegistry(pauser)
	bc.SetPositions(&fakePosLister{positions: []*models.PriceGapPosition{{ID: "p1"}}})

	rec, err := bc.TestFire(context.Background(), true)
	if err != nil {
		t.Fatalf("TestFire(dry) err=%v", err)
	}
	if rec.Source != "test_fire_dry_run" {
		t.Fatalf("rec.Source=%q want test_fire_dry_run", rec.Source)
	}
	if rec.TripPnLUSDT != -120 {
		t.Fatalf("rec.TripPnLUSDT=%v want -120", rec.TripPnLUSDT)
	}
	// Aggregator IS called.
	if agg.callCount != 1 {
		t.Fatalf("aggregator calls=%d want 1", agg.callCount)
	}
	// State NOT mutated.
	if store.fakeBreakerStore.saveCount != 0 {
		t.Fatalf("save count=%d want 0 (dry-run no mutations)", store.fakeBreakerStore.saveCount)
	}
	if store.fakeBreakerStore.state.PaperModeStickyUntil != 0 {
		t.Fatalf("sticky=%d want 0 (dry-run preserves)",
			store.fakeBreakerStore.state.PaperModeStickyUntil)
	}
	// AppendBreakerTrip NOT called.
	if len(store.fakeBreakerStore.appended) != 0 {
		t.Fatalf("appended=%d want 0", len(store.fakeBreakerStore.appended))
	}
	// Notifier NOT called.
	if len(notifier.fakeBreakerNotifier.calls) != 0 {
		t.Fatalf("notifier calls=%d want 0", len(notifier.fakeBreakerNotifier.calls))
	}
	// WS NOT called.
	if len(ws.calls) != 0 {
		t.Fatalf("ws calls=%d want 0", len(ws.calls))
	}
	// Pauser NOT called.
	if pauser.fakePauser.calls != 0 {
		t.Fatalf("pauser calls=%d want 0", pauser.fakePauser.calls)
	}
}

// TestRecover_PreservesOperatorDisabled — D-11: ClearAllPausedByBreaker does
// NOT touch operator-set Disabled state. Recovery clears only the breaker-
// owned PausedByBreaker flag. (Validated structurally — the registry chokepoint
// in registry.go already enforces this; here we lock the recovery path to the
// chokepoint.)
func TestRecover_PreservesOperatorDisabled(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	store := newFullStore()
	store.fakeBreakerStore.exists = true
	store.fakeBreakerStore.state = models.BreakerState{PaperModeStickyUntil: math.MaxInt64}
	store.tripsAtIdx[0] = models.BreakerTripRecord{Source: "live"}

	notifier := newFullNotifier()
	ws := &fakeBreakerWS{}
	pauser := &fakeRegistryClearer{fakePauser: &fakePauser{}, clearCount: 4}

	bc := NewBreakerController(cfg, store, nil, notifier, utils.NewLogger("test"))
	bc.SetWSBroadcaster(ws)
	bc.SetRegistry(pauser)

	if err := bc.Recover(context.Background(), "operator"); err != nil {
		t.Fatalf("Recover err=%v", err)
	}
	// Recovery MUST go through ClearAllPausedByBreaker chokepoint, NOT
	// any Disabled-touching path.
	if pauser.clearCalls != 1 {
		t.Fatalf("ClearAllPausedByBreaker calls=%d want 1 (chokepoint)", pauser.clearCalls)
	}
}

// TestBreaker_RecoveryReenablesCandidatesAndClearsSticky — full integration:
// pre-state has sticky=MaxInt64 + paused candidates; Recover; post-state
// sticky=0, candidates cleared, Telegram dispatched, WS broadcast, trip log
// LSet backfilled at index 0.
func TestBreaker_RecoveryReenablesCandidatesAndClearsSticky(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	store := newFullStore()
	store.fakeBreakerStore.exists = true
	store.fakeBreakerStore.state = models.BreakerState{
		PaperModeStickyUntil: math.MaxInt64,
		PendingStrike:        0,
		Strike1Ts:            0,
	}
	store.tripsAtIdx[0] = models.BreakerTripRecord{
		TripTs:      12345,
		TripPnLUSDT: -75,
		Threshold:   -50,
		Source:      "live",
	}

	notifier := newFullNotifier()
	ws := &fakeBreakerWS{}
	pauser := &fakeRegistryClearer{fakePauser: &fakePauser{}, clearCount: 5}

	bc := NewBreakerController(cfg, store, nil, notifier, utils.NewLogger("test"))
	bc.SetWSBroadcaster(ws)
	bc.SetRegistry(pauser)

	if err := bc.Recover(context.Background(), "bob"); err != nil {
		t.Fatalf("Recover err=%v", err)
	}
	// Sticky cleared.
	if store.fakeBreakerStore.state.PaperModeStickyUntil != 0 {
		t.Fatalf("sticky=%d want 0",
			store.fakeBreakerStore.state.PaperModeStickyUntil)
	}
	// Pending strike state cleared too (forward-progress invariant).
	if store.fakeBreakerStore.state.PendingStrike != 0 {
		t.Fatalf("pending=%d want 0", store.fakeBreakerStore.state.PendingStrike)
	}
	// Candidates cleared.
	if pauser.clearCalls != 1 {
		t.Fatalf("ClearAllPausedByBreaker calls=%d want 1", pauser.clearCalls)
	}
	// Telegram dispatched.
	if len(notifier.recoveryCalls) != 1 {
		t.Fatalf("notifier recovery calls=%d want 1", len(notifier.recoveryCalls))
	}
	if notifier.recoveryCalls[0].operator != "bob" {
		t.Fatalf("recovery operator=%q want bob", notifier.recoveryCalls[0].operator)
	}
	// WS broadcast fired.
	if len(ws.calls) != 1 {
		t.Fatalf("ws calls=%d want 1", len(ws.calls))
	}
	if ws.calls[0].event != "pg.breaker.recover" {
		t.Fatalf("ws event=%q want pg.breaker.recover", ws.calls[0].event)
	}
	// Trip log LSet backfilled at index 0.
	if len(store.updateCalls) != 1 {
		t.Fatalf("UpdateBreakerTripRecovery calls=%d want 1", len(store.updateCalls))
	}
	if store.updateCalls[0].index != 0 {
		t.Fatalf("update index=%d want 0", store.updateCalls[0].index)
	}
	if store.updateCalls[0].operator != "bob" {
		t.Fatalf("update operator=%q want bob", store.updateCalls[0].operator)
	}
}

// TestRecover_NotTrippedReturnsError — operator cannot recover when sticky=0.
func TestRecover_NotTrippedReturnsError(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	store := newFullStore()
	store.fakeBreakerStore.exists = true
	store.fakeBreakerStore.state = models.BreakerState{PaperModeStickyUntil: 0}
	notifier := newFullNotifier()
	pauser := &fakeRegistryClearer{fakePauser: &fakePauser{}}

	bc := NewBreakerController(cfg, store, nil, notifier, utils.NewLogger("test"))
	bc.SetRegistry(pauser)

	err := bc.Recover(context.Background(), "alice")
	if err == nil {
		t.Fatalf("Recover err=nil want non-nil (not tripped)")
	}
	// Side effects must NOT fire.
	if pauser.clearCalls != 0 {
		t.Fatalf("ClearAllPausedByBreaker calls=%d want 0", pauser.clearCalls)
	}
	if len(notifier.recoveryCalls) != 0 {
		t.Fatalf("notifier calls=%d want 0", len(notifier.recoveryCalls))
	}
}

// silence unused warnings in helpers
var _ = errors.New
