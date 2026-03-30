package spotengine

import (
	"fmt"
)

// RiskGateResult holds the outcome of a pre-entry risk gate evaluation.
type RiskGateResult struct {
	Allowed bool
	Reason  string // empty if allowed; descriptive tag if blocked
}

// checkRiskGate runs all pre-entry risk checks for a spot-futures auto-entry.
// All checks must pass for the entry to proceed. Checks are ordered so that
// cheap/fast checks run first (capacity, duplicate, cooldown) before the more
// stateful persistence check. Dry-run is evaluated last so that its "would
// enter" log only fires when the opportunity genuinely passes all real gates.
func (e *SpotEngine) checkRiskGate(opp SpotArbOpportunity) RiskGateResult {
	symbol := opp.Symbol
	exchName := opp.Exchange

	// 1. Capacity: activeCount < SpotFuturesMaxPositions.
	active, err := e.db.GetActiveSpotPositions()
	if err != nil {
		e.log.Error("risk-gate: failed to get active positions: %v", err)
		return RiskGateResult{Allowed: false, Reason: "db_error"}
	}
	if len(active) >= e.cfg.SpotFuturesMaxPositions {
		return RiskGateResult{Allowed: false, Reason: fmt.Sprintf("at_capacity_%d/%d", len(active), e.cfg.SpotFuturesMaxPositions)}
	}

	// 2. Duplicate: no active position for same symbol (any exchange).
	for _, pos := range active {
		if pos.Symbol == symbol {
			return RiskGateResult{Allowed: false, Reason: fmt.Sprintf("duplicate_%s_on_%s", symbol, pos.Exchange)}
		}
	}

	// 3. Cooldown: no arb:spot_cooldown:{symbol} key in Redis.
	cooled, err := e.db.HasSpotCooldown(symbol)
	if err != nil {
		e.log.Error("risk-gate: cooldown check error for %s: %v", symbol, err)
		return RiskGateResult{Allowed: false, Reason: "cooldown_check_error"}
	}
	if cooled {
		return RiskGateResult{Allowed: false, Reason: fmt.Sprintf("cooldown_%s", symbol)}
	}

	// 4. Persistence: symbol must appear in N consecutive scans.
	required := e.cfg.SpotFuturesPersistenceScans
	if required > 0 {
		count := e.getPersistenceCount(symbol)
		if count < required {
			return RiskGateResult{Allowed: false, Reason: fmt.Sprintf("persistence_%d/%d", count, required)}
		}
	}

	// 5. Dry-run: all real checks passed — log the would-be entry and skip execution.
	if e.cfg.SpotFuturesDryRun {
		e.log.Info("risk-gate: DRY RUN — would enter %s on %s (%s, net %.1f%% APR)",
			symbol, exchName, opp.Direction, opp.NetAPR*100)
		return RiskGateResult{Allowed: false, Reason: "dry_run"}
	}

	return RiskGateResult{Allowed: true}
}
