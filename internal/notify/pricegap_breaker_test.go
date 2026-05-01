// Package notify — pricegap_breaker_test.go covers Phase 15 Plan 15-04
// drawdown breaker Telegram dispatch (NotifyPriceGapBreakerTrip + Recovery).
// Critical-bucket dispatch (allowlist bypass) per CONTEXT D-17.
package notify

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"arb/internal/models"
)

type pgBreakerCaptured struct {
	chatID    string
	text      string
	parseMode string
}

func startBreakerCaptureServer(t *testing.T) (*httptest.Server, *[]pgBreakerCaptured, *sync.Mutex) {
	t.Helper()
	mu := &sync.Mutex{}
	captured := []pgBreakerCaptured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		captured = append(captured, pgBreakerCaptured{
			chatID:    r.PostFormValue("chat_id"),
			text:      r.PostFormValue("text"),
			parseMode: r.PostFormValue("parse_mode"),
		})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	return srv, &captured, mu
}

func waitForBreakerCaptured(captured *[]pgBreakerCaptured, mu *sync.Mutex, want int, deadline time.Duration) bool {
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		mu.Lock()
		got := len(*captured)
		mu.Unlock()
		if got >= want {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// TestNotifyPriceGapBreakerTrip_CriticalBucket — D-17: trip alert dispatches
// via the critical bucket (bypasses allowlist + cooldown). Must contain 24h
// PnL, threshold, ramp stage, paused count, recovery instruction line, and
// source label.
func TestNotifyPriceGapBreakerTrip_CriticalBucket(t *testing.T) {
	srv, captured, mu := startBreakerCaptureServer(t)
	defer srv.Close()
	prior := SetTelegramAPIBase(srv.URL)
	defer SetTelegramAPIBase(prior)

	tn := NewTelegram("test-token", "test-chat")
	if tn == nil {
		t.Fatal("NewTelegram returned nil")
	}
	rec := models.BreakerTripRecord{
		TripTs:               time.Date(2026, 5, 1, 4, 35, 0, 0, time.UTC).UnixMilli(),
		TripPnLUSDT:          -75.50,
		Threshold:            -50.00,
		RampStage:            2,
		PausedCandidateCount: 3,
		Source:               "live",
	}
	if err := tn.NotifyPriceGapBreakerTrip(rec); err != nil {
		t.Fatalf("NotifyPriceGapBreakerTrip err=%v", err)
	}

	if !waitForBreakerCaptured(captured, mu, 1, 2*time.Second) {
		t.Fatalf("expected 1 send, got %d", len(*captured))
	}
	mu.Lock()
	body := (*captured)[0].text
	mu.Unlock()
	for _, want := range []string{
		"BREAKER",
		"-75.50",                             // 24h PnL
		"-50.00",                             // threshold
		"Ramp Stage: 2",                      // ramp stage
		"3 candidate",                        // paused count
		"pg-admin breaker recover --confirm", // recovery instruction
		"LIVE",                               // source label (upper-cased)
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected body to contain %q, got: %q", want, body)
		}
	}

	// Critical-bucket: a second call with the SAME content fires again
	// (no cooldown suppression on critical alerts).
	if err := tn.NotifyPriceGapBreakerTrip(rec); err != nil {
		t.Fatalf("second NotifyPriceGapBreakerTrip err=%v", err)
	}
	if !waitForBreakerCaptured(captured, mu, 2, 2*time.Second) {
		mu.Lock()
		got := len(*captured)
		mu.Unlock()
		t.Fatalf("expected 2 sends (critical bypass), got %d", got)
	}
}

// TestNotifyPriceGapBreakerRecovery_CriticalBucket — recovery alert is also
// critical-bucket: must contain operator name, recovery timestamp, original
// trip PnL + threshold.
func TestNotifyPriceGapBreakerRecovery_CriticalBucket(t *testing.T) {
	srv, captured, mu := startBreakerCaptureServer(t)
	defer srv.Close()
	prior := SetTelegramAPIBase(srv.URL)
	defer SetTelegramAPIBase(prior)

	tn := NewTelegram("test-token", "test-chat")
	if tn == nil {
		t.Fatal("NewTelegram returned nil")
	}
	rec := models.BreakerTripRecord{
		TripTs:      time.Date(2026, 5, 1, 4, 35, 0, 0, time.UTC).UnixMilli(),
		TripPnLUSDT: -75.50,
		Threshold:   -50.00,
		RampStage:   2,
		Source:      "live",
	}
	if err := tn.NotifyPriceGapBreakerRecovery(rec, "alice"); err != nil {
		t.Fatalf("NotifyPriceGapBreakerRecovery err=%v", err)
	}

	if !waitForBreakerCaptured(captured, mu, 1, 2*time.Second) {
		t.Fatalf("expected 1 send, got %d", len(*captured))
	}
	mu.Lock()
	body := (*captured)[0].text
	mu.Unlock()
	for _, want := range []string{
		"RECOVERED",
		"alice",  // operator
		"-75.50", // original trip pnl
		"-50.00", // threshold
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected body to contain %q, got: %q", want, body)
		}
	}
}
