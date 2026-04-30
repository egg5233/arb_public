package pricegaptrader

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"arb/internal/config"
	"arb/internal/models"
)

// TestNoopBroadcaster_DoesNotPanic — the zero-behavior broadcaster must accept
// every call without side effect and without panicking.
func TestNoopBroadcaster_DoesNotPanic(t *testing.T) {
	var b Broadcaster = NoopBroadcaster{}
	b.BroadcastPriceGapPositions(nil)
	b.BroadcastPriceGapPositions([]*models.PriceGapPosition{{ID: "x"}})
	b.BroadcastPriceGapEvent(PriceGapEvent{Type: "entry"})
	b.BroadcastPriceGapCandidateUpdate(PriceGapCandidateUpdate{Symbol: "SOONUSDT", Disabled: true})
}

// TestTracker_DefaultBroadcasterIsNoop — NewTracker must install
// NoopBroadcaster so tracker code can unconditionally call broadcaster methods.
func TestTracker_DefaultBroadcasterIsNoop(t *testing.T) {
	tr := NewTracker(nil, nil, nil, &config.Config{})
	if tr.broadcaster == nil {
		t.Fatal("broadcaster nil after NewTracker — should default to NoopBroadcaster")
	}
	if _, ok := tr.broadcaster.(NoopBroadcaster); !ok {
		t.Fatalf("default broadcaster: got %T, want NoopBroadcaster", tr.broadcaster)
	}
}

// mockBroadcaster records calls so we can assert the DI field is reachable.
type mockBroadcaster struct {
	positions int64
	events    int64
	candidate int64
}

func (m *mockBroadcaster) BroadcastPriceGapPositions(_ []*models.PriceGapPosition) {
	atomic.AddInt64(&m.positions, 1)
}

func (m *mockBroadcaster) BroadcastPriceGapEvent(PriceGapEvent) {
	atomic.AddInt64(&m.events, 1)
}

func (m *mockBroadcaster) BroadcastPriceGapCandidateUpdate(PriceGapCandidateUpdate) {
	atomic.AddInt64(&m.candidate, 1)
}

// TestTracker_SetBroadcaster_Reachable — SetBroadcaster installs a custom
// broadcaster, and the tracker's field routes every call into it.
func TestTracker_SetBroadcaster_Reachable(t *testing.T) {
	tr := NewTracker(nil, nil, nil, &config.Config{})
	mb := &mockBroadcaster{}
	tr.SetBroadcaster(mb)

	tr.broadcaster.BroadcastPriceGapPositions(nil)
	tr.broadcaster.BroadcastPriceGapEvent(PriceGapEvent{})
	tr.broadcaster.BroadcastPriceGapCandidateUpdate(PriceGapCandidateUpdate{})

	if atomic.LoadInt64(&mb.positions) != 1 {
		t.Errorf("positions calls: got %d, want 1", mb.positions)
	}
	if atomic.LoadInt64(&mb.events) != 1 {
		t.Errorf("events calls: got %d, want 1", mb.events)
	}
	if atomic.LoadInt64(&mb.candidate) != 1 {
		t.Errorf("candidate calls: got %d, want 1", mb.candidate)
	}
}

// TestTracker_SetBroadcaster_NilRevertsToNoop — passing nil must restore
// NoopBroadcaster so callers can never see a nil broadcaster field.
func TestTracker_SetBroadcaster_NilRevertsToNoop(t *testing.T) {
	tr := NewTracker(nil, nil, nil, &config.Config{})
	tr.SetBroadcaster(&mockBroadcaster{})
	tr.SetBroadcaster(nil)
	if _, ok := tr.broadcaster.(NoopBroadcaster); !ok {
		t.Fatalf("SetBroadcaster(nil) broadcaster: got %T, want NoopBroadcaster", tr.broadcaster)
	}
}

// Phase 14 Plan 14-04 notifier tests.
//
// Dispatch-shape assertions for NotifyPriceGapDailyDigest +
// NotifyPriceGapReconcileFailure live in internal/notify (alongside other
// telegram_*_test.go files) — placing them here would create an import
// cycle (pricegaptrader imports notify via pricegap_assert.go test build).
//
// This file keeps the static allowlist invariant lock + Wave-0 stub
// stand-ins.

// TestNotifier_DailyDigest_DispatchesNonCritical lives in
// internal/notify/telegram_pricegap_phase14_test.go (Plan 14-04). The Wave-0
// stub is replaced here by a thin sibling that re-asserts the implementation
// exists at compile time (NoopNotifier conforms to the widened interface).
func TestNotifier_DailyDigest_DispatchesNonCritical(t *testing.T) {
	var n PriceGapNotifier = NoopNotifier{}
	rec := DailyReconcileRecord{Anomalies: DailyReconcileAnomalies{FlaggedIDs: []string{}}}
	n.NotifyPriceGapDailyDigest("2026-04-29", rec, models.RampState{CurrentStage: 1})
	// Real dispatch behavior: see internal/notify/telegram_pricegap_phase14_test.go.
}

// TestNotifier_ReconcileFailure_DispatchesCritical — see comment above.
func TestNotifier_ReconcileFailure_DispatchesCritical(t *testing.T) {
	var n PriceGapNotifier = NoopNotifier{}
	n.NotifyPriceGapReconcileFailure("2026-04-29", nil)
	// Real dispatch behavior: see internal/notify/telegram_pricegap_phase14_test.go.
}

// TestNotifier_AddsRampToAllowlist verifies the priceGapGateAllowlist in
// telegram.go contains the "ramp" entry — the static check that locks the
// allowlist invariant against future regression. We open the source file
// (relative to runtime caller) and regex-match the allowlist body.
func TestNotifier_AddsRampToAllowlist(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	telegramGo := filepath.Join(filepath.Dir(thisFile), "..", "notify", "telegram.go")
	src, err := os.ReadFile(telegramGo)
	if err != nil {
		t.Fatalf("read telegram.go: %v", err)
	}
	re := regexp.MustCompile(`(?s)priceGapGateAllowlist\s*=\s*map\[string\]struct\{\}\{(.+?)\n\}`)
	m := re.FindStringSubmatch(string(src))
	if len(m) < 2 {
		t.Fatalf("priceGapGateAllowlist literal not found in %s", telegramGo)
	}
	body := m[1]
	if !strings.Contains(body, `"ramp"`) {
		t.Errorf(`priceGapGateAllowlist missing "ramp" entry; allowlist body: %s`, body)
	}
}
