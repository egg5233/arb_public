package engine

import (
	"sort"
	"sync"
	"testing"
	"time"

	"arb/internal/config"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// allocator_parity_test.go — frozen-fixture parity tests guarding the
// selection_core.go refactor. Before the refactor, solveAllocator and
// greedyAllocatorSeed ran an inline branch-and-bound / greedy walk over
// allocatorChoice values. After the refactor, both go through
// selection_core.solveGroupedSearch / greedySeed. These tests pin the
// outputs on a deterministic fixture so any accidental behavior drift
// for the rebalance pathway trips them.
//
// The fixture is crafted so dryRunTransferPlan is trivially feasible
// with zero fees:
//   - every exchange has futures >> need, so no cross-exchange
//     transfer is triggered (Pass 2 deficit == 0, inner loop skipped)
//   - spot == 0 on every exchange, so Pass 1 spot->futures block is
//     skipped (!isUnified && sim[exch].spot > 0 is false)
//   - futuresTotal == 0, so ratio-based checks short-circuit feasible
//     (postTradeMarginRatio returns ok=true when futuresTotal<=0)
// Result: dryRunTransferPlan returns {Feasible: true, TotalFee: 0}
// regardless of which subset is evaluated, so the solver output is
// driven purely by candidate ordering and slot caps.

// newTestEngineForParity builds a minimal Engine wired for the fields
// the rebalance allocator reads during solveAllocator /
// greedyAllocatorSeed. exchanges is left empty so the unified-checker
// type assertion fails everywhere (all exchanges treated split).
func newTestEngineForParity(t *testing.T) *Engine {
	t.Helper()
	cfg := &config.Config{}
	cfg.MarginL4Threshold = 0.80
	cfg.MarginSafetyMultiplier = 2.0
	return &Engine{
		cfg:       cfg,
		log:       utils.NewLogger("parity"),
		exchanges: nil, // type assertion `IsUnified` will fail on nil — split path
	}
}

type parityExchangeStub struct {
	name    string
	fee     float64
	minWd   float64
	unified bool
	feeErr  error
}

func (s *parityExchangeStub) Name() string                                         { return s.name }
func (s *parityExchangeStub) SetMetricsCallback(exchange.MetricsCallback)          {}
func (s *parityExchangeStub) PlaceOrder(exchange.PlaceOrderParams) (string, error) { return "", nil }
func (s *parityExchangeStub) CancelOrder(string, string) error                     { return nil }
func (s *parityExchangeStub) GetPendingOrders(string) ([]exchange.Order, error)    { return nil, nil }
func (s *parityExchangeStub) GetOrderFilledQty(string, string) (float64, error)    { return 0, nil }
func (s *parityExchangeStub) GetPosition(string) ([]exchange.Position, error)      { return nil, nil }
func (s *parityExchangeStub) GetAllPositions() ([]exchange.Position, error)        { return nil, nil }
func (s *parityExchangeStub) SetLeverage(string, string, string) error             { return nil }
func (s *parityExchangeStub) SetMarginMode(string, string) error                   { return nil }
func (s *parityExchangeStub) LoadAllContracts() (map[string]exchange.ContractInfo, error) {
	return nil, nil
}
func (s *parityExchangeStub) GetFundingRate(string) (*exchange.FundingRate, error) { return nil, nil }
func (s *parityExchangeStub) GetFundingInterval(string) (time.Duration, error)     { return 0, nil }
func (s *parityExchangeStub) GetFuturesBalance() (*exchange.Balance, error) {
	return &exchange.Balance{}, nil
}
func (s *parityExchangeStub) GetSpotBalance() (*exchange.Balance, error) {
	return &exchange.Balance{}, nil
}
func (s *parityExchangeStub) Withdraw(exchange.WithdrawParams) (*exchange.WithdrawResult, error) {
	return nil, nil
}
func (s *parityExchangeStub) WithdrawFeeInclusive() bool { return false }
func (s *parityExchangeStub) GetWithdrawFee(string, string) (float64, float64, error) {
	return s.fee, s.minWd, s.feeErr
}
func (s *parityExchangeStub) TransferToSpot(string, string) error                   { return nil }
func (s *parityExchangeStub) TransferToFutures(string, string) error                { return nil }
func (s *parityExchangeStub) GetOrderbook(string, int) (*exchange.Orderbook, error) { return nil, nil }
func (s *parityExchangeStub) StartPriceStream([]string)                             {}
func (s *parityExchangeStub) SubscribeSymbol(string) bool                           { return false }
func (s *parityExchangeStub) GetBBO(string) (exchange.BBO, bool)                    { return exchange.BBO{}, false }
func (s *parityExchangeStub) GetPriceStore() *sync.Map                              { return nil }
func (s *parityExchangeStub) SubscribeDepth(string) bool                            { return false }
func (s *parityExchangeStub) UnsubscribeDepth(string) bool                          { return false }
func (s *parityExchangeStub) GetDepth(string) (*exchange.Orderbook, bool)           { return nil, false }
func (s *parityExchangeStub) StartPrivateStream()                                   {}
func (s *parityExchangeStub) GetOrderUpdate(string) (exchange.OrderUpdate, bool) {
	return exchange.OrderUpdate{}, false
}
func (s *parityExchangeStub) SetOrderCallback(func(exchange.OrderUpdate))           {}
func (s *parityExchangeStub) PlaceStopLoss(exchange.StopLossParams) (string, error) { return "", nil }
func (s *parityExchangeStub) CancelStopLoss(string, string) error                   { return nil }
func (s *parityExchangeStub) PlaceTakeProfit(exchange.TakeProfitParams) (string, error) {
	return "", nil
}
func (s *parityExchangeStub) CancelTakeProfit(string, string) error { return nil }
func (s *parityExchangeStub) GetUserTrades(string, time.Time, int) ([]exchange.Trade, error) {
	return nil, nil
}
func (s *parityExchangeStub) GetFundingFees(string, time.Time) ([]exchange.FundingPayment, error) {
	return nil, nil
}
func (s *parityExchangeStub) GetClosePnL(string, time.Time) ([]exchange.ClosePnL, error) {
	return nil, nil
}
func (s *parityExchangeStub) EnsureOneWayMode() error      { return nil }
func (s *parityExchangeStub) CancelAllOrders(string) error { return nil }
func (s *parityExchangeStub) Close()                       {}
func (s *parityExchangeStub) IsUnified() bool              { return s.unified }

// parityFixture returns a deterministic 3-candidate allocator input.
// Each candidate has a single choice with a distinct base value and a
// unique long/short pair so nothing collides during map flattening.
func parityFixture() (candidates []allocatorCandidate, capacity map[string]float64, balances map[string]rebalanceBalanceInfo) {
	candidates = []allocatorCandidate{
		{
			symbol: "BTCUSDT",
			choices: []allocatorChoice{{
				symbol:         "BTCUSDT",
				longExchange:   "binance",
				shortExchange:  "bybit",
				requiredMargin: 50,
				baseValue:      100,
			}},
		},
		{
			symbol: "ETHUSDT",
			choices: []allocatorChoice{{
				symbol:         "ETHUSDT",
				longExchange:   "okx",
				shortExchange:  "gateio",
				requiredMargin: 50,
				baseValue:      80,
			}},
		},
		{
			symbol: "SOLUSDT",
			choices: []allocatorChoice{{
				symbol:         "SOLUSDT",
				longExchange:   "bitget",
				shortExchange:  "bingx",
				requiredMargin: 50,
				baseValue:      50,
			}},
		},
	}
	// Capacity is unused by the refactored solver (feasibility via
	// dryRunTransferPlan) but kept populated for signature compatibility.
	capacity = map[string]float64{
		"binance": 10000, "bybit": 10000,
		"okx": 10000, "gateio": 10000,
		"bitget": 10000, "bingx": 10000,
	}
	balances = map[string]rebalanceBalanceInfo{
		"binance": {futures: 10000, spot: 0, futuresTotal: 0},
		"bybit":   {futures: 10000, spot: 0, futuresTotal: 0},
		"okx":     {futures: 10000, spot: 0, futuresTotal: 0},
		"gateio":  {futures: 10000, spot: 0, futuresTotal: 0},
		"bitget":  {futures: 10000, spot: 0, futuresTotal: 0},
		"bingx":   {futures: 10000, spot: 0, futuresTotal: 0},
	}
	return candidates, capacity, balances
}

// symbolSet collects selected symbols for order-insensitive comparison.
// Post-refactor ordering inside the solver is deterministic by group
// key + value sort, but parity is the caller-visible invariant "same
// input -> same chosen set" — so we assert on sorted symbols, not slice
// order.
func symbolSet(choices []allocatorChoice) []string {
	out := make([]string, 0, len(choices))
	for _, c := range choices {
		out = append(out, c.symbol)
	}
	sort.Strings(out)
	return out
}

// TestRunPoolAllocator_AfterRefactorSameOutputAsBaseline pins
// solveAllocator's output for the 3-candidate fixture under slot cap 2.
// Baseline (pre-refactor) observed picks: BTCUSDT (value 100) +
// ETHUSDT (value 80) = total value 180. The lowest-value candidate
// SOLUSDT is dropped.
func TestRunPoolAllocator_AfterRefactorSameOutputAsBaseline(t *testing.T) {
	e := newTestEngineForParity(t)
	candidates, capacity, balances := parityFixture()
	feeCache := map[string]feeEntry{}

	got := e.solveAllocator(candidates, capacity, balances, 2, 50*time.Millisecond, feeCache)

	if len(got) != 2 {
		t.Fatalf("want 2 choices, got %d: %+v", len(got), got)
	}
	gotSyms := symbolSet(got)
	wantSyms := []string{"BTCUSDT", "ETHUSDT"}
	for i := range wantSyms {
		if gotSyms[i] != wantSyms[i] {
			t.Fatalf("want symbols %v, got %v", wantSyms, gotSyms)
		}
	}
	// Parity also demands each chosen pair matches the fixture mapping.
	for _, c := range got {
		switch c.symbol {
		case "BTCUSDT":
			if c.longExchange != "binance" || c.shortExchange != "bybit" {
				t.Fatalf("BTCUSDT pair mismatch: got %s/%s", c.longExchange, c.shortExchange)
			}
		case "ETHUSDT":
			if c.longExchange != "okx" || c.shortExchange != "gateio" {
				t.Fatalf("ETHUSDT pair mismatch: got %s/%s", c.longExchange, c.shortExchange)
			}
		default:
			t.Fatalf("unexpected pick %q", c.symbol)
		}
	}
}

// TestGreedyAllocatorSeed_AfterRefactorSameOutput pins
// greedyAllocatorSeed's output for the 3-candidate fixture under slot
// cap 2. Baseline greedy picks top-value candidate per group in
// candidate order, stopping at the slot cap: BTCUSDT + ETHUSDT.
func TestGreedyAllocatorSeed_AfterRefactorSameOutput(t *testing.T) {
	e := newTestEngineForParity(t)
	candidates, capacity, balances := parityFixture()
	feeCache := map[string]feeEntry{}

	got := e.greedyAllocatorSeed(candidates, capacity, balances, feeCache, 2)

	if len(got) != 2 {
		t.Fatalf("want 2 choices, got %d: %+v", len(got), got)
	}
	gotSyms := symbolSet(got)
	wantSyms := []string{"BTCUSDT", "ETHUSDT"}
	for i := range wantSyms {
		if gotSyms[i] != wantSyms[i] {
			t.Fatalf("want symbols %v, got %v", wantSyms, gotSyms)
		}
	}
	for _, c := range got {
		switch c.symbol {
		case "BTCUSDT":
			if c.longExchange != "binance" || c.shortExchange != "bybit" {
				t.Fatalf("BTCUSDT pair mismatch: got %s/%s", c.longExchange, c.shortExchange)
			}
		case "ETHUSDT":
			if c.longExchange != "okx" || c.shortExchange != "gateio" {
				t.Fatalf("ETHUSDT pair mismatch: got %s/%s", c.longExchange, c.shortExchange)
			}
		}
	}
}

// TestRunPoolAllocator_AfterRefactorSameOutputWithAlternatives verifies
// mutual exclusion survives the refactor when a single symbol carries
// multiple pair alternatives. With two choices for BTCUSDT and a
// single-choice ETHUSDT under slot cap 2, both the pre- and
// post-refactor solver must pick BTCUSDT's top-value alt (binance/bybit,
// 120) plus ETHUSDT (80), never both BTCUSDT alts.
func TestRunPoolAllocator_AfterRefactorSameOutputWithAlternatives(t *testing.T) {
	e := newTestEngineForParity(t)
	candidates := []allocatorCandidate{
		{
			symbol: "BTCUSDT",
			choices: []allocatorChoice{
				// Pre-sorted by baseValue desc to match buildAllocatorCandidates.
				{
					symbol:         "BTCUSDT",
					longExchange:   "binance",
					shortExchange:  "bybit",
					requiredMargin: 50,
					baseValue:      120,
				},
				{
					symbol:         "BTCUSDT",
					longExchange:   "bitget",
					shortExchange:  "okx",
					requiredMargin: 50,
					baseValue:      90,
				},
			},
		},
		{
			symbol: "ETHUSDT",
			choices: []allocatorChoice{{
				symbol:         "ETHUSDT",
				longExchange:   "okx",
				shortExchange:  "gateio",
				requiredMargin: 50,
				baseValue:      80,
			}},
		},
	}
	_, capacity, balances := parityFixture()
	feeCache := map[string]feeEntry{}

	got := e.solveAllocator(candidates, capacity, balances, 2, 50*time.Millisecond, feeCache)

	if len(got) != 2 {
		t.Fatalf("want 2 choices (BTCUSDT best alt + ETHUSDT), got %d: %+v", len(got), got)
	}
	seen := map[string]int{}
	for _, c := range got {
		seen[c.symbol]++
	}
	if seen["BTCUSDT"] != 1 {
		t.Fatalf("mutual exclusion broken: BTCUSDT appears %d times", seen["BTCUSDT"])
	}
	if seen["ETHUSDT"] != 1 {
		t.Fatalf("ETHUSDT missing: appears %d times", seen["ETHUSDT"])
	}
	for _, c := range got {
		if c.symbol == "BTCUSDT" && (c.longExchange != "binance" || c.shortExchange != "bybit") {
			t.Fatalf("BTCUSDT should prefer binance/bybit (best value), got %s/%s",
				c.longExchange, c.shortExchange)
		}
	}
}

func TestAllocator_FrozenFixtureNonZeroTransferFees(t *testing.T) {
	e := newTestEngineForParity(t)
	e.cfg.ExchangeAddresses = map[string]map[string]string{
		"bybit": {"BEP20": "bybit-usdt-address"},
	}
	e.exchanges = map[string]exchange.Exchange{
		"binance": &parityExchangeStub{name: "binance", fee: 1.25, minWd: 5},
		"bybit":   &parityExchangeStub{name: "bybit"},
		"okx":     &parityExchangeStub{name: "okx"},
		"gateio":  &parityExchangeStub{name: "gateio"},
	}

	candidates := []allocatorCandidate{
		{
			symbol: "BTCUSDT",
			choices: []allocatorChoice{{
				symbol:         "BTCUSDT",
				longExchange:   "bybit",
				shortExchange:  "okx",
				requiredMargin: 60,
				baseValue:      120,
			}},
		},
		{
			symbol: "ETHUSDT",
			choices: []allocatorChoice{{
				symbol:         "ETHUSDT",
				longExchange:   "okx",
				shortExchange:  "gateio",
				requiredMargin: 40,
				baseValue:      80,
			}},
		},
	}
	capacity := map[string]float64{
		"binance": 250,
		"bybit":   10,
		"okx":     100,
		"gateio":  100,
	}
	balances := map[string]rebalanceBalanceInfo{
		"binance": {futures: 200, spot: 50, futuresTotal: 0},
		"bybit":   {futures: 10, spot: 0, futuresTotal: 0},
		"okx":     {futures: 100, spot: 0, futuresTotal: 0},
		"gateio":  {futures: 100, spot: 0, futuresTotal: 0},
	}
	feeCache := map[string]feeEntry{}

	choices := e.solveAllocator(candidates, capacity, balances, 2, 50*time.Millisecond, feeCache)
	if len(choices) != 2 {
		t.Fatalf("want 2 choices, got %d: %+v", len(choices), choices)
	}
	gotSyms := symbolSet(choices)
	wantSyms := []string{"BTCUSDT", "ETHUSDT"}
	for i := range wantSyms {
		if gotSyms[i] != wantSyms[i] {
			t.Fatalf("want symbols %v, got %v", wantSyms, gotSyms)
		}
	}

	result := e.dryRunTransferPlan(choices, balances, feeCache)
	if !result.Feasible {
		t.Fatal("dryRunTransferPlan must stay feasible for the frozen fixture")
	}
	if len(result.Steps) != 1 {
		t.Fatalf("transfer steps = %d, want 1", len(result.Steps))
	}
	step := result.Steps[0]
	if step.From != "binance" || step.To != "bybit" || step.Amount != 50 || step.Fee != 1.25 || step.Chain != "BEP20" {
		t.Fatalf("unexpected transfer step: %+v", step)
	}
	if result.TotalFee != 1.25 {
		t.Fatalf("total fee = %.2f, want 1.25", result.TotalFee)
	}

	entry, ok := feeCache["binance|bybit|BEP20"]
	if !ok || !entry.valid {
		t.Fatalf("fee cache missing valid donor->recipient entry: %+v", feeCache)
	}
	if entry.fee != 1.25 || entry.minWd != 5 {
		t.Fatalf("fee cache entry = %+v, want fee=1.25 minWd=5", entry)
	}
}
