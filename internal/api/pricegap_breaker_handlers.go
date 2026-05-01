// Package api — pricegap_breaker_handlers.go: Phase 15 Plan 15-04 REST
// surface for the drawdown circuit breaker (PG-LIVE-02).
//
// Routes (registered in server.go alongside the existing /api/pg/* surface):
//
//	GET  /api/pg/breaker/state      — read-only Snapshot + last-trip + tripped flag
//	POST /api/pg/breaker/recover    — operator-driven recovery (typed-phrase guarded)
//	POST /api/pg/breaker/test-fire  — synthetic fire (typed-phrase guarded)
//
// All three routes inherit the Bearer-token auth middleware (T-15-12). The
// two POST routes additionally enforce a typed `confirmation_phrase` body
// field — exact case-sensitive match against "RECOVER" / "TEST-FIRE" per
// CONTEXT D-12 (T-15-13 mitigation: replayed shell-history requests fail).
//
// Server-side guards (T-15-16):
//   - POST /recover when sticky=0 → 409 Conflict ("breaker not tripped").
//   - POST /test-fire when sticky != 0 → 409 Conflict ("breaker already tripped — recover first").
//
// Operator name on POST endpoints: extracted from token claims when available;
// falls back to "operator" for anonymous bearer tokens.
//
// 503 when the *BreakerControllerAPI is not wired (paper-mode-only / unit
// tests that don't exercise these routes). Mirrors /api/pg/ramp behavior.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"arb/internal/models"
)

// BreakerControllerAPI is the narrow read+write surface the api Server holds
// onto for the /api/pg/breaker/* routes. *pricegaptrader.BreakerController
// satisfies it via the Plan 15-03 Snapshot + Plan 15-04 TestFire/Recover
// methods. Module boundary: pricegaptrader does NOT import internal/api —
// the interface lives here, the concrete type is wired at cmd/main.go via
// SetPgBreaker (Plan 12-03 D-15 narrow-interface precedent).
type BreakerControllerAPI interface {
	Snapshot() (models.BreakerState, error)
	Recover(ctx context.Context, operator string) error
	TestFire(ctx context.Context, dryRun bool) (models.BreakerTripRecord, error)
}

// BreakerTripsReader — read-side surface for `GET /api/pg/breaker/state`'s
// last_trip field. *database.Client satisfies via LoadBreakerTripAt
// (Plan 15-04 helper).
type BreakerTripsReader interface {
	LoadBreakerTripAt(index int64) (models.BreakerTripRecord, bool, error)
}

// SetPgBreaker injects the Phase 15 Plan 15-04 BreakerController surface for
// the three /api/pg/breaker/* routes. Passing nil leaves the routes returning
// 503. cmd/main.go wires *pricegaptrader.BreakerController in production.
func (s *Server) SetPgBreaker(b BreakerControllerAPI) {
	s.pgBreaker = b
}

// SetPgBreakerTrips injects the Phase 15 Plan 15-04 trips reader for
// `GET /api/pg/breaker/state`'s last_trip sidecar. Passing nil simply omits
// the last_trip field; the rest of the route still works.
func (s *Server) SetPgBreakerTrips(r BreakerTripsReader) {
	s.pgBreakerTrips = r
}

// breakerRecoverRequest is the POST body for /api/pg/breaker/recover.
type breakerRecoverRequest struct {
	ConfirmationPhrase string `json:"confirmation_phrase"`
	Operator           string `json:"operator,omitempty"`
}

// breakerTestFireRequest is the POST body for /api/pg/breaker/test-fire.
type breakerTestFireRequest struct {
	ConfirmationPhrase string `json:"confirmation_phrase"`
	DryRun             bool   `json:"dry_run,omitempty"`
}

