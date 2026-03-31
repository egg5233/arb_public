package spotengine

import (
	"sync"
	"time"

	"arb/internal/models"
)

const maxRateVelocitySamples = 256

type rateSample struct {
	at  time.Time
	apr float64
}

type rateVelocityWindow struct {
	samples   [maxRateVelocitySamples]rateSample
	start     int
	count     int
	triggered bool
}

type BorrowRateSpike struct {
	BaselineAPR  float64
	CurrentAPR   float64
	AbsoluteMove float64
	Multiplier   float64
	BaselineAt   time.Time
	CurrentAt    time.Time
}

// RateVelocityDetector keeps a bounded history of borrow APR samples and
// surfaces one-shot spike events per key.
type RateVelocityDetector struct {
	mu      sync.Mutex
	windows map[string]*rateVelocityWindow
}

func NewRateVelocityDetector() *RateVelocityDetector {
	return &RateVelocityDetector{
		windows: make(map[string]*rateVelocityWindow),
	}
}

func (d *RateVelocityDetector) Delete(key string) {
	if d == nil || key == "" {
		return
	}
	d.mu.Lock()
	delete(d.windows, key)
	d.mu.Unlock()
}

func (d *RateVelocityDetector) SampleAndCheck(
	key string,
	now time.Time,
	currentAPR float64,
	window time.Duration,
	multiplier float64,
	minAbsolute float64,
) (BorrowRateSpike, bool) {
	if d == nil || key == "" || window <= 0 {
		return BorrowRateSpike{}, false
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	state := d.windows[key]
	if state == nil {
		state = &rateVelocityWindow{}
		d.windows[key] = state
	}

	state.push(rateSample{at: now, apr: currentAPR})
	state.prune(now.Add(-window))

	baseline, found := state.minBeforeLatest()
	if !found {
		state.triggered = false
		return BorrowRateSpike{}, false
	}

	absoluteMove := currentAPR - baseline.apr
	if absoluteMove < minAbsolute {
		state.triggered = false
		return BorrowRateSpike{}, false
	}

	ratioOK := baseline.apr <= 0
	if baseline.apr > 0 {
		ratioOK = currentAPR >= baseline.apr*multiplier
	}
	if !ratioOK {
		state.triggered = false
		return BorrowRateSpike{}, false
	}

	if state.triggered {
		return BorrowRateSpike{}, false
	}
	state.triggered = true

	var ratio float64
	if baseline.apr > 0 {
		ratio = currentAPR / baseline.apr
	}

	return BorrowRateSpike{
		BaselineAPR:  baseline.apr,
		CurrentAPR:   currentAPR,
		AbsoluteMove: absoluteMove,
		Multiplier:   ratio,
		BaselineAt:   baseline.at,
		CurrentAt:    now,
	}, true
}

func (w *rateVelocityWindow) push(sample rateSample) {
	if w.count == maxRateVelocitySamples {
		w.start = (w.start + 1) % maxRateVelocitySamples
		w.count--
	}

	idx := (w.start + w.count) % maxRateVelocitySamples
	w.samples[idx] = sample
	w.count++
}

func (w *rateVelocityWindow) prune(cutoff time.Time) {
	for w.count > 0 {
		head := w.samples[w.start]
		if !head.at.Before(cutoff) {
			break
		}
		w.start = (w.start + 1) % maxRateVelocitySamples
		w.count--
	}
}

func (w *rateVelocityWindow) minBeforeLatest() (rateSample, bool) {
	if w.count < 2 {
		return rateSample{}, false
	}

	latestIdx := (w.start + w.count - 1) % maxRateVelocitySamples
	var min rateSample
	found := false
	for i := 0; i < w.count-1; i++ {
		idx := (w.start + i) % maxRateVelocitySamples
		if idx == latestIdx {
			continue
		}
		sample := w.samples[idx]
		if !found || sample.apr < min.apr {
			min = sample
			found = true
		}
	}
	return min, found
}

func (e *SpotEngine) evaluateBorrowRateSpike(pos *models.SpotFuturesPosition, now time.Time) (string, bool) {
	if pos == nil || pos.Direction != "borrow_sell_long" || e.borrowVelocity == nil {
		return "", false
	}

	windowMin := e.cfg.BorrowSpikeWindowMin
	if windowMin <= 0 {
		windowMin = 60
	}
	multiplier := e.cfg.BorrowSpikeMultiplier
	if multiplier <= 0 {
		multiplier = 2.0
	}
	minAbsolute := e.cfg.BorrowSpikeMinAbsolute
	if minAbsolute < 0 {
		minAbsolute = 0
	}

	spike, ok := e.borrowVelocity.SampleAndCheck(
		pos.ID,
		now,
		pos.CurrentBorrowAPR,
		time.Duration(windowMin)*time.Minute,
		multiplier,
		minAbsolute,
	)
	if !ok {
		return "", false
	}

	if !e.cfg.EnableBorrowSpikeDetection {
		e.log.Warn(
			"borrow spike observed for %s but auto-exit disabled: current=%.2f%% baseline=%.2f%% delta=%.2f%% window=%dm",
			pos.Symbol,
			spike.CurrentAPR*100,
			spike.BaselineAPR*100,
			spike.AbsoluteMove*100,
			windowMin,
		)
		return "", false
	}

	e.log.Warn(
		"exit trigger: %s borrow APR spike current=%.2f%% baseline=%.2f%% delta=%.2f%% over %dm",
		pos.Symbol,
		spike.CurrentAPR*100,
		spike.BaselineAPR*100,
		spike.AbsoluteMove*100,
		windowMin,
	)
	return "borrow_rate_spike", false
}
