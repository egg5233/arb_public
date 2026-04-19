package gateio

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetMarginInterestRateHistoryGateioGolden(t *testing.T) {
	// Three timestamps: before start, within range, after end.
	const inMs = int64(1729050000000)     // within range
	const outOldMs = int64(1729040000000) // before start
	const outNewMs = int64(1729060000000) // after end

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/unified/history_loan_rate" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("currency") != "BTC" {
			t.Fatalf("expected currency=BTC, got %q", r.URL.Query().Get("currency"))
		}
		w.Header().Set("Content-Type", "application/json")
		// Sorted newest-first per Gate.io spec.
		fmt.Fprintf(w, `{"currency":"BTC","tier":"0","tier_up_rate":"","rates":[`+
			`{"time":%d,"rate":"0.00020000"},`+
			`{"time":%d,"rate":"0.00010287"},`+
			`{"time":%d,"rate":"0.00005000"}`+
			`]}`, outNewMs, inMs, outOldMs)
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClientWithBase(srv.URL)}

	start := time.UnixMilli(inMs - 1000)
	end := time.UnixMilli(inMs + 1000)

	points, err := adapter.GetMarginInterestRateHistory(context.Background(), "BTC", start, end)
	if err != nil {
		t.Fatalf("GetMarginInterestRateHistory: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point (filtered), got %d", len(points))
	}
	wantHourly := 0.00010287
	if math.Abs(points[0].HourlyRate-wantHourly) > 1e-10 {
		t.Fatalf("HourlyRate = %v, want %v", points[0].HourlyRate, wantHourly)
	}
	if points[0].Timestamp.UnixMilli() != inMs {
		t.Fatalf("Timestamp = %d, want %d", points[0].Timestamp.UnixMilli(), inMs)
	}
}

func TestGetMarginInterestRateHistoryGateioError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"label":"SERVER_ERROR","message":"internal error"}`)
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClientWithBase(srv.URL)}
	_, err := adapter.GetMarginInterestRateHistory(context.Background(), "BTC",
		time.Now().Add(-24*time.Hour), time.Now())
	if err == nil {
		t.Fatal("expected error on server failure")
	}
}
