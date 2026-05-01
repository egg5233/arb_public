// Package pricegaptrader — breaker_recovery.go is Phase 15 Plan 15-04's
// operator-driven recovery path on *BreakerController.
//
// Recover(ctx, operator) is the inverse of trip(). Where trip persists sticky
// FIRST (load-bearing safety: any partial failure leaves the engine in
// safe-paper mode), Recover persists sticky LAST so any partial failure
// earlier leaves the breaker still tripped — also the safer state.
//
// 5-step ordering:
//   STEP 1 (best-effort): ClearAllPausedByBreaker via Registry chokepoint.
//                         Operator-set Disabled state is NOT touched (D-11).
//   STEP 2 (best-effort): UpdateBreakerTripRecovery LSet at index 0 — backfills
//                         RecoveryTs + RecoveryOperator on the most-recent trip
//                         record so the audit trail captures who recovered.
//   STEP 3 (CRITICAL):   SaveBreakerState with PaperModeStickyUntil=0 +
//                         PendingStrike=0 + Strike1Ts=0. Failure here returns
//                         an error; engine remains in paper mode.
//   STEP 4 (best-effort): NotifyPriceGapBreakerRecovery via Telegram critical
//                         bucket. Failure logged.
//   STEP 5 (best-effort): WS broadcast "pg.breaker.recover".
//
// Pre-condition: state.PaperModeStickyUntil != 0 (breaker actually tripped).
// Calling Recover when sticky=0 returns an explicit error — operator cannot
// recover what was not tripped.
package pricegaptrader

import (
	"context"
	"fmt"
	"strings"
	"time"

	"arb/internal/models"
)

// Recover clears the sticky paper-mode flag, un-pauses all candidates that
// were paused by the breaker, and dispatches the recovery alert. Operator-set
// Disabled state is preserved (chokepoint only flips PausedByBreaker).
//
// Returns an explicit error if the breaker is not tripped (sticky=0). Returns
// the SaveBreakerState error if Step 3 fails (engine remains in paper mode).
// Other steps are best-effort — failures are logged but do not abort.
func (bc *BreakerController) Recover(ctx context.Context, operator string) error {
	_ = ctx // reserved for future ctx-aware Redis path
	if bc == nil {
		return fmt.Errorf("breaker: nil controller")
	}
	if bc.store == nil {
		return fmt.Errorf("breaker: store not wired")
	}

	op := strings.TrimSpace(operator)
	if op == "" {
		op = "operator"
	}

	state, _, err := bc.store.LoadBreakerState()
	if err != nil {
		return fmt.Errorf("recover: load state: %w", err)
	}
	if state.PaperModeStickyUntil == 0 {
		return fmt.Errorf("breaker not tripped (sticky=0)")
	}

	recoveryTs := time.Now().UnixMilli()

	// STEP 1 (best-effort): un-pause candidates via chokepoint.
	if bc.registry != nil {
		count, cerr := bc.registry.ClearAllPausedByBreaker()
		if cerr != nil {
			bc.log.Warn("breaker: RECOVER step 1 ClearAllPausedByBreaker: %v", cerr)
		} else if count > 0 {
			bc.log.Info("breaker: recover cleared paused_by_breaker on %d candidate(s)", count)
		}
	}

	// STEP 2 (best-effort): backfill recovery fields on most-recent trip record.
	if uerr := bc.store.UpdateBreakerTripRecovery(0, recoveryTs, op); uerr != nil {
		bc.log.Warn("breaker: RECOVER step 2 UpdateBreakerTripRecovery: %v", uerr)
	}

	// Best-effort load of most-recent trip for the alert payload (post-update
	// so the loaded record carries the recovery fields).
	var lastTrip models.BreakerTripRecord
	if rec, exists, lerr := bc.store.LoadBreakerTripAt(0); lerr != nil {
		bc.log.Warn("breaker: RECOVER LoadBreakerTripAt(0): %v", lerr)
	} else if exists {
		lastTrip = rec
	}

	// STEP 3 (CRITICAL): clear sticky LAST. Failure here aborts; engine
	// remains in paper mode (the safer state).
	state.PaperModeStickyUntil = 0
	state.PendingStrike = 0
	state.Strike1Ts = 0
	if serr := bc.store.SaveBreakerState(state); serr != nil {
		return fmt.Errorf("recover step 3 (save sticky=0): %w", serr)
	}

	// STEP 4 (best-effort): Telegram critical-bucket recovery alert.
	if bc.notifier != nil {
		if nerr := bc.notifier.NotifyPriceGapBreakerRecovery(lastTrip, op); nerr != nil {
			bc.log.Warn("breaker: RECOVER step 4 NotifyPriceGapBreakerRecovery: %v", nerr)
		}
	}

	// STEP 5 (best-effort): WS broadcast.
	if bc.ws != nil {
		bc.ws.BroadcastPriceGapBreakerEvent("pg.breaker.recover", map[string]any{
			"operator":        op,
			"recovery_ts_ms":  recoveryTs,
			"trip":            lastTrip,
		})
	}

	bc.log.Info("breaker: RECOVERED — sticky cleared, candidates re-enabled, operator=%s", op)
	return nil
}
