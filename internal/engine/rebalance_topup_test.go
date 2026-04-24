package engine

import (
	"testing"

	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/exchange"
)

// TestPass1_TopUpApprovedRoutesToCaseB verifies that an approved approval with
// non-empty TopUpApplied is treated as a rescue candidate (case b), not case (a).
// Baseline bug: METUSDT was routed to case (a), no transfer planned, executor rejected.
//
// Observable contract for routing verification:
//
//	Case (a) BUG path:  candidates=0 → early-return fires → pass2=0 AND kept=0.
//	  METUSDT is deleted from `remaining` (disjoint-set invariant) so it does
//	  NOT appear in pass2Input. Signal: kept=0 AND pass2=0.
//
//	Case (b) FIX path:  candidates≥1 → early-return NOT taken → post-transfer
//	  replay runs. Relay may succeed (METUSDT in kept) or fail (METUSDT stays
//	  in remaining → pass2Input). Either way: kept+len(pass2)>0.
//
// Therefore: asserting NOT (kept=0 AND pass2=0) is sufficient to prove routing.
func TestPass1_TopUpApprovedRoutesToCaseB(t *testing.T) {
	cfg := rankFirstCfg()
	exchanges := map[string]exchange.Exchange{
		"gateio": richStub(),
		"bitget": richStub(),
	}
	e := buildRankFirstEngine(t, exchanges, cfg)

	// Inject a controlled simulator that always returns approved-with-topup.
	e.testSimulatorFn = func(
		opp models.Opportunity,
		longExch, shortExch string,
		reserved map[string]float64,
		alt *models.AlternativePair,
		cache *risk.PrefetchCache,
	) (*models.RiskApproval, error) {
		return &models.RiskApproval{
			Approved:          true,
			Size:              1596,
			Price:             0.188,
			RequiredMargin:    200,
			LongMarginNeeded:  199.93,
			ShortMarginNeeded: 199.71,
			TopUpApplied: map[string]float64{
				"gateio": 124,
				"bitget": 246,
			},
		}, nil
	}

	opps := []models.Opportunity{
		goodOpp("METUSDT", "gateio", "bitget"),
	}

	balances := map[string]rebalanceBalanceInfo{}
	for name, exch := range exchanges {
		bi := rebalanceBalanceInfo{}
		if futBal, err := exch.GetFuturesBalance(); err == nil {
			bi.futures = futBal.Available
			bi.futuresTotal = futBal.Total
		}
		balances[name] = bi
	}

	kept, pass2Input, _ := e.rebalancePass1(opps, nil, balances)

	// Primary assertion: case (b) routing was entered.
	// The bug (case-a swallow) produces kept=0 AND pass2=0 because case (a)
	// deletes the symbol from `remaining` with no rescue attempt.
	// The fix produces kept>0 OR pass2>0 because case (b) enqueues a candidate
	// and does NOT delete from `remaining`.
	if len(kept) == 0 && len(pass2Input) == 0 {
		t.Fatalf("approved-with-topup routed to case(a): METUSDT silently dropped — kept=0 pass2=0 means no rescue was attempted (transfer never scheduled)")
	}

	// Secondary: log which path the rescue took.
	for _, c := range kept {
		if c.symbol == "METUSDT" {
			t.Logf("METUSDT in kept — rescue succeeded (ideal path); kept=%d pass2=%d", len(kept), len(pass2Input))
			return
		}
	}
	for _, o := range pass2Input {
		if o.Symbol == "METUSDT" {
			t.Logf("METUSDT in pass2Input — rescue attempted, post-replay dropped (real margin constraints); case(b) routing verified; kept=%d pass2=%d", len(kept), len(pass2Input))
			return
		}
	}
	t.Errorf("METUSDT neither in kept nor pass2Input despite kept+pass2>0 — unexpected state; kept=%d pass2=%d", len(kept), len(pass2Input))
}

