// Package api — pricegap_handlers.go: HTTP surface for the Phase 9 Price-Gap
// dashboard. All routes are bearer-gated via s.cors(s.authMiddleware(...)).
//
// Routes:
//
//	GET  /api/pricegap/state                        — aggregate seed (flags + candidates + positions + history + metrics stub)
//	GET  /api/pricegap/candidates                   — candidate list with disable state
//	GET  /api/pricegap/positions                    — active positions only
//	GET  /api/pricegap/closed?offset=0&limit=100    — paged history (newest-first, max limit 500)
//	GET  /api/pricegap/metrics                      — per-candidate metrics (Plan 05 stub; empty slice here)
//	POST /api/pricegap/candidate/{symbol}/disable   — {"reason":"..."} → pg:candidate:disabled:<symbol>
//	POST /api/pricegap/candidate/{symbol}/enable    — DEL pg:candidate:disabled:<symbol>
//
// Disable/enable routes enforce symbol allowlist against cfg.PriceGapCandidates
// (T-09-07) and cap the reason string at 256 bytes with control chars stripped
// (T-09-08).
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"arb/internal/models"
	"arb/internal/pricegaptrader"
)

// priceGapCandidateView pairs a configured candidate with its current disable
// state for the dashboard REST seed.
type priceGapCandidateView struct {
	models.PriceGapCandidate
	Disabled   bool   `json:"disabled"`
	Reason     string `json:"reason,omitempty"`
	DisabledAt int64  `json:"disabled_at,omitempty"`
}

// priceGapCandidateMetrics is the per-candidate metrics row. Plan 05 fills in
// the computation; this plan ships an empty slice so the UI can render.
type priceGapCandidateMetrics struct {
	Symbol string `json:"symbol"`
}

// priceGapStateResponse is the aggregate seed payload.
type priceGapStateResponse struct {
	Enabled         bool                         `json:"enabled"`
	PaperMode       bool                         `json:"paper_mode"`
	Budget          float64                      `json:"budget"`
	Candidates      []priceGapCandidateView      `json:"candidates"`
	ActivePositions []*models.PriceGapPosition   `json:"active_positions"`
	RecentClosed    []*models.PriceGapPosition   `json:"recent_closed"`
	Metrics         []priceGapCandidateMetrics   `json:"metrics"`
}

// priceGapHistoryLimitDefault is the default page size for /api/pricegap/closed.
const (
	priceGapHistoryLimitDefault = 100
	priceGapHistoryLimitMax     = 500
	priceGapReasonMaxBytes      = 256
)

// buildCandidateViews enriches the configured candidates with disable state
// from Redis. Shared by /state and /candidates handlers.
func (s *Server) buildCandidateViews() []priceGapCandidateView {
	candidates := s.cfg.PriceGapCandidates
	out := make([]priceGapCandidateView, 0, len(candidates))
	for _, c := range candidates {
		view := priceGapCandidateView{PriceGapCandidate: c}
		if s.db != nil {
			if disabled, reason, disabledAt, err := s.db.IsCandidateDisabled(c.Symbol); err == nil && disabled {
				view.Disabled = true
				view.Reason = reason
				view.DisabledAt = disabledAt
			}
		}
		out = append(out, view)
	}
	return out
}

// handlePriceGapState returns the full dashboard seed aggregate.
func (s *Server) handlePriceGapState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	candidates := s.buildCandidateViews()

	var active []*models.PriceGapPosition
	if s.db != nil {
		if positions, err := s.db.GetActivePriceGapPositions(); err == nil {
			active = positions
		}
	}
	if active == nil {
		active = []*models.PriceGapPosition{}
	}

	var history []*models.PriceGapPosition
	if s.db != nil {
		if rows, err := s.db.GetPriceGapHistory(0, priceGapHistoryLimitDefault); err == nil {
			history = rows
		}
	}
	if history == nil {
		history = []*models.PriceGapPosition{}
	}

	resp := priceGapStateResponse{
		Enabled:         s.cfg.PriceGapEnabled,
		PaperMode:       s.cfg.PriceGapPaperMode,
		Budget:          s.cfg.PriceGapBudget,
		Candidates:      candidates,
		ActivePositions: active,
		RecentClosed:    history,
		Metrics:         []priceGapCandidateMetrics{}, // Plan 05 fills this in
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: resp})
}

// handlePriceGapCandidates returns candidates + disable state only.
func (s *Server) handlePriceGapCandidates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: s.buildCandidateViews()})
}

// handlePriceGapPositions returns active pricegap positions only.
func (s *Server) handlePriceGapPositions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var active []*models.PriceGapPosition
	if s.db != nil {
		if positions, err := s.db.GetActivePriceGapPositions(); err == nil {
			active = positions
		}
	}
	if active == nil {
		active = []*models.PriceGapPosition{}
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: active})
}

// handlePriceGapClosed returns closed pricegap positions with pagination.
// Query params: offset (default 0), limit (default 100, max 500 — T-09-12).
func (s *Server) handlePriceGapClosed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	offset := 0
	limit := priceGapHistoryLimitDefault
	if q := r.URL.Query().Get("offset"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n >= 0 {
			offset = n
		}
	}
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > priceGapHistoryLimitMax {
		limit = priceGapHistoryLimitMax
	}

	var history []*models.PriceGapPosition
	if s.db != nil {
		if rows, err := s.db.GetPriceGapHistory(offset, limit); err == nil {
			history = rows
		}
	}
	if history == nil {
		history = []*models.PriceGapPosition{}
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: history})
}

// handlePriceGapMetrics — stub per-candidate metrics endpoint. Plan 05
// replaces the body with the real computer; here we return an empty slice
// so the UI has a stable shape to bind to.
func (s *Server) handlePriceGapMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: []priceGapCandidateMetrics{}})
}

