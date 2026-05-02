package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Phase 16 PG-FIX-02 D-06 — server-side guard tests.
//
// The /api/config POST handler must reject any write touching
// price_gap_paper_mode (flat) or price_gap.paper_mode (nested) UNLESS the
// request body sets operator_action=true. The reject tests here are written
// so that removing the guard makes them red by construction (they assert the
// in-memory cfg flag is UNCHANGED after a no-marker POST — only the guard
// makes that true; without the guard the existing apply paths silently flip
// it).
//
// Phase 9 pos.Mode immutability is per-position; this guard sits at the
// engine-wide config-write layer. Phase 15 IsPaperModeActive sticky-flag
// chokepoint is unchanged.

// helper — POST a body to /api/config via the handleConfig router and return
// the response recorder.
func postConfigBody(t *testing.T, s *Server, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handleConfig)
	handler(w, req)
	return w
}

// Test 1: flat write path — no operator_action marker → 409, state unchanged.
// Removing the guard causes the apply at handlers.go:1666 to silently flip
// the flag, and the "before == after" assertion goes red.
func TestConfig_PaperMode_RequiresOperatorAction_RejectFlat(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	// Setup: cfg starts with PaperMode=true (from newPriceGapTestServer).
	before := s.cfg.PriceGapPaperMode
	if !before {
		t.Fatalf("test setup invariant: expected cfg.PriceGapPaperMode=true initially, got %v", before)
	}

	// Attempt flat write WITHOUT operator_action.
	body := `{"price_gap_paper_mode":false}`
	w := postConfigBody(t, s, token, body)

	if w.Code != http.StatusConflict {
		t.Errorf("status: got %d, want %d; body=%s", w.Code, http.StatusConflict, w.Body.String())
	}

	// Response envelope: ok=false, error contains "operator_action".
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, w.Body.String())
	}
	if !bytes.Contains([]byte(resp.Error), []byte("operator_action")) {
		t.Errorf("error should mention operator_action; got %q", resp.Error)
	}

	// State UNCHANGED — this is the assertion that goes red without the guard.
	after := s.cfg.PriceGapPaperMode
	if after != before {
		t.Errorf("cfg.PriceGapPaperMode mutated despite reject: before=%v after=%v", before, after)
	}
}

// Test 2: nested write path — no operator_action marker → 409, state unchanged.
// Removing the guard causes the apply at handlers.go:1617 to silently flip
// the flag, and the "before == after" assertion goes red.
func TestConfig_PaperMode_RequiresOperatorAction_RejectNested(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	before := s.cfg.PriceGapPaperMode
	if !before {
		t.Fatalf("test setup invariant: expected cfg.PriceGapPaperMode=true initially, got %v", before)
	}

	// Attempt nested write WITHOUT operator_action.
	body := `{"price_gap":{"paper_mode":false}}`
	w := postConfigBody(t, s, token, body)

	if w.Code != http.StatusConflict {
		t.Errorf("status: got %d, want %d; body=%s", w.Code, http.StatusConflict, w.Body.String())
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, w.Body.String())
	}
	if !bytes.Contains([]byte(resp.Error), []byte("operator_action")) {
		t.Errorf("error should mention operator_action; got %q", resp.Error)
	}

	// State UNCHANGED — this is the assertion that goes red without the guard.
	after := s.cfg.PriceGapPaperMode
	if after != before {
		t.Errorf("cfg.PriceGapPaperMode mutated despite reject: before=%v after=%v", before, after)
	}
}

// Test 3: with marker → accept + state flips. Round-trip behavior preserved.
func TestConfig_PaperMode_AcceptWithOperatorAction(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	body := `{"price_gap_paper_mode":false,"operator_action":true}`
	w := postConfigBody(t, s, token, body)

	// Accept path — handler may emit 200 even if non-critical Redis writes
	// fail in the test sandbox; assert NOT 409 and state did flip.
	if w.Code == http.StatusConflict {
		t.Errorf("accept path returned 409; body=%s", w.Body.String())
	}
	if s.cfg.PriceGapPaperMode != false {
		t.Errorf("cfg.PriceGapPaperMode=%v, want false; status=%d body=%s",
			s.cfg.PriceGapPaperMode, w.Code, w.Body.String())
	}
}

// Test 4: non-paper-mode writes are unaffected — guard scoped narrowly.
func TestConfig_PaperMode_NonPaperWritesUnaffected(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	beforePaper := s.cfg.PriceGapPaperMode

	// Write a non-paper price_gap field (DebugLog) WITHOUT operator_action —
	// must succeed. (No paper_mode key in the body, so guard does not trigger.)
	body := `{"price_gap":{"debug_log":true}}`
	w := postConfigBody(t, s, token, body)

	if w.Code == http.StatusConflict {
		t.Errorf("non-paper write rejected with 409; guard is too broad. body=%s", w.Body.String())
	}
	if !s.cfg.PriceGapDebugLog {
		t.Errorf("cfg.PriceGapDebugLog not updated; status=%d body=%s", w.Code, w.Body.String())
	}
	// Paper mode UNCHANGED (guard didn't fire, but neither did paper-mode logic).
	if s.cfg.PriceGapPaperMode != beforePaper {
		t.Errorf("cfg.PriceGapPaperMode unexpectedly changed: before=%v after=%v",
			beforePaper, s.cfg.PriceGapPaperMode)
	}
}

// Test 5: GET /api/config does NOT mutate cfg.PriceGapPaperMode across N reads.
// Pitfall-1-class regression: VALIDATION.md test ID TestConfigGetDoesNotMutate.
// Closes a regression class even if no current GET handler is the offender.
func TestConfigGet_DoesNotMutate_PaperMode(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	// Pin paper mode to true.
	s.cfg.Lock()
	s.cfg.PriceGapPaperMode = true
	s.cfg.Unlock()
	want := true

	const N = 20
	for i := 0; i < N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler := s.authMiddleware(s.handleConfig)
		handler(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("GET /api/config iter=%d status=%d body=%s", i, w.Code, w.Body.String())
		}

		s.cfg.RLock()
		got := s.cfg.PriceGapPaperMode
		s.cfg.RUnlock()
		if got != want {
			t.Errorf("GET /api/config mutated cfg.PriceGapPaperMode at iter=%d: got=%v want=%v",
				i, got, want)
		}
	}
}
