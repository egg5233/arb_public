// Package engine — tests for rebalance_argmax.go (PLAN-enumerate-argmax-rebalance v8 §9b).
//
// Covers:
//   - pure helpers (hasDuplicateSymbols, enumerateCombinations,
//     argmaxNeedsFromChoices, violatesDonorFloorFromSteps)
//   - buildArgmaxCandidates admits capital-rescue candidates and drops
//     genuinely unpriceable ones as capital_unpriced
//   - end-to-end smoke: rebalanceFundsArgmax publishes overrides when argmax
//     finds a feasible subset, and does not leak stale overrides when it does
//     not.
//
// Follows the Option A fixture pattern used by engine_rank_first_test.go:
// real risk.Manager via risk.NewManager + miniredis + stub exchanges.
package engine

import (
	"sort"
	"testing"

	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/exchange"
)

// ---------------------------------------------------------------------------
// Pure helpers
// ---------------------------------------------------------------------------

func TestHasDuplicateSymbols(t *testing.T) {
	tests := []struct {
		name    string
		choices []allocatorChoice
		want    bool
	}{
		{"empty", nil, false},
		{"single", []allocatorChoice{{symbol: "A"}}, false},
		{"two_distinct", []allocatorChoice{{symbol: "A"}, {symbol: "B"}}, false},
		{"duplicate", []allocatorChoice{{symbol: "A"}, {symbol: "B"}, {symbol: "A"}}, true},
		{"duplicate_via_alt", []allocatorChoice{{symbol: "A", longExchange: "x"}, {symbol: "A", longExchange: "y"}}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasDuplicateSymbols(tc.choices); got != tc.want {
				t.Errorf("hasDuplicateSymbols(%+v) = %v, want %v", tc.choices, got, tc.want)
			}
		})
	}
}

func TestEnumerateCombinations(t *testing.T) {
	items := []allocatorChoice{
		{symbol: "A"}, {symbol: "B"}, {symbol: "C"}, {symbol: "D"},
	}

	tests := []struct {
		name string
		k    int
		want [][]string
	}{
		{"k_zero_noop", 0, nil},
		{"k_larger_than_n", 5, nil},
		{"k_1", 1, [][]string{{"A"}, {"B"}, {"C"}, {"D"}}},
		{"k_2", 2, [][]string{{"A", "B"}, {"A", "C"}, {"A", "D"}, {"B", "C"}, {"B", "D"}, {"C", "D"}}},
		{"k_3", 3, [][]string{{"A", "B", "C"}, {"A", "B", "D"}, {"A", "C", "D"}, {"B", "C", "D"}}},
		{"k_n", 4, [][]string{{"A", "B", "C", "D"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got [][]string
			enumerateCombinations(items, tc.k, func(combo []allocatorChoice) bool {
				syms := make([]string, len(combo))
				for i, c := range combo {
					syms[i] = c.symbol
				}
				got = append(got, syms)
				return true
			})
			if len(got) != len(tc.want) {
				t.Fatalf("got %d combos, want %d: got=%v want=%v", len(got), len(tc.want), got, tc.want)
			}
			for i := range got {
				if len(got[i]) != len(tc.want[i]) {
					t.Errorf("combo %d len mismatch", i)
					continue
				}
				for j := range got[i] {
					if got[i][j] != tc.want[i][j] {
						t.Errorf("combo %d: got %v want %v", i, got[i], tc.want[i])
						break
					}
				}
			}
		})
	}
}

// Stopping iteration via visit=false should halt enumeration — used for the
// deadline break in rebalanceFundsArgmax.
func TestEnumerateCombinations_EarlyStop(t *testing.T) {
	items := []allocatorChoice{{symbol: "A"}, {symbol: "B"}, {symbol: "C"}}
	count := 0
	enumerateCombinations(items, 2, func(combo []allocatorChoice) bool {
		count++
		return count < 2 // stop after the 2nd combo
	})
	if count != 2 {
		t.Errorf("expected 2 combos visited before stop, got %d", count)
	}
}

func TestArgmaxNeedsFromChoices(t *testing.T) {
	choices := []allocatorChoice{
		{longExchange: "bitget", shortExchange: "bingx", requiredMargin: 147},
		{longExchange: "bitget", shortExchange: "okx", requiredMargin: 50},
		{longExchange: "binance", shortExchange: "bingx", requiredMargin: 100},
	}
	got := argmaxNeedsFromChoices(choices)
	want := map[string]float64{
		"bitget":  147 + 50,
		"bingx":   147 + 100,
		"okx":     50,
		"binance": 100,
	}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %d want %d (got=%v)", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("needs[%s] = %v, want %v", k, got[k], v)
		}
	}
}

