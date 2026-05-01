package pricegaptrader

import (
	"context"
	"errors"
	"testing"
	"time"

	"arb/internal/models"
)

// fakeBreakerAggregatorStore — in-memory fake satisfying BreakerAggregatorStore.
// Keys are UTC date strings ("2006-01-02"); positions map id -> *PriceGapPosition.
// errOnDate is non-nil to simulate per-key SMEMBERS error.
type fakeBreakerAggregatorStore struct {
	sets       map[string][]string
	positions  map[string]*models.PriceGapPosition
	errOnDate  map[string]error
	notFoundOK map[string]bool // id -> true means LoadPriceGapPosition returns (nil,false,nil)
}

func newFakeBreakerAggStore() *fakeBreakerAggregatorStore {
	return &fakeBreakerAggregatorStore{
		sets:       map[string][]string{},
		positions:  map[string]*models.PriceGapPosition{},
		errOnDate:  map[string]error{},
		notFoundOK: map[string]bool{},
	}
}

func (f *fakeBreakerAggregatorStore) GetPriceGapClosedPositionsForDate(date string) ([]string, error) {
	if err, ok := f.errOnDate[date]; ok {
		return nil, err
	}
	return f.sets[date], nil
}

func (f *fakeBreakerAggregatorStore) LoadPriceGapPosition(id string) (*models.PriceGapPosition, bool, error) {
	if f.notFoundOK[id] {
		return nil, false, nil
	}
	pos, ok := f.positions[id]
	if !ok {
		return nil, false, nil
	}
	return pos, true, nil
}

// TestAggregator_24hFilter — Pitfall 5: positions with close_ts older than
// now-24h must NOT contribute to the rolling sum.
func TestAggregator_24hFilter(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	today := now.UTC().Format("2006-01-02")
	yesterday := now.Add(-24 * time.Hour).UTC().Format("2006-01-02")

	f := newFakeBreakerAggStore()
	// A — 23h ago, PnL=-30 → in window
	f.positions["A"] = &models.PriceGapPosition{ID: "A", RealizedPnL: -30, ExchangeClosedAt: now.Add(-23 * time.Hour)}
	// B — 25h ago, PnL=-100 → outside window
	f.positions["B"] = &models.PriceGapPosition{ID: "B", RealizedPnL: -100, ExchangeClosedAt: now.Add(-25 * time.Hour)}
	// C — 1h ago, PnL=-15 → in window
	f.positions["C"] = &models.PriceGapPosition{ID: "C", RealizedPnL: -15, ExchangeClosedAt: now.Add(-1 * time.Hour)}

	f.sets[yesterday] = []string{"A", "B"}
	f.sets[today] = []string{"C"}

	agg := NewRealizedPnLAggregator(f)
	got, err := agg.Realized24h(context.Background(), now)
	if err != nil {
		t.Fatalf("Realized24h: %v", err)
	}
	if want := -45.0; got != want {
		t.Fatalf("Realized24h = %v, want %v (A+C; B excluded by 24h cutoff)", got, want)
	}
}

// TestAggregator_MissingYesterdaySet — Pitfall 6: missing yesterday key returns
// empty (Redis SMEMBERS on missing key returns empty list, not error in our wrapper);
// aggregator must handle today-only operation without panic.
func TestAggregator_MissingYesterdaySet(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	today := now.UTC().Format("2006-01-02")

	f := newFakeBreakerAggStore()
	f.positions["X"] = &models.PriceGapPosition{ID: "X", RealizedPnL: -7, ExchangeClosedAt: now.Add(-30 * time.Minute)}
	f.sets[today] = []string{"X"}
	// yesterday set absent — fake returns nil, nil

	agg := NewRealizedPnLAggregator(f)
	got, err := agg.Realized24h(context.Background(), now)
	if err != nil {
		t.Fatalf("Realized24h: %v", err)
	}
	if want := -7.0; got != want {
		t.Fatalf("Realized24h = %v, want %v", got, want)
	}
}

// TestAggregator_DedupAcrossTodayYesterday — IDs may appear in both today and
// yesterday SETs around midnight (clock skew during SADD). Aggregator must
// dedupe before summing.
func TestAggregator_DedupAcrossTodayYesterday(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 5, 0, 0, time.UTC) // just past midnight
	today := now.UTC().Format("2006-01-02")
	yesterday := now.Add(-24 * time.Hour).UTC().Format("2006-01-02")

	f := newFakeBreakerAggStore()
	f.positions["D"] = &models.PriceGapPosition{ID: "D", RealizedPnL: -50, ExchangeClosedAt: now.Add(-10 * time.Minute)}
	// D is in BOTH sets — dedup must count once.
	f.sets[yesterday] = []string{"D"}
	f.sets[today] = []string{"D"}

	agg := NewRealizedPnLAggregator(f)
	got, err := agg.Realized24h(context.Background(), now)
	if err != nil {
		t.Fatalf("Realized24h: %v", err)
	}
	if want := -50.0; got != want {
		t.Fatalf("Realized24h = %v, want %v (D dedupted across sets)", got, want)
	}
}