// priceGapDisableRequest is the optional POST body for the disable endpoint.
type priceGapDisableRequest struct {
	Reason string `json:"reason"`
}

// sanitizeReason enforces length cap (T-09-08) and strips control chars.
// Empty reason → "manual" (matches pg-admin default).
func sanitizeReason(raw string) string {
	r := strings.TrimSpace(raw)
	if r == "" {
		return "manual"
	}
	// Strip ASCII control characters (0x00–0x1F) to neutralise downstream
	// Telegram / log injection (T-09-08).
	var b strings.Builder
	b.Grow(len(r))
	for _, ch := range r {
		if ch < 0x20 {
			continue
		}
		b.WriteRune(ch)
	}
	return b.String()
}

// symbolAllowed checks the posted symbol against cfg.PriceGapCandidates.
// Prevents arbitrary Redis key writes (T-09-07).
func (s *Server) symbolAllowed(symbol string) bool {
	for _, c := range s.cfg.PriceGapCandidates {
		if strings.EqualFold(c.Symbol, symbol) {
			return true
		}
	}
	return false
}

// extractCandidateSymbol pulls {symbol} from a path shaped like
// /api/pricegap/candidate/<symbol>/<action>.
func extractCandidateSymbol(path string) string {
	const prefix = "/api/pricegap/candidate/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	// Strip action suffix ("/disable" or "/enable").
	if i := strings.LastIndex(rest, "/"); i >= 0 {
		rest = rest[:i]
	}
	return strings.ToUpper(rest)
}

// handlePriceGapCandidateDisable sets the disable flag for a candidate.
func (s *Server) handlePriceGapCandidateDisable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	symbol := extractCandidateSymbol(r.URL.Path)
	if symbol == "" {
		writeJSON(w, http.StatusBadRequest, Response{Error: "missing symbol"})
		return
	}
	if !s.symbolAllowed(symbol) {
		writeJSON(w, http.StatusBadRequest, Response{Error: fmt.Sprintf("unknown candidate symbol: %s", symbol)})
		return
	}

	var req priceGapDisableRequest
	// Empty body is OK — defaults to "manual".
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			writeJSON(w, http.StatusBadRequest, Response{Error: "invalid request body"})
			return
		}
	}

	// Input validation (ASVS V5): length cap BEFORE sanitization so we reject
	// raw oversize payloads rather than silently truncating.
	if len(req.Reason) > priceGapReasonMaxBytes {
		writeJSON(w, http.StatusBadRequest, Response{
			Error: fmt.Sprintf("reason exceeds %d bytes", priceGapReasonMaxBytes),
		})
		return
	}
	cleanedReason := sanitizeReason(req.Reason)

	if err := s.db.SetCandidateDisabled(symbol, cleanedReason); err != nil {
		s.log.Error("SetCandidateDisabled(%s): %v", symbol, err)
		writeJSON(w, http.StatusInternalServerError, Response{Error: "failed to disable candidate"})
		return
	}

	s.BroadcastPriceGapCandidateUpdate(pricegaptrader.PriceGapCandidateUpdate{
		Symbol:     symbol,
		Disabled:   true,
		Reason:     cleanedReason,
		DisabledAt: time.Now().Unix(),
	})
	writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]interface{}{
		"symbol":   symbol,
		"disabled": true,
		"reason":   cleanedReason,
	}})
}

// handlePriceGapCandidateEnable clears the disable flag for a candidate.
func (s *Server) handlePriceGapCandidateEnable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	symbol := extractCandidateSymbol(r.URL.Path)
	if symbol == "" {
		writeJSON(w, http.StatusBadRequest, Response{Error: "missing symbol"})
		return
	}
	if !s.symbolAllowed(symbol) {
		writeJSON(w, http.StatusBadRequest, Response{Error: fmt.Sprintf("unknown candidate symbol: %s", symbol)})
		return
	}

	if err := s.db.ClearCandidateDisabled(symbol); err != nil {
		s.log.Error("ClearCandidateDisabled(%s): %v", symbol, err)
		writeJSON(w, http.StatusInternalServerError, Response{Error: "failed to enable candidate"})
		return
	}

	s.BroadcastPriceGapCandidateUpdate(pricegaptrader.PriceGapCandidateUpdate{
		Symbol:   symbol,
		Disabled: false,
	})
	writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]interface{}{
		"symbol":   symbol,
		"disabled": false,
	}})
}

// ---------------------------------------------------------------------------
// Broadcaster implementation — satisfies pricegaptrader.Broadcaster from Plan 01.
// Plan 06 wires *Server into the tracker via one line in cmd/main.go.
// ---------------------------------------------------------------------------

// Compile-time assertion: *Server satisfies pricegaptrader.Broadcaster.
var _ pricegaptrader.Broadcaster = (*Server)(nil)

// BroadcastPriceGapPositions pushes the active positions slice to the dashboard
// WS clients under the "pg_positions" channel.
func (s *Server) BroadcastPriceGapPositions(positions []*models.PriceGapPosition) {
	s.hub.Broadcast("pg_positions", positions)
}

// BroadcastPriceGapEvent pushes a lifecycle event (entry/exit/auto_disable)
// to the dashboard WS clients under the "pg_event" channel.
func (s *Server) BroadcastPriceGapEvent(evt pricegaptrader.PriceGapEvent) {
	s.hub.Broadcast("pg_event", evt)
}

// BroadcastPriceGapCandidateUpdate pushes a candidate disable-state change to
// the dashboard WS clients under the "pg_candidate_update" channel.
func (s *Server) BroadcastPriceGapCandidateUpdate(upd pricegaptrader.PriceGapCandidateUpdate) {
	s.hub.Broadcast("pg_candidate_update", upd)
}
