package engine

import (
	"math"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
)

// unified_value_test.go — unit tests for scoreSpotEntry in
// internal/engine/unified_value.go. Covers each direction, horizon
// source (MinHoldTime — NOT a hardcoded 8h), and the decimal-APR
// convention per plans/PLAN-unified-capital-allocator.md Section 6/11.

// makeSpotPlan builds a SpotEntryPlan carrying the supplied APR / fee
// inputs and the requested PlannedNotionalUSDT. Default direction is
// Dir A ("borrow_sell_long").
func makeSpotPlan(direction string, fundingAPR, borrowAPR, feePct, notional float64) *models.SpotEntryPlan {
	if direction == "" {
		direction = "borrow_sell_long"
	}
	return &models.SpotEntryPlan{
		Candidate: models.SpotEntryCandidate{
			Symbol:     "BTCUSDT",
			BaseCoin:   "BTC",
			Exchange:   "binance",
			Direction:  direction,
			FundingAPR: fundingAPR,
			BorrowAPR:  borrowAPR,
			FeePct:     feePct,
		},
		PlannedNotionalUSDT: notional,
		CapitalBudgetUSDT:   notional * 2, // intentionally different from PlannedNotionalUSDT
		PlannedBaseSize:     1.0,
		MidPrice:            notional, // irrelevant to scoring
	}
}

