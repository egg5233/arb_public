package spotengine

import (
	"fmt"

	"arb/pkg/exchange"
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

	// 5. Price gap gate: reject entry if spot-vs-futures gap is too wide.
	if e.cfg.SpotFuturesEnablePriceGapGate {
		gapPct, err := e.calculateEntryPriceGap(symbol, exchName, opp.Direction)
		if err != nil {
			e.log.Warn("risk-gate: price gap check failed for %s on %s: %v", symbol, exchName, err)
			return RiskGateResult{Allowed: false, Reason: "price_gap_check_error"}
		}
		maxGap := e.cfg.SpotFuturesMaxPriceGapPct
		if maxGap <= 0 {
			maxGap = 0.5
		}
		if gapPct > maxGap {
			return RiskGateResult{
				Allowed: false,
				Reason:  fmt.Sprintf("price_gap_%.2f%%>%.2f%%", gapPct, maxGap),
			}
		}
	}

	// 6. Maintenance rate: reject if survivable drop < leverage-scaled threshold.
	//    Per D-01: inserts as check 6, before dry-run.
	//    Per D-02: threshold = 90% / leverage.
	//    Per D-05: same threshold for all exchanges.
	if e.cfg.SpotFuturesEnableMaintenanceGate {
		// Calculate planned notional for tier matching (per D-18).
		capitalPerLeg := e.cfg.SpotFuturesCapitalSeparate
		if !isSeparateAccount(exchName) {
			capitalPerLeg = e.cfg.SpotFuturesCapitalUnified
		}
		leverage := float64(e.cfg.SpotFuturesLeverage)
		if leverage <= 0 {
			leverage = 3.0
		}
		plannedNotional := capitalPerLeg * leverage

		maintenanceRate := e.getMaintenanceRate(symbol, exchName, plannedNotional)
		survivableDrop := (1.0 / leverage) - maintenanceRate
		entryThreshold := 0.90 / leverage

		if survivableDrop < entryThreshold {
			return RiskGateResult{
				Allowed: false,
				Reason:  fmt.Sprintf("maintenance_survivable_%.1f%%<%.1f%%", survivableDrop*100, entryThreshold*100),
			}
		}
	}

	// 7. Dry-run: all real checks passed — log the would-be entry and skip execution.
	if e.cfg.SpotFuturesDryRun {
		e.log.Info("risk-gate: DRY RUN — would enter %s on %s (%s, net %.1f%% APR)",
			symbol, exchName, opp.Direction, opp.NetAPR*100)
		return RiskGateResult{Allowed: false, Reason: "dry_run"}
	}

	return RiskGateResult{Allowed: true}
}

// calculateEntryPriceGap calculates the real spot-vs-futures entry gap as a percentage.
func (e *SpotEngine) calculateEntryPriceGap(symbol, exchName, direction string) (float64, error) {
	futExch, ok := e.exchanges[exchName]
	if !ok {
		return 0, fmt.Errorf("exchange %s not found", exchName)
	}
	spotExch, ok := e.spotMargin[exchName]
	if !ok {
		return 0, fmt.Errorf("spot margin exchange %s not found", exchName)
	}

	futBBO, err := getFuturesBBO(futExch, symbol)
	if err != nil {
		return 0, err
	}
	spotBBO, err := spotExch.GetSpotBBO(symbol)
	if err != nil {
		return 0, fmt.Errorf("GetSpotBBO: %w", err)
	}
	if spotBBO.Bid <= 0 || spotBBO.Ask <= 0 {
		return 0, fmt.Errorf("invalid spot bid/ask for %s", symbol)
	}

	if direction == "borrow_sell_long" {
		return (futBBO.Ask - spotBBO.Bid) / spotBBO.Bid * 100, nil
	}
	if futBBO.Bid <= 0 {
		return 0, fmt.Errorf("invalid futures bid for %s", symbol)
	}
	return (spotBBO.Ask - futBBO.Bid) / futBBO.Bid * 100, nil
}

// MaintenanceWarning returns a human-readable warning string if the given
// symbol on the given exchange has a maintenance rate that would fail the
// pre-entry gate. Returns "" if the gate is disabled or the rate is safe.
// Used by the API handler to annotate manual open responses (per D-04).
func (e *SpotEngine) MaintenanceWarning(symbol, exchName string) string {
	if !e.cfg.SpotFuturesEnableMaintenanceGate {
		return ""
	}
	leverage := float64(e.cfg.SpotFuturesLeverage)
	if leverage <= 0 {
		leverage = 3.0
	}
	capitalPerLeg := e.cfg.SpotFuturesCapitalSeparate
	if !isSeparateAccount(exchName) {
		capitalPerLeg = e.cfg.SpotFuturesCapitalUnified
	}
	notional := capitalPerLeg * leverage
	mr := e.getMaintenanceRate(symbol, exchName, notional)
	survivable := (1.0 / leverage) - mr
	threshold := 0.90 / leverage
	if survivable < threshold {
		return fmt.Sprintf("maintenance_rate=%.1f%%, survivable=%.1f%%<%.1f%% threshold — position is at elevated liquidation risk",
			mr*100, survivable*100, threshold*100)
	}
	return ""
}

func getFuturesBBO(exch exchange.Exchange, symbol string) (exchange.BBO, error) {
	if bbo, ok := exch.GetBBO(symbol); ok {
		if bbo.Bid > 0 && bbo.Ask > 0 {
			return bbo, nil
		}
	}

	ob, err := exch.GetOrderbook(symbol, 5)
	if err != nil {
		return exchange.BBO{}, fmt.Errorf("GetOrderbook: %w", err)
	}
	if len(ob.Bids) == 0 || len(ob.Asks) == 0 {
		return exchange.BBO{}, fmt.Errorf("empty orderbook for %s", symbol)
	}
	if ob.Bids[0].Price <= 0 || ob.Asks[0].Price <= 0 {
		return exchange.BBO{}, fmt.Errorf("invalid futures bid/ask for %s", symbol)
	}
	return exchange.BBO{Bid: ob.Bids[0].Price, Ask: ob.Asks[0].Price}, nil
}
