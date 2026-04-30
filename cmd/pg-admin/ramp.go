// Phase 14 Plan 14-04 — pg-admin ramp subcommands.
//
//	pg-admin ramp show                                  — table of the 5 RampState fields.
//	pg-admin ramp reset --reason=...                    — back to stage=1, counter=0, demote_count=0.
//	pg-admin ramp force-promote --reason=...            — bump stage by 1; counter PRESERVED (D-15 #3).
//	pg-admin ramp force-demote --reason=...             — drop stage by 1; counter ZEROED (D-15 #4).
//
// Force-op operator is hard-coded "pg-admin" so the audit trail in
// pg:ramp:events distinguishes these from daemon-side automatic eval calls
// (which use operator="daemon"). Reason flag is sanitized to avoid Telegram
// injection issues at the dispatch layer.
//
// Exit codes:
//   0 success
//   2 missing dep, registry/state error, or already-at-min/max for force ops
package main

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"
)

// runRamp dispatches ramp subcommands.
func runRamp(args []string, deps Dependencies) int {
	if len(args) == 0 {
		fmt.Fprintln(deps.Stderr, "pg-admin: ramp requires 'show', 'reset', 'force-promote', or 'force-demote'")
		return 2
	}
	if deps.Ramp == nil {
		fmt.Fprintln(deps.Stderr, "pg-admin: ramp requires ramp controller (production-only or test fake)")
		return 1
	}
	switch args[0] {
	case "show":
		return cmdRampShow(args[1:], deps)
	case "reset":
		return cmdRampReset(args[1:], deps)
	case "force-promote":
		return cmdRampForcePromote(args[1:], deps)
	case "force-demote":
		return cmdRampForceDemote(args[1:], deps)
	default:
		fmt.Fprintf(deps.Stderr, "pg-admin: ramp: unknown subcommand %q\n", args[0])
		return 2
	}
}

// cmdRampShow renders the 5-field RampState as a Field/Value table.
func cmdRampShow(_ []string, deps Dependencies) int {
	snap := deps.Ramp.Snapshot()
	w := tabwriter.NewWriter(deps.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Field\tValue")
	fmt.Fprintf(w, "current_stage\t%d\n", snap.CurrentStage)
	fmt.Fprintf(w, "clean_day_counter\t%d\n", snap.CleanDayCounter)
	fmt.Fprintf(w, "last_eval_ts\t%s\n", formatRampTime(snap.LastEvalTs))
	fmt.Fprintf(w, "last_loss_day_ts\t%s\n", formatRampTime(snap.LastLossDayTs))
	fmt.Fprintf(w, "demote_count\t%d\n", snap.DemoteCount)
	if err := w.Flush(); err != nil {
		fmt.Fprintf(deps.Stderr, "pg-admin: tabwriter flush: %v\n", err)
		return 2
	}
	return 0
}

// cmdRampReset wipes state to (stage=1, counter=0, demote_count=0).
func cmdRampReset(args []string, deps Dependencies) int {
	reason := parseReasonFlag(args)
	if err := deps.Ramp.Reset("pg-admin", reason); err != nil {
		fmt.Fprintf(deps.Stderr, "pg-admin: ramp reset FAILED: %v\n", err)
		return 2
	}
	fmt.Fprintln(deps.Stdout, "ramp reset to stage 1, counter=0, demote_count=0")
	return 0
}

// cmdRampForcePromote bumps the stage by 1; counter is preserved (D-15 #3).
func cmdRampForcePromote(args []string, deps Dependencies) int {
	reason := parseReasonFlag(args)
	prior := deps.Ramp.Snapshot().CurrentStage
	if err := deps.Ramp.ForcePromote("pg-admin", reason); err != nil {
		fmt.Fprintf(deps.Stderr, "pg-admin: ramp force-promote FAILED: %v\n", err)
		return 2
	}
	next := deps.Ramp.Snapshot().CurrentStage
	fmt.Fprintf(deps.Stdout, "ramp force-promoted: %d -> %d (counter preserved)\n", prior, next)
	return 0
}

// cmdRampForceDemote drops the stage by 1; counter is zeroed (D-15 #4).
func cmdRampForceDemote(args []string, deps Dependencies) int {
	reason := parseReasonFlag(args)
	prior := deps.Ramp.Snapshot().CurrentStage
	if err := deps.Ramp.ForceDemote("pg-admin", reason); err != nil {
		fmt.Fprintf(deps.Stderr, "pg-admin: ramp force-demote FAILED: %v\n", err)
		return 2
	}
	next := deps.Ramp.Snapshot().CurrentStage
	fmt.Fprintf(deps.Stdout, "ramp force-demoted: %d -> %d (counter zeroed)\n", prior, next)
	return 0
}

// parseReasonFlag scans args for --reason=... ; defaults to "no_reason_provided".
func parseReasonFlag(args []string) string {
	for _, a := range args {
		if len(a) > 9 && a[:9] == "--reason=" {
			return a[9:]
		}
	}
	return "no_reason_provided"
}

// formatRampTime renders a time.Time in RFC3339 or "(never)" if zero.
func formatRampTime(t time.Time) string {
	if t.IsZero() {
		return "(never)"
	}
	// Format in UTC to match daemon-side observers; the operator sees
	// timestamps in the same timezone the daemon writes them.
	return t.UTC().Format(time.RFC3339)
}

// quiet linter on strings import retained for future force-op reason
// sanitization (Plan 14-05 may consume this).
var _ = strings.TrimSpace
