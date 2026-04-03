package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"arb/internal/config"
	"arb/internal/database"
	"arb/pkg/utils"

	"github.com/alicebob/miniredis/v2"
)

// newTestServer creates a Server backed by an in-memory Redis for handler tests.
func newTestServer(t *testing.T) (*Server, *miniredis.Miniredis) {
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
	s := &Server{
		db:             db,
		cfg:            &config.Config{},
		log:            utils.NewLogger("test"),
		configNotifier: config.NewConfigNotifier(),
	}
	return s, mr
}

// TestHandleGetSpotStats_ColdStart verifies that a fresh instance with no
// closed trades returns zero-valued counters instead of an empty object.
func TestHandleGetSpotStats_ColdStart(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/spot/stats", nil)
	w := httptest.NewRecorder()
	s.handleGetSpotStats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		OK   bool              `json:"ok"`
		Data map[string]string `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected ok=true")
	}

	for _, key := range []string{"total_pnl", "win_count", "loss_count", "trade_count"} {
		val, ok := resp.Data[key]
		if !ok {
			t.Errorf("missing field %q in cold start response", key)
		} else if val != "0" {
			t.Errorf("expected %q = \"0\", got %q", key, val)
		}
	}
}

// TestHandleGetSpotStats_PartialHash verifies that missing fields in a partial
// Redis hash are normalized to zero.
func TestHandleGetSpotStats_PartialHash(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	// Seed only total_pnl and win_count — leave loss_count and trade_count missing.
	mr.HSet("arb:spot_stats", "total_pnl", "12.50")
	mr.HSet("arb:spot_stats", "win_count", "3")

	req := httptest.NewRequest(http.MethodGet, "/api/spot/stats", nil)
	w := httptest.NewRecorder()
	s.handleGetSpotStats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		OK   bool              `json:"ok"`
		Data map[string]string `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Existing fields should be preserved.
	if resp.Data["total_pnl"] != "12.50" {
		t.Errorf("expected total_pnl = \"12.50\", got %q", resp.Data["total_pnl"])
	}
	if resp.Data["win_count"] != "3" {
		t.Errorf("expected win_count = \"3\", got %q", resp.Data["win_count"])
	}

	// Missing fields should be normalized to "0".
	if resp.Data["loss_count"] != "0" {
		t.Errorf("expected loss_count = \"0\", got %q", resp.Data["loss_count"])
	}
	if resp.Data["trade_count"] != "0" {
		t.Errorf("expected trade_count = \"0\", got %q", resp.Data["trade_count"])
	}
}

// TestHandleGetSpotStats_FullHash verifies that existing non-zero stats are
// returned unchanged.
func TestHandleGetSpotStats_FullHash(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	mr.HSet("arb:spot_stats", "total_pnl", "45.67")
	mr.HSet("arb:spot_stats", "win_count", "5")
	mr.HSet("arb:spot_stats", "loss_count", "2")
	mr.HSet("arb:spot_stats", "trade_count", "7")

	req := httptest.NewRequest(http.MethodGet, "/api/spot/stats", nil)
	w := httptest.NewRecorder()
	s.handleGetSpotStats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		OK   bool              `json:"ok"`
		Data map[string]string `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	expect := map[string]string{
		"total_pnl":   "45.67",
		"win_count":   "5",
		"loss_count":  "2",
		"trade_count": "7",
	}
	for k, v := range expect {
		if resp.Data[k] != v {
			t.Errorf("expected %q = %q, got %q", k, v, resp.Data[k])
		}
	}
}
