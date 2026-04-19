package bybit

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetMarginInterestRateHistoryBybitGolden(t *testing.T) {
	const tsMs = int64(1721469600000)
	const wantRateStr = "0.000014621596"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v5/spot-margin-trade/interest-rate-history" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("currency") != "BTC" {
			t.Fatalf("expected currency=BTC, got %q", r.URL.Query().Get("currency"))
		}
		fmt.Fprintf(w, `{"retCode":0,"retMsg":"OK","result":{"list":[{"timestamp":%d,"currency":"BTC","hourlyBorrowRate":%q,"vipLevel":"No VIP"}]}}`, tsMs, wantRateStr)
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClient("key", "secret")}
	adapter.client.baseURL = srv.URL
	adapter.client.httpClient = srv.Client()

	start := time.UnixMilli(tsMs - 1000)
	end := time.UnixMilli(tsMs + 1000)

	points, err := adapter.GetMarginInterestRateHistory(context.Background(), "BTC", start, end)
	if err != nil {
		t.Fatalf("GetMarginInterestRateHistory: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	wantHourly := 0.000014621596
	if math.Abs(points[0].HourlyRate-wantHourly) > 1e-15 {
		t.Fatalf("HourlyRate = %v, want %v", points[0].HourlyRate, wantHourly)
	}
	if points[0].VipLevel != "No VIP" {
		t.Fatalf("VipLevel = %q, want \"No VIP\"", points[0].VipLevel)
	}
	if points[0].Timestamp.UnixMilli() != tsMs {
		t.Fatalf("Timestamp = %d, want %d", points[0].Timestamp.UnixMilli(), tsMs)
	}
}

func TestGetMarginInterestRateHistoryBybitError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"retCode":10001,"retMsg":"server error","result":{}}`)
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClient("key", "secret")}
	adapter.client.baseURL = srv.URL
	adapter.client.httpClient = srv.Client()

	_, err := adapter.GetMarginInterestRateHistory(context.Background(), "BTC",
		time.Now().Add(-24*time.Hour), time.Now())
	if err == nil {
		t.Fatal("expected error on non-zero retCode")
	}
}

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
