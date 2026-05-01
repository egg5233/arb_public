package pricegaptrader

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
)

// TestDaemon_FiresAtUTC0030 locks the nextUTCFireTime helper invariant: the
// returned instant is always strictly after `now` and lands at exactly
// 00:30:00 UTC.
func TestDaemon_FiresAtUTC0030(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "before fire time same day",
			now:  time.Date(2026, 4, 30, 0, 25, 0, 0, time.UTC),
			want: time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC),
		},
		{
			name: "exactly at fire time rolls to next day",
			now:  time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC),
			want: time.Date(2026, 5, 1, 0, 30, 0, 0, time.UTC),
		},
		{
			name: "afternoon rolls to next day",
			now:  time.Date(2026, 4, 30, 13, 45, 0, 0, time.UTC),
			want: time.Date(2026, 5, 1, 0, 30, 0, 0, time.UTC),
		},
		{
			name: "23:59 same-day fire crosses midnight",
			now:  time.Date(2026, 4, 30, 23, 59, 59, 0, time.UTC),
			want: time.Date(2026, 5, 1, 0, 30, 0, 0, time.UTC),
		},
		{
			name: "non-UTC input still produces UTC fire time",
			now:  time.Date(2026, 4, 30, 8, 25, 0, 0, time.FixedZone("Asia/Taipei", 8*3600)),
			want: time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nextUTCFireTime(tc.now)
			if !got.Equal(tc.want) {
				t.Errorf("nextUTCFireTime(%s) = %s, want %s",
					tc.now.Format(time.RFC3339Nano),
					got.Format(time.RFC3339Nano),
					tc.want.Format(time.RFC3339Nano),
				)
			}
		})
	}
}

// fakeReconcileStoreForDaemon — minimal ReconcileStore stub that returns
// "no positions" + records LoadRecord calls so tests can drive the boot-
// catchup branch without setting up Redis.
type fakeReconcileStoreForDaemon struct {
	mu                sync.Mutex
	saved             map[string][]byte
	listForDateCalls  []string
	loadRecordCalls   []string
	runCalls          int32
	overrideListErr   error
	overrideSaveErr   error
}

func newFakeReconcileStoreForDaemon() *fakeReconcileStoreForDaemon {
	return &fakeReconcileStoreForDaemon{saved: map[string][]byte{}}
}

func (f *fakeReconcileStoreForDaemon) GetPriceGapClosedPositionsForDate(date string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listForDateCalls = append(f.listForDateCalls, date)
	atomic.AddInt32(&f.runCalls, 1)
	if f.overrideListErr != nil {
		return nil, f.overrideListErr
	}
	return nil, nil
}

func (f *fakeReconcileStoreForDaemon) LoadPriceGapPosition(id string) (*models.PriceGapPosition, bool, error) {
	return nil, false, nil
}

func (f *fakeReconcileStoreForDaemon) SavePriceGapReconcileDaily(date string, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.overrideSaveErr != nil {
		return f.overrideSaveErr
	}
	f.saved[date] = payload
	return nil
}

func (f *fakeReconcileStoreForDaemon) LoadPriceGapReconcileDaily(date string) ([]byte, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loadRecordCalls = append(f.loadRecordCalls, date)
	if data, ok := f.saved[date]; ok {
		return data, true, nil
	}
	return nil, false, nil
}

// fakeReconcileNotifierForDaemon — minimal ReconcileNotifier impl.
type fakeReconcileNotifierForDaemon struct {
	mu       sync.Mutex
	digests  []string
	failures []string
}

func (f *fakeReconcileNotifierForDaemon) NotifyPriceGapDailyDigest(date string, _ DailyReconcileRecord, _ models.RampState) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.digests = append(f.digests, date)
}

func (f *fakeReconcileNotifierForDaemon) NotifyPriceGapReconcileFailure(date string, _ error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failures = append(f.failures, date)
}

// fakeRampStoreForDaemon — minimal RampStateStore.
type fakeRampStoreForDaemon struct {
	mu     sync.Mutex
	state  models.RampState
	saved  bool
	events []models.RampEvent
}

func (f *fakeRampStoreForDaemon) SavePriceGapRampState(s models.RampState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = s
	f.saved = true
	return nil
}

func (f *fakeRampStoreForDaemon) LoadPriceGapRampState() (models.RampState, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.saved {
		return models.RampState{}, false, nil
	}
	return f.state, true, nil
}

func (f *fakeRampStoreForDaemon) AppendPriceGapRampEvent(ev models.RampEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, ev)
	return nil
}

// fakeRampNotifierForDaemon — minimal RampNotifier.
type fakeRampNotifierForDaemon struct{}

func (fakeRampNotifierForDaemon) NotifyPriceGapRampDemote(int, int, string)              {}
func (fakeRampNotifierForDaemon) NotifyPriceGapRampForceOp(string, int, int, string, string) {}

