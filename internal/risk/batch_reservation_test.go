package risk

import (
	"context"
	"testing"
)

// TestReserveBatch_AtomicAllOrNothing verifies that when any single item in a
// batch would breach a cap, NO reservations persist — the batch fails and
// Redis state is unchanged.
func TestReserveBatch_AtomicAllOrNothing(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)

	// Per-exchange cap = MaxTotalExposureUSDT * MaxPerExchangePct = 100 * 0.60 = 60.
	// First item fits (binance=30). Second item pushes binance to 65 which
	// breaches the exchange cap — whole batch must be rejected with zero
	// persistence.
	items := []BatchReservationItem{
		{
			Key:       "pair-1",
			Strategy:  StrategyPerpPerp,
			Exposures: map[string]float64{"binance": 30, "bybit": 20},
		},
		{
			Key:       "pair-2",
			Strategy:  StrategyPerpPerp,
			Exposures: map[string]float64{"binance": 35, "bybit": 10},
		},
	}

	batch, err := allocator.ReserveBatch(items)
	if err == nil {
		t.Fatalf("expected batch rejection when cumulative exchange cap breached, got nil err and batch=%+v", batch)
	}
	if batch != nil {
		t.Fatalf("expected nil batch on rejection, got %+v", batch)
	}

	summary, err := allocator.Summary()
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if summary.TotalExposure != 0 {
		t.Fatalf("expected zero exposure after failed batch, got %.2f", summary.TotalExposure)
	}
	if summary.Reservations != 0 {
		t.Fatalf("expected zero live reservations after failed batch, got %d", summary.Reservations)
	}
}

// TestReserveBatch_RespectsPerStrategyCap verifies that cumulative
// per-strategy exposures are checked across the batch, not just per-item.
func TestReserveBatch_RespectsPerStrategyCap(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)

	// Spot strategy cap = MaxTotalExposureUSDT * MaxSpotFuturesPct = 100 * 0.40 = 40.
	// Each item is within cap individually but cumulative spot exposure = 30 + 20 = 50 > 40.
	items := []BatchReservationItem{
		{
			Key:       "spot-1",
			Strategy:  StrategySpotFutures,
			Exposures: map[string]float64{"binance": 30},
		},
		{
			Key:       "spot-2",
			Strategy:  StrategySpotFutures,
			Exposures: map[string]float64{"bybit": 20},
		},
	}

	batch, err := allocator.ReserveBatch(items)
	if err == nil {
		t.Fatalf("expected cumulative strategy cap rejection, got nil err")
	}
	if batch != nil {
		t.Fatalf("expected nil batch, got %+v", batch)
	}

	summary, err := allocator.Summary()
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if got := summary.ByStrategy[StrategySpotFutures]; got != 0 {
		t.Fatalf("expected zero spot exposure after failed batch, got %.2f", got)
	}
	if summary.Reservations != 0 {
		t.Fatalf("expected zero live reservations after failed batch, got %d", summary.Reservations)
	}
}

// TestReserveBatch_RespectsPerExchangeCap verifies that cumulative
// per-exchange exposures are checked across strategies, since per-exchange
// caps count both strategies together.
func TestReserveBatch_RespectsPerExchangeCap(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)

	// Per-exchange cap = 100 * 0.60 = 60. Mixing strategies on the same
	// exchange must still be aggregated against the per-exchange cap.
	items := []BatchReservationItem{
		{
			Key:       "perp-1",
			Strategy:  StrategyPerpPerp,
			Exposures: map[string]float64{"binance": 40},
		},
		{
			Key:       "spot-1",
			Strategy:  StrategySpotFutures,
			Exposures: map[string]float64{"binance": 25},
		},
	}

	batch, err := allocator.ReserveBatch(items)
	if err == nil {
		t.Fatalf("expected per-exchange cumulative cap rejection, got nil err")
	}
	if batch != nil {
		t.Fatalf("expected nil batch, got %+v", batch)
	}

	summary, err := allocator.Summary()
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if got := summary.ByExchange["binance"]; got != 0 {
		t.Fatalf("expected zero binance exposure after failed batch, got %.2f", got)
	}
}

