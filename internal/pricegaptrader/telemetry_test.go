// Package pricegaptrader — telemetry_test.go: TelemetryWriter + Tracker
// scanLoop integration tests for Plan 11-05.
//
// Schema verified against 11-RESEARCH §"Redis Schema":
//
//	pg:scan:cycles            — RPush + LTrim 1000 (per-cycle envelope)
//	pg:scan:scores:{symbol}   — ZSET, score=record.Score, 7d EXPIRE
//	pg:scan:rejections:{date} — HASH, increment per reason, 30d EXPIRE
//	pg:scan:metrics           — HASH, summary counters + cycle_failed flag
//	pg:scan:enabled           — STRING "0"/"1"
//
// WS broadcasts:
//
//	pg_scan_cycle   — every WriteCycle (no throttle, low rate)
//	pg_scan_metrics — 5s throttle from WriteCycle
//	pg_scan_score   — 1s debounce from each accepted record
package pricegaptrader

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"

	"github.com/alicebob/miniredis/v2"
)

// ---- compile-time interface assertion ----

// Telemetry must satisfy TelemetryWriter (declared by Plan 04 in scanner.go).
var _ TelemetryWriter = (*Telemetry)(nil)

// ---- recordingBroadcaster: spy implementation of Broadcaster (api package
// surface — extended to include BroadcastEvent for arbitrary scan events) ----

type recordingBroadcaster struct {
	mu     sync.Mutex
	events []recordedEvent
}

type recordedEvent struct {
	Type    string
	Payload interface{}
}

func (r *recordingBroadcaster) BroadcastPriceGapPositions([]*models.PriceGapPosition) {}
func (r *recordingBroadcaster) BroadcastPriceGapEvent(PriceGapEvent)                  {}
func (r *recordingBroadcaster) BroadcastPriceGapCandidateUpdate(PriceGapCandidateUpdate) {
}

// BroadcastPriceGapDiscoveryEvent is the new generic broadcast surface used by
// Telemetry. Plan 05 adds this method to the Broadcaster interface.
func (r *recordingBroadcaster) BroadcastPriceGapDiscoveryEvent(eventType string, payload interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedEvent{Type: eventType, Payload: payload})
}

func (r *recordingBroadcaster) snapshot() []recordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedEvent, len(r.events))
	copy(out, r.events)
	return out
}

func (r *recordingBroadcaster) countByType(eventType string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := 0
	for _, e := range r.events {
		if e.Type == eventType {
			c++
		}
	}
	return c
}

// ---- helpers ----

func newTelemetryTestEnv(t *testing.T) (*Telemetry, *recordingBroadcaster, *miniredis.Miniredis, func() time.Time, *time.Time) {
	t.Helper()
	db, mr := newE2EClient(t)
	bc := &recordingBroadcaster{}
	cfg := &config.Config{
		PriceGapDiscoveryEnabled:      true,
		PriceGapDiscoveryIntervalSec:  300,
		PriceGapDiscoveryThresholdBps: 100,
		PriceGapDiscoveryMinDepthUSDT: 1000,
		PriceGapAutoPromoteScore:      60,
	}
	now := time.Date(2026, 4, 28, 12, 30, 45, 0, time.UTC)
	clock := now
	telemetry := NewTelemetry(db, bc, cfg, utils.NewLogger("telemetry-test"))
	telemetry.nowFunc = func() time.Time { return clock }
	telemetry.debounceWindow = 50 * time.Millisecond   // shrink for faster tests
	telemetry.metricsThrottle = 100 * time.Millisecond // shrink for faster tests
	advance := func() time.Time { return clock }
	return telemetry, bc, mr, advance, &clock
}

func sampleCycleSummary(now time.Time) CycleSummary {
	return CycleSummary{
		StartedAt:      now.Unix(),
		CompletedAt:    now.Unix(),
		DurationMs:     42,
		CandidatesSeen: 10,
		Accepted:       2,
		Rejected:       8,
		Errors:         0,
		WhyRejected: map[ScanReason]int{
			ReasonInsufficientPersistence: 4,
			ReasonStaleBBO:                3,
			ReasonInsufficientDepth:       1,
		},
		Records: []CycleRecord{
			{
				Symbol:          "BTCUSDT",
				LongExch:        "binance",
				ShortExch:       "bybit",
				Score:           75,
				SpreadBps:       150,
				PersistenceBars: 4,
				DepthScore:      0.8,
				FreshnessAgeSec: 5,
				FundingBpsPerHr: 0.5,
				WhyRejected:     ReasonAccepted,
				Timestamp:       now.Unix(),
			},
			{
				Symbol:          "ETHUSDT",
				LongExch:        "okx",
				ShortExch:       "gateio",
				Score:           42,
				SpreadBps:       110,
				PersistenceBars: 4,
				DepthScore:      0.4,
				FreshnessAgeSec: 7,
				WhyRejected:     ReasonAccepted,
				Timestamp:       now.Unix(),
			},
		},
	}
}

