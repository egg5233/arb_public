package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestGetMarginInterestRateHistoryBinanceGolden(t *testing.T) {
	const tsMs = int64(1611544731000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sapi/v1/margin/interestRateHistory" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("asset") != "BTC" {
			t.Fatalf("expected asset=BTC, got %q", r.URL.Query().Get("asset"))
		}
		fmt.Fprintf(w, `[{"asset":"BTC","dailyInterestRate":"0.00024000","timestamp":%d,"vipLevel":0}]`, tsMs)
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClient("key", "secret").WithBaseURL(srv.URL)}
	start := time.UnixMilli(tsMs - 1000)
	end := time.UnixMilli(tsMs + 1000)

	points, err := adapter.GetMarginInterestRateHistory(context.Background(), "BTC", start, end)
	if err != nil {
		t.Fatalf("GetMarginInterestRateHistory: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	wantHourly := 0.00024 / 24
	if math.Abs(points[0].HourlyRate-wantHourly) > 1e-12 {
		t.Fatalf("HourlyRate = %v, want %v", points[0].HourlyRate, wantHourly)
	}
	if points[0].VipLevel != "0" {
		t.Fatalf("VipLevel = %q, want \"0\"", points[0].VipLevel)
	}
	if points[0].Timestamp.UnixMilli() != tsMs {
		t.Fatalf("Timestamp = %d, want %d", points[0].Timestamp.UnixMilli(), tsMs)
	}
}

func TestGetMarginInterestRateHistoryBinanceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"code":-1003,"msg":"server error"}`)
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClient("key", "secret").WithBaseURL(srv.URL)}
	_, err := adapter.GetMarginInterestRateHistory(context.Background(), "BTC",
		time.Now().Add(-24*time.Hour), time.Now())
	if err == nil {
		t.Fatal("expected error on server failure")
	}
}

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
