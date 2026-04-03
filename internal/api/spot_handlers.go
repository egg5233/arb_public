package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// handleGetSpotPositions returns all active spot-futures positions.
func (s *Server) handleGetSpotPositions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	positions, err := s.db.GetActiveSpotPositions()
	if err != nil {
		s.log.Error("get active spot positions: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{Error: "failed to fetch spot positions"})
		return
	}

	if positions == nil {
		writeJSON(w, http.StatusOK, Response{OK: true, Data: []interface{}{}})
		return
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: positions})
}

// handleGetSpotHistory returns closed spot-futures positions.
func (s *Server) handleGetSpotHistory(w http.ResponseWriter, r *http.Request) {
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

	history, err := s.db.GetSpotHistory(limit)
	if err != nil {
		s.log.Error("get spot history: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{Error: "failed to fetch spot history"})
		return
	}

	if history == nil {
		writeJSON(w, http.StatusOK, Response{OK: true, Data: []interface{}{}})
		return
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: history})
}

// handleGetSpotStats returns win/loss/PnL stats for spot-futures.
func (s *Server) handleGetSpotStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := s.db.GetSpotStats()
	if err != nil {
		s.log.Error("get spot stats: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{Error: "failed to fetch spot stats"})
		return
	}

	// Normalize missing fields to zero for cold start / partial hash.
	for _, key := range []string{"total_pnl", "win_count", "loss_count", "trade_count"} {
		if _, ok := stats[key]; !ok {
			stats[key] = "0"
		}
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: stats})
}

// handleGetSpotOpportunities returns the latest spot-futures opportunities.
func (s *Server) handleGetSpotOpportunities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opps, _ := s.spotOpps.Load().([]interface{})
	if opps == nil {
		writeJSON(w, http.StatusOK, Response{OK: true, Data: []interface{}{}})
		return
	}

	// Cap to 100 entries to avoid overwhelming the dashboard.
	if len(opps) > 100 {
		opps = opps[:100]
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: opps})
}

// BroadcastSpotPositionUpdate sends a spot-futures position update to all
// WebSocket clients, following the same pattern as BroadcastPositionUpdate.
func (s *Server) BroadcastSpotPositionUpdate(pos *models.SpotFuturesPosition) {
	s.hub.Broadcast("spot_position_update", pos)

	positions, err := s.db.GetActiveSpotPositions()
	if err == nil {
		s.hub.Broadcast("spot_positions", positions)
	}
}

// BroadcastSpotHealth sends a position health/monitoring update to all WS clients.
func (s *Server) BroadcastSpotHealth(pos *models.SpotFuturesPosition) {
	s.hub.Broadcast("spot_position_health", pos)
}

// SetSpotOpportunities updates the cached spot opportunities for the GET endpoint.
func (s *Server) SetSpotOpportunities(opps []interface{}) {
	s.spotOpps.Store(opps)
}

// BroadcastSpotOpportunities sends spot-futures opportunities to all WS clients.
func (s *Server) BroadcastSpotOpportunities(opps []interface{}) {
	s.hub.Broadcast("spot_opportunities", opps)
}

