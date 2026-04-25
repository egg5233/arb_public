package main

import (
	"os"
	"strings"
	"testing"
)

// TestMainWire_ShutdownOrder asserts D-03 invariant: pgTracker.Stop() must run
// BEFORE spotEng.Stop() during graceful shutdown. The shutdown order is
// structurally encoded in cmd/main.go via the source order of the deferred /
// invoked Stop() calls; mocking the full main() bootstrap is impractical, so
// this test treats source order as the contract and fails if a future edit
// reorders the calls.
//
// Phase 8 — Plan 08-07 Task 4 — REQ PG-OPS-06 — threat T-08-28
func TestMainWire_ShutdownOrder(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	body := string(src)

	pgIdx := strings.Index(body, "pgTracker.Stop()")
	if pgIdx < 0 {
		t.Fatalf("pgTracker.Stop() call not found in cmd/main.go — Phase 8 wiring regressed")
	}
	spotIdx := strings.Index(body, "spotEng.Stop()")
	if spotIdx < 0 {
		t.Fatalf("spotEng.Stop() call not found in cmd/main.go")
	}
	if pgIdx >= spotIdx {
		t.Fatalf("D-03 violation: pgTracker.Stop() (offset=%d) must come BEFORE spotEng.Stop() (offset=%d) in shutdown order", pgIdx, spotIdx)
	}
}

// TestMainWire_PriceGapDisabled asserts the conditional-wire pattern is
// present: NewTracker / Start must live inside `if cfg.PriceGapEnabled`.
//
// Phase 8 — Plan 08-07 Task 3 — REQ PG-OPS-06 — threat T-08-27
func TestMainWire_PriceGapDisabled(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	body := string(src)

	gateIdx := strings.Index(body, "if cfg.PriceGapEnabled")
	if gateIdx < 0 {
		t.Fatalf("if cfg.PriceGapEnabled gate not found in cmd/main.go — default-OFF guarantee broken")
	}
	startIdx := strings.Index(body, "pgTracker.Start()")
	if startIdx < 0 {
		t.Fatalf("pgTracker.Start() not found in cmd/main.go")
	}
	if startIdx <= gateIdx {
		t.Fatalf("PG-OPS-06 violation: pgTracker.Start() (offset=%d) must appear AFTER and inside `if cfg.PriceGapEnabled` (offset=%d)", startIdx, gateIdx)
	}
	// Confirm the disabled-path log line is present so operators see the OFF state.
	if !strings.Contains(body, "Price-gap tracker disabled") {
		t.Fatalf("disabled-path log line missing from cmd/main.go")
	}
}
