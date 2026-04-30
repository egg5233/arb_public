// Package notify — telegram_pricegap_phase14_test.go covers Phase 14 Plan
// 14-04 reconcile + ramp dispatch shapes (NotifyPriceGapDailyDigest,
// NotifyPriceGapReconcileFailure, NotifyPriceGapRampDemote,
// NotifyPriceGapRampForceOp). Tests live here (not in pricegaptrader) to
// avoid the import cycle through pricegap_assert.go.
package notify

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"arb/internal/models"
	"arb/internal/pricegaptrader"
)

type pgPhase14Captured struct {
	chatID    string
	text      string
	parseMode string
}

func startPhase14CaptureServer(t *testing.T) (*httptest.Server, *[]pgPhase14Captured, *sync.Mutex) {
	t.Helper()
	mu := &sync.Mutex{}
	captured := []pgPhase14Captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		captured = append(captured, pgPhase14Captured{
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

func waitForPhase14Captured(captured *[]pgPhase14Captured, mu *sync.Mutex, want int, deadline time.Duration) bool {
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

// TestNotifier_DailyDigest_DispatchesNonCritical asserts NotifyPriceGapDailyDigest
// dispatches a single send containing the date, $ marker, "stage" keyword,
// and a Wins/Losses summary that reflects the input record.
func TestNotifier_DailyDigest_DispatchesNonCritical(t *testing.T) {
	srv, captured, mu := startPhase14CaptureServer(t)
	defer srv.Close()
	prior := SetTelegramAPIBase(srv.URL)
	defer SetTelegramAPIBase(prior)

	tn := NewTelegram("test-token", "test-chat")
	if tn == nil {
		t.Fatal("NewTelegram returned nil")
	}
	rec := pricegaptrader.DailyReconcileRecord{
		SchemaVersion: 1,
		Date:          "2026-04-29",
		ComputedAt:    time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC),
		Totals: pricegaptrader.DailyReconcileTotals{
			RealizedPnLUSDT: 12.34,
			PositionsClosed: 5,
			Wins:            4,
			Losses:          1,
			NetClean:        true,
		},
		Positions: []pricegaptrader.DailyReconcilePosition{},
		Anomalies: pricegaptrader.DailyReconcileAnomalies{FlaggedIDs: []string{}},
	}
	rs := models.RampState{
		CurrentStage:    2,
		CleanDayCounter: 3,
		LastEvalTs:      time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC),
		DemoteCount:     0,
	}
	tn.NotifyPriceGapDailyDigest("2026-04-29", rec, rs)

	if !waitForPhase14Captured(captured, mu, 1, 2*time.Second) {
		t.Fatalf("expected 1 send, got %d", len(*captured))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(*captured) != 1 {
		t.Fatalf("expected exactly 1 send, got %d", len(*captured))
	}
	body := (*captured)[0].text
	if !strings.Contains(body, "$") {
		t.Errorf("digest missing $ marker: %q", body)
	}
	if !strings.Contains(body, "2026-04-29") {
		t.Errorf("digest missing date: %q", body)
	}
	if !strings.Contains(strings.ToLower(body), "stage") {
		t.Errorf("digest missing 'stage': %q", body)
	}
	if !strings.Contains(body, "Wins/Losses: 4/1") {
		t.Errorf("digest missing wins/losses 4/1: %q", body)
	}
}

// TestNotifier_ReconcileFailure_DispatchesCritical asserts NotifyPriceGapReconcileFailure
// dispatches a single message tagged with FAILED + RECONCILE markers + the date.
func TestNotifier_ReconcileFailure_DispatchesCritical(t *testing.T) {
	srv, captured, mu := startPhase14CaptureServer(t)
	defer srv.Close()
	prior := SetTelegramAPIBase(srv.URL)
	defer SetTelegramAPIBase(prior)

	tn := NewTelegram("test-token", "test-chat")
	if tn == nil {
		t.Fatal("NewTelegram returned nil")
	}
	tn.NotifyPriceGapReconcileFailure("2026-04-29", errors.New("redis: connection refused"))

	if !waitForPhase14Captured(captured, mu, 1, 2*time.Second) {
		t.Fatalf("expected 1 send, got %d", len(*captured))
	}
	mu.Lock()
	defer mu.Unlock()
	body := (*captured)[0].text
	if !strings.Contains(body, "FAILED") {
		t.Errorf("expected FAILED marker in body: %q", body)
	}
	if !strings.Contains(body, "RECONCILE") {
		t.Errorf("expected RECONCILE marker in body: %q", body)
	}
	if !strings.Contains(body, "2026-04-29") {
		t.Errorf("expected date in body: %q", body)
	}
}

// TestNotifier_RampDemote_Dispatches asserts NotifyPriceGapRampDemote sends a
// single critical message including the prior/next stages and reason.
func TestNotifier_RampDemote_Dispatches(t *testing.T) {
	srv, captured, mu := startPhase14CaptureServer(t)
	defer srv.Close()
	prior := SetTelegramAPIBase(srv.URL)
	defer SetTelegramAPIBase(prior)

	tn := NewTelegram("test-token", "test-chat")
	if tn == nil {
		t.Fatal("NewTelegram returned nil")
	}
	tn.NotifyPriceGapRampDemote(2, 1, "loss_day pnl=-15.00")

	if !waitForPhase14Captured(captured, mu, 1, 2*time.Second) {
		t.Fatalf("expected 1 send, got %d", len(*captured))
	}
	mu.Lock()
	defer mu.Unlock()
	body := (*captured)[0].text
	if !strings.Contains(body, "DEMOTE") {
		t.Errorf("expected DEMOTE marker: %q", body)
	}
	if !strings.Contains(body, "Stage 2 -> 1") {
		t.Errorf("expected stage transition 2 -> 1: %q", body)
	}
	if !strings.Contains(body, "loss_day") {
		t.Errorf("expected reason in body: %q", body)
	}
}

// TestNotifier_RampForceOp_Dispatches asserts NotifyPriceGapRampForceOp sends a
// single critical message including action, prior/next, operator, and reason.
func TestNotifier_RampForceOp_Dispatches(t *testing.T) {
	srv, captured, mu := startPhase14CaptureServer(t)
	defer srv.Close()
	prior := SetTelegramAPIBase(srv.URL)
	defer SetTelegramAPIBase(prior)

	tn := NewTelegram("test-token", "test-chat")
	if tn == nil {
		t.Fatal("NewTelegram returned nil")
	}
	tn.NotifyPriceGapRampForceOp("force_promote", 1, 2, "pg-admin", "early ramp test")

	if !waitForPhase14Captured(captured, mu, 1, 2*time.Second) {
		t.Fatalf("expected 1 send, got %d", len(*captured))
	}
	mu.Lock()
	defer mu.Unlock()
	body := (*captured)[0].text
	if !strings.Contains(body, "FORCE_PROMOTE") {
		t.Errorf("expected uppercased action FORCE_PROMOTE: %q", body)
	}
	if !strings.Contains(body, "Stage 1 -> 2") {
		t.Errorf("expected stage transition 1 -> 2: %q", body)
	}
	if !strings.Contains(body, "pg-admin") {
		t.Errorf("expected operator pg-admin: %q", body)
	}
	if !strings.Contains(body, "early ramp test") {
		t.Errorf("expected reason text: %q", body)
	}
}
