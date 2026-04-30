// Package pricegaptrader — ramp_controller.go is Phase 14 Plan 14-03's
// state machine for the live-capital ramp (PG-LIVE-01).
//
// State machine: 3 stages (1=100, 2=500, 3=1000 USDT/leg at v2.2 defaults).
// Promotion is slow: PriceGapCleanDaysToPromote consecutive clean days = 1
// step up. Demotion is fast: any single loss day demotes one stage AND
// zeroes the clean-day counter (asymmetric ratchet).
//
// State persistence (T-14-02 mitigation): the 5-field RampState HASH is
// written to Redis after every state-changing operation (Eval, ForcePromote,
// ForceDemote, Reset). kill -9 mid-stage + restart resumes correctly via
// SavePriceGapRampState/LoadPriceGapRampState round-trip.
//
// Module boundary (D-15): this file imports only context, fmt, sync, time,
// arb/internal/config, arb/internal/models, arb/pkg/utils. No internal/api,
// no internal/database, no internal/notify, no internal/engine,
// no internal/spotengine.
package pricegaptrader

import (
	"context"
	"fmt"
	"sync"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/utils"
)

// RampStateStore — narrow interface for RampController persistence (D-15
// boundary). *database.Client satisfies via the methods added in Plan 14-01.
type RampStateStore interface {
	SavePriceGapRampState(s models.RampState) error
	LoadPriceGapRampState() (models.RampState, bool, error)
	AppendPriceGapRampEvent(ev models.RampEvent) error
}

// RampNotifier — narrow interface for telegram notifications surfaced by the
// ramp state machine. Plan 14-04 wires *notify.TelegramNotifier; tests use
// fakeRampNotifier. All methods MUST be nil-receiver-safe on the impl side.
type RampNotifier interface {
	NotifyPriceGapRampDemote(prior, next int, reason string)
	NotifyPriceGapRampForceOp(action string, prior, next int, operator, reason string)
}

// RampController is the live-capital ramp state machine. All mutation
// operations are serialized by mu (single ramp instance per process; daemon
// + pg-admin invoke through the same controller via the wired daemon —
// concurrent mutation across processes is not supported in v2.2).
type RampController struct {
	mu       sync.Mutex
	cfg      *config.Config
	log      *utils.Logger
	store    RampStateStore
	notifier RampNotifier
	nowFn    func() time.Time
	state    models.RampState
}

// NewRampController constructs a ramp controller and bootstraps state from
// the store. On empty/legacy state it materializes (CurrentStage=1, counter=0)
// and persists it (T-14-12 mitigation: ramp never assumes a stage > 1 from
// missing state).
//
// log MAY be nil — the controller does not crash but warning lines will be
// silently swallowed. Production callers always supply a real logger.
func NewRampController(
	store RampStateStore,
	notifier RampNotifier,
	cfg *config.Config,
	log *utils.Logger,
	nowFn func() time.Time,
) *RampController {
	if log == nil {
		log = utils.NewLogger("pg-ramp")
	}
	if nowFn == nil {
		nowFn = time.Now
	}
	rc := &RampController{
		store:    store,
		notifier: notifier,
		cfg:      cfg,
		log:      log,
		nowFn:    nowFn,
	}
	rc.bootstrapState()
	return rc
}

// bootstrapState loads existing state from the store; on miss/legacy it
// materializes (stage=1, counter=0) and persists. Called once from the
// constructor.
func (rc *RampController) bootstrapState() {
	s, exists, err := rc.store.LoadPriceGapRampState()
	if err != nil {
		// Don't crash; surface and assume fresh state. The next state-changing
		// operation will persist whatever we end up with.
		rc.log.Warn("pg-ramp: load state failed: %v", err)
	}
	if !exists || s.CurrentStage < 1 {
		// T-14-12: legacy/empty/corrupt → bootstrap to stage 1.
		s = models.RampState{CurrentStage: 1, CleanDayCounter: 0}
		if err := rc.store.SavePriceGapRampState(s); err != nil {
			rc.log.Warn("pg-ramp: bootstrap save failed: %v", err)
		}
	}
	rc.state = s
}

// Snapshot returns a copy of the current ramp state. Safe for concurrent
// reads.
func (rc *RampController) Snapshot() models.RampState {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.state
}

