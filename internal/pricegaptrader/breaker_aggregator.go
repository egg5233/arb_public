package pricegaptrader

// Phase 15 (PG-LIVE-02) — RealizedPnLAggregator.
//
// Sums realized PnL of positions closed within `[now-24h, now]` by reading the
// Phase 14 `pg:positions:closed:{YYYY-MM-DD}` SET indices (CONTEXT D-02 — read-
// only consumer, no new key). Uses ExchangeClosedAt as the close-time source
// with ClosedAt as fallback (mirrors reconciler.go:255 precedence).
//
// Key properties (RESEARCH):
//   - Pitfall 5: stale entries past 24h are filtered out by the cutoff.
//   - Pitfall 6: missing yesterday SET (e.g. fresh deploy) is treated as empty,
//     not an error. Aggregator continues with today-only.
//   - Dedup: an ID appearing in BOTH today and yesterday SETs (clock-skew SADD
//     window around midnight) contributes exactly once.
//   - LoadPriceGapPosition errors / not-found: the aggregator SKIPS the offending
//     ID and continues — a single missing position must not abort the entire
//     trip evaluation.
//
// This file delivers ONLY the aggregator math; the breaker eval tick that
// consumes Realized24h ships in Plan 15-03.

import (
	"context"
	"fmt"
	"time"

	"arb/internal/models"
)

// BreakerAggregatorStore — narrow interface for RealizedPnLAggregator. Production
// wires *database.Client (which already implements both methods); tests inject
// an in-memory fake. The narrow surface preserves the D-15 module boundary —
// the aggregator does not import internal/database.
type BreakerAggregatorStore interface {
	GetPriceGapClosedPositionsForDate(date string) ([]string, error)
	LoadPriceGapPosition(id string) (*models.PriceGapPosition, bool, error)
}

// RealizedPnLAggregator computes rolling-24h realized PnL by reading the
// Phase 14 pg:positions:closed:{YYYY-MM-DD} SET indices.
type RealizedPnLAggregator struct {
	store BreakerAggregatorStore
}

// NewRealizedPnLAggregator binds the aggregator to a store. Production wires
// *database.Client; tests inject a fake.
func NewRealizedPnLAggregator(store BreakerAggregatorStore) *RealizedPnLAggregator {
	return &RealizedPnLAggregator{store: store}
}

// Realized24h returns the sum of pos.RealizedPnL for all positions closed
// within `[now-24h, now]` per CONTEXT D-02. Uses ExchangeClosedAt with ClosedAt
// fallback (matches Phase 14 reconciler.go close-time precedence). Resilient
// to missing SET keys (Pitfall 6) and stale entries past 24h (Pitfall 5).
//
// Closed-trade losses only — open-position drawdown is intentionally out of
// scope per CONTEXT D-02 (rolling 24h window).
func (a *RealizedPnLAggregator) Realized24h(ctx context.Context, now time.Time) (float64, error) {
	today := now.UTC().Format("2006-01-02")
	yesterday := now.Add(-24 * time.Hour).UTC().Format("2006-01-02")
	cutoff := now.Add(-24 * time.Hour)

	idsToday, err := a.store.GetPriceGapClosedPositionsForDate(today)
	if err != nil {
		return 0, fmt.Errorf("breaker_aggregator: smembers today=%s: %w", today, err)
	}
	idsYesterday, err := a.store.GetPriceGapClosedPositionsForDate(yesterday)
	if err != nil {
		return 0, fmt.Errorf("breaker_aggregator: smembers yesterday=%s: %w", yesterday, err)
	}

	// Dedup across the two SETs (clock-skew edge case around midnight SADD).
	seen := make(map[string]struct{}, len(idsToday)+len(idsYesterday))
	sum := 0.0

	all := make([]string, 0, len(idsToday)+len(idsYesterday))
	all = append(all, idsToday...)
	all = append(all, idsYesterday...)

	for _, id := range all {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}

		pos, ok, lerr := a.store.LoadPriceGapPosition(id)
		if lerr != nil || !ok || pos == nil {
			// Skip on load error / not-found — do NOT abort the aggregation.
			// A missing single position must not block the breaker's eval tick.
			continue
		}

		// ExchangeClosedAt preferred (mirrors reconciler.go:255), ClosedAt fallback.
		closeTs := pos.ExchangeClosedAt
		if closeTs.IsZero() {
			closeTs = pos.ClosedAt
		}
		if closeTs.Before(cutoff) {
			continue // Pitfall 5 — past the rolling 24h window.
		}

		sum += pos.RealizedPnL
	}
	return sum, nil
}
