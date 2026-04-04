package risk

import (
	"sync"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"

	"github.com/alicebob/miniredis/v2"
)

func newAllocatorTest(t *testing.T) (*CapitalAllocator, *database.Client, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	db, err := database.New(mr.Addr(), "", 2)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{
		EnableCapitalAllocator: true,
		MaxTotalExposureUSDT:   100,
		MaxPerpPerpPct:         0.60,
		MaxSpotFuturesPct:      0.40,
		MaxPerExchangePct:      0.60,
		ReservationTTLSec:      2,
		Leverage:               5,
	}

	return NewCapitalAllocator(db, cfg), db, mr
}

func TestCapitalAllocatorReserveAndCommit(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)

	res, err := allocator.Reserve(StrategyPerpPerp, map[string]float64{
		"binance": 20,
		"bybit":   20,
	})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if res == nil {
		t.Fatal("expected reservation")
	}

	if err := allocator.Commit(res, "pos-1", map[string]float64{
		"binance": 18,
		"bybit":   18,
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	summary, err := allocator.Summary()
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if got := summary.ByStrategy[StrategyPerpPerp]; got != 36 {
		t.Fatalf("expected perp exposure 36, got %.2f", got)
	}
	if got := summary.ByExchange["binance"]; got != 18 {
		t.Fatalf("expected binance exposure 18, got %.2f", got)
	}
	if summary.Reservations != 0 {
		t.Fatalf("expected 0 live reservations, got %d", summary.Reservations)
	}
}

func TestCapitalAllocatorRejectsStrategyCap(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)

	if _, err := allocator.Reserve(StrategySpotFutures, map[string]float64{"binance": 45}); err == nil {
		t.Fatal("expected strategy cap rejection")
	}
}

func TestCapitalAllocatorCountsBothStrategiesAgainstExchangeCap(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)

	perp, err := allocator.Reserve(StrategyPerpPerp, map[string]float64{"binance": 40})
	if err != nil {
		t.Fatalf("Reserve perp: %v", err)
	}
	if err := allocator.Commit(perp, "perp-1", nil); err != nil {
		t.Fatalf("Commit perp: %v", err)
	}

	if _, err := allocator.Reserve(StrategySpotFutures, map[string]float64{"binance": 25}); err == nil {
		t.Fatal("expected exchange cap rejection")
	}
}

func TestCapitalAllocatorReleasePosition(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)

	res, err := allocator.Reserve(StrategyPerpPerp, map[string]float64{"binance": 20, "bybit": 20})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := allocator.Commit(res, "pos-2", nil); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := allocator.ReleasePosition("pos-2"); err != nil {
		t.Fatalf("ReleasePosition: %v", err)
	}

	summary, err := allocator.Summary()
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if summary.TotalExposure != 0 {
		t.Fatalf("expected no committed exposure after release, got %.2f", summary.TotalExposure)
	}
}

func TestCapitalAllocatorReservationTTLExpiry(t *testing.T) {
	allocator, _, mr := newAllocatorTest(t)

	if _, err := allocator.Reserve(StrategyPerpPerp, map[string]float64{"binance": 20}); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	mr.FastForward(3 * time.Second)

	summary, err := allocator.Summary()
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if summary.TotalExposure != 0 {
		t.Fatalf("expected reservation to expire, got total %.2f", summary.TotalExposure)
	}
}

