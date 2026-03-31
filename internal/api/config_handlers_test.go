package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"arb/internal/config"
)

func TestHandlePostConfig_PersistsDisabledSpotFutures(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	s.cfg.SpotFuturesEnabled = true
	s.cfg.SpotFuturesMaxPositions = 2
	s.cfg.SpotFuturesCapitalPerPosition = 500
	s.cfg.SpotFuturesLeverage = 3
	s.cfg.SpotFuturesMonitorIntervalSec = 60
	s.cfg.SpotFuturesMinNetYieldAPR = 0.12
	s.cfg.SpotFuturesMaxBorrowAPR = 0.40
	s.cfg.SpotFuturesExchanges = []string{"binance", "bybit"}
	s.cfg.SpotFuturesScanIntervalMin = 10
	s.cfg.SpotFuturesBorrowGraceMin = 30
	s.cfg.SpotFuturesPriceExitPct = 20
	s.cfg.SpotFuturesPriceEmergencyPct = 30
	s.cfg.SpotFuturesMarginExitPct = 85
	s.cfg.SpotFuturesMarginEmergencyPct = 95
	s.cfg.SpotFuturesLossCooldownHours = 4
	s.cfg.SpotFuturesAutoEnabled = true
	s.cfg.SpotFuturesDryRun = true
	s.cfg.SpotFuturesPersistenceScans = 2
	s.cfg.SpotFuturesProfitTransferEnabled = true
	s.cfg.SpotFuturesSeparateAcctMaxUSDT = 200
	s.cfg.SpotFuturesUnifiedAcctMaxUSDT = 500

	configPath := filepath.Join(t.TempDir(), "config.json")
	initialConfig := `{
  "spot_futures": {
    "enabled": true,
    "max_positions": 2,
    "capital_per_position": 500
  }
}`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	req := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(`{"spot_futures":{"enabled":false}}`))
	w := httptest.NewRecorder()
	s.handlePostConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		OK   bool `json:"ok"`
		Data struct {
			SpotFutures *struct {
				Enabled            bool    `json:"enabled"`
				MaxPositions       int     `json:"max_positions"`
				CapitalPerPosition float64 `json:"capital_per_position"`
			} `json:"spot_futures"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected ok=true")
	}
	if resp.Data.SpotFutures == nil {
		t.Fatal("expected spot_futures in response when disabled")
	}
	if resp.Data.SpotFutures.Enabled {
		t.Fatal("expected response spot_futures.enabled=false")
	}
	if resp.Data.SpotFutures.MaxPositions != 2 {
		t.Fatalf("expected max_positions=2, got %d", resp.Data.SpotFutures.MaxPositions)
	}
	if resp.Data.SpotFutures.CapitalPerPosition != 500 {
		t.Fatalf("expected capital_per_position=500, got %v", resp.Data.SpotFutures.CapitalPerPosition)
	}

	enabled, err := s.db.GetConfigField("spot_futures_enabled")
	if err != nil {
		t.Fatalf("redis get spot_futures_enabled: %v", err)
	}
	if enabled != "false" {
		t.Fatalf("expected redis spot_futures_enabled=false, got %q", enabled)
	}

	reloaded := config.Load()
	if reloaded.SpotFuturesEnabled {
		t.Fatal("expected reloaded spot_futures.enabled=false")
	}
	if reloaded.SpotFuturesMaxPositions != 2 {
		t.Fatalf("expected reloaded max_positions=2, got %d", reloaded.SpotFuturesMaxPositions)
	}
	if reloaded.SpotFuturesCapitalPerPosition != 500 {
		t.Fatalf("expected reloaded capital_per_position=500, got %v", reloaded.SpotFuturesCapitalPerPosition)
	}
}

func TestHandleConfig_SpotFuturesAutoDryRunRoundTrip(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	s.cfg.SpotFuturesEnabled = true
	s.cfg.SpotFuturesDryRun = true

	getReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	getW := httptest.NewRecorder()
	s.handleGetConfig(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("GET expected 200, got %d: %s", getW.Code, getW.Body.String())
	}

	var getResp struct {
		OK   bool                   `json:"ok"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(getW.Body).Decode(&getResp); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	spotFutures, ok := getResp.Data["spot_futures"].(map[string]interface{})
	if !ok {
		t.Fatalf("spot_futures payload missing or wrong type: %#v", getResp.Data["spot_futures"])
	}
	if spotFutures["auto_dry_run"] != true {
		t.Fatalf("expected auto_dry_run=true, got %#v", spotFutures["auto_dry_run"])
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(`{"spot_futures":{"auto_dry_run":false}}`))
	postW := httptest.NewRecorder()
	s.handlePostConfig(postW, postReq)

	if postW.Code != http.StatusOK {
		t.Fatalf("POST expected 200, got %d: %s", postW.Code, postW.Body.String())
	}
	if s.cfg.SpotFuturesDryRun {
		t.Fatal("expected SpotFuturesDryRun=false after POST")
	}

	persisted, err := s.db.GetConfigField("spot_futures_dry_run")
	if err != nil {
		t.Fatalf("redis get spot_futures_dry_run: %v", err)
	}
	if persisted != "false" {
		t.Fatalf("expected redis spot_futures_dry_run=false, got %q", persisted)
	}
}

