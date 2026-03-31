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
	db  *database.Client
	cfg *config.Config
}

func NewSpreadStabilityChecker(db *database.Client, cfg *config.Config) *SpreadStabilityChecker {
	return &SpreadStabilityChecker{
		db:  db,
		cfg: cfg,
	}
}

// Check returns a rejection reason when spread history is too sparse or too volatile.
func (c *SpreadStabilityChecker) Check(opp models.Opportunity, automated bool) (string, error) {
	if c == nil || c.db == nil || c.cfg == nil {
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
	cutoff := time.Now().Add(-lookback)
	spreads := make([]float64, 0, len(history))
	for _, point := range history {
		if !point.Timestamp.Before(cutoff) {
			spreads = append(spreads, point.Spread)
		}
	}

	if len(spreads) < minSamples {
		return fmt.Sprintf("spread stability rejected: insufficient history (%d/%d samples in %v)", len(spreads), minSamples, lookback), nil
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
		multiplier := c.cfg.SpreadStabilityAutoCVMultiplier
		if multiplier > 0 {
			threshold *= multiplier
		}
		tag = ", auto"
	}

	if cv > threshold {
		return fmt.Sprintf("spread unstable (CV=%.2f > %.2f, n=%d%s)", cv, threshold, len(spreads), tag), nil
	}

	return "", nil
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
