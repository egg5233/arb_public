// Package main — pg-admin: operator CLI for the price-gap tracker.
//
// Subcommands:
//
//	pg-admin status                                — summary: enabled flag, budget, active count, disabled flags
//	pg-admin enable <symbol>                       — clears pg:candidate:disabled:<symbol>
//	pg-admin disable <symbol> [why]                — sets pg:candidate:disabled:<symbol>=[why|"manual"]
//	pg-admin positions list                        — lists all active positions (table)
//	pg-admin positions purge <id>                  — removes id from pg:positions:active
//	                                                  (does NOT close the exchange position — use only
//	                                                  for ghosts that never reached the wire)
//	pg-admin candidates list                       — JSON dump of cfg.PriceGapCandidates via Registry.List
//	pg-admin candidates add --symbol .. --long .. --short .. [--threshold-bps N] [--max-position-usdt N]
//	                                                — Registry.Add(ctx, "pg-admin", c)
//	pg-admin candidates delete --idx N             — Registry.Delete(ctx, "pg-admin", N)
//	pg-admin candidates replace --file <path>      — Registry.Replace(ctx, "pg-admin", next)
//
// Plan 11-03 Task 2: candidates subcommands route through *pricegaptrader.Registry
// so dashboard + pg-admin share the chokepoint (PG-DISC-04). Validation uses the
// shared `pricegaptrader.ValidateCandidates` so handler + CLI never drift (T-11-19).
//
// All operations work on Redis or via Registry (which writes config.json
// atomically + .bak ring). pg-admin still NEVER writes config.json directly
// (CLAUDE.local.md rule; D-20). Phase 9 will replace this CLI with a
// dashboard UI; until then this is the ONLY reversal path for the
// exec-quality auto-disable (PG-RISK-03), preventing Pitfall #7.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
	"arb/internal/pricegaptrader"
	"arb/pkg/utils"
)

// CandidateRegistry is the chokepoint contract pg-admin invokes for every
// candidate-mutation subcommand. Production wires *pricegaptrader.Registry;
// tests inject a fake double via Run().
type CandidateRegistry interface {
	Add(ctx context.Context, source string, c models.PriceGapCandidate) error
	Update(ctx context.Context, source string, idx int, c models.PriceGapCandidate) error
	Delete(ctx context.Context, source string, idx int) error
	Replace(ctx context.Context, source string, next []models.PriceGapCandidate) error
	Get(idx int) (models.PriceGapCandidate, bool)
	List() []models.PriceGapCandidate
}

// ReconcileRunner is the narrow surface pg-admin needs from the Phase 14
// Plan 14-02 reconciler. Tests inject a fake; production wires
// *pricegaptrader.Reconciler.
type ReconcileRunner interface {
	RunForDate(ctx context.Context, date string) error
	LoadRecord(ctx context.Context, date string) (pricegaptrader.DailyReconcileRecord, bool, error)
}

// RampOps is the narrow surface pg-admin needs from the Phase 14 Plan 14-03
// ramp controller. Tests inject a fake; production wires
// *pricegaptrader.RampController.
type RampOps interface {
	Snapshot() models.RampState
	ForcePromote(operator, reason string) error
	ForceDemote(operator, reason string) error
	Reset(operator, reason string) error
}

// Dependencies bundles the test-overridable surface of pg-admin so Run() can
// be invoked from a unit test with a fake registry, captured stdout/stderr,
// and (where exposed) a fake DB. Production main() builds these from the
// real *config.Config + *database.Client + *pricegaptrader.Registry.
type Dependencies struct {
	Registry   CandidateRegistry
	Reconciler ReconcileRunner // Phase 14 Plan 14-04 — reconcile run/show
	Ramp       RampOps         // Phase 14 Plan 14-04 — ramp show/reset/force-promote/force-demote
	DB         *database.Client // optional — only the legacy non-candidates subcommands need it
	Cfg        *config.Config   // optional — used by the status subcommand
	Stdout     io.Writer
	Stderr     io.Writer
}

