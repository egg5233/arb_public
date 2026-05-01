// Phase 15 Plan 15-04 — pg-admin breaker subcommands.
//
//	pg-admin breaker show                                   — print BreakerState + last 10 trips.
//	pg-admin breaker recover --confirm                      — operator-driven recovery; requires
//	                                                           literal RECOVER typed phrase from stdin.
//	pg-admin breaker test-fire --confirm [--dry-run]        — synthetic fire; requires literal
//	                                                           TEST-FIRE typed phrase from stdin.
//	                                                           Default is REAL TRIP; --dry-run skips
//	                                                           ALL mutations (preview-only).
//
// Typed-phrase prompts are intentionally case-sensitive — exact-match enforces
// deliberate operator intent (CONTEXT D-12). Symmetric with the dashboard
// modal in Plan 15-05.
//
// Operator name: pg-admin hard-codes operator="pg-admin" (mirrors ramp.go) so
// the audit trail in pg:breaker:trips distinguishes CLI recoveries from
// dashboard recoveries (which carry "operator" or token-claim names).
//
// Exit codes:
//   0 success
//   1 missing dep / store error
//   2 missing required flag / wrong typed phrase / invalid args
package main

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"arb/internal/models"
)

// BreakerOps is the narrow surface pg-admin needs from the Phase 15
// BreakerController. Tests inject a fake; production wires
// *pricegaptrader.BreakerController.
type BreakerOps interface {
	Snapshot() (models.BreakerState, error)
	Recover(ctx context.Context, operator string) error
	TestFire(ctx context.Context, dryRun bool) (models.BreakerTripRecord, error)
}

// BreakerTripsReader is the narrow surface pg-admin's `breaker show` subcommand
// needs to render the recent-trips tail. Tests inject a fake; production wires
// *database.Client (LoadBreakerTripAt is the existing Plan 15-04 helper).
type BreakerTripsReader interface {
	LoadBreakerTripAt(index int64) (models.BreakerTripRecord, bool, error)
}

// runBreaker dispatches breaker subcommands.
func runBreaker(args []string, deps Dependencies) int {
	if len(args) == 0 {
		fmt.Fprintln(deps.Stderr, "pg-admin: breaker requires 'show', 'recover', or 'test-fire'")
		return 2
	}
	if deps.Breaker == nil {
		fmt.Fprintln(deps.Stderr, "pg-admin: breaker requires breaker controller (production-only or test fake)")
		return 1
	}
	switch args[0] {
	case "show":
		return cmdBreakerShow(args[1:], deps)
	case "recover":
		return cmdBreakerRecover(args[1:], deps)
	case "test-fire":
		return cmdBreakerTestFire(args[1:], deps)
	default:
		fmt.Fprintf(deps.Stderr, "pg-admin: breaker: unknown subcommand %q\n", args[0])
		return 2
	}
}