func TestCapitalAllocatorReconcileRebuildsState(t *testing.T) {
	allocator, db, _ := newAllocatorTest(t)

	now := time.Now().UTC()
	if err := db.SavePosition(&models.ArbitragePosition{
		ID:            "perp-a",
		Symbol:        "BTCUSDT",
		LongExchange:  "binance",
		ShortExchange: "bybit",
		LongSize:      0.1,
		ShortSize:     0.1,
		LongEntry:     1000,
		ShortEntry:    1000,
		Status:        models.StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("SavePosition: %v", err)
	}
	if err := db.SaveSpotPosition(&models.SpotFuturesPosition{
		ID:           "spot-a",
		Symbol:       "ETHUSDT",
		Exchange:     "binance",
		Status:       models.SpotStatusActive,
		NotionalUSDT: 25,
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("SaveSpotPosition: %v", err)
	}

	if err := allocator.Reconcile(); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	summary, err := allocator.Summary()
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if got := summary.ByStrategy[StrategyPerpPerp]; got != 40 {
		t.Fatalf("expected perp exposure 40, got %.2f", got)
	}
	if got := summary.ByStrategy[StrategySpotFutures]; got != 25 {
		t.Fatalf("expected spot exposure 25, got %.2f", got)
	}
}

// ---------------------------------------------------------------------------
// ComputeEffectiveAllocation tests (pure function, no allocator needed)
// ---------------------------------------------------------------------------

func TestPerformanceWeightedAllocationEqualAPR(t *testing.T) {
	perpPct, spotPct := ComputeEffectiveAllocation(10, 10, 0.50, 0.50, 0.20, 0.80)
	if perpPct != 0.50 || spotPct != 0.50 {
		t.Errorf("equal APR: got (%.4f, %.4f), want (0.50, 0.50)", perpPct, spotPct)
	}
}

func TestPerformanceWeightedAllocationPerpOutperforms(t *testing.T) {
	perpPct, spotPct := ComputeEffectiveAllocation(30, 10, 0.50, 0.50, 0.20, 0.80)
	if perpPct <= 0.50 {
		t.Errorf("perp outperforms: perpPct=%.4f should be > 0.50", perpPct)
	}
	if spotPct >= 0.50 {
		t.Errorf("perp outperforms: spotPct=%.4f should be < 0.50", spotPct)
	}
	sum := perpPct + spotPct
	if sum < 0.999 || sum > 1.001 {
		t.Errorf("sum should be 1.0, got %.6f", sum)
	}
}

func TestPerformanceWeightedAllocationBothZeroAPR(t *testing.T) {
	perpPct, spotPct := ComputeEffectiveAllocation(0, 0, 0.50, 0.50, 0.20, 0.80)
	if perpPct != 0.50 || spotPct != 0.50 {
		t.Errorf("zero APR: got (%.4f, %.4f), want (0.50, 0.50)", perpPct, spotPct)
	}
}

func TestPerformanceWeightedAllocationBothNegativeAPR(t *testing.T) {
	perpPct, spotPct := ComputeEffectiveAllocation(-5, -3, 0.50, 0.50, 0.20, 0.80)
	if perpPct != 0.50 || spotPct != 0.50 {
		t.Errorf("negative APR: got (%.4f, %.4f), want base split (0.50, 0.50)", perpPct, spotPct)
	}
}

func TestPerformanceWeightedAllocationClampedToFloorCeiling(t *testing.T) {
	// Extreme APR: perp=100, spot=1 — should be clamped
	perpPct, spotPct := ComputeEffectiveAllocation(100, 1, 0.50, 0.50, 0.20, 0.80)
	if perpPct > 0.80 {
		t.Errorf("perpPct=%.4f exceeds ceiling 0.80", perpPct)
	}
	if spotPct < 0.20 {
		t.Errorf("spotPct=%.4f below floor 0.20", spotPct)
	}
}

func TestPerformanceWeightedAllocationSumsToOne(t *testing.T) {
	cases := [][2]float64{
		{10, 10},
		{30, 10},
		{5, 50},
		{100, 1},
		{1, 100},
	}
	for _, c := range cases {
		perpPct, spotPct := ComputeEffectiveAllocation(c[0], c[1], 0.50, 0.50, 0.20, 0.80)
		sum := perpPct + spotPct
		if sum < 0.999 || sum > 1.001 {
			t.Errorf("APR (%.0f, %.0f): sum=%.6f, want 1.0", c[0], c[1], sum)
		}
	}
}

// ---------------------------------------------------------------------------
// EffectiveCapitalPerLeg tests
// ---------------------------------------------------------------------------

func TestEffectiveCapitalPerLegManualOverride(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)
	allocator.cfg.CapitalPerLeg = 500
	allocator.cfg.TotalCapitalUSDT = 10000
	allocator.cfg.EnableUnifiedCapital = true

	got := allocator.EffectiveCapitalPerLeg()
	if got != 500 {
		t.Errorf("manual override: got %.2f, want 500 (CapitalPerLeg takes precedence)", got)
	}
}

func TestEffectiveCapitalPerLegDerived(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)
	allocator.cfg.CapitalPerLeg = 0
	allocator.cfg.EnableUnifiedCapital = true
	allocator.cfg.TotalCapitalUSDT = 6000
	allocator.cfg.MaxPositions = 3
	allocator.cfg.SizeMultiplier = 1.0

	// Expected: 6000 / 3 / 2 = 1000
	got := allocator.EffectiveCapitalPerLeg()
	if got != 1000 {
		t.Errorf("derived: got %.2f, want 1000", got)
	}
}

func TestEffectiveCapitalPerLegAutoFromBalance(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)
	allocator.cfg.CapitalPerLeg = 0
	allocator.cfg.TotalCapitalUSDT = 0

	got := allocator.EffectiveCapitalPerLeg()
	if got != 0 {
		t.Errorf("auto from balance: got %.2f, want 0", got)
	}
}