// TestAggregator_RealizedPnLFieldName — locks pos.RealizedPnL as canonical.
// If PriceGapPosition.RealizedPnL is renamed in models, this test breaks.
func TestAggregator_RealizedPnLFieldName(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	today := now.UTC().Format("2006-01-02")

	f := newFakeBreakerAggStore()
	f.positions["F"] = &models.PriceGapPosition{ID: "F", RealizedPnL: -7.5, ExchangeClosedAt: now.Add(-2 * time.Hour)}
	f.sets[today] = []string{"F"}

	agg := NewRealizedPnLAggregator(f)
	got, err := agg.Realized24h(context.Background(), now)
	if err != nil {
		t.Fatalf("Realized24h: %v", err)
	}
	if want := -7.5; got != want {
		t.Fatalf("Realized24h = %v, want %v (canonical RealizedPnL field)", got, want)
	}
}

// TestAggregator_ExchangeClosedAtPreferredOverClosedAt — position with ClosedAt
// just outside 24h but ExchangeClosedAt inside 24h should be INCLUDED. Mirrors
// reconciler.go:255 close-time precedence (ExchangeClosedAt preferred).
func TestAggregator_ExchangeClosedAtPreferredOverClosedAt(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 1, 0, 0, time.UTC)
	today := now.UTC().Format("2006-01-02")

	f := newFakeBreakerAggStore()
	// ClosedAt = 2026-04-30 23:59:00 (just outside cutoff = 2026-04-30 00:01)? Wait math:
	// cutoff = now - 24h = 2026-04-30 00:01:00. So ClosedAt 2026-04-29 23:59 is BEFORE cutoff.
	// ExchangeClosedAt 2026-04-30 00:05 is AFTER cutoff. Aggregator should pick ExchangeClosedAt → INCLUDE.
	closedAtBeforeCutoff := time.Date(2026, 4, 29, 23, 59, 0, 0, time.UTC)
	exchangeClosedAtInside := time.Date(2026, 4, 30, 0, 5, 0, 0, time.UTC)
	f.positions["E"] = &models.PriceGapPosition{
		ID:               "E",
		RealizedPnL:      -42,
		ClosedAt:         closedAtBeforeCutoff,
		ExchangeClosedAt: exchangeClosedAtInside,
	}
	f.sets[today] = []string{"E"}

	agg := NewRealizedPnLAggregator(f)
	got, err := agg.Realized24h(context.Background(), now)
	if err != nil {
		t.Fatalf("Realized24h: %v", err)
	}
	if want := -42.0; got != want {
		t.Fatalf("Realized24h = %v, want %v (ExchangeClosedAt should be preferred)", got, want)
	}
}

// TestAggregator_BothKeysEmpty — fresh database, no closed positions; aggregator
// returns 0, no error.
func TestAggregator_BothKeysEmpty(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	f := newFakeBreakerAggStore()

	agg := NewRealizedPnLAggregator(f)
	got, err := agg.Realized24h(context.Background(), now)
	if err != nil {
		t.Fatalf("Realized24h: %v", err)
	}
	if got != 0 {
		t.Fatalf("Realized24h = %v, want 0", got)
	}
}

// TestAggregator_LoadPositionError_Skipped — position ID present in SET but
// LoadPriceGapPosition returns (nil, false, nil); aggregator skips and continues
// with remaining IDs (does NOT abort).
func TestAggregator_LoadPositionError_Skipped(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	today := now.UTC().Format("2006-01-02")

	f := newFakeBreakerAggStore()
	f.notFoundOK["GHOST"] = true // load returns (nil, false, nil)
	f.positions["REAL"] = &models.PriceGapPosition{ID: "REAL", RealizedPnL: -3, ExchangeClosedAt: now.Add(-1 * time.Hour)}
	f.sets[today] = []string{"GHOST", "REAL"}

	agg := NewRealizedPnLAggregator(f)
	got, err := agg.Realized24h(context.Background(), now)
	if err != nil {
		t.Fatalf("Realized24h: %v", err)
	}
	if want := -3.0; got != want {
		t.Fatalf("Realized24h = %v, want %v (GHOST skipped, REAL counted)", got, want)
	}
}

// Sanity: confirm error from store IS surfaced (defensive — not in plan's 7 tests
// but useful to lock the contract).
func TestAggregator_StoreErrorPropagated(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	today := now.UTC().Format("2006-01-02")

	f := newFakeBreakerAggStore()
	f.errOnDate[today] = errors.New("redis down")

	agg := NewRealizedPnLAggregator(f)
	_, err := agg.Realized24h(context.Background(), now)
	if err == nil {
		t.Fatalf("Realized24h err=nil, want non-nil from store error")
	}
}