// Eval drives ramp transitions from a single day's reconcile record. The
// caller (typically the Phase 14 daemon at UTC 00:30) is responsible for
// loading the previous day's record and passing it here.
//
// Idempotency: re-evaluating the same date is a no-op (LastEvalTs day-key
// guard prevents double-counting).
func (rc *RampController) Eval(ctx context.Context, date string, record DailyReconcileRecord) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	evalDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		rc.log.Warn("pg-ramp: bad date %q: %v", date, err)
		return
	}
	// Idempotency: same-date reentry → no-op. Compares the YYYY-MM-DD prefix
	// of LastEvalTs against the requested date.
	if !rc.state.LastEvalTs.IsZero() && rc.state.LastEvalTs.Format("2006-01-02") == date {
		return
	}

	prior := rc.state.CurrentStage

	// D-05: no-activity day (PositionsClosed == 0) → HOLD. State is unchanged
	// EXCEPT we still update LastEvalTs so subsequent same-date calls no-op.
	if record.Totals.PositionsClosed == 0 {
		rc.state.LastEvalTs = evalDate
		_ = rc.persistLocked("evaluate", prior, prior, "no_activity_hold")
		return
	}

	if record.Totals.NetClean {
		rc.state.CleanDayCounter++
		// Promotion threshold reached AND not already at max stage.
		if rc.state.CurrentStage < 3 && rc.cfg != nil && rc.state.CleanDayCounter >= rc.cfg.PriceGapCleanDaysToPromote {
			rc.state.CurrentStage++
			rc.state.CleanDayCounter = 0
		}
	} else {
		// Loss day → asymmetric ratchet: counter=0 + demote one stage if
		// above stage 1.
		rc.state.CleanDayCounter = 0
		rc.state.LastLossDayTs = evalDate
		if rc.state.CurrentStage > 1 {
			demotedFrom := rc.state.CurrentStage
			rc.state.CurrentStage--
			rc.state.DemoteCount++
			if rc.notifier != nil {
				rc.notifier.NotifyPriceGapRampDemote(demotedFrom, rc.state.CurrentStage,
					fmt.Sprintf("loss_day pnl=%.2f", record.Totals.RealizedPnLUSDT))
			}
		}
	}
	rc.state.LastEvalTs = evalDate

	reason := "clean_day"
	if !record.Totals.NetClean {
		reason = "loss_day"
	}
	_ = rc.persistLocked("evaluate", prior, rc.state.CurrentStage, reason)
}

// ForcePromote bumps the stage by one (max 3). Operator override is
// intentional — the clean-day counter is PRESERVED (D-15 #3).
func (rc *RampController) ForcePromote(operator, reason string) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	prior := rc.state.CurrentStage
	if prior >= 3 {
		return fmt.Errorf("ramp: already at max stage 3")
	}
	rc.state.CurrentStage++
	// D-15 #3: counter PRESERVED — operator override is intentional.
	if err := rc.persistLocked("force_promote", prior, rc.state.CurrentStage, fmt.Sprintf("op=%s reason=%s", operator, reason)); err != nil {
		return err
	}
	if rc.notifier != nil {
		rc.notifier.NotifyPriceGapRampForceOp("force_promote", prior, rc.state.CurrentStage, operator, reason)
	}
	return nil
}

// ForceDemote drops the stage by one (min 1). D-15 #4: counter is zeroed and
// DemoteCount incremented (matches the asymmetric-ratchet contract).
func (rc *RampController) ForceDemote(operator, reason string) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	prior := rc.state.CurrentStage
	if prior <= 1 {
		return fmt.Errorf("ramp: already at min stage 1")
	}
	rc.state.CurrentStage--
	// D-15 #4: counter zeroed (matches asymmetric ratchet).
	rc.state.CleanDayCounter = 0
	rc.state.DemoteCount++
	if err := rc.persistLocked("force_demote", prior, rc.state.CurrentStage, fmt.Sprintf("op=%s reason=%s", operator, reason)); err != nil {
		return err
	}
	if rc.notifier != nil {
		rc.notifier.NotifyPriceGapRampForceOp("force_demote", prior, rc.state.CurrentStage, operator, reason)
	}
	return nil
}

// Reset rolls state to (stage=1, counter=0). DemoteCount and timestamps are
// also cleared. D-15 #5.
func (rc *RampController) Reset(operator, reason string) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	prior := rc.state.CurrentStage
	rc.state = models.RampState{CurrentStage: 1, CleanDayCounter: 0}
	if err := rc.persistLocked("reset", prior, 1, fmt.Sprintf("op=%s reason=%s", operator, reason)); err != nil {
		return err
	}
	if rc.notifier != nil {
		rc.notifier.NotifyPriceGapRampForceOp("reset", prior, 1, operator, reason)
	}
	return nil
}

// persistLocked writes the current state to the store and appends a RampEvent
// to the events list. Caller MUST hold rc.mu. Returns the SavePriceGapRampState
// error (Append errors are logged best-effort and swallowed).
func (rc *RampController) persistLocked(action string, prior, next int, reason string) error {
	if err := rc.store.SavePriceGapRampState(rc.state); err != nil {
		rc.log.Error("pg-ramp: SavePriceGapRampState failed: %v", err)
		return err
	}
	ev := models.RampEvent{
		Action:     action,
		PriorStage: prior,
		NewStage:   next,
		Operator:   "daemon",
		Timestamp:  rc.nowFn().UTC(),
		Reason:     reason,
	}
	if err := rc.store.AppendPriceGapRampEvent(ev); err != nil {
		rc.log.Warn("pg-ramp: AppendPriceGapRampEvent failed: %v", err)
	}
	return nil
}
