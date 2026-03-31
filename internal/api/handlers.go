package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// processStartTime records when the process started, used to detect binary drift.
var processStartTime = time.Now()

// Response is the standard JSON response wrapper.
type Response struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// writeJSON serialises v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// handleGetPositions returns all active positions.
func (s *Server) handleGetPositions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	positions, err := s.db.GetActivePositions()
	if err != nil {
		s.log.Error("get active positions: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{Error: "failed to fetch positions"})
		return
	}

	if positions == nil {
		writeJSON(w, http.StatusOK, Response{OK: true, Data: []interface{}{}})
		return
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: positions})
}

// handleClosePosition triggers a manual close for an active position.
func (s *Server) handleClosePosition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		PositionID string `json:"position_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PositionID == "" {
		writeJSON(w, http.StatusBadRequest, Response{Error: "position_id required"})
		return
	}
	if s.closePosition == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Error: "close handler not available"})
		return
	}
	if err := s.closePosition(req.PositionID); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			writeJSON(w, http.StatusNotFound, Response{Error: errMsg})
		} else if strings.Contains(errMsg, "not active") || strings.Contains(errMsg, "already") {
			writeJSON(w, http.StatusConflict, Response{Error: errMsg})
		} else {
			writeJSON(w, http.StatusInternalServerError, Response{Error: errMsg})
		}
		return
	}
	writeJSON(w, http.StatusAccepted, Response{OK: true})
}

// handleOpenPosition triggers a manual open for an opportunity.
func (s *Server) handleOpenPosition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Symbol        string `json:"symbol"`
		LongExchange  string `json:"long_exchange"`
		ShortExchange string `json:"short_exchange"`
		Force         bool   `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Symbol == "" || req.LongExchange == "" || req.ShortExchange == "" {
		writeJSON(w, http.StatusBadRequest, Response{Error: "symbol, long_exchange, short_exchange required"})
		return
	}
	if s.openPosition == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Error: "open handler not available"})
		return
	}
	if err := s.openPosition(req.Symbol, req.LongExchange, req.ShortExchange, req.Force); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			writeJSON(w, http.StatusNotFound, Response{Error: errMsg})
		} else if strings.Contains(errMsg, "rejected") || strings.Contains(errMsg, "risk check failed") || strings.Contains(errMsg, "dry run") {
			writeJSON(w, http.StatusUnprocessableEntity, Response{Error: errMsg})
		} else if strings.Contains(errMsg, "already") || strings.Contains(errMsg, "capacity") {
			writeJSON(w, http.StatusConflict, Response{Error: errMsg})
		} else {
			writeJSON(w, http.StatusInternalServerError, Response{Error: errMsg})
		}
		return
	}
	writeJSON(w, http.StatusAccepted, Response{OK: true})
}

// handleGetHistory returns completed trades.
func (s *Server) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 50
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			limit = n
		}
	}

	history, err := s.db.GetHistory(limit)
	if err != nil {
		s.log.Error("get history: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{Error: "failed to fetch history"})
		return
	}

	if history == nil {
		writeJSON(w, http.StatusOK, Response{OK: true, Data: []interface{}{}})
		return
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: history})
}

// handleGetPositionFunding returns per-payment funding history for a position.
func (s *Server) handleGetPositionFunding(w http.ResponseWriter, r *http.Request) {
	posID := r.PathValue("id")
	if posID == "" {
		writeJSON(w, http.StatusBadRequest, Response{Error: "position id required"})
		return
	}

	pos, err := s.db.GetPosition(posID)
	if err != nil || pos == nil {
		writeJSON(w, http.StatusNotFound, Response{Error: "position not found"})
		return
	}

	type fundingEvent struct {
		Exchange string    `json:"exchange"`
		Side     string    `json:"side"`
		Amount   float64   `json:"amount"`
		Time     time.Time `json:"time"`
	}

	var events []fundingEvent

	// Build time-bounded leg windows so we only include funding payments
	// that occurred while THIS position held each leg. Without this,
	// account-wide GetFundingFees can include payments from other positions
	// on the same symbol/exchange.
	type legWindow struct {
		name  string
		side  string
		start time.Time
		end   time.Time // zero = still active
	}

	var legs []legWindow

	// Current legs: active from creation (or last rotation) until now.
	longStart, shortStart := pos.CreatedAt, pos.CreatedAt
	// Walk rotation history chronologically to find when current legs started.
	for _, rot := range pos.RotationHistory {
		if rot.LegSide == "long" {
			longStart = rot.Timestamp
		} else {
			shortStart = rot.Timestamp
		}
	}
	legs = append(legs, legWindow{pos.LongExchange, "long", longStart, time.Time{}})
	legs = append(legs, legWindow{pos.ShortExchange, "short", shortStart, time.Time{}})

	// Rotated-away legs: active from their start until the rotation timestamp.
	// Walk rotations to reconstruct each previous leg's window.
	prevLongExch, prevShortExch := pos.LongExchange, pos.ShortExchange
	prevLongStart, prevShortStart := pos.CreatedAt, pos.CreatedAt
	for _, rot := range pos.RotationHistory {
		if rot.LegSide == "long" {
			legs = append(legs, legWindow{prevLongExch, "long", prevLongStart, rot.Timestamp})
			prevLongExch = rot.To
			prevLongStart = rot.Timestamp
		} else {
			legs = append(legs, legWindow{prevShortExch, "short", prevShortStart, rot.Timestamp})
			prevShortExch = rot.To
			prevShortStart = rot.Timestamp
		}
	}

	// Deduplicate: if a current leg was never rotated, it appears twice.
	// Keep the widest window (start=CreatedAt, end=zero).
	seen := make(map[string]bool)
	for _, leg := range legs {
		key := leg.name + ":" + leg.side
		if seen[key] {
			continue
		}
		seen[key] = true

		exch, ok := s.exchanges[leg.name]
		if !ok {
			continue
		}
		fees, err := exch.GetFundingFees(pos.Symbol, leg.start)
		if err != nil {
			continue
		}
		for _, f := range fees {
			// Filter by end time if this was a rotated-away leg.
			if !leg.end.IsZero() && f.Time.After(leg.end) {
				continue
			}
			events = append(events, fundingEvent{
				Exchange: leg.name,
				Side:     leg.side,
				Amount:   f.Amount,
				Time:     f.Time,
			})
		}
	}

	// Sort by time ascending (oldest first), then by side for stable ordering.
	sort.Slice(events, func(i, j int) bool {
		ti := events[i].Time.Truncate(time.Hour)
		tj := events[j].Time.Truncate(time.Hour)
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return events[i].Side < events[j].Side
	})

	if events == nil {
		writeJSON(w, http.StatusOK, Response{OK: true, Data: []interface{}{}})
		return
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: events})
}

// handleGetOpportunities returns the cached opportunities slice.
func (s *Server) handleGetOpportunities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opps := s.opps
	if opps == nil {
		writeJSON(w, http.StatusOK, Response{OK: true, Data: []interface{}{}})
		return
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: opps})
}

// handleGetStats returns PnL, win rate, and trade count.
func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := s.db.GetStats()
	if err != nil {
		s.log.Error("get stats: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{Error: "failed to fetch stats"})
		return
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: stats})
}

// ---------- Nested config API response structs ----------

type configResponse struct {
	DryRun      bool                              `json:"dry_run"`
	Strategy    configStrategyResponse            `json:"strategy"`
	Fund        configFundResponse                `json:"fund"`
	Risk        configRiskResponse                `json:"risk"`
	AI          configAIResponse                  `json:"ai"`
	Exchanges   map[string]configExchangeResponse `json:"exchanges"`
	SpotFutures *configSpotFuturesResponse        `json:"spot_futures,omitempty"`
}

type configSpotFuturesResponse struct {
	Enabled                    bool     `json:"enabled"`
	MaxPositions               int      `json:"max_positions"`
	CapitalPerPosition         float64  `json:"capital_per_position"`
	Leverage                   int      `json:"leverage"`
	MonitorIntervalSec         int      `json:"monitor_interval_sec"`
	MinNetYieldAPR             float64  `json:"min_net_yield_apr"`
	MaxBorrowAPR               float64  `json:"max_borrow_apr"`
	EnableBorrowSpikeDetection bool     `json:"enable_borrow_spike_detection"`
	BorrowSpikeWindowMin       int      `json:"borrow_spike_window_min"`
	BorrowSpikeMultiplier      float64  `json:"borrow_spike_multiplier"`
	BorrowSpikeMinAbsolute     float64  `json:"borrow_spike_min_absolute"`
	Exchanges                  []string `json:"exchanges"`
	ScanIntervalMin            int      `json:"scan_interval_min"`
	BorrowGraceMin             int      `json:"borrow_grace_min"`
	PriceExitPct               float64  `json:"price_exit_pct"`
	PriceEmergencyPct          float64  `json:"price_emergency_pct"`
	MarginExitPct              float64  `json:"margin_exit_pct"`
	MarginEmergencyPct         float64  `json:"margin_emergency_pct"`
	LossCooldownHours          int      `json:"loss_cooldown_hours"`
	AutoEnabled                bool     `json:"auto_enabled"`
	DryRun                     bool     `json:"auto_dry_run"`
	PersistenceScans           int      `json:"persistence_scans"`
	ProfitTransferEnabled      bool     `json:"profit_transfer_enabled"`
	SeparateAcctMaxUSDT        float64  `json:"separate_acct_max_usdt"`
	UnifiedAcctMaxUSDT         float64  `json:"unified_acct_max_usdt"`
}

type configExchangeResponse struct {
	Enabled       bool              `json:"enabled"`
	HasAPIKey     bool              `json:"has_api_key"`
	APIKeyPreview string            `json:"api_key_preview"`
	HasSecretKey  bool              `json:"has_secret_key"`
	HasPassphrase *bool             `json:"has_passphrase,omitempty"`
	Address       map[string]string `json:"address"`
}

type configAIResponse struct {
	Endpoint  string `json:"endpoint"`
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
	HasKey    bool   `json:"has_key"`
}

type configStrategyResponse struct {
	TopOpportunities    int                     `json:"top_opportunities"`
	ScanMinutes         []int                   `json:"scan_minutes"`
	EntryScanMinute     int                     `json:"entry_scan_minute"`
	ExitScanMinute      int                     `json:"exit_scan_minute"`
	RotateScanMinute    int                     `json:"rotate_scan_minute"`
	RebalanceScanMinute int                     `json:"rebalance_scan_minute"`
	RebalanceAfterExit  bool                    `json:"rebalance_after_exit"`
	Discovery           configDiscoveryResponse `json:"discovery"`
	Entry               configEntryResponse     `json:"entry"`
	Exit                configExitResponse      `json:"exit"`
	Rotation            configRotationResponse  `json:"rotation"`
}

type configDiscoveryResponse struct {
	MinHoldTimeHours        int                       `json:"min_hold_time_hours"`
	MaxCostRatio            float64                   `json:"max_cost_ratio"`
	MaxPriceGapBPS          float64                   `json:"max_price_gap_bps"`
	PriceGapFreeBPS         float64                   `json:"price_gap_free_bps"`
	MaxGapRecoveryIntervals float64                   `json:"max_gap_recovery_intervals"`
	MaxIntervalHours        float64                   `json:"max_interval_hours"`
	DelistFilter            bool                      `json:"delist_filter"`
	Persistence             configPersistenceResponse `json:"persistence"`
}

