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

func TestSpreadStabilityChecker(t *testing.T) {
	tests := []struct {
		name       string
		spreads    []float64
		automated  bool
		wantReason string
	}{
		{
			name:       "new pair without history rejects",
			spreads:    nil,
			wantReason: "insufficient history",
		},
		{
			name:       "insufficient samples reject",
			spreads:    []float64{10, 11},
			wantReason: "insufficient history",
		},
		{
			name:       "stable history passes manual entry",
			spreads:    []float64{10, 10.5, 9.5, 10.2},
			wantReason: "",
		},
		{
			name:       "volatile history rejects manual entry",
			spreads:    []float64{10, 1, 20, 8},
			wantReason: "spread unstable",
		},
		{
			name:       "automated entries use stricter threshold",
			spreads:    []float64{10, 5, 15, 10},
			automated:  true,
			wantReason: "spread unstable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr := miniredis.RunT(t)
			db, err := database.New(mr.Addr(), "", 2)
			if err != nil {
				t.Fatalf("database.New: %v", err)
			}
			t.Cleanup(func() { _ = db.Close() })

			cfg := &config.Config{
				PersistLookback1h:               time.Hour,
				PersistLookback4h:               2 * time.Hour,
				PersistLookback8h:               4 * time.Hour,
				SpreadVolatilityMaxCV:           0.5,
				SpreadVolatilityMinSamples:      3,
				SpreadStabilityStricterForAuto:  true,
				SpreadStabilityAutoCVMultiplier: 0.7,
			}

			checker := NewSpreadStabilityChecker(db, cfg)
			opp := models.Opportunity{
				Symbol:        "BTCUSDT",
				LongExchange:  "binance",
				ShortExchange: "bybit",
				Spread:        10,
				IntervalHours: 1,
			}

			now := time.Now().Add(-10 * time.Minute)
			for i, spread := range tt.spreads {
				opp.Spread = spread
				if err := db.AddSpreadHistoryBatch([]models.Opportunity{opp}, now.Add(time.Duration(i)*time.Minute)); err != nil {
					t.Fatalf("AddSpreadHistoryBatch: %v", err)
				}
			}

			reason, err := checker.Check(opp, tt.automated)
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			if tt.wantReason == "" && reason != "" {
				t.Fatalf("unexpected rejection: %s", reason)
			}
			if tt.wantReason != "" && !strings.Contains(reason, tt.wantReason) {
				t.Fatalf("reason %q does not contain %q", reason, tt.wantReason)
			}
		})
	}
}