// TestPass1_PlainApprovedStaysCaseA verifies approved WITHOUT top-up still goes
// case (a): no rescue candidate enqueued, symbol removed from remaining (pass2=0),
// and margin IS reserved on both legs.
func TestPass1_PlainApprovedStaysCaseA(t *testing.T) {
	cfg := rankFirstCfg()
	exchanges := map[string]exchange.Exchange{
		"binance": richStub(),
		"okx":     richStub(),
	}
	e := buildRankFirstEngine(t, exchanges, cfg)

	// Inject approved approval with NO TopUpApplied — real balance sufficient.
	e.testSimulatorFn = func(
		opp models.Opportunity,
		longExch, shortExch string,
		reserved map[string]float64,
		alt *models.AlternativePair,
		cache *risk.PrefetchCache,
	) (*models.RiskApproval, error) {
		return &models.RiskApproval{
			Approved:          true,
			Size:              1000,
			Price:             0.5,
			RequiredMargin:    150,
			LongMarginNeeded:  150,
			ShortMarginNeeded: 150,
			// TopUpApplied intentionally nil.
		}, nil
	}

	opps := []models.Opportunity{
		goodOpp("ABCUSDT", "binance", "okx"),
	}

	balances := map[string]rebalanceBalanceInfo{}
	for name, exch := range exchanges {
		bi := rebalanceBalanceInfo{}
		if futBal, err := exch.GetFuturesBalance(); err == nil {
			bi.futures = futBal.Available
			bi.futuresTotal = futBal.Total
		}
		balances[name] = bi
	}

	kept, pass2Input, reserved := e.rebalancePass1(opps, nil, balances)

	// Case (a): no rescue candidate → kept=0.
	if len(kept) != 0 {
		t.Errorf("plain approved should NOT produce rescue candidate, got kept=%d", len(kept))
	}

	// Case (a) deletes symbol from `remaining` → NOT in pass2Input.
	for _, o := range pass2Input {
		if o.Symbol == "ABCUSDT" {
			t.Errorf("plain approved ABCUSDT must not appear in pass2Input (case-a removes from remaining)")
		}
	}

	// Case (a) DOES reserve margin on both legs.
	if reserved["binance"] <= 0 || reserved["okx"] <= 0 {
		t.Errorf("plain approved must reserve margin on both legs: reserved=%v", reserved)
	}
}

// TestPass1_TopUpReplayRejectsIfRealBalanceStillInsufficient verifies the
// post-transfer replay correctly refuses a top-up rescue when the simulated
// transfer would not land enough capital. Protects against the case where
// top-up amount > actual donor capacity.
//
// testSimulatorFn (initial Pass-1 call) → approved-with-topup → routes to case(b).
// testReplaySimulatorFn (replay call inside postTransferReplayFilter) → rejected →
// candidate dropped from kept.
func TestPass1_TopUpReplayRejectsIfRealBalanceStillInsufficient(t *testing.T) {
	cfg := rankFirstCfg()
	exchanges := map[string]exchange.Exchange{
		"gateio": richStub(),
		"bitget": richStub(),
	}
	e := buildRankFirstEngine(t, exchanges, cfg)

	// Initial Pass-1 call: approved with top-up applied (routes to case b).
	e.testSimulatorFn = func(
		opp models.Opportunity,
		longExch, shortExch string,
		reserved map[string]float64,
		alt *models.AlternativePair,
		cache *risk.PrefetchCache,
	) (*models.RiskApproval, error) {
		return &models.RiskApproval{
			Approved:          true,
			Size:              1596,
			Price:             0.188,
			RequiredMargin:    200,
			LongMarginNeeded:  199.93,
			ShortMarginNeeded: 199.71,
			TopUpApplied: map[string]float64{
				"gateio": 150,
				"bitget": 150,
			},
		}, nil
	}

	// Replay call (postTransferReplayFilter): transfer only moves 50 per leg —
	// insufficient for the real margin need. Simulator rejects.
	e.testReplaySimulatorFn = func(
		opp models.Opportunity,
		longExch, shortExch string,
		reserved map[string]float64,
		alt *models.AlternativePair,
		cache *risk.PrefetchCache,
	) (*models.RiskApproval, error) {
		return &models.RiskApproval{
			Approved: false,
			Kind:     models.RejectionKindCapital,
			Reason:   "post-trade margin ratio would reach 0.91",
		}, nil
	}

	opps := []models.Opportunity{
		goodOpp("METUSDT", "gateio", "bitget"),
	}

	balances := map[string]rebalanceBalanceInfo{}
	for name, exch := range exchanges {
		bi := rebalanceBalanceInfo{}
		if futBal, err := exch.GetFuturesBalance(); err == nil {
			bi.futures = futBal.Available
			bi.futuresTotal = futBal.Total
		}
		balances[name] = bi
	}

	kept, _, _ := e.rebalancePass1(opps, nil, balances)

	// Replay rejection must drop the candidate: kept must be empty.
	if len(kept) != 0 {
		t.Errorf("replay should have dropped top-up rescue when transfer insufficient, kept=%d", len(kept))
	}
}
