package risk

import (
	"math"
	"testing"
	"time"

	"arb/internal/config"
)

func newTestLiqTrendTracker(minSamples int, warningSlope, criticalSlope float64, projectionMinutes int) *LiqTrendTracker {
	return &LiqTrendTracker{
		projectionMinutes:   projectionMinutes,
		warningSlopeThresh:  warningSlope,
		criticalSlopeThresh: criticalSlope,
		minSamples:          minSamples,
		series:              make(map[string]*liqTrendSeries),
	}
}

func TestLiqTrendTracker_ColdStartGuard(t *testing.T) {
	tracker := newTestLiqTrendTracker(5, 0.002, 0.004, 15)
	base := time.Unix(0, 0)
	for i, ratio := range []float64{0.40, 0.41, 0.42, 0.43} {
		got := tracker.Sample("binance", ratio, base.Add(time.Duration(i)*30*time.Second), 0.50, 0.80)
		if got.State != LiqTrendStable {
			t.Fatalf("sample %d: expected stable during cold start, got %s", i, got.State)
		}
		if got.SampleCount != i+1 {
			t.Fatalf("sample %d: expected sample count %d, got %d", i, i+1, got.SampleCount)
		}
	}
}

func TestLiqTrendTracker_LinearRegressionSlope(t *testing.T) {
	tracker := newTestLiqTrendTracker(3, 0.001, 0.002, 15)
	base := time.Unix(0, 0)
	var got LiqTrendResult
	for i, ratio := range []float64{0.30, 0.31, 0.32, 0.33} {
		got = tracker.Sample("binance", ratio, base.Add(time.Duration(i)*time.Minute), 0.50, 0.80)
	}
	if math.Abs(got.SlopePerMinute-0.01) > 1e-9 {
		t.Fatalf("expected slope 0.01/min, got %.6f", got.SlopePerMinute)
	}
}

func TestLiqTrendTracker_WarningState(t *testing.T) {
	tracker := newTestLiqTrendTracker(5, 0.003, 0.02, 15)
	base := time.Unix(0, 0)
	var got LiqTrendResult
	for i, ratio := range []float64{0.44, 0.445, 0.45, 0.455, 0.46} {
		got = tracker.Sample("binance", ratio, base.Add(time.Duration(i)*time.Minute), 0.50, 0.80)
	}
	if got.State != LiqTrendWarning {
		t.Fatalf("expected warning, got %s", got.State)
	}
	if got.ProjectedRatio < 0.50 || got.ProjectedRatio >= 0.80 {
		t.Fatalf("expected projected ratio to cross L3 but stay below L4, got %.4f", got.ProjectedRatio)
	}
}

func TestLiqTrendTracker_CriticalState(t *testing.T) {
	tracker := newTestLiqTrendTracker(5, 0.003, 0.01, 15)
	base := time.Unix(0, 0)
	var got LiqTrendResult
	for i, ratio := range []float64{0.44, 0.47, 0.50, 0.53, 0.56} {
		got = tracker.Sample("binance", ratio, base.Add(time.Duration(i)*time.Minute), 0.50, 0.80)
	}
	if got.State != LiqTrendCritical {
		t.Fatalf("expected critical, got %s", got.State)
	}
	if got.ProjectedRatio < 0.80 {
		t.Fatalf("expected projected ratio to cross L4, got %.4f", got.ProjectedRatio)
	}
}

func TestLiqTrendTracker_SkipsZeroRatioSamples(t *testing.T) {
	tracker := newTestLiqTrendTracker(3, 0.001, 0.002, 15)
	base := time.Unix(0, 0)
	for i, ratio := range []float64{0.30, 0.31, 0.32} {
		tracker.Sample("binance", ratio, base.Add(time.Duration(i)*time.Minute), 0.50, 0.80)
	}
	tracker.Sample("binance", 0, base.Add(3*time.Minute), 0.50, 0.80)
	if len(tracker.series["binance"].samples) != 3 {
		t.Fatalf("expected zero ratio sample to be ignored, got %d stored samples", len(tracker.series["binance"].samples))
	}
}

func TestLiqTrendTracker_IgnoresSingleTickSpike(t *testing.T) {
	tracker := newTestLiqTrendTracker(5, 0.002, 0.004, 15)
	base := time.Unix(0, 0)
	for i, ratio := range []float64{0.40, 0.405, 0.41, 0.415, 0.42} {
		tracker.Sample("binance", ratio, base.Add(time.Duration(i)*time.Minute), 0.50, 0.80)
	}
	got := tracker.Sample("binance", 0.70, base.Add(5*time.Minute), 0.50, 0.80)
	if got.State != LiqTrendStable {
		t.Fatalf("expected single spike to stay stable, got %s", got.State)
	}
	if got.SampleCount != 5 {
		t.Fatalf("expected spike filter to evaluate prior 5 samples, got %d", got.SampleCount)
	}
}

func TestNewLiqTrendTracker_UsesConfig(t *testing.T) {
	cfg := &config.Config{
		LiqProjectionMinutes:   20,
		LiqWarningSlopeThresh:  0.003,
		LiqCriticalSlopeThresh: 0.006,
		LiqMinSamples:          7,
	}
	tracker := NewLiqTrendTracker(cfg)
	if tracker.projectionMinutes != 20 || tracker.warningSlopeThresh != 0.003 || tracker.criticalSlopeThresh != 0.006 || tracker.minSamples != 7 {
		t.Fatalf("tracker config mismatch: %#v", tracker)
	}
}
