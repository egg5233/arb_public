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