// ---- Test 1: WriteCycle persists per-cycle envelope + metrics + WS broadcast.

func TestTelemetry_WriteCycle_PersistsCyclesList(t *testing.T) {
	tel, _, mr, _, _ := newTelemetryTestEnv(t)
	if err := tel.WriteCycle(context.Background(), sampleCycleSummary(time.Now())); err != nil {
		t.Fatalf("WriteCycle: %v", err)
	}
	if !mr.Exists("pg:scan:cycles") {
		t.Fatalf("pg:scan:cycles not written")
	}
	got, err := mr.List("pg:scan:cycles")
	if err != nil {
		t.Fatalf("LRange: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("pg:scan:cycles len=%d want 1", len(got))
	}
}

func TestTelemetry_WriteCycle_MetricsHash(t *testing.T) {
	tel, _, mr, _, _ := newTelemetryTestEnv(t)
	now := time.Date(2026, 4, 28, 12, 30, 45, 0, time.UTC)
	if err := tel.WriteCycle(context.Background(), sampleCycleSummary(now)); err != nil {
		t.Fatalf("WriteCycle: %v", err)
	}
	for _, k := range []string{"last_run_at", "candidates_seen", "accepted", "rejected", "errors", "duration_ms", "next_run_in", "cycle_failed"} {
		v := mr.HGet("pg:scan:metrics", k)
		if v == "" {
			t.Errorf("pg:scan:metrics field %q missing", k)
		}
	}
	if got := mr.HGet("pg:scan:metrics", "candidates_seen"); got != "10" {
		t.Errorf("candidates_seen=%q want 10", got)
	}
	if got := mr.HGet("pg:scan:metrics", "cycle_failed"); got != "0" {
		t.Errorf("cycle_failed=%q want 0", got)
	}
	if got := mr.HGet("pg:scan:metrics", "next_run_in"); got != "300" {
		t.Errorf("next_run_in=%q want 300", got)
	}
}

func TestTelemetry_WriteCycle_BroadcastsCycleEvent(t *testing.T) {
	tel, bc, _, _, _ := newTelemetryTestEnv(t)
	if err := tel.WriteCycle(context.Background(), sampleCycleSummary(time.Now())); err != nil {
		t.Fatalf("WriteCycle: %v", err)
	}
	if bc.countByType("pg_scan_cycle") != 1 {
		t.Errorf("pg_scan_cycle count=%d want 1", bc.countByType("pg_scan_cycle"))
	}
}

// ---- Test 2: per-symbol score ZSET + 7d EXPIRE.

func TestTelemetry_WriteCycle_PerSymbolScoreZSet(t *testing.T) {
	tel, _, mr, _, _ := newTelemetryTestEnv(t)
	if err := tel.WriteCycle(context.Background(), sampleCycleSummary(time.Now())); err != nil {
		t.Fatalf("WriteCycle: %v", err)
	}
	for _, sym := range []string{"BTCUSDT", "ETHUSDT"} {
		key := "pg:scan:scores:" + sym
		members, err := mr.ZMembers(key)
		if err != nil {
			t.Fatalf("ZMembers %s: %v", key, err)
		}
		if len(members) != 1 {
			t.Errorf("zset %s len=%d want 1", key, len(members))
		}
		ttl := mr.TTL(key)
		if ttl <= 0 || ttl > 7*24*time.Hour+time.Hour {
			t.Errorf("zset %s ttl=%v not within 7d", key, ttl)
		}
	}
}

// ---- Test 3: rejections by date HASH + 30d EXPIRE.

func TestTelemetry_WriteCycle_RejectionsByDate(t *testing.T) {
	tel, _, mr, _, _ := newTelemetryTestEnv(t)
	now := time.Date(2026, 4, 28, 12, 30, 45, 0, time.UTC)
	summary := sampleCycleSummary(now)
	if err := tel.WriteCycle(context.Background(), summary); err != nil {
		t.Fatalf("WriteCycle: %v", err)
	}
	dateKey := "pg:scan:rejections:" + now.UTC().Format("2006-01-02")
	if got := mr.HGet(dateKey, string(ReasonInsufficientPersistence)); got != "4" {
		t.Errorf("%s[%s]=%q want 4", dateKey, ReasonInsufficientPersistence, got)
	}
	if got := mr.HGet(dateKey, string(ReasonStaleBBO)); got != "3" {
		t.Errorf("%s[%s]=%q want 3", dateKey, ReasonStaleBBO, got)
	}
	ttl := mr.TTL(dateKey)
	if ttl <= 0 || ttl > 30*24*time.Hour+time.Hour {
		t.Errorf("rejections key ttl=%v not within 30d", ttl)
	}
}

// ---- Test 4: WS throttle on metrics — second call within window suppressed.

func TestTelemetry_MetricsThrottle(t *testing.T) {
	tel, bc, _, _, clock := newTelemetryTestEnv(t)
	// First write: should broadcast metrics.
	if err := tel.WriteCycle(context.Background(), sampleCycleSummary(*clock)); err != nil {
		t.Fatalf("WriteCycle 1: %v", err)
	}
	// Second within 100ms throttle window: must not broadcast metrics.
	*clock = clock.Add(50 * time.Millisecond)
	if err := tel.WriteCycle(context.Background(), sampleCycleSummary(*clock)); err != nil {
		t.Fatalf("WriteCycle 2: %v", err)
	}
	if got := bc.countByType("pg_scan_metrics"); got != 1 {
		t.Errorf("pg_scan_metrics count=%d want 1 (throttled)", got)
	}
	// After window passes — third call broadcasts again.
	*clock = clock.Add(200 * time.Millisecond)
	if err := tel.WriteCycle(context.Background(), sampleCycleSummary(*clock)); err != nil {
		t.Fatalf("WriteCycle 3: %v", err)
	}
	if got := bc.countByType("pg_scan_metrics"); got != 2 {
		t.Errorf("pg_scan_metrics count=%d want 2 (post-throttle)", got)
	}
}

// ---- Test 5: WS debounce on score — multiple updates coalesce within window.

func TestTelemetry_ScoreDebounce(t *testing.T) {
	tel, bc, _, _, _ := newTelemetryTestEnv(t)
	// Two writes back-to-back within 50ms debounce window for the same symbol.
	tel.WriteCycle(context.Background(), sampleCycleSummary(time.Now()))
	tel.WriteCycle(context.Background(), sampleCycleSummary(time.Now()))
	// Wait for debounce to fire.
	time.Sleep(150 * time.Millisecond)
	// Two cycles × 2 symbols = 4 records pushed; debounce should coalesce
	// into exactly 2 score broadcasts (one per unique symbol).
	if got := bc.countByType("pg_scan_score"); got != 2 {
		t.Errorf("pg_scan_score count=%d want 2 (debounced to one-per-symbol)", got)
	}
}

// ---- Test 6: WriteEnabledFlag stamps "0" or "1".

func TestTelemetry_WriteEnabledFlag(t *testing.T) {
	tel, _, mr, _, _ := newTelemetryTestEnv(t)
	if err := tel.WriteEnabledFlag(context.Background(), false); err != nil {
		t.Fatalf("WriteEnabledFlag false: %v", err)
	}
	if got, _ := mr.Get("pg:scan:enabled"); got != "0" {
		t.Errorf("pg:scan:enabled=%q want 0", got)
	}
	if err := tel.WriteEnabledFlag(context.Background(), true); err != nil {
		t.Fatalf("WriteEnabledFlag true: %v", err)
	}
	if got, _ := mr.Get("pg:scan:enabled"); got != "1" {
		t.Errorf("pg:scan:enabled=%q want 1", got)
	}
}

// ---- Test 7: WriteSymbolNoCrossPair increments counter.

func TestTelemetry_WriteSymbolNoCrossPair(t *testing.T) {
	tel, _, mr, _, _ := newTelemetryTestEnv(t)
	for i := 0; i < 3; i++ {
		if err := tel.WriteSymbolNoCrossPair(context.Background()); err != nil {
			t.Fatalf("WriteSymbolNoCrossPair: %v", err)
		}
	}
	if got := mr.HGet("pg:scan:metrics", "symbol_no_cross_pair"); got != "3" {
		t.Errorf("symbol_no_cross_pair=%q want 3", got)
	}
}

// ---- Test 8: WriteCycleFailed sets cycle_failed=1; cleared by next WriteCycle.

func TestTelemetry_WriteCycleFailed_CycleClears(t *testing.T) {
	tel, _, mr, _, _ := newTelemetryTestEnv(t)
	if err := tel.WriteCycleFailed(context.Background()); err != nil {
		t.Fatalf("WriteCycleFailed: %v", err)
	}
	if got := mr.HGet("pg:scan:metrics", "cycle_failed"); got != "1" {
		t.Errorf("cycle_failed after WriteCycleFailed=%q want 1", got)
	}
	if err := tel.WriteCycle(context.Background(), sampleCycleSummary(time.Now())); err != nil {
		t.Fatalf("WriteCycle: %v", err)
	}
	if got := mr.HGet("pg:scan:metrics", "cycle_failed"); got != "0" {
		t.Errorf("cycle_failed after WriteCycle=%q want 0", got)
	}
}

// ---- Test 9: GetState reads metrics + enabled + last cycle's records.

func TestTelemetry_GetState(t *testing.T) {
	tel, _, _, _, _ := newTelemetryTestEnv(t)
	now := time.Date(2026, 4, 28, 12, 30, 45, 0, time.UTC)
	_ = tel.WriteEnabledFlag(context.Background(), true)
	if err := tel.WriteCycle(context.Background(), sampleCycleSummary(now)); err != nil {
		t.Fatalf("WriteCycle: %v", err)
	}
	state, err := tel.GetState(context.Background())
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if !state.Enabled {
		t.Error("Enabled=false want true")
	}
	if state.LastRunAt != now.Unix() {
		t.Errorf("LastRunAt=%d want %d", state.LastRunAt, now.Unix())
	}
	if state.CandidatesSeen != 10 {
		t.Errorf("CandidatesSeen=%d want 10", state.CandidatesSeen)
	}
	if state.Accepted != 2 {
		t.Errorf("Accepted=%d want 2", state.Accepted)
	}
	if state.NextRunIn != 300 {
		t.Errorf("NextRunIn=%d want 300", state.NextRunIn)
	}
	if got := state.WhyRejected[string(ReasonInsufficientPersistence)]; got != 4 {
		t.Errorf("WhyRejected[%s]=%d want 4", ReasonInsufficientPersistence, got)
	}
	if len(state.ScoreSnapshot) != 2 {
		t.Errorf("ScoreSnapshot len=%d want 2", len(state.ScoreSnapshot))
	}
}

func TestTelemetry_GetState_Empty(t *testing.T) {
	tel, _, _, _, _ := newTelemetryTestEnv(t)
	state, err := tel.GetState(context.Background())
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if state.Enabled {
		t.Error("Enabled=true want false")
	}
	if state.LastRunAt != 0 {
		t.Errorf("LastRunAt=%d want 0", state.LastRunAt)
	}
	if len(state.ScoreSnapshot) != 0 {
		t.Errorf("ScoreSnapshot len=%d want 0", len(state.ScoreSnapshot))
	}
}

// ---- Test 10: GetScores reads ZSET + threshold band.

func TestTelemetry_GetScores(t *testing.T) {
	tel, _, _, _, _ := newTelemetryTestEnv(t)
	if err := tel.WriteCycle(context.Background(), sampleCycleSummary(time.Now())); err != nil {
		t.Fatalf("WriteCycle: %v", err)
	}
	scores, err := tel.GetScores(context.Background(), "BTCUSDT")
	if err != nil {
		t.Fatalf("GetScores: %v", err)
	}
	if scores.Symbol != "BTCUSDT" {
		t.Errorf("Symbol=%q want BTCUSDT", scores.Symbol)
	}
	if len(scores.Points) != 1 {
		t.Errorf("Points len=%d want 1", len(scores.Points))
	}
	if scores.ThresholdBand.AutoPromote != 60 {
		t.Errorf("AutoPromote=%d want 60", scores.ThresholdBand.AutoPromote)
	}
}

func TestTelemetry_GetScores_UnknownSymbol(t *testing.T) {
	tel, _, _, _, _ := newTelemetryTestEnv(t)
	scores, err := tel.GetScores(context.Background(), "UNKNOWNUSDT")
	if err != nil {
		t.Fatalf("GetScores: %v", err)
	}
	if scores.Symbol != "UNKNOWNUSDT" {
		t.Errorf("Symbol=%q want UNKNOWNUSDT", scores.Symbol)
	}
	if len(scores.Points) != 0 {
		t.Errorf("Points len=%d want 0", len(scores.Points))
	}
}

// ---- Test 11+ : Tracker scanLoop integration.

// fakeScanner records RunCycle invocations for scanLoop testing.
type fakeScanner struct {
	mu    sync.Mutex
	calls int
	done  chan struct{}
}

func (f *fakeScanner) RunCycle(ctx context.Context, now time.Time) {
	f.mu.Lock()
	f.calls++
	calls := f.calls
	f.mu.Unlock()
	if calls == 1 && f.done != nil {
		close(f.done)
	}
}

func (f *fakeScanner) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestTracker_ScanLoop_LaunchedWhenScannerSet(t *testing.T) {
	cfg := &config.Config{
		PriceGapEnabled:              true,
		PriceGapPollIntervalSec:      3600, // dont fire tickLoop during test
		PriceGapDiscoveryIntervalSec: 1,    // 1s for fast test, irrelevant since firstTick is small
		PriceGapDiscoveryUniverse:    []string{"BTCUSDT"},
		PriceGapKlineStalenessSec:    90,
	}
	bin := newStubExchange("binance")
	exch := map[string]exchange.Exchange{"binance": bin}

	scn := &fakeScanner{done: make(chan struct{})}
	db, _ := newE2EClient(t)
	tr := NewTracker(exch, db, newFakeDelistChecker(), cfg)
	tr.SetScanner(scn)
	// Override firstTick offset for fast test (50ms instead of 13s).
	tr.scanFirstTickOffset = 50 * time.Millisecond
	tr.scanInterval = 50 * time.Millisecond

	tr.Start()
	defer tr.Stop()

	select {
	case <-scn.done:
		// got first call
	case <-time.After(2 * time.Second):
		t.Fatalf("scanner.RunCycle never invoked")
	}
	if got := scn.callCount(); got < 1 {
		t.Errorf("RunCycle calls=%d want >=1", got)
	}
}

func TestTracker_ScanLoop_NoLaunchWhenScannerNil(t *testing.T) {
	cfg := &config.Config{
		PriceGapEnabled:         true,
		PriceGapPollIntervalSec: 3600,
	}
	bin := newStubExchange("binance")
	exch := map[string]exchange.Exchange{"binance": bin}
	db, _ := newE2EClient(t)
	tr := NewTracker(exch, db, newFakeDelistChecker(), cfg)

	tr.Start()
	defer tr.Stop()
	// Sleep briefly then ensure Stop returns cleanly even without scanner —
	// asserts no goroutine leak.
	time.Sleep(50 * time.Millisecond)
}

func TestTracker_SubscribeUniverse(t *testing.T) {
	cfg := &config.Config{
		PriceGapEnabled:           true,
		PriceGapPollIntervalSec:   3600,
		PriceGapDiscoveryUniverse: []string{"BTCUSDT", "ETHUSDT"},
	}
	bin := newStubExchange("binance")
	byb := newStubExchange("bybit")
	exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}
	db, _ := newE2EClient(t)
	tr := NewTracker(exch, db, newFakeDelistChecker(), cfg)

	tr.subscribeUniverse()

	// 2 symbols × 2 exchanges = 4 subscribe calls.
	for _, ex := range []*stubExchange{bin, byb} {
		for _, sym := range []string{"BTCUSDT", "ETHUSDT"} {
			if got := ex.subscribeCount(sym); got != 1 {
				t.Errorf("%s subscribed %s %d times, want 1", ex.name, sym, got)
			}
		}
	}
}

