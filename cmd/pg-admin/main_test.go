package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"arb/internal/database"
	"arb/internal/models"

	"github.com/alicebob/miniredis/v2"
)

// newTestClient spins up miniredis + a *database.Client pointing at it.
// Mirrors internal/database/pricegap_state_test.go pattern.
func newTestClient(t *testing.T) (*database.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	db, err := database.New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, mr
}

// captureBuffers returns two buffers usable as stdout/stderr in the
// io.Writer-based cmd* helpers (Plan 11-03 refactor).
func captureBuffers() (*bytes.Buffer, *bytes.Buffer) {
	return &bytes.Buffer{}, &bytes.Buffer{}
}

// TestPgAdmin_EnableClearsFlag — pre-seed disabled flag, cmdEnable wipes it.
func TestPgAdmin_EnableClearsFlag(t *testing.T) {
	db, mr := newTestClient(t)

	// Seed: pg:candidate:disabled:SOON = "auto:exec_quality"
	mr.Set("pg:candidate:disabled:SOON", "auto:exec_quality")

	stdout, stderr := captureBuffers()
	if rc := cmdEnable(db, "SOON", stdout, stderr); rc != 0 {
		t.Fatalf("cmdEnable rc=%d, stderr=%q", rc, stderr.String())
	}

	if mr.Exists("pg:candidate:disabled:SOON") {
		t.Fatalf("expected disabled flag cleared, still present")
	}
	if !strings.Contains(stdout.String(), "enabled") {
		t.Fatalf("expected stdout to contain 'enabled', got: %q", stdout.String())
	}
}

// TestPgAdmin_DisableSetsFlag — empty miniredis, cmdDisable sets flag+reason.
func TestPgAdmin_DisableSetsFlag(t *testing.T) {
	db, mr := newTestClient(t)

	stdout, stderr := captureBuffers()
	if rc := cmdDisable(db, "DRIFT", "manual test", stdout, stderr); rc != 0 {
		t.Fatalf("cmdDisable rc=%d, stderr=%q", rc, stderr.String())
	}

	v, err := mr.Get("pg:candidate:disabled:DRIFT")
	if err != nil {
		t.Fatalf("expected disabled flag set, Get err: %v", err)
	}
	if !strings.Contains(v, "manual test") {
		t.Fatalf("expected reason 'manual test' in value, got: %q", v)
	}
	out := stdout.String()
	if !strings.Contains(out, "DRIFT") || !strings.Contains(out, "manual test") {
		t.Fatalf("expected stdout with 'DRIFT' and 'manual test', got: %q", out)
	}
}

// TestPgAdmin_PositionsList_Empty — no active set, prints "(no active positions)".
func TestPgAdmin_PositionsList_Empty(t *testing.T) {
	db, _ := newTestClient(t)

	stdout, stderr := captureBuffers()
	if rc := cmdPositionsList(db, stdout, stderr); rc != 0 {
		t.Fatalf("cmdPositionsList rc=%d, stderr=%q", rc, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "(no active positions)") {
		t.Fatalf("expected '(no active positions)' in output, got: %q", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "pg_") {
			t.Fatalf("unexpected position row: %q", line)
		}
	}
}

// TestPgAdmin_PositionsList_OnePosition — seed one open position, table shows it.
func TestPgAdmin_PositionsList_OnePosition(t *testing.T) {
	db, _ := newTestClient(t)

	p := &models.PriceGapPosition{
		ID:             "pg_SOON_binance_gate_1700000000",
		Symbol:         "SOON",
		LongExchange:   "binance",
		ShortExchange:  "gate",
		Status:         models.PriceGapStatusOpen,
		NotionalUSDT:   5000,
		EntrySpreadBps: 247.5,
		OpenedAt:       time.Now().UTC(),
	}
	if err := db.SavePriceGapPosition(p); err != nil {
		t.Fatalf("SavePriceGapPosition: %v", err)
	}

	stdout, stderr := captureBuffers()
	if rc := cmdPositionsList(db, stdout, stderr); rc != 0 {
		t.Fatalf("cmdPositionsList rc=%d, stderr=%q", rc, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{
		"pg_SOON_binance_gate_1700000000",
		"SOON",
		"binance",
		"gate",
		"5000.00",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q; got: %q", want, out)
		}
	}
	if strings.Contains(out, "(no active positions)") {
		t.Fatalf("unexpected empty marker in populated list: %q", out)
	}
}