// handleSpotPositionHealth returns health metrics for a single spot-futures position.
func (s *Server) handleSpotPositionHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, Response{Error: "position id required"})
		return
	}

	pos, err := s.db.GetSpotPosition(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, Response{Error: err.Error()})
		} else {
			writeJSON(w, http.StatusInternalServerError, Response{Error: "failed to fetch position"})
		}
		return
	}

	hoursOpen := time.Since(pos.CreatedAt).Hours()
	var negativeYieldMin float64
	if pos.NegativeYieldSince != nil {
		negativeYieldMin = time.Since(*pos.NegativeYieldSince).Minutes()
	}

	// Expose null instead of year-zero for unset borrow rate check.
	var lastBorrowRateCheck interface{}
	if !pos.LastBorrowRateCheck.IsZero() {
		lastBorrowRateCheck = pos.LastBorrowRateCheck
	}

	health := map[string]interface{}{
		"position_id":            pos.ID,
		"symbol":                 pos.Symbol,
		"exchange":               pos.Exchange,
		"direction":              pos.Direction,
		"current_borrow_apr":     pos.CurrentBorrowAPR,
		"funding_apr":            pos.FundingAPR,
		"fee_pct":                pos.FeePct,
		"current_funding_apr":    pos.CurrentFundingAPR,
		"current_fee_pct":        pos.CurrentFeePct,
		"current_net_yield_apr":  pos.CurrentNetYieldAPR,
		"yield_data_source":      pos.YieldDataSource,
		"yield_snapshot_at":      pos.YieldSnapshotAt,
		"borrow_cost_accrued":    pos.BorrowCostAccrued,
		"hours_open":             hoursOpen,
		"negative_yield":         pos.NegativeYieldSince != nil,
		"negative_yield_min":     negativeYieldMin,
		"last_borrow_rate_check": lastBorrowRateCheck,
		"negative_yield_since":   pos.NegativeYieldSince,
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: health})
}

// handleSpotManualOpen triggers a manual spot-futures entry.
func (s *Server) handleSpotManualOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Symbol    string `json:"symbol"`
		Exchange  string `json:"exchange"`
		Direction string `json:"direction"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Symbol == "" || req.Exchange == "" || req.Direction == "" {
		writeJSON(w, http.StatusBadRequest, Response{Error: "symbol, exchange, direction required"})
		return
	}

	if s.spotOpenPosition == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Error: "spot engine not available"})
		return
	}

	if err := s.spotOpenPosition(req.Symbol, req.Exchange, req.Direction); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			writeJSON(w, http.StatusNotFound, Response{Error: errMsg})
		} else if strings.Contains(errMsg, "already") || strings.Contains(errMsg, "capacity") {
			writeJSON(w, http.StatusConflict, Response{Error: errMsg})
		} else if strings.Contains(errMsg, "is filtered") {
			writeJSON(w, http.StatusUnprocessableEntity, Response{Error: errMsg})
		} else if strings.Contains(errMsg, "pending confirmation") {
			writeJSON(w, http.StatusAccepted, Response{OK: true, Data: map[string]string{"status": "pending", "message": errMsg}})
		} else if strings.Contains(errMsg, "dry run") {
			writeJSON(w, http.StatusUnprocessableEntity, Response{Error: errMsg})
		} else {
			writeJSON(w, http.StatusInternalServerError, Response{Error: errMsg})
		}
		return
	}

	writeJSON(w, http.StatusOK, Response{OK: true})
}

// handleSpotCheckPriceGap fetches live spot and futures BBOs for a one-off price gap check.
func (s *Server) handleSpotCheckPriceGap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Symbol    string `json:"symbol"`
		Exchange  string `json:"exchange"`
		Direction string `json:"direction"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Error: "invalid JSON"})
		return
	}

	req.Symbol = strings.ToUpper(strings.TrimSpace(req.Symbol))
	req.Exchange = strings.ToLower(strings.TrimSpace(req.Exchange))
	req.Direction = strings.TrimSpace(req.Direction)
	if req.Symbol == "" || req.Exchange == "" || req.Direction == "" {
		writeJSON(w, http.StatusBadRequest, Response{Error: "symbol, exchange, direction required"})
		return
	}

	futExch, ok := s.exchanges[req.Exchange]
	if !ok {
		writeJSON(w, http.StatusNotFound, Response{Error: "exchange not found"})
		return
	}
	spotExch, ok := futExch.(exchange.SpotMarginExchange)
	if !ok {
		writeJSON(w, http.StatusBadRequest, Response{Error: "exchange does not support spot margin"})
		return
	}

	futBBO, err := getFuturesBBOForAPI(futExch, req.Symbol)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Error: err.Error()})
		return
	}
	spotBBO, err := spotExch.GetSpotBBO(req.Symbol)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Error: err.Error()})
		return
	}

	var gapPct float64
	switch req.Direction {
	case "borrow_sell_long":
		if spotBBO.Bid <= 0 {
			writeJSON(w, http.StatusInternalServerError, Response{Error: "invalid spot bid"})
			return
		}
		gapPct = (futBBO.Ask - spotBBO.Bid) / spotBBO.Bid * 100
	case "buy_spot_short":
		if futBBO.Bid <= 0 {
			writeJSON(w, http.StatusInternalServerError, Response{Error: "invalid futures bid"})
			return
		}
		gapPct = (spotBBO.Ask - futBBO.Bid) / futBBO.Bid * 100
	default:
		writeJSON(w, http.StatusBadRequest, Response{Error: "invalid direction"})
		return
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]interface{}{
		"spot_bid":    spotBBO.Bid,
		"spot_ask":    spotBBO.Ask,
		"futures_bid": futBBO.Bid,
		"futures_ask": futBBO.Ask,
		"gap_pct":     gapPct,
		"direction":   req.Direction,
	}})
}

