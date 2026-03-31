package risk

import (
	"math"
	"sort"
	"sync"
	"time"

	"arb/internal/config"
	"arb/pkg/exchange"
)

const (
	exchangeLatencyWeight = 0.3
	exchangeUptimeWeight  = 0.4
	exchangeFillWeight    = 0.3
	wsMessageMaxAge       = 2 * time.Minute
)

type latencySample struct {
	timestamp time.Time
	latency   time.Duration
	failed    bool
}

type wsTransition struct {
	timestamp time.Time
	connected bool
}

type orderMetric struct {
	placedAt time.Time
	filled   bool
}

type exchangeScoreState struct {
	requests      []latencySample
	wsTransitions []wsTransition
	lastMessageAt time.Time
	orders        map[string]orderMetric
}

// ExchangeScoreSnapshot is the dashboard-facing view of an exchange health score.
type ExchangeScoreSnapshot struct {
	Exchange         string        `json:"exchange"`
	Score            float64       `json:"score"`
	LatencyScore     float64       `json:"latency_score"`
	UptimeScore      float64       `json:"uptime_score"`
	FillRateScore    float64       `json:"fill_rate_score"`
	LatencyP95       time.Duration `json:"latency_p95"`
	LatencyAvg       time.Duration `json:"latency_avg"`
	RequestCount     int64         `json:"request_count"`
	ErrorCount       int64         `json:"error_count"`
	WSUptime         float64       `json:"ws_uptime"`
	WSReconnects     int           `json:"ws_reconnects"`
	WSConnected      bool          `json:"ws_connected"`
	LastMessageAge   time.Duration `json:"last_message_age"`
	OrdersPlaced     int64         `json:"orders_placed"`
	OrdersFilled     int64         `json:"orders_filled"`
	FillRate         float64       `json:"fill_rate"`
	WindowMinutes    int           `json:"window_minutes"`
	MinScoreForEntry float64       `json:"min_score_for_entry"`
}

// ExchangeScorer computes weighted exchange-health scores over a rolling window.
type ExchangeScorer struct {
	cfg    *config.Config
	mu     sync.RWMutex
	states map[string]*exchangeScoreState
}

// NewExchangeScorer creates a new exchange health scorer.
func NewExchangeScorer(cfg *config.Config) *ExchangeScorer {
	return &ExchangeScorer{
		cfg:    cfg,
		states: make(map[string]*exchangeScoreState),
	}
}

func (s *ExchangeScorer) window() time.Duration {
	if s == nil || s.cfg == nil || s.cfg.ExchHealthWindowMin <= 0 {
		return 60 * time.Minute
	}
	return time.Duration(s.cfg.ExchHealthWindowMin) * time.Minute
}

func (s *ExchangeScorer) latencyThreshold() time.Duration {
	if s == nil || s.cfg == nil || s.cfg.ExchHealthLatencyMs <= 0 {
		return 2 * time.Second
	}
	return time.Duration(s.cfg.ExchHealthLatencyMs) * time.Millisecond
}

func (s *ExchangeScorer) minUptime() float64 {
	if s == nil || s.cfg == nil || s.cfg.ExchHealthMinUptime <= 0 {
		return 0.95
	}
	return s.cfg.ExchHealthMinUptime
}

func (s *ExchangeScorer) minFillRate() float64 {
	if s == nil || s.cfg == nil || s.cfg.ExchHealthMinFillRate <= 0 {
		return 0.80
	}
	return s.cfg.ExchHealthMinFillRate
}

func (s *ExchangeScorer) minScore() float64 {
	if s == nil || s.cfg == nil || s.cfg.ExchHealthMinScore <= 0 {
		return 0.50
	}
	return s.cfg.ExchHealthMinScore
}

func (s *ExchangeScorer) state(exchangeName string) *exchangeScoreState {
	st := s.states[exchangeName]
	if st == nil {
		st = &exchangeScoreState{orders: make(map[string]orderMetric)}
		s.states[exchangeName] = st
	}
	return st
}