func TestHandleConfig_BorrowSpikeDetectionRoundTrip(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	s.cfg.SpotFuturesEnabled = true
	s.cfg.EnableBorrowSpikeDetection = false
	s.cfg.BorrowSpikeWindowMin = 60
	s.cfg.BorrowSpikeMultiplier = 2.0
	s.cfg.BorrowSpikeMinAbsolute = 0.10

	configPath := filepath.Join(t.TempDir(), "config.json")
	initialConfig := `{
  "spot_futures": {
    "enabled": true,
    "enable_borrow_spike_detection": false,
    "borrow_spike_window_min": 60,
    "borrow_spike_multiplier": 2.0,
    "borrow_spike_min_absolute": 0.10
  }
}`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	getReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	getW := httptest.NewRecorder()
	s.handleGetConfig(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GET expected 200, got %d: %s", getW.Code, getW.Body.String())
	}

	var getResp struct {
		OK   bool                   `json:"ok"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(getW.Body).Decode(&getResp); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	spotFutures, ok := getResp.Data["spot_futures"].(map[string]interface{})
	if !ok {
		t.Fatalf("spot_futures payload missing or wrong type: %#v", getResp.Data["spot_futures"])
	}
	if spotFutures["enable_borrow_spike_detection"] != false {
		t.Fatalf("expected enable_borrow_spike_detection=false, got %#v", spotFutures["enable_borrow_spike_detection"])
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(`{"spot_futures":{"enable_borrow_spike_detection":true,"borrow_spike_window_min":45,"borrow_spike_multiplier":1.8,"borrow_spike_min_absolute":0.06}}`))
	postW := httptest.NewRecorder()
	s.handlePostConfig(postW, postReq)
	if postW.Code != http.StatusOK {
		t.Fatalf("POST expected 200, got %d: %s", postW.Code, postW.Body.String())
	}

	if !s.cfg.EnableBorrowSpikeDetection {
		t.Fatal("expected EnableBorrowSpikeDetection=true after POST")
	}
	if s.cfg.BorrowSpikeWindowMin != 45 {
		t.Fatalf("expected BorrowSpikeWindowMin=45, got %d", s.cfg.BorrowSpikeWindowMin)
	}
	if s.cfg.BorrowSpikeMultiplier != 1.8 {
		t.Fatalf("expected BorrowSpikeMultiplier=1.8, got %v", s.cfg.BorrowSpikeMultiplier)
	}
	if s.cfg.BorrowSpikeMinAbsolute != 0.06 {
		t.Fatalf("expected BorrowSpikeMinAbsolute=0.06, got %v", s.cfg.BorrowSpikeMinAbsolute)
	}

	persistedToggle, err := s.db.GetConfigField("spot_futures_enable_borrow_spike_detection")
	if err != nil {
		t.Fatalf("redis get spot_futures_enable_borrow_spike_detection: %v", err)
	}
	if persistedToggle != "true" {
		t.Fatalf("expected redis spot_futures_enable_borrow_spike_detection=true, got %q", persistedToggle)
	}
	persistedWindow, err := s.db.GetConfigField("spot_futures_borrow_spike_window_min")
	if err != nil {
		t.Fatalf("redis get spot_futures_borrow_spike_window_min: %v", err)
	}
	if persistedWindow != "45" {
		t.Fatalf("expected redis spot_futures_borrow_spike_window_min=45, got %q", persistedWindow)
	}
	persistedMultiplier, err := s.db.GetConfigField("spot_futures_borrow_spike_multiplier")
	if err != nil {
		t.Fatalf("redis get spot_futures_borrow_spike_multiplier: %v", err)
	}
	if persistedMultiplier != "1.8" {
		t.Fatalf("expected redis spot_futures_borrow_spike_multiplier=1.8, got %q", persistedMultiplier)
	}
	persistedAbsolute, err := s.db.GetConfigField("spot_futures_borrow_spike_min_absolute")
	if err != nil {
		t.Fatalf("redis get spot_futures_borrow_spike_min_absolute: %v", err)
	}
	if persistedAbsolute != "0.06" {
		t.Fatalf("expected redis spot_futures_borrow_spike_min_absolute=0.06, got %q", persistedAbsolute)
	}

	reloaded := config.Load()
	if !reloaded.EnableBorrowSpikeDetection {
		t.Fatal("expected reloaded EnableBorrowSpikeDetection=true")
	}
	if reloaded.BorrowSpikeWindowMin != 45 {
		t.Fatalf("expected reloaded BorrowSpikeWindowMin=45, got %d", reloaded.BorrowSpikeWindowMin)
	}
	if reloaded.BorrowSpikeMultiplier != 1.8 {
		t.Fatalf("expected reloaded BorrowSpikeMultiplier=1.8, got %v", reloaded.BorrowSpikeMultiplier)
	}
	if reloaded.BorrowSpikeMinAbsolute != 0.06 {
		t.Fatalf("expected reloaded BorrowSpikeMinAbsolute=0.06, got %v", reloaded.BorrowSpikeMinAbsolute)
	}
}