func getFuturesBBOForAPI(exch exchange.Exchange, symbol string) (exchange.BBO, error) {
	if bbo, ok := exch.GetBBO(symbol); ok {
		if bbo.Bid > 0 && bbo.Ask > 0 {
			return bbo, nil
		}
	}

	ob, err := exch.GetOrderbook(symbol, 5)
	if err != nil {
		return exchange.BBO{}, fmt.Errorf("GetOrderbook: %w", err)
	}
	if len(ob.Bids) == 0 || len(ob.Asks) == 0 {
		return exchange.BBO{}, fmt.Errorf("empty orderbook for %s", symbol)
	}
	bid := ob.Bids[0].Price
	ask := ob.Asks[0].Price
	if bid <= 0 || ask <= 0 {
		return exchange.BBO{}, fmt.Errorf("invalid orderbook prices for %s", symbol)
	}
	return exchange.BBO{Bid: bid, Ask: ask}, nil
}

// handleSpotTestInject injects synthetic test opportunities for lifecycle verification.
func (s *Server) handleSpotTestInject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Symbol   string `json:"symbol"`
		Exchange string `json:"exchange"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Symbol == "" || req.Exchange == "" {
		writeJSON(w, http.StatusBadRequest, Response{Error: "symbol and exchange required"})
		return
	}
	if s.spotInjectTestOpp == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Error: "spot engine not available"})
		return
	}
	s.spotInjectTestOpp(req.Symbol, req.Exchange)
	writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]string{
		"status": "injected",
		"symbol": strings.ToUpper(req.Symbol),
	}})
}

// handleSpotAutoConfig handles GET and POST for spot-futures auto-entry configuration.
// GET returns current auto-entry settings; POST updates them and persists to Redis.
func (s *Server) handleSpotAutoConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, Response{OK: true, Data: s.spotAutoConfigResponse()})

	case http.MethodPost:
		var req struct {
			Enabled               *bool    `json:"enabled"`
			DryRun                *bool    `json:"dry_run"`
			PersistenceScans      *int     `json:"persistence_scans"`
			NativeScannerEnabled  *bool    `json:"native_scanner_enabled"`
			EnableMinHold         *bool    `json:"enable_min_hold"`
			MinHoldHours          *int     `json:"min_hold_hours"`
			EnableSettlementGuard *bool    `json:"enable_settlement_guard"`
			SettlementWindowMin   *int     `json:"settlement_window_min"`
			EnablePriceGapGate    *bool    `json:"enable_price_gap_gate"`
			MaxPriceGapPct        *float64 `json:"max_price_gap_pct"`
			EnableExitSpreadGate  *bool    `json:"enable_exit_spread_gate"`
			ExitSpreadPct         *float64 `json:"exit_spread_pct"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, Response{Error: "invalid JSON"})
			return
		}

		s.cfg.Lock()
		if req.Enabled != nil {
			s.cfg.SpotFuturesAutoEnabled = *req.Enabled
			s.db.SetConfigField("spot_futures_auto_enabled", strconv.FormatBool(*req.Enabled))
		}
		if req.DryRun != nil {
			s.cfg.SpotFuturesDryRun = *req.DryRun
			s.db.SetConfigField("spot_futures_dry_run", strconv.FormatBool(*req.DryRun))
		}
		if req.PersistenceScans != nil && *req.PersistenceScans >= 0 {
			s.cfg.SpotFuturesPersistenceScans = *req.PersistenceScans
			s.db.SetConfigField("spot_futures_persistence_scans", strconv.Itoa(*req.PersistenceScans))
		}
		if req.NativeScannerEnabled != nil {
			s.cfg.SpotFuturesNativeScannerEnabled = *req.NativeScannerEnabled
			s.db.SetConfigField("spot_futures_native_scanner_enabled", strconv.FormatBool(*req.NativeScannerEnabled))
		}
		if req.EnableMinHold != nil {
			s.cfg.SpotFuturesEnableMinHold = *req.EnableMinHold
			s.db.SetConfigField("spot_futures_enable_min_hold", strconv.FormatBool(*req.EnableMinHold))
		}
		if req.MinHoldHours != nil && *req.MinHoldHours > 0 {
			s.cfg.SpotFuturesMinHoldHours = *req.MinHoldHours
			s.db.SetConfigField("spot_futures_min_hold_hours", strconv.Itoa(*req.MinHoldHours))
		}
		if req.EnableSettlementGuard != nil {
			s.cfg.SpotFuturesEnableSettlementGuard = *req.EnableSettlementGuard
			s.db.SetConfigField("spot_futures_enable_settlement_guard", strconv.FormatBool(*req.EnableSettlementGuard))
		}
		if req.SettlementWindowMin != nil && *req.SettlementWindowMin > 0 {
			s.cfg.SpotFuturesSettlementWindowMin = *req.SettlementWindowMin
			s.db.SetConfigField("spot_futures_settlement_window_min", strconv.Itoa(*req.SettlementWindowMin))
		}
		if req.EnablePriceGapGate != nil {
			s.cfg.SpotFuturesEnablePriceGapGate = *req.EnablePriceGapGate
			s.db.SetConfigField("spot_futures_enable_price_gap_gate", strconv.FormatBool(*req.EnablePriceGapGate))
		}
		if req.MaxPriceGapPct != nil && *req.MaxPriceGapPct > 0 {
			s.cfg.SpotFuturesMaxPriceGapPct = *req.MaxPriceGapPct
			s.db.SetConfigField("spot_futures_max_price_gap_pct", fmt.Sprintf("%.4f", *req.MaxPriceGapPct))
		}
		if req.EnableExitSpreadGate != nil {
			s.cfg.SpotFuturesEnableExitSpreadGate = *req.EnableExitSpreadGate
			s.db.SetConfigField("spot_futures_enable_exit_spread_gate", strconv.FormatBool(*req.EnableExitSpreadGate))
		}
		if req.ExitSpreadPct != nil && *req.ExitSpreadPct > 0 {
			s.cfg.SpotFuturesExitSpreadPct = *req.ExitSpreadPct
			s.db.SetConfigField("spot_futures_exit_spread_pct", fmt.Sprintf("%.4f", *req.ExitSpreadPct))
		}
		s.cfg.Unlock()

		s.log.Info("spot auto config updated: enabled=%v dry_run=%v persistence_scans=%d",
			s.cfg.SpotFuturesAutoEnabled, s.cfg.SpotFuturesDryRun, s.cfg.SpotFuturesPersistenceScans)

		writeJSON(w, http.StatusOK, Response{OK: true, Data: s.spotAutoConfigResponse()})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSpotManualClose triggers a manual close of a spot-futures position.
