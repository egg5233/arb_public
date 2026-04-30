// Package api — pricegap_promote_events_test.go: Plan 12-02 Task 2 coverage
// for GET /api/pg/discovery/promote-events (D-12 REST seed).
//
// The handler reads pg:promote:events LIST (RPushed by Plan 12-02
// RedisWSPromoteSink), unmarshals each entry as a PromoteEvent, and returns
// the result NEWEST-FIRST so the dashboard timeline renders top-down without
// a client-side sort.
//
// Auth: Bearer token via the existing authMiddleware (mirrors
// handlePgDiscoveryState pattern).
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"arb/internal/pricegaptrader"
)

// seedPromoteEvent RPushes one PromoteEvent (oldest-first ordering matches
// production write path).
func seedPromoteEvent(t *testing.T, s *Server, ev pricegaptrader.PromoteEvent) {
	t.Helper()
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal PromoteEvent: %v", err)
	}
	if err := s.db.Redis().RPush(t.Context(), "pg:promote:events", raw).Err(); err != nil {
		t.Fatalf("RPush: %v", err)
	}
}

// ---- Test 5: returns the LIST in newest-first order ----

func TestHandlePgDiscoveryPromoteEvents_ReturnsNewestFirst(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	// Seed three events oldest → newest (RPush semantics: oldest at index 0).
	seedPromoteEvent(t, s, pricegaptrader.PromoteEvent{
		TS: 100, Action: "promote", Symbol: "BTCUSDT",
		LongExch: "binance", ShortExch: "bybit", Direction: "bidirectional",
		Score: 80, StreakCycles: 6, Reason: "score_threshold_met",
	})
	seedPromoteEvent(t, s, pricegaptrader.PromoteEvent{
		TS: 200, Action: "demote", Symbol: "ETHUSDT",
		LongExch: "okx", ShortExch: "gateio", Direction: "bidirectional",
		Score: 40, StreakCycles: 6, Reason: "score_below_threshold",
	})
	seedPromoteEvent(t, s, pricegaptrader.PromoteEvent{
		TS: 300, Action: "promote", Symbol: "SOLUSDT",
		LongExch: "binance", ShortExch: "okx", Direction: "bidirectional",
		Score: 75, StreakCycles: 7, Reason: "score_threshold_met",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/pg/discovery/promote-events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgDiscoveryPromoteEvents)
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d want 200; body=%s", w.Code, w.Body.String())
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if !resp.OK {
		t.Errorf("OK=false; want true")
	}
	body, _ := json.Marshal(resp.Data)
	var events []pricegaptrader.PromoteEvent
	if err := json.Unmarshal(body, &events); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("len=%d want 3", len(events))
	}
	// Newest first: TS=300 then 200 then 100.
	if events[0].TS != 300 || events[1].TS != 200 || events[2].TS != 100 {
		t.Errorf("ordering not newest-first: TS=[%d,%d,%d] want [300,200,100]",
			events[0].TS, events[1].TS, events[2].TS)
	}
	if events[0].Symbol != "SOLUSDT" {
		t.Errorf("events[0].Symbol=%q want SOLUSDT", events[0].Symbol)
	}
}

// ---- Test 6: empty LIST returns { ok: true, data: [] } ----

func TestHandlePgDiscoveryPromoteEvents_EmptyList(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	req := httptest.NewRequest(http.MethodGet, "/api/pg/discovery/promote-events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgDiscoveryPromoteEvents)
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d want 200", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.OK {
		t.Errorf("OK=false; want true")
	}
	body, _ := json.Marshal(resp.Data)
	var events []pricegaptrader.PromoteEvent
	if err := json.Unmarshal(body, &events); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("len=%d want 0 (empty LIST)", len(events))
	}
}

// ---- Test 7: missing/invalid auth → 401 ----

func TestHandlePgDiscoveryPromoteEvents_RequiresAuth(t *testing.T) {
	s, _, _, td := newPriceGapTestServer(t)
	defer td()

	req := httptest.NewRequest(http.MethodGet, "/api/pg/discovery/promote-events", nil)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgDiscoveryPromoteEvents)
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d want 401", w.Code)
	}
}

// ---- Test 8: malformed entries are skipped, valid ones still returned ----

func TestHandlePgDiscoveryPromoteEvents_SkipsMalformedEntries(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	// One valid + one garbage + one valid.
	seedPromoteEvent(t, s, pricegaptrader.PromoteEvent{
		TS: 100, Action: "promote", Symbol: "BTCUSDT",
		LongExch: "binance", ShortExch: "bybit", Direction: "bidirectional",
		Score: 80, StreakCycles: 6, Reason: "score_threshold_met",
	})
	if err := s.db.Redis().RPush(t.Context(), "pg:promote:events", "{not-valid-json").Err(); err != nil {
		t.Fatalf("seed garbage: %v", err)
	}
	seedPromoteEvent(t, s, pricegaptrader.PromoteEvent{
		TS: 200, Action: "demote", Symbol: "ETHUSDT",
		LongExch: "okx", ShortExch: "gateio", Direction: "bidirectional",
		Score: 40, StreakCycles: 6, Reason: "score_below_threshold",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/pg/discovery/promote-events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgDiscoveryPromoteEvents)
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d want 200 (T-12-07 mitigation: skip malformed)", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	body, _ := json.Marshal(resp.Data)
	var events []pricegaptrader.PromoteEvent
	json.Unmarshal(body, &events)
	if len(events) != 2 {
		t.Errorf("len=%d want 2 (garbage skipped, two valids returned)", len(events))
	}
}
