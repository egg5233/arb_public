package spotengine

import (
	"fmt"
	"strings"

	"arb/internal/api"
	"arb/internal/models"
	"arb/internal/strategy"
)

func spotStrategyForDirection(direction string) (strategy.Strategy, bool) {
	switch strings.TrimSpace(direction) {
	case "borrow_sell_long":
		return strategy.StrategyDirA, true
	case "buy_spot_short":
		return strategy.StrategyDirB, true
	default:
		return "", false
	}
}

func spotStrategyLegKeys(symbol, exchName, direction string) []strategy.LegKey {
	keys := []strategy.LegKey{
		{Exchange: exchName, Market: "futures", Symbol: symbol},
	}
	if direction == "borrow_sell_long" {
		keys = append(keys, strategy.LegKey{Exchange: exchName, Market: "spot_margin", Symbol: symbol})
	} else {
		keys = append(keys, strategy.LegKey{Exchange: exchName, Market: "spot", Symbol: symbol})
	}
	return keys
}

func spotStrategyEV(opp *SpotArbOpportunity, expectedHoldHours float64) strategy.EVBreakdown {
	if opp == nil {
		return strategy.EVBreakdown{}
	}
	funding := opp.FundingAPR * 10000 / 8760
	fees := opp.FeePct * 10000
	if expectedHoldHours <= 0 {
		expectedHoldHours = 24
	}
	feeBpsH := fees / expectedHoldHours
	borrow := opp.BorrowAPR * 10000 / 8760
	return strategy.EVBreakdown{
		FundingBpsH: funding,
		FeesBpsH:    feeBpsH,
		BorrowBpsH:  borrow,
		NetBpsH:     funding - feeBpsH - borrow,
	}
}

func spotStrategyCandidateID(snap strategy.StrategySnapshot, strat strategy.Strategy, source, symbol, exchName, direction string) string {
	keys := strategy.LegKeyStrings(spotStrategyLegKeys(symbol, exchName, direction))
	return fmt.Sprintf("epoch=%d strategy=%s source=%s direction=%s keys=%s", snap.Epoch, strat, source, direction, strings.Join(keys, ","))
}

func (e *SpotEngine) openWithStrategyOptions(symbol, exchName, direction string, opts api.ManualOpenOptions, source string) error {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	exchName = strings.ToLower(strings.TrimSpace(exchName))
	direction = strings.TrimSpace(direction)

	if e.strategyCoord == nil {
		return e.manualOpen(symbol, exchName, direction, nil)
	}
	snap := e.strategyCoord.Snapshot()
	if !snap.EnableStrategyPriority {
		return e.manualOpen(symbol, exchName, direction, nil)
	}

	strat, ok := spotStrategyForDirection(direction)
	if !ok {
		return fmt.Errorf("invalid direction %q - must be borrow_sell_long or buy_spot_short", direction)
	}
	var opp *SpotArbOpportunity
	opps := e.getLatestOpps()
	for i := range opps {
		candidate := opps[i]
		if candidate.Symbol == symbol && candidate.Exchange == exchName && candidate.Direction == direction {
			opp = &candidate
			break
		}
	}
	ev := spotStrategyEV(opp, snap.ExpectedHoldHours)
	e.log.Info("[ev] candidate=%s strategy=%s funding_bps_h=%.4f fees_bps_h=%.4f borrow_bps_h=%.4f rotation_bps_h=%.4f ev_net_bps_h=%.4f",
		symbol, strat, ev.FundingBpsH, ev.FeesBpsH, ev.BorrowBpsH, ev.RotationBpsH, ev.NetBpsH)
	res := e.strategyCoord.TryReserveMany(snap, spotStrategyLegKeys(symbol, exchName, direction), strat, ev.NetBpsH, strategy.ReserveOptions{
		ManualOverride: opts.OverrideStrategyPriority,
		Source:         source,
		CandidateID:    spotStrategyCandidateID(snap, strat, source, symbol, exchName, direction),
	})
	if !res.Granted {
		return fmt.Errorf("strategy priority denied: %s", res.Reason)
	}
	if res.ReservationID == "" {
		return e.manualOpen(symbol, exchName, direction, nil)
	}
	if err := e.strategyCoord.MarkInFlight(res.ReservationID); err != nil {
		e.strategyCoord.Release(res.ReservationID)
		return fmt.Errorf("strategy reservation lost: %w", err)
	}

	meta, _ := e.strategyCoord.ReservationMeta(res.ReservationID)
	err := e.manualOpen(symbol, exchName, direction, &meta)
	if err != nil {
		e.strategyCoord.CompleteReservation(res.ReservationID, strategy.ReservationOutcomeAborted)
		e.strategyCoord.Release(res.ReservationID)
		return err
	}
	e.strategyCoord.CompleteReservation(res.ReservationID, strategy.ReservationOutcomeActive)
	e.strategyCoord.Release(res.ReservationID)
	return nil
}

func applySpotStrategyMeta(pos *models.SpotFuturesPosition, meta *strategy.PositionStrategyMeta) {
	if pos == nil || meta == nil || meta.ReservationID == "" {
		return
	}
	pos.StrategyReservationID = meta.ReservationID
	pos.StrategyCandidateID = meta.CandidateID
	pos.StrategyEpoch = meta.StrategyEpoch
	pos.Strategy = string(meta.Strategy)
	pos.StrategyLegKeys = strategy.LegKeyStrings(meta.Keys)
}

func (e *SpotEngine) bindSpotStrategyOrder(pos *models.SpotFuturesPosition, market, orderID string) {
	if e.strategyCoord == nil || pos == nil || pos.StrategyReservationID == "" || orderID == "" {
		return
	}
	key := strategy.LegKey{Exchange: pos.Exchange, Market: market, Symbol: pos.Symbol}
	if err := e.strategyCoord.BindOrder(pos.StrategyReservationID, key, orderID); err != nil {
		e.log.Warn("strategy bind order %s %s: %v", strategy.LegKeyString(key), orderID, err)
	}
}
