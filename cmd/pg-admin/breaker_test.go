// Phase 15 Plan 15-04 — pg-admin breaker subcommand tests.
package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"arb/internal/models"
)

// fakeBreakerOps — minimal BreakerOps double for cmd-level tests.
type fakeBreakerOps struct {
	mu             sync.Mutex
	state          models.BreakerState
	snapshotErr    error
	recoverCalls   []string
	recoverErr     error
	testFireCalls  []bool // dryRun arg per call
	testFireRec    models.BreakerTripRecord
	testFireErr    error
}

func (f *fakeBreakerOps) Snapshot() (models.BreakerState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state, f.snapshotErr
}

func (f *fakeBreakerOps) Recover(_ context.Context, operator string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recoverCalls = append(f.recoverCalls, operator)
	return f.recoverErr
}

func (f *fakeBreakerOps) TestFire(_ context.Context, dryRun bool) (models.BreakerTripRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.testFireCalls = append(f.testFireCalls, dryRun)
	return f.testFireRec, f.testFireErr
}

// fakeBreakerTrips — minimal BreakerTripsReader for show subcommand.
type fakeBreakerTrips struct {
	trips []models.BreakerTripRecord // index 0 = newest
}

func (f *fakeBreakerTrips) LoadBreakerTripAt(index int64) (models.BreakerTripRecord, bool, error) {
	if index < 0 || int(index) >= len(f.trips) {
		return models.BreakerTripRecord{}, false, nil
	}
	return f.trips[int(index)], true, nil
}

// TestPgAdminBreakerRecover_TypedPhraseRequired — D-09 + D-12: stdin "wrong"
// → exit non-zero + "expected 'RECOVER'" in stderr; stdin "RECOVER" → calls
// Recover, exit 0, "Recovered" in stdout.
func TestPgAdminBreakerRecover_TypedPhraseRequired(t *testing.T) {
	t.Run("missing --confirm", func(t *testing.T) {
		ops := &fakeBreakerOps{state: models.BreakerState{PaperModeStickyUntil: 1234}}
		stdout, stderr := captureBuffers()
		rc := Run([]string{"breaker", "recover"}, Dependencies{
			Breaker: ops,
			Stdin:   strings.NewReader("RECOVER\n"),
			Stdout:  stdout,
			Stderr:  stderr,
		})
		if rc != 2 {
			t.Fatalf("rc=%d, stderr=%q", rc, stderr.String())
		}
		if len(ops.recoverCalls) != 0 {
			t.Errorf("Recover called %d times, want 0", len(ops.recoverCalls))
		}
	})

	t.Run("wrong phrase rejected", func(t *testing.T) {
		ops := &fakeBreakerOps{state: models.BreakerState{PaperModeStickyUntil: 1234}}
		stdout, stderr := captureBuffers()
		rc := Run([]string{"breaker", "recover", "--confirm"}, Dependencies{
			Breaker: ops,
			Stdin:   strings.NewReader("recover\n"),
			Stdout:  stdout,
			Stderr:  stderr,
		})
		if rc != 2 {
			t.Fatalf("rc=%d, want 2 (lowercase rejected); stderr=%q", rc, stderr.String())
		}
		if !strings.Contains(stderr.String(), "RECOVER") {
			t.Errorf("expected stderr to mention RECOVER, got: %q", stderr.String())
		}
		if len(ops.recoverCalls) != 0 {
			t.Errorf("Recover called when phrase mismatched")
		}
	})

	t.Run("correct phrase invokes Recover", func(t *testing.T) {
		ops := &fakeBreakerOps{state: models.BreakerState{PaperModeStickyUntil: 1234}}
		stdout, stderr := captureBuffers()
		rc := Run([]string{"breaker", "recover", "--confirm"}, Dependencies{
			Breaker: ops,
			Stdin:   strings.NewReader("RECOVER\n"),
			Stdout:  stdout,
			Stderr:  stderr,
		})
		if rc != 0 {
			t.Fatalf("rc=%d, stderr=%q", rc, stderr.String())
		}
		if len(ops.recoverCalls) != 1 {
			t.Fatalf("Recover called %d times, want 1", len(ops.recoverCalls))
		}
		if ops.recoverCalls[0] != "pg-admin" {
			t.Errorf("Recover operator=%q want pg-admin", ops.recoverCalls[0])
		}
		if !strings.Contains(stdout.String(), "Recovered") {
			t.Errorf("expected 'Recovered' in stdout, got: %q", stdout.String())
		}
	})

	t.Run("Recover error returns non-zero", func(t *testing.T) {
		ops := &fakeBreakerOps{
			state:      models.BreakerState{PaperModeStickyUntil: 0},
			recoverErr: errors.New("breaker not tripped (sticky=0)"),
		}
		stdout, stderr := captureBuffers()
		rc := Run([]string{"breaker", "recover", "--confirm"}, Dependencies{
			Breaker: ops,
			Stdin:   strings.NewReader("RECOVER\n"),
			Stdout:  stdout,
			Stderr:  stderr,
		})
		if rc != 1 {
			t.Fatalf("rc=%d, want 1 (Recover err)", rc)
		}
		if !strings.Contains(stderr.String(), "breaker not tripped") {
			t.Errorf("expected error in stderr, got: %q", stderr.String())
		}
	})
}

