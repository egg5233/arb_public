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
