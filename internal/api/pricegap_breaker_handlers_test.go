// Phase 15 Plan 15-04 — REST handler tests for /api/pg/breaker/* routes.
// Coverage: auth, typed-phrase enforcement, recover/test-fire happy paths,
// state GET, server-side guards (409 on tripped/not-tripped misuse).
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"arb/internal/models"
)

// fakeBreakerCtl satisfies BreakerControllerAPI for handler tests.
type fakeBreakerCtl struct {
	mu             sync.Mutex
	state          models.BreakerState
	snapshotErr    error
	recoverCalls   []string
	recoverErr     error
	testFireCalls  []bool
	testFireRec    models.BreakerTripRecord
	testFireErr    error
}

func (f *fakeBreakerCtl) Snapshot() (models.BreakerState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state, f.snapshotErr
}

func (f *fakeBreakerCtl) Recover(_ context.Context, operator string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recoverCalls = append(f.recoverCalls, operator)
	return f.recoverErr
}

func (f *fakeBreakerCtl) TestFire(_ context.Context, dryRun bool) (models.BreakerTripRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.testFireCalls = append(f.testFireCalls, dryRun)
	return f.testFireRec, f.testFireErr
}

// fakeBreakerTripsReader satisfies BreakerTripsReader for handler tests.
type fakeBreakerTripsReader struct {
	trips []models.BreakerTripRecord // index 0 = newest
}

func (f *fakeBreakerTripsReader) LoadBreakerTripAt(index int64) (models.BreakerTripRecord, bool, error) {
	if index < 0 || int(index) >= len(f.trips) {
		return models.BreakerTripRecord{}, false, nil
	}
	return f.trips[int(index)], true, nil
}

// ---- Test 1: GET /api/pg/breaker/state happy path -----------------------

func TestPgBreakerHandlers_StateGetEndpoint(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	s.cfg.PriceGapBreakerEnabled = true
	s.cfg.PriceGapDrawdownLimitUSDT = -50
	s.SetPgBreaker(&fakeBreakerCtl{state: models.BreakerState{
		PendingStrike:        1,
		Strike1Ts:            1234567890123,
		LastEvalTs:           1234567899999,
		LastEvalPnLUSDT:      -75.5,
		PaperModeStickyUntil: 0,
	}})
	s.SetPgBreakerTrips(&fakeBreakerTripsReader{trips: []models.BreakerTripRecord{
		{TripTs: 1234567890123, TripPnLUSDT: -100, Threshold: -50, Source: "live"},
	}})

	req := httptest.NewRequest(http.MethodGet, "/api/pg/breaker/state", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgBreakerStateGet)
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d want 200; body=%s", w.Code, w.Body.String())
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.OK {
		t.Fatalf("OK=false")
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Data not a map: %T", resp.Data)
	}
	for _, k := range []string{
		"enabled", "pending_strike", "strike1_ts_ms", "sticky_until_ms",
		"last_eval_pnl_usdt", "last_eval_ts_ms", "threshold_usdt",
		"armed", "tripped", "last_trip",
	} {
		if _, ok := data[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	if data["enabled"].(bool) != true {
		t.Errorf("enabled=%v want true", data["enabled"])
	}
	if data["armed"].(bool) != true {
		t.Errorf("armed=%v want true (sticky=0 + enabled)", data["armed"])
	}
	if data["tripped"].(bool) != false {
		t.Errorf("tripped=%v want false", data["tripped"])
	}
	if data["last_trip"] == nil {
		t.Errorf("last_trip should be populated when trips exist")
	}
}

// ---- Test 2: pgBreaker not wired -> 503 ----------------------------------

func TestPgBreakerHandlers_StateGet_503WhenUnwired(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()
	// pgBreaker nil by default.
	req := httptest.NewRequest(http.MethodGet, "/api/pg/breaker/state", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgBreakerStateGet)
	handler(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d want 503", w.Code)
	}
}

// ---- Test 3: Auth required on all 3 routes ------------------------------

func TestPgBreakerHandlers_AuthRequired(t *testing.T) {
	s, _, _, td := newPriceGapTestServer(t)
	defer td()
	s.SetPgBreaker(&fakeBreakerCtl{})

	// GET /state without bearer.
	req := httptest.NewRequest(http.MethodGet, "/api/pg/breaker/state", nil)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePgBreakerStateGet)
	handler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET /state no-auth: got %d want 401", w.Code)
	}

	// POST /recover without bearer.
	body := bytes.NewBufferString(`{"confirmation_phrase":"RECOVER"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/pg/breaker/recover", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler = s.authMiddleware(s.handlePgBreakerRecover)
	handler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("POST /recover no-auth: got %d want 401", w.Code)
	}

	// Wrong token rejected.
	body = bytes.NewBufferString(`{"confirmation_phrase":"RECOVER"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/pg/breaker/recover", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer bogus-token")
	w = httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("POST /recover wrong-token: got %d want 401", w.Code)
	}
}

// ---- Test 4: Typed-phrase enforcement on POST /recover -------------------

func TestPgBreakerHandlers_TypedPhraseRequired(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()
	s.SetPgBreaker(&fakeBreakerCtl{state: models.BreakerState{PaperModeStickyUntil: 12345}})

	t.Run("recover lowercase rejected", func(t *testing.T) {
		body := bytes.NewBufferString(`{"confirmation_phrase":"recover"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/pg/breaker/recover", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.authMiddleware(s.handlePgBreakerRecover)(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d want 400; body=%s", w.Code, w.Body.String())
		}
		if !bytes.Contains(w.Body.Bytes(), []byte("must equal 'RECOVER'")) {
			t.Errorf("expected 'must equal RECOVER' in body, got: %s", w.Body.String())
		}
	})

	t.Run("test-fire lowercase rejected", func(t *testing.T) {
		body := bytes.NewBufferString(`{"confirmation_phrase":"test-fire"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/pg/breaker/test-fire", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.authMiddleware(s.handlePgBreakerTestFire)(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d want 400; body=%s", w.Code, w.Body.String())
		}
		if !bytes.Contains(w.Body.Bytes(), []byte("must equal 'TEST-FIRE'")) {
			t.Errorf("expected 'must equal TEST-FIRE' in body, got: %s", w.Body.String())
		}
	})
}