// TestPgAdminBreakerTestFire_TypedPhraseRequired — D-13 + D-14.
func TestPgAdminBreakerTestFire_TypedPhraseRequired(t *testing.T) {
	t.Run("missing --confirm", func(t *testing.T) {
		ops := &fakeBreakerOps{}
		stdout, stderr := captureBuffers()
		rc := Run([]string{"breaker", "test-fire"}, Dependencies{
			Breaker: ops,
			Stdin:   strings.NewReader("TEST-FIRE\n"),
			Stdout:  stdout,
			Stderr:  stderr,
		})
		if rc != 2 {
			t.Fatalf("rc=%d, want 2 (missing --confirm)", rc)
		}
		if len(ops.testFireCalls) != 0 {
			t.Errorf("TestFire called when --confirm missing")
		}
	})

	t.Run("wrong phrase rejected", func(t *testing.T) {
		ops := &fakeBreakerOps{}
		stdout, stderr := captureBuffers()
		rc := Run([]string{"breaker", "test-fire", "--confirm"}, Dependencies{
			Breaker: ops,
			Stdin:   strings.NewReader("test-fire\n"),
			Stdout:  stdout,
			Stderr:  stderr,
		})
		if rc != 2 {
			t.Fatalf("rc=%d, want 2 (lowercase phrase rejected)", rc)
		}
		if !strings.Contains(stderr.String(), "TEST-FIRE") {
			t.Errorf("expected stderr to mention TEST-FIRE")
		}
		if len(ops.testFireCalls) != 0 {
			t.Errorf("TestFire called when phrase mismatched")
		}
	})

	t.Run("default real trip", func(t *testing.T) {
		ops := &fakeBreakerOps{
			testFireRec: models.BreakerTripRecord{
				TripPnLUSDT: -120,
				Threshold:   -50,
				Source:      "test_fire",
				TripTs:      1234567890123,
			},
		}
		stdout, stderr := captureBuffers()
		rc := Run([]string{"breaker", "test-fire", "--confirm"}, Dependencies{
			Breaker: ops,
			Stdin:   strings.NewReader("TEST-FIRE\n"),
			Stdout:  stdout,
			Stderr:  stderr,
		})
		if rc != 0 {
			t.Fatalf("rc=%d, stderr=%q", rc, stderr.String())
		}
		if len(ops.testFireCalls) != 1 {
			t.Fatalf("TestFire calls=%d, want 1", len(ops.testFireCalls))
		}
		if ops.testFireCalls[0] != false {
			t.Errorf("TestFire dryRun=%v, want false (default real)", ops.testFireCalls[0])
		}
		if !strings.Contains(stdout.String(), "TRIPPED") {
			t.Errorf("expected 'TRIPPED' in stdout, got: %q", stdout.String())
		}
		// WARNING line must surface so operators know it's a real trip.
		if !strings.Contains(stdout.String(), "WARNING: default behavior is REAL TRIP") {
			t.Errorf("expected WARNING line in stdout, got: %q", stdout.String())
		}
	})

	t.Run("--dry-run flag", func(t *testing.T) {
		ops := &fakeBreakerOps{
			testFireRec: models.BreakerTripRecord{
				TripPnLUSDT: -120,
				Threshold:   -50,
				Source:      "test_fire_dry_run",
			},
		}
		stdout, stderr := captureBuffers()
		rc := Run([]string{"breaker", "test-fire", "--confirm", "--dry-run"}, Dependencies{
			Breaker: ops,
			Stdin:   strings.NewReader("TEST-FIRE\n"),
			Stdout:  stdout,
			Stderr:  stderr,
		})
		if rc != 0 {
			t.Fatalf("rc=%d, stderr=%q", rc, stderr.String())
		}
		if len(ops.testFireCalls) != 1 {
			t.Fatalf("TestFire calls=%d, want 1", len(ops.testFireCalls))
		}
		if ops.testFireCalls[0] != true {
			t.Errorf("TestFire dryRun=%v, want true", ops.testFireCalls[0])
		}
		if !strings.Contains(stdout.String(), "DRY RUN") {
			t.Errorf("expected 'DRY RUN' in stdout, got: %q", stdout.String())
		}
		if strings.Contains(stdout.String(), "WARNING: default behavior is REAL TRIP") {
			t.Errorf("WARNING should NOT appear in dry-run mode, got: %q", stdout.String())
		}
	})
}