type configPersistenceResponse struct {
	LookbackMin1h int `json:"lookback_min_1h"`
	MinCount1h    int `json:"min_count_1h"`
	LookbackMin4h int `json:"lookback_min_4h"`
	MinCount4h    int `json:"min_count_4h"`
	LookbackMin8h int `json:"lookback_min_8h"`
	MinCount8h    int `json:"min_count_8h"`

	SpreadStabilityRatio1h  float64 `json:"spread_stability_ratio_1h"`
	SpreadStabilityOIRank1h int     `json:"spread_stability_oi_rank_1h"`
	SpreadStabilityRatio4h  float64 `json:"spread_stability_ratio_4h"`
	SpreadStabilityOIRank4h int     `json:"spread_stability_oi_rank_4h"`
	SpreadStabilityRatio8h  float64 `json:"spread_stability_ratio_8h"`
	SpreadStabilityOIRank8h int     `json:"spread_stability_oi_rank_8h"`

	EnableSpreadStabilityGate       bool    `json:"enable_spread_stability_gate"`
	SpreadVolatilityMaxCV           float64 `json:"spread_volatility_max_cv"`
	SpreadVolatilityMinSamples      int     `json:"spread_volatility_min_samples"`
	SpreadStabilityStricterForAuto  bool    `json:"spread_stability_stricter_for_auto"`
	SpreadStabilityAutoCVMultiplier float64 `json:"spread_stability_auto_cv_multiplier"`
	FundingWindowMin                int     `json:"funding_window_min"`
}

type configEntryResponse struct {
	SlippageLimitBPS     float64 `json:"slippage_limit_bps"`
	MinChunkUSDT         float64 `json:"min_chunk_usdt"`
	EntryTimeoutSec      int     `json:"entry_timeout_sec"`
	LossCooldownHours    float64 `json:"loss_cooldown_hours"`
	ReEnterCooldownHours float64 `json:"re_enter_cooldown_hours"`
	BacktestDays         int     `json:"backtest_days"`
	BacktestMinProfit    float64 `json:"backtest_min_profit"`
}

type configExitResponse struct {
	DepthTimeoutSec         int  `json:"depth_timeout_sec"`
	EnableSpreadReversal    bool `json:"enable_spread_reversal"`
	SpreadReversalTolerance int  `json:"spread_reversal_tolerance"`
	ReversalResetOnRecover  bool `json:"reversal_reset_on_recover"`
	ZeroSpreadTolerance     int  `json:"zero_spread_tolerance"`
}

type configRotationResponse struct {
	ThresholdBPS float64 `json:"threshold_bps"`
	CooldownMin  int     `json:"cooldown_min"`
}

type configFundResponse struct {
	MaxPositions  int     `json:"max_positions"`
	Leverage      int     `json:"leverage"`
	CapitalPerLeg float64 `json:"capital_per_leg"`
}

type configRiskResponse struct {
	MarginL3Threshold           float64 `json:"margin_l3_threshold"`
	MarginL4Threshold           float64 `json:"margin_l4_threshold"`
	MarginL5Threshold           float64 `json:"margin_l5_threshold"`
	L4ReduceFraction            float64 `json:"l4_reduce_fraction"`
	MarginSafetyMultiplier      float64 `json:"margin_safety_multiplier"`
	RiskMonitorIntervalSec      int     `json:"risk_monitor_interval_sec"`
	EnableLiqTrendTracking      bool    `json:"enable_liq_trend_tracking"`
	LiqProjectionMinutes        int     `json:"liq_projection_minutes"`
	LiqWarningSlopeThresh       float64 `json:"liq_warning_slope_thresh"`
	LiqCriticalSlopeThresh      float64 `json:"liq_critical_slope_thresh"`
	LiqMinSamples               int     `json:"liq_min_samples"`
	EnableCapitalAllocator      bool    `json:"enable_capital_allocator"`
	MaxTotalExposureUSDT        float64 `json:"max_total_exposure_usdt"`
	MaxPerpPerpPct              float64 `json:"max_perp_perp_pct"`
	MaxSpotFuturesPct           float64 `json:"max_spot_futures_pct"`
	MaxPerExchangePct           float64 `json:"max_per_exchange_pct"`
	ReservationTTLSec           int     `json:"reservation_ttl_sec"`
	EnableExchangeHealthScoring bool    `json:"enable_exchange_health_scoring"`
	ExchHealthLatencyMs         int     `json:"exch_health_latency_ms"`
	ExchHealthMinUptime         float64 `json:"exch_health_min_uptime"`
	ExchHealthMinFillRate       float64 `json:"exch_health_min_fill_rate"`
	ExchHealthMinScore          float64 `json:"exch_health_min_score"`
	ExchHealthWindowMin         int     `json:"exch_health_window_min"`
}

// apiKeyPreview returns the first 6 characters of a key followed by "...", or empty if blank.
func apiKeyPreview(key string) string {
	if len(key) <= 6 {
		return key
	}
	return key[:6] + "..."
}

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool { return &b }

// buildExchangeResponse builds the exchange config info for a single exchange.
func buildExchangeInfo(apiKey, secretKey, passphrase string, hasPassphraseField bool, address map[string]string, enabled bool) configExchangeResponse {
	resp := configExchangeResponse{
		Enabled:       enabled,
		HasAPIKey:     apiKey != "",
		APIKeyPreview: apiKeyPreview(apiKey),
		HasSecretKey:  secretKey != "",
		Address:       address,
	}
	if hasPassphraseField {
		resp.HasPassphrase = boolPtr(passphrase != "")
	}
	if resp.Address == nil {
		resp.Address = make(map[string]string)
	}
	return resp
}

func (s *Server) buildConfigResponse() configResponse {
	resp := configResponse{
		DryRun: s.cfg.DryRun,
		Strategy: configStrategyResponse{
			TopOpportunities:    s.cfg.TopOpportunities,
			ScanMinutes:         s.cfg.ScanMinutes,
			EntryScanMinute:     s.cfg.EntryScanMinute,
			ExitScanMinute:      s.cfg.ExitScanMinute,
			RotateScanMinute:    s.cfg.RotateScanMinute,
			RebalanceScanMinute: s.cfg.RebalanceScanMinute,
			RebalanceAfterExit:  s.cfg.RebalanceAfterExit,
			Discovery: configDiscoveryResponse{
				MinHoldTimeHours:        int(s.cfg.MinHoldTime.Hours()),
				MaxCostRatio:            s.cfg.MaxCostRatio,
				MaxPriceGapBPS:          s.cfg.MaxPriceGapBPS,
				PriceGapFreeBPS:         s.cfg.PriceGapFreeBPS,
				MaxGapRecoveryIntervals: s.cfg.MaxGapRecoveryIntervals,
				MaxIntervalHours:        s.cfg.MaxIntervalHours,
				DelistFilter:            s.cfg.DelistFilterEnabled,
				Persistence: configPersistenceResponse{
					LookbackMin1h: int(s.cfg.PersistLookback1h.Minutes()),
					MinCount1h:    s.cfg.PersistMinCount1h,
					LookbackMin4h: int(s.cfg.PersistLookback4h.Minutes()),
					MinCount4h:    s.cfg.PersistMinCount4h,
					LookbackMin8h: int(s.cfg.PersistLookback8h.Minutes()),
					MinCount8h:    s.cfg.PersistMinCount8h,

					SpreadStabilityRatio1h:  s.cfg.SpreadStabilityRatio1h,
					SpreadStabilityOIRank1h: s.cfg.SpreadStabilityOIRank1h,
					SpreadStabilityRatio4h:  s.cfg.SpreadStabilityRatio4h,
					SpreadStabilityOIRank4h: s.cfg.SpreadStabilityOIRank4h,
					SpreadStabilityRatio8h:  s.cfg.SpreadStabilityRatio8h,
					SpreadStabilityOIRank8h: s.cfg.SpreadStabilityOIRank8h,

					EnableSpreadStabilityGate:       s.cfg.EnableSpreadStabilityGate,
					SpreadVolatilityMaxCV:           s.cfg.SpreadVolatilityMaxCV,
					SpreadVolatilityMinSamples:      s.cfg.SpreadVolatilityMinSamples,
					SpreadStabilityStricterForAuto:  s.cfg.SpreadStabilityStricterForAuto,
					SpreadStabilityAutoCVMultiplier: s.cfg.SpreadStabilityAutoCVMultiplier,
					FundingWindowMin:                s.cfg.FundingWindowMin,
				},
			},
			Entry: configEntryResponse{
				SlippageLimitBPS:     s.cfg.SlippageBPS,
				MinChunkUSDT:         s.cfg.MinChunkUSDT,
				EntryTimeoutSec:      s.cfg.EntryTimeoutSec,
				LossCooldownHours:    s.cfg.LossCooldownHours,
				ReEnterCooldownHours: s.cfg.ReEnterCooldownHours,
				BacktestDays:         s.cfg.BacktestDays,
				BacktestMinProfit:    s.cfg.BacktestMinProfit,
			},
			Exit: configExitResponse{
				DepthTimeoutSec:         s.cfg.ExitDepthTimeoutSec,
				EnableSpreadReversal:    s.cfg.EnableSpreadReversal,
				SpreadReversalTolerance: s.cfg.SpreadReversalTolerance,
				ReversalResetOnRecover:  s.cfg.ReversalResetOnRecover,
				ZeroSpreadTolerance:     s.cfg.ZeroSpreadTolerance,
			},
			Rotation: configRotationResponse{
				ThresholdBPS: s.cfg.RotationThresholdBPS,
				CooldownMin:  s.cfg.RotationCooldownMin,
			},
		},
		Fund: configFundResponse{
			MaxPositions:  s.cfg.MaxPositions,
			Leverage:      s.cfg.Leverage,
			CapitalPerLeg: s.cfg.CapitalPerLeg,
		},
		Risk: configRiskResponse{
			MarginL3Threshold:           s.cfg.MarginL3Threshold,
			MarginL4Threshold:           s.cfg.MarginL4Threshold,
			MarginL5Threshold:           s.cfg.MarginL5Threshold,
			L4ReduceFraction:            s.cfg.L4ReduceFraction,
			MarginSafetyMultiplier:      s.cfg.MarginSafetyMultiplier,
			RiskMonitorIntervalSec:      s.cfg.RiskMonitorIntervalSec,
			EnableLiqTrendTracking:      s.cfg.EnableLiqTrendTracking,
			LiqProjectionMinutes:        s.cfg.LiqProjectionMinutes,
			LiqWarningSlopeThresh:       s.cfg.LiqWarningSlopeThresh,
			LiqCriticalSlopeThresh:      s.cfg.LiqCriticalSlopeThresh,
			LiqMinSamples:               s.cfg.LiqMinSamples,
			EnableCapitalAllocator:      s.cfg.EnableCapitalAllocator,
			MaxTotalExposureUSDT:        s.cfg.MaxTotalExposureUSDT,
			MaxPerpPerpPct:              s.cfg.MaxPerpPerpPct,
			MaxSpotFuturesPct:           s.cfg.MaxSpotFuturesPct,
			MaxPerExchangePct:           s.cfg.MaxPerExchangePct,
			ReservationTTLSec:           s.cfg.ReservationTTLSec,
			EnableExchangeHealthScoring: s.cfg.EnableExchangeHealthScoring,
			ExchHealthLatencyMs:         s.cfg.ExchHealthLatencyMs,
			ExchHealthMinUptime:         s.cfg.ExchHealthMinUptime,
			ExchHealthMinFillRate:       s.cfg.ExchHealthMinFillRate,
			ExchHealthMinScore:          s.cfg.ExchHealthMinScore,
			ExchHealthWindowMin:         s.cfg.ExchHealthWindowMin,
		},
		AI: configAIResponse{
			Endpoint:  s.cfg.AIEndpoint,
			Model:     s.cfg.AIModel,
			MaxTokens: s.cfg.AIMaxTokens,
			HasKey:    s.cfg.AIAPIKey != "",
		},
		Exchanges: s.buildExchangesResponse(),
	}
	resp.SpotFutures = &configSpotFuturesResponse{
		Enabled:                    s.cfg.SpotFuturesEnabled,
		MaxPositions:               s.cfg.SpotFuturesMaxPositions,
		CapitalPerPosition:         s.cfg.SpotFuturesCapitalPerPosition,
		Leverage:                   s.cfg.SpotFuturesLeverage,
		MonitorIntervalSec:         s.cfg.SpotFuturesMonitorIntervalSec,
		MinNetYieldAPR:             s.cfg.SpotFuturesMinNetYieldAPR,
		MaxBorrowAPR:               s.cfg.SpotFuturesMaxBorrowAPR,
		EnableBorrowSpikeDetection: s.cfg.EnableBorrowSpikeDetection,
		BorrowSpikeWindowMin:       s.cfg.BorrowSpikeWindowMin,
		BorrowSpikeMultiplier:      s.cfg.BorrowSpikeMultiplier,
		BorrowSpikeMinAbsolute:     s.cfg.BorrowSpikeMinAbsolute,
		Exchanges:                  s.cfg.SpotFuturesExchanges,
		ScanIntervalMin:            s.cfg.SpotFuturesScanIntervalMin,
		BorrowGraceMin:             s.cfg.SpotFuturesBorrowGraceMin,
		PriceExitPct:               s.cfg.SpotFuturesPriceExitPct,
		PriceEmergencyPct:          s.cfg.SpotFuturesPriceEmergencyPct,
		MarginExitPct:              s.cfg.SpotFuturesMarginExitPct,
		MarginEmergencyPct:         s.cfg.SpotFuturesMarginEmergencyPct,
		LossCooldownHours:          s.cfg.SpotFuturesLossCooldownHours,
		AutoEnabled:                s.cfg.SpotFuturesAutoEnabled,
		DryRun:                     s.cfg.SpotFuturesDryRun,
		PersistenceScans:           s.cfg.SpotFuturesPersistenceScans,
		ProfitTransferEnabled:      s.cfg.SpotFuturesProfitTransferEnabled,
		SeparateAcctMaxUSDT:        s.cfg.SpotFuturesSeparateAcctMaxUSDT,
		UnifiedAcctMaxUSDT:         s.cfg.SpotFuturesUnifiedAcctMaxUSDT,
	}
	return resp
}

