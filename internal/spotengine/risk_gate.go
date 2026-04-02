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

	// 5. Basis gate: reject entry if spot-futures basis is too wide (per D-10).
	if e.cfg.SpotFuturesEnableBasisGate {
		basisPct, err := e.calculateEntryBasis(symbol, exchName)
		if err != nil {
			e.log.Warn("risk-gate: basis check failed for %s on %s: %v", symbol, exchName, err)
			return RiskGateResult{Allowed: false, Reason: "basis_check_error"}
		}
		maxBasis := e.cfg.SpotFuturesMaxBasisPct
		if maxBasis <= 0 {
			maxBasis = 0.5
		}
		if basisPct > maxBasis {
			return RiskGateResult{
				Allowed: false,
				Reason:  fmt.Sprintf("basis_%.2f%%>%.2f%%", basisPct, maxBasis),
			}
		}
	}

	// 6. Dry-run: all real checks passed — log the would-be entry and skip execution.
	if e.cfg.SpotFuturesDryRun {
		e.log.Info("risk-gate: DRY RUN — would enter %s on %s (%s, net %.1f%% APR)",
			symbol, exchName, opp.Direction, opp.NetAPR*100)
		return RiskGateResult{Allowed: false, Reason: "dry_run"}
	}

	return RiskGateResult{Allowed: true}
}

// calculateEntryBasis estimates the spot-futures basis as a percentage.
// Uses the futures orderbook bid-ask spread as a proxy for basis since
// we don't have a separate spot price feed. For USDT pairs, this is
// a conservative estimate (actual basis is typically smaller than the
// full bid-ask spread).
func (e *SpotEngine) calculateEntryBasis(symbol, exchName string) (float64, error) {
	exch, ok := e.exchanges[exchName]
	if !ok {
		return 0, fmt.Errorf("exchange %s not found", exchName)
	}
	ob, err := exch.GetOrderbook(symbol, 5)
	if err != nil {
		return 0, fmt.Errorf("GetOrderbook: %w", err)
	}
	if len(ob.Bids) == 0 || len(ob.Asks) == 0 {
		return 0, fmt.Errorf("empty orderbook for %s", symbol)
	}
	mid := (ob.Bids[0].Price + ob.Asks[0].Price) / 2
	if mid <= 0 {
		return 0, fmt.Errorf("zero mid price for %s", symbol)
	}
	// Spread as basis proxy: (ask - bid) / mid * 100.
	spreadPct := (ob.Asks[0].Price - ob.Bids[0].Price) / mid * 100
	return spreadPct, nil
}
