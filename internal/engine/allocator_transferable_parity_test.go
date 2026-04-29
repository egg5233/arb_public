package engine

import (
	"math"
	"reflect"
	"testing"

	"arb/internal/config"
	"arb/internal/risk"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// parityStubbedExchange is a minimal exchange stub for buildTransferableCache tests.
// It does not expose IsUnified(), so rebalanceAvailable returns futures+spot.
type parityStubbedExchange struct {
	exchange.Exchange
	name string
}

func (s *parityStubbedExchange) Name() string                                   { return s.name }
func (s *parityStubbedExchange) SetMetricsCallback(fn exchange.MetricsCallback) {}
func (s *parityStubbedExchange) GetWithdrawFee(coin, chain string) (float64, float64, error) {
	return 0, 1, nil
}
func (s *parityStubbedExchange) WithdrawFeeInclusive() bool { return false }

// parityUnifiedExchange implements IsUnified() = true so rebalanceAvailable returns futures only.
type parityUnifiedExchange struct {
	exchange.Exchange
	name string
}

func (s *parityUnifiedExchange) Name() string                                   { return s.name }
func (s *parityUnifiedExchange) SetMetricsCallback(fn exchange.MetricsCallback) {}
func (s *parityUnifiedExchange) IsUnified() bool                                { return true }
func (s *parityUnifiedExchange) GetWithdrawFee(coin, chain string) (float64, float64, error) {
	return 0, 1, nil
}
func (s *parityUnifiedExchange) WithdrawFeeInclusive() bool { return false }

// newParityEngine builds a minimal Engine for buildTransferableCache tests.
func newParityEngine(t *testing.T, exchanges map[string]exchange.Exchange) *Engine {
	t.Helper()
	cfg := &config.Config{
		MarginL4Threshold: 0.80,
		MarginL5Threshold: 0.95,
		MarginL4Headroom:  0.05,
	}
	return &Engine{
		cfg:       cfg,
		log:       utils.NewLogger("test-parity"),
		exchanges: exchanges,
	}
}

// manualTransferable replicates the donor-loop logic from buildTransferableCache
// (allocator.go:731-762) so we can compare outputs directly.
func manualTransferable(e *Engine, balances map[string]rebalanceBalanceInfo) map[string]float64 {
	transferable := make(map[string]float64, len(balances))
	for recipient := range e.exchanges {
		var totalSurplus float64
		for donor, bal := range balances {
			if donor == recipient {
				continue
			}
			if !e.donorHasHeadroom(bal) {
				continue
			}
			surplus := e.rebalanceAvailable(donor, bal)
			if bal.hasPositions {
				healthCap := e.capByMarginHealth(bal)
				if surplus > healthCap {
					surplus = healthCap
				}
			}
			if surplus > 0 {
				totalSurplus += surplus
			}
		}
		transferable[recipient] = totalSurplus
	}
	return transferable
}

func TestBuildTransferableCache_ParityWithInlineLogic(t *testing.T) {
	// Use a deterministic fixture with 4 exchanges covering all interesting branches:
	// - "alpha":  donor with headroom, no positions → surplus = futures+spot
	// - "beta":   donor with headroom, has positions, healthCap caps surplus
	// - "gateio": donor WITHOUT headroom (usage >= L4) → skipped as donor
	// - "okx":    unified exchange → rebalanceAvailable returns futures only
	exchanges := map[string]exchange.Exchange{
		"alpha":  &parityStubbedExchange{name: "alpha"},
		"beta":   &parityStubbedExchange{name: "beta"},
		"gateio": &parityStubbedExchange{name: "gateio"},
		"okx":    &parityUnifiedExchange{name: "okx"},
	}
	e := newParityEngine(t, exchanges)

	// Build the balances fixture.
	// alpha: futures=300, spot=100, total=500, ratio=0 (usage = 1-300/500 = 0.40 < L4=0.80 → has headroom)
	//   no positions → surplus = futures+spot = 400; healthCap = MaxFloat64 (no positions) → not capped
	// beta: futures=200, spot=50, total=400, marginRatio=0.1, hasPositions=true
	//   usage = 1-200/400 = 0.50 < 0.80 → has headroom
	//   surplus = futures+spot = 250
	//   healthCap (capUsage): frozen=400-200=200; limit=400-200/0.80=400-250=150
	//   healthCap (capMaint): maint=0.1*400=40; limit=400-40/0.95≈400-42.1≈357.9
	//   cap = min(150, 357.9) = 150 > 0 → surplus capped at 150
	// gateio: futures=50, spot=0, total=100
	//   usage = 1-50/100 = 0.50 < 0.80 → wait, need to make this NOT have headroom.
	//   Set futures=10, total=100: usage = 1-10/100 = 0.90 >= 0.80 → NO headroom → skipped
	// okx (unified): futures=200, spot=999(ignored), total=300
	//   usage = 1-200/300 = 0.333 < 0.80 → has headroom
	//   surplus = futures only (IsUnified) = 200; no positions → not capped
	balances := map[string]rebalanceBalanceInfo{
		"alpha": {
			futures:      300,
			spot:         100,
			futuresTotal: 500,
			hasPositions: false,
		},
		"beta": {
			futures:      200,
			spot:         50,
			futuresTotal: 400,
			marginRatio:  0.1,
			hasPositions: true,
		},
		"gateio": {
			futures:      10,
			spot:         0,
			futuresTotal: 100,
			hasPositions: false,
		},
		"okx": {
			futures:      200,
			spot:         999, // ignored for unified
			futuresTotal: 300,
			hasPositions: false,
		},
	}

	// Get result from helper.
	cache := e.buildTransferableCache(balances)
	got := cache.TransferablePerExchange

	// Compute expected using the manual replica.
	want := manualTransferable(e, balances)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildTransferableCache mismatch:\ngot:  %v\nwant: %v", got, want)
	}

	// Spot-check key invariants:

	// gateio has no headroom → its surplus contribution to others should be 0.
	// Every recipient's total should equal sum of eligible donors (alpha, beta, okx minus self).
	for recipient, total := range got {
		if total < 0 {
			t.Errorf("recipient %s has negative transferable: %.4f", recipient, total)
		}
	}

	// alpha is a pure non-unified donor: its contribution to non-alpha recipients
	// should equal rebalanceAvailable("alpha", balances["alpha"]) = futures+spot = 400.
	alphaContrib := 400.0
	for recipient := range exchanges {
		if recipient == "alpha" {
			continue
		}
		// Compute what this recipient's total would be if alpha is the only donor.
		singleDonor := map[string]rebalanceBalanceInfo{"alpha": balances["alpha"]}
		singleExchanges := map[string]exchange.Exchange{
			recipient: exchanges[recipient],
			"alpha":   exchanges["alpha"],
		}
		eSmall := newParityEngine(t, singleExchanges)
		small := eSmall.buildTransferableCache(singleDonor)
		if got, ok := small.TransferablePerExchange[recipient]; ok {
			if math.Abs(got-alphaContrib) > 1e-9 {
				t.Errorf("alpha contribution to %s = %.4f, want %.4f", recipient, got, alphaContrib)
			}
		}
	}

	// beta has positions: its surplus should be capped at healthCap=150, not 250.
	// Verify by checking a single-donor scenario where only beta donates to alpha.
	betaOnlyExchanges := map[string]exchange.Exchange{
		"alpha": exchanges["alpha"],
		"beta":  exchanges["beta"],
	}
	eBeta := newParityEngine(t, betaOnlyExchanges)
	betaCache := eBeta.buildTransferableCache(map[string]rebalanceBalanceInfo{
		"beta": balances["beta"],
	})
	// Expected cap: capUsage = total - frozen/L4 = 400 - (400-200)/0.80 = 400 - 250 = 150
	wantBetaCap := 150.0
	if gotBetaContrib, ok := betaCache.TransferablePerExchange["alpha"]; ok {
		if math.Abs(gotBetaContrib-wantBetaCap) > 1e-6 {
			t.Errorf("beta→alpha capped contribution = %.6f, want %.6f", gotBetaContrib, wantBetaCap)
		}
	}

	// gateio has no headroom: its contribution to any recipient should be 0.
	gateioOnlyExchanges := map[string]exchange.Exchange{
		"alpha":  exchanges["alpha"],
		"gateio": exchanges["gateio"],
	}
	eGateio := newParityEngine(t, gateioOnlyExchanges)
	gateioCache := eGateio.buildTransferableCache(map[string]rebalanceBalanceInfo{
		"gateio": balances["gateio"],
	})
	if contrib := gateioCache.TransferablePerExchange["alpha"]; contrib != 0 {
		t.Errorf("gateio (no headroom) contributed %.4f to alpha, want 0", contrib)
	}

	// okx is unified: its rebalanceAvailable = futures only = 200.
	okxOnlyExchanges := map[string]exchange.Exchange{
		"alpha": exchanges["alpha"],
		"okx":   exchanges["okx"],
	}
	eOkx := newParityEngine(t, okxOnlyExchanges)
	okxCache := eOkx.buildTransferableCache(map[string]rebalanceBalanceInfo{
		"okx": balances["okx"],
	})
	if got, ok := okxCache.TransferablePerExchange["alpha"]; ok {
		if math.Abs(got-200.0) > 1e-9 {
			t.Errorf("okx (unified) contribution to alpha = %.4f, want 200.0", got)
		}
	}

	// Returned cache must have nil Balances and Orderbooks fields (buildTransferableCache
	// only fills TransferablePerExchange).
	if cache.Balances != nil {
		t.Errorf("cache.Balances should be nil, got %v", cache.Balances)
	}
	if cache.Orderbooks != nil {
		t.Errorf("cache.Orderbooks should be nil, got %v", cache.Orderbooks)
	}

	// Confirm the returned value is a *risk.PrefetchCache (type assertion sanity check).
	var _ *risk.PrefetchCache = cache
}

