package pricegaptrader

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/utils"
)

// ---- Fakes for BreakerController tests --------------------------------------

// fakeBreakerStore satisfies BreakerStateStore (Load/Save/Append) AND
// BreakerStateLoader. Mock state is captured in-memory; errors are injectable
// per-method to drive Step 1 / Step 3 failure paths.
type fakeBreakerStore struct {
	mu sync.Mutex

	state  models.BreakerState
	exists bool
	err    error // returned by LoadBreakerState

	saveErr   error                       // returned by SaveBreakerState
	saveCount int                         // number of SaveBreakerState calls (counts ALL, not just success)
	saved     []models.BreakerState       // every saved state in order
	appendErr error                       // returned by AppendBreakerTrip
	appended  []models.BreakerTripRecord  // every record appended (only on success)
}

func (f *fakeBreakerStore) LoadBreakerState() (models.BreakerState, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state, f.exists, f.err
}

func (f *fakeBreakerStore) SaveBreakerState(state models.BreakerState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saveCount++
	f.saved = append(f.saved, state)
	if f.saveErr != nil {
		return f.saveErr
	}
	f.state = state
	f.exists = true
	return nil
}

func (f *fakeBreakerStore) AppendBreakerTrip(record models.BreakerTripRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.appendErr != nil {
		return f.appendErr
	}
	f.appended = append(f.appended, record)
	return nil
}

// LoadBreakerTripAt — no-op default for trip-flow tests; recovery-flow tests
// use fakeBreakerStoreFull (Plan 15-04 wrapper) for backed lookups.
func (f *fakeBreakerStore) LoadBreakerTripAt(_ int64) (models.BreakerTripRecord, bool, error) {
	return models.BreakerTripRecord{}, false, nil
}

// UpdateBreakerTripRecovery — no-op default; recovery-flow tests override.
func (f *fakeBreakerStore) UpdateBreakerTripRecovery(_ int64, _ int64, _ string) error {
	return nil
}

// fakeBreakerStoreOneShotSaveErr lets Step 1 fail while a subsequent save (if
// the controller mistakenly tries another) succeeds — used to lock the "trip
// aborted on Step 1 failure" property without false-positive from a second
// save.
type fakeBreakerStoreOneShotSaveErr struct {
	*fakeBreakerStore
	saveErrFires int  // remaining saves to fail before normal behavior
	mu2          sync.Mutex
}

func (f *fakeBreakerStoreOneShotSaveErr) SaveBreakerState(state models.BreakerState) error {
	f.mu2.Lock()
	if f.saveErrFires > 0 {
		f.saveErrFires--
		f.mu2.Unlock()
		// Record the attempt but return error.
		f.fakeBreakerStore.mu.Lock()
		f.fakeBreakerStore.saveCount++
		f.fakeBreakerStore.saved = append(f.fakeBreakerStore.saved, state)
		f.fakeBreakerStore.mu.Unlock()
		return errors.New("simulated save error")
	}
	f.mu2.Unlock()
	return f.fakeBreakerStore.SaveBreakerState(state)
}

// fakeAggregator satisfies breakerAggregator. PnL preset; calls counted.
type fakeAggregator struct {
	mu        sync.Mutex
	pnl       float64
	err       error
	callCount int
}

func (f *fakeAggregator) Realized24h(_ context.Context, _ time.Time) (float64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callCount++
	return f.pnl, f.err
}

// fakeBreakerNotifier records NotifyPriceGapBreakerTrip calls + injectable err.
// Plan 15-04 widened BreakerNotifier to include NotifyPriceGapBreakerRecovery;
// this fake provides a no-op default. Recovery-flow tests wrap with
// fakeBreakerNotifierFull to record recovery calls.
type fakeBreakerNotifier struct {
	mu          sync.Mutex
	calls       []models.BreakerTripRecord
	notifyErr   error
}

func (f *fakeBreakerNotifier) NotifyPriceGapBreakerTrip(record models.BreakerTripRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, record)
	return f.notifyErr
}

// NotifyPriceGapBreakerRecovery — no-op default; recovery-flow tests override.
func (f *fakeBreakerNotifier) NotifyPriceGapBreakerRecovery(_ models.BreakerTripRecord, _ string) error {
	return nil
}

