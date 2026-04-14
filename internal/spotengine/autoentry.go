package spotengine

import (
	"strings"
)

// attemptAutoEntries processes discovered opportunities and attempts automated
// entry for qualifying candidates. Entries are executed sequentially (one at a
// time) to prevent overcommit from parallel entry attempts.
//
// When the unified cross-strategy entry selector is the installed owner
// (unifiedOwnerReady), this legacy per-exchange path skips entirely so the
// selector's batch-reserved dispatch is the single source of auto-entry. When
// the flag is set but the capital allocator is unavailable the selector cannot
// safely preheld — in that case we fall through to the legacy path to keep
// spot-futures trading live (matches the legacy-path-runs regression test).
func (e *SpotEngine) attemptAutoEntries(opps []SpotArbOpportunity) {
	if e.unifiedOwnerReady() {
		// Unified owner installed — selector dispatches spot entries.
		return
	}

	if !e.cfg.SpotFuturesAutoEnabled {
		return
	}

	if len(opps) == 0 {
		return
	}

	e.log.Info("auto-entry: evaluating %d opportunities", len(opps))

	for _, opp := range opps {
		// Run risk gate checks.
		result := e.checkRiskGate(opp)
		if !result.Allowed {
			if result.Reason != "dry_run" {
				e.log.Info("auto-entry: BLOCKED %s on %s — %s", opp.Symbol, opp.Exchange, result.Reason)
			}
			// Stop scan on: capacity exhausted, db error, or dry-run
			// (one-entry-per-scan: dry-run stops after first eligible candidate).
			if strings.HasPrefix(result.Reason, "at_capacity") || result.Reason == "db_error" || result.Reason == "dry_run" {
				return
			}
			continue
		}

		e.log.Info("auto-entry: ENTERING %s on %s (%s, net %.1f%% APR)",
			opp.Symbol, opp.Exchange, opp.Direction, opp.NetAPR*100)

		err := e.ManualOpen(opp.Symbol, opp.Exchange, opp.Direction)
		if err != nil {
			e.log.Error("auto-entry: FAILED %s on %s: %v", opp.Symbol, opp.Exchange, err)
			continue
		}

		e.log.Info("auto-entry: SUCCESS %s on %s (%s)", opp.Symbol, opp.Exchange, opp.Direction)

		// Telegram alert for auto-entry.
		if e.telegram != nil {
			if pos := e.findActivePosition(opp.Symbol, opp.Exchange); pos != nil {
				e.telegram.NotifyAutoEntry(pos, opp.NetAPR)
			}
		}
		// Sequential: only one entry per scan cycle. After success, the next
		// scan will re-evaluate with updated capacity.
		return
	}
}
