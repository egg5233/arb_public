// Package api — pricegap_ramp_handlers.go: Phase 14 / Plan 14-05 REST
// surface for the live-capital ramp + daily reconcile (PG-LIVE-01 / PG-LIVE-03).
//
// Routes (registered in server.go alongside the existing /api/pg/* surface):
//
//	GET /api/pg/ramp                      — 5-field RampState + size/ceiling sidecar + live_capital flag
//	GET /api/pg/reconcile/{date}          — typed DailyReconcileRecord (404 when day not reconciled)
//
// Both routes inherit the Bearer-token auth middleware. The Pricegap-tracker
// dashboard tab calls them on mount to render the read-only Ramp + Reconcile
// widget.
//
// D-14 (read-only): NO mutators surface here. Force-promote / force-demote /
// reset / reconcile run live in pg-admin CLI only this phase. Phase 16
// (PG-OPS-09) will absorb the widget into the new top-level Pricegap tab and
// MAY introduce mutators behind a separate auth layer.
//
// T-14-15 mitigation: handlePgReconcileDay validates the {date} path param
// against `^\d{4}-\d{2}-\d{2}$` regex BEFORE forwarding to LoadRecord. Anything
// non-conforming returns 400. Mirrors the pg-admin reconcile run validation.
//
// T-14-18 mitigation: live_capital flag is read from cfg server-side (not
// from a client-supplied value) so the dashboard badge is server-authoritative.
package api

import (
	"net/http"
	"regexp"
)

// pgReconcileDateRegex defines the allowed shape for the {date} path param.
// Matches the cmdReconcileRun shape (Plan 14-04). Note: the regex permits
// 99-99 month/day combinations — the reconciler returns 404 (record absent)
// for those, but the date path is at minimum guarded against traversal
// (e.g. "../etc/passwd") which is the actual T-14-15 concern.
var pgReconcileDateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// handlePgRampState — GET /api/pg/ramp.
//
// Returns the 5-field RampState snapshot plus a sidecar of the per-stage
// sizing config + hard ceiling so the dashboard can render the full ramp
// context without a separate /api/config call. The live_capital flag is
// server-authoritative — reads cfg.PriceGapLiveCapital, never trusts a
// client-side cached value (T-14-18).
//
// Response envelope: { ok: true, data: {current_stage, clean_day_counter,
// last_eval_ts, last_loss_day_ts, demote_count, live_capital,
// stage_1_size_usdt, stage_2_size_usdt, stage_3_size_usdt, hard_ceiling_usdt,
// clean_days_to_promote} }
//
// 503 when the ramp controller is not wired (unit tests, paper-mode-only
// deployments) — the dashboard widget renders a "Loading…" empty state in
// that case.
func (s *Server) handlePgRampState(w http.ResponseWriter, r *http.Request) {
	if s.pgRamp == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Error: "ramp controller not configured"})
		return
	}
	snap := s.pgRamp.Snapshot()

	// Server-authoritative live-capital flag (NOT from any client config
	// cache). cfg may be nil in unit tests that don't exercise this path —
	// degrade gracefully to live_capital=false rather than panicking.
	var (
		liveCapital         bool
		stage1, stage2, st3 float64
		ceiling             float64
		cleanDaysToPromote  int
	)
	if s.cfg != nil {
		liveCapital = s.cfg.PriceGapLiveCapital
		stage1 = s.cfg.PriceGapStage1SizeUSDT
		stage2 = s.cfg.PriceGapStage2SizeUSDT
		st3 = s.cfg.PriceGapStage3SizeUSDT
		ceiling = s.cfg.PriceGapHardCeilingUSDT
		cleanDaysToPromote = s.cfg.PriceGapCleanDaysToPromote
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]interface{}{
		"current_stage":         snap.CurrentStage,
		"clean_day_counter":     snap.CleanDayCounter,
		"last_eval_ts":          snap.LastEvalTs,
		"last_loss_day_ts":      snap.LastLossDayTs,
		"demote_count":          snap.DemoteCount,
		"live_capital":          liveCapital,
		"stage_1_size_usdt":     stage1,
		"stage_2_size_usdt":     stage2,
		"stage_3_size_usdt":     st3,
		"hard_ceiling_usdt":     ceiling,
		"clean_days_to_promote": cleanDaysToPromote,
	}})
}

// handlePgReconcileDay — GET /api/pg/reconcile/{date}.
//
// Returns the typed DailyReconcileRecord for the requested UTC date when
// pg:reconcile:daily:{date} exists. 404 with {ok:false, error:...} when the
// day has not been reconciled. 400 on invalid date format (T-14-15).
//
// 503 when the reconciler is not wired (unit tests, paper-mode-only
// deployments) — same shape as /api/pg/ramp.
func (s *Server) handlePgReconcileDay(w http.ResponseWriter, r *http.Request) {
	if s.pgReconciler == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Error: "reconciler not configured"})
		return
	}
	date := r.PathValue("date")
	if !pgReconcileDateRegex.MatchString(date) {
		writeJSON(w, http.StatusBadRequest, Response{Error: "invalid date format (expect YYYY-MM-DD)"})
		return
	}
	rec, exists, err := s.pgReconciler.LoadRecord(r.Context(), date)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Error: err.Error()})
		return
	}
	if !exists {
		writeJSON(w, http.StatusNotFound, Response{Error: "reconcile not found for date"})
		return
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: rec})
}