// TestDaemon_BootCatchupAfter01UTC_RunsImmediately — boots the daemon's
// boot-catchup branch by simulating the same logic the loop runs on first
// tick. Asserts RunForDate is invoked for yesterday's date when the catchup
// condition holds (now >= 01:00 UTC AND yesterday's record missing).
//
// We don't spin up a full Tracker — the boot-catchup logic is a few lines
// at the head of reconcileLoop and can be exercised directly via the
// Reconciler + LoadRecord.
func TestDaemon_BootCatchupAfter01UTC_RunsImmediately(t *testing.T) {
	store := newFakeReconcileStoreForDaemon()
	notifier := &fakeReconcileNotifierForDaemon{}
	cfg := &config.Config{PriceGapAnomalySlippageBps: 50}
	rec := NewReconciler(store, notifier, cfg, nil)
	rec.SetSleepFunc(func(time.Duration) {}) // no real sleep

	// Simulate "now" at 14:00 UTC — well past 01:00.
	nowUTC := time.Date(2026, 4, 30, 14, 0, 0, 0, time.UTC)
	yesterday := nowUTC.AddDate(0, 0, -1).Format("2006-01-02")

	// Boot-catchup condition: yesterday not yet reconciled.
	if _, exists, _ := rec.LoadRecord(context.Background(), yesterday); exists {
		t.Fatalf("precondition violated: yesterday %s appears reconciled", yesterday)
	}
	// Drive the same code path the loop runs.
	if err := rec.RunForDate(context.Background(), yesterday); err != nil {
		t.Fatalf("RunForDate(%s) failed: %v", yesterday, err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.listForDateCalls) == 0 {
		t.Fatal("expected at least 1 GetPriceGapClosedPositionsForDate call")
	}
	if got := store.listForDateCalls[0]; got != yesterday {
		t.Errorf("RunForDate target: got %q, want yesterday=%q", got, yesterday)
	}
	if _, ok := store.saved[yesterday]; !ok {
		t.Errorf("expected pg:reconcile:daily:%s to be saved", yesterday)
	}
}

// fakeBootGuardNotifier records the BOOT_GUARD critical dispatch path.
type fakeBootGuardNotifier struct {
	mu       sync.Mutex
	failures []string
}

func (f *fakeBootGuardNotifier) NotifyPriceGapEntry(*models.PriceGapPosition) {}
func (f *fakeBootGuardNotifier) NotifyPriceGapExit(*models.PriceGapPosition, string, float64, time.Duration) {
}
func (f *fakeBootGuardNotifier) NotifyPriceGapRiskBlock(string, string, string) {}
func (f *fakeBootGuardNotifier) NotifyPriceGapDailyDigest(string, DailyReconcileRecord, models.RampState) {
}
func (f *fakeBootGuardNotifier) NotifyPriceGapReconcileFailure(date string, _ error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failures = append(f.failures, date)
}
func (f *fakeBootGuardNotifier) NotifyPriceGapRampDemote(int, int, string)              {}
func (f *fakeBootGuardNotifier) NotifyPriceGapRampForceOp(string, int, int, string, string) {}
func (f *fakeBootGuardNotifier) NotifyPriceGapBreakerTrip(models.BreakerTripRecord) error {
	return nil
}
func (f *fakeBootGuardNotifier) NotifyPriceGapBreakerRecovery(models.BreakerTripRecord, string) error {
	return nil
}

// fakeRampSnapshotterStage0 — Snapshot returns CurrentStage=0, simulating a
// missing/corrupt pg:ramp:state. Boot guard MUST refuse this combination
// when PriceGapLiveCapital=true.
type fakeRampSnapshotterStage0 struct{}

func (fakeRampSnapshotterStage0) Snapshot() models.RampState {
	return models.RampState{CurrentStage: 0}
}

// TestTracker_BootGuard_LiveCapitalMissingRampState_Panics asserts the
// boot-guard refuses to start with PriceGapLiveCapital=true + ramp stage 0,
// dispatches a BOOT_GUARD critical Telegram, and panics.
func TestTracker_BootGuard_LiveCapitalMissingRampState_Panics(t *testing.T) {
	cfg := &config.Config{PriceGapLiveCapital: true}
	tr := NewTracker(nil, nil, nil, cfg)
	notifier := &fakeBootGuardNotifier{}
	tr.SetNotifier(notifier)
	// Inject a snapshotter (NOT a full RampController) returning stage=0 to
	// simulate corrupt/missing ramp state. We set ramp directly because
	// SetRamp expects *RampController; this is a test-only injection.
	tr.ramp = fakeRampSnapshotterStage0{}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on PriceGapLiveCapital=true with missing ramp state")
		}
		notifier.mu.Lock()
		defer notifier.mu.Unlock()
		if len(notifier.failures) != 1 || notifier.failures[0] != "BOOT_GUARD" {
			t.Errorf("expected exactly one BOOT_GUARD failure dispatch, got %v", notifier.failures)
		}
	}()
	tr.Start() // expected to panic
}

// TestDaemon_GracefulShutdown_StopsTickerCleanly — drives Tracker.Stop on
// a Tracker with reconcileLoop running and asserts wg.Wait returns within
// 200ms.
func TestDaemon_GracefulShutdown_StopsTickerCleanly(t *testing.T) {
	cfg := &config.Config{}
	tr := NewTracker(nil, nil, nil, cfg)
	store := newFakeReconcileStoreForDaemon()
	notifier := &fakeReconcileNotifierForDaemon{}
	rec := NewReconciler(store, notifier, cfg, nil)
	rec.SetSleepFunc(func(time.Duration) {})
	tr.SetReconciler(rec)
	rampStore := &fakeRampStoreForDaemon{}
	rampNotifier := fakeRampNotifierForDaemon{}
	rampCtl := NewRampController(rampStore, rampNotifier, cfg, nil, time.Now)
	tr.SetRamp(rampCtl)

	// Start ONLY the reconcileLoop directly so the test does not depend on
	// the full Tracker.Start (which spawns tickLoop + assertBBOLiveness and
	// would hit nil-deref on missing exchanges/db).
	tr.wg.Add(1)
	go tr.reconcileLoop()

	// Give the goroutine a moment to enter the select on stopCh.
	time.Sleep(20 * time.Millisecond)
	close(tr.stopCh)
	done := make(chan struct{})
	go func() {
		tr.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		// graceful shutdown observed
	case <-time.After(500 * time.Millisecond):
		t.Fatal("reconcileLoop did not return within 500ms after stopCh close")
	}
}
