package pricegaptrader

import (
	"errors"
	"math"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
)

// Wave 0 stubs for the BreakerController state machine + lifecycle.
// Plan 15-03 implements two-strike + blackout + recovery + boot guard.

// fakeBreakerStore — satisfies BreakerStateLoader for IsPaperModeActive tests.
type fakeBreakerStore struct {
	state  models.BreakerState
	exists bool
	err    error
}

func (f *fakeBreakerStore) LoadBreakerState() (models.BreakerState, bool, error) {
	return f.state, f.exists, f.err
}

// TestBreaker_DisabledByDefault — when PriceGapBreakerEnabled=false the
// daemon performs zero work (no Redis reads, no PnL aggregation).
func TestBreaker_DisabledByDefault(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-03 implements BreakerController")
}

// TestBreaker_FreshBootInit — D-Discretion boot guard: missing pg:breaker:state
// HASH on enabled-breaker startup writes a zero-init state.
func TestBreaker_FreshBootInit(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-03 implements boot guard")
}

// TestBreaker_BootGuard_PreservesExistingTrip — if pg:breaker:state exists
// with sticky=MaxInt64 (post-trip), boot must NOT zero the sticky flag.
func TestBreaker_BootGuard_PreservesExistingTrip(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-03 implements boot guard")
}

// TestBreaker_SingleStrike_NoTrip — one in-breach tick sets pending=1,
// Strike1Ts; does NOT trip the breaker (D-05 two-strike rule).
func TestBreaker_SingleStrike_NoTrip(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-03 implements two-strike state machine")
}

// TestBreaker_TwoStrikeTrips — two in-breach ticks ≥5 min apart trip the
// breaker (sticky flag set, candidates paused, alerts dispatched).
func TestBreaker_TwoStrikeTrips(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-03 implements two-strike state machine")
}

// TestBreaker_TwoStrikeRequiresTwoSeparateEvaluations — the ≥5-min-apart
// requirement: two in-breach ticks <5 min apart must NOT trip.
func TestBreaker_TwoStrikeRequiresTwoSeparateEvaluations(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-03 implements two-strike spacing enforcement")
}

// TestBreaker_RecoveryClearsPendingStrike — D-08: a non-breach tick between
// Strike-1 and Strike-2 zeroes pending_strike + Strike1Ts.
func TestBreaker_RecoveryClearsPendingStrike(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-03 implements D-08 PnL recovery clear")
}

// TestBreaker_BlackoutSuppression — D-03: wall-clock minute in [:04, :05:30)
// suppresses the entire eval tick (no PnL fetch, no state mutation).
func TestBreaker_BlackoutSuppression(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-03 implements D-03 blackout gate")
}

// TestBreaker_PendingSurvivesBlackout — D-04: a blackout-suppressed tick
// does NOT clear pending_strike. State carries through.
func TestBreaker_PendingSurvivesBlackout(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-03 implements D-04 blackout state preservation")
}

// TestBreaker_StateSurvivesRestart — D-05 kill-9: persisted HASH state
// reloads cleanly on BreakerController restart (pending_strike carries).
func TestBreaker_StateSurvivesRestart(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-03 implements restart survival via HASH persistence")
}

// TestBreaker_TripOrdering_StickyFirstWhenStepsFail — D-15 atomicity anchor.
// Step 1 (sticky flag) MUST persist even when steps 2-5 fail (LIST append,
// candidate pause, Telegram, WS broadcast).
func TestBreaker_TripOrdering_StickyFirstWhenStepsFail(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-03 implements D-15 trip ordering")
}

// TestTracker_IsPaperModeActive_CfgFlagOnly — cfg=true overrides everything.
// No breaker store wired; helper short-circuits on cfg flag.
func TestTracker_IsPaperModeActive_CfgFlagOnly(t *testing.T) {
	tr := &Tracker{cfg: &config.Config{PriceGapPaperMode: true}}
	got, err := tr.IsPaperModeActive(t.Context())
	if err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
	if !got {
		t.Fatalf("got=false, want true (cfg flag short-circuit)")
	}
}

