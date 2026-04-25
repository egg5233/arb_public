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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"arb/internal/models"
	"arb/internal/pricegaptrader"
)

// ---------------------------------------------------------------------------
// Phase 10 (Plan 10-01) — POST /api/config price_gap.candidates validation
// + active-position safety guard. Helpers live here so the handler-side apply
// block in handlers.go keeps to its existing shape.
// ---------------------------------------------------------------------------

// symbolRE matches the internal canonical symbol format (D-05).
var symbolRE = regexp.MustCompile(`^[A-Z0-9]+USDT$`)

// allowedExch is the closed enum of exchange identifiers the price-gap engine
// supports today (D-06). Phase 10 ships paper-only against the same 6 CEXes
// the rest of the platform talks to.
var allowedExch = map[string]struct{}{
	"binance": {},
	"bybit":   {},
	"gateio":  {},
	"bitget":  {},
	"okx":     {},
	"bingx":   {},
}

// validatePriceGapCandidates enforces D-05..D-11 invariants. Returns a slice
// of human-readable error strings, one per failed candidate (errors collated
// rather than fail-fast so the dashboard can surface every issue at once).
//
// Caller is expected to have already normalised Symbol/LongExch/ShortExch
// case + whitespace per Pitfall 3 so the validator + storage agree on the
// canonical form.
func validatePriceGapCandidates(cs []models.PriceGapCandidate) []string {
	var errs []string
	seen := make(map[string]struct{}, len(cs))
	for i, c := range cs {
		prefix := fmt.Sprintf("candidate[%d] %s/%s/%s", i, c.Symbol, c.LongExch, c.ShortExch)
		if !symbolRE.MatchString(c.Symbol) {
			errs = append(errs, prefix+": symbol must match ^[A-Z0-9]+USDT$")
		}
		if _, ok := allowedExch[c.LongExch]; !ok {
			errs = append(errs, prefix+": long_exch invalid")
		}
		if _, ok := allowedExch[c.ShortExch]; !ok {
			errs = append(errs, prefix+": short_exch invalid")
		}
		if c.LongExch == c.ShortExch {
			errs = append(errs, prefix+": long_exch must differ from short_exch")
		}
		if c.ThresholdBps < 50 || c.ThresholdBps > 1000 {
			errs = append(errs, prefix+": threshold_bps out of range [50,1000]")
		}
		if c.MaxPositionUSDT < 100 || c.MaxPositionUSDT > 50000 {
			errs = append(errs, prefix+": max_position_usdt out of range [100,50000]")
		}
		if c.ModeledSlippageBps < 0 || c.ModeledSlippageBps > 100 {
			errs = append(errs, prefix+": modeled_slippage_bps out of range [0,100]")
		}
		key := c.Symbol + "|" + c.LongExch + "|" + c.ShortExch
		if _, dup := seen[key]; dup {
			errs = append(errs, prefix+": duplicate tuple")
		}
		seen[key] = struct{}{}
	}
	return errs
}

// guardActivePositionRemoval blocks the apply path when any tuple present in
// `prev` but absent from `next` has an open position in pg:positions:active
// (D-14, Pitfall 4 — covers both outright delete AND tuple-change edits).
//
// Returns ("", nil) when the operation is safe to apply. Returns a
// human-readable blocker string + nil err for HTTP 409. Returns ("", err) for
// downstream Redis failures (HTTP 500).
func (s *Server) guardActivePositionRemoval(prev, next []models.PriceGapCandidate) (string, error) {
	nextSet := make(map[string]struct{}, len(next))
	for _, c := range next {
		nextSet[c.Symbol+"|"+c.LongExch+"|"+c.ShortExch] = struct{}{}
	}
	var removed []models.PriceGapCandidate
	for _, c := range prev {
		if _, kept := nextSet[c.Symbol+"|"+c.LongExch+"|"+c.ShortExch]; !kept {
			removed = append(removed, c)
		}
	}
	if len(removed) == 0 {
		return "", nil
	}
	if s.db == nil {
		// Tests without a DB — skip the active-position guard.
		return "", nil
	}
	active, err := s.db.GetActivePriceGapPositions()
	if err != nil {
		return "", err
	}
	for _, p := range active {
		for _, rm := range removed {
			if p.Symbol == rm.Symbol && p.LongExchange == rm.LongExch && p.ShortExchange == rm.ShortExch {
				return fmt.Sprintf("candidate %s/%s/%s has active position; close it first (position id=%s)",
					rm.Symbol, rm.LongExch, rm.ShortExch, p.ID), nil
			}
		}
	}
	return "", nil
}