// fakeBreakerWS records BroadcastPriceGapBreakerEvent calls.
type fakeBreakerWS struct {
	mu    sync.Mutex
	calls []struct {
		event   string
		payload any
	}
}

func (f *fakeBreakerWS) BroadcastPriceGapBreakerEvent(event string, payload any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, struct {
		event   string
		payload any
	}{event, payload})
}

// fakeRampSnap satisfies RampSnapshotter with a fixed CurrentStage.
type fakeRampSnap struct{ stage int }

func (f fakeRampSnap) Snapshot() models.RampState {
	return models.RampState{CurrentStage: f.stage}
}

// fakePauser records PauseAllOpenCandidates calls.
// Plan 15-04 widened CandidatePauser to include ClearAllPausedByBreaker for
// the Recover path; this fake provides a no-op default. Plan 15-04 tests that
// exercise recovery wrap *fakePauser with fakeRegistryClearer to record clear
// calls.
type fakePauser struct {
	mu    sync.Mutex
	calls int
	last  []*models.PriceGapPosition
	count int
	err   error
}

func (f *fakePauser) PauseAllOpenCandidates(positions []*models.PriceGapPosition) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.last = positions
	if f.err != nil {
		return 0, f.err
	}
	return f.count, nil
}

// ClearAllPausedByBreaker — no-op default; recovery-flow tests override.
func (f *fakePauser) ClearAllPausedByBreaker() (int, error) { return 0, nil }

// fakePosLister returns a preset list of active positions.
type fakePosLister struct {
	positions []*models.PriceGapPosition
	err       error
}

func (f *fakePosLister) GetActivePriceGapPositions() ([]*models.PriceGapPosition, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.positions, nil
}

// newBreakerControllerForTest constructs a controller with sensible defaults.
// Override fields after.
func newBreakerControllerForTest(cfg *config.Config, store BreakerStateStore, agg breakerAggregator, notifier BreakerNotifier) *BreakerController {
	return NewBreakerController(cfg, store, agg, notifier, utils.NewLogger("test"))
}

// helper: build a cfg with defaults for tests (enabled=true, threshold=-50).
func breakerCfgEnabled(threshold float64) *config.Config {
	return &config.Config{
		PriceGapBreakerEnabled:     true,
		PriceGapDrawdownLimitUSDT:  threshold,
		PriceGapBreakerIntervalSec: 300,
	}
}

// helper: pick a non-blackout time. Bybit blackout = minute 4 or
// (minute 5 && second < 30). Use minute 10 second 0.
func nonBlackoutTime() time.Time {
	return time.Date(2026, 5, 1, 12, 10, 0, 0, time.UTC)
}

func blackoutTime() time.Time {
	// Minute 4 second 30 — inside [:04, :05:30).
	return time.Date(2026, 5, 1, 12, 4, 30, 0, time.UTC)
}

// ---- Tests ------------------------------------------------------------------

// TestBreaker_DisabledByDefault — when cfg.PriceGapBreakerEnabled=false the
// daemon Run() returns immediately and evalTick is a no-op (no Redis read,
// no aggregator call).
func TestBreaker_DisabledByDefault(t *testing.T) {
	cfg := &config.Config{PriceGapBreakerEnabled: false}
	store := &fakeBreakerStore{}
	agg := &fakeAggregator{pnl: -100}
	notifier := &fakeBreakerNotifier{}
	bc := newBreakerControllerForTest(cfg, store, agg, notifier)

	// Run should return immediately when disabled.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		bc.Run(ctx)
		close(done)
	}()
	select {
	case <-done:
		// good
	case <-time.After(200 * time.Millisecond):
		cancel()
		<-done
		t.Fatalf("Run did not return immediately when disabled")
	}
	cancel()

	// Manual evalTick must also no-op.
	if err := bc.evalTick(context.Background(), nonBlackoutTime()); err != nil {
		t.Fatalf("evalTick disabled err=%v, want nil", err)
	}
	if agg.callCount != 0 {
		t.Fatalf("aggregator called %d times when disabled, want 0", agg.callCount)
	}
	if store.saveCount != 0 {
		t.Fatalf("store saved %d times when disabled, want 0", store.saveCount)
	}
}

