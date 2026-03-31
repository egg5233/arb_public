package engine

import (
	"fmt"
	"time"

	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/utils"
)

func (e *Engine) createPendingPosition(opp models.Opportunity) *models.ArbitragePosition {
	now := time.Now().UTC()
	return &models.ArbitragePosition{
		ID:            utils.GenerateID(opp.Symbol, now.UnixMilli()),
		Symbol:        opp.Symbol,
		LongExchange:  opp.LongExchange,
		ShortExchange: opp.ShortExchange,
		Status:        models.StatusPending,
		EntrySpread:   opp.Spread,
		AllExchanges:  []string{opp.LongExchange, opp.ShortExchange},
		NextFunding:   e.computeNextFunding(opp.Symbol, opp.LongExchange, opp.ShortExchange),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func (e *Engine) reservePerpCapital(opp models.Opportunity, approval *models.RiskApproval) (*risk.CapitalReservation, error) {
	if e.allocator == nil || !e.allocator.Enabled() {
		return nil, nil
	}
	return e.allocator.Reserve(risk.StrategyPerpPerp, map[string]float64{
		opp.LongExchange:  approval.RequiredMargin,
		opp.ShortExchange: approval.RequiredMargin,
	})
}

func (e *Engine) commitPerpCapital(res *risk.CapitalReservation, posID string) error {
	if e.allocator == nil || !e.allocator.Enabled() || res == nil {
		return nil
	}
	pos, err := e.db.GetPosition(posID)
	if err != nil {
		return fmt.Errorf("load position for capital commit: %w", err)
	}
	leverage := float64(e.cfg.Leverage)
	if leverage <= 0 {
		leverage = 1
	}
	return e.allocator.Commit(res, posID, map[string]float64{
		pos.LongExchange:  (pos.LongSize * pos.LongEntry) / leverage,
		pos.ShortExchange: (pos.ShortSize * pos.ShortEntry) / leverage,
	})
}

func (e *Engine) releasePerpReservation(res *risk.CapitalReservation) {
	if e.allocator == nil || !e.allocator.Enabled() || res == nil {
		return
	}
	if err := e.allocator.ReleaseReservation(res.ID); err != nil {
		e.log.Error("allocator release reservation %s: %v", res.ID, err)
	}
}

func (e *Engine) releasePerpPosition(posID string) {
	if e.allocator == nil || !e.allocator.Enabled() || posID == "" {
		return
	}
	if err := e.allocator.ReleasePosition(posID); err != nil {
		e.log.Error("allocator release position %s: %v", posID, err)
	}
}