func (s *Server) handleSpotManualClose(w http.ResponseWriter, r *http.Request) {
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

	if s.spotClosePosition == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Error: "spot engine not available"})
		return
	}

	if err := s.spotClosePosition(req.PositionID); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			writeJSON(w, http.StatusNotFound, Response{Error: errMsg})
		} else if strings.Contains(errMsg, "not active") || strings.Contains(errMsg, "already exiting") {
			writeJSON(w, http.StatusConflict, Response{Error: errMsg})
		} else {
			writeJSON(w, http.StatusInternalServerError, Response{Error: errMsg})
		}
		return
	}

	writeJSON(w, http.StatusOK, Response{OK: true})
}

// spotAutoConfigResponse builds the shared response for GET and POST /api/spot/config/auto.
func (s *Server) spotAutoConfigResponse() map[string]interface{} {
	return map[string]interface{}{
		"auto_enabled":            s.cfg.SpotFuturesAutoEnabled,
		"dry_run":                 s.cfg.SpotFuturesDryRun,
		"persistence_scans":       s.cfg.SpotFuturesPersistenceScans,
		"max_positions":           s.cfg.SpotFuturesMaxPositions,
		"capital_separate_usdt":   s.cfg.SpotFuturesCapitalSeparate,
		"capital_unified_usdt":    s.cfg.SpotFuturesCapitalUnified,
		"native_scanner_enabled":  s.cfg.SpotFuturesNativeScannerEnabled,
		"enable_min_hold":         s.cfg.SpotFuturesEnableMinHold,
		"min_hold_hours":          s.cfg.SpotFuturesMinHoldHours,
		"enable_settlement_guard": s.cfg.SpotFuturesEnableSettlementGuard,
		"settlement_window_min":   s.cfg.SpotFuturesSettlementWindowMin,
		"enable_price_gap_gate":   s.cfg.SpotFuturesEnablePriceGapGate,
		"max_price_gap_pct":       s.cfg.SpotFuturesMaxPriceGapPct,
		"enable_exit_spread_gate": s.cfg.SpotFuturesEnableExitSpreadGate,
		"exit_spread_pct":         s.cfg.SpotFuturesExitSpreadPct,
	}
}