func (s *Server) buildExchangesResponse() map[string]configExchangeResponse {
	getAddr := func(name string) map[string]string {
		if a, ok := s.cfg.ExchangeAddresses[name]; ok {
			return a
		}
		return nil
	}
	return map[string]configExchangeResponse{
		"binance": buildExchangeInfo(s.cfg.BinanceAPIKey, s.cfg.BinanceSecretKey, "", false, getAddr("binance"), s.cfg.IsExchangeEnabled("binance")),
		"bybit":   buildExchangeInfo(s.cfg.BybitAPIKey, s.cfg.BybitSecretKey, "", false, getAddr("bybit"), s.cfg.IsExchangeEnabled("bybit")),
		"gateio":  buildExchangeInfo(s.cfg.GateioAPIKey, s.cfg.GateioSecretKey, "", false, getAddr("gateio"), s.cfg.IsExchangeEnabled("gateio")),
		"bitget":  buildExchangeInfo(s.cfg.BitgetAPIKey, s.cfg.BitgetSecretKey, s.cfg.BitgetPassphrase, true, getAddr("bitget"), s.cfg.IsExchangeEnabled("bitget")),
		"okx":     buildExchangeInfo(s.cfg.OKXAPIKey, s.cfg.OKXSecretKey, s.cfg.OKXPassphrase, true, getAddr("okx"), s.cfg.IsExchangeEnabled("okx")),
		"bingx":   buildExchangeInfo(s.cfg.BingXAPIKey, s.cfg.BingXSecretKey, "", false, getAddr("bingx"), s.cfg.IsExchangeEnabled("bingx")),
	}
}

// handleGetConfig returns the current runtime configuration (safe fields only).
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, Response{OK: true, Data: s.buildConfigResponse()})
}

func (s *Server) handleGetExchangeHealth(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]interface{}, len(s.exchanges))
	for name := range s.exchanges {
		if s.scorer == nil {
			data[name] = map[string]interface{}{
				"exchange":            name,
				"score":               1.0,
				"latency_score":       1.0,
				"uptime_score":        1.0,
				"fill_rate_score":     1.0,
				"ws_uptime":           1.0,
				"fill_rate":           1.0,
				"window_minutes":      60,
				"min_score_for_entry": 0.5,
			}
			continue
		}
		data[name] = s.scorer.Snapshot(name)
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: data})
}

// ---------- Nested config update structs (pointer fields to detect presence) ----------

type configUpdate struct {
	DryRun      *bool                      `json:"dry_run"`
	Strategy    *strategyUpdate            `json:"strategy"`
	Fund        *fundUpdate                `json:"fund"`
	Risk        *riskUpdate                `json:"risk"`
	AI          *aiUpdate                  `json:"ai"`
	Exchanges   map[string]*exchangeUpdate `json:"exchanges"`
	SpotFutures *spotFuturesUpdate         `json:"spot_futures"`
}

type spotFuturesUpdate struct {
	Enabled                    *bool    `json:"enabled"`
	MaxPositions               *int     `json:"max_positions"`
	CapitalPerPosition         *float64 `json:"capital_per_position"`
	Leverage                   *int     `json:"leverage"`
	MonitorIntervalSec         *int     `json:"monitor_interval_sec"`
	MinNetYieldAPR             *float64 `json:"min_net_yield_apr"`
	MaxBorrowAPR               *float64 `json:"max_borrow_apr"`
	EnableBorrowSpikeDetection *bool    `json:"enable_borrow_spike_detection"`
	BorrowSpikeWindowMin       *int     `json:"borrow_spike_window_min"`
	BorrowSpikeMultiplier      *float64 `json:"borrow_spike_multiplier"`
	BorrowSpikeMinAbsolute     *float64 `json:"borrow_spike_min_absolute"`
	Exchanges                  []string `json:"exchanges"`
	ScanIntervalMin            *int     `json:"scan_interval_min"`
	BorrowGraceMin             *int     `json:"borrow_grace_min"`
	PriceExitPct               *float64 `json:"price_exit_pct"`
	PriceEmergencyPct          *float64 `json:"price_emergency_pct"`
	MarginExitPct              *float64 `json:"margin_exit_pct"`
	MarginEmergencyPct         *float64 `json:"margin_emergency_pct"`
	LossCooldownHours          *int     `json:"loss_cooldown_hours"`
	AutoEnabled                *bool    `json:"auto_enabled"`
	DryRun                     *bool    `json:"auto_dry_run"`
	LegacyDryRun               *bool    `json:"dry_run"`
	PersistenceScans           *int     `json:"persistence_scans"`
	ProfitTransferEnabled      *bool    `json:"profit_transfer_enabled"`
	SeparateAcctMaxUSDT        *float64 `json:"separate_acct_max_usdt"`
	UnifiedAcctMaxUSDT         *float64 `json:"unified_acct_max_usdt"`
}

type exchangeUpdate struct {
	Enabled    *bool             `json:"enabled"`
	APIKey     *string           `json:"api_key"`
	SecretKey  *string           `json:"secret_key"`
	Passphrase *string           `json:"passphrase"`
	Address    map[string]string `json:"address"`
}

type aiUpdate struct {
	Endpoint  *string `json:"endpoint"`
	APIKey    *string `json:"api_key"`
	Model     *string `json:"model"`
	MaxTokens *int    `json:"max_tokens"`
}

type strategyUpdate struct {
	TopOpportunities    *int             `json:"top_opportunities"`
	ScanMinutes         []int            `json:"scan_minutes"`
	EntryScanMinute     *int             `json:"entry_scan_minute"`
	ExitScanMinute      *int             `json:"exit_scan_minute"`
	RotateScanMinute    *int             `json:"rotate_scan_minute"`
	RebalanceScanMinute *int             `json:"rebalance_scan_minute"`
	RebalanceAfterExit  *bool            `json:"rebalance_after_exit"`
	Discovery           *discoveryUpdate `json:"discovery"`
	Entry               *entryUpdate     `json:"entry"`
	Exit                *exitUpdate      `json:"exit"`
	Rotation            *rotationUpdate  `json:"rotation"`
}

type discoveryUpdate struct {
	MinHoldTimeHours        *int               `json:"min_hold_time_hours"`
	MaxCostRatio            *float64           `json:"max_cost_ratio"`
	MaxPriceGapBPS          *float64           `json:"max_price_gap_bps"`
	PriceGapFreeBPS         *float64           `json:"price_gap_free_bps"`
	MaxGapRecoveryIntervals *float64           `json:"max_gap_recovery_intervals"`
	MaxIntervalHours        *float64           `json:"max_interval_hours"`
	DelistFilter            *bool              `json:"delist_filter"`
	Persistence             *persistenceUpdate `json:"persistence"`
}

type persistenceUpdate struct {
	LookbackMin1h *int `json:"lookback_min_1h"`
	MinCount1h    *int `json:"min_count_1h"`
	LookbackMin4h *int `json:"lookback_min_4h"`
	MinCount4h    *int `json:"min_count_4h"`
	LookbackMin8h *int `json:"lookback_min_8h"`
	MinCount8h    *int `json:"min_count_8h"`

	SpreadStabilityRatio1h  *float64 `json:"spread_stability_ratio_1h"`
	SpreadStabilityOIRank1h *int     `json:"spread_stability_oi_rank_1h"`
	SpreadStabilityRatio4h  *float64 `json:"spread_stability_ratio_4h"`
	SpreadStabilityOIRank4h *int     `json:"spread_stability_oi_rank_4h"`
	SpreadStabilityRatio8h  *float64 `json:"spread_stability_ratio_8h"`
	SpreadStabilityOIRank8h *int     `json:"spread_stability_oi_rank_8h"`

	EnableSpreadStabilityGate       *bool    `json:"enable_spread_stability_gate"`
	SpreadVolatilityMaxCV           *float64 `json:"spread_volatility_max_cv"`
	SpreadVolatilityMinSamples      *int     `json:"spread_volatility_min_samples"`
	SpreadStabilityStricterForAuto  *bool    `json:"spread_stability_stricter_for_auto"`
	SpreadStabilityAutoCVMultiplier *float64 `json:"spread_stability_auto_cv_multiplier"`
	FundingWindowMin                *int     `json:"funding_window_min"`
}

