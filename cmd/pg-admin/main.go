// Package main — pg-admin: operator CLI for the price-gap tracker.
//
// Subcommands:
//
//	pg-admin status                  — summary: enabled flag, budget, active count, disabled flags
//	pg-admin enable <symbol>         — clears pg:candidate:disabled:<symbol>
//	pg-admin disable <symbol> [why]  — sets pg:candidate:disabled:<symbol>=[why|"manual"]
//	pg-admin positions list          — lists all active positions (table)
//	pg-admin positions purge <id>    — removes id from pg:positions:active
//	                                    (does NOT close the exchange position — use
//	                                    only for ghosts that never reached the wire,
//	                                    e.g. synth positions orphaned by a bug)
//
// All operations work directly on Redis — config.json is NEVER written
// (CLAUDE.local.md rule; D-20). Phase 9 will replace this CLI with a
// dashboard UI; until then this is the ONLY reversal path for the
// exec-quality auto-disable (PG-RISK-03), preventing Pitfall #7 (a
// disabled candidate staying disabled forever).
package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"arb/internal/config"
	"arb/internal/database"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cfg := config.Load()
	db, err := database.New(cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)
	if err != nil {
		fatal("database.New: %v", err)
	}
	defer db.Close()

	sub := os.Args[1]
	switch sub {
	case "status":
		cmdStatus(cfg, db)
	case "enable":
		if len(os.Args) < 3 {
			fatal("enable requires <symbol>")
		}
		cmdEnable(db, strings.ToUpper(os.Args[2]))
	case "disable":
		if len(os.Args) < 3 {
			fatal("disable requires <symbol>")
		}
		reason := "manual"
		if len(os.Args) >= 4 {
			reason = strings.Join(os.Args[3:], " ")
		}
		cmdDisable(db, strings.ToUpper(os.Args[2]), reason)
	case "positions":
		if len(os.Args) < 3 {
			fatal("positions requires 'list' or 'purge <id>'")
		}
		switch os.Args[2] {
		case "list":
			cmdPositionsList(db)
		case "purge":
			if len(os.Args) < 4 {
				fatal("positions purge requires <id>")
			}
			cmdPositionsPurge(db, os.Args[3])
		default:
			fatal("positions: unknown subcommand %q (want 'list' or 'purge')", os.Args[2])
		}
	case "-h", "--help", "help":
		usage()
		os.Exit(0)
	default:
		usage()
		os.Exit(2)
	}
}

func cmdStatus(cfg *config.Config, db *database.Client) {
	fmt.Printf("Price-Gap Tracker\n")
	fmt.Printf("  Enabled:                %v\n", cfg.PriceGapEnabled)
	fmt.Printf("  Budget (USDT):          %.2f\n", cfg.PriceGapBudget)
	fmt.Printf("  MaxConcurrent:          %d\n", cfg.PriceGapMaxConcurrent)
	fmt.Printf("  GateConcentrationPct:   %.2f\n", cfg.PriceGapGateConcentrationPct)
	fmt.Printf("  MaxHoldMin:             %d\n", cfg.PriceGapMaxHoldMin)
	fmt.Printf("  Candidates configured:  %d\n", len(cfg.PriceGapCandidates))

	active, err := db.GetActivePriceGapPositions()
	if err != nil {
		fatal("GetActivePriceGapPositions: %v", err)
	}
	fmt.Printf("  Active positions:       %d\n", len(active))

	fmt.Printf("\nDisabled candidates:\n")
	anyDisabled := false
	for _, cand := range cfg.PriceGapCandidates {
		disabled, reason, disabledAt, err := db.IsCandidateDisabled(cand.Symbol)
		if err != nil {
			continue
		}
		if disabled {
			anyDisabled = true
			if disabledAt > 0 {
				fmt.Printf("  %-10s  reason: %s  (since %s)\n",
					cand.Symbol, reason, time.Unix(disabledAt, 0).Format("2006-01-02 15:04:05"))
			} else {
				fmt.Printf("  %-10s  reason: %s\n", cand.Symbol, reason)
			}
		}
	}
	if !anyDisabled {
		fmt.Printf("  (none)\n")
	}
}

func cmdEnable(db *database.Client, symbol string) {
	if err := db.ClearCandidateDisabled(symbol); err != nil {
		fatal("ClearCandidateDisabled: %v", err)
	}
	fmt.Printf("Candidate %s enabled (disabled flag cleared).\n", symbol)
}

func cmdDisable(db *database.Client, symbol, reason string) {
	if err := db.SetCandidateDisabled(symbol, reason); err != nil {
		fatal("SetCandidateDisabled: %v", err)
	}
	fmt.Printf("Candidate %s disabled. Reason: %s\n", symbol, reason)
}

func cmdPositionsPurge(db *database.Client, id string) {
	if err := db.RemoveActivePriceGapPosition(id); err != nil {
		fatal("RemoveActivePriceGapPosition: %v", err)
	}
	fmt.Printf("Purged %s from pg:positions:active. (Position record stays in pg:positions for analytics.)\n", id)
}

func cmdPositionsList(db *database.Client) {
	active, err := db.GetActivePriceGapPositions()
	if err != nil {
		fatal("GetActivePriceGapPositions: %v", err)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSYMBOL\tLONG\tSHORT\tNOTIONAL\tENTRY_BPS\tSTATUS\tOPENED")
	for _, p := range active {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.2f\t%.1f\t%s\t%s\n",
			p.ID, p.Symbol, p.LongExchange, p.ShortExchange,
			p.NotionalUSDT, p.EntrySpreadBps, p.Status,
			p.OpenedAt.Format("2006-01-02 15:04:05"))
	}
	w.Flush()
	if len(active) == 0 {
		fmt.Println("(no active positions)")
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: pg-admin <status|enable|disable|positions list> [args...]")
	fmt.Fprintln(os.Stderr, "  pg-admin status")
	fmt.Fprintln(os.Stderr, "  pg-admin enable <symbol>")
	fmt.Fprintln(os.Stderr, "  pg-admin disable <symbol> [reason...]")
	fmt.Fprintln(os.Stderr, "  pg-admin positions list")
	fmt.Fprintln(os.Stderr, "  pg-admin positions purge <id>")
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "pg-admin: "+format+"\n", args...)
	os.Exit(1)
}