// approx returns true if a and b are within the given tolerance.
func approx(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

// TestScoreSpotEntry_DirA_DeductsBorrowAPRAndFees — Dir A carries a
// borrow cost; GrossCarryAPR must be FundingAPR - BorrowAPR, and the
// one-time fee must be subtracted from the gross carry in USDT terms.
//
// Inputs: 12% funding APR, 5% borrow APR, 0.1% fee, 1000 USDT notional,
// 16h hold. Expected: GrossCarryAPR = 0.07, GrossCarryUSDT = 0.07 *
// 16/8760 * 1000 ≈ 0.12785 USDT, FeeUSDT = 0.001 * 1000 = 1.0.
// NetValueUSDT = GrossCarryUSDT - FeeUSDT (negative here — the test
// cares about the ARITHMETIC, not the sign).
func TestScoreSpotEntry_DirA_DeductsBorrowAPRAndFees(t *testing.T) {
	cfg := &config.Config{MinHoldTime: 16 * time.Hour}
	plan := makeSpotPlan("borrow_sell_long", 0.12, 0.05, 0.001, 1000)

	got := scoreSpotEntry(plan, cfg)

	if got.Direction != "borrow_sell_long" {
		t.Fatalf("Direction: want borrow_sell_long, got %q", got.Direction)
	}
	if !approx(got.HoldHours, 16.0, 1e-9) {
		t.Fatalf("HoldHours: want 16, got %v", got.HoldHours)
	}
	if !approx(got.GrossCarryAPR, 0.07, 1e-9) {
		t.Fatalf("GrossCarryAPR: want 0.07, got %v", got.GrossCarryAPR)
	}
	wantGrossUSDT := 0.07 * 16.0 / 8760.0 * 1000.0
	if !approx(got.GrossCarryUSDT, wantGrossUSDT, 1e-6) {
		t.Fatalf("GrossCarryUSDT: want %v, got %v", wantGrossUSDT, got.GrossCarryUSDT)
	}
	if !approx(got.FeeUSDT, 1.0, 1e-9) {
		t.Fatalf("FeeUSDT: want 1.0, got %v", got.FeeUSDT)
	}
	wantNet := wantGrossUSDT - 1.0
	if !approx(got.NetValueUSDT, wantNet, 1e-6) {
		t.Fatalf("NetValueUSDT: want %v, got %v", wantNet, got.NetValueUSDT)
	}
}

// TestScoreSpotEntry_DirB_NoBorrowCost — Dir B has BorrowAPR = 0 at
// candidate level; GrossCarryAPR collapses to FundingAPR, no borrow
// term applied.
func TestScoreSpotEntry_DirB_NoBorrowCost(t *testing.T) {
	cfg := &config.Config{MinHoldTime: 16 * time.Hour}
	plan := makeSpotPlan("buy_spot_short", 0.20, 0.0, 0.0015, 2000)

	got := scoreSpotEntry(plan, cfg)

	if got.Direction != "buy_spot_short" {
		t.Fatalf("Direction: want buy_spot_short, got %q", got.Direction)
	}
	if got.BorrowAPR != 0 {
		t.Fatalf("BorrowAPR should be 0 for Dir B, got %v", got.BorrowAPR)
	}
	if !approx(got.GrossCarryAPR, 0.20, 1e-9) {
		t.Fatalf("GrossCarryAPR: want 0.20, got %v", got.GrossCarryAPR)
	}
	wantGrossUSDT := 0.20 * 16.0 / 8760.0 * 2000.0
	if !approx(got.GrossCarryUSDT, wantGrossUSDT, 1e-6) {
		t.Fatalf("GrossCarryUSDT: want %v, got %v", wantGrossUSDT, got.GrossCarryUSDT)
	}
	wantFee := 0.0015 * 2000.0
	if !approx(got.FeeUSDT, wantFee, 1e-6) {
		t.Fatalf("FeeUSDT: want %v, got %v", wantFee, got.FeeUSDT)
	}
}

// TestScoreSpotEntry_UsesMinHoldTimeNot8h — the horizon must come from
// cfg.MinHoldTime. Changing MinHoldTime must change GrossCarryUSDT
// proportionally; a hardcoded 8h would not react.
//
// Test: compute the same candidate at 8h and at 32h. The 32h result's
// GrossCarryUSDT must be exactly 4x the 8h result (linear in hours).
func TestScoreSpotEntry_UsesMinHoldTimeNot8h(t *testing.T) {
	plan := makeSpotPlan("borrow_sell_long", 0.10, 0.02, 0.0, 1000)

	got8 := scoreSpotEntry(plan, &config.Config{MinHoldTime: 8 * time.Hour})
	got32 := scoreSpotEntry(plan, &config.Config{MinHoldTime: 32 * time.Hour})

	if !approx(got8.HoldHours, 8, 1e-9) {
		t.Fatalf("8h scoring HoldHours: want 8, got %v", got8.HoldHours)
	}
	if !approx(got32.HoldHours, 32, 1e-9) {
		t.Fatalf("32h scoring HoldHours: want 32, got %v", got32.HoldHours)
	}
	// 32h carry should be exactly 4x 8h carry (linear in hours).
	ratio := got32.GrossCarryUSDT / got8.GrossCarryUSDT
	if !approx(ratio, 4.0, 1e-6) {
		t.Fatalf("GrossCarryUSDT must scale linearly with MinHoldTime: ratio 32h/8h want 4.0, got %v", ratio)
	}
}

// TestScoreSpotEntry_APRDecimalNotPercent — APRs are decimal fractions
// (0.12 for 12%), not percentages (12 for 12%). Confirms the formula
// does NOT divide by 100 internally.
//
// Input: FundingAPR=0.10 (10%), 1000 notional, 8760h (1 year) hold.
// Expected GrossCarryUSDT exactly 100.0 (10% of 1000 over a year).
// If the code treated 0.10 as "0.10%", we would see 0.10 USDT instead
// of 100.0 — off by 1000x.
func TestScoreSpotEntry_APRDecimalNotPercent(t *testing.T) {
	cfg := &config.Config{MinHoldTime: 8760 * time.Hour} // 1 year
	plan := makeSpotPlan("buy_spot_short", 0.10, 0.0, 0.0, 1000)

	got := scoreSpotEntry(plan, cfg)

	// 10% of 1000 over a full year = 100 USDT.
	if !approx(got.GrossCarryUSDT, 100.0, 1e-6) {
		t.Fatalf("GrossCarryUSDT at 10%% APR over 1 year on 1000 notional must be 100 (decimal APR), got %v",
			got.GrossCarryUSDT)
	}
	if !approx(got.NotionalUSDT, 1000.0, 1e-9) {
		t.Fatalf("NotionalUSDT must be PlannedNotionalUSDT (1000), got %v", got.NotionalUSDT)
	}
	// Must NOT have used CapitalBudgetUSDT (2000 in our helper).
	if approx(got.NotionalUSDT, 2000.0, 1e-6) {
		t.Fatalf("NotionalUSDT should be PlannedNotionalUSDT not CapitalBudgetUSDT")
	}
}