type entryUpdate struct {
	SlippageLimitBPS     *float64 `json:"slippage_limit_bps"`
	MinChunkUSDT         *float64 `json:"min_chunk_usdt"`
	EntryTimeoutSec      *int     `json:"entry_timeout_sec"`
	LossCooldownHours    *float64 `json:"loss_cooldown_hours"`
	ReEnterCooldownHours *float64 `json:"re_enter_cooldown_hours"`
	BacktestDays         *int     `json:"backtest_days"`
	BacktestMinProfit    *float64 `json:"backtest_min_profit"`
}

type exitUpdate struct {
	DepthTimeoutSec         *int  `json:"depth_timeout_sec"`
	EnableSpreadReversal    *bool `json:"enable_spread_reversal"`
	SpreadReversalTolerance *int  `json:"spread_reversal_tolerance"`
	ReversalResetOnRecover  *bool `json:"reversal_reset_on_recover"`
	ZeroSpreadTolerance     *int  `json:"zero_spread_tolerance"`
}

type rotationUpdate struct {
	ThresholdBPS *float64 `json:"threshold_bps"`
	CooldownMin  *int     `json:"cooldown_min"`
}

type fundUpdate struct {
	MaxPositions  *int     `json:"max_positions"`
	Leverage      *int     `json:"leverage"`
	CapitalPerLeg *float64 `json:"capital_per_leg"`
}

type riskUpdate struct {
	MarginL3Threshold           *float64 `json:"margin_l3_threshold"`
	MarginL4Threshold           *float64 `json:"margin_l4_threshold"`
	MarginL5Threshold           *float64 `json:"margin_l5_threshold"`
	L4ReduceFraction            *float64 `json:"l4_reduce_fraction"`
	MarginSafetyMultiplier      *float64 `json:"margin_safety_multiplier"`
	RiskMonitorIntervalSec      *int     `json:"risk_monitor_interval_sec"`
	EnableLiqTrendTracking      *bool    `json:"enable_liq_trend_tracking"`
	LiqProjectionMinutes        *int     `json:"liq_projection_minutes"`
	LiqWarningSlopeThresh       *float64 `json:"liq_warning_slope_thresh"`
	LiqCriticalSlopeThresh      *float64 `json:"liq_critical_slope_thresh"`
	LiqMinSamples               *int     `json:"liq_min_samples"`
	EnableCapitalAllocator      *bool    `json:"enable_capital_allocator"`
	MaxTotalExposureUSDT        *float64 `json:"max_total_exposure_usdt"`
	MaxPerpPerpPct              *float64 `json:"max_perp_perp_pct"`
	MaxSpotFuturesPct           *float64 `json:"max_spot_futures_pct"`
	MaxPerExchangePct           *float64 `json:"max_per_exchange_pct"`
	ReservationTTLSec           *int     `json:"reservation_ttl_sec"`
	EnableExchangeHealthScoring *bool    `json:"enable_exchange_health_scoring"`
	ExchHealthLatencyMs         *int     `json:"exch_health_latency_ms"`
	ExchHealthMinUptime         *float64 `json:"exch_health_min_uptime"`
	ExchHealthMinFillRate       *float64 `json:"exch_health_min_fill_rate"`
	ExchHealthMinScore          *float64 `json:"exch_health_min_score"`
	ExchHealthWindowMin         *int     `json:"exch_health_window_min"`
}

