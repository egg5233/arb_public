// Phase 14 Plan 14-04 — pg-admin ramp subcommand tests.
package main

import (
	"strings"
	"sync"
	"testing"
	"time"

	"arb/internal/models"
)

// fakeRampOps — minimal RampOps double for cmd-level tests.
type fakeRampOps struct {
	mu                sync.Mutex
	state             models.RampState
	forcePromoteCalls []rampOpCall
	forceDemoteCalls  []rampOpCall
	resetCalls        []rampOpCall
	forcePromoteErr   error
	forceDemoteErr    error
	resetErr          error
}

type rampOpCall struct {
	operator string
	reason   string
}

func (f *fakeRampOps) Snapshot() models.RampState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}

func (f *fakeRampOps) ForcePromote(operator, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forcePromoteCalls = append(f.forcePromoteCalls, rampOpCall{operator, reason})
	if f.forcePromoteErr != nil {
		return f.forcePromoteErr
	}
	if f.state.CurrentStage < 3 {
		f.state.CurrentStage++
	}
	return nil
}

func (f *fakeRampOps) ForceDemote(operator, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forceDemoteCalls = append(f.forceDemoteCalls, rampOpCall{operator, reason})
	if f.forceDemoteErr != nil {
		return f.forceDemoteErr
	}
	if f.state.CurrentStage > 1 {
		f.state.CurrentStage--
		f.state.CleanDayCounter = 0
		f.state.DemoteCount++
	}
	return nil
}

func (f *fakeRampOps) Reset(operator, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resetCalls = append(f.resetCalls, rampOpCall{operator, reason})
	if f.resetErr != nil {
		return f.resetErr
	}
	f.state = models.RampState{CurrentStage: 1}
	return nil
}

// TestCmdRampShow_OutputsAllFiveFields verifies all 5 RampState field names
// appear in the rendered table — regression-locks the field set against
// future pruning.
func TestCmdRampShow_OutputsAllFiveFields(t *testing.T) {
	r := &fakeRampOps{state: models.RampState{
		CurrentStage:    2,
		CleanDayCounter: 3,
		LastEvalTs:      time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC),
		LastLossDayTs:   time.Date(2026, 4, 25, 0, 30, 0, 0, time.UTC),
		DemoteCount:     1,
	}}
	stdout, stderr := captureBuffers()
	rc := Run([]string{"ramp", "show"}, Dependencies{
		Ramp:   r,
		Stdout: stdout,
		Stderr: stderr,
	})
	if rc != 0 {
		t.Fatalf("expected rc=0, got %d (stderr=%q)", rc, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"current_stage",
		"clean_day_counter",
		"last_eval_ts",
		"last_loss_day_ts",
		"demote_count",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected field %q in ramp show output, got: %q", want, out)
		}
	}
	if !strings.Contains(out, "2") {
		t.Errorf("expected current_stage value 2 in output, got: %q", out)
	}
}

// TestCmdRampShow_RendersZeroTimeAsNever asserts (never) marker for zero ts.
func TestCmdRampShow_RendersZeroTimeAsNever(t *testing.T) {
	r := &fakeRampOps{state: models.RampState{CurrentStage: 1}}
	stdout, stderr := captureBuffers()
	rc := Run([]string{"ramp", "show"}, Dependencies{
		Ramp:   r,
		Stdout: stdout,
		Stderr: stderr,
	})
	if rc != 0 {
		t.Fatalf("expected rc=0, got %d (stderr=%q)", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), "(never)") {
		t.Errorf("expected '(never)' for zero timestamps, got: %q", stdout.String())
	}
}

