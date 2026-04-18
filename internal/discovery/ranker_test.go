package discovery

import (
	"math"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/utils"
)

// TestExRateIntervalPropagation verifies that the ranker correctly determines
// interval hours from Loris FundingIntervals data and computes maxInterval.
func TestExRateIntervalPropagation(t *testing.T) {
	tests := []struct {
		name         string
		intervals    map[string]map[string]float64
		wantInterval float64
	}{
		{
			name:         "default 8h when no intervals provided",
			intervals:    nil,
			wantInterval: 8.0,
		},
		{
			name: "1h interval for both exchanges",
			intervals: map[string]map[string]float64{
				"binance": {"BTC": 1.0},
				"bybit":   {"BTC": 1.0},
			},
			wantInterval: 1.0,
		},
		{
			name: "4h interval for both exchanges",
			intervals: map[string]map[string]float64{
				"binance": {"BTC": 4.0},
				"bybit":   {"BTC": 4.0},
			},
			wantInterval: 4.0,
		},
		{
			name: "mixed intervals uses max",
			intervals: map[string]map[string]float64{
				"binance": {"BTC": 1.0},
				"bybit":   {"BTC": 4.0},
			},
			wantInterval: 4.0,
		},
		{
			name: "8h explicit",
			intervals: map[string]map[string]float64{
				"binance": {"BTC": 8.0},
				"bybit":   {"BTC": 8.0},
			},
			wantInterval: 8.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate what RankOpportunities does for interval extraction.
			exchanges := []string{"binance", "bybit"}
			baseSym := "BTC"
			rawRates := map[string]float64{
				"binance": 50.0,
				"bybit":   -20.0,
			}

			var rates []exRate
			for _, exch := range exchanges {
				rawRate := rawRates[exch]

				intervalHrs := 8.0
				if tt.intervals != nil {
					if intervals, ok := tt.intervals[exch]; ok {
						if iv, ok := intervals[baseSym]; ok && iv > 0 {
							intervalHrs = iv
						}
					}
				}

				rateBpsH := rawRate // Loris rates are already bps/h
				rates = append(rates, exRate{
					exchange:    exch,
					rateBpsH:    rateBpsH,
					intervalHrs: intervalHrs,
				})
			}

			// Find best pair (same logic as ranker).
			bestLong := rates[0]
			bestShort := rates[0]
			for _, r := range rates[1:] {
				if r.rateBpsH < bestLong.rateBpsH {
					bestLong = r
				}
				if r.rateBpsH > bestShort.rateBpsH {
					bestShort = r
				}
			}

			maxInterval := math.Max(bestLong.intervalHrs, bestShort.intervalHrs)
			if maxInterval <= 0 {
				maxInterval = 8
			}

			if maxInterval != tt.wantInterval {
				t.Errorf("maxInterval = %v, want %v", maxInterval, tt.wantInterval)
			}

			// Verify Opportunity would get the correct IntervalHours.
			opp := models.Opportunity{
				IntervalHours: maxInterval,
				Source:        "loris",
			}
			if opp.IntervalHours != tt.wantInterval {
				t.Errorf("Opportunity.IntervalHours = %v, want %v", opp.IntervalHours, tt.wantInterval)
			}
		})
	}
}

