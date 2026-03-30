package api

import (
	"encoding/json"
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
// GET endpoint exposes max_positions, capital_per_position, and account limits.
func TestHandleSpotAutoConfig_GET_ReturnsGuardrails(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	s.cfg.SpotFuturesAutoEnabled = false
	s.cfg.SpotFuturesDryRun = true
	s.cfg.SpotFuturesPersistenceScans = 3
	s.cfg.SpotFuturesMaxPositions = 2
	s.cfg.SpotFuturesCapitalPerPosition = 500
	s.cfg.SpotFuturesSeparateAcctMaxUSDT = 200
	s.cfg.SpotFuturesUnifiedAcctMaxUSDT = 1000

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

	// New guardrail fields.
	if resp.Data["max_positions"] != float64(2) {
		t.Errorf("expected max_positions=2, got %v", resp.Data["max_positions"])
	}
	if resp.Data["capital_per_position"] != float64(500) {
		t.Errorf("expected capital_per_position=500, got %v", resp.Data["capital_per_position"])
	}
	if resp.Data["separate_acct_max_usdt"] != float64(200) {
		t.Errorf("expected separate_acct_max_usdt=200, got %v", resp.Data["separate_acct_max_usdt"])
	}
	if resp.Data["unified_acct_max_usdt"] != float64(1000) {
		t.Errorf("expected unified_acct_max_usdt=1000, got %v", resp.Data["unified_acct_max_usdt"])
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
	s.cfg.SpotFuturesCapitalPerPosition = 200
	s.cfg.SpotFuturesSeparateAcctMaxUSDT = 200
	s.cfg.SpotFuturesUnifiedAcctMaxUSDT = 500

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
	if resp.Data["capital_per_position"] != float64(200) {
		t.Errorf("expected capital_per_position=200, got %v", resp.Data["capital_per_position"])
	}
}