// TestBreaker_FreshBootInit — missing pg:breaker:state HASH (exists=false)
// triggers fresh-init path. Must not panic; must not trip; saves a clean
// baseline state.
func TestBreaker_FreshBootInit(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	store := &fakeBreakerStore{exists: false}
	agg := &fakeAggregator{pnl: -10} // above threshold (recovery / not-in-breach)
	notifier := &fakeBreakerNotifier{}
	bc := newBreakerControllerForTest(cfg, store, agg, notifier)

	if err := bc.evalTick(context.Background(), nonBlackoutTime()); err != nil {
		t.Fatalf("evalTick err=%v", err)
	}
	if store.saveCount != 1 {
		t.Fatalf("save count=%d, want 1 (fresh state persisted)", store.saveCount)
	}
	if store.state.PaperModeStickyUntil != 0 {
		t.Fatalf("sticky=%d, want 0 (fresh init must NOT trip)", store.state.PaperModeStickyUntil)
	}
	if store.state.PendingStrike != 0 {
		t.Fatalf("pending=%d, want 0", store.state.PendingStrike)
	}
}

// TestBreaker_BootGuard_PreservesExistingTrip — existing sticky=MaxInt64
// (post-trip) HASH means the breaker is already tripped. evalTick must
// observe this and return immediately without aggregating or mutating state.
func TestBreaker_BootGuard_PreservesExistingTrip(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	store := &fakeBreakerStore{
		state:  models.BreakerState{PaperModeStickyUntil: math.MaxInt64},
		exists: true,
	}
	agg := &fakeAggregator{pnl: -100}
	notifier := &fakeBreakerNotifier{}
	bc := newBreakerControllerForTest(cfg, store, agg, notifier)

	if err := bc.evalTick(context.Background(), nonBlackoutTime()); err != nil {
		t.Fatalf("evalTick err=%v", err)
	}
	if agg.callCount != 0 {
		t.Fatalf("aggregator called %d times with sticky non-zero, want 0", agg.callCount)
	}
	if store.saveCount != 0 {
		t.Fatalf("save count=%d, want 0 (already-tripped tick must not mutate)", store.saveCount)
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("notifier called %d times, want 0", len(notifier.calls))
	}
	// Sticky must remain MaxInt64.
	if store.state.PaperModeStickyUntil != math.MaxInt64 {
		t.Fatalf("sticky=%d, want MaxInt64 (preserved)", store.state.PaperModeStickyUntil)
	}
}

// TestBreaker_SingleStrike_NoTrip — first in-breach tick sets PendingStrike=1
// and Strike1Ts. Does NOT trip (sticky stays 0). Does NOT call notifier.
func TestBreaker_SingleStrike_NoTrip(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	store := &fakeBreakerStore{exists: true} // empty existing state
	agg := &fakeAggregator{pnl: -100}        // below threshold
	notifier := &fakeBreakerNotifier{}
	bc := newBreakerControllerForTest(cfg, store, agg, notifier)
	now := nonBlackoutTime()

	if err := bc.evalTick(context.Background(), now); err != nil {
		t.Fatalf("evalTick err=%v", err)
	}
	if store.state.PendingStrike != 1 {
		t.Fatalf("pending=%d, want 1", store.state.PendingStrike)
	}
	if store.state.Strike1Ts != now.UnixMilli() {
		t.Fatalf("Strike1Ts=%d, want %d", store.state.Strike1Ts, now.UnixMilli())
	}
	if store.state.PaperModeStickyUntil != 0 {
		t.Fatalf("sticky=%d, want 0 (single strike must NOT trip)", store.state.PaperModeStickyUntil)
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("notifier called on single strike — should only fire on Strike-2")
	}
}