// TestBuildTransferableCache_EmptyBalances verifies an empty balances map
// yields a zero-filled map (one entry per exchange, all 0).
func TestBuildTransferableCache_EmptyBalances(t *testing.T) {
	exchanges := map[string]exchange.Exchange{
		"alpha": &parityStubbedExchange{name: "alpha"},
		"beta":  &parityStubbedExchange{name: "beta"},
	}
	e := newParityEngine(t, exchanges)
	cache := e.buildTransferableCache(map[string]rebalanceBalanceInfo{})

	for recipient, val := range cache.TransferablePerExchange {
		if val != 0 {
			t.Errorf("recipient %s: want 0 with empty balances, got %.4f", recipient, val)
		}
	}
	if len(cache.TransferablePerExchange) != len(exchanges) {
		t.Errorf("len(TransferablePerExchange) = %d, want %d", len(cache.TransferablePerExchange), len(exchanges))
	}
}

// TestBuildTransferableCache_SingleDonorSingleRecipient is a minimal parity check
// with one donor and one recipient, no complications.
func TestBuildTransferableCache_SingleDonorSingleRecipient(t *testing.T) {
	exchanges := map[string]exchange.Exchange{
		"donor":     &parityStubbedExchange{name: "donor"},
		"recipient": &parityStubbedExchange{name: "recipient"},
	}
	e := newParityEngine(t, exchanges)

	balances := map[string]rebalanceBalanceInfo{
		"donor": {futures: 500, spot: 200, futuresTotal: 1000, hasPositions: false},
		// recipient not in balances (zero-value → usage=0 → has headroom but 0 surplus)
	}

	cache := e.buildTransferableCache(balances)
	want := manualTransferable(e, balances)

	if !reflect.DeepEqual(cache.TransferablePerExchange, want) {
		t.Errorf("single-donor parity mismatch:\ngot:  %v\nwant: %v",
			cache.TransferablePerExchange, want)
	}

	// donor → recipient: surplus = futures+spot = 700
	if got := cache.TransferablePerExchange["recipient"]; math.Abs(got-700.0) > 1e-9 {
		t.Errorf("recipient transferable = %.4f, want 700.0", got)
	}
	// donor receiving: the only other entry is "donor" itself; donor has no other donors.
	if got := cache.TransferablePerExchange["donor"]; got != 0 {
		t.Errorf("donor transferable to self-pool = %.4f, want 0 (recipient not in balances = 0 surplus)", got)
	}
}

