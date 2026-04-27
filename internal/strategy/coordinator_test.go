package strategy

import (
	"sync"
	"testing"

	"arb/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		StrategyPriority:  config.StrategyPriorityPerpPerpFirst,
		ExpectedHoldHours: 24,
	}
}

func TestCoordinatorDefaultSnapshot(t *testing.T) {
	c := NewCoordinator(testConfig())
	s := c.Snapshot()

	if s.EnableStrategyPriority {
		t.Fatalf("expected EnableStrategyPriority=false")
	}
	if s.Mode != config.StrategyPriorityPerpPerpFirst {
		t.Fatalf("expected Mode=%s, got %s", config.StrategyPriorityPerpPerpFirst, s.Mode)
	}
	if s.EffectiveMode != config.StrategyPriorityPerpPerpFirst {
		t.Fatalf("expected EffectiveMode=%s, got %s", config.StrategyPriorityPerpPerpFirst, s.EffectiveMode)
	}
	if s.ExpectedHoldHours != 24 {
		t.Fatalf("expected ExpectedHoldHours=24, got %v", s.ExpectedHoldHours)
	}
	if s.Epoch != 0 {
		t.Fatalf("expected Epoch=0, got %d", s.Epoch)
	}
}

func TestCoordinatorStagedModeDisabledKeepsEffectiveDefault(t *testing.T) {
	cfg := testConfig()
	cfg.StrategyPriority = config.StrategyPriorityDirBFirst

	c := NewCoordinator(cfg)
	s := c.Snapshot()

	if s.Mode != config.StrategyPriorityDirBFirst {
		t.Fatalf("expected staged Mode=%s, got %s", config.StrategyPriorityDirBFirst, s.Mode)
	}
	if s.EffectiveMode != config.StrategyPriorityPerpPerpFirst {
		t.Fatalf("expected disabled EffectiveMode=%s, got %s", config.StrategyPriorityPerpPerpFirst, s.EffectiveMode)
	}
}

func TestCoordinatorEnabledExposesStagedMode(t *testing.T) {
	cfg := testConfig()
	cfg.EnableStrategyPriority = true
	cfg.StrategyPriority = config.StrategyPriorityDirBFirst

	c := NewCoordinator(cfg)
	s := c.Snapshot()

	if !s.EnableStrategyPriority {
		t.Fatalf("expected EnableStrategyPriority=true")
	}
	if s.EffectiveMode != config.StrategyPriorityDirBFirst {
		t.Fatalf("expected EffectiveMode=%s, got %s", config.StrategyPriorityDirBFirst, s.EffectiveMode)
	}
}

func TestCoordinatorCapitalAllocatorMirrorsConfig(t *testing.T) {
	cfg := testConfig()
	cfg.EnableCapitalAllocator = true

	c := NewCoordinator(cfg)
	if !c.Snapshot().CapitalAllocatorOn {
		t.Fatalf("expected CapitalAllocatorOn=true")
	}
}

func TestCoordinatorEpochChangesOnlyOnSnapshotChange(t *testing.T) {
	cfg := testConfig()
	c := NewCoordinator(cfg)

	c.UpdatePriority(cfg)
	if got := c.Snapshot().Epoch; got != 0 {
		t.Fatalf("expected unchanged epoch 0, got %d", got)
	}

	cfg.Lock()
	cfg.StrategyPriority = config.StrategyPriorityDirBOnly
	cfg.Unlock()
	c.UpdatePriority(cfg)
	if got := c.Snapshot().Epoch; got != 1 {
		t.Fatalf("expected epoch 1 after change, got %d", got)
	}

	c.UpdatePriority(cfg)
	if got := c.Snapshot().Epoch; got != 1 {
		t.Fatalf("expected epoch unchanged at 1, got %d", got)
	}
}

