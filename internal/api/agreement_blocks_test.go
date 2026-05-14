package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"arb/internal/database"
)

func TestHandleListAgreementBlocks_Empty(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/agreement-blocks", nil)
	w := httptest.NewRecorder()
	s.handleListAgreementBlocks(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		OK   bool                      `json:"ok"`
		Data []database.AgreementBlock `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected ok=true")
	}
	if resp.Data == nil {
		t.Fatal("expected non-nil data slice")
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(resp.Data))
	}
}

func TestHandleListAgreementBlocks_WithBlocks(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	if err := s.db.SetAgreementBlock("bybit", "HOODUSDT", "error 110126: must sign required agreement"); err != nil {
		t.Fatalf("SetAgreementBlock: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/agreement-blocks", nil)
	w := httptest.NewRecorder()
	s.handleListAgreementBlocks(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		OK   bool                      `json:"ok"`
		Data []database.AgreementBlock `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected ok=true")
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 block, got %d", len(resp.Data))
	}
	if resp.Data[0].Exchange != "bybit" || resp.Data[0].Symbol != "HOODUSDT" {
		t.Errorf("unexpected block: %+v", resp.Data[0])
	}
}

func TestHandleListAgreementBlocks_MethodNotAllowed(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/agreement-blocks", nil)
	w := httptest.NewRecorder()
	s.handleListAgreementBlocks(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleClearAgreementBlock_OK(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()
	s.hub = NewHub() // needed for broadcast inside handler

	if err := s.db.SetAgreementBlock("bybit", "HOOKUSDT", "110126: agreement required"); err != nil {
		t.Fatalf("SetAgreementBlock: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"exchange": "bybit", "symbol": "HOOKUSDT"})
	req := httptest.NewRequest(http.MethodPost, "/api/agreement-blocks/clear", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.handleClearAgreementBlock(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify block is gone.
	if s.db.IsAgreementBlocked("bybit", "HOOKUSDT") {
		t.Error("expected block to be cleared")
	}
}

func TestHandleClearAgreementBlock_MissingFields(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	body, _ := json.Marshal(map[string]string{"exchange": "bybit"})
	req := httptest.NewRequest(http.MethodPost, "/api/agreement-blocks/clear", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.handleClearAgreementBlock(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleClearAgreementBlock_MethodNotAllowed(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/agreement-blocks/clear", nil)
	w := httptest.NewRecorder()
	s.handleClearAgreementBlock(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestBroadcastAgreementBlock_DoesNotPanic(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()
	s.hub = NewHub()

	if err := s.db.SetAgreementBlock("bybit", "SOONUSDT", "110126: must sign"); err != nil {
		t.Fatalf("SetAgreementBlock: %v", err)
	}

	// BroadcastAgreementBlock should not panic with no connected WS clients.
	block := database.AgreementBlock{Exchange: "bybit", Symbol: "SOONUSDT", Reason: "110126: must sign"}
	s.BroadcastAgreementBlock(block)

	// The block should still be visible via ListAgreementBlocks.
	blocks := s.db.ListAgreementBlocks()
	if len(blocks) != 1 || blocks[0].Symbol != "SOONUSDT" {
		t.Errorf("expected 1 block after broadcast, got %d", len(blocks))
	}
}
