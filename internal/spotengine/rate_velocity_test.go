package spotengine

import (
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/utils"
)

func TestRateVelocityDetectorSampleAndCheck(t *testing.T) {
	base := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		samples     []rateSample
		window      time.Duration
		multiplier  float64
		minAbsolute float64
		wantSpike   bool
	}{
		{
			name:        "insufficient history",
			samples:     []rateSample{{at: base, apr: 0.10}},
			window:      60 * time.Minute,
			multiplier:  2.0,
			minAbsolute: 0.10,
			wantSpike:   false,
		},
		{
			name: "below multiplier",
			samples: []rateSample{
				{at: base, apr: 0.10},
				{at: base.Add(30 * time.Minute), apr: 0.15},
			},
			window:      60 * time.Minute,
			multiplier:  2.0,
			minAbsolute: 0.03,
			wantSpike:   false,
		},
		{
			name: "below absolute threshold",
			samples: []rateSample{
				{at: base, apr: 0.10},
				{at: base.Add(30 * time.Minute), apr: 0.19},
			},
			window:      60 * time.Minute,
			multiplier:  1.8,
			minAbsolute: 0.10,
			wantSpike:   false,
		},
		{
			name: "doubles within window",
			samples: []rateSample{
				{at: base, apr: 0.10},
				{at: base.Add(30 * time.Minute), apr: 0.22},
			},
			window:      60 * time.Minute,
			multiplier:  2.0,
			minAbsolute: 0.10,
			wantSpike:   true,
		},
		{
			name: "zero to small rate does not trigger",
			samples: []rateSample{
				{at: base, apr: 0.00},
				{at: base.Add(30 * time.Minute), apr: 0.04},
			},
			window:      60 * time.Minute,
			multiplier:  2.0,
			minAbsolute: 0.10,
			wantSpike:   false,
		},
		{
			name: "zero to large rate triggers",
			samples: []rateSample{
				{at: base, apr: 0.00},
				{at: base.Add(30 * time.Minute), apr: 0.25},
			},
			window:      60 * time.Minute,
			multiplier:  2.0,
			minAbsolute: 0.10,
			wantSpike:   true,
		},
		{
			name: "samples outside window ignored",
			samples: []rateSample{
				{at: base, apr: 0.10},
				{at: base.Add(2 * time.Hour), apr: 0.30},
			},
			window:      60 * time.Minute,
			multiplier:  2.0,
			minAbsolute: 0.10,
			wantSpike:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			detector := NewRateVelocityDetector()
			got := false
			for _, sample := range tc.samples {
				_, got = detector.SampleAndCheck("BTCUSDT", sample.at, sample.apr, tc.window, tc.multiplier, tc.minAbsolute)
			}
			if got != tc.wantSpike {
				t.Fatalf("SampleAndCheck spike = %v, want %v", got, tc.wantSpike)
			}
		})
	}
}

func TestEvaluateBorrowRateSpikeDebounceAndToggle(t *testing.T) {
	now := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	newEngine := func(enabled bool) *SpotEngine {
		return &SpotEngine{
			cfg: &config.Config{
				EnableBorrowSpikeDetection: enabled,
				BorrowSpikeWindowMin:       60,
				BorrowSpikeMultiplier:      2.0,
				BorrowSpikeMinAbsolute:     0.10,
			},
			borrowVelocity: NewRateVelocityDetector(),
			log:            utils.NewLogger("test"),
		}
	}

	pos := &models.SpotFuturesPosition{
		ID:               "pos-1",
		Symbol:           "BTCUSDT",
		Direction:        "borrow_sell_long",
		CurrentBorrowAPR: 0.10,
	}

	t.Run("disabled detector samples without exit", func(t *testing.T) {
		engine := newEngine(false)
		if reason, _ := engine.evaluateBorrowRateSpike(pos, now); reason != "" {
			t.Fatalf("unexpected spike reason on first sample: %q", reason)
		}
		pos.CurrentBorrowAPR = 0.25
		if reason, _ := engine.evaluateBorrowRateSpike(pos, now.Add(30*time.Minute)); reason != "" {
			t.Fatalf("unexpected spike reason when disabled: %q", reason)
		}
	})

	t.Run("enabled detector fires once until recovery", func(t *testing.T) {
		engine := newEngine(true)
		pos.CurrentBorrowAPR = 0.10
		if reason, _ := engine.evaluateBorrowRateSpike(pos, now); reason != "" {
			t.Fatalf("unexpected spike reason on first sample: %q", reason)
		}

		pos.CurrentBorrowAPR = 0.25
		if reason, _ := engine.evaluateBorrowRateSpike(pos, now.Add(30*time.Minute)); reason != "borrow_rate_spike" {
			t.Fatalf("expected borrow_rate_spike, got %q", reason)
		}

		pos.CurrentBorrowAPR = 0.27
		if reason, _ := engine.evaluateBorrowRateSpike(pos, now.Add(40*time.Minute)); reason != "" {
			t.Fatalf("expected debounce to suppress repeated spike, got %q", reason)
		}

		pos.CurrentBorrowAPR = 0.11
		if reason, _ := engine.evaluateBorrowRateSpike(pos, now.Add(70*time.Minute)); reason != "" {
			t.Fatalf("expected recovery sample to clear trigger, got %q", reason)
		}

		pos.CurrentBorrowAPR = 0.30
		if reason, _ := engine.evaluateBorrowRateSpike(pos, now.Add(100*time.Minute)); reason != "borrow_rate_spike" {
			t.Fatalf("expected borrow_rate_spike after recovery, got %q", reason)
		}
	})
}