// Run is the test-friendly entrypoint. main() forwards os.Args[1:] +
// production deps; tests call directly with synthetic args + a fake registry.
// Returns the process exit code (0 on success). Exit-code map for candidates:
//
//	0 success
//	1 generic
//	2 validation / usage
//	3 duplicate (Registry.ErrDuplicateCandidate)
//	4 cap exceeded (Registry.ErrCapExceeded)
//	5 idx out of range (Registry.ErrIndexOutOfRange)
func Run(args []string, deps Dependencies) int {
	if deps.Stdout == nil {
		deps.Stdout = os.Stdout
	}
	if deps.Stderr == nil {
		deps.Stderr = os.Stderr
	}
	if len(args) == 0 {
		usage(deps.Stderr)
		return 2
	}
	switch args[0] {
	case "status":
		if deps.Cfg == nil || deps.DB == nil {
			fmt.Fprintln(deps.Stderr, "pg-admin: status requires cfg+db (production-only)")
			return 1
		}
		cmdStatus(deps.Cfg, deps.DB, deps.Stdout)
		return 0
	case "enable":
		if len(args) < 2 {
			fmt.Fprintln(deps.Stderr, "pg-admin: enable requires <symbol>")
			return 2
		}
		if deps.DB == nil {
			fmt.Fprintln(deps.Stderr, "pg-admin: enable requires db (production-only)")
			return 1
		}
		return cmdEnable(deps.DB, strings.ToUpper(args[1]), deps.Stdout, deps.Stderr)
	case "disable":
		if len(args) < 2 {
			fmt.Fprintln(deps.Stderr, "pg-admin: disable requires <symbol>")
			return 2
		}
		if deps.DB == nil {
			fmt.Fprintln(deps.Stderr, "pg-admin: disable requires db (production-only)")
			return 1
		}
		reason := "manual"
		if len(args) >= 3 {
			reason = strings.Join(args[2:], " ")
		}
		return cmdDisable(deps.DB, strings.ToUpper(args[1]), reason, deps.Stdout, deps.Stderr)
	case "positions":
		if len(args) < 2 {
			fmt.Fprintln(deps.Stderr, "pg-admin: positions requires 'list' or 'purge <id>'")
			return 2
		}
		if deps.DB == nil {
			fmt.Fprintln(deps.Stderr, "pg-admin: positions requires db (production-only)")
			return 1
		}
		switch args[1] {
		case "list":
			return cmdPositionsList(deps.DB, deps.Stdout, deps.Stderr)
		case "purge":
			if len(args) < 3 {
				fmt.Fprintln(deps.Stderr, "pg-admin: positions purge requires <id>")
				return 2
			}
			return cmdPositionsPurge(deps.DB, args[2], deps.Stdout, deps.Stderr)
		default:
			fmt.Fprintf(deps.Stderr, "pg-admin: positions: unknown subcommand %q (want 'list' or 'purge')\n", args[1])
			return 2
		}
	case "candidates":
		return runCandidates(args[1:], deps)
	case "reconcile":
		return runReconcile(args[1:], deps)
	case "ramp":
		return runRamp(args[1:], deps)
	case "-h", "--help", "help":
		usage(deps.Stdout)
		return 0
	default:
		usage(deps.Stderr)
		return 2
	}
}

// runCandidates dispatches the four candidate subcommands. Registry must be
// populated; production builds it from cfg + db + logger and passes it via
// Dependencies; tests pass a fake.
func runCandidates(args []string, deps Dependencies) int {
	if len(args) == 0 {
		fmt.Fprintln(deps.Stderr, "pg-admin: candidates requires a sub-subcommand: list | add | delete | replace")
		return 2
	}
	if deps.Registry == nil {
		fmt.Fprintln(deps.Stderr, "pg-admin: candidates requires registry (production-only or test fake)")
		return 1
	}
	ctx := context.Background()
	switch args[0] {
	case "list":
		return cmdCandidatesList(deps.Registry, deps.Stdout, deps.Stderr)
	case "add":
		return cmdCandidatesAdd(ctx, args[1:], deps.Registry, deps.Stdout, deps.Stderr)
	case "delete":
		return cmdCandidatesDelete(ctx, args[1:], deps.Registry, deps.Stdout, deps.Stderr)
	case "replace":
		return cmdCandidatesReplace(ctx, args[1:], deps.Registry, deps.Stdout, deps.Stderr)
	default:
		fmt.Fprintf(deps.Stderr, "pg-admin: candidates: unknown subcommand %q (want list | add | delete | replace)\n", args[0])
		return 2
	}
}

