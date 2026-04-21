package pricegaptrader

import (
	"strconv"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
)

// rehydrate runs at Start(); for each position in pg:positions:active:
//   - Verify BOTH legs still exist on the exchange (GetPosition total > 0).
//   - If either leg is zero-size (exchange side closed while we were down),
//     stamp ExitReasonOrphan and move to history.
//   - Otherwise, re-enroll in monitorPosition goroutine.
//
// Idempotent: startMonitor cancels any prior monitor for the same posID before
// registering the replacement (see monitor.go). So calling rehydrate twice
// leaves exactly one live monitor per survivor. Covered by
// TestRehydrate_IdempotentOnSecondCall.
//
// Pitfall guardrails (RESEARCH §Pitfall 3 — "ghost position replay"): the
// exchange may have closed one or both legs during downtime via stop-loss,
// liquidation, or manual action. If we re-enroll an unbalanced position, the
// monitor would later fire an exit on a leg that no longer exists — double-fee
// at best, adverse-selection at worst. legsZeroOrGone vetoes this.
func (t *Tracker) rehydrate() {
	active, err := t.db.GetActivePriceGapPositions()
	if err != nil {
		t.log.Warn("pricegap: rehydrate GetActivePriceGapPositions failed: %v", err)
		return
	}
	t.log.Info("pricegap: rehydrating %d active positions", len(active))

	for _, pos := range active {
		if t.legsZeroOrGone(pos) {
			pos.ExitReason = models.ExitReasonOrphan
			pos.Status = models.PriceGapStatusClosed
			pos.ClosedAt = time.Now()
			if err := t.db.SavePriceGapPosition(pos); err != nil {
				t.log.Warn("pricegap: rehydrate SavePriceGapPosition %s: %v", pos.ID, err)
			}
			_ = t.db.RemoveActivePriceGapPosition(pos.ID)
			_ = t.db.AddPriceGapHistory(pos)
			t.log.Warn("pricegap: rehydrate orphaned %s — marked closed (one leg zero-size on exchange)", pos.ID)
			continue
		}
		t.startMonitor(pos)
	}
}

// legsZeroOrGone returns true if either leg's on-exchange position size is 0
// (exchange already closed it while tracker was offline) OR unreachable.
//
// Adapter surface notes:
//   - GetPosition(symbol) returns ([]exchange.Position, error) — a slice because
//     some adapters report long+short simultaneously (hedge mode). We sum the
//     parsed Total across entries; empty slice or all-zero Total → zero-size.
//   - Position.Total is a string (exchange-native precision preserved); parse
//     float; non-numeric or negative Total treated as zero.
//   - Network / adapter errors: conservatively treat as "gone" to trigger
//     orphan stamping — better to close the book on a possibly-stale position
//     than re-enroll and fire an exit against nothing.
func (t *Tracker) legsZeroOrGone(pos *models.PriceGapPosition) bool {
	longEx, ok := t.exchanges[pos.LongExchange]
	if !ok {
		return true
	}
	shortEx, ok := t.exchanges[pos.ShortExchange]
	if !ok {
		return true
	}

	longPositions, err := longEx.GetPosition(pos.Symbol)
	if err != nil {
		return true
	}
	shortPositions, err := shortEx.GetPosition(pos.Symbol)
	if err != nil {
		return true
	}

	if positionTotalSum(longPositions) <= 0 {
		return true
	}
	if positionTotalSum(shortPositions) <= 0 {
		return true
	}
	return false
}

// positionTotalSum parses Position.Total (string, exchange-native precision)
// across a slice and returns the sum of non-negative values. Invalid strings
// and negatives contribute 0, matching the "conservative = treat as zero" policy
// in legsZeroOrGone.
func positionTotalSum(ps []exchange.Position) float64 {
	var sum float64
	for _, p := range ps {
		if p.Total == "" {
			continue
		}
		v, err := strconv.ParseFloat(p.Total, 64)
		if err != nil || v <= 0 {
			continue
		}
		sum += v
	}
	return sum
}
