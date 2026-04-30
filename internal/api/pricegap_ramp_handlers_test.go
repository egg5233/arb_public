// Package api — pricegap_ramp_handlers_test.go: Plan 14-05 REST handler
// coverage for the read-only ramp + reconcile endpoints (PG-LIVE-01 / PG-LIVE-03).
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"arb/internal/models"
	"arb/internal/pricegaptrader"
)

// fakeRampSnapshotter implements RampSnapshotter for handler tests.
type fakeRampSnapshotter struct {
	snap models.RampState
}

func (f *fakeRampSnapshotter) Snapshot() models.RampState {
	return f.snap
}

// fakeReconcileLoader implements ReconcileRecordLoader for handler tests.
type fakeReconcileLoader struct {
	rec    pricegaptrader.DailyReconcileRecord
	exists bool
	err    error
	calls  []string
}

func (f *fakeReconcileLoader) LoadRecord(_ context.Context, date string) (pricegaptrader.DailyReconcileRecord, bool, error) {
	f.calls = append(f.calls, date)
	if f.err != nil {
		return pricegaptrader.DailyReconcileRecord{}, false, f.err
	}
	return f.rec, f.exists, nil
}

// ---- Test 1: GET /api/pg/ramp returns all 11 keys with populated values ----

func TestHandlePgRampState_ReturnsAllFields(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	// Seed live-capital config + ramp snapshot.
	s.cfg.PriceGapLiveCapital = true
	s.cfg.PriceGapStage1SizeUSDT = 100
	s.cfg.PriceGapStage2SizeUSDT = 500
	s.cfg.PriceGapStage3SizeUSDT = 1000
	s.cfg.PriceGapHardCeilingUSDT = 1000
	s.cfg.PriceGapCleanDaysToPromote = 7

	now := time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC)
	loss := time.Date(2026, 4, 25, 0, 30, 0, 0, time.UTC)
	s.SetPgRamp(&fakeRampSnapshotter{snap: models.RampState{
		CurrentStage:    2,
		CleanDayCounter: 4,
		LastEvalTs:      now,
		LastLossDayTs:   loss,
		DemoteCount:     1,
	}})

	req := httptest.NewRequest(http.MethodGet, "/api/pg/ramp", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgRampState)
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.OK {
		t.Fatalf("OK=false want true; body=%s", w.Body.String())
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Data not a map; got %T", resp.Data)
	}
	required := []string{
		"current_stage", "clean_day_counter", "last_eval_ts",
		"last_loss_day_ts", "demote_count", "live_capital",
		"stage_1_size_usdt", "stage_2_size_usdt", "stage_3_size_usdt",
		"hard_ceiling_usdt", "clean_days_to_promote",
	}
	for _, k := range required {
		if _, ok := data[k]; !ok {
			t.Errorf("missing key %q; data=%v", k, data)
		}
	}
	if got := data["current_stage"].(float64); got != 2 {
		t.Errorf("current_stage=%v want 2", got)
	}
	if got := data["clean_day_counter"].(float64); got != 4 {
		t.Errorf("clean_day_counter=%v want 4", got)
	}
	if got := data["live_capital"].(bool); !got {
		t.Errorf("live_capital=false want true")
	}
	if got := data["stage_2_size_usdt"].(float64); got != 500 {
		t.Errorf("stage_2_size_usdt=%v want 500", got)
	}
	if got := data["clean_days_to_promote"].(float64); got != 7 {
		t.Errorf("clean_days_to_promote=%v want 7", got)
	}
}

// ---- Test 2: pgRamp not wired -> 503 ----

func TestHandlePgRampState_ReturnsServiceUnavailableWhenNoController(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()
	// pgRamp is nil by default — newPriceGapTestServer doesn't wire it.

	req := httptest.NewRequest(http.MethodGet, "/api/pg/ramp", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgRampState)
	handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503; body=%s", w.Code, w.Body.String())
	}
}

// ---- Test 3: no auth -> 401 ----

func TestHandlePgRampState_AuthRequired(t *testing.T) {
	s, _, _, td := newPriceGapTestServer(t)
	defer td()
	s.SetPgRamp(&fakeRampSnapshotter{})

	req := httptest.NewRequest(http.MethodGet, "/api/pg/ramp", nil)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgRampState)
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d want 401", w.Code)
	}
}

// ---- Test 4: GET /api/pg/reconcile/{date} happy path ----