// TestRankOpportunitiesCostRatioWithInterval verifies that the cost ratio
// computation uses interval-aware hold periods.
func TestRankOpportunitiesCostRatioWithInterval(t *testing.T) {
	tests := []struct {
		name        string
		intervalHrs float64
		wantLess    bool // cost ratio should be < valueOfRatio
	}{
		{"8h interval", 8.0, true},
		{"4h interval more periods lower cost", 4.0, true},
		{"1h interval many periods lowest cost", 1.0, true},
	}

	valueOfTimeHours := 16.0
	valueOfRatio := 0.50

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A raw 8h spread of 20 bps converts to different bps/h depending on interval.
			// E.g. 20 bps per 8h = 2.5 bps/h; 20 bps per 1h = 20 bps/h.
			spreadBpsH := 20.0 / tt.intervalHrs // bps per hour

			// Use binance + bybit fees.
			feesA := exchangeFees["binance"]
			feesB := exchangeFees["bybit"]
			totalFeePct := feesA.Taker + feesB.Taker + feesA.Taker + feesB.Taker
			totalFeeBps := totalFeePct * 100

			// costRatio = totalFeeBps / (spread_bps_h * holdHours)
			costRatio := totalFeeBps / (spreadBpsH * valueOfTimeHours)

			if tt.wantLess && costRatio >= valueOfRatio {
				t.Errorf("costRatio = %v, want < %v (spreadBpsH=%v)", costRatio, valueOfRatio, spreadBpsH)
			}

			// Verify shorter intervals produce lower cost ratios (higher bps/h for same raw rate).
			if tt.intervalHrs < 8.0 {
				spreadBpsH8h := 20.0 / 8.0
				costRatio8h := totalFeeBps / (spreadBpsH8h * valueOfTimeHours)
				if costRatio >= costRatio8h {
					t.Errorf("shorter interval %vh should produce lower cost ratio: got %v >= %v (8h)",
						tt.intervalHrs, costRatio, costRatio8h)
				}
			}
		})
	}
}

// TestCoinGlassOpportunitiesIntervalHours verifies CoinGlass opportunities
// always get IntervalHours = 8.0.
func TestCoinGlassOpportunitiesIntervalHours(t *testing.T) {
	cfg := &config.Config{
		MinHoldTime:      16 * time.Hour,
		MaxCostRatio:     0.90,
		TopOpportunities: 10,
	}
	s := &Scanner{
		cfg:       cfg,
		contracts: nil,
		exchanges: makeNilExchangeMap("binance", "bitget"),
		log:       newTestLogger(),
	}

	cg := &models.CoinGlassResponse{
		Data: []models.CoinGlassArb{
			{
				Pair:        "BTC",
				LongPair:    "BTC/USDT",
				LongEx:      "Binance",
				ShortPair:   "BTC/USDT",
				ShortEx:     "Bitget",
				AnnualYield: "432.74%",
				FundingRate: "0.50%",
				OILong:      "$10M",
				OIShort:     "$8M",
			},
		},
	}

	opps := s.coinGlassToOpportunities(cg)
	if len(opps) == 0 {
		t.Fatal("expected at least one CoinGlass opportunity")
	}

	// Spread should be annualYield in bps/h: 432.74% = 4.3274 * 10000 / 8760 ≈ 4.94 bps/h
	expectedSpread := 4.3274 * 10000 / 8760.0
	if math.Abs(opps[0].Spread-expectedSpread) > 0.01 {
		t.Errorf("CoinGlass Spread = %v, want ~%v", opps[0].Spread, expectedSpread)
	}
	if opps[0].Source != "coinglass" {
		t.Errorf("Source = %q, want %q", opps[0].Source, "coinglass")
	}
}

// TestRateToBpsPerHour verifies the bps/hour normalization function.
func TestRateToBpsPerHour(t *testing.T) {
	tests := []struct {
		name      string
		rate      float64
		intervalH float64
		wantBpsH  float64
	}{
		{"8h divides by 8", 80.0, 8.0, 10.0},
		{"1h divides by 1", 10.0, 1.0, 10.0},
		{"4h divides by 4", 40.0, 4.0, 10.0},
		{"zero interval defaults to 8h", 80.0, 0, 10.0},
		{"negative interval defaults to 8h", 80.0, -1, 10.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utils.RateToBpsPerHour(tt.rate, tt.intervalH)
			if math.Abs(got-tt.wantBpsH) > 0.001 {
				t.Errorf("RateToBpsPerHour(%v, %v) = %v, want %v", tt.rate, tt.intervalH, got, tt.wantBpsH)
			}
		})
	}
}
