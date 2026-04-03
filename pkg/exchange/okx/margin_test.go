package okx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetSpotBBOMapsMissingSpotInstrument(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/market/ticker" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"51001","msg":"Instrument ID, Instrument ID code, or Spread ID doesn't exist.","data":[]}`))
	}))
	defer srv.Close()

	adapter := &Adapter{
		client:    NewClientWithBase(srv.URL),
		priceSyms: make(map[string]bool),
		depthSyms: make(map[string]bool),
	}

	_, err := adapter.GetSpotBBO("ONTUSDT")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no OKX spot market for ONTUSDT") {
		t.Fatalf("unexpected error: %v", err)
	}
}