// handlePostConfig accepts a nested JSON body and updates config fields,
// persisting changes to Redis at key arb:config.
func (s *Server) handlePostConfig(w http.ResponseWriter, r *http.Request) {
	var upd configUpdate
	if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Error: "invalid request body"})
		return
	}

	// Apply updates
	if upd.DryRun != nil {
		s.cfg.DryRun = *upd.DryRun
	}

	if st := upd.Strategy; st != nil {
		if len(st.ScanMinutes) > 0 {
			s.cfg.ScanMinutes = st.ScanMinutes
		}
		if st.EntryScanMinute != nil && *st.EntryScanMinute >= 0 && *st.EntryScanMinute < 60 {
			s.cfg.EntryScanMinute = *st.EntryScanMinute
		}
		if st.ExitScanMinute != nil && *st.ExitScanMinute >= 0 && *st.ExitScanMinute < 60 {
			s.cfg.ExitScanMinute = *st.ExitScanMinute
		}
		if st.RotateScanMinute != nil && *st.RotateScanMinute >= 0 && *st.RotateScanMinute < 60 {
			s.cfg.RotateScanMinute = *st.RotateScanMinute
		}
		if st.RebalanceScanMinute != nil && *st.RebalanceScanMinute >= 0 && *st.RebalanceScanMinute < 60 {
			s.cfg.RebalanceScanMinute = *st.RebalanceScanMinute
		}
		if st.RebalanceAfterExit != nil {
			s.cfg.RebalanceAfterExit = *st.RebalanceAfterExit
		}
		if st.TopOpportunities != nil && *st.TopOpportunities > 0 {
			s.cfg.TopOpportunities = *st.TopOpportunities
		}
		// Ensure special scan minutes are in the schedule after any updates.
		s.cfg.EnsureScanMinutes()
		if d := st.Discovery; d != nil {
			if d.MinHoldTimeHours != nil && *d.MinHoldTimeHours > 0 {
				s.cfg.MinHoldTime = time.Duration(*d.MinHoldTimeHours) * time.Hour
			}
			if d.MaxCostRatio != nil && *d.MaxCostRatio > 0 {
				s.cfg.MaxCostRatio = *d.MaxCostRatio
			}
			if d.MaxPriceGapBPS != nil && *d.MaxPriceGapBPS >= 0 {
				s.cfg.MaxPriceGapBPS = *d.MaxPriceGapBPS
			}
			if d.PriceGapFreeBPS != nil && *d.PriceGapFreeBPS >= 0 {
				s.cfg.PriceGapFreeBPS = *d.PriceGapFreeBPS
			}
			if d.MaxGapRecoveryIntervals != nil && *d.MaxGapRecoveryIntervals >= 0 {
				s.cfg.MaxGapRecoveryIntervals = *d.MaxGapRecoveryIntervals
			}
			if d.MaxIntervalHours != nil && *d.MaxIntervalHours >= 0 {
				s.cfg.MaxIntervalHours = *d.MaxIntervalHours
			}
			if d.DelistFilter != nil {
				s.cfg.DelistFilterEnabled = *d.DelistFilter
			}
			if p := d.Persistence; p != nil {
				if p.LookbackMin1h != nil && *p.LookbackMin1h > 0 {
					s.cfg.PersistLookback1h = time.Duration(*p.LookbackMin1h) * time.Minute
				}
				if p.MinCount1h != nil && *p.MinCount1h > 0 {
					s.cfg.PersistMinCount1h = *p.MinCount1h
				}
				if p.LookbackMin4h != nil && *p.LookbackMin4h > 0 {
					s.cfg.PersistLookback4h = time.Duration(*p.LookbackMin4h) * time.Minute
				}
				if p.MinCount4h != nil && *p.MinCount4h > 0 {
					s.cfg.PersistMinCount4h = *p.MinCount4h
				}
				if p.LookbackMin8h != nil && *p.LookbackMin8h > 0 {
					s.cfg.PersistLookback8h = time.Duration(*p.LookbackMin8h) * time.Minute
				}
				if p.MinCount8h != nil && *p.MinCount8h > 0 {
					s.cfg.PersistMinCount8h = *p.MinCount8h
				}
				if p.SpreadStabilityRatio1h != nil && *p.SpreadStabilityRatio1h >= 0 {
					s.cfg.SpreadStabilityRatio1h = *p.SpreadStabilityRatio1h
				}
				if p.SpreadStabilityOIRank1h != nil && *p.SpreadStabilityOIRank1h >= 0 {
					s.cfg.SpreadStabilityOIRank1h = *p.SpreadStabilityOIRank1h
				}
				if p.SpreadStabilityRatio4h != nil && *p.SpreadStabilityRatio4h >= 0 {
					s.cfg.SpreadStabilityRatio4h = *p.SpreadStabilityRatio4h
				}
				if p.SpreadStabilityOIRank4h != nil && *p.SpreadStabilityOIRank4h >= 0 {
					s.cfg.SpreadStabilityOIRank4h = *p.SpreadStabilityOIRank4h
				}
				if p.SpreadStabilityRatio8h != nil && *p.SpreadStabilityRatio8h >= 0 {
					s.cfg.SpreadStabilityRatio8h = *p.SpreadStabilityRatio8h
				}
				if p.SpreadStabilityOIRank8h != nil && *p.SpreadStabilityOIRank8h >= 0 {
					s.cfg.SpreadStabilityOIRank8h = *p.SpreadStabilityOIRank8h
				}
				if p.EnableSpreadStabilityGate != nil {
					s.cfg.EnableSpreadStabilityGate = *p.EnableSpreadStabilityGate
				}
				if p.SpreadVolatilityMaxCV != nil && *p.SpreadVolatilityMaxCV >= 0 {
					s.cfg.SpreadVolatilityMaxCV = *p.SpreadVolatilityMaxCV
				}
				if p.SpreadVolatilityMinSamples != nil && *p.SpreadVolatilityMinSamples >= 0 {
					s.cfg.SpreadVolatilityMinSamples = *p.SpreadVolatilityMinSamples
				}
				if p.SpreadStabilityStricterForAuto != nil {
					s.cfg.SpreadStabilityStricterForAuto = *p.SpreadStabilityStricterForAuto
				}
				if p.SpreadStabilityAutoCVMultiplier != nil && *p.SpreadStabilityAutoCVMultiplier >= 0 {
					s.cfg.SpreadStabilityAutoCVMultiplier = *p.SpreadStabilityAutoCVMultiplier
				}
				if p.FundingWindowMin != nil && *p.FundingWindowMin > 0 {
					s.cfg.FundingWindowMin = *p.FundingWindowMin
				}
			}
		}
		if e := st.Entry; e != nil {
			if e.SlippageLimitBPS != nil && *e.SlippageLimitBPS >= 0 {
				s.cfg.SlippageBPS = *e.SlippageLimitBPS
			}
			if e.MinChunkUSDT != nil && *e.MinChunkUSDT >= 0 {
				s.cfg.MinChunkUSDT = *e.MinChunkUSDT
			}
			if e.EntryTimeoutSec != nil && *e.EntryTimeoutSec > 0 {
				s.cfg.EntryTimeoutSec = *e.EntryTimeoutSec
			}
			if e.LossCooldownHours != nil && *e.LossCooldownHours >= 0 {
				s.cfg.LossCooldownHours = *e.LossCooldownHours
			}
			if e.ReEnterCooldownHours != nil && *e.ReEnterCooldownHours >= 0 {
				s.cfg.ReEnterCooldownHours = *e.ReEnterCooldownHours
			}
			if e.BacktestDays != nil && *e.BacktestDays >= 0 {
				s.cfg.BacktestDays = *e.BacktestDays
			}
			if e.BacktestMinProfit != nil {
				s.cfg.BacktestMinProfit = *e.BacktestMinProfit
			}
		}
		if x := st.Exit; x != nil {
			if x.DepthTimeoutSec != nil && *x.DepthTimeoutSec > 0 {
				s.cfg.ExitDepthTimeoutSec = *x.DepthTimeoutSec
			}
			if x.EnableSpreadReversal != nil {
				s.cfg.EnableSpreadReversal = *x.EnableSpreadReversal
			}
			if x.SpreadReversalTolerance != nil && *x.SpreadReversalTolerance >= 0 {
				s.cfg.SpreadReversalTolerance = *x.SpreadReversalTolerance
			}
			if x.ReversalResetOnRecover != nil {
				s.cfg.ReversalResetOnRecover = *x.ReversalResetOnRecover
			}
			if x.ZeroSpreadTolerance != nil && *x.ZeroSpreadTolerance >= 0 {
				s.cfg.ZeroSpreadTolerance = *x.ZeroSpreadTolerance
			}
		}
		if rot := st.Rotation; rot != nil {
			if rot.ThresholdBPS != nil && *rot.ThresholdBPS >= 0 {
				s.cfg.RotationThresholdBPS = *rot.ThresholdBPS
			}
			if rot.CooldownMin != nil && *rot.CooldownMin >= 0 {
				s.cfg.RotationCooldownMin = *rot.CooldownMin
			}
		}
	}

	if f := upd.Fund; f != nil {
		if f.MaxPositions != nil && *f.MaxPositions > 0 {
			s.cfg.MaxPositions = *f.MaxPositions
		}
		if f.Leverage != nil && *f.Leverage > 0 {
			s.cfg.Leverage = *f.Leverage
		}
		if f.CapitalPerLeg != nil && *f.CapitalPerLeg >= 0 {
			s.cfg.CapitalPerLeg = *f.CapitalPerLeg
		}
	}

	if rk := upd.Risk; rk != nil {
		if rk.MarginL3Threshold != nil && *rk.MarginL3Threshold > 0 && *rk.MarginL3Threshold <= 1 {
			s.cfg.MarginL3Threshold = *rk.MarginL3Threshold
		}
		if rk.MarginL4Threshold != nil && *rk.MarginL4Threshold > 0 && *rk.MarginL4Threshold <= 1 {
			s.cfg.MarginL4Threshold = *rk.MarginL4Threshold
		}
		if rk.MarginL5Threshold != nil && *rk.MarginL5Threshold > 0 && *rk.MarginL5Threshold <= 1 {
			s.cfg.MarginL5Threshold = *rk.MarginL5Threshold
		}
		if rk.L4ReduceFraction != nil && *rk.L4ReduceFraction > 0 && *rk.L4ReduceFraction <= 1 {
			s.cfg.L4ReduceFraction = *rk.L4ReduceFraction
		}
		if rk.MarginSafetyMultiplier != nil && *rk.MarginSafetyMultiplier >= 1.0 {
			s.cfg.MarginSafetyMultiplier = *rk.MarginSafetyMultiplier
		}
		if rk.RiskMonitorIntervalSec != nil && *rk.RiskMonitorIntervalSec > 0 {
			s.cfg.RiskMonitorIntervalSec = *rk.RiskMonitorIntervalSec
		}
		if rk.EnableLiqTrendTracking != nil {
			s.cfg.EnableLiqTrendTracking = *rk.EnableLiqTrendTracking
		}
		if rk.LiqProjectionMinutes != nil && *rk.LiqProjectionMinutes > 0 {
			s.cfg.LiqProjectionMinutes = *rk.LiqProjectionMinutes
		}
		if rk.LiqWarningSlopeThresh != nil && *rk.LiqWarningSlopeThresh >= 0 {
			s.cfg.LiqWarningSlopeThresh = *rk.LiqWarningSlopeThresh
		}
		if rk.LiqCriticalSlopeThresh != nil && *rk.LiqCriticalSlopeThresh >= 0 {
			s.cfg.LiqCriticalSlopeThresh = *rk.LiqCriticalSlopeThresh
		}
		if rk.LiqMinSamples != nil && *rk.LiqMinSamples > 0 {
			s.cfg.LiqMinSamples = *rk.LiqMinSamples
		}
		if rk.EnableCapitalAllocator != nil {
			s.cfg.EnableCapitalAllocator = *rk.EnableCapitalAllocator
		}
		if rk.MaxTotalExposureUSDT != nil && *rk.MaxTotalExposureUSDT >= 0 {
			s.cfg.MaxTotalExposureUSDT = *rk.MaxTotalExposureUSDT
		}
		if rk.MaxPerpPerpPct != nil && *rk.MaxPerpPerpPct >= 0 && *rk.MaxPerpPerpPct <= 1 {
			s.cfg.MaxPerpPerpPct = *rk.MaxPerpPerpPct
		}
		if rk.MaxSpotFuturesPct != nil && *rk.MaxSpotFuturesPct >= 0 && *rk.MaxSpotFuturesPct <= 1 {
			s.cfg.MaxSpotFuturesPct = *rk.MaxSpotFuturesPct
		}
		if rk.MaxPerExchangePct != nil && *rk.MaxPerExchangePct >= 0 && *rk.MaxPerExchangePct <= 1 {
			s.cfg.MaxPerExchangePct = *rk.MaxPerExchangePct
		}
		if rk.ReservationTTLSec != nil && *rk.ReservationTTLSec > 0 {
			s.cfg.ReservationTTLSec = *rk.ReservationTTLSec
		}
		if rk.EnableExchangeHealthScoring != nil {
			s.cfg.EnableExchangeHealthScoring = *rk.EnableExchangeHealthScoring
		}
		if rk.ExchHealthLatencyMs != nil && *rk.ExchHealthLatencyMs > 0 {
			s.cfg.ExchHealthLatencyMs = *rk.ExchHealthLatencyMs
		}
		if rk.ExchHealthMinUptime != nil && *rk.ExchHealthMinUptime >= 0 && *rk.ExchHealthMinUptime <= 1 {
			s.cfg.ExchHealthMinUptime = *rk.ExchHealthMinUptime
		}
		if rk.ExchHealthMinFillRate != nil && *rk.ExchHealthMinFillRate >= 0 && *rk.ExchHealthMinFillRate <= 1 {
			s.cfg.ExchHealthMinFillRate = *rk.ExchHealthMinFillRate
		}
		if rk.ExchHealthMinScore != nil && *rk.ExchHealthMinScore >= 0 && *rk.ExchHealthMinScore <= 1 {
			s.cfg.ExchHealthMinScore = *rk.ExchHealthMinScore
		}
		if rk.ExchHealthWindowMin != nil && *rk.ExchHealthWindowMin > 0 {
			s.cfg.ExchHealthWindowMin = *rk.ExchHealthWindowMin
		}
	}

	if ai := upd.AI; ai != nil {
		if ai.Endpoint != nil {
			s.cfg.AIEndpoint = *ai.Endpoint
		}
		if ai.APIKey != nil {
			s.cfg.AIAPIKey = *ai.APIKey
		}
		if ai.Model != nil && *ai.Model != "" {
			s.cfg.AIModel = *ai.Model
		}
		if ai.MaxTokens != nil && *ai.MaxTokens > 0 {
			s.cfg.AIMaxTokens = *ai.MaxTokens
		}
	}

	// Exchanges
	exchangeSecretOverrides := make(map[string]config.ExchangeSecretOverride)
	if upd.Exchanges != nil {
		for name, eu := range upd.Exchanges {
			if eu == nil {
				continue
			}
			if eu.Enabled != nil {
				v := *eu.Enabled
				switch name {
				case "binance":
					s.cfg.BinanceEnabled = &v
				case "bybit":
					s.cfg.BybitEnabled = &v
				case "gateio":
					s.cfg.GateioEnabled = &v
				case "bitget":
					s.cfg.BitgetEnabled = &v
				case "okx":
					s.cfg.OKXEnabled = &v
				case "bingx":
					s.cfg.BingXEnabled = &v
				}
			}
			// Only update secret fields when the user actually typed a new value.
			// Empty strings are ignored to prevent accidental key wipe from
			// dashboard saves that don't include secret fields.
			var override config.ExchangeSecretOverride
			switch name {
			case "binance":
				if eu.APIKey != nil && *eu.APIKey != "" {
					s.cfg.BinanceAPIKey = *eu.APIKey
					override.APIKey = *eu.APIKey
				}
				if eu.SecretKey != nil && *eu.SecretKey != "" {
					s.cfg.BinanceSecretKey = *eu.SecretKey
					override.SecretKey = *eu.SecretKey
				}
			case "bybit":
				if eu.APIKey != nil && *eu.APIKey != "" {
					s.cfg.BybitAPIKey = *eu.APIKey
					override.APIKey = *eu.APIKey
				}
				if eu.SecretKey != nil && *eu.SecretKey != "" {
					s.cfg.BybitSecretKey = *eu.SecretKey
					override.SecretKey = *eu.SecretKey
				}
			case "gateio":
				if eu.APIKey != nil && *eu.APIKey != "" {
					s.cfg.GateioAPIKey = *eu.APIKey
					override.APIKey = *eu.APIKey
				}
				if eu.SecretKey != nil && *eu.SecretKey != "" {
					s.cfg.GateioSecretKey = *eu.SecretKey
					override.SecretKey = *eu.SecretKey
				}
			case "bitget":
				if eu.APIKey != nil && *eu.APIKey != "" {
					s.cfg.BitgetAPIKey = *eu.APIKey
					override.APIKey = *eu.APIKey
				}
				if eu.SecretKey != nil && *eu.SecretKey != "" {
					s.cfg.BitgetSecretKey = *eu.SecretKey
					override.SecretKey = *eu.SecretKey
				}
				if eu.Passphrase != nil && *eu.Passphrase != "" {
					s.cfg.BitgetPassphrase = *eu.Passphrase
					override.Passphrase = *eu.Passphrase
				}
			case "okx":
				if eu.APIKey != nil && *eu.APIKey != "" {
					s.cfg.OKXAPIKey = *eu.APIKey
					override.APIKey = *eu.APIKey
				}
				if eu.SecretKey != nil && *eu.SecretKey != "" {
					s.cfg.OKXSecretKey = *eu.SecretKey
					override.SecretKey = *eu.SecretKey
				}
				if eu.Passphrase != nil && *eu.Passphrase != "" {
					s.cfg.OKXPassphrase = *eu.Passphrase
					override.Passphrase = *eu.Passphrase
				}
			case "bingx":
				if eu.APIKey != nil && *eu.APIKey != "" {
					s.cfg.BingXAPIKey = *eu.APIKey
					override.APIKey = *eu.APIKey
				}
				if eu.SecretKey != nil && *eu.SecretKey != "" {
					s.cfg.BingXSecretKey = *eu.SecretKey
					override.SecretKey = *eu.SecretKey
				}
			}
			if override.APIKey != "" || override.SecretKey != "" || override.Passphrase != "" {
				exchangeSecretOverrides[name] = override
			}
			// Update addresses
			if eu.Address != nil {
				if s.cfg.ExchangeAddresses == nil {
					s.cfg.ExchangeAddresses = make(map[string]map[string]string)
				}
				if s.cfg.ExchangeAddresses[name] == nil {
					s.cfg.ExchangeAddresses[name] = make(map[string]string)
				}
				for chain, addr := range eu.Address {
					if addr == "" {
						delete(s.cfg.ExchangeAddresses[name], chain)
					} else {
						s.cfg.ExchangeAddresses[name][chain] = addr
					}
				}
			}
		}
	}

	if sf := upd.SpotFutures; sf != nil {
		if sf.Enabled != nil {
			s.cfg.SpotFuturesEnabled = *sf.Enabled
		}
		if sf.MaxPositions != nil && *sf.MaxPositions > 0 {
			s.cfg.SpotFuturesMaxPositions = *sf.MaxPositions
		}
		if sf.CapitalPerPosition != nil && *sf.CapitalPerPosition > 0 {
			s.cfg.SpotFuturesCapitalPerPosition = *sf.CapitalPerPosition
		}
		if sf.Leverage != nil && *sf.Leverage > 0 {
			s.cfg.SpotFuturesLeverage = *sf.Leverage
		}
		if sf.MonitorIntervalSec != nil && *sf.MonitorIntervalSec > 0 {
			s.cfg.SpotFuturesMonitorIntervalSec = *sf.MonitorIntervalSec
		}
		if sf.MinNetYieldAPR != nil && *sf.MinNetYieldAPR >= 0 {
			s.cfg.SpotFuturesMinNetYieldAPR = *sf.MinNetYieldAPR
		}
		if sf.MaxBorrowAPR != nil && *sf.MaxBorrowAPR >= 0 {
			s.cfg.SpotFuturesMaxBorrowAPR = *sf.MaxBorrowAPR
		}
		if sf.EnableBorrowSpikeDetection != nil {
			s.cfg.EnableBorrowSpikeDetection = *sf.EnableBorrowSpikeDetection
		}
		if sf.BorrowSpikeWindowMin != nil && *sf.BorrowSpikeWindowMin > 0 {
			s.cfg.BorrowSpikeWindowMin = *sf.BorrowSpikeWindowMin
		}
		if sf.BorrowSpikeMultiplier != nil && *sf.BorrowSpikeMultiplier > 0 {
			s.cfg.BorrowSpikeMultiplier = *sf.BorrowSpikeMultiplier
		}
		if sf.BorrowSpikeMinAbsolute != nil && *sf.BorrowSpikeMinAbsolute >= 0 {
			s.cfg.BorrowSpikeMinAbsolute = *sf.BorrowSpikeMinAbsolute
		}
		if sf.Exchanges != nil {
			s.cfg.SpotFuturesExchanges = sf.Exchanges
		}
		if sf.ScanIntervalMin != nil && *sf.ScanIntervalMin > 0 {
			s.cfg.SpotFuturesScanIntervalMin = *sf.ScanIntervalMin
		}
		if sf.BorrowGraceMin != nil && *sf.BorrowGraceMin >= 0 {
			s.cfg.SpotFuturesBorrowGraceMin = *sf.BorrowGraceMin
		}
		if sf.PriceExitPct != nil && *sf.PriceExitPct > 0 {
			s.cfg.SpotFuturesPriceExitPct = *sf.PriceExitPct
		}
		if sf.PriceEmergencyPct != nil && *sf.PriceEmergencyPct > 0 {
			s.cfg.SpotFuturesPriceEmergencyPct = *sf.PriceEmergencyPct
		}
		if sf.MarginExitPct != nil && *sf.MarginExitPct > 0 && *sf.MarginExitPct <= 100 {
			s.cfg.SpotFuturesMarginExitPct = *sf.MarginExitPct
		}
		if sf.MarginEmergencyPct != nil && *sf.MarginEmergencyPct > 0 && *sf.MarginEmergencyPct <= 100 {
			s.cfg.SpotFuturesMarginEmergencyPct = *sf.MarginEmergencyPct
		}
		if sf.LossCooldownHours != nil && *sf.LossCooldownHours >= 0 {
			s.cfg.SpotFuturesLossCooldownHours = *sf.LossCooldownHours
		}
		if sf.AutoEnabled != nil {
			s.cfg.SpotFuturesAutoEnabled = *sf.AutoEnabled
		}
		if sf.DryRun != nil {
			s.cfg.SpotFuturesDryRun = *sf.DryRun
		} else if sf.LegacyDryRun != nil {
			s.cfg.SpotFuturesDryRun = *sf.LegacyDryRun
		}
		if sf.PersistenceScans != nil && *sf.PersistenceScans >= 0 {
			s.cfg.SpotFuturesPersistenceScans = *sf.PersistenceScans
		}
		if sf.ProfitTransferEnabled != nil {
			s.cfg.SpotFuturesProfitTransferEnabled = *sf.ProfitTransferEnabled
		}
		if sf.SeparateAcctMaxUSDT != nil && *sf.SeparateAcctMaxUSDT > 0 {
			s.cfg.SpotFuturesSeparateAcctMaxUSDT = *sf.SeparateAcctMaxUSDT
		}
		if sf.UnifiedAcctMaxUSDT != nil && *sf.UnifiedAcctMaxUSDT > 0 {
			s.cfg.SpotFuturesUnifiedAcctMaxUSDT = *sf.UnifiedAcctMaxUSDT
		}
	}

	// Persist config fields to Redis HASH (arb:config) using flat keys.
	snapshot := s.buildConfigResponse()

	fields := map[string]interface{}{
		"min_hold_time_hours":                 strconv.Itoa(snapshot.Strategy.Discovery.MinHoldTimeHours),
		"max_cost_ratio":                      strconv.FormatFloat(snapshot.Strategy.Discovery.MaxCostRatio, 'f', -1, 64),
		"max_positions":                       strconv.Itoa(snapshot.Fund.MaxPositions),
		"leverage":                            strconv.Itoa(snapshot.Fund.Leverage),
		"slippage_limit_bps":                  strconv.FormatFloat(snapshot.Strategy.Entry.SlippageLimitBPS, 'f', -1, 64),
		"rebalance_scan_minute":               strconv.Itoa(snapshot.Strategy.RebalanceScanMinute),
		"rebalance_after_exit":                strconv.FormatBool(snapshot.Strategy.RebalanceAfterExit),
		"top_opportunities":                   strconv.Itoa(snapshot.Strategy.TopOpportunities),
		"entry_scan_minute":                   strconv.Itoa(snapshot.Strategy.EntryScanMinute),
		"exit_scan_minute":                    strconv.Itoa(snapshot.Strategy.ExitScanMinute),
		"rotate_scan_minute":                  strconv.Itoa(snapshot.Strategy.RotateScanMinute),
		"capital_per_leg":                     strconv.FormatFloat(snapshot.Fund.CapitalPerLeg, 'f', -1, 64),
		"dry_run":                             strconv.FormatBool(snapshot.DryRun),
		"entry_timeout_sec":                   strconv.Itoa(snapshot.Strategy.Entry.EntryTimeoutSec),
		"min_chunk_usdt":                      strconv.FormatFloat(snapshot.Strategy.Entry.MinChunkUSDT, 'f', -1, 64),
		"price_gap_free_bps":                  strconv.FormatFloat(snapshot.Strategy.Discovery.PriceGapFreeBPS, 'f', -1, 64),
		"max_price_gap_bps":                   strconv.FormatFloat(snapshot.Strategy.Discovery.MaxPriceGapBPS, 'f', -1, 64),
		"max_gap_recovery_intervals":          strconv.FormatFloat(snapshot.Strategy.Discovery.MaxGapRecoveryIntervals, 'f', -1, 64),
		"max_interval_hours":                  strconv.FormatFloat(snapshot.Strategy.Discovery.MaxIntervalHours, 'f', -1, 64),
		"delist_filter":                       strconv.FormatBool(snapshot.Strategy.Discovery.DelistFilter),
		"margin_l3_threshold":                 strconv.FormatFloat(snapshot.Risk.MarginL3Threshold, 'f', -1, 64),
		"margin_l4_threshold":                 strconv.FormatFloat(snapshot.Risk.MarginL4Threshold, 'f', -1, 64),
		"margin_l5_threshold":                 strconv.FormatFloat(snapshot.Risk.MarginL5Threshold, 'f', -1, 64),
		"l4_reduce_fraction":                  strconv.FormatFloat(snapshot.Risk.L4ReduceFraction, 'f', -1, 64),
		"margin_safety_multiplier":            strconv.FormatFloat(snapshot.Risk.MarginSafetyMultiplier, 'f', -1, 64),
		"risk_monitor_interval_sec":           strconv.Itoa(snapshot.Risk.RiskMonitorIntervalSec),
		"enable_liq_trend_tracking":           strconv.FormatBool(snapshot.Risk.EnableLiqTrendTracking),
		"liq_projection_minutes":              strconv.Itoa(snapshot.Risk.LiqProjectionMinutes),
		"liq_warning_slope_thresh":            strconv.FormatFloat(snapshot.Risk.LiqWarningSlopeThresh, 'f', -1, 64),
		"liq_critical_slope_thresh":           strconv.FormatFloat(snapshot.Risk.LiqCriticalSlopeThresh, 'f', -1, 64),
		"liq_min_samples":                     strconv.Itoa(snapshot.Risk.LiqMinSamples),
		"enable_capital_allocator":            strconv.FormatBool(snapshot.Risk.EnableCapitalAllocator),
		"max_total_exposure_usdt":             strconv.FormatFloat(snapshot.Risk.MaxTotalExposureUSDT, 'f', -1, 64),
		"max_perp_perp_pct":                   strconv.FormatFloat(snapshot.Risk.MaxPerpPerpPct, 'f', -1, 64),
		"max_spot_futures_pct":                strconv.FormatFloat(snapshot.Risk.MaxSpotFuturesPct, 'f', -1, 64),
		"max_per_exchange_pct":                strconv.FormatFloat(snapshot.Risk.MaxPerExchangePct, 'f', -1, 64),
		"reservation_ttl_sec":                 strconv.Itoa(snapshot.Risk.ReservationTTLSec),
		"enable_exchange_health_scoring":      strconv.FormatBool(snapshot.Risk.EnableExchangeHealthScoring),
		"exch_health_latency_ms":              strconv.Itoa(snapshot.Risk.ExchHealthLatencyMs),
		"exch_health_min_uptime":              strconv.FormatFloat(snapshot.Risk.ExchHealthMinUptime, 'f', -1, 64),
		"exch_health_min_fill_rate":           strconv.FormatFloat(snapshot.Risk.ExchHealthMinFillRate, 'f', -1, 64),
		"exch_health_min_score":               strconv.FormatFloat(snapshot.Risk.ExchHealthMinScore, 'f', -1, 64),
		"exch_health_window_min":              strconv.Itoa(snapshot.Risk.ExchHealthWindowMin),
		"exit_depth_timeout_sec":              strconv.Itoa(snapshot.Strategy.Exit.DepthTimeoutSec),
		"enable_spread_reversal":              strconv.FormatBool(snapshot.Strategy.Exit.EnableSpreadReversal),
		"spread_reversal_tolerance":           strconv.Itoa(snapshot.Strategy.Exit.SpreadReversalTolerance),
		"reversal_reset_on_recover":           strconv.FormatBool(snapshot.Strategy.Exit.ReversalResetOnRecover),
		"zero_spread_tolerance":               strconv.Itoa(snapshot.Strategy.Exit.ZeroSpreadTolerance),
		"rotation_threshold_bps":              strconv.FormatFloat(snapshot.Strategy.Rotation.ThresholdBPS, 'f', -1, 64),
		"rotation_cooldown_min":               strconv.Itoa(snapshot.Strategy.Rotation.CooldownMin),
		"persist_lookback_min_1h":             strconv.Itoa(snapshot.Strategy.Discovery.Persistence.LookbackMin1h),
		"persist_min_count_1h":                strconv.Itoa(snapshot.Strategy.Discovery.Persistence.MinCount1h),
		"persist_lookback_min_4h":             strconv.Itoa(snapshot.Strategy.Discovery.Persistence.LookbackMin4h),
		"persist_min_count_4h":                strconv.Itoa(snapshot.Strategy.Discovery.Persistence.MinCount4h),
		"persist_lookback_min_8h":             strconv.Itoa(snapshot.Strategy.Discovery.Persistence.LookbackMin8h),
		"persist_min_count_8h":                strconv.Itoa(snapshot.Strategy.Discovery.Persistence.MinCount8h),
		"spread_stability_ratio_1h":           strconv.FormatFloat(snapshot.Strategy.Discovery.Persistence.SpreadStabilityRatio1h, 'f', -1, 64),
		"spread_stability_oi_rank_1h":         strconv.Itoa(snapshot.Strategy.Discovery.Persistence.SpreadStabilityOIRank1h),
		"spread_stability_ratio_4h":           strconv.FormatFloat(snapshot.Strategy.Discovery.Persistence.SpreadStabilityRatio4h, 'f', -1, 64),
		"spread_stability_oi_rank_4h":         strconv.Itoa(snapshot.Strategy.Discovery.Persistence.SpreadStabilityOIRank4h),
		"spread_stability_ratio_8h":           strconv.FormatFloat(snapshot.Strategy.Discovery.Persistence.SpreadStabilityRatio8h, 'f', -1, 64),
		"spread_stability_oi_rank_8h":         strconv.Itoa(snapshot.Strategy.Discovery.Persistence.SpreadStabilityOIRank8h),
		"enable_spread_stability_gate":        strconv.FormatBool(snapshot.Strategy.Discovery.Persistence.EnableSpreadStabilityGate),
		"spread_volatility_max_cv":            strconv.FormatFloat(snapshot.Strategy.Discovery.Persistence.SpreadVolatilityMaxCV, 'f', -1, 64),
		"spread_volatility_min_samples":       strconv.Itoa(snapshot.Strategy.Discovery.Persistence.SpreadVolatilityMinSamples),
		"spread_stability_stricter_for_auto":  strconv.FormatBool(snapshot.Strategy.Discovery.Persistence.SpreadStabilityStricterForAuto),
		"spread_stability_auto_cv_multiplier": strconv.FormatFloat(snapshot.Strategy.Discovery.Persistence.SpreadStabilityAutoCVMultiplier, 'f', -1, 64),
		"funding_window_min":                  strconv.Itoa(snapshot.Strategy.Discovery.Persistence.FundingWindowMin),
		"loss_cooldown_hours":                 strconv.FormatFloat(snapshot.Strategy.Entry.LossCooldownHours, 'f', -1, 64),
		"re_enter_cooldown_hours":             strconv.FormatFloat(snapshot.Strategy.Entry.ReEnterCooldownHours, 'f', -1, 64),
		"backtest_days":                       strconv.Itoa(snapshot.Strategy.Entry.BacktestDays),
		"backtest_min_profit":                 strconv.FormatFloat(snapshot.Strategy.Entry.BacktestMinProfit, 'f', -1, 64),
	}

	if sf := snapshot.SpotFutures; sf != nil {
		fields["spot_futures_enabled"] = strconv.FormatBool(sf.Enabled)
		fields["spot_futures_max_positions"] = strconv.Itoa(sf.MaxPositions)
		fields["spot_futures_capital_per_position"] = strconv.FormatFloat(sf.CapitalPerPosition, 'f', -1, 64)
		fields["spot_futures_leverage"] = strconv.Itoa(sf.Leverage)
		fields["spot_futures_monitor_interval_sec"] = strconv.Itoa(sf.MonitorIntervalSec)
		fields["spot_futures_min_net_yield_apr"] = strconv.FormatFloat(sf.MinNetYieldAPR, 'f', -1, 64)
		fields["spot_futures_max_borrow_apr"] = strconv.FormatFloat(sf.MaxBorrowAPR, 'f', -1, 64)
		fields["spot_futures_enable_borrow_spike_detection"] = strconv.FormatBool(sf.EnableBorrowSpikeDetection)
		fields["spot_futures_borrow_spike_window_min"] = strconv.Itoa(sf.BorrowSpikeWindowMin)
		fields["spot_futures_borrow_spike_multiplier"] = strconv.FormatFloat(sf.BorrowSpikeMultiplier, 'f', -1, 64)
		fields["spot_futures_borrow_spike_min_absolute"] = strconv.FormatFloat(sf.BorrowSpikeMinAbsolute, 'f', -1, 64)
		fields["spot_futures_exchanges"] = strings.Join(sf.Exchanges, ",")
		fields["spot_futures_scan_interval_min"] = strconv.Itoa(sf.ScanIntervalMin)
		fields["spot_futures_borrow_grace_min"] = strconv.Itoa(sf.BorrowGraceMin)
		fields["spot_futures_price_exit_pct"] = strconv.FormatFloat(sf.PriceExitPct, 'f', -1, 64)
		fields["spot_futures_price_emergency_pct"] = strconv.FormatFloat(sf.PriceEmergencyPct, 'f', -1, 64)
		fields["spot_futures_margin_exit_pct"] = strconv.FormatFloat(sf.MarginExitPct, 'f', -1, 64)
		fields["spot_futures_margin_emergency_pct"] = strconv.FormatFloat(sf.MarginEmergencyPct, 'f', -1, 64)
		fields["spot_futures_loss_cooldown_hours"] = strconv.Itoa(sf.LossCooldownHours)
		fields["spot_futures_auto_enabled"] = strconv.FormatBool(sf.AutoEnabled)
		fields["spot_futures_dry_run"] = strconv.FormatBool(sf.DryRun)
		fields["spot_futures_persistence_scans"] = strconv.Itoa(sf.PersistenceScans)
		fields["spot_futures_profit_transfer_enabled"] = strconv.FormatBool(sf.ProfitTransferEnabled)
		fields["spot_futures_separate_acct_max_usdt"] = strconv.FormatFloat(sf.SeparateAcctMaxUSDT, 'f', -1, 64)
		fields["spot_futures_unified_acct_max_usdt"] = strconv.FormatFloat(sf.UnifiedAcctMaxUSDT, 'f', -1, 64)
	}

	if err := s.db.SetConfigFields(fields); err != nil {
		s.log.Error("save config to redis: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{Error: "failed to persist config"})
		return
	}

	// Also persist to config.json so changes survive fresh installs.
	if err := s.cfg.SaveJSONWithExchangeSecretOverrides(exchangeSecretOverrides); err != nil {
		s.log.Warn("save config.json: %v (Redis saved OK)", err)
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: snapshot})
}