func TestSymbolsOf(t *testing.T) {
	choices := []allocatorChoice{
		{symbol: "PIEVERSEUSDT"}, {symbol: "ETHUSDT"}, {symbol: "BTCUSDT"},
	}
	got := symbolsOf(choices)
	want := []string{"PIEVERSEUSDT", "ETHUSDT", "BTCUSDT"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("symbolsOf[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFormatArgmaxSteps(t *testing.T) {
	// Empty steps should render as "[]" so log lines stay compact.
	if got := formatArgmaxSteps(nil); got != "[]" {
		t.Errorf("empty: got %q, want [] ", got)
	}
	steps := []transferStep{
		{From: "bitget", To: "bingx", Amount: 147.0, Fee: 0.5, Chain: "APT"},
		{From: "okx", To: "binance", Amount: 50.0},
	}
	got := formatArgmaxSteps(steps)
	// Assertion is substring-based so format tweaks don't break the test.
	for _, want := range []string{"bitget->bingx:147.00", "fee 0.50", "chain APT", "okx->binance:50.00"} {
		if !contains(got, want) {
			t.Errorf("formatArgmaxSteps %q missing substring %q", got, want)
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// violatesDonorFloorFromSteps — needs real Engine for allocatorDonorGrossCapacity,
// so drive via a minimal engine fixture.
// ---------------------------------------------------------------------------

func TestViolatesDonorFloorFromSteps_Disabled(t *testing.T) {
	cfg := rankFirstCfg()
	e := buildRankFirstEngine(t, map[string]exchange.Exchange{}, cfg)
	// floorPct=0 short-circuits before any computation.
	if e.violatesDonorFloorFromSteps(
		[]transferStep{{From: "bitget", To: "bingx", Amount: 1000}},
		map[string]rebalanceBalanceInfo{"bitget": {futures: 100, futuresTotal: 100, maxTransferOut: 100}},
		[]allocatorChoice{{longExchange: "bitget", shortExchange: "bingx", requiredMargin: 50}},
		0,
	) {
		t.Error("floorPct=0 must not report violation")
	}
}

func TestViolatesDonorFloorFromSteps_NoSteps(t *testing.T) {
	cfg := rankFirstCfg()
	e := buildRankFirstEngine(t, map[string]exchange.Exchange{}, cfg)
	if e.violatesDonorFloorFromSteps(nil, nil, nil, 0.05) {
		t.Error("empty steps must not report violation")
	}
}

// ---------------------------------------------------------------------------
// buildArgmaxCandidates — admits capital-rescue candidates (Size/Price set)
// and drops genuinely unpriceable cases (which don't occur in production
// because pricedCapitalRejection always fills Size/Price, but we simulate via
// a custom stub that forces size=0).
// ---------------------------------------------------------------------------

// argmaxZeroSizeStub returns an orderbook whose mid price is so high that the
// computed size rounds to zero. This triggers the "insufficient capital for
// minimum position size" branch at manager.go:466, which returns a plain
// RejectionKindCapital WITHOUT size/price/margin. buildArgmaxCandidates must
// drop it as capital_unpriced.
type argmaxZeroSizeStub struct {
	*rankFirstStub
}

func (s *argmaxZeroSizeStub) GetOrderbook(sym string, depth int) (*exchange.Orderbook, error) {
	// Absurd price so size = capPerLeg*lev / price underflows for the tiny
	// capital we feed in.
	return &exchange.Orderbook{
		Asks: []exchange.PriceLevel{{Price: 1e12, Quantity: 1}},
		Bids: []exchange.PriceLevel{{Price: 1e12 - 1, Quantity: 1}},
	}, nil
}

func TestBuildArgmaxCandidates_AdmitsCapitalRescue(t *testing.T) {
	cfg := rankFirstCfg()
	cfg.EnableArgmaxRebalance = true
	// Default L4 (0.80). Scenario is calibrated so the first-pass approval
	// post-trade-ratio-rejects as Capital, but the projected post-transfer
	// balance passes all gates in the replay — which is the realistic rescue
	// window (tighter L4 values cannot be rescued by any reasonable transfer,
	// and the replay correctly drops those candidates).

	exchanges := map[string]exchange.Exchange{
		// binance long leg: available=40 / total=100. With CapitalPerLeg=100,
		// lev=2, MSM=2:
		//   size = min(100*2, 40*2/2)/price = min(200, 40)/2 = 20 (price=2.0)
		//   longMarginPerLeg = 20*2/2 = 20; buffer = 20*2 = 40 == avail → passes
		//   post-trade: avail-margin=20, total=100 → ratio = 0.80 → fails L4.
		// Replay bumps +buffer=40 → avail=80, total=140:
		//   size = min(200, 80)/2 = 40; margin = 40; buffer = 80 → passes.
		//   post-trade: 80-40=40, total=140 → ratio = 0.714 < 0.80 → approves.
		"binance": newRankFirstStub(40, 100, 0),
		"bybit":   richStub(), // short leg, healthy
		"gateio":  richStub(), // donor for cache top-up
	}
	e := buildRankFirstEngine(t, exchanges, cfg)

	opp := goodOpp("RESCUEUSDT", "binance", "bybit")
	balances, ok := e.snapshotArgmaxBalances(nil)
	if !ok {
		t.Fatal("snapshotArgmaxBalances returned ok=false")
	}

	cands, rejects := e.buildArgmaxCandidates([]models.Opportunity{opp}, nil, balances)
	if len(cands) == 0 {
		t.Fatalf("expected ≥1 capital-rescue candidate, got 0 (rejects=%v)", rejects)
	}
	var found bool
	for _, c := range cands {
		if c.symbol == "RESCUEUSDT" && c.longExchange == "binance" && c.shortExchange == "bybit" {
			if c.requiredMargin <= 0 {
				t.Errorf("rescue candidate has zero requiredMargin: %+v", c)
			}
			if c.entryNotional <= 0 {
				t.Errorf("rescue candidate has zero entryNotional: %+v", c)
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected RESCUEUSDT binance>bybit in candidates, got %+v", cands)
	}
}

// ---------------------------------------------------------------------------
// rebalanceFundsArgmax end-to-end smoke: capital-rescue scenario publishes
// overrides; early-return path clears stale overrides.
// ---------------------------------------------------------------------------

func TestRebalanceFundsArgmax_NoOpps_ClearsStaleOverrides(t *testing.T) {
	cfg := rankFirstCfg()
	cfg.EnableArgmaxRebalance = true

	exchanges := map[string]exchange.Exchange{
		"binance": richStub(),
		"bybit":   richStub(),
	}
	e := buildRankFirstEngine(t, exchanges, cfg)

	// Plant a stale override so we can verify the wrapper clears it before
	// argmax dispatch, per plan §2b.
	e.allocOverrides = map[string]allocatorChoice{
		"STALEUSDT": {symbol: "STALEUSDT", longExchange: "binance", shortExchange: "bybit"},
	}

	e.rebalanceFunds([]models.Opportunity{}) // empty opps → argmax early-returns

	e.allocOverrideMu.Lock()
	defer e.allocOverrideMu.Unlock()
	if e.allocOverrides != nil && len(e.allocOverrides) > 0 {
		t.Errorf("stale override leaked into next cycle: %+v", e.allocOverrides)
	}
}

func TestRebalanceFundsArgmax_CapacityAtMax_EarlyReturn(t *testing.T) {
	cfg := rankFirstCfg()
	cfg.EnableArgmaxRebalance = true
	cfg.MaxPositions = 0 // no slots → argmax returns before enumeration

	exchanges := map[string]exchange.Exchange{
		"binance": richStub(),
		"bybit":   richStub(),
	}
	e := buildRankFirstEngine(t, exchanges, cfg)

	e.allocOverrides = map[string]allocatorChoice{
		"STALEUSDT": {symbol: "STALEUSDT"},
	}

	e.rebalanceFunds([]models.Opportunity{goodOpp("FOOUSDT", "binance", "bybit")})

	e.allocOverrideMu.Lock()
	defer e.allocOverrideMu.Unlock()
	if e.allocOverrides != nil && len(e.allocOverrides) > 0 {
		t.Errorf("overrides should be cleared on capacity early-return: %+v", e.allocOverrides)
	}
}

// End-to-end happy path: both opps fully approved, no transfers needed. Argmax
// enumeration must still publish kept overrides and call through to execute.
func TestRebalanceFundsArgmax_PicksAndPublishesOverrides(t *testing.T) {
	cfg := rankFirstCfg()
	cfg.EnableArgmaxRebalance = true
	cfg.RebalanceMinNetPnLUSDT = 0                 // accept any non-negative feasible subset
	cfg.RebalanceDonorFloorPct = 0                 // disable donor floor for smoke test
	cfg.AllocatorTimeoutMs = 10000
	cfg.MinHoldTime = 16 * 60 * 60 * 1000000000    // 16h in ns so computeAllocatorBaseValue returns a real positive number

	// Balanced 10k/10k keeps every exchange below the target usage so
	// dryRunTransferPlan does not demand pre-existing ratio relief.
	veryRich := func() *rankFirstStub { return newRankFirstStub(10000, 10000, 0) }
	exchanges := map[string]exchange.Exchange{
		// rank-1: both legs rich → approved directly
		"bingx":  veryRich(),
		"bybit":  veryRich(),
		// rank-2 donor/recipients
		"gateio": veryRich(),
		"okx":    veryRich(),
	}
	e := buildRankFirstEngine(t, exchanges, cfg)

	opps := []models.Opportunity{
		goodOpp("APPROVEDUSDT", "bingx", "bybit"),
		goodOpp("SECONDUSDT", "gateio", "okx"),
	}
	e.rebalanceFunds(opps)

	e.allocOverrideMu.Lock()
	defer e.allocOverrideMu.Unlock()
	if len(e.allocOverrides) == 0 {
		t.Fatal("expected argmax to publish ≥1 override, got 0")
	}
	// The published symbols should be a subset of our opps.
	valid := map[string]bool{"APPROVEDUSDT": true, "SECONDUSDT": true}
	var picked []string
	for sym := range e.allocOverrides {
		if !valid[sym] {
			t.Errorf("unexpected symbol in overrides: %q", sym)
		}
		picked = append(picked, sym)
	}
	sort.Strings(picked)
	t.Logf("argmax published overrides for: %v", picked)
}

// ---------------------------------------------------------------------------
// Capital-rescue replay (§9b core assertion): the legacy tail-prune path would
// have dropped this opportunity before enumeration; argmax must admit it and
// plan a donor transfer.
//
// Scenario mirrors 2026-04-20 15:45:
//   - "bingx" has tiny futures balance → primary-pair Capital-rejected on the
//     short leg via post-trade margin ratio projection
//   - "bitget" has ample balance → valid donor with cross-chain address
//   - Argmax must: (a) admit the rescue candidate, (b) plan a
//     bitget→bingx transfer in best.dryRun.Steps, (c) publish an override.
// ---------------------------------------------------------------------------

// withdrawFeeStub extends rankFirstStub with a non-zero GetWithdrawFee so
// dryRunTransferPlan's fee lookup succeeds (default stub returns zeros which
// would also succeed, but we use realistic values here to keep the test
// representative).
type withdrawFeeStub struct {
	*rankFirstStub
}

func (s *withdrawFeeStub) GetWithdrawFee(coin, chain string) (float64, float64, error) {
	return 0.5, 1.0, nil
}

func TestRebalanceFundsArgmax_CapitalRescueReplay(t *testing.T) {
	cfg := rankFirstCfg()
	cfg.EnableArgmaxRebalance = true
	cfg.RebalanceMinNetPnLUSDT = 0
	cfg.RebalanceDonorFloorPct = 0
	cfg.AllocatorTimeoutMs = 10000
	cfg.MinHoldTime = 16 * 60 * 60 * 1000000000 // 16h
	cfg.CapitalPerLeg = 100
	cfg.Leverage = 2
	cfg.MarginSafetyMultiplier = 2.0
	// Default L4=0.80. Scenario calibrated so first-pass Capital-rejects but
	// projected post-transfer (= first-pass requiredMargin of 40 USDT added to
	// recipient) passes the replay approval. See TestBuildArgmaxCandidates_
	// AdmitsCapitalRescue for the full arithmetic.

	// Donor: "bitget" with ample balance + cross-chain address so dryRun
	// can plan a transfer to the recipient.
	bitgetDonor := &withdrawFeeStub{newRankFirstStub(10000, 10000, 0)}
	// Recipient: "bingx" short leg with a low balance (40/100) that will
	// Capital-reject via post-trade-ratio on the first pass and pass on the
	// replay after a +40 USDT bump, so the rescue admission is exercised.
	bingxRecipient := &withdrawFeeStub{newRankFirstStub(40, 100, 0)}
	// Long leg: richly funded so it does not get rejected first.
	binanceLong := &withdrawFeeStub{newRankFirstStub(10000, 10000, 0)}

	exchanges := map[string]exchange.Exchange{
		"binance": binanceLong,
		"bingx":   bingxRecipient,
		"bitget":  bitgetDonor,
	}

	// Configure exchange addresses so dryRun can pick a chain for the
	// bitget→bingx transfer. Without this, dryRun returns "no chain" → infeasible.
	cfg.ExchangeAddresses = map[string]map[string]string{
		"bingx":  {"APT": "aptos_bingx_deposit_addr"},
		"bitget": {"APT": "aptos_bitget_deposit_addr"},
	}

	e := buildRankFirstEngine(t, exchanges, cfg)

	opps := []models.Opportunity{
		goodOpp("RESCUEUSDT", "binance", "bingx"), // short leg bingx is under-funded, bitget donates
	}

	// Assertion 1: buildArgmaxCandidates admits the candidate under the
	// argmax admission path. With rich donor balances present, cache top-up
	// typically lets the first pass approve directly — this is the same
	// admission path that legacy tail-prune would have allowed. The
	// first-pass-reject / replay-approve path specifically is covered by
	// TestReplayArgmaxApprovalAfterFunding_* below.
	balances, bOK := e.snapshotArgmaxBalances(nil)
	if !bOK {
		t.Fatal("snapshotArgmaxBalances failed")
	}
	cands, rejects := e.buildArgmaxCandidates(opps, nil, balances)
	if len(cands) == 0 {
		t.Fatalf("expected ≥1 argmax candidate (approved or rescue), got 0 (rejects=%v)", rejects)
	}
	if cands[0].symbol != "RESCUEUSDT" || cands[0].longExchange != "binance" || cands[0].shortExchange != "bingx" {
		t.Errorf("candidate[0] mismatch: got %+v, want symbol=RESCUEUSDT long=binance short=bingx", cands[0])
	}
	if cands[0].requiredMargin <= 0 || cands[0].entryNotional <= 0 {
		t.Errorf("candidate missing priced data: %+v", cands[0])
	}

	// Assertion 2 (the §9b key assertion): dryRunTransferPlan produces a
	// feasible plan that includes a bitget→bingx transfer step.
	// This is what legacy tail-prune would have excluded from consideration
	// before enumeration, and what argmax must discover.
	feeCache := map[string]feeEntry{}
	plan := e.dryRunTransferPlan(cands, balances, feeCache)
	if !plan.Feasible {
		t.Fatalf("expected feasible plan for rescue, got infeasible (steps=%v)", plan.Steps)
	}
	var sawBingxTransfer bool
	for _, s := range plan.Steps {
		t.Logf("planned step: %s → %s amount=%.2f fee=%.2f chain=%s", s.From, s.To, s.Amount, s.Fee, s.Chain)
		if s.To == "bingx" && s.From == "bitget" && s.Amount > 0 {
			sawBingxTransfer = true
		}
	}
	if !sawBingxTransfer {
		t.Errorf("expected a bitget→bingx transfer with Amount>0 in dryRun.Steps; got %+v", plan.Steps)
	}

	// Assertion 3: end-to-end argmax invocation runs to completion without
	// panicking and emits the [argmax] pick log line. Stub exchanges cannot
	// simulate a real post-withdraw deposit landing, so keepFundedChoices
	// will drop the override — that is a fixture limitation, not a bug.
	// The override-publish happy path is covered by
	// TestRebalanceFundsArgmax_PicksAndPublishesOverrides above.
	e.rebalanceFunds(opps)
}

// ---------------------------------------------------------------------------
// replayArgmaxApprovalAfterFunding — targeted tests that prove the first-pass
// Capital-rejection → replay-approval path actually fires. These bypass
// buildArgmaxCandidates (and its cache top-up) so the replay path is the ONLY
// thing being exercised. See plan §9b and the independent-review follow-up.
// ---------------------------------------------------------------------------

// makeRejectedCapitalApproval constructs a RiskApproval matching what
// pricedCapitalRejection would emit for a size=20, price=2, MSM=2 long-leg
// post-trade-ratio rejection at CapitalPerLeg=100, Leverage=2.
func makeRejectedCapitalApproval() *models.RiskApproval {
	return &models.RiskApproval{
		Approved:          false,
		Kind:              models.RejectionKindCapital,
		Reason:            "simulated first-pass capital rejection (post-trade ratio)",
		Size:              20,
		Price:             2.0,
		RequiredMargin:    40,
		LongMarginNeeded:  40,
		ShortMarginNeeded: 40,
	}
}

func TestReplayArgmaxApprovalAfterFunding_PromotesRescue(t *testing.T) {
	cfg := rankFirstCfg()
	// Default L4=0.80: after +40 bump, recipient a goes to 80/140, long-leg
	// post-trade ratio = 1 - (80-40)/140 = 0.714 < 0.80 → approves.
	exchanges := map[string]exchange.Exchange{
		// Recipient long leg: 40/100 — under the artificially-low balance that
		// caused the simulated first-pass rejection.
		"a": newRankFirstStub(40, 100, 0),
		// Short leg: rich enough that its own approval gates pass.
		"b": newRankFirstStub(10000, 10000, 0),
	}
	e := buildRankFirstEngine(t, exchanges, cfg)

	balances := map[string]rebalanceBalanceInfo{
		"a": {futures: 40, spot: 0, futuresTotal: 100},
		"b": {futures: 10000, spot: 0, futuresTotal: 10000},
	}
	// Empty orig cache (no TransferablePerExchange top-up) — we are testing
	// the replay path, not the cache rescue path.
	origCache := &risk.PrefetchCache{}
	opp := goodOpp("PROMOTEUSDT", "a", "b")
	rejected := makeRejectedCapitalApproval()

	result := e.replayArgmaxApprovalAfterFunding(opp, "a", "b", nil, balances, origCache, rejected)
	if result == nil {
		t.Fatal("expected replay to APPROVE with projected post-transfer balance, got nil")
	}
	if !result.Approved {
		t.Fatalf("expected Approved=true, got Approved=%v reason=%s", result.Approved, result.Reason)
	}
	// Post-transfer sizing should be at least as large as the rejected size
	// (the replay sees more available balance, so it may size up to the
	// CapitalPerLeg cap rather than the balance-bounded 20).
	if result.Size < rejected.Size {
		t.Errorf("expected post-transfer size >= rejected size %.1f, got %.1f", rejected.Size, result.Size)
	}
	if result.RequiredMargin <= 0 || result.Price <= 0 {
		t.Errorf("replay-approved candidate missing priced data: size=%.1f price=%.2f required=%.2f",
			result.Size, result.Price, result.RequiredMargin)
	}
	t.Logf("replay-approved: size=%.1f price=%.2f required=%.2f long=%.2f short=%.2f",
		result.Size, result.Price, result.RequiredMargin,
		result.LongMarginNeeded, result.ShortMarginNeeded)
}

func TestReplayArgmaxApprovalAfterFunding_RejectsWhenStillInfeasible(t *testing.T) {
	cfg := rankFirstCfg()
	// Artificially tight L4 so even after the +40 bump the post-trade-ratio
	// check still fails — the rescue is genuinely infeasible and should be
	// dropped. This is the safety net that prevents argmax from executing
	// donor transfers for pairs that would still reject at entry time.
	cfg.MarginL4Threshold = 0.05

	exchanges := map[string]exchange.Exchange{
		"a": newRankFirstStub(40, 100, 0),
		"b": newRankFirstStub(10000, 10000, 0),
	}
	e := buildRankFirstEngine(t, exchanges, cfg)

	balances := map[string]rebalanceBalanceInfo{
		"a": {futures: 40, spot: 0, futuresTotal: 100},
		"b": {futures: 10000, spot: 0, futuresTotal: 10000},
	}
	origCache := &risk.PrefetchCache{}
	opp := goodOpp("STILLREJECTEDUSDT", "a", "b")
	rejected := makeRejectedCapitalApproval()

	result := e.replayArgmaxApprovalAfterFunding(opp, "a", "b", nil, balances, origCache, rejected)
	if result != nil {
		t.Errorf("expected nil (rescue dropped) when replay still rejects, got %+v", result)
	}
}