// TestBreaker_TwoStrikeTrips — pending strike + breach + ≥5min elapsed →
// trip. Sticky=MaxInt64 persisted, AppendBreakerTrip called once,
// NotifyPriceGapBreakerTrip called once with Source="live".
func TestBreaker_TwoStrikeTrips(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	now := nonBlackoutTime()
	prior := now.Add(-6 * time.Minute) // 6 min ago — > 5min threshold
	store := &fakeBreakerStore{
		state: models.BreakerState{
			PendingStrike: 1,
			Strike1Ts:     prior.UnixMilli(),
		},
		exists: true,
	}
	agg := &fakeAggregator{pnl: -100}
	notifier := &fakeBreakerNotifier{}
	bc := newBreakerControllerForTest(cfg, store, agg, notifier)

	if err := bc.evalTick(context.Background(), now); err != nil {
		t.Fatalf("evalTick err=%v", err)
	}
	if store.state.PaperModeStickyUntil != math.MaxInt64 {
		t.Fatalf("sticky=%d, want MaxInt64 (trip)", store.state.PaperModeStickyUntil)
	}
	if len(store.appended) != 1 {
		t.Fatalf("AppendBreakerTrip called %d times, want 1", len(store.appended))
	}
	if store.appended[0].Source != "live" {
		t.Fatalf("trip source=%q, want %q", store.appended[0].Source, "live")
	}
	if store.appended[0].TripPnLUSDT != -100 {
		t.Fatalf("trip pnl=%v, want -100", store.appended[0].TripPnLUSDT)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("notifier called %d times, want 1", len(notifier.calls))
	}
}

// TestBreaker_TwoStrikeRequiresTwoSeparateEvaluations — pending strike but
// elapsed < 5 min: defensive skip. State unchanged-but-saved (LastEval
// updated); no trip; no notifier.
func TestBreaker_TwoStrikeRequiresTwoSeparateEvaluations(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	now := nonBlackoutTime()
	prior := now.Add(-2 * time.Minute) // only 2 min ago — < 5min
	store := &fakeBreakerStore{
		state: models.BreakerState{
			PendingStrike: 1,
			Strike1Ts:     prior.UnixMilli(),
		},
		exists: true,
	}
	agg := &fakeAggregator{pnl: -100}
	notifier := &fakeBreakerNotifier{}
	bc := newBreakerControllerForTest(cfg, store, agg, notifier)

	if err := bc.evalTick(context.Background(), now); err != nil {
		t.Fatalf("evalTick err=%v", err)
	}
	if store.state.PaperModeStickyUntil != 0 {
		t.Fatalf("sticky=%d, want 0 (defensive skip — < 5min)", store.state.PaperModeStickyUntil)
	}
	if store.state.PendingStrike != 1 {
		t.Fatalf("pending=%d, want 1 (preserved across defensive skip)", store.state.PendingStrike)
	}
	if store.state.Strike1Ts != prior.UnixMilli() {
		t.Fatalf("Strike1Ts changed after defensive skip: got %d, want %d",
			store.state.Strike1Ts, prior.UnixMilli())
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("notifier called on defensive skip — should not fire")
	}
}

// TestBreaker_RecoveryClearsPendingStrike — D-08: pending strike + non-breach
// PnL → strike cleared (PendingStrike=0, Strike1Ts=0). Sticky stays 0.
func TestBreaker_RecoveryClearsPendingStrike(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	now := nonBlackoutTime()
	prior := now.Add(-3 * time.Minute)
	store := &fakeBreakerStore{
		state: models.BreakerState{
			PendingStrike: 1,
			Strike1Ts:     prior.UnixMilli(),
		},
		exists: true,
	}
	agg := &fakeAggregator{pnl: -10} // above threshold (recovery)
	notifier := &fakeBreakerNotifier{}
	bc := newBreakerControllerForTest(cfg, store, agg, notifier)

	if err := bc.evalTick(context.Background(), now); err != nil {
		t.Fatalf("evalTick err=%v", err)
	}
	if store.state.PendingStrike != 0 {
		t.Fatalf("pending=%d, want 0 (recovery clears)", store.state.PendingStrike)
	}
	if store.state.Strike1Ts != 0 {
		t.Fatalf("Strike1Ts=%d, want 0 (recovery clears)", store.state.Strike1Ts)
	}
	if store.state.PaperModeStickyUntil != 0 {
		t.Fatalf("sticky=%d, want 0", store.state.PaperModeStickyUntil)
	}
}

