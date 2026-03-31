package risk

import (
	"testing"
	"time"

	"arb/internal/config"
)

func newTestExchangeScorer() *ExchangeScorer {
	return NewExchangeScorer(&config.Config{
		ExchHealthLatencyMs:   2000,
		ExchHealthMinUptime:   0.95,
		ExchHealthMinFillRate: 0.80,
		ExchHealthMinScore:    0.50,
		ExchHealthWindowMin:   60,
	})
}

func TestExchangeScorer_AllMetricsHealthy(t *testing.T) {
	scorer := newTestExchangeScorer()
	now := time.Now()
	st := scorer.state("binance")
	st.requests = []latencySample{{timestamp: now.Add(-time.Minute), latency: 100 * time.Millisecond}}
	st.wsTransitions = []wsTransition{{timestamp: now.Add(-30 * time.Minute), connected: true}}
	st.lastMessageAt = now.Add(-10 * time.Second)
	st.orders["o1"] = orderMetric{placedAt: now.Add(-time.Minute), filled: true}

	got := scorer.snapshotLocked("binance", st, now)
	if got.Score < 0.98 || got.Score > 0.99 {
		t.Fatalf("score = %.4f, want about 0.985", got.Score)
	}
}

func TestExchangeScorer_HighLatencyStillPassesWSAndFillWeight(t *testing.T) {
	scorer := newTestExchangeScorer()
	now := time.Now()
	st := scorer.state("bybit")
	st.requests = []latencySample{{timestamp: now.Add(-time.Minute), latency: 1500 * time.Millisecond}}
	st.wsTransitions = []wsTransition{{timestamp: now.Add(-30 * time.Minute), connected: true}}
	st.lastMessageAt = now.Add(-5 * time.Second)
	st.orders["o1"] = orderMetric{placedAt: now.Add(-time.Minute), filled: true}

	got := scorer.snapshotLocked("bybit", st, now)
	if got.Score < 0.77 || got.Score > 0.78 {
		t.Fatalf("score = %.4f, want about 0.775", got.Score)
	}
}

func TestExchangeScorer_WSDownDropsScoreToLatencyAndFillOnly(t *testing.T) {
	scorer := newTestExchangeScorer()
	now := time.Now()
	st := scorer.state("okx")
	st.requests = []latencySample{{timestamp: now.Add(-time.Minute), latency: 50 * time.Millisecond}}
	st.wsTransitions = []wsTransition{{timestamp: now.Add(-20 * time.Minute), connected: false}}
	st.orders["o1"] = orderMetric{placedAt: now.Add(-time.Minute), filled: true}

	got := scorer.snapshotLocked("okx", st, now)
	if got.Score < 0.59 || got.Score > 0.61 {
		t.Fatalf("score = %.4f, want about 0.6", got.Score)
	}
}

func TestExchangeScorer_AllMetricsDegradedBlocks(t *testing.T) {
	scorer := newTestExchangeScorer()
	now := time.Now()
	st := scorer.state("gateio")
	st.requests = []latencySample{{timestamp: now.Add(-time.Minute), latency: 5 * time.Second, failed: true}}
	st.wsTransitions = []wsTransition{{timestamp: now.Add(-15 * time.Minute), connected: false}}
	st.orders["o1"] = orderMetric{placedAt: now.Add(-time.Minute), filled: false}

	got := scorer.snapshotLocked("gateio", st, now)
	if got.Score >= scorer.minScore() {
		t.Fatalf("score = %.4f, want below %.2f", got.Score, scorer.minScore())
	}
}

func TestExchangeScorer_ColdStartStaysOptimistic(t *testing.T) {
	scorer := newTestExchangeScorer()
	got := scorer.Snapshot("bingx")
	if got.Score != 1.0 {
		t.Fatalf("score = %.4f, want 1.0", got.Score)
	}
}

func TestExchangeScorer_RecoveryAfterOutageRestoresScore(t *testing.T) {
	scorer := newTestExchangeScorer()
	now := time.Now()
	st := scorer.state("bitget")
	st.requests = []latencySample{
		{timestamp: now.Add(-2 * time.Minute), latency: 200 * time.Millisecond},
		{timestamp: now.Add(-time.Minute), latency: 250 * time.Millisecond},
	}
	st.wsTransitions = []wsTransition{
		{timestamp: now.Add(-20 * time.Minute), connected: false},
		{timestamp: now.Add(-5 * time.Minute), connected: true},
	}
	st.lastMessageAt = now.Add(-5 * time.Second)
	st.orders["o1"] = orderMetric{placedAt: now.Add(-2 * time.Minute), filled: true}
	st.orders["o2"] = orderMetric{placedAt: now.Add(-time.Minute), filled: true}

	got := scorer.snapshotLocked("bitget", st, now)
	if got.Score <= scorer.minScore() {
		t.Fatalf("score = %.4f, want above %.2f after recovery", got.Score, scorer.minScore())
	}
}
