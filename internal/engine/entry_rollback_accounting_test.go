package engine

import (
	"errors"
	"math"
	"testing"

	"arb/internal/models"
	"arb/pkg/exchange"
)

func TestRollbackEntryLegWithAccountingRecordsLongLoss(t *testing.T) {
	exch := newFullStub(exchange.BBO{}, false)
	exch.placeOrderFn = func(p exchange.PlaceOrderParams) (string, error) {
		if p.Side != exchange.SideSell || p.OrderType != "market" || !p.ReduceOnly {
			t.Fatalf("rollback order params = %+v", p)
		}
		exch.storeOrder("close-1", exchange.OrderUpdate{OrderID: "close-1", Status: "filled", FilledVolume: 1, AvgPrice: 77.56})
		return "close-1", nil
	}

	e := newMinimalEngine()
	e.contracts = map[string]map[string]exchange.ContractInfo{
		"stub": {"HOODUSDT": {MinSize: 0.1, StepSize: 0.1, SizeDecimals: 1}},
	}
	pos := &models.ArbitragePosition{ID: "hoodusdt-test", Symbol: "HOODUSDT"}

	rem := e.rollbackEntryLegWithAccounting(pos, exch, "HOODUSDT", exchange.SideSell, 1, 77.59, "long")
	if rem != 0 {
		t.Fatalf("remaining = %.6f, want 0", rem)
	}
	if !pos.HasReconciled {
		t.Fatal("HasReconciled = false, want true")
	}
	if pos.PartialReconcile {
		t.Fatal("PartialReconcile = true, want false")
	}
	wantPnL := -0.03
	if math.Abs(pos.RealizedPnL-wantPnL) > 1e-9 {
		t.Fatalf("RealizedPnL = %.12f, want %.12f", pos.RealizedPnL, wantPnL)
	}
	if math.Abs(pos.LongClosePnL-wantPnL) > 1e-9 {
		t.Fatalf("LongClosePnL = %.12f, want %.12f", pos.LongClosePnL, wantPnL)
	}
	if math.Abs(pos.BasisGainLoss-wantPnL) > 1e-9 {
		t.Fatalf("BasisGainLoss = %.12f, want %.12f", pos.BasisGainLoss, wantPnL)
	}
	if pos.LongExit != 77.56 {
		t.Fatalf("LongExit = %.8f, want 77.56", pos.LongExit)
	}
}

func TestRollbackEntryLegWithAccountingMarksPartialWhenCloseIncomplete(t *testing.T) {
	exch := newFullStub(exchange.BBO{}, false)
	called := false
	exch.placeOrderFn = func(p exchange.PlaceOrderParams) (string, error) {
		if called {
			return "", errors.New("stop after first partial close")
		}
		called = true
		exch.storeOrder("close-1", exchange.OrderUpdate{OrderID: "close-1", Status: "filled", FilledVolume: 0.4, AvgPrice: 77.56})
		return "close-1", nil
	}

	e := newMinimalEngine()
	e.contracts = map[string]map[string]exchange.ContractInfo{
		"stub": {"HOODUSDT": {MinSize: 0.1, StepSize: 0.1, SizeDecimals: 1}},
	}
	pos := &models.ArbitragePosition{ID: "hoodusdt-test", Symbol: "HOODUSDT"}

	rem := e.rollbackEntryLegWithAccounting(pos, exch, "HOODUSDT", exchange.SideSell, 1, 77.59, "long")
	if rem <= 0 {
		t.Fatalf("remaining = %.6f, want positive", rem)
	}
	if pos.HasReconciled {
		t.Fatal("HasReconciled = true, want false for incomplete close")
	}
	if !pos.PartialReconcile {
		t.Fatal("PartialReconcile = false, want true")
	}
	if pos.RealizedPnL >= 0 {
		t.Fatalf("RealizedPnL = %.12f, want negative partial loss", pos.RealizedPnL)
	}
}
