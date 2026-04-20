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
	// Binance's /sapi/v1/margin/interestRateHistory returns one record per
	// day. The adapter expands each daily record into 24 hourly-equivalent
	// points to honor the SpotMarginExchange contract (hourly rates). This
	// test uses a full-day window so all 24 expanded points survive the
	// [start, end] filter.
	dayStart, _ := time.Parse(time.RFC3339, "2026-04-20T00:00:00Z")
	tsMs := dayStart.UnixMilli()
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
	// Start before the day, end after — include all 24 hours of expansion.
	start := dayStart.Add(-time.Hour)
	end := dayStart.Add(25 * time.Hour)

	points, err := adapter.GetMarginInterestRateHistory(context.Background(), "BTC", start, end)
	if err != nil {
		t.Fatalf("GetMarginInterestRateHistory: %v", err)
	}
	if len(points) != 24 {
		t.Fatalf("expected 24 hourly points from 1 daily record, got %d", len(points))
	}
	wantHourly := 0.00024 / 24
	for i, p := range points {
		if math.Abs(p.HourlyRate-wantHourly) > 1e-12 {
			t.Fatalf("point[%d].HourlyRate = %v, want %v", i, p.HourlyRate, wantHourly)
		}
		if p.VipLevel != "0" {
			t.Fatalf("point[%d].VipLevel = %q, want \"0\"", i, p.VipLevel)
		}
		wantTS := dayStart.Add(time.Duration(i) * time.Hour)
		if !p.Timestamp.Equal(wantTS) {
			t.Fatalf("point[%d].Timestamp = %v, want %v", i, p.Timestamp, wantTS)
		}
	}
}

// TestGetMarginInterestRateHistoryBinanceDailyExpansionCostAccuracy verifies the
// critical economic property: summing HourlyRate×10000 across expanded points
// over a 7-day window equals the total borrow cost in bps (daily × 7 × 10000),
// NOT daily×7×10000/24 (the pre-fix under-counting). Regression guard for the
// bug that would produce artificially-favorable Dir A NetBps on Binance.
func TestGetMarginInterestRateHistoryBinanceDailyExpansionCostAccuracy(t *testing.T) {
	baseTime, _ := time.Parse(time.RFC3339, "2026-04-14T00:00:00Z")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 7 daily records over a 7-day window.
		var records []string
		for d := 0; d < 7; d++ {
			ts := baseTime.Add(time.Duration(d) * 24 * time.Hour).UnixMilli()
			records = append(records, fmt.Sprintf(`{"asset":"BTC","dailyInterestRate":"0.00024000","timestamp":%d,"vipLevel":0}`, ts))
		}
		fmt.Fprintf(w, "[%s]", joinStrings(records, ","))
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClient("key", "secret").WithBaseURL(srv.URL)}
	start := baseTime
	end := baseTime.Add(7 * 24 * time.Hour)

	points, err := adapter.GetMarginInterestRateHistory(context.Background(), "BTC", start, end)
	if err != nil {
		t.Fatalf("GetMarginInterestRateHistory: %v", err)
	}
	// 7 days × 24 hours = 168 expanded points.
	if len(points) != 168 {
		t.Fatalf("expected 168 expanded points (7 days × 24 hours), got %d", len(points))
	}

	// Sum of HourlyRate * 10000 must equal the total 7-day borrow cost in bps.
	var totalBps float64
	for _, p := range points {
		totalBps += p.HourlyRate * 10000
	}
	const dailyRate = 0.00024
	wantTotalBps := dailyRate * 7 * 10000
	if math.Abs(totalBps-wantTotalBps) > 1e-9 {
		t.Fatalf("total borrow cost = %.6f bps, want %.6f bps (daily × 7 × 10000) — adapter is under- or over-counting",
			totalBps, wantTotalBps)
	}
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += sep + p
	}
	return out
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
