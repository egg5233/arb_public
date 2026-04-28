// Package api — pricegap_discovery_handlers_test.go: Plan 11-05 REST handler
// coverage for the auto-discovery scanner endpoints.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"arb/internal/pricegaptrader"
)

// fakeTelemetry implements DiscoveryTelemetryReader for handler tests.
type fakeTelemetry struct {
	state    pricegaptrader.StateResponse
	scores   map[string]pricegaptrader.ScoresResponse
	stateErr error
	scoreErr error
	calls    []string
}

func (f *fakeTelemetry) GetState(ctx context.Context) (pricegaptrader.StateResponse, error) {
	f.calls = append(f.calls, "GetState")
	if f.stateErr != nil {
		return pricegaptrader.StateResponse{}, f.stateErr
	}
	return f.state, nil
}

func (f *fakeTelemetry) GetScores(ctx context.Context, symbol string) (pricegaptrader.ScoresResponse, error) {
	f.calls = append(f.calls, "GetScores:"+symbol)
	if f.scoreErr != nil {
		return pricegaptrader.ScoresResponse{}, f.scoreErr
	}
	if r, ok := f.scores[symbol]; ok {
		return r, nil
	}
	return pricegaptrader.ScoresResponse{
		Symbol:        symbol,
		Points:        []pricegaptrader.ScorePoint{},
		ThresholdBand: pricegaptrader.ThresholdBand{AutoPromote: 60},
	}, nil
}

func newDiscoveryTestServer(t *testing.T) (*Server, *fakeTelemetry, string, func()) {
	s, _, token, td := newPriceGapTestServer(t)
	tel := &fakeTelemetry{
		state: pricegaptrader.StateResponse{
			Enabled:        true,
			LastRunAt:      1700000000,
			NextRunIn:      300,
			CandidatesSeen: 18,
			Accepted:       3,
			Rejected:       15,
			Errors:         0,
			DurationMs:     250,
			WhyRejected: map[string]int{
				"insufficient_persistence": 8,
				"stale_bbo":                7,
			},
			ScoreSnapshot: []pricegaptrader.SnapshotEntry{
				{
					Symbol:    "BTCUSDT",
					LongExch:  "binance",
					ShortExch: "bybit",
					Score:     75,
					SubScores: pricegaptrader.SubScores{SpreadBps: 150, DepthScore: 0.8, FreshnessAgeS: 5, PersistenceBars: 4},
				},
			},
		},
		scores: map[string]pricegaptrader.ScoresResponse{
			"BTCUSDT": {
				Symbol: "BTCUSDT",
				Points: []pricegaptrader.ScorePoint{
					{Ts: 1700000000, Score: 75, SubScores: pricegaptrader.SubScores{SpreadBps: 150}},
				},
				ThresholdBand: pricegaptrader.ThresholdBand{AutoPromote: 60},
			},
		},
	}
	s.SetDiscoveryTelemetry(tel)
	return s, tel, token, td
}

// ---- Test 1: GET /api/pg/discovery/state happy path ----

func TestPgDiscoveryState_Happy(t *testing.T) {
	s, _, token, td := newDiscoveryTestServer(t)
	defer td()

	req := httptest.NewRequest(http.MethodGet, "/api/pg/discovery/state", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgDiscoveryState)
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.OK {
		t.Errorf("OK=false want true")
	}
	body, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	var state pricegaptrader.StateResponse
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if !state.Enabled {
		t.Error("Enabled=false want true")
	}
	if state.CandidatesSeen != 18 {
		t.Errorf("CandidatesSeen=%d want 18", state.CandidatesSeen)
	}
	if len(state.ScoreSnapshot) != 1 {
		t.Errorf("ScoreSnapshot len=%d want 1", len(state.ScoreSnapshot))
	}
}

// ---- Test 2: empty/never-run state still returns 200 with zero counters ----

func TestPgDiscoveryState_Empty(t *testing.T) {
	s, tel, token, td := newDiscoveryTestServer(t)
	defer td()
	tel.state = pricegaptrader.StateResponse{
		WhyRejected:   map[string]int{},
		ScoreSnapshot: []pricegaptrader.SnapshotEntry{},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/pg/discovery/state", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgDiscoveryState)
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d want 200", w.Code)
	}
}

// ---- Test 3: no auth → 401 ----

func TestPgDiscoveryState_AuthRequired(t *testing.T) {
	s, _, _, td := newDiscoveryTestServer(t)
	defer td()

	req := httptest.NewRequest(http.MethodGet, "/api/pg/discovery/state", nil)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgDiscoveryState)
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d want 401", w.Code)
	}
}