func clamp(v, low, high float64) float64 {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func (s *ExchangeScorer) pruneLocked(st *exchangeScoreState, now time.Time) {
	cutoff := now.Add(-s.window())

	reqs := st.requests[:0]
	for _, sample := range st.requests {
		if !sample.timestamp.Before(cutoff) {
			reqs = append(reqs, sample)
		}
	}
	st.requests = reqs

	transitions := st.wsTransitions[:0]
	for _, sample := range st.wsTransitions {
		if !sample.timestamp.Before(cutoff) {
			transitions = append(transitions, sample)
		}
	}
	st.wsTransitions = transitions

	for orderID, sample := range st.orders {
		if sample.placedAt.Before(cutoff) {
			delete(st.orders, orderID)
		}
	}

	if !st.lastMessageAt.IsZero() && st.lastMessageAt.Before(cutoff) {
		st.lastMessageAt = time.Time{}
	}
}

// RecordLatency stores a REST latency sample for an exchange endpoint.
func (s *ExchangeScorer) RecordLatency(exchangeName, endpoint string, latency time.Duration, err error) {
	if s == nil || exchangeName == "" {
		return
	}
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.state(exchangeName)
	st.requests = append(st.requests, latencySample{
		timestamp: now,
		latency:   latency,
		failed:    err != nil,
	})
	s.pruneLocked(st, now)
}

// RecordWSEvent stores a public WebSocket lifecycle event for an exchange.
func (s *ExchangeScorer) RecordWSEvent(exchangeName string, event exchange.WSEvent) {
	if s == nil || exchangeName == "" {
		return
	}
	ts := event.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.state(exchangeName)
	switch event.Type {
	case exchange.WSEventConnect:
		st.wsTransitions = append(st.wsTransitions, wsTransition{timestamp: ts, connected: true})
	case exchange.WSEventDisconnect:
		st.wsTransitions = append(st.wsTransitions, wsTransition{timestamp: ts, connected: false})
	case exchange.WSEventMessage:
		st.lastMessageAt = ts
	}
	s.pruneLocked(st, ts)
}

// RecordOrderEvent stores a placement/fill event used for fill-rate scoring.
func (s *ExchangeScorer) RecordOrderEvent(exchangeName string, event exchange.OrderMetricEvent) {
	if s == nil || exchangeName == "" || event.OrderID == "" {
		return
	}
	ts := event.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.state(exchangeName)
	current := st.orders[event.OrderID]
	if current.placedAt.IsZero() {
		current.placedAt = ts
	}
	switch event.Type {
	case exchange.OrderMetricPlaced:
		current.placedAt = ts
	case exchange.OrderMetricFilled:
		current.filled = current.filled || event.FilledQty > 0
	}
	st.orders[event.OrderID] = current
	s.pruneLocked(st, ts)
}

func (s *ExchangeScorer) snapshotLocked(exchangeName string, st *exchangeScoreState, now time.Time) ExchangeScoreSnapshot {
	s.pruneLocked(st, now)

	snapshot := ExchangeScoreSnapshot{
		Exchange:         exchangeName,
		Score:            1.0,
		LatencyScore:     1.0,
		UptimeScore:      1.0,
		FillRateScore:    1.0,
		WSUptime:         1.0,
		WSConnected:      true,
		FillRate:         1.0,
		WindowMinutes:    int(s.window().Minutes()),
		MinScoreForEntry: s.minScore(),
	}

	if len(st.requests) > 0 {
		snapshot.RequestCount = int64(len(st.requests))
		var total float64
		latencies := make([]float64, 0, len(st.requests))
		for _, sample := range st.requests {
			total += float64(sample.latency)
			latencies = append(latencies, float64(sample.latency))
			if sample.failed {
				snapshot.ErrorCount++
			}
		}
		sort.Float64s(latencies)
		p95Index := int(math.Ceil(float64(len(latencies))*0.95)) - 1
		if p95Index < 0 {
			p95Index = 0
		}
		if p95Index >= len(latencies) {
			p95Index = len(latencies) - 1
		}
		snapshot.LatencyAvg = time.Duration(total / float64(len(latencies)))
		snapshot.LatencyP95 = time.Duration(latencies[p95Index])
		snapshot.LatencyScore = clamp(1.0-float64(snapshot.LatencyP95)/float64(s.latencyThreshold()), 0, 1)
	}

	if len(st.wsTransitions) > 0 {
		windowStart := now.Add(-s.window())
		connected := true
		havePriorState := false
		for _, sample := range st.wsTransitions {
			if sample.timestamp.Before(windowStart) {
				connected = sample.connected
				havePriorState = true
			}
		}
		if !havePriorState {
			connected = st.wsTransitions[0].connected
		}

		lastPoint := windowStart
		var connectedDur time.Duration
		reconnects := 0
		for idx, sample := range st.wsTransitions {
			if sample.timestamp.Before(windowStart) {
				continue
			}
			if connected {
				connectedDur += sample.timestamp.Sub(lastPoint)
			}
			if sample.connected && !(idx == 0 && !havePriorState) {
				reconnects++
			}
			connected = sample.connected
			lastPoint = sample.timestamp
		}
		if connected {
			connectedDur += now.Sub(lastPoint)
		}

		snapshot.WSUptime = clamp(float64(connectedDur)/float64(s.window()), 0, 1)
		snapshot.WSReconnects = reconnects
		snapshot.WSConnected = connected
	}

	if !st.lastMessageAt.IsZero() {
		snapshot.LastMessageAge = now.Sub(st.lastMessageAt)
		if snapshot.LastMessageAge > wsMessageMaxAge {
			snapshot.WSUptime = 0
			snapshot.WSConnected = false
		}
	}

	snapshot.UptimeScore = clamp(snapshot.WSUptime/s.minUptime(), 0, 1)
	if snapshot.WSReconnects > 0 {
		snapshot.UptimeScore = clamp(snapshot.UptimeScore-(0.05*float64(snapshot.WSReconnects)), 0, 1)
	}

	if len(st.orders) > 0 {
		snapshot.OrdersPlaced = int64(len(st.orders))
		filled := 0
		for _, sample := range st.orders {
			if sample.filled {
				filled++
			}
		}
		snapshot.OrdersFilled = int64(filled)
		snapshot.FillRate = float64(filled) / float64(len(st.orders))
		snapshot.FillRateScore = clamp(snapshot.FillRate/s.minFillRate(), 0, 1)
	}

	snapshot.Score = clamp(
		(snapshot.LatencyScore*exchangeLatencyWeight)+
			(snapshot.UptimeScore*exchangeUptimeWeight)+
			(snapshot.FillRateScore*exchangeFillWeight),
		0, 1,
	)

	return snapshot
}

// Snapshot returns the current score snapshot for a single exchange.
func (s *ExchangeScorer) Snapshot(exchangeName string) ExchangeScoreSnapshot {
	if s == nil || exchangeName == "" {
		return ExchangeScoreSnapshot{Exchange: exchangeName, Score: 1.0, LatencyScore: 1.0, UptimeScore: 1.0, FillRateScore: 1.0, WSUptime: 1.0, FillRate: 1.0}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.state(exchangeName)
	return s.snapshotLocked(exchangeName, st, time.Now())
}

// Snapshots returns the current score snapshot for every exchange with recorded state.
func (s *ExchangeScorer) Snapshots() map[string]ExchangeScoreSnapshot {
	if s == nil {
		return map[string]ExchangeScoreSnapshot{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	out := make(map[string]ExchangeScoreSnapshot, len(s.states))
	for exchangeName, st := range s.states {
		out[exchangeName] = s.snapshotLocked(exchangeName, st, now)
	}
	return out
}

// Score returns the weighted health score for a single exchange.
func (s *ExchangeScorer) Score(exchangeName string) float64 {
	return s.Snapshot(exchangeName).Score
}
