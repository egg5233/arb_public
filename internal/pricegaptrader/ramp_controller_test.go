package pricegaptrader

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/utils"
)

// Phase 14 Plan 14-03 — RampController tests. The 7 tests below lock the
// asymmetric ratchet (PG-LIVE-01) + kill-nine persistence (T-14-02) +
// no-activity hold (D-05) + force-op semantics (D-15 #3, #4, #5).

// ---- Fakes -----------------------------------------------------------------

// fakeRampStore is the RampStateStore + RampEventSink fake. It records every
// Save/Load/AppendEvent call so tests can assert persistence correctness.
type fakeRampStore struct {
	state     models.RampState
	exists    bool
	saveCalls int
	saveErr   error
	loadErr   error
	events    []models.RampEvent
	appendErr error
}

func (f *fakeRampStore) SavePriceGapRampState(s models.RampState) error {
	f.saveCalls++
	if f.saveErr != nil {
		return f.saveErr
	}
	f.state = s
	f.exists = true
	return nil
}

func (f *fakeRampStore) LoadPriceGapRampState() (models.RampState, bool, error) {
	if f.loadErr != nil {
		return models.RampState{}, false, f.loadErr
	}
	return f.state, f.exists, nil
}

func (f *fakeRampStore) AppendPriceGapRampEvent(ev models.RampEvent) error {
	if f.appendErr != nil {
		return f.appendErr
	}
	f.events = append(f.events, ev)
	return nil
}

// fakeRampNotifier records every demote / force-op invocation. Tests can
// assert presence/absence via len(demoteCalls) / len(forceOpCalls).
type fakeRampNotifier struct {
	demoteCalls  []demoteRecord
	forceOpCalls []forceOpRecord
}

type demoteRecord struct {
	prior, next int
	reason      string
}

type forceOpRecord struct {
	action            string
	prior, next       int
	operator, reason  string
}

func (f *fakeRampNotifier) NotifyPriceGapRampDemote(prior, next int, reason string) {
	f.demoteCalls = append(f.demoteCalls, demoteRecord{prior: prior, next: next, reason: reason})
}

func (f *fakeRampNotifier) NotifyPriceGapRampForceOp(action string, prior, next int, operator, reason string) {
	f.forceOpCalls = append(f.forceOpCalls, forceOpRecord{
		action: action, prior: prior, next: next, operator: operator, reason: reason,
	})
}

// rampTestCfg returns a baseline config with v2.2 defaults + live capital ON.
func rampTestCfg() *config.Config {
	return &config.Config{
		PriceGapLiveCapital:        true,
		PriceGapStage1SizeUSDT:     100,
		PriceGapStage2SizeUSDT:     500,
		PriceGapStage3SizeUSDT:     1000,
		PriceGapHardCeilingUSDT:    1000,
		PriceGapCleanDaysToPromote: 7,
	}
}

// fixedClock returns a now func that always returns ts.
func fixedClock(ts time.Time) func() time.Time {
	return func() time.Time { return ts }
}

// cleanRecord returns a DailyReconcileRecord representing a clean day with
// PnL=pnl USDT and one position closed.
func cleanRecord(pnl float64) DailyReconcileRecord {
	return DailyReconcileRecord{
		Totals: DailyReconcileTotals{
			RealizedPnLUSDT: pnl,
			PositionsClosed: 1,
			NetClean:        true,
		},
	}
}

// lossRecord returns a DailyReconcileRecord representing a loss day.
func lossRecord(pnl float64) DailyReconcileRecord {
	return DailyReconcileRecord{
		Totals: DailyReconcileTotals{
			RealizedPnLUSDT: pnl,
			PositionsClosed: 1,
			NetClean:        false,
		},
	}
}

// ---- Tests -----------------------------------------------------------------

// TestRampController_AsymmetricRatchetInvariant — PG-LIVE-01 invariant lock.
// Promotion is slow (7 clean days = 1 step up); demotion is immediate (1 loss
// day = counter to 0 + step down). This test must never go symmetric.
func TestRampController_AsymmetricRatchetInvariant(t *testing.T) {
	cfg := rampTestCfg()
	store := &fakeRampStore{state: models.RampState{CurrentStage: 1}, exists: true}
	notifier := &fakeRampNotifier{}
	rc := NewRampController(store, notifier, cfg, utils.NewLogger("ramp-test"),
		fixedClock(time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC)))

	// 7 clean days → stage 1 → 2 (counter resets on promotion).
	for i := 1; i <= 7; i++ {
		rc.Eval(context.Background(), fmt.Sprintf("2026-04-%02d", i), cleanRecord(1.0))
	}
	if got := rc.Snapshot().CurrentStage; got != 2 {
		t.Fatalf("after 7 clean days expected stage=2, got %d", got)
	}
	if got := rc.Snapshot().CleanDayCounter; got != 0 {
		t.Fatalf("after promotion counter should reset to 0, got %d", got)
	}

	// 1 loss day at stage 2 → counter=0, stage=1 (asymmetric demote).
	rc.Eval(context.Background(), "2026-05-08", lossRecord(-1.0))
	if rc.Snapshot().CurrentStage != 1 || rc.Snapshot().CleanDayCounter != 0 {
		t.Fatalf("loss day should demote AND zero counter, got stage=%d counter=%d",
			rc.Snapshot().CurrentStage, rc.Snapshot().CleanDayCounter)
	}
	if len(notifier.demoteCalls) != 1 {
		t.Fatalf("loss-day demote should fire NotifyPriceGapRampDemote once, got %d", len(notifier.demoteCalls))
	}

	// 1 loss day at stage 1 → stage stays at 1 (can't go below 1); counter still 0.
	rc.Eval(context.Background(), "2026-05-09", lossRecord(-1.0))
	if rc.Snapshot().CurrentStage != 1 {
		t.Fatalf("stage should not go below 1, got %d", rc.Snapshot().CurrentStage)
	}
}

