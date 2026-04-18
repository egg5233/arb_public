package spotengine

import (
	"testing"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// liqDistEngine creates a SpotEngine for liquidation distance trigger testing.
// It returns the engine along with a mockMaintenanceProvider so tests can set
// the rate returned by getMaintenanceRate.
func liqDistEngine(markPrice, maintRate float64, leverage int, gateEnabled bool) (*SpotEngine, *mockMaintenanceProvider) {
	provider := &mockMaintenanceProvider{rate: maintRate}
	mock := &mockExchange{
		Exchange: priceStubExchange{price: markPrice},
		provider: provider,
	}

	cfg := &config.Config{
		SpotFuturesEnableMaintenanceGate: gateEnabled,
		SpotFuturesLeverage:              leverage,
		SpotFuturesMaintenanceDefault:    0.05,
		SpotFuturesMaintenanceCacheTTL:   60,
		// Set high thresholds for other triggers so they don't fire.
		SpotFuturesPriceExitPct:       999.0,
		SpotFuturesPriceEmergencyPct:  999.0,
		SpotFuturesMarginExitPct:      999.0,
		SpotFuturesMarginEmergencyPct: 999.0,
	}

	e := &SpotEngine{
		exchanges: map[string]exchange.Exchange{"testexch": mock},
		cfg:       cfg,
		log:       utils.NewLogger("test-liq-dist"),
	}
	return e, provider
}

// TestLiqDistanceEmergency_HighMaintenanceRate tests the GUAUSDT scenario:
// 30% maintenance at 3x, price near entry -> triggers emergency.
func TestLiqDistanceEmergency_HighMaintenanceRate(t *testing.T) {
	// 3x leverage, 30% maintenance, entry=100, mark=100
	// estLiqPrice(long) = 100*(1 - 1/3 + 0.30) = 100*(0.667 + 0.30) = 96.7
	// distance = |100-96.7|/100 = 3.3%
	// entryThreshold = 0.90/3 = 0.30
	// emergThreshold = 0.30 * 0.10 = 0.03 = 3%
	// 3.3% > 3% => actually above emergency, but below exit threshold (6%).
	// Wait, let me recalculate: distance=3.3%, emergThreshold=3% -> 3.3% > 3%
	// So this should be between emergency and exit. Let me set mark=99 to push closer.
	// Actually mark=100: distance = (100-96.7)/100 = 0.033 = 3.3%
	// emergThreshold = 0.03, so 3.3% > 3% => not emergency.
	// Let me use mark=97 to get distance < 3%.
	// estLiqPrice = 96.7, mark=97, distance = (97-96.7)/97 = 0.003 = 0.31%
	// 0.31% < 3% => emergency!

	e, _ := liqDistEngine(97.0, 0.30, 3, true)
	pos := &models.SpotFuturesPosition{
		Symbol:       "GUAUSDT",
		Exchange:     "testexch",
		Direction:    "borrow_sell_long",
		FuturesEntry: 100.0,
		FuturesSide:  "long",
		NotionalUSDT: 1000.0,
		Status:       models.SpotStatusActive,
	}

	reason, isEmergency := e.checkExitTriggers(pos)
	if reason != "liq_distance_emergency" {
		t.Errorf("reason = %q, want %q", reason, "liq_distance_emergency")
	}
	if !isEmergency {
		t.Error("expected isEmergency = true")
	}
}

// TestLiqDistanceExit_ModerateDanger tests exit trigger (not emergency).
// 5% maintenance at 3x, price dropped enough to be below exit threshold but above emergency.
func TestLiqDistanceExit_ModerateDanger(t *testing.T) {
	// 3x leverage, 5% maintenance, entry=100
	// estLiqPrice(long) = 100*(1 - 1/3 + 0.05) = 100*0.717 = 71.7
	// entryThreshold = 0.90/3 = 0.30
	// exitThreshold = 0.30 * 0.20 = 0.06 = 6%
	// emergThreshold = 0.30 * 0.10 = 0.03 = 3%
	// To be below exit (6%) but above emerg (3%): distance needs to be ~4-5%.
	// mark=75: distance = (75-71.7)/75 = 3.3/75 = 4.4% -> below 6%, above 3%.
	e, _ := liqDistEngine(75.0, 0.05, 3, true)
	pos := &models.SpotFuturesPosition{
		Symbol:       "TESTUSDT",
		Exchange:     "testexch",
		Direction:    "borrow_sell_long",
		FuturesEntry: 100.0,
		FuturesSide:  "long",
		NotionalUSDT: 1000.0,
		Status:       models.SpotStatusActive,
	}

	reason, isEmergency := e.checkExitTriggers(pos)
	if reason != "liq_distance_exit" {
		t.Errorf("reason = %q, want %q", reason, "liq_distance_exit")
	}
	if isEmergency {
		t.Error("expected isEmergency = false for exit-level trigger")
	}
}

// TestLiqDistanceWarn_SlightDanger tests warn-only scenario (no exit trigger returned).
// 5% maintenance at 3x, price dropped enough for warn but not exit.
func TestLiqDistanceWarn_SlightDanger(t *testing.T) {
	// entryThreshold = 0.90/3 = 0.30
	// warnThreshold = 0.30 * 0.50 = 0.15 = 15%
	// exitThreshold = 0.30 * 0.20 = 0.06 = 6%
	// estLiqPrice(long) = 100*(1 - 1/3 + 0.05) = 71.7
	// Need distance between 6% and 15%.
	// mark=82: distance = (82-71.7)/82 = 10.3/82 = 12.6% -> below 15%, above 6% -> warn only
	e, _ := liqDistEngine(82.0, 0.05, 3, true)
	pos := &models.SpotFuturesPosition{
		Symbol:       "TESTUSDT",
		Exchange:     "testexch",
		Direction:    "borrow_sell_long",
		FuturesEntry: 100.0,
		FuturesSide:  "long",
		NotionalUSDT: 1000.0,
		Status:       models.SpotStatusActive,
	}

	reason, _ := e.checkExitTriggers(pos)
	if reason != "" {
		t.Errorf("reason = %q, want empty (warn only, no exit trigger)", reason)
	}
}

// TestLiqDistanceSafe_LowMaintenance tests that no trigger fires when distance is safe.
// 0.5% maintenance at 3x, mark near entry.
func TestLiqDistanceSafe_LowMaintenance(t *testing.T) {
	// 3x leverage, 0.5% maintenance, entry=100, mark=85
	// estLiqPrice(long) = 100*(1 - 1/3 + 0.005) = 100*0.672 = 67.2
	// distance = (85-67.2)/85 = 17.8/85 = 20.9%
	// warnThreshold = 0.30 * 0.50 = 0.15 = 15%
	// 20.9% > 15% -> safe, no trigger
	e, _ := liqDistEngine(85.0, 0.005, 3, true)
	pos := &models.SpotFuturesPosition{
		Symbol:       "TESTUSDT",
		Exchange:     "testexch",
		Direction:    "borrow_sell_long",
		FuturesEntry: 100.0,
		FuturesSide:  "long",
		NotionalUSDT: 1000.0,
		Status:       models.SpotStatusActive,
	}

	reason, _ := e.checkExitTriggers(pos)
	if reason != "" {
		t.Errorf("reason = %q, want empty (safe distance)", reason)
	}
}

// TestLiqDistance_DisabledWhenGateOff tests that trigger 2b is completely skipped
// when EnableMaintenanceGate = false.
func TestLiqDistance_DisabledWhenGateOff(t *testing.T) {
	// Same values as emergency test, but gate is OFF.
	e, _ := liqDistEngine(97.0, 0.30, 3, false)
	pos := &models.SpotFuturesPosition{
		Symbol:       "GUAUSDT",
		Exchange:     "testexch",
		Direction:    "borrow_sell_long",
		FuturesEntry: 100.0,
		FuturesSide:  "long",
		NotionalUSDT: 1000.0,
		Status:       models.SpotStatusActive,
	}

	reason, _ := e.checkExitTriggers(pos)
	if reason != "" {
		t.Errorf("reason = %q, want empty (gate disabled)", reason)
	}
}

// TestLiqDistance_ShortFutures tests Direction B (short futures).
// Verifies the short formula: estLiqPrice = entry * (1 + 1/leverage - maintenanceRate).
func TestLiqDistance_ShortFutures(t *testing.T) {
	// 3x leverage, 5% maintenance, entry=100, mark=120
	// estLiqPrice(short) = 100*(1 + 1/3 - 0.05) = 100*1.283 = 128.3
	// distance = (128.3-120)/120 = 8.3/120 = 6.9%
	// entryThreshold = 0.90/3 = 0.30
	// exitThreshold = 0.30 * 0.20 = 0.06 = 6%
	// warnThreshold = 0.30 * 0.50 = 0.15 = 15%
	// 6.9% > exitThreshold(6%) but < warnThreshold(15%) -> warn only, no exit
	e, _ := liqDistEngine(120.0, 0.05, 3, true)
	pos := &models.SpotFuturesPosition{
		Symbol:       "TESTUSDT",
		Exchange:     "testexch",
		Direction:    "buy_spot_short",
		FuturesEntry: 100.0,
		FuturesSide:  "short",
		NotionalUSDT: 1000.0,
		Status:       models.SpotStatusActive,
	}

	reason, _ := e.checkExitTriggers(pos)
	if reason != "" {
		t.Errorf("reason = %q, want empty (short futures, distance 6.9%% > exit 6%% = warn only)", reason)
	}
}

// TestLiqDistance_ShortFuturesEmergency tests short futures emergency trigger.
func TestLiqDistance_ShortFuturesEmergency(t *testing.T) {
	// 3x leverage, 5% maintenance, entry=100
	// estLiqPrice(short) = 100*(1 + 1/3 - 0.05) = 128.3
	// mark=127: distance = (128.3-127)/127 = 1.3/127 = 1.02%
	// emergThreshold = 0.30 * 0.10 = 3%
	// 1.02% < 3% -> emergency!
	e, _ := liqDistEngine(127.0, 0.05, 3, true)
	pos := &models.SpotFuturesPosition{
		Symbol:       "TESTUSDT",
		Exchange:     "testexch",
		Direction:    "buy_spot_short",
		FuturesEntry: 100.0,
		FuturesSide:  "short",
		NotionalUSDT: 1000.0,
		Status:       models.SpotStatusActive,
	}

	reason, isEmergency := e.checkExitTriggers(pos)
	if reason != "liq_distance_emergency" {
		t.Errorf("reason = %q, want %q", reason, "liq_distance_emergency")
	}
	if !isEmergency {
		t.Error("expected isEmergency = true")
	}
}
