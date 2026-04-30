// Package pricegaptrader — promote_event_sink_test.go: Plan 12-02 Task 1
// coverage for RedisWSPromoteSink (Redis pg:promote:events LIST + WS
// hub.Broadcast) and Telemetry.IncCapFullSkip (HASH HINCRBY pattern).
//
// Schema verified against 12-02-PLAN behavior contract:
//
//	pg:promote:events       LIST  RPush + LTrim 1000 (newest at tail)
//	pg:scan:metrics         HASH  HINCRBY cap_full_skips:{symbol}
//
// WS broadcast policy (D-11):
//
//	pg_promote_event       — every Emit (no throttle, low rate)
//
// Tests use the existing newE2EClient miniredis helper from tracker_test.go.
package pricegaptrader

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"arb/internal/config"
	"arb/pkg/utils"
)

// ---- compile-time interface assertions ----

// RedisWSPromoteSink must satisfy PromoteEventSink (declared by Plan 01).
var _ PromoteEventSink = (*RedisWSPromoteSink)(nil)

// Telemetry must satisfy TelemetrySink (declared by Plan 01) — provides the
// IncCapFullSkip method that PromotionController consults on cap-full skip.
var _ TelemetrySink = (*Telemetry)(nil)

// ---- fakeHub: spy implementation of WSBroadcaster ----

type hubCall struct {
	EventType string
	Payload   interface{}
}

type fakeHub struct {
	mu    sync.Mutex
	calls []hubCall
}

func (f *fakeHub) Broadcast(eventType string, payload interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, hubCall{EventType: eventType, Payload: payload})
}

func (f *fakeHub) snapshot() []hubCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]hubCall, len(f.calls))
	copy(out, f.calls)
	return out
}

// ---- Test 1: Emit writes a single JSON-encoded event to pg:promote:events ----

func TestRedisWSPromoteSink_EmitWritesToRedis(t *testing.T) {
	db, mr := newE2EClient(t)
	hub := &fakeHub{}
	sink := NewRedisWSPromoteSink(db, hub)

	ev := PromoteEvent{
		TS:           1700000000000,
		Action:       "promote",
		Symbol:       "BTCUSDT",
		LongExch:     "binance",
		ShortExch:    "bybit",
		Direction:    "bidirectional",
		Score:        82,
		StreakCycles: 6,
		Reason:       "score_threshold_met",
	}
	if err := sink.Emit(context.Background(), ev); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	if !mr.Exists("pg:promote:events") {
		t.Fatalf("pg:promote:events LIST not written")
	}
	items, err := mr.List("pg:promote:events")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("LIST len=%d want 1", len(items))
	}
	var got PromoteEvent
	if err := json.Unmarshal([]byte(items[0]), &got); err != nil {
		t.Fatalf("unmarshal: %v; raw=%s", err, items[0])
	}
	if got != ev {
		t.Errorf("event round-trip drift\n got=%+v\nwant=%+v", got, ev)
	}
}

// ---- Test 2: Emit > 1000 events triggers LTrim 1000 ----

func TestRedisWSPromoteSink_EmitTrimsTo1000(t *testing.T) {
	db, mr := newE2EClient(t)
	sink := NewRedisWSPromoteSink(db, nil) // hub may be nil

	const N = 1500
	for i := 0; i < N; i++ {
		ev := PromoteEvent{
			TS:           int64(i),
			Action:       "promote",
			Symbol:       "BTCUSDT",
			LongExch:     "binance",
			ShortExch:    "bybit",
			Direction:    "bidirectional",
			Score:        60 + i%20,
			StreakCycles: 6,
			Reason:       "score_threshold_met",
		}
		if err := sink.Emit(context.Background(), ev); err != nil {
			t.Fatalf("Emit[%d]: %v", i, err)
		}
	}
	items, err := mr.List("pg:promote:events")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1000 {
		t.Fatalf("LIST len=%d want 1000 (LTrim should drop oldest 500)", len(items))
	}
	// Oldest retained should be TS=500 (events 0..499 trimmed off).
	var first PromoteEvent
	if err := json.Unmarshal([]byte(items[0]), &first); err != nil {
		t.Fatalf("unmarshal first: %v", err)
	}
	if first.TS != 500 {
		t.Errorf("oldest retained TS=%d want 500", first.TS)
	}
	// Newest (last) entry should be TS=1499.
	var last PromoteEvent
	if err := json.Unmarshal([]byte(items[len(items)-1]), &last); err != nil {
		t.Fatalf("unmarshal last: %v", err)
	}
	if last.TS != int64(N-1) {
		t.Errorf("newest TS=%d want %d", last.TS, N-1)
	}
}

