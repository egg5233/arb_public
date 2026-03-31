package risk

import (
	"math"
	"time"

	"arb/internal/config"
)

const (
	liqTrendWindowSamples = 30
	liqTrendSpikeDelta    = 0.15
)

type LiqTrendState string

const (
	LiqTrendStable   LiqTrendState = "stable"
	LiqTrendWarning  LiqTrendState = "warning"
	LiqTrendCritical LiqTrendState = "critical"
)

type liqTrendSample struct {
	at    time.Time
	ratio float64
}

type liqTrendSeries struct {
	samples []liqTrendSample
}

type LiqTrendResult struct {
	Exchange       string
	State          LiqTrendState
	CurrentRatio   float64
	ProjectedRatio float64
	SlopePerMinute float64
	SampleCount    int
}

type LiqTrendTracker struct {
	projectionMinutes   int
	warningSlopeThresh  float64
	criticalSlopeThresh float64
	minSamples          int
	series              map[string]*liqTrendSeries
}

func NewLiqTrendTracker(cfg *config.Config) *LiqTrendTracker {
	return &LiqTrendTracker{
		projectionMinutes:   cfg.LiqProjectionMinutes,
		warningSlopeThresh:  cfg.LiqWarningSlopeThresh,
		criticalSlopeThresh: cfg.LiqCriticalSlopeThresh,
		minSamples:          cfg.LiqMinSamples,
		series:              make(map[string]*liqTrendSeries),
	}
}

func (t *LiqTrendTracker) Sample(exchange string, ratio float64, at time.Time, warningRatio, criticalRatio float64) LiqTrendResult {
	result := LiqTrendResult{
		Exchange:     exchange,
		State:        LiqTrendStable,
		CurrentRatio: ratio,
	}
	if exchange == "" || ratio <= 0 {
		return result
	}
	if t.minSamples <= 0 {
		t.minSamples = 1
	}
	if t.projectionMinutes <= 0 {
		t.projectionMinutes = 15
	}

	series := t.series[exchange]
	if series == nil {
		series = &liqTrendSeries{}
		t.series[exchange] = series
	}
	series.samples = append(series.samples, liqTrendSample{at: at, ratio: ratio})
	if len(series.samples) > liqTrendWindowSamples {
		series.samples = append([]liqTrendSample(nil), series.samples[len(series.samples)-liqTrendWindowSamples:]...)
	}

	samples := filterLiqTrendSamples(series.samples)
	result.SampleCount = len(samples)
	if len(samples) == 0 {
		return result
	}
	result.CurrentRatio = samples[len(samples)-1].ratio
	if len(samples) < t.minSamples {
		return result
	}

	slope := liqTrendSlopePerMinute(samples)
	projected := result.CurrentRatio + slope*float64(t.projectionMinutes)
	if projected < 0 {
		projected = 0
	}
	if projected > 1 {
		projected = 1
	}

	result.SlopePerMinute = slope
	result.ProjectedRatio = projected

	switch {
	case slope >= t.criticalSlopeThresh && projected >= criticalRatio:
		result.State = LiqTrendCritical
	case slope >= t.warningSlopeThresh && projected >= warningRatio:
		result.State = LiqTrendWarning
	default:
		result.State = LiqTrendStable
	}
	return result
}

func filterLiqTrendSamples(samples []liqTrendSample) []liqTrendSample {
	n := len(samples)
	if n < 2 {
		return append([]liqTrendSample(nil), samples...)
	}

	// Ignore a fresh unconfirmed jump until a follow-up sample proves it is not
	// a one-tick spike.
	if math.Abs(samples[n-1].ratio-samples[n-2].ratio) > liqTrendSpikeDelta {
		if n == 2 || math.Abs(samples[n-2].ratio-samples[n-3].ratio) < liqTrendSpikeDelta/2 {
			return append([]liqTrendSample(nil), samples[:n-1]...)
		}
	}

	// If the last three samples form a spike-and-revert pattern, drop the middle
	// outlier from regression.
	if n >= 3 {
		a := samples[n-3]
		b := samples[n-2]
		c := samples[n-1]
		if math.Abs(b.ratio-a.ratio) > liqTrendSpikeDelta &&
			math.Abs(b.ratio-c.ratio) > liqTrendSpikeDelta &&
			math.Abs(c.ratio-a.ratio) < liqTrendSpikeDelta/2 {
			filtered := append([]liqTrendSample(nil), samples[:n-2]...)
			filtered = append(filtered, c)
			return filtered
		}
	}

	return append([]liqTrendSample(nil), samples...)
}

func liqTrendSlopePerMinute(samples []liqTrendSample) float64 {
	if len(samples) < 2 {
		return 0
	}

	base := samples[0].at
	var sumX, sumY float64
	for _, sample := range samples {
		x := sample.at.Sub(base).Minutes()
		sumX += x
		sumY += sample.ratio
	}
	meanX := sumX / float64(len(samples))
	meanY := sumY / float64(len(samples))

	var cov, variance float64
	for _, sample := range samples {
		x := sample.at.Sub(base).Minutes()
		dx := x - meanX
		dy := sample.ratio - meanY
		cov += dx * dy
		variance += dx * dx
	}
	if variance == 0 {
		return 0
	}
	return cov / variance
}