func TestEffectiveCapitalPerLegClampsMaxPositions(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)
	allocator.cfg.CapitalPerLeg = 0
	allocator.cfg.EnableUnifiedCapital = true
	allocator.cfg.TotalCapitalUSDT = 1000
	allocator.cfg.MaxPositions = 0 // would cause division by zero
	allocator.cfg.SizeMultiplier = 1.0

	got := allocator.EffectiveCapitalPerLeg()
	// maxPos clamped to 1: 1000 / 1 / 2 = 500
	if got != 500 {
		t.Errorf("clamp maxPositions: got %.2f, want 500", got)
	}
}

func TestEffectiveCapitalPerLegAppliesSizeMultiplier(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)
	allocator.cfg.CapitalPerLeg = 0
	allocator.cfg.EnableUnifiedCapital = true
	allocator.cfg.TotalCapitalUSDT = 6000
	allocator.cfg.MaxPositions = 3
	allocator.cfg.SizeMultiplier = 1.3

	// Expected: (6000 / 3 / 2) * 1.3 = 1000 * 1.3 = 1300
	got := allocator.EffectiveCapitalPerLeg()
	if got != 1300 {
		t.Errorf("size multiplier: got %.2f, want 1300", got)
	}
}

// ---------------------------------------------------------------------------
// DynamicStrategyPct tests
// ---------------------------------------------------------------------------

func TestDynamicShiftingSpotNoOpps(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)
	allocator.cfg.EnableUnifiedCapital = true
	allocator.cfg.AllocationCeilingPct = 0.80
	allocator.SetEffectiveAllocation(0.50, 0.50)

	committed := map[Strategy]float64{
		StrategyPerpPerp:    10,
		StrategySpotFutures: 5,
	}

	perpPct := allocator.DynamicStrategyPct(StrategyPerpPerp, true, false, committed)
	if perpPct <= 0.50 {
		t.Errorf("spot has no opps: perpPct=%.4f should be > 0.50", perpPct)
	}
	if perpPct > 0.80 {
		t.Errorf("perpPct=%.4f exceeds ceiling 0.80", perpPct)
	}
}

func TestDynamicShiftingPerpNoOpps(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)
	allocator.cfg.EnableUnifiedCapital = true
	allocator.cfg.AllocationCeilingPct = 0.80
	allocator.SetEffectiveAllocation(0.50, 0.50)

	committed := map[Strategy]float64{
		StrategyPerpPerp:    5,
		StrategySpotFutures: 10,
	}

	spotPct := allocator.DynamicStrategyPct(StrategySpotFutures, false, true, committed)
	if spotPct <= 0.50 {
		t.Errorf("perp has no opps: spotPct=%.4f should be > 0.50", spotPct)
	}
	if spotPct > 0.80 {
		t.Errorf("spotPct=%.4f exceeds ceiling 0.80", spotPct)
	}
}

func TestDynamicShiftingDoesNotFreeCommitted(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)
	allocator.cfg.EnableUnifiedCapital = true
	allocator.cfg.MaxTotalExposureUSDT = 100
	allocator.cfg.AllocationCeilingPct = 0.80
	allocator.SetEffectiveAllocation(0.50, 0.50)

	// Spot has committed all its allocation (50 out of 50)
	committed := map[Strategy]float64{
		StrategyPerpPerp:    0,
		StrategySpotFutures: 50,
	}

	perpPct := allocator.DynamicStrategyPct(StrategyPerpPerp, true, false, committed)
	// Spot allocation = 50% * 100 = 50. Committed = 50. Freed = max(0, 50-50)/100 = 0
	// So perpPct should remain 0.50
	if perpPct != 0.50 {
		t.Errorf("committed not freed: perpPct=%.4f, want 0.50", perpPct)
	}
}

func TestDynamicShiftingBothHaveOpps(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)
	allocator.cfg.EnableUnifiedCapital = true
	allocator.cfg.AllocationCeilingPct = 0.80
	allocator.SetEffectiveAllocation(0.60, 0.40)

	committed := map[Strategy]float64{}

	perpPct := allocator.DynamicStrategyPct(StrategyPerpPerp, true, true, committed)
	if perpPct != 0.60 {
		t.Errorf("both have opps: perpPct=%.4f, want 0.60", perpPct)
	}

	spotPct := allocator.DynamicStrategyPct(StrategySpotFutures, true, true, committed)
	if spotPct != 0.40 {
		t.Errorf("both have opps: spotPct=%.4f, want 0.40", spotPct)
	}
}

func TestCapitalAllocatorConcurrentReservePreventsOvercommit(t *testing.T) {
	allocator, _, _ := newAllocatorTest(t)

	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0

	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := allocator.Reserve(StrategyPerpPerp, map[string]float64{"binance": 35})
			if err != nil || res == nil {
				return
			}
			mu.Lock()
			successes++
			mu.Unlock()
		}()
	}
	wg.Wait()

	if successes != 1 {
		t.Fatalf("expected exactly 1 successful reservation, got %d", successes)
	}
}
