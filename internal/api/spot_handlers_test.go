package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"arb/internal/models"
)

// TestHandleSpotPositionHealth_WithTimestamps verifies that the health endpoint
// returns last_borrow_rate_check and negative_yield_since when populated.
func TestHandleSpotPositionHealth_WithTimestamps(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	now := time.Now().UTC().Truncate(time.Second)
	negSince := now.Add(-30 * time.Minute)
	pos := &models.SpotFuturesPosition{
		ID:                  "test-pos-1",
		Symbol:              "BTC",
		Exchange:            "binance",
		Direction:           "borrow_sell_long",
		Status:              "active",
		LastBorrowRateCheck: now,
		NegativeYieldSince:  &negSince,
		CurrentBorrowAPR:    0.12,
		CreatedAt:           now.Add(-2 * time.Hour),
	}
	if err := s.db.SaveSpotPosition(pos); err != nil {
		t.Fatalf("save position: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/spot/positions/{id}/health", s.handleSpotPositionHealth)

	req := httptest.NewRequest(http.MethodGet, "/api/spot/positions/test-pos-1/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		OK   bool                   `json:"ok"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected ok=true")
	}

	// Verify new fields are present.
	if resp.Data["last_borrow_rate_check"] == nil {
		t.Error("expected last_borrow_rate_check to be non-null")
	}
	if resp.Data["negative_yield_since"] == nil {
		t.Error("expected negative_yield_since to be non-null")
	}

	// Verify existing fields still present.
	if resp.Data["negative_yield"] != true {
		t.Errorf("expected negative_yield=true, got %v", resp.Data["negative_yield"])
	}
	if resp.Data["position_id"] != "test-pos-1" {
		t.Errorf("expected position_id=test-pos-1, got %v", resp.Data["position_id"])
	}
}