// TestRampController_KillNinePersistence — T-14-02 mitigation lock.
// Drives state to (stage=1, counter=4), then constructs a fresh controller
// from the SAME store and asserts the state survives (Save+Load round-trip).
func TestRampController_KillNinePersistence(t *testing.T) {
	cfg := rampTestCfg()
	store := &fakeRampStore{}
	rc1 := NewRampController(store, &fakeRampNotifier{}, cfg, utils.NewLogger("ramp-test"),
		fixedClock(time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC)))

	// 4 clean days (below threshold of 7 — counter should sit at 4, no promote).
	for i := 1; i <= 4; i++ {
		rc1.Eval(context.Background(), fmt.Sprintf("2026-04-%02d", i), cleanRecord(1.0))
	}
	snap1 := rc1.Snapshot()
	if snap1.CleanDayCounter != 4 {
		t.Fatalf("counter=%d, want 4", snap1.CleanDayCounter)
	}
	if snap1.CurrentStage != 1 {
		t.Fatalf("stage=%d, want 1 (still under promote threshold)", snap1.CurrentStage)
	}

	// Simulate kill -9 + restart: fresh controller, same store.
	rc2 := NewRampController(store, &fakeRampNotifier{}, cfg, utils.NewLogger("ramp-test"),
		fixedClock(time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC)))
	snap2 := rc2.Snapshot()
	if snap2.CurrentStage != snap1.CurrentStage {
		t.Fatalf("post-restart stage diverged: before=%d after=%d", snap1.CurrentStage, snap2.CurrentStage)
	}
	if snap2.CleanDayCounter != snap1.CleanDayCounter {
		t.Fatalf("post-restart counter diverged: before=%d after=%d", snap1.CleanDayCounter, snap2.CleanDayCounter)
	}
}

// TestRampController_HardCeilingFloor — at max stage, additional clean days
// do not promote further; counter does NOT auto-reset on hitting max stage.
// (D-05 spirit: continued accumulation of clean days at max stage is benign.)
func TestRampController_HardCeilingFloor(t *testing.T) {
	cfg := rampTestCfg()
	// Bootstrap controller already at stage=3 with counter=6 (one short of
	// the promote threshold of 7).
	store := &fakeRampStore{
		state:  models.RampState{CurrentStage: 3, CleanDayCounter: 6},
		exists: true,
	}
	rc := NewRampController(store, &fakeRampNotifier{}, cfg, utils.NewLogger("ramp-test"),
		fixedClock(time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC)))

	// One more clean day → would be the 7th, but we are already at max stage.
	rc.Eval(context.Background(), "2026-04-30", cleanRecord(1.0))
	if rc.Snapshot().CurrentStage != 3 {
		t.Fatalf("at max stage 3 — must stay at 3, got %d", rc.Snapshot().CurrentStage)
	}
	// Implementations may either hold or increment counter at max stage; both
	// are benign per D-05 (no further promotion). The invariant is that the
	// stage does not change.
	if rc.Snapshot().CleanDayCounter < 6 {
		t.Fatalf("counter should not regress from 6 at max stage, got %d", rc.Snapshot().CleanDayCounter)
	}
}

// TestRampController_NoActivityHoldsCounter — D-05 lock. A reconcile record
// with PositionsClosed=0 means no trades that day → state is HELD (no
// increment, no decrement; LastEvalTs may update).
func TestRampController_NoActivityHoldsCounter(t *testing.T) {
	cfg := rampTestCfg()
	store := &fakeRampStore{
		state:  models.RampState{CurrentStage: 1, CleanDayCounter: 3},
		exists: true,
	}
	rc := NewRampController(store, &fakeRampNotifier{}, cfg, utils.NewLogger("ramp-test"),
		fixedClock(time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC)))

	noActivity := DailyReconcileRecord{
		Totals: DailyReconcileTotals{
			RealizedPnLUSDT: 0,
			PositionsClosed: 0,
			NetClean:        false,
		},
	}
	rc.Eval(context.Background(), "2026-04-30", noActivity)
	if rc.Snapshot().CurrentStage != 1 {
		t.Fatalf("no-activity day must not change stage, got %d", rc.Snapshot().CurrentStage)
	}
	if rc.Snapshot().CleanDayCounter != 3 {
		t.Fatalf("no-activity day must HOLD counter at 3, got %d", rc.Snapshot().CleanDayCounter)
	}
}

