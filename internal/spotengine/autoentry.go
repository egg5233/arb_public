package spotengine

import (
	"strings"
	"time"
)

// attemptAutoEntries processes discovered opportunities and attempts automated
// entry for qualifying candidates. Entries are executed sequentially (one at a
// time) to prevent overcommit from parallel entry attempts.
func (e *SpotEngine) attemptAutoEntries(opps []SpotArbOpportunity) {
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
			// If at capacity or db error, stop evaluating further opportunities.
			if strings.HasPrefix(result.Reason, "at_capacity") || result.Reason == "db_error" {
				return
			}
			continue
		}

		// Acquire entry lock to prevent concurrent entries across scans.
		lockKey := "spot_auto_entry"
		acquired, err := e.db.AcquireLock(lockKey, 5*time.Minute)
		if err != nil || !acquired {
			e.log.Warn("auto-entry: could not acquire entry lock — skipping this scan")
			return
		}

		e.log.Info("auto-entry: ENTERING %s on %s (%s, net %.1f%% APR)",
			opp.Symbol, opp.Exchange, opp.Direction, opp.NetAPR*100)

		err = e.ManualOpen(opp.Symbol, opp.Exchange, opp.Direction)
		e.db.ReleaseLock(lockKey)

		if err != nil {
			e.log.Error("auto-entry: FAILED %s on %s: %v", opp.Symbol, opp.Exchange, err)
			continue
		}

		e.log.Info("auto-entry: SUCCESS %s on %s (%s)", opp.Symbol, opp.Exchange, opp.Direction)
		// Sequential: only one entry per scan cycle. After success, the next
		// scan will re-evaluate with updated capacity.
		return
	}
}
