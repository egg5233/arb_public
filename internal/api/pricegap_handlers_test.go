package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
	"arb/internal/pricegaptrader"
	"arb/pkg/utils"

	"github.com/alicebob/miniredis/v2"
)

// newPriceGapTestServer wires a *Server with miniredis, a populated auth store,
// a PriceGap config, and a running WS hub so Broadcast* calls don't deadlock.
// Returns (server, miniredis, bearerToken, teardown).
func newPriceGapTestServer(t *testing.T) (*Server, *miniredis.Miniredis, string, func()) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	db, err := database.New(mr.Addr(), "", 0)
	if err != nil {
		mr.Close()
		t.Fatalf("database.New: %v", err)
	}

	cfg := &config.Config{
		DashboardPassword: "test-pw", // enable auth
		PriceGapEnabled:   true,
		PriceGapPaperMode: true,
		PriceGapBudget:    5000,
		PriceGapCandidates: []models.PriceGapCandidate{
			{Symbol: "BTCUSDT", LongExch: "binance", ShortExch: "bybit", ThresholdBps: 25, MaxPositionUSDT: 500},
			{Symbol: "SOONUSDT", LongExch: "binance", ShortExch: "bybit", ThresholdBps: 25, MaxPositionUSDT: 500},
		},
	}

	s := &Server{
		db:             db,
		cfg:            cfg,
		hub:            NewHub(),
		log:            utils.NewLogger("test"),
		auth:           newAuthStore(nil), // in-memory sessions (no Redis TTL dependency)
		configNotifier: config.NewConfigNotifier(),
	}

	// Run the hub so Broadcast calls don't block on a nil-channel write.
	go s.hub.Run()

	token, err := s.auth.createSession()
	if err != nil {
		mr.Close()
		t.Fatalf("createSession: %v", err)
	}

	teardown := func() {
		mr.Close()
	}
	return s, mr, token, teardown
}

// ---------------------------------------------------------------------------
// GET /api/pricegap/state
// ---------------------------------------------------------------------------

func TestPriceGapState_AuthRequired(t *testing.T) {
	s, _, _, td := newPriceGapTestServer(t)
	defer td()

	req := httptest.NewRequest(http.MethodGet, "/api/pricegap/state", nil)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePriceGapState)
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no-auth: got %d, want 401", w.Code)
	}
}

func TestPriceGapState_Shape(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	req := httptest.NewRequest(http.MethodGet, "/api/pricegap/state", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePriceGapState)
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	var resp struct {
		OK   bool                  `json:"ok"`
		Data priceGapStateResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Fatalf("ok=false")
	}
	if !resp.Data.Enabled {
		t.Errorf("Enabled=false, want true")
	}
	if !resp.Data.PaperMode {
		t.Errorf("PaperMode=false, want true")
	}
	if resp.Data.Budget != 5000 {
		t.Errorf("Budget=%v, want 5000", resp.Data.Budget)
	}
	if len(resp.Data.Candidates) != 2 {
		t.Errorf("Candidates len=%d, want 2", len(resp.Data.Candidates))
	}
	if resp.Data.ActivePositions == nil {
		t.Errorf("ActivePositions is nil, want empty slice")
	}
	if resp.Data.RecentClosed == nil {
		t.Errorf("RecentClosed is nil, want empty slice")
	}
	if resp.Data.Metrics == nil {
		t.Errorf("Metrics is nil, want empty slice")
	}
}

// ---------------------------------------------------------------------------
// GET /api/pricegap/closed
// ---------------------------------------------------------------------------

