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
	"arb/internal/risk"
	"arb/pkg/exchange"
)

func TestHandlePostConfig_PersistsDisabledSpotFutures(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	s.cfg.SpotFuturesEnabled = true
	s.cfg.SpotFuturesMaxPositions = 2
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
	s.cfg.SpotFuturesCapitalSeparate = 200
	s.cfg.SpotFuturesCapitalUnified = 500

	s.cfg.SpotFuturesCapitalSeparate = 300

	configPath := filepath.Join(t.TempDir(), "config.json")
	initialConfig := `{
  "spot_futures": {
    "enabled": true,
    "max_positions": 2,
    "capital_separate_usdt": 300
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
				Enabled             bool    `json:"enabled"`
				MaxPositions        int     `json:"max_positions"`
				CapitalSeparateUSDT float64 `json:"capital_separate_usdt"`
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
	if resp.Data.SpotFutures.CapitalSeparateUSDT != 300 {
		t.Fatalf("expected capital_separate_usdt=300, got %v", resp.Data.SpotFutures.CapitalSeparateUSDT)
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
	if reloaded.SpotFuturesCapitalSeparate != 300 {
		t.Fatalf("expected reloaded capital_separate_usdt=300, got %v", reloaded.SpotFuturesCapitalSeparate)
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

func TestHandleConfig_SpotFuturesDiscoveryFieldsRoundTrip(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	s.cfg.SpotFuturesNativeScannerEnabled = true
	s.cfg.SpotFuturesEnableMinHold = false
	s.cfg.SpotFuturesMinHoldHours = 8
	s.cfg.SpotFuturesEnableSettlementGuard = false
	s.cfg.SpotFuturesSettlementWindowMin = 10
	s.cfg.SpotFuturesEnableBasisGate = false
	s.cfg.SpotFuturesMaxBasisPct = 0.5
	s.cfg.SpotFuturesEnableExitSpreadGate = false
	s.cfg.SpotFuturesExitSpreadPct = 0.3

	configPath := filepath.Join(t.TempDir(), "config.json")
	initialConfig := `{
  "spot_futures": {
    "native_scanner_enabled": true,
    "enable_min_hold": false,
    "min_hold_hours": 8,
    "enable_settlement_guard": false,
    "settlement_window_min": 10,
    "enable_basis_gate": false,
    "max_basis_pct": 0.5,
    "enable_exit_spread_gate": false,
    "exit_spread_pct": 0.3
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
	if spotFutures["native_scanner_enabled"] != true {
		t.Fatalf("expected native_scanner_enabled=true, got %#v", spotFutures["native_scanner_enabled"])
	}
	if spotFutures["enable_basis_gate"] != false {
		t.Fatalf("expected enable_basis_gate=false, got %#v", spotFutures["enable_basis_gate"])
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(`{"spot_futures":{"native_scanner_enabled":false,"enable_min_hold":true,"min_hold_hours":12,"enable_settlement_guard":true,"settlement_window_min":15,"enable_basis_gate":true,"max_basis_pct":0.9,"enable_exit_spread_gate":true,"exit_spread_pct":0.45}}`))
	postW := httptest.NewRecorder()
	s.handlePostConfig(postW, postReq)
	if postW.Code != http.StatusOK {
		t.Fatalf("POST expected 200, got %d: %s", postW.Code, postW.Body.String())
	}

	if s.cfg.SpotFuturesNativeScannerEnabled {
		t.Fatal("expected SpotFuturesNativeScannerEnabled=false after POST")
	}
	if !s.cfg.SpotFuturesEnableMinHold {
		t.Fatal("expected SpotFuturesEnableMinHold=true after POST")
	}
	if s.cfg.SpotFuturesMinHoldHours != 12 {
		t.Fatalf("expected SpotFuturesMinHoldHours=12, got %d", s.cfg.SpotFuturesMinHoldHours)
	}
	if !s.cfg.SpotFuturesEnableSettlementGuard {
		t.Fatal("expected SpotFuturesEnableSettlementGuard=true after POST")
	}
	if s.cfg.SpotFuturesSettlementWindowMin != 15 {
		t.Fatalf("expected SpotFuturesSettlementWindowMin=15, got %d", s.cfg.SpotFuturesSettlementWindowMin)
	}
	if !s.cfg.SpotFuturesEnableBasisGate {
		t.Fatal("expected SpotFuturesEnableBasisGate=true after POST")
	}
	if s.cfg.SpotFuturesMaxBasisPct != 0.9 {
		t.Fatalf("expected SpotFuturesMaxBasisPct=0.9, got %v", s.cfg.SpotFuturesMaxBasisPct)
	}
	if !s.cfg.SpotFuturesEnableExitSpreadGate {
		t.Fatal("expected SpotFuturesEnableExitSpreadGate=true after POST")
	}
	if s.cfg.SpotFuturesExitSpreadPct != 0.45 {
		t.Fatalf("expected SpotFuturesExitSpreadPct=0.45, got %v", s.cfg.SpotFuturesExitSpreadPct)
	}

	persisted, err := s.db.GetConfigField("spot_futures_native_scanner_enabled")
	if err != nil {
		t.Fatalf("redis get spot_futures_native_scanner_enabled: %v", err)
	}
	if persisted != "false" {
		t.Fatalf("expected redis spot_futures_native_scanner_enabled=false, got %q", persisted)
	}
	persisted, err = s.db.GetConfigField("spot_futures_enable_basis_gate")
	if err != nil {
		t.Fatalf("redis get spot_futures_enable_basis_gate: %v", err)
	}
	if persisted != "true" {
		t.Fatalf("expected redis spot_futures_enable_basis_gate=true, got %q", persisted)
	}
	persisted, err = s.db.GetConfigField("spot_futures_exit_spread_pct")
	if err != nil {
		t.Fatalf("redis get spot_futures_exit_spread_pct: %v", err)
	}
	if persisted != "0.45" {
		t.Fatalf("expected redis spot_futures_exit_spread_pct=0.45, got %q", persisted)
	}

	reloaded := config.Load()
	if reloaded.SpotFuturesNativeScannerEnabled {
		t.Fatal("expected reloaded SpotFuturesNativeScannerEnabled=false")
	}
	if !reloaded.SpotFuturesEnableMinHold {
		t.Fatal("expected reloaded SpotFuturesEnableMinHold=true")
	}
	if reloaded.SpotFuturesMinHoldHours != 12 {
		t.Fatalf("expected reloaded SpotFuturesMinHoldHours=12, got %d", reloaded.SpotFuturesMinHoldHours)
	}
	if !reloaded.SpotFuturesEnableSettlementGuard {
		t.Fatal("expected reloaded SpotFuturesEnableSettlementGuard=true")
	}
	if reloaded.SpotFuturesSettlementWindowMin != 15 {
		t.Fatalf("expected reloaded SpotFuturesSettlementWindowMin=15, got %d", reloaded.SpotFuturesSettlementWindowMin)
	}
	if !reloaded.SpotFuturesEnableBasisGate {
		t.Fatal("expected reloaded SpotFuturesEnableBasisGate=true")
	}
	if reloaded.SpotFuturesMaxBasisPct != 0.9 {
		t.Fatalf("expected reloaded SpotFuturesMaxBasisPct=0.9, got %v", reloaded.SpotFuturesMaxBasisPct)
	}
	if !reloaded.SpotFuturesEnableExitSpreadGate {
		t.Fatal("expected reloaded SpotFuturesEnableExitSpreadGate=true")
	}
	if reloaded.SpotFuturesExitSpreadPct != 0.45 {
		t.Fatalf("expected reloaded SpotFuturesExitSpreadPct=0.45, got %v", reloaded.SpotFuturesExitSpreadPct)
	}
}

func TestHandleConfig_ExchangeHealthScoringRoundTrip(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	s.cfg.EnableExchangeHealthScoring = false
	s.cfg.ExchHealthLatencyMs = 2000
	s.cfg.ExchHealthMinUptime = 0.95
	s.cfg.ExchHealthMinFillRate = 0.80
	s.cfg.ExchHealthMinScore = 0.50
	s.cfg.ExchHealthWindowMin = 60

	configPath := filepath.Join(t.TempDir(), "config.json")
	initialConfig := `{
  "risk": {
    "enable_exchange_health_scoring": false,
    "exch_health_latency_ms": 2000,
    "exch_health_min_uptime": 0.95,
    "exch_health_min_fill_rate": 0.80,
    "exch_health_min_score": 0.50,
    "exch_health_window_min": 60
  }
}`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	getReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	getW := httptest.NewRecorder()
	s.handleGetConfig(getW, getReq)

	var getResp struct {
		OK   bool                   `json:"ok"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(getW.Body).Decode(&getResp); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	riskData, ok := getResp.Data["risk"].(map[string]interface{})
	if !ok {
		t.Fatalf("risk payload missing or wrong type: %#v", getResp.Data["risk"])
	}
	if riskData["enable_exchange_health_scoring"] != false {
		t.Fatalf("expected enable_exchange_health_scoring=false, got %#v", riskData["enable_exchange_health_scoring"])
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(`{"risk":{"enable_exchange_health_scoring":true,"exch_health_latency_ms":1500,"exch_health_min_uptime":0.9,"exch_health_min_fill_rate":0.75,"exch_health_min_score":0.6,"exch_health_window_min":30}}`))
	postW := httptest.NewRecorder()
	s.handlePostConfig(postW, postReq)
	if postW.Code != http.StatusOK {
		t.Fatalf("POST expected 200, got %d: %s", postW.Code, postW.Body.String())
	}

	if !s.cfg.EnableExchangeHealthScoring || s.cfg.ExchHealthLatencyMs != 1500 || s.cfg.ExchHealthWindowMin != 30 {
		t.Fatalf("config not updated correctly: %+v", s.cfg)
	}

	persisted, err := s.db.GetConfigField("enable_exchange_health_scoring")
	if err != nil {
		t.Fatalf("redis get enable_exchange_health_scoring: %v", err)
	}
	if persisted != "true" {
		t.Fatalf("expected redis enable_exchange_health_scoring=true, got %q", persisted)
	}

	reloaded := config.Load()
	if !reloaded.EnableExchangeHealthScoring || reloaded.ExchHealthLatencyMs != 1500 || reloaded.ExchHealthWindowMin != 30 {
		t.Fatalf("reloaded config mismatch: %+v", reloaded)
	}
}

func TestHandleGetExchangeHealth_ColdStart(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	s.exchanges = map[string]exchange.Exchange{
		"binance": nil,
		"bybit":   nil,
	}
	s.SetExchangeScorer(risk.NewExchangeScorer(s.cfg))

	req := httptest.NewRequest(http.MethodGet, "/api/exchanges/health", nil)
	w := httptest.NewRecorder()
	s.handleGetExchangeHealth(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		OK   bool                              `json:"ok"`
		Data map[string]map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("expected exchange health data")
	}
	for name, snapshot := range resp.Data {
		if snapshot["exchange"] != name {
			t.Fatalf("exchange field mismatch for %s: %#v", name, snapshot["exchange"])
		}
		if snapshot["score"] != float64(1) {
			t.Fatalf("expected cold-start score=1 for %s, got %#v", name, snapshot["score"])
		}
	}
}

func TestHandleConfig_LiqTrendTrackingRoundTrip(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	s.cfg.EnableLiqTrendTracking = false
	s.cfg.LiqProjectionMinutes = 15
	s.cfg.LiqWarningSlopeThresh = 0.002
	s.cfg.LiqCriticalSlopeThresh = 0.004
	s.cfg.LiqMinSamples = 5

	configPath := filepath.Join(t.TempDir(), "config.json")
	initialConfig := `{
  "risk": {
    "enable_liq_trend_tracking": false,
    "liq_projection_minutes": 15,
    "liq_warning_slope_thresh": 0.002,
    "liq_critical_slope_thresh": 0.004,
    "liq_min_samples": 5
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
	riskData, ok := getResp.Data["risk"].(map[string]interface{})
	if !ok {
		t.Fatalf("risk payload missing or wrong type: %#v", getResp.Data["risk"])
	}
	if riskData["enable_liq_trend_tracking"] != false {
		t.Fatalf("expected enable_liq_trend_tracking=false, got %#v", riskData["enable_liq_trend_tracking"])
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(`{"risk":{"enable_liq_trend_tracking":true,"liq_projection_minutes":12,"liq_warning_slope_thresh":0.003,"liq_critical_slope_thresh":0.007,"liq_min_samples":6}}`))
	postW := httptest.NewRecorder()
	s.handlePostConfig(postW, postReq)
	if postW.Code != http.StatusOK {
		t.Fatalf("POST expected 200, got %d: %s", postW.Code, postW.Body.String())
	}

	if !s.cfg.EnableLiqTrendTracking {
		t.Fatal("expected EnableLiqTrendTracking=true after POST")
	}
	if s.cfg.LiqProjectionMinutes != 12 {
		t.Fatalf("expected LiqProjectionMinutes=12, got %d", s.cfg.LiqProjectionMinutes)
	}
	if s.cfg.LiqWarningSlopeThresh != 0.003 {
		t.Fatalf("expected LiqWarningSlopeThresh=0.003, got %v", s.cfg.LiqWarningSlopeThresh)
	}
	if s.cfg.LiqCriticalSlopeThresh != 0.007 {
		t.Fatalf("expected LiqCriticalSlopeThresh=0.007, got %v", s.cfg.LiqCriticalSlopeThresh)
	}
	if s.cfg.LiqMinSamples != 6 {
		t.Fatalf("expected LiqMinSamples=6, got %d", s.cfg.LiqMinSamples)
	}

	for field, want := range map[string]string{
		"enable_liq_trend_tracking": "true",
		"liq_projection_minutes":    "12",
		"liq_warning_slope_thresh":  "0.003",
		"liq_critical_slope_thresh": "0.007",
		"liq_min_samples":           "6",
	} {
		got, err := s.db.GetConfigField(field)
		if err != nil {
			t.Fatalf("redis get %s: %v", field, err)
		}
		if got != want {
			t.Fatalf("expected redis %s=%s, got %q", field, want, got)
		}
	}

	reloaded := config.Load()
	if !reloaded.EnableLiqTrendTracking {
		t.Fatal("expected reloaded EnableLiqTrendTracking=true")
	}
	if reloaded.LiqProjectionMinutes != 12 {
		t.Fatalf("expected reloaded LiqProjectionMinutes=12, got %d", reloaded.LiqProjectionMinutes)
	}
	if reloaded.LiqWarningSlopeThresh != 0.003 {
		t.Fatalf("expected reloaded LiqWarningSlopeThresh=0.003, got %v", reloaded.LiqWarningSlopeThresh)
	}
	if reloaded.LiqCriticalSlopeThresh != 0.007 {
		t.Fatalf("expected reloaded LiqCriticalSlopeThresh=0.007, got %v", reloaded.LiqCriticalSlopeThresh)
	}
	if reloaded.LiqMinSamples != 6 {
		t.Fatalf("expected reloaded LiqMinSamples=6, got %d", reloaded.LiqMinSamples)
	}
}

func TestHandlePostConfig_IgnoresEmptyExchangeCredentialUpdates(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	s.cfg.BinanceAPIKey = "runtime-key"
	s.cfg.BinanceSecretKey = "runtime-secret"

	configPath := filepath.Join(t.TempDir(), "config.json")
	initialConfig := `{
  "exchanges": {
    "binance": {
      "api_key": "file-key",
      "secret_key": "file-secret",
      "enabled": true
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	req := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(`{"exchanges":{"binance":{"api_key":"","secret_key":""}}}`))
	w := httptest.NewRecorder()
	s.handlePostConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if s.cfg.BinanceAPIKey != "runtime-key" {
		t.Fatalf("expected in-memory api key to remain unchanged, got %q", s.cfg.BinanceAPIKey)
	}
	if s.cfg.BinanceSecretKey != "runtime-secret" {
		t.Fatalf("expected in-memory secret key to remain unchanged, got %q", s.cfg.BinanceSecretKey)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var saved map[string]interface{}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	exchanges, ok := saved["exchanges"].(map[string]interface{})
	if !ok {
		t.Fatalf("exchanges missing or wrong type: %#v", saved["exchanges"])
	}
	binance, ok := exchanges["binance"].(map[string]interface{})
	if !ok {
		t.Fatalf("binance missing or wrong type: %#v", exchanges["binance"])
	}
	if got := binance["api_key"]; got != "file-key" {
		t.Fatalf("expected config api_key to preserve file value, got %#v", got)
	}
	if got := binance["secret_key"]; got != "file-secret" {
		t.Fatalf("expected config secret_key to preserve file value, got %#v", got)
	}

	if _, err := os.Stat(configPath + ".bak"); err != nil {
		t.Fatalf("expected backup config to be created: %v", err)
	}
}

func TestHandlePostConfig_PersistsExplicitExchangeCredentialUpdates(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	s.cfg.BinanceAPIKey = "runtime-old-key"
	s.cfg.BinanceSecretKey = "runtime-old-secret"

	configPath := filepath.Join(t.TempDir(), "config.json")
	initialConfig := `{
  "exchanges": {
    "binance": {
      "api_key": "file-old-key",
      "secret_key": "file-old-secret",
      "enabled": true
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	req := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(`{"exchanges":{"binance":{"api_key":"new-key","secret_key":"new-secret"}}}`))
	w := httptest.NewRecorder()
	s.handlePostConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if s.cfg.BinanceAPIKey != "new-key" {
		t.Fatalf("expected in-memory api key update, got %q", s.cfg.BinanceAPIKey)
	}
	if s.cfg.BinanceSecretKey != "new-secret" {
		t.Fatalf("expected in-memory secret key update, got %q", s.cfg.BinanceSecretKey)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var saved map[string]interface{}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	exchanges, ok := saved["exchanges"].(map[string]interface{})
	if !ok {
		t.Fatalf("exchanges missing or wrong type: %#v", saved["exchanges"])
	}
	binance, ok := exchanges["binance"].(map[string]interface{})
	if !ok {
		t.Fatalf("binance missing or wrong type: %#v", exchanges["binance"])
	}
	if got := binance["api_key"]; got != "new-key" {
		t.Fatalf("expected config api_key update, got %#v", got)
	}
	if got := binance["secret_key"]; got != "new-secret" {
		t.Fatalf("expected config secret_key update, got %#v", got)
	}
}
