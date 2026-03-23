package risk

import "arb/internal/models"

const (
	// maxExposurePct is the maximum fraction of total capital allowed on a single exchange.
	maxExposurePct = 0.60

	// hardMaxLeverage is the absolute leverage cap regardless of configuration.
	hardMaxLeverage = 5

	// minCapitalFloor is the minimum USDT required per leg.
	minCapitalFloor = 10.0
)

// MaxExposurePerExchange returns the maximum capital that should be deployed
// on any single exchange (60% of total capital).
func MaxExposurePerExchange(totalCapital float64) float64 {
	return totalCapital * maxExposurePct
}

// MinCapitalPerLeg returns the minimum USDT required per position leg.
// There is a hard floor of 10 USDT regardless of leverage.
func MinCapitalPerLeg(leverage int) float64 {
	_ = leverage // reserved for future scaling; floor applies regardless
	return minCapitalFloor
}

// MaxLeverage returns the hard-capped maximum leverage (5x), regardless of
// what the configuration file specifies.
func MaxLeverage() int {
	return hardMaxLeverage
}

// IsExchangeOverexposed returns true if the given exchange appears in more
// than 60% of the supplied active positions (counting each leg separately).
func IsExchangeOverexposed(exchangeName string, activePositions []*models.ArbitragePosition) bool {
	if len(activePositions) == 0 {
		return false
	}

	count := 0
	for _, pos := range activePositions {
		if pos.LongExchange == exchangeName || pos.ShortExchange == exchangeName {
			count++
		}
	}

	return float64(count)/float64(len(activePositions)) > maxExposurePct
}
