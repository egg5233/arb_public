package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"arb/internal/models"
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

	writeJSON(w, http.StatusOK, Response{OK: true, Data: stats})
}

// handleGetSpotOpportunities returns the latest spot-futures opportunities.
func (s *Server) handleGetSpotOpportunities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opps := s.spotOpps
	if opps == nil {
		writeJSON(w, http.StatusOK, Response{OK: true, Data: []interface{}{}})
		return
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
	s.spotOpps = opps
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
	netYieldAPR := pos.FundingAPR - pos.CurrentBorrowAPR
	var negativeYieldMin float64
	if pos.NegativeYieldSince != nil {
		negativeYieldMin = time.Since(*pos.NegativeYieldSince).Minutes()
	}

	health := map[string]interface{}{
		"position_id":        pos.ID,
		"symbol":             pos.Symbol,
		"exchange":           pos.Exchange,
		"direction":          pos.Direction,
		"current_borrow_apr": pos.CurrentBorrowAPR,
		"funding_apr":        pos.FundingAPR,
		"net_yield_apr":      netYieldAPR,
		"borrow_cost_accrued": pos.BorrowCostAccrued,
		"hours_open":         hoursOpen,
		"negative_yield":     pos.NegativeYieldSince != nil,
		"negative_yield_min": negativeYieldMin,
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
		} else if strings.Contains(errMsg, "dry run") {
			writeJSON(w, http.StatusUnprocessableEntity, Response{Error: errMsg})
		} else {
			writeJSON(w, http.StatusInternalServerError, Response{Error: errMsg})
		}
		return
	}

	writeJSON(w, http.StatusOK, Response{OK: true})
}