// handleGetRejections returns the in-memory list of recently rejected opportunities.
func (s *Server) handleGetRejections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.rejStore == nil {
		writeJSON(w, http.StatusOK, Response{OK: true, Data: []struct{}{}})
		return
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: s.rejStore.GetAll()})
}

// exchangeInfo is the JSON representation of an enabled exchange.
type exchangeInfo struct {
	Name        string  `json:"name"`
	Balance     float64 `json:"balance"`
	SpotBalance float64 `json:"spot_balance"`
	AccountType string  `json:"account_type"` // "unified" or "separate"
}

// unifiedExchanges are exchanges with unified account models where
// spot and futures share the same balance pool.
// Gate.io is conditionally unified based on runtime detection.
// NOTE: OKX is NOT unified here — despite "unified account mode", OKX
// still separates funding (type 6) and trading (type 18) accounts for
// balances below ~10 000 USDT. The dashboard must show both.
var unifiedExchanges = map[string]bool{
	"bybit": true,
}

// isUnifiedExchange checks if an exchange uses a unified account model.
// For Gate.io, checks runtime detection via the adapter. If detection
// failed (no unified API permission), falls back to true because the
// balance refresh overrides with spot equity — showing both would double-count.
func (s *Server) isUnifiedExchange(name string) bool {
	if unifiedExchanges[name] {
		return true
	}
	if name == "gateio" {
		if exch, ok := s.exchanges[name]; ok {
			type unifiedChecker interface{ IsUnified() bool }
			if uc, ok := exch.(unifiedChecker); ok {
				return uc.IsUnified()
			}
		}
		// Detection unavailable — balance refresh overrides with spot,
		// so display as unified to avoid showing the same number twice.
		return true
	}
	return false
}