// ---- Test 3: Emit calls hub.Broadcast with eventType "pg_promote_event" ----

func TestRedisWSPromoteSink_EmitCallsBroadcastWithEventType(t *testing.T) {
	db, _ := newE2EClient(t)
	hub := &fakeHub{}
	sink := NewRedisWSPromoteSink(db, hub)

	ev := PromoteEvent{
		TS:           1700000000111,
		Action:       "demote",
		Symbol:       "ETHUSDT",
		LongExch:     "okx",
		ShortExch:    "gateio",
		Direction:    "bidirectional",
		Score:        40,
		StreakCycles: 6,
		Reason:       "score_below_threshold",
	}
	if err := sink.Emit(context.Background(), ev); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	calls := hub.snapshot()
	if len(calls) != 1 {
		t.Fatalf("hub calls=%d want 1", len(calls))
	}
	if calls[0].EventType != "pg_promote_event" {
		t.Errorf("eventType=%q want pg_promote_event", calls[0].EventType)
	}
	gotEv, ok := calls[0].Payload.(PromoteEvent)
	if !ok {
		t.Fatalf("payload type=%T want PromoteEvent", calls[0].Payload)
	}
	if gotEv != ev {
		t.Errorf("payload drift\n got=%+v\nwant=%+v", gotEv, ev)
	}
}

// ---- Test 4: Telemetry.IncCapFullSkip increments the per-symbol HASH field ----

func TestTelemetry_IncCapFullSkipIncrementsHashField(t *testing.T) {
	db, mr := newE2EClient(t)
	tel := NewTelemetry(db, nil, &config.Config{}, utils.NewLogger("test"))

	ctx := context.Background()
	if err := tel.IncCapFullSkip(ctx, "BTCUSDT"); err != nil {
		t.Fatalf("first IncCapFullSkip: %v", err)
	}
	if err := tel.IncCapFullSkip(ctx, "BTCUSDT"); err != nil {
		t.Fatalf("second IncCapFullSkip: %v", err)
	}
	got := mr.HGet("pg:scan:metrics", "cap_full_skips:BTCUSDT")
	if got != "2" {
		t.Errorf("cap_full_skips:BTCUSDT=%q want \"2\"", got)
	}

	// Per-symbol isolation: a second symbol gets its own counter.
	if err := tel.IncCapFullSkip(ctx, "ETHUSDT"); err != nil {
		t.Fatalf("ETH IncCapFullSkip: %v", err)
	}
	got2 := mr.HGet("pg:scan:metrics", "cap_full_skips:ETHUSDT")
	if got2 != "1" {
		t.Errorf("cap_full_skips:ETHUSDT=%q want \"1\"", got2)
	}

	// Empty symbol is silently ignored (defensive — prevents poisoning the
	// HASH with a "cap_full_skips:" zero-suffix field).
	if err := tel.IncCapFullSkip(ctx, ""); err != nil {
		t.Errorf("empty-symbol IncCapFullSkip should be no-op, got: %v", err)
	}
	if mr.Exists("pg:scan:metrics") {
		// fine — BTC + ETH writes already exist
		fields, err := mr.HKeys("pg:scan:metrics")
		if err != nil {
			t.Fatalf("HKeys: %v", err)
		}
		for _, f := range fields {
			if f == "cap_full_skips:" {
				t.Errorf("empty-symbol path wrote a poisoned field 'cap_full_skips:'")
			}
		}
	}
}