// handlePgBreakerStateGet — GET /api/pg/breaker/state. Returns the current
// BreakerState + most-recent trip + derived (armed, tripped) flags.
func (s *Server) handlePgBreakerStateGet(w http.ResponseWriter, r *http.Request) {
	if s.pgBreaker == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Error: "breaker controller not configured"})
		return
	}
	state, err := s.pgBreaker.Snapshot()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Error: err.Error()})
		return
	}

	enabled := false
	threshold := 0.0
	if s.cfg != nil {
		enabled = s.cfg.PriceGapBreakerEnabled
		threshold = s.cfg.PriceGapDrawdownLimitUSDT
	}
	tripped := state.PaperModeStickyUntil != 0
	armed := enabled && !tripped

	var lastTrip *models.BreakerTripRecord
	if s.pgBreakerTrips != nil {
		if rec, exists, lerr := s.pgBreakerTrips.LoadBreakerTripAt(0); lerr == nil && exists {
			recCopy := rec
			lastTrip = &recCopy
		}
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]interface{}{
		"enabled":              enabled,
		"pending_strike":       state.PendingStrike,
		"strike1_ts_ms":        state.Strike1Ts,
		"sticky_until_ms":      state.PaperModeStickyUntil,
		"last_eval_pnl_usdt":   state.LastEvalPnLUSDT,
		"last_eval_ts_ms":      state.LastEvalTs,
		"threshold_usdt":       threshold,
		"armed":                armed,
		"tripped":              tripped,
		"last_trip":            lastTrip,
	}})
}

// handlePgBreakerRecover — POST /api/pg/breaker/recover. Typed-phrase
// guarded; checks sticky != 0 before invoking Recover.
func (s *Server) handlePgBreakerRecover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.pgBreaker == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Error: "breaker controller not configured"})
		return
	}
	var body breakerRecoverRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Error: "invalid request body"})
		return
	}
	if body.ConfirmationPhrase != "RECOVER" {
		writeJSON(w, http.StatusBadRequest, Response{Error: "confirmation_phrase must equal 'RECOVER'"})
		return
	}
	// Guard: cannot recover when not tripped (T-15-16).
	state, err := s.pgBreaker.Snapshot()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Error: err.Error()})
		return
	}
	if state.PaperModeStickyUntil == 0 {
		writeJSON(w, http.StatusConflict, Response{Error: "breaker not tripped"})
		return
	}
	operator := body.Operator
	if operator == "" {
		operator = "operator"
	}
	if rerr := s.pgBreaker.Recover(r.Context(), operator); rerr != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Error: rerr.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]interface{}{
		"recovery_ts_ms": nowMs(),
		"operator":       operator,
	}})
}

// handlePgBreakerTestFire — POST /api/pg/breaker/test-fire. Typed-phrase
// guarded; rejects when already tripped (T-15-16: bounded recursion).
func (s *Server) handlePgBreakerTestFire(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.pgBreaker == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Error: "breaker controller not configured"})
		return
	}
	var body breakerTestFireRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Error: "invalid request body"})
		return
	}
	if body.ConfirmationPhrase != "TEST-FIRE" {
		writeJSON(w, http.StatusBadRequest, Response{Error: "confirmation_phrase must equal 'TEST-FIRE'"})
		return
	}
	// Guard: cannot test-fire when already tripped (T-15-16). Skip check on
	// dry-run because dry-run never mutates state.
	if !body.DryRun {
		state, err := s.pgBreaker.Snapshot()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, Response{Error: err.Error()})
			return
		}
		if state.PaperModeStickyUntil != 0 {
			writeJSON(w, http.StatusConflict, Response{Error: "breaker already tripped — recover first"})
			return
		}
	}
	rec, terr := s.pgBreaker.TestFire(r.Context(), body.DryRun)
	if terr != nil {
		writeJSON(w, http.StatusInternalServerError, Response{Error: terr.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]interface{}{
		"trip_pnl_usdt":  rec.TripPnLUSDT,
		"threshold_usdt": rec.Threshold,
		"source":         rec.Source,
		"dry_run":        body.DryRun,
		"trip_ts_ms":     rec.TripTs,
	}})
}

// nowMs returns current unix-ms. Tests do not monkey-patch — recovery_ts_ms
// is informational only (the controller's Recover path persists its own
// authoritative timestamp).
func nowMs() int64 {
	return time.Now().UnixMilli()
}