// handleGetExchanges returns the list of enabled exchanges with cached balances.
func (s *Server) handleGetExchanges(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	names := s.cfg.EnabledExchanges()
	exchanges := make([]exchangeInfo, 0, len(names))
	for _, name := range names {
		bal, _ := s.db.GetBalance(name)
		spotBal, _ := s.db.GetSpotBalance(name)
		acctType := "separate"
		if s.isUnifiedExchange(name) {
			acctType = "unified"
		}
		exchanges = append(exchanges, exchangeInfo{Name: name, Balance: bal, SpotBalance: spotBal, AccountType: acctType})
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: exchanges})
}

// ---------------------------------------------------------------------------
// Transfers
// ---------------------------------------------------------------------------

type transferRequest struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Coin   string `json:"coin"`
	Chain  string `json:"chain"`
	Amount string `json:"amount"`
}

// handleTransfer executes a withdrawal from one exchange to another.
func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req transferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Error: "invalid request body"})
		return
	}

	if req.From == "" || req.To == "" || req.Amount == "" || req.Chain == "" {
		writeJSON(w, http.StatusBadRequest, Response{Error: "from, to, chain, and amount are required"})
		return
	}
	if req.Coin == "" {
		req.Coin = "USDT"
	}

	// Look up destination address from config
	destAddrs, ok := s.cfg.ExchangeAddresses[req.To]
	if !ok {
		writeJSON(w, http.StatusBadRequest, Response{Error: fmt.Sprintf("no addresses configured for %s", req.To)})
		return
	}
	addr, ok := destAddrs[req.Chain]
	if !ok {
		writeJSON(w, http.StatusBadRequest, Response{Error: fmt.Sprintf("no %s address for %s", req.Chain, req.To)})
		return
	}

	// Look up source exchange adapter
	exc, ok := s.exchanges[req.From]
	if !ok {
		writeJSON(w, http.StatusBadRequest, Response{Error: fmt.Sprintf("exchange %s not available", req.From)})
		return
	}

	// Move funds to withdrawable account (futures→spot / unified→fund)
	if err := exc.TransferToSpot(req.Coin, req.Amount); err != nil {
		s.log.Warn("internal transfer %s: %v (may already be in spot)", req.From, err)
	}

	// Execute withdrawal
	result, err := exc.Withdraw(exchange.WithdrawParams{
		Coin:    req.Coin,
		Chain:   req.Chain,
		Address: addr,
		Amount:  req.Amount,
	})
	if err != nil {
		s.log.Error("transfer %s->%s failed: %v", req.From, req.To, err)
		writeJSON(w, http.StatusInternalServerError, Response{Error: fmt.Sprintf("withdrawal failed: %v", err)})
		return
	}

	// Save transfer record
	record := &database.TransferRecord{
		ID:        fmt.Sprintf("tx-%d", time.Now().UnixNano()),
		From:      req.From,
		To:        req.To,
		Coin:      req.Coin,
		Chain:     req.Chain,
		Amount:    req.Amount,
		Fee:       result.Fee,
		TxID:      result.TxID,
		Status:    result.Status,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.db.SaveTransfer(record); err != nil {
		s.log.Error("save transfer record: %v", err)
	}

	s.log.Info("transfer %s %s %s from %s to %s: txID=%s", req.Amount, req.Coin, req.Chain, req.From, req.To, result.TxID)
	writeJSON(w, http.StatusOK, Response{OK: true, Data: record})
}