// TestReserveBatch_SuccessPersistsAllItems confirms the happy path: every
// item in a valid batch produces a live reservation and the batch carries
// them back keyed by caller-supplied Key.
func TestReserveBatch_SuccessPersistsAllItems(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)

	items := []BatchReservationItem{
		{
			Key:       "pair-1",
			Strategy:  StrategyPerpPerp,
			Exposures: map[string]float64{"binance": 20, "bybit": 20},
		},
		{
			Key:       "spot-1",
			Strategy:  StrategySpotFutures,
			Exposures: map[string]float64{"okx": 15},
		},
	}

	batch, err := allocator.ReserveBatch(items)
	if err != nil {
		t.Fatalf("ReserveBatch: %v", err)
	}
	if batch == nil {
		t.Fatal("expected non-nil batch")
	}
	if len(batch.Items) != 2 {
		t.Fatalf("expected 2 reservations in batch, got %d", len(batch.Items))
	}
	for _, key := range []string{"pair-1", "spot-1"} {
		if _, ok := batch.Items[key]; !ok {
			t.Fatalf("expected batch to contain key %q", key)
		}
	}

	summary, err := allocator.Summary()
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if summary.Reservations != 2 {
		t.Fatalf("expected 2 live reservations, got %d", summary.Reservations)
	}
	if summary.TotalExposure != 55 {
		t.Fatalf("expected total exposure 55, got %.2f", summary.TotalExposure)
	}
}

// TestReleaseBatch_KeepsCommittedReservations verifies that reservations
// already committed into positions are NOT rolled back by ReleaseBatch —
// ReleaseBatch only touches uncommitted reservations so the selector can
// commit winners then release losers safely.
func TestReleaseBatch_KeepsCommittedReservations(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)

	items := []BatchReservationItem{
		{
			Key:       "winner",
			Strategy:  StrategyPerpPerp,
			Exposures: map[string]float64{"binance": 20, "bybit": 20},
		},
		{
			Key:       "loser",
			Strategy:  StrategyPerpPerp,
			Exposures: map[string]float64{"gateio": 15},
		},
	}

	batch, err := allocator.ReserveBatch(items)
	if err != nil {
		t.Fatalf("ReserveBatch: %v", err)
	}

	// Commit the winner before releasing the batch.
	winner := batch.Items["winner"]
	if err := allocator.Commit(winner, "pos-winner", nil); err != nil {
		t.Fatalf("Commit winner: %v", err)
	}

	if err := allocator.ReleaseBatch(batch); err != nil {
		t.Fatalf("ReleaseBatch: %v", err)
	}

	summary, err := allocator.Summary()
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	// Winner is committed (40 total) — exposure stays. Loser reservation was
	// released, so no extra reservation exposure.
	if got := summary.ByStrategy[StrategyPerpPerp]; got != 40 {
		t.Fatalf("expected committed perp exposure 40 after ReleaseBatch, got %.2f", got)
	}
	if summary.Reservations != 0 {
		t.Fatalf("expected zero live reservations after ReleaseBatch, got %d", summary.Reservations)
	}
}

// TestReleaseBatch_ReleasesOnlyUncommitted verifies that uncommitted
// reservation keys are deleted and Redis exposure drops back to zero.
func TestReleaseBatch_ReleasesOnlyUncommitted(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)

	items := []BatchReservationItem{
		{
			Key:       "a",
			Strategy:  StrategyPerpPerp,
			Exposures: map[string]float64{"binance": 10},
		},
		{
			Key:       "b",
			Strategy:  StrategyPerpPerp,
			Exposures: map[string]float64{"bybit": 10},
		},
	}

	batch, err := allocator.ReserveBatch(items)
	if err != nil {
		t.Fatalf("ReserveBatch: %v", err)
	}

	// Pre-check: both reservation keys exist.
	ctx := context.Background()
	for key, res := range batch.Items {
		ex, err := allocator.db.Redis().Exists(ctx, allocator.reservationKey(res.ID)).Result()
		if err != nil {
			t.Fatalf("pre-check Exists %q: %v", key, err)
		}
		if ex != 1 {
			t.Fatalf("expected reservation %q to exist before release", key)
		}
	}

	if err := allocator.ReleaseBatch(batch); err != nil {
		t.Fatalf("ReleaseBatch: %v", err)
	}

	// Post-check: both reservation keys are gone.
	for key, res := range batch.Items {
		ex, err := allocator.db.Redis().Exists(ctx, allocator.reservationKey(res.ID)).Result()
		if err != nil {
			t.Fatalf("post-check Exists %q: %v", key, err)
		}
		if ex != 0 {
			t.Fatalf("expected reservation %q to be deleted after release", key)
		}
	}

	summary, err := allocator.Summary()
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if summary.TotalExposure != 0 {
		t.Fatalf("expected zero exposure after ReleaseBatch, got %.2f", summary.TotalExposure)
	}
	if summary.Reservations != 0 {
		t.Fatalf("expected zero live reservations after ReleaseBatch, got %d", summary.Reservations)
	}
}