// TestPgAdminBreakerShow_PrintsState — read-only; no typed-phrase needed.
// Output must contain status, all 5 state fields, and at least one trip row
// when trips are present.
func TestPgAdminBreakerShow_PrintsState(t *testing.T) {
	ops := &fakeBreakerOps{state: models.BreakerState{
		PendingStrike:        1,
		Strike1Ts:            1234567890123,
		LastEvalTs:           1234567899999,
		LastEvalPnLUSDT:      -75.5,
		PaperModeStickyUntil: 0,
	}}
	trips := &fakeBreakerTrips{trips: []models.BreakerTripRecord{
		{TripTs: 1234567890123, TripPnLUSDT: -100, Threshold: -50, Source: "live"},
	}}
	stdout, stderr := captureBuffers()
	rc := Run([]string{"breaker", "show"}, Dependencies{
		Breaker:      ops,
		BreakerTrips: trips,
		Stdout:       stdout,
		Stderr:       stderr,
	})
	if rc != 0 {
		t.Fatalf("rc=%d, stderr=%q", rc, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"status",
		"Armed", // sticky=0 → armed
		"pending_strike",
		"strike1_ts",
		"last_eval_ts",
		"last_eval_pnl_usdt",
		"paper_mode_sticky_until",
		"Recent trips",
		"-100.00", // trip pnl
		"live",    // trip source
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in show output, got: %q", want, out)
		}
	}
}

// TestPgAdminBreaker_RequiresBreakerDep — Dependencies.Breaker nil → exit 1.
func TestPgAdminBreaker_RequiresBreakerDep(t *testing.T) {
	stdout, stderr := captureBuffers()
	rc := Run([]string{"breaker", "show"}, Dependencies{
		Breaker: nil,
		Stdout:  stdout,
		Stderr:  stderr,
	})
	if rc != 1 {
		t.Fatalf("rc=%d, want 1", rc)
	}
}

// TestPgAdminBreaker_UnknownSubcommand — exit 2.
func TestPgAdminBreaker_UnknownSubcommand(t *testing.T) {
	ops := &fakeBreakerOps{}
	stdout, stderr := captureBuffers()
	rc := Run([]string{"breaker", "wat"}, Dependencies{
		Breaker: ops,
		Stdout:  stdout,
		Stderr:  stderr,
	})
	if rc != 2 {
		t.Fatalf("rc=%d, want 2", rc)
	}
}
