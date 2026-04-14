package engine

import (
	"arb/internal/config"
	"arb/internal/models"
)

// unified_value.go — spot-futures scoring for the cross-strategy entry
// selector.
//
// The perp-perp side already has `computeAllocatorBaseValue` in
// allocator.go which returns USDT value over `cfg.MinHoldTime.Hours()`
// for a perp pair given its spread, notional, per-exchange fees and
// hold horizon. The B&B solver in selection_core.go is objective-agnostic;
// it just wants a comparable USDT number per candidate.
//
// To compare a spot-futures candidate alongside perp pairs the solver
// needs a matching "value over the same horizon" number for each spot
// plan. scoreSpotEntry produces that number using the same horizon and
// the exact USDT that will be reserved (PlannedNotionalUSDT, NOT the
// raw CapitalBudgetUSDT), so reservation and scoring see identical
// notional.

// SpotValueBreakdown is the output of scoreSpotEntry. NetValueUSDT is
// the comparable USDT-over-horizon number the B&B solver ranks on; the
// remaining fields are surfaced for logging and test assertions so it
// is easy to inspect why a given candidate was preferred or dropped.
//
// All APR / FeePct inputs are decimal fractions (e.g. 0.12 for 12%),
// NOT percentage points. The plan enforces this convention at the
// SpotEntryCandidate level; scoreSpotEntry trusts it and performs no
// percent conversion. See plan Section 6.
type SpotValueBreakdown struct {
	// Direction is the candidate direction label — carried through to
	// the breakdown for logging ("borrow_sell_long" or "buy_spot_short").
	Direction string

	// HoldHours is cfg.MinHoldTime.Hours(). Identical to the horizon
	// computeAllocatorBaseValue uses for the perp side so the two
	// strategies can be compared apples-to-apples.
	HoldHours float64

	// NotionalUSDT is plan.PlannedNotionalUSDT — the exact USDT the
	// batch reservation will hold. All downstream terms scale off this
	// number, NOT the pre-cap CapitalBudgetUSDT.
	NotionalUSDT float64

	// FundingAPR is the candidate's annualized funding rate (decimal
	// fraction) on the leg that receives funding for its direction.
	FundingAPR float64

	// BorrowAPR is the annualized spot-margin borrow cost (decimal
	// fraction). Zero for Direction B (no borrow required).
	BorrowAPR float64

	// GrossCarryAPR is FundingAPR - BorrowAPR. For Direction B this
	// simplifies to FundingAPR because BorrowAPR is zero at the candidate
	// level already.
	GrossCarryAPR float64

	// FeePct is the one-time round-trip fee estimate (decimal fraction).
	FeePct float64

	// GrossCarryUSDT is the pre-fee USDT carry earned over the horizon:
	// GrossCarryAPR * HoldHours/8760 * NotionalUSDT.
	GrossCarryUSDT float64

	// FeeUSDT is the one-time fee cost in USDT: FeePct * NotionalUSDT.
	FeeUSDT float64

	// NetValueUSDT is GrossCarryUSDT - FeeUSDT — the USDT-over-horizon
	// number the B&B compares with perp's computeAllocatorBaseValue.
	NetValueUSDT float64
}

// scoreSpotEntry returns the SpotValueBreakdown for a feasibility-checked
// SpotEntryPlan, using cfg.MinHoldTime as the horizon so spot and perp
// candidates share the same hold window.
//
// Design rules (per plan Section 6):
//   - Use plan.PlannedNotionalUSDT (NOT CapitalBudgetUSDT). The scoring
//     input must match the reservation input so the solver ranks on the
//     same USDT that ReserveBatch will actually hold.
//   - HoldHours = cfg.MinHoldTime.Hours(). Identical to the perp horizon.
//   - APR fields are decimal fractions; do NOT divide by 100.
//   - GrossCarryAPR = FundingAPR - BorrowAPR. Dir B has BorrowAPR = 0 at
//     the candidate level, so GrossCarryAPR collapses to FundingAPR.
//   - SpotArbOpportunity.NetAPR is NOT consulted here — the live field
//     does not include fees (discovery.go:247,274,450), so the selector
//     does its own math from FundingAPR / BorrowAPR / FeePct.
//
// Defensive behavior: a nil plan or a nil config returns an all-zero
// breakdown rather than panicking — the selector is expected to skip
// any candidate whose breakdown produces a non-positive NetValueUSDT
// anyway, so a zero breakdown is a safe "drop me" signal.
func scoreSpotEntry(plan *models.SpotEntryPlan, cfg *config.Config) SpotValueBreakdown {
	if plan == nil || cfg == nil {
		return SpotValueBreakdown{}
	}

	holdHours := cfg.MinHoldTime.Hours()
	notional := plan.PlannedNotionalUSDT

	fundingAPR := plan.Candidate.FundingAPR
	borrowAPR := plan.Candidate.BorrowAPR
	feePct := plan.Candidate.FeePct

	grossCarryAPR := fundingAPR - borrowAPR
	// 8760 = hours per year (24 * 365). Matches plan's published formula.
	grossCarryUSDT := grossCarryAPR * holdHours / 8760.0 * notional
	feeUSDT := feePct * notional
	netValueUSDT := grossCarryUSDT - feeUSDT

	return SpotValueBreakdown{
		Direction:      plan.Candidate.Direction,
		HoldHours:      holdHours,
		NotionalUSDT:   notional,
		FundingAPR:     fundingAPR,
		BorrowAPR:      borrowAPR,
		GrossCarryAPR:  grossCarryAPR,
		FeePct:         feePct,
		GrossCarryUSDT: grossCarryUSDT,
		FeeUSDT:        feeUSDT,
		NetValueUSDT:   netValueUSDT,
	}
}
