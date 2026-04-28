// Package api — pricegap_discovery_handlers.go: Phase 11 / Plan 11-05 REST
// surface for the auto-discovery scanner.
//
// Routes (registered in server.go alongside the existing pricegap surface):
//
//	GET /api/pg/discovery/state             — full state envelope (D-17 REST seed)
//	GET /api/pg/discovery/scores/{symbol}   — per-symbol score history (7d ZSET)
//
// Both routes inherit the existing Bearer-token auth middleware. Plan 06 UI
// fetches both at mount time and consumes WS pg_scan_* events for live updates.
//
// Module boundary: handlers call telemetry.GetState / telemetry.GetScores —
// the read path; no Redis access happens directly in this file. The Telemetry
// helper is the same instance Plan 04's Scanner writes through, guaranteeing
// the seed and the WS stream are coherent.
package api

import (
	"net/http"
	"strings"

	"arb/internal/pricegaptrader"
)

// extractScoresSymbol pulls {symbol} from a path shaped like
// /api/pg/discovery/scores/<symbol>. Returns the upper-cased segment so
// callers can validate against the canonical regex without doing the
// upper-case conversion themselves.
func extractScoresSymbol(path string) string {
	const prefix = "/api/pg/discovery/scores/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	// Trim a trailing slash so /scores/BTCUSDT/ also works.
	rest = strings.TrimSuffix(rest, "/")
	// Reject paths with a sub-segment.
	if strings.Contains(rest, "/") {
		return ""
	}
	return strings.ToUpper(rest)
}

// handlePgDiscoveryState — GET /api/pg/discovery/state.
//
// Returns the StateResponse envelope (Plan 05 / 06 UI-SPEC). Empty / never-run
// state returns 200 with all-zero counters and `enabled=false` so the UI can
// render its empty-state widget without a special 404 path.
func (s *Server) handlePgDiscoveryState(w http.ResponseWriter, r *http.Request) {
	if s.telemetry == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Error: "discovery telemetry unavailable"})
		return
	}
	state, err := s.telemetry.GetState(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: state})
}

// handlePgDiscoveryScores — GET /api/pg/discovery/scores/{symbol}.
//
// Validates the path symbol against pricegaptrader.ValidateSymbol (T-11-33
// mitigation: bounded regex prevents Redis-key injection / pathological
// length). Unknown symbols return 200 + empty points array (the UI shows
// "no score history yet"); only the canonical-form check produces 400.
func (s *Server) handlePgDiscoveryScores(w http.ResponseWriter, r *http.Request) {
	if s.telemetry == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Error: "discovery telemetry unavailable"})
		return
	}
	symbol := extractScoresSymbol(r.URL.Path)
	if err := pricegaptrader.ValidateSymbol(symbol); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Error: "invalid symbol format: " + err.Error()})
		return
	}
	scores, err := s.telemetry.GetScores(r.Context(), symbol)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: scores})
}