func TestCoordinatorConcurrentSnapshotAndUpdate(t *testing.T) {
	cfg := testConfig()
	c := NewCoordinator(cfg)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			cfg.Lock()
			cfg.EnableStrategyPriority = i%2 == 0
			if i%3 == 0 {
				cfg.StrategyPriority = config.StrategyPriorityDirBFirst
			} else {
				cfg.StrategyPriority = config.StrategyPriorityPerpPerpFirst
			}
			cfg.ExpectedHoldHours = float64(12 + i)
			cfg.EnableCapitalAllocator = i%4 == 0
			cfg.Unlock()
			c.UpdatePriority(cfg)
		}(i)
		go func() {
			defer wg.Done()
			_ = c.Snapshot()
		}()
	}
	wg.Wait()
}

func TestTryReserveManyModeDenialAndManualOverride(t *testing.T) {
	cfg := testConfig()
	cfg.EnableStrategyPriority = true
	cfg.StrategyPriority = config.StrategyPriorityDirBOnly
	c := NewCoordinator(cfg)
	snap := c.Snapshot()
	keys := []LegKey{{Exchange: "binance", Market: "futures", Symbol: "BTCUSDT"}}

	res := c.TryReserveMany(snap, keys, StrategyPP, 10, ReserveOptions{Source: "pp_manual", CandidateID: "manual"})
	if res.Granted || res.Reason != ReserveReasonManualOverrideNeeded {
		t.Fatalf("expected override_required, got granted=%v reason=%s", res.Granted, res.Reason)
	}

	res = c.TryReserveMany(snap, keys, StrategyPP, 10, ReserveOptions{ManualOverride: true, Source: "pp_manual", CandidateID: "manual"})
	if !res.Granted || res.ReservationID == "" {
		t.Fatalf("expected manual override reservation, got %+v", res)
	}
}

func TestTryReserveManyDirBFirstBumpsPendingPP(t *testing.T) {
	cfg := testConfig()
	cfg.EnableStrategyPriority = true
	cfg.StrategyPriority = config.StrategyPriorityDirBFirst
	c := NewCoordinator(cfg)
	snap := c.Snapshot()
	keys := []LegKey{{Exchange: "binance", Market: "futures", Symbol: "BTCUSDT"}}

	pp := c.TryReserveMany(snap, keys, StrategyPP, 100, ReserveOptions{Source: "pp_auto", CandidateID: "pp"})
	if !pp.Granted {
		t.Fatalf("expected pp pending reservation, got %+v", pp)
	}
	dirB := c.TryReserveMany(snap, keys, StrategyDirB, 1, ReserveOptions{Source: "sf_auto", CandidateID: "dirb"})
	if !dirB.Granted || dirB.ReservationID == pp.ReservationID {
		t.Fatalf("expected dir_b to bump pending pp, pp=%+v dirB=%+v", pp, dirB)
	}
	if err := c.MarkInFlight(pp.ReservationID); err == nil {
		t.Fatalf("expected bumped pp reservation to be gone")
	}
}

func TestTryReserveManyInFlightNotBumpable(t *testing.T) {
	cfg := testConfig()
	cfg.EnableStrategyPriority = true
	cfg.StrategyPriority = config.StrategyPriorityDirBFirst
	c := NewCoordinator(cfg)
	snap := c.Snapshot()
	keys := []LegKey{{Exchange: "binance", Market: "futures", Symbol: "BTCUSDT"}}

	pp := c.TryReserveMany(snap, keys, StrategyPP, 100, ReserveOptions{Source: "pp_auto", CandidateID: "pp"})
	if !pp.Granted {
		t.Fatalf("expected pp reservation, got %+v", pp)
	}
	if err := c.MarkInFlight(pp.ReservationID); err != nil {
		t.Fatalf("MarkInFlight: %v", err)
	}
	dirB := c.TryReserveMany(snap, keys, StrategyDirB, 1000, ReserveOptions{Source: "sf_auto", CandidateID: "dirb"})
	if dirB.Granted || dirB.Reason != ReserveReasonConflictInFlight {
		t.Fatalf("expected in-flight conflict, got %+v", dirB)
	}
}