func TestDryRunTransferPlanUsesMaxTransferOutForRecipientDeficit(t *testing.T) {
	exchanges := map[string]exchange.Exchange{
		"binance": &parityStubbedExchange{name: "binance"},
		"bybit":   &parityUnifiedExchange{name: "bybit"},
		"gateio":  &parityUnifiedExchange{name: "gateio"},
	}
	e := newParityEngine(t, exchanges)
	e.cfg.MarginSafetyMultiplier = 2
	e.cfg.ExchangeAddresses = map[string]map[string]string{
		"bybit": {"BEP20": "test-address"},
	}

	choice := allocatorChoice{
		symbol:              "PTBUSDT",
		longExchange:        "bybit",
		shortExchange:       "gateio",
		requiredMargin:      200,
		longRequiredMargin:  200,
		shortRequiredMargin: 200,
		entryNotional:       300,
	}
	balances := map[string]rebalanceBalanceInfo{
		"binance": {
			futures:        300,
			futuresTotal:   320,
			maxTransferOut: 300,
		},
		"bybit": {
			futures:                     225.43,
			futuresTotal:                225.43,
			maxTransferOut:              52.60,
			maxTransferOutAuthoritative: true,
			hasPositions:                true,
		},
		"gateio": {
			futures:      232.10,
			futuresTotal: 410,
			hasPositions: true,
		},
	}

	result := e.dryRunTransferPlan([]allocatorChoice{choice}, balances, map[string]feeEntry{})
	if !result.Feasible {
		t.Fatalf("dryRunTransferPlan was infeasible")
	}
	var bybitStep *transferStep
	for i := range result.Steps {
		if result.Steps[i].To == "bybit" {
			bybitStep = &result.Steps[i]
			break
		}
	}
	if bybitStep == nil {
		t.Fatalf("expected a transfer step to bybit when maxTransferOut is below required margin")
	}
	if bybitStep.Amount < 140 || bybitStep.Amount > 150 {
		t.Fatalf("bybit transfer amount = %.4f, want about 147.4", bybitStep.Amount)
	}
}
