package spotengine

import (
	"fmt"

	"arb/internal/risk"
)

// reserveSpotCapital reserves capital for a spot-futures entry via the allocator.
// When capOverride > 0 it is forwarded to risk.ReserveWithCap (CA-04 dynamic
// shifting); capOverride == 0 means no override (same semantics as Reserve).
// Returns (nil, nil) when the allocator is disabled/absent so legacy callers
// that don't run under the allocator are unaffected.
func (e *SpotEngine) reserveSpotCapital(exchangeName string, amount float64, capOverride float64) (*risk.CapitalReservation, error) {
	if e.allocator == nil || !e.allocator.Enabled() {
		return nil, nil
	}
	return e.allocator.ReserveWithCap(risk.StrategySpotFutures, map[string]float64{
		exchangeName: amount,
	}, capOverride)
}

func (e *SpotEngine) commitSpotCapital(res *risk.CapitalReservation, posID string, amount float64) error {
	if e.allocator == nil || !e.allocator.Enabled() || res == nil {
		return nil
	}
	var exchangeName string
	for name := range res.Exposures {
		exchangeName = name
		break
	}
	if exchangeName == "" {
		return nil
	}
	if amount <= 0 {
		pos, err := e.db.GetSpotPosition(posID)
		if err != nil {
			return fmt.Errorf("load spot position for capital commit: %w", err)
		}
		amount = pos.NotionalUSDT
	}
	return e.allocator.Commit(res, posID, map[string]float64{
		exchangeName: amount,
	})
}

// commitSpotFromPreheld commits a preheld batch reservation supplied by the
// unified entry selector. It does not re-fetch the position notional from
// Redis — the amount must be the authoritative planned notional the selector
// used when reserving. Symmetrical to the perp-side commitExistingReservation.
func (e *SpotEngine) commitSpotFromPreheld(res *risk.CapitalReservation, posID string, amount float64) error {
	if e.allocator == nil || !e.allocator.Enabled() || res == nil {
		return nil
	}
	if posID == "" {
		return fmt.Errorf("commitSpotFromPreheld: empty posID")
	}
	if amount <= 0 {
		return fmt.Errorf("commitSpotFromPreheld: non-positive amount %.6f", amount)
	}
	var exchangeName string
	for name := range res.Exposures {
		exchangeName = name
		break
	}
	if exchangeName == "" {
		return fmt.Errorf("commitSpotFromPreheld: reservation has no exchange exposure")
	}
	return e.allocator.Commit(res, posID, map[string]float64{
		exchangeName: amount,
	})
}

func (e *SpotEngine) releaseSpotReservation(res *risk.CapitalReservation) {
	if e.allocator == nil || !e.allocator.Enabled() || res == nil {
		return
	}
	if err := e.allocator.ReleaseReservation(res.ID); err != nil {
		e.log.Error("allocator release reservation %s: %v", res.ID, err)
	}
}

func (e *SpotEngine) releaseSpotPosition(posID string) {
	if e.allocator == nil || !e.allocator.Enabled() || posID == "" {
		return
	}
	if err := e.allocator.ReleasePosition(posID); err != nil {
		e.log.Error("allocator release position %s: %v", posID, err)
	}
}

func (e *SpotEngine) updateSpotPositionCapital(posID, exchangeName string, amount float64) {
	if e.allocator == nil || !e.allocator.Enabled() || posID == "" || exchangeName == "" {
		return
	}
	if err := e.allocator.UpdatePosition(posID, map[string]float64{
		exchangeName: amount,
	}); err != nil {
		e.log.Error("allocator update position %s: %v", posID, err)
	}
}