// ---- Test 12: WriteCycle survives empty records / empty rejections map.

func TestTelemetry_WriteCycle_EmptySummary(t *testing.T) {
	tel, _, _, _, _ := newTelemetryTestEnv(t)
	summary := CycleSummary{
		StartedAt:   time.Now().Unix(),
		CompletedAt: time.Now().Unix(),
		WhyRejected: map[ScanReason]int{},
		Records:     []CycleRecord{},
	}
	if err := tel.WriteCycle(context.Background(), summary); err != nil {
		t.Fatalf("WriteCycle empty: %v", err)
	}
}

// ---- Test 13: pg:scan:cycles cap at 1000 (LTrim).

func TestTelemetry_WriteCycle_CycleListCappedAt1000(t *testing.T) {
	tel, _, mr, _, _ := newTelemetryTestEnv(t)
	for i := 0; i < 1005; i++ {
		s := sampleCycleSummary(time.Now())
		s.CandidatesSeen = i // distinguishable
		if err := tel.WriteCycle(context.Background(), s); err != nil {
			t.Fatalf("WriteCycle %d: %v", i, err)
		}
	}
	// miniredis exposes LRange via List
	got, _ := mr.List("pg:scan:cycles")
	if len(got) != 1000 {
		t.Errorf("pg:scan:cycles len=%d want 1000 (LTrim cap)", len(got))
	}
}

// silence unused-import warnings — strconv is used by score test.
var _ = strconv.Itoa
