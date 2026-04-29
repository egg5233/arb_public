package engine

import (
	"errors"
	"math"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
)

func TestAnnotateFailedEntryRollbackPreservesFillEvidence(t *testing.T) {
	pos := &models.ArbitragePosition{
		ID:            "PTBUSDT-1",
		Symbol:        "PTBUSDT",
		LongExchange:  "bybit",
		ShortExchange: "gateio",
		EntrySpread:   3.8,
		CreatedAt:     time.Now().UTC(),
	}

	annotateFailedEntryRollback(pos,
		"depth fills below minimum (PTBUSDT: long=12.000000 short=12.000000)",
		"depth_fill",
		12, 12,
		0.1000, 0.1010,
		0.0995, 0.1015,
	)

	if pos.Status != models.StatusClosed {
		t.Fatalf("status = %q, want closed", pos.Status)
	}
	if pos.LongSize != 12 || pos.ShortSize != 12 {
		t.Fatalf("sizes = %.6f/%.6f, want executed fill sizes", pos.LongSize, pos.ShortSize)
	}
	if pos.LongEntry == 0 || pos.ShortEntry == 0 || pos.LongExit == 0 || pos.ShortExit == 0 {
		t.Fatalf("entry/exit prices not preserved: long %.6f->%.6f short %.6f->%.6f",
			pos.LongEntry, pos.LongExit, pos.ShortEntry, pos.ShortExit)
	}
	if pos.EntryNotional <= 0 {
		t.Fatal("entry notional was not populated")
	}
	if pos.FailureReason == "" || pos.FailureStage != "depth_fill" {
		t.Fatalf("failure metadata missing: reason=%q stage=%q", pos.FailureReason, pos.FailureStage)
	}
	if pos.ExitReason == "" {
		t.Fatal("exit reason was not populated")
	}

	wantPnL := (0.0995-0.1000)*12 + (0.1010-0.1015)*12
	if math.Abs(pos.RealizedPnL-wantPnL) > 1e-9 {
		t.Fatalf("realized pnl = %.10f, want %.10f", pos.RealizedPnL, wantPnL)
	}
	if math.Abs(pos.BasisGainLoss-wantPnL) > 1e-9 {
		t.Fatalf("basis pnl = %.10f, want %.10f", pos.BasisGainLoss, wantPnL)
	}
}

func TestRetrySecondLegDownsizesAndKeepsToppingUpAfterMarginReject(t *testing.T) {
	const refPrice = 1.0
	exch := newFullStub(exchange.BBO{Bid: 0.99, Ask: 1.01}, true)
	attempt := 0
	exch.placeOrderFn = func(p exchange.PlaceOrderParams) (string, error) {
		attempt++
		if attempt == 1 {
			return "", errors.New("bybit API error code=110007 msg=ab not enough for new order")
		}
		oid := "oid-downsized-" + p.Size
		if p.Size != "50.000000" {
			t.Fatalf("retry size = %q, want 50.000000", p.Size)
		}
		exch.storeOrder(oid, exchange.OrderUpdate{
			OrderID:      oid,
			Status:       "filled",
			FilledVolume: 50,
			AvgPrice:     1.01,
			ReduceOnly:   false,
			Symbol:       "PTBUSDT",
		})
		return oid, nil
	}
	exch.getFilledFn = func(orderID, symbol string) (float64, error) {
		return 50, nil
	}

	e := newMinimalEngine()
	e.cfg = &config.Config{SlippageBPS: 10, MinChunkUSDT: 10}
	e.exchanges["bybit"] = exch

	filled, avg, err := e.retrySecondLeg(exch, "bybit", "PTBUSDT", exchange.SideBuy, 100, refPrice, "bybit", "gateio")
	if err != nil {
		t.Fatalf("retrySecondLeg returned error: %v", err)
	}
	if filled != 100 {
		t.Fatalf("filled = %.6f, want 100", filled)
	}
	if math.Abs(avg-1.01) > 1e-9 {
		t.Fatalf("avg = %.8f, want 1.01", avg)
	}
	if attempt != 3 {
		t.Fatalf("attempts = %d, want 3", attempt)
	}
}
