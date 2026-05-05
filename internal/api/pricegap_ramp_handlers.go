// Package api — REST surface for the live-capital ramp + daily reconcile
// (PG-LIVE-01 / PG-LIVE-03).
//
// Routes (registered in server.go alongside the existing /api/pg/* surface):
//
//	GET  /api/pg/ramp                     — RampState + size/ceiling sidecar + live_capital flag
//	POST /api/pg/ramp/reset               — typed-phrase guarded reset
//	POST /api/pg/ramp/force-promote       — typed-phrase guarded promotion
//	POST /api/pg/ramp/force-demote        — typed-phrase guarded demotion
//	GET  /api/pg/reconcile/{date}         — typed DailyReconcileRecord (404 when day not reconciled)
//	POST /api/pg/reconcile/{date}/run     — run daily reconcile for a UTC date
//
// All routes inherit Bearer-token auth middleware; POST routes are also marked
// mutating so passwordless deployments reject them.
//
// T-14-15 mitigation: handlePgReconcileDay validates the {date} path param
// against `^\d{4}-\d{2}-\d{2}$` regex BEFORE forwarding to LoadRecord. Anything
// non-conforming returns 400. Mirrors the pg-admin reconcile run validation.
//
// T-14-18 mitigation: live_capital flag is read from cfg server-side (not
// from a client-supplied value) so the dashboard badge is server-authoritative.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// pgReconcileDateRegex defines the allowed shape for the {date} path param.
// Matches the cmdReconcileRun shape (Plan 14-04). Note: the regex permits
// 99-99 month/day combinations — the reconciler returns 404 (record absent)
// for those, but the date path is at minimum guarded against traversal
// (e.g. "../etc/passwd") which is the actual T-14-15 concern.
var pgReconcileDateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type pgRampActionRequest struct {
	ConfirmationPhrase string `json:"confirmation_phrase"`
	Reason             string `json:"reason"`
	Operator           string `json:"operator,omitempty"`
}

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

func (s *Server) handlePgRampReset(w http.ResponseWriter, r *http.Request) {
	s.handlePgRampAction(w, r, "RESET-RAMP", "reset", func(operator, reason string) error {
		return s.pgRamp.Reset(operator, reason)
	})
}

func (s *Server) handlePgRampForcePromote(w http.ResponseWriter, r *http.Request) {
	s.handlePgRampAction(w, r, "FORCE-PROMOTE", "force_promote", func(operator, reason string) error {
		return s.pgRamp.ForcePromote(operator, reason)
	})
}

func (s *Server) handlePgRampForceDemote(w http.ResponseWriter, r *http.Request) {
	s.handlePgRampAction(w, r, "FORCE-DEMOTE", "force_demote", func(operator, reason string) error {
		return s.pgRamp.ForceDemote(operator, reason)
	})
}

func (s *Server) handlePgRampAction(w http.ResponseWriter, r *http.Request, phrase, action string, fn func(operator, reason string) error) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.pgRamp == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Error: "ramp controller not configured"})
		return
	}
	var body pgRampActionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Error: "invalid request body"})
		return
	}
	if body.ConfirmationPhrase != phrase {
		writeJSON(w, http.StatusBadRequest, Response{Error: "confirmation_phrase must equal '" + phrase + "'"})
		return
	}
	if strings.TrimSpace(body.Reason) == "" {
		writeJSON(w, http.StatusBadRequest, Response{Error: "reason required"})
		return
	}
	if len(body.Reason) > priceGapReasonMaxBytes {
		writeJSON(w, http.StatusBadRequest, Response{Error: "reason too long"})
		return
	}
	operator := strings.TrimSpace(body.Operator)
	if operator == "" {
		operator = "dashboard"
	}
	reason := sanitizeReason(body.Reason)
	if err := fn(operator, reason); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "already at") {
			status = http.StatusConflict
		}
		writeJSON(w, status, Response{Error: err.Error()})
		return
	}
	snap := s.pgRamp.Snapshot()
	writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]interface{}{
		"action": action,
		"state":  snap,
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

func (s *Server) handlePgReconcileRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.pgReconciler == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Error: "reconciler not configured"})
		return
	}
	date := r.PathValue("date")
	if err := validatePgReconcileDate(date); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Error: err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	_, existed, _ := s.pgReconciler.LoadRecord(ctx, date)
	if err := s.pgReconciler.RunForDate(ctx, date); err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Error: err.Error()})
		return
	}
	rec, exists, err := s.pgReconciler.LoadRecord(ctx, date)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Error: err.Error()})
		return
	}
	data := map[string]interface{}{
		"date":            date,
		"already_existed": existed,
	}
	if exists {
		data["record"] = rec
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: data})
}

func validatePgReconcileDate(date string) error {
	if !pgReconcileDateRegex.MatchString(date) {
		return errInvalidPgReconcileDate
	}
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return errInvalidPgReconcileDate
	}
	if parsed.After(time.Now().UTC().AddDate(0, 0, 1)) {
		return errFuturePgReconcileDate
	}
	return nil
}

var (
	errInvalidPgReconcileDate = apiError("invalid date format (expect YYYY-MM-DD)")
	errFuturePgReconcileDate  = apiError("invalid future date")
)

type apiError string

func (e apiError) Error() string { return string(e) }
