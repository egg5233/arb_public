package api

import (
	"net/http"
	"strconv"
	"time"

	"arb/internal/analytics"
	"arb/internal/models"
)

// handleGetAnalyticsPnLHistory returns time-series PnL snapshots.
// Query params: from (unix seconds), to (unix seconds), strategy (perp|spot|all)
func (s *Server) handleGetAnalyticsPnLHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.analyticsStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Error: "analytics not available"})
		return
	}

	from, _ := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
	to, _ := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
	strategy := r.URL.Query().Get("strategy")

	if to == 0 {
		to = time.Now().Unix()
	}
	if from == 0 {
		from = to - 30*86400 // default 30 days
	}
	if strategy == "" {
		strategy = "all"
	}

	snapshots, err := s.analyticsStore.GetPnLHistory(from, to, strategy)
	if err != nil {
		s.log.Error("analytics pnl-history: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{Error: "analytics query failed"})
		return
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: snapshots})
}

// handleGetAnalyticsSummary returns strategy summaries and exchange metrics.
// Query params: from (unix seconds), to (unix seconds)
func (s *Server) handleGetAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get perp history from Redis
	perps, err := s.db.GetHistory(500)
	if err != nil {
		s.log.Error("analytics summary perp history: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{Error: "failed to fetch perp history"})
		return
	}
	// Get spot history from Redis
	spots, err := s.db.GetSpotHistory(500)
	if err != nil {
		s.log.Error("analytics summary spot history: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{Error: "failed to fetch spot history"})
		return
	}

	// Apply time filter if provided
	from, _ := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
	to, _ := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
	if to == 0 {
		to = time.Now().Unix()
	}
	if from == 0 {
		from = to - 30*86400
	}
	fromT := time.Unix(from, 0)
	toT := time.Unix(to, 0)

	// Filter perps by close time (UpdatedAt for closed positions)
	var filteredPerps []*models.ArbitragePosition
	for _, p := range perps {
		if p.UpdatedAt.After(fromT) && p.UpdatedAt.Before(toT) {
			filteredPerps = append(filteredPerps, p)
		}
	}
	var filteredSpots []*models.SpotFuturesPosition
	for _, p := range spots {
		if p.UpdatedAt.After(fromT) && p.UpdatedAt.Before(toT) {
			filteredSpots = append(filteredSpots, p)
		}
	}

	type summaryResponse struct {
		Strategies      []analytics.StrategySummary `json:"strategies"`
		ExchangeMetrics []analytics.ExchangeMetric  `json:"exchange_metrics"`
	}

	resp := summaryResponse{
		Strategies:      analytics.ComputeStrategySummary(filteredPerps, filteredSpots),
		ExchangeMetrics: analytics.ComputeExchangeMetrics(filteredPerps, filteredSpots),
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: resp})
}