// ---------------------------------------------------------------------------
// Lifecycle test handler
// ---------------------------------------------------------------------------

// lifecycleStepResult records the outcome of a single open/close step.
type lifecycleStepResult struct {
	Status string `json:"status"` // "ok" or "error"
	Error  string `json:"error"`
}

// lifecycleFuturesResult records the futures-position verification outcome.
type lifecycleFuturesResult struct {
	Found bool    `json:"found"`
	Side  string  `json:"side"`
	Size  float64 `json:"size"`
}

// lifecycleBorrowResult records the margin-borrowed verification outcome.
type lifecycleBorrowResult struct {
	Found  bool    `json:"found"`
	Amount float64 `json:"amount"`
}

// lifecycleSpotResult records the spot-balance verification outcome.
type lifecycleSpotResult struct {
	Found  bool    `json:"found"`
	Amount float64 `json:"amount"`
}

// lifecycleDirAResult holds the full Dir A verification report.
type lifecycleDirAResult struct {
	Open        *lifecycleStepResult   `json:"open"`
	VerifyOpen  *lifecycleDirAOpenVfy  `json:"verify_open,omitempty"`
	Close       *lifecycleStepResult   `json:"close,omitempty"`
	VerifyClose *lifecycleDirACloseVfy `json:"verify_close,omitempty"`
}

type lifecycleDirAOpenVfy struct {
	Futures  lifecycleFuturesResult `json:"futures"`
	Borrowed lifecycleBorrowResult  `json:"borrowed"`
}

type lifecycleDirACloseVfy struct {
	Futures  lifecycleFuturesResult `json:"futures"`
	Borrowed lifecycleBorrowResult  `json:"borrowed"`
}

// lifecycleDirBResult holds the full Dir B verification report.
type lifecycleDirBResult struct {
	Open        *lifecycleStepResult   `json:"open"`
	VerifyOpen  *lifecycleDirBOpenVfy  `json:"verify_open,omitempty"`
	Close       *lifecycleStepResult   `json:"close,omitempty"`
	VerifyClose *lifecycleDirBCloseVfy `json:"verify_close,omitempty"`
}