// TestCmdRampForcePromote_PreservesCounter — fakeRamp.ForcePromote called
// with operator="pg-admin"; no ForceDemote/Reset calls.
func TestCmdRampForcePromote_PreservesCounter(t *testing.T) {
	r := &fakeRampOps{state: models.RampState{CurrentStage: 1, CleanDayCounter: 4}}
	stdout, stderr := captureBuffers()
	rc := Run([]string{"ramp", "force-promote", "--reason=early ramp test"}, Dependencies{
		Ramp:   r,
		Stdout: stdout,
		Stderr: stderr,
	})
	if rc != 0 {
		t.Fatalf("expected rc=0, got %d (stderr=%q)", rc, stderr.String())
	}
	if len(r.forcePromoteCalls) != 1 {
		t.Fatalf("expected 1 ForcePromote call, got %d", len(r.forcePromoteCalls))
	}
	got := r.forcePromoteCalls[0]
	if got.operator != "pg-admin" {
		t.Errorf("expected operator=pg-admin, got %q", got.operator)
	}
	if got.reason != "early ramp test" {
		t.Errorf("expected reason='early ramp test', got %q", got.reason)
	}
	if len(r.forceDemoteCalls) != 0 || len(r.resetCalls) != 0 {
		t.Errorf("unexpected concurrent ops: demote=%d reset=%d", len(r.forceDemoteCalls), len(r.resetCalls))
	}
	if !strings.Contains(stdout.String(), "force-promoted") {
		t.Errorf("expected 'force-promoted' in stdout, got: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "counter preserved") {
		t.Errorf("expected 'counter preserved' marker in stdout, got: %q", stdout.String())
	}
}

// TestCmdRampForceDemote_ZeroesCounter — fakeRamp.ForceDemote called.
func TestCmdRampForceDemote_ZeroesCounter(t *testing.T) {
	r := &fakeRampOps{state: models.RampState{CurrentStage: 2, CleanDayCounter: 5}}
	stdout, stderr := captureBuffers()
	rc := Run([]string{"ramp", "force-demote", "--reason=ops investigation"}, Dependencies{
		Ramp:   r,
		Stdout: stdout,
		Stderr: stderr,
	})
	if rc != 0 {
		t.Fatalf("expected rc=0, got %d (stderr=%q)", rc, stderr.String())
	}
	if len(r.forceDemoteCalls) != 1 {
		t.Fatalf("expected 1 ForceDemote call, got %d", len(r.forceDemoteCalls))
	}
	if r.forceDemoteCalls[0].operator != "pg-admin" {
		t.Errorf("expected operator=pg-admin, got %q", r.forceDemoteCalls[0].operator)
	}
	if !strings.Contains(stdout.String(), "force-demoted") {
		t.Errorf("expected 'force-demoted' in stdout, got: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "counter zeroed") {
		t.Errorf("expected 'counter zeroed' marker in stdout, got: %q", stdout.String())
	}
}

// TestCmdRampReset_ZeroesAll — fakeRamp.Reset called; counter & demote_count zeroed.
func TestCmdRampReset_ZeroesAll(t *testing.T) {
	r := &fakeRampOps{state: models.RampState{
		CurrentStage:    3,
		CleanDayCounter: 6,
		DemoteCount:     2,
	}}
	stdout, stderr := captureBuffers()
	rc := Run([]string{"ramp", "reset", "--reason=quarterly review"}, Dependencies{
		Ramp:   r,
		Stdout: stdout,
		Stderr: stderr,
	})
	if rc != 0 {
		t.Fatalf("expected rc=0, got %d (stderr=%q)", rc, stderr.String())
	}
	if len(r.resetCalls) != 1 {
		t.Fatalf("expected 1 Reset call, got %d", len(r.resetCalls))
	}
	if r.resetCalls[0].operator != "pg-admin" {
		t.Errorf("expected operator=pg-admin, got %q", r.resetCalls[0].operator)
	}
	if r.resetCalls[0].reason != "quarterly review" {
		t.Errorf("expected reason='quarterly review', got %q", r.resetCalls[0].reason)
	}
	if !strings.Contains(stdout.String(), "ramp reset") {
		t.Errorf("expected 'ramp reset' in stdout, got: %q", stdout.String())
	}
}

// TestCmdRamp_RequiresRampDep — Dependencies.Ramp nil → exit 1.
func TestCmdRamp_RequiresRampDep(t *testing.T) {
	stdout, stderr := captureBuffers()
	rc := Run([]string{"ramp", "show"}, Dependencies{
		Ramp:   nil,
		Stdout: stdout,
		Stderr: stderr,
	})
	if rc != 1 {
		t.Fatalf("expected rc=1, got %d (stderr=%q)", rc, stderr.String())
	}
}

// TestCmdRamp_UnknownSubcommand — invalid subcommand → exit 2.
func TestCmdRamp_UnknownSubcommand(t *testing.T) {
	r := &fakeRampOps{}
	stdout, stderr := captureBuffers()
	rc := Run([]string{"ramp", "wat"}, Dependencies{
		Ramp:   r,
		Stdout: stdout,
		Stderr: stderr,
	})
	if rc != 2 {
		t.Fatalf("expected rc=2, got %d (stderr=%q)", rc, stderr.String())
	}
}
