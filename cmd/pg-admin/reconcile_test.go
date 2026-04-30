// Phase 14 Plan 14-04 — pg-admin reconcile subcommand tests.
package main

import (
	"context"
	"strings"
	"testing"

	"arb/internal/pricegaptrader"
)

// fakeReconcileRunner — minimal ReconcileRunner stub for cmd-level tests.
type fakeReconcileRunner struct {
	runCalls       []string
	loadCalls      []string
	loadReturn     map[string]pricegaptrader.DailyReconcileRecord
	loadExists     map[string]bool
	runForDateErr  error
	loadRecordErr  error
}

func newFakeReconcileRunner() *fakeReconcileRunner {
	return &fakeReconcileRunner{
		loadReturn: map[string]pricegaptrader.DailyReconcileRecord{},
		loadExists: map[string]bool{},
	}
}

func (f *fakeReconcileRunner) RunForDate(_ context.Context, date string) error {
	f.runCalls = append(f.runCalls, date)
	if f.runForDateErr != nil {
		return f.runForDateErr
	}
	// Synthesize a deterministic minimal record so subsequent LoadRecord
	// calls behave like the real reconciler did the work.
	f.loadReturn[date] = pricegaptrader.DailyReconcileRecord{
		SchemaVersion: 1,
		Date:          date,
		Anomalies:     pricegaptrader.DailyReconcileAnomalies{FlaggedIDs: []string{}},
	}
	f.loadExists[date] = true
	return nil
}

func (f *fakeReconcileRunner) LoadRecord(_ context.Context, date string) (pricegaptrader.DailyReconcileRecord, bool, error) {
	f.loadCalls = append(f.loadCalls, date)
	if f.loadRecordErr != nil {
		return pricegaptrader.DailyReconcileRecord{}, false, f.loadRecordErr
	}
	rec, ok := f.loadReturn[date]
	exists := f.loadExists[date]
	if !ok {
		return pricegaptrader.DailyReconcileRecord{}, exists, nil
	}
	return rec, exists, nil
}

// TestCmdReconcileRun_HappyPath — date in the past with no existing record →
// fakeReconciler.RunForDate is invoked → exit 0 + stdout "OK".
func TestCmdReconcileRun_HappyPath(t *testing.T) {
	rec := newFakeReconcileRunner()
	stdout, stderr := captureBuffers()
	rc := Run([]string{"reconcile", "run", "--date=2026-04-29"}, Dependencies{
		Reconciler: rec,
		Stdout:     stdout,
		Stderr:     stderr,
	})
	if rc != 0 {
		t.Fatalf("rc=%d, stderr=%q", rc, stderr.String())
	}
	if len(rec.runCalls) != 1 || rec.runCalls[0] != "2026-04-29" {
		t.Errorf("expected RunForDate(2026-04-29), got %v", rec.runCalls)
	}
	if !strings.Contains(stdout.String(), "OK") {
		t.Errorf("expected 'OK' in stdout, got: %q", stdout.String())
	}
}

// TestCmdReconcileRun_RejectsInvalidDate — non-YYYY-MM-DD argument → exit 2.
func TestCmdReconcileRun_RejectsInvalidDate(t *testing.T) {
	rec := newFakeReconcileRunner()
	stdout, stderr := captureBuffers()
	rc := Run([]string{"reconcile", "run", "--date=abc"}, Dependencies{
		Reconciler: rec,
		Stdout:     stdout,
		Stderr:     stderr,
	})
	if rc != 2 {
		t.Fatalf("expected rc=2, got %d (stderr=%q)", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "invalid") {
		t.Errorf("expected 'invalid' marker in stderr, got: %q", stderr.String())
	}
	if len(rec.runCalls) != 0 {
		t.Errorf("expected zero RunForDate calls on invalid date, got %d", len(rec.runCalls))
	}
}

// TestCmdReconcileRun_RejectsFutureDate — YYYY-MM-DD beyond +1 day → exit 2.
func TestCmdReconcileRun_RejectsFutureDate(t *testing.T) {
	rec := newFakeReconcileRunner()
	stdout, stderr := captureBuffers()
	rc := Run([]string{"reconcile", "run", "--date=2099-01-01"}, Dependencies{
		Reconciler: rec,
		Stdout:     stdout,
		Stderr:     stderr,
	})
	if rc != 2 {
		t.Fatalf("expected rc=2, got %d (stderr=%q)", rc, stderr.String())
	}
	if len(rec.runCalls) != 0 {
		t.Errorf("expected zero RunForDate calls on future date, got %d", len(rec.runCalls))
	}
}

// TestCmdReconcileRun_RequiresDate — missing --date= → exit 2.
func TestCmdReconcileRun_RequiresDate(t *testing.T) {
	rec := newFakeReconcileRunner()
	stdout, stderr := captureBuffers()
	rc := Run([]string{"reconcile", "run"}, Dependencies{
		Reconciler: rec,
		Stdout:     stdout,
		Stderr:     stderr,
	})
	if rc != 2 {
		t.Fatalf("expected rc=2, got %d (stderr=%q)", rc, stderr.String())
	}
}

// TestCmdReconcileShow_NotFound_Returns1 — reconciler.LoadRecord exists=false → exit 1.
func TestCmdReconcileShow_NotFound_Returns1(t *testing.T) {
	rec := newFakeReconcileRunner()
	// no record stored
	stdout, stderr := captureBuffers()
	rc := Run([]string{"reconcile", "show", "--date=2026-04-29"}, Dependencies{
		Reconciler: rec,
		Stdout:     stdout,
		Stderr:     stderr,
	})
	if rc != 1 {
		t.Fatalf("expected rc=1, got %d (stdout=%q stderr=%q)", rc, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "no reconcile record") {
		t.Errorf("expected 'no reconcile record' in stderr, got: %q", stderr.String())
	}
}

// TestCmdReconcileShow_HappyPath — pre-seeded record renders as JSON.
func TestCmdReconcileShow_HappyPath(t *testing.T) {
	rec := newFakeReconcileRunner()
	rec.loadReturn["2026-04-29"] = pricegaptrader.DailyReconcileRecord{
		SchemaVersion: 1,
		Date:          "2026-04-29",
		Anomalies:     pricegaptrader.DailyReconcileAnomalies{FlaggedIDs: []string{}},
	}
	rec.loadExists["2026-04-29"] = true

	stdout, stderr := captureBuffers()
	rc := Run([]string{"reconcile", "show", "--date=2026-04-29"}, Dependencies{
		Reconciler: rec,
		Stdout:     stdout,
		Stderr:     stderr,
	})
	if rc != 0 {
		t.Fatalf("expected rc=0, got %d (stderr=%q)", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"date": "2026-04-29"`) {
		t.Errorf("expected JSON with date 2026-04-29, got: %q", stdout.String())
	}
}

// TestCmdReconcile_RequiresReconciler — Dependencies.Reconciler nil → exit 1.
func TestCmdReconcile_RequiresReconciler(t *testing.T) {
	stdout, stderr := captureBuffers()
	rc := Run([]string{"reconcile", "run", "--date=2026-04-29"}, Dependencies{
		Reconciler: nil,
		Stdout:     stdout,
		Stderr:     stderr,
	})
	if rc != 1 {
		t.Fatalf("expected rc=1, got %d (stderr=%q)", rc, stderr.String())
	}
}