type lifecycleDirBOpenVfy struct {
	Futures     lifecycleFuturesResult `json:"futures"`
	SpotBalance lifecycleSpotResult    `json:"spot_balance"`
}

type lifecycleDirBCloseVfy struct {
	Futures     lifecycleFuturesResult `json:"futures"`
	SpotBalance lifecycleSpotResult    `json:"spot_balance"`
}

// lifecycleReport is the top-level response payload.
type lifecycleReport struct {
	Exchange string              `json:"exchange"`
	Symbol   string              `json:"symbol"`
	DirA     lifecycleDirAResult `json:"dir_a"`
	DirB     lifecycleDirBResult `json:"dir_b"`
}

// handleSpotTestLifecycle automates Dir A + Dir B lifecycle testing for a given
// exchange and symbol, verifying exchange state after each open and close step.
//
// POST /api/spot/test-lifecycle
// Body: {"symbol":"BTCUSDT","exchange":"bitget"}
func (s *Server) handleSpotTestLifecycle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Symbol   string `json:"symbol"`
		Exchange string `json:"exchange"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Symbol == "" || req.Exchange == "" {
		writeJSON(w, http.StatusBadRequest, Response{Error: "symbol and exchange required"})
		return
	}

	if s.spotInjectTestOpp == nil || s.spotOpenPosition == nil || s.spotClosePosition == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Error: "spot engine not available"})
		return
	}

	exch, ok := s.exchanges[req.Exchange]
	if !ok {
		writeJSON(w, http.StatusBadRequest, Response{Error: "unknown exchange: " + req.Exchange})
		return
	}

	symbol := strings.ToUpper(req.Symbol)
	baseCoin := strings.TrimSuffix(symbol, "USDT")
	report := lifecycleReport{
		Exchange: req.Exchange,
		Symbol:   symbol,
	}

	// Helper: find futures position for symbol with the given holdSide.
	findFuturesPos := func(holdSide string) lifecycleFuturesResult {
		res := lifecycleFuturesResult{}
		positions, err := exch.GetAllPositions()
		if err != nil {
			return res
		}
		for _, p := range positions {
			if strings.EqualFold(p.Symbol, symbol) && strings.EqualFold(p.HoldSide, holdSide) {
				size, _ := utils.ParseFloat(p.Total)
				if size > 0 {
					res.Found = true
					res.Side = strings.ToLower(holdSide)
					res.Size = size
					return res
				}
			}
		}
		return res
	}

	// Helper: check futures position is gone (size=0 or absent).
	checkFuturesClosed := func() lifecycleFuturesResult {
		res := lifecycleFuturesResult{}
		positions, err := exch.GetAllPositions()
		if err != nil {
			return res
		}
		for _, p := range positions {
			if strings.EqualFold(p.Symbol, symbol) {
				size, _ := utils.ParseFloat(p.Total)
				if size > 0 {
					res.Found = true
					res.Side = strings.ToLower(p.HoldSide)
					res.Size = size
					return res
				}
			}
		}
		return res // Found=false means closed
	}

	// Helper: get margin borrow amount.
	getMarginBorrowed := func() lifecycleBorrowResult {
		res := lifecycleBorrowResult{}
		marginExch, ok := exch.(exchange.SpotMarginExchange)
		if !ok {
			return res
		}
		bal, err := marginExch.GetMarginBalance(baseCoin)
		if err != nil {
			return res
		}
		if bal.Borrowed > 0 {
			res.Found = true
			res.Amount = bal.Borrowed
		}
		return res
	}

	// Helper: get spot/margin base-coin total balance.
	getSpotBalance := func() lifecycleSpotResult {
		res := lifecycleSpotResult{}
		marginExch, ok := exch.(exchange.SpotMarginExchange)
		if !ok {
			return res
		}
		bal, err := marginExch.GetMarginBalance(baseCoin)
		if err != nil {
			return res
		}
		if bal.TotalBalance > 0 {
			res.Found = true
			res.Amount = bal.TotalBalance
		}
		return res
	}

	// Helper: find active spot position for this symbol+exchange.
	findActivePosition := func() string {
		positions, err := s.db.GetActiveSpotPositions()
		if err != nil {
			return ""
		}
		for _, p := range positions {
			if strings.EqualFold(p.Symbol, symbol) && strings.EqualFold(p.Exchange, req.Exchange) {
				return p.ID
			}
		}
		return ""
	}

	// -----------------------------------------------------------------------
	// Dir A: borrow_sell_long
	// -----------------------------------------------------------------------
	s.log.Info("lifecycle test Dir A: inject opp %s/%s", symbol, req.Exchange)
	s.spotInjectTestOpp(symbol, req.Exchange)

	openStepA := &lifecycleStepResult{}
	if err := s.spotOpenPosition(symbol, req.Exchange, "borrow_sell_long"); err != nil {
		openStepA.Status = "error"
		openStepA.Error = err.Error()
		report.DirA.Open = openStepA
		s.log.Error("lifecycle test Dir A open failed: %v", err)
	} else {
		openStepA.Status = "ok"
		report.DirA.Open = openStepA

		time.Sleep(5 * time.Second)

		// Verify open
		report.DirA.VerifyOpen = &lifecycleDirAOpenVfy{
			Futures:  findFuturesPos("long"),
			Borrowed: getMarginBorrowed(),
		}

		// Find and close
		posID := findActivePosition()
		if posID == "" {
			report.DirA.Close = &lifecycleStepResult{Status: "error", Error: "position not found in DB after open"}
		} else {
			closeStepA := &lifecycleStepResult{}
			if err := s.spotClosePosition(posID); err != nil {
				closeStepA.Status = "error"
				closeStepA.Error = err.Error()
				s.log.Error("lifecycle test Dir A close failed: %v", err)
			} else {
				closeStepA.Status = "ok"
			}
			report.DirA.Close = closeStepA

			time.Sleep(5 * time.Second)

			// Verify close
			report.DirA.VerifyClose = &lifecycleDirACloseVfy{
				Futures:  checkFuturesClosed(),
				Borrowed: getMarginBorrowed(),
			}
		}
	}

	// -----------------------------------------------------------------------
	// Dir B: buy_spot_short
	// -----------------------------------------------------------------------
	s.log.Info("lifecycle test Dir B: inject opp %s/%s", symbol, req.Exchange)
	s.spotInjectTestOpp(symbol, req.Exchange)

	openStepB := &lifecycleStepResult{}
	if err := s.spotOpenPosition(symbol, req.Exchange, "buy_spot_short"); err != nil {
		openStepB.Status = "error"
		openStepB.Error = err.Error()
		report.DirB.Open = openStepB
		s.log.Error("lifecycle test Dir B open failed: %v", err)
	} else {
		openStepB.Status = "ok"
		report.DirB.Open = openStepB

		time.Sleep(5 * time.Second)

		// Verify open
		report.DirB.VerifyOpen = &lifecycleDirBOpenVfy{
			Futures:     findFuturesPos("short"),
			SpotBalance: getSpotBalance(),
		}

		// Find and close
		posID := findActivePosition()
		if posID == "" {
			report.DirB.Close = &lifecycleStepResult{Status: "error", Error: "position not found in DB after open"}
		} else {
			closeStepB := &lifecycleStepResult{}
			if err := s.spotClosePosition(posID); err != nil {
				closeStepB.Status = "error"
				closeStepB.Error = err.Error()
				s.log.Error("lifecycle test Dir B close failed: %v", err)
			} else {
				closeStepB.Status = "ok"
			}
			report.DirB.Close = closeStepB

			time.Sleep(5 * time.Second)

			// Verify close
			report.DirB.VerifyClose = &lifecycleDirBCloseVfy{
				Futures:     checkFuturesClosed(),
				SpotBalance: getSpotBalance(),
			}
		}
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: report})
}