// priceGapCandidateView pairs a configured candidate with its current disable
// state for the dashboard REST seed.
type priceGapCandidateView struct {
	models.PriceGapCandidate
	Disabled   bool   `json:"disabled"`
	Reason     string `json:"reason,omitempty"`
	DisabledAt int64  `json:"disabled_at,omitempty"`
}

// priceGapStateResponse is the aggregate seed payload.
//
// Metrics is the real per-candidate rolling-window aggregator output
// (pricegaptrader.CandidateMetrics, Plan 09-05). Padded with zero-activity rows
// for every configured candidate without history so the UI always renders a
// row per configured pair (UI-SPEC § Rolling Metrics table).
type priceGapStateResponse struct {
	Enabled         bool                              `json:"enabled"`
	PaperMode       bool                              `json:"paper_mode"`
	DebugLog        bool                              `json:"debug_log"` // Phase 9 gap-closure Gap #1 (Plan 09-09)
	Budget          float64                           `json:"budget"`
	Candidates      []priceGapCandidateView           `json:"candidates"`
	ActivePositions []*models.PriceGapPosition        `json:"active_positions"`
	RecentClosed    []*models.PriceGapPosition        `json:"recent_closed"`
	Metrics         []pricegaptrader.CandidateMetrics `json:"metrics"`
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

	metrics, _ := s.computeMetricsForResponse()

	resp := priceGapStateResponse{
		Enabled:         s.cfg.PriceGapEnabled,
		PaperMode:       s.cfg.PriceGapPaperMode,
		DebugLog:        s.cfg.PriceGapDebugLog,
		Budget:          s.cfg.PriceGapBudget,
		Candidates:      candidates,
		ActivePositions: active,
		RecentClosed:    history,
		Metrics:         metrics,
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: resp})
}

// computeMetricsForResponse reads the full pg:history window (capped at 500 by
// Phase 8 D-14, so T-09-22 DoS ceiling is preserved here) and returns
// per-candidate rolling metrics padded with zero-activity rows for every
// configured candidate that has no trade history. Shared by handlePriceGapState
// and handlePriceGapMetrics so the two responses cannot drift.
//
// Only reads pg:history: the D-23 write-path stamps every field the aggregator
// needs onto each history row (D-24 simplification — single-data-source rule).
func (s *Server) computeMetricsForResponse() ([]pricegaptrader.CandidateMetrics, error) {
	var history []*models.PriceGapPosition
	if s.db != nil {
		rows, err := s.db.GetPriceGapHistory(0, priceGapHistoryLimitMax)
		if err != nil {
			return padWithConfigCandidates(nil, s.cfg.PriceGapCandidates), err
		}
		history = rows
	}
	computed := pricegaptrader.ComputeCandidateMetrics(history, time.Now())
	return padWithConfigCandidates(computed, s.cfg.PriceGapCandidates), nil
}

// padWithConfigCandidates appends a zero-valued CandidateMetrics row for every
// entry in `configured` whose <symbol>_<long>_<short> key is not already
// present in `computed`. Final slice is re-sorted desc by Bps30dPerDay so
// populated rows rank above zero-activity rows.
func padWithConfigCandidates(
	computed []pricegaptrader.CandidateMetrics,
	configured []models.PriceGapCandidate,
) []pricegaptrader.CandidateMetrics {
	out := make([]pricegaptrader.CandidateMetrics, 0, len(computed)+len(configured))
	out = append(out, computed...)
	seen := make(map[string]struct{}, len(computed))
	for _, r := range computed {
		seen[r.Candidate] = struct{}{}
	}
	for _, c := range configured {
		key := c.Symbol + "_" + c.LongExch + "_" + c.ShortExch
		if _, ok := seen[key]; ok {
			continue
		}
		out = append(out, pricegaptrader.CandidateMetrics{
			Candidate:     key,
			Symbol:        c.Symbol,
			LongExchange:  c.LongExch,
			ShortExchange: c.ShortExch,
		})
		seen[key] = struct{}{}
	}
	// Re-sort so populated rows stay on top; stable tiebreak on Candidate key.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bps30dPerDay != out[j].Bps30dPerDay {
			return out[i].Bps30dPerDay > out[j].Bps30dPerDay
		}
		return out[i].Candidate < out[j].Candidate
	})
	return out
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

// handlePriceGapMetrics returns per-candidate rolling 24h / 7d / 30d metrics,
// padded with zero-activity rows for every configured candidate that has no
// trade history yet (UI-SPEC § Rolling Metrics table).
//
// Backed by pricegaptrader.ComputeCandidateMetrics — a pure function — reading
// pg:history via GetPriceGapHistory; no secondary store read (D-24).
func (s *Server) handlePriceGapMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	metrics, err := s.computeMetricsForResponse()
	if err != nil {
		s.log.Error("computeMetricsForResponse: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{Error: "failed to compute metrics"})
		return
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: metrics})
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