// cmdCandidatesList prints the current candidate slice as JSON to stdout.
func cmdCandidatesList(reg CandidateRegistry, stdout io.Writer, stderr io.Writer) int {
	cs := reg.List()
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cs); err != nil {
		fmt.Fprintf(stderr, "pg-admin: encode list: %v\n", err)
		return 1
	}
	return 0
}

// cmdCandidatesAdd parses flags, normalises, validates, and routes through
// Registry.Add with source="pg-admin".
func cmdCandidatesAdd(ctx context.Context, args []string, reg CandidateRegistry, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("candidates add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	symbol := fs.String("symbol", "", "canonical symbol (e.g. BTCUSDT)")
	longExch := fs.String("long", "", "long-side exchange identifier (e.g. binance)")
	shortExch := fs.String("short", "", "short-side exchange identifier (e.g. bybit)")
	thresholdBps := fs.Float64("threshold-bps", 100, "spread trigger (bps)")
	maxPosUSDT := fs.Float64("max-position-usdt", 1000, "per-leg notional cap (USDT)")
	slippageBps := fs.Float64("modeled-slippage-bps", 5, "expected slippage (bps)")
	direction := fs.String("direction", "", "pinned | bidirectional (default empty → pinned)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	c := models.PriceGapCandidate{
		Symbol:             strings.ToUpper(strings.TrimSpace(*symbol)),
		LongExch:           strings.ToLower(strings.TrimSpace(*longExch)),
		ShortExch:          strings.ToLower(strings.TrimSpace(*shortExch)),
		ThresholdBps:       *thresholdBps,
		MaxPositionUSDT:    *maxPosUSDT,
		ModeledSlippageBps: *slippageBps,
		Direction:          *direction,
	}
	if errs := pricegaptrader.ValidateCandidates([]models.PriceGapCandidate{c}); len(errs) > 0 {
		fmt.Fprintf(stderr, "pg-admin: validation failed: %s\n", strings.Join(errs, "; "))
		return 2
	}
	models.NormalizeDirection(&c)
	if err := reg.Add(ctx, "pg-admin", c); err != nil {
		switch {
		case errors.Is(err, pricegaptrader.ErrDuplicateCandidate):
			fmt.Fprintf(stderr, "pg-admin: duplicate candidate (%s/%s/%s already configured)\n", c.Symbol, c.LongExch, c.ShortExch)
			return 3
		case errors.Is(err, pricegaptrader.ErrCapExceeded):
			fmt.Fprintf(stderr, "pg-admin: cap exceeded — adjust PriceGapMaxCandidates first\n")
			return 4
		default:
			fmt.Fprintf(stderr, "pg-admin: registry add: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "added %s/%s/%s\n", c.Symbol, c.LongExch, c.ShortExch)
	return 0
}

// cmdCandidatesDelete parses --idx and routes Registry.Delete.
func cmdCandidatesDelete(ctx context.Context, args []string, reg CandidateRegistry, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("candidates delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	idx := fs.Int("idx", -1, "candidate index (0-based)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *idx < 0 {
		fmt.Fprintln(stderr, "pg-admin: --idx must be >= 0")
		return 2
	}
	if err := reg.Delete(ctx, "pg-admin", *idx); err != nil {
		if errors.Is(err, pricegaptrader.ErrIndexOutOfRange) {
			fmt.Fprintf(stderr, "pg-admin: idx %d out of range\n", *idx)
			return 5
		}
		fmt.Fprintf(stderr, "pg-admin: registry delete: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "deleted idx %d\n", *idx)
	return 0
}

// cmdCandidatesReplace parses --file (JSON array of PriceGapCandidate),
// validates the slice, and routes Registry.Replace.
func cmdCandidatesReplace(ctx context.Context, args []string, reg CandidateRegistry, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("candidates replace", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("file", "", "path to JSON array of PriceGapCandidate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *file == "" {
		fmt.Fprintln(stderr, "pg-admin: --file is required")
		return 2
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(stderr, "pg-admin: read %s: %v\n", *file, err)
		return 1
	}
	var next []models.PriceGapCandidate
	if err := json.Unmarshal(data, &next); err != nil {
		fmt.Fprintf(stderr, "pg-admin: parse %s: %v\n", *file, err)
		return 2
	}
	for i := range next {
		next[i].Symbol = strings.ToUpper(strings.TrimSpace(next[i].Symbol))
		next[i].LongExch = strings.ToLower(strings.TrimSpace(next[i].LongExch))
		next[i].ShortExch = strings.ToLower(strings.TrimSpace(next[i].ShortExch))
	}
	if errs := pricegaptrader.ValidateCandidates(next); len(errs) > 0 {
		fmt.Fprintf(stderr, "pg-admin: validation failed: %s\n", strings.Join(errs, "; "))
		return 2
	}
	for i := range next {
		models.NormalizeDirection(&next[i])
	}
	if err := reg.Replace(ctx, "pg-admin", next); err != nil {
		if errors.Is(err, pricegaptrader.ErrCapExceeded) {
			fmt.Fprintf(stderr, "pg-admin: cap exceeded — adjust PriceGapMaxCandidates first\n")
			return 4
		}
		fmt.Fprintf(stderr, "pg-admin: registry replace: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "replaced %d candidates\n", len(next))
	return 0
}

func main() {
	cfg := config.Load()
	db, err := database.New(cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pg-admin: database.New: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	registry := pricegaptrader.NewRegistry(cfg, db.PriceGapAudit(), utils.NewLogger("pg-admin"))

	// Phase 14 Plan 14-04 — Reconciler + RampController construction. Both
	// share the single *database.Client store; daemon-side notifications
	// are nil here (pg-admin is a one-shot CLI that does not own the
	// running Telegram client). Force-op telegram notifications still fire
	// because the running arb daemon's RampController has the notifier
	// wired and its in-memory state is refreshed lazily — pg-admin writes
	// to the same Redis keys the daemon reads at next Eval.
	pgReconciler := pricegaptrader.NewReconciler(db, nil, cfg, utils.NewLogger("pg-admin-reconcile"))
	pgRamp := pricegaptrader.NewRampController(db, nil, cfg, utils.NewLogger("pg-admin-ramp"), time.Now)

	deps := Dependencies{
		Registry:   registry,
		Reconciler: pgReconciler,
		Ramp:       pgRamp,
		DB:         db,
		Cfg:        cfg,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}
	os.Exit(Run(os.Args[1:], deps))
}

func cmdStatus(cfg *config.Config, db *database.Client, stdout io.Writer) {
	fmt.Fprintf(stdout, "Price-Gap Tracker\n")
	fmt.Fprintf(stdout, "  Enabled:                %v\n", cfg.PriceGapEnabled)
	fmt.Fprintf(stdout, "  Budget (USDT):          %.2f\n", cfg.PriceGapBudget)
	fmt.Fprintf(stdout, "  MaxConcurrent:          %d\n", cfg.PriceGapMaxConcurrent)
	fmt.Fprintf(stdout, "  GateConcentrationPct:   %.2f\n", cfg.PriceGapGateConcentrationPct)
	fmt.Fprintf(stdout, "  MaxHoldMin:             %d\n", cfg.PriceGapMaxHoldMin)
	fmt.Fprintf(stdout, "  Candidates configured:  %d\n", len(cfg.PriceGapCandidates))

	active, err := db.GetActivePriceGapPositions()
	if err != nil {
		fmt.Fprintf(stdout, "  GetActivePriceGapPositions: %v\n", err)
		return
	}
	fmt.Fprintf(stdout, "  Active positions:       %d\n", len(active))

	fmt.Fprintf(stdout, "\nDisabled candidates:\n")
	anyDisabled := false
	for _, cand := range cfg.PriceGapCandidates {
		disabled, reason, disabledAt, err := db.IsCandidateDisabled(cand.Symbol)
		if err != nil {
			continue
		}
		if disabled {
			anyDisabled = true
			if disabledAt > 0 {
				fmt.Fprintf(stdout, "  %-10s  reason: %s  (since %s)\n",
					cand.Symbol, reason, time.Unix(disabledAt, 0).Format("2006-01-02 15:04:05"))
			} else {
				fmt.Fprintf(stdout, "  %-10s  reason: %s\n", cand.Symbol, reason)
			}
		}
	}
	if !anyDisabled {
		fmt.Fprintf(stdout, "  (none)\n")
	}
}

func cmdEnable(db *database.Client, symbol string, stdout io.Writer, stderr io.Writer) int {
	if err := db.ClearCandidateDisabled(symbol); err != nil {
		fmt.Fprintf(stderr, "pg-admin: ClearCandidateDisabled: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Candidate %s enabled (disabled flag cleared).\n", symbol)
	return 0
}

func cmdDisable(db *database.Client, symbol, reason string, stdout io.Writer, stderr io.Writer) int {
	if err := db.SetCandidateDisabled(symbol, reason); err != nil {
		fmt.Fprintf(stderr, "pg-admin: SetCandidateDisabled: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Candidate %s disabled. Reason: %s\n", symbol, reason)
	return 0
}

func cmdPositionsPurge(db *database.Client, id string, stdout io.Writer, stderr io.Writer) int {
	if err := db.RemoveActivePriceGapPosition(id); err != nil {
		fmt.Fprintf(stderr, "pg-admin: RemoveActivePriceGapPosition: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Purged %s from pg:positions:active. (Position record stays in pg:positions for analytics.)\n", id)
	return 0
}

func cmdPositionsList(db *database.Client, stdout io.Writer, stderr io.Writer) int {
	active, err := db.GetActivePriceGapPositions()
	if err != nil {
		fmt.Fprintf(stderr, "pg-admin: GetActivePriceGapPositions: %v\n", err)
		return 1
	}
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSYMBOL\tLONG\tSHORT\tNOTIONAL\tENTRY_BPS\tSTATUS\tOPENED")
	for _, p := range active {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.2f\t%.1f\t%s\t%s\n",
			p.ID, p.Symbol, p.LongExchange, p.ShortExchange,
			p.NotionalUSDT, p.EntrySpreadBps, p.Status,
			p.OpenedAt.Format("2006-01-02 15:04:05"))
	}
	w.Flush()
	if len(active) == 0 {
		fmt.Fprintln(stdout, "(no active positions)")
	}
	return 0
}

func usage(out io.Writer) {
	fmt.Fprintln(out, "usage: pg-admin <subcommand> [args...]")
	fmt.Fprintln(out, "  pg-admin status")
	fmt.Fprintln(out, "  pg-admin enable <symbol>")
	fmt.Fprintln(out, "  pg-admin disable <symbol> [reason...]")
	fmt.Fprintln(out, "  pg-admin positions list")
	fmt.Fprintln(out, "  pg-admin positions purge <id>")
	fmt.Fprintln(out, "  pg-admin candidates list")
	fmt.Fprintln(out, "  pg-admin candidates add --symbol .. --long .. --short .. [--threshold-bps N] [--max-position-usdt N] [--modeled-slippage-bps N] [--direction pinned|bidirectional]")
	fmt.Fprintln(out, "  pg-admin candidates delete --idx N")
	fmt.Fprintln(out, "  pg-admin candidates replace --file <path>")
	fmt.Fprintln(out, "  pg-admin reconcile run --date=YYYY-MM-DD")
	fmt.Fprintln(out, "  pg-admin reconcile show --date=YYYY-MM-DD")
	fmt.Fprintln(out, "  pg-admin ramp show")
	fmt.Fprintln(out, "  pg-admin ramp reset --reason=...")
	fmt.Fprintln(out, "  pg-admin ramp force-promote --reason=...")
	fmt.Fprintln(out, "  pg-admin ramp force-demote --reason=...")
}
