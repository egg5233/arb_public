package risk

import (
	"fmt"
	"math"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
)

const spreadHistoryRiskLimit = 200

// SpreadStabilityChecker enforces a Redis-backed spread CV gate at entry time.
type SpreadStabilityChecker struct {
	db              *database.Client
	cfg             *config.Config
	historyProvider func(models.Opportunity, int, time.Duration) []database.SpreadHistoryPoint
}

func NewSpreadStabilityChecker(db *database.Client, cfg *config.Config) *SpreadStabilityChecker {
	return &SpreadStabilityChecker{
		db:  db,
		cfg: cfg,
	}
}

func (c *SpreadStabilityChecker) SetHistoryProvider(provider func(models.Opportunity, int, time.Duration) []database.SpreadHistoryPoint) {
	if c == nil {
		return
	}
	c.historyProvider = provider
}

// Check returns a rejection reason when spread history is too volatile.
func (c *SpreadStabilityChecker) Check(opp models.Opportunity, automated bool) (string, error) {
	if c == nil || c.db == nil || c.cfg == nil {
		return "", nil
	}
	if !c.cfg.EnableSpreadStabilityGate {
		return "", nil
	}
	if c.cfg.SpreadVolatilityMaxCV <= 0 {
		return "", nil
	}

	minSamples := c.cfg.SpreadVolatilityMinSamples
	if minSamples < 2 {
		minSamples = 2
	}

	history, err := c.db.GetSpreadHistory(opp, spreadHistoryRiskLimit)
	if err != nil {
		return "", err
	}

	lookback := c.lookbackForInterval(opp.IntervalHours)
	spreads := spreadsWithinLookback(history, lookback)
	if len(spreads) < minSamples && c.historyProvider != nil {
		fallback := spreadsWithinLookback(c.historyProvider(opp, spreadHistoryRiskLimit, lookback), lookback)
		if len(fallback) > len(spreads) {
			spreads = fallback
		}
	}

	if len(spreads) < minSamples {
		return fmt.Sprintf("insufficient spread history (n=%d < %d)", len(spreads), minSamples), nil
	}

	var sum float64
	for _, spread := range spreads {
		sum += spread
	}
	mean := sum / float64(len(spreads))
	if mean <= 0 {
		return fmt.Sprintf("spread stability rejected: non-positive mean spread (n=%d)", len(spreads)), nil
	}

	var sqDiffSum float64
	for _, spread := range spreads {
		diff := spread - mean
		sqDiffSum += diff * diff
	}
	stddev := math.Sqrt(sqDiffSum / float64(len(spreads)))
	cv := stddev / mean

	threshold := c.cfg.SpreadVolatilityMaxCV
	tag := ""
	if automated && c.cfg.SpreadStabilityStricterForAuto {
		threshold *= c.cfg.SpreadStabilityAutoCVMultiplier
		tag = ", auto"
	}

	if cv > threshold {
		return fmt.Sprintf("spread unstable (CV=%.2f > %.2f, n=%d%s)", cv, threshold, len(spreads), tag), nil
	}

	return "", nil
}

func spreadsWithinLookback(history []database.SpreadHistoryPoint, lookback time.Duration) []float64 {
	cutoff := time.Now().Add(-lookback)
	spreads := make([]float64, 0, len(history))
	for _, point := range history {
		if !point.Timestamp.Before(cutoff) {
			spreads = append(spreads, point.Spread)
		}
	}
	return spreads
}

func (c *SpreadStabilityChecker) lookbackForInterval(intervalHours float64) time.Duration {
	switch {
	case intervalHours <= 1:
		return c.cfg.PersistLookback1h
	case intervalHours <= 4:
		return c.cfg.PersistLookback4h
	default:
		return c.cfg.PersistLookback8h
	}
}