// TestHandleSpotPositionHealth_NullTimestamps verifies that unset timestamps
// return null instead of year-zero values.
func TestHandleSpotPositionHealth_NullTimestamps(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	// Direction B position: no borrow leg, no negative yield.
	pos := &models.SpotFuturesPosition{
		ID:        "test-pos-2",
		Symbol:    "ETH",
		Exchange:  "bybit",
		Direction: "buy_spot_short",
		Status:    "active",
		// LastBorrowRateCheck is zero value.
		// NegativeYieldSince is nil.
		CreatedAt: time.Now().UTC(),
	}
	if err := s.db.SaveSpotPosition(pos); err != nil {
		t.Fatalf("save position: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/spot/positions/{id}/health", s.handleSpotPositionHealth)

	req := httptest.NewRequest(http.MethodGet, "/api/spot/positions/test-pos-2/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		OK   bool                   `json:"ok"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Both timestamps should be null for Direction B with no monitor data.
	if resp.Data["last_borrow_rate_check"] != nil {
		t.Errorf("expected last_borrow_rate_check=null, got %v", resp.Data["last_borrow_rate_check"])
	}
	if resp.Data["negative_yield_since"] != nil {
		t.Errorf("expected negative_yield_since=null, got %v", resp.Data["negative_yield_since"])
	}
	if resp.Data["negative_yield"] != false {
		t.Errorf("expected negative_yield=false, got %v", resp.Data["negative_yield"])
	}
}

// TestHandleSpotAutoConfig_GET_ReturnsGuardrails verifies that the auto-config
// GET endpoint exposes max_positions and capital limits.
func TestHandleSpotAutoConfig_GET_ReturnsGuardrails(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	s.cfg.SpotFuturesAutoEnabled = false
	s.cfg.SpotFuturesDryRun = true
	s.cfg.SpotFuturesPersistenceScans = 3
	s.cfg.SpotFuturesMaxPositions = 2
	s.cfg.SpotFuturesCapitalSeparate = 200
	s.cfg.SpotFuturesCapitalUnified = 1000

	req := httptest.NewRequest(http.MethodGet, "/api/spot/config/auto", nil)
	w := httptest.NewRecorder()
	s.handleSpotAutoConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		OK   bool                   `json:"ok"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected ok=true")
	}

	// Existing fields.
	if resp.Data["auto_enabled"] != false {
		t.Errorf("expected auto_enabled=false, got %v", resp.Data["auto_enabled"])
	}
	if resp.Data["dry_run"] != true {
		t.Errorf("expected dry_run=true, got %v", resp.Data["dry_run"])
	}
	if resp.Data["persistence_scans"] != float64(3) {
		t.Errorf("expected persistence_scans=3, got %v", resp.Data["persistence_scans"])
	}

	// Guardrail fields.
	if resp.Data["max_positions"] != float64(2) {
		t.Errorf("expected max_positions=2, got %v", resp.Data["max_positions"])
	}
	if resp.Data["capital_separate_usdt"] != float64(200) {
		t.Errorf("expected capital_separate_usdt=200, got %v", resp.Data["capital_separate_usdt"])
	}
	if resp.Data["capital_unified_usdt"] != float64(1000) {
		t.Errorf("expected capital_unified_usdt=1000, got %v", resp.Data["capital_unified_usdt"])
	}
}

// TestHandleSpotAutoConfig_POST_EchoesWidenedPayload verifies that POST still
// only writes toggle fields but responds with the full widened payload.
func TestHandleSpotAutoConfig_POST_EchoesWidenedPayload(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	s.cfg.SpotFuturesAutoEnabled = false
	s.cfg.SpotFuturesDryRun = true
	s.cfg.SpotFuturesPersistenceScans = 2
	s.cfg.SpotFuturesMaxPositions = 1
	s.cfg.SpotFuturesCapitalSeparate = 200
	s.cfg.SpotFuturesCapitalUnified = 500

	body := `{"enabled": true, "dry_run": false}`
	req := httptest.NewRequest(http.MethodPost, "/api/spot/config/auto", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.handleSpotAutoConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		OK   bool                   `json:"ok"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Toggle fields should reflect the POST update.
	if resp.Data["auto_enabled"] != true {
		t.Errorf("expected auto_enabled=true after POST, got %v", resp.Data["auto_enabled"])
	}
	if resp.Data["dry_run"] != false {
		t.Errorf("expected dry_run=false after POST, got %v", resp.Data["dry_run"])
	}

	// Guardrail fields should be present in POST response.
	if resp.Data["max_positions"] != float64(1) {
		t.Errorf("expected max_positions=1, got %v", resp.Data["max_positions"])
	}
	if resp.Data["capital_separate_usdt"] != float64(200) {
		t.Errorf("expected capital_separate_usdt=200, got %v", resp.Data["capital_separate_usdt"])
	}
}

func TestHandleSpotManualOpen_RejectsFilteredOpportunity(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	s.spotOpenPosition = func(string, string, string, ManualOpenOptions) error {
		return errors.New("opportunity BTCUSDT on stub (buy_spot_short) is filtered: margin unavailable")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/spot/open", strings.NewReader(`{"symbol":"BTCUSDT","exchange":"stub","direction":"buy_spot_short"}`))
	w := httptest.NewRecorder()
	s.handleSpotManualOpen(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSpotManualOpen_ReturnsAcceptedForPendingConfirmation(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	s.spotOpenPosition = func(string, string, string, ManualOpenOptions) error {
		return errors.New("spot entry pending confirmation on order spot-123")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/spot/open", strings.NewReader(`{"symbol":"BTCUSDT","exchange":"stub","direction":"buy_spot_short"}`))
	w := httptest.NewRecorder()
	s.handleSpotManualOpen(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		OK   bool                   `json:"ok"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected ok=true")
	}
	if resp.Data["status"] != "pending" {
		t.Fatalf("expected pending status, got %v", resp.Data["status"])
	}
}

func TestHandleSpotManualOpen_PassesOverrideStrategyPriority(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	var got ManualOpenOptions
	s.SetSpotOpenHandlerWithOptions(func(symbol, exchange, direction string, opts ManualOpenOptions) error {
		if symbol != "BTCUSDT" || exchange != "stub" || direction != "buy_spot_short" {
			t.Fatalf("unexpected spot open tuple: %s %s %s", symbol, exchange, direction)
		}
		got = opts
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/api/spot/open", strings.NewReader(`{"symbol":"BTCUSDT","exchange":"stub","direction":"buy_spot_short","override_strategy_priority":true}`))
	w := httptest.NewRecorder()
	s.handleSpotManualOpen(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !got.OverrideStrategyPriority {
		t.Fatal("expected override_strategy_priority option true")
	}
}

// TestHandleSpotBacktest_AcceptsBothDirections verifies the handler routes
// Dir B (buy_spot_short) and Dir A (borrow_sell_long) to the engine, and
// rejects unknown directions with 400 before hitting the engine.
func TestHandleSpotBacktest_AcceptsBothDirections(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	var gotDirections []string
	s.spotRunBacktest = func(_ context.Context, symbol, exchange, direction string, days int) (interface{}, error) {
		gotDirections = append(gotDirections, direction)
		return map[string]any{"symbol": symbol, "exchange": exchange, "direction": direction, "days": days}, nil
	}

	cases := []struct {
		name       string
		direction  string
		wantStatus int
	}{
		{"dirB", "buy_spot_short", http.StatusOK},
		{"dirA", "borrow_sell_long", http.StatusOK},
		{"unknown", "long_only", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"symbol":"BTCUSDT","exchange":"binance","direction":"` + tc.direction + `","days":7}`
			req := httptest.NewRequest(http.MethodPost, "/api/spot/backtest", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			s.handleSpotBacktest(w, req)
			if w.Code != tc.wantStatus {
				t.Fatalf("%s: expected %d, got %d: %s", tc.name, tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}

	// Engine must have been called for Dir B and Dir A but NOT for unknown.
	if len(gotDirections) != 2 {
		t.Fatalf("engine should have been called twice (Dir B + Dir A), got %d: %v", len(gotDirections), gotDirections)
	}
	if gotDirections[0] != "buy_spot_short" || gotDirections[1] != "borrow_sell_long" {
		t.Fatalf("unexpected direction order: %v", gotDirections)
	}
}

// TestHandleSpotBacktest_EngineErrorSurfacesAs400 verifies the handler maps
// engine errors (e.g. "unsupported on okx") to 400 so the UI can show them inline.
func TestHandleSpotBacktest_EngineErrorSurfacesAs400(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	s.spotRunBacktest = func(_ context.Context, _, _, _ string, _ int) (interface{}, error) {
		return nil, errors.New("historical borrow rate not supported on okx")
	}

	body := `{"symbol":"BTCUSDT","exchange":"okx","direction":"borrow_sell_long","days":7}`
	req := httptest.NewRequest(http.MethodPost, "/api/spot/backtest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleSpotBacktest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
