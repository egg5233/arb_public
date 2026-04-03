package binance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestGetSpotBBOUsesUnsignedPublicEndpoint(t *testing.T) {
	var gotQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		if r.URL.Path != "/api/v3/ticker/bookTicker" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"bidPrice": "100.1",
			"askPrice": "100.2",
		})
	}))
	defer srv.Close()

	adapter := &Adapter{
		client: NewClient("", "").WithBaseURL(srv.URL),
	}

	bbo, err := adapter.GetSpotBBO("BTCUSDT")
	if err != nil {
		t.Fatalf("GetSpotBBO: %v", err)
	}
	if bbo.Bid != 100.1 || bbo.Ask != 100.2 {
		t.Fatalf("unexpected bbo: %+v", bbo)
	}
	if gotQuery.Get("symbol") != "BTCUSDT" {
		t.Fatalf("expected symbol=BTCUSDT, got %q", gotQuery.Get("symbol"))
	}
	if gotQuery.Get("timestamp") != "" {
		t.Fatalf("expected no timestamp for public endpoint, got %q", gotQuery.Get("timestamp"))
	}
	if gotQuery.Get("signature") != "" {
		t.Fatalf("expected no signature for public endpoint, got %q", gotQuery.Get("signature"))
	}
}