// TestBreaker_BlackoutSuppression — D-03: tick during Bybit blackout window
// is a no-op. Aggregator NOT called; state UNCHANGED.
func TestBreaker_BlackoutSuppression(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	prior := blackoutTime().Add(-10 * time.Minute)
	store := &fakeBreakerStore{
		state: models.BreakerState{
			PendingStrike: 1,
			Strike1Ts:     prior.UnixMilli(),
		},
		exists: true,
	}
	agg := &fakeAggregator{pnl: -100}
	notifier := &fakeBreakerNotifier{}
	bc := newBreakerControllerForTest(cfg, store, agg, notifier)

	if err := bc.evalTick(context.Background(), blackoutTime()); err != nil {
		t.Fatalf("evalTick err=%v", err)
	}
	if agg.callCount != 0 {
		t.Fatalf("aggregator called %d times during blackout, want 0", agg.callCount)
	}
	if store.saveCount != 0 {
		t.Fatalf("save count=%d during blackout, want 0", store.saveCount)
	}
	if store.state.PendingStrike != 1 {
		t.Fatalf("pending=%d, want 1 (preserved across blackout)", store.state.PendingStrike)
	}
}

// TestBreaker_PendingSurvivesBlackout — D-04: blackout-suppressed tick does
// NOT clear pending strike. The next non-blackout tick with breach + elapsed
// ≥5min fires the trip.
func TestBreaker_PendingSurvivesBlackout(t *testing.T) {
	cfg := breakerCfgEnabled(-50)

	// Strike-1 was 10 min ago at minute :04:15 (blackout). Then a tick at
	// minute :04:30 (still blackout) is suppressed; pending preserved. Then a
	// normal tick at minute :07:00 fires.
	t0 := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC) // 12:00:00
	store := &fakeBreakerStore{
		state: models.BreakerState{
			PendingStrike: 1,
			Strike1Ts:     t0.UnixMilli(),
		},
		exists: true,
	}
	agg := &fakeAggregator{pnl: -100}
	notifier := &fakeBreakerNotifier{}
	bc := newBreakerControllerForTest(cfg, store, agg, notifier)

	// Tick at 12:04:15 — blackout. No-op.
	tBlackout := time.Date(2026, 5, 1, 12, 4, 15, 0, time.UTC)
	if err := bc.evalTick(context.Background(), tBlackout); err != nil {
		t.Fatalf("evalTick blackout err=%v", err)
	}
	if store.state.PendingStrike != 1 {
		t.Fatalf("pending=%d after blackout, want 1", store.state.PendingStrike)
	}

	// Tick at 12:07:00 — normal, breach, elapsed > 5 min. Trip!
	tNormal := time.Date(2026, 5, 1, 12, 7, 0, 0, time.UTC)
	if err := bc.evalTick(context.Background(), tNormal); err != nil {
		t.Fatalf("evalTick normal err=%v", err)
	}
	if store.state.PaperModeStickyUntil != math.MaxInt64 {
		t.Fatalf("sticky=%d, want MaxInt64 (trip after blackout passthrough)",
			store.state.PaperModeStickyUntil)
	}
}