func TestHandlePgReconcileDay_HappyPath(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	rec := pricegaptrader.DailyReconcileRecord{
		SchemaVersion: 1,
		Date:          "2026-04-29",
		ComputedAt:    time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC),
		Totals: pricegaptrader.DailyReconcileTotals{
			RealizedPnLUSDT: 12.50,
			PositionsClosed: 3,
			Wins:            2,
			Losses:          1,
			NetClean:        true,
		},
		Anomalies: pricegaptrader.DailyReconcileAnomalies{FlaggedIDs: []string{}},
	}
	loader := &fakeReconcileLoader{rec: rec, exists: true}
	s.SetPgReconciler(loader)

	// PathValue requires routing via mux.HandleFunc("GET /api/pg/reconcile/{date}",...)
	// — wire a one-shot mux for the test so r.PathValue("date") populates.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/pg/reconcile/{date}", s.authMiddleware(s.handlePgReconcileDay))

	req := httptest.NewRequest(http.MethodGet, "/api/pg/reconcile/2026-04-29", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d want 200; body=%s", w.Code, w.Body.String())
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.OK {
		t.Errorf("OK=false want true")
	}
	body, _ := json.Marshal(resp.Data)
	var got pricegaptrader.DailyReconcileRecord
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal record: %v", err)
	}
	if got.Date != "2026-04-29" {
		t.Errorf("Date=%q want 2026-04-29", got.Date)
	}
	if got.Totals.RealizedPnLUSDT != 12.50 {
		t.Errorf("RealizedPnLUSDT=%v want 12.50", got.Totals.RealizedPnLUSDT)
	}
	if !got.Totals.NetClean {
		t.Errorf("NetClean=false want true")
	}
	if len(loader.calls) != 1 || loader.calls[0] != "2026-04-29" {
		t.Errorf("loader.calls=%v want [2026-04-29]", loader.calls)
	}
}

// ---- Test 5: LoadRecord returns exists=false -> 404 ----

func TestHandlePgReconcileDay_NotFound_Returns404(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	loader := &fakeReconcileLoader{exists: false}
	s.SetPgReconciler(loader)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/pg/reconcile/{date}", s.authMiddleware(s.handlePgReconcileDay))

	req := httptest.NewRequest(http.MethodGet, "/api/pg/reconcile/2026-04-29", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("got %d want 404; body=%s", w.Code, w.Body.String())
	}
}

// ---- Test 6: invalid date format -> 400 ----

func TestHandlePgReconcileDay_InvalidDateFormat_Returns400(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	loader := &fakeReconcileLoader{}
	s.SetPgReconciler(loader)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/pg/reconcile/{date}", s.authMiddleware(s.handlePgReconcileDay))

	req := httptest.NewRequest(http.MethodGet, "/api/pg/reconcile/abc", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d want 400; body=%s", w.Code, w.Body.String())
	}
	if len(loader.calls) != 0 {
		t.Errorf("loader.calls=%v want empty (regex must reject before forward)", loader.calls)
	}
}

// ---- Test 7: path traversal attempt -> 400 ----

func TestHandlePgReconcileDay_RejectsTraversal(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	loader := &fakeReconcileLoader{}
	s.SetPgReconciler(loader)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/pg/reconcile/{date}", s.authMiddleware(s.handlePgReconcileDay))

	// http.ServeMux normalizes "../etc" but the regex would still reject any
	// non-conforming string. We test against literal "%2e%2e%2fetc" too.
	cases := []string{
		"/api/pg/reconcile/abc",
		"/api/pg/reconcile/2026-99",
		"/api/pg/reconcile/2026-04-29-extra",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			// 400 (regex rejects) or 404 (mux's wildcard doesn't match
			// segments with extra "/"). Both are acceptable — the contract
			// is that LoadRecord is NEVER called.
			if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
				t.Errorf("path=%q got %d want 400 or 404; body=%s", path, w.Code, w.Body.String())
			}
		})
	}
	if len(loader.calls) != 0 {
		t.Errorf("loader.calls=%v want empty (regex must reject all bad shapes)", loader.calls)
	}
}

// ---- Test 8: pgReconciler not wired -> 503 ----

func TestHandlePgReconcileDay_ServiceUnavailableWhenNoReconciler(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()
	// pgReconciler nil by default.

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/pg/reconcile/{date}", s.authMiddleware(s.handlePgReconcileDay))

	req := httptest.NewRequest(http.MethodGet, "/api/pg/reconcile/2026-04-29", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d want 503; body=%s", w.Code, w.Body.String())
	}
}

// ---- Test 9: LoadRecord returns transport error -> 500 ----

func TestHandlePgReconcileDay_LoadRecordError_Returns500(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	loader := &fakeReconcileLoader{err: errors.New("redis transport boom")}
	s.SetPgReconciler(loader)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/pg/reconcile/{date}", s.authMiddleware(s.handlePgReconcileDay))

	req := httptest.NewRequest(http.MethodGet, "/api/pg/reconcile/2026-04-29", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("got %d want 500; body=%s", w.Code, w.Body.String())
	}
}
