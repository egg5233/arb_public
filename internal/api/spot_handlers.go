package api

import (
	"net/http"
	"strconv"

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

// SetSpotOpportunities updates the cached spot opportunities for the GET endpoint.
func (s *Server) SetSpotOpportunities(opps []interface{}) {
	s.spotOpps = opps
}