// TestBreaker_StateSurvivesRestart — D-05: simulate kill-9 between Strike-1
// and Strike-2. Persisted state reloads; second eval ticks fires trip.
func TestBreaker_StateSurvivesRestart(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	store := &fakeBreakerStore{exists: false}
	agg := &fakeAggregator{pnl: -100}
	notifier := &fakeBreakerNotifier{}

	// Pre-restart: a Strike-1 was already persisted (simulated by writing to
	// the store directly).
	t0 := nonBlackoutTime()
	if err := store.SaveBreakerState(models.BreakerState{
		PendingStrike: 1,
		Strike1Ts:     t0.UnixMilli(),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Fresh BreakerController instance (post-restart).
	bc := newBreakerControllerForTest(cfg, store, agg, notifier)

	// Eval at t0+6min: breach + elapsed > 5min → trip.
	tNext := t0.Add(6 * time.Minute)
	if err := bc.evalTick(context.Background(), tNext); err != nil {
		t.Fatalf("evalTick: %v", err)
	}
	if store.state.PaperModeStickyUntil != math.MaxInt64 {
		t.Fatalf("sticky=%d, want MaxInt64 (trip on Strike-2 after restart)",
			store.state.PaperModeStickyUntil)
	}
	if len(store.appended) != 1 {
		t.Fatalf("trip records=%d, want 1", len(store.appended))
	}
}

// TestBreaker_TripOrdering_StickyFirstWhenStepsFail — D-15 atomicity anchor.
// Steps 2-5 all fail (registry returns error, AppendBreakerTrip returns
// error, notifier returns error). Step 1 (sticky save) MUST still succeed
// and the load-bearing safety property must hold: state has sticky=MaxInt64.
func TestBreaker_TripOrdering_StickyFirstWhenStepsFail(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	now := nonBlackoutTime()
	prior := now.Add(-6 * time.Minute)
	store := &fakeBreakerStore{
		state: models.BreakerState{
			PendingStrike: 1,
			Strike1Ts:     prior.UnixMilli(),
		},
		exists: true,
		// SaveBreakerState succeeds; AppendBreakerTrip fails.
		appendErr: errors.New("simulated append err"),
	}
	agg := &fakeAggregator{pnl: -100}
	notifier := &fakeBreakerNotifier{
		notifyErr: errors.New("simulated notify err"),
	}
	bc := newBreakerControllerForTest(cfg, store, agg, notifier)

	// Wire registry + positions so Step 2 runs and we can fail it.
	pauser := &fakePauser{err: errors.New("simulated pause err")}
	bc.SetRegistry(pauser)
	bc.SetPositions(&fakePosLister{positions: []*models.PriceGapPosition{{ID: "p1"}}})

	// trip should return nil — only Step 1 failure aborts; Steps 2-5 are
	// best-effort.
	err := bc.evalTick(context.Background(), now)
	if err != nil {
		t.Fatalf("evalTick err=%v, want nil (Step 1 succeeded; Steps 2-5 best-effort)", err)
	}

	// Load-bearing: sticky persisted.
	if store.state.PaperModeStickyUntil != math.MaxInt64 {
		t.Fatalf("sticky=%d, want MaxInt64 — Step 1 load-bearing safety property violated",
			store.state.PaperModeStickyUntil)
	}
	// Step 2 failed but was attempted.
	if pauser.calls != 1 {
		t.Fatalf("pauser calls=%d, want 1", pauser.calls)
	}
	// Step 3 attempted (AppendBreakerTrip returned error, so f.appended stays empty).
	if len(store.appended) != 0 {
		t.Fatalf("appended=%d, want 0 (append errored)", len(store.appended))
	}
	// Step 4 attempted (notifier saw the call even on error).
	if len(notifier.calls) != 1 {
		t.Fatalf("notifier calls=%d, want 1", len(notifier.calls))
	}
}

// TestBreaker_TripOrdering_Step1FailureAborts — when Step 1 SaveBreakerState
// fails, trip MUST return error and Steps 2-5 must NOT run. Sticky NOT
// persisted means caller (the eval tick) sees critical failure.
func TestBreaker_TripOrdering_Step1FailureAborts(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	now := nonBlackoutTime()
	prior := now.Add(-6 * time.Minute)

	innerStore := &fakeBreakerStore{
		state: models.BreakerState{
			PendingStrike: 1,
			Strike1Ts:     prior.UnixMilli(),
		},
		exists: true,
	}
	store := &fakeBreakerStoreOneShotSaveErr{
		fakeBreakerStore: innerStore,
		saveErrFires:     1,
	}
	agg := &fakeAggregator{pnl: -100}
	notifier := &fakeBreakerNotifier{}
	bc := newBreakerControllerForTest(cfg, store, agg, notifier)
	pauser := &fakePauser{count: 5}
	bc.SetRegistry(pauser)
	bc.SetPositions(&fakePosLister{positions: []*models.PriceGapPosition{{ID: "p1"}}})

	err := bc.evalTick(context.Background(), now)
	if err == nil {
		t.Fatalf("evalTick err=nil, want non-nil (Step 1 must abort the trip)")
	}

	// Sticky NOT persisted — verify by reading current store state. The fake
	// keeps the prior state on save error.
	if innerStore.state.PaperModeStickyUntil == math.MaxInt64 {
		t.Fatalf("sticky=MaxInt64 after Step 1 failure — should NOT have persisted")
	}
	// Steps 2-5 must NOT have run.
	if pauser.calls != 0 {
		t.Fatalf("pauser calls=%d, want 0 (Step 1 fail must skip Step 2)", pauser.calls)
	}
	if len(innerStore.appended) != 0 {
		t.Fatalf("appended=%d, want 0 (Step 1 fail must skip Step 3)", len(innerStore.appended))
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("notifier calls=%d, want 0 (Step 1 fail must skip Step 4)", len(notifier.calls))
	}
}

// TestBreaker_TripIncludesRampStage — when ramp is wired, trip record's
// RampStage carries the snapshot value. Lock-down test for D-15 step 1
// snapshot timing.
func TestBreaker_TripIncludesRampStage(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	now := nonBlackoutTime()
	prior := now.Add(-6 * time.Minute)
	store := &fakeBreakerStore{
		state: models.BreakerState{
			PendingStrike: 1,
			Strike1Ts:     prior.UnixMilli(),
		},
		exists: true,
	}
	agg := &fakeAggregator{pnl: -100}
	notifier := &fakeBreakerNotifier{}
	bc := newBreakerControllerForTest(cfg, store, agg, notifier)
	bc.SetRamp(fakeRampSnap{stage: 2})

	if err := bc.evalTick(context.Background(), now); err != nil {
		t.Fatalf("evalTick: %v", err)
	}
	if len(store.appended) != 1 {
		t.Fatalf("appended=%d, want 1", len(store.appended))
	}
	if store.appended[0].RampStage != 2 {
		t.Fatalf("RampStage=%d, want 2", store.appended[0].RampStage)
	}
}

// TestBreaker_TripPausedCandidateCount — Step 2 records how many candidates
// were paused; the count appears in the trip record.
func TestBreaker_TripPausedCandidateCount(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	now := nonBlackoutTime()
	prior := now.Add(-6 * time.Minute)
	store := &fakeBreakerStore{
		state: models.BreakerState{
			PendingStrike: 1,
			Strike1Ts:     prior.UnixMilli(),
		},
		exists: true,
	}
	agg := &fakeAggregator{pnl: -100}
	notifier := &fakeBreakerNotifier{}
	pauser := &fakePauser{count: 3}
	bc := newBreakerControllerForTest(cfg, store, agg, notifier)
	bc.SetRegistry(pauser)
	bc.SetPositions(&fakePosLister{positions: []*models.PriceGapPosition{
		{ID: "p1"}, {ID: "p2"}, {ID: "p3"},
	}})

	if err := bc.evalTick(context.Background(), now); err != nil {
		t.Fatalf("evalTick: %v", err)
	}
	if len(store.appended) != 1 {
		t.Fatalf("appended=%d, want 1", len(store.appended))
	}
	if store.appended[0].PausedCandidateCount != 3 {
		t.Fatalf("PausedCandidateCount=%d, want 3", store.appended[0].PausedCandidateCount)
	}
}

// TestBreaker_WSBroadcastFiredOnTrip — Step 5 invokes the broadcaster.
func TestBreaker_WSBroadcastFiredOnTrip(t *testing.T) {
	cfg := breakerCfgEnabled(-50)
	now := nonBlackoutTime()
	prior := now.Add(-6 * time.Minute)
	store := &fakeBreakerStore{
		state: models.BreakerState{
			PendingStrike: 1,
			Strike1Ts:     prior.UnixMilli(),
		},
		exists: true,
	}
	agg := &fakeAggregator{pnl: -100}
	notifier := &fakeBreakerNotifier{}
	ws := &fakeBreakerWS{}
	bc := newBreakerControllerForTest(cfg, store, agg, notifier)
	bc.SetWSBroadcaster(ws)

	if err := bc.evalTick(context.Background(), now); err != nil {
		t.Fatalf("evalTick: %v", err)
	}
	if len(ws.calls) != 1 {
		t.Fatalf("ws calls=%d, want 1", len(ws.calls))
	}
	if ws.calls[0].event != "pg.breaker.trip" {
		t.Fatalf("ws event=%q, want pg.breaker.trip", ws.calls[0].event)
	}
}

// TestBreaker_Snapshot — read-only Snapshot returns current state.
func TestBreaker_Snapshot(t *testing.T) {
	store := &fakeBreakerStore{
		state:  models.BreakerState{PendingStrike: 1, Strike1Ts: 12345},
		exists: true,
	}
	bc := NewBreakerController(&config.Config{}, store, nil, nil, utils.NewLogger("test"))
	got, err := bc.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot err=%v", err)
	}
	if got.PendingStrike != 1 || got.Strike1Ts != 12345 {
		t.Fatalf("Snapshot mismatch: %+v", got)
	}
}

// ---- Re-implemented IsPaperModeActive helper tests (preserved from Plan 15-02 stub) ----
//
// These tests live alongside the BreakerController tests because Plan 15-01's
// breaker_controller_test.go file reserved the namespace for them. Plan 15-03
// preserves them so the existing test surface continues to pass.

// TestTracker_IsPaperModeActive_CfgFlagOnly — cfg=true short-circuits.
func TestTracker_IsPaperModeActive_CfgFlagOnly(t *testing.T) {
	tr := &Tracker{cfg: &config.Config{PriceGapPaperMode: true}}
	got, err := tr.IsPaperModeActive(context.Background())
	if err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
	if !got {
		t.Fatalf("got=false, want true (cfg flag short-circuit)")
	}
}

// TestTracker_IsPaperModeActive_CfgFlagFalse_NoSticky — cfg=false + sticky=0 → live.
func TestTracker_IsPaperModeActive_CfgFlagFalse_NoSticky(t *testing.T) {
	tr := &Tracker{
		cfg:          &config.Config{PriceGapPaperMode: false},
		breakerStore: &fakeBreakerStore{state: models.BreakerState{PaperModeStickyUntil: 0}, exists: true},
	}
	got, err := tr.IsPaperModeActive(context.Background())
	if err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
	if got {
		t.Fatalf("got=true, want false")
	}
}

// TestTracker_IsPaperModeActive_StickyMaxInt64 — D-07 sentinel forces paper.
func TestTracker_IsPaperModeActive_StickyMaxInt64(t *testing.T) {
	tr := &Tracker{
		cfg:          &config.Config{PriceGapPaperMode: false},
		breakerStore: &fakeBreakerStore{state: models.BreakerState{PaperModeStickyUntil: math.MaxInt64}, exists: true},
	}
	got, err := tr.IsPaperModeActive(context.Background())
	if err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
	if !got {
		t.Fatalf("got=false, want true (sticky=MaxInt64)")
	}
}

// TestTracker_IsPaperModeActive_StickyFutureTimestamp — timed sticky window active.
func TestTracker_IsPaperModeActive_StickyFutureTimestamp(t *testing.T) {
	future := time.Now().Add(1 * time.Hour).UnixMilli()
	tr := &Tracker{
		cfg:          &config.Config{PriceGapPaperMode: false},
		breakerStore: &fakeBreakerStore{state: models.BreakerState{PaperModeStickyUntil: future}, exists: true},
	}
	got, err := tr.IsPaperModeActive(context.Background())
	if err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
	if !got {
		t.Fatalf("got=false, want true (sticky window active)")
	}
}

// TestTracker_IsPaperModeActive_StickyExpired — expired window → live.
func TestTracker_IsPaperModeActive_StickyExpired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour).UnixMilli()
	tr := &Tracker{
		cfg:          &config.Config{PriceGapPaperMode: false},
		breakerStore: &fakeBreakerStore{state: models.BreakerState{PaperModeStickyUntil: past}, exists: true},
	}
	got, err := tr.IsPaperModeActive(context.Background())
	if err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
	if got {
		t.Fatalf("got=true, want false")
	}
}

// TestTracker_IsPaperModeActive_RedisError_FailsToPaper — Pitfall 8.
func TestTracker_IsPaperModeActive_RedisError_FailsToPaper(t *testing.T) {
	wantErr := errors.New("redis down")
	tr := &Tracker{
		cfg:          &config.Config{PriceGapPaperMode: false},
		breakerStore: &fakeBreakerStore{err: wantErr},
	}
	got, err := tr.IsPaperModeActive(context.Background())
	if err == nil {
		t.Fatalf("err=nil, want non-nil")
	}
	if !got {
		t.Fatalf("got=false, want true (fail-safe to paper)")
	}
}

// TestTracker_IsPaperModeActive_NilStore — pre-wired path.
func TestTracker_IsPaperModeActive_NilStore(t *testing.T) {
	tr := &Tracker{cfg: &config.Config{PriceGapPaperMode: false}}
	got, err := tr.IsPaperModeActive(context.Background())
	if err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
	if got {
		t.Fatalf("got=true, want false")
	}
}

// silence unused warnings (package-level helper kept here for future expansion).
var _ = fmt.Sprint
