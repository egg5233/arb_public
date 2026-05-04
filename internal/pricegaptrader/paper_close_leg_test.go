package pricegaptrader

import (
	"testing"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
)

// TestCloseLegMarket_PaperMode_NoExchangeCall is a regression test for the
// 2026-04-24 UAT incident. With PriceGapPaperMode=true placeLeg correctly
// synthesized fills without hitting exchanges, but openPair's unwind-to-match
// path called closeLegMarket which invoked ex.PlaceOrder directly — sending
// real reduceOnly market orders (bingx 101290) against positions the
// exchange had never actually seen. This violated D-12 / Pitfall 2
// "paper mode is a single chokepoint at ex.PlaceOrder". Gate it here.
func TestCloseLegMarket_PaperMode_NoExchangeCall(t *testing.T) {
	stub := newStubExchange("gateio")
	tr := NewTracker(
		map[string]exchange.Exchange{"gateio": stub},
		newFakeStore(),
		newFakeDelistChecker(),
		&config.Config{
			PriceGapEnabled:   true,
			PriceGapPaperMode: true,
		},
	)

	tr.closeLegMarket(stub, "SOONUSDT", exchange.SideSell, 100, 2)

	placed := stub.placedOrders()
	if len(placed) != 0 {
		t.Fatalf("paper mode must not call PlaceOrder; got %d calls: %+v", len(placed), placed)
	}
}

// TestCloseLegMarket_LiveMode_DoesCall — negative control for the paper-mode
// gate. In live mode (PriceGapPaperMode=false) closeLegMarket must still
// reach ex.PlaceOrder with the expected params.
func TestCloseLegMarket_LiveMode_DoesCall(t *testing.T) {
	stub := newStubExchange("gateio")
	tr := NewTracker(
		map[string]exchange.Exchange{"gateio": stub},
		newFakeStore(),
		newFakeDelistChecker(),
		&config.Config{
			PriceGapEnabled:   true,
			PriceGapPaperMode: false,
		},
	)

	tr.closeLegMarket(stub, "SOONUSDT", exchange.SideSell, 100, 2)

	placed := stub.placedOrders()
	if len(placed) != 1 {
		t.Fatalf("live mode must call PlaceOrder once; got %d", len(placed))
	}
	p := placed[0]
	if p.Symbol != "SOONUSDT" || p.Side != exchange.SideSell ||
		p.OrderType != "market" || !p.ReduceOnly {
		t.Fatalf("params wrong: %+v", p)
	}
}

func TestCloseLegMarketForPos_LivePositionIgnoresGlobalPaperFlag(t *testing.T) {
	stub := newStubExchange("gateio")
	tr := NewTracker(
		map[string]exchange.Exchange{"gateio": stub},
		newFakeStore(),
		newFakeDelistChecker(),
		&config.Config{
			PriceGapEnabled:   true,
			PriceGapPaperMode: true,
		},
	)
	pos := &models.PriceGapPosition{Symbol: "SOONUSDT", Mode: models.PriceGapModeLive}

	tr.closeLegMarketForPos(stub, pos, exchange.SideSell, 100, 2, 1)

	if len(stub.placedOrders()) != 1 {
		t.Fatalf("live position close must call PlaceOrder despite global paper flag; got %d", len(stub.placedOrders()))
	}
}

// Compile-time sanity — NewTracker accepts the full dependency set; a
// configuration error here would catch us during edit rather than at the
// deploy edge.
var _ models.PriceGapStore = (*fakeStore)(nil)
