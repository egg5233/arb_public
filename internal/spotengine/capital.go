package spotengine

import (
	"fmt"

	"arb/internal/risk"
)

func (e *SpotEngine) reserveSpotCapital(exchangeName string, amount float64) (*risk.CapitalReservation, error) {
	if e.allocator == nil || !e.allocator.Enabled() {
		return nil, nil
	}
	return e.allocator.Reserve(risk.StrategySpotFutures, map[string]float64{
		exchangeName: amount,
	})
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