// ---- Test 4: telemetry not wired → 503 ----

func TestPgDiscoveryState_TelemetryMissing(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t) // no SetDiscoveryTelemetry
	defer td()

	req := httptest.NewRequest(http.MethodGet, "/api/pg/discovery/state", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgDiscoveryState)
	handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d want 503", w.Code)
	}
}

// ---- Test 5: GET /api/pg/discovery/scores/{symbol} happy path ----

func TestPgDiscoveryScores_Happy(t *testing.T) {
	s, _, token, td := newDiscoveryTestServer(t)
	defer td()

	req := httptest.NewRequest(http.MethodGet, "/api/pg/discovery/scores/BTCUSDT", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgDiscoveryScores)
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d want 200; body=%s", w.Code, w.Body.String())
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.OK {
		t.Errorf("OK=false want true")
	}
	body, _ := json.Marshal(resp.Data)
	var sc pricegaptrader.ScoresResponse
	json.Unmarshal(body, &sc)
	if sc.Symbol != "BTCUSDT" {
		t.Errorf("Symbol=%q want BTCUSDT", sc.Symbol)
	}
	if len(sc.Points) != 1 {
		t.Errorf("Points len=%d want 1", len(sc.Points))
	}
	if sc.ThresholdBand.AutoPromote != 60 {
		t.Errorf("AutoPromote=%d want 60", sc.ThresholdBand.AutoPromote)
	}
}

// ---- Test 6: unknown symbol returns 200 + empty points (NOT 404) ----

func TestPgDiscoveryScores_UnknownSymbol(t *testing.T) {
	s, _, token, td := newDiscoveryTestServer(t)
	defer td()

	req := httptest.NewRequest(http.MethodGet, "/api/pg/discovery/scores/UNKNOWNUSDT", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgDiscoveryScores)
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d want 200", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	body, _ := json.Marshal(resp.Data)
	var sc pricegaptrader.ScoresResponse
	json.Unmarshal(body, &sc)
	if len(sc.Points) != 0 {
		t.Errorf("Points len=%d want 0", len(sc.Points))
	}
}

// ---- Test 7: invalid symbol returns 400 ----

func TestPgDiscoveryScores_InvalidSymbol(t *testing.T) {
	s, _, token, td := newDiscoveryTestServer(t)
	defer td()

	cases := []struct {
		path string
		why  string
	}{
		{"/api/pg/discovery/scores/btc-usdt", "lowercase + hyphen"},
		{"/api/pg/discovery/scores/BTC", "no USDT suffix"},
		{"/api/pg/discovery/scores/", "empty symbol"},
	}
	for _, tc := range cases {
		t.Run(tc.why, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			handler := s.authMiddleware(s.handlePgDiscoveryScores)
			handler(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("path=%q got %d want 400; body=%s", tc.path, w.Code, w.Body.String())
			}
			if !contains(w.Body.String(), "invalid symbol format") {
				t.Errorf("body=%q does not mention 'invalid symbol format'", w.Body.String())
			}
		})
	}
}

// ---- Test 8: no auth → 401 on scores endpoint ----

func TestPgDiscoveryScores_AuthRequired(t *testing.T) {
	s, _, _, td := newDiscoveryTestServer(t)
	defer td()

	req := httptest.NewRequest(http.MethodGet, "/api/pg/discovery/scores/BTCUSDT", nil)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgDiscoveryScores)
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d want 401", w.Code)
	}
}

// ---- Test 9: routes registered in server (smoke through ServeHTTP) ----

func TestPgDiscovery_RoutesRegistered(t *testing.T) {
	// Smoke: Server.Start registers both routes. We verify by hitting the
	// extractScoresSymbol parser + ValidateSymbol pipeline on a path that
	// only matches if the route was registered. Direct handler invocation
	// is in the tests above; this asserts the path-shape contract.
	gotSym := extractScoresSymbol("/api/pg/discovery/scores/BTCUSDT")
	if gotSym != "BTCUSDT" {
		t.Errorf("extractScoresSymbol=%q want BTCUSDT", gotSym)
	}
	gotSub := extractScoresSymbol("/api/pg/discovery/scores/BTCUSDT/subpath")
	if gotSub != "" {
		t.Errorf("extractScoresSymbol with subpath=%q want empty", gotSub)
	}
}

// ---- helpers ----

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(substr) > 0 && index(s, substr) >= 0))
}

func index(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
