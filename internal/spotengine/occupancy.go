package spotengine

import "arb/internal/models"

func (e *SpotEngine) loadUnifiedAdmission() (int, int, map[string]struct{}, error) {
	occupied := make(map[string]struct{})

	perpActive, err := e.db.GetActivePositions()
	if err != nil {
		return 0, 0, nil, err
	}
	activePerp := 0
	for _, pos := range perpActive {
		if pos == nil || pos.Status == models.StatusClosed {
			continue
		}
		activePerp++
		if pos.Symbol != "" {
			occupied[pos.Symbol] = struct{}{}
		}
	}

	spotActive, err := e.db.GetActiveSpotPositions()
	if err != nil {
		return 0, 0, nil, err
	}
	activeSpot := 0
	for _, pos := range spotActive {
		if pos == nil || pos.Status == models.SpotStatusClosed {
			continue
		}
		activeSpot++
		if pos.Symbol != "" {
			occupied[pos.Symbol] = struct{}{}
		}
	}

	return activePerp, activeSpot, occupied, nil
}