// handleGetTransfers returns transfer history.
func (s *Server) handleGetTransfers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 50
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			limit = n
		}
	}

	transfers, err := s.db.GetTransfers(limit)
	if err != nil {
		s.log.Error("get transfers: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{Error: "failed to fetch transfers"})
		return
	}

	if transfers == nil {
		writeJSON(w, http.StatusOK, Response{OK: true, Data: []interface{}{}})
		return
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: transfers})
}

// handleGetLogs returns recent log entries from the log file.
func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 200
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			if n > 500 {
				n = 500
			}
			limit = n
		}
	}

	utils.FlushLog()
	entries := utils.TailLogFile(limit)
	if entries == nil {
		writeJSON(w, http.StatusOK, Response{OK: true, Data: []interface{}{}})
		return
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: entries})
}

// handleAddresses routes GET and POST to the appropriate handler.
func (s *Server) handleAddresses(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetAddresses(w, r)
	case http.MethodPost, http.MethodPut:
		s.handlePostAddresses(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetAddresses returns configured deposit addresses for all exchanges.
func (s *Server) handleGetAddresses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, Response{OK: true, Data: s.cfg.ExchangeAddresses})
}

// handlePostAddresses updates deposit addresses for exchanges.
// Expects: { "binance": { "APT": "0x...", "BEP20": "0x..." }, ... }
func (s *Server) handlePostAddresses(w http.ResponseWriter, r *http.Request) {
	var updates map[string]map[string]string
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Error: "invalid JSON"})
		return
	}

	if s.cfg.ExchangeAddresses == nil {
		s.cfg.ExchangeAddresses = make(map[string]map[string]string)
	}

	for exch, chains := range updates {
		if s.cfg.ExchangeAddresses[exch] == nil {
			s.cfg.ExchangeAddresses[exch] = make(map[string]string)
		}
		for chain, addr := range chains {
			if addr == "" {
				delete(s.cfg.ExchangeAddresses[exch], chain)
			} else {
				s.cfg.ExchangeAddresses[exch][chain] = addr
			}
		}
	}

	if err := s.cfg.SaveJSON(); err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Error: "failed to save: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: s.cfg.ExchangeAddresses})
}

// handleDiagnose collects error/warn logs + positions, sends to AI for analysis.
func (s *Server) handleDiagnose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.cfg.AIEndpoint == "" || s.cfg.AIAPIKey == "" {
		writeJSON(w, http.StatusBadRequest, Response{Error: "AI endpoint or API key not configured"})
		return
	}

	// Collect context data.
	utils.FlushLog()
	allLogs := utils.TailLogFile(500)
	var errorLogs []string
	for _, entry := range allLogs {
		if entry.Level == "ERROR" || entry.Level == "WARN" {
			errorLogs = append(errorLogs, fmt.Sprintf("%s [%s] [%s] %s", entry.Timestamp, entry.Level, entry.Module, entry.Message))
		}
	}
	if len(errorLogs) > 100 {
		errorLogs = errorLogs[len(errorLogs)-100:]
	}

	positions, _ := s.db.GetActivePositions()
	history, _ := s.db.GetHistory(10)

	// Build exchange balances.
	var balances []map[string]interface{}
	for _, name := range s.cfg.EnabledExchanges() {
		bal, _ := s.db.GetBalance(name)
		spotBal, _ := s.db.GetSpotBalance(name)
		balances = append(balances, map[string]interface{}{
			"exchange":     name,
			"balance":      bal,
			"spot_balance": spotBal,
		})
	}

	posJSON, _ := json.MarshalIndent(positions, "", "  ")
	histJSON, _ := json.MarshalIndent(history, "", "  ")
	balJSON, _ := json.MarshalIndent(balances, "", "  ")

	prompt := fmt.Sprintf(`You are analyzing a funding rate arbitrage bot. Review the following data and identify any issues, anomalies, or concerns.

## Recent Error/Warning Logs
%s

## Active Positions
%s

## Recent Trade History (last 10)
%s

## Exchange Balances
%s

請用繁體中文回覆。提供簡明的分析，重點關注：需要注意的錯誤、持倉健康狀況、任何失敗模式，以及建議的行動。`,
		strings.Join(errorLogs, "\n"),
		string(posJSON),
		string(histJSON),
		string(balJSON),
	)

	// Call AI API.
	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":      s.cfg.AIModel,
		"max_tokens": s.cfg.AIMaxTokens,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
	})

	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("POST", s.cfg.AIEndpoint, strings.NewReader(string(reqBody)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Error: fmt.Sprintf("build request: %v", err)})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.cfg.AIAPIKey)

	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, Response{Error: fmt.Sprintf("AI request failed: %v", err)})
		return
	}
	defer resp.Body.Close()

	var aiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		// OpenAI-style fallback
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&aiResp); err != nil {
		writeJSON(w, http.StatusBadGateway, Response{Error: fmt.Sprintf("parse AI response: %v", err)})
		return
	}

	// Extract text from content blocks, skipping thinking blocks.
	var analysis string
	for _, block := range aiResp.Content {
		if block.Type == "text" && block.Text != "" {
			analysis = block.Text
			break
		}
	}
	if analysis == "" {
		for _, choice := range aiResp.Choices {
			if choice.Message.Content != "" {
				analysis = choice.Message.Content
				break
			}
		}
	}
	if analysis == "" {
		writeJSON(w, http.StatusBadGateway, Response{Error: "empty AI response"})
		return
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]string{"analysis": analysis}})
}

// handleGetPermissions returns the startup API key permission check results.
func (s *Server) handleGetPermissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: s.permissions})
}

// handleCheckUpdate compares local VERSION with remote origin/main:VERSION.
func (s *Server) handleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentVersion := strings.TrimSpace(readFileString("VERSION"))
	if currentVersion == "" {
		currentVersion = "unknown"
	}

	// git fetch origin main
	cmd := exec.Command("git", "fetch", "origin", "main", "--quiet")
	cmd.Dir = workingDir()
	if err := cmd.Run(); err != nil {
		writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]interface{}{
			"currentVersion": currentVersion,
			"latestVersion":  currentVersion,
			"hasUpdate":      false,
			"error":          fmt.Sprintf("git fetch failed: %v", err),
		}})
		return
	}

	// Read remote VERSION
	latestVersion := currentVersion
	cmd = exec.Command("git", "show", "origin/main:VERSION")
	cmd.Dir = workingDir()
	if out, err := cmd.Output(); err == nil {
		latestVersion = strings.TrimSpace(string(out))
	}

	// Read remote CHANGELOG.md (first 3000 chars)
	changelog := ""
	cmd = exec.Command("git", "show", "origin/main:CHANGELOG.md")
	cmd.Dir = workingDir()
	if out, err := cmd.Output(); err == nil {
		cl := string(out)
		if len(cl) > 3000 {
			cl = cl[:3000]
		}
		changelog = cl
	}

	hasUpdate := versionNewer(latestVersion, currentVersion)

	// Structured runtime drift assessment (ARB-87).
	prov := runtimeProvenance()

	writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]interface{}{
		"currentVersion": currentVersion,
		"latestVersion":  latestVersion,
		"hasUpdate":      hasUpdate,
		"binaryDrift":    prov.DriftDetected,
		"runtime":        prov,
		"changelog":      changelog,
	}})
}

// versionNewer returns true when remote is a newer semver than local.
// Both are expected in "major.minor.patch" format (e.g. "0.22.30").
// Returns false on parse errors or when local >= remote.
func versionNewer(remote, local string) bool {
	parse := func(v string) (int, int, int, bool) {
		parts := strings.SplitN(strings.TrimSpace(v), ".", 3)
		if len(parts) != 3 {
			return 0, 0, 0, false
		}
		maj, e1 := strconv.Atoi(parts[0])
		min, e2 := strconv.Atoi(parts[1])
		pat, e3 := strconv.Atoi(parts[2])
		if e1 != nil || e2 != nil || e3 != nil {
			return 0, 0, 0, false
		}
		return maj, min, pat, true
	}
	rMaj, rMin, rPat, rok := parse(remote)
	lMaj, lMin, lPat, lok := parse(local)
	if !rok || !lok {
		return remote != local // fallback to inequality if unparseable
	}
	if rMaj != lMaj {
		return rMaj > lMaj
	}
	if rMin != lMin {
		return rMin > lMin
	}
	return rPat > lPat
}

// handleUpdate downloads the latest GitHub Release binary via pull-release.sh and restarts.
func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dir := workingDir()
	var steps []string

	// Step 1: run pull-release.sh to download latest release binary
	script := filepath.Join(dir, "scripts", "pull-release.sh")
	steps = append(steps, "> scripts/pull-release.sh")
	cmd := exec.Command(script)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		steps = append(steps, string(out))
		steps = append(steps, fmt.Sprintf("ERROR: %v", err))
		writeJSON(w, http.StatusInternalServerError, Response{OK: false, Error: strings.Join(steps, "\n")})
		return
	} else {
		steps = append(steps, strings.TrimSpace(string(out)))
	}

	// Step 2: update VERSION file from remote
	cmd = exec.Command("git", "fetch", "origin", "main", "--quiet")
	cmd.Dir = dir
	_ = cmd.Run()
	cmd = exec.Command("git", "show", "origin/main:VERSION")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		newVersion := strings.TrimSpace(string(out))
		os.WriteFile(filepath.Join(dir, "VERSION"), []byte(newVersion+"\n"), 0644)
	}

	newVersion := strings.TrimSpace(readFileString("VERSION"))
	steps = append(steps, fmt.Sprintf("\n更新完成: v%s", newVersion))
	steps = append(steps, "2 秒後自動重啟...")

	s.log.Info("update complete: v%s, restarting in 2s...", newVersion)
	writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]string{"output": strings.Join(steps, "\n")}})

	// Exit after 2s — systemd (Restart=on-failure) will respawn with the new binary.
	go func() {
		time.Sleep(2 * time.Second)
		os.Exit(1)
	}()
}

// workingDir returns the current working directory.
func workingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

// readFileString reads a file and returns its content as string.
func readFileString(name string) string {
	data, err := os.ReadFile(name)
	if err != nil {
		return ""
	}
	return string(data)
}
