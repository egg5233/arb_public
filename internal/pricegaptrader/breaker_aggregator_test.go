package pricegaptrader

import "testing"

// Wave 0 stubs for the RealizedPnLAggregator behaviors. Plan 15-02 fills in.

// TestAggregator_24hFilter — Pitfall 5: positions with close_ts older than
// now-24h must NOT contribute to the rolling sum.
func TestAggregator_24hFilter(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-02 implements RealizedPnLAggregator")
}

// TestAggregator_MissingYesterdaySet — Pitfall 6: SMEMBERS on missing key
// returns empty list, not error; aggregator must handle today-only operation.
func TestAggregator_MissingYesterdaySet(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-02 implements RealizedPnLAggregator")
}

// TestAggregator_DedupAcrossTodayYesterday — IDs may appear in both today and
// yesterday SETs around midnight; aggregator must dedup before summing.
func TestAggregator_DedupAcrossTodayYesterday(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-02 implements RealizedPnLAggregator")
}

// TestAggregator_RealizedPnLFieldName — verifies pos.RealizedPnL is the
// canonical source field (not RealizedPnLUSDT or any aliased name).
// Assumption A1 in 15-RESEARCH.md.
func TestAggregator_RealizedPnLFieldName(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-02 implements RealizedPnLAggregator")
}
