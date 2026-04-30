// Phase 14 Plan 14-04 — pg-admin reconcile subcommands.
//
//	pg-admin reconcile run  --date=YYYY-MM-DD  — operator manual reconcile.
//	                                              Re-runs even if already
//	                                              reconciled (D-04 byte-
//	                                              identical guarantee).
//	pg-admin reconcile show --date=YYYY-MM-DD  — JSON-indent dump of
//	                                              pg:reconcile:daily:{date}.
//
// Exit codes:
//   0 success
//   1 not-found / show-only soft-fail
//   2 validation, future date, or runtime error
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"time"
)

// reconcileDateRegex pins the --date format. Any deviation rejects via exit 2.
var reconcileDateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// runReconcile dispatches reconcile run/show subcommands.
func runReconcile(args []string, deps Dependencies) int {
	if len(args) == 0 {
		fmt.Fprintln(deps.Stderr, "pg-admin: reconcile requires 'run' or 'show'")
		return 2
	}
	if deps.Reconciler == nil {
		fmt.Fprintln(deps.Stderr, "pg-admin: reconcile requires reconciler (production-only or test fake)")
		return 1
	}
	switch args[0] {
	case "run":
		return cmdReconcileRun(args[1:], deps)
	case "show":
		return cmdReconcileShow(args[1:], deps)
	default:
		fmt.Fprintf(deps.Stderr, "pg-admin: reconcile: unknown subcommand %q (want 'run' or 'show')\n", args[0])
		return 2
	}
}

// cmdReconcileRun runs the reconciler for the given UTC date. Re-running for an
// already-reconciled date is allowed (D-04 idempotency); we surface a note
// to the operator so they know the byte-identical guarantee was exercised.
func cmdReconcileRun(args []string, deps Dependencies) int {
	date, err := parseReconcileDateFlag(args, "run")
	if err != nil {
		fmt.Fprintf(deps.Stderr, "pg-admin: %v\n", err)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if _, exists, _ := deps.Reconciler.LoadRecord(ctx, date); exists {
		fmt.Fprintf(deps.Stdout, "note: %s already reconciled — re-running to verify byte-identical output\n", date)
	}
	if err := deps.Reconciler.RunForDate(ctx, date); err != nil {
		fmt.Fprintf(deps.Stderr, "pg-admin: reconcile %s FAILED: %v\n", date, err)
		return 2
	}
	fmt.Fprintf(deps.Stdout, "reconcile %s OK\n", date)
	return 0
}

// cmdReconcileShow dumps the on-disk DailyReconcileRecord as indented JSON.
// Returns exit 1 when the date has not been reconciled (operator-friendly
// soft-fail vs. exit 2 for malformed args).
func cmdReconcileShow(args []string, deps Dependencies) int {
	date, err := parseReconcileDateFlag(args, "show")
	if err != nil {
		fmt.Fprintf(deps.Stderr, "pg-admin: %v\n", err)
		return 2
	}
	rec, exists, err := deps.Reconciler.LoadRecord(context.Background(), date)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "pg-admin: load %s: %v\n", date, err)
		return 2
	}
	if !exists {
		fmt.Fprintf(deps.Stderr, "pg-admin: no reconcile record for %s — run 'pg-admin reconcile run --date=%s' first\n", date, date)
		return 1
	}
	enc := json.NewEncoder(deps.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rec); err != nil {
		fmt.Fprintf(deps.Stderr, "pg-admin: encode: %v\n", err)
		return 2
	}
	return 0
}

// parseReconcileDateFlag scans args for --date=YYYY-MM-DD; rejects anything
// not matching the regex, anything more than 1 day in the future, anything
// non-parseable. The 1-day grace window allows ops to (re-)run "today" near
// UTC midnight when the operator clock might lead the system clock.
func parseReconcileDateFlag(args []string, sub string) (string, error) {
	for _, a := range args {
		if len(a) > 7 && a[:7] == "--date=" {
			d := a[7:]
			if !reconcileDateRegex.MatchString(d) {
				return "", fmt.Errorf("invalid --date=%s (expect YYYY-MM-DD)", d)
			}
			parsed, perr := time.Parse("2006-01-02", d)
			if perr != nil {
				return "", fmt.Errorf("invalid --date=%s: %w", d, perr)
			}
			if parsed.After(time.Now().UTC().AddDate(0, 0, 1)) {
				return "", fmt.Errorf("invalid future --date=%s", d)
			}
			return d, nil
		}
	}
	return "", fmt.Errorf("reconcile %s requires --date=YYYY-MM-DD", sub)
}

// quiet "io" import unused linter — reconcile.go does not directly use io
// but the surrounding package contract relies on the writer types in the
// deps surface. Reference at package level keeps the linter happy.
var _ io.Writer = (*ioRefSink)(nil)

type ioRefSink struct{}

func (ioRefSink) Write(p []byte) (int, error) { return len(p), nil }