// cmdBreakerShow prints the current BreakerState + last 10 trip records.
// Read-only — no typed-phrase prompt.
func cmdBreakerShow(_ []string, deps Dependencies) int {
	state, err := deps.Breaker.Snapshot()
	if err != nil {
		fmt.Fprintf(deps.Stderr, "pg-admin: breaker show snapshot: %v\n", err)
		return 1
	}
	status := "Armed"
	if state.PaperModeStickyUntil != 0 {
		status = "Tripped"
	}
	w := tabwriter.NewWriter(deps.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Field\tValue")
	fmt.Fprintf(w, "status\t%s\n", status)
	fmt.Fprintf(w, "pending_strike\t%d\n", state.PendingStrike)
	fmt.Fprintf(w, "strike1_ts\t%s\n", formatBreakerMs(state.Strike1Ts))
	fmt.Fprintf(w, "last_eval_ts\t%s\n", formatBreakerMs(state.LastEvalTs))
	fmt.Fprintf(w, "last_eval_pnl_usdt\t%.2f\n", state.LastEvalPnLUSDT)
	fmt.Fprintf(w, "paper_mode_sticky_until\t%s\n", formatStickyValue(state.PaperModeStickyUntil))
	if err := w.Flush(); err != nil {
		fmt.Fprintf(deps.Stderr, "pg-admin: tabwriter flush: %v\n", err)
		return 1
	}

	// Last 10 trips (best-effort — empty list is fine).
	if deps.BreakerTrips != nil {
		fmt.Fprintln(deps.Stdout, "")
		fmt.Fprintln(deps.Stdout, "Recent trips (newest first, up to 10):")
		anyTrip := false
		tw := tabwriter.NewWriter(deps.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "IDX\tTRIP_TS\tPNL\tTHRESH\tSRC\tRECOVERED_BY\tRECOVERY_TS")
		for i := int64(0); i < 10; i++ {
			rec, exists, lerr := deps.BreakerTrips.LoadBreakerTripAt(i)
			if lerr != nil {
				fmt.Fprintf(deps.Stderr, "pg-admin: load trip @%d: %v\n", i, lerr)
				break
			}
			if !exists {
				break
			}
			anyTrip = true
			recOp := "-"
			recTs := "-"
			if rec.RecoveryOperator != nil {
				recOp = *rec.RecoveryOperator
			}
			if rec.RecoveryTs != nil {
				recTs = formatBreakerMs(*rec.RecoveryTs)
			}
			fmt.Fprintf(tw, "%d\t%s\t%.2f\t%.2f\t%s\t%s\t%s\n",
				i, formatBreakerMs(rec.TripTs), rec.TripPnLUSDT, rec.Threshold,
				rec.Source, recOp, recTs)
		}
		_ = tw.Flush()
		if !anyTrip {
			fmt.Fprintln(deps.Stdout, "(no trips)")
		}
	}
	return 0
}

// cmdBreakerRecover requires --confirm flag + literal RECOVER phrase from stdin
// before invoking BreakerController.Recover.
func cmdBreakerRecover(args []string, deps Dependencies) int {
	if !hasFlag(args, "--confirm") {
		fmt.Fprintln(deps.Stderr, "pg-admin: breaker recover requires --confirm")
		return 2
	}
	fmt.Fprint(deps.Stdout, "Type 'RECOVER' to confirm operator-initiated recovery: ")
	line, ok := readPhrase(deps)
	if !ok {
		fmt.Fprintln(deps.Stderr, "pg-admin: stdin read failed")
		return 2
	}
	if line != "RECOVER" {
		fmt.Fprintln(deps.Stderr, "pg-admin: aborted — phrase did not match (expected 'RECOVER')")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := deps.Breaker.Recover(ctx, "pg-admin"); err != nil {
		fmt.Fprintf(deps.Stderr, "pg-admin: breaker recover FAILED: %v\n", err)
		return 1
	}
	fmt.Fprintln(deps.Stdout, "Recovered — sticky flag cleared, candidates re-enabled.")
	return 0
}

// cmdBreakerTestFire requires --confirm + literal TEST-FIRE phrase. Default
// is REAL TRIP; --dry-run flag opts into preview-only (no mutations).
func cmdBreakerTestFire(args []string, deps Dependencies) int {
	if !hasFlag(args, "--confirm") {
		fmt.Fprintln(deps.Stderr, "pg-admin: breaker test-fire requires --confirm")
		return 2
	}
	dryRun := hasFlag(args, "--dry-run")

	if !dryRun {
		fmt.Fprintln(deps.Stdout, "WARNING: default behavior is REAL TRIP. Use --dry-run for simulation.")
	}
	fmt.Fprint(deps.Stdout, "Type 'TEST-FIRE' to confirm synthetic breaker test-fire: ")
	line, ok := readPhrase(deps)
	if !ok {
		fmt.Fprintln(deps.Stderr, "pg-admin: stdin read failed")
		return 2
	}
	if line != "TEST-FIRE" {
		fmt.Fprintln(deps.Stderr, "pg-admin: aborted — phrase did not match (expected 'TEST-FIRE')")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rec, err := deps.Breaker.TestFire(ctx, dryRun)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "pg-admin: breaker test-fire FAILED: %v\n", err)
		return 1
	}
	if dryRun {
		fmt.Fprintf(deps.Stdout,
			"Test-fire DRY RUN — would trip with 24h=$%.2f, threshold=$%.2f. No mutations performed.\n",
			rec.TripPnLUSDT, rec.Threshold)
	} else {
		fmt.Fprintf(deps.Stdout,
			"Test-fire executed — engine TRIPPED to paper mode (24h=$%.2f, threshold=$%.2f, source=%s, ts=%s).\n",
			rec.TripPnLUSDT, rec.Threshold, rec.Source, formatBreakerMs(rec.TripTs))
	}
	return 0
}

// hasFlag returns true when args contains an exact match for flag (no value
// extraction needed; --confirm and --dry-run are pure boolean flags).
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// readPhrase reads a single line from deps.Stdin (or os.Stdin in production).
// Trims trailing CR/LF/whitespace; case-sensitive comparison happens at the
// caller per CONTEXT D-12.
func readPhrase(deps Dependencies) (string, bool) {
	if deps.Stdin == nil {
		return "", false
	}
	scanner := bufio.NewScanner(deps.Stdin)
	if !scanner.Scan() {
		return "", false
	}
	return strings.TrimSpace(scanner.Text()), true
}

// formatBreakerMs renders a unix-ms timestamp in Asia/Taipei. Zero → "(never)".
func formatBreakerMs(ms int64) string {
	if ms <= 0 {
		return "(never)"
	}
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil || loc == nil {
		loc = time.UTC
	}
	return time.UnixMilli(ms).In(loc).Format("2006-01-02 15:04:05 MST")
}

// formatStickyValue renders the int64 sticky-until field with a human label
// for the MaxInt64 sentinel.
func formatStickyValue(v int64) string {
	if v == 0 {
		return "0 (armed)"
	}
	// MaxInt64 == 9223372036854775807; render as a stable sticky label rather
	// than a 292-billion-year date.
	if v == int64(^uint64(0)>>1) {
		return "MaxInt64 (sticky-until-operator)"
	}
	return fmt.Sprintf("%d (%s)", v, formatBreakerMs(v))
}
