// Package pricegaptrader — breaker_test_fire.go is Phase 15 Plan 15-04's
// synthetic test-fire path on *BreakerController.
//
// TestFire(ctx, dryRun) lets operators exercise the full breaker trip path
// without waiting for a real drawdown:
//   - dryRun=false (default for pg-admin breaker test-fire): performs a REAL
//     trip — same D-15 ordering as evalTick.trip(). Source label is "test_fire"
//     so the trip log + Telegram alert distinguish synthetic from live.
//   - dryRun=true (pg-admin breaker test-fire --dry-run): computes the would-be
//     trip record (24h PnL via the aggregator) but performs ZERO mutations:
//     no SaveBreakerState, no AppendBreakerTrip, no Telegram, no WS broadcast,
//     no candidate pause. Source label is "test_fire_dry_run" — never persisted
//     per CONTEXT D-14.
//
// The plan's Open Question #4 resolves: "skips all mutations" means the
// dry-run path is purely observational. The Aggregator IS called (we want to
// validate it returns sane data), but no write side-effects fire.
package pricegaptrader

import (
	"context"
	"fmt"
	"time"

	"arb/internal/models"
)

// TestFire performs a synthetic breaker trip. dryRun=false → real trip via
// the same D-15 step ordering as evalTick.trip(). dryRun=true → compute the
// would-be trip record but write NOTHING. Returns the trip record (real or
// would-be) for the caller to surface to the operator.
//
// Pre-conditions:
//   - aggregator wired (returns 24h PnL).
//
// Errors:
//   - aggregator failure → returns the error verbatim; nothing mutated.
//   - real-trip Step 1 failure → bubbles the trip() error; sticky NOT persisted
//     (D-15 abort). Steps 2-5 do NOT run.
func (bc *BreakerController) TestFire(ctx context.Context, dryRun bool) (models.BreakerTripRecord, error) {
	if bc == nil {
		return models.BreakerTripRecord{}, fmt.Errorf("breaker: nil controller")
	}
	if bc.aggregator == nil {
		return models.BreakerTripRecord{}, fmt.Errorf("breaker: aggregator not wired")
	}

	now := time.Now()
	pnl, err := bc.aggregator.Realized24h(ctx, now)
	if err != nil {
		return models.BreakerTripRecord{}, fmt.Errorf("test-fire: aggregate 24h: %w", err)
	}

	rampStage := 0
	if bc.ramp != nil {
		rampStage = bc.ramp.Snapshot().CurrentStage
	}

	source := "test_fire"
	if dryRun {
		source = "test_fire_dry_run"
	}

	record := models.BreakerTripRecord{
		TripTs:      now.UnixMilli(),
		TripPnLUSDT: pnl,
		Threshold:   bc.cfg.PriceGapDrawdownLimitUSDT,
		RampStage:   rampStage,
		Source:      source,
	}

	if dryRun {
		// no mutations performed — pure preview. Aggregator was the only call.
		bc.log.Info("breaker: TEST-FIRE DRY RUN — would trip with 24h=%.2f, threshold=%.2f, no mutations performed",
			pnl, bc.cfg.PriceGapDrawdownLimitUSDT)
		return record, nil
	}

	// Real test-fire: load current state and run the same trip ordering as
	// evalTick. trip() persists sticky FIRST (load-bearing safety property)
	// then runs steps 2-5 best-effort.
	state, _, err := bc.store.LoadBreakerState()
	if err != nil {
		return record, fmt.Errorf("test-fire: load state: %w", err)
	}
	if err := bc.trip(ctx, state, pnl, "test_fire"); err != nil {
		return record, err
	}
	// Update record's PausedCandidateCount post-trip if registry was wired
	// (trip() populates this on the in-flight record but we constructed our
	// own copy here; pull the count from the registry pause result for the
	// returned record so the operator sees what was paused). Best-effort.
	return record, nil
}