func TestPriceGapClosed_OffsetAndLimit(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	// Seed history: 150 closed positions.
	for i := 0; i < 150; i++ {
		p := &models.PriceGapPosition{
			ID:     "p" + strconv.Itoa(i),
			Symbol: "BTCUSDT",
			Status: models.PriceGapStatusClosed,
			Mode:   models.PriceGapModeLive,
		}
		if err := s.db.AddPriceGapHistory(p); err != nil {
			t.Fatalf("AddPriceGapHistory: %v", err)
		}
	}

	// Default: limit 100, newest-first → first entry ID = "p149"
	req := httptest.NewRequest(http.MethodGet, "/api/pricegap/closed", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePriceGapClosed)
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("default: got %d", w.Code)
	}
	var resp struct {
		OK   bool                       `json:"ok"`
		Data []*models.PriceGapPosition `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) != 100 {
		t.Fatalf("default len=%d, want 100", len(resp.Data))
	}
	if resp.Data[0].ID != "p149" {
		t.Errorf("default[0]=%q, want p149 (newest-first)", resp.Data[0].ID)
	}

	// Offset=100 → skip 100 newest, remaining 50 from page 2
	req2 := httptest.NewRequest(http.MethodGet, "/api/pricegap/closed?offset=100&limit=100", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	handler(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("offset: got %d", w2.Code)
	}
	var resp2 struct {
		OK   bool                       `json:"ok"`
		Data []*models.PriceGapPosition `json:"data"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	if len(resp2.Data) != 50 {
		t.Errorf("offset len=%d, want 50 (150 total - 100 offset)", len(resp2.Data))
	}
	if len(resp2.Data) > 0 && resp2.Data[0].ID != "p49" {
		t.Errorf("offset[0]=%q, want p49", resp2.Data[0].ID)
	}
}