// TestTracker_IsPaperModeActive_CfgFlagFalse_NoSticky — cfg=false, sticky=0:
// returns live mode (false).
func TestTracker_IsPaperModeActive_CfgFlagFalse_NoSticky(t *testing.T) {
	tr := &Tracker{
		cfg:          &config.Config{PriceGapPaperMode: false},
		breakerStore: &fakeBreakerStore{state: models.BreakerState{PaperModeStickyUntil: 0}, exists: true},
	}
	got, err := tr.IsPaperModeActive(t.Context())
	if err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
	if got {
		t.Fatalf("got=true, want false (live mode, no sticky)")
	}
}

// TestTracker_IsPaperModeActive_StickyMaxInt64 — sticky-until-operator (D-07
// sentinel): cfg=false but PaperModeStickyUntil=MaxInt64 → forced paper.
func TestTracker_IsPaperModeActive_StickyMaxInt64(t *testing.T) {
	tr := &Tracker{
		cfg:          &config.Config{PriceGapPaperMode: false},
		breakerStore: &fakeBreakerStore{state: models.BreakerState{PaperModeStickyUntil: math.MaxInt64}, exists: true},
	}
	got, err := tr.IsPaperModeActive(t.Context())
	if err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
	if !got {
		t.Fatalf("got=false, want true (sticky=MaxInt64 forces paper)")
	}
}

// TestTracker_IsPaperModeActive_StickyFutureTimestamp — timed sticky:
// PaperModeStickyUntil = now+1h → forced paper while window is active.
func TestTracker_IsPaperModeActive_StickyFutureTimestamp(t *testing.T) {
	future := time.Now().Add(1 * time.Hour).UnixMilli()
	tr := &Tracker{
		cfg:          &config.Config{PriceGapPaperMode: false},
		breakerStore: &fakeBreakerStore{state: models.BreakerState{PaperModeStickyUntil: future}, exists: true},
	}
	got, err := tr.IsPaperModeActive(t.Context())
	if err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
	if !got {
		t.Fatalf("got=false, want true (sticky window active)")
	}
}

// TestTracker_IsPaperModeActive_StickyExpired — sticky window expired (now > sticky_until):
// returns live mode.
func TestTracker_IsPaperModeActive_StickyExpired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour).UnixMilli()
	tr := &Tracker{
		cfg:          &config.Config{PriceGapPaperMode: false},
		breakerStore: &fakeBreakerStore{state: models.BreakerState{PaperModeStickyUntil: past}, exists: true},
	}
	got, err := tr.IsPaperModeActive(t.Context())
	if err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
	if got {
		t.Fatalf("got=true, want false (sticky expired, falls back to live)")
	}
}

// TestTracker_IsPaperModeActive_RedisError_FailsToPaper — Pitfall 8:
// LoadBreakerState err → fail-safe to paper, helper returns (true, err).
func TestTracker_IsPaperModeActive_RedisError_FailsToPaper(t *testing.T) {
	wantErr := errors.New("redis down")
	tr := &Tracker{
		cfg:          &config.Config{PriceGapPaperMode: false},
		breakerStore: &fakeBreakerStore{err: wantErr},
	}
	got, err := tr.IsPaperModeActive(t.Context())
	if err == nil {
		t.Fatalf("err=nil, want non-nil (Redis error must surface)")
	}
	if !got {
		t.Fatalf("got=false, want true (fail-safe-to-paper on Redis error)")
	}
}

// TestTracker_IsPaperModeActive_NilStore — pre-15-01 / pre-wired path: no
// breakerStore set. Helper falls back to cfg flag value.
func TestTracker_IsPaperModeActive_NilStore(t *testing.T) {
	tr := &Tracker{cfg: &config.Config{PriceGapPaperMode: false}}
	got, err := tr.IsPaperModeActive(t.Context())
	if err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
	if got {
		t.Fatalf("got=true, want false (cfg=false, no store, returns cfg flag)")
	}
}