// TestRampController_ForcePromoteDoesNotZeroCounter — D-15 #3 lock. Operator
// override is intentional; counter must be PRESERVED.
func TestRampController_ForcePromoteDoesNotZeroCounter(t *testing.T) {
	cfg := rampTestCfg()
	store := &fakeRampStore{
		state:  models.RampState{CurrentStage: 1, CleanDayCounter: 4},
		exists: true,
	}
	notifier := &fakeRampNotifier{}
	rc := NewRampController(store, notifier, cfg, utils.NewLogger("ramp-test"),
		fixedClock(time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC)))

	if err := rc.ForcePromote("operator", "manual override"); err != nil {
		t.Fatalf("ForcePromote returned err=%v", err)
	}
	snap := rc.Snapshot()
	if snap.CurrentStage != 2 {
		t.Fatalf("ForcePromote 1→2 should land at stage=2, got %d", snap.CurrentStage)
	}
	if snap.CleanDayCounter != 4 {
		t.Fatalf("ForcePromote MUST preserve counter (D-15 #3), got %d (want 4)", snap.CleanDayCounter)
	}
	// Event recorded with action=force_promote.
	if len(store.events) == 0 {
		t.Fatalf("ForcePromote must AppendPriceGapRampEvent, got 0 events")
	}
	last := store.events[len(store.events)-1]
	if last.Action != "force_promote" {
		t.Fatalf("expected event action=force_promote, got %q", last.Action)
	}
}

// TestRampController_ForceDemoteZeroesCounter — D-15 #4 lock. Force-demote
// matches the asymmetric ratchet — counter zeroed, demoteCount incremented.
func TestRampController_ForceDemoteZeroesCounter(t *testing.T) {
	cfg := rampTestCfg()
	store := &fakeRampStore{
		state:  models.RampState{CurrentStage: 2, CleanDayCounter: 5, DemoteCount: 0},
		exists: true,
	}
	rc := NewRampController(store, &fakeRampNotifier{}, cfg, utils.NewLogger("ramp-test"),
		fixedClock(time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC)))

	if err := rc.ForceDemote("operator", "ops review"); err != nil {
		t.Fatalf("ForceDemote returned err=%v", err)
	}
	snap := rc.Snapshot()
	if snap.CurrentStage != 1 {
		t.Fatalf("ForceDemote 2→1 should land at stage=1, got %d", snap.CurrentStage)
	}
	if snap.CleanDayCounter != 0 {
		t.Fatalf("ForceDemote MUST zero counter (D-15 #4), got %d", snap.CleanDayCounter)
	}
	if snap.DemoteCount != 1 {
		t.Fatalf("ForceDemote should increment DemoteCount, got %d", snap.DemoteCount)
	}
	if len(store.events) == 0 || store.events[len(store.events)-1].Action != "force_demote" {
		t.Fatalf("ForceDemote must AppendPriceGapRampEvent with action=force_demote")
	}
}

// TestRampController_ResetZeroesCounterAndStage1 — D-15 #5 lock.
func TestRampController_ResetZeroesCounterAndStage1(t *testing.T) {
	cfg := rampTestCfg()
	store := &fakeRampStore{
		state:  models.RampState{CurrentStage: 3, CleanDayCounter: 6, DemoteCount: 2},
		exists: true,
	}
	rc := NewRampController(store, &fakeRampNotifier{}, cfg, utils.NewLogger("ramp-test"),
		fixedClock(time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC)))

	if err := rc.Reset("operator", "factory reset"); err != nil {
		t.Fatalf("Reset returned err=%v", err)
	}
	snap := rc.Snapshot()
	if snap.CurrentStage != 1 {
		t.Fatalf("Reset must set stage=1, got %d", snap.CurrentStage)
	}
	if snap.CleanDayCounter != 0 {
		t.Fatalf("Reset must zero counter, got %d", snap.CleanDayCounter)
	}
	if len(store.events) == 0 || store.events[len(store.events)-1].Action != "reset" {
		t.Fatalf("Reset must AppendPriceGapRampEvent with action=reset")
	}
}

// Compile-time assertion: ensure fakeRampStore + fakeRampNotifier satisfy the
// expected interfaces declared by ramp_controller.go.
var _ RampStateStore = (*fakeRampStore)(nil)
var _ RampNotifier = (*fakeRampNotifier)(nil)

// errIgnore — silence unused-import warnings during initial RED phase if any.
var _ = errors.New
