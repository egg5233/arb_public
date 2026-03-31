package risk

import (
	"strings"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"

	"github.com/alicebob/miniredis/v2"
)

func TestSpreadStabilityCheckerDisabledGateBypassesHistory(t *testing.T) {
	checker, _, opp := newSpreadStabilityTestHarness(t, &config.Config{
		EnableSpreadStabilityGate:       false,
		PersistLookback1h:               time.Hour,
		PersistLookback4h:               2 * time.Hour,
		PersistLookback8h:               4 * time.Hour,
		SpreadVolatilityMaxCV:           0.5,
		SpreadVolatilityMinSamples:      3,
		SpreadStabilityStricterForAuto:  true,
		SpreadStabilityAutoCVMultiplier: 0.7,
	})

	reason, err := checker.Check(opp, false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if reason != "" {
		t.Fatalf("unexpected rejection: %s", reason)
	}
}

func TestSpreadStabilityCheckerFallsBackToScannerHistory(t *testing.T) {
	checker, _, opp := newSpreadStabilityTestHarness(t, baseSpreadStabilityConfig())
	checker.SetHistoryProvider(func(models.Opportunity, int, time.Duration) []database.SpreadHistoryPoint {
		return historyPoints([]float64{10, 10.2, 9.8, 10.1})
	})

	reason, err := checker.Check(opp, false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if reason != "" {
		t.Fatalf("unexpected rejection: %s", reason)
	}
}

func TestSpreadStabilityCheckerAllowsWarmupWhileHistorySparse(t *testing.T) {
	checker, _, opp := newSpreadStabilityTestHarness(t, baseSpreadStabilityConfig())
	checker.SetHistoryProvider(func(models.Opportunity, int, time.Duration) []database.SpreadHistoryPoint {
		return historyPoints([]float64{10, 10.2})
	})

	reason, err := checker.Check(opp, false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if reason != "" {
		t.Fatalf("unexpected rejection while warming history: %s", reason)
	}
}

func TestSpreadStabilityCheckerRejectsVolatileHistory(t *testing.T) {
	checker, db, opp := newSpreadStabilityTestHarness(t, baseSpreadStabilityConfig())
	seedSpreadHistory(t, db, opp, []float64{10, 1, 20, 8})

	reason, err := checker.Check(opp, false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !strings.Contains(reason, "spread unstable") {
		t.Fatalf("reason %q does not contain spread unstable", reason)
	}
}

func TestSpreadStabilityCheckerHonorsZeroAutoMultiplier(t *testing.T) {
	cfg := baseSpreadStabilityConfig()
	cfg.SpreadStabilityAutoCVMultiplier = 0

	checker, db, opp := newSpreadStabilityTestHarness(t, cfg)
	seedSpreadHistory(t, db, opp, []float64{10, 10.4, 9.6, 10.1})

	reason, err := checker.Check(opp, true)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !strings.Contains(reason, "spread unstable") {
		t.Fatalf("reason %q does not contain spread unstable", reason)
	}
}

func baseSpreadStabilityConfig() *config.Config {
	return &config.Config{
		EnableSpreadStabilityGate:       true,
		PersistLookback1h:               time.Hour,
		PersistLookback4h:               2 * time.Hour,
		PersistLookback8h:               4 * time.Hour,
		SpreadVolatilityMaxCV:           0.5,
		SpreadVolatilityMinSamples:      3,
		SpreadStabilityStricterForAuto:  true,
		SpreadStabilityAutoCVMultiplier: 0.7,
	}
}

func newSpreadStabilityTestHarness(t *testing.T, cfg *config.Config) (*SpreadStabilityChecker, *database.Client, models.Opportunity) {
	t.Helper()

	mr := miniredis.RunT(t)
	db, err := database.New(mr.Addr(), "", 2)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	checker := NewSpreadStabilityChecker(db, cfg)
	opp := models.Opportunity{
		Symbol:        "BTCUSDT",
		LongExchange:  "binance",
		ShortExchange: "bybit",
		Spread:        10,
		IntervalHours: 1,
	}

	return checker, db, opp
}

func seedSpreadHistory(t *testing.T, db *database.Client, opp models.Opportunity, spreads []float64) {
	t.Helper()

	now := time.Now().Add(-10 * time.Minute)
	for i, spread := range spreads {
		opp.Spread = spread
		if err := db.AddSpreadHistoryBatch([]models.Opportunity{opp}, now.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("AddSpreadHistoryBatch: %v", err)
		}
	}
}

func historyPoints(spreads []float64) []database.SpreadHistoryPoint {
	now := time.Now().Add(-10 * time.Minute)
	points := make([]database.SpreadHistoryPoint, 0, len(spreads))
	for i, spread := range spreads {
		points = append(points, database.SpreadHistoryPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Spread:    spread,
		})
	}
	return points
}
