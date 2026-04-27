package engine

import (
	"fmt"
	"strings"

	"arb/internal/api"
	"arb/internal/models"
	"arb/internal/strategy"
)

func (e *Engine) ppStrategySnapshot() (strategy.StrategySnapshot, bool) {
	if e == nil || e.strategyCoord == nil {
		return strategy.StrategySnapshot{}, false
	}
	snap := e.strategyCoord.Snapshot()
	return snap, snap.EnableStrategyPriority
}

func ppStrategyLegKeys(opp models.Opportunity) []strategy.LegKey {
	return []strategy.LegKey{
		{Exchange: opp.LongExchange, Market: "futures", Symbol: opp.Symbol},
		{Exchange: opp.ShortExchange, Market: "futures", Symbol: opp.Symbol},
	}
}

func ppStrategyCandidateID(snap strategy.StrategySnapshot, source string, opp models.Opportunity) string {
	keys := strategy.LegKeyStrings(ppStrategyLegKeys(opp))
	return fmt.Sprintf("epoch=%d strategy=%s source=%s keys=%s", snap.Epoch, strategy.StrategyPP, source, strings.Join(keys, ","))
}

func (e *Engine) ppEVBreakdown(snap strategy.StrategySnapshot, opp models.Opportunity) strategy.EVBreakdown {
	feeBps := 0.0
	if e.discovery != nil {
		fees := e.discovery.GetExchangeFees()
		longFee := fees[opp.LongExchange].Taker * 100
		shortFee := fees[opp.ShortExchange].Taker * 100
		feeBps = (longFee + shortFee) * 2
	}
	return strategy.PPEV(opp.ShortRate, opp.LongRate, feeBps, 0, snap.ExpectedHoldHours)
}

func (e *Engine) tryReservePPStrategy(snap strategy.StrategySnapshot, source string, opp models.Opportunity, opts api.ManualOpenOptions) (string, error) {
	if e.strategyCoord == nil || !snap.EnableStrategyPriority {
		return "", nil
	}
	ev := e.ppEVBreakdown(snap, opp)
	e.log.Info("[ev] candidate=%s strategy=%s funding_bps_h=%.4f fees_bps_h=%.4f borrow_bps_h=%.4f rotation_bps_h=%.4f ev_net_bps_h=%.4f",
		opp.Symbol, strategy.StrategyPP, ev.FundingBpsH, ev.FeesBpsH, ev.BorrowBpsH, ev.RotationBpsH, ev.NetBpsH)
	res := e.strategyCoord.TryReserveMany(snap, ppStrategyLegKeys(opp), strategy.StrategyPP, ev.NetBpsH, strategy.ReserveOptions{
		ManualOverride: opts.OverrideStrategyPriority,
		Source:         source,
		CandidateID:    ppStrategyCandidateID(snap, source, opp),
	})
	if res.Granted {
		return res.ReservationID, nil
	}
	return "", fmt.Errorf("strategy priority denied: %s", res.Reason)
}

func (e *Engine) applyPPStrategyMeta(pos *models.ArbitragePosition, reservationID string) {
	if pos == nil || e.strategyCoord == nil || reservationID == "" {
		return
	}
	meta, ok := e.strategyCoord.ReservationMeta(reservationID)
	if !ok {
		return
	}
	pos.StrategyReservationID = meta.ReservationID
	pos.StrategyCandidateID = meta.CandidateID
	pos.StrategyEpoch = meta.StrategyEpoch
	pos.Strategy = string(meta.Strategy)
	pos.StrategyLegKeys = strategy.LegKeyStrings(meta.Keys)
}

func (e *Engine) completePPStrategyReservation(reservationID string, outcome strategy.ReservationOutcome) {
	if e.strategyCoord == nil || reservationID == "" {
		return
	}
	e.strategyCoord.CompleteReservation(reservationID, outcome)
	e.strategyCoord.Release(reservationID)
}

func (e *Engine) bindPPStrategyOrder(pos *models.ArbitragePosition, exchName, orderID string) {
	if e.strategyCoord == nil || pos == nil || pos.StrategyReservationID == "" || orderID == "" {
		return
	}
	key := strategy.LegKey{Exchange: exchName, Market: "futures", Symbol: pos.Symbol}
	if err := e.strategyCoord.BindOrder(pos.StrategyReservationID, key, orderID); err != nil {
		e.log.Warn("strategy bind order %s %s: %v", strategy.LegKeyString(key), orderID, err)
	}
}