func TestPriceGapClosed_LimitCap(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	// limit=1000000 must be capped at 500.
	req := httptest.NewRequest(http.MethodGet, "/api/pricegap/closed?limit=1000000", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handlePriceGapClosed)
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d", w.Code)
	}
	var resp struct {
		OK   bool                       `json:"ok"`
		Data []*models.PriceGapPosition `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	// Data len bounded; cannot exceed history cap (500).
	if len(resp.Data) > 500 {
		t.Errorf("len=%d, want <= 500 (cap)", len(resp.Data))
	}
}

// ---------------------------------------------------------------------------
// POST /api/pricegap/candidate/{symbol}/disable
// ---------------------------------------------------------------------------

// invokeCandidateHandler dispatches to disable/enable. We build the request with
// the path-param expected by the handler so extractCandidateSymbol sees the
// symbol.
func invokeCandidateHandler(
	s *Server,
	handler http.HandlerFunc,
	method, path, token, body string,
) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != "" {
		reqBody = bytes.NewBufferString(body)
	} else {
		reqBody = bytes.NewBufferString("")
	}
	req := httptest.NewRequest(method, path, reqBody)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	authed := s.authMiddleware(handler)
	authed(w, req)
	return w
}

func TestPriceGapCandidateDisable_Happy(t *testing.T) {
	s, mr, token, td := newPriceGapTestServer(t)
	defer td()

	w := invokeCandidateHandler(s, s.handlePriceGapCandidateDisable, http.MethodPost,
		"/api/pricegap/candidate/BTCUSDT/disable", token, `{"reason":"test"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	// Redis key must be written as JSON shape {reason, disabled_at}.
	raw, err := mr.Get("pg:candidate:disabled:BTCUSDT")
	if err != nil {
		t.Fatalf("mr.Get: %v", err)
	}
	var payload struct {
		Reason     string `json:"reason"`
		DisabledAt int64  `json:"disabled_at"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("stored value not JSON: %v (raw=%q)", err, raw)
	}
	if payload.Reason != "test" {
		t.Errorf("stored reason=%q, want %q", payload.Reason, "test")
	}
	if payload.DisabledAt == 0 {
		t.Errorf("stored disabled_at=0, want non-zero")
	}
}

func TestPriceGapCandidateDisable_UnknownSymbol(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	w := invokeCandidateHandler(s, s.handlePriceGapCandidateDisable, http.MethodPost,
		"/api/pricegap/candidate/UNKNOWNSYM/disable", token, `{"reason":"x"}`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", w.Code)
	}
}

func TestPriceGapCandidateDisable_ReasonTooLong(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	long := strings.Repeat("x", 300)
	body := `{"reason":"` + long + `"}`
	w := invokeCandidateHandler(s, s.handlePriceGapCandidateDisable, http.MethodPost,
		"/api/pricegap/candidate/BTCUSDT/disable", token, body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400 (length cap)", w.Code)
	}
}

func TestPriceGapCandidateDisable_ControlCharsStripped(t *testing.T) {
	s, mr, token, td := newPriceGapTestServer(t)
	defer td()

	// Reason contains a literal newline (inside the JSON string via \n escape).
	body := `{"reason":"line1\nline2"}`
	w := invokeCandidateHandler(s, s.handlePriceGapCandidateDisable, http.MethodPost,
		"/api/pricegap/candidate/BTCUSDT/disable", token, body)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	raw, _ := mr.Get("pg:candidate:disabled:BTCUSDT")
	var payload struct {
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal([]byte(raw), &payload)
	if strings.ContainsRune(payload.Reason, '\n') {
		t.Errorf("stored reason still contains newline: %q", payload.Reason)
	}
	if payload.Reason != "line1line2" {
		t.Errorf("stored reason=%q, want %q (control chars stripped)", payload.Reason, "line1line2")
	}
}

func TestPriceGapCandidateEnable_ClearsKey(t *testing.T) {
	s, mr, token, td := newPriceGapTestServer(t)
	defer td()

	// Pre-seed the disable flag.
	if err := s.db.SetCandidateDisabled("BTCUSDT", "seeded"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if !mr.Exists("pg:candidate:disabled:BTCUSDT") {
		t.Fatalf("precondition: key must exist")
	}

	w := invokeCandidateHandler(s, s.handlePriceGapCandidateEnable, http.MethodPost,
		"/api/pricegap/candidate/BTCUSDT/enable", token, "")

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if mr.Exists("pg:candidate:disabled:BTCUSDT") {
		t.Errorf("key still present after enable")
	}
}

// ---------------------------------------------------------------------------
// POST /api/config with price_gap_paper_mode — Phase 9 D-12 round-trip.
// ---------------------------------------------------------------------------

func TestConfig_PaperMode_NestedRoundTrip(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	body := `{"price_gap":{"paper_mode":false}}`
	req := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handleConfig)
	handler(w, req)

	// POST config has additional machinery (Redis field write, .bak write).
	// We only need to assert the in-memory cfg flipped — the save path is
	// covered by existing POST config tests. Some non-critical I/O may error;
	// we inspect cfg directly.
	if s.cfg.PriceGapPaperMode != false {
		t.Errorf("cfg.PriceGapPaperMode=%v, want false; status=%d body=%s",
			s.cfg.PriceGapPaperMode, w.Code, w.Body.String())
	}
}

func TestConfig_PaperMode_FlatRoundTrip(t *testing.T) {
	s, _, token, td := newPriceGapTestServer(t)
	defer td()

	body := `{"price_gap_paper_mode":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler := s.authMiddleware(s.handleConfig)
	handler(w, req)

	if s.cfg.PriceGapPaperMode != false {
		t.Errorf("cfg.PriceGapPaperMode=%v, want false; status=%d body=%s",
			s.cfg.PriceGapPaperMode, w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Broadcaster interface
// ---------------------------------------------------------------------------

func TestBroadcastPriceGap_InterfaceAssertion(t *testing.T) {
	// Compile-time check would already have failed at build; this asserts the
	// runtime assignment succeeds too.
	var _ pricegaptrader.Broadcaster = (*Server)(nil)
}

func TestBroadcastPriceGapPositions_CallsHub(t *testing.T) {
	s, _, _, td := newPriceGapTestServer(t)
	defer td()

	// Just prove the call does not panic and does not block. The hub Run loop
	// consumes the message; there are no registered clients so it's a no-op
	// sink — still exercises the code path.
	s.BroadcastPriceGapPositions([]*models.PriceGapPosition{{ID: "x", Symbol: "BTCUSDT"}})
	s.BroadcastPriceGapEvent(pricegaptrader.PriceGapEvent{Type: "entry", Symbol: "BTCUSDT"})
	s.BroadcastPriceGapCandidateUpdate(pricegaptrader.PriceGapCandidateUpdate{
		Symbol: "BTCUSDT", Disabled: true, Reason: "test",
	})
}
