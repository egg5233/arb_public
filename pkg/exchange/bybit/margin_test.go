package bybit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSpotMarginOrderFallsBackToHistoryWhenRealtimeEmpty(t *testing.T) {
	var realtimeCalls, historyCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/order/realtime":
			realtimeCalls++
			fmt.Fprint(w, `{"retCode":0,"retMsg":"OK","result":{"list":[]}}`)
		case "/v5/order/history":
			historyCalls++
			fmt.Fprint(w, `{"retCode":0,"retMsg":"OK","result":{"list":[{"orderId":"abc","symbol":"BTCUSDT","orderStatus":"Filled","cumExecQty":"0.25","avgPrice":"101.5"}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClient("key", "secret")}
	adapter.client.baseURL = srv.URL
	adapter.client.httpClient = srv.Client()

	status, err := adapter.GetSpotMarginOrder("abc", "BTCUSDT")
	if err != nil {
		t.Fatalf("GetSpotMarginOrder: %v", err)
	}
	if realtimeCalls != 1 {
		t.Fatalf("realtime calls = %d, want 1", realtimeCalls)
	}
	if historyCalls != 1 {
		t.Fatalf("history calls = %d, want 1", historyCalls)
	}
	if status == nil {
		t.Fatal("expected status from history fallback")
	}
	if status.Status != "filled" {
		t.Fatalf("status = %q, want filled", status.Status)
	}
	if status.FilledQty != 0.25 {
		t.Fatalf("filled qty = %.2f, want 0.25", status.FilledQty)
	}
	if status.AvgPrice != 101.5 {
		t.Fatalf("avg price = %.2f, want 101.5", status.AvgPrice)
	}
}