// ---- Test 5: POST /recover happy path ------------------------------------

func TestPgBreakerHandlers_RecoverEndpoint(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()
	ctl := &fakeBreakerCtl{state: models.BreakerState{PaperModeStickyUntil: 12345}}
	s.SetPgBreaker(ctl)

	body := bytes.NewBufferString(`{"confirmation_phrase":"RECOVER","operator":"alice"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/pg/breaker/recover", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.authMiddleware(s.handlePgBreakerRecover)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d want 200; body=%s", w.Code, w.Body.String())
	}
	if len(ctl.recoverCalls) != 1 {
		t.Fatalf("Recover calls=%d want 1", len(ctl.recoverCalls))
	}
	if ctl.recoverCalls[0] != "alice" {
		t.Errorf("Recover operator=%q want alice", ctl.recoverCalls[0])
	}
	var resp Response
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	if _, ok := data["recovery_ts_ms"]; !ok {
		t.Errorf("recovery_ts_ms missing in response data")
	}
	if data["operator"].(string) != "alice" {
		t.Errorf("operator=%v want alice", data["operator"])
	}
}

// ---- Test 6: POST /test-fire happy path (real + dry-run) -----------------

func TestPgBreakerHandlers_TestFireEndpoint(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()
	ctl := &fakeBreakerCtl{
		state: models.BreakerState{PaperModeStickyUntil: 0}, // armed
		testFireRec: models.BreakerTripRecord{
			TripPnLUSDT: -120,
			Threshold:   -50,
			Source:      "test_fire",
		},
	}
	s.SetPgBreaker(ctl)

	t.Run("real trip", func(t *testing.T) {
		body := bytes.NewBufferString(`{"confirmation_phrase":"TEST-FIRE","dry_run":false}`)
		req := httptest.NewRequest(http.MethodPost, "/api/pg/breaker/test-fire", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.authMiddleware(s.handlePgBreakerTestFire)(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d want 200; body=%s", w.Code, w.Body.String())
		}
		if len(ctl.testFireCalls) != 1 {
			t.Fatalf("TestFire calls=%d want 1", len(ctl.testFireCalls))
		}
		if ctl.testFireCalls[0] != false {
			t.Errorf("TestFire dryRun=%v want false", ctl.testFireCalls[0])
		}
	})

	t.Run("dry-run", func(t *testing.T) {
		ctl.testFireRec.Source = "test_fire_dry_run"
		body := bytes.NewBufferString(`{"confirmation_phrase":"TEST-FIRE","dry_run":true}`)
		req := httptest.NewRequest(http.MethodPost, "/api/pg/breaker/test-fire", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.authMiddleware(s.handlePgBreakerTestFire)(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d want 200; body=%s", w.Code, w.Body.String())
		}
		if len(ctl.testFireCalls) != 2 {
			t.Fatalf("TestFire calls=%d want 2 (after dry-run)", len(ctl.testFireCalls))
		}
		if ctl.testFireCalls[1] != true {
			t.Errorf("TestFire dryRun=%v want true", ctl.testFireCalls[1])
		}
	})
}

// ---- Test 7: POST /test-fire while tripped -> 409 ------------------------

func TestPgBreakerHandlers_TestFireWhileTripped(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()
	ctl := &fakeBreakerCtl{state: models.BreakerState{PaperModeStickyUntil: 12345}}
	s.SetPgBreaker(ctl)

	body := bytes.NewBufferString(`{"confirmation_phrase":"TEST-FIRE","dry_run":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/pg/breaker/test-fire", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.authMiddleware(s.handlePgBreakerTestFire)(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("got %d want 409; body=%s", w.Code, w.Body.String())
	}
	if len(ctl.testFireCalls) != 0 {
		t.Errorf("TestFire called while tripped (server-side guard violated)")
	}
}

// ---- Test 8: POST /recover when not tripped -> 409 ----------------------

func TestPgBreakerHandlers_RecoverWhenNotTripped(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()
	ctl := &fakeBreakerCtl{state: models.BreakerState{PaperModeStickyUntil: 0}}
	s.SetPgBreaker(ctl)

	body := bytes.NewBufferString(`{"confirmation_phrase":"RECOVER"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/pg/breaker/recover", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.authMiddleware(s.handlePgBreakerRecover)(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("got %d want 409; body=%s", w.Code, w.Body.String())
	}
	if len(ctl.recoverCalls) != 0 {
		t.Errorf("Recover called when not tripped")
	}
}

// ---- Test 9: Recover error from controller -> 500 -----------------------

func TestPgBreakerHandlers_RecoverError_500(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()
	ctl := &fakeBreakerCtl{
		state:      models.BreakerState{PaperModeStickyUntil: 12345},
		recoverErr: errors.New("redis transport boom"),
	}
	s.SetPgBreaker(ctl)

	body := bytes.NewBufferString(`{"confirmation_phrase":"RECOVER"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/pg/breaker/recover", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.authMiddleware(s.handlePgBreakerRecover)(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("got %d want 500; body=%s", w.Code, w.Body.String())
	}
}
